package entity

import (
	"fmt"
	"time"
)

// MediaInfo 媒体信息
type MediaInfo struct {
	FilePath        string                `json:"file_path"`
	Format          string                `json:"format,omitempty"`
	Duration        *time.Duration        `json:"duration,omitempty"`
	FileSize        int64                 `json:"file_size"`
	Bitrate         int64                 `json:"bitrate"`
	VideoStreams    []*VideoStreamInfo    `json:"video_streams,omitempty"`
	AudioStreams    []*AudioStreamInfo    `json:"audio_streams,omitempty"`
	SubtitleStreams []*SubtitleStreamInfo `json:"subtitle_streams,omitempty"`
	Chapters        []*ChapterInfo        `json:"chapters,omitempty"`
	Metadata        map[string]string     `json:"metadata,omitempty"`
}

// VideoStreamInfo 视频流信息
type VideoStreamInfo struct {
	Index       int            `json:"index"`
	Codec       string         `json:"codec,omitempty"`
	Bitrate     int64          `json:"bitrate"`
	Width       int            `json:"width"`
	Height      int            `json:"height"`
	FrameRate   float64        `json:"frame_rate"`
	PixelFormat string         `json:"pixel_format,omitempty"`
	Duration    *time.Duration `json:"duration,omitempty"`
	Language    string         `json:"language,omitempty"`
	Title       string         `json:"title,omitempty"`
}

// AudioStreamInfo 音频流信息
type AudioStreamInfo struct {
	Index         int            `json:"index"`
	Codec         string         `json:"codec,omitempty"`
	Bitrate       int64          `json:"bitrate"`
	SampleRate    int            `json:"sample_rate"`
	Channels      int            `json:"channels"`
	ChannelLayout string         `json:"channel_layout,omitempty"`
	Duration      *time.Duration `json:"duration,omitempty"`
	Language      string         `json:"language,omitempty"`
	Title         string         `json:"title,omitempty"`
}

// SubtitleStreamInfo 字幕流信息
type SubtitleStreamInfo struct {
	Index     int    `json:"index"`
	Codec     string `json:"codec,omitempty"`
	Language  string `json:"language,omitempty"`
	Title     string `json:"title,omitempty"`
	IsDefault bool   `json:"is_default"`
	IsForced  bool   `json:"is_forced"`
}

// ChapterInfo 章节信息
type ChapterInfo struct {
	Index     int           `json:"index"`
	StartTime time.Duration `json:"start_time"`
	EndTime   time.Duration `json:"end_time"`
	Title     string        `json:"title,omitempty"`
}

// NewMediaInfo 创建新的媒体信息
func NewMediaInfo(filePath string) *MediaInfo {
	return &MediaInfo{
		FilePath:        filePath,
		VideoStreams:    make([]*VideoStreamInfo, 0),
		AudioStreams:    make([]*AudioStreamInfo, 0),
		SubtitleStreams: make([]*SubtitleStreamInfo, 0),
		Chapters:        make([]*ChapterInfo, 0),
		Metadata:        make(map[string]string),
	}
}

// GetDurationString 获取时长字符串
func (m *MediaInfo) GetDurationString() string {
	if m.Duration == nil {
		return "00:00:00"
	}

	hours := int(m.Duration.Hours())
	minutes := int(m.Duration.Minutes()) % 60
	seconds := int(m.Duration.Seconds()) % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// GetVideoResolution 获取视频分辨率
func (m *MediaInfo) GetVideoResolution() string {
	if len(m.VideoStreams) == 0 {
		return ""
	}

	video := m.VideoStreams[0]
	return fmt.Sprintf("%dx%d", video.Width, video.Height)
}

// ToString 转换为字符串表示
func (m *MediaInfo) ToString() string {
	result := fmt.Sprintf("File: %s", m.FilePath)
	if m.Format != "" {
		result += fmt.Sprintf(" [%s]", m.Format)
	}
	if m.Duration != nil {
		result += fmt.Sprintf(" Duration: %s", m.GetDurationString())
	}
	return result
}
