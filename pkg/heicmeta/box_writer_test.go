package heicmeta

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestIdatWriter(t *testing.T) {
	writer := NewIdatWriter()
	
	// Write some items
	data1 := []byte("item 1 data")
	data2 := []byte("item 2 data")
	
	if err := writer.WriteItemData(1, data1); err != nil {
		t.Fatalf("WriteItemData failed: %v", err)
	}
	
	if err := writer.WriteItemData(2, data2); err != nil {
		t.Fatalf("WriteItemData failed: %v", err)
	}
	
	// Check offsets
	offset1, exists := writer.GetOffset(1)
	if !exists {
		t.Error("Offset for item 1 not found")
	}
	if offset1 != 0 {
		t.Errorf("Expected offset 0 for item 1, got %d", offset1)
	}
	
	offset2, exists := writer.GetOffset(2)
	if !exists {
		t.Error("Offset for item 2 not found")
	}
	if offset2 != uint64(len(data1)) {
		t.Errorf("Expected offset %d for item 2, got %d", len(data1), offset2)
	}
	
	// Check buffer
	result := writer.Bytes()
	expected := append(data1, data2...)
	if !bytes.Equal(result, expected) {
		t.Errorf("Buffer mismatch: got %v, want %v", result, expected)
	}
}

func TestBuildFilteredIdat(t *testing.T) {
	// Create a temporary test file with item data
	tmpFile, err := os.CreateTemp("", "test-idat-*.heic")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()
	
	// Write some test data
	originalData1 := buildTestHEICExifWithGPS()
	originalData2 := []byte("non-metadata item")
	
	// Create a simple structure: write data at specific offsets
	// Offset 0: original data 1
	// Offset len(originalData1): original data 2
	tmpFile.Write(originalData1)
	tmpFile.Write(originalData2)
	tmpFile.Seek(0, io.SeekStart)
	
	// Create box tree (simplified for test)
	tree := &BoxTree{
		Root: []*BoxNode{
			{
				Type:   strToBoxType("idat"),
				Offset: 0,
				Size:   uint64(len(originalData1) + len(originalData2)),
			},
		},
	}
	
	// Create items
	items := []ItemInfo{
		{ItemID: 1, ItemType: "Exif"},
		{ItemID: 2, ItemType: "hvc1"},
	}
	
	// Create locations
	locations := []ItemLocation{
		{
			ItemID: 1,
			Extents: []ItemExtent{
				{ExtentOffset: 0, ExtentLength: uint64(len(originalData1))},
			},
		},
		{
			ItemID: 2,
			Extents: []ItemExtent{
				{ExtentOffset: uint64(len(originalData1)), ExtentLength: uint64(len(originalData2))},
			},
		},
	}
	
	// Create modifications (filter item 1)
	filteredData1 := buildTestHEICExifOnlyOrientation()
	modifications := map[uint32]ItemModification{
		1: {
			ItemID:       1,
			Action:       ModifyItem,
			FilteredData: filteredData1,
		},
	}
	
	// Build filtered idat
	idatPayload, offsets, err := BuildFilteredIdat(tmpFile, tree, items, locations, modifications)
	if err != nil {
		t.Fatalf("BuildFilteredIdat failed: %v", err)
	}
	
	// Check that we have both items
	if len(offsets) != 2 {
		t.Errorf("Expected 2 items, got %d", len(offsets))
	}
	
	// Check offsets
	if offset1, exists := offsets[1]; !exists {
		t.Error("Offset for item 1 not found")
	} else if offset1 != 0 {
		t.Errorf("Expected offset 0 for item 1, got %d", offset1)
	}
	
	if offset2, exists := offsets[2]; !exists {
		t.Error("Offset for item 2 not found")
	} else if offset2 != uint64(len(filteredData1)) {
		t.Errorf("Expected offset %d for item 2, got %d", len(filteredData1), offset2)
	}
	
	// Check that filtered data is used
	expectedSize := len(filteredData1) + len(originalData2)
	if len(idatPayload) != expectedSize {
		t.Errorf("Expected payload size %d, got %d", expectedSize, len(idatPayload))
	}
}

func TestBuildFilteredIdat_WithRemoval(t *testing.T) {
	// Create a temporary test file
	tmpFile, err := os.CreateTemp("", "test-idat-*.heic")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()
	
	// Write test data
	data1 := buildTestHEICExifGPSOnly()
	data2 := []byte("pixel data")
	
	tmpFile.Write(data1)
	tmpFile.Write(data2)
	tmpFile.Seek(0, io.SeekStart)
	
	// Create tree
	tree := &BoxTree{
		Root: []*BoxNode{
			{
				Type:   strToBoxType("idat"),
				Offset: 0,
				Size:   uint64(len(data1) + len(data2)),
			},
		},
	}
	
	// Items and locations
	items := []ItemInfo{
		{ItemID: 1, ItemType: "Exif"},
		{ItemID: 2, ItemType: "hvc1"},
	}
	
	locations := []ItemLocation{
		{ItemID: 1, Extents: []ItemExtent{{ExtentOffset: 0, ExtentLength: uint64(len(data1))}}},
		{ItemID: 2, Extents: []ItemExtent{{ExtentOffset: uint64(len(data1)), ExtentLength: uint64(len(data2))}}},
	}
	
	// Remove item 1
	modifications := map[uint32]ItemModification{
		1: {ItemID: 1, Action: RemoveItem},
	}
	
	// Build filtered idat
	idatPayload, offsets, err := BuildFilteredIdat(tmpFile, tree, items, locations, modifications)
	if err != nil {
		t.Fatalf("BuildFilteredIdat failed: %v", err)
	}
	
	// Should only have item 2
	if len(offsets) != 1 {
		t.Errorf("Expected 1 item, got %d", len(offsets))
	}
	
	// Item 2 should be at offset 0 now
	if offset2, exists := offsets[2]; !exists {
		t.Error("Offset for item 2 not found")
	} else if offset2 != 0 {
		t.Errorf("Expected offset 0 for item 2, got %d", offset2)
	}
	
	// Payload should only contain item 2
	if len(idatPayload) != len(data2) {
		t.Errorf("Expected payload size %d, got %d", len(data2), len(idatPayload))
	}
}

func TestWriteBoxHeader(t *testing.T) {
	buf := new(bytes.Buffer)
	
	// Test normal size
	if err := WriteBoxHeader(buf, "test", 100); err != nil {
		t.Fatalf("WriteBoxHeader failed: %v", err)
	}
	
	result := buf.Bytes()
	if len(result) != 8 {
		t.Errorf("Expected 8 bytes, got %d", len(result))
	}
	
	// Check size
	size := uint32(result[0])<<24 | uint32(result[1])<<16 | uint32(result[2])<<8 | uint32(result[3])
	if size != 100 {
		t.Errorf("Expected size 100, got %d", size)
	}
	
	// Check type
	if string(result[4:8]) != "test" {
		t.Errorf("Expected type 'test', got '%s'", string(result[4:8]))
	}
}

func TestFindBoxInTree(t *testing.T) {
	// Create a simple tree
	root := &BoxNode{
		Type: strToBoxType("moov"),
		Children: []*BoxNode{
			{Type: strToBoxType("meta")},
			{
				Type: strToBoxType("trak"),
				Children: []*BoxNode{
					{Type: strToBoxType("mdia")},
				},
			},
		},
	}
	
	// Find meta
	metaBox := FindBoxInTree(root, "meta")
	if metaBox == nil {
		t.Error("Failed to find meta box")
	} else if metaBox.TypeString() != "meta" {
		t.Errorf("Expected meta, got %s", metaBox.TypeString())
	}
	
	// Find mdia
	mdiaBox := FindBoxInTree(root, "mdia")
	if mdiaBox == nil {
		t.Error("Failed to find mdia box")
	}
	
	// Find non-existent
	fooBox := FindBoxInTree(root, "fooo")
	if fooBox != nil {
		t.Error("Expected nil for non-existent box")
	}
}

// Helper to create test EXIF with only Orientation
func buildTestHEICExifOnlyOrientation() []byte {
	payload := make([]byte, 0, 100)
	// HEIC prefix
	payload = append(payload, 0x00, 0x00, 0x00, 0x06)
	payload = append(payload, 'E', 'x', 'i', 'f', 0x00, 0x00)
	// TIFF header
	payload = append(payload, 'M', 'M', 0x00, 0x2a)
	payload = append(payload, 0x00, 0x00, 0x00, 0x08)
	// IFD0: 1 entry (Orientation only)
	payload = append(payload, 0x00, 0x01)
	// Orientation (0x0112) = 1
	payload = append(payload, 0x01, 0x12, 0x00, 0x03, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00)
	// Next IFD = 0
	payload = append(payload, 0x00, 0x00, 0x00, 0x00)
	return payload
}

func strToBoxType(s string) [4]byte {
	var t [4]byte
	copy(t[:], s)
	return t
}
