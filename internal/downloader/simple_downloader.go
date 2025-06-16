package downloader

import (
	"fmt"
	"path/filepath"
	"time"

	"N_m3u8DL-RE-GO/internal/crypto"
	"N_m3u8DL-RE-GO/internal/entity"
	"N_m3u8DL-RE-GO/internal/util"
)

// SpeedCounter is an interface for speed counting to decouple from util.SpeedContainer.
type SpeedCounter interface {
	Add(int64)
}

// SimpleDownloadConfig 简单下载器配置
type SimpleDownloadConfig struct {
	OutputDir          string
	SaveName           string
	SaveDir            string
	TmpDir             string
	ThreadCount        int
	RetryCount         int
	Headers            map[string]string
	CheckLength        bool
	DeleteAfterDone    bool
	BinaryMerge        bool
	SkipMerge          bool
	ConcurrentDownload bool
	NoAnsiColor        bool
	LogLevel           util.LogLevel
}

// SimpleDownloader 简单下载器，类似C#版本的SimpleDownloader
type SimpleDownloader struct {
	config      *SimpleDownloadConfig
	retryConfig util.RetryConfig
}

// NewSimpleDownloader 创建简单下载器
func NewSimpleDownloader(config *SimpleDownloadConfig) *SimpleDownloader {
	retryConfig := util.RetryConfig{
		MaxRetries: config.RetryCount,
		RetryDelay: time.Second,
		Backoff:    2.0,
	}

	return &SimpleDownloader{
		config:      config,
		retryConfig: retryConfig,
	}
}

// DownloadSegment 下载分段，返回下载结果
func (sd *SimpleDownloader) DownloadSegment(segment *entity.MediaSegment, outputPath string, speedCounter SpeedCounter, headers map[string]string, decryptTask *util.Task) *DownloadResult {
	result := &DownloadResult{
		SegmentIndex: segment.Index,
		FilePath:     outputPath,
		Success:      false,
	}

	// 合并headers
	mergedHeaders := make(map[string]string)
	for k, v := range sd.config.Headers {
		mergedHeaders[k] = v
	}
	for k, v := range headers {
		mergedHeaders[k] = v
	}

	// 下载逻辑
	err := util.DoRetry(func() error {
		util.Logger.Debug("正在下载分段 %d: %s", segment.Index, segment.URL)

		// 创建输出目录
		if err := util.CreateDir(filepath.Dir(outputPath)); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}

		// 下载数据
		data, err := util.GetBytes(segment.URL, mergedHeaders)
		if err != nil {
			return err
		}

		// 更新速度统计
		if speedCounter != nil {
			speedCounter.Add(int64(len(data)))
		}

		// 解密（如果需要）
		if segment.IsEncrypted && segment.EncryptInfo != nil {
			util.Logger.Debug("分段 %d 需要解密，方法: %s, 密钥长度: %d, IV长度: %d",
				segment.Index, segment.EncryptInfo.Method.String(),
				len(segment.EncryptInfo.Key), len(segment.EncryptInfo.IV))

			decryptedData, err := sd.decryptSegment(data, segment.EncryptInfo)
			if err != nil {
				if decryptTask != nil {
					// Mark the overall decrypt task as error if one segment fails.
					// Note: This might be too aggressive if some segments are fine.
					// Consider if partial success is acceptable for decryption.
					decryptTask.SetError(fmt.Errorf("分段 %d 解密失败: %w", segment.Index, err))
				}
				return fmt.Errorf("解密失败: %w", err)
			}

			util.Logger.Debug("分段 %d 解密完成，原始大小: %d, 解密后大小: %d",
				segment.Index, len(data), len(decryptedData))

			// 检查解密后的数据是否是有效的TS数据
			if len(decryptedData) > 4 {
				// TS包应该以0x47开头
				if decryptedData[0] == 0x47 {
					util.Logger.Debug("分段 %d 解密后数据格式正确 (TS包)", segment.Index)
				} else {
					util.Logger.Warn("分段 %d 解密后数据可能格式不正确，首字节: 0x%02x", segment.Index, decryptedData[0])
				}
			}

			data = decryptedData
			if decryptTask != nil {
				decryptTask.GetSpeedContainer().Add(int64(len(data))) // Add decrypted size to speed counter
			}
		}

		// 写入文件
		err = util.WriteFile(outputPath, data)
		if err != nil {
			return fmt.Errorf("写入文件失败: %w", err)
		}

		util.Logger.Debug("分段 %d 下载完成", segment.Index)
		return nil
	}, sd.retryConfig)

	if err != nil {
		result.Error = err
		util.Logger.Error("分段 %d 下载失败: %s", segment.Index, err.Error())
	} else {
		result.Success = true
		if decryptTask != nil && segment.IsEncrypted && segment.EncryptInfo != nil && segment.EncryptInfo.Method == entity.EncryptMethodAES128 {
			decryptTask.Increment(1) // Increment overall decrypt task
		}
	}

	return result
}

// decryptSegment 解密分段数据
func (sd *SimpleDownloader) decryptSegment(data []byte, encryptInfo *entity.EncryptInfo) ([]byte, error) {
	if encryptInfo.Method == entity.EncryptMethodNone {
		return data, nil
	}

	switch encryptInfo.Method {
	case entity.EncryptMethodAES128:
		return sd.decryptAES128(data, encryptInfo)
	case entity.EncryptMethodAES128ECB:
		return sd.decryptAESECB(data, encryptInfo)
	case entity.EncryptMethodAESCTR:
		return sd.decryptAESCTR(data, encryptInfo)
	case entity.EncryptMethodChacha20:
		return sd.decryptChaCha20(data, encryptInfo)
	case entity.EncryptMethodSampleAES:
		return sd.decryptSampleAES(data, encryptInfo)
	case entity.EncryptMethodCENC:
		util.Logger.Warn("CENC解密需要外部工具支持")
		return data, nil
	default:
		return nil, fmt.Errorf("不支持的加密方法: %s", encryptInfo.Method.String())
	}
}

// decryptAES128 解密AES-128
func (sd *SimpleDownloader) decryptAES128(data []byte, encryptInfo *entity.EncryptInfo) ([]byte, error) {
	if encryptInfo.Key == nil {
		return nil, fmt.Errorf("AES-128解密缺少密钥")
	}

	if encryptInfo.IV == nil {
		return nil, fmt.Errorf("AES-128解密缺少IV")
	}

	return crypto.AES128CBCDecrypt(data, encryptInfo.Key, encryptInfo.IV)
}

// decryptAESCTR 解密AES-CTR
func (sd *SimpleDownloader) decryptAESCTR(data []byte, encryptInfo *entity.EncryptInfo) ([]byte, error) {
	if encryptInfo.Key == nil {
		return nil, fmt.Errorf("AES-CTR解密缺少密钥")
	}

	if encryptInfo.IV == nil {
		return nil, fmt.Errorf("AES-CTR解密缺少IV")
	}

	// TODO: Implement AES-CTR decryption if needed, for now, it might fall back or error.
	// For HLS, CBC is the standard.
	return nil, fmt.Errorf("AES-CTR decryption not yet fully implemented in crypto package for this path")
}

// decryptAESECB 解密AES-ECB
func (sd *SimpleDownloader) decryptAESECB(data []byte, encryptInfo *entity.EncryptInfo) ([]byte, error) {
	if encryptInfo.Key == nil {
		return nil, fmt.Errorf("AES-ECB解密缺少密钥")
	}

	if encryptInfo.IV == nil {
		// ECB模式不需要IV，创建一个零IV
		encryptInfo.IV = make([]byte, 16)
	}

	// TODO: Implement AES-ECB decryption if needed.
	return nil, fmt.Errorf("AES-ECB decryption not yet fully implemented in crypto package for this path")
}

// decryptChaCha20 解密ChaCha20
func (sd *SimpleDownloader) decryptChaCha20(data []byte, encryptInfo *entity.EncryptInfo) ([]byte, error) {
	if encryptInfo.Key == nil {
		return nil, fmt.Errorf("ChaCha20解密缺少密钥")
	}

	if encryptInfo.IV == nil {
		return nil, fmt.Errorf("ChaCha20解密缺少nonce")
	}

	// 使用按块解密的方式，类似C#版本
	return crypto.ChaCha20DecryptPer1024Bytes(data, encryptInfo.Key, encryptInfo.IV)
}

// decryptSampleAES 解密Sample-AES
func (sd *SimpleDownloader) decryptSampleAES(data []byte, encryptInfo *entity.EncryptInfo) ([]byte, error) {
	// Sample-AES是一种特殊的AES模式，通常用于音频/视频的部分加密
	// 这里先用CTR模式作为替代实现
	util.Logger.Warn("Sample-AES解密使用CTR模式替代实现")
	return sd.decryptAESCTR(data, encryptInfo)
}
