package heicmeta

import (
	"testing"
)

func TestFilterItems_ExifRemoval(t *testing.T) {
	// Create test items
	items := []ItemInfo{
		{ItemID: 1, ItemType: "Exif"},
		{ItemID: 2, ItemType: "hvc1"}, // Not metadata
	}
	
	// Create EXIF data with only sensitive tags (will be removed)
	exifData := buildTestHEICExifGPSOnly()
	itemData := map[uint32][]byte{
		1: exifData,
		2: []byte("pixel data"),
	}
	
	// Filter items
	mods, err := FilterItems(items, itemData)
	if err != nil {
		t.Fatalf("FilterItems failed: %v", err)
	}
	
	// Check EXIF item is marked for removal
	if mod, exists := mods[1]; !exists {
		t.Error("Expected modification for item 1")
	} else if mod.Action != RemoveItem {
		t.Errorf("Expected RemoveItem, got %v", mod.Action)
	}
	
	// Check non-metadata item is kept
	if mod, exists := mods[2]; !exists {
		t.Error("Expected modification for item 2")
	} else if mod.Action != KeepItem {
		t.Errorf("Expected KeepItem, got %v", mod.Action)
	}
}

func TestFilterItems_ExifModification(t *testing.T) {
	// Create test items
	items := []ItemInfo{
		{ItemID: 1, ItemType: "Exif"},
	}
	
	// Create EXIF data with mixed tags (will be modified)
	exifData := buildTestHEICExifWithGPS()
	itemData := map[uint32][]byte{
		1: exifData,
	}
	
	// Filter items
	mods, err := FilterItems(items, itemData)
	if err != nil {
		t.Fatalf("FilterItems failed: %v", err)
	}
	
	// Check EXIF item is marked for modification
	if mod, exists := mods[1]; !exists {
		t.Error("Expected modification for item 1")
	} else if mod.Action != ModifyItem {
		t.Errorf("Expected ModifyItem, got %v", mod.Action)
	} else if len(mod.FilteredData) >= len(exifData) {
		t.Errorf("Expected filtered data to be smaller, got %d >= %d", len(mod.FilteredData), len(exifData))
	}
}

func TestRebuildIinfBox_WithRemovals(t *testing.T) {
	items := []ItemInfo{
		{ItemID: 1, ItemType: "Exif"},
		{ItemID: 2, ItemType: "hvc1"},
		{ItemID: 3, ItemType: "mime"},
	}
	
	modifications := map[uint32]ItemModification{
		1: {ItemID: 1, Action: RemoveItem},
		3: {ItemID: 3, Action: RemoveItem},
	}
	
	payload, err := RebuildIinfBox(items, modifications)
	if err != nil {
		t.Fatalf("RebuildIinfBox failed: %v", err)
	}
	
	// Check payload structure
	if len(payload) < 6 {
		t.Errorf("Payload too short: %d bytes", len(payload))
	}
	
	// Version should be 0
	if payload[0] != 0x00 {
		t.Errorf("Expected version 0, got 0x%02x", payload[0])
	}
	
	// Entry count should be 1 (only item 2 remains)
	entryCount := uint16(payload[4])<<8 | uint16(payload[5])
	if entryCount != 1 {
		t.Errorf("Expected 1 entry, got %d", entryCount)
	}
}

func TestRebuildIinfBox_NoItems(t *testing.T) {
	items := []ItemInfo{
		{ItemID: 1, ItemType: "Exif"},
	}
	
	modifications := map[uint32]ItemModification{
		1: {ItemID: 1, Action: RemoveItem},
	}
	
	payload, err := RebuildIinfBox(items, modifications)
	if err != nil {
		t.Fatalf("RebuildIinfBox failed: %v", err)
	}
	
	// Should return minimal iinf with 0 entries
	if len(payload) < 6 {
		t.Errorf("Payload too short: %d bytes", len(payload))
	}
	
	entryCount := uint16(payload[4])<<8 | uint16(payload[5])
	if entryCount != 0 {
		t.Errorf("Expected 0 entries, got %d", entryCount)
	}
}

func TestRebuildIlocBox_WithUpdates(t *testing.T) {
	locations := []ItemLocation{
		{
			ItemID:   1,
			Extents:  []ItemExtent{{ExtentOffset: 1000, ExtentLength: 318}},
		},
		{
			ItemID:   2,
			Extents:  []ItemExtent{{ExtentOffset: 2000, ExtentLength: 5000}},
		},
	}
	
	modifications := map[uint32]ItemModification{
		1: {
			ItemID:       1,
			Action:       ModifyItem,
			FilteredData: make([]byte, 88), // Reduced from 318
		},
	}
	
	newOffsets := map[uint32]uint64{
		1: 1000, // Same offset
	}
	
	payload, err := RebuildIlocBox(locations, modifications, newOffsets)
	if err != nil {
		t.Fatalf("RebuildIlocBox failed: %v", err)
	}
	
	// Check payload structure
	if len(payload) < 7 {
		t.Errorf("Payload too short: %d bytes", len(payload))
	}
	
	// Version should be 0
	if payload[0] != 0x00 {
		t.Errorf("Expected version 0, got 0x%02x", payload[0])
	}
	
	// Item count should be 2
	itemCount := uint16(payload[5])<<8 | uint16(payload[6])
	if itemCount != 2 {
		t.Errorf("Expected 2 items, got %d", itemCount)
	}
}
