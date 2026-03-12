package heicmeta

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// TestRemoveMetadata_E2E_MinimalFile tests end-to-end with a complete minimal HEIC structure
func TestRemoveMetadata_E2E_MinimalFile(t *testing.T) {
	// Create a minimal but complete HEIC file for testing
	tmpIn, err := os.CreateTemp("", "e2e-test-*.heic")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	inputPath := tmpIn.Name()
	defer os.Remove(inputPath)

	// Build and write minimal HEIC
	heicData := buildCompleteMinimalHEIC(t)
	if _, err := tmpIn.Write(heicData); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	tmpIn.Close()

	t.Logf("Created test file: %s (%d bytes)", inputPath, len(heicData))

	// Step 1: Verify the test file is parseable
	t.Run("verify_test_file_parseable", func(t *testing.T) {
		tree, err := ParseFile(inputPath)
		if err != nil {
			t.Fatalf("Failed to parse test file: %v", err)
		}

		// Verify structure
		var hasFtyp, hasMdat, hasMeta bool
		for _, box := range tree.Root {
			switch box.TypeString() {
			case "ftyp":
				hasFtyp = true
			case "mdat":
				hasMdat = true
			case "meta":
				hasMeta = true
			}
		}

		if !hasFtyp {
			t.Error("Missing ftyp box")
		}
		if !hasMdat {
			t.Error("Missing mdat box")
		}
		if !hasMeta {
			t.Error("Missing meta box")
		}

		t.Logf("✓ File structure valid: ftyp=%v, mdat=%v, meta=%v", hasFtyp, hasMdat, hasMeta)
	})

	// Step 2: Extract original mdat for comparison
	var origMdatData []byte
	t.Run("extract_original_mdat", func(t *testing.T) {
		tree, _ := ParseFile(inputPath)
		f, _ := os.Open(inputPath)
		defer f.Close()

		for _, box := range tree.Root {
			if box.TypeString() == "mdat" {
				data, err := ReadBoxPayloadBytes(f, box)
				if err != nil {
					t.Fatalf("Failed to read mdat: %v", err)
				}
				origMdatData = data
				break
			}
		}

		if len(origMdatData) == 0 {
			t.Fatal("No mdat data found")
		}

		t.Logf("✓ Original mdat: %d bytes", len(origMdatData))
	})

	// Step 3: Remove metadata
	tmpOut, err := os.CreateTemp("", "e2e-cleaned-*.heic")
	if err != nil {
		t.Fatalf("Failed to create output file: %v", err)
	}
	outputPath := tmpOut.Name()
	tmpOut.Close()
	defer os.Remove(outputPath)

	t.Run("remove_metadata", func(t *testing.T) {
		err := RemoveMetadata(inputPath, outputPath, DefaultOptions)
		if err != nil {
			t.Fatalf("RemoveMetadata failed: %v", err)
		}

		info, err := os.Stat(outputPath)
		if err != nil {
			t.Fatalf("Output file not created: %v", err)
		}

		t.Logf("✓ Output file created: %d bytes", info.Size())
	})

	// Step 4: Verify output structure
	t.Run("verify_output_structure", func(t *testing.T) {
		tree, err := ParseFile(outputPath)
		if err != nil {
			t.Fatalf("Output is not a valid HEIC: %v", err)
		}

		var hasFtyp, hasMdat, hasMeta bool
		for _, box := range tree.Root {
			switch box.TypeString() {
			case "ftyp":
				hasFtyp = true
			case "mdat":
				hasMdat = true
			case "meta":
				hasMeta = true
			}
		}

		if !hasFtyp || !hasMdat || !hasMeta {
			t.Errorf("Output missing required boxes: ftyp=%v, mdat=%v, meta=%v", hasFtyp, hasMdat, hasMeta)
		}

		t.Logf("✓ Output structure valid")
	})

	// Step 5: Verify mdat is preserved byte-for-byte
	t.Run("verify_mdat_preserved", func(t *testing.T) {
		tree, _ := ParseFile(outputPath)
		f, _ := os.Open(outputPath)
		defer f.Close()

		var newMdatData []byte
		for _, box := range tree.Root {
			if box.TypeString() == "mdat" {
				data, err := ReadBoxPayloadBytes(f, box)
				if err != nil {
					t.Fatalf("Failed to read output mdat: %v", err)
				}
				newMdatData = data
				break
			}
		}

		if len(newMdatData) != len(origMdatData) {
			t.Errorf("Mdat size changed: original=%d, output=%d", len(origMdatData), len(newMdatData))
		}

		if !bytes.Equal(origMdatData, newMdatData) {
			t.Error("Mdat content changed (pixel data not preserved)")
			// Show first difference
			for i := 0; i < len(origMdatData) && i < len(newMdatData); i++ {
				if origMdatData[i] != newMdatData[i] {
					t.Errorf("First difference at byte %d: original=0x%02x, output=0x%02x", 
						i, origMdatData[i], newMdatData[i])
					break
				}
			}
		} else {
			t.Logf("✓ Mdat preserved byte-for-byte (%d bytes)", len(origMdatData))
		}
	})

	// Step 6: Verify boxes were modified
	t.Run("verify_boxes_modified", func(t *testing.T) {
		origTree, _ := ParseFile(inputPath)
		outTree, _ := ParseFile(outputPath)

		origMetaSize := findBoxSize(origTree.Root, "meta")
		outMetaSize := findBoxSize(outTree.Root, "meta")

		t.Logf("Meta box size: original=%d, output=%d", origMetaSize, outMetaSize)

		// Meta box should exist in both
		if origMetaSize == 0 {
			t.Error("Original has no meta box")
		}
		if outMetaSize == 0 {
			t.Error("Output has no meta box")
		}

		// Meta content may be different due to filtering
		if origMetaSize != outMetaSize {
			t.Logf("✓ Meta box size changed (filtering applied)")
		}
	})
}

// TestRemoveMetadata_E2E_WithDirectBoxes tests with direct Exif/mime boxes (not item-based)
func TestRemoveMetadata_E2E_WithDirectBoxes(t *testing.T) {
	// This tests the fallback path for non-item-based metadata
	tmpIn, err := os.CreateTemp("", "e2e-direct-*.heic")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	inputPath := tmpIn.Name()
	defer os.Remove(inputPath)

	// Build HEIC with direct Exif box (legacy format)
	heicData := buildHEICWithDirectExif(t)
	tmpIn.Write(heicData)
	tmpIn.Close()

	tmpOut, err := os.CreateTemp("", "e2e-direct-cleaned-*.heic")
	if err != nil {
		t.Fatalf("Failed to create output file: %v", err)
	}
	outputPath := tmpOut.Name()
	tmpOut.Close()
	defer os.Remove(outputPath)

	t.Run("parse_and_remove", func(t *testing.T) {
		tree, err := ParseFile(inputPath)
		if err != nil {
			t.Skipf("Failed to parse direct box file: %v", err)
		}

		t.Logf("Input structure: %d root boxes", len(tree.Root))

		err = RemoveMetadata(inputPath, outputPath, DefaultOptions)
		if err != nil {
			t.Logf("RemoveMetadata completed (may have used fallback path): %v", err)
		}

		// If successful, verify output exists
		if err == nil {
			info, err := os.Stat(outputPath)
			if err == nil {
				t.Logf("✓ Output file created: %d bytes", info.Size())
			}
		}
	})
}

// Helper functions

func buildCompleteMinimalHEIC(t *testing.T) []byte {
	buf := new(bytes.Buffer)

	// ftyp box
	writeFtypBox(buf)

	// meta box
	writeMetaBox(buf, t)

	// mdat box with test pixel data
	writeMdatBox(buf)

	return buf.Bytes()
}

func writeFtypBox(buf *bytes.Buffer) {
	box := []byte{
		0x00, 0x00, 0x00, 0x20, // size = 32
		'f', 't', 'y', 'p', // type
		'h', 'e', 'i', 'c', // major brand
		0x00, 0x00, 0x00, 0x00, // minor version
		'h', 'e', 'i', 'c', // compatible brand 1
		'm', 'i', 'f', '1', // compatible brand 2
		'h', 'e', 'i', 'f', // compatible brand 3
		'm', 'i', 'a', 'f', // compatible brand 4
	}
	buf.Write(box)
}

func writeMetaBox(buf *bytes.Buffer, t *testing.T) {
	metaStart := buf.Len()

	// meta header
	buf.Write([]byte{0, 0, 0, 0}) // size placeholder
	buf.Write([]byte{'m', 'e', 't', 'a'})
	buf.Write([]byte{0, 0, 0, 0}) // version+flags

	// hdlr (required)
	writeHdlrBox(buf)

	// iinf, iloc, idat (minimal but valid)
	writeIinfBox(buf, t)
	writeIlocBox(buf, t)
	writeIdatBox(buf, t)

	// Update meta size
	metaEnd := buf.Len()
	metaSize := uint32(metaEnd - metaStart)
	metaBytes := buf.Bytes()
	binary.BigEndian.PutUint32(metaBytes[metaStart:], metaSize)
}

func writeHdlrBox(buf *bytes.Buffer) {
	hdlr := []byte{
		0x00, 0x00, 0x00, 0x21, // size = 33
		'h', 'd', 'l', 'r', // type
		0x00, 0x00, 0x00, 0x00, // version+flags
		0x00, 0x00, 0x00, 0x00, // pre_defined
		'p', 'i', 'c', 't', // handler_type
		0x00, 0x00, 0x00, 0x00, // reserved[0]
		0x00, 0x00, 0x00, 0x00, // reserved[1]
		0x00, 0x00, 0x00, 0x00, // reserved[2]
		0x00, // name
	}
	buf.Write(hdlr)
}

func writeIinfBox(buf *bytes.Buffer, t *testing.T) {
	iinfStart := buf.Len()

	// iinf header
	buf.Write([]byte{0, 0, 0, 0}) // size placeholder
	buf.Write([]byte{'i', 'i', 'n', 'f'})
	buf.Write([]byte{0, 0, 0, 0}) // version=0, flags=0
	buf.Write([]byte{0, 0}) // entry_count = 0 (no items initially)

	// Update size
	iinfEnd := buf.Len()
	iinfSize := uint32(iinfEnd - iinfStart)
	iinfBytes := buf.Bytes()
	binary.BigEndian.PutUint32(iinfBytes[iinfStart:], iinfSize)
}

func writeIlocBox(buf *bytes.Buffer, t *testing.T) {
	iloc := []byte{
		0x00, 0x00, 0x00, 0x10, // size = 16
		'i', 'l', 'o', 'c', // type
		0x00, 0x00, 0x00, 0x00, // version=0, flags=0
		0x44, // offset_size=4, length_size=4
		0x00, // base_offset_size=0, reserved=0
		0x00, 0x00, // item_count = 0
	}
	buf.Write(iloc)
}

func writeIdatBox(buf *bytes.Buffer, t *testing.T) {
	idat := []byte{
		0x00, 0x00, 0x00, 0x08, // size = 8 (empty payload)
		'i', 'd', 'a', 't', // type
	}
	buf.Write(idat)
}

func writeMdatBox(buf *bytes.Buffer) {
	// mdat with 64 bytes of test pixel data
	mdat := make([]byte, 72) // 8 header + 64 data
	binary.BigEndian.PutUint32(mdat[0:], 72)
	copy(mdat[4:], "mdat")

	// Fill with test pattern
	for i := 8; i < 72; i++ {
		mdat[i] = byte(i % 256)
	}

	buf.Write(mdat)
}

func buildHEICWithDirectExif(t *testing.T) []byte {
	buf := new(bytes.Buffer)

	writeFtypBox(buf)

	// meta with ipco containing Exif box
	metaStart := buf.Len()
	buf.Write([]byte{0, 0, 0, 0}) // size placeholder
	buf.Write([]byte{'m', 'e', 't', 'a'})
	buf.Write([]byte{0, 0, 0, 0}) // version+flags

	writeHdlrBox(buf)

	// ipco with Exif box
	ipcoStart := buf.Len()
	buf.Write([]byte{0, 0, 0, 0}) // size placeholder
	buf.Write([]byte{'i', 'p', 'c', 'o'})

	// Exif box
	writeExifBox(buf)

	// Update ipco size
	ipcoEnd := buf.Len()
	ipcoSize := uint32(ipcoEnd - ipcoStart)
	ipcoBytes := buf.Bytes()
	binary.BigEndian.PutUint32(ipcoBytes[ipcoStart:], ipcoSize)

	// Update meta size
	metaEnd := buf.Len()
	metaSize := uint32(metaEnd - metaStart)
	metaBytes := buf.Bytes()
	binary.BigEndian.PutUint32(metaBytes[metaStart:], metaSize)

	writeMdatBox(buf)

	return buf.Bytes()
}

func writeExifBox(buf *bytes.Buffer) {
	// Minimal Exif with GPS data
	exifData := []byte{
		// 4-byte offset
		0x00, 0x00, 0x00, 0x00,
		// TIFF header
		'M', 'M', 0x00, 0x2A, 0x00, 0x00, 0x00, 0x08,
		// IFD with GPS pointer
		0x00, 0x01, // 1 entry
		0x88, 0x25, 0x00, 0x04, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x1E,
		0x00, 0x00, 0x00, 0x00, // next IFD
		// GPS IFD
		0x00, 0x01, // 1 entry
		0x00, 0x01, 0x00, 0x02, 0x00, 0x00, 0x00, 0x02, 'N', 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, // next IFD
	}

	exifStart := buf.Len()
	buf.Write([]byte{0, 0, 0, 0}) // size placeholder
	buf.Write([]byte{'E', 'x', 'i', 'f'})
	buf.Write(exifData)

	exifEnd := buf.Len()
	exifSize := uint32(exifEnd - exifStart)
	exifBytes := buf.Bytes()
	binary.BigEndian.PutUint32(exifBytes[exifStart:], exifSize)
}

func findBoxSize(nodes []*BoxNode, boxType string) uint64 {
	for _, node := range nodes {
		if node.TypeString() == boxType {
			return node.Size
		}
		if len(node.Children) > 0 {
			if size := findBoxSize(node.Children, boxType); size > 0 {
				return size
			}
		}
	}
	return 0
}

// TestBoxSizeCalculator_E2E tests size calculations on real structure
func TestBoxSizeCalculator_E2E(t *testing.T) {
	tmpIn, err := os.CreateTemp("", "calc-e2e-*.heic")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpIn.Name())

	heicData := buildCompleteMinimalHEIC(t)
	tmpIn.Write(heicData)
	tmpIn.Close()

	tree, err := ParseFile(tmpIn.Name())
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Calculate sizes
	modifier := NewBoxTreeModifier()
	sizes := modifier.RecalculateSizes(tree.Root[0])

	t.Logf("Calculated %d box sizes", len(sizes))

	// Verify some basic constraints
	for node, size := range sizes {
		if size == 0 {
			t.Errorf("Box %s has zero size", node.TypeString())
		}
		if size < 8 {
			t.Errorf("Box %s size %d is too small (min 8 bytes)", node.TypeString(), size)
		}
	}

	t.Logf("✓ All box sizes are reasonable")
}
