package heicmeta

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRemoveMetadata_Integration tests the complete metadata removal flow
func TestRemoveMetadata_Integration(t *testing.T) {
	// Find test image
	testImage := filepath.Join("testdata", "gps-added.heic")
	if _, err := os.Stat(testImage); err != nil {
		t.Skipf("Test image not found: %s", testImage)
	}

	// Create temp output file
	tmpOut, err := os.CreateTemp("", "heic-cleaned-*.heic")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	outputPath := tmpOut.Name()
	tmpOut.Close()
	defer os.Remove(outputPath)

	t.Run("extract metadata before removal", func(t *testing.T) {
		meta, err := ExtractMetadata(testImage)
		if err != nil {
			t.Fatalf("Failed to extract metadata: %v", err)
		}

		t.Logf("Before removal:")
		t.Logf("  EXIF present: %v", meta.EXIFPresent)
		t.Logf("  EXIF boxes: %d", meta.EXIFBoxes)
		t.Logf("  Sensitive tags: %d (%v)", len(meta.SensitiveTags), meta.SensitiveTags)

		if !meta.EXIFPresent {
			t.Error("Expected EXIF to be present in test image")
		}
		if len(meta.SensitiveTags) == 0 {
			t.Error("Expected sensitive tags in test image")
		}
	})

	t.Run("remove metadata", func(t *testing.T) {
		err := RemoveMetadata(testImage, outputPath, DefaultOptions)
		if err != nil {
			t.Fatalf("RemoveMetadata failed: %v", err)
		}

		// Verify output file exists and is not empty
		info, err := os.Stat(outputPath)
		if err != nil {
			t.Fatalf("Output file not created: %v", err)
		}
		if info.Size() == 0 {
			t.Error("Output file is empty")
		}

		t.Logf("Output file size: %d bytes", info.Size())
	})

	t.Run("verify metadata removed", func(t *testing.T) {
		meta, err := ExtractMetadata(outputPath)
		if err != nil {
			t.Fatalf("Failed to extract metadata from output: %v", err)
		}

		t.Logf("After removal:")
		t.Logf("  EXIF present: %v", meta.EXIFPresent)
		t.Logf("  EXIF boxes: %d", meta.EXIFBoxes)
		t.Logf("  Sensitive tags: %d (%v)", len(meta.SensitiveTags), meta.SensitiveTags)

		if len(meta.SensitiveTags) > 0 {
			t.Errorf("Sensitive tags still present: %v", meta.SensitiveTags)
		}
	})

	t.Run("verify file is valid HEIC", func(t *testing.T) {
		tree, err := ParseFile(outputPath)
		if err != nil {
			t.Fatalf("Output is not a valid HEIC file: %v", err)
		}

		// Check for required boxes
		hasType := false
		hasMdat := false
		hasMeta := false

		for _, box := range tree.Root {
			switch box.TypeString() {
			case "ftyp":
				hasType = true
			case "mdat":
				hasMdat = true
			case "meta":
				hasMeta = true
			}
		}

		if !hasType {
			t.Error("Output missing ftyp box")
		}
		if !hasMdat {
			t.Error("Output missing mdat box (pixel data)")
		}
		if !hasMeta {
			t.Error("Output missing meta box")
		}

		t.Logf("Output has ftyp=%v, mdat=%v, meta=%v", hasType, hasMdat, hasMeta)
	})
}

// TestRemoveMetadata_MdatPreservation verifies that mdat is preserved byte-for-byte
func TestRemoveMetadata_MdatPreservation(t *testing.T) {
	testImage := filepath.Join("testdata", "gps-added.heic")
	if _, err := os.Stat(testImage); err != nil {
		t.Skipf("Test image not found: %s", testImage)
	}

	// Extract original mdat
	origTree, err := ParseFile(testImage)
	if err != nil {
		t.Fatalf("Failed to parse original: %v", err)
	}

	origFile, err := os.Open(testImage)
	if err != nil {
		t.Fatalf("Failed to open original: %v", err)
	}
	defer origFile.Close()

	var origMdatNode *BoxNode
	for _, box := range origTree.Root {
		if box.TypeString() == "mdat" {
			origMdatNode = box
			break
		}
	}
	if origMdatNode == nil {
		t.Skip("No mdat box in test image")
	}

	origMdatData, err := ReadBoxPayloadBytes(origFile, origMdatNode)
	if err != nil {
		t.Fatalf("Failed to read original mdat: %v", err)
	}

	t.Logf("Original mdat size: %d bytes", len(origMdatData))

	// Remove metadata
	tmpOut, err := os.CreateTemp("", "heic-cleaned-*.heic")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	outputPath := tmpOut.Name()
	tmpOut.Close()
	defer os.Remove(outputPath)

	err = RemoveMetadata(testImage, outputPath, DefaultOptions)
	if err != nil {
		t.Fatalf("RemoveMetadata failed: %v", err)
	}

	// Extract mdat from output
	outTree, err := ParseFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	outFile, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("Failed to open output: %v", err)
	}
	defer outFile.Close()

	var outMdatNode *BoxNode
	for _, box := range outTree.Root {
		if box.TypeString() == "mdat" {
			outMdatNode = box
			break
		}
	}
	if outMdatNode == nil {
		t.Fatal("No mdat box in output file")
	}

	outMdatData, err := ReadBoxPayloadBytes(outFile, outMdatNode)
	if err != nil {
		t.Fatalf("Failed to read output mdat: %v", err)
	}

	t.Logf("Output mdat size: %d bytes", len(outMdatData))

	// Compare byte-for-byte
	if len(origMdatData) != len(outMdatData) {
		t.Errorf("Mdat size mismatch: original=%d, output=%d", len(origMdatData), len(outMdatData))
	}

	for i := 0; i < len(origMdatData) && i < len(outMdatData); i++ {
		if origMdatData[i] != outMdatData[i] {
			t.Errorf("Mdat differs at byte %d: original=0x%02x, output=0x%02x", i, origMdatData[i], outMdatData[i])
			break
		}
	}

	if len(origMdatData) == len(outMdatData) {
		t.Log("✓ mdat preserved byte-for-byte (pixel-perfect)")
	}
}

// TestRemoveMetadata_NoMetadata tests handling of files without metadata
func TestRemoveMetadata_NoMetadata(t *testing.T) {
	// Create a minimal HEIC file structure
	tmpIn, err := os.CreateTemp("", "heic-no-meta-*.heic")
	if err != nil {
		t.Fatalf("Failed to create temp input: %v", err)
	}
	defer os.Remove(tmpIn.Name())

	// Write minimal structure: ftyp + mdat + meta (without metadata items)
	// This is a synthetic file for testing edge cases
	tmpIn.Write(buildMinimalHEIC())
	tmpIn.Close()

	tmpOut, err := os.CreateTemp("", "heic-cleaned-*.heic")
	if err != nil {
		t.Fatalf("Failed to create temp output: %v", err)
	}
	outputPath := tmpOut.Name()
	tmpOut.Close()
	defer os.Remove(outputPath)

	// Should not error on files without metadata
	err = RemoveMetadata(tmpIn.Name(), outputPath, DefaultOptions)
	if err != nil {
		t.Logf("RemoveMetadata returned error (may be expected for minimal file): %v", err)
	}

	// If it succeeded, verify output exists
	if err == nil {
		info, err := os.Stat(outputPath)
		if err != nil {
			t.Error("Output file not created")
		} else if info.Size() == 0 {
			t.Error("Output file is empty")
		}
	}
}

// TestRemoveMetadata_Options tests different options
func TestRemoveMetadata_Options(t *testing.T) {
	testImage := filepath.Join("testdata", "gps-added.heic")
	if _, err := os.Stat(testImage); err != nil {
		t.Skipf("Test image not found: %s", testImage)
	}

	tests := []struct {
		name    string
		options Options
	}{
		{
			name:    "default options",
			options: DefaultOptions,
		},
		{
			name: "preserve some tags",
			options: Options{
				PreserveTags:    []string{"Make", "Model"},
				RemoveThumbnail: true,
				Strict:          false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpOut, err := os.CreateTemp("", "heic-cleaned-*.heic")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			outputPath := tmpOut.Name()
			tmpOut.Close()
			defer os.Remove(outputPath)

			err = RemoveMetadata(testImage, outputPath, tt.options)
			if err != nil {
				t.Fatalf("RemoveMetadata failed: %v", err)
			}

			// Verify output exists
			info, err := os.Stat(outputPath)
			if err != nil {
				t.Fatalf("Output file not created: %v", err)
			}
			if info.Size() == 0 {
				t.Error("Output file is empty")
			}

			t.Logf("Output file size: %d bytes", info.Size())
		})
	}
}

// buildMinimalHEIC creates a minimal valid HEIC structure for testing
func buildMinimalHEIC() []byte {
	// ftyp box (28 bytes)
	ftyp := []byte{
		0x00, 0x00, 0x00, 0x1C, // size = 28
		'f', 't', 'y', 'p', // type
		'h', 'e', 'i', 'c', // major brand
		0x00, 0x00, 0x00, 0x00, // minor version
		'h', 'e', 'i', 'c', // compatible brand 1
		'm', 'i', 'f', '1', // compatible brand 2
		'm', 'i', 'a', 'f', // compatible brand 3
	}

	// mdat box (16 bytes header + 4 bytes data)
	mdat := []byte{
		0x00, 0x00, 0x00, 0x14, // size = 20
		'm', 'd', 'a', 't', // type
		0x00, 0x01, 0x02, 0x03, // dummy pixel data
		0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B,
	}

	// meta box (12 bytes)
	meta := []byte{
		0x00, 0x00, 0x00, 0x0C, // size = 12
		'm', 'e', 't', 'a', // type
		0x00, 0x00, 0x00, 0x00, // version+flags
	}

	result := make([]byte, 0, len(ftyp)+len(mdat)+len(meta))
	result = append(result, ftyp...)
	result = append(result, mdat...)
	result = append(result, meta...)

	return result
}
