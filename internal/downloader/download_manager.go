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
	speedContainers  map[int]*util.SpeedContainer
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
		config.SubtitleFormat = "srt" // 默认设为srt
	}

	return &DownloadManager{
		downloader:       NewSimpleDownloader(simpleConfig),
		config:           config,
		selectedStreams:  streams,
		outputFiles:      make([]*OutputFile, 0),
		speedContainers:  make(map[int]*util.SpeedContainer),
		fileDictionaries: make(map[*entity.StreamSpec]map[int]string),
		streamKIDs:       make(map[*entity.StreamSpec]string),
		validationFailed: false,
	}
}

// StartDownloadAsync starts the download process asynchronously.
func (dm *DownloadManager) StartDownloadAsync() error {
	util.Logger.InfoMarkUp("[white on green]开始下载任务[/]")

	if dm.config.MuxAfterDone {
		dm.config.BinaryMerge = true
		util.Logger.WarnMarkUp("你已开启下载完成后混流，自动开启二进制合并")
	}

	progress := util.NewProgress()
	tasks := make(map[*entity.StreamSpec]*util.ProgressTask)
	downloadResults := make(map[*entity.StreamSpec]*DownloadStreamResult)

	for i, stream := range dm.selectedStreams {
		description := dm.getStreamDescription(stream, i)
		task := progress.AddTask(description)
		tasks[stream] = task
		speedContainer := progress.GetSpeedContainer(task.ID)
		dm.speedContainers[task.ID] = speedContainer
	}

	var downloadError error
	progress.StartAsync(func(p *util.Progress) {
		if dm.config.ConcurrentDownload {
			downloadError = dm.downloadStreamsConcurrently(tasks, downloadResults)
		} else {
			downloadError = dm.downloadStreamsSequentially(tasks, downloadResults)
		}
	})

	if downloadError != nil {
		util.Logger.ErrorMarkUp("[white on red]下载失败[/]: %s", downloadError.Error())
		return downloadError
	}

	if !dm.config.SkipMerge {
		dm.mergeWaitGroup.Wait()
	}

	progress.Stop()
	time.Sleep(100 * time.Millisecond)

	muxSuccess := true
	if dm.config.MuxAfterDone && len(dm.outputFiles) > 0 {
		util.Logger.InfoMarkUp("[white on green]开始混流处理[/]")
		muxSuccess = dm.performMuxAfterDone()
		if !muxSuccess {
			util.Logger.ErrorMarkUp("[white on red]混流失败[/]")
		}
	}

	if dm.config.DeleteAfterDone && !dm.config.SkipMerge && muxSuccess && !dm.validationFailed {
		dm.cleanupTempFiles()
	} else if !muxSuccess || dm.validationFailed {
		util.Logger.InfoMarkUp("任务失败，保留临时文件以便调试")
	}

	return nil
}

func (dm *DownloadManager) downloadStreamsSequentially(tasks map[*entity.StreamSpec]*util.ProgressTask, results map[*entity.StreamSpec]*DownloadStreamResult) error {
	for i, stream := range dm.selectedStreams {
		task := tasks[stream]
		speedContainer := dm.speedContainers[task.ID]
		result := dm.downloadSingleStreamOnly(stream, task, speedContainer)
		results[stream] = result
		if !result.Success {
			return fmt.Errorf("流 %d 下载失败: %v", i, result.Error)
		}
		if !dm.config.SkipMerge {
			dm.mergeWaitGroup.Add(1)
			go dm.mergeStreamInBackground(stream, result, task)
		}
	}
	return nil
}

func (dm *DownloadManager) downloadStreamsConcurrently(tasks map[*entity.StreamSpec]*util.ProgressTask, results map[*entity.StreamSpec]*DownloadStreamResult) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, len(dm.selectedStreams))

	for i, stream := range dm.selectedStreams {
		wg.Add(1)
		go func(idx int, s *entity.StreamSpec) {
			defer wg.Done()
			task := tasks[s]
			speedContainer := dm.speedContainers[task.ID]
			result := dm.downloadSingleStreamOnly(s, task, speedContainer)
			mu.Lock()
			results[s] = result
			mu.Unlock()
			if !result.Success {
				errChan <- fmt.Errorf("流 %d 下载失败: %v", idx, result.Error)
				return
			}
			if !dm.config.SkipMerge {
				dm.mergeWaitGroup.Add(1)
				go dm.mergeStreamInBackground(s, result, task)
			}
		}(i, stream)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		return err
	}
	return nil
}

func (dm *DownloadManager) downloadSingleStreamOnly(stream *entity.StreamSpec, task *util.ProgressTask, speedContainer *util.SpeedContainer) *DownloadStreamResult {
	result := &DownloadStreamResult{Success: false}
	speedContainer.ResetVars()

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
	task.SetMaxValue(float64(totalSegments))
	task.StartTask()

	streamDir := dm.getStreamOutputDir(stream, task.ID)
	result.StreamDir = streamDir
	if err := util.CreateDir(streamDir); err != nil {
		result.Error = fmt.Errorf("创建输出目录失败: %w", err)
		return result
	}

	util.Logger.InfoMarkUp("开始下载...%s", dm.getStreamShortDescription(stream))

	var currentKID string
	var readInfo bool

	if stream.Playlist.MediaInit != nil {
		if !dm.config.BinaryMerge && (stream.MediaType == nil || *stream.MediaType != entity.MediaTypeSubtitles) {
			dm.config.BinaryMerge = true
			util.Logger.WarnMarkUp("检测到fMP4，自动开启二进制合并")
		}
		initPath := filepath.Join(streamDir, "_init.mp4.tmp")
		downloadResult := dm.downloader.DownloadSegment(stream.Playlist.MediaInit, initPath, speedContainer, dm.config.Headers)
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

		if dm.config.MP4RealTimeDecryption && currentKID != "" && len(dm.config.Keys) > 0 {
			decPath := strings.Replace(mp4InitFile, ".tmp", "_dec.tmp", 1)
			if success, _ := util.Decrypt(dm.config.DecryptionEngine, dm.config.DecryptionBinaryPath, dm.config.Keys, mp4InitFile, decPath, currentKID, false); success {
				dm.mu.Lock()
				dm.fileDictionaries[stream][-1] = decPath
				dm.mu.Unlock()
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
		for _, seg := range segments {
			if seg.EncryptInfo.Method == entity.EncryptMethodCENC {
				dm.config.BinaryMerge = true
				util.Logger.WarnMarkUp("检测到CENC加密，自动开启二进制合并")
				break
			}
		}
	}

	if len(segments) > 0 && (stream.Playlist.MediaInit == nil || stream.ExtractorType == entity.ExtractorTypeMSS) {
		firstSegment := segments[0]
		segments = segments[1:]
		padLength := len(fmt.Sprintf("%d", len(stream.Playlist.GetAllSegments())))
		ext := "ts"
		if stream.Extension != "" {
			ext = stream.Extension
		}
		fileName := fmt.Sprintf("%0*d.%s.tmp", padLength, firstSegment.Index, ext)
		segmentPath := filepath.Join(streamDir, fileName)
		downloadResult := dm.downloader.DownloadSegment(firstSegment, segmentPath, speedContainer, dm.config.Headers)
		if downloadResult == nil || !downloadResult.Success {
			result.Error = fmt.Errorf("第一个分片下载失败")
			return result
		}
		task.Increment(1)
		dm.mu.Lock()
		dm.fileDictionaries[stream][int(firstSegment.Index)] = downloadResult.FilePath
		dm.mu.Unlock()

		if stream.Playlist.MediaInit == nil {
			mp4Info, _ := util.GetMP4Info(downloadResult.FilePath)
			currentKID = mp4Info.KID
			if key, _ := util.SearchKeyFromFile(dm.config.KeyTextFile, currentKID); key != "" {
				dm.config.Keys = append(dm.config.Keys, key)
			}
			if dm.config.MP4RealTimeDecryption && currentKID != "" && len(dm.config.Keys) > 0 {
				decPath := strings.Replace(downloadResult.FilePath, ".tmp", "_dec.tmp", 1)
				if success, _ := util.Decrypt(dm.config.DecryptionEngine, dm.config.DecryptionBinaryPath, dm.config.Keys, downloadResult.FilePath, decPath, currentKID, false); success {
					downloadResult.FilePath = decPath
					dm.mu.Lock()
					dm.fileDictionaries[stream][int(firstSegment.Index)] = decPath
					dm.mu.Unlock()
				}
			}
			if !readInfo {
				util.Logger.WarnMarkUp("读取媒体信息...")
				infos, err := util.GetMediaInfo(dm.config.FFmpegPath, downloadResult.FilePath)
				if err == nil {
					result.Mediainfos = infos
					dm.changeSpecInfo(stream, infos)
					// C# Logic: if it's audio only, disable binary merge
					if stream.MediaType != nil && *stream.MediaType == entity.MediaTypeAudio {
						dm.config.BinaryMerge = false
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
			firstSegmentBytes, err := os.ReadFile(downloadResult.FilePath)
			if err != nil {
				result.Error = fmt.Errorf("读取第一个分片失败: %w", err)
				return result
			}
			header, err := processor.GenHeader(firstSegmentBytes)
			if err != nil {
				result.Error = fmt.Errorf("生成MSS头部失败: %w", err)
				return result
			}
			initFilePath := dm.fileDictionaries[stream][-1]
			if err := os.WriteFile(initFilePath, header, 0644); err != nil {
				result.Error = fmt.Errorf("写入MSS头部失败: %w", err)
				return result
			}
			util.Logger.Info("MSS init box生成并写入成功")
		}
	}

	dm.mu.Lock()
	dm.streamKIDs[stream] = currentKID
	dm.mu.Unlock()

	if success := dm.downloadSegments(segments, streamDir, task, speedContainer, stream, currentKID); !success {
		result.Error = fmt.Errorf("分段下载失败")
		return result
	}

	result.Success = true
	return result
}

func (dm *DownloadManager) downloadSegments(segments []*entity.MediaSegment, outputDir string, task *util.ProgressTask, speedContainer *util.SpeedContainer, stream *entity.StreamSpec, currentKID string) bool {
	padLength := len(fmt.Sprintf("%d", len(segments)))
	if dm.config.ThreadCount <= 1 {
		for _, segment := range segments {
			ext := "ts"
			if stream.Extension != "" {
				ext = stream.Extension
			}
			fileName := fmt.Sprintf("%0*d.%s.tmp", padLength, segment.Index, ext)
			segmentPath := filepath.Join(outputDir, fileName)
			result := dm.downloader.DownloadSegment(segment, segmentPath, speedContainer, dm.config.Headers)
			if result != nil && result.Success {
				if dm.config.MP4RealTimeDecryption && currentKID != "" && len(dm.config.Keys) > 0 {
					decPath := strings.Replace(result.FilePath, ".tmp", "_dec.tmp", 1)
					if success, _ := util.Decrypt(dm.config.DecryptionEngine, dm.config.DecryptionBinaryPath, dm.config.Keys, result.FilePath, decPath, currentKID, false); success {
						result.FilePath = decPath
					}
				}
				dm.mu.Lock()
				dm.fileDictionaries[stream][int(segment.Index)] = result.FilePath
				dm.mu.Unlock()
				task.Increment(1)
			} else {
				if dm.config.CheckLength {
					return false
				}
			}
		}
	} else {
		return dm.downloadSegmentsConcurrently(segments, outputDir, task, speedContainer, padLength, stream, currentKID)
	}
	return true
}

func (dm *DownloadManager) downloadSegmentsConcurrently(segments []*entity.MediaSegment, outputDir string, task *util.ProgressTask, speedContainer *util.SpeedContainer, padLength int, stream *entity.StreamSpec, currentKID string) bool {
	maxWorkers := dm.config.ThreadCount
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
				result := dm.downloader.DownloadSegment(item.segment, segmentPath, speedContainer, dm.config.Headers)
				if result != nil && result.Success {
					if dm.config.MP4RealTimeDecryption && currentKID != "" && len(dm.config.Keys) > 0 {
						decPath := strings.Replace(result.FilePath, ".tmp", "_dec.tmp", 1)
						if success, _ := util.Decrypt(dm.config.DecryptionEngine, dm.config.DecryptionBinaryPath, dm.config.Keys, result.FilePath, decPath, currentKID, false); success {
							result.FilePath = decPath
						}
					}
					dm.mu.Lock()
					dm.fileDictionaries[stream][item.index] = result.FilePath
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

	return dm.ffmpegMergeFiles(inputDir, *outputPath, stream, inputDir) // 工作目录就是输入目录
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

func (dm *DownloadManager) mergeStreamInBackground(stream *entity.StreamSpec, result *DownloadStreamResult, task *util.ProgressTask) {
	defer dm.mergeWaitGroup.Done()

	if err := dm.postProcessStreamData(stream, result); err != nil {
		util.Logger.Error("数据后处理失败: %v", err)
		dm.mu.Lock()
		dm.validationFailed = true
		dm.mu.Unlock()
		return
	}

	outputPath := dm.getOutputPath(stream, task.ID)
	if dm.mergeSegments(result.StreamDir, &outputPath, stream) {
		dm.mu.RLock()
		currentKID := dm.streamKIDs[stream]
		dm.mu.RUnlock()

		if !dm.config.MP4RealTimeDecryption && currentKID != "" && len(dm.config.Keys) > 0 {
			util.Logger.Info("正在解密合并后的文件...")
			decPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + "_dec" + filepath.Ext(outputPath)
			if success, _ := util.Decrypt(dm.config.DecryptionEngine, dm.config.DecryptionBinaryPath, dm.config.Keys, outputPath, decPath, currentKID, false); success {
				os.Remove(outputPath)
				os.Rename(decPath, outputPath)
			}
		}

		dm.mu.Lock()
		defer dm.mu.Unlock()

		isAlreadyAdded := false
		for _, f := range dm.outputFiles {
			if f.Index == task.ID {
				f.FilePath = outputPath
				isAlreadyAdded = true
				break
			}
		}

		if !isAlreadyAdded {
			dm.outputFiles = append(dm.outputFiles, &OutputFile{
				Index:       task.ID,
				FilePath:    outputPath,
				LangCode:    stream.Language,
				Description: stream.Name,
				MediaType:   *stream.MediaType,
				Mediainfos:  result.Mediainfos,
			})
		}
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
			// Sort indices to process files in order
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
		util.Logger.Warn("没有找到视频文件，无法执行混流操作")
		return true
	}

	allMuxSuccess := true
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

		util.Logger.Info("准备混流以下文件:")
		for _, f := range inputs {
			util.Logger.WarnMarkUp("[grey]%s[/]", filepath.Base(f.FilePath))
		}

		baseName := strings.TrimSuffix(videoFile.FilePath, filepath.Ext(videoFile.FilePath))
		outputPath := fmt.Sprintf("%s.MUX", baseName)
		finalMuxPath := outputPath + dm.getMuxExtension()
		util.Logger.InfoMarkUp("Muxing to [grey]%s[/]", filepath.Base(finalMuxPath))

		var currentMuxSuccess bool
		// 对于轨道混流，所有输入都是绝对路径，工作目录可以是当前目录
		workingDir, _ := os.Getwd()

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

		if currentMuxSuccess {
			util.Logger.InfoMarkUp("[white on green]混流完成[/]: %s", finalMuxPath)
		} else {
			util.Logger.ErrorMarkUp("[white on red]混流失败[/]: %s", finalMuxPath)
			allMuxSuccess = false
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
	return util.GetMuxExtension(string(dm.config.MuxOptions.MuxFormat))
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
