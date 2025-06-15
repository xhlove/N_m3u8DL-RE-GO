package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// readFile 读取文件内容
func readFile(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(file)
}

// WriteFile 写入文件
func WriteFile(filePath string, data []byte) error {
	return os.WriteFile(filePath, data, 0644)
}

// FileExists 检查文件是否存在
func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// CreateDir 创建目录
func CreateDir(dirPath string) error {
	return os.MkdirAll(dirPath, 0755)
}

// RemoveFile 删除文件
func RemoveFile(filePath string) error {
	return os.Remove(filePath)
}

// RemoveDir 删除目录
func RemoveDir(dirPath string) error {
	return os.RemoveAll(dirPath)
}

// GetFileSize 获取文件大小
func GetFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// FormatFileSize 格式化文件大小，类似C#版本的GlobalUtil.FormatFileSize
func FormatFileSize(bytes int64) string {
	if bytes < 0 {
		return "0 B"
	}

	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp+1])
}

// FindExecutable 查找可执行文件，类似C#版本的GlobalUtil.FindExecutable
func FindExecutable(name string) string {
	// 重要修复：参考C#版本逻辑，按照正确的搜索顺序
	// C#版本搜索顺序：1.当前目录 2.程序目录 3.PATH环境变量

	// 确定文件扩展名
	fileExt := ""
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		fileExt = ".exe"
	}

	// 构建搜索路径：参考C#版本第69行逻辑
	var searchPaths []string

	// 1. 当前工作目录
	if currentDir, err := os.Getwd(); err == nil {
		searchPaths = append(searchPaths, currentDir)
	}

	// 2. 程序所在目录 - 重要修复：这是Go版本缺失的关键逻辑
	if execPath, err := os.Executable(); err == nil {
		if execDir := filepath.Dir(execPath); execDir != "" {
			searchPaths = append(searchPaths, execDir)
		}
	}

	// 3. PATH环境变量中的目录
	if pathEnv := os.Getenv("PATH"); pathEnv != "" {
		pathDirs := strings.Split(pathEnv, string(os.PathListSeparator))
		searchPaths = append(searchPaths, pathDirs...)
	}

	// 在所有搜索路径中查找可执行文件
	for _, dir := range searchPaths {
		if dir == "" {
			continue
		}

		fullPath := filepath.Join(dir, name+fileExt)
		if FileExists(fullPath) {
			return fullPath
		}
	}

	// 如果都找不到，返回原名称（可能在PATH中但检测不到）
	return name
}
