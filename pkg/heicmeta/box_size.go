package heicmeta

import (
	"fmt"
)

// BoxSizeCalculator handles recalculating box sizes after modifications.
type BoxSizeCalculator struct {
	sizeOverrides map[*BoxNode]uint64
}

// NewBoxSizeCalculator creates a new size calculator.
func NewBoxSizeCalculator() *BoxSizeCalculator {
	return &BoxSizeCalculator{
		sizeOverrides: make(map[*BoxNode]uint64),
	}
}

// SetSize overrides the size for a specific box.
// This is used when we've modified a box's content (e.g., idat, iinf, iloc).
func (c *BoxSizeCalculator) SetSize(node *BoxNode, newSize uint64) {
	c.sizeOverrides[node] = newSize
}

// CalculateSize calculates the total size of a box including header and children.
// Returns the new size for the box.
func (c *BoxSizeCalculator) CalculateSize(node *BoxNode) uint64 {
	// If we have an override, use it
	if overrideSize, exists := c.sizeOverrides[node]; exists {
		return overrideSize
	}

	// If box has no children, return original size (leaf box)
	if len(node.Children) == 0 {
		return node.Size
	}

	// Calculate size of all children
	var childrenSize uint64
	for _, child := range node.Children {
		childrenSize += c.CalculateSize(child)
	}

	// Add header size
	// Header is: size(4) + type(4) = 8 bytes
	// Or: size(4) + type(4) + extended_size(8) = 16 bytes if size > 4GB
	// For meta and iref, add version+flags (4 bytes) after type

	headerSize := node.HeaderSize
	if headerSize == 0 {
		// Estimate: most boxes have 8-byte header
		headerSize = 8
	}

	// For FullBox containers, add version+flags to payload
	totalPayload := childrenSize
	if isFullBoxContainerType(node.Type) {
		totalPayload += 4 // version(1) + flags(3)
	}

	totalSize := headerSize + totalPayload

	return totalSize
}

// RecalculateAllSizes performs a bottom-up traversal and recalculates all sizes.
// Returns a map of node -> new size.
func (c *BoxSizeCalculator) RecalculateAllSizes(root *BoxNode) map[*BoxNode]uint64 {
	result := make(map[*BoxNode]uint64)
	c.recalculateRecursive(root, result)
	return result
}

func (c *BoxSizeCalculator) recalculateRecursive(node *BoxNode, result map[*BoxNode]uint64) uint64 {
	// If we have an override, use it and don't recurse
	if overrideSize, exists := c.sizeOverrides[node]; exists {
		result[node] = overrideSize
		return overrideSize
	}

	// If no children, use original size
	if len(node.Children) == 0 {
		result[node] = node.Size
		return node.Size
	}

	// Calculate children first (bottom-up)
	var childrenSize uint64
	for _, child := range node.Children {
		childrenSize += c.recalculateRecursive(child, result)
	}

	// Calculate this box's size
	headerSize := node.HeaderSize
	if headerSize == 0 {
		headerSize = 8 // Default
	}

	// For FullBox containers, add version+flags to payload
	totalPayload := childrenSize
	if isFullBoxContainerType(node.Type) {
		totalPayload += 4 // version(1) + flags(3)
	}

	totalSize := headerSize + totalPayload
	result[node] = totalSize

	return totalSize
}

// BoxTreeModifier handles modifying a box tree with new sizes and content.
type BoxTreeModifier struct {
	calculator   *BoxSizeCalculator
	replacements map[*BoxNode][]byte // Box -> new payload
}

// NewBoxTreeModifier creates a new tree modifier.
func NewBoxTreeModifier() *BoxTreeModifier {
	return &BoxTreeModifier{
		calculator:   NewBoxSizeCalculator(),
		replacements: make(map[*BoxNode][]byte),
	}
}

// ReplaceBox marks a box to be replaced with new content.
func (m *BoxTreeModifier) ReplaceBox(node *BoxNode, newPayload []byte) {
	m.replacements[node] = newPayload

	// Calculate new size: header + payload
	headerSize := node.HeaderSize
	if headerSize == 0 {
		headerSize = 8
	}
	newSize := headerSize + uint64(len(newPayload))

	m.calculator.SetSize(node, newSize)
}

// GetCalculator returns the size calculator.
func (m *BoxTreeModifier) GetCalculator() *BoxSizeCalculator {
	return m.calculator
}

// GetReplacement returns the replacement payload for a box.
func (m *BoxTreeModifier) GetReplacement(node *BoxNode) ([]byte, bool) {
	payload, exists := m.replacements[node]
	return payload, exists
}

// RecalculateSizes recalculates all sizes in the tree.
func (m *BoxTreeModifier) RecalculateSizes(root *BoxNode) map[*BoxNode]uint64 {
	return m.calculator.RecalculateAllSizes(root)
}

// ValidateBoxSize checks if a calculated size is reasonable.
func ValidateBoxSize(boxType string, size uint64) error {
	// Basic sanity checks
	if size < 8 {
		return fmt.Errorf("box size too small: %d (must be at least 8)", size)
	}

	// Header size check
	if size > 0xFFFFFFFF {
		// Would need extended size, header should be 16
		if size < 16 {
			return fmt.Errorf("extended size box too small: %d", size)
		}
	}

	// Type-specific checks
	switch boxType {
	case "ftyp":
		// ftyp should have at least major brand (4) + minor version (4) = 8 bytes payload
		if size < 16 {
			return fmt.Errorf("ftyp box too small: %d", size)
		}
	case "meta":
		// meta is a FullBox (version+flags = 4 bytes) + children
		if size < 12 {
			return fmt.Errorf("meta box too small: %d", size)
		}
	}

	return nil
}

// EstimateBoxHeaderSize estimates the header size for a box.
func EstimateBoxHeaderSize(payloadSize uint64, isFullBox bool) uint64 {
	headerSize := uint64(8) // size(4) + type(4)

	if payloadSize+headerSize > 0xFFFFFFFF {
		// Need extended size
		headerSize = 16 // size(4) + type(4) + extended_size(8)
	}

	// Note: FullBox version+flags (4 bytes) is part of payload, not header

	return headerSize
}

// CalculateContainerSize calculates the size of a container box from its children.
func CalculateContainerSize(children []*BoxNode, childSizes map[*BoxNode]uint64, isFullBox bool) uint64 {
	var childrenTotal uint64
	for _, child := range children {
		if size, exists := childSizes[child]; exists {
			childrenTotal += size
		} else {
			childrenTotal += child.Size
		}
	}

	// Add version+flags for FullBox containers (part of payload)
	payloadSize := childrenTotal
	if isFullBox {
		payloadSize += 4 // version(1) + flags(3)
	}

	// Calculate header size
	headerSize := EstimateBoxHeaderSize(payloadSize, isFullBox)

	return headerSize + payloadSize
}

// UpdateBoxTreeSizes updates the Size field in all BoxNodes based on calculations.
func UpdateBoxTreeSizes(root *BoxNode, newSizes map[*BoxNode]uint64) {
	if newSize, exists := newSizes[root]; exists {
		root.Size = newSize
	}

	for _, child := range root.Children {
		UpdateBoxTreeSizes(child, newSizes)
	}
}
