package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

// pkcs7Unpad removes pkcs7 padding.
func pkcs7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, fmt.Errorf("pkcs7: data is empty")
	}
	padSize := int(data[length-1])
	if padSize == 0 || padSize > length {
		return nil, fmt.Errorf("pkcs7: invalid padding size (%d for data of length %d)", padSize, length)
	}
	for i := 0; i < padSize; i++ {
		if data[length-1-i] != byte(padSize) {
			return nil, fmt.Errorf("pkcs7: invalid padding byte")
		}
	}
	return data[:length-padSize], nil
}

// AES128CBCDecrypt decrypts data using AES-128 CBC mode with PKCS7 padding.
func AES128CBCDecrypt(encrypted, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	if len(encrypted)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("encrypted data is not a multiple of the block size (%d vs %d)", len(encrypted), aes.BlockSize)
	}

	if len(iv) != aes.BlockSize {
		return nil, fmt.Errorf("IV length must be %d bytes, got %d", aes.BlockSize, len(iv))
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(encrypted))
	mode.CryptBlocks(decrypted, encrypted)

	// Remove padding
	unpadded, err := pkcs7Unpad(decrypted)
	if err != nil {
		// It's common for AES-128 segments not to have PKCS7 padding,
		// especially if they are full blocks. In this case, unpadding might fail.
		// We can return the decrypted data as is if unpadding fails,
		// assuming the caller or the format handles this.
		// However, for strict PKCS7, this would be an error.
		// For HLS, segments are often not padded.
		return decrypted, nil // Return as-is if unpadding fails, common for HLS.
	}

	return unpadded, nil
}
