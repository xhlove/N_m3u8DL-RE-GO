package crypto

import (
	"fmt"

	"golang.org/x/crypto/chacha20"
)

// ChaCha20Decrypt ChaCha20解密
func ChaCha20Decrypt(data, key, nonce []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("ChaCha20密钥长度必须是32字节，实际: %d", len(key))
	}

	if len(nonce) != 12 {
		return nil, fmt.Errorf("ChaCha20 nonce长度必须是12字节，实际: %d", len(nonce))
	}

	cipher, err := chacha20.NewUnauthenticatedCipher(key, nonce)
	if err != nil {
		return nil, fmt.Errorf("创建ChaCha20 cipher失败: %w", err)
	}

	decrypted := make([]byte, len(data))
	cipher.XORKeyStream(decrypted, data)

	return decrypted, nil
}

// ChaCha20DecryptPer1024Bytes 按1024字节块解密ChaCha20（类似C#版本的实现）
func ChaCha20DecryptPer1024Bytes(data, key, nonce []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("ChaCha20密钥长度必须是32字节，实际: %d", len(key))
	}

	if len(nonce) != 12 {
		return nil, fmt.Errorf("ChaCha20 nonce长度必须是12字节，实际: %d", len(nonce))
	}

	decrypted := make([]byte, len(data))
	copy(decrypted, data)

	const blockSize = 1024
	blocks := (len(data) + blockSize - 1) / blockSize

	for i := 0; i < blocks; i++ {
		start := i * blockSize
		end := start + blockSize
		if end > len(data) {
			end = len(data)
		}

		// 为每个块创建新的cipher实例
		cipher, err := chacha20.NewUnauthenticatedCipher(key, nonce)
		if err != nil {
			return nil, fmt.Errorf("创建ChaCha20 cipher失败: %w", err)
		}

		// 跳过前面的字节以同步到当前块
		discard := make([]byte, start)
		cipher.XORKeyStream(discard, discard)

		// 解密当前块
		cipher.XORKeyStream(decrypted[start:end], data[start:end])
	}

	return decrypted, nil
}

// ChaCha20DecryptFile ChaCha20解密文件（原地替换）
func ChaCha20DecryptFile(filePath string, key, nonce []byte) error {
	data, err := readFileBytes(filePath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	decrypted, err := ChaCha20DecryptPer1024Bytes(data, key, nonce)
	if err != nil {
		return fmt.Errorf("解密失败: %w", err)
	}

	err = writeFileBytes(filePath, decrypted)
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}
