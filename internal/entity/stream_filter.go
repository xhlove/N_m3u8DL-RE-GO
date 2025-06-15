package entity

import "regexp"

// StreamFilter 流过滤器
type StreamFilter struct {
	For              string         `json:"for"`
	GroupIdReg       *regexp.Regexp `json:"groupIdReg,omitempty"`
	LanguageReg      *regexp.Regexp `json:"languageReg,omitempty"`
	NameReg          *regexp.Regexp `json:"nameReg,omitempty"`
	CodecsReg        *regexp.Regexp `json:"codecsReg,omitempty"`
	ResolutionReg    *regexp.Regexp `json:"resolutionReg,omitempty"`
	FrameRateReg     *regexp.Regexp `json:"frameRateReg,omitempty"`
	ChannelsReg      *regexp.Regexp `json:"channelsReg,omitempty"`
	VideoRangeReg    *regexp.Regexp `json:"videoRangeReg,omitempty"`
	UrlReg           *regexp.Regexp `json:"urlReg,omitempty"`
	SegmentsMinCount *int64         `json:"segmentsMinCount,omitempty"`
	SegmentsMaxCount *int64         `json:"segmentsMaxCount,omitempty"`
	PlaylistMinDur   *float64       `json:"playlistMinDur,omitempty"`
	PlaylistMaxDur   *float64       `json:"playlistMaxDur,omitempty"`
	BandwidthMin     *int64         `json:"bandwidthMin,omitempty"`
	BandwidthMax     *int64         `json:"bandwidthMax,omitempty"`
	Role             *RoleType      `json:"role,omitempty"`
}

// String 返回字符串表示
func (sf *StreamFilter) String() string {
	return sf.For
}
