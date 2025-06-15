package parser

import (
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"N_m3u8DL-RE-GO/internal/entity"
	"N_m3u8DL-RE-GO/internal/util"
)

// MSSTags MSS标签常量
const (
	MSSTagBitrate   = "{Bitrate}"
	MSSTagStartTime = "{start time}"
)

// MSSParser Microsoft Smooth Streaming 解析器
type MSSParser struct {
	baseURL string
	headers map[string]string
}

// NewMSSParser 创建MSS解析器
func NewMSSParser() *MSSParser {
	return &MSSParser{
		headers: make(map[string]string),
	}
}

// SmoothStreamingMedia MSS根元素
type SmoothStreamingMedia struct {
	XMLName     xml.Name `xml:"SmoothStreamingMedia"`
	TimeScale   string   `xml:"TimeScale,attr"`
	Duration    string   `xml:"Duration,attr"`
	IsLive      string   `xml:"IsLive,attr"`
	Protection  *Protection
	StreamIndex []StreamIndex `xml:"StreamIndex"`
}

// Protection 保护信息
type Protection struct {
	XMLName          xml.Name `xml:"Protection"`
	ProtectionHeader *ProtectionHeader
}

// ProtectionHeader 保护头
type ProtectionHeader struct {
	XMLName  xml.Name `xml:"ProtectionHeader"`
	SystemID string   `xml:"SystemID,attr"`
	Data     string   `xml:",chardata"`
}

// StreamIndex 流索引
type StreamIndex struct {
	XMLName      xml.Name       `xml:"StreamIndex"`
	Type         string         `xml:"Type,attr"`
	Name         string         `xml:"Name,attr"`
	Subtype      string         `xml:"Subtype,attr"`
	URL          string         `xml:"Url,attr"`
	Language     string         `xml:"Language,attr"`
	C            []CElement     `xml:"c"`
	QualityLevel []QualityLevel `xml:"QualityLevel"`
}

// CElement C标签元素
type CElement struct {
	XMLName xml.Name `xml:"c"`
	T       string   `xml:"t,attr"` // start time
	D       string   `xml:"d,attr"` // duration
	R       string   `xml:"r,attr"` // repeat count
}

// QualityLevel 质量等级
type QualityLevel struct {
	XMLName            xml.Name `xml:"QualityLevel"`
	Index              string   `xml:"Index,attr"`
	Bitrate            string   `xml:"Bitrate,attr"`
	FourCC             string   `xml:"FourCC,attr"`
	MaxWidth           string   `xml:"MaxWidth,attr"`
	MaxHeight          string   `xml:"MaxHeight,attr"`
	CodecPrivateData   string   `xml:"CodecPrivateData,attr"`
	SamplingRate       string   `xml:"SamplingRate,attr"`
	Channels           string   `xml:"Channels,attr"`
	BitsPerSample      string   `xml:"BitsPerSample,attr"`
	NALUnitLengthField string   `xml:"NALUnitLengthField,attr"`
	AudioTag           string   `xml:"AudioTag,attr"`
	URL                string   `xml:"Url,attr"`
}

// ParseManifest 解析MSS清单
func (p *MSSParser) ParseManifest(content, baseURL string, headers map[string]string) ([]*entity.StreamSpec, error) {
	p.baseURL = baseURL
	p.headers = headers

	// 解析XML
	var manifest SmoothStreamingMedia
	err := xml.Unmarshal([]byte(content), &manifest)
	if err != nil {
		return nil, fmt.Errorf("解析MSS清单失败: %w", err)
	}

	var streams []*entity.StreamSpec

	// 解析基本参数
	timeScale := 10000000 // 默认值
	if manifest.TimeScale != "" {
		if ts, err := strconv.Atoi(manifest.TimeScale); err == nil {
			timeScale = ts
		}
	}

	var duration int64
	if manifest.Duration != "" {
		if d, err := strconv.ParseInt(manifest.Duration, 10, 64); err == nil {
			duration = d
		}
	}

	isLive := false
	if strings.ToUpper(manifest.IsLive) == "TRUE" {
		isLive = true
	}

	// 检查加密保护
	isProtection := false
	protectionSystemID := ""

	if manifest.Protection != nil && manifest.Protection.ProtectionHeader != nil {
		isProtection = true
		protectionSystemID = manifest.Protection.ProtectionHeader.SystemID
		if protectionSystemID == "" {
			protectionSystemID = "9A04F079-9840-4286-AB92-E65BE0885F95"
		}
	}

	// 处理每个StreamIndex
	for _, streamIndex := range manifest.StreamIndex {
		urlPattern := streamIndex.URL

		// 处理每个QualityLevel
		for _, qualityLevel := range streamIndex.QualityLevel {
			// URL模式优先使用QualityLevel中的
			if qualityLevel.URL != "" {
				urlPattern = qualityLevel.URL
			}

			// 替换URL模式中的标记
			urlPattern = strings.ReplaceAll(urlPattern, "{Bitrate}", MSSTagBitrate)
			urlPattern = strings.ReplaceAll(urlPattern, "{bitrate}", MSSTagBitrate)
			urlPattern = strings.ReplaceAll(urlPattern, "{start time}", MSSTagStartTime)
			urlPattern = strings.ReplaceAll(urlPattern, "{start_time}", MSSTagStartTime)

			stream := entity.NewStreamSpec()
			stream.ExtractorType = entity.ExtractorTypeMSS
			stream.Extension = "m4s"
			stream.OriginalURL = baseURL

			// 设置媒体类型
			switch strings.ToLower(streamIndex.Type) {
			case "audio":
				mediaType := entity.MediaTypeAudio
				stream.MediaType = &mediaType
			case "text":
				mediaType := entity.MediaTypeSubtitles
				stream.MediaType = &mediaType
			default:
				// 默认为视频
				mediaType := entity.MediaTypeVideo
				stream.MediaType = &mediaType
			}

			// 设置基本属性
			if qualityLevel.Bitrate != "" {
				if bitrate, err := strconv.Atoi(qualityLevel.Bitrate); err == nil {
					stream.Bandwidth = &bitrate
				}
			}

			stream.GroupID = streamIndex.Name
			if stream.GroupID == "" {
				stream.GroupID = qualityLevel.Index
			}

			stream.Language = streamIndex.Language
			// 去除不规范的语言标签
			if len(stream.Language) != 3 {
				stream.Language = ""
			}

			// 设置分辨率
			if qualityLevel.MaxWidth != "" && qualityLevel.MaxHeight != "" {
				width, _ := strconv.Atoi(qualityLevel.MaxWidth)
				height, _ := strconv.Atoi(qualityLevel.MaxHeight)
				if width > 0 && height > 0 {
					stream.Resolution = fmt.Sprintf("%dx%d", width, height)
				}
			}

			stream.Channels = qualityLevel.Channels
			stream.Codecs = p.parseCodecs(qualityLevel.FourCC, qualityLevel.CodecPrivateData)
			stream.URL = baseURL

			// 创建播放列表
			playlist := entity.NewPlaylist()
			playlist.URL = baseURL
			playlist.IsLive = isLive

			if util.CanHandle(qualityLevel.FourCC) {
				// 设置MSS数据
				stream.MSSData = &entity.MSSData{
					FourCC:             qualityLevel.FourCC,
					CodecPrivateData:   qualityLevel.CodecPrivateData,
					Type:               streamIndex.Type,
					Timescale:          timeScale,
					Duration:           duration,
					SamplingRate:       44100, // 默认值
					Channels:           2,     // 默认值
					BitsPerSample:      16,    // 默认值
					NalUnitLengthField: 4,     // 默认值
					IsProtection:       isProtection,
					ProtectionData:     "", // TODO: 处理保护数据
					ProtectionSystemID: protectionSystemID,
				}

				// 解析采样率和声道数
				if qualityLevel.SamplingRate != "" {
					if sr, err := strconv.Atoi(qualityLevel.SamplingRate); err == nil {
						stream.MSSData.SamplingRate = sr
					}
				}
				if qualityLevel.Channels != "" {
					if ch, err := strconv.Atoi(qualityLevel.Channels); err == nil {
						stream.MSSData.Channels = ch
					}
				}
				if qualityLevel.BitsPerSample != "" {
					if bps, err := strconv.Atoi(qualityLevel.BitsPerSample); err == nil {
						stream.MSSData.BitsPerSample = bps
					}
				}
				if qualityLevel.NALUnitLengthField != "" {
					if nulf, err := strconv.Atoi(qualityLevel.NALUnitLengthField); err == nil {
						stream.MSSData.NalUnitLengthField = nulf
					}
				}

				// MOOV头部将在下载阶段根据第一个分片生成
				// 这里只创建一个空的init segment占位
				initSegment := entity.NewMediaSegment()
				initSegment.Index = -1
				playlist.MediaInit = initSegment
			} else {
				// 对于不支持的编解码器，使用原始格式
				if qualityLevel.CodecPrivateData != "" {
					initSegment := entity.NewMediaSegment()
					initSegment.Index = -1
					initSegment.URL = fmt.Sprintf("hex://%s", qualityLevel.CodecPrivateData)
					playlist.MediaInit = initSegment
				}
			}

			// 创建媒体部分
			mediaPart := entity.NewMediaPart()

			// 解析C元素生成分段
			currentTime := int64(0)
			segIndex := 0
			bitrate := 0
			if stream.Bandwidth != nil {
				bitrate = *stream.Bandwidth
			}

			for _, c := range streamIndex.C {
				// 解析时间和时长
				if c.T != "" {
					if t, err := strconv.ParseInt(c.T, 10, 64); err == nil {
						currentTime = t
					}
				}

				var duration int64
				if c.D != "" {
					if d, err := strconv.ParseInt(c.D, 10, 64); err == nil {
						duration = d
					}
				}

				var repeatCount int64 = 0
				if c.R != "" {
					if r, err := strconv.ParseInt(c.R, 10, 64); err == nil {
						repeatCount = r
						if repeatCount > 0 {
							repeatCount -= 1 // MSS格式是1-based
						}
					}
				}

				// 创建分段
				segment := p.createSegment(urlPattern, currentTime, duration, float64(timeScale), bitrate, segIndex)
				mediaPart.AddSegment(segment)
				segIndex++

				// 处理重复分段
				if repeatCount < 0 {
					// 负数表示重复到结束
					if duration > 0 {
						repeatCount = duration/duration - 1
					}
				}

				for i := int64(0); i < repeatCount; i++ {
					currentTime += duration
					segment = p.createSegment(urlPattern, currentTime, duration, float64(timeScale), bitrate, segIndex)
					mediaPart.AddSegment(segment)
					segIndex++
				}

				currentTime += duration
			}

			// 如果支持加密，设置加密信息
			if isProtection && streamIndex.Type != "text" {
				if playlist.MediaInit != nil {
					playlist.MediaInit.EncryptInfo.Method = entity.EncryptMethodCENC
				}
				for _, segment := range mediaPart.MediaSegments {
					segment.EncryptInfo.Method = entity.EncryptMethodCENC
				}
			}

			playlist.AddMediaPart(mediaPart)
			stream.Playlist = playlist

			// 只添加支持的编解码器
			if util.CanHandle(qualityLevel.FourCC) {
				streams = append(streams, stream)
			} else {
				fmt.Printf("[警告] 不支持的编解码器 %s，已跳过\n", qualityLevel.FourCC)
			}
		}
	}

	// 设置默认轨道关联
	p.setDefaultTracks(streams)

	return streams, nil
}

// createSegment 创建媒体分段
func (p *MSSParser) createSegment(urlPattern string, startTime, duration int64, timeScale float64, bitrate, index int) *entity.MediaSegment {
	segment := entity.NewMediaSegment()
	segment.Index = int64(index)
	segment.Duration = float64(duration) / timeScale

	// 替换URL中的变量
	segmentURL := urlPattern
	segmentURL = strings.ReplaceAll(segmentURL, MSSTagBitrate, strconv.Itoa(bitrate))
	segmentURL = strings.ReplaceAll(segmentURL, MSSTagStartTime, strconv.FormatInt(startTime, 10))

	// 解析为绝对URL
	segment.URL = p.resolveURL(segmentURL)

	// 设置名称变量
	if strings.Contains(urlPattern, MSSTagStartTime) {
		segment.NameFromVar = strconv.FormatInt(startTime, 10)
	}

	return segment
}

// parseCodecs 解析编解码器信息
func (p *MSSParser) parseCodecs(fourCC, privateData string) string {
	if fourCC == "TTML" {
		return "stpp"
	}

	if privateData == "" {
		return strings.ToLower(fourCC)
	}

	switch strings.ToUpper(fourCC) {
	case "H264", "X264", "DAVC", "AVC1":
		return p.parseAVCCodecs(privateData)
	case "AAC", "AACL", "AACH", "AACP":
		return p.parseAACCodecs(fourCC, privateData)
	default:
		return strings.ToLower(fourCC)
	}
}

// parseAVCCodecs 解析AVC编解码器
func (p *MSSParser) parseAVCCodecs(privateData string) string {
	// 使用正则表达式匹配AVC配置
	re := regexp.MustCompile(`00000001\d7([0-9a-fA-F]{6})`)
	matches := re.FindStringSubmatch(privateData)
	if len(matches) > 1 {
		return fmt.Sprintf("avc1.%s", matches[1])
	}
	return "avc1.4D401E" // 默认值
}

// parseAACCodecs 解析AAC编解码器
func (p *MSSParser) parseAACCodecs(fourCC, privateData string) string {
	mpProfile := 2 // 默认Profile

	if fourCC == "AACH" {
		mpProfile = 5 // High Efficiency AAC Profile
	} else if len(privateData) >= 2 {
		if b, err := hex.DecodeString(privateData[:2]); err == nil && len(b) > 0 {
			mpProfile = int((b[0] & 0xF8) >> 3)
		}
	}

	return fmt.Sprintf("mp4a.40.%d", mpProfile)
}

// setDefaultTracks 设置默认轨道关联
func (p *MSSParser) setDefaultTracks(streams []*entity.StreamSpec) {
	var audioStreams, subtitleStreams []*entity.StreamSpec

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

	// 为视频流设置关联的音频和字幕轨道
	for _, stream := range streams {
		if stream.MediaType == nil || *stream.MediaType == entity.MediaTypeVideo {
			if len(audioStreams) > 0 {
				stream.AudioID = audioStreams[0].GroupID
			}
			if len(subtitleStreams) > 0 {
				stream.SubtitleID = subtitleStreams[0].GroupID
			}
		}
	}
}

// resolveURL 解析相对URL为绝对URL
func (p *MSSParser) resolveURL(urlStr string) string {
	if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
		return urlStr
	}

	baseURL, err := url.Parse(p.baseURL)
	if err != nil {
		return urlStr
	}

	relativeURL, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}

	return baseURL.ResolveReference(relativeURL).String()
}
