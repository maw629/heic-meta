package heicmeta

import (
	"encoding/binary"
	"fmt"
	"io"
)

// FilterSensitiveEXIF removes sensitive tags from HEIC EXIF payload.
// Returns filtered EXIF data or nil if all tags were sensitive.
func FilterSensitiveEXIF(payload []byte) ([]byte, error) {
	if len(payload) < 12 {
		return nil, fmt.Errorf("invalid EXIF payload length: %d", len(payload))
	}

	tiffStart, err := resolveHEICExifTIFFStart(payload)
	if err != nil {
		return nil, err
	}

	tiffData := payload[tiffStart:]
	if len(tiffData) < 8 {
		return nil, fmt.Errorf("invalid TIFF data length: %d", len(tiffData))
	}

	// Determine byte order
	var bo binary.ByteOrder
	switch {
	case tiffData[0] == 'I' && tiffData[1] == 'I':
		bo = binary.LittleEndian
	case tiffData[0] == 'M' && tiffData[1] == 'M':
		bo = binary.BigEndian
	default:
		return nil, fmt.Errorf("invalid TIFF byte order marker")
	}

	// Verify TIFF magic number
	if bo.Uint16(tiffData[2:4]) != 42 {
		return nil, fmt.Errorf("invalid TIFF magic number")
	}

	// Get IFD0 offset
	ifd0Offset := bo.Uint32(tiffData[4:8])
	if ifd0Offset < 8 || int(ifd0Offset) >= len(tiffData) {
		return nil, fmt.Errorf("invalid IFD0 offset: %d", ifd0Offset)
	}

	// Filter the TIFF data
	filteredTIFF, err := filterTIFFIFDs(tiffData, bo)
	if err != nil {
		return nil, err
	}

	// If no tags remain, return nil
	if len(filteredTIFF) == 0 {
		return nil, nil
	}

	// Rebuild HEIC EXIF payload with filtered TIFF
	result := make([]byte, 0, tiffStart+len(filteredTIFF))
	result = append(result, payload[:tiffStart]...)
	result = append(result, filteredTIFF...)

	return result, nil
}

func filterTIFFIFDs(tiffData []byte, bo binary.ByteOrder) ([]byte, error) {
	// For Step 3 core implementation, we'll do a simple approach:
	// Parse IFD entries, remove sensitive tags, rebuild IFD structure
	
	// Get IFD0 offset
	ifd0Offset := bo.Uint32(tiffData[4:8])
	if int(ifd0Offset)+2 > len(tiffData) {
		return nil, fmt.Errorf("IFD0 offset out of bounds")
	}

	// Read entry count
	entryCount := bo.Uint16(tiffData[ifd0Offset : ifd0Offset+2])
	ifdStart := int(ifd0Offset)
	entriesStart := ifdStart + 2
	entriesEnd := entriesStart + int(entryCount)*12

	if entriesEnd+4 > len(tiffData) {
		return nil, fmt.Errorf("IFD entries out of bounds")
	}

	// Filter entries
	keptEntries := make([][]byte, 0)
	dataSegments := make([][]byte, 0)
	currentDataOffset := uint32(entriesEnd + 4) // After nextIFD pointer

	for i := 0; i < int(entryCount); i++ {
		entryOffset := entriesStart + i*12
		entry := tiffData[entryOffset : entryOffset+12]
		
		tagID := bo.Uint16(entry[0:2])
		
		// Check if this tag is sensitive
		if _, isSensitive := sensitiveExifTagNames[tagID]; isSensitive {
			// Skip sensitive tags
			continue
		}

		// Check if tag has data outside the entry (count * typeSize > 4)
		tagType := bo.Uint16(entry[2:4])
		count := bo.Uint32(entry[4:8])
		
		typeSize := getTagTypeSize(tagType)
		dataSize := count * uint32(typeSize)

		if dataSize > 4 {
			// Data is stored externally, need to copy it
			dataOffset := bo.Uint32(entry[8:12])
			if int(dataOffset)+int(dataSize) <= len(tiffData) {
				// Copy external data
				externalData := tiffData[dataOffset : dataOffset+dataSize]
				dataSegments = append(dataSegments, externalData)
				
				// Update entry to point to new offset
				newEntry := make([]byte, 12)
				copy(newEntry, entry)
				bo.PutUint32(newEntry[8:12], currentDataOffset)
				keptEntries = append(keptEntries, newEntry)
				
				currentDataOffset += dataSize
			}
		} else {
			// Data fits in entry, keep as-is
			keptEntries = append(keptEntries, entry)
		}
	}

	// If no entries remain, return empty
	if len(keptEntries) == 0 {
		return nil, nil
	}

	// Rebuild TIFF structure
	result := make([]byte, 0, 8+2+len(keptEntries)*12+4+int(currentDataOffset))
	
	// TIFF header
	result = append(result, tiffData[0:4]...) // Byte order + magic
	
	// IFD0 offset (always 8)
	ifd0OffsetBytes := make([]byte, 4)
	bo.PutUint32(ifd0OffsetBytes, 8)
	result = append(result, ifd0OffsetBytes...)
	
	// Entry count
	entryCountBytes := make([]byte, 2)
	bo.PutUint16(entryCountBytes, uint16(len(keptEntries)))
	result = append(result, entryCountBytes...)
	
	// Entries
	for _, entry := range keptEntries {
		result = append(result, entry...)
	}
	
	// Next IFD pointer (0 = no next IFD)
	result = append(result, 0, 0, 0, 0)
	
	// External data segments
	for _, data := range dataSegments {
		result = append(result, data...)
	}

	return result, nil
}

func getTagTypeSize(tagType uint16) int {
	switch tagType {
	case 1: // BYTE
		return 1
	case 2: // ASCII
		return 1
	case 3: // SHORT
		return 2
	case 4: // LONG
		return 4
	case 5: // RATIONAL
		return 8
	case 6: // SBYTE
		return 1
	case 7: // UNDEFINED
		return 1
	case 8: // SSHORT
		return 2
	case 9: // SLONG
		return 4
	case 10: // SRATIONAL
		return 8
	case 11: // FLOAT
		return 4
	case 12: // DOUBLE
		return 8
	default:
		return 1
	}
}

// Helper to read data from specific offset
func readBytesAt(r io.ReadSeeker, offset, length int64) ([]byte, error) {
	if _, err := r.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}
