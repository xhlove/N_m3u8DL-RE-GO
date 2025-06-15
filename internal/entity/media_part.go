package entity

// MediaPart 媒体部分
type MediaPart struct {
	MediaSegments []*MediaSegment `json:"mediaSegments"`
}

// NewMediaPart 创建新的媒体部分
func NewMediaPart() *MediaPart {
	return &MediaPart{
		MediaSegments: make([]*MediaSegment, 0),
	}
}

// AddSegment 添加媒体段
func (m *MediaPart) AddSegment(segment *MediaSegment) {
	m.MediaSegments = append(m.MediaSegments, segment)
}

// GetDuration 获取总时长
func (m *MediaPart) GetDuration() float64 {
	var total float64
	for _, segment := range m.MediaSegments {
		total += segment.Duration
	}
	return total
}

// GetSegmentCount 获取段数量
func (m *MediaPart) GetSegmentCount() int {
	return len(m.MediaSegments)
}
