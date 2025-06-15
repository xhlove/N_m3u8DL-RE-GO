package parser

import (
	"fmt"
	"strings"

	"N_m3u8DL-RE-GO/internal/entity"
	"N_m3u8DL-RE-GO/internal/util"
)

// StreamExtractor 流提取器
type StreamExtractor struct {
	hlsParser  *HLSParser
	mssParser  *MSSParser
	dashParser *DASHParser
}

// NewStreamExtractor 创建流提取器
func NewStreamExtractor() *StreamExtractor {
	return &StreamExtractor{
		hlsParser: NewHLSParser(),
		mssParser: NewMSSParser(),
	}
}

// ExtractStreams 提取流信息
func (e *StreamExtractor) ExtractStreams(url string, headers map[string]string) ([]*entity.StreamSpec, error) {
	util.Logger.Info(fmt.Sprintf("正在提取流信息: %s", url))

	// 获取内容
	content, finalURL, err := util.GetStringAndURL(url, headers)
	if err != nil {
		return nil, fmt.Errorf("获取内容失败: %w", err)
	}

	util.Logger.Debug(fmt.Sprintf("最终URL: %s", finalURL))
	util.Logger.Debug(fmt.Sprintf("内容长度: %d", len(content)))

	// 检测流类型并提取
	extractorType := e.detectExtractorType(content, finalURL)

	switch extractorType {
	case entity.ExtractorTypeHLS:
		return e.extractHLS(content, finalURL, headers)
	case entity.ExtractorTypeDASH:
		return e.extractDASH(content, finalURL, headers)
	case entity.ExtractorTypeMSS:
		return e.extractMSS(content, finalURL, headers)
	case entity.ExtractorTypeLiveTS:
		return e.extractLiveTS(finalURL, headers)
	default:
		return nil, fmt.Errorf("不支持的流类型")
	}
}

// detectExtractorType 检测提取器类型
func (e *StreamExtractor) detectExtractorType(content, url string) entity.ExtractorType {
	contentLower := strings.ToLower(content)
	urlLower := strings.ToLower(url)

	// 检查是否是直播TS流
	if content == "Live TS Stream detected" {
		return entity.ExtractorTypeLiveTS
	}

	// 检查HLS
	if strings.Contains(contentLower, "#extm3u") ||
		strings.Contains(urlLower, ".m3u8") ||
		strings.Contains(urlLower, "/m3u8/") {
		return entity.ExtractorTypeHLS
	}

	// 检查DASH
	if strings.Contains(contentLower, "<mpd") ||
		strings.Contains(urlLower, ".mpd") ||
		strings.Contains(contentLower, "urn:mpeg:dash:schema") {
		return entity.ExtractorTypeDASH
	}

	// 检查MSS (Smooth Streaming)
	if strings.Contains(contentLower, "<smoothstreamingmedia") ||
		strings.Contains(urlLower, "/manifest") ||
		strings.Contains(urlLower, ".ism/") {
		return entity.ExtractorTypeMSS
	}

	// 默认尝试HLS（很多情况下M3U8响应可能没有明确标识）
	return entity.ExtractorTypeHLS
}

// extractHLS 提取HLS流
func (e *StreamExtractor) extractHLS(content, url string, headers map[string]string) ([]*entity.StreamSpec, error) {
	util.Logger.Info("正在解析HLS流")
	streams, err := e.hlsParser.ParseM3U8(content, url, headers)
	if err != nil {
		util.Logger.Error(fmt.Sprintf("HLS解析失败: %v", err))
		return nil, err
	}

	util.Logger.Debug(fmt.Sprintf("HLS解析完成，返回 %d 个流", len(streams)))
	for i, stream := range streams {
		util.Logger.Debug(fmt.Sprintf("流 %d: %s", i, stream.ToString()))
	}

	return streams, nil
}

// extractDASH 提取DASH流
func (e *StreamExtractor) extractDASH(content, url string, headers map[string]string) ([]*entity.StreamSpec, error) {
	util.Logger.Info("正在解析DASH流")
	if e.dashParser == nil {
		e.dashParser = NewDASHParser(url)
	}
	return e.dashParser.Parse(content)
}

// extractMSS 提取MSS流
func (e *StreamExtractor) extractMSS(content, url string, headers map[string]string) ([]*entity.StreamSpec, error) {
	util.Logger.Info("正在解析MSS流")
	return e.mssParser.ParseManifest(content, url, headers)
}

// extractLiveTS 提取直播TS流
func (e *StreamExtractor) extractLiveTS(url string, headers map[string]string) ([]*entity.StreamSpec, error) {
	util.Logger.Info("正在处理直播TS流")

	stream := entity.NewStreamSpec()
	mediaType := entity.MediaTypeVideo
	stream.MediaType = &mediaType
	stream.URL = url
	stream.OriginalURL = url
	stream.Extension = "ts"

	// 创建简单的播放列表用于直播TS
	playlist := entity.NewPlaylist()
	playlist.URL = url
	playlist.IsLive = true

	// 创建单个段表示整个直播流
	mediaPart := entity.NewMediaPart()
	segment := entity.NewMediaSegment()
	segment.URL = url
	segment.Index = 0
	segment.Duration = 0 // 直播流时长未知

	mediaPart.AddSegment(segment)
	playlist.AddMediaPart(mediaPart)
	stream.Playlist = playlist

	return []*entity.StreamSpec{stream}, nil
}

// FilterStreams 过滤流
func (e *StreamExtractor) FilterStreams(streams []*entity.StreamSpec, videoSelect, audioSelect, subtitleSelect string) []*entity.StreamSpec {
	var filtered []*entity.StreamSpec

	// 分类流
	var videoStreams, audioStreams, subtitleStreams []*entity.StreamSpec

	for _, stream := range streams {
		if stream.MediaType != nil {
			switch *stream.MediaType {
			case entity.MediaTypeVideo:
				videoStreams = append(videoStreams, stream)
			case entity.MediaTypeAudio:
				audioStreams = append(audioStreams, stream)
			case entity.MediaTypeSubtitles:
				subtitleStreams = append(subtitleStreams, stream)
			}
		}
	}

	// 应用视频选择
	filtered = append(filtered, e.selectStreams(videoStreams, videoSelect)...)

	// 应用音频选择
	filtered = append(filtered, e.selectStreams(audioStreams, audioSelect)...)

	// 应用字幕选择
	filtered = append(filtered, e.selectStreams(subtitleStreams, subtitleSelect)...)

	return filtered
}

// selectStreams 选择流
func (e *StreamExtractor) selectStreams(streams []*entity.StreamSpec, selection string) []*entity.StreamSpec {
	if len(streams) == 0 {
		return streams
	}

	switch strings.ToLower(selection) {
	case "all":
		return streams
	case "best":
		return []*entity.StreamSpec{e.getBestStream(streams)}
	case "worst":
		return []*entity.StreamSpec{e.getWorstStream(streams)}
	case "none", "":
		return []*entity.StreamSpec{}
	default:
		// TODO: 支持更复杂的选择逻辑（如按语言、编解码器等）
		return []*entity.StreamSpec{e.getBestStream(streams)}
	}
}

// getBestStream 获取最佳流（按带宽）
func (e *StreamExtractor) getBestStream(streams []*entity.StreamSpec) *entity.StreamSpec {
	if len(streams) == 0 {
		return nil
	}

	best := streams[0]
	bestBandwidth := 0

	if best.Bandwidth != nil {
		bestBandwidth = *best.Bandwidth
	}

	for _, stream := range streams[1:] {
		if stream.Bandwidth != nil && *stream.Bandwidth > bestBandwidth {
			best = stream
			bestBandwidth = *stream.Bandwidth
		}
	}

	return best
}

// getWorstStream 获取最差流（按带宽）
func (e *StreamExtractor) getWorstStream(streams []*entity.StreamSpec) *entity.StreamSpec {
	if len(streams) == 0 {
		return nil
	}

	worst := streams[0]
	worstBandwidth := int(^uint(0) >> 1) // 最大int值

	if worst.Bandwidth != nil {
		worstBandwidth = *worst.Bandwidth
	}

	for _, stream := range streams[1:] {
		if stream.Bandwidth != nil && *stream.Bandwidth < worstBandwidth {
			worst = stream
			worstBandwidth = *stream.Bandwidth
		}
	}

	return worst
}

// FetchPlayList 获取播放列表并更新扩展名
func (e *StreamExtractor) FetchPlayList(streams []*entity.StreamSpec, headers map[string]string) error {
	for _, stream := range streams {
		if stream.URL == "" {
			continue
		}

		// 获取播放列表内容
		content, finalURL, err := util.GetStringAndURL(stream.URL, headers)
		if err != nil {
			util.Logger.Warn(fmt.Sprintf("无法加载播放列表 %s: %v", stream.URL, err))
			continue
		}

		// 解析播放列表
		extractorType := e.detectExtractorType(content, finalURL)

		switch extractorType {
		case entity.ExtractorTypeHLS:
			newStreams, err := e.hlsParser.ParseM3U8(content, finalURL, headers)
			if err != nil {
				util.Logger.Warn(fmt.Sprintf("解析HLS播放列表失败: %v", err))
				continue
			}

			if len(newStreams) > 0 {
				newPlaylist := newStreams[0].Playlist
				if stream.Playlist != nil && stream.Playlist.MediaInit != nil {
					// 不更新init，只更新MediaParts
					if newPlaylist != nil {
						stream.Playlist.MediaParts = newPlaylist.MediaParts
					}
				} else {
					stream.Playlist = newPlaylist
				}

				// 更新扩展名 - 参照C#版本的逻辑 (lines 554-564)
				if stream.MediaType != nil && *stream.MediaType == entity.MediaTypeSubtitles {
					// 检查字幕文件类型
					hasTtml := false
					hasVtt := false

					if stream.Playlist != nil {
						for _, part := range stream.Playlist.MediaParts {
							for _, segment := range part.MediaSegments {
								if strings.Contains(segment.URL, ".ttml") {
									hasTtml = true
								}
								if strings.Contains(segment.URL, ".vtt") || strings.Contains(segment.URL, ".webvtt") {
									hasVtt = true
								}
							}
						}
					}

					if hasTtml {
						stream.Extension = "ttml"
					} else if hasVtt {
						stream.Extension = "vtt"
					}
				} else {
					// 音频/视频流
					if stream.Playlist != nil && stream.Playlist.MediaInit != nil {
						stream.Extension = "m4s"
					} else {
						stream.Extension = "ts"
					}
				}
			}
		}
	}

	return nil
}
