package entity

// MuxOptions 复用选项
type MuxOptions struct {
	UseMkvmerge  bool          `json:"useMkvmerge"`
	MuxFormat    MuxFormat     `json:"muxFormat"`
	KeepFiles    bool          `json:"keepFiles"`
	SkipSubtitle bool          `json:"skipSubtitle"`
	BinPath      string        `json:"binPath,omitempty"`
	MuxerPath    string        `json:"muxerPath,omitempty"`
	MuxImports   []*OutputFile `json:"muxImports,omitempty"` // 重要修复：添加MuxImports字段
}

// OutputFile 输出文件
type OutputFile struct {
	Index       int    `json:"index"`
	FilePath    string `json:"filePath"`
	LangCode    string `json:"langCode,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"` // audio or subtitle
}

// String 返回字符串表示
func (mo *MuxOptions) String() string {
	return mo.MuxFormat.String()
}
