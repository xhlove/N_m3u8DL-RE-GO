package util

import (
	"N_m3u8DL-RE-GO/internal/entity"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TTML (Timed Text Markup Language) structure for parsing
type TT struct {
	XMLName xml.Name `xml:"tt"`
	Body    Body     `xml:"body"`
}

type Body struct {
	XMLName xml.Name `xml:"body"`
	Div     Div      `xml:"div"`
}

type Div struct {
	XMLName xml.Name `xml:"div"`
	Ps      []P      `xml:"p"`
}

type P struct {
	XMLName xml.Name `xml:"p"`
	Begin   string   `xml:"begin,attr"`
	End     string   `xml:"end,attr"`
	Content string   `xml:",innerxml"` // Use innerxml to capture HTML-like content
	Image   Image    `xml:"image"`
}

type Image struct {
	XMLName xml.Name `xml:"image"`
	Content string   `xml:",chardata"`
}

// ExtractTTMLSubsFromMp4s extracts TTML subtitles from a list of MP4 segment files.
func ExtractTTMLSubsFromMp4s(filePaths []string) (*entity.WebVttSub, error) {
	var xmls []string
	multiElementsFixRegex := regexp.MustCompile(`(?s)<tt.*?</tt>`)

	for _, filePath := range filePaths {
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			Logger.Warn("读取TTML MP4分片失败 %s: %v", filePath, err)
			continue
		}

		parser := NewMP4Parser().
			Box("mdat", AllData(func(data []byte) {
				// SplitMultipleRootElements logic from C#
				matches := multiElementsFixRegex.FindAllString(string(data), -1)
				if len(matches) > 0 {
					xmls = append(xmls, matches...)
				} else {
					xmls = append(xmls, string(data))
				}
			}))

		if err := parser.Parse(fileData); err != nil {
			Logger.Warn("解析TTML MP4分片失败 %s: %v", filePath, err)
		}
	}

	return extractSub(xmls)
}

// extractSub is the final parsing stage, similar to C#'s ExtractSub.
func extractSub(xmls []string) (*entity.WebVttSub, error) {
	finalVtt := &entity.WebVttSub{}
	imageRegex := regexp.MustCompile(`(?s)<smpte:image.*xml:id="(.*?)".*>([\s\S]*?)</smpte:image>`)

	for _, xmlContent := range xmls {
		if !strings.Contains(xmlContent, "<tt") {
			continue
		}

		var tt TT
		decoder := xml.NewDecoder(strings.NewReader(xmlContent))
		decoder.Strict = false
		decoder.AutoClose = xml.HTMLAutoClose
		decoder.Entity = xml.HTMLEntity

		if err := decoder.Decode(&tt); err != nil {
			Logger.Warn("解码TTML失败: %v", err)
			continue
		}

		imageDic := make(map[string]string)
		imageMatches := imageRegex.FindAllStringSubmatch(xmlContent, -1)
		for _, match := range imageMatches {
			imageDic[match[1]] = strings.TrimSpace(match[2])
		}

		for _, p := range tt.Body.Div.Ps {
			begin, err := parseTTMLTime(p.Begin)
			if err != nil {
				continue
			}
			end, err := parseTTMLTime(p.End)
			if err != nil {
				continue
			}

			var payload string
			var isImage bool
			var dataBase64 string

			bgImageAttr := ""
			if strings.Contains(p.Content, "smpte:backgroundImage") {
				re := regexp.MustCompile(`smpte:backgroundImage="#(.*?)"`)
				matches := re.FindStringSubmatch(p.Content)
				if len(matches) > 1 {
					bgImageAttr = matches[1]
				}
			}

			if bgImageAttr != "" {
				if base64Data, ok := imageDic[bgImageAttr]; ok {
					isImage = true
					dataBase64 = base64Data
					payload = "Base64::" + base64Data
				}
			} else {
				payload = cleanXMLContent(p.Content)
			}

			finalVtt.Cues = append(finalVtt.Cues, entity.SubCue{
				StartTime:  begin,
				EndTime:    end,
				Payload:    payload,
				IsImage:    isImage,
				DataBase64: dataBase64,
			})
		}
	}

	// Sort cues by start time
	for i := 0; i < len(finalVtt.Cues)-1; i++ {
		for j := i + 1; j < len(finalVtt.Cues); j++ {
			if finalVtt.Cues[i].StartTime > finalVtt.Cues[j].StartTime {
				finalVtt.Cues[i], finalVtt.Cues[j] = finalVtt.Cues[j], finalVtt.Cues[i]
			}
		}
	}

	return finalVtt, nil
}

// parseTTMLTime parses TTML time format (e.g., 00:00:05.088 or 5.088s)
func parseTTMLTime(t string) (time.Duration, error) {
	t = strings.TrimSpace(t)
	// Handle formats like "5.088s"
	if strings.HasSuffix(t, "s") {
		t = strings.TrimSuffix(t, "s")
		return time.ParseDuration(t + "s")
	}

	// Handle formats like "00:00:05.088"
	parts := strings.Split(t, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid time format: %s", t)
	}

	secParts := strings.Split(parts[2], ".")
	if len(secParts) > 2 {
		return 0, fmt.Errorf("invalid time format: %s", t)
	}

	// Reformat to something time.ParseDuration understands
	h := parts[0]
	m := parts[1]
	s := secParts[0]
	ms := "0"
	if len(secParts) == 2 {
		ms = secParts[1]
	}

	return time.ParseDuration(fmt.Sprintf("%sh%sm%ss%sms", h, m, s, ms))
}

// cleanXMLContent removes XML/HTML tags from a string.
func cleanXMLContent(raw string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return strings.TrimSpace(re.ReplaceAllString(raw, ""))
}

// ExtractFromTTML extracts subtitles from a raw TTML file.
func ExtractFromTTML(filePath string, mpegtsTimestamp int64) (*entity.WebVttSub, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ttml file: %w", err)
	}

	var tt TT
	decoder := xml.NewDecoder(bytes.NewReader(fileData))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity

	if err := decoder.Decode(&tt); err != nil {
		return nil, fmt.Errorf("failed to decode TTML: %w", err)
	}

	vtt := &entity.WebVttSub{
		MpegtsTimestamp: mpegtsTimestamp,
	}
	for _, p := range tt.Body.Div.Ps {
		begin, err := parseTTMLTime(p.Begin)
		if err != nil {
			Logger.Warn(fmt.Sprintf("invalid begin time format %s: %v", p.Begin, err))
			continue
		}
		end, err := parseTTMLTime(p.End)
		if err != nil {
			Logger.Warn(fmt.Sprintf("invalid end time format %s: %v", p.End, err))
			continue
		}

		cleanContent := cleanXMLContent(p.Content)
		isImage := p.Image.Content != ""
		payload := cleanContent
		if isImage {
			payload = "Base64::" + p.Image.Content
		}

		vtt.Cues = append(vtt.Cues, entity.SubCue{
			StartTime:  begin,
			EndTime:    end,
			Payload:    payload,
			IsImage:    isImage,
			DataBase64: p.Image.Content,
		})
	}

	return vtt, nil
}

// TryWriteImagePngsAsync decodes and writes base64 image cues to PNG files.
func TryWriteImagePngs(finalVtt *entity.WebVttSub, tmpDir string) error {
	hasImage := false
	for _, cue := range finalVtt.Cues {
		if cue.IsImage {
			hasImage = true
			break
		}
	}

	if finalVtt != nil && hasImage {
		Logger.WarnMarkUp("检测到图形字幕，正在处理...")
		i := 0
		for idx := range finalVtt.Cues {
			cue := &finalVtt.Cues[idx]
			if !cue.IsImage || !strings.HasPrefix(cue.Payload, "Base64::") {
				continue
			}

			name := fmt.Sprintf("%d.png", i)
			i++
			dest := filepath.Join(tmpDir, name)
			for FileExists(dest) {
				name = fmt.Sprintf("%d.png", i)
				i++
				dest = filepath.Join(tmpDir, name)
			}

			base64Data := strings.TrimPrefix(cue.Payload, "Base64::")
			pngData, err := base64.StdEncoding.DecodeString(base64Data)
			if err != nil {
				Logger.Warn(fmt.Sprintf("解码图形字幕失败 %s: %v", name, err))
				continue
			}

			if err := os.WriteFile(dest, pngData, 0644); err != nil {
				Logger.Warn(fmt.Sprintf("写入图形字幕失败 %s: %v", name, err))
				continue
			}
			// Update payload to the filename
			cue.Payload = name
		}
	}
	return nil
}
