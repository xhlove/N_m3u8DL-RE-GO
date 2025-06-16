package util

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"N_m3u8DL-RE-GO/internal/entity"
)

// CombineMultipleFilesIntoSingleFile 合并多个文件到单个文件
func CombineMultipleFilesIntoSingleFile(files []string, outputFilePath string) error {
	if len(files) == 0 {
		return nil
	}

	// 只有一个文件，直接复制
	if len(files) == 1 {
		return copyFile(files[0], outputFilePath)
	}

	// 确保输出目录存在
	if err := os.MkdirAll(filepath.Dir(outputFilePath), 0755); err != nil {
		return err
	}

	// 创建输出文件
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// 逐个复制文件内容
	for _, inputFilePath := range files {
		if inputFilePath == "" {
			continue
		}

		inputFile, err := os.Open(inputFilePath)
		if err != nil {
			Logger.Warn(fmt.Sprintf("无法打开文件 %s: %s", inputFilePath, err.Error()))
			continue
		}

		_, err = io.Copy(outputFile, inputFile)
		inputFile.Close()

		if err != nil {
			return fmt.Errorf("复制文件 %s 失败: %w", inputFilePath, err)
		}
	}

	return nil
}

// PartialCombineMultipleFiles 部分合并多个文件（避免命令行参数过长）
func PartialCombineMultipleFiles(files []string) ([]string, error) {
	var newFiles []string
	div := 100
	if len(files) > 90000 {
		div = 200
	}

	outputDir := filepath.Dir(files[0])
	outputName := filepath.Join(outputDir, "T")
	index := 0

	// 按照div的容量分割为小数组
	for i := 0; i < len(files); i += div {
		end := i + div
		if end > len(files) {
			end = len(files)
		}

		batch := files[i:end]
		if len(batch) == 0 {
			continue
		}

		output := outputName + fmt.Sprintf("%04d", index) + ".ts"
		err := CombineMultipleFilesIntoSingleFile(batch, output)
		if err != nil {
			return nil, err
		}

		newFiles = append(newFiles, output)

		// 合并后删除这些文件
		for _, file := range batch {
			os.Remove(file)
		}

		index++
	}

	return newFiles, nil
}

// MergeByFFmpeg 使用FFmpeg合并文件
func MergeByFFmpeg(ffmpegPath string, files []string, outputPath string, muxFormat string, useAACFilter bool, options *MergeOptions, workingDir string) error {
	if len(files) == 0 {
		return fmt.Errorf("没有文件需要合并")
	}

	absOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}

	if options == nil {
		options = &MergeOptions{}
	}

	dateString := time.Now().Format(time.RFC3339Nano)
	if options.RecTime != "" {
		if t, err := time.Parse(time.RFC3339, options.RecTime); err == nil {
			dateString = t.Format(time.RFC3339Nano)
		}
	}

	var ddpAudio string
	ddpAudioFile := strings.TrimSuffix(absOutputPath, filepath.Ext(absOutputPath)) + ".mp4.txt"
	if FileExists(ddpAudioFile) {
		if data, err := os.ReadFile(ddpAudioFile); err == nil {
			ddpAudio = strings.TrimSpace(string(data))
			if ddpAudio != "" {
				useAACFilter = false
			}
		}
	}

	var args []string
	args = append(args, "-loglevel", "warning", "-nostdin")

	if options.UseConcatDemuxer {
		// concat demuxer的txt 应该放在和分段一个目录下面
		tempFile, err := createConcatFile(files, workingDir)
		if err != nil {
			return err
		}
		defer os.Remove(tempFile)
		// 使用相对路径
		args = append(args, "-f", "concat", "-safe", "0", "-i", filepath.Base(tempFile))
	} else {
		var concatValue strings.Builder
		concatValue.WriteString("concat:")
		for i, file := range files {
			if i > 0 {
				concatValue.WriteString("|")
			}
			concatValue.WriteString(filepath.Base(file))
		}
		args = append(args, "-i", concatValue.String())
	}

	var finalOutputFile string
	switch strings.ToUpper(muxFormat) {
	case "MP4":
		if options.Poster != "" {
			args = append(args, "-i", options.Poster)
		}
		if ddpAudio != "" {
			args = append(args, "-i", ddpAudio)
		}

		if ddpAudio == "" {
			args = append(args, "-map", "0:v?", "-map", "0:a?", "-map", "0:s?")
		} else {
			posterIndex := "1"
			if options.Poster != "" {
				posterIndex = "2"
			}
			args = append(args, "-map", "0:v?", fmt.Sprintf("-map %s:a", posterIndex), "-map", "0:a?", "-map", "0:s?")
		}

		if options.Poster != "" {
			args = append(args, "-map", "1", "-c:v:1", "copy", "-disposition:v:1", "attached_pic")
		}

		if options.WriteDate {
			args = append(args, "-metadata", fmt.Sprintf("date=%s", dateString))
		}
		if options.EncodingTool != "" {
			args = append(args, "-metadata", fmt.Sprintf("encoding_tool=%s", options.EncodingTool))
		}
		if options.Title != "" {
			args = append(args, "-metadata", fmt.Sprintf("title=%s", options.Title))
		}
		if options.Copyright != "" {
			args = append(args, "-metadata", fmt.Sprintf("copyright=%s", options.Copyright))
		}
		if options.Comment != "" {
			args = append(args, "-metadata", fmt.Sprintf("comment=%s", options.Comment))
		}

		if options.AudioName != "" {
			audioIndex := "0"
			if ddpAudio != "" {
				audioIndex = "1"
			}
			args = append(args, fmt.Sprintf("-metadata:s:a:%s", audioIndex), fmt.Sprintf("title=%s", options.AudioName), fmt.Sprintf("-metadata:s:a:%s", audioIndex), fmt.Sprintf("handler=%s", options.AudioName))
		}
		if ddpAudio != "" {
			args = append(args, "-metadata:s:a:0", "title=DD+", "-metadata:s:a:0", "handler=DD+")
		}

		if options.FastStart {
			args = append(args, "-movflags", "+faststart")
		}
		args = append(args, "-c", "copy", "-y")
		if useAACFilter {
			args = append(args, "-bsf:a", "aac_adtstoasc")
		}
		finalOutputFile = fmt.Sprintf("%s.mp4", absOutputPath)
		args = append(args, finalOutputFile)

	case "MKV":
		args = append(args, "-map", "0", "-c", "copy", "-y")
		if useAACFilter {
			args = append(args, "-bsf:a", "aac_adtstoasc")
		}
		finalOutputFile = fmt.Sprintf("%s.mkv", absOutputPath)
		args = append(args, finalOutputFile)
	case "FLV":
		args = append(args, "-map", "0", "-c", "copy", "-y")
		if useAACFilter {
			args = append(args, "-bsf:a", "aac_adtstoasc")
		}
		finalOutputFile = fmt.Sprintf("%s.flv", absOutputPath)
		args = append(args, finalOutputFile)
	case "M4A":
		args = append(args, "-map", "0", "-c", "copy", "-f", "mp4", "-y")
		if useAACFilter {
			args = append(args, "-bsf:a", "aac_adtstoasc")
		}
		finalOutputFile = fmt.Sprintf("%s.m4a", absOutputPath)
		args = append(args, finalOutputFile)
	case "TS":
		args = append(args, "-map", "0", "-c", "copy", "-y", "-f", "mpegts", "-bsf:v", "h264_mp4toannexb")
		finalOutputFile = fmt.Sprintf("%s.ts", absOutputPath)
		args = append(args, finalOutputFile)
	case "EAC3":
		args = append(args, "-map", "0:a", "-c", "copy", "-y")
		finalOutputFile = fmt.Sprintf("%s.eac3", absOutputPath)
		args = append(args, finalOutputFile)
	case "AAC":
		args = append(args, "-map", "0:a", "-c", "copy", "-y")
		finalOutputFile = fmt.Sprintf("%s.m4a", absOutputPath)
		args = append(args, finalOutputFile)
	case "AC3":
		args = append(args, "-map", "0:a", "-c", "copy", "-y")
		finalOutputFile = fmt.Sprintf("%s.ac3", absOutputPath)
		args = append(args, finalOutputFile)
	default:
		return fmt.Errorf("不支持的格式: %s", muxFormat)
	}

	err = invokeFFmpeg(ffmpegPath, args, workingDir)
	if err != nil {
		return err
	}

	// 你应该用合并命令中的输出文件完整路径进行检查
	if !FileExists(finalOutputFile) {
		// 如果ffmpeg声称成功，但文件不存在，则返回错误
		return fmt.Errorf("ffmpeg post-check: merged file not found at %s", finalOutputFile)
	}

	return nil
}

// MergeOptions FFmpeg合并选项 - 重要修复：与C#版本参数保持一致
type MergeOptions struct {
	FastStart        bool
	WriteDate        bool
	UseConcatDemuxer bool
	Poster           string // 封面图片路径
	AudioName        string
	Title            string
	Copyright        string
	Comment          string
	EncodingTool     string
	RecTime          string
}

// OutputFile 输出文件信息
type OutputFile struct {
	FilePath    string
	MediaType   entity.MediaType
	LangCode    string
	Description string
	Mediainfos  []*MediaInfo
}

// MuxInputsByFFmpeg 使用FFmpeg复用多个输入文件
func MuxInputsByFFmpeg(ffmpegPath string, files []*OutputFile, outputPath string, muxFormat string, dateinfo bool, workingDir string) error {
	if len(files) == 0 {
		return fmt.Errorf("没有文件需要复用")
	}

	dateString := time.Now().Format(time.RFC3339)

	// 构建参数数组
	var args []string
	args = append(args, "-loglevel", "warning", "-nostdin", "-y", "-dn")

	// 添加输入文件
	for _, file := range files {
		args = append(args, "-i", file.FilePath)
	}

	// 映射所有流
	for i := 0; i < len(files); i++ {
		args = append(args, "-map", fmt.Sprintf("%d", i))
	}

	// 根据格式设置编解码器
	hasSrt := false
	for _, file := range files {
		if strings.HasSuffix(file.FilePath, ".srt") {
			hasSrt = true
			break
		}
	}

	switch strings.ToUpper(muxFormat) {
	case "MP4":
		args = append(args, "-strict", "unofficial", "-c:a", "copy", "-c:v", "copy", "-c:s", "mov_text")
	case "TS":
		args = append(args, "-strict", "unofficial", "-c:a", "copy", "-c:v", "copy")
	case "MKV":
		if hasSrt {
			args = append(args, "-strict", "unofficial", "-c:a", "copy", "-c:v", "copy", "-c:s", "srt")
		} else {
			args = append(args, "-strict", "unofficial", "-c:a", "copy", "-c:v", "copy", "-c:s", "webvtt")
		}
	default:
		return fmt.Errorf("不支持的复用格式: %s", muxFormat)
	}

	// 清除metadata
	args = append(args, "-map_metadata", "-1")

	// 设置语言和标题 - 重要修复：参考C#版本第206-225行的streamIndex逻辑
	streamIndex := 0
	for _, file := range files {
		// 转换语言代码 - 参考C#版本第210行
		langCode := file.LangCode
		if langCode != "" {
			langCode = NormalizeLanguageCode(langCode)
		}
		if langCode == "" {
			langCode = "und"
		}
		args = append(args, fmt.Sprintf("-metadata:s:%d", streamIndex), fmt.Sprintf("language=%s", langCode))

		if file.Description != "" {
			args = append(args, fmt.Sprintf("-metadata:s:%d", streamIndex), fmt.Sprintf("title=%s", file.Description))
		}

		// 参照C#版本，根据mediainfo的流数量来增加streamIndex
		if len(file.Mediainfos) > 0 {
			streamIndex += len(file.Mediainfos)
		} else {
			streamIndex++
		}
	}

	// 设置默认轨道
	var hasVideo, hasAudio bool
	var audioCount int
	for _, file := range files {
		if file.MediaType == entity.MediaTypeVideo {
			hasVideo = true
		} else if file.MediaType == entity.MediaTypeAudio {
			hasAudio = true
			audioCount++
		}
	}

	if hasVideo {
		args = append(args, "-disposition:v:0", "default")
	}
	// 字幕都不设置默认
	args = append(args, "-disposition:s", "0")
	if hasAudio {
		// 音频只有第一个设置默认
		args = append(args, "-disposition:a:0", "default")
		for i := 1; i < audioCount; i++ {
			args = append(args, fmt.Sprintf("-disposition:a:%d", i), "0")
		}
	}

	if dateinfo {
		args = append(args, "-metadata", fmt.Sprintf("date=%s", dateString))
	}

	args = append(args, "-ignore_unknown", "-copy_unknown")

	// 设置输出文件扩展名
	var ext string
	switch strings.ToUpper(muxFormat) {
	case "MP4":
		ext = ".mp4"
	case "MKV":
		ext = ".mkv"
	case "TS":
		ext = ".ts"
	default:
		ext = ".mp4"
	}

	args = append(args, fmt.Sprintf("%s%s", outputPath, ext))

	// 执行FFmpeg命令
	return invokeFFmpeg(ffmpegPath, args, workingDir)
}

// MuxInputsByMkvmerge 使用mkvmerge复用多个输入文件 - 重要修复：添加缺失的方法
func MuxInputsByMkvmerge(mkvmergePath string, files []*OutputFile, outputPath string, workingDir string) error {
	if len(files) == 0 {
		return fmt.Errorf("没有文件需要复用")
	}

	// 构建参数数组 - 参考C#版本第254行
	var args []string
	args = append(args, "-q", "--output", fmt.Sprintf("%s.mkv", outputPath))

	// 添加无章节参数 - 参考C#版本第256行
	args = append(args, "--no-chapters")

	dFlag := false // 用于音频默认轨道标记

	// 添加语言和名称参数 - 参考C#版本第261-279行
	for _, file := range files {
		// 转换语言代码 - 参考C#版本第264行
		langCode := file.LangCode
		if langCode != "" {
			langCode = NormalizeLanguageCode(langCode)
		}
		if langCode == "" {
			langCode = "und"
		}
		args = append(args, "--language", fmt.Sprintf("0:%s", langCode))

		// 字幕都不设置默认 - 参考C#版本第267-268行
		if file.MediaType == entity.MediaTypeSubtitles || file.MediaType == entity.MediaTypeClosedCaptions {
			args = append(args, "--default-track", "0:no")
		}

		// 音频除了第一个音轨都不设置默认 - 参考C#版本第270-275行
		if file.MediaType == entity.MediaTypeAudio {
			if dFlag {
				args = append(args, "--default-track", "0:no")
			}
			dFlag = true
		}

		// 添加轨道名称 - 参考C#版本第276-277行
		description := file.Description
		if description == "" && langCode != "" && langCode != "und" {
			description = GetLanguageFromCode(langCode)
		}
		if description != "" {
			args = append(args, "--track-name", fmt.Sprintf("0:%s", description))
		}

		// 添加文件路径 - 参考C#版本第278行
		args = append(args, file.FilePath)
	}

	Logger.Debug(fmt.Sprintf("mkvmerge命令: %s %s", mkvmergePath, strings.Join(args, " ")))

	// 执行命令
	return invokeFFmpeg(mkvmergePath, args, workingDir)
}

// invokeFFmpeg 执行FFmpeg命令（修复版本 - 增强日志和错误报告）
func invokeFFmpeg(binary string, args []string, workingDir string) error {
	// 将要执行的命令提升到INFO级别，确保可见
	Logger.Debug(fmt.Sprintf("Executing FFmpeg: %s %s", binary, strings.Join(args, " ")))

	cmd := exec.Command(binary, args...)
	cmd.Dir = workingDir

	// 使用 bytes.Buffer 来捕获完整的 stderr 输出
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	// 启动命令
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// 等待命令完成
	err := cmd.Wait()
	if err != nil {
		// 当命令失败时，将完整的 stderr 输出附加到错误信息中
		// 这对于调试FFmpeg问题至关重要
		return fmt.Errorf("FFmpeg command failed with error: %w. Stderr: %s", err, stderrBuf.String())
	}

	// 成功时，以DEBUG级别打印输出（通常为空或仅包含版本信息）
	if stderrBuf.Len() > 0 {
		Logger.Debug("FFmpeg output: %s", stderrBuf.String())
	}

	Logger.Debug("FFmpeg command completed successfully.")
	return nil
}

// createConcatFile 在指定目录创建concat demuxer使用的文件列表文件
func createConcatFile(files []string, dir string) (string, error) {
	tempFile, err := os.CreateTemp(dir, "concat_*.txt")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	for _, file := range files {
		// 使用相对路径
		_, err = tempFile.WriteString(fmt.Sprintf("file '%s'\n", filepath.Base(file)))
		if err != nil {
			os.Remove(tempFile.Name())
			return "", err
		}
	}

	return tempFile.Name(), nil
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// GetMuxExtension 获取复用格式的扩展名
func GetMuxExtension(muxFormat string) string {
	switch strings.ToUpper(muxFormat) {
	case "MP4":
		return ".mp4"
	case "MKV":
		return ".mkv"
	case "TS":
		return ".ts"
	case "M4A":
		return ".m4a"
	case "EAC3":
		return ".eac3"
	case "AAC":
		return ".m4a"
	case "AC3":
		return ".ac3"
	case "FLV":
		return ".flv"
	default:
		return ".mp4"
	}
}
