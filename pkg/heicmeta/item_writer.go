package heicmeta

import (
	"bytes"
	"encoding/binary"
	"io"
)

// ItemModification represents a modification to an item.
type ItemModification struct {
	ItemID       uint32
	Action       ItemAction
	FilteredData []byte // New data after filtering (if Action == ModifyItem)
}

// ItemAction defines what to do with an item.
type ItemAction int

const (
	KeepItem ItemAction = iota
	RemoveItem
	ModifyItem
)

// ItemWriter handles writing modified item structures.
type ItemWriter struct {
	modifications map[uint32]ItemModification
}

// NewItemWriter creates a new ItemWriter.
func NewItemWriter() *ItemWriter {
	return &ItemWriter{
		modifications: make(map[uint32]ItemModification),
	}
}

// AddModification registers a modification for an item.
func (w *ItemWriter) AddModification(mod ItemModification) {
	w.modifications[mod.ItemID] = mod
}

// FilterItems applies filtering to metadata items.
// Returns a map of itemID -> filtered data for items that should be modified.
func FilterItems(items []ItemInfo, itemData map[uint32][]byte) (map[uint32]ItemModification, error) {
	modifications := make(map[uint32]ItemModification)

	for _, item := range items {
		data, exists := itemData[item.ItemID]
		if !exists {
			continue
		}

		switch item.ItemType {
		case "Exif":
			// Apply EXIF filtering
			filtered, err := FilterSensitiveEXIF(data)
			if err != nil {
				// If filtering fails, keep original
				modifications[item.ItemID] = ItemModification{
					ItemID: item.ItemID,
					Action: KeepItem,
				}
				continue
			}

			if filtered == nil {
				// All data was sensitive, remove item
				modifications[item.ItemID] = ItemModification{
					ItemID: item.ItemID,
					Action: RemoveItem,
				}
			} else if len(filtered) < len(data) {
				// Data was filtered, use modified version
				modifications[item.ItemID] = ItemModification{
					ItemID:       item.ItemID,
					Action:       ModifyItem,
					FilteredData: filtered,
				}
			} else {
				// No change needed
				modifications[item.ItemID] = ItemModification{
					ItemID: item.ItemID,
					Action: KeepItem,
				}
			}

		case "mime":
			// Check if it's XMP
			fields, isXMP, _ := DetectSensitiveXMPFields(data)
			if !isXMP || len(fields) == 0 {
				// Not XMP or no sensitive fields, keep as-is
				modifications[item.ItemID] = ItemModification{
					ItemID: item.ItemID,
					Action: KeepItem,
				}
				continue
			}

			// Apply XMP filtering
			filtered, err := FilterSensitiveXMP(data)
			if err != nil {
				// If filtering fails, keep original
				modifications[item.ItemID] = ItemModification{
					ItemID: item.ItemID,
					Action: KeepItem,
				}
				continue
			}

			if filtered == nil {
				// All data was sensitive, remove item
				modifications[item.ItemID] = ItemModification{
					ItemID: item.ItemID,
					Action: RemoveItem,
				}
			} else if len(filtered) < len(data) {
				// Data was filtered, use modified version
				modifications[item.ItemID] = ItemModification{
					ItemID:       item.ItemID,
					Action:       ModifyItem,
					FilteredData: filtered,
				}
			} else {
				// No change needed
				modifications[item.ItemID] = ItemModification{
					ItemID: item.ItemID,
					Action: KeepItem,
				}
			}

		default:
			// Non-metadata item, keep as-is
			modifications[item.ItemID] = ItemModification{
				ItemID: item.ItemID,
				Action: KeepItem,
			}
		}
	}

	return modifications, nil
}

// RebuildIinfBox creates a new iinf box with items removed/modified.
func RebuildIinfBox(items []ItemInfo, modifications map[uint32]ItemModification) ([]byte, error) {
	// Filter out removed items
	keptItems := make([]ItemInfo, 0, len(items))
	for _, item := range items {
		mod, exists := modifications[item.ItemID]
		if !exists || mod.Action != RemoveItem {
			keptItems = append(keptItems, item)
		}
	}

	if len(keptItems) == 0 {
		// No items left, return minimal iinf
		return buildMinimalIinf(), nil
	}

	// Build new iinf payload
	buf := new(bytes.Buffer)

	// Version and flags (version 0)
	buf.Write([]byte{0x00, 0x00, 0x00, 0x00})

	// Entry count (2 bytes for version 0)
	_ = binary.Write(buf, binary.BigEndian, uint16(len(keptItems)))

	// Write each item entry
	for _, item := range keptItems {
		if err := writeItemInfoEntry(buf, item); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// RebuildIlocBox creates a new iloc box with updated offsets and sizes.
func RebuildIlocBox(locations []ItemLocation, modifications map[uint32]ItemModification, newOffsets map[uint32]uint64) ([]byte, error) {
	// Filter out removed items and update offsets
	keptLocs := make([]ItemLocation, 0, len(locations))
	for _, loc := range locations {
		mod, exists := modifications[loc.ItemID]
		if exists && mod.Action == RemoveItem {
			continue
		}

		// Update location if we have new offset/size
		newLoc := loc
		if newOffset, hasNew := newOffsets[loc.ItemID]; hasNew {
			newLoc.BaseOffset = newOffset
		}

		// Update extent length if data was modified
		if exists && mod.Action == ModifyItem {
			if len(newLoc.Extents) > 0 {
				newLoc.Extents[0].ExtentLength = uint64(len(mod.FilteredData))
			}
		}

		keptLocs = append(keptLocs, newLoc)
	}

	if len(keptLocs) == 0 {
		// No items left, return minimal iloc
		return buildMinimalIloc(), nil
	}

	// Build new iloc payload
	buf := new(bytes.Buffer)

	// Version and flags (version 0)
	buf.Write([]byte{0x00, 0x00, 0x00, 0x00})

	// Offset size, length size, base offset size, reserved (4 bits each)
	// Using: offset_size=4, length_size=4, base_offset_size=0, reserved=0
	buf.WriteByte(0x44)

	// Item count (2 bytes for version 0)
	_ = binary.Write(buf, binary.BigEndian, uint16(len(keptLocs)))

	// Write each location
	for _, loc := range keptLocs {
		if err := writeItemLocationEntry(buf, loc); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func buildMinimalIinf() []byte {
	buf := new(bytes.Buffer)
	buf.Write([]byte{0x00, 0x00, 0x00, 0x00}) // version/flags
	buf.Write([]byte{0x00, 0x00})             // count = 0
	return buf.Bytes()
}

func buildMinimalIloc() []byte {
	buf := new(bytes.Buffer)
	buf.Write([]byte{0x00, 0x00, 0x00, 0x00}) // version/flags
	buf.WriteByte(0x44)                       // sizes
	buf.Write([]byte{0x00, 0x00})             // count = 0
	return buf.Bytes()
}

func writeItemInfoEntry(w io.Writer, item ItemInfo) error {
	// Simplified infe entry (version 2)
	buf := new(bytes.Buffer)

	// Type 'infe'
	buf.Write([]byte("infe"))

	// Version (2) and flags
	buf.Write([]byte{0x02, 0x00, 0x00, 0x00})

	// Item ID (2 bytes for version 2)
	_ = binary.Write(buf, binary.BigEndian, uint16(item.ItemID))

	// Item protection index (2 bytes)
	buf.Write([]byte{0x00, 0x00})

	// Item type (4 bytes)
	itemType := item.ItemType
	if len(itemType) > 4 {
		itemType = itemType[:4]
	}
	for len(itemType) < 4 {
		itemType += " "
	}
	buf.WriteString(itemType)

	// Item name (null-terminated string) - empty
	buf.WriteByte(0x00)

	// Write size at beginning
	entryData := buf.Bytes()
	sizeData := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeData, uint32(len(entryData)+4))

	_, _ = w.Write(sizeData)
	_, _ = w.Write(entryData)

	return nil
}

func writeItemLocationEntry(w io.Writer, loc ItemLocation) error {
	// Item ID (2 bytes for version 0)
	_ = binary.Write(w, binary.BigEndian, uint16(loc.ItemID))

	// Construction method (lower 4 bits of next byte)
	// Data reference index (16 bits)
	methodAndRef := make([]byte, 2)
	methodAndRef[0] = byte(loc.ConstructionMethod & 0x0F)
	binary.BigEndian.PutUint16(methodAndRef, loc.DataReferenceIndex)
	// Actually the format is: [method:4bits][reserved:4bits][data_ref:16bits]
	// Let's simplify: method in lower 4 bits of first byte
	_, _ = w.Write([]byte{byte(loc.ConstructionMethod & 0x0F), 0x00})
	_ = binary.Write(w, binary.BigEndian, loc.DataReferenceIndex)

	// Base offset (not written for version 0 if base_offset_size=0)

	// Extent count
	_ = binary.Write(w, binary.BigEndian, uint16(len(loc.Extents)))

	// Write extents
	for _, ext := range loc.Extents {
		// Extent offset (4 bytes based on offset_size=4)
		_ = binary.Write(w, binary.BigEndian, uint32(ext.ExtentOffset))
		// Extent length (4 bytes based on length_size=4)
		_ = binary.Write(w, binary.BigEndian, uint32(ext.ExtentLength))
	}

	return nil
}
