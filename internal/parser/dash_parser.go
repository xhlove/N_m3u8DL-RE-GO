package parser

import (
	"encoding/xml"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"N_m3u8DL-RE-GO/internal/entity"
	"N_m3u8DL-RE-GO/internal/util"
)

// DASHParser DASH解析器
type DASHParser struct {
	mpdURL     string
	baseURL    string
	mpdContent string
}

// MPD XML结构定义
type MPD struct {
	XMLName                   xml.Name `xml:"MPD"`
	Type                      string   `xml:"type,attr"`
	MaxSegmentDuration        string   `xml:"maxSegmentDuration,attr"`
	AvailabilityStartTime     string   `xml:"availabilityStartTime,attr"`
	TimeShiftBufferDepth      string   `xml:"timeShiftBufferDepth,attr"`
	PublishTime               string   `xml:"publishTime,attr"`
	MediaPresentationDuration string   `xml:"mediaPresentationDuration,attr"`
	BaseURL                   string   `xml:"BaseURL"`
	Periods                   []Period `xml:"Period"`
}

type Period struct {
	ID             string          `xml:"id,attr"`
	Duration       string          `xml:"duration,attr"`
	BaseURL        string          `xml:"BaseURL"`
	AdaptationSets []AdaptationSet `xml:"AdaptationSet"`
}

type AdaptationSet struct {
	ContentType               string                    `xml:"contentType,attr"`
	MimeType                  string                    `xml:"mimeType,attr"`
	FrameRate                 string                    `xml:"frameRate,attr"`
	Lang                      string                    `xml:"lang,attr"`
	Codecs                    string                    `xml:"codecs,attr"`
	BaseURL                   string                    `xml:"BaseURL"`
	Role                      Role                      `xml:"Role"`
	Representations           []Representation          `xml:"Representation"`
	SegmentTemplate           SegmentTemplate           `xml:"SegmentTemplate"`
	AudioChannelConfiguration AudioChannelConfiguration `xml:"AudioChannelConfiguration"`
	ContentProtection         []ContentProtection       `xml:"ContentProtection"`
}

type Representation struct {
	ID                        string                    `xml:"id,attr"`
	Bandwidth                 int                       `xml:"bandwidth,attr"`
	Width                     int                       `xml:"width,attr"`
	Height                    int                       `xml:"height,attr"`
	FrameRate                 string                    `xml:"frameRate,attr"`
	Codecs                    string                    `xml:"codecs,attr"`
	MimeType                  string                    `xml:"mimeType,attr"`
	Lang                      string                    `xml:"lang,attr"`
	VolumeAdjust              string                    `xml:"volumeAdjust,attr"`
	BaseURL                   string                    `xml:"BaseURL"`
	Role                      Role                      `xml:"Role"`
	SegmentBase               SegmentBase               `xml:"SegmentBase"`
	SegmentList               SegmentList               `xml:"SegmentList"`
	SegmentTemplate           SegmentTemplate           `xml:"SegmentTemplate"`
	AudioChannelConfiguration AudioChannelConfiguration `xml:"AudioChannelConfiguration"`
	ContentProtection         []ContentProtection       `xml:"ContentProtection"`
}

type Role struct {
	Value string `xml:"value,attr"`
}

type SegmentBase struct {
	Initialization Initialization `xml:"Initialization"`
}

type SegmentList struct {
	Duration       string         `xml:"duration,attr"`
	Timescale      string         `xml:"timescale,attr"`
	Initialization Initialization `xml:"Initialization"`
	SegmentURLs    []SegmentURL   `xml:"SegmentURL"`
}

type SegmentTemplate struct {
	Initialization         string          `xml:"initialization,attr"`
	Media                  string          `xml:"media,attr"`
	Duration               string          `xml:"duration,attr"`
	StartNumber            string          `xml:"startNumber,attr"`
	Timescale              string          `xml:"timescale,attr"`
	PresentationTimeOffset string          `xml:"presentationTimeOffset,attr"`
	SegmentTimeline        SegmentTimeline `xml:"SegmentTimeline"`
}

type SegmentTimeline struct {
	S []S `xml:"S"`
}

type S struct {
	T string `xml:"t,attr"`
	D string `xml:"d,attr"`
	R string `xml:"r,attr"`
}

type Initialization struct {
	SourceURL string `xml:"sourceURL,attr"`
	Range     string `xml:"range,attr"`
}

type SegmentURL struct {
	Media      string `xml:"media,attr"`
	MediaRange string `xml:"mediaRange,attr"`
}

type AudioChannelConfiguration struct {
	Value string `xml:"value,attr"`
}

type ContentProtection struct {
	SchemeIdUri string `xml:"schemeIdUri,attr"`
}

// NewDASHParser 创建DASH解析器
func NewDASHParser(mpdURL string) *DASHParser {
	return &DASHParser{
		mpdURL:  mpdURL,
		baseURL: mpdURL,
	}
}

// Parse 解析DASH流
func (p *DASHParser) Parse(mpdContent string) ([]*entity.StreamSpec, error) {
	p.mpdContent = mpdContent

	var mpd MPD
	err := xml.Unmarshal([]byte(mpdContent), &mpd)
	if err != nil {
		return nil, fmt.Errorf("解析MPD XML失败: %v", err)
	}

	util.Logger.Debug(fmt.Sprintf("解析MPD: type=%s, periods=%d", mpd.Type, len(mpd.Periods)))

	var streams []*entity.StreamSpec
	isLive := mpd.Type == "dynamic"

	// 处理MPD级别的BaseURL
	if mpd.BaseURL != "" {
		baseURL := mpd.BaseURL
		// 特殊处理kkbox的情况，类似C#版本
		if strings.Contains(baseURL, "kkbox.com.tw/") {
			baseURL = strings.Replace(baseURL, "//https:%2F%2F", "//", -1)
		}
		p.baseURL = p.combineURL(p.mpdURL, baseURL)
	}

	// 解析所有Period
	for _, period := range mpd.Periods {
		periodStreams, err := p.parsePeriod(period, mpd, isLive)
		if err != nil {
			util.Logger.Warn(fmt.Sprintf("解析Period失败: %v", err))
			continue
		}
		streams = append(streams, periodStreams...)
	}

	// 设置默认轨道关联
	p.setDefaultTrackAssociations(streams)

	return streams, nil
}

// parsePeriod 解析Period
func (p *DASHParser) parsePeriod(period Period, mpd MPD, isLive bool) ([]*entity.StreamSpec, error) {
	var streams []*entity.StreamSpec

	// 处理Period级别的BaseURL，使用extendBaseURL方法
	periodBaseURL := p.extendBaseURL(period, p.baseURL)

	for _, adaptationSet := range period.AdaptationSets {
		adaptationStreams, err := p.parseAdaptationSet(adaptationSet, period, mpd, periodBaseURL, isLive)
		if err != nil {
			util.Logger.Warn(fmt.Sprintf("解析AdaptationSet失败: %v", err))
			continue
		}
		streams = append(streams, adaptationStreams...)
	}

	return streams, nil
}

// parseAdaptationSet 解析AdaptationSet
func (p *DASHParser) parseAdaptationSet(adaptationSet AdaptationSet, period Period, mpd MPD, baseURL string, isLive bool) ([]*entity.StreamSpec, error) {
	var streams []*entity.StreamSpec

	// 处理AdaptationSet级别的BaseURL，使用extendBaseURL方法
	adaptationBaseURL := p.extendBaseURL(adaptationSet, baseURL)

	mimeType := adaptationSet.ContentType
	if mimeType == "" {
		mimeType = adaptationSet.MimeType
	}

	for _, representation := range adaptationSet.Representations {
		stream, err := p.parseRepresentation(representation, adaptationSet, period, mpd, adaptationBaseURL, mimeType, isLive)
		if err != nil {
			util.Logger.Warn(fmt.Sprintf("解析Representation失败: %v", err))
			continue
		}
		streams = append(streams, stream)
	}

	return streams, nil
}

// parseRepresentation 解析Representation
func (p *DASHParser) parseRepresentation(repr Representation, adaptationSet AdaptationSet, period Period, mpd MPD, baseURL string, mimeType string, isLive bool) (*entity.StreamSpec, error) {
	// 处理Representation级别的BaseURL，使用extendBaseURL方法
	reprBaseURL := p.extendBaseURL(repr, baseURL)

	// 创建StreamSpec
	stream := &entity.StreamSpec{
		OriginalURL: p.mpdURL,
		PeriodID:    period.ID,
		GroupID:     repr.ID,
		URL:         p.mpdURL,
		Playlist: &entity.Playlist{
			MediaParts: []*entity.MediaPart{{}},
			IsLive:     isLive,
		},
	}

	// 设置带宽
	if repr.Bandwidth > 0 {
		stream.Bandwidth = &repr.Bandwidth
	}

	// 设置媒体类型
	if mimeType == "" {
		mimeType = repr.MimeType
	}
	if mimeType == "" {
		mimeType = adaptationSet.MimeType
	}

	if mimeType != "" {
		parts := strings.Split(mimeType, "/")
		if len(parts) >= 2 {
			mediaType := entity.MediaTypeUnknown
			switch parts[0] {
			case "text":
				mediaType = entity.MediaTypeSubtitles
			case "audio":
				mediaType = entity.MediaTypeAudio
			case "video":
				mediaType = entity.MediaTypeVideo
			}
			stream.MediaType = &mediaType
			stream.Extension = parts[1]
		}
	}

	// 设置编码和语言
	stream.Codecs = repr.Codecs
	if stream.Codecs == "" {
		stream.Codecs = adaptationSet.Codecs
	}

	stream.Language = p.filterLanguage(repr.Lang)
	if stream.Language == "" {
		stream.Language = p.filterLanguage(adaptationSet.Lang)
	}

	// 设置分辨率
	if repr.Width > 0 && repr.Height > 0 {
		stream.Resolution = fmt.Sprintf("%dx%d", repr.Width, repr.Height)
	}

	// 设置帧率
	frameRate := repr.FrameRate
	if frameRate == "" {
		frameRate = adaptationSet.FrameRate
	}
	if frameRate != "" {
		if fr := p.parseFrameRate(frameRate); fr > 0 {
			stream.FrameRate = &fr
		}
	}

	// 设置声道
	channels := repr.AudioChannelConfiguration.Value
	if channels == "" {
		channels = adaptationSet.AudioChannelConfiguration.Value
	}
	if channels != "" {
		stream.Channels = channels
	}

	// 处理角色
	role := repr.Role.Value
	if role == "" {
		role = adaptationSet.Role.Value
	}
	if role != "" {
		roleType := p.parseRole(role)
		stream.Role = &roleType
		if *stream.Role == entity.RoleTypeSubtitle {
			mediaType := entity.MediaTypeSubtitles
			stream.MediaType = &mediaType
			if mimeType != "" && strings.Contains(mimeType, "ttml") {
				stream.Extension = "ttml"
			}
		}
	}

	// 优化字幕场景识别 - 关键TTML检测逻辑
	if stream.Codecs == "stpp" || stream.Codecs == "wvtt" {
		mediaType := entity.MediaTypeSubtitles
		stream.MediaType = &mediaType
		// 重要：检测TTML字幕并标记需要转换
		if stream.Codecs == "stpp" && stream.Extension == "m4s" {
			// 这是TTML字幕，需要转换为SRT
			stream.Extension = "ttml" // 临时标记，下载后会转换为srt
			stream.NeedTTMLConversion = true
		}
	}

	// VolumeAdjust处理
	if repr.VolumeAdjust != "" {
		stream.GroupID += "-" + repr.VolumeAdjust
	}

	// 设置刷新间隔
	if isLive && mpd.TimeShiftBufferDepth != "" {
		if duration, err := p.parseISO8601Duration(mpd.TimeShiftBufferDepth); err == nil {
			stream.Playlist.RefreshIntervalMs = float64(duration.Milliseconds()) / 2
		}
	}

	// 设置发布时间
	if mpd.PublishTime != "" {
		if publishTime, err := time.Parse(time.RFC3339, mpd.PublishTime); err == nil {
			stream.PublishTime = &publishTime
		}
	}

	// 解析分片信息
	err := p.parseSegments(stream, repr, adaptationSet, period, mpd, reprBaseURL, isLive)
	if err != nil {
		return nil, fmt.Errorf("解析分片失败: %v", err)
	}

	// 处理加密
	if p.hasContentProtection(repr.ContentProtection) || p.hasContentProtection(adaptationSet.ContentProtection) {
		if stream.Playlist.MediaInit != nil {
			stream.Playlist.MediaInit.EncryptInfo.Method = entity.EncryptMethodCENC
		}
		for _, part := range stream.Playlist.MediaParts {
			for _, seg := range part.MediaSegments {
				seg.EncryptInfo.Method = entity.EncryptMethodCENC
			}
		}
	}

	// 修复扩展名
	if stream.MediaType != nil && *stream.MediaType == entity.MediaTypeSubtitles && stream.Extension == "mp4" {
		stream.Extension = "m4s"
	}
	if stream.MediaType != nil && *stream.MediaType != entity.MediaTypeSubtitles && (stream.Extension == "" || len(stream.Playlist.MediaParts[0].MediaSegments) > 1) {
		stream.Extension = "m4s"
	}

	return stream, nil
}

// parseSegments 解析分片信息
func (p *DASHParser) parseSegments(stream *entity.StreamSpec, repr Representation, adaptationSet AdaptationSet, period Period, mpd MPD, baseURL string, isLive bool) error {
	// 1. 处理SegmentBase
	if repr.SegmentBase.Initialization.SourceURL != "" {
		return p.parseSegmentBase(stream, repr.SegmentBase, baseURL, period, mpd)
	}

	// 2. 处理SegmentList
	if len(repr.SegmentList.SegmentURLs) > 0 {
		return p.parseSegmentList(stream, repr.SegmentList, baseURL)
	}

	// 3. 处理SegmentTemplate
	segmentTemplate := repr.SegmentTemplate
	if segmentTemplate.Media == "" {
		segmentTemplate = adaptationSet.SegmentTemplate
	}
	if segmentTemplate.Media != "" {
		return p.parseSegmentTemplate(stream, segmentTemplate, adaptationSet.SegmentTemplate, repr, period, mpd, baseURL, isLive)
	}

	// 4. 如果都没有，使用BaseURL作为单个分片
	if len(stream.Playlist.MediaParts[0].MediaSegments) == 0 {
		duration := 0.0
		if period.Duration != "" {
			if d, err := p.parseISO8601Duration(period.Duration); err == nil {
				duration = d.Seconds()
			}
		} else if mpd.MediaPresentationDuration != "" {
			if d, err := p.parseISO8601Duration(mpd.MediaPresentationDuration); err == nil {
				duration = d.Seconds()
			}
		}

		segment := entity.NewMediaSegment()
		segment.Index = 0
		segment.URL = baseURL
		segment.Duration = duration
		stream.Playlist.MediaParts[0].MediaSegments = append(stream.Playlist.MediaParts[0].MediaSegments, segment)
	}

	return nil
}

// parseSegmentBase 解析SegmentBase
func (p *DASHParser) parseSegmentBase(stream *entity.StreamSpec, segmentBase SegmentBase, baseURL string, period Period, mpd MPD) error {
	init := segmentBase.Initialization
	if init.SourceURL == "" {
		// 没有init URL，直接使用baseURL
		duration := 0.0
		if period.Duration != "" {
			if d, err := p.parseISO8601Duration(period.Duration); err == nil {
				duration = d.Seconds()
			}
		} else if mpd.MediaPresentationDuration != "" {
			if d, err := p.parseISO8601Duration(mpd.MediaPresentationDuration); err == nil {
				duration = d.Seconds()
			}
		}

		segment := entity.NewMediaSegment()
		segment.Index = 0
		segment.URL = baseURL
		segment.Duration = duration
		stream.Playlist.MediaParts[0].MediaSegments = append(stream.Playlist.MediaParts[0].MediaSegments, segment)
	} else {
		// 有init URL
		initURL := p.combineURL(baseURL, init.SourceURL)
		stream.Playlist.MediaInit = entity.NewMediaSegment()
		stream.Playlist.MediaInit.Index = -1
		stream.Playlist.MediaInit.URL = initURL

		if init.Range != "" {
			start, expectLength := p.parseRange(init.Range)
			stream.Playlist.MediaInit.StartRange = &start
			stream.Playlist.MediaInit.ExpectLength = &expectLength
		}
	}

	return nil
}

// parseSegmentList 解析SegmentList
func (p *DASHParser) parseSegmentList(stream *entity.StreamSpec, segmentList SegmentList, baseURL string) error {
	// 处理init
	if segmentList.Initialization.SourceURL != "" {
		initURL := p.combineURL(baseURL, segmentList.Initialization.SourceURL)
		stream.Playlist.MediaInit = entity.NewMediaSegment()
		stream.Playlist.MediaInit.Index = -1
		stream.Playlist.MediaInit.URL = initURL

		if segmentList.Initialization.Range != "" {
			start, expectLength := p.parseRange(segmentList.Initialization.Range)
			stream.Playlist.MediaInit.StartRange = &start
			stream.Playlist.MediaInit.ExpectLength = &expectLength
		}
	}

	// 处理分片
	timescale := 1
	if segmentList.Timescale != "" {
		if ts, err := strconv.Atoi(segmentList.Timescale); err == nil {
			timescale = ts
		}
	}

	duration := 0
	if segmentList.Duration != "" {
		if d, err := strconv.Atoi(segmentList.Duration); err == nil {
			duration = d
		}
	}

	for i, segURL := range segmentList.SegmentURLs {
		mediaURL := p.combineURL(baseURL, segURL.Media)
		segment := entity.NewMediaSegment()
		segment.Index = int64(i)
		segment.URL = mediaURL
		segment.Duration = float64(duration) / float64(timescale)

		if segURL.MediaRange != "" {
			start, expectLength := p.parseRange(segURL.MediaRange)
			segment.StartRange = &start
			segment.ExpectLength = &expectLength
		}

		stream.Playlist.MediaParts[0].MediaSegments = append(stream.Playlist.MediaParts[0].MediaSegments, segment)
	}

	return nil
}

// parseSegmentTemplate 解析SegmentTemplate
func (p *DASHParser) parseSegmentTemplate(stream *entity.StreamSpec, segmentTemplate, outerTemplate SegmentTemplate, repr Representation, period Period, mpd MPD, baseURL string, isLive bool) error {
	// 合并模板属性
	template := p.mergeSegmentTemplates(segmentTemplate, outerTemplate)

	// 变量字典
	vars := map[string]string{
		"$RepresentationID$": repr.ID,
		"$Bandwidth$":        strconv.Itoa(repr.Bandwidth),
	}

	// 处理init
	if template.Initialization != "" {
		initURL := p.replaceVars(template.Initialization, vars)
		initURL = p.combineURL(baseURL, initURL)
		stream.Playlist.MediaInit = entity.NewMediaSegment()
		stream.Playlist.MediaInit.Index = -1
		stream.Playlist.MediaInit.URL = initURL
	}

	// 处理分片
	if template.Media != "" {
		return p.parseSegmentTemplateMedia(stream, template, vars, repr, period, mpd, baseURL, isLive)
	}

	return nil
}

// parseSegmentTemplateMedia 解析SegmentTemplate的Media
func (p *DASHParser) parseSegmentTemplateMedia(stream *entity.StreamSpec, template SegmentTemplate, vars map[string]string, repr Representation, period Period, mpd MPD, baseURL string, isLive bool) error {
	timescale := 1
	if template.Timescale != "" {
		if ts, err := strconv.Atoi(template.Timescale); err == nil {
			timescale = ts
		}
	}

	startNumber := int64(1)
	if template.StartNumber != "" {
		if sn, err := strconv.ParseInt(template.StartNumber, 10, 64); err == nil {
			startNumber = sn
		}
	}

	// 有SegmentTimeline的情况
	if len(template.SegmentTimeline.S) > 0 {
		return p.parseSegmentTimeline(stream, template, vars, baseURL, timescale, startNumber)
	}

	// 没有SegmentTimeline，需要计算
	duration := 0
	if template.Duration != "" {
		if d, err := strconv.Atoi(template.Duration); err == nil {
			duration = d
		}
	}

	if duration == 0 {
		return fmt.Errorf("duration为0，无法计算分片数量")
	}

	// 计算总分片数
	totalDuration := 0.0
	if period.Duration != "" {
		if d, err := p.parseISO8601Duration(period.Duration); err == nil {
			totalDuration = d.Seconds()
		}
	} else if mpd.MediaPresentationDuration != "" {
		if d, err := p.parseISO8601Duration(mpd.MediaPresentationDuration); err == nil {
			totalDuration = d.Seconds()
		}
	}

	totalNumber := int64(0)
	if totalDuration > 0 {
		totalNumber = int64(math.Ceil(totalDuration * float64(timescale) / float64(duration)))
	}

	// 直播情况下计算
	if totalNumber == 0 && isLive {
		if mpd.AvailabilityStartTime != "" && mpd.TimeShiftBufferDepth != "" {
			// 计算直播分片
			now := time.Now()
			if availableTime, err := time.Parse(time.RFC3339, mpd.AvailabilityStartTime); err == nil {
				if bufferDepth, err := p.parseISO8601Duration(mpd.TimeShiftBufferDepth); err == nil {
					ts := now.Sub(availableTime)
					startNumber += int64((ts.Seconds() - bufferDepth.Seconds()) * float64(timescale) / float64(duration))
					totalNumber = int64(bufferDepth.Seconds() * float64(timescale) / float64(duration))
				}
			}
		}
	}

	// 生成分片
	for i := int64(0); i < totalNumber; i++ {
		index := startNumber + i
		segVars := make(map[string]string)
		for k, v := range vars {
			segVars[k] = v
		}
		segVars["$Number$"] = strconv.FormatInt(index, 10)

		mediaURL := p.replaceVars(template.Media, segVars)
		mediaURL = p.combineURL(baseURL, mediaURL)

		segment := entity.NewMediaSegment()
		segment.URL = mediaURL
		segment.Duration = float64(duration) / float64(timescale)
		segment.NameFromVar = strconv.FormatInt(index, 10)

		if isLive {
			segment.Index = index
		} else {
			segment.Index = i
		}

		stream.Playlist.MediaParts[0].MediaSegments = append(stream.Playlist.MediaParts[0].MediaSegments, segment)
	}

	return nil
}

// parseSegmentTimeline 解析SegmentTimeline
func (p *DASHParser) parseSegmentTimeline(stream *entity.StreamSpec, template SegmentTemplate, vars map[string]string, baseURL string, timescale int, startNumber int64) error {
	currentTime := int64(0)
	segIndex := int64(0)
	segNumber := startNumber

	for _, s := range template.SegmentTimeline.S {
		// 解析S元素属性
		if s.T != "" {
			if t, err := strconv.ParseInt(s.T, 10, 64); err == nil {
				currentTime = t
			}
		}

		duration := int64(0)
		if s.D != "" {
			if d, err := strconv.ParseInt(s.D, 10, 64); err == nil {
				duration = d
			}
		}

		repeatCount := int64(0)
		if s.R != "" {
			if r, err := strconv.ParseInt(s.R, 10, 64); err == nil {
				repeatCount = r
			}
		}

		// 创建第一个分片
		segVars := make(map[string]string)
		for k, v := range vars {
			segVars[k] = v
		}
		segVars["$Time$"] = strconv.FormatInt(currentTime, 10)
		segVars["$Number$"] = strconv.FormatInt(segNumber, 10)

		hasTime := strings.Contains(template.Media, "$Time$")
		mediaURL := p.replaceVars(template.Media, segVars)
		mediaURL = p.combineURL(baseURL, mediaURL)

		segment := entity.NewMediaSegment()
		segment.Index = segIndex
		segment.URL = mediaURL
		segment.Duration = float64(duration) / float64(timescale)

		if hasTime {
			segment.NameFromVar = strconv.FormatInt(currentTime, 10)
		}

		stream.Playlist.MediaParts[0].MediaSegments = append(stream.Playlist.MediaParts[0].MediaSegments, segment)
		segIndex++
		segNumber++

		// 处理重复
		for i := int64(0); i < repeatCount; i++ {
			currentTime += duration
			segVars["$Time$"] = strconv.FormatInt(currentTime, 10)
			segVars["$Number$"] = strconv.FormatInt(segNumber, 10)

			mediaURL := p.replaceVars(template.Media, segVars)
			mediaURL = p.combineURL(baseURL, mediaURL)

			segment := entity.NewMediaSegment()
			segment.Index = segIndex
			segment.URL = mediaURL
			segment.Duration = float64(duration) / float64(timescale)

			if hasTime {
				segment.NameFromVar = strconv.FormatInt(currentTime, 10)
			}

			stream.Playlist.MediaParts[0].MediaSegments = append(stream.Playlist.MediaParts[0].MediaSegments, segment)
			segIndex++
			segNumber++
		}

		currentTime += duration
	}

	return nil
}

// 辅助函数们

func (p *DASHParser) combineURL(baseURL, relativeURL string) string {
	util.Logger.Debug("CombineURL: base='%s', relative='%s'", baseURL, relativeURL)

	if baseURL == "" {
		util.Logger.Debug("CombineURL result: '%s' (empty base)", relativeURL)
		return relativeURL
	}

	if relativeURL == "" {
		util.Logger.Debug("CombineURL result: '%s' (empty relative)", baseURL)
		return baseURL
	}

	// 如果relativeURL是绝对URL，直接返回
	if strings.HasPrefix(relativeURL, "http://") || strings.HasPrefix(relativeURL, "https://") {
		util.Logger.Debug("CombineURL result: '%s' (absolute URL)", relativeURL)
		return relativeURL
	}

	// 特殊处理：如果相对路径以 "/" 开头，它是相对于域名根目录的
	if strings.HasPrefix(relativeURL, "/") {
		base, err := url.Parse(baseURL)
		if err == nil {
			// 构造新的URL，只保留scheme和host部分
			result := fmt.Sprintf("%s://%s%s", base.Scheme, base.Host, relativeURL)
			util.Logger.Debug("CombineURL result: '%s' (root relative)", result)
			return result
		}
	}

	// 使用Go的url包正确处理URL拼接，类似C#的Uri类
	base, err := url.Parse(baseURL)
	if err != nil {
		// 如果baseURL解析失败，回退到简单拼接
		result := p.simpleCombineURL(baseURL, relativeURL)
		util.Logger.Debug("CombineURL result: '%s' (fallback simple)", result)
		return result
	}

	relative, err := url.Parse(relativeURL)
	if err != nil {
		// 如果relativeURL解析失败，回退到简单拼接
		result := p.simpleCombineURL(baseURL, relativeURL)
		util.Logger.Debug("CombineURL result: '%s' (fallback simple)", result)
		return result
	}

	result := base.ResolveReference(relative)
	resultStr := result.String()
	util.Logger.Debug("CombineURL result: '%s' (standard)", resultStr)
	return resultStr
}

// 简单的URL拼接，作为fallback，参考C#版本的逻辑
func (p *DASHParser) simpleCombineURL(base, relative string) string {
	// 移除base末尾的文件名（如果存在的话）
	lastSlash := strings.LastIndex(base, "/")
	if lastSlash > 7 { // 确保不是协议部分的斜杠
		// 检查最后一个斜杠后是否有文件扩展名
		afterSlash := base[lastSlash+1:]
		if strings.Contains(afterSlash, ".") && !strings.HasSuffix(base, "/") {
			// 看起来像文件名，移除它
			base = base[:lastSlash+1]
		}
	}

	// 确保base以/结尾
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}

	// 移除relative开头的./
	if strings.HasPrefix(relative, "./") {
		relative = relative[2:]
	}

	// 移除relative开头的/
	if strings.HasPrefix(relative, "/") {
		relative = relative[1:]
	}

	return base + relative
}

// extendBaseURL 处理BaseURL嵌套，类似C#版本的ExtendBaseUrl
func (p *DASHParser) extendBaseURL(element interface{}, oriBaseURL string) string {
	var baseURL string

	switch elem := element.(type) {
	case Period:
		baseURL = elem.BaseURL
	case AdaptationSet:
		baseURL = elem.BaseURL
	case Representation:
		baseURL = elem.BaseURL
	default:
		return oriBaseURL
	}

	if baseURL != "" {
		// 特殊处理kkbox的情况，类似C#版本
		if strings.Contains(baseURL, "kkbox.com.tw/") {
			baseURL = strings.Replace(baseURL, "//https:%2F%2F", "//", -1)
		}
		return p.combineURL(oriBaseURL, baseURL)
	}

	return oriBaseURL
}

func (p *DASHParser) filterLanguage(lang string) string {
	if lang == "" {
		return ""
	}

	// 简单的语言代码验证
	matched, _ := regexp.MatchString(`^[\w_\-\d]+$`, lang)
	if matched {
		return lang
	}
	return "und"
}

func (p *DASHParser) parseFrameRate(frameRate string) float64 {
	if !strings.Contains(frameRate, "/") {
		if fr, err := strconv.ParseFloat(frameRate, 64); err == nil {
			return fr
		}
		return 0
	}

	parts := strings.Split(frameRate, "/")
	if len(parts) != 2 {
		return 0
	}

	numerator, err1 := strconv.ParseFloat(parts[0], 64)
	denominator, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || denominator == 0 {
		return 0
	}

	result := numerator / denominator
	return math.Round(result*1000) / 1000
}

func (p *DASHParser) parseRole(role string) entity.RoleType {
	// 处理带短横线的情况
	if strings.Contains(role, "-") {
		role = strings.ReplaceAll(role, "-", "")
	}

	switch strings.ToLower(role) {
	case "subtitle":
		return entity.RoleTypeSubtitle
	case "main":
		return entity.RoleTypeMain
	case "alternate":
		return entity.RoleTypeAlternate
	case "supplementary":
		return entity.RoleTypeSupplementary
	case "commentary":
		return entity.RoleTypeCommentary
	case "dub":
		return entity.RoleTypeDub
	default:
		return entity.RoleTypeMain
	}
}

func (p *DASHParser) parseRange(rangeStr string) (int64, int64) {
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return 0, 0
	}

	start, err1 := strconv.ParseInt(parts[0], 10, 64)
	end, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0
	}

	return start, end - start + 1
}

func (p *DASHParser) parseISO8601Duration(duration string) (time.Duration, error) {
	// 简单的ISO8601 duration解析
	// 支持格式如: PT1M30S, PT30S, PT1H等
	if !strings.HasPrefix(duration, "PT") {
		return 0, fmt.Errorf("无效的duration格式: %s", duration)
	}

	duration = duration[2:] // 移除"PT"

	var totalSeconds float64

	// 解析小时
	if idx := strings.Index(duration, "H"); idx != -1 {
		if hours, err := strconv.ParseFloat(duration[:idx], 64); err == nil {
			totalSeconds += hours * 3600
		}
		duration = duration[idx+1:]
	}

	// 解析分钟
	if idx := strings.Index(duration, "M"); idx != -1 {
		if minutes, err := strconv.ParseFloat(duration[:idx], 64); err == nil {
			totalSeconds += minutes * 60
		}
		duration = duration[idx+1:]
	}

	// 解析秒
	if idx := strings.Index(duration, "S"); idx != -1 {
		if seconds, err := strconv.ParseFloat(duration[:idx], 64); err == nil {
			totalSeconds += seconds
		}
	}

	return time.Duration(totalSeconds * float64(time.Second)), nil
}

func (p *DASHParser) replaceVars(template string, vars map[string]string) string {
	result := template

	// 先处理普通变量替换
	for k, v := range vars {
		if strings.Contains(result, k) {
			result = strings.ReplaceAll(result, k, v)
		}
	}

	// 处理特殊形式的$Number%05d$格式，类似C#版本的处理
	numberValue, hasNumber := vars["$Number$"]
	if hasNumber {
		re := regexp.MustCompile(`\$Number%([^$]+)d\$`)
		result = re.ReplaceAllStringFunc(result, func(match string) string {
			// 提取格式化部分
			matches := re.FindStringSubmatch(match)
			if len(matches) >= 2 {
				formatStr := matches[1]
				// 解析宽度，如 "05" -> 5
				if width, err := strconv.Atoi(formatStr); err == nil && width > 0 {
					// 格式化数字，左侧补0
					if num, err := strconv.Atoi(numberValue); err == nil {
						return fmt.Sprintf("%0*d", width, num)
					}
				}
			}
			return numberValue
		})
	}

	return result
}

func (p *DASHParser) mergeSegmentTemplates(inner, outer SegmentTemplate) SegmentTemplate {
	result := inner

	if result.Initialization == "" {
		result.Initialization = outer.Initialization
	}
	if result.Media == "" {
		result.Media = outer.Media
	}
	if result.Duration == "" {
		result.Duration = outer.Duration
	}
	if result.StartNumber == "" {
		result.StartNumber = outer.StartNumber
	}
	if result.Timescale == "" {
		result.Timescale = outer.Timescale
	}
	if result.PresentationTimeOffset == "" {
		result.PresentationTimeOffset = outer.PresentationTimeOffset
	}

	return result
}

func (p *DASHParser) hasContentProtection(protections []ContentProtection) bool {
	return len(protections) > 0
}

func (p *DASHParser) setDefaultTrackAssociations(streams []*entity.StreamSpec) {
	var audioStreams []*entity.StreamSpec
	var subtitleStreams []*entity.StreamSpec

	for _, stream := range streams {
		if stream.MediaType != nil {
			switch *stream.MediaType {
			case entity.MediaTypeAudio:
				audioStreams = append(audioStreams, stream)
			case entity.MediaTypeSubtitles:
				subtitleStreams = append(subtitleStreams, stream)
			}
		}
	}

	// 为视频流设置默认音频和字幕轨道
	for _, stream := range streams {
		if stream.Resolution != "" { // 视频流
			if len(audioStreams) > 0 {
				// 选择最高码率的音频
				bestAudio := audioStreams[0]
				for _, audio := range audioStreams[1:] {
					if audio.Bandwidth != nil && bestAudio.Bandwidth != nil && *audio.Bandwidth > *bestAudio.Bandwidth {
						bestAudio = audio
					}
				}
				stream.AudioID = bestAudio.GroupID
			}

			if len(subtitleStreams) > 0 {
				// 选择最高码率的字幕
				bestSubtitle := subtitleStreams[0]
				for _, subtitle := range subtitleStreams[1:] {
					if subtitle.Bandwidth != nil && bestSubtitle.Bandwidth != nil && *subtitle.Bandwidth > *bestSubtitle.Bandwidth {
						bestSubtitle = subtitle
					}
				}
				stream.SubtitleID = bestSubtitle.GroupID
			}
		}
	}
}
