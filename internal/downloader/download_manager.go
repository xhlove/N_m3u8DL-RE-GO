package downloader

import (
	"N_m3u8DL-RE-GO/internal/entity"
	"N_m3u8DL-RE-GO/internal/util"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DownloadManager downloads and manages streams.
type DownloadManager struct {
	downloader       *SimpleDownloader
	config           *ManagerConfig
	selectedStreams  []*entity.StreamSpec
	outputFiles      []*OutputFile
	mu               sync.RWMutex
	mergeWaitGroup   sync.WaitGroup
	fileDictionaries map[*entity.StreamSpec]map[int]string
	streamKIDs       map[*entity.StreamSpec]string // Store KID per stream
	validationFailed bool
}

// OutputFile represents a downloaded and merged file.
type OutputFile struct {
	Index       int
	FilePath    string
	LangCode    string
	Description string
	MediaType   entity.MediaType
	Mediainfos  []*util.MediaInfo
}

// DownloadStreamResult represents the result of a single stream download.
type DownloadStreamResult struct {
	Success    bool
	StreamDir  string
	Error      error
	Mediainfos []*util.MediaInfo
}

// ManagerConfig holds the configuration for the DownloadManager.
type ManagerConfig struct {
	OutputDir              string
	SaveName               string
	SaveDir                string
	TmpDir                 string
	ThreadCount            int
	RetryCount             int
	Headers                map[string]string
	CheckLength            bool
	DeleteAfterDone        bool
	BinaryMerge            bool
	SkipMerge              bool
	ConcurrentDownload     bool
	NoAnsiColor            bool
	LogLevel               util.LogLevel
	FFmpegPath             string
	MkvmergePath           string
	MuxFormat              string
	UseAACFilter           bool
	MuxAfterDone           bool
	MuxOptions             *entity.MuxOptions
	UseFFmpegConcatDemuxer bool
	SubtitleFormat         string
	MP4RealTimeDecryption  bool
	Keys                   []string
	DecryptionBinaryPath   string
	DecryptionEngine       string
	KeyTextFile            string
}

// NewDownloadManager creates a new DownloadManager.
func NewDownloadManager(config *ManagerConfig, streams []*entity.StreamSpec) *DownloadManager {
	simpleConfig := &SimpleDownloadConfig{
		OutputDir:          config.OutputDir,
		SaveName:           config.SaveName,
		SaveDir:            config.SaveDir,
		TmpDir:             config.TmpDir,
		ThreadCount:        config.ThreadCount,
		RetryCount:         config.RetryCount,
		Headers:            config.Headers,
		CheckLength:        config.CheckLength,
		DeleteAfterDone:    config.DeleteAfterDone,
		BinaryMerge:        config.BinaryMerge,
		SkipMerge:          config.SkipMerge,
		ConcurrentDownload: config.ConcurrentDownload,
		NoAnsiColor:        config.NoAnsiColor,
		LogLevel:           config.LogLevel,
	}

	if config.SubtitleFormat == "" {
		config.SubtitleFormat = "srt"
	}

	return &DownloadManager{
		downloader:       NewSimpleDownloader(simpleConfig),
		config:           config,
		selectedStreams:  streams,
		outputFiles:      make([]*OutputFile, 0),
		fileDictionaries: make(map[*entity.StreamSpec]map[int]string),
		streamKIDs:       make(map[*entity.StreamSpec]string),
		validationFailed: false,
	}
}

// StartDownload starts the download process.
func (dm *DownloadManager) StartDownload() error {
	util.UI.Start()
	util.Logger.SetUIActive(true)

	// Defer the final actions
	defer func() {
		util.UI.Stop()
		util.Logger.SetUIActive(false)
		util.Logger.Info("所有下载任务完成")
	}()

	util.Logger.InfoMarkUp("[white on green]开始下载任务[/]")

	if dm.config.MuxAfterDone {
		dm.config.BinaryMerge = true
		util.Logger.WarnMarkUp("你已开启下载完成后混流，自动开启二进制合并")
	}

	streamTaskMap := make(map[*entity.StreamSpec]*util.Task)
	downloadResults := make(map[*entity.StreamSpec]*DownloadStreamResult)

	for i, stream := range dm.selectedStreams {
		description := dm.getStreamDescription(stream, i)
		// Placeholder values, will be updated in downloadSingleStreamOnly
		task := util.UI.AddTask(util.TaskTypeDownload, description, 0, 0)
		streamTaskMap[stream] = task
	}

	var downloadError error
	var downloadWg sync.WaitGroup
	downloadWg.Add(1)
	go func() {
		defer downloadWg.Done()
		if dm.config.ConcurrentDownload {
			downloadError = dm.downloadStreamsConcurrently(streamTaskMap, downloadResults)
		} else {
			downloadError = dm.downloadStreamsSequentially(streamTaskMap, downloadResults)
		}
	}()

	downloadWg.Wait()
	dm.mergeWaitGroup.Wait()

	if downloadError != nil {
		// Error is already logged
	}

	muxSuccess := true
	if dm.config.MuxAfterDone && len(dm.outputFiles) > 0 {
		util.Logger.InfoMarkUp("[white on green]开始混流处理[/]")
		muxSuccess = dm.performMuxAfterDone()
		if !muxSuccess {
			util.Logger.ErrorMarkUp("[white on red]混流失败[/]")
		}
	}

	if dm.config.DeleteAfterDone {
		// Check if all critical steps were successful
		// SkipMerge being true means we should not delete files if DeleteAfterDone is true,
		// as merging is a critical step for a "successful" operation in that context.
		allStepsSuccess := downloadError == nil && !dm.config.SkipMerge && muxSuccess && !dm.validationFailed
		if allStepsSuccess {
			dm.cleanupTempFiles()
			util.Logger.InfoMarkUp("任务成功完成，临时文件已清理")
		} else {
			// This 'else' covers cases where DeleteAfterDone is true, but something failed or was skipped.
			var reasons []string
			if downloadError != nil {
				reasons = append(reasons, "下载失败")
			}
			// If SkipMerge is true, and user wants to delete on success, we treat skipping merge as a reason to keep files.
			if dm.config.SkipMerge {
				reasons = append(reasons, "用户设置了跳过合并")
			}
			if !muxSuccess {
				reasons = append(reasons, "混流失败")
			}
			if dm.validationFailed {
				reasons = append(reasons, "文件验证或后处理失败")
			}

			if len(reasons) > 0 {
				util.Logger.InfoMarkUp("由于 (%s)，临时文件已保留以便调试。", strings.Join(reasons, ", "))
			} else if !allStepsSuccess {
				// Fallback message if no specific reason was caught by the checks above but allStepsSuccess is false.
				// This could happen if the logic for allStepsSuccess has a subtle case not covered by the explicit reason checks.
				util.Logger.InfoMarkUp("任务未能完全成功，临时文件已保留以便调试。")
			}
		}
	} else {
		// DeleteAfterDone is false
		util.Logger.InfoMarkUp("根据设置 (--delete-after-done=false)，临时文件已保留。")
	}

	return downloadError
}

func (dm *DownloadManager) downloadStreamsSequentially(tasks map[*entity.StreamSpec]*util.Task, results map[*entity.StreamSpec]*DownloadStreamResult) error {
	var firstError error
	for _, stream := range dm.selectedStreams {
		task := tasks[stream]
		result := dm.downloadSingleStreamOnly(stream, task)
		results[stream] = result
		if !result.Success {
			task.SetError(result.Error)
			if firstError == nil {
				firstError = fmt.Errorf("流 %s 下载失败: %v", dm.getStreamDescription(stream, task.ID), result.Error)
			}
		}
		if !dm.config.SkipMerge && result.Success {
			dm.mergeWaitGroup.Add(1)
			go dm.mergeStreamInBackground(stream, result, task)
		}
	}
	return firstError
}

func (dm *DownloadManager) downloadStreamsConcurrently(tasks map[*entity.StreamSpec]*util.Task, results map[*entity.StreamSpec]*DownloadStreamResult) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, len(dm.selectedStreams))

	for _, stream := range dm.selectedStreams {
		wg.Add(1)
		go func(s *entity.StreamSpec) {
			defer wg.Done()
			task := tasks[s]
			result := dm.downloadSingleStreamOnly(s, task)
			mu.Lock()
			results[s] = result
			mu.Unlock()
			if !result.Success {
				task.SetError(result.Error)
				errChan <- fmt.Errorf("流 %s 下载失败: %v", dm.getStreamDescription(s, task.ID), result.Error)
				return
			}
			if !dm.config.SkipMerge {
				dm.mergeWaitGroup.Add(1)
				go dm.mergeStreamInBackground(s, result, task)
			}
		}(stream)
	}

	wg.Wait()
	close(errChan)

	var firstError error
	for err := range errChan {
		if firstError == nil {
			firstError = err
		}
	}
	return firstError
}

func (dm *DownloadManager) downloadSingleStreamOnly(stream *entity.StreamSpec, task *util.Task) *DownloadStreamResult {
	result := &DownloadStreamResult{Success: false}

	if stream.Playlist == nil {
		result.Error = fmt.Errorf("流的播放列表为空")
		return result
	}

	dm.mu.Lock()
	if dm.fileDictionaries[stream] == nil {
		dm.fileDictionaries[stream] = make(map[int]string)
	}
	dm.mu.Unlock()

	segments := stream.Playlist.GetAllSegments()

	if len(segments) == 1 {
		splitSegments, err := util.SplitUrlAsync(segments[0], dm.config.Headers)
		if err == nil && splitSegments != nil {
			segments = splitSegments
			util.Logger.WarnMarkUp("检测到单个巨大分片，已自动切片进行并行下载")
		}
	}

	totalSegments := len(segments)
	if stream.Playlist.MediaInit != nil {
		totalSegments++
	}
	task.Total = float64(totalSegments)
	task.TotalCount = int64(totalSegments)
	if stream.Playlist.TotalBytes > 0 {
		task.TotalBytes = stream.Playlist.TotalBytes
	}

	streamDir := dm.getStreamOutputDir(stream, task.ID)
	result.StreamDir = streamDir
	if err := util.CreateDir(streamDir); err != nil {
		result.Error = fmt.Errorf("创建输出目录失败: %w", err)
		return result
	}

	util.Logger.InfoMarkUp("开始下载...%s", dm.getStreamShortDescription(stream))

	var currentKID string
	var readInfo bool
	speedContainer := task.GetSpeedContainer()
	var overallAesDecryptTask *util.Task  // Task for the entire stream's AES-128 decryption
	var overallCencDecryptTask *util.Task // Task for the entire stream's CENC decryption

	if stream.Playlist.MediaInit != nil {
		if !dm.config.BinaryMerge && (stream.MediaType == nil || *stream.MediaType != entity.MediaTypeSubtitles) {
			dm.config.BinaryMerge = true
			util.Logger.WarnMarkUp("检测到fMP4，自动开启二进制合并")
		}
		initPath := filepath.Join(streamDir, "_init.mp4.tmp")
		downloadResult := dm.downloader.DownloadSegment(stream.Playlist.MediaInit, initPath, speedContainer, dm.config.Headers, nil) // Pass nil for overallAesDecryptTask for init
		if downloadResult == nil || !downloadResult.Success {
			result.Error = fmt.Errorf("初始化段下载失败")
			return result
		}
		mp4InitFile := downloadResult.FilePath
		dm.mu.Lock()
		dm.fileDictionaries[stream][-1] = mp4InitFile
		dm.mu.Unlock()
		task.Increment(1)

		mp4Info, _ := util.GetMP4Info(mp4InitFile)
		currentKID = mp4Info.KID
		if key, _ := util.SearchKeyFromFile(dm.config.KeyTextFile, currentKID); key != "" {
			dm.config.Keys = append(dm.config.Keys, key)
		}

		// CENC decryption for init segment (if applicable)
		// This still creates a temporary task for the init segment only, as it's a single operation.
		// The overallCencDecryptTask is for the main segments.
		if dm.config.MP4RealTimeDecryption && currentKID != "" && len(dm.config.Keys) > 0 && stream.Playlist.MediaInit.EncryptInfo != nil && stream.Playlist.MediaInit.EncryptInfo.Method == entity.EncryptMethodCENC {
			decPath := strings.Replace(mp4InitFile, ".tmp", "_dec.tmp", 1)
			var segmentSize int64
			if info, err := os.Stat(mp4InitFile); err == nil {
				segmentSize = info.Size()
			}
			// Create a temporary task for this specific init segment CENC decryption
			initCencDecryptTask := util.UI.AddTask(util.TaskTypeDecrypt, filepath.Base(mp4InitFile)+"(Init CENC)", 1, segmentSize)
			if success, _ := util.Decrypt(dm.config.DecryptionEngine, dm.config.DecryptionBinaryPath, dm.config.Keys, mp4InitFile, decPath, currentKID, initCencDecryptTask); success {
				dm.mu.Lock()
				dm.fileDictionaries[stream][-1] = decPath
				dm.mu.Unlock()
			} else {
				// Handle init segment CENC decryption failure if necessary
				util.Logger.Error("Init segment CENC decryption failed for %s", mp4InitFile)
			}
		}

		util.Logger.WarnMarkUp("读取媒体信息...")
		infos, err := util.GetMediaInfo(dm.config.FFmpegPath, dm.fileDictionaries[stream][-1])
		if err == nil {
			result.Mediainfos = infos
			dm.changeSpecInfo(stream, infos)
			for idx, info := range infos {
				util.Logger.InfoMarkUp("[grey][[%d]] %s, %s (%s), %s[/]", idx, info.Type, info.Format, info.FormatInfo, info.Bitrate)
			}
			readInfo = true
		}
	}

	if !dm.config.BinaryMerge {
		for _, seg := range segments { // Check all segments, not just the remaining ones
			if seg.EncryptInfo != nil && seg.EncryptInfo.Method == entity.EncryptMethodCENC {
				dm.config.BinaryMerge = true
				util.Logger.WarnMarkUp("检测到CENC加密，自动开启二进制合并")
				break
			}
		}
	}

	// Create overall decryption tasks *before* processing the first data segment (if any)
	// This ensures tasks exist if the first data segment itself needs decryption.
	if stream.Playlist != nil {
		// AES-128 overall task
		var aesEncryptedSegmentsCount int64
		var totalAesEncryptedBytes int64
		isStreamAesEncrypted := false
		for _, seg := range stream.Playlist.GetAllSegments() { // Iterate all segments for accurate count
			if seg.IsEncrypted && seg.EncryptInfo != nil && seg.EncryptInfo.Method == entity.EncryptMethodAES128 {
				isStreamAesEncrypted = true
				aesEncryptedSegmentsCount++
				if seg.ExpectLength != nil {
					totalAesEncryptedBytes += *seg.ExpectLength
				}
			}
		}
		if isStreamAesEncrypted && aesEncryptedSegmentsCount > 0 {
			if totalAesEncryptedBytes == 0 && stream.Playlist.TotalBytes > 0 {
				totalAesEncryptedBytes = stream.Playlist.TotalBytes // Fallback
			}
			overallAesDecryptTask = util.UI.AddTask(util.TaskTypeDecrypt, dm.getStreamDescription(stream, task.ID)+" (AES-128)", aesEncryptedSegmentsCount, totalAesEncryptedBytes)
		}

		// CENC overall task
		if dm.config.MP4RealTimeDecryption && currentKID != "" { // currentKID might be from init or first segment
			var cencSegmentsCount int64
			var totalCencBytes int64
			isStreamCencEncrypted := false
			for _, seg := range stream.Playlist.GetAllSegments() { // Iterate all segments
				if seg.IsEncrypted && seg.EncryptInfo != nil && seg.EncryptInfo.Method == entity.EncryptMethodCENC {
					isStreamCencEncrypted = true
					cencSegmentsCount++
					if seg.ExpectLength != nil {
						totalCencBytes += *seg.ExpectLength
					}
				}
			}
			if isStreamCencEncrypted && cencSegmentsCount > 0 {
				if totalCencBytes == 0 && stream.Playlist.TotalBytes > 0 {
					totalCencBytes = stream.Playlist.TotalBytes // Fallback
				}
				overallCencDecryptTask = util.UI.AddTask(util.TaskTypeDecrypt, dm.getStreamDescription(stream, task.ID)+" (CENC)", cencSegmentsCount, totalCencBytes)
			}
		}
	}

	if len(segments) > 0 && (stream.Playlist.MediaInit == nil || stream.ExtractorType == entity.ExtractorTypeMSS) {
		firstSegment := segments[0]
		// segments = segments[1:] // Process first segment, then the rest

		padLength := len(fmt.Sprintf("%d", len(stream.Playlist.GetAllSegments())))
		ext := "ts"
		if stream.Extension != "" {
			ext = stream.Extension
		}
		fileName := fmt.Sprintf("%0*d.%s.tmp", padLength, firstSegment.Index, ext)
		segmentPath := filepath.Join(streamDir, fileName)

		// Pass overallAesDecryptTask for AES-128 decryption within DownloadSegment
		downloadResult := dm.downloader.DownloadSegment(firstSegment, segmentPath, speedContainer, dm.config.Headers, overallAesDecryptTask)
		if downloadResult == nil || !downloadResult.Success {
			result.Error = fmt.Errorf("第一个分片下载失败")
			return result
		}
		task.Increment(1)

		decryptedFilePath := downloadResult.FilePath
		// Handle CENC decryption for the first segment if applicable, using overallCencDecryptTask
		if dm.config.MP4RealTimeDecryption && currentKID == "" { // If KID wasn't from init, try to get it now
			mp4Info, _ := util.GetMP4Info(downloadResult.FilePath)
			currentKID = mp4Info.KID
			if key, _ := util.SearchKeyFromFile(dm.config.KeyTextFile, currentKID); key != "" {
				dm.config.Keys = append(dm.config.Keys, key)
			}
			// Re-evaluate overallCencDecryptTask creation if KID is now available and task not yet created
			if overallCencDecryptTask == nil && dm.config.MP4RealTimeDecryption && currentKID != "" {
				var cencSegmentsCount int64
				var totalCencBytes int64
				isStreamCencEncrypted := false
				for _, seg := range stream.Playlist.GetAllSegments() {
					if seg.IsEncrypted && seg.EncryptInfo != nil && seg.EncryptInfo.Method == entity.EncryptMethodCENC {
						isStreamCencEncrypted = true
						cencSegmentsCount++
						if seg.ExpectLength != nil {
							totalCencBytes += *seg.ExpectLength
						}
					}
				}
				if isStreamCencEncrypted && cencSegmentsCount > 0 {
					if totalCencBytes == 0 && stream.Playlist.TotalBytes > 0 {
						totalCencBytes = stream.Playlist.TotalBytes
					}
					overallCencDecryptTask = util.UI.AddTask(util.TaskTypeDecrypt, dm.getStreamDescription(stream, task.ID)+" (CENC)", cencSegmentsCount, totalCencBytes)
				}
			}
		}

		if dm.config.MP4RealTimeDecryption && currentKID != "" && len(dm.config.Keys) > 0 && firstSegment.EncryptInfo != nil && firstSegment.EncryptInfo.Method == entity.EncryptMethodCENC {
			decPath := strings.Replace(downloadResult.FilePath, ".tmp", "_dec.tmp", 1)
			if success, _ := util.Decrypt(dm.config.DecryptionEngine, dm.config.DecryptionBinaryPath, dm.config.Keys, downloadResult.FilePath, decPath, currentKID, overallCencDecryptTask); success {
				decryptedFilePath = decPath
			} else {
				if overallCencDecryptTask != nil { // Check if task exists
					overallCencDecryptTask.SetError(fmt.Errorf("CENC实时解密失败: %s", filepath.Base(downloadResult.FilePath)))
				}
			}
		}
		dm.mu.Lock()
		dm.fileDictionaries[stream][int(firstSegment.Index)] = decryptedFilePath
		dm.mu.Unlock()

		if stream.Playlist.MediaInit == nil { // Only read media info if no init segment
			if !readInfo {
				util.Logger.WarnMarkUp("读取媒体信息...")
				infos, err := util.GetMediaInfo(dm.config.FFmpegPath, decryptedFilePath)
				if err == nil {
					result.Mediainfos = infos
					dm.changeSpecInfo(stream, infos)
					if stream.MediaType != nil && *stream.MediaType == entity.MediaTypeAudio {
						dm.config.BinaryMerge = false // If it's audio, prefer ffmpeg merge
					}
					for idx, info := range infos {
						util.Logger.InfoMarkUp("[grey][[%d]] %s, %s (%s), %s[/]", idx, info.Type, info.Format, info.FormatInfo, info.Bitrate)
					}
				}
			}
		}

		if stream.ExtractorType == entity.ExtractorTypeMSS {
			util.Logger.Info("正在为MSS流生成init box...")
			processor, err := util.NewMSSMoovProcessor(stream)
			if err != nil {
				result.Error = fmt.Errorf("创建MSS处理器失败: %w", err)
				return result
			}
			firstSegmentBytes, err := os.ReadFile(decryptedFilePath)
			if err != nil {
				result.Error = fmt.Errorf("读取第一个分片失败: %w", err)
				return result
			}
			header, err := processor.GenHeader(firstSegmentBytes)
			if err != nil {
				result.Error = fmt.Errorf("生成MSS头部失败: %w", err)
				return result
			}
			initFilePath := dm.fileDictionaries[stream][-1]             // Assuming init is always -1
			if initFilePath == "" && stream.Playlist.MediaInit != nil { // Should not happen if MediaInit is nil
				initFilePath = filepath.Join(streamDir, "_init.mp4.tmp") // Fallback, though logic implies init should exist
				dm.mu.Lock()
				dm.fileDictionaries[stream][-1] = initFilePath
				dm.mu.Unlock()
			}
			if err := os.WriteFile(initFilePath, header, 0644); err != nil {
				result.Error = fmt.Errorf("写入MSS头部失败: %w", err)
				return result
			}
			util.Logger.Info("MSS init box生成并写入成功")
		}
		segments = segments[1:] // Now remove the first segment for the loop
	}

	dm.mu.Lock()
	dm.streamKIDs[stream] = currentKID
	dm.mu.Unlock()

	if success := dm.downloadSegments(segments, streamDir, task, stream, currentKID, overallAesDecryptTask, overallCencDecryptTask); !success {
		if overallAesDecryptTask != nil && !overallAesDecryptTask.IsFinished {
			overallAesDecryptTask.SetError(fmt.Errorf("依赖的下载任务失败"))
		}
		if overallCencDecryptTask != nil && !overallCencDecryptTask.IsFinished {
			overallCencDecryptTask.SetError(fmt.Errorf("依赖的下载任务失败"))
		}
		result.Error = fmt.Errorf("分段下载失败")
		return result
	}

	result.Success = true
	return result
}

func (dm *DownloadManager) downloadSegments(segments []*entity.MediaSegment, outputDir string, task *util.Task, stream *entity.StreamSpec, currentKID string, overallAesDecryptTask *util.Task, overallCencDecryptTask *util.Task) bool {
	padLength := len(fmt.Sprintf("%d", len(stream.Playlist.GetAllSegments()))) // Use all segments for pad length
	speedContainer := task.GetSpeedContainer()
	if dm.config.ThreadCount <= 1 {
		for _, segment := range segments {
			ext := "ts"
			if stream.Extension != "" {
				ext = stream.Extension
			}
			fileName := fmt.Sprintf("%0*d.%s.tmp", padLength, segment.Index, ext)
			segmentPath := filepath.Join(outputDir, fileName)
			downloadSegResult := dm.downloader.DownloadSegment(segment, segmentPath, speedContainer, dm.config.Headers, overallAesDecryptTask) // AES handled by simple downloader
			if downloadSegResult != nil && downloadSegResult.Success {
				decryptedFilePath := downloadSegResult.FilePath
				if dm.config.MP4RealTimeDecryption && currentKID != "" && len(dm.config.Keys) > 0 && segment.EncryptInfo != nil && segment.EncryptInfo.Method == entity.EncryptMethodCENC {
					decPath := strings.Replace(downloadSegResult.FilePath, ".tmp", "_dec.tmp", 1)
					if success, _ := util.Decrypt(dm.config.DecryptionEngine, dm.config.DecryptionBinaryPath, dm.config.Keys, downloadSegResult.FilePath, decPath, currentKID, overallCencDecryptTask); success {
						decryptedFilePath = decPath
					} else {
						if overallCencDecryptTask != nil {
							overallCencDecryptTask.SetError(fmt.Errorf("CENC实时解密失败: %s", filepath.Base(downloadSegResult.FilePath)))
						}
						// Decide if one CENC failure should stop everything or just mark this segment
					}
				}
				dm.mu.Lock()
				dm.fileDictionaries[stream][int(segment.Index)] = decryptedFilePath
				dm.mu.Unlock()
				task.Increment(1)
			} else {
				if dm.config.CheckLength {
					return false
				}
			}
		}
	} else {
		return dm.downloadSegmentsConcurrently(segments, outputDir, task, padLength, stream, currentKID, overallAesDecryptTask, overallCencDecryptTask)
	}
	return true
}

func (dm *DownloadManager) downloadSegmentsConcurrently(segments []*entity.MediaSegment, outputDir string, task *util.Task, padLength int, stream *entity.StreamSpec, currentKID string, overallAesDecryptTask *util.Task, overallCencDecryptTask *util.Task) bool {
	maxWorkers := dm.config.ThreadCount
	speedContainer := task.GetSpeedContainer()
	segmentChan := make(chan struct {
		segment *entity.MediaSegment
		index   int
	}, len(segments))
	var wg sync.WaitGroup
	var failureCount int32

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range segmentChan {
				ext := "ts"
				if stream.Extension != "" {
					ext = stream.Extension
				}
				fileName := fmt.Sprintf("%0*d.%s.tmp", padLength, item.index, ext)
				segmentPath := filepath.Join(outputDir, fileName)
				downloadSegResult := dm.downloader.DownloadSegment(item.segment, segmentPath, speedContainer, dm.config.Headers, overallAesDecryptTask) // AES handled by simple downloader
				if downloadSegResult != nil && downloadSegResult.Success {
					decryptedFilePath := downloadSegResult.FilePath
					if dm.config.MP4RealTimeDecryption && currentKID != "" && len(dm.config.Keys) > 0 && item.segment.EncryptInfo.Method == entity.EncryptMethodCENC {
						decPath := strings.Replace(downloadSegResult.FilePath, ".tmp", "_dec.tmp", 1)
						if success, _ := util.Decrypt(dm.config.DecryptionEngine, dm.config.DecryptionBinaryPath, dm.config.Keys, downloadSegResult.FilePath, decPath, currentKID, overallCencDecryptTask); success {
							decryptedFilePath = decPath
						} else {
							if overallCencDecryptTask != nil {
								overallCencDecryptTask.SetError(fmt.Errorf("CENC实时解密失败: %s", filepath.Base(downloadSegResult.FilePath)))
							}
							// Decide if one CENC failure should stop everything
						}
					}
					dm.mu.Lock()
					dm.fileDictionaries[stream][item.index] = decryptedFilePath
					dm.mu.Unlock()
					task.Increment(1)
				} else {
					failureCount++
				}
			}
		}()
	}

	go func() {
		defer close(segmentChan)
		for _, segment := range segments {
			segmentChan <- struct {
				segment *entity.MediaSegment
				index   int
			}{segment, int(segment.Index)}
		}
	}()

	wg.Wait()
	return !(failureCount > 0 && dm.config.CheckLength)
}

func (dm *DownloadManager) mergeSegments(inputDir string, outputPath *string, stream *entity.StreamSpec) bool {
	mediaType := entity.MediaTypeVideo
	if stream.MediaType != nil {
		mediaType = *stream.MediaType
	}

	if mediaType == entity.MediaTypeSubtitles || dm.config.BinaryMerge || dm.config.FFmpegPath == "" {
		return dm.binaryMergeFiles(inputDir, *outputPath, stream)
	}

	return dm.ffmpegMergeFiles(inputDir, *outputPath, stream, inputDir)
}

func (dm *DownloadManager) binaryMergeFiles(inputDir, outputPath string, stream *entity.StreamSpec) bool {
	dm.mu.RLock()
	fileDic, exists := dm.fileDictionaries[stream]
	dm.mu.RUnlock()
	if !exists {
		return false
	}

	var indices []int
	for index := range fileDic {
		indices = append(indices, index)
	}
	for i := 0; i < len(indices)-1; i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[i] > indices[j] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	var files []string
	for _, index := range indices {
		files = append(files, fileDic[index])
	}

	return util.CombineMultipleFilesIntoSingleFile(files, outputPath) == nil
}

func (dm *DownloadManager) ffmpegMergeFiles(inputDir, outputPath string, stream *entity.StreamSpec, workingDir string) bool {
	dm.mu.RLock()
	fileDic, exists := dm.fileDictionaries[stream]
	dm.mu.RUnlock()
	if !exists {
		return false
	}

	var indices []int
	for index := range fileDic {
		indices = append(indices, index)
	}
	for i := 0; i < len(indices)-1; i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[i] > indices[j] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	var files []string
	for _, index := range indices {
		files = append(files, fileDic[index])
	}

	if len(files) == 0 {
		return false
	}

	if len(files) > 1000 {
		util.Logger.InfoMarkUp("文件数量过多 (%d)，进行预合并", len(files))
		newFiles, err := util.PartialCombineMultipleFiles(files)
		if err != nil {
			util.Logger.Error("预合并失败: %s", err.Error())
			return false
		}
		files = newFiles
	}

	outputBase := strings.TrimSuffix(outputPath, filepath.Ext(outputPath))
	muxFormat := "MP4"
	if stream.MediaType != nil && *stream.MediaType == entity.MediaTypeAudio {
		muxFormat = "M4A"
	}

	return util.MergeByFFmpeg(dm.config.FFmpegPath, files, outputBase, muxFormat, dm.config.UseAACFilter, &util.MergeOptions{UseConcatDemuxer: dm.config.UseFFmpegConcatDemuxer}, workingDir) == nil
}

func (dm *DownloadManager) mergeStreamInBackground(stream *entity.StreamSpec, result *DownloadStreamResult, task *util.Task) {
	defer dm.mergeWaitGroup.Done()

	if err := dm.postProcessStreamData(stream, result); err != nil {
		util.Logger.Error("数据后处理失败: %v", err)
		dm.mu.Lock()
		dm.validationFailed = true
		dm.mu.Unlock()
		return
	}

	outputPath := dm.getOutputPath(stream, task.ID)

	// --- MERGE TASK & PROGRESS ---
	dm.mu.RLock()
	fileDic := dm.fileDictionaries[stream]
	dm.mu.RUnlock()

	var totalSize int64
	for _, filePath := range fileDic {
		if info, err := os.Stat(filePath); err == nil {
			totalSize += info.Size()
		}
	}

	mergeTask := util.UI.AddTask(util.TaskTypeMerge, dm.getStreamDescription(stream, task.ID), 1, totalSize)
	stopProgress := make(chan bool)
	go func() {
		var lastSize int64
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopProgress:
				return
			case <-ticker.C:
				// 检查预期的输出路径，如果FFmpeg合并，则检查实际的输出路径
				checkPath := outputPath
				mediaType := entity.MediaTypeVideo
				if stream.MediaType != nil {
					mediaType = *stream.MediaType
				}
				if !dm.config.BinaryMerge && mediaType != entity.MediaTypeSubtitles && dm.config.FFmpegPath != "" {
					outputBase := strings.TrimSuffix(outputPath, filepath.Ext(outputPath))
					muxFormat := "MP4"
					if stream.MediaType != nil && *stream.MediaType == entity.MediaTypeAudio {
						muxFormat = "M4A"
					}
					checkPath = outputBase + util.GetMuxExtension(muxFormat)
				}

				if info, err := os.Stat(checkPath); err == nil {
					currentSize := info.Size()
					increment := currentSize - lastSize
					if increment > 0 {
						mergeTask.GetSpeedContainer().Add(increment)
					}
					mergeTask.Update(0, currentSize)
					lastSize = currentSize
				} else if !os.IsNotExist(err) { // Log error if it's not "file not found"
					util.Logger.Debug("Error stating file for merge progress: %s, %v", checkPath, err)
				}
			}
		}
	}()

	mergeSuccess := dm.mergeSegments(result.StreamDir, &outputPath, stream)
	close(stopProgress) // Stop the progress checker

	finalOutputPath := outputPath
	// 如果使用FFmpeg合并，则修复文件路径
	mediaType := entity.MediaTypeVideo
	if stream.MediaType != nil {
		mediaType = *stream.MediaType
	}
	if !dm.config.BinaryMerge && mediaType != entity.MediaTypeSubtitles && dm.config.FFmpegPath != "" {
		outputBase := strings.TrimSuffix(outputPath, filepath.Ext(outputPath))
		muxFormat := "MP4"
		if stream.MediaType != nil && *stream.MediaType == entity.MediaTypeAudio {
			muxFormat = "M4A"
		}
		finalOutputPath = outputBase + util.GetMuxExtension(muxFormat) // 使用 util.GetMuxExtension
	}

	if mergeSuccess {
		var actualMergedSize int64
		if info, err := os.Stat(finalOutputPath); err == nil { // 使用 finalOutputPath
			actualMergedSize = info.Size()
		} else {
			util.Logger.Warn("无法获取合并后文件 %s 的大小，将使用预估大小进行进度更新", finalOutputPath) // 使用 finalOutputPath
			actualMergedSize = totalSize                                        // totalSize is sum of pre-merge segments
		}
		mergeTask.Update(1, actualMergedSize) // Mark as complete using actual merged size
		mergeTask.ProcessedCount = 1
		dm.mu.RLock()
		currentKID := dm.streamKIDs[stream]
		dm.mu.RUnlock()

		// Post-merge decryption is ONLY for CENC (KID-based) when not using real-time decryption.
		// AES-128 is decrypted segment by segment in SimpleDownloader.
		if !dm.config.MP4RealTimeDecryption && currentKID != "" && len(dm.config.Keys) > 0 && dm.config.DecryptionEngine == "MP4DECRYPT" {
			util.Logger.Info("正在对合并后的CENC加密文件进行解密...")
			var decryptTotalSize int64
			if info, err := os.Stat(finalOutputPath); err == nil { // 使用 finalOutputPath
				decryptTotalSize = info.Size()
			}
			decryptTask := util.UI.AddTask(util.TaskTypeDecrypt, filepath.Base(finalOutputPath), 1, decryptTotalSize)              // 使用 finalOutputPath
			decPath := strings.TrimSuffix(finalOutputPath, filepath.Ext(finalOutputPath)) + "_dec" + filepath.Ext(finalOutputPath) // 使用 finalOutputPath

			if success, _ := util.Decrypt(dm.config.DecryptionEngine, dm.config.DecryptionBinaryPath, dm.config.Keys, finalOutputPath, decPath, currentKID, decryptTask); success { // 使用 finalOutputPath
				os.Remove(finalOutputPath)              // 使用 finalOutputPath
				os.Rename(decPath, finalOutputPath)     // 使用 finalOutputPath
				decryptTask.Update(1, decryptTotalSize) // Mark as complete
				decryptTask.ProcessedCount = 1
			} else {
				// Decryption failed, mark merge as failed too for consistency
				mergeTask.SetError(fmt.Errorf("合并后CENC解密失败"))
				return // Stop further processing
			}
		}

		dm.mu.Lock()
		defer dm.mu.Unlock()

		isAlreadyAdded := false
		for _, f := range dm.outputFiles {
			if f.Index == task.ID {
				f.FilePath = finalOutputPath // 使用 finalOutputPath
				isAlreadyAdded = true
				break
			}
		}

		if !isAlreadyAdded {
			dm.outputFiles = append(dm.outputFiles, &OutputFile{
				Index:       task.ID,
				FilePath:    finalOutputPath, // 使用 finalOutputPath
				LangCode:    stream.Language,
				Description: stream.Name,
				MediaType:   *stream.MediaType,
				Mediainfos:  result.Mediainfos,
			})
		}
	} else {
		err := fmt.Errorf("合并失败: %s", dm.getStreamDescription(stream, task.ID))
		mergeTask.SetError(err)
		dm.mu.Lock()
		dm.validationFailed = true
		dm.mu.Unlock()
	}
}

func (dm *DownloadManager) postProcessStreamData(stream *entity.StreamSpec, result *DownloadStreamResult) error {
	totalExpectedSegments := len(stream.Playlist.GetAllSegments())
	if stream.Playlist.MediaInit != nil {
		totalExpectedSegments++
	}
	dm.mu.RLock()
	downloadedCount := len(dm.fileDictionaries[stream])
	dm.mu.RUnlock()
	if downloadedCount < totalExpectedSegments {
		msg := fmt.Sprintf("分片数量校验未通过，预期: %d, 实际: %d", totalExpectedSegments, downloadedCount)
		util.Logger.Error(msg)
		return fmt.Errorf(msg)
	}

	mediaType := entity.MediaTypeVideo
	if stream.MediaType != nil {
		mediaType = *stream.MediaType
	}

	if mediaType == entity.MediaTypeSubtitles {
		tempFixedPath := filepath.Join(result.StreamDir, "fixed_sub.tmp")
		finalSubPath := ""
		var err error

		if stream.Extension == "m4s" && strings.Contains(stream.Codecs, "stpp") {
			util.Logger.WarnMarkUp("正在提取TTML(mp4)字幕...")

			dm.mu.RLock()
			fileDic := dm.fileDictionaries[stream]
			dm.mu.RUnlock()

			var files []string
			var indices []int
			for index := range fileDic {
				indices = append(indices, index)
			}
			for i := 0; i < len(indices)-1; i++ {
				for j := i + 1; j < len(indices); j++ {
					if indices[i] > indices[j] {
						indices[i], indices[j] = indices[j], indices[i]
					}
				}
			}
			for _, index := range indices {
				if path, ok := fileDic[index]; ok {
					files = append(files, path)
				}
			}

			vttSub, err := util.ExtractTTMLSubsFromMp4s(files)
			if err != nil {
				return fmt.Errorf("TTML字幕提取失败: %w", err)
			}

			finalSubPath, err = dm.writeSubtitleFile(vttSub, tempFixedPath)
			if err != nil {
				return err
			}
		} else if stream.Extension == "m4s" && strings.Contains(stream.Codecs, "wvtt") {
			util.Logger.WarnMarkUp("正在提取WebVTT(mp4)字幕...")
			tempMp4Path := filepath.Join(result.StreamDir, "temp_sub.mp4")
			if !dm.binaryMergeFiles(result.StreamDir, tempMp4Path, stream) {
				return fmt.Errorf("合并VTT MP4分片失败")
			}
			defer os.Remove(tempMp4Path)
			vttSub, err := util.ExtractVTTSubsFromMp4(tempMp4Path)
			if err != nil {
				return fmt.Errorf("WebVTT字幕提取失败: %w", err)
			}
			finalSubPath, err = dm.writeSubtitleFile(vttSub, tempFixedPath)
			if err != nil {
				return err
			}
		} else if stream.Extension == "vtt" {
			util.Logger.WarnMarkUp("正在修复VTT字幕...")
			finalSubPath, err = dm.fixRawSubtitles(stream, result.StreamDir, tempFixedPath, dm.fixVttSegment)
			if err != nil {
				return fmt.Errorf("修复VTT字幕失败: %w", err)
			}
		} else if stream.Extension == "ttml" {
			util.Logger.WarnMarkUp("正在修复TTML字幕...")
			finalSubPath, err = dm.fixRawSubtitles(stream, result.StreamDir, tempFixedPath, dm.fixTtmlSegment)
			if err != nil {
				return fmt.Errorf("修复TTML字幕失败: %w", err)
			}
		}

		if finalSubPath != "" {
			dm.mu.Lock()
			dm.fileDictionaries[stream] = make(map[int]string)
			dm.fileDictionaries[stream][0] = finalSubPath
			dm.mu.Unlock()
		}
	}

	return nil
}

func (dm *DownloadManager) fixRawSubtitles(stream *entity.StreamSpec, streamDir, tempFixedPath string,
	fixerFunc func(filePath string, mpegtsTimestamp int64) (*entity.WebVttSub, error)) (string, error) {

	dm.mu.RLock()
	fileDic := dm.fileDictionaries[stream]
	dm.mu.RUnlock()

	var indices []int
	for index := range fileDic {
		indices = append(indices, index)
	}
	for i := 0; i < len(indices)-1; i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[i] > indices[j] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	finalVtt := &entity.WebVttSub{}
	first := true

	for _, index := range indices {
		segmentPath := fileDic[index]
		vtt, err := fixerFunc(segmentPath, 0)
		if err != nil {
			util.Logger.Warn("修复字幕分片失败 %s: %v", segmentPath, err)
			continue
		}

		if first {
			finalVtt = vtt
			first = false
		} else {
			finalVtt.AddCuesFromOne(vtt)
		}
	}

	if stream.SkippedDuration != nil && *stream.SkippedDuration > 0 {
		offset := time.Duration(*stream.SkippedDuration * float64(time.Second))
		finalVtt.LeftShiftTime(offset)
	}

	if err := util.TryWriteImagePngs(finalVtt, streamDir); err != nil {
		util.Logger.Warn("写出图形字幕时出错: %v", err)
	}

	return dm.writeSubtitleFile(finalVtt, tempFixedPath)
}

func (dm *DownloadManager) fixVttSegment(filePath string, mpegtsTimestamp int64) (*entity.WebVttSub, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	vtt, err := entity.Parse(string(content))
	if err != nil {
		return nil, err
	}
	vtt.MpegtsTimestamp = mpegtsTimestamp
	return vtt, nil
}

func (dm *DownloadManager) fixTtmlSegment(filePath string, mpegtsTimestamp int64) (*entity.WebVttSub, error) {
	return util.ExtractFromTTML(filePath, mpegtsTimestamp)
}

func (dm *DownloadManager) writeSubtitleFile(sub *entity.WebVttSub, tempPathBase string) (string, error) {
	var subContent string
	var finalSubPath string

	if strings.ToLower(dm.config.SubtitleFormat) == "vtt" {
		subContent = sub.ToVtt()
		finalSubPath = tempPathBase + ".vtt"
	} else {
		subContent = sub.ToSrt()
		finalSubPath = tempPathBase + ".srt"
	}

	if err := os.WriteFile(finalSubPath, []byte(subContent), 0644); err != nil {
		return "", fmt.Errorf("写入修复后的字幕文件失败: %w", err)
	}

	return finalSubPath, nil
}

func (dm *DownloadManager) performMuxAfterDone() bool {
	if dm.config.MuxOptions == nil {
		return false
	}

	dm.mu.RLock()
	defer dm.mu.RUnlock()

	var videoFiles, audioFiles, subtitleFiles []*OutputFile
	for _, file := range dm.outputFiles {
		switch file.MediaType {
		case entity.MediaTypeVideo:
			videoFiles = append(videoFiles, file)
		case entity.MediaTypeAudio:
			audioFiles = append(audioFiles, file)
		case entity.MediaTypeSubtitles:
			if !dm.config.MuxOptions.SkipSubtitle {
				subtitleFiles = append(subtitleFiles, file)
			}
		}
	}

	if dm.config.MuxOptions.MuxImports != nil {
		for _, importFile := range dm.config.MuxOptions.MuxImports {
			if importFile.Type == "audio" {
				audioFiles = append(audioFiles, &OutputFile{FilePath: importFile.FilePath, LangCode: importFile.LangCode, Description: importFile.Description, MediaType: entity.MediaTypeAudio})
			} else if importFile.Type == "subtitle" {
				subtitleFiles = append(subtitleFiles, &OutputFile{FilePath: importFile.FilePath, LangCode: importFile.LangCode, Description: importFile.Description, MediaType: entity.MediaTypeSubtitles})
			}
		}
	}

	if len(videoFiles) == 0 {
		util.Logger.Warn("准备进行混流操作，但在已下载/合并的文件列表中没有找到可用的视频轨道。")

		selectedVideoCount := 0
		for _, s := range dm.selectedStreams {
			if s.MediaType != nil && *s.MediaType == entity.MediaTypeVideo {
				selectedVideoCount++
			}
		}
		if selectedVideoCount > 0 {
			util.Logger.Warn(fmt.Sprintf("程序最初选择了 %d 个视频流进行下载，但它们未能成功生成最终文件或在之前的步骤中失败。", selectedVideoCount))
		}

		hasExternalVideo := false
		if dm.config.MuxOptions.MuxImports != nil {
			for _, imp := range dm.config.MuxOptions.MuxImports {
				if strings.ToLower(imp.Type) == "video" {
					hasExternalVideo = true
					util.Logger.Info(fmt.Sprintf("检测到外部导入的视频文件: %s", imp.FilePath))
					break
				}
			}
		}

		if !hasExternalVideo && len(audioFiles) == 0 {
			util.Logger.Warn("同时，在通过 -MuxImport 指定的外部文件中也没有视频轨道。混流操作将被跳过。")
			return true // Skip muxing if no video internally and no video externally
		}
		if !hasExternalVideo && len(audioFiles) > 0 {
			// This is the audio-only case
		} else {
			util.Logger.Info("将尝试仅使用通过 -MuxImport 指定的外部视频轨道进行混流。")
		}
	}

	allMuxSuccess := true
	if len(videoFiles) > 0 {
		// Video-based muxing
		for _, videoFile := range videoFiles {
			inputs := []*util.OutputFile{
				{FilePath: videoFile.FilePath, MediaType: videoFile.MediaType, LangCode: videoFile.LangCode, Description: videoFile.Description},
			}
			for _, audioFile := range audioFiles {
				inputs = append(inputs, &util.OutputFile{FilePath: audioFile.FilePath, MediaType: audioFile.MediaType, LangCode: audioFile.LangCode, Description: audioFile.Description})
			}
			for _, subtitleFile := range subtitleFiles {
				inputs = append(inputs, &util.OutputFile{FilePath: subtitleFile.FilePath, MediaType: subtitleFile.MediaType, LangCode: subtitleFile.LangCode, Description: subtitleFile.Description})
			}

			baseName := strings.TrimSuffix(videoFile.FilePath, filepath.Ext(videoFile.FilePath))
			if !dm.executeMux(inputs, baseName) {
				allMuxSuccess = false
			}
		}
	} else if len(audioFiles) > 0 {
		// Audio-only muxing
		inputs := []*util.OutputFile{}
		for _, audioFile := range audioFiles {
			inputs = append(inputs, &util.OutputFile{FilePath: audioFile.FilePath, MediaType: audioFile.MediaType, LangCode: audioFile.LangCode, Description: audioFile.Description})
		}
		for _, subtitleFile := range subtitleFiles {
			inputs = append(inputs, &util.OutputFile{FilePath: subtitleFile.FilePath, MediaType: subtitleFile.MediaType, LangCode: subtitleFile.LangCode, Description: subtitleFile.Description})
		}

		if len(inputs) > 0 {
			var baseName string
			if dm.config.SaveName != "" {
				// Ensure the directory exists
				if err := os.MkdirAll(dm.config.SaveDir, 0755); err != nil {
					util.Logger.Error("创建输出目录失败: %v", err)
					return false
				}
				baseName = filepath.Join(dm.config.SaveDir, dm.config.SaveName)
			} else {
				baseName = strings.TrimSuffix(audioFiles[0].FilePath, filepath.Ext(audioFiles[0].FilePath))
			}
			if !dm.executeMux(inputs, baseName) {
				allMuxSuccess = false
			}
		}
	}

	return allMuxSuccess
}

func (dm *DownloadManager) cleanupTempFiles() {
	if dm.config.TmpDir != "" {
		os.RemoveAll(dm.config.TmpDir)
	}
}

func (dm *DownloadManager) getStreamDescription(stream *entity.StreamSpec, index int) string {
	return stream.ToShortString()
}

func (dm *DownloadManager) getStreamShortDescription(stream *entity.StreamSpec) string {
	return stream.ToString()
}

func (dm *DownloadManager) getStreamOutputDir(stream *entity.StreamSpec, taskID int) string {
	dirName := fmt.Sprintf("%d_%s", taskID, dm.sanitizeFileName(stream.ToShortString()))
	if dm.config.TmpDir != "" {
		return filepath.Join(dm.config.TmpDir, dirName)
	}
	return filepath.Join(dm.config.OutputDir, dirName)
}

func (dm *DownloadManager) getOutputPath(stream *entity.StreamSpec, taskID int) string {
	saveDir := dm.config.SaveDir
	if saveDir == "" {
		saveDir = dm.config.OutputDir
	}

	var saveName string
	if dm.config.SaveName != "" {
		saveName = dm.config.SaveName
		if stream.Language != "" && stream.MediaType != nil && *stream.MediaType != entity.MediaTypeVideo {
			saveName = fmt.Sprintf("%s.%s", saveName, stream.Language)
		}
	} else {
		var parts []string
		if stream.GroupID != "" {
			parts = append(parts, stream.GroupID)
		}
		if stream.Codecs != "" {
			parts = append(parts, stream.Codecs)
		}
		if stream.Resolution != "" {
			parts = append(parts, stream.Resolution)
		}
		if stream.Bandwidth != nil {
			parts = append(parts, fmt.Sprintf("%d", *stream.Bandwidth))
		}
		if stream.Language != "" {
			parts = append(parts, stream.Language)
		}
		if len(parts) > 0 {
			saveName = strings.Join(parts, "_")
		} else {
			saveName = fmt.Sprintf("track_%d", taskID)
		}
	}

	finalSaveName := saveName
	counter := 1
	for {
		outputPath := filepath.Join(saveDir, dm.sanitizeFileName(finalSaveName)+dm.getOutputExtension(stream))
		if !util.FileExists(outputPath) {
			existsInOutput := false
			dm.mu.RLock()
			for _, f := range dm.outputFiles {
				if f.FilePath == outputPath {
					existsInOutput = true
					break
				}
			}
			dm.mu.RUnlock()
			if !existsInOutput {
				return outputPath
			}
		}
		finalSaveName = fmt.Sprintf("%s_%d", saveName, counter)
		counter++
	}
}

func (dm *DownloadManager) getOutputExtension(stream *entity.StreamSpec) string {
	if stream.MediaType != nil {
		if *stream.MediaType == entity.MediaTypeSubtitles {
			return "." + dm.config.SubtitleFormat
		}
		if *stream.MediaType == entity.MediaTypeAudio && (stream.Extension == "m4s" || stream.Extension == "mp4") {
			return ".m4a"
		}
	}
	if stream.Extension == "m4s" || stream.Extension == "mp4" {
		return ".mp4"
	}
	return ".ts"
}

func (dm *DownloadManager) sanitizeFileName(name string) string {
	return strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(name)
}

func (dm *DownloadManager) getMuxOutputPath() string {
	saveDir := dm.config.SaveDir
	if saveDir == "" {
		saveDir = dm.config.OutputDir
	}
	dirName := filepath.Base(dm.config.TmpDir)
	if dirName == "" || dirName == "." {
		dirName = dm.config.SaveName
		if dirName == "" {
			dirName = fmt.Sprintf("download_%s", time.Now().Format("2006-01-02_15-04-05"))
		}
	}
	return filepath.Join(saveDir, fmt.Sprintf("%s.MUX", dirName))
}

func (dm *DownloadManager) getMuxExtension() string {
	return util.GetMuxExtension(dm.config.MuxOptions.MuxFormat.String())
}

func (dm *DownloadManager) changeSpecInfo(stream *entity.StreamSpec, mediaInfos []*util.MediaInfo) {
	if !dm.config.BinaryMerge {
		for _, info := range mediaInfos {
			if info.DolbyVision {
				dm.config.BinaryMerge = true
				util.Logger.WarnMarkUp("检测到杜比视界(Dolby Vision)，自动开启二进制合并")
				break
			}
		}
	}

	allAudio := true
	for _, info := range mediaInfos {
		if info.Type != "Audio" {
			allAudio = false
			break
		}
	}
	if allAudio {
		mediaType := entity.MediaTypeAudio
		stream.MediaType = &mediaType
	}
}

func (dm *DownloadManager) executeMux(inputs []*util.OutputFile, baseName string) bool {
	util.Logger.Info("准备混流以下文件:")
	for _, f := range inputs {
		util.Logger.WarnMarkUp("[grey]%s[/]", filepath.Base(f.FilePath))
	}

	outputPath := fmt.Sprintf("%s.MUX", baseName)
	finalMuxPath := outputPath + dm.getMuxExtension()
	util.Logger.InfoMarkUp("Muxing to [grey]%s[/]", filepath.Base(finalMuxPath))

	var currentMuxSuccess bool
	workingDir, _ := os.Getwd()

	// --- MUX TASK & PROGRESS ---
	var totalSize int64
	for _, f := range inputs {
		if info, err := os.Stat(f.FilePath); err == nil {
			totalSize += info.Size()
		}
	}
	muxTask := util.UI.AddTask(util.TaskTypeMux, filepath.Base(finalMuxPath), 1, totalSize)
	stopProgress := make(chan bool)
	go func() {
		var lastSize int64
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopProgress:
				return
			case <-ticker.C:
				// Muxer might create the final file late, so check for the temp mux file first
				checkPath := finalMuxPath
				if _, err := os.Stat(checkPath); os.IsNotExist(err) {
					checkPath = outputPath // Check for intermediate file
				}
				if info, err := os.Stat(checkPath); err == nil {
					currentSize := info.Size()
					increment := currentSize - lastSize
					if increment > 0 {
						muxTask.GetSpeedContainer().Add(increment)
					}
					muxTask.Update(0, currentSize)
					lastSize = currentSize
				}
			}
		}
	}()

	if dm.config.MuxOptions.UseMkvmerge {
		err := util.MuxInputsByMkvmerge(dm.config.MkvmergePath, inputs, outputPath, workingDir)
		if err != nil {
			util.Logger.Error("Mkvmerge混流失败: %+v", err)
		}
		currentMuxSuccess = err == nil
	} else {
		err := util.MuxInputsByFFmpeg(dm.config.FFmpegPath, inputs, outputPath, dm.config.MuxOptions.MuxFormat.String(), true, workingDir)
		if err != nil {
			util.Logger.Error("FFmpeg混流失败: %+v", err)
		}
		currentMuxSuccess = err == nil
	}

	close(stopProgress) // Stop the progress checker

	if currentMuxSuccess {
		// Final update to 100%
		if info, err := os.Stat(finalMuxPath); err == nil {
			muxTask.Update(1, info.Size())
		}
		muxTask.ProcessedCount = 1
		util.Logger.InfoMarkUp("[white on green]混流完成[/]: %s", finalMuxPath)
	} else {
		err := fmt.Errorf("混流失败: %s", filepath.Base(finalMuxPath))
		muxTask.SetError(err)
	}
	return currentMuxSuccess
}
