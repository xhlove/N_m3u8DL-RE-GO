package util

import (
	"N_m3u8DL-RE-GO/internal/entity"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	trackEnabled   = 0x1
	trackInMovie   = 0x2
	trackInPreview = 0x4
	selfContained  = 0x1
)

// MSSMoovProcessor is used to generate a moov box for Microsoft Smooth Streaming media.
type MSSMoovProcessor struct {
	StreamSpec         *entity.StreamSpec
	TrackID            uint32
	FourCC             string
	CodecPrivateData   string
	Timescale          uint32
	Duration           uint64
	Language           string
	Width              uint16
	Height             uint16
	StreamType         string
	Channels           uint16
	BitsPerSample      uint16
	SamplingRate       uint32
	NalUnitLengthField int
	CreationTime       uint64
	IsProtection       bool
	ProtectionSystemID string
	ProtectionData     string
	ProtectionKID      string
}

// NewMSSMoovProcessor creates a new MSSMoovProcessor.
func NewMSSMoovProcessor(spec *entity.StreamSpec) (*MSSMoovProcessor, error) {
	if spec.MSSData == nil {
		return nil, fmt.Errorf("MSSData is missing from StreamSpec")
	}

	data := spec.MSSData
	width, height := 0, 0
	if spec.Resolution != "" {
		parts := strings.Split(spec.Resolution, "x")
		if len(parts) == 2 {
			fmt.Sscanf(parts[0], "%d", &width)
			fmt.Sscanf(parts[1], "%d", &height)
		}
	}

	p := &MSSMoovProcessor{
		StreamSpec:         spec,
		TrackID:            2, // Default track ID
		FourCC:             data.FourCC,
		CodecPrivateData:   data.CodecPrivateData,
		Timescale:          uint32(data.Timescale),
		Duration:           uint64(data.Duration),
		Language:           "und",
		Width:              uint16(width),
		Height:             uint16(height),
		StreamType:         data.Type,
		Channels:           uint16(data.Channels),
		BitsPerSample:      uint16(data.BitsPerSample),
		SamplingRate:       uint32(data.SamplingRate),
		NalUnitLengthField: data.NalUnitLengthField,
		CreationTime:       uint64(time.Now().Unix()),
		IsProtection:       data.IsProtection,
		ProtectionSystemID: data.ProtectionSystemID,
		ProtectionData:     data.ProtectionData,
	}

	if spec.Language != "" {
		p.Language = spec.Language
	}

	// Further initialization can be added here, like GenCodecPrivateDataForAAC and ExtractKID
	// For now, we'll keep it simple and add them in subsequent steps.

	return p, nil
}

// CanHandle checks if the given fourCC is supported by the MSSMoovProcessor.
func CanHandle(fourCC string) bool {
	supported := []string{"HVC1", "HEV1", "AACL", "AACH", "EC-3", "H264", "AVC1", "DAVC", "TTML", "DVHE", "DVH1"}
	for _, s := range supported {
		if s == fourCC {
			return true
		}
	}
	return false
}

// GenHeader generates the complete moov box header.
// This is the main entry point.
func (p *MSSMoovProcessor) GenHeader(firstSegment []byte) ([]byte, error) {
	// First, parse the first segment to get the trackId if necessary
	parser := NewMP4Parser().
		Box("moof", Children).
		Box("traf", Children).
		FullBox("tfhd", func(b *Box) {
			// Read track_ID
			if b.Reader.Len() >= 4 {
				p.TrackID = binary.BigEndian.Uint32(readBytes(b.Reader, 4))
			}
		})

	if err := parser.Parse(firstSegment); err != nil {
		Logger.Warn(fmt.Sprintf("Failed to parse first segment to get trackId: %v. Using default: %d", err, p.TrackID))
	}

	return p.genMoov(), nil
}

// genMoov creates the main 'moov' box
func (p *MSSMoovProcessor) genMoov() []byte {
	var moovPayload []byte
	moovPayload = append(moovPayload, p.genMvhd()...)
	moovPayload = append(moovPayload, p.genTrak()...)
	moovPayload = append(moovPayload, p.genMvex()...)
	// TODO: Add pssh box if protection is enabled
	return box("moov", moovPayload)
}

// genMvhd creates the 'mvhd' (Movie Header) box.
func (p *MSSMoovProcessor) genMvhd() []byte {
	buf := new(bytes.Buffer)
	// version 1
	binary.Write(buf, binary.BigEndian, uint64(p.CreationTime))
	binary.Write(buf, binary.BigEndian, uint64(p.CreationTime)) // modification_time
	binary.Write(buf, binary.BigEndian, uint32(p.Timescale))
	binary.Write(buf, binary.BigEndian, uint64(p.Duration))
	binary.Write(buf, binary.BigEndian, uint32(0x00010000)) // rate
	binary.Write(buf, binary.BigEndian, uint16(0x0100))     // volume
	buf.Write(make([]byte, 10))                             // reserved
	buf.Write(p.getUnityMatrix())
	buf.Write(make([]byte, 24))                             // pre_defined
	binary.Write(buf, binary.BigEndian, uint32(0xFFFFFFFF)) // next_track_ID
	return fullBox("mvhd", 1, 0, buf.Bytes())
}

// genTrak creates the 'trak' (Track) box.
func (p *MSSMoovProcessor) genTrak() []byte {
	var trakPayload []byte
	trakPayload = append(trakPayload, p.genTkhd()...)
	trakPayload = append(trakPayload, p.genMdia()...)
	return box("trak", trakPayload)
}

// genTkhd creates the 'tkhd' (Track Header) box.
func (p *MSSMoovProcessor) genTkhd() []byte {
	buf := new(bytes.Buffer)
	flags := uint32(trackEnabled | trackInMovie | trackInPreview)
	// version 1
	binary.Write(buf, binary.BigEndian, uint64(p.CreationTime))
	binary.Write(buf, binary.BigEndian, uint64(p.CreationTime))
	binary.Write(buf, binary.BigEndian, uint32(p.TrackID))
	buf.Write(make([]byte, 4)) // reserved
	binary.Write(buf, binary.BigEndian, uint64(p.Duration))
	buf.Write(make([]byte, 8))                     // reserved
	binary.Write(buf, binary.BigEndian, uint16(0)) // layer
	binary.Write(buf, binary.BigEndian, uint16(0)) // alternate_group
	if p.StreamType == "audio" {
		binary.Write(buf, binary.BigEndian, uint16(0x0100)) // volume
	} else {
		binary.Write(buf, binary.BigEndian, uint16(0))
	}
	buf.Write(make([]byte, 2)) // reserved
	buf.Write(p.getUnityMatrix())
	binary.Write(buf, binary.BigEndian, uint32(uint32(p.Width)<<16))
	binary.Write(buf, binary.BigEndian, uint32(uint32(p.Height)<<16))
	return fullBox("tkhd", 1, flags, buf.Bytes())
}

// genMdia creates the 'mdia' (Media) box.
func (p *MSSMoovProcessor) genMdia() []byte {
	var mdiaPayload []byte
	mdiaPayload = append(mdiaPayload, p.genMdhd()...)
	mdiaPayload = append(mdiaPayload, p.genHdlr()...)
	mdiaPayload = append(mdiaPayload, p.genMinf()...)
	return box("mdia", mdiaPayload)
}

// genMdhd creates the 'mdhd' (Media Header) box.
func (p *MSSMoovProcessor) genMdhd() []byte {
	buf := new(bytes.Buffer)
	// version 1
	binary.Write(buf, binary.BigEndian, uint64(p.CreationTime))
	binary.Write(buf, binary.BigEndian, uint64(p.CreationTime))
	binary.Write(buf, binary.BigEndian, uint32(p.Timescale))
	binary.Write(buf, binary.BigEndian, uint64(p.Duration))
	// language
	langBytes := []byte(p.Language)
	packedLang := (uint16(langBytes[0]-0x60) << 10) | (uint16(langBytes[1]-0x60) << 5) | uint16(langBytes[2]-0x60)
	binary.Write(buf, binary.BigEndian, packedLang)
	binary.Write(buf, binary.BigEndian, uint16(0)) // pre_defined
	return fullBox("mdhd", 1, 0, buf.Bytes())
}

// genHdlr creates the 'hdlr' (Handler Reference) box.
func (p *MSSMoovProcessor) genHdlr() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(0)) // pre_defined
	switch p.StreamType {
	case "audio":
		buf.WriteString("soun")
	case "video":
		buf.WriteString("vide")
	case "text":
		buf.WriteString("subt")
	}
	buf.Write(make([]byte, 12)) // reserved
	buf.WriteString("RE Handler\x00")
	return fullBox("hdlr", 0, 0, buf.Bytes())
}

// genMinf creates the 'minf' (Media Information) box.
func (p *MSSMoovProcessor) genMinf() []byte {
	var minfPayload []byte
	switch p.StreamType {
	case "audio":
		minfPayload = append(minfPayload, fullBox("smhd", 0, 0, make([]byte, 4))...)
	case "video":
		minfPayload = append(minfPayload, fullBox("vmhd", 0, 1, make([]byte, 8))...)
	case "text":
		minfPayload = append(minfPayload, fullBox("sthd", 0, 0, []byte{})...)
	}
	minfPayload = append(minfPayload, p.genDinf()...)
	minfPayload = append(minfPayload, p.genStbl()...)
	return box("minf", minfPayload)
}

// genDinf creates the 'dinf' (Data Information) box.
func (p *MSSMoovProcessor) genDinf() []byte {
	drefPayload := fullBox("dref", 0, 0, append([]byte{0, 0, 0, 1}, fullBox("url ", 0, selfContained, []byte{})...))
	return box("dinf", drefPayload)
}

// genStbl creates the 'stbl' (Sample Table) box.
func (p *MSSMoovProcessor) genStbl() []byte {
	var stblPayload []byte
	stblPayload = append(stblPayload, p.genStsd()...)
	stblPayload = append(stblPayload, fullBox("stts", 0, 0, make([]byte, 4))...)
	stblPayload = append(stblPayload, fullBox("stsc", 0, 0, make([]byte, 4))...)
	stblPayload = append(stblPayload, fullBox("stsz", 0, 0, make([]byte, 8))...)
	stblPayload = append(stblPayload, fullBox("stco", 0, 0, make([]byte, 4))...)
	return box("stbl", stblPayload)
}

// genStsd creates the 'stsd' (Sample Description) box.
func (p *MSSMoovProcessor) genStsd() []byte {
	sampleEntryBox, err := p.getSampleEntryBox()
	if err != nil {
		// In a real implementation, handle this error properly
		return []byte{}
	}
	stsdPayload := append([]byte{0, 0, 0, 1}, sampleEntryBox...)
	return fullBox("stsd", 0, 0, stsdPayload)
}

// getSampleEntryBox creates the appropriate sample entry box (e.g., 'mp4a', 'avc1').
func (p *MSSMoovProcessor) getSampleEntryBox() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.Write(make([]byte, 6))                     // reserved
	binary.Write(buf, binary.BigEndian, uint16(1)) // data_reference_index

	switch p.StreamType {
	case "audio":
		binary.Write(buf, binary.BigEndian, uint32(0)) // reserved
		binary.Write(buf, binary.BigEndian, uint32(0))
		binary.Write(buf, binary.BigEndian, p.Channels)
		binary.Write(buf, binary.BigEndian, p.BitsPerSample)
		binary.Write(buf, binary.BigEndian, uint16(0)) // pre_defined
		binary.Write(buf, binary.BigEndian, uint16(0)) // reserved
		binary.Write(buf, binary.BigEndian, uint32(p.SamplingRate)<<16)

		codecPrivateBytes, err := hex.DecodeString(p.CodecPrivateData)
		if err != nil {
			return nil, fmt.Errorf("invalid codec private data: %w", err)
		}
		buf.Write(p.genEsds(codecPrivateBytes))

		// TODO: Handle protection ('enca')
		return box("mp4a", buf.Bytes()), nil

	case "video":
		buf.Write(make([]byte, 16)) // pre_defined, reserved
		binary.Write(buf, binary.BigEndian, p.Width)
		binary.Write(buf, binary.BigEndian, p.Height)
		binary.Write(buf, binary.BigEndian, uint32(0x00480000)) // horizresolution
		binary.Write(buf, binary.BigEndian, uint32(0x00480000)) // vertresolution
		binary.Write(buf, binary.BigEndian, uint32(0))          // reserved
		binary.Write(buf, binary.BigEndian, uint16(1))          // frame_count
		buf.Write(make([]byte, 32))                             // compressorname
		binary.Write(buf, binary.BigEndian, uint16(0x0018))     // depth
		binary.Write(buf, binary.BigEndian, int16(-1))          // pre_defined

		if p.FourCC == "H264" || p.FourCC == "AVC1" {
			// TODO: Implement GetAvcC
			// buf.Write(p.getAvcC())
			return box("avc1", buf.Bytes()), nil
		}
		if p.FourCC == "HVC1" || p.FourCC == "HEV1" {
			// TODO: Implement GetHvcC
			// buf.Write(p.getHvcC())
			return box("hvc1", buf.Bytes()), nil
		}
		// TODO: Handle protection ('encv')
		return nil, fmt.Errorf("unsupported video fourCC: %s", p.FourCC)

	case "text":
		// TODO: Implement TTML ('stpp') entry
		return nil, fmt.Errorf("text stream type not yet implemented")

	default:
		return nil, fmt.Errorf("unsupported stream type: %s", p.StreamType)
	}
}

// genEsds creates the 'esds' (Elementary Stream Descriptor) box for audio.
func (p *MSSMoovProcessor) genEsds(audioSpecificConfig []byte) []byte {
	// Simplified version for now
	esdsPayload := new(bytes.Buffer)
	// ES_Descriptor
	esdsPayload.WriteByte(0x03)                                // tag
	esdsPayload.WriteByte(byte(20 + len(audioSpecificConfig))) // size
	binary.Write(esdsPayload, binary.BigEndian, uint16(p.TrackID))
	esdsPayload.WriteByte(0x00) // flags
	// DecoderConfigDescriptor
	esdsPayload.WriteByte(0x04)                                // tag
	esdsPayload.WriteByte(byte(15 + len(audioSpecificConfig))) // size
	esdsPayload.WriteByte(0x40)                                // objectTypeIndication (MPEG-4 AAC)
	esdsPayload.WriteByte(0x15)                                // streamType (AudioStream)
	esdsPayload.Write(make([]byte, 3))                         // bufferSizeDB
	if p.StreamSpec.Bandwidth != nil {
		binary.Write(esdsPayload, binary.BigEndian, uint32(*p.StreamSpec.Bandwidth)) // maxBitrate
		binary.Write(esdsPayload, binary.BigEndian, uint32(*p.StreamSpec.Bandwidth)) // avgBitrate
	} else {
		binary.Write(esdsPayload, binary.BigEndian, uint32(0)) // maxBitrate
		binary.Write(esdsPayload, binary.BigEndian, uint32(0)) // avgBitrate
	}
	// DecoderSpecificInfo
	esdsPayload.WriteByte(0x05) // tag
	esdsPayload.WriteByte(byte(len(audioSpecificConfig)))
	esdsPayload.Write(audioSpecificConfig)
	return fullBox("esds", 0, 0, esdsPayload.Bytes())
}

// genMvex creates the 'mvex' (Movie Extends) box.
func (p *MSSMoovProcessor) genMvex() []byte {
	var mvexPayload []byte
	mvexPayload = append(mvexPayload, p.genMehd()...)
	mvexPayload = append(mvexPayload, p.genTrex()...)
	return box("mvex", mvexPayload)
}

// genMehd creates the 'mehd' (Movie Extends Header) box.
func (p *MSSMoovProcessor) genMehd() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, p.Duration)
	return fullBox("mehd", 1, 0, buf.Bytes())
}

// genTrex creates the 'trex' (Track Extends) box.
func (p *MSSMoovProcessor) genTrex() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, p.TrackID)
	binary.Write(buf, binary.BigEndian, uint32(1)) // default_sample_description_index
	binary.Write(buf, binary.BigEndian, uint32(0)) // default_sample_duration
	binary.Write(buf, binary.BigEndian, uint32(0)) // default_sample_size
	binary.Write(buf, binary.BigEndian, uint32(0)) // default_sample_flags
	return fullBox("trex", 0, 0, buf.Bytes())
}

// getUnityMatrix returns the 3x3 unity matrix for video tracks.
func (p *MSSMoovProcessor) getUnityMatrix() []byte {
	return []byte{
		0, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 1, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 64, 0, 0, 0,
	}
}

// box is a helper to construct a basic MP4 box.
func box(boxType string, payload []byte) []byte {
	size := uint32(8 + len(payload))
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, size)
	buf.WriteString(boxType)
	buf.Write(payload)
	return buf.Bytes()
}

// fullBox is a helper to construct a full MP4 box (with version and flags).
func fullBox(boxType string, version uint8, flags uint32, payload []byte) []byte {
	versionAndFlags := uint32(version)<<24 | flags
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, versionAndFlags)
	buf.Write(payload)
	return box(boxType, buf.Bytes())
}
