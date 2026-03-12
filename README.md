# heic-meta

Pure Go library for HEIC/HEIF metadata operations.

## Current status

Step 1 (Foundation & Box Parser) is implemented:
- Go module initialized
- `github.com/abema/go-mp4` added as parser foundation
- ISO BMFF box tree parser added under `pkg/heicmeta`
- `HEICHandler` skeleton added for upcoming metadata APIs

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
