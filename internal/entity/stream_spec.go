package entity

import (
	"fmt"
	"strings"
	"time"
)

// MSSData MSS特定数据
type MSSData struct {
	FourCC             string `json:"fourCC,omitempty"`
	CodecPrivateData   string `json:"codecPrivateData,omitempty"`
	Type               string `json:"type,omitempty"`
	Timescale          int    `json:"timescale,omitempty"`
	SamplingRate       int    `json:"samplingRate,omitempty"`
	Channels           int    `json:"channels,omitempty"`
	BitsPerSample      int    `json:"bitsPerSample,omitempty"`
	NalUnitLengthField int    `json:"nalUnitLengthField,omitempty"`
	Duration           int64  `json:"duration,omitempty"`

	IsProtection       bool   `json:"isProtection,omitempty"`
	ProtectionSystemID string `json:"protectionSystemID,omitempty"`
	ProtectionData     string `json:"protectionData,omitempty"`

	// 兼容原有字段
	StartNumber       int64  `json:"startNumber,omitempty"`
	ChunkTemplate     string `json:"chunkTemplate,omitempty"`
	QualityLevels     int    `json:"qualityLevels,omitempty"`
	StreamIndex       int    `json:"streamIndex,omitempty"`
	ParentStreamIndex int    `json:"parentStreamIndex,omitempty"`
	Subtype           string `json:"subtype,omitempty"`
	Name              string `json:"name,omitempty"`
	Language          string `json:"language,omitempty"`
}

// StreamSpec 流规格
type StreamSpec struct {
	MediaType       *MediaType `json:"mediaType,omitempty"`
	GroupID         string     `json:"groupId,omitempty"`
	Language        string     `json:"language,omitempty"`
	Name            string     `json:"name,omitempty"`
	Default         *Choice    `json:"default,omitempty"`
	SkippedDuration *float64   `json:"skippedDuration,omitempty"`

	// MSS信息
	MSSData *MSSData `json:"mssData,omitempty"`

	// 基本信息
	Bandwidth  *int     `json:"bandwidth,omitempty"`
	Codecs     string   `json:"codecs,omitempty"`
	Resolution string   `json:"resolution,omitempty"`
	FrameRate  *float64 `json:"frameRate,omitempty"`
	Channels   string   `json:"channels,omitempty"`
	Extension  string   `json:"extension,omitempty"`

	// DASH
	Role *RoleType `json:"role,omitempty"`

	// 补充信息-色域
	VideoRange string `json:"videoRange,omitempty"`
	// 补充信息-特征
	Characteristics string `json:"characteristics,omitempty"`
	// 发布时间（仅MPD需要）
	PublishTime *time.Time `json:"publishTime,omitempty"`

	// 外部轨道GroupID (后续寻找对应轨道信息)
	AudioID    string `json:"audioId,omitempty"`
	VideoID    string `json:"videoId,omitempty"`
	SubtitleID string `json:"subtitleId,omitempty"`

	PeriodID string `json:"periodId,omitempty"`

	// URL
	URL         string `json:"url"`
	OriginalURL string `json:"originalUrl"`

	// TTML转换标记 - 用于标识需要从TTML转换为SRT的字幕
	NeedTTMLConversion bool `json:"needTtmlConversion,omitempty"`

	Playlist      *Playlist     `json:"playlist,omitempty"`
	ExtractorType ExtractorType `json:"-"` // Don't serialize to JSON
}

// NewStreamSpec 创建新的流规格
func NewStreamSpec() *StreamSpec {
	return &StreamSpec{
		URL:         "",
		OriginalURL: "",
	}
}

// GetSegmentsCount 获取段数量
func (s *StreamSpec) GetSegmentsCount() int {
	if s.Playlist != nil {
		return s.Playlist.GetSegmentsCount()
	}
	return 0
}

// ToShortString 转换为短字符串表示
func (s *StreamSpec) ToShortString() string {
	var prefixStr string
	var returnStr string

	if s.MediaType != nil {
		switch *s.MediaType {
		case MediaTypeAudio:
			prefixStr = "[Aud]"
			parts := []string{}
			if s.GroupID != "" {
				parts = append(parts, s.GroupID)
			}
			if s.Bandwidth != nil {
				parts = append(parts, fmt.Sprintf("%d Kbps", *s.Bandwidth/1000))
			}
			if s.Name != "" {
				parts = append(parts, s.Name)
			}
			if s.Codecs != "" {
				parts = append(parts, s.Codecs)
			}
			if s.Language != "" {
				parts = append(parts, s.Language)
			}
			if s.Channels != "" {
				parts = append(parts, s.Channels+"CH")
			}
			if s.Role != nil {
				parts = append(parts, s.Role.String())
			}
			returnStr = strings.Join(parts, " | ")

		case MediaTypeSubtitles:
			prefixStr = "[Sub]"
			parts := []string{}
			if s.GroupID != "" {
				parts = append(parts, s.GroupID)
			}
			if s.Language != "" {
				parts = append(parts, s.Language)
			}
			if s.Name != "" {
				parts = append(parts, s.Name)
			}
			if s.Codecs != "" {
				parts = append(parts, s.Codecs)
			}
			if s.Role != nil {
				parts = append(parts, s.Role.String())
			}
			returnStr = strings.Join(parts, " | ")

		default:
			prefixStr = "[Vid]"
			parts := []string{}
			if s.Resolution != "" {
				parts = append(parts, s.Resolution)
			}
			if s.Bandwidth != nil {
				parts = append(parts, fmt.Sprintf("%d Kbps", *s.Bandwidth/1000))
			}
			if s.GroupID != "" {
				parts = append(parts, s.GroupID)
			}
			if s.FrameRate != nil {
				parts = append(parts, fmt.Sprintf("%.2f", *s.FrameRate))
			}
			if s.Codecs != "" {
				parts = append(parts, s.Codecs)
			}
			if s.VideoRange != "" {
				parts = append(parts, s.VideoRange)
			}
			if s.Role != nil {
				parts = append(parts, s.Role.String())
			}
			returnStr = strings.Join(parts, " | ")
		}
	}

	returnStr = prefixStr + " " + returnStr
	returnStr = strings.TrimSpace(returnStr)
	returnStr = strings.ReplaceAll(returnStr, "|  |", "|")
	returnStr = strings.TrimRight(returnStr, " |")

	return returnStr
}

// ToString 转换为完整字符串表示
func (s *StreamSpec) ToString() string {
	baseStr := s.ToShortString()

	// 添加段数信息
	segmentsCount := s.GetSegmentsCount()
	if segmentsCount > 0 {
		if segmentsCount > 1 {
			baseStr += fmt.Sprintf(" | %d Segments", segmentsCount)
		} else {
			baseStr += fmt.Sprintf(" | %d Segment", segmentsCount)
		}
	}

	// 添加加密信息
	if s.Playlist != nil && s.Playlist.HasEncryptedSegments() {
		methods := s.Playlist.GetEncryptMethods()
		methodStrs := make([]string, len(methods))
		for i, method := range methods {
			methodStrs[i] = method.String()
		}
		baseStr = "[*" + strings.Join(methodStrs, ",") + "] " + baseStr
	}

	// 计算时长
	if s.Playlist != nil {
		total := s.Playlist.GetTotalDuration()
		baseStr += " | ~" + formatDuration(int(total))
	}

	// 调试信息已移除，避免循环导入问题

	return baseStr
}

// formatDuration 格式化时长
func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	} else if seconds < 3600 {
		return fmt.Sprintf("%dm%ds", seconds/60, seconds%60)
	} else {
		return fmt.Sprintf("%dh%dm%ds", seconds/3600, (seconds%3600)/60, seconds%60)
	}
}
