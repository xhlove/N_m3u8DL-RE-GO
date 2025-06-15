package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"os"
)

// AESDecrypt 执行AES解密
func AESDecrypt(data, key, iv []byte, mode string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建AES cipher失败: %w", err)
	}

	switch mode {
	case "CBC":
		return aesDecryptCBC(block, data, iv)
	case "CTR":
		return aesDecryptCTR(block, data, iv)
	case "ECB":
		return aesDecryptECB(block, data)
	default:
		return nil, fmt.Errorf("不支持的AES模式: %s", mode)
	}
}

// aesDecryptCBC AES-CBC解密
func aesDecryptCBC(block cipher.Block, data, iv []byte) ([]byte, error) {
	if len(data) < aes.BlockSize {
		return nil, fmt.Errorf("密文长度不足")
	}

	if len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("密文长度不是块大小的倍数")
	}

	// 创建解密后的数据副本，避免原地修改
	decrypted := make([]byte, len(data))
	copy(decrypted, data)

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(decrypted, decrypted)

	// 去除PKCS7填充
	return removePKCS7Padding(decrypted)
}

// aesDecryptCTR AES-CTR解密
func aesDecryptCTR(block cipher.Block, data, iv []byte) ([]byte, error) {
	stream := cipher.NewCTR(block, iv)
	decrypted := make([]byte, len(data))
	stream.XORKeyStream(decrypted, data)
	return decrypted, nil
}

// aesDecryptECB AES-ECB解密
func aesDecryptECB(block cipher.Block, data []byte) ([]byte, error) {
	if len(data) < aes.BlockSize {
		return nil, fmt.Errorf("密文长度不足")
	}

	if len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("密文长度不是块大小的倍数")
	}

	decrypted := make([]byte, len(data))
	for i := 0; i < len(data); i += aes.BlockSize {
		block.Decrypt(decrypted[i:i+aes.BlockSize], data[i:i+aes.BlockSize])
	}

	// 去除PKCS7填充
	return removePKCS7Padding(decrypted)
}

// removePKCS7Padding 移除PKCS7填充
func removePKCS7Padding(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("数据为空")
	}

	padding := int(data[len(data)-1])

	// 验证填充值的合理性
	if padding == 0 || padding > 16 || padding > len(data) {
		// 可能没有填充或填充无效，直接返回原数据
		return data, nil
	}

	// 验证填充字节是否都相同
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			// 填充无效，可能没有填充，直接返回原数据
			return data, nil
		}
	}

	return data[:len(data)-padding], nil
}

// readFileBytes 读取文件字节
func readFileBytes(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}

// writeFileBytes 写入文件字节
func writeFileBytes(filePath string, data []byte) error {
	return os.WriteFile(filePath, data, 0644)
}

// AES128Decrypt AES-128解密文件（原地替换）
func AES128Decrypt(filePath string, key, iv []byte, mode string) error {
	data, err := readFileBytes(filePath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	decrypted, err := AESDecrypt(data, key, iv, mode)
	if err != nil {
		return fmt.Errorf("解密失败: %w", err)
	}

	err = writeFileBytes(filePath, decrypted)
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}
