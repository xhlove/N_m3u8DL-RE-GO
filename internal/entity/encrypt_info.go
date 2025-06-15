package entity

// EncryptInfo 加密信息
type EncryptInfo struct {
	Method     EncryptMethod `json:"Method"`
	Key        []byte        `json:"Key,omitempty"`
	IV         []byte        `json:"IV,omitempty"`
	URI        string        `json:"URI,omitempty"`
	KeyFormat  string        `json:"KeyFormat,omitempty"`
	KeyFormatV string        `json:"KeyFormatVersions,omitempty"`
}

// NewEncryptInfo 创建新的加密信息
func NewEncryptInfo() *EncryptInfo {
	return &EncryptInfo{
		Method: EncryptMethodNone,
	}
}

// IsEncrypted 判断是否加密
func (e *EncryptInfo) IsEncrypted() bool {
	return e.Method != EncryptMethodNone
}
