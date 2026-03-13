package heicmeta

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"testing"
)

func TestFileWriter_WriteSimpleFile(t *testing.T) {
	// Create a simple test input file
	inputFile, err := os.CreateTemp("", "test-input-*.heic")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(inputFile.Name())
	
	// Write a simple structure: ftyp + mdat
	// ftyp box
	ftypData := buildFtypBox()
	inputFile.Write(ftypData)
	
	// mdat box with some data
	mdatData := buildMdatBox([]byte("pixel data here"))
	inputFile.Write(mdatData)
	
	inputFile.Close()
	
	// Parse the file
	tree, err := ParseFile(inputFile.Name())
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	
	// Create output file
	outputFile, err := os.CreateTemp("", "test-output-*.heic")
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}
	outputPath := outputFile.Name()
	outputFile.Close()
	defer os.Remove(outputPath)
	
	// Write with no modifications
	modifier := NewBoxTreeModifier()
	err = WriteModifiedFile(inputFile.Name(), outputPath, tree, modifier)
	if err != nil {
		t.Fatalf("WriteModifiedFile failed: %v", err)
	}
	
	// Verify output file size matches input
	inInfo, _ := os.Stat(inputFile.Name())
	outInfo, _ := os.Stat(outputPath)
	
	if outInfo.Size() != inInfo.Size() {
		t.Errorf("Output size %d != input size %d", outInfo.Size(), inInfo.Size())
	}
}

func TestFileWriter_WithBoxReplacement(t *testing.T) {
	// Create input file with a meta box containing idat
	inputFile, err := os.CreateTemp("", "test-input-*.heic")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(inputFile.Name())
	
	// Write structure: ftyp + meta(idat) + mdat
	inputFile.Write(buildFtypBox())
	
	// Build meta box manually
	metaStart, _ := inputFile.Seek(0, io.SeekCurrent)
	
	// meta header (placeholder)
	metaHeaderPos, _ := inputFile.Seek(0, io.SeekCurrent)
	inputFile.Write(make([]byte, 8)) // size + type
	
	// meta version+flags
	inputFile.Write([]byte{0x00, 0x00, 0x00, 0x00})
	
	// idat child
	idatData := []byte("original item data")
	idatBox := buildGenericBox("idat", idatData)
	inputFile.Write(idatBox)
	
	// Go back and write meta size
	metaEnd, _ := inputFile.Seek(0, io.SeekCurrent)
	metaSize := uint32(metaEnd - metaStart)
	inputFile.Seek(metaHeaderPos, io.SeekStart)
	binary.Write(inputFile, binary.BigEndian, metaSize)
	inputFile.Write([]byte("meta"))
	inputFile.Seek(metaEnd, io.SeekStart)
	
	// mdat
	inputFile.Write(buildMdatBox([]byte("pixel data")))
	
	inputFile.Close()
	
	// Parse
	tree, err := ParseFile(inputFile.Name())
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	
	// Find idat box
	var idatNode *BoxNode
	for _, root := range tree.Root {
		idatNode = FindBoxInTree(root, "idat")
		if idatNode != nil {
			break
		}
	}
	
	if idatNode == nil {
		t.Fatal("idat box not found")
	}
	
	// Modify idat with new data
	modifier := NewBoxTreeModifier()
	newIdatPayload := []byte("modified item data")
	modifier.ReplaceBox(idatNode, newIdatPayload)
	
	// Write output
	outputFile, err := os.CreateTemp("", "test-output-*.heic")
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}
	outputPath := outputFile.Name()
	outputFile.Close()
	defer os.Remove(outputPath)
	
	err = WriteModifiedFile(inputFile.Name(), outputPath, tree, modifier)
	if err != nil {
		t.Fatalf("WriteModifiedFile failed: %v", err)
	}
	
	// Verify output is smaller (modified data is shorter)
	inInfo, _ := os.Stat(inputFile.Name())
	outInfo, _ := os.Stat(outputPath)
	
	sizeReduction := len("original item data") - len("modified item data")
	expectedSize := inInfo.Size() - int64(sizeReduction)
	
	if outInfo.Size() != expectedSize {
		t.Errorf("Output size %d != expected %d (reduction: %d)", 
			outInfo.Size(), expectedSize, sizeReduction)
	}
}

func TestFileWriter_MdatPreservation(t *testing.T) {
	// Create input with mdat containing specific pixel data
	inputFile, err := os.CreateTemp("", "test-input-*.heic")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(inputFile.Name())
	
	// Specific pixel data pattern
	pixelData := []byte("PIXEL_DATA_PATTERN_12345")
	
	inputFile.Write(buildFtypBox())
	inputFile.Write(buildMdatBox(pixelData))
	inputFile.Close()
	
	// Parse and write
	tree, _ := ParseFile(inputFile.Name())
	
	outputFile, _ := os.CreateTemp("", "test-output-*.heic")
	outputPath := outputFile.Name()
	outputFile.Close()
	defer os.Remove(outputPath)
	
	modifier := NewBoxTreeModifier()
	WriteModifiedFile(inputFile.Name(), outputPath, tree, modifier)
	
	// Read output and verify mdat content
	outFile, _ := os.Open(outputPath)
	defer outFile.Close()
	
	// Parse output
	outTree, _ := ParseFile(outputPath)
	
	// Find mdat
	var mdatNode *BoxNode
	for _, node := range outTree.Mdats {
		mdatNode = node
		break
	}
	
	if mdatNode == nil {
		t.Fatal("mdat not found in output")
	}
	
	// Read mdat payload
	payloadOffset := mdatNode.Offset + mdatNode.HeaderSize
	outFile.Seek(int64(payloadOffset), io.SeekStart)
	
	mdatPayload := make([]byte, mdatNode.Size-mdatNode.HeaderSize)
	io.ReadFull(outFile, mdatPayload)
	
	// Verify exact match
	if !bytes.Equal(mdatPayload, pixelData) {
		t.Errorf("mdat payload mismatch:\nExpected: %v\nGot: %v", pixelData, mdatPayload)
	}
}

func TestWriteBoxHeaderWithSize(t *testing.T) {
	tests := []struct {
		name     string
		boxType  string
		size     uint64
		wantSize int
	}{
		{"normal size", "test", 100, 8},
		{"large size", "test", 0x7FFFFFFF, 8},
		{"extended size", "test", 0x100000000, 16},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create output file for testing
			tmpFile, err := os.CreateTemp("", "test-header-*.bin")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())
			
			writer := &FileWriter{output: tmpFile}
			
			err = writer.writeBoxHeaderWithSize(tt.boxType, tt.size)
			if err != nil {
				t.Fatalf("writeBoxHeaderWithSize failed: %v", err)
			}
			
			// Read back and verify
			tmpFile.Seek(0, io.SeekStart)
			header := make([]byte, tt.wantSize)
			io.ReadFull(tmpFile, header)
			tmpFile.Close()
			
			if len(header) != tt.wantSize {
				t.Errorf("Header size: got %d, want %d", len(header), tt.wantSize)
			}
			var typePos int
			if tt.size > 0xFFFFFFFF {
				typePos = 4 // After size field (which is 1)
			} else {
				typePos = 4 // After size field
			}
			
			if string(header[typePos:typePos+4]) != tt.boxType {
				t.Errorf("Type: got %s, want %s", 
					string(header[typePos:typePos+4]), tt.boxType)
			}
		})
	}
}

// Helper functions

func buildFtypBox() []byte {
	buf := new(bytes.Buffer)
	
	// ftyp box
	ftypPayload := []byte{
		'h', 'e', 'i', 'c', // major brand
		0x00, 0x00, 0x00, 0x00, // minor version
		'h', 'e', 'i', 'c', // compatible brand
		'm', 'i', 'f', '1', // compatible brand
	}
	
	size := uint32(8 + len(ftypPayload))
	binary.Write(buf, binary.BigEndian, size)
	buf.Write([]byte("ftyp"))
	buf.Write(ftypPayload)
	
	return buf.Bytes()
}

func buildMdatBox(data []byte) []byte {
	buf := new(bytes.Buffer)
	
	size := uint32(8 + len(data))
	binary.Write(buf, binary.BigEndian, size)
	buf.Write([]byte("mdat"))
	buf.Write(data)
	
	return buf.Bytes()
}

func buildGenericBox(boxType string, data []byte) []byte {
	buf := new(bytes.Buffer)
	
	size := uint32(8 + len(data))
	binary.Write(buf, binary.BigEndian, size)
	buf.Write([]byte(boxType))
	buf.Write(data)
	
	return buf.Bytes()
}
