package entity

import (
	"math"
	"time"
)

// MediaSegment 媒体段
type MediaSegment struct {
	Index        int64        `json:"Index"`
	Duration     float64      `json:"Duration"`
	Title        string       `json:"Title,omitempty"`
	DateTime     *time.Time   `json:"DateTime,omitempty"`
	StartRange   *int64       `json:"StartRange,omitempty"`
	ExpectLength *int64       `json:"ExpectLength,omitempty"`
	EncryptInfo  *EncryptInfo `json:"EncryptInfo"`
	IsEncrypted  bool         `json:"IsEncrypted"`
	URL          string       `json:"Url"`
	NameFromVar  string       `json:"NameFromVar,omitempty"` // MPD分段文件名
}

// NewMediaSegment 创建新的媒体段
func NewMediaSegment() *MediaSegment {
	encryptInfo := NewEncryptInfo()
	return &MediaSegment{
		EncryptInfo: encryptInfo,
		IsEncrypted: encryptInfo.IsEncrypted(),
	}
}

// GetStopRange 获取结束范围
func (m *MediaSegment) GetStopRange() *int64 {
	if m.StartRange != nil && m.ExpectLength != nil {
		stop := *m.StartRange + *m.ExpectLength - 1
		return &stop
	}
	return nil
}

// Equals 比较两个媒体段是否相等
func (m *MediaSegment) Equals(other *MediaSegment) bool {
	if other == nil {
		return false
	}

	return m.Index == other.Index &&
		math.Abs(m.Duration-other.Duration) < 0.001 &&
		m.Title == other.Title &&
		m.StartRange == other.StartRange &&
		m.GetStopRange() == other.GetStopRange() &&
		m.ExpectLength == other.ExpectLength &&
		m.URL == other.URL
}
