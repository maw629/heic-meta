package heicmeta

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestParseBoxTreeFindsMdat(t *testing.T) {
	exif := makeBox("Exif", []byte{0, 0, 0, 0, 'I', 'I', '*', 0})
	ipco := makeBox("ipco", exif)
	iprp := makeBox("iprp", ipco)
	metaPayload := append([]byte{0, 0, 0, 0}, iprp...)
	meta := makeBox("meta", metaPayload)
	ftyp := makeBox("ftyp", []byte("heic0000"))
	mdatPayload := []byte{0xde, 0xad, 0xbe, 0xef, 1, 2, 3, 4}
	mdat := makeBox("mdat", mdatPayload)
	file := append(append(ftyp, meta...), mdat...)

	tree, err := ParseBoxTree(bytes.NewReader(file))
	if err != nil {
		t.Fatalf("ParseBoxTree returned error: %v", err)
	}

	if len(tree.Root) != 3 {
		t.Fatalf("expected 3 root boxes, got %d", len(tree.Root))
	}
	if tree.Root[0].TypeString() != "ftyp" {
		t.Fatalf("expected first root box ftyp, got %s", tree.Root[0].TypeString())
	}
	if tree.Root[1].TypeString() != "meta" {
		t.Fatalf("expected second root box meta, got %s", tree.Root[1].TypeString())
	}
	if tree.Root[2].TypeString() != "mdat" {
		t.Fatalf("expected third root box mdat, got %s", tree.Root[2].TypeString())
	}

	if !tree.HasMdat() {
		t.Fatalf("expected tree.HasMdat() to be true")
	}
	if len(tree.Mdats) != 1 {
		t.Fatalf("expected exactly one mdat, got %d", len(tree.Mdats))
	}
}

func TestReadBoxBytesPreservesMdatBytes(t *testing.T) {
	ftyp := makeBox("ftyp", []byte("heic0000"))
	mdatPayload := []byte{9, 8, 7, 6, 5, 4, 3, 2}
	mdat := makeBox("mdat", mdatPayload)
	file := append(ftyp, mdat...)

	r := bytes.NewReader(file)
	tree, err := ParseBoxTree(r)
	if err != nil {
		t.Fatalf("ParseBoxTree returned error: %v", err)
	}
	mdatNode := tree.FirstMdat()
	if mdatNode == nil {
		t.Fatalf("expected mdat node")
	}

	wholeBox, err := ReadBoxBytes(bytes.NewReader(file), mdatNode)
	if err != nil {
		t.Fatalf("ReadBoxBytes returned error: %v", err)
	}
	wantWhole := file[mdatNode.Offset : mdatNode.Offset+mdatNode.Size]
	if !bytes.Equal(wholeBox, wantWhole) {
		t.Fatalf("mdat box bytes mismatch")
	}

	payload, err := ReadBoxPayloadBytes(bytes.NewReader(file), mdatNode)
	if err != nil {
		t.Fatalf("ReadBoxPayloadBytes returned error: %v", err)
	}
	if !bytes.Equal(payload, mdatPayload) {
		t.Fatalf("mdat payload mismatch")
	}
}

func makeBox(boxType string, payload []byte) []byte {
	b := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(b[:4], uint32(len(b)))
	copy(b[4:8], []byte(boxType))
	copy(b[8:], payload)
	return b
}
