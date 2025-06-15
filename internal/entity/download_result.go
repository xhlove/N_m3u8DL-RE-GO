package entity

import "time"

// DownloadResult 下载结果
type DownloadResult struct {
	// 实际下载时间
	ActualFileSizeB int64 `json:"actual_file_size_b"`

	// 文件大小（字节）
	FileSizeB int64 `json:"file_size_b"`

	// 成功标记
	Success bool `json:"success"`

	// 消息
	Message string `json:"message,omitempty"`

	// 下载开始时间
	StartTime time.Time `json:"start_time"`

	// 下载结束时间
	EndTime time.Time `json:"end_time"`

	// 下载用时
	Duration time.Duration `json:"duration"`

	// 下载速度（字节/秒）
	AvgSpeed int64 `json:"avg_speed"`

	// 重试次数
	RetryCount int `json:"retry_count"`
}

// NewDownloadResult 创建新的下载结果
func NewDownloadResult() *DownloadResult {
	return &DownloadResult{
		Success:    false,
		StartTime:  time.Now(),
		RetryCount: 0,
	}
}

// CalculateSpeed 计算平均下载速度
func (d *DownloadResult) CalculateSpeed() {
	if d.Duration > 0 {
		d.AvgSpeed = int64(float64(d.ActualFileSizeB) / d.Duration.Seconds())
	}
}

// SetSuccess 设置成功状态
func (d *DownloadResult) SetSuccess(fileSize int64) {
	d.Success = true
	d.EndTime = time.Now()
	d.Duration = d.EndTime.Sub(d.StartTime)
	d.ActualFileSizeB = fileSize
	d.FileSizeB = fileSize
	d.CalculateSpeed()
}

// SetFailure 设置失败状态
func (d *DownloadResult) SetFailure(message string) {
	d.Success = false
	d.Message = message
	d.EndTime = time.Now()
	d.Duration = d.EndTime.Sub(d.StartTime)
}
