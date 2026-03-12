package heicmeta

import (
	"errors"
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
	return ErrNotImplemented
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
