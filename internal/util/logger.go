package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelOff
)

// String 返回日志级别的字符串表示
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelOff:
		return "OFF"
	default:
		return "UNKNOWN"
	}
}

// LogManager 日志管理器
type LogManager struct {
	Level       LogLevel
	IsWriteFile bool
	LogFilePath string
	logMutex    sync.RWMutex
	isUIActive  bool
}

// Logger 全局日志实例
var Logger *LogManager

func init() {
	Logger = &LogManager{
		Level:       LogLevelInfo,
		IsWriteFile: true,
	}
}

// SetUIActive a flag to redirect logs to the UIManager
func (l *LogManager) SetUIActive(active bool) {
	l.isUIActive = active
}

// SetLogLevel 设置日志级别
func SetLogLevel(level LogLevel) {
	Logger.Level = level
}

// InitLogFile 初始化日志文件
func (l *LogManager) InitLogFile() error {
	if !l.IsWriteFile {
		return nil
	}

	var logDir string
	if l.LogFilePath != "" {
		logDir = filepath.Dir(l.LogFilePath)
	} else {
		exePath, err := os.Executable()
		if err != nil {
			logDir = "Logs"
		} else {
			logDir = filepath.Join(filepath.Dir(exePath), "Logs")
		}
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	if l.LogFilePath == "" {
		now := time.Now()
		l.LogFilePath = filepath.Join(logDir, now.Format("2006-01-02_15-04-05-000")+".log")
		index := 1
		baseFileName := strings.TrimSuffix(l.LogFilePath, ".log")
		for {
			if _, err := os.Stat(l.LogFilePath); os.IsNotExist(err) {
				break
			}
			l.LogFilePath = fmt.Sprintf("%s-%d.log", baseFileName, index)
			index++
		}
	}

	now := time.Now()
	initContent := fmt.Sprintf("LOG %s\n", now.Format("2006/01/02"))
	initContent += fmt.Sprintf("Save Path: %s\n", filepath.Dir(l.LogFilePath))
	initContent += fmt.Sprintf("Task Start: %s\n", now.Format("2006/01/02 15:04:05"))
	initContent += fmt.Sprintf("Task CommandLine: %s\n\n", strings.Join(os.Args, " "))

	return os.WriteFile(l.LogFilePath, []byte(initContent), 0644)
}

func (l *LogManager) writeToFile(content string) {
	if !l.IsWriteFile || l.LogFilePath == "" {
		return
	}
	l.logMutex.Lock()
	defer l.logMutex.Unlock()
	file, err := os.OpenFile(l.LogFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	plainContent := Console.removeMarkup(content)
	file.WriteString(plainContent + "\n")
}

func getCurrentTime() string {
	return time.Now().Format("15:04:05.000")
}

func replaceVars(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

func (l *LogManager) handleLog(markup, plain string) {
	if l.isUIActive {
		// The new UI.Log handles printing and redrawing atomically.
		// We pass the markup directly.
		UI.Log(markup)
	} else {
		if markup != "" {
			Console.MarkupLine(markup)
		} else {
			fmt.Println(plain)
		}
	}

	content := markup
	if content == "" {
		content = plain
	}
	l.writeToFile(content)
}

// Debug 输出调试日志
func (l *LogManager) Debug(format string, args ...interface{}) {
	if l.Level > LogLevelDebug {
		return
	}
	message := replaceVars(format, args...)
	timeStr := getCurrentTime()
	plain := fmt.Sprintf("%s DEBUG: %s", timeStr, message)
	markup := fmt.Sprintf("%s [underline grey]DEBUG[/]: %s", timeStr, message)
	l.handleLog(markup, plain)
}

// DebugMarkUp 输出带标记的调试日志
func (l *LogManager) DebugMarkUp(format string, args ...interface{}) {
	if l.Level > LogLevelDebug {
		return
	}
	message := replaceVars(format, args...)
	timeStr := getCurrentTime()
	markup := fmt.Sprintf("%s [underline grey]DEBUG[/]: %s", timeStr, message)
	l.handleLog(markup, "")
}

// Info 输出信息日志
func (l *LogManager) Info(format string, args ...interface{}) {
	if l.Level > LogLevelInfo {
		return
	}
	message := replaceVars(format, args...)
	timeStr := getCurrentTime()
	plain := fmt.Sprintf("%s INFO : %s", timeStr, message)
	markup := fmt.Sprintf("%s [underline #548c26]INFO[/] : %s", timeStr, message)
	l.handleLog(markup, plain)
}

// InfoMarkUp 输出带标记的信息日志
func (l *LogManager) InfoMarkUp(format string, args ...interface{}) {
	if l.Level > LogLevelInfo {
		return
	}
	message := replaceVars(format, args...)
	timeStr := getCurrentTime()
	markup := fmt.Sprintf("%s [underline #548c26]INFO[/] : %s", timeStr, message)
	l.handleLog(markup, "")
}

// Warn 输出警告日志
func (l *LogManager) Warn(format string, args ...interface{}) {
	if l.Level > LogLevelWarn {
		return
	}
	message := replaceVars(format, args...)
	timeStr := getCurrentTime()
	plain := fmt.Sprintf("%s WARN : %s", timeStr, message)
	markup := fmt.Sprintf("%s [underline #a89022]WARN[/] : %s", timeStr, message)
	l.handleLog(markup, plain)
}

// WarnMarkUp 输出带标记的警告日志
func (l *LogManager) WarnMarkUp(format string, args ...interface{}) {
	if l.Level > LogLevelWarn {
		return
	}
	message := replaceVars(format, args...)
	timeStr := getCurrentTime()
	markup := fmt.Sprintf("%s [underline #a89022]WARN[/] : %s", timeStr, message)
	l.handleLog(markup, "")
}

// Error 输出错误日志
func (l *LogManager) Error(format string, args ...interface{}) {
	if l.Level > LogLevelError {
		return
	}
	message := replaceVars(format, args...)
	timeStr := getCurrentTime()
	plain := fmt.Sprintf("%s ERROR: %s", timeStr, message)
	markup := fmt.Sprintf("%s [underline red1]ERROR[/]: %s", timeStr, message)
	l.handleLog(markup, plain)
}

// ErrorMarkUp 输出带标记的错误日志
func (l *LogManager) ErrorMarkUp(format string, args ...interface{}) {
	if l.Level > LogLevelError {
		return
	}
	message := replaceVars(format, args...)
	timeStr := getCurrentTime()
	markup := fmt.Sprintf("%s [underline red1]ERROR[/]: %s", timeStr, message)
	l.handleLog(markup, "")
}

// Extra 仅写入文件的额外日志
func (l *LogManager) Extra(format string, args ...interface{}) {
	if !l.IsWriteFile || l.LogFilePath == "" {
		return
	}
	message := replaceVars(format, args...)
	timeStr := getCurrentTime()
	content := fmt.Sprintf("%s EXTRA: %s", timeStr, message)
	l.writeToFile(content)
}
