package util

import (
	"N_m3u8DL-RE-GO/internal/entity"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
)

// TFHD (Track Fragment Header) box data
type TFHD struct {
	TrackID               uint32
	DefaultSampleDuration uint32
	DefaultSampleSize     uint32
}

// TRUN (Track Run) box data
type TRUN struct {
	SampleCount uint32
	SampleData  []Sample
}

// Sample data from a TRUN box
type Sample struct {
	SampleDuration              uint32
	SampleSize                  uint32
	SampleCompositionTimeOffset int32 // Can be signed
}

// ExtractVTTSubsFromMp4 extracts WebVTT subtitles from a binary merged MP4 file.
func ExtractVTTSubsFromMp4(filePath string) (*entity.WebVttSub, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read mp4 file for vtt: %w", err)
	}

	var timescale uint32
	var baseTime uint64
	var defaultDuration uint32
	var presentations []Sample
	var rawPayload []byte

	parser := NewMP4Parser().
		Box("moov", Children).
		Box("trak", Children).
		Box("mdia", Children).
		FullBox("mdhd", func(b *Box) {
			if *b.Version == 0 {
				b.Reader.Seek(4, io.SeekCurrent) // creation_time
				b.Reader.Seek(4, io.SeekCurrent) // modification_time
				timescale = binary.BigEndian.Uint32(readBytes(b.Reader, 4))
			} else { // version 1
				b.Reader.Seek(8, io.SeekCurrent) // creation_time
				b.Reader.Seek(8, io.SeekCurrent) // modification_time
				timescale = binary.BigEndian.Uint32(readBytes(b.Reader, 4))
			}
		}).
		Box("moof", Children).
		Box("traf", Children).
		FullBox("tfdt", func(b *Box) {
			if *b.Version == 1 {
				baseTime = binary.BigEndian.Uint64(readBytes(b.Reader, 8))
			} else {
				baseTime = uint64(binary.BigEndian.Uint32(readBytes(b.Reader, 4)))
			}
		}).
		FullBox("tfhd", func(b *Box) {
			tfhd := parseTFHD(b.Reader, *b.Flags)
			defaultDuration = tfhd.DefaultSampleDuration
		}).
		FullBox("trun", func(b *Box) {
			trun := parseTRUN(b.Reader, *b.Version, *b.Flags)
			presentations = trun.SampleData
		}).
		Box("mdat", AllData(func(data []byte) {
			rawPayload = data
		}))

	if err := parser.Parse(fileData); err != nil {
		return nil, fmt.Errorf("failed to parse mp4 for vtt: %w", err)
	}

	if timescale == 0 {
		return nil, fmt.Errorf("missing timescale for VTT content")
	}
	if rawPayload == nil {
		return nil, fmt.Errorf("missing mdat box for VTT content")
	}

	cues := []entity.SubCue{}
	currentTime := baseTime
	payloadReader := bytes.NewReader(rawPayload)

	for _, presentation := range presentations {
		duration := presentation.SampleDuration
		if duration == 0 {
			duration = defaultDuration
		}

		startTime := currentTime
		if presentation.SampleCompositionTimeOffset != 0 {
			startTime = baseTime + uint64(presentation.SampleCompositionTimeOffset)
		}

		endTime := startTime + uint64(duration)
		currentTime = endTime

		if payloadReader.Len() < int(presentation.SampleSize) {
			return nil, fmt.Errorf("not enough data in mdat for sample")
		}

		sampleData := make([]byte, presentation.SampleSize)
		payloadReader.Read(sampleData)

		cue := parseVTTC(sampleData, float64(startTime)/float64(timescale), float64(endTime)/float64(timescale))
		if cue != nil {
			cues = append(cues, *cue)
		}
	}

	return &entity.WebVttSub{Cues: cues}, nil
}

func parseTFHD(r *bytes.Reader, flags uint32) TFHD {
	tfhd := TFHD{}
	tfhd.TrackID = binary.BigEndian.Uint32(readBytes(r, 4))

	if flags&0x000001 != 0 { // base_data_offset_present
		r.Seek(8, io.SeekCurrent)
	}
	if flags&0x000002 != 0 { // sample_description_index_present
		r.Seek(4, io.SeekCurrent)
	}
	if flags&0x000008 != 0 { // default_sample_duration_present
		tfhd.DefaultSampleDuration = binary.BigEndian.Uint32(readBytes(r, 4))
	}
	if flags&0x000010 != 0 { // default_sample_size_present
		tfhd.DefaultSampleSize = binary.BigEndian.Uint32(readBytes(r, 4))
	}
	return tfhd
}

func parseTRUN(r *bytes.Reader, version uint8, flags uint32) TRUN {
	trun := TRUN{}
	trun.SampleCount = binary.BigEndian.Uint32(readBytes(r, 4))

	if flags&0x000001 != 0 { // data_offset_present
		r.Seek(4, io.SeekCurrent)
	}
	if flags&0x000004 != 0 { // first_sample_flags_present
		r.Seek(4, io.SeekCurrent)
	}

	for i := 0; i < int(trun.SampleCount); i++ {
		sample := Sample{}
		if flags&0x000100 != 0 { // sample_duration_present
			sample.SampleDuration = binary.BigEndian.Uint32(readBytes(r, 4))
		}
		if flags&0x000200 != 0 { // sample_size_present
			sample.SampleSize = binary.BigEndian.Uint32(readBytes(r, 4))
		}
		if flags&0x000400 != 0 { // sample_flags_present
			r.Seek(4, io.SeekCurrent)
		}
		if flags&0x000800 != 0 { // sample_composition_time_offset_present
			if version == 0 {
				sample.SampleCompositionTimeOffset = int32(binary.BigEndian.Uint32(readBytes(r, 4)))
			} else {
				sample.SampleCompositionTimeOffset = int32(binary.BigEndian.Uint32(readBytes(r, 4))) // Should be ReadInt32, but Go's binary doesn't have it directly
			}
		}
		trun.SampleData = append(trun.SampleData, sample)
	}
	return trun
}

func parseVTTC(data []byte, startTime, endTime float64) *entity.SubCue {
	var payload string
	var settings string

	parser := NewMP4Parser().
		Box("payl", AllData(func(d []byte) {
			payload = string(d)
		})).
		Box("sttg", AllData(func(d []byte) {
			settings = string(d)
		}))

	parser.Parse(data)

	if payload != "" {
		return &entity.SubCue{
			StartTime: time.Duration(startTime * float64(time.Second)),
			EndTime:   time.Duration(endTime * float64(time.Second)),
			Payload:   payload,
			Settings:  settings,
		}
	}
	return nil
}
