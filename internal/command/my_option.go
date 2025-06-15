package command

import (
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	"N_m3u8DL-RE-GO/internal/entity"
	"N_m3u8DL-RE-GO/internal/util"
)

// MyOption 命令行选项结构体
type MyOption struct {
	// 基础输入
	Input string `json:"input"`

	// HTTP相关
	Headers   map[string]string `json:"headers,omitempty"`
	BaseURL   *string           `json:"base_url,omitempty"`
	MaxSpeed  *int64            `json:"max_speed,omitempty"`
	UserAgent *string           `json:"user_agent,omitempty"`

	// 网络和代理
	UseSystemProxy bool     `json:"use_system_proxy"`
	CustomProxy    *url.URL `json:"custom_proxy,omitempty"`

	// 密钥相关
	Keys            []string              `json:"keys,omitempty"`
	KeyTextFile     *string               `json:"key_text_file,omitempty"`
	CustomHLSMethod *entity.EncryptMethod `json:"custom_hls_method,omitempty"`
	CustomHLSKey    []byte                `json:"custom_hls_key,omitempty"`
	CustomHLSIV     []byte                `json:"custom_hls_iv,omitempty"`

	// 日志和调试
	LogLevel         entity.LogLevel `json:"log_level"`
	LogFilePath      *string         `json:"log_file_path,omitempty"`
	NoDateInfo       bool            `json:"no_date_info"`
	NoLog            bool            `json:"no_log"`
	ForceAnsiConsole bool            `json:"force_ansi_console"`
	NoAnsiColor      bool            `json:"no_ansi_color"`

	// 界面和语言
	UILanguage *string `json:"ui_language,omitempty"`

	// 下载控制
	ThreadCount          int     `json:"thread_count"`
	DownloadRetryCount   int     `json:"download_retry_count"`
	WebRequestRetryCount int     `json:"web_request_retry_count"`
	HTTPRequestTimeout   float64 `json:"http_request_timeout"`
	AutoSelect           bool    `json:"auto_select"`
	DisableUpdateCheck   bool    `json:"disable_update_check"`
	ConcurrentDownload   bool    `json:"concurrent_download"`

	// 文件路径和目录
	TmpDir      *string `json:"tmp_dir,omitempty"`
	SaveDir     *string `json:"save_dir,omitempty"`
	SaveName    *string `json:"save_name,omitempty"`
	SavePattern *string `json:"save_pattern,omitempty"`

	// 下载后处理
	SkipMerge              bool `json:"skip_merge"`
	SkipDownload           bool `json:"skip_download"`
	BinaryMerge            bool `json:"binary_merge"`
	UseFFmpegConcatDemuxer bool `json:"use_ffmpeg_concat_demuxer"`
	DelAfterDone           bool `json:"del_after_done"`
	CheckSegmentsCount     bool `json:"check_segments_count"`
	WriteMetaJSON          bool `json:"write_meta_json"`
	AppendURLParams        bool `json:"append_url_params"`

	// 字幕相关
	SubOnly         bool                  `json:"sub_only"`
	SubtitleFormat  entity.SubtitleFormat `json:"subtitle_format"`
	AutoSubtitleFix bool                  `json:"auto_subtitle_fix"`

	// 解密相关
	MP4RealTimeDecryption bool                 `json:"mp4_real_time_decryption"`
	DecryptionEngine      entity.DecryptEngine `json:"decryption_engine"`
	DecryptionBinaryPath  *string              `json:"decryption_binary_path,omitempty"`

	// 二进制程序路径
	FFmpegBinaryPath   *string `json:"ffmpeg_binary_path,omitempty"`
	MkvmergeBinaryPath *string `json:"mkvmerge_binary_path,omitempty"`

	// 混流相关
	MuxAfterDone bool                 `json:"mux_after_done"`
	MuxOptions   *entity.MuxOptions   `json:"mux_options,omitempty"`
	MuxImports   []*entity.OutputFile `json:"mux_imports,omitempty"`

	// 流过滤器
	VideoFilter        *entity.StreamFilter `json:"video_filter,omitempty"`
	AudioFilter        *entity.StreamFilter `json:"audio_filter,omitempty"`
	SubtitleFilter     *entity.StreamFilter `json:"subtitle_filter,omitempty"`
	DropVideoFilter    *entity.StreamFilter `json:"drop_video_filter,omitempty"`
	DropAudioFilter    *entity.StreamFilter `json:"drop_audio_filter,omitempty"`
	DropSubtitleFilter *entity.StreamFilter `json:"drop_subtitle_filter,omitempty"`

	// 范围和关键词过滤
	CustomRange *entity.CustomRange `json:"custom_range,omitempty"`
	AdKeywords  []string            `json:"ad_keywords,omitempty"`

	// 直播相关
	LivePerformAsVod  bool           `json:"live_perform_as_vod"`
	LiveRealTimeMerge bool           `json:"live_real_time_merge"`
	LiveKeepSegments  bool           `json:"live_keep_segments"`
	LivePipeMux       bool           `json:"live_pipe_mux"`
	LiveFixVttByAudio bool           `json:"live_fix_vtt_by_audio"`
	LiveRecordLimit   *time.Duration `json:"live_record_limit,omitempty"`
	LiveWaitTime      *int           `json:"live_wait_time,omitempty"`
	LiveTakeCount     int            `json:"live_take_count"`

	// 任务调度
	TaskStartAt *time.Time `json:"task_start_at,omitempty"`

	// URL处理器
	URLProcessorArgs *string `json:"url_processor_args,omitempty"`

	// 其他高级选项
	AllowHlsMultiExtMap bool `json:"allow_hls_multi_ext_map"`
}

// NewMyOption 创建新的选项实例
func NewMyOption() *MyOption {
	return &MyOption{
		// 设置默认值
		Headers:              make(map[string]string),
		LogLevel:             entity.LogLevelInfo,
		ThreadCount:          8,
		DownloadRetryCount:   3,
		WebRequestRetryCount: 3,
		HTTPRequestTimeout:   60.0,
		UseSystemProxy:       true,
		DelAfterDone:         true,
		CheckSegmentsCount:   true,
		WriteMetaJSON:        true,
		AutoSubtitleFix:      true,
		SubtitleFormat:       entity.SubtitleFormatSRT,
		DecryptionEngine:     entity.DecryptEngineMP4Decrypt,
		LiveKeepSegments:     true,
		LiveTakeCount:        15,
		Keys:                 make([]string, 0),
		AdKeywords:           make([]string, 0),
		MuxImports:           make([]*entity.OutputFile, 0),
	}
}

// Validate 验证选项的合法性
func (opt *MyOption) Validate() error {
	if opt.Input == "" {
		return fmt.Errorf("input URL is required")
	}

	if opt.ThreadCount <= 0 {
		opt.ThreadCount = 1
	}

	if opt.DownloadRetryCount < 0 {
		opt.DownloadRetryCount = 0
	}

	if opt.WebRequestRetryCount < 0 {
		opt.WebRequestRetryCount = 0
	}

	if opt.HTTPRequestTimeout <= 0 {
		opt.HTTPRequestTimeout = 60.0
	}

	if opt.LiveTakeCount <= 0 {
		opt.LiveTakeCount = 15
	}

	return nil
}

// GetSaveName 获取保存名称
func (opt *MyOption) GetSaveName() string {
	if opt.SaveName != nil && *opt.SaveName != "" {
		return *opt.SaveName
	}
	return fmt.Sprintf("download_%s", time.Now().Format("2006-01-02_15-04-05"))
}

// GetSaveDir 获取保存目录
func (opt *MyOption) GetSaveDir() string {
	if opt.SaveDir != nil && *opt.SaveDir != "" {
		return *opt.SaveDir
	}
	return "Downloads"
}

// GetTmpDir 获取临时目录
func (opt *MyOption) GetTmpDir() string {
	if opt.TmpDir != nil && *opt.TmpDir != "" {
		return *opt.TmpDir
	}
	return filepath.Join(opt.GetSaveDir(), opt.GetSaveName())
}

// HasVideoFilter 是否有视频过滤器
func (opt *MyOption) HasVideoFilter() bool {
	return opt.VideoFilter != nil || opt.DropVideoFilter != nil
}

// HasAudioFilter 是否有音频过滤器
func (opt *MyOption) HasAudioFilter() bool {
	return opt.AudioFilter != nil || opt.DropAudioFilter != nil
}

// HasSubtitleFilter 是否有字幕过滤器
func (opt *MyOption) HasSubtitleFilter() bool {
	return opt.SubtitleFilter != nil || opt.DropSubtitleFilter != nil
}

// IsLiveMode 是否为直播模式
func (opt *MyOption) IsLiveMode() bool {
	return opt.LiveRealTimeMerge || opt.LivePerformAsVod || opt.LivePipeMux
}

// GetFFmpegPath 获取FFmpeg路径
func (opt *MyOption) GetFFmpegPath() string {
	if opt.FFmpegBinaryPath != nil && *opt.FFmpegBinaryPath != "" {
		return *opt.FFmpegBinaryPath
	}
	return util.FindExecutable("ffmpeg")
}

// GetDecryptionBinaryPath 获取解密程序路径
func (opt *MyOption) GetDecryptionBinaryPath() string {
	if opt.DecryptionBinaryPath != nil && *opt.DecryptionBinaryPath != "" {
		return *opt.DecryptionBinaryPath
	}

	switch opt.DecryptionEngine {
	case entity.DecryptEngineMP4Decrypt:
		return util.FindExecutable("mp4decrypt")
	case entity.DecryptEngineShaka:
		return util.FindExecutable("packager")
	default:
		return util.FindExecutable("mp4decrypt")
	}
}

// String 返回选项的字符串表示
func (opt *MyOption) String() string {
	return fmt.Sprintf("MyOption{Input: %s, ThreadCount: %d, LogLevel: %v}",
		opt.Input, opt.ThreadCount, opt.LogLevel)
}
