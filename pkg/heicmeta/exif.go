package heicmeta

import (
	"encoding/binary"
	"fmt"
)

const (
	exifTagExifIFDPointer = 0x8769
	exifTagGPSIFDPointer  = 0x8825
	exifTagInteropPointer = 0xA005
)

var sensitiveExifTagNames = map[uint16]string{
	0x0001: "GPSLatitudeRef",
	0x0002: "GPSLatitude",
	0x0003: "GPSLongitudeRef",
	0x0004: "GPSLongitude",
	0x0005: "GPSAltitudeRef",
	0x0006: "GPSAltitude",
	0x0007: "GPSTimeStamp",
	0x000D: "GPSSpeed",
	0x0010: "GPSImgDirection",
	0x001D: "GPSDateStamp",

	0x010F: "Make",
	0x0110: "Model",
	0x0131: "Software",
	0x0132: "DateTime",
	0x013B: "Artist",
	0x8298: "Copyright",
	0x9003: "DateTimeOriginal",
	0x9004: "DateTimeDigitized",
	0x9286: "UserComment",
	0xA420: "ImageUniqueID",
	0xA430: "CameraOwnerName",
	0xA431: "BodySerialNumber",
	0xA433: "LensMake",
	0xA434: "LensModel",
	0xA435: "LensSerialNumber",

	// Sub-IFD pointers (remove entire sub-IFDs if they point to sensitive data)
	0x8769: "ExifIFDPointer", // Points to Exif sub-IFD (contains DateTimeOriginal, etc.)
	0x8825: "GPSIFDPointer",  // Points to GPS sub-IFD (all GPS data is sensitive)
}

// DetectSensitiveEXIFTagsFromHEICPayload detects sensitive EXIF tags from HEIC Exif box payload.
// HEIC Exif payload starts with a 4-byte offset prefix before TIFF data.
func DetectSensitiveEXIFTagsFromHEICPayload(payload []byte) ([]string, error) {
	if len(payload) < 12 {
		return nil, fmt.Errorf("invalid Exif payload length: %d", len(payload))
	}

	tiffStart, err := resolveHEICExifTIFFStart(payload)
	if err != nil {
		return nil, err
	}

	tagIDs, err := parseTIFFTagIDs(payload[tiffStart:])
	if err != nil {
		return nil, err
	}

	out := make(map[string]struct{})
	for tagID := range tagIDs {
		if name, ok := sensitiveExifTagNames[tagID]; ok {
			out[name] = struct{}{}
		}
	}
	return sortedKeys(out), nil
}

func resolveHEICExifTIFFStart(payload []byte) (int, error) {
	if len(payload) < 12 {
		return 0, fmt.Errorf("invalid Exif payload length: %d", len(payload))
	}

	offset := int(binary.BigEndian.Uint32(payload[:4]))
	candidates := []int{4}
	if offset > 0 {
		candidates = append([]int{offset}, candidates...)
		candidates = append(candidates, 4+offset)
	}

	for _, c := range candidates {
		if c < 0 || c+8 > len(payload) {
			continue
		}
		if looksLikeTIFF(payload[c:]) {
			return c, nil
		}
	}

	return 0, fmt.Errorf("failed to locate TIFF header in Exif payload")
}

func looksLikeTIFF(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	little := data[0] == 'I' && data[1] == 'I'
	big := data[0] == 'M' && data[1] == 'M'
	if !little && !big {
		return false
	}
	var bo binary.ByteOrder = binary.BigEndian
	if little {
		bo = binary.LittleEndian
	}
	return bo.Uint16(data[2:4]) == 42
}

func parseTIFFTagIDs(data []byte) (map[uint16]struct{}, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("invalid TIFF data length: %d", len(data))
	}

	var bo binary.ByteOrder
	switch {
	case data[0] == 'I' && data[1] == 'I':
		bo = binary.LittleEndian
	case data[0] == 'M' && data[1] == 'M':
		bo = binary.BigEndian
	default:
		return nil, fmt.Errorf("invalid TIFF byte order marker")
	}
	if bo.Uint16(data[2:4]) != 42 {
		return nil, fmt.Errorf("invalid TIFF magic number")
	}

	firstIFD := bo.Uint32(data[4:8])
	tags := make(map[uint16]struct{})
	seen := make(map[uint32]struct{})

	var walkIFD func(offset uint32, depth int) error
	walkIFD = func(offset uint32, depth int) error {
		if offset == 0 || depth > 16 {
			return nil
		}
		if _, exists := seen[offset]; exists {
			return nil
		}
		seen[offset] = struct{}{}

		base := int(offset)
		if base+2 > len(data) {
			return fmt.Errorf("IFD offset out of range: %d", offset)
		}

		count := int(bo.Uint16(data[base : base+2]))
		entriesStart := base + 2
		entriesEnd := entriesStart + count*12
		if entriesEnd+4 > len(data) {
			return fmt.Errorf("IFD entries out of range at offset %d", offset)
		}

		for i := 0; i < count; i++ {
			entry := entriesStart + i*12
			tag := bo.Uint16(data[entry : entry+2])
			tags[tag] = struct{}{}

			valueOffset := bo.Uint32(data[entry+8 : entry+12])
			switch tag {
			case exifTagExifIFDPointer, exifTagGPSIFDPointer, exifTagInteropPointer:
				if err := walkIFD(valueOffset, depth+1); err != nil {
					return err
				}
			}
		}

		next := bo.Uint32(data[entriesEnd : entriesEnd+4])
		return walkIFD(next, depth+1)
	}

	if err := walkIFD(firstIFD, 0); err != nil {
		return nil, err
	}
	return tags, nil
}
