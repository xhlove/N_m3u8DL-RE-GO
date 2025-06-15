package entity

// MediaType 媒体类型枚举 - 重要修复：与C#版本保持一致的枚举值
type MediaType int

const (
	MediaTypeAudio          MediaType = 0  // 对应C# AUDIO = 0
	MediaTypeVideo          MediaType = 1  // 对应C# VIDEO = 1
	MediaTypeSubtitles      MediaType = 2  // 对应C# SUBTITLES = 2
	MediaTypeClosedCaptions MediaType = 3  // 对应C# CLOSED_CAPTIONS = 3
	MediaTypeUnknown        MediaType = -1 // 未知类型
)

func (m MediaType) String() string {
	switch m {
	case MediaTypeAudio:
		return "AUDIO"
	case MediaTypeVideo:
		return "VIDEO"
	case MediaTypeSubtitles:
		return "SUBTITLES"
	case MediaTypeClosedCaptions:
		return "CLOSED_CAPTIONS"
	default:
		return "UNKNOWN"
	}
}

// EncryptMethod 加密方法枚举
type EncryptMethod int

const (
	EncryptMethodNone EncryptMethod = iota
	EncryptMethodAES128
	EncryptMethodAES128ECB
	EncryptMethodAESCTR
	EncryptMethodAESCBC
	EncryptMethodSampleAES
	EncryptMethodSampleAESCTR
	EncryptMethodChacha20
	EncryptMethodCENC
	EncryptMethodCBCS
	EncryptMethodClearKey
	EncryptMethodUNKNOWN
)

func (e EncryptMethod) String() string {
	switch e {
	case EncryptMethodNone:
		return "NONE"
	case EncryptMethodAES128:
		return "AES-128"
	case EncryptMethodAES128ECB:
		return "AES-128-ECB"
	case EncryptMethodAESCTR:
		return "AES-CTR"
	case EncryptMethodAESCBC:
		return "AES-CBC"
	case EncryptMethodSampleAES:
		return "SAMPLE-AES"
	case EncryptMethodSampleAESCTR:
		return "SAMPLE-AES-CTR"
	case EncryptMethodChacha20:
		return "CHACHA20"
	case EncryptMethodCENC:
		return "CENC"
	case EncryptMethodCBCS:
		return "CBCS"
	case EncryptMethodClearKey:
		return "CLEAR-KEY"
	case EncryptMethodUNKNOWN:
		return "UNKNOWN"
	default:
		return "UNKNOWN"
	}
}

// MarshalJSON 实现JSON序列化
func (e EncryptMethod) MarshalJSON() ([]byte, error) {
	return []byte(`"` + e.String() + `"`), nil
}

// UnmarshalJSON 实现JSON反序列化
func (e *EncryptMethod) UnmarshalJSON(data []byte) error {
	str := string(data)
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}

	switch str {
	case "NONE":
		*e = EncryptMethodNone
	case "AES-128":
		*e = EncryptMethodAES128
	case "AES-128-ECB":
		*e = EncryptMethodAES128ECB
	case "AES-CTR":
		*e = EncryptMethodAESCTR
	case "AES-CBC":
		*e = EncryptMethodAESCBC
	case "SAMPLE-AES":
		*e = EncryptMethodSampleAES
	case "SAMPLE-AES-CTR":
		*e = EncryptMethodSampleAESCTR
	case "CHACHA20":
		*e = EncryptMethodChacha20
	case "CENC":
		*e = EncryptMethodCENC
	case "CBCS":
		*e = EncryptMethodCBCS
	case "CLEAR-KEY":
		*e = EncryptMethodClearKey
	default:
		*e = EncryptMethodUNKNOWN
	}
	return nil
}

// ExtractorType 提取器类型枚举
type ExtractorType int

const (
	ExtractorTypeUnknown ExtractorType = iota
	ExtractorTypeHLS
	ExtractorTypeDASH
	ExtractorTypeMSS
	ExtractorTypeLiveTS
)

func (e ExtractorType) String() string {
	switch e {
	case ExtractorTypeHLS:
		return "HLS"
	case ExtractorTypeDASH:
		return "DASH"
	case ExtractorTypeMSS:
		return "MSS"
	case ExtractorTypeLiveTS:
		return "LIVE_TS"
	default:
		return "UNKNOWN"
	}
}

// RoleType 角色类型枚举
type RoleType int

const (
	RoleTypeNone RoleType = iota
	RoleTypeMain
	RoleTypeAlternate
	RoleTypeSupplementary
	RoleTypeCommentary
	RoleTypeDub
	RoleTypeEmergency
	RoleTypeSign
	RoleTypeMetadata
	RoleTypeSubtitle
	RoleTypeCaption
	RoleTypeDescription
)

func (r RoleType) String() string {
	switch r {
	case RoleTypeMain:
		return "MAIN"
	case RoleTypeAlternate:
		return "ALTERNATE"
	case RoleTypeSupplementary:
		return "SUPPLEMENTARY"
	case RoleTypeCommentary:
		return "COMMENTARY"
	case RoleTypeDub:
		return "DUB"
	case RoleTypeEmergency:
		return "EMERGENCY"
	case RoleTypeSign:
		return "SIGN"
	case RoleTypeMetadata:
		return "METADATA"
	case RoleTypeSubtitle:
		return "SUBTITLE"
	case RoleTypeCaption:
		return "CAPTION"
	case RoleTypeDescription:
		return "DESCRIPTION"
	default:
		return "NONE"
	}
}

// Choice 选择枚举
type Choice int

const (
	ChoiceNo Choice = iota
	ChoiceYes
)

func (c Choice) String() string {
	switch c {
	case ChoiceYes:
		return "YES"
	case ChoiceNo:
		return "NO"
	default:
		return "NO"
	}
}

// MuxFormat 合并格式枚举
type MuxFormat int

const (
	MuxFormatMP4 MuxFormat = iota
	MuxFormatMKV
	MuxFormatTS
	MuxFormatMOV
	MuxFormatFLV
)

func (m MuxFormat) String() string {
	switch m {
	case MuxFormatMP4:
		return "MP4"
	case MuxFormatMKV:
		return "MKV"
	case MuxFormatTS:
		return "TS"
	case MuxFormatMOV:
		return "MOV"
	case MuxFormatFLV:
		return "FLV"
	default:
		return "MP4"
	}
}

// SubtitleFormat 字幕格式枚举
type SubtitleFormat int

const (
	SubtitleFormatVTT SubtitleFormat = iota
	SubtitleFormatSRT
	SubtitleFormatASS
)

func (s SubtitleFormat) String() string {
	switch s {
	case SubtitleFormatVTT:
		return "VTT"
	case SubtitleFormatSRT:
		return "SRT"
	case SubtitleFormatASS:
		return "ASS"
	default:
		return "SRT"
	}
}

// LogLevel 日志级别枚举
type LogLevel int

const (
	LogLevelOff LogLevel = iota
	LogLevelError
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelOff:
		return "OFF"
	case LogLevelError:
		return "ERROR"
	case LogLevelWarn:
		return "WARN"
	case LogLevelInfo:
		return "INFO"
	case LogLevelDebug:
		return "DEBUG"
	default:
		return "INFO"
	}
}

// DecryptEngine 解密引擎枚举
type DecryptEngine int

const (
	DecryptEngineAuto DecryptEngine = iota
	DecryptEngineNone
	DecryptEngineBento4
	DecryptEngineMP4Decrypt
	DecryptEngineShaka
	DecryptEngineNative
)

func (d DecryptEngine) String() string {
	switch d {
	case DecryptEngineAuto:
		return "AUTO"
	case DecryptEngineNone:
		return "NONE"
	case DecryptEngineBento4:
		return "BENTO4"
	case DecryptEngineMP4Decrypt:
		return "MP4DECRYPT"
	case DecryptEngineShaka:
		return "SHAKA"
	case DecryptEngineNative:
		return "NATIVE"
	default:
		return "AUTO"
	}
}
