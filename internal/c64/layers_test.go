package c64

import "testing"

func cellColorsOK(t *testing.T, p *Picture) {
	t.Helper()
	for cell := 0; cell < Cells; cell++ {
		cnt, n := p.cellUnion(cell, -1, -1)
		_ = cnt
		if n > 3 {
			t.Fatalf("cell %d: the layers use %d colors besides the background", cell, n)
		}
	}
}

func TestLayersFromFlat(t *testing.T) {
	p := NewPicture(6, 0)
	for x := 0; x < 40; x++ {
		p.Plot(x, 5, 2, ConflictBlock, nil)
		p.Plot(x, 6, 7, ConflictBlock, nil)
	}
	q := p.Clone()
	q.ToLayers()
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			if p.ColorAt(x, y) != q.ColorAt(x, y) {
				t.Fatalf("ToLayers changed pixel %d,%d", x, y)
			}
		}
	}
	if q.LayerPixels(0) != 80 || q.LayerPixels(1) != 0 {
		t.Fatalf("layer pixels: %d %d", q.LayerPixels(0), q.LayerPixels(1))
	}
}

func TestLayersStackAndVisibility(t *testing.T) {
	p := NewPicture(0, 0)
	p.ToLayers()
	p.Active = 0
	p.Plot(10, 10, 2, ConflictBlock, nil) // layer 2 (index 0, bottom): red
	p.Active = 2
	p.Plot(10, 10, 7, ConflictBlock, nil) // layer 4 (index 2, top): yellow
	if p.ColorAt(10, 10) != 7 {
		t.Fatal("the top layer must win")
	}
	p.SetLayerHidden(2, true)
	if p.ColorAt(10, 10) != 2 {
		t.Fatal("hiding the top layer must show the layer below")
	}
	p.SetLayerHidden(2, false)
	p.Erase(10, 10)
	if p.ColorAt(10, 10) != 2 {
		t.Fatal("erasing the top pixel must show the layer below")
	}
	p.Active = 0
	p.Erase(10, 10)
	if p.ColorAt(10, 10) != 0 {
		t.Fatal("erased everywhere: background expected")
	}
	cellColorsOK(t, p)
}

func TestLayersRulesOverAllLayers(t *testing.T) {
	p := NewPicture(0, 0)
	p.ToLayers()
	// layer 2: three colors in cell 0
	p.Plot(0, 0, 2, ConflictBlock, nil)
	p.Plot(1, 0, 5, ConflictBlock, nil)
	p.Active = 1
	p.Plot(2, 0, 7, ConflictBlock, nil)
	// a fourth color on another layer is blocked, even with layer 2 hidden
	p.Active = 2
	p.SetLayerHidden(0, true)
	if r := p.Plot(3, 3, 14, ConflictBlock, nil); r != PlotBlocked {
		t.Fatalf("4th color must be blocked, got %d", r)
	}
	// existing colors and the background are always allowed
	if r := p.Plot(3, 3, 5, ConflictBlock, nil); r != PlotOK {
		t.Fatal("an existing color must be allowed")
	}
	if r := p.Plot(3, 4, 0, ConflictBlock, nil); r != PlotOK {
		t.Fatal("the background must be allowed")
	}
	// replace: the rarest color is replaced in all layers
	p.SetLayerHidden(0, false)
	if r := p.Plot(3, 5, 14, ConflictReplace, nil); r != PlotReplaced {
		t.Fatalf("replace expected, got %d", r)
	}
	cellColorsOK(t, p)
	pal := &Palettes[0]
	if r := p.Plot(3, 6, 1, ConflictNearest, pal); r != PlotSubstituted {
		t.Fatalf("nearest expected, got %d", r)
	}
	cellColorsOK(t, p)
	if p.FullCells() != 1 {
		t.Fatalf("full cells: %d", p.FullCells())
	}
}

func TestLayersBackgroundAndSwap(t *testing.T) {
	p := NewPicture(0, 0)
	p.ToLayers()
	p.Plot(0, 0, 6, ConflictBlock, nil)
	p.Active = 1
	p.Plot(1, 0, 6, ConflictBlock, nil)
	p.Plot(2, 0, 0, ConflictBlock, nil) // explicit background pixel
	// swap blue -> green in every layer
	px, cells := p.ReplaceColor(6, 5, false)
	if px != 2 || cells != 1 || p.ColorAt(0, 0) != 5 || p.ColorAt(1, 0) != 5 {
		t.Fatalf("swap in layers: %d px %d cells, colors %d %d", px, cells, p.ColorAt(0, 0), p.ColorAt(1, 0))
	}
	// changing the background: explicit background pixels follow it, pixels
	// in the new background color become background pixels
	p.SetBG(5)
	if p.ColorAt(2, 0) != 5 || p.L[0][0] != LayerBG {
		t.Fatal("background change in layers")
	}
	p.SetBG(9)
	if p.ColorAt(0, 0) != 9 || p.ColorAt(2, 0) != 9 {
		t.Fatal("background pixels must follow $D021")
	}
	cellColorsOK(t, p)
}
