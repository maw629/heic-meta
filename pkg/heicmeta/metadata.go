package heicmeta

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sort"

	mp4 "github.com/abema/go-mp4"
)

var ErrNotImplemented = errors.New("not implemented")

type Options struct {
	PreserveTags    []string
	RemoveThumbnail bool
	Strict          bool
}

var DefaultOptions = Options{
	PreserveTags:    nil,
	RemoveThumbnail: true,
	Strict:          true,
}

type Metadata struct {
	EXIFPresent        bool
	XMPPresent         bool
	EXIFBoxes          int
	XMPBoxes           int
	EXIFSensitiveTags  []string
	XMPSensitiveFields []string
	SensitiveTags      []string
}

type HEICHandler struct{}

func (h *HEICHandler) Parse(path string) (*BoxTree, error) {
	return ParseFile(path)
}

func (h *HEICHandler) ExtractMetadata(path string) (Metadata, error) {
	tree, err := ParseFile(path)
	if err != nil {
		return Metadata{}, err
	}

	f, err := os.Open(path)
	if err != nil {
		return Metadata{}, err
	}
	defer f.Close()

	metadata := Metadata{}

	// Check for direct Exif/mime boxes in ipco
	exifNodes := findBoxesByType(tree.Root, mp4.StrToBoxType("Exif"))
	metadata.EXIFBoxes = len(exifNodes)
	metadata.EXIFPresent = len(exifNodes) > 0

	exifSet := make(map[string]struct{})
	for _, node := range exifNodes {
		payload, err := ReadBoxPayloadBytes(f, node)
		if err != nil {
			return Metadata{}, err
		}
		tags, err := DetectSensitiveEXIFTagsFromHEICPayload(payload)
		if err != nil {
			return Metadata{}, err
		}
		for _, tag := range tags {
			exifSet[tag] = struct{}{}
		}
	}

	xmpNodes := findBoxesByType(tree.Root, mp4.StrToBoxType("mime"))
	xmpSet := make(map[string]struct{})
	for _, node := range xmpNodes {
		payload, err := ReadBoxPayloadBytes(f, node)
		if err != nil {
			return Metadata{}, err
		}
		fields, isXMP, err := DetectSensitiveXMPFields(payload)
		if err != nil {
			return Metadata{}, err
		}
		if !isXMP {
			continue
		}
		metadata.XMPBoxes++
		for _, field := range fields {
			xmpSet[field] = struct{}{}
		}
	}

	// Check for item-based metadata (iinf/iloc/idat)
	exifItems, xmpItems, err := ExtractItemBasedMetadata(path, tree)
	if err == nil {
		for _, itemData := range exifItems {
			tags, err := DetectSensitiveEXIFTagsFromHEICPayload(itemData)
			if err == nil {
				for _, tag := range tags {
					exifSet[tag] = struct{}{}
				}
			}
		}
		metadata.EXIFBoxes += len(exifItems)
		if len(exifItems) > 0 {
			metadata.EXIFPresent = true
		}

		for _, itemData := range xmpItems {
			fields, isXMP, err := DetectSensitiveXMPFields(itemData)
			if err == nil && isXMP {
				for _, field := range fields {
					xmpSet[field] = struct{}{}
				}
				metadata.XMPBoxes++
			}
		}
	}

	metadata.EXIFSensitiveTags = sortedKeys(exifSet)
	metadata.XMPSensitiveFields = sortedKeys(xmpSet)
	metadata.XMPPresent = metadata.XMPBoxes > 0

	allSet := make(map[string]struct{}, len(metadata.EXIFSensitiveTags)+len(metadata.XMPSensitiveFields))
	for _, tag := range metadata.EXIFSensitiveTags {
		allSet[tag] = struct{}{}
	}
	for _, tag := range metadata.XMPSensitiveFields {
		allSet[tag] = struct{}{}
	}
	metadata.SensitiveTags = sortedKeys(allSet)

	return metadata, nil
}

func (h *HEICHandler) RemoveMetadata(inputPath, outputPath string, options Options) error {
	// Step 4: Complete implementation with item-based metadata removal
	
	// Parse to understand structure
	tree, err := ParseFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to parse input: %w", err)
	}

	// Open input file for reading
	inFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input: %w", err)
	}
	defer inFile.Close()

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output: %w", err)
	}
	defer func() {
		outFile.Close()
		// Remove output file if there was an error
		if err != nil {
			os.Remove(outputPath)
		}
	}()

	// Find iinf and iloc boxes (search all root boxes)
	var iinfNode, ilocNode *BoxNode
	for _, root := range tree.Root {
		if iinfNode == nil {
			iinfNode = FindBoxInTree(root, "iinf")
		}
		if ilocNode == nil {
			ilocNode = FindBoxInTree(root, "iloc")
		}
		if iinfNode != nil && ilocNode != nil {
			break
		}
	}

	// If no iinf or iloc, fall back to direct box filtering
	if iinfNode == nil || ilocNode == nil {
		return h.removeMetadataDirectBoxes(tree, inFile, outFile, options)
	}

	// Step 1: Parse item information
	items, err := ParseItemInfo(inFile, iinfNode)
	if err != nil {
		return fmt.Errorf("failed to parse item info: %w", err)
	}

	locations, err := ParseItemLocation(inFile, ilocNode)
	if err != nil {
		return fmt.Errorf("failed to parse item location: %w", err)
	}

	// Step 2: Read all item data
	itemData := make(map[uint32][]byte)
	for _, item := range items {
		// Find location for this item
		var itemLoc *ItemLocation
		for i := range locations {
			if locations[i].ItemID == item.ItemID {
				itemLoc = &locations[i]
				break
			}
		}
		if itemLoc == nil {
			continue
		}

		data, err := ReadItemData(inFile, tree, *itemLoc)
		if err != nil {
			// Item may not have data (e.g., not metadata)
			continue
		}
		itemData[item.ItemID] = data
	}

	// Step 3: Filter items
	modifications, err := FilterItems(items, itemData)
	if err != nil {
		return fmt.Errorf("failed to filter items: %w", err)
	}

	// Step 4: Build filtered idat with new offsets
	newIdatPayload, newOffsets, err := BuildFilteredIdat(inFile, tree, items, locations, modifications)
	if err != nil {
		return fmt.Errorf("failed to build filtered idat: %w", err)
	}

	// Step 5: Rebuild iinf and iloc boxes
	newIinfPayload, err := RebuildIinfBox(items, modifications)
	if err != nil {
		return fmt.Errorf("failed to rebuild iinf: %w", err)
	}

	newIlocPayload, err := RebuildIlocBox(locations, modifications, newOffsets)
	if err != nil {
		return fmt.Errorf("failed to rebuild iloc: %w", err)
	}

	// Step 6: Set up box replacements and recalculate sizes
	modifier := NewBoxTreeModifier()
	
	// Find and replace idat (search all root boxes)
	var idatNode *BoxNode
	for _, root := range tree.Root {
		idatNode = FindBoxInTree(root, "idat")
		if idatNode != nil {
			break
		}
	}
	if idatNode != nil {
		modifier.ReplaceBox(idatNode, newIdatPayload)
	}
	
	// Replace iinf with updated item info
	modifier.ReplaceBox(iinfNode, newIinfPayload)
	
	// Replace iloc with updated item locations
	modifier.ReplaceBox(ilocNode, newIlocPayload)

	// Recalculate sizes for all root boxes that were modified
	// This ensures all parent boxes have correct sizes
	modified := make(map[*BoxNode]bool)
	modified[iinfNode] = true
	modified[ilocNode] = true
	if idatNode != nil {
		modified[idatNode] = true
	}
	
	// Find all ancestor boxes and recalculate their sizes
	for _, root := range tree.Root {
		modifier.RecalculateSizes(root)
	}

	// Step 7: Write modified file
	writer := &FileWriter{
		input:    inFile,
		output:   outFile,
		modifier: modifier,
	}

	if err = writer.WriteTree(tree); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}

// removeMetadataDirectBoxes handles files without item-based metadata
// (legacy format or files with only direct Exif/mime boxes)
func (h *HEICHandler) removeMetadataDirectBoxes(tree *BoxTree, inFile *os.File, outFile *os.File, options Options) error {
	// Process boxes
	for _, box := range tree.Root {
		if err := h.copyBoxWithFilter(box, inFile, outFile, options); err != nil {
			outFile.Close()
			os.Remove(outFile.Name())
			return fmt.Errorf("failed to process: %w", err)
		}
	}
	return nil
}

func (h *HEICHandler) copyBoxWithFilter(box *BoxNode, inFile *os.File, outFile *os.File, options Options) error {
	boxType := box.TypeString()

	// Special handling for metadata boxes
	if boxType == "Exif" {
		return h.processExifBox(box, inFile, outFile, options)
	}

	if boxType == "mime" {
		return h.processMimeBox(box, inFile, outFile, options)
	}

	// For all other boxes (including containers), copy as-is
	// This preserves the structure and mdat pixel data
	boxData, err := ReadBoxBytes(inFile, box)
	if err != nil {
		return fmt.Errorf("failed to read box %s: %w", boxType, err)
	}

	if _, err := outFile.Write(boxData); err != nil {
		return fmt.Errorf("failed to write box %s: %w", boxType, err)
	}

	return nil
}

func (h *HEICHandler) processExifBox(box *BoxNode, inFile *os.File, outFile *os.File, options Options) error {
	payload, err := ReadBoxPayloadBytes(inFile, box)
	if err != nil {
		return err
	}

	// Filter sensitive EXIF tags
	filteredPayload, err := FilterSensitiveEXIF(payload)
	if err != nil {
		// If filtering fails, skip the box entirely
		return nil
	}

	// If all tags were sensitive, skip the box
	if filteredPayload == nil {
		return nil
	}

	// Write box with filtered payload
	return h.writeBox(box.Type, filteredPayload, outFile)
}

func (h *HEICHandler) processMimeBox(box *BoxNode, inFile *os.File, outFile *os.File, options Options) error {
	payload, err := ReadBoxPayloadBytes(inFile, box)
	if err != nil {
		return err
	}

	// Check if it's XMP
	_, isXMP, err := DetectSensitiveXMPFields(payload)
	if err != nil || !isXMP {
		// Not XMP or error, copy as-is
		boxData, err := ReadBoxBytes(inFile, box)
		if err != nil {
			return err
		}
		_, err = outFile.Write(boxData)
		return err
	}

	// Filter sensitive XMP
	filteredPayload, err := FilterSensitiveXMP(payload)
	if err != nil {
		// If filtering fails, skip the box
		return nil
	}

	// If all data was sensitive, skip the box
	if filteredPayload == nil {
		return nil
	}

	// Write box with filtered payload
	return h.writeBox(box.Type, filteredPayload, outFile)
}


func (h *HEICHandler) writeBox(boxType mp4.BoxType, payload []byte, outFile *os.File) error {
	// Calculate size: 8 bytes header + payload length
	size := uint32(8 + len(payload))
	
	// Write size
	sizeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBytes, size)
	if _, err := outFile.Write(sizeBytes); err != nil {
		return err
	}
	
	// Write type
	typeBytes := []byte(boxType.String())
	if _, err := outFile.Write(typeBytes); err != nil {
		return err
	}
	
	// Write payload
	if _, err := outFile.Write(payload); err != nil {
		return err
	}
	
	return nil
}

func ExtractMetadata(path string) (Metadata, error) {
	h := &HEICHandler{}
	return h.ExtractMetadata(path)
}

// PreviewMetadata is an alias for ExtractMetadata, aligned with Step 2 terminology.
func (h *HEICHandler) PreviewMetadata(path string) (Metadata, error) {
	return h.ExtractMetadata(path)
}

// PreviewMetadata is an alias for ExtractMetadata, aligned with Step 2 terminology.
func PreviewMetadata(path string) (Metadata, error) {
	return ExtractMetadata(path)
}

func RemoveMetadata(inputPath, outputPath string, options Options) error {
	h := &HEICHandler{}
	return h.RemoveMetadata(inputPath, outputPath, options)
}

func findBoxesByType(nodes []*BoxNode, boxType mp4.BoxType) []*BoxNode {
	matches := make([]*BoxNode, 0, 2)
	for _, node := range nodes {
		if node.Type == boxType {
			matches = append(matches, node)
		}
		if len(node.Children) > 0 {
			matches = append(matches, findBoxesByType(node.Children, boxType)...)
		}
	}
	return matches
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
