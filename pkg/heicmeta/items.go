package heicmeta

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	mp4 "github.com/abema/go-mp4"
)

// ItemInfo represents one item from the iinf box.
type ItemInfo struct {
	ItemID   uint32
	ItemType string
}

// ItemLocation represents where an item's data is stored.
type ItemLocation struct {
	ItemID             uint32
	ConstructionMethod uint16
	DataReferenceIndex uint16
	BaseOffset         uint64
	Extents            []ItemExtent
}

type ItemExtent struct {
	ExtentIndex  uint64
	ExtentOffset uint64
	ExtentLength uint64
}

// ParseItemInfo extracts item information from iinf box.
func ParseItemInfo(r io.ReadSeeker, node *BoxNode) ([]ItemInfo, error) {
	if node == nil || node.TypeString() != "iinf" {
		return nil, fmt.Errorf("invalid iinf node")
	}

	payload, err := ReadBoxPayloadBytes(r, node)
	if err != nil {
		return nil, err
	}

	if len(payload) < 6 {
		return nil, fmt.Errorf("iinf payload too short: %d", len(payload))
	}

	version := payload[0]
	var entryCount uint32
	var offset int

	if version == 0 {
		entryCount = uint32(binary.BigEndian.Uint16(payload[4:6]))
		offset = 6
	} else {
		if len(payload) < 8 {
			return nil, fmt.Errorf("iinf v%d payload too short", version)
		}
		entryCount = binary.BigEndian.Uint32(payload[4:8])
		offset = 8
	}

	items := make([]ItemInfo, 0, entryCount)

	for i := uint32(0); i < entryCount && offset+8 <= len(payload); i++ {
		entrySize := binary.BigEndian.Uint32(payload[offset : offset+4])
		if entrySize < 8 || offset+int(entrySize) > len(payload) {
			break
		}

		entryType := string(payload[offset+4 : offset+8])
		if entryType != "infe" {
			offset += int(entrySize)
			continue
		}

		entryPayload := payload[offset+8 : offset+int(entrySize)]
		if len(entryPayload) < 6 {
			offset += int(entrySize)
			continue
		}

		entryVersion := entryPayload[0]
		var itemID uint32
		var itemType string

		if entryVersion == 2 || entryVersion == 3 {
			if len(entryPayload) < 8 {
				offset += int(entrySize)
				continue
			}
			if entryVersion == 2 {
				itemID = uint32(binary.BigEndian.Uint16(entryPayload[4:6]))
			} else {
				itemID = binary.BigEndian.Uint32(entryPayload[4:8])
			}

			typeOffset := 8
			if entryVersion == 3 {
				typeOffset = 10
			}
			if len(entryPayload) >= typeOffset+4 {
				itemType = string(entryPayload[typeOffset : typeOffset+4])
			}
		}

		if itemType == "Exif" || itemType == "mime" {
			items = append(items, ItemInfo{
				ItemID:   itemID,
				ItemType: itemType,
			})
		}

		offset += int(entrySize)
	}

	return items, nil
}

// ParseItemLocation extracts item location information from iloc box.
func ParseItemLocation(r io.ReadSeeker, node *BoxNode) ([]ItemLocation, error) {
	if node == nil || node.TypeString() != "iloc" {
		return nil, fmt.Errorf("invalid iloc node")
	}

	payload, err := ReadBoxPayloadBytes(r, node)
	if err != nil {
		return nil, err
	}

	if len(payload) < 8 {
		return nil, fmt.Errorf("iloc payload too short: %d", len(payload))
	}

	version := payload[0]
	offsetSize := int((payload[4] >> 4) & 0xF)
	lengthSize := int(payload[4] & 0xF)
	baseOffsetSize := int((payload[5] >> 4) & 0xF)
	indexSize := 0
	if version == 1 || version == 2 {
		indexSize = int(payload[5] & 0xF)
	}

	var itemCount uint32
	offset := 6
	if version < 2 {
		itemCount = uint32(binary.BigEndian.Uint16(payload[offset : offset+2]))
		offset += 2
	} else {
		itemCount = binary.BigEndian.Uint32(payload[offset : offset+4])
		offset += 4
	}

	locations := make([]ItemLocation, 0, itemCount)

	for i := uint32(0); i < itemCount; i++ {
		loc := ItemLocation{}

		// Read item ID
		if version < 2 {
			if offset+2 > len(payload) {
				break
			}
			loc.ItemID = uint32(binary.BigEndian.Uint16(payload[offset : offset+2]))
			offset += 2
		} else {
			if offset+4 > len(payload) {
				break
			}
			loc.ItemID = binary.BigEndian.Uint32(payload[offset : offset+4])
			offset += 4
		}

		// Read construction method (version 1+)
		if version >= 1 {
			if offset+2 > len(payload) {
				break
			}
			loc.ConstructionMethod = binary.BigEndian.Uint16(payload[offset : offset+2])
			offset += 2
		}

		// Read data reference index
		if offset+2 > len(payload) {
			break
		}
		loc.DataReferenceIndex = binary.BigEndian.Uint16(payload[offset : offset+2])
		offset += 2

		// Read base offset
		if baseOffsetSize > 0 {
			if offset+baseOffsetSize > len(payload) {
				break
			}
			loc.BaseOffset = readUintN(payload[offset:], baseOffsetSize)
			offset += baseOffsetSize
		}

		// Read extent count
		if offset+2 > len(payload) {
			break
		}
		extentCount := binary.BigEndian.Uint16(payload[offset : offset+2])
		offset += 2

		// Read extents
		for j := uint16(0); j < extentCount; j++ {
			extent := ItemExtent{}

			if version >= 1 && indexSize > 0 {
				if offset+indexSize > len(payload) {
					break
				}
				extent.ExtentIndex = readUintN(payload[offset:], indexSize)
				offset += indexSize
			}

			if offsetSize > 0 {
				if offset+offsetSize > len(payload) {
					break
				}
				extent.ExtentOffset = readUintN(payload[offset:], offsetSize)
				offset += offsetSize
			}

			if lengthSize > 0 {
				if offset+lengthSize > len(payload) {
					break
				}
				extent.ExtentLength = readUintN(payload[offset:], lengthSize)
				offset += lengthSize
			}

			loc.Extents = append(loc.Extents, extent)
		}

		locations = append(locations, loc)
	}

	return locations, nil
}

func readUintN(data []byte, n int) uint64 {
	if n <= 0 || n > 8 || len(data) < n {
		return 0
	}
	var val uint64
	for i := 0; i < n; i++ {
		val = (val << 8) | uint64(data[i])
	}
	return val
}

// ReadItemData reads the data for an item from either idat or mdat.
func ReadItemData(f *os.File, tree *BoxTree, loc ItemLocation) ([]byte, error) {
	if len(loc.Extents) == 0 {
		return nil, fmt.Errorf("no extents for item %d", loc.ItemID)
	}

	// For simplicity, handle single extent case
	extent := loc.Extents[0]

	// Construction method 0: data is in file (idat or mdat)
	// Data reference index 0 means this file
	if loc.ConstructionMethod != 0 || loc.DataReferenceIndex != 0 {
		return nil, fmt.Errorf("unsupported construction method or data reference")
	}

	// Calculate absolute offset
	offset := loc.BaseOffset + extent.ExtentOffset
	length := extent.ExtentLength

	if length == 0 {
		return nil, fmt.Errorf("zero length extent for item %d", loc.ItemID)
	}

	// Read the data
	if _, err := f.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek failed for item %d: %w", loc.ItemID, err)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, fmt.Errorf("read failed for item %d: %w", loc.ItemID, err)
	}

	return data, nil
}

// ExtractItemBasedMetadata finds and extracts Exif/XMP metadata stored as items.
func ExtractItemBasedMetadata(path string, tree *BoxTree) (exifItems [][]byte, xmpItems [][]byte, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	// Find iinf and iloc boxes
	iinfNodes := findBoxesByType(tree.Root, mp4.StrToBoxType("iinf"))
	ilocNodes := findBoxesByType(tree.Root, mp4.StrToBoxType("iloc"))

	if len(iinfNodes) == 0 || len(ilocNodes) == 0 {
		return nil, nil, nil // No item-based metadata
	}

	// Parse item info
	items, err := ParseItemInfo(f, iinfNodes[0])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse iinf: %w", err)
	}

	// Parse item locations
	locations, err := ParseItemLocation(f, ilocNodes[0])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse iloc: %w", err)
	}

	// Build location map
	locMap := make(map[uint32]ItemLocation)
	for _, loc := range locations {
		locMap[loc.ItemID] = loc
	}

	// Extract Exif and mime items
	for _, item := range items {
		loc, ok := locMap[item.ItemID]
		if !ok {
			continue
		}

		data, err := ReadItemData(f, tree, loc)
		if err != nil {
			continue // Skip items we can't read
		}

		if item.ItemType == "Exif" {
			exifItems = append(exifItems, data)
		} else if item.ItemType == "mime" {
			xmpItems = append(xmpItems, data)
		}
	}

	return exifItems, xmpItems, nil
}
