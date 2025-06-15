package entity

import "fmt"

// CustomRange 自定义范围
type CustomRange struct {
	InputStr      string   `json:"inputStr"`
	StartSec      *float64 `json:"startSec,omitempty"`
	EndSec        *float64 `json:"endSec,omitempty"`
	StartSegIndex *int64   `json:"startSegIndex,omitempty"`
	EndSegIndex   *int64   `json:"endSegIndex,omitempty"`
}

func (c *CustomRange) String() string {
	return fmt.Sprintf("StartSec: %v, EndSec: %v, StartSegIndex: %v, EndSegIndex: %v",
		c.StartSec, c.EndSec, c.StartSegIndex, c.EndSegIndex)
}
