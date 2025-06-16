package parser

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"N_m3u8DL-RE-GO/internal/entity"
	"N_m3u8DL-RE-GO/internal/util"
)

// HLSTags HLS标签常量
const (
	TagEXTM3U              = "#EXTM3U"
	TagEXTINF              = "#EXTINF"
	TagEXTXVERSION         = "#EXT-X-VERSION"
	TagEXTXTARGETDUR       = "#EXT-X-TARGETDURATION"
	TagEXTXMEDIASEQ        = "#EXT-X-MEDIA-SEQUENCE"
	TagEXTXDISCONTINUITY   = "#EXT-X-DISCONTINUITY"
	TagEXTXENDLIST         = "#EXT-X-ENDLIST"
	TagEXTXPLAYLIST        = "#EXT-X-PLAYLIST-TYPE"
	TagEXTXKEY             = "#EXT-X-KEY"
	TagEXTXMAP             = "#EXT-X-MAP"
	TagEXTXSTREAM          = "#EXT-X-STREAM-INF"
	TagEXTXMEDIA           = "#EXT-X-MEDIA"
	TagEXTXBYTERANGE       = "#EXT-X-BYTERANGE"
	TagEXTXPROGRAMDATETIME = "#EXT-X-PROGRAM-DATE-TIME"
)

// HLSParser HLS解析器
type HLSParser struct {
	baseURL string
	headers map[string]string
}

// NewHLSParser 创建HLS解析器
func NewHLSParser() *HLSParser {
	return &HLSParser{
		headers: make(map[string]string),
	}
}

// ParseM3U8 解析M3U8内容
func (p *HLSParser) ParseM3U8(content, baseURL string, headers map[string]string) ([]*entity.StreamSpec, error) {
	p.baseURL = baseURL
	p.headers = headers

	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	util.Logger.Debug(fmt.Sprintf("ParseM3U8: 总行数=%d", len(lines)))

	// 显示前10行和后10行用于调试
	util.Logger.Debug("M3U8内容前10行:")
	for i := 0; i < 10 && i < len(lines); i++ {
		util.Logger.Debug(fmt.Sprintf("  %d: %s", i+1, lines[i]))
	}
	if len(lines) > 20 {
		util.Logger.Debug("M3U8内容后10行:")
		for i := len(lines) - 10; i < len(lines); i++ {
			util.Logger.Debug(fmt.Sprintf("  %d: %s", i+1, lines[i]))
		}
	}

	// 检查是否是有效的M3U8文件
	if len(lines) == 0 || !strings.HasPrefix(lines[0], TagEXTM3U) {
		util.Logger.Debug(fmt.Sprintf("无效的M3U8文件: 行数=%d, 第一行=%s", len(lines), lines[0]))
		return nil, fmt.Errorf("不是有效的M3U8文件")
	}

	// 判断是主播放列表还是媒体播放列表
	isMaster := p.isMasterPlaylist(lines)
	util.Logger.Debug(fmt.Sprintf("是否为主播放列表: %t", isMaster))

	if isMaster {
		util.Logger.Debug("调用parseMasterPlaylist")
		return p.parseMasterPlaylist(lines)
	} else {
		util.Logger.Debug("调用parseMediaPlaylist")
		return p.parseMediaPlaylist(lines)
	}
}

// isMasterPlaylist 判断是否是主播放列表
func (p *HLSParser) isMasterPlaylist(lines []string) bool {
	for i, line := range lines {
		if strings.HasPrefix(line, TagEXTXSTREAM) {
			util.Logger.Debug(fmt.Sprintf("发现EXT-X-STREAM-INF标签在第%d行: %s", i+1, line))
			return true
		}
		// 注意：必须精确匹配 "#EXT-X-MEDIA:"，而不是 "#EXT-X-MEDIA"
		// 因为 "#EXT-X-MEDIA-SEQUENCE" 也包含 "#EXT-X-MEDIA"
		if strings.HasPrefix(line, TagEXTXMEDIA+":") {
			util.Logger.Debug(fmt.Sprintf("发现EXT-X-MEDIA标签在第%d行: %s", i+1, line))
			return true
		}
	}
	util.Logger.Debug("未发现主播放列表标签，判断为媒体播放列表")
	return false
}

// parseMasterPlaylist 解析主播放列表
func (p *HLSParser) parseMasterPlaylist(lines []string) ([]*entity.StreamSpec, error) {
	var streams []*entity.StreamSpec
	var currentStream *entity.StreamSpec

	for i, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, TagEXTXSTREAM) {
			// 解析流信息
			currentStream = entity.NewStreamSpec()
			mediaType := entity.MediaTypeVideo
			currentStream.MediaType = &mediaType

			p.parseStreamAttributes(line, currentStream)

			// 下一行应该是URL
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				if !strings.HasPrefix(nextLine, "#") {
					currentStream.URL = p.resolveURL(nextLine)
					currentStream.OriginalURL = nextLine
					streams = append(streams, currentStream)
				}
			}
		} else if strings.HasPrefix(line, TagEXTXMEDIA) {
			// 解析媒体信息（音频、字幕等）
			mediaStream := entity.NewStreamSpec()
			p.parseMediaAttributes(line, mediaStream)

			if mediaStream.URL != "" {
				mediaStream.URL = p.resolveURL(mediaStream.URL)
				streams = append(streams, mediaStream)
			}
		}
	}

	// 为主播放列表中的流设置扩展名
	for _, stream := range streams {
		if stream.MediaType != nil && *stream.MediaType == entity.MediaTypeSubtitles {
			// 字幕流默认扩展名，稍后会在FetchPlayList中更新
			stream.Extension = "vtt"
		} else {
			// 视频和音频流需要解析播放列表后才能确定
			stream.Extension = "ts"
		}
	}

	return streams, nil
}

// parseMediaPlaylist 解析媒体播放列表
func (p *HLSParser) parseMediaPlaylist(lines []string) ([]*entity.StreamSpec, error) {
	util.Logger.Debug(fmt.Sprintf("开始解析媒体播放列表，行数: %d", len(lines)))

	stream := entity.NewStreamSpec()

	// 根据URL推断媒体类型
	mediaType := entity.MediaTypeVideo // 默认为视频
	util.Logger.Debug("=== 开始媒体类型推断 ===")
	util.Logger.Debug("分析URL: %s", p.baseURL)

	lowerURL := strings.ToLower(p.baseURL)
	util.Logger.Debug("转换为小写URL: %s", lowerURL)

	// 检查各种音频关键词
	audioKeywords := []string{"audio", "aac", "mp3", "audiohls", "bps_96k", "sound"}
	foundKeywords := []string{}

	for _, keyword := range audioKeywords {
		if strings.Contains(lowerURL, keyword) {
			foundKeywords = append(foundKeywords, keyword)
		}
	}

	if len(foundKeywords) > 0 {
		mediaType = entity.MediaTypeAudio
		util.Logger.Debug("✓ 检测到音频流，找到关键词: %v", foundKeywords)
		util.Logger.Debug("✓ 设置MediaType为Audio (%d)", int(mediaType))
	} else {
		util.Logger.Debug("✗ 未检测到音频关键词，默认为视频流")
		util.Logger.Debug("✗ 设置MediaType为Video (%d)", int(mediaType))
	}

	stream.MediaType = &mediaType
	util.Logger.Debug("=== 媒体类型推断完成: %s ===", mediaType.String())
	stream.URL = p.baseURL
	stream.OriginalURL = p.baseURL

	playlist := entity.NewPlaylist()
	playlist.URL = p.baseURL
	playlist.IsLive = true // 默认为直播，遇到ENDLIST再设置为false

	mediaPart := entity.NewMediaPart()
	var mediaParts []*entity.MediaPart

	var currentSegment *entity.MediaSegment
	var currentEncryptInfo *entity.EncryptInfo
	var segIndex int64 = 0
	var isEndlist bool = false
	var hasAd bool = false
	var totalBytes int64 = 0

	// 扫描广告相关标记
	var isAd bool = false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, TagEXTXTARGETDUR) {
			// 目标时长
			if duration, err := p.parseTargetDuration(line); err == nil {
				playlist.TargetDuration = &duration
			}
		} else if strings.HasPrefix(line, TagEXTXMEDIASEQ) {
			// 媒体序列
			if seq, err := p.parseMediaSequence(line); err == nil {
				segIndex = seq
			}
		} else if strings.HasPrefix(line, TagEXTXKEY) {
			// 解析加密信息
			currentEncryptInfo = p.parseKeyInfo(line)
		} else if strings.HasPrefix(line, TagEXTXMAP) {
			// 处理初始化段
			if playlist.MediaInit == nil || hasAd {
				initSegment := p.parseMapInfo(line)
				if initSegment != nil {
					playlist.MediaInit = initSegment
					if currentEncryptInfo != nil && currentEncryptInfo.Method != entity.EncryptMethodNone {
						initSegment.EncryptInfo = currentEncryptInfo
					}
				}
			} else {
				// 遇到其他map说明前面的片段应该单独成为一部分
				if len(mediaPart.MediaSegments) > 0 {
					mediaParts = append(mediaParts, mediaPart)
					mediaPart = entity.NewMediaPart()
				}
				// 这里可以选择是否继续或结束（根据配置）
				// 简化处理，直接结束
				isEndlist = true
				break
			}
		} else if strings.HasPrefix(line, TagEXTINF) {
			// 分段信息
			currentSegment = entity.NewMediaSegment()
			currentSegment.Index = segIndex
			segIndex++

			if duration, err := p.parseExtInf(line); err == nil {
				currentSegment.Duration = duration
			}

			// 设置加密信息
			if currentEncryptInfo != nil && currentEncryptInfo.Method != entity.EncryptMethodNone {
				currentSegment.EncryptInfo = entity.NewEncryptInfo()
				currentSegment.EncryptInfo.Method = currentEncryptInfo.Method
				currentSegment.EncryptInfo.Key = currentEncryptInfo.Key
				currentSegment.EncryptInfo.URI = currentEncryptInfo.URI
				currentSegment.IsEncrypted = true // 重要：标记分段为加密

				// 如果没有IV，使用segment index生成默认IV
				if currentEncryptInfo.IV != nil && len(currentEncryptInfo.IV) > 0 {
					currentSegment.EncryptInfo.IV = currentEncryptInfo.IV
				} else {
					// 生成默认IV：按照C#版本的逻辑
					// Convert.ToString(segIndex, 16).PadLeft(32, '0')
					ivStr := fmt.Sprintf("%032x", currentSegment.Index)
					if iv, err := hex.DecodeString(ivStr); err == nil {
						currentSegment.EncryptInfo.IV = iv
						util.Logger.Debug("为分段 %d 生成默认IV: %s", currentSegment.Index, ivStr)
					}
				}

				util.Logger.Debug("分段 %d 标记为加密: 方法=%s, 密钥长度=%d, IV长度=%d",
					currentSegment.Index, currentEncryptInfo.Method.String(),
					len(currentEncryptInfo.Key), len(currentSegment.EncryptInfo.IV))
			}
		} else if strings.HasPrefix(line, TagEXTXBYTERANGE) {
			// 字节范围
			if currentSegment != nil {
				p.parseByteRange(line, currentSegment)
			}
		} else if strings.HasPrefix(line, TagEXTXPROGRAMDATETIME) {
			// 程序时间
			if currentSegment != nil && len(line) > len(TagEXTXPROGRAMDATETIME)+1 {
				timeStr := line[len(TagEXTXPROGRAMDATETIME)+1:]
				if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
					currentSegment.DateTime = &t
				}
			}
		} else if strings.HasPrefix(line, TagEXTXDISCONTINUITY) {
			// 不连续标记，需要创建新的MediaPart
			if hasAd && len(mediaParts) > 0 {
				// 修复广告后的遗留问题
				lastPart := mediaParts[len(mediaParts)-1]
				mediaPart.MediaSegments = append(mediaPart.MediaSegments, lastPart.MediaSegments...)
				mediaParts = mediaParts[:len(mediaParts)-1]
				hasAd = false
				continue
			}

			// 正常的discontinuity，创建新part
			if len(mediaPart.MediaSegments) > 0 {
				mediaParts = append(mediaParts, mediaPart)
				mediaPart = entity.NewMediaPart()
			}
		} else if strings.HasPrefix(line, "#UPLYNK-SEGMENT") {
			// 国家地理去广告处理
			if strings.Contains(line, ",ad") {
				isAd = true
			} else if strings.Contains(line, ",segment") {
				isAd = false
			}
		} else if isAd {
			// 跳过广告片段
			continue
		} else if !strings.HasPrefix(line, "#") && line != "" {
			// URL行
			if currentSegment != nil {
				segmentURL := p.resolveURL(line)
				currentSegment.URL = segmentURL

				util.Logger.Debug(fmt.Sprintf("处理分段URL: %s", segmentURL))

				// 检查是否为广告片段（优酷等平台）
				if (strings.Contains(segmentURL, "ccode=") && strings.Contains(segmentURL, "/ad/") && strings.Contains(segmentURL, "duration=")) ||
					(strings.Contains(segmentURL, "ccode=0902") && strings.Contains(segmentURL, "duration=")) {
					// 这是广告片段，不添加到列表中
					util.Logger.Debug("检测到广告片段，跳过")
					hasAd = true
					segIndex-- // 回退序号
				} else {
					util.Logger.Debug(fmt.Sprintf("添加分段到mediaPart，当前分段数: %d", len(mediaPart.MediaSegments)))
					mediaPart.AddSegment(currentSegment)
					if currentSegment.ExpectLength != nil {
						totalBytes += *currentSegment.ExpectLength
					}
				}
				currentSegment = nil
			}
		} else if strings.HasPrefix(line, TagEXTXENDLIST) {
			// 结束标签
			util.Logger.Debug(fmt.Sprintf("遇到ENDLIST标签，当前mediaPart分段数: %d", len(mediaPart.MediaSegments)))
			playlist.IsLive = false
			isEndlist = true
			// 将最后的segments添加到parts中
			if len(mediaPart.MediaSegments) > 0 {
				mediaParts = append(mediaParts, mediaPart)
				util.Logger.Debug(fmt.Sprintf("添加mediaPart到mediaParts，总parts数: %d", len(mediaParts)))
			}
		}
	}

	// 处理直播情况，没有遇到endlist标签
	if !isEndlist && len(mediaPart.MediaSegments) > 0 {
		mediaParts = append(mediaParts, mediaPart)
	}

	util.Logger.Debug(fmt.Sprintf("解析完成: isEndlist=%t, mediaParts数量=%d, 当前mediaPart分段数=%d",
		isEndlist, len(mediaParts), len(mediaPart.MediaSegments)))

	// 添加所有parts到playlist
	for _, part := range mediaParts {
		playlist.AddMediaPart(part)
	}

	// 如果没有任何parts，至少添加一个空的
	if len(playlist.MediaParts) == 0 {
		util.Logger.Debug("没有找到任何parts，添加空的mediaPart")
		playlist.AddMediaPart(mediaPart)
	}

	// 设置直播刷新间隔
	if playlist.IsLive && playlist.TargetDuration != nil {
		playlist.RefreshIntervalMs = (*playlist.TargetDuration) * 2 * 1000
	}

	// 设置扩展名 - 按照C#版本逻辑 (lines 562-564)
	if playlist.MediaInit != nil {
		stream.Extension = "m4s"
	} else {
		stream.Extension = "ts"
	}

	stream.Playlist = playlist
	playlist.TotalBytes = totalBytes

	// 添加调试信息
	totalSegments := len(playlist.GetAllSegments())
	util.Logger.Debug(fmt.Sprintf("解析媒体播放列表完成: 媒体类型=%s, 分段数=%d, MediaParts数=%d",
		stream.MediaType.String(), totalSegments, len(playlist.MediaParts)))

	// 重要：即使没有分段，也要返回流（与C#版本保持一致）
	// C#版本总是返回一个StreamSpec，即使playlist为空
	return []*entity.StreamSpec{stream}, nil
}

// parseStreamAttributes 解析流属性
func (p *HLSParser) parseStreamAttributes(line string, stream *entity.StreamSpec) {
	// 提取属性部分
	attrStr := line[len(TagEXTXSTREAM)+1:]
	attrs := p.parseAttributes(attrStr)

	if bandwidth, ok := attrs["BANDWIDTH"]; ok {
		if bw, err := strconv.Atoi(bandwidth); err == nil {
			stream.Bandwidth = &bw
		}
	}

	if codecs, ok := attrs["CODECS"]; ok {
		stream.Codecs = strings.Trim(codecs, `"`)
	}

	if resolution, ok := attrs["RESOLUTION"]; ok {
		stream.Resolution = resolution
	}

	if frameRate, ok := attrs["FRAME-RATE"]; ok {
		if fr, err := strconv.ParseFloat(frameRate, 64); err == nil {
			stream.FrameRate = &fr
		}
	}

	if groupID, ok := attrs["VIDEO"]; ok {
		stream.VideoID = strings.Trim(groupID, `"`)
	}

	if groupID, ok := attrs["AUDIO"]; ok {
		stream.AudioID = strings.Trim(groupID, `"`)
	}

	if groupID, ok := attrs["SUBTITLES"]; ok {
		stream.SubtitleID = strings.Trim(groupID, `"`)
	}
}

// parseMediaAttributes 解析媒体属性
func (p *HLSParser) parseMediaAttributes(line string, stream *entity.StreamSpec) {
	attrStr := line[len(TagEXTXMEDIA)+1:]
	attrs := p.parseAttributes(attrStr)

	if mediaType, ok := attrs["TYPE"]; ok {
		switch strings.ToUpper(mediaType) {
		case "AUDIO":
			mt := entity.MediaTypeAudio
			stream.MediaType = &mt
		case "VIDEO":
			mt := entity.MediaTypeVideo
			stream.MediaType = &mt
		case "SUBTITLES":
			mt := entity.MediaTypeSubtitles
			stream.MediaType = &mt
		}
	}

	if groupID, ok := attrs["GROUP-ID"]; ok {
		stream.GroupID = strings.Trim(groupID, `"`)
	}

	if name, ok := attrs["NAME"]; ok {
		stream.Name = strings.Trim(name, `"`)
	}

	if language, ok := attrs["LANGUAGE"]; ok {
		stream.Language = strings.Trim(language, `"`)
	}

	if uri, ok := attrs["URI"]; ok {
		stream.URL = strings.Trim(uri, `"`)
	}

	if defaultVal, ok := attrs["DEFAULT"]; ok {
		if strings.ToUpper(defaultVal) == "YES" {
			choice := entity.ChoiceYes
			stream.Default = &choice
		} else {
			choice := entity.ChoiceNo
			stream.Default = &choice
		}
	}

	if channels, ok := attrs["CHANNELS"]; ok {
		stream.Channels = strings.Trim(channels, `"`)
	}
}

// parseAttributes 解析属性字符串
func (p *HLSParser) parseAttributes(attrStr string) map[string]string {
	attrs := make(map[string]string)

	// 使用正则表达式解析属性
	re := regexp.MustCompile(`([A-Z-]+)=([^,]*(?:"[^"]*"[^,]*)?)`)
	matches := re.FindAllStringSubmatch(attrStr, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			key := match[1]
			value := strings.TrimSpace(match[2])
			attrs[key] = value
		}
	}

	return attrs
}

// parseTargetDuration 解析目标时长
func (p *HLSParser) parseTargetDuration(line string) (float64, error) {
	parts := strings.Split(line, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("无效的TARGETDURATION格式")
	}
	return strconv.ParseFloat(parts[1], 64)
}

// parseMediaSequence 解析媒体序列
func (p *HLSParser) parseMediaSequence(line string) (int64, error) {
	parts := strings.Split(line, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("无效的MEDIA-SEQUENCE格式")
	}
	return strconv.ParseInt(parts[1], 10, 64)
}

// parseExtInf 解析EXTINF标签
func (p *HLSParser) parseExtInf(line string) (float64, error) {
	// #EXTINF:10.0,
	content := line[len(TagEXTINF)+1:]
	parts := strings.Split(content, ",")
	if len(parts) == 0 {
		return 0, fmt.Errorf("无效的EXTINF格式")
	}

	return strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
}

// parseKeyInfo 解析加密密钥信息
func (p *HLSParser) parseKeyInfo(line string) *entity.EncryptInfo {
	encryptInfo := entity.NewEncryptInfo()
	attrStr := line[len(TagEXTXKEY)+1:]
	attrs := p.parseAttributes(attrStr)

	if method, ok := attrs["METHOD"]; ok {
		switch strings.ToUpper(method) {
		case "AES-128":
			encryptInfo.Method = entity.EncryptMethodAES128
		case "AES-CTR":
			encryptInfo.Method = entity.EncryptMethodAESCTR
		case "SAMPLE-AES":
			encryptInfo.Method = entity.EncryptMethodSampleAES
		case "NONE":
			encryptInfo.Method = entity.EncryptMethodNone
		}
	}

	if uri, ok := attrs["URI"]; ok {
		encryptInfo.URI = p.resolveURL(strings.Trim(uri, `"`))
		// 获取密钥
		encryptInfo.Key = p.fetchKey(encryptInfo.URI)
	}

	if iv, ok := attrs["IV"]; ok {
		// 解析IV - 移除0x前缀并转换为字节
		ivStr := strings.TrimPrefix(strings.ToLower(iv), "0x")
		if ivBytes, err := hex.DecodeString(ivStr); err == nil {
			encryptInfo.IV = ivBytes
		}
	}

	return encryptInfo
}

// parseMapInfo 解析MAP信息
func (p *HLSParser) parseMapInfo(line string) *entity.MediaSegment {
	attrStr := line[len(TagEXTXMAP)+1:]
	attrs := p.parseAttributes(attrStr)

	if uri, ok := attrs["URI"]; ok {
		segment := entity.NewMediaSegment()
		segment.URL = p.resolveURL(strings.Trim(uri, `"`))

		if byteRange, ok := attrs["BYTERANGE"]; ok {
			p.parseByteRangeFromString(byteRange, segment)
		}

		return segment
	}

	return nil
}

// parseByteRange 解析字节范围
func (p *HLSParser) parseByteRange(line string, segment *entity.MediaSegment) {
	rangeStr := line[len(TagEXTXBYTERANGE)+1:]
	p.parseByteRangeFromString(rangeStr, segment)
}

// parseByteRangeFromString 从字符串解析字节范围
func (p *HLSParser) parseByteRangeFromString(rangeStr string, segment *entity.MediaSegment) {
	// 格式: length[@offset]
	parts := strings.Split(rangeStr, "@")
	if len(parts) >= 1 {
		if length, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
			segment.ExpectLength = &length
		}
	}
	if len(parts) >= 2 {
		if offset, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
			segment.StartRange = &offset
		}
	}
}

// resolveURL 解析相对URL为绝对URL
func (p *HLSParser) resolveURL(urlStr string) string {
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

// fetchKey 获取加密密钥
func (p *HLSParser) fetchKey(uri string) []byte {
	if uri == "" {
		return nil
	}

	// 处理base64编码的密钥
	if strings.HasPrefix(strings.ToLower(uri), "base64:") {
		if keyBytes, err := base64.StdEncoding.DecodeString(uri[7:]); err == nil {
			return keyBytes
		}
		return nil
	}

	if strings.HasPrefix(strings.ToLower(uri), "data:;base64,") {
		if keyBytes, err := base64.StdEncoding.DecodeString(uri[13:]); err == nil {
			return keyBytes
		}
		return nil
	}

	if strings.HasPrefix(strings.ToLower(uri), "data:text/plain;base64,") {
		if keyBytes, err := base64.StdEncoding.DecodeString(uri[23:]); err == nil {
			return keyBytes
		}
		return nil
	}

	// 处理文件路径
	if !strings.HasPrefix(uri, "http://") && !strings.HasPrefix(uri, "https://") {
		if keyBytes, err := os.ReadFile(uri); err == nil {
			return keyBytes
		} else {
			// 文件读取失败，记录警告但不返回nil，继续尝试其他方式
			fmt.Printf("警告: 无法读取密钥文件 %s: %v\n", uri, err)
		}
	}

	// 处理HTTP URL
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		if keyBytes, err := util.GetBytes(uri, p.headers); err == nil {
			return keyBytes
		} else {
			// HTTP获取失败，记录警告但不阻止流的解析
			fmt.Printf("警告: 无法获取密钥 %s: %v\n", uri, err)
		}
	}

	// 即使密钥获取失败，也返回空字节数组而不是nil，这样不会阻止流的解析
	// 在实际下载时再处理密钥问题
	return []byte{}
}
