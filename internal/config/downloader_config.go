package config

import "N_m3u8DL-RE-GO/internal/command"

// DownloaderConfig 下载器配置
type DownloaderConfig struct {
	// 选项配置
	MyOptions *command.MyOption `json:"my_options"`

	// 前置阶段生成的文件夹名
	DirPrefix string `json:"dir_prefix"`

	// 文件名模板
	SavePattern *string `json:"save_pattern,omitempty"`

	// 校验响应头的文件大小和实际大小
	CheckContentLength bool `json:"check_content_length"`

	// 请求头
	Headers map[string]string `json:"headers"`
}

// NewDownloaderConfig 创建新的下载器配置
func NewDownloaderConfig(options *command.MyOption) *DownloaderConfig {
	return &DownloaderConfig{
		MyOptions:          options,
		DirPrefix:          "",
		SavePattern:        nil,
		CheckContentLength: true,
		Headers:            make(map[string]string),
	}
}
