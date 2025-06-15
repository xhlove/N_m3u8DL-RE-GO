package util

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"N_m3u8DL-RE-GO/internal/entity"
)

// DoFilterKeep 保留符合条件的流
func DoFilterKeep(streams []*entity.StreamSpec, filter *entity.StreamFilter) []*entity.StreamSpec {
	if filter == nil {
		return []*entity.StreamSpec{}
	}

	var result []*entity.StreamSpec

	// 应用过滤条件
	for _, stream := range streams {
		match := true

		// 检查各个过滤条件
		if filter.GroupIdReg != nil && (stream.GroupID == "" || !filter.GroupIdReg.MatchString(stream.GroupID)) {
			match = false
		}
		if filter.LanguageReg != nil && (stream.Language == "" || !filter.LanguageReg.MatchString(stream.Language)) {
			match = false
		}
		if filter.NameReg != nil && (stream.Name == "" || !filter.NameReg.MatchString(stream.Name)) {
			match = false
		}
		if filter.CodecsReg != nil && (stream.Codecs == "" || !filter.CodecsReg.MatchString(stream.Codecs)) {
			match = false
		}
		if filter.ResolutionReg != nil && (stream.Resolution == "" || !filter.ResolutionReg.MatchString(stream.Resolution)) {
			match = false
		}
		if filter.FrameRateReg != nil && (stream.FrameRate == nil || !filter.FrameRateReg.MatchString(fmt.Sprintf("%g", *stream.FrameRate))) {
			match = false
		}
		if filter.ChannelsReg != nil && (stream.Channels == "" || !filter.ChannelsReg.MatchString(stream.Channels)) {
			match = false
		}
		if filter.VideoRangeReg != nil && (stream.VideoRange == "" || !filter.VideoRangeReg.MatchString(stream.VideoRange)) {
			match = false
		}
		if filter.UrlReg != nil && (stream.URL == "" || !filter.UrlReg.MatchString(stream.URL)) {
			match = false
		}

		segmentsCount := stream.GetSegmentsCount()
		// 修正：只有当所有流的段数大于0时才应用段数过滤
		hasNonZeroSegments := false
		for _, s := range streams {
			if s.GetSegmentsCount() > 0 {
				hasNonZeroSegments = true
				break
			}
		}

		if filter.SegmentsMaxCount != nil && hasNonZeroSegments && segmentsCount > int(*filter.SegmentsMaxCount) {
			match = false
		}
		if filter.SegmentsMinCount != nil && hasNonZeroSegments && segmentsCount < int(*filter.SegmentsMinCount) {
			match = false
		}

		if filter.PlaylistMinDur != nil && stream.Playlist != nil {
			if stream.Playlist.GetTotalDuration() <= *filter.PlaylistMinDur {
				match = false
			}
		}
		if filter.PlaylistMaxDur != nil && stream.Playlist != nil {
			if stream.Playlist.GetTotalDuration() >= *filter.PlaylistMaxDur {
				match = false
			}
		}
		if filter.BandwidthMin != nil && stream.Bandwidth != nil && int64(*stream.Bandwidth) < *filter.BandwidthMin {
			match = false
		}
		if filter.BandwidthMax != nil && stream.Bandwidth != nil && int64(*stream.Bandwidth) > *filter.BandwidthMax {
			match = false
		}
		if filter.Role != nil {
			if stream.Role == nil || *stream.Role != *filter.Role {
				match = false
			}
		}

		if match {
			result = append(result, stream)
		}
	}

	// 应用 best/worst/数量 过滤
	if len(result) == 0 {
		return result
	}

	bestNumberStr := strings.Replace(filter.For, "best", "", -1)
	worstNumberStr := strings.Replace(filter.For, "worst", "", -1)

	if filter.For == "best" && len(result) > 0 {
		result = result[:1]
	} else if filter.For == "worst" && len(result) > 0 {
		result = result[len(result)-1:]
	} else if bestNumber, err := strconv.Atoi(bestNumberStr); err == nil && len(result) > 0 {
		if len(result) > bestNumber {
			result = result[:bestNumber]
		}
	} else if worstNumber, err := strconv.Atoi(worstNumberStr); err == nil && len(result) > 0 {
		if len(result) > worstNumber {
			result = result[len(result)-worstNumber:]
		}
	}

	return result
}

// DoFilterDrop 排除符合条件的流
func DoFilterDrop(streams []*entity.StreamSpec, filter *entity.StreamFilter) []*entity.StreamSpec {
	if filter == nil {
		return streams
	}

	selected := DoFilterKeep(streams, filter)

	// 创建选中流的映射以便快速查找
	selectedMap := make(map[string]bool)
	for _, s := range selected {
		selectedMap[s.ToString()] = true
	}

	var result []*entity.StreamSpec
	for _, stream := range streams {
		if !selectedMap[stream.ToString()] {
			result = append(result, stream)
		}
	}

	return result
}

// SyncStreams 同步多个流的起始时间（用于直播）
func SyncStreams(selectedStreams []*entity.StreamSpec, takeLastCount int) {
	if len(selectedStreams) == 0 {
		return
	}

	if takeLastCount == 0 {
		takeLastCount = 15 // 默认值
	}

	// 检查是否所有流都有DateTime信息
	allHaveDateTime := true
	for _, stream := range selectedStreams {
		if stream.Playlist == nil || len(stream.Playlist.MediaParts) == 0 {
			continue
		}
		for _, segment := range stream.Playlist.MediaParts[0].MediaSegments {
			if segment.DateTime == nil {
				allHaveDateTime = false
				break
			}
		}
		if !allHaveDateTime {
			break
		}
	}

	if allHaveDateTime {
		// 通过DateTime同步
		var maxMinDate *time.Time

		for _, stream := range selectedStreams {
			if stream.Playlist == nil || len(stream.Playlist.MediaParts) == 0 {
				continue
			}

			var minDate *time.Time
			for _, segment := range stream.Playlist.MediaParts[0].MediaSegments {
				if segment.DateTime != nil && (minDate == nil || segment.DateTime.Before(*minDate)) {
					minDate = segment.DateTime
				}
			}

			if minDate != nil && (maxMinDate == nil || minDate.After(*maxMinDate)) {
				maxMinDate = minDate
			}
		}

		if maxMinDate != nil {
			for _, stream := range selectedStreams {
				if stream.Playlist == nil {
					continue
				}
				for _, part := range stream.Playlist.MediaParts {
					var newSegments []*entity.MediaSegment
					for _, segment := range part.MediaSegments {
						// 秒级同步，忽略毫秒
						if segment.DateTime != nil && segment.DateTime.Unix() >= maxMinDate.Unix() {
							newSegments = append(newSegments, segment)
						}
					}
					part.MediaSegments = newSegments
				}
			}
		}
	} else {
		// 通过index同步
		var maxMinIndex int64 = -1

		for _, stream := range selectedStreams {
			if stream.Playlist == nil || len(stream.Playlist.MediaParts) == 0 {
				continue
			}

			var minIndex int64 = -1
			for _, segment := range stream.Playlist.MediaParts[0].MediaSegments {
				if minIndex == -1 || segment.Index < minIndex {
					minIndex = segment.Index
				}
			}

			if minIndex != -1 && minIndex > maxMinIndex {
				maxMinIndex = minIndex
			}
		}

		if maxMinIndex != -1 {
			for _, stream := range selectedStreams {
				if stream.Playlist == nil {
					continue
				}
				for _, part := range stream.Playlist.MediaParts {
					var newSegments []*entity.MediaSegment
					for _, segment := range part.MediaSegments {
						if segment.Index >= maxMinIndex {
							newSegments = append(newSegments, segment)
						}
					}
					part.MediaSegments = newSegments
				}
			}
		}
	}

	// 取最新的N个分片
	// 检查是否有流的分片数超过takeLastCount
	hasMoreSegments := false
	for _, stream := range selectedStreams {
		if stream.Playlist != nil && len(stream.Playlist.MediaParts) > 0 {
			if len(stream.Playlist.MediaParts[0].MediaSegments) > takeLastCount {
				hasMoreSegments = true
				break
			}
		}
	}

	if hasMoreSegments {
		// 计算最小分片数
		minSegmentCount := -1
		for _, stream := range selectedStreams {
			if stream.Playlist == nil || len(stream.Playlist.MediaParts) == 0 {
				continue
			}
			segCount := len(stream.Playlist.MediaParts[0].MediaSegments)
			if minSegmentCount == -1 || segCount < minSegmentCount {
				minSegmentCount = segCount
			}
		}

		skipCount := minSegmentCount - takeLastCount + 1
		if skipCount < 0 {
			skipCount = 0
		}

		for _, stream := range selectedStreams {
			if stream.Playlist == nil {
				continue
			}
			for _, part := range stream.Playlist.MediaParts {
				if len(part.MediaSegments) > skipCount {
					part.MediaSegments = part.MediaSegments[skipCount:]
				}
			}
		}
	}
}

// ApplyCustomRange 应用自定义分片范围
func ApplyCustomRange(selectedStreams []*entity.StreamSpec, customRange *entity.CustomRange) {
	if customRange == nil {
		return
	}

	Logger.Info("发现自定义范围: " + customRange.InputStr)
	Logger.Warn("注意: 使用自定义范围可能导致音视频不同步")

	filterByIndex := customRange.StartSegIndex != nil && customRange.EndSegIndex != nil
	filterByTime := customRange.StartSec != nil && customRange.EndSec != nil

	if !filterByIndex && !filterByTime {
		Logger.Error("自定义范围格式无效")
		return
	}

	for _, stream := range selectedStreams {
		var skippedDur float64 = 0
		if stream.Playlist == nil {
			continue
		}

		for _, part := range stream.Playlist.MediaParts {
			var newSegments []*entity.MediaSegment

			if filterByIndex {
				for _, segment := range part.MediaSegments {
					if segment.Index >= *customRange.StartSegIndex && segment.Index <= *customRange.EndSegIndex {
						newSegments = append(newSegments, segment)
					}
				}
			} else {
				totalDur := 0.0
				for _, segment := range part.MediaSegments {
					if totalDur >= *customRange.StartSec && totalDur <= *customRange.EndSec {
						newSegments = append(newSegments, segment)
					}
					totalDur += segment.Duration
				}
			}

			if len(newSegments) > 0 {
				for _, segment := range part.MediaSegments {
					if segment.Index < newSegments[0].Index {
						skippedDur += segment.Duration
					} else {
						break
					}
				}
			}
			part.MediaSegments = newSegments
		}
		stream.SkippedDuration = &skippedDur
	}
}

// CleanAd 根据关键词清除广告分片
func CleanAd(selectedStreams []*entity.StreamSpec, keywords []string) {
	if len(keywords) == 0 {
		return
	}

	var regList []*regexp.Regexp
	for _, keyword := range keywords {
		if reg, err := regexp.Compile(keyword); err == nil {
			regList = append(regList, reg)
			Logger.Info("添加广告过滤关键词: " + keyword)
		}
	}

	for _, stream := range selectedStreams {
		if stream.Playlist == nil {
			continue
		}

		countBefore := stream.GetSegmentsCount()

		for _, part := range stream.Playlist.MediaParts {
			// 检查是否有广告分片
			hasAd := false
			for _, segment := range part.MediaSegments {
				for _, reg := range regList {
					if reg.MatchString(segment.URL) {
						hasAd = true
						break
					}
				}
				if hasAd {
					break
				}
			}

			if !hasAd {
				continue
			}

			// 过滤掉广告分片
			var newSegments []*entity.MediaSegment
			for _, segment := range part.MediaSegments {
				isAd := false
				for _, reg := range regList {
					if reg.MatchString(segment.URL) {
						isAd = true
						break
					}
				}
				if !isAd {
					newSegments = append(newSegments, segment)
				}
			}
			part.MediaSegments = newSegments
		}

		// 清理空的 part
		var newParts []*entity.MediaPart
		for _, part := range stream.Playlist.MediaParts {
			if len(part.MediaSegments) > 0 {
				newParts = append(newParts, part)
			}
		}
		stream.Playlist.MediaParts = newParts

		countAfter := stream.GetSegmentsCount()
		if countBefore != countAfter {
			Logger.Warn("段数变化: %d => %d", countBefore, countAfter)
		}
	}
}

// getOrder 获取音频流的声道优先级（对应C#版本的GetOrder）
func getOrder(streamSpec *entity.StreamSpec) int {
	if streamSpec.Channels == "" {
		return 0
	}

	parts := strings.Split(streamSpec.Channels, "/")
	if len(parts) == 0 {
		return 0
	}

	if order, err := strconv.Atoi(parts[0]); err == nil {
		return order
	}

	return 0
}

// SortStreams 按照C#版本的逻辑排序流
// OrderBy(MediaType).ThenByDescending(Bandwidth).ThenByDescending(GetOrder)
func SortStreams(streams []*entity.StreamSpec) []*entity.StreamSpec {
	// 创建副本以避免修改原始切片
	sorted := make([]*entity.StreamSpec, len(streams))
	copy(sorted, streams)

	// 使用Go的sort包进行多级排序
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]

		// 1. 按MediaType排序（null/Video < Audio < Subtitles）
		aType := getMediaTypeOrder(a.MediaType)
		bType := getMediaTypeOrder(b.MediaType)
		if aType != bType {
			return aType < bType
		}

		// 2. 按Bandwidth降序排序
		aBandwidth := 0
		if a.Bandwidth != nil {
			aBandwidth = *a.Bandwidth
		}
		bBandwidth := 0
		if b.Bandwidth != nil {
			bBandwidth = *b.Bandwidth
		}
		if aBandwidth != bBandwidth {
			return aBandwidth > bBandwidth // 降序
		}

		// 3. 按GetOrder（声道数）降序排序
		aOrder := getOrder(a)
		bOrder := getOrder(b)
		return aOrder > bOrder // 降序
	})

	return sorted
}

// getMediaTypeOrder 获取媒体类型的排序优先级
func getMediaTypeOrder(mediaType *entity.MediaType) int {
	if mediaType == nil {
		return 0 // 基本流/视频流
	}
	switch *mediaType {
	case entity.MediaTypeVideo:
		return 0 // 视频流
	case entity.MediaTypeAudio:
		return 1 // 音频流
	case entity.MediaTypeSubtitles:
		return 2 // 字幕流
	default:
		return 0
	}
}

// SelectStreams 选择流的主函数，对应C#版本的FilterUtil.SelectStreams
func SelectStreams(streams []*entity.StreamSpec) []*entity.StreamSpec {
	if len(streams) == 1 {
		return streams
	}

	// 首先按照C#版本的逻辑排序
	sortedStreams := SortStreams(streams)

	// 分类流
	var basicStreams, audioStreams, subtitleStreams []*entity.StreamSpec

	for _, stream := range sortedStreams {
		if stream.MediaType == nil || *stream.MediaType == entity.MediaTypeVideo {
			basicStreams = append(basicStreams, stream)
		} else if *stream.MediaType == entity.MediaTypeAudio {
			audioStreams = append(audioStreams, stream)
		} else if *stream.MediaType == entity.MediaTypeSubtitles {
			subtitleStreams = append(subtitleStreams, stream)
		}
	}

	// 使用交互式选择器并调用新的函数
	return SelectStreamsInteractive(sortedStreams)
}
