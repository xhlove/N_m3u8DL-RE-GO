package util

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MediaInfo 媒体信息结构
type MediaInfo struct {
	ID          string
	Type        string
	Text        string
	Format      string
	FormatInfo  string
	BaseInfo    string
	Resolution  string
	Bitrate     string
	Fps         string
	HDR         bool
	DolbyVision bool
	StartTime   *time.Duration
}

// ToString 转换为字符串显示
func (m *MediaInfo) ToString() string {
	var parts []string

	if m.ID != "" {
		parts = append(parts, fmt.Sprintf("[%s]", m.ID))
	}

	if m.Type != "" {
		parts = append(parts, fmt.Sprintf("%s", strings.Title(m.Type)))
	}

	if m.BaseInfo != "" {
		parts = append(parts, m.BaseInfo)
	}

	if m.Resolution != "" {
		parts = append(parts, m.Resolution)
	}

	if m.Bitrate != "" {
		parts = append(parts, m.Bitrate)
	}

	if m.Fps != "" {
		parts = append(parts, m.Fps)
	}

	if m.HDR {
		parts = append(parts, "HDR")
	}

	if m.DolbyVision {
		parts = append(parts, "DolbyVision")
	}

	return strings.Join(parts, ", ")
}

// GetMediaInfo 读取媒体信息
func GetMediaInfo(ffmpegPath, filePath string) ([]*MediaInfo, error) {
	var result []*MediaInfo

	if filePath == "" || !FileExists(filePath) {
		return result, fmt.Errorf("文件不存在: %s", filePath)
	}

	// 执行FFmpeg命令获取媒体信息
	cmd := exec.Command(ffmpegPath, "-hide_banner", "-i", filePath)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return result, err
	}

	if err := cmd.Start(); err != nil {
		return result, err
	}

	// 读取stderr输出（FFmpeg将信息输出到stderr）
	var output strings.Builder
	buf := make([]byte, 1024)
	for {
		n, err := stderr.Read(buf)
		if n > 0 {
			output.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	cmd.Wait()

	// 解析输出
	return parseFFmpegOutput(output.String()), nil
}

// parseFFmpegOutput 解析FFmpeg输出
func parseFFmpegOutput(output string) []*MediaInfo {
	var result []*MediaInfo

	// 正则表达式模式
	streamRegex := regexp.MustCompile(`  Stream #.*`)
	idRegex := regexp.MustCompile(`#0:\d(\[0x\w+?\])`)
	typeRegex := regexp.MustCompile(`: (\w+): (.*)`)
	baseInfoRegex := regexp.MustCompile(`(.*?)(,|$)`)
	replaceRegex := regexp.MustCompile(` \/ 0x\w+`)
	resRegex := regexp.MustCompile(`\d{2,}x\d+`)
	bitrateRegex := regexp.MustCompile(`\d+ kb\/s`)
	fpsRegex := regexp.MustCompile(`(\d+(\.\d+)?) fps`)
	doViRegex := regexp.MustCompile(`DOVI configuration record.*profile: (\d).*compatibility id: (\d)`)
	startRegex := regexp.MustCompile(`Duration.*?start: (\d+\.?\d{0,3})`)

	// 查找所有流信息
	streams := streamRegex.FindAllString(output, -1)

	for _, stream := range streams {
		info := &MediaInfo{}

		// 提取ID
		if match := idRegex.FindStringSubmatch(stream); len(match) > 1 {
			info.ID = match[1]
		}

		// 提取类型和文本
		if match := typeRegex.FindStringSubmatch(stream); len(match) > 2 {
			info.Type = match[1]
			info.Text = strings.TrimSpace(match[2])
		}

		// 提取基础信息
		if match := baseInfoRegex.FindStringSubmatch(info.Text); len(match) > 1 {
			info.BaseInfo = match[1]
			info.BaseInfo = replaceRegex.ReplaceAllString(info.BaseInfo, "")
			// hevc (Main 10), yuv420p10le(tv, bt2020nc/bt2020/smpte2084)
			// aac (LC), 48000 Hz, stereo, fltp, 131 kb/s
			parts := strings.Split(info.BaseInfo, "(")
			info.Format = strings.TrimSpace(parts[0])
			if len(parts) > 1 {
				info.FormatInfo = strings.TrimSpace(strings.TrimRight(parts[1], ")"))
			}
		}

		// 提取分辨率
		if match := resRegex.FindString(info.Text); match != "" {
			info.Resolution = match
		}

		// 提取码率
		if match := bitrateRegex.FindString(info.Text); match != "" {
			info.Bitrate = match
		}

		// 提取帧率
		if match := fpsRegex.FindString(info.Text); match != "" {
			info.Fps = match
		}

		// 检查HDR
		info.HDR = strings.Contains(info.Text, "/bt2020/")

		// 检查DolbyVision
		info.DolbyVision = strings.Contains(info.BaseInfo, "dvhe") ||
			strings.Contains(info.BaseInfo, "dvh1") ||
			strings.Contains(info.BaseInfo, "DOVI") ||
			strings.Contains(info.Type, "dvvideo") ||
			(doViRegex.MatchString(output) && info.Type == "Video")

		// 提取开始时间
		if match := startRegex.FindStringSubmatch(output); len(match) > 1 {
			if f, err := strconv.ParseFloat(match[1], 64); err == nil {
				duration := time.Duration(f * float64(time.Second))
				info.StartTime = &duration
			}
		}

		result = append(result, info)
	}

	// 如果没有找到任何流信息，添加一个未知类型
	if len(result) == 0 {
		result = append(result, &MediaInfo{
			Type: "Unknown",
		})
	}

	return result
}
