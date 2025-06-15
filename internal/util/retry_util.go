package util

import (
	"fmt"
	"time"
)

// RetryFunc 重试函数类型
type RetryFunc func() error

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries int
	RetryDelay time.Duration
	Backoff    float64 // 退避因子
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = RetryConfig{
	MaxRetries: 3,
	RetryDelay: time.Second,
	Backoff:    2.0,
}

// DoRetry 执行带重试的操作
func DoRetry(fn RetryFunc, config RetryConfig) error {
	var lastErr error
	delay := config.RetryDelay

	for i := 0; i <= config.MaxRetries; i++ {
		if i > 0 {
			Logger.Warn(fmt.Sprintf("第 %d 次重试，延迟 %v", i, delay))
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * config.Backoff)
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// 检查是否是不应重试的错误
		if nonRetryableErr, ok := err.(*NonRetryableHTTPError); ok {
			Logger.Error(fmt.Sprintf("遇到不可重试错误: %s", nonRetryableErr.Message))
			return err
		}

		Logger.Warn(fmt.Sprintf("操作失败: %s", err.Error()))
	}

	return fmt.Errorf("重试 %d 次后仍然失败: %w", config.MaxRetries, lastErr)
}

// DoRetryWithDefault 使用默认配置执行带重试的操作
func DoRetryWithDefault(fn RetryFunc) error {
	return DoRetry(fn, DefaultRetryConfig)
}
