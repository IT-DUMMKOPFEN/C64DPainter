package c64

import "testing"

func TestDynamicAndFreeLayers(t *testing.T) {
	pal := &Palettes[0]
	p := NewPicture(0, 0)
	p.ToLayers()
	if p.LayerCount() != DefaultLayers {
		t.Fatal("default layers")
	}
	// three colors in cell 0 on layer 0
	for i, c := range []uint8{2, 5, 7} {
		p.Plot(i, 0, c, ConflictBlock, pal)
	}
	// a free layer may use a fourth color, the rules of the others stay
	p.InsertLayer(1)
	p.Free[1] = true
	if r := p.Plot(3, 1, 1, ConflictBlock, pal); r != PlotOK || p.ColorAt(3, 1) != 1 {
		t.Fatal("free layer pixel")
	}
	if r := func() PlotResult { p.Active = 0; return p.Plot(3, 2, 14, ConflictBlock, pal) }(); r != PlotBlocked {
		t.Fatal("the free pixel must not use a slot of the normal layers")
	}
	if !p.HasFreeLayers() {
		t.Fatal("HasFreeLayers")
	}
	// clone is deep
	q := p.Clone()
	q.L[1][0] = 9
	if p.L[1][0] == 9 {
		t.Fatal("clone shares layers")
	}
	// move, duplicate, delete, merge
	p.Active = 1
	p.MoveLayer(1, 1)
	if p.Active != 2 || !p.Free[2] || p.ColorAt(3, 1) != 1 {
		t.Fatal("move")
	}
	p.DuplicateLayer(2)
	if p.LayerCount() != 5 || p.L[3] != p.L[2] {
		t.Fatal("duplicate")
	}
	p.DeleteLayer(3)
	for p.LayerCount() < MaxLayers {
		if !p.InsertLayer(p.LayerCount()) {
			t.Fatal("insert")
		}
	}
	if p.InsertLayer(0) {
		t.Fatal("more than MaxLayers")
	}
	for p.LayerCount() > 4 {
		p.DeleteLayer(p.LayerCount() - 1)
	}
	// the free layer becomes normal: the fourth color takes the nearest cell color
	p.SetFree(2, false, pal)
	if p.Free[2] || p.HasFreeLayers() {
		t.Fatal("not free any more")
	}
	cnt, n := p.cellUnion(0, -1, -1)
	if n > 3 || cnt[1] != 0 {
		t.Fatalf("rules not applied: %d colors", n)
	}
	// merge down keeps the rules
	p.Active = 2
	if !p.MergeDown(2, pal) || p.LayerCount() != 3 {
		t.Fatal("merge")
	}
	if _, n := p.cellUnion(0, -1, -1); n > 3 {
		t.Fatal("merge broke the rules")
	}
}
