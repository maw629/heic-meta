# Using heic-meta as a Go Module

## Quick Start

### 1. Add the dependency to your project

In your Go project directory:

```bash
go get github.com/maw629/heic-meta/pkg/heicmeta
```

Or simply import it in your code and run `go mod tidy`:

```go
import "github.com/maw629/heic-meta/pkg/heicmeta"
```

Then:
```bash
go mod tidy
```

### 2. Basic Usage

#### Extract Metadata
```go
package main

import (
    "fmt"
    "log"
    "github.com/maw629/heic-meta/pkg/heicmeta"
)

func main() {
    // Extract metadata from a HEIC file
    result, err := heicmeta.ExtractMetadata("photo.heic")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("EXIF Present: %v\n", result.EXIFPresent)
    fmt.Printf("XMP Present: %v\n", result.XMPPresent)
    fmt.Printf("Sensitive Tags: %v\n", result.SensitiveTags)
}
```

#### Remove Metadata
```go
package main

import (
    "log"
    "github.com/maw629/heic-meta/pkg/heicmeta"
)

func main() {
    // Remove metadata from a HEIC file
    err := heicmeta.RemoveMetadata(
        "input.heic",
        "output.heic",
        heicmeta.Options{},
    )
    if err != nil {
        log.Fatal(err)
    }
}
```

## Installation Methods

### Method 1: Direct Import (Recommended)
Simplest approach - just import and let Go handle it:

```go
import "github.com/maw629/heic-meta/pkg/heicmeta"
```

Then run:
```bash
go mod tidy
```

### Method 2: Explicit go get
Add the dependency explicitly:

```bash
go get github.com/maw629/heic-meta/pkg/heicmeta@latest
```

### Method 3: Specific Version
Pin to a specific version or commit:

```bash
# Use a specific commit
go get github.com/maw629/heic-meta/pkg/heicmeta@8ef1dd2

# Or use a tag (when available)
go get github.com/maw629/heic-meta/pkg/heicmeta@v0.1.0
```

## Example Project

Create a new project:

```bash
# Create project directory
mkdir my-heic-tool
cd my-heic-tool

# Initialize Go module
go mod init github.com/yourusername/my-heic-tool

# Create main.go
cat > main.go << 'ENDOFFILE'
package main

import (
    "fmt"
    "log"
    "os"
    "github.com/maw629/heic-meta/pkg/heicmeta"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: my-heic-tool <input.heic>")
        os.Exit(1)
    }
    
    inputFile := os.Args[1]
    
    // Extract metadata
    result, err := heicmeta.ExtractMetadata(inputFile)
    if err != nil {
        log.Fatalf("Error: %v", err)
    }
    
    fmt.Printf("File: %s\n", inputFile)
    fmt.Printf("EXIF: %v\n", result.EXIFPresent)
    fmt.Printf("XMP: %v\n", result.XMPPresent)
    
    if len(result.SensitiveTags) > 0 {
        fmt.Printf("\nSensitive tags found:\n")
        for _, tag := range result.SensitiveTags {
            fmt.Printf("  - %s\n", tag)
        }
        
        // Remove metadata
        outputFile := "cleaned_" + inputFile
        err = heicmeta.RemoveMetadata(inputFile, outputFile, heicmeta.Options{})
        if err != nil {
            log.Fatalf("Error removing metadata: %v", err)
        }
        fmt.Printf("\nCleaned file saved to: %s\n", outputFile)
    }
}
ENDOFFILE

# Install dependencies
go mod tidy

# Build
go build

# Run
./my-heic-tool photo.heic
```

## API Reference

### ExtractMetadata
```go
func ExtractMetadata(path string) (Metadata, error)
```
Extracts metadata from a HEIC file.

**Returns**: `Metadata` struct with:
- `EXIFPresent` - Whether EXIF data exists
- `XMPPresent` - Whether XMP data exists
- `EXIFBoxes` - Number of EXIF boxes
- `XMPBoxes` - Number of XMP boxes
- `EXIFSensitiveTags` - List of sensitive EXIF tags found
- `XMPSensitiveFields` - List of sensitive XMP fields found
- `SensitiveTags` - Combined list of all sensitive metadata

### RemoveMetadata
```go
func RemoveMetadata(inputPath, outputPath string, options Options) error
```
Removes metadata from a HEIC file, preserving image data.

**Parameters**:
- `inputPath` - Path to input HEIC file
- `outputPath` - Path to save cleaned file
- `options` - Configuration options (currently unused, reserved for future)

**Features**:
- Removes EXIF metadata (GPS, camera info, timestamps)
- Removes XMP metadata
- Preserves image data byte-for-byte
- Maintains valid HEIC structure

## Requirements

- Go 1.22.2 or later
- No system dependencies (pure Go)

## Testing

The library includes comprehensive tests:

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific test
go test -run TestRemoveMetadata
```

## Production Ready

✅ **100% format compliance** - Validated against 63 Nokia HEIF conformance files  
✅ **Metadata removal verified** - Tested with real iPhone HEIC files  
✅ **Pixel-perfect preservation** - Image data unchanged after processing  
✅ **Pure Go** - No CGO or system dependencies  

## Support

- **Repository**: https://github.com/maw629/heic-meta
- **Issues**: https://github.com/maw629/heic-meta/issues
- **Documentation**: See README.md and inline code documentation

## License

See LICENSE file in the repository.
