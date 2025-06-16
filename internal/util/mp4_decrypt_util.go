package util

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// SearchKeyFromFile searches for a key in a text file based on the KID.
// The expected format in the file is KID:KEY.
func SearchKeyFromFile(keyTextFile, kid string) (string, error) {
	if keyTextFile == "" || kid == "" {
		return "", nil
	}

	file, err := os.Open(keyTextFile)
	if err != nil {
		return "", fmt.Errorf("failed to open key file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "--key") {
			line = strings.TrimPrefix(line, "--key ")
		}
		parts := strings.Split(line, ":")
		if len(parts) == 2 {
			fileKid := strings.ReplaceAll(parts[0], "-", "")
			fileKey := parts[1]
			if strings.EqualFold(fileKid, strings.ReplaceAll(kid, "-", "")) {
				Logger.Info("从文件 %s 中找到KID %s", keyTextFile, kid)
				return fmt.Sprintf("%s:%s", kid, fileKey), nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read key file: %w", err)
	}

	return "", nil // Not found
}

// Decrypt invokes an external decryption tool to decrypt a file.
func Decrypt(decryptEngine, decryptionBinaryPath string, keys []string, encFile, decFile, kid string, task *Task) (bool, error) {
	if decryptEngine != "MP4DECRYPT" {
		// For now, only MP4DECRYPT is supported for CENC.
		// AES-128 is handled internally.
		return false, fmt.Errorf("unsupported decrypt engine for this function: %s", decryptEngine)
	}

	if _, err := os.Stat(decryptionBinaryPath); os.IsNotExist(err) {
		err = fmt.Errorf("解密程序 %s 不存在", decryptionBinaryPath)
		if task != nil {
			task.SetError(err)
		}
		Logger.Error(err.Error())
		return false, err
	}

	var args []string
	switch strings.ToUpper(decryptEngine) {
	case "MP4DECRYPT":
		for _, key := range keys {
			args = append(args, "--key", key)
		}
		args = append(args, encFile, decFile)
	// Add other engines like SHAKA_PACKAGER later
	default:
		err := fmt.Errorf("不支持的解密引擎: %s", decryptEngine)
		if task != nil {
			task.SetError(err)
		}
		return false, err
	}

	cmd := exec.Command(decryptionBinaryPath, args...)
	Logger.Debug("执行解密命令: %s %s", decryptionBinaryPath, strings.Join(args, " "))

	output, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("解密命令执行失败: %w, output: %s", err, string(output))
		if task != nil {
			task.SetError(err)
		}
		Logger.Error("解密失败: %s", string(output))
		return false, err
	}

	if task != nil {
		task.Increment(1) // Increment the overall task by 1 for each successful segment decryption
	}
	Logger.Info("解密成功: %s", decFile)
	return true, nil
}
