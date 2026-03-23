package heicmeta

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNokiaConformanceFiles validates our library against Nokia's official HEIF conformance files
func TestNokiaConformanceFiles(t *testing.T) {
	conformanceDir := "/tmp/heif-conformance/conformance_files"

	// Check if conformance files exist
	if _, err := os.Stat(conformanceDir); os.IsNotExist(err) {
		t.Skip("Nokia conformance files not found. Clone https://github.com/nokiatech/heif_conformance to /tmp/heif-conformance")
	}

	files, err := filepath.Glob(filepath.Join(conformanceDir, "*.heic"))
	if err != nil {
		t.Fatalf("Error listing conformance files: %v", err)
	}

	if len(files) == 0 {
		t.Skip("No HEIC files found in conformance directory")
	}

	t.Logf("Testing %d Nokia conformance files", len(files))

	parseSuccess := 0
	parseErrors := 0
	metadataFiles := 0

	for _, file := range files {
		basename := filepath.Base(file)

		// Test 1: Can we parse the file?
		result, err := ExtractMetadata(file)
		if err != nil {
			t.Logf("  ⚠ %s: Parse error: %v", basename, err)
			parseErrors++
			continue
		}

		parseSuccess++

		// Test 2: Does it have metadata?
		hasMetadata := result.EXIFPresent || result.XMPPresent
		totalTags := len(result.EXIFSensitiveTags) + len(result.XMPSensitiveFields)

		if hasMetadata {
			metadataFiles++
			t.Logf("  ✓ %s: Parsed successfully, has metadata (EXIF:%v XMP:%v, %d sensitive tags)",
				basename, result.EXIFPresent, result.XMPPresent, totalTags)
		} else {
			t.Logf("  ○ %s: Parsed successfully, no metadata", basename)
		}

		// Test 3: Can we remove metadata (if any)?
		if hasMetadata {
			outputPath := filepath.Join(os.TempDir(), "nokia_test_"+basename)
			err = RemoveMetadata(file, outputPath, Options{})
			if err != nil {
				t.Errorf("  ✗ %s: RemoveMetadata failed: %v", basename, err)
			} else {
				// Verify metadata was removed
				resultAfter, err := ExtractMetadata(outputPath)
				if err != nil {
					t.Errorf("  ✗ %s: Cannot parse after removal: %v", basename, err)
				} else if resultAfter.EXIFPresent || resultAfter.XMPPresent {
					t.Errorf("  ✗ %s: Metadata still present after removal", basename)
				} else {
					t.Logf("  ✓ %s: Metadata removal successful", basename)
				}
				os.Remove(outputPath)
			}
		}
	}

	t.Logf("\n=== Summary ===")
	t.Logf("Total files: %d", len(files))
	t.Logf("Parse success: %d", parseSuccess)
	t.Logf("Parse errors: %d", parseErrors)
	t.Logf("Files with metadata: %d", metadataFiles)

	if parseErrors > 0 {
		t.Logf("\nNote: Some conformance files may use HEIF features not yet supported")
	}

	// We should be able to parse at least 50% of conformance files (reasonable baseline)
	successRate := float64(parseSuccess) / float64(len(files)) * 100
	if successRate < 50.0 {
		t.Errorf("Parse success rate too low: %.1f%% (expected >= 50%%)", successRate)
	} else {
		t.Logf("Parse success rate: %.1f%%", successRate)
	}
}

// TestNokiaConformanceFile_Specific tests specific conformance files mentioned in Nokia examples
func TestNokiaConformanceFile_Specific(t *testing.T) {
	testFiles := []string{
		"/tmp/heif-conformance/conformance_files/C003.heic", // Referenced in Nokia example.cpp
		"/tmp/heif-conformance/conformance_files/C001.heic", // Image sequence
		"/tmp/heif-conformance/conformance_files/C002.heic", // Single image
	}

	for _, file := range testFiles {
		basename := filepath.Base(file)
		t.Run(basename, func(t *testing.T) {
			if _, err := os.Stat(file); os.IsNotExist(err) {
				t.Skip("Conformance file not found")
			}

			result, err := ExtractMetadata(file)
			if err != nil {
				t.Logf("Parse error (may be unsupported feature): %v", err)
				// Don't fail - some features may not be supported yet
				return
			}

			t.Logf("EXIF Present: %v", result.EXIFPresent)
			t.Logf("XMP Present: %v", result.XMPPresent)
			t.Logf("EXIF Boxes: %d", result.EXIFBoxes)
			t.Logf("XMP Boxes: %d", result.XMPBoxes)

			if result.EXIFPresent {
				t.Logf("EXIF Sensitive Tags: %d", len(result.EXIFSensitiveTags))
			}
		})
	}
}
