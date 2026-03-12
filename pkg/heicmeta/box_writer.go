package heicmeta

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// IdatWriter handles writing item data to idat boxes.
type IdatWriter struct {
	buffer     *bytes.Buffer
	itemOffset map[uint32]uint64 // itemID -> offset in idat
}

// NewIdatWriter creates a new IdatWriter.
func NewIdatWriter() *IdatWriter {
	return &IdatWriter{
		buffer:     new(bytes.Buffer),
		itemOffset: make(map[uint32]uint64),
	}
}

// WriteItemData writes item data to the buffer and tracks offset.
func (w *IdatWriter) WriteItemData(itemID uint32, data []byte) error {
	// Record offset before writing
	w.itemOffset[itemID] = uint64(w.buffer.Len())
	
	// Write data
	_, err := w.buffer.Write(data)
	return err
}

// GetOffset returns the offset of an item in the idat buffer.
func (w *IdatWriter) GetOffset(itemID uint32) (uint64, bool) {
	offset, exists := w.itemOffset[itemID]
	return offset, exists
}

// GetOffsets returns all item offsets.
func (w *IdatWriter) GetOffsets() map[uint32]uint64 {
	return w.itemOffset
}

// Bytes returns the complete idat payload.
func (w *IdatWriter) Bytes() []byte {
	return w.buffer.Bytes()
}

// BuildFilteredIdat creates a new idat box with filtered metadata.
// Returns the idat payload and a map of itemID -> new offset.
func BuildFilteredIdat(
	file *os.File,
	tree *BoxTree,
	items []ItemInfo,
	locations []ItemLocation,
	modifications map[uint32]ItemModification,
) ([]byte, map[uint32]uint64, error) {
	
	writer := NewIdatWriter()
	
	// Process each item in order
	for _, item := range items {
		// Get modification for this item
		mod, hasMod := modifications[item.ItemID]
		
		// If item should be removed, skip it
		if hasMod && mod.Action == RemoveItem {
			continue
		}
		
		// Get item location
		var itemLoc *ItemLocation
		for i := range locations {
			if locations[i].ItemID == item.ItemID {
				itemLoc = &locations[i]
				break
			}
		}
		
		if itemLoc == nil {
			// No location for this item, skip
			continue
		}
		
		// Determine what data to write
		var dataToWrite []byte
		
		if hasMod && mod.Action == ModifyItem {
			// Use filtered data
			dataToWrite = mod.FilteredData
		} else {
			// Read original data
			originalData, err := ReadItemData(file, tree, *itemLoc)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to read item %d: %w", item.ItemID, err)
			}
			dataToWrite = originalData
		}
		
		// Write to idat buffer
		if err := writer.WriteItemData(item.ItemID, dataToWrite); err != nil {
			return nil, nil, fmt.Errorf("failed to write item %d: %w", item.ItemID, err)
		}
	}
	
	return writer.Bytes(), writer.GetOffsets(), nil
}

// ReadBoxTree reads a box and all its children into memory.
// This is useful for boxes that need to be reconstructed.
func ReadBoxTree(r io.ReadSeeker, node *BoxNode) ([]byte, error) {
	// Seek to box start
	if _, err := r.Seek(int64(node.Offset), io.SeekStart); err != nil {
		return nil, err
	}
	
	// Read entire box including header
	boxData := make([]byte, node.Size)
	if _, err := io.ReadFull(r, boxData); err != nil {
		return nil, err
	}
	
	return boxData, nil
}

// WriteBoxHeader writes a box header with the given type and size.
func WriteBoxHeader(w io.Writer, boxType string, size uint64) error {
	// Size (4 bytes)
	if size > 0xFFFFFFFF {
		// Use extended size
		if _, err := w.Write([]byte{0x00, 0x00, 0x00, 0x01}); err != nil {
			return err
		}
	} else {
		sizeBytes := []byte{
			byte(size >> 24),
			byte(size >> 16),
			byte(size >> 8),
			byte(size),
		}
		if _, err := w.Write(sizeBytes); err != nil {
			return err
		}
	}
	
	// Type (4 bytes)
	typeBytes := []byte(boxType)
	if len(typeBytes) != 4 {
		return fmt.Errorf("invalid box type length: %d", len(typeBytes))
	}
	if _, err := w.Write(typeBytes); err != nil {
		return err
	}
	
	// Extended size if needed
	if size > 0xFFFFFFFF {
		extSizeBytes := []byte{
			byte(size >> 56),
			byte(size >> 48),
			byte(size >> 40),
			byte(size >> 32),
			byte(size >> 24),
			byte(size >> 16),
			byte(size >> 8),
			byte(size),
		}
		if _, err := w.Write(extSizeBytes); err != nil {
			return err
		}
	}
	
	return nil
}

// CopyBoxToWriter copies a box from reader to writer.
func CopyBoxToWriter(r io.ReadSeeker, w io.Writer, node *BoxNode) error {
	// Seek to box start
	if _, err := r.Seek(int64(node.Offset), io.SeekStart); err != nil {
		return err
	}
	
	// Copy entire box
	if _, err := io.CopyN(w, r, int64(node.Size)); err != nil {
		return err
	}
	
	return nil
}

// FindBoxInTree finds a box by type in the tree.
func FindBoxInTree(root *BoxNode, boxType string) *BoxNode {
	if root.TypeString() == boxType {
		return root
	}
	
	for _, child := range root.Children {
		if found := FindBoxInTree(child, boxType); found != nil {
			return found
		}
	}
	
	return nil
}

// FindAllBoxesInTree finds all boxes of a given type.
func FindAllBoxesInTree(root *BoxNode, boxType string) []*BoxNode {
	var result []*BoxNode
	
	if root.TypeString() == boxType {
		result = append(result, root)
	}
	
	for _, child := range root.Children {
		result = append(result, FindAllBoxesInTree(child, boxType)...)
	}
	
	return result
}

// CalculateBoxSize calculates the total size of a box including header and children.
func CalculateBoxSize(payloadSize uint64, hasExtendedSize bool) uint64 {
	headerSize := uint64(8) // Type (4) + Size (4)
	if hasExtendedSize {
		headerSize = 16 // Size (4) + Type (4) + Extended Size (8)
	}
	return headerSize + payloadSize
}
