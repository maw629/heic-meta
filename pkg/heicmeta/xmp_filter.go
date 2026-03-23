package heicmeta

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// FilterSensitiveXMP removes sensitive fields from XMP XML data.
// Returns filtered XMP data or nil if all data was sensitive.
func FilterSensitiveXMP(payload []byte) ([]byte, error) {
	xmlData, isXMP := extractXMPXML(payload)
	if !isXMP {
		return nil, fmt.Errorf("not XMP data")
	}

	// Parse XML
	decoder := xml.NewDecoder(bytes.NewReader(xmlData))
	var buf bytes.Buffer
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")

	// Track whether we're inside a sensitive element
	var elemStack []xml.StartElement
	skipDepth := 0

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			// Check if this element or its attributes contain sensitive data
			isSensitive := isXMPElementSensitive(t)

			if isSensitive {
				// Start skipping
				if skipDepth == 0 {
					skipDepth = 1
				} else {
					skipDepth++
				}
			} else if skipDepth == 0 {
				// Not sensitive and not skipping, keep it
				if err := encoder.EncodeToken(t); err != nil {
					return nil, err
				}
				elemStack = append(elemStack, t)
			} else {
				// Inside skip region, increase depth
				skipDepth++
			}

		case xml.EndElement:
			if skipDepth > 0 {
				skipDepth--
			} else {
				if err := encoder.EncodeToken(t); err != nil {
					return nil, err
				}
				if len(elemStack) > 0 {
					elemStack = elemStack[:len(elemStack)-1]
				}
			}

		case xml.CharData:
			if skipDepth == 0 {
				if err := encoder.EncodeToken(t); err != nil {
					return nil, err
				}
			}

		case xml.Comment:
			if skipDepth == 0 {
				if err := encoder.EncodeToken(t); err != nil {
					return nil, err
				}
			}

		case xml.ProcInst:
			if skipDepth == 0 {
				if err := encoder.EncodeToken(t); err != nil {
					return nil, err
				}
			}

		case xml.Directive:
			if skipDepth == 0 {
				if err := encoder.EncodeToken(t); err != nil {
					return nil, err
				}
			}
		}
	}

	if err := encoder.Flush(); err != nil {
		return nil, err
	}

	filteredXML := buf.Bytes()

	// Check if anything meaningful remains (just the XML envelope is ~50-600 bytes depending on namespaces)
	// Count actual data elements (not just structure)
	// If we have very few elements, it's likely just the structure
	tagCount := bytes.Count(filteredXML, []byte("<"))
	closingTagCount := bytes.Count(filteredXML, []byte("</"))

	// If we have <= 7 tags total and few data elements, return nil
	// (7 = proc instr + 3 structure elements * 2 for open/close + a bit extra)
	if tagCount <= 7 || (closingTagCount >= 3 && tagCount-closingTagCount <= 4) {
		return nil, nil
	}

	// Rebuild with mime prefix if needed
	// For simplicity, return just the XML for now
	return filteredXML, nil
}

func isXMPElementSensitive(elem xml.StartElement) bool {
	// Check element's namespace (exif namespace contains camera metadata)
	namespace := strings.ToLower(elem.Name.Space)
	if strings.Contains(namespace, "exif") {
		// This is EXIF namespace, check if the element itself is sensitive
		localName := strings.ToLower(elem.Name.Local)
		for _, keyword := range xmpSensitiveKeywords {
			if strings.Contains(localName, strings.ToLower(keyword)) {
				return true
			}
		}
	}

	// Check element name (e.g., GPS fields)
	localName := strings.ToLower(elem.Name.Local)
	for _, keyword := range xmpSensitiveKeywords {
		if strings.Contains(localName, strings.ToLower(keyword)) {
			return true
		}
	}

	// Check attributes (but skip xmlns declarations)
	for _, attr := range elem.Attr {
		// Skip xmlns attributes - they're just namespace declarations
		if attr.Name.Space == "xmlns" || attr.Name.Local == "xmlns" || strings.HasPrefix(attr.Name.Local, "xmlns:") {
			continue
		}

		attrName := strings.ToLower(attr.Name.Local)
		attrValue := strings.ToLower(attr.Value)

		for _, keyword := range xmpSensitiveKeywords {
			kw := strings.ToLower(keyword)
			if strings.Contains(attrName, kw) || strings.Contains(attrValue, kw) {
				return true
			}
		}
	}

	return false
}
