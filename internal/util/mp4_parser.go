package util

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// ParsedMP4Info holds information extracted from an MP4 file.
type ParsedMP4Info struct {
	KID        string
	IsMultiDRM bool
}

// Box represents a parsed MP4 box.
type Box struct {
	Name       string
	Size       uint64
	Version    *uint8
	Flags      *uint32
	Reader     *bytes.Reader
	Children   []*Box
	parser     *MP4Parser
	headerSize uint64
	isFullBox  bool
	absStart   uint64
}

// MP4Parser is a parser for MP4 file structures (boxes).
type MP4Parser struct {
	boxDefs map[string]func(*Box)
}

// NewMP4Parser creates a new MP4Parser.
func NewMP4Parser() *MP4Parser {
	return &MP4Parser{
		boxDefs: make(map[string]func(*Box)),
	}
}

// Box defines a handler for a basic box type.
func (p *MP4Parser) Box(name string, handler func(*Box)) *MP4Parser {
	p.boxDefs[name] = handler
	return p
}

// FullBox defines a handler for a full box type (with version and flags).
func (p *MP4Parser) FullBox(name string, handler func(*Box)) *MP4Parser {
	p.boxDefs[name] = handler
	return p
}

// Parse starts parsing the data from the reader.
func (p *MP4Parser) Parse(data []byte) error {
	reader := bytes.NewReader(data)
	for reader.Len() > 0 {
		if err := p.parseNext(reader, 0); err != nil {
			if err == io.EOF {
				return nil // Clean EOF is not an error
			}
			return err
		}
	}
	return nil
}

func (p *MP4Parser) parseNext(reader *bytes.Reader, absStart uint64) error {
	startPos := uint64(reader.Len())
	if startPos < 8 {
		return io.EOF
	}

	header := make([]byte, 8)
	if _, err := io.ReadFull(reader, header); err != nil {
		return err
	}

	size := uint64(binary.BigEndian.Uint32(header[0:4]))
	name := string(header[4:8])
	headerSize := uint64(8)

	if size == 1 {
		// 64-bit size
		if reader.Len() < 8 {
			return io.ErrUnexpectedEOF
		}
		largeSizeHeader := make([]byte, 8)
		if _, err := io.ReadFull(reader, largeSizeHeader); err != nil {
			return err
		}
		size = binary.BigEndian.Uint64(largeSizeHeader)
		headerSize += 8
	} else if size == 0 {
		// To the end of the file
		size = uint64(reader.Len()) + headerSize
	}

	box := &Box{
		Name:       name,
		Size:       size,
		parser:     p,
		headerSize: headerSize,
		absStart:   absStart + (startPos - uint64(reader.Len())),
	}

	handler, ok := p.boxDefs[name]
	if !ok {
		// Skip unknown boxes
		payloadSize := int64(size - headerSize)
		if payloadSize < 0 {
			return fmt.Errorf("invalid payload size for box %s", name)
		}
		if _, err := reader.Seek(payloadSize, io.SeekCurrent); err != nil {
			return fmt.Errorf("failed to seek past box %s: %w", name, err)
		}
		return nil
	}

	// Check if it's a full box
	if p.isFullBox(name) {
		box.isFullBox = true
		if reader.Len() < 4 {
			return io.ErrUnexpectedEOF
		}
		versionAndFlags := make([]byte, 4)
		if _, err := io.ReadFull(reader, versionAndFlags); err != nil {
			return err
		}
		v := versionAndFlags[0]
		f := binary.BigEndian.Uint32(append([]byte{0}, versionAndFlags[1:]...))
		box.Version = &v
		box.Flags = &f
		box.headerSize += 4
	}

	payloadSize := int64(size - box.headerSize)
	if payloadSize < 0 {
		return fmt.Errorf("invalid payload size for box %s", name)
	}

	payload := make([]byte, payloadSize)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return fmt.Errorf("failed to read payload for box %s: %w", name, err)
	}
	box.Reader = bytes.NewReader(payload)

	handler(box)
	return nil
}

// Children is a handler that parses the children of a box.
func Children(box *Box) {
	for box.Reader.Len() > 0 {
		box.parser.parseNext(box.Reader, box.absStart+box.headerSize)
	}
}

// SampleDescription is a handler for stsd boxes.
func SampleDescription(box *Box) {
	if box.Reader.Len() < 4 {
		return
	}
	count := binary.BigEndian.Uint32(readBytes(box.Reader, 4))
	for i := 0; i < int(count); i++ {
		if box.Reader.Len() == 0 {
			break
		}
		box.parser.parseNext(box.Reader, box.absStart+box.headerSize)
	}
}

// AllData is a handler that passes the entire payload to a callback.
func AllData(handler func([]byte)) func(*Box) {
	return func(box *Box) {
		payload := make([]byte, box.Reader.Len())
		box.Reader.Read(payload)
		handler(payload)
	}
}

func (p *MP4Parser) isFullBox(name string) bool {
	// This is a simplification. In a real scenario, you'd have a list
	// of known full boxes. For this use case, we'll define them as needed.
	fullBoxes := map[string]bool{
		"mdhd": true, "tfdt": true, "tfhd": true, "trun": true, "stsd": true, "pssh": true,
	}
	return fullBoxes[name]
}

func readBytes(r *bytes.Reader, n int) []byte {
	b := make([]byte, n)
	r.Read(b)
	return b
}

// GetMP4Info parses an MP4 file and returns extracted information.
func GetMP4Info(filePath string) (*ParsedMP4Info, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	info := &ParsedMP4Info{}
	var psshData [][]byte

	parser := NewMP4Parser().
		Box("moov", Children).
		Box("trak", Children).
		Box("mdia", Children).
		Box("minf", Children).
		Box("stbl", Children).
		Box("stsd", SampleDescription).
		Box("encv", Children).
		Box("enca", Children).
		Box("sinf", Children).
		Box("frma", AllData(nil)).
		Box("schm", AllData(nil)).
		Box("schi", Children).
		Box("tenc", AllData(nil)).
		FullBox("pssh", func(b *Box) {
			payload := make([]byte, b.Reader.Len())
			b.Reader.Read(payload)
			psshData = append(psshData, payload)
		})

	if err := parser.Parse(data); err != nil {
		return nil, err
	}

	// Widevine PSSH box
	for _, pssh := range psshData {
		if len(pssh) > 32 && (bytes.Equal(pssh[12:28], []byte{0xed, 0xef, 0x8b, 0xa9, 0x79, 0xd6, 0x4a, 0xce, 0xa3, 0xc8, 0x27, 0xdc, 0xd5, 0x1d, 0x21, 0xed})) {
			info.KID = fmt.Sprintf("%x", pssh[len(pssh)-16:])
			break
		}
	}

	return info, nil
}
