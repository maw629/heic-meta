package heicmeta

import "errors"

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

type Metadata struct{}

type HEICHandler struct{}

func (h *HEICHandler) Parse(path string) (*BoxTree, error) {
	return ParseFile(path)
}

func (h *HEICHandler) ExtractMetadata(path string) (Metadata, error) {
	return Metadata{}, ErrNotImplemented
}

func (h *HEICHandler) RemoveMetadata(inputPath, outputPath string, options Options) error {
	return ErrNotImplemented
}

func ExtractMetadata(path string) (Metadata, error) {
	h := &HEICHandler{}
	return h.ExtractMetadata(path)
}

func RemoveMetadata(inputPath, outputPath string, options Options) error {
	h := &HEICHandler{}
	return h.RemoveMetadata(inputPath, outputPath, options)
}
