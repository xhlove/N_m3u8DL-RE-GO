package downloader

import (
	"fmt"
	"path/filepath"
	"sync"

	"N_m3u8DL-RE-GO/internal/entity"
	"N_m3u8DL-RE-GO/internal/util"
)

// DownloadResult 下载结果
type DownloadResult struct {
	Success      bool
	FilePath     string
	Error        error
	SegmentIndex int64
	RetryCount   int
}

// SegmentDownloader 段下载器
type SegmentDownloader struct {
	headers     map[string]string
	retryConfig util.RetryConfig
}

// NewSegmentDownloader 创建段下载器
func NewSegmentDownloader(headers map[string]string, retryConfig util.RetryConfig) *SegmentDownloader {
	return &SegmentDownloader{
		headers:     headers,
		retryConfig: retryConfig,
	}
}

// DownloadSegment 下载单个段
func (d *SegmentDownloader) DownloadSegment(segment *entity.MediaSegment, outputDir string) *DownloadResult {
	result := &DownloadResult{
		SegmentIndex: segment.Index,
		Success:      false,
	}

	// 生成文件名
	fileName := fmt.Sprintf("segment_%d.ts", segment.Index)
	if segment.NameFromVar != "" {
		fileName = segment.NameFromVar
	}

	outputPath := filepath.Join(outputDir, fileName)
	result.FilePath = outputPath

	// 下载逻辑
	err := util.DoRetry(func() error {
		util.Logger.Debug(fmt.Sprintf("正在下载段 %d: %s", segment.Index, segment.URL))

		data, err := util.GetBytes(segment.URL, d.headers)
		if err != nil {
			return err
		}

		// 解密（如果需要）
		if segment.IsEncrypted {
			decryptedData, err := d.decryptSegment(data, segment.EncryptInfo)
			if err != nil {
				return fmt.Errorf("解密失败: %w", err)
			}
			data = decryptedData
		}

		// 写入文件
		err = util.WriteFile(outputPath, data)
		if err != nil {
			return fmt.Errorf("写入文件失败: %w", err)
		}

		util.Logger.Debug(fmt.Sprintf("段 %d 下载完成", segment.Index))
		return nil
	}, d.retryConfig)

	if err != nil {
		result.Error = err
		util.Logger.Error(fmt.Sprintf("段 %d 下载失败: %s", segment.Index, err.Error()))
	} else {
		result.Success = true
	}

	return result
}

// decryptSegment 解密段数据
func (d *SegmentDownloader) decryptSegment(data []byte, encryptInfo *entity.EncryptInfo) ([]byte, error) {
	if encryptInfo.Method == entity.EncryptMethodNone {
		return data, nil
	}

	// TODO: 实现各种解密算法
	switch encryptInfo.Method {
	case entity.EncryptMethodAES128:
		return d.decryptAES128(data, encryptInfo)
	case entity.EncryptMethodAESCTR:
		return d.decryptAESCTR(data, encryptInfo)
	case entity.EncryptMethodSampleAES:
		return d.decryptSampleAES(data, encryptInfo)
	default:
		return nil, fmt.Errorf("不支持的加密方法: %s", encryptInfo.Method.String())
	}
}

// decryptAES128 解密AES-128
func (d *SegmentDownloader) decryptAES128(data []byte, encryptInfo *entity.EncryptInfo) ([]byte, error) {
	// TODO: 实现AES-128解密
	util.Logger.Warn("AES-128解密尚未实现，返回原始数据")
	return data, nil
}

// decryptAESCTR 解密AES-CTR
func (d *SegmentDownloader) decryptAESCTR(data []byte, encryptInfo *entity.EncryptInfo) ([]byte, error) {
	// TODO: 实现AES-CTR解密
	util.Logger.Warn("AES-CTR解密尚未实现，返回原始数据")
	return data, nil
}

// decryptSampleAES 解密Sample-AES
func (d *SegmentDownloader) decryptSampleAES(data []byte, encryptInfo *entity.EncryptInfo) ([]byte, error) {
	// TODO: 实现Sample-AES解密
	util.Logger.Warn("Sample-AES解密尚未实现，返回原始数据")
	return data, nil
}

// StreamDownloader 流下载器
type StreamDownloader struct {
	segmentDownloader *SegmentDownloader
	threadCount       int
}

// NewStreamDownloader 创建流下载器
func NewStreamDownloader(headers map[string]string, threadCount int, retryConfig util.RetryConfig) *StreamDownloader {
	return &StreamDownloader{
		segmentDownloader: NewSegmentDownloader(headers, retryConfig),
		threadCount:       threadCount,
	}
}

// DownloadStream 下载流
func (d *StreamDownloader) DownloadStream(stream *entity.StreamSpec, outputDir string) error {
	util.Logger.Info(fmt.Sprintf("开始下载流: %s", stream.ToString()))

	if stream.Playlist == nil {
		return fmt.Errorf("播放列表为空")
	}

	// 创建输出目录
	err := util.CreateDir(outputDir)
	if err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 获取所有段
	segments := stream.Playlist.GetAllSegments()
	if len(segments) == 0 {
		return fmt.Errorf("没有找到可下载的段")
	}

	util.Logger.Info(fmt.Sprintf("共有 %d 个段需要下载", len(segments)))

	// 下载初始化段（如果有）
	if stream.Playlist.MediaInit != nil {
		util.Logger.Info("下载初始化段")
		initResult := d.segmentDownloader.DownloadSegment(stream.Playlist.MediaInit, outputDir)
		if !initResult.Success {
			util.Logger.Warn(fmt.Sprintf("初始化段下载失败: %s", initResult.Error))
		}
	}

	// 并发下载段
	return d.downloadSegmentsConcurrently(segments, outputDir)
}

// downloadSegmentsConcurrently 并发下载段
func (d *StreamDownloader) downloadSegmentsConcurrently(segments []*entity.MediaSegment, outputDir string) error {
	maxWorkers := d.threadCount
	if maxWorkers <= 0 {
		maxWorkers = 8 // 默认8个线程
	}

	// 创建工作池
	segmentChan := make(chan *entity.MediaSegment, len(segments))
	resultChan := make(chan *DownloadResult, len(segments))

	// 启动工作goroutine
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for segment := range segmentChan {
				result := d.segmentDownloader.DownloadSegment(segment, outputDir)
				resultChan <- result
			}
		}()
	}

	// 发送段到工作池
	go func() {
		defer close(segmentChan)
		for _, segment := range segments {
			segmentChan <- segment
		}
	}()

	// 等待所有工作完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	var failedResults []*DownloadResult
	successCount := 0

	for result := range resultChan {
		if result.Success {
			successCount++
		} else {
			failedResults = append(failedResults, result)
		}
	}

	util.Logger.Info(fmt.Sprintf("下载完成: 成功 %d, 失败 %d", successCount, len(failedResults)))

	if len(failedResults) > 0 {
		util.Logger.Error("以下段下载失败:")
		for _, result := range failedResults {
			util.Logger.Error(fmt.Sprintf("段 %d: %s", result.SegmentIndex, result.Error))
		}
		return fmt.Errorf("下载完成，但有 %d 个段失败", len(failedResults))
	}

	return nil
}
