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
func Decrypt(decryptEngine, decryptionBinaryPath string, keys []string, encFile, decFile, kid string, isMultiDRM bool) (bool, error) {
	if _, err := os.Stat(decryptionBinaryPath); os.IsNotExist(err) {
		Logger.Error("解密程序 %s 不存在", decryptionBinaryPath)
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
		return false, fmt.Errorf("不支持的解密引擎: %s", decryptEngine)
	}

	cmd := exec.Command(decryptionBinaryPath, args...)
	Logger.Debug("执行解密命令: %s %s", decryptionBinaryPath, strings.Join(args, " "))

	output, err := cmd.CombinedOutput()
	if err != nil {
		Logger.Error("解密失败: %s", string(output))
		return false, fmt.Errorf("解密命令执行失败: %w", err)
	}

	Logger.Info("解密成功: %s", decFile)
	return true, nil
}
