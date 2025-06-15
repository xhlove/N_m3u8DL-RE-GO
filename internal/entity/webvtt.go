package entity

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// SubCue represents a single subtitle cue.
type SubCue struct {
	StartTime  time.Duration
	EndTime    time.Duration
	Payload    string
	Settings   string
	IsImage    bool
	DataBase64 string
}

// WebVttSub represents a WebVTT subtitle file.
type WebVttSub struct {
	MpegtsTimestamp int64
	Cues            []SubCue
}

// ToVtt converts the WebVttSub object to a VTT format string.
func (w *WebVttSub) ToVtt() string {
	var sb strings.Builder
	sb.WriteString("WEBVTT\n\n")
	for _, cue := range w.Cues {
		sb.WriteString(fmt.Sprintf("%s --> %s %s\n", formatVttTime(cue.StartTime), formatVttTime(cue.EndTime), cue.Settings))
		sb.WriteString(cue.Payload + "\n\n")
	}
	return sb.String()
}

// ToSrt converts the WebVTT subtitles to SRT format.
func (w *WebVttSub) ToSrt() string {
	var sb strings.Builder
	if len(w.Cues) == 0 {
		return "1\n00:00:00,000 --> 00:00:00,000\n\n"
	}
	for i, cue := range w.Cues {
		sb.WriteString(fmt.Sprintf("%d\n", i+1))
		sb.WriteString(fmt.Sprintf("%s --> %s\n", formatSrtTime(cue.StartTime), formatSrtTime(cue.EndTime)))
		sb.WriteString(cue.Payload + "\n\n")
	}
	return sb.String()
}

// AddCuesFromOne merges cues from another WebVttSub object.
func (w *WebVttSub) AddCuesFromOne(other *WebVttSub) {
	for _, cue := range other.Cues {
		newCue := cue
		// Adjust timestamp based on MPEGTS, similar to C# logic
		if w.MpegtsTimestamp != 0 {
			newCue.StartTime += time.Duration(other.MpegtsTimestamp-w.MpegtsTimestamp) * time.Second / 90000
			newCue.EndTime += time.Duration(other.MpegtsTimestamp-w.MpegtsTimestamp) * time.Second / 90000
		}
		w.Cues = append(w.Cues, newCue)
	}
}

// LeftShiftTime applies a time shift to all cues.
func (w *WebVttSub) LeftShiftTime(offset time.Duration) {
	if offset == 0 {
		return
	}
	for i := range w.Cues {
		w.Cues[i].StartTime -= offset
		w.Cues[i].EndTime -= offset
		if w.Cues[i].StartTime < 0 {
			w.Cues[i].StartTime = 0
		}
		if w.Cues[i].EndTime < 0 {
			w.Cues[i].EndTime = 0
		}
	}
}

// Parse parses a WebVTT string content into a WebVttSub object.
func Parse(content string) (*WebVttSub, error) {
	vtt := &WebVttSub{}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")

	// Regex to find timestamps and settings
	timeRegex := regexp.MustCompile(`(\d{2}:\d{2}:\d{2}\.\d{3})\s*-->\s*(\d{2}:\d{2}:\d{2}\.\d{3})(.*)`)

	var currentCue *SubCue
	inPayload := false

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if currentCue != nil {
				vtt.Cues = append(vtt.Cues, *currentCue)
				currentCue = nil
			}
			inPayload = false
			continue
		}

		if !inPayload {
			matches := timeRegex.FindStringSubmatch(line)
			if len(matches) > 3 {
				startTime, err1 := parseVttTime(matches[1])
				endTime, err2 := parseVttTime(matches[2])
				if err1 == nil && err2 == nil {
					currentCue = &SubCue{
						StartTime: startTime,
						EndTime:   endTime,
						Settings:  strings.TrimSpace(matches[3]),
					}
					inPayload = true
					continue
				}
			}
		}

		if inPayload && currentCue != nil {
			if currentCue.Payload != "" {
				currentCue.Payload += "\n"
			}
			currentCue.Payload += line
		}
	}

	// Add the last cue if it exists
	if currentCue != nil {
		vtt.Cues = append(vtt.Cues, *currentCue)
	}

	return vtt, nil
}

// formatSrtTime formats a time.Duration into the hh:mm:ss,ms format for SRT files.
func formatSrtTime(t time.Duration) string {
	return formatTime(t, ",", 3)
}

// formatVttTime formats a time.Duration into the hh:mm:ss.ms format for VTT files.
func formatVttTime(t time.Duration) string {
	return formatTime(t, ".", 3)
}

func formatTime(t time.Duration, separator string, precision int) string {
	totalSeconds := int64(t.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	milliseconds := int64(t.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", hours, minutes, seconds, separator, milliseconds)
}

func parseVttTime(s string) (time.Duration, error) {
	// VTT time format is hh:mm:ss.ms
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid time format: %s", s)
	}
	secParts := strings.Split(parts[2], ".")
	if len(secParts) != 2 {
		return 0, fmt.Errorf("invalid time format: %s", s)
	}
	return time.ParseDuration(fmt.Sprintf("%sh%sm%ss%sms", parts[0], parts[1], secParts[0], secParts[1]))
}
