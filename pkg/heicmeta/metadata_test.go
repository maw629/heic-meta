package heicmeta

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractMetadataDetectsExifAndXMP(t *testing.T) {
	exifPayload := buildHEICExifPayloadForTest()
	exif := makeBox("Exif", exifPayload)

	xmpXML := `<?xpacket begin=""?>
<x:xmpmeta xmlns:x="adobe:ns:meta/" xmlns:exif="http://ns.adobe.com/exif/1.0/" xmlns:tiff="http://ns.adobe.com/tiff/1.0/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description exif:GPSLatitude="37,46.5N" exif:GPSLongitude="122,25.1W" tiff:Model="iPhone 15"/>
  </rdf:RDF>
</x:xmpmeta>`
	xmpPayload := append([]byte{0, 0, 0, 0, 'a', 'p', 'p', 'l', 'i', 'c', 'a', 't', 'i', 'o', 'n', '/', 'r', 'd', 'f', '+', 'x', 'm', 'l', 0, 0}, []byte(xmpXML)...)
	mime := makeBox("mime", xmpPayload)

	ipco := makeBox("ipco", append(exif, mime...))
	iprp := makeBox("iprp", ipco)
	meta := makeBox("meta", append([]byte{0, 0, 0, 0}, iprp...))
	ftyp := makeBox("ftyp", []byte("heic0000"))
	fileData := append(ftyp, meta...)

	path := filepath.Join(t.TempDir(), "sample.heic")
	if err := os.WriteFile(path, fileData, 0o600); err != nil {
		t.Fatalf("failed to write sample file: %v", err)
	}

	md, err := ExtractMetadata(path)
	if err != nil {
		t.Fatalf("ExtractMetadata returned error: %v", err)
	}

	if !md.EXIFPresent || md.EXIFBoxes != 1 {
		t.Fatalf("expected EXIF to be present in 1 box, got present=%v boxes=%d", md.EXIFPresent, md.EXIFBoxes)
	}
	if !md.XMPPresent || md.XMPBoxes != 1 {
		t.Fatalf("expected XMP to be present in 1 box, got present=%v boxes=%d", md.XMPPresent, md.XMPBoxes)
	}

	assertContains(t, md.EXIFSensitiveTags, "DateTimeOriginal")
	assertContains(t, md.EXIFSensitiveTags, "GPSLatitude")
	assertContains(t, md.XMPSensitiveFields, "xmp:GPSLatitude")
	assertContains(t, md.XMPSensitiveFields, "xmp:GPSLongitude")
	assertContains(t, md.SensitiveTags, "DateTimeOriginal")
	assertContains(t, md.SensitiveTags, "xmp:GPSLatitude")
}

func buildHEICExifPayloadForTest() []byte {
	// TIFF (little-endian):
	// - IFD0 includes ExifIFD pointer + GPSIFD pointer
	// - ExifIFD includes DateTimeOriginal
	// - GPSIFD includes GPSLatitude
	tiff := make([]byte, 74)
	copy(tiff[0:2], []byte{'I', 'I'})
	binary.LittleEndian.PutUint16(tiff[2:4], 42)
	binary.LittleEndian.PutUint32(tiff[4:8], 8) // first IFD offset

	ifd0 := 8
	binary.LittleEndian.PutUint16(tiff[ifd0:ifd0+2], 2)
	// Entry 1: ExifIFD pointer -> offset 38
	e0 := ifd0 + 2
	binary.LittleEndian.PutUint16(tiff[e0:e0+2], 0x8769)
	binary.LittleEndian.PutUint16(tiff[e0+2:e0+4], 4)
	binary.LittleEndian.PutUint32(tiff[e0+4:e0+8], 1)
	binary.LittleEndian.PutUint32(tiff[e0+8:e0+12], 38)
	// Entry 2: GPSIFD pointer -> offset 56
	e1 := e0 + 12
	binary.LittleEndian.PutUint16(tiff[e1:e1+2], 0x8825)
	binary.LittleEndian.PutUint16(tiff[e1+2:e1+4], 4)
	binary.LittleEndian.PutUint32(tiff[e1+4:e1+8], 1)
	binary.LittleEndian.PutUint32(tiff[e1+8:e1+12], 56)
	// next IFD = 0
	binary.LittleEndian.PutUint32(tiff[ifd0+26:ifd0+30], 0)

	// ExifIFD at offset 38, one tag: DateTimeOriginal
	exifIFD := 38
	binary.LittleEndian.PutUint16(tiff[exifIFD:exifIFD+2], 1)
	exifEntry := exifIFD + 2
	binary.LittleEndian.PutUint16(tiff[exifEntry:exifEntry+2], 0x9003)
	binary.LittleEndian.PutUint16(tiff[exifEntry+2:exifEntry+4], 2)
	binary.LittleEndian.PutUint32(tiff[exifEntry+4:exifEntry+8], 20)
	binary.LittleEndian.PutUint32(tiff[exifEntry+8:exifEntry+12], 0)
	binary.LittleEndian.PutUint32(tiff[exifIFD+14:exifIFD+18], 0)

	// GPSIFD at offset 56, one tag: GPSLatitude
	gpsIFD := 56
	binary.LittleEndian.PutUint16(tiff[gpsIFD:gpsIFD+2], 1)
	gpsEntry := gpsIFD + 2
	binary.LittleEndian.PutUint16(tiff[gpsEntry:gpsEntry+2], 0x0002)
	binary.LittleEndian.PutUint16(tiff[gpsEntry+2:gpsEntry+4], 5)
	binary.LittleEndian.PutUint32(tiff[gpsEntry+4:gpsEntry+8], 3)
	binary.LittleEndian.PutUint32(tiff[gpsEntry+8:gpsEntry+12], 0)
	binary.LittleEndian.PutUint32(tiff[gpsIFD+14:gpsIFD+18], 0)

	// HEIC Exif payload starts with 4-byte prefix.
	payload := make([]byte, 4+len(tiff))
	copy(payload[4:], tiff)
	return payload
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, v := range values {
		if v == want {
			return
		}
	}
	t.Fatalf("expected %q in %v", want, values)
}
