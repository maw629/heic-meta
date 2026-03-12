package heicmeta

import (
"os"
"testing"
)

func TestExtractItemBasedMetadataWithRealSample(t *testing.T) {
// This test uses the integration sample if available
testFile := "/tmp/heic-integration/image4.heic"
if _, err := os.Stat(testFile); os.IsNotExist(err) {
t.Skip("Skipping test - sample file not available")
}

tree, err := ParseFile(testFile)
if err != nil {
t.Fatal(err)
}

exifItems, xmpItems, err := ExtractItemBasedMetadata(testFile, tree)
if err != nil {
t.Fatal(err)
}

if len(exifItems) != 1 {
t.Errorf("Expected 1 exif item, got %d", len(exifItems))
}
if len(xmpItems) != 0 {
t.Errorf("Expected 0 xmp items, got %d", len(xmpItems))
}

if len(exifItems) > 0 {
// Verify we can detect tags from the extracted item
tags, err := DetectSensitiveEXIFTagsFromHEICPayload(exifItems[0])
if err != nil {
t.Fatalf("Failed to detect tags: %v", err)
}
// image4.heic has UserComment
found := false
for _, tag := range tags {
if tag == "UserComment" {
found = true
break
}
}
if !found {
t.Error("Expected to find UserComment tag in image4.heic")
}
}
}
