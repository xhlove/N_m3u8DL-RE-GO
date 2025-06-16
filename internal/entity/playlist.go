package entity

// Playlist 播放列表
type Playlist struct {
	URL               string        `json:"url"`
	IsLive            bool          `json:"isLive"`
	RefreshIntervalMs float64       `json:"refreshIntervalMs"`
	TargetDuration    *float64      `json:"targetDuration,omitempty"`
	MediaInit         *MediaSegment `json:"mediaInit,omitempty"`
	MediaParts        []*MediaPart  `json:"mediaParts"`
	TotalBytes        int64         `json:"totalBytes"`
}

// NewPlaylist 创建新的播放列表
func NewPlaylist() *Playlist {
	return &Playlist{
		IsLive:            false,
		RefreshIntervalMs: 15000, // 默认15秒
		MediaParts:        make([]*MediaPart, 0),
	}
}

// AddMediaPart 添加媒体部分
func (p *Playlist) AddMediaPart(part *MediaPart) {
	p.MediaParts = append(p.MediaParts, part)
}

// GetTotalDuration 获取总时长
func (p *Playlist) GetTotalDuration() float64 {
	var total float64
	for _, part := range p.MediaParts {
		total += part.GetDuration()
	}
	return total
}

// GetSegmentsCount 获取总段数
func (p *Playlist) GetSegmentsCount() int {
	var count int
	for _, part := range p.MediaParts {
		count += part.GetSegmentCount()
	}
	return count
}

// GetAllSegments 获取所有段
func (p *Playlist) GetAllSegments() []*MediaSegment {
	segments := make([]*MediaSegment, 0)
	for _, part := range p.MediaParts {
		segments = append(segments, part.MediaSegments...)
	}
	return segments
}

// HasEncryptedSegments 是否有加密段
func (p *Playlist) HasEncryptedSegments() bool {
	for _, part := range p.MediaParts {
		for _, segment := range part.MediaSegments {
			if segment.IsEncrypted {
				return true
			}
		}
	}
	return false
}

// GetEncryptMethods 获取加密方法列表
func (p *Playlist) GetEncryptMethods() []EncryptMethod {
	methodSet := make(map[EncryptMethod]bool)
	methods := make([]EncryptMethod, 0)

	for _, part := range p.MediaParts {
		for _, segment := range part.MediaSegments {
			if segment.IsEncrypted {
				method := segment.EncryptInfo.Method
				if !methodSet[method] {
					methodSet[method] = true
					methods = append(methods, method)
				}
			}
		}
	}

	return methods
}

// GetFirstEncryptedSegment returns the first segment that is marked as encrypted.
func (p *Playlist) GetFirstEncryptedSegment() *MediaSegment {
	for _, part := range p.MediaParts {
		for _, segment := range part.MediaSegments {
			if segment.IsEncrypted {
				return segment
			}
		}
	}
	return nil
}
