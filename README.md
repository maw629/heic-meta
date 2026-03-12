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
