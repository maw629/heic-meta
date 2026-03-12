package heicmeta

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// FileWriter handles writing a modified HEIC file.
type FileWriter struct {
	input      *os.File
	output     *os.File
	modifier   *BoxTreeModifier
	newSizes   map[*BoxNode]uint64
	mdatCopied bool
}

// NewFileWriter creates a new file writer.
func NewFileWriter(inputPath, outputPath string, modifier *BoxTreeModifier) (*FileWriter, error) {
	input, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open input: %w", err)
	}
	
	output, err := os.Create(outputPath)
	if err != nil {
		input.Close()
		return nil, fmt.Errorf("failed to create output: %w", err)
	}
	
	return &FileWriter{
		input:    input,
		output:   output,
		modifier: modifier,
		newSizes: make(map[*BoxNode]uint64),
	}, nil
}

// Close closes both input and output files.
func (w *FileWriter) Close() error {
	var err1, err2 error
	if w.input != nil {
		err1 = w.input.Close()
	}
	if w.output != nil {
		err2 = w.output.Close()
	}
	
	if err1 != nil {
		return err1
	}
	return err2
}

// WriteTree writes the entire box tree to output.
func (w *FileWriter) WriteTree(tree *BoxTree) error {
	// Initialize newSizes map if not already done
	if w.newSizes == nil {
		w.newSizes = make(map[*BoxNode]uint64)
	}

	// Calculate all new sizes first
	for _, root := range tree.Root {
		sizes := w.modifier.RecalculateSizes(root)
		for node, size := range sizes {
			w.newSizes[node] = size
		}
	}
	
	// Write each root box
	for _, root := range tree.Root {
		if err := w.writeBox(root); err != nil {
			return fmt.Errorf("failed to write box %s: %w", root.TypeString(), err)
		}
	}
	
	return nil
}

// writeBox writes a single box and its children.
func (w *FileWriter) writeBox(node *BoxNode) error {
	boxType := node.TypeString()
	
	// Special handling for mdat (pixel data - must copy unchanged)
	if boxType == "mdat" {
		return w.writeMdatBox(node)
	}
	
	// Check if we have a replacement for this box
	if replacement, exists := w.modifier.GetReplacement(node); exists {
		return w.writeReplacedBox(node, replacement)
	}
	
	// If box has children, write as container
	if len(node.Children) > 0 {
		return w.writeContainerBox(node)
	}
	
	// Leaf box without replacement - copy as-is
	return w.copyBoxAsIs(node)
}

// writeMdatBox copies mdat box unchanged (pixel-perfect preservation).
func (w *FileWriter) writeMdatBox(node *BoxNode) error {
	// Seek to mdat in input
	if _, err := w.input.Seek(int64(node.Offset), io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to mdat: %w", err)
	}
	
	// Copy entire mdat box (header + data)
	if _, err := io.CopyN(w.output, w.input, int64(node.Size)); err != nil {
		return fmt.Errorf("failed to copy mdat: %w", err)
	}
	
	w.mdatCopied = true
	return nil
}

// writeReplacedBox writes a box with new payload.
func (w *FileWriter) writeReplacedBox(node *BoxNode, newPayload []byte) error {
	// Get new size (should be in newSizes)
	newSize := w.newSizes[node]
	if newSize == 0 {
		// Calculate: header + payload
		headerSize := node.HeaderSize
		if headerSize == 0 {
			headerSize = 8
		}
		newSize = headerSize + uint64(len(newPayload))
	}
	
	// Write header
	if err := w.writeBoxHeaderWithSize(node.TypeString(), newSize); err != nil {
		return err
	}
	
	// Note: If this is a FullBox (iinf, iloc), version+flags should be
	// included in newPayload already (our rebuild functions include it)
	
	// Write new payload
	if _, err := w.output.Write(newPayload); err != nil {
		return fmt.Errorf("failed to write payload: %w", err)
	}
	
	return nil
}

// writeContainerBox writes a container box with children.
func (w *FileWriter) writeContainerBox(node *BoxNode) error {
	// Get new size
	newSize := w.newSizes[node]
	if newSize == 0 {
		// This shouldn't happen if RecalculateSizes was called
		return fmt.Errorf("no size calculated for container %s", node.TypeString())
	}
	
	// Write header
	if err := w.writeBoxHeaderWithSize(node.TypeString(), newSize); err != nil {
		return err
	}
	
	// Write version+flags if FullBox container
	if isFullBoxContainerType(node.Type) {
		// Read original version+flags from input
		versionFlags, err := w.readVersionFlags(node)
		if err != nil {
			// Use defaults if can't read
			versionFlags = []byte{0x00, 0x00, 0x00, 0x00}
		}
		if _, err := w.output.Write(versionFlags); err != nil {
			return fmt.Errorf("failed to write version+flags: %w", err)
		}
	}
	
	// Write children
	for _, child := range node.Children {
		if err := w.writeBox(child); err != nil {
			return err
		}
	}
	
	return nil
}

// copyBoxAsIs copies a box from input to output unchanged.
func (w *FileWriter) copyBoxAsIs(node *BoxNode) error {
	// Seek to box in input
	if _, err := w.input.Seek(int64(node.Offset), io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}
	
	// Copy entire box
	if _, err := io.CopyN(w.output, w.input, int64(node.Size)); err != nil {
		return fmt.Errorf("failed to copy box: %w", err)
	}
	
	return nil
}

// writeBoxHeaderWithSize writes a box header with the specified size.
func (w *FileWriter) writeBoxHeaderWithSize(boxType string, size uint64) error {
	// Prepare type bytes
	typeBytes := []byte(boxType)
	if len(typeBytes) != 4 {
		return fmt.Errorf("invalid box type length: %d", len(typeBytes))
	}
	
	// Check if we need extended size
	if size > 0xFFFFFFFF {
		// Extended size format:
		// size = 1 (4 bytes)
		// type (4 bytes)
		// actual size (8 bytes)
		binary.Write(w.output, binary.BigEndian, uint32(1))
		w.output.Write(typeBytes)
		binary.Write(w.output, binary.BigEndian, size)
	} else {
		// Normal format:
		// size (4 bytes)
		// type (4 bytes)
		binary.Write(w.output, binary.BigEndian, uint32(size))
		w.output.Write(typeBytes)
	}
	
	return nil
}

// readVersionFlags reads version+flags from a FullBox in the input file.
func (w *FileWriter) readVersionFlags(node *BoxNode) ([]byte, error) {
	// Seek to payload start (after header)
	payloadOffset := node.Offset + node.HeaderSize
	if _, err := w.input.Seek(int64(payloadOffset), io.SeekStart); err != nil {
		return nil, err
	}
	
	// Read 4 bytes (version + flags)
	versionFlags := make([]byte, 4)
	if _, err := io.ReadFull(w.input, versionFlags); err != nil {
		return nil, err
	}
	
	return versionFlags, nil
}

// GetMdatCopied returns whether mdat was copied.
func (w *FileWriter) GetMdatCopied() bool {
	return w.mdatCopied
}

// WriteModifiedFile is a high-level function to write a modified HEIC file.
func WriteModifiedFile(inputPath, outputPath string, tree *BoxTree, modifier *BoxTreeModifier) error {
	writer, err := NewFileWriter(inputPath, outputPath, modifier)
	if err != nil {
		return err
	}
	defer writer.Close()
	
	if err := writer.WriteTree(tree); err != nil {
		return err
	}
	
	// Verify mdat was copied
	if !writer.GetMdatCopied() {
		return fmt.Errorf("mdat not found or not copied")
	}
	
	return nil
}
