package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"N_m3u8DL-RE-GO/internal/downloader"
	"N_m3u8DL-RE-GO/internal/entity"
	"N_m3u8DL-RE-GO/internal/parser"
	"N_m3u8DL-RE-GO/internal/util"

	"github.com/spf13/cobra"
)

const VERSION_INFO = "N-M3U8DL-RE-GO (Beta version) 20250615"

var rootCmd = &cobra.Command{
	Use:   "N-M3U8DL-RE-GO",
	Short: "M3U8/MPD下载工具",
	Long: `N_m3u8DL-RE的Golang重写版本
一个用于下载M3U8/MPD流媒体的命令行工具`,
	Version: VERSION_INFO,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("请提供M3U8/MPD URL")
			cmd.Help()
			return
		}

		err := runDownload(cmd, args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "下载失败: %s\n", err.Error())
			os.Exit(1)
		}
	},
}

// runDownload 执行下载
func runDownload(cmd *cobra.Command, url string) error {
	// 获取基础参数
	outputDir, _ := cmd.Flags().GetString("output")
	saveName, _ := cmd.Flags().GetString("save-name")
	saveDir, _ := cmd.Flags().GetString("save-dir")
	tmpDir, _ := cmd.Flags().GetString("tmp-dir")
	threadCount, _ := cmd.Flags().GetInt("thread-count")
	headerFlags, _ := cmd.Flags().GetStringSlice("header")
	retryCount, _ := cmd.Flags().GetInt("retry-count")
	downloadRetryCount, _ := cmd.Flags().GetInt("download-retry-count")
	webRequestRetryCount, _ := cmd.Flags().GetInt("web-request-retry-count")
	httpRequestTimeout, _ := cmd.Flags().GetFloat64("http-request-timeout")

	// 流选择参数 - 支持简写形式
	videoSelect, _ := cmd.Flags().GetString("select-video")
	if sv, _ := cmd.Flags().GetString("sv"); sv != "best" {
		videoSelect = sv
	}
	audioSelect, _ := cmd.Flags().GetString("select-audio")
	if sa, _ := cmd.Flags().GetString("sa"); sa != "best" {
		audioSelect = sa
	}
	subtitleSelect, _ := cmd.Flags().GetString("select-subtitle")
	if ss, _ := cmd.Flags().GetString("ss"); ss != "all" {
		subtitleSelect = ss
	}
	dropVideo, _ := cmd.Flags().GetString("drop-video")
	if dv, _ := cmd.Flags().GetString("dv"); dv != "" {
		dropVideo = dv
	}
	dropAudio, _ := cmd.Flags().GetString("drop-audio")
	if da, _ := cmd.Flags().GetString("da"); da != "" {
		dropAudio = da
	}
	dropSubtitle, _ := cmd.Flags().GetString("drop-subtitle")
	if ds, _ := cmd.Flags().GetString("ds"); ds != "" {
		dropSubtitle = ds
	}

	// 日志和界面参数
	logLevel, _ := cmd.Flags().GetString("log-level")
	logFilePath, _ := cmd.Flags().GetString("log-file-path")
	uiLanguage, _ := cmd.Flags().GetString("ui-language")
	forceAnsiConsole, _ := cmd.Flags().GetBool("force-ansi-console")
	noAnsiColor, _ := cmd.Flags().GetBool("no-ansi-color")
	noLog, _ := cmd.Flags().GetBool("no-log")

	// 下载控制参数
	autoSelect, _ := cmd.Flags().GetBool("auto-select")
	subOnly, _ := cmd.Flags().GetBool("sub-only")
	skipMerge, _ := cmd.Flags().GetBool("skip-merge")
	skipDownload, _ := cmd.Flags().GetBool("skip-download")
	binaryMerge, _ := cmd.Flags().GetBool("binary-merge")
	useFFmpegConcatDemuxer, _ := cmd.Flags().GetBool("use-ffmpeg-concat-demuxer")
	deleteAfterDone, _ := cmd.Flags().GetBool("delete-after-done")
	checkSegmentsCount, _ := cmd.Flags().GetBool("check-segments-count")
	writeMetaJson, _ := cmd.Flags().GetBool("write-meta-json")
	appendUrlParams, _ := cmd.Flags().GetBool("append-url-params")
	concurrentDownload, _ := cmd.Flags().GetBool("concurrent-download")
	maxSpeed, _ := cmd.Flags().GetString("max-speed")

	// 网络设置参数
	baseUrl, _ := cmd.Flags().GetString("base-url")
	userAgent, _ := cmd.Flags().GetString("user-agent")
	customProxy, _ := cmd.Flags().GetString("custom-proxy")
	useSystemProxy, _ := cmd.Flags().GetBool("use-system-proxy")
	customRange, _ := cmd.Flags().GetString("custom-range")
	adKeywords, _ := cmd.Flags().GetStringSlice("ad-keyword")

	// 加密解密参数
	decryptEngine, _ := cmd.Flags().GetString("decrypt-engine")
	keys, _ := cmd.Flags().GetStringSlice("key")
	keyTextFile, _ := cmd.Flags().GetString("key-text-file")
	mp4RealTimeDecryption, _ := cmd.Flags().GetBool("mp4-real-time-decryption")
	useShakaPackager, _ := cmd.Flags().GetBool("use-shaka-packager")
	customHlsMethod, _ := cmd.Flags().GetString("custom-hls-method")
	customHlsKey, _ := cmd.Flags().GetString("custom-hls-key")
	customHlsIv, _ := cmd.Flags().GetString("custom-hls-iv")

	// 复用和后处理参数
	muxFormat, _ := cmd.Flags().GetString("mux-format")
	muxAfterDone, _ := cmd.Flags().GetString("mux-after-done")
	muxImports, _ := cmd.Flags().GetStringSlice("mux-import")
	subtitleFormat, _ := cmd.Flags().GetString("subtitle-format")
	autoSubtitleFix, _ := cmd.Flags().GetBool("auto-subtitle-fix")

	// 直播相关参数
	livePerformAsVod, _ := cmd.Flags().GetBool("live-perform-as-vod")
	liveRealTimeMerge, _ := cmd.Flags().GetBool("live-real-time-merge")
	liveKeepSegments, _ := cmd.Flags().GetBool("live-keep-segments")
	livePipeMux, _ := cmd.Flags().GetBool("live-pipe-mux")
	liveRecordLimit, _ := cmd.Flags().GetString("live-record-limit")
	liveWaitTime, _ := cmd.Flags().GetString("live-wait-time")
	liveTakeCount, _ := cmd.Flags().GetInt("live-take-count")
	liveFixVttByAudio, _ := cmd.Flags().GetBool("live-fix-vtt-by-audio")

	// 高级设置参数
	taskStartAt, _ := cmd.Flags().GetString("task-start-at")
	urlProcessor, _ := cmd.Flags().GetStringSlice("url-processor")
	urlProcessorArgs, _ := cmd.Flags().GetString("url-processor-args")
	ffmpegBinaryPath, _ := cmd.Flags().GetString("ffmpeg-binary-path")
	mp4decryptBinaryPath, _ := cmd.Flags().GetString("mp4decrypt-binary-path")
	decryptionBinaryPath, _ := cmd.Flags().GetString("decryption-binary-path")
	disableUpdateCheck, _ := cmd.Flags().GetBool("disable-update-check")
	allowHlsMultiExtMap, _ := cmd.Flags().GetBool("allow-hls-multi-ext-map")

	// 忽略未使用的变量警告
	_ = saveDir
	_ = tmpDir
	_ = downloadRetryCount
	_ = webRequestRetryCount
	_ = httpRequestTimeout
	_ = dropVideo
	_ = dropAudio
	_ = dropSubtitle
	_ = logFilePath
	_ = uiLanguage
	_ = forceAnsiConsole
	_ = noAnsiColor
	_ = noLog
	_ = subOnly
	_ = skipMerge
	_ = useFFmpegConcatDemuxer
	_ = deleteAfterDone
	_ = checkSegmentsCount
	_ = appendUrlParams
	_ = baseUrl
	_ = userAgent
	_ = useSystemProxy
	_ = customRange
	_ = adKeywords
	_ = decryptEngine
	_ = keys
	_ = keyTextFile
	_ = mp4RealTimeDecryption
	_ = useShakaPackager
	_ = customHlsMethod
	_ = customHlsKey
	_ = customHlsIv
	// 解析混流参数
	var muxOptions *entity.MuxOptions
	if muxAfterDone != "" {
		var err error
		muxOptions, err = parseMuxAfterDone(muxAfterDone)
		if err != nil {
			return fmt.Errorf("解析混流参数失败: %w", err)
		}

		// 显示警告信息：你已开启下载完成后混流，自动开启二进制合并
		util.Logger.Warn("你已开启下载完成后混流，自动开启二进制合并")
		binaryMerge = true

		// 解析 muxImports
		if len(muxImports) > 0 {
			muxOptions.MuxImports = make([]*entity.OutputFile, 0, len(muxImports))
			for i, importStr := range muxImports {
				parser := util.NewComplexParamParser(importStr)
				filePath := parser.GetValue("path")
				if filePath == "" {
					// 如果没有path，则认为整个字符串是路径
					filePath = importStr
				}
				outputFile := &entity.OutputFile{
					Index:       i, // 使用索引作为默认值
					FilePath:    filePath,
					LangCode:    parser.GetValue("lang"),
					Description: parser.GetValue("desc"),
					Type:        parser.GetValue("type"), // 解析type字段
				}
				// 尝试从path中解析index
				if parser.HasKey("index") {
					if idx, err := util.StrToInt(parser.GetValue("index")); err == nil {
						outputFile.Index = idx
					}
				}
				muxOptions.MuxImports = append(muxOptions.MuxImports, outputFile)
			}
		}
	}

	_ = subtitleFormat
	_ = autoSubtitleFix
	_ = livePerformAsVod
	_ = liveRealTimeMerge
	_ = liveKeepSegments
	_ = livePipeMux
	_ = liveRecordLimit
	_ = liveWaitTime
	_ = liveTakeCount
	_ = liveFixVttByAudio
	_ = taskStartAt
	_ = urlProcessor
	_ = urlProcessorArgs
	_ = ffmpegBinaryPath
	_ = mp4decryptBinaryPath
	_ = decryptionBinaryPath
	_ = disableUpdateCheck
	_ = allowHlsMultiExtMap
	_ = maxSpeed
	_ = concurrentDownload

	// 设置日志级别
	switch strings.ToUpper(logLevel) {
	case "DEBUG":
		util.SetLogLevel(util.LogLevelDebug)
	case "INFO":
		util.SetLogLevel(util.LogLevelInfo)
	case "WARN":
		util.SetLogLevel(util.LogLevelWarn)
	case "ERROR":
		util.SetLogLevel(util.LogLevelError)
	}

	// 解析headers
	headers := make(map[string]string)
	for _, header := range headerFlags {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) == 2 {
			headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	// 设置HTTP代理
	if customProxy != "" {
		util.SetHTTPProxy(customProxy)
		util.Logger.Info(fmt.Sprintf("已设置HTTP代理: %s", customProxy))
	}

	util.Logger.Info(fmt.Sprintf("开始分析: %s", url))

	// 创建流提取器
	extractor := parser.NewStreamExtractor()

	// 提取流信息
	streams, err := extractor.ExtractStreams(url, headers)
	if err != nil {
		return fmt.Errorf("提取流信息失败: %w", err)
	}

	if len(streams) == 0 {
		return fmt.Errorf("没有找到可用的流")
	}

	util.Logger.Info(fmt.Sprintf("解析完成，找到 %d 个流", len(streams)))

	// 对流进行排序（按照C#版本的逻辑）
	streams = util.SortStreams(streams)

	// 显示所有流信息
	util.Logger.Info("流信息:")
	for _, stream := range streams {
		util.Logger.InfoMarkUp(stream.ToString())
	}

	// 这个变量已经在上面声明了，不需要再次声明

	var filteredStreams []*entity.StreamSpec

	// 判断是否有明确的选择条件
	hasSelectConditions := videoSelect != "best" || audioSelect != "best" || subtitleSelect != "all"

	if autoSelect {
		// 自动选择模式：选择最佳视频+所有音频+所有字幕
		filteredStreams = autoSelectStreams(streams)
		util.Logger.Info("自动选择模式已启用")
	} else if hasSelectConditions {
		// 有明确的过滤条件，直接过滤
		filteredStreams = extractor.FilterStreams(streams, videoSelect, audioSelect, subtitleSelect)
	} else {
		// 没有明确的过滤条件，尝试交互式选择
		if len(streams) == 1 {
			// 只有一个流，直接选择
			filteredStreams = streams
		} else {
			// 多个流，需要用户选择
			util.Logger.Info("\n请选择要下载的流:")
			filteredStreams = selectStreamsInteractive(streams)
		}
	}

	if len(filteredStreams) == 0 {
		return fmt.Errorf("没有选择任何流进行下载")
	}

	// 获取播放列表并更新扩展名
	util.Logger.Info("正在获取播放列表...")
	if err := extractor.FetchPlayList(filteredStreams, headers); err != nil {
		util.Logger.Warn(fmt.Sprintf("获取播放列表时出现警告: %v", err))
	}

	util.Logger.Info(fmt.Sprintf("选择了 %d 个流进行下载", len(filteredStreams)))
	util.Logger.Info("已选择的流:")
	for _, stream := range filteredStreams {
		util.Logger.InfoMarkUp(stream.ToString())
	}

	// 生成保存名称
	if saveName == "" {
		saveName = fmt.Sprintf("download_%s", time.Now().Format("2006-01-02_15-04-05"))
	}

	// 设置输出目录
	if outputDir == "" {
		outputDir = "Downloads"
	}

	// 创建临时目录
	tmpDir = filepath.Join(outputDir, saveName)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 写出meta.json和meta_selected.json文件
	if writeMetaJson {
		if err := writeMetaJsonFiles(tmpDir, streams, filteredStreams); err != nil {
			util.Logger.Warn(fmt.Sprintf("写入meta.json失败: %s", err.Error()))
		} else {
			util.Logger.Warn("已写入meta.json和meta_selected.json文件")
		}
	}

	// 如果只是跳过下载，直接返回
	if skipDownload {
		util.Logger.Info("跳过下载，任务完成")
		return nil
	}

	// 获取FFmpeg相关参数
	ffmpegPath := ffmpegBinaryPath
	// muxFormat 和 binaryMerge 已经在上面声明了

	// 如果没有指定FFmpeg路径，自动查找可执行文件
	if ffmpegPath == "" {
		ffmpegPath = util.FindExecutable("ffmpeg")
		util.Logger.Debug(fmt.Sprintf("自动查找到FFmpeg路径: %s", ffmpegPath))
	}

	// 创建下载管理器配置
	managerConfig := &downloader.ManagerConfig{
		OutputDir:              outputDir,
		SaveName:               saveName,
		SaveDir:                outputDir,
		TmpDir:                 tmpDir, // 使用正确的临时目录
		ThreadCount:            threadCount,
		RetryCount:             retryCount,
		Headers:                headers,
		CheckLength:            true,
		DeleteAfterDone:        true,
		BinaryMerge:            binaryMerge,
		SkipMerge:              false,
		ConcurrentDownload:     threadCount > 1,
		NoAnsiColor:            false,
		LogLevel:               util.Logger.Level,
		FFmpegPath:             ffmpegPath,                 // FFmpeg可执行文件路径
		MuxFormat:              strings.ToUpper(muxFormat), // 默认值，会被MuxOptions覆盖
		UseAACFilter:           true,                       // 默认使用AAC过滤器
		MuxAfterDone:           muxOptions != nil,          // 是否开启混流
		MuxOptions:             muxOptions,                 // 混流选项
		UseFFmpegConcatDemuxer: useFFmpegConcatDemuxer,
	}

	// 如果通过 -M 参数设置了muxOptions，则使用其中的MuxFormat
	if muxOptions != nil && muxOptions.MuxFormat.String() != "" {
		managerConfig.MuxFormat = muxOptions.MuxFormat.String()
	}

	// 创建下载管理器
	downloadManager := downloader.NewDownloadManager(managerConfig, filteredStreams)

	// 开始下载
	err = downloadManager.StartDownloadAsync()
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}

	util.Logger.Info("所有下载任务完成")
	return nil
}

func init() {
	// 基础标志
	rootCmd.PersistentFlags().StringP("output", "o", "", "输出目录")
	rootCmd.PersistentFlags().StringP("save-name", "n", "", "保存文件名")
	rootCmd.PersistentFlags().IntP("thread-count", "t", 8, "下载线程数")
	rootCmd.PersistentFlags().StringSliceP("header", "H", []string{}, "自定义HTTP请求头")
	rootCmd.PersistentFlags().Int("retry-count", 3, "重试次数")
	rootCmd.PersistentFlags().Int("download-retry-count", 3, "下载重试次数")
	rootCmd.PersistentFlags().Int("web-request-retry-count", 10, "网络请求重试次数")
	rootCmd.PersistentFlags().Float64("http-request-timeout", 100, "HTTP请求超时时间(秒)")

	// 输出控制
	rootCmd.PersistentFlags().String("tmp-dir", "", "临时目录")
	rootCmd.PersistentFlags().String("save-dir", "", "保存目录")
	rootCmd.PersistentFlags().String("save-pattern", "", "保存模式")
	rootCmd.PersistentFlags().Bool("no-date-info", false, "不显示日期信息")

	// 日志和界面
	rootCmd.PersistentFlags().String("log-level", "INFO", "日志级别")
	rootCmd.PersistentFlags().String("log-file-path", "", "日志文件路径")
	rootCmd.PersistentFlags().Bool("force-ansi-console", false, "强制ANSI控制台")
	rootCmd.PersistentFlags().Bool("no-ansi-color", false, "禁用ANSI颜色")
	rootCmd.PersistentFlags().Bool("no-log", false, "禁用日志")
	rootCmd.PersistentFlags().String("ui-language", "", "界面语言")

	// 下载控制
	rootCmd.PersistentFlags().Bool("auto-select", false, "自动选择")
	rootCmd.PersistentFlags().Bool("sub-only", false, "仅下载字幕")
	rootCmd.PersistentFlags().Bool("skip-merge", false, "跳过合并")
	rootCmd.PersistentFlags().Bool("skip-download", false, "跳过下载")
	rootCmd.PersistentFlags().Bool("binary-merge", false, "二进制合并")
	rootCmd.PersistentFlags().Bool("use-ffmpeg-concat-demuxer", false, "使用FFmpeg concat demuxer")
	rootCmd.PersistentFlags().Bool("delete-after-done", true, "完成后删除临时文件")
	rootCmd.PersistentFlags().Bool("check-segments-count", true, "检查分段数量")
	rootCmd.PersistentFlags().Bool("write-meta-json", true, "写出meta.json文件")
	rootCmd.PersistentFlags().Bool("append-url-params", false, "附加URL参数")
	rootCmd.PersistentFlags().Bool("concurrent-download", false, "并发下载")
	rootCmd.PersistentFlags().String("max-speed", "", "最大下载速度")

	// 流选择
	rootCmd.PersistentFlags().StringP("select-video", "v", "best", "视频流选择")
	rootCmd.PersistentFlags().String("sv", "best", "视频流选择（简写）")
	rootCmd.PersistentFlags().StringP("select-audio", "a", "best", "音频流选择")
	rootCmd.PersistentFlags().String("sa", "best", "音频流选择（简写）")
	rootCmd.PersistentFlags().StringP("select-subtitle", "s", "all", "字幕流选择")
	rootCmd.PersistentFlags().String("ss", "all", "字幕流选择（简写）")
	rootCmd.PersistentFlags().String("drop-video", "", "排除视频轨道")
	rootCmd.PersistentFlags().String("dv", "", "排除视频轨道（简写）")
	rootCmd.PersistentFlags().String("drop-audio", "", "排除音频轨道")
	rootCmd.PersistentFlags().String("da", "", "排除音频轨道（简写）")
	rootCmd.PersistentFlags().String("drop-subtitle", "", "排除字幕轨道")
	rootCmd.PersistentFlags().String("ds", "", "排除字幕轨道（简写）")

	// 加密和解密
	rootCmd.PersistentFlags().String("decrypt-engine", "MP4DECRYPT", "解密引擎")
	rootCmd.PersistentFlags().StringSlice("key", []string{}, "解密密钥")
	rootCmd.PersistentFlags().String("key-text-file", "", "密钥文本文件")
	rootCmd.PersistentFlags().Bool("mp4-real-time-decryption", false, "MP4实时解密")
	rootCmd.PersistentFlags().Bool("use-shaka-packager", false, "使用Shaka Packager")
	rootCmd.PersistentFlags().String("custom-hls-method", "", "自定义HLS加密方法")
	rootCmd.PersistentFlags().String("custom-hls-key", "", "自定义HLS密钥")
	rootCmd.PersistentFlags().String("custom-hls-iv", "", "自定义HLS初始向量")

	// 复用和后处理
	rootCmd.PersistentFlags().String("mux-format", "mp4", "复用格式")
	rootCmd.PersistentFlags().StringP("mux-after-done", "M", "", "完成后复用参数")
	rootCmd.PersistentFlags().StringSlice("mux-import", []string{}, "复用导入文件")
	rootCmd.PersistentFlags().String("subtitle-format", "SRT", "字幕格式")
	rootCmd.PersistentFlags().Bool("auto-subtitle-fix", true, "自动修复字幕")

	// 网络设置
	rootCmd.PersistentFlags().String("base-url", "", "基础URL")
	rootCmd.PersistentFlags().String("user-agent", "", "自定义User-Agent")
	rootCmd.PersistentFlags().String("custom-proxy", "", "自定义代理")
	rootCmd.PersistentFlags().Bool("use-system-proxy", true, "使用系统代理")
	rootCmd.PersistentFlags().String("custom-range", "", "自定义范围")
	rootCmd.PersistentFlags().StringSlice("ad-keyword", []string{}, "广告关键词过滤")

	// 直播相关
	rootCmd.PersistentFlags().Bool("live-perform-as-vod", false, "直播当作点播处理")
	rootCmd.PersistentFlags().Bool("live-real-time-merge", false, "直播实时合并")
	rootCmd.PersistentFlags().Bool("live-keep-segments", true, "直播保留分片")
	rootCmd.PersistentFlags().Bool("live-pipe-mux", false, "直播管道复用")
	rootCmd.PersistentFlags().String("live-record-limit", "", "直播录制时长限制")
	rootCmd.PersistentFlags().String("live-wait-time", "", "直播等待时间")
	rootCmd.PersistentFlags().Int("live-take-count", 16, "直播分片获取数量")
	rootCmd.PersistentFlags().Bool("live-fix-vtt-by-audio", false, "通过音频修复直播VTT")

	// 高级设置
	rootCmd.PersistentFlags().String("task-start-at", "", "任务开始时间")
	rootCmd.PersistentFlags().StringSlice("url-processor", []string{}, "URL处理器")
	rootCmd.PersistentFlags().String("url-processor-args", "", "URL处理器参数")
	rootCmd.PersistentFlags().String("ffmpeg-binary-path", "", "FFmpeg二进制路径")
	rootCmd.PersistentFlags().String("mp4decrypt-binary-path", "", "mp4decrypt二进制路径")
	rootCmd.PersistentFlags().String("decryption-binary-path", "", "解密二进制路径")
	rootCmd.PersistentFlags().Bool("disable-update-check", false, "禁用更新检查")
	rootCmd.PersistentFlags().Bool("allow-hls-multi-ext-map", false, "允许HLS多扩展映射")
	rootCmd.PersistentFlags().Bool("more-output", false, "更多输出信息")
}

// writeMetaJsonFiles 写出meta.json和meta_selected.json文件
func writeMetaJsonFiles(tmpDir string, allStreams, selectedStreams []*entity.StreamSpec) error {
	// 写出meta.json (所有流)
	metaJsonPath := filepath.Join(tmpDir, "meta.json")
	allStreamsJson, err := json.MarshalIndent(allStreams, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化所有流失败: %w", err)
	}
	if err := os.WriteFile(metaJsonPath, allStreamsJson, 0644); err != nil {
		return fmt.Errorf("写入meta.json失败: %w", err)
	}

	// 写出meta_selected.json (选中的流)
	metaSelectedJsonPath := filepath.Join(tmpDir, "meta_selected.json")
	selectedStreamsJson, err := json.MarshalIndent(selectedStreams, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化选中流失败: %w", err)
	}
	if err := os.WriteFile(metaSelectedJsonPath, selectedStreamsJson, 0644); err != nil {
		return fmt.Errorf("写入meta_selected.json失败: %w", err)
	}

	return nil
}

// autoSelectStreams 自动选择流
func autoSelectStreams(streams []*entity.StreamSpec) []*entity.StreamSpec {
	var selected []*entity.StreamSpec

	// 分类流
	var basicStreams, audioStreams, subtitleStreams []*entity.StreamSpec

	for _, stream := range streams {
		if stream.MediaType == nil || *stream.MediaType == entity.MediaTypeVideo {
			basicStreams = append(basicStreams, stream)
		} else if *stream.MediaType == entity.MediaTypeAudio {
			audioStreams = append(audioStreams, stream)
		} else if *stream.MediaType == entity.MediaTypeSubtitles {
			subtitleStreams = append(subtitleStreams, stream)
		}
	}

	// 选择最佳视频流（第一个）
	if len(basicStreams) > 0 {
		selected = append(selected, basicStreams[0])
	}

	// 选择所有音频流
	selected = append(selected, audioStreams...)

	// 选择所有字幕流
	selected = append(selected, subtitleStreams...)

	return selected
}

// selectStreamsInteractive 交互式选择流
func selectStreamsInteractive(streams []*entity.StreamSpec) []*entity.StreamSpec {
	// 使用交互式选择器，类似C#版本
	return util.SelectStreamsInteractive(streams)
}

// parseMuxAfterDone 解析混流参数
func parseMuxAfterDone(input string) (*entity.MuxOptions, error) {
	parser := util.NewComplexParamParser(input)

	// 解析混流格式
	format := parser.GetValue("format")
	if format == "" {
		// 如果没有指定format，使用冒号前的字符串作为format
		parts := strings.Split(input, ":")
		if len(parts) > 0 {
			format = parts[0]
		}
	}
	if format == "" {
		format = "mp4" // 默认格式
	}

	var muxFormat entity.MuxFormat
	switch strings.ToUpper(format) {
	case "MP4":
		muxFormat = entity.MuxFormatMP4
	case "MKV":
		muxFormat = entity.MuxFormatMKV
	case "TS":
		muxFormat = entity.MuxFormatTS
	case "MOV":
		muxFormat = entity.MuxFormatMOV
	case "FLV":
		muxFormat = entity.MuxFormatFLV
	default:
		return nil, fmt.Errorf("不支持的混流格式: %s", format)
	}

	// 解析混流器
	muxer := parser.GetValue("muxer")
	if muxer == "" {
		muxer = "ffmpeg" // 默认使用ffmpeg
	}
	useMkvmerge := false
	if muxer == "mkvmerge" {
		useMkvmerge = true
		// mkvmerge不能输出mp4格式
		if muxFormat == entity.MuxFormatMP4 {
			return nil, fmt.Errorf("mkvmerge不支持mp4格式")
		}
	} else if muxer != "ffmpeg" {
		return nil, fmt.Errorf("不支持的混流器: %s", muxer)
	}

	// 解析二进制路径
	binPath := parser.GetValue("bin_path")
	if binPath == "auto" || binPath == "" {
		binPath = ""
	}

	// 解析是否保留文件
	keep := parser.GetValue("keep")
	keepFiles := keep == "true"

	// 解析是否跳过字幕
	skipSub := parser.GetValue("skip_sub")
	skipSubtitle := skipSub == "true"

	return &entity.MuxOptions{
		UseMkvmerge:  useMkvmerge,
		MuxFormat:    muxFormat,
		KeepFiles:    keepFiles,
		SkipSubtitle: skipSubtitle,
		BinPath:      binPath,
		MuxerPath:    binPath, // MuxerPath is an alias for BinPath for now
	}, nil
}

// Execute 执行根命令
func Execute() error {
	return rootCmd.Execute()
}
