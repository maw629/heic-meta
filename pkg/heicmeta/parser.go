package heicmeta

import (
	"fmt"
	"io"
	"os"

	mp4 "github.com/abema/go-mp4"
)

var containerTypes = map[mp4.BoxType]struct{}{
	mp4.BoxTypeMeta():        {},
	mp4.BoxTypeMoov():        {},
	mp4.BoxTypeTrak():        {},
	mp4.BoxTypeMdia():        {},
	mp4.BoxTypeMinf():        {},
	mp4.BoxTypeStbl():        {},
	mp4.BoxTypeEdts():        {},
	mp4.BoxTypeDinf():        {},
	mp4.BoxTypeUdta():        {},
	mp4.StrToBoxType("iprp"): {},
	mp4.StrToBoxType("ipco"): {},
	mp4.StrToBoxType("iref"): {},
}

var fullBoxContainerTypes = map[mp4.BoxType]struct{}{
	mp4.BoxTypeMeta():        {},
	mp4.StrToBoxType("iref"): {},
}

// BoxNode represents one ISO BMFF box in a parsed tree.
type BoxNode struct {
	Type        mp4.BoxType
	Offset      uint64
	Size        uint64
	HeaderSize  uint64
	ExtendToEOF bool
	Children    []*BoxNode
}

func (n *BoxNode) TypeString() string {
	return n.Type.String()
}

type BoxTree struct {
	Root  []*BoxNode
	Mdats []*BoxNode
}

func (t *BoxTree) HasMdat() bool {
	return len(t.Mdats) > 0
}

func (t *BoxTree) FirstMdat() *BoxNode {
	if len(t.Mdats) == 0 {
		return nil
	}
	return t.Mdats[0]
}

func ParseFile(path string) (*BoxTree, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ParseBoxTree(f)
}

func ParseBoxTree(r io.ReadSeeker) (*BoxTree, error) {
	end, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	if end < 0 {
		return nil, fmt.Errorf("invalid stream size: %d", end)
	}

	tree := &BoxTree{}
	root, err := parseRange(r, 0, uint64(end), tree)
	if err != nil {
		return nil, err
	}
	tree.Root = root
	return tree, nil
}

func parseRange(r io.ReadSeeker, start, end uint64, tree *BoxTree) ([]*BoxNode, error) {
	if _, err := r.Seek(int64(start), io.SeekStart); err != nil {
		return nil, err
	}

	nodes := make([]*BoxNode, 0, 8)
	for {
		pos, err := r.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}
		if uint64(pos)+mp4.SmallHeaderSize > end {
			break
		}

		bi, err := mp4.ReadBoxInfo(r)
		if err != nil {
			return nil, err
		}
		if bi.Size < bi.HeaderSize {
			return nil, fmt.Errorf("invalid box size at offset %d", bi.Offset)
		}
		boxEnd := bi.Offset + bi.Size
		if boxEnd > end {
			return nil, fmt.Errorf("box %s at offset %d exceeds parent bounds", bi.Type.String(), bi.Offset)
		}

		node := &BoxNode{
			Type:        bi.Type,
			Offset:      bi.Offset,
			Size:        bi.Size,
			HeaderSize:  bi.HeaderSize,
			ExtendToEOF: bi.ExtendToEOF,
		}
		nodes = append(nodes, node)

		if bi.Type == mp4.BoxTypeMdat() {
			tree.Mdats = append(tree.Mdats, node)
		}

		if isContainerType(bi.Type) {
			childStart := bi.Offset + bi.HeaderSize
			if isFullBoxContainerType(bi.Type) {
				if bi.Size < bi.HeaderSize+4 {
					return nil, fmt.Errorf("invalid fullbox container size at offset %d", bi.Offset)
				}
				childStart += 4
			}
			children, err := parseRange(r, childStart, boxEnd, tree)
			if err != nil {
				return nil, err
			}
			node.Children = children
		}

		if _, err := r.Seek(int64(boxEnd), io.SeekStart); err != nil {
			return nil, err
		}
	}

	if _, err := r.Seek(int64(end), io.SeekStart); err != nil {
		return nil, err
	}
	return nodes, nil
}

func isContainerType(boxType mp4.BoxType) bool {
	_, ok := containerTypes[boxType]
	return ok
}

func isFullBoxContainerType(boxType mp4.BoxType) bool {
	_, ok := fullBoxContainerTypes[boxType]
	return ok
}

func ReadBoxBytes(r io.ReadSeeker, node *BoxNode) ([]byte, error) {
	if node == nil {
		return nil, fmt.Errorf("box node is nil")
	}
	if _, err := r.Seek(int64(node.Offset), io.SeekStart); err != nil {
		return nil, err
	}
	b := make([]byte, node.Size)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}
	return b, nil
}

func ReadBoxPayloadBytes(r io.ReadSeeker, node *BoxNode) ([]byte, error) {
	if node == nil {
		return nil, fmt.Errorf("box node is nil")
	}
	if node.Size < node.HeaderSize {
		return nil, fmt.Errorf("invalid node size")
	}
	if _, err := r.Seek(int64(node.Offset+node.HeaderSize), io.SeekStart); err != nil {
		return nil, err
	}
	payloadSize := node.Size - node.HeaderSize
	b := make([]byte, payloadSize)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}
	return b, nil
}
