package util

import (
	"strings"
)

// ComplexParamParser 复杂参数解析器
// 用于解析类似 "format=mp4:muxer=ffmpeg:keep=true" 这样的参数
type ComplexParamParser struct {
	params map[string]string
}

// NewComplexParamParser 创建新的复杂参数解析器
func NewComplexParamParser(input string) *ComplexParamParser {
	parser := &ComplexParamParser{
		params: make(map[string]string),
	}
	parser.parse(input)
	return parser
}

// parse 解析参数字符串
func (p *ComplexParamParser) parse(input string) {
	if input == "" {
		return
	}

	// 按冒号分割参数
	parts := strings.Split(input, ":")
	for _, part := range parts {
		// 按等号分割键值对
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			// 去除引号
			if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
				value = value[1 : len(value)-1]
			}
			p.params[key] = value
		}
	}
}

// GetValue 获取参数值
func (p *ComplexParamParser) GetValue(key string) string {
	return p.params[key]
}

// HasKey 检查是否存在指定键
func (p *ComplexParamParser) HasKey(key string) bool {
	_, exists := p.params[key]
	return exists
}

// GetAllParams 获取所有参数
func (p *ComplexParamParser) GetAllParams() map[string]string {
	result := make(map[string]string)
	for k, v := range p.params {
		result[k] = v
	}
	return result
}
