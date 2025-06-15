package util

import (
	"N_m3u8DL-RE-GO/internal/entity"
	"net/http"
	"strconv"
)

const (
	// 10MB per clip
	perSize = 10 * 1024 * 1024
)

// SplitUrlAsync attempts to split a large single file URL into multiple segments for parallel download.
func SplitUrlAsync(segment *entity.MediaSegment, headers map[string]string) ([]*entity.MediaSegment, error) {
	url := segment.URL
	canSplit, err := canSplitAsync(url, headers)
	if err != nil || !canSplit {
		return nil, err
	}

	// If the segment already has a range, we don't split it further.
	if segment.StartRange != nil {
		return nil, nil
	}

	fileSize, err := getFileSizeAsync(url, headers)
	if err != nil || fileSize == 0 {
		return nil, err
	}

	clips := getAllClips(fileSize)
	splitSegments := make([]*entity.MediaSegment, 0, len(clips))
	for _, clip := range clips {
		var expectLength *int64
		if clip.To != -1 {
			val := clip.To - clip.From + 1
			expectLength = &val
		}
		splitSegments = append(splitSegments, &entity.MediaSegment{
			Index:        int64(clip.Index),
			URL:          url,
			StartRange:   &clip.From,
			ExpectLength: expectLength,
			EncryptInfo:  segment.EncryptInfo,
		})
	}

	return splitSegments, nil
}

// canSplitAsync checks if the server supports range requests.
func canSplitAsync(url string, headers map[string]string) (bool, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.Header.Get("Accept-Ranges") != "", nil
}

// getFileSizeAsync gets the content length of the URL.
func getFileSizeAsync(url string, headers map[string]string) (int64, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	contentLength := resp.Header.Get("Content-Length")
	if contentLength == "" {
		return 0, nil
	}

	return strconv.ParseInt(contentLength, 10, 64)
}

type clip struct {
	Index int
	From  int64
	To    int64
}

// getAllClips creates logical clips from a file size.
func getAllClips(fileSize int64) []clip {
	var clips []clip
	var index int
	var counter int64

	for fileSize > 0 {
		c := clip{
			Index: index,
			From:  counter,
			To:    counter + perSize,
		}

		if fileSize-perSize > 0 {
			fileSize -= perSize
			counter += perSize + 1
			index++
			clips = append(clips, c)
		} else {
			c.To = -1 // Download to the end
			clips = append(clips, c)
			break
		}
	}
	return clips
}
