package media

import (
	"encoding/binary"
	"fmt"
)

const (
	maxPostMotionDurationMS = uint32(60000)
	maxPostMotionEdge       = uint32(8192)
)

type mp4Inspection struct {
	Width      uint32
	Height     uint32
	DurationMS uint32
}

// inspectMP4 performs a bounded structural inspection. It deliberately reads
// no external metadata and never trusts client supplied dimensions or duration.
// The parser only accepts a normal ISO BMFF file with ftyp, moov, mdat, mvhd
// and one usable video track header.
func inspectMP4(raw []byte) (mp4Inspection, error) {
	if len(raw) < 16 {
		return mp4Inspection{}, fmt.Errorf("%w: mp4 is too short", ErrCompletionValidation)
	}
	top, err := parseMP4Boxes(raw)
	if err != nil {
		return mp4Inspection{}, fmt.Errorf("%w: invalid mp4 box", ErrCompletionValidation)
	}
	var ftyp, moov, mdat *mp4Box
	for index := range top {
		switch top[index].kind {
		case "ftyp":
			ftyp = &top[index]
		case "moov":
			moov = &top[index]
		case "mdat":
			mdat = &top[index]
		}
	}
	if ftyp == nil || !validMP4Brands(ftyp.payload) || moov == nil || mdat == nil {
		return mp4Inspection{}, fmt.Errorf("%w: missing mp4 structure", ErrCompletionValidation)
	}
	mvhd := findMP4Box(*moov, "mvhd")
	if mvhd == nil {
		return mp4Inspection{}, fmt.Errorf("%w: missing mp4 duration", ErrCompletionValidation)
	}
	durationMS, err := parseMP4Duration(mvhd.payload)
	if err != nil || durationMS == 0 || durationMS > maxPostMotionDurationMS {
		return mp4Inspection{}, fmt.Errorf("%w: mp4 duration exceeds 60 seconds", ErrCompletionValidation)
	}
	tkhd := findVideoTrackHeader(*moov)
	if tkhd == nil {
		return mp4Inspection{}, fmt.Errorf("%w: missing mp4 dimensions", ErrCompletionValidation)
	}
	width, height, err := parseMP4Dimensions(tkhd.payload)
	if err != nil || width == 0 || height == 0 || width > maxPostMotionEdge || height > maxPostMotionEdge {
		return mp4Inspection{}, fmt.Errorf("%w: mp4 dimensions are invalid", ErrCompletionValidation)
	}
	return mp4Inspection{Width: width, Height: height, DurationMS: durationMS}, nil
}

type mp4Box struct {
	kind     string
	payload  []byte
	children []mp4Box
}

func parseMP4Boxes(raw []byte) ([]mp4Box, error) {
	return parseMP4BoxesAtDepth(raw, 0)
}

func parseMP4BoxesAtDepth(raw []byte, depth int) ([]mp4Box, error) {
	if depth > 6 {
		return nil, fmt.Errorf("nested mp4 too deep")
	}
	boxes := make([]mp4Box, 0, 8)
	for offset := 0; offset < len(raw); {
		if len(raw)-offset < 8 {
			return nil, fmt.Errorf("truncated box")
		}
		size := uint64(binary.BigEndian.Uint32(raw[offset : offset+4]))
		header := 8
		if size == 1 {
			if len(raw)-offset < 16 {
				return nil, fmt.Errorf("truncated large box")
			}
			size = binary.BigEndian.Uint64(raw[offset+8 : offset+16])
			header = 16
		} else if size == 0 {
			size = uint64(len(raw) - offset)
		}
		if size < uint64(header) || size > uint64(len(raw)-offset) {
			return nil, fmt.Errorf("invalid box size")
		}
		kind := string(raw[offset+4 : offset+8])
		payload := raw[offset+header : offset+int(size)]
		box := mp4Box{kind: kind, payload: payload}
		if kind == "moov" || kind == "trak" || kind == "mdia" || kind == "minf" || kind == "stbl" || kind == "edts" {
			children, childErr := parseMP4BoxesAtDepth(payload, depth+1)
			if childErr != nil {
				return nil, childErr
			}
			box.children = children
		}
		boxes = append(boxes, box)
		offset += int(size)
	}
	return boxes, nil
}

func findMP4Box(box mp4Box, kind string) *mp4Box {
	for index := range box.children {
		if box.children[index].kind == kind {
			return &box.children[index]
		}
		if nested := findMP4Box(box.children[index], kind); nested != nil {
			return nested
		}
	}
	return nil
}

func validMP4Brands(payload []byte) bool {
	if len(payload) < 8 {
		return false
	}
	brands := make([]string, 0, (len(payload)-8)/4+1)
	brands = append(brands, string(payload[0:4]))
	for offset := 8; offset+4 <= len(payload); offset += 4 {
		brands = append(brands, string(payload[offset:offset+4]))
	}
	for _, brand := range brands {
		switch brand {
		case "isom", "iso2", "iso5", "iso6", "mp41", "mp42", "avc1", "M4V ":
			return true
		}
	}
	return false
}

func parseMP4Duration(payload []byte) (uint32, error) {
	if len(payload) < 20 {
		return 0, fmt.Errorf("mvhd too short")
	}
	version := payload[0]
	var timescale uint32
	var duration uint64
	switch version {
	case 0:
		if len(payload) < 20 {
			return 0, fmt.Errorf("mvhd v0 too short")
		}
		timescale = binary.BigEndian.Uint32(payload[12:16])
		duration = uint64(binary.BigEndian.Uint32(payload[16:20]))
	case 1:
		if len(payload) < 32 {
			return 0, fmt.Errorf("mvhd v1 too short")
		}
		timescale = binary.BigEndian.Uint32(payload[20:24])
		duration = binary.BigEndian.Uint64(payload[24:32])
	default:
		return 0, fmt.Errorf("unsupported mvhd version")
	}
	if timescale == 0 || duration == 0 || duration > uint64(timescale)*60 {
		return 0, fmt.Errorf("invalid mvhd duration")
	}
	value := (duration*1000 + uint64(timescale)/2) / uint64(timescale)
	if value > uint64(^uint32(0)) {
		return 0, fmt.Errorf("mvhd duration overflow")
	}
	return uint32(value), nil
}

func parseMP4Dimensions(payload []byte) (uint32, uint32, error) {
	if len(payload) < 84 {
		return 0, 0, fmt.Errorf("tkhd too short")
	}
	version := payload[0]
	if version > 1 {
		return 0, 0, fmt.Errorf("unsupported tkhd version")
	}
	offset := 76
	if version == 1 {
		offset = 88
	}
	if len(payload) < offset+8 {
		return 0, 0, fmt.Errorf("tkhd dimensions missing")
	}
	widthFixed := binary.BigEndian.Uint32(payload[offset : offset+4])
	heightFixed := binary.BigEndian.Uint32(payload[offset+4 : offset+8])
	return widthFixed >> 16, heightFixed >> 16, nil
}

func findVideoTrackHeader(moov mp4Box) *mp4Box {
	for _, track := range moov.children {
		if track.kind != "trak" {
			continue
		}
		handler := findMP4Box(track, "hdlr")
		if handler != nil && len(handler.payload) >= 12 && string(handler.payload[8:12]) == "vide" {
			return findMP4Box(track, "tkhd")
		}
	}
	return nil
}
