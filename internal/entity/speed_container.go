package entity

import (
	"fmt"
	"sync"
	"time"
)

// SpeedContainer 速度统计容器
type SpeedContainer struct {
	// 互斥锁
	mutex sync.RWMutex

	// 速度记录
	speedRecords []SpeedRecord

	// 总下载字节数
	totalBytes int64

	// 开始时间
	startTime time.Time

	// 最大记录数
	maxRecords int
}

// SpeedRecord 速度记录
type SpeedRecord struct {
	Timestamp time.Time
	Bytes     int64
	Speed     int64 // 字节/秒
}

// NewSpeedContainer 创建新的速度容器
func NewSpeedContainer(maxRecords int) *SpeedContainer {
	if maxRecords <= 0 {
		maxRecords = 100 // 默认最大100条记录
	}

	return &SpeedContainer{
		speedRecords: make([]SpeedRecord, 0, maxRecords),
		maxRecords:   maxRecords,
		startTime:    time.Now(),
	}
}

// AddBytes 添加下载字节数
func (sc *SpeedContainer) AddBytes(bytes int64) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	now := time.Now()
	sc.totalBytes += bytes

	// 计算即时速度（基于最近的记录）
	var speed int64
	if len(sc.speedRecords) > 0 {
		lastRecord := sc.speedRecords[len(sc.speedRecords)-1]
		duration := now.Sub(lastRecord.Timestamp)
		if duration.Seconds() > 0 {
			speed = int64(float64(bytes) / duration.Seconds())
		}
	}

	record := SpeedRecord{
		Timestamp: now,
		Bytes:     bytes,
		Speed:     speed,
	}

	sc.speedRecords = append(sc.speedRecords, record)

	// 如果超过最大记录数，删除最旧的记录
	if len(sc.speedRecords) > sc.maxRecords {
		sc.speedRecords = sc.speedRecords[1:]
	}
}

// GetCurrentSpeed 获取当前下载速度（基于最近几秒的平均值）
func (sc *SpeedContainer) GetCurrentSpeed() int64 {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	if len(sc.speedRecords) < 2 {
		return 0
	}

	now := time.Now()
	windowSize := 5 * time.Second // 计算最近5秒的平均速度

	var totalBytes int64
	var oldestTime time.Time
	recordCount := 0

	// 从最新记录开始，向前查找5秒内的记录
	for i := len(sc.speedRecords) - 1; i >= 0; i-- {
		record := sc.speedRecords[i]
		if now.Sub(record.Timestamp) > windowSize {
			break
		}
		totalBytes += record.Bytes
		oldestTime = record.Timestamp
		recordCount++
	}

	if recordCount < 2 {
		return 0
	}

	duration := now.Sub(oldestTime)
	if duration.Seconds() <= 0 {
		return 0
	}

	return int64(float64(totalBytes) / duration.Seconds())
}

// GetAverageSpeed 获取平均下载速度
func (sc *SpeedContainer) GetAverageSpeed() int64 {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	if sc.totalBytes == 0 {
		return 0
	}

	duration := time.Since(sc.startTime)
	if duration.Seconds() <= 0 {
		return 0
	}

	return int64(float64(sc.totalBytes) / duration.Seconds())
}

// GetTotalBytes 获取总下载字节数
func (sc *SpeedContainer) GetTotalBytes() int64 {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()
	return sc.totalBytes
}

// GetDuration 获取下载持续时间
func (sc *SpeedContainer) GetDuration() time.Duration {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()
	return time.Since(sc.startTime)
}

// Reset 重置速度统计
func (sc *SpeedContainer) Reset() {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	sc.speedRecords = make([]SpeedRecord, 0, sc.maxRecords)
	sc.totalBytes = 0
	sc.startTime = time.Now()
}

// GetSpeedHistory 获取速度历史记录
func (sc *SpeedContainer) GetSpeedHistory() []SpeedRecord {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	// 返回副本，避免并发修改
	history := make([]SpeedRecord, len(sc.speedRecords))
	copy(history, sc.speedRecords)
	return history
}

// FormatSpeed 格式化速度显示
func FormatSpeed(bytesPerSecond int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytesPerSecond >= GB:
		return fmt.Sprintf("%.2f GB/s", float64(bytesPerSecond)/GB)
	case bytesPerSecond >= MB:
		return fmt.Sprintf("%.2f MB/s", float64(bytesPerSecond)/MB)
	case bytesPerSecond >= KB:
		return fmt.Sprintf("%.2f KB/s", float64(bytesPerSecond)/KB)
	default:
		return fmt.Sprintf("%d B/s", bytesPerSecond)
	}
}
