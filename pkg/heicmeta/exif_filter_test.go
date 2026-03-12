package heicmeta

import (
	"encoding/binary"
	"testing"
)

func TestFilterSensitiveEXIF(t *testing.T) {
	// Create HEIC EXIF payload with GPS tags (sensitive) and Orientation (not sensitive)
	payload := buildTestHEICExifWithGPS()
	
	// Filter
	filtered, err := FilterSensitiveEXIF(payload)
	if err != nil {
		t.Fatalf("FilterSensitiveEXIF failed: %v", err)
	}
	
	// Should not be nil (Orientation should remain)
	if filtered == nil {
		t.Error("Expected some data to remain (Orientation tag)")
	}
	
	// Filtered should be smaller than original
	if len(filtered) >= len(payload) {
		t.Errorf("Filtered size (%d) should be less than original (%d)", len(filtered), len(payload))
	}
	
	// Parse filtered to verify GPS tags are gone
	tags, err := DetectSensitiveEXIFTagsFromHEICPayload(filtered)
	if err != nil {
		t.Fatalf("Failed to detect tags in filtered data: %v", err)
	}
	
	// Should have no sensitive tags
	if len(tags) > 0 {
		t.Errorf("Expected no sensitive tags, got: %v", tags)
	}
}

func TestFilterSensitiveEXIFAllSensitive(t *testing.T) {
	// Create EXIF with only sensitive tags (GPS only)
	payload := buildTestHEICExifGPSOnly()
	
	// Filter
	filtered, err := FilterSensitiveEXIF(payload)
	if err != nil {
		t.Fatalf("FilterSensitiveEXIF failed: %v", err)
	}
	
	// Should be nil (all tags were sensitive)
	if filtered != nil {
		t.Error("Expected nil when all tags are sensitive")
	}
}

func buildTestHEICExifWithGPS() []byte {
	// Build HEIC EXIF: 4-byte offset + "Exif\0\0" + TIFF with Orientation + Make (sensitive)
	payload := make([]byte, 0, 200)
	
	// HEIC EXIF prefix
	payload = append(payload, 0x00, 0x00, 0x00, 0x06) // offset 6
	payload = append(payload, 'E', 'x', 'i', 'f', 0x00, 0x00)
	
	// TIFF header (big-endian)
	payload = append(payload, 'M', 'M', 0x00, 0x2a)
	payload = append(payload, 0x00, 0x00, 0x00, 0x08) // IFD0 at offset 8
	
	// IFD0: 2 entries
	payload = append(payload, 0x00, 0x02)
	
	// Entry 1: Orientation (0x0112) = 1 (not sensitive)
	payload = append(payload, 0x01, 0x12, 0x00, 0x03, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00)
	
	// Entry 2: Make (0x010F) - ASCII string "Canon" (sensitive)
	// Type 2 (ASCII), count 6, data at offset after IFD
	dataOffset := uint32(8 + 2 + 2*12 + 4) // After IFD0
	payload = append(payload, 0x01, 0x0F, 0x00, 0x02, 0x00, 0x00, 0x00, 0x06)
	offsetBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(offsetBytes, dataOffset)
	payload = append(payload, offsetBytes...)
	
	// Next IFD = 0
	payload = append(payload, 0x00, 0x00, 0x00, 0x00)
	
	// External data: "Canon\0"
	payload = append(payload, 'C', 'a', 'n', 'o', 'n', 0x00)
	
	return payload
}

func buildTestHEICExifGPSOnly() []byte {
	// Build HEIC EXIF with only Make (sensitive)
	payload := make([]byte, 0, 100)
	
	// HEIC EXIF prefix
	payload = append(payload, 0x00, 0x00, 0x00, 0x06)
	payload = append(payload, 'E', 'x', 'i', 'f', 0x00, 0x00)
	
	// TIFF header
	payload = append(payload, 'M', 'M', 0x00, 0x2a)
	payload = append(payload, 0x00, 0x00, 0x00, 0x08)
	
	// IFD0: 1 entry (Make only)
	payload = append(payload, 0x00, 0x01)
	
	// Make (0x010F)
	dataOffset := uint32(8 + 2 + 12 + 4)
	payload = append(payload, 0x01, 0x0F, 0x00, 0x02, 0x00, 0x00, 0x00, 0x06)
	offsetBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(offsetBytes, dataOffset)
	payload = append(payload, offsetBytes...)
	
	// Next IFD = 0
	payload = append(payload, 0x00, 0x00, 0x00, 0x00)
	
	// External data: "Canon\0"
	payload = append(payload, 'C', 'a', 'n', 'o', 'n', 0x00)
	
	return payload
}
