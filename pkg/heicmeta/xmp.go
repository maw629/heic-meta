package heicmeta

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
)

var xmpSensitiveKeywords = []string{
	"gps",
	"latitude",
	"longitude",
	"location",
	"datetime",
	"date",
	"time",
	"creator",
	"author",
	"artist",
	"owner",
	"serial",
	"lens",
	"camera",
	"make",
	"model",
	"city",
	"state",
	"country",
	"keywords",
}

// DetectSensitiveXMPFields detects sensitive XMP fields from mime box payload.
func DetectSensitiveXMPFields(payload []byte) (fields []string, isXMP bool, err error) {
	xmlData, ok := extractXMPXML(payload)
	if !ok {
		return nil, false, nil
	}

	decoder := xml.NewDecoder(bytes.NewReader(xmlData))
	sensitive := make(map[string]struct{})

	for {
		tok, decodeErr := decoder.Token()
		if decodeErr == io.EOF {
			break
		}
		if decodeErr != nil {
			return nil, true, decodeErr
		}

		switch t := tok.(type) {
		case xml.StartElement:
			recordSensitiveXMPName(sensitive, t.Name.Local)
			for _, attr := range t.Attr {
				recordSensitiveXMPName(sensitive, attr.Name.Local)
			}
		}
	}

	return sortedKeys(sensitive), true, nil
}

func extractXMPXML(payload []byte) ([]byte, bool) {
	if len(payload) == 0 {
		return nil, false
	}

	data := payload
	if len(data) >= 4 && data[0] == 0 && data[1] == 0 && data[2] == 0 {
		data = data[4:]
	}

	start := bytes.IndexByte(data, '<')
	if start < 0 {
		return nil, false
	}

	xmlData := bytes.TrimSpace(data[start:])
	if len(xmlData) == 0 {
		return nil, false
	}

	lower := strings.ToLower(string(xmlData))
	if !strings.Contains(lower, "xmp") && !strings.Contains(lower, "rdf:rdf") {
		return nil, false
	}

	return xmlData, true
}

func recordSensitiveXMPName(out map[string]struct{}, name string) {
	if name == "" {
		return
	}
	lower := strings.ToLower(name)
	for _, keyword := range xmpSensitiveKeywords {
		if strings.Contains(lower, keyword) {
			out["xmp:"+name] = struct{}{}
			return
		}
	}
}
