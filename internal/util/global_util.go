package util

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"N_m3u8DL-RE-GO/internal/entity"
)

// GlobalUtil 全局工具类，对应C#版本的GlobalUtil
type GlobalUtil struct{}

// ConvertToJSON 将对象转换为JSON字符串，类似C#版本的GlobalUtil.ConvertToJson
func ConvertToJSON(obj interface{}) string {
	switch v := obj.(type) {
	case *entity.StreamSpec:
		if data, err := json.MarshalIndent(v, "", "  "); err == nil {
			return string(data)
		}
	case []*entity.StreamSpec:
		if data, err := json.MarshalIndent(v, "", "  "); err == nil {
			return string(data)
		}
	case []*entity.MediaSegment:
		if data, err := json.MarshalIndent(v, "", "  "); err == nil {
			return string(data)
		}
	default:
		if data, err := json.MarshalIndent(obj, "", "  "); err == nil {
			return string(data)
		}
	}
	return "{NOT SUPPORTED}"
}

// FormatTime 格式化时间显示，类似C#版本的GlobalUtil.FormatTime
func FormatTime(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}

	duration := time.Duration(seconds) * time.Second
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	secs := int(duration.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%02dh%02dm%02ds", hours, minutes, secs)
	}
	return fmt.Sprintf("%02dm%02ds", minutes, secs)
}

// FormatDuration 格式化时间段显示
func FormatDuration(duration time.Duration) string {
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%02dh%02dm%02ds", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02dm%02ds", minutes, seconds)
}

// FormatTimeSpan 格式化时间跨度 (从秒数开始)
func FormatTimeSpan(totalSeconds float64) string {
	if totalSeconds < 0 {
		totalSeconds = 0
	}

	seconds := int(totalSeconds)
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%02d:%02d", minutes, secs)
}

// GetCurrentTimeString 获取当前时间字符串
func GetCurrentTimeString() string {
	return time.Now().Format("15:04:05.000")
}

// GetDateTimeString 获取日期时间字符串
func GetDateTimeString() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// ParseTimeDuration 解析时间字符串为Duration（避免与complex_param_parser.go冲突）
func ParseTimeDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

// SanitizeFileName 清理文件名中的非法字符
func SanitizeFileName(filename string) string {
	// Windows文件名非法字符: < > : " | ? * \ /
	// 其他非法字符
	illegal := []string{"<", ">", ":", "\"", "|", "?", "*", "\\", "/"}
	result := filename

	for _, char := range illegal {
		result = strings.ReplaceAll(result, char, "_")
	}

	return result
}

// Min 返回两个整数的最小值
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max 返回两个整数的最大值
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// StrToInt 将字符串转换为整数
func StrToInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// MinInt64 返回两个int64的最小值
func MinInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// MaxInt64 返回两个int64的最大值
func MaxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
