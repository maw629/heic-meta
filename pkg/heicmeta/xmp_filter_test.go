package heicmeta

import (
	"strings"
	"testing"
)

func TestFilterSensitiveXMP(t *testing.T) {
	// XMP with sensitive and non-sensitive fields
	xmpXML := `<?xml version="1.0"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
<rdf:Description rdf:about=""
  xmlns:exif="http://ns.adobe.com/exif/1.0/"
  xmlns:xap="http://ns.adobe.com/xap/1.0/">
  <exif:Make>Canon</exif:Make>
  <exif:GPSLatitude>37.7749</exif:GPSLatitude>
  <xap:CreateDate>2023-01-01</xap:CreateDate>
</rdf:Description>
</rdf:RDF>
</x:xmpmeta>`
	
	// Filter
	filtered, err := FilterSensitiveXMP([]byte(xmpXML))
	if err != nil {
		t.Fatalf("FilterSensitiveXMP failed: %v", err)
	}
	
	// Should not be nil (CreateDate should remain)
	if filtered == nil {
		t.Error("Expected some data to remain (CreateDate)")
	}
	
	// Check that sensitive fields are gone
	filteredStr := string(filtered)
	if strings.Contains(filteredStr, "Make") || strings.Contains(filteredStr, "Canon") {
		t.Error("Sensitive field 'Make' should be removed")
	}
	if strings.Contains(filteredStr, "GPS") {
		t.Error("GPS field should be removed")
	}
	
	// Check that non-sensitive field remains
	if !strings.Contains(filteredStr, "CreateDate") {
		t.Error("Non-sensitive field 'CreateDate' should remain")
	}
}

func TestFilterSensitiveXMPAllSensitive(t *testing.T) {
	// XMP with only sensitive fields
	xmpXML := `<?xml version="1.0"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
<rdf:Description rdf:about=""
  xmlns:exif="http://ns.adobe.com/exif/1.0/">
  <exif:Make>Canon</exif:Make>
  <exif:Model>EOS</exif:Model>
</rdf:Description>
</rdf:RDF>
</x:xmpmeta>`
	
	// Filter
	filtered, err := FilterSensitiveXMP([]byte(xmpXML))
	if err != nil {
		t.Fatalf("FilterSensitiveXMP failed: %v", err)
	}
	
	// Should be nil (all fields were sensitive, only structure remains)
	// The check in the filter looks for < 5 tags or < 150 bytes
	if filtered != nil {
		t.Errorf("Expected nil when all fields are sensitive, got %d bytes", len(filtered))
	}
}

func TestFilterSensitiveXMPInvalid(t *testing.T) {
	// Invalid XML
	xmpXML := []byte("not xml")
	
	// Filter
	filtered, err := FilterSensitiveXMP(xmpXML)
	
	// Should return error for invalid data
	if err == nil {
		t.Error("Expected error for invalid XML")
	}
	
	// Should return nil
	if filtered != nil {
		t.Error("Expected nil for invalid XML")
	}
}
