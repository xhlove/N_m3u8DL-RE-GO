package util

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// ANSI颜色代码
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Italic = "\033[3m"
	Under  = "\033[4m"

	// 前景色
	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"

	// 背景色
	BgBlack   = "\033[40m"
	BgRed     = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
	BgWhite   = "\033[47m"

	// 高亮色
	BrightBlack   = "\033[90m"
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"
)

// ConsoleManager 控制台管理器
type ConsoleManager struct {
	supportColors    bool
	forceAnsiConsole bool
	noAnsiColor      bool
}

var Console *ConsoleManager

func init() {
	Console = &ConsoleManager{
		supportColors: supportsColors(),
	}
}

// InitConsole 初始化控制台
func InitConsole(forceAnsiConsole, noAnsiColor bool) {
	Console.forceAnsiConsole = forceAnsiConsole
	Console.noAnsiColor = noAnsiColor

	if forceAnsiConsole {
		Console.supportColors = true
	}

	if noAnsiColor {
		Console.supportColors = false
	}
}

// supportsColors 检测控制台是否支持颜色
func supportsColors() bool {
	// 在Windows上，现代终端通常支持ANSI
	if runtime.GOOS == "windows" {
		// Windows Terminal, ConEmu, 以及Windows 10 1903+的cmd都支持ANSI
		// 检查环境变量
		if os.Getenv("WT_SESSION") != "" {
			return true // Windows Terminal
		}
		if os.Getenv("ConEmuPID") != "" {
			return true // ConEmu
		}
		if os.Getenv("ANSICON") != "" {
			return true // ANSICON
		}
		// 检查TERM环境变量
		if term := os.Getenv("TERM"); term != "" {
			return true
		}
		// 现代Windows系统默认支持ANSI，直接返回true
		return true
	}

	// Unix-like系统通常支持ANSI颜色
	return true
}

// Colorize 给文本着色
func (c *ConsoleManager) Colorize(text, color string) string {
	if !c.supportColors {
		return text
	}
	return color + text + Reset
}

// MarkupLine 输出带标记的行
func (c *ConsoleManager) MarkupLine(text string) {
	if c.supportColors {
		text = c.parseMarkup(text)
	} else {
		text = c.removeMarkup(text)
	}
	fmt.Println(text)
}

// Markup 输出带标记的文本（不换行）
func (c *ConsoleManager) Markup(text string) {
	if c.supportColors {
		text = c.parseMarkup(text)
	} else {
		text = c.removeMarkup(text)
	}
	fmt.Print(text)
}

// parseMarkup 解析类似Spectre.Console的标记语法
func (c *ConsoleManager) parseMarkup(text string) string {
	// 简化的标记解析，支持常用的颜色标记
	replacements := map[string]string{
		"[red]":                    Red,
		"[/]":                      Reset,
		"[green]":                  Green,
		"[blue]":                   Blue,
		"[yellow]":                 Yellow,
		"[cyan]":                   Cyan,
		"[magenta]":                Magenta,
		"[white]":                  White,
		"[grey]":                   BrightBlack,
		"[underline]":              Under,
		"[bold]":                   Bold,
		"[red1]":                   BrightRed,
		"[deepskyblue1]":           BrightCyan,
		"[darkorange3_1]":          Yellow,
		"[#548c26]":                Green,
		"[#a89022]":                Yellow,
		"[underline #548c26]":      Under + Green,
		"[underline #a89022]":      Under + Yellow,
		"[underline red1]":         Under + BrightRed,
		"[underline grey]":         Under + BrightBlack,
		"[white on green]":         BgGreen + White,
		"[white on red]":           BgRed + White,
		"[white on darkorange3_1]": BgYellow + White,
	}

	for markup, ansi := range replacements {
		text = strings.ReplaceAll(text, markup, ansi)
	}

	return text
}

// removeMarkup 移除标记语法
func (c *ConsoleManager) removeMarkup(text string) string {
	// 移除所有 [xxx] 标记
	for {
		start := strings.Index(text, "[")
		if start == -1 {
			break
		}
		end := strings.Index(text[start:], "]")
		if end == -1 {
			break
		}
		text = text[:start] + text[start+end+1:]
	}
	return text
}

// ProgressStyle 进度条样式
type ProgressStyle struct {
	BarChar    string
	EmptyChar  string
	BarColor   string
	EmptyColor string
	TextColor  string
	Width      int
}

// DefaultProgressStyle 默认进度条样式
func DefaultProgressStyle() ProgressStyle {
	return ProgressStyle{
		BarChar:    "█",
		EmptyChar:  "░",
		BarColor:   Green,
		EmptyColor: BrightBlack,
		TextColor:  White,
		Width:      50,
	}
}

// RenderProgressBar 渲染进度条
func (c *ConsoleManager) RenderProgressBar(current, total float64, style ProgressStyle) string {
	percentage := current / total
	if percentage > 1.0 {
		percentage = 1.0
	}
	if percentage < 0.0 {
		percentage = 0.0
	}

	filledWidth := int(percentage * float64(style.Width))
	emptyWidth := style.Width - filledWidth

	var bar strings.Builder

	if c.supportColors {
		// 添加已填充部分
		if filledWidth > 0 {
			bar.WriteString(style.BarColor)
			bar.WriteString(strings.Repeat(style.BarChar, filledWidth))
		}

		// 添加空白部分
		if emptyWidth > 0 {
			bar.WriteString(style.EmptyColor)
			bar.WriteString(strings.Repeat(style.EmptyChar, emptyWidth))
		}

		bar.WriteString(Reset)
	} else {
		// 无颜色版本
		bar.WriteString(strings.Repeat("=", filledWidth))
		bar.WriteString(strings.Repeat("-", emptyWidth))
	}

	return bar.String()
}

// GetCurrentTime 获取当前时间字符串
func GetCurrentTime() string {
	return time.Now().Format("15:04:05.000")
}

// EscapeMarkup 转义标记字符
func EscapeMarkup(text string) string {
	text = strings.ReplaceAll(text, "[", "[[")
	text = strings.ReplaceAll(text, "]", "]]")
	return text
}
