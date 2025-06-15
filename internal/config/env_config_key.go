package config

// EnvConfigKey 通过配置环境变量来实现更细节地控制某些逻辑
const (
	// ReKeepImageSegments 当此值为1时, 在图形字幕处理逻辑中PNG生成后不再删除m4s文件
	ReKeepImageSegments = "RE_KEEP_IMAGE_SEGMENTS"

	// ReLivePipeOptions 控制启用PipeMux时, 具体ffmpeg命令行
	ReLivePipeOptions = "RE_LIVE_PIPE_OPTIONS"

	// ReLivePipeTmpDir 控制启用PipeMux时, 非Windows环境下命名管道文件的生成目录
	ReLivePipeTmpDir = "RE_LIVE_PIPE_TMP_DIR"
)
