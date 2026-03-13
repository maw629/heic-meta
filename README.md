# heic-meta

Pure Go library for HEIC/HEIF metadata operations.

## Current status

Step 1 (Foundation & Box Parser) is implemented:
- Go module initialized
- `github.com/abema/go-mp4` added as parser foundation
- ISO BMFF box tree parser added under `pkg/heicmeta`
- `HEICHandler` skeleton added for upcoming metadata APIs

Step 2 (Metadata Detection) is implemented:
- `ExtractMetadata` / `PreviewMetadata` now detect EXIF and XMP metadata
- HEIC EXIF 4-byte prefix handling with TIFF IFD traversal
- Sensitive EXIF/XMP field detection summary
- Item-based metadata extraction (iinf/iloc/idat) for real iPhone photos
- Tested with actual public HEIC samples from GitHub

## Quick example

```go
tree, err := heicmeta.ParseFile("photo.heic")
if err != nil {
    return err
}
if tree.HasMdat() {
    // mdat exists and can be copied byte-for-byte later.
}
```

```go
md, err := heicmeta.PreviewMetadata("photo.heic")
if err != nil {
    return err
}
fmt.Println(md.EXIFPresent, md.XMPPresent, md.SensitiveTags)
```

## Testing with Nokia HEIF Conformance Files

The library has been validated against Nokia's official HEIF conformance test suite:

```bash
# Clone Nokia conformance files
git clone https://github.com/nokiatech/heif_conformance.git /tmp/heif-conformance

# Run validation tests
cd pkg/heicmeta
go test -v -run TestNokiaConformance
```

**Results**: 100% parse success rate on all 63 conformance files. See [NOKIA_VALIDATION_RESULTS.md](NOKIA_VALIDATION_RESULTS.md) for details.
