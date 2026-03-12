package heicmeta

import (
	"testing"
)

func TestBoxSizeCalculator_SimpleTree(t *testing.T) {
	// Create a simple tree: root with two children
	// root (meta)
	//   ├── child1 (iinf) - 100 bytes
	//   └── child2 (iloc) - 200 bytes
	
	child1 := &BoxNode{
		Type:       strToBoxType("iinf"),
		Size:       100,
		HeaderSize: 8,
	}
	
	child2 := &BoxNode{
		Type:       strToBoxType("iloc"),
		Size:       200,
		HeaderSize: 8,
	}
	
	root := &BoxNode{
		Type:       strToBoxType("meta"),
		HeaderSize: 8,
		Children:   []*BoxNode{child1, child2},
	}
	
	calc := NewBoxSizeCalculator()
	newSizes := calc.RecalculateAllSizes(root)
	
	// Check child sizes (unchanged)
	if size := newSizes[child1]; size != 100 {
		t.Errorf("child1 size: expected 100, got %d", size)
	}
	if size := newSizes[child2]; size != 200 {
		t.Errorf("child2 size: expected 200, got %d", size)
	}
	
	// Check root size: header(8) + child1(100) + child2(200) = 308
	expectedRootSize := uint64(8 + 100 + 200)
	if size := newSizes[root]; size != expectedRootSize {
		t.Errorf("root size: expected %d, got %d", expectedRootSize, size)
	}
}

func TestBoxSizeCalculator_WithOverride(t *testing.T) {
	// Tree with override on one child
	child1 := &BoxNode{
		Type:       strToBoxType("iinf"),
		Size:       100,
		HeaderSize: 8,
	}
	
	child2 := &BoxNode{
		Type:       strToBoxType("iloc"),
		Size:       200,
		HeaderSize: 8,
	}
	
	root := &BoxNode{
		Type:       strToBoxType("meta"),
		HeaderSize: 8,
		Children:   []*BoxNode{child1, child2},
	}
	
	calc := NewBoxSizeCalculator()
	
	// Override child1 to be 50 bytes instead of 100
	calc.SetSize(child1, 50)
	
	newSizes := calc.RecalculateAllSizes(root)
	
	// child1 should be 50 (overridden)
	if size := newSizes[child1]; size != 50 {
		t.Errorf("child1 size: expected 50, got %d", size)
	}
	
	// child2 should be 200 (unchanged)
	if size := newSizes[child2]; size != 200 {
		t.Errorf("child2 size: expected 200, got %d", size)
	}
	
	// root should be: header(8) + child1(50) + child2(200) = 258
	expectedRootSize := uint64(8 + 50 + 200)
	if size := newSizes[root]; size != expectedRootSize {
		t.Errorf("root size: expected %d, got %d", expectedRootSize, size)
	}
}

func TestBoxSizeCalculator_NestedTree(t *testing.T) {
	// Nested tree:
	// root (moov)
	//   └── trak
	//       └── mdia (100 bytes)
	
	mdia := &BoxNode{
		Type:       strToBoxType("mdia"),
		Size:       100,
		HeaderSize: 8,
	}
	
	trak := &BoxNode{
		Type:       strToBoxType("trak"),
		HeaderSize: 8,
		Children:   []*BoxNode{mdia},
	}
	
	root := &BoxNode{
		Type:       strToBoxType("moov"),
		HeaderSize: 8,
		Children:   []*BoxNode{trak},
	}
	
	calc := NewBoxSizeCalculator()
	newSizes := calc.RecalculateAllSizes(root)
	
	// mdia: 100 (leaf)
	if size := newSizes[mdia]; size != 100 {
		t.Errorf("mdia size: expected 100, got %d", size)
	}
	
	// trak: header(8) + mdia(100) = 108
	if size := newSizes[trak]; size != 108 {
		t.Errorf("trak size: expected 108, got %d", size)
	}
	
	// root: header(8) + trak(108) = 116
	if size := newSizes[root]; size != 116 {
		t.Errorf("root size: expected 116, got %d", size)
	}
}

func TestBoxTreeModifier_ReplaceBox(t *testing.T) {
	child := &BoxNode{
		Type:       strToBoxType("iinf"),
		Size:       100,
		HeaderSize: 8,
	}
	
	root := &BoxNode{
		Type:       strToBoxType("meta"),
		HeaderSize: 8,
		Children:   []*BoxNode{child},
	}
	
	modifier := NewBoxTreeModifier()
	
	// Replace child with new payload (50 bytes)
	newPayload := make([]byte, 50)
	modifier.ReplaceBox(child, newPayload)
	
	// Check replacement is stored
	if payload, exists := modifier.GetReplacement(child); !exists {
		t.Error("Replacement not found")
	} else if len(payload) != 50 {
		t.Errorf("Expected payload size 50, got %d", len(payload))
	}
	
	// Recalculate sizes
	newSizes := modifier.RecalculateSizes(root)
	
	// child should be: header(8) + payload(50) = 58
	if size := newSizes[child]; size != 58 {
		t.Errorf("child size: expected 58, got %d", size)
	}
	
	// root should be: header(8) + child(58) = 66
	if size := newSizes[root]; size != 66 {
		t.Errorf("root size: expected 66, got %d", size)
	}
}

func TestValidateBoxSize(t *testing.T) {
	tests := []struct {
		name    string
		boxType string
		size    uint64
		wantErr bool
	}{
		{"normal size", "test", 100, false},
		{"too small", "test", 4, true},
		{"minimum size", "test", 8, false},
		{"ftyp too small", "ftyp", 12, true},
		{"ftyp valid", "ftyp", 20, false},
		{"meta too small", "meta", 10, true},
		{"meta valid", "meta", 100, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBoxSize(tt.boxType, tt.size)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBoxSize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEstimateBoxHeaderSize(t *testing.T) {
	tests := []struct {
		name        string
		payloadSize uint64
		isFullBox   bool
		wantHeader  uint64
	}{
		{"small payload", 100, false, 8},
		{"large payload", 1000000, false, 8},
		{"huge payload", 5000000000, false, 16}, // > 4GB, needs extended
		{"fullbox small", 100, true, 8},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateBoxHeaderSize(tt.payloadSize, tt.isFullBox)
			if got != tt.wantHeader {
				t.Errorf("EstimateBoxHeaderSize() = %d, want %d", got, tt.wantHeader)
			}
		})
	}
}

func TestCalculateContainerSize(t *testing.T) {
	child1 := &BoxNode{Type: strToBoxType("iinf"), Size: 100}
	child2 := &BoxNode{Type: strToBoxType("iloc"), Size: 200}
	
	children := []*BoxNode{child1, child2}
	childSizes := make(map[*BoxNode]uint64)
	
	// Test without overrides (use original sizes)
	size := CalculateContainerSize(children, childSizes, false)
	// header(8) + child1(100) + child2(200) = 308
	if size != 308 {
		t.Errorf("Expected 308, got %d", size)
	}
	
	// Test with overrides
	childSizes[child1] = 50 // Override child1 to 50
	size = CalculateContainerSize(children, childSizes, false)
	// header(8) + child1(50) + child2(200) = 258
	if size != 258 {
		t.Errorf("Expected 258, got %d", size)
	}
	
	// Test with FullBox
	size = CalculateContainerSize(children, childSizes, true)
	// header(8) + version+flags(4) + child1(50) + child2(200) = 262
	if size != 262 {
		t.Errorf("Expected 262, got %d", size)
	}
}

func TestUpdateBoxTreeSizes(t *testing.T) {
	child1 := &BoxNode{Type: strToBoxType("iinf"), Size: 100}
	child2 := &BoxNode{Type: strToBoxType("iloc"), Size: 200}
	root := &BoxNode{
		Type:     strToBoxType("meta"),
		Size:     308,
		Children: []*BoxNode{child1, child2},
	}
	
	newSizes := map[*BoxNode]uint64{
		child1: 50,
		child2: 180,
		root:   238,
	}
	
	UpdateBoxTreeSizes(root, newSizes)
	
	if root.Size != 238 {
		t.Errorf("root size: expected 238, got %d", root.Size)
	}
	if child1.Size != 50 {
		t.Errorf("child1 size: expected 50, got %d", child1.Size)
	}
	if child2.Size != 180 {
		t.Errorf("child2 size: expected 180, got %d", child2.Size)
	}
}

func TestBoxSizeCalculator_RealScenario(t *testing.T) {
	// Simulate a real HEIC structure with metadata removal
	// meta
	//   ├── iinf (was 200, now 100 after removal)
	//   ├── iloc (was 300, now 150 after updates)
	//   └── idat (was 5000, now 4500 after filtering)
	
	iinf := &BoxNode{Type: strToBoxType("iinf"), Size: 200, HeaderSize: 8}
	iloc := &BoxNode{Type: strToBoxType("iloc"), Size: 300, HeaderSize: 8}
	idat := &BoxNode{Type: strToBoxType("idat"), Size: 5000, HeaderSize: 8}
	
	meta := &BoxNode{
		Type:       strToBoxType("meta"),
		HeaderSize: 8,
		Children:   []*BoxNode{iinf, iloc, idat},
	}
	
	moov := &BoxNode{
		Type:       strToBoxType("moov"),
		HeaderSize: 8,
		Children:   []*BoxNode{meta},
	}
	
	// Apply modifications
	calc := NewBoxSizeCalculator()
	calc.SetSize(iinf, 100) // Removed some items
	calc.SetSize(iloc, 150) // Updated offsets
	calc.SetSize(idat, 4500) // Filtered data
	
	newSizes := calc.RecalculateAllSizes(moov)
	
	// Check leaf boxes
	if newSizes[iinf] != 100 {
		t.Errorf("iinf: expected 100, got %d", newSizes[iinf])
	}
	if newSizes[iloc] != 150 {
		t.Errorf("iloc: expected 150, got %d", newSizes[iloc])
	}
	if newSizes[idat] != 4500 {
		t.Errorf("idat: expected 4500, got %d", newSizes[idat])
	}
	
	// Check meta: header(8) + iinf(100) + iloc(150) + idat(4500) = 4758
	expectedMeta := uint64(8 + 100 + 150 + 4500)
	if newSizes[meta] != expectedMeta {
		t.Errorf("meta: expected %d, got %d", expectedMeta, newSizes[meta])
	}
	
	// Check moov: header(8) + meta(4758) = 4766
	expectedMoov := uint64(8 + 4758)
	if newSizes[moov] != expectedMoov {
		t.Errorf("moov: expected %d, got %d", expectedMoov, newSizes[moov])
	}
}
