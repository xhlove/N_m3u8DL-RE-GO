package util

import (
	"fmt"
	"os/exec"
	"strings"
)

// ExecuteCommand 执行命令
func ExecuteCommand(binary string, args ...string) error {
	Logger.Debug("执行命令: %s %s", binary, strings.Join(args, " "))

	cmd := exec.Command(binary, args...)

	// 捕获输出
	output, err := cmd.CombinedOutput()
	if err != nil {
		Logger.Error("命令执行失败: %s", err.Error())
		if len(output) > 0 {
			Logger.Error("命令输出: %s", string(output))
		}
		return fmt.Errorf("命令执行失败: %w", err)
	}

	if len(output) > 0 {
		Logger.Debug("命令输出: %s", string(output))
	}

	Logger.Debug("命令执行成功")
	return nil
}

// ExecuteCommandWithOutput 执行命令并返回输出
func ExecuteCommandWithOutput(binary string, args ...string) (string, error) {
	Logger.Debug("执行命令: %s %s", binary, strings.Join(args, " "))

	cmd := exec.Command(binary, args...)

	// 捕获输出
	output, err := cmd.CombinedOutput()
	if err != nil {
		Logger.Error("命令执行失败: %s", err.Error())
		if len(output) > 0 {
			Logger.Error("命令输出: %s", string(output))
		}
		return string(output), fmt.Errorf("命令执行失败: %w", err)
	}

	Logger.Debug("命令执行成功")
	return string(output), nil
}
