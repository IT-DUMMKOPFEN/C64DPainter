package c64

import "testing"

func TestHiresRules(t *testing.T) {
	p := NewHiresPicture(ModeHires, 0, 0)
	if p.Width() != 320 || !p.Layered {
		t.Fatal("hires picture")
	}
	if r := p.Plot(9, 0, 2, ConflictBlock, nil); r != PlotOK || p.ColorAt(9, 0) != 2 {
		t.Fatal("first color")
	}
	// cell 1 has paper (black) + red: a third color is blocked
	if r := p.Plot(10, 0, 5, ConflictBlock, nil); r != PlotBlocked || p.ColorAt(10, 0) != 0 {
		t.Fatal("third color must be blocked")
	}
	// other cells are free
	if r := p.Plot(0, 0, 5, ConflictBlock, nil); r != PlotOK {
		t.Fatal("other cell")
	}
	// replace: red becomes green in the cell
	if r := p.Plot(10, 0, 5, ConflictReplace, nil); r != PlotReplaced || p.ColorAt(9, 0) != 5 {
		t.Fatal("replace")
	}
	// fill a whole cell with two colors, then erasing would bring the paper
	for y := 16; y < 24; y++ {
		for x := 16; x < 24; x++ {
			p.Plot(x, y, 7, ConflictBlock, nil)
		}
	}
	for y := 16; y < 24; y++ {
		for x := 20; x < 24; x++ {
			if p.Plot(x, y, 2, ConflictBlock, nil) != PlotOK {
				t.Fatal("second color without paper")
			}
		}
	}
	p.Erase(16, 16)
	if p.ColorAt(16, 16) != 7 {
		t.Fatal("erasing must not bring a third color")
	}
	// free mode: no rules
	f := NewHiresPicture(ModeFree, 0, 0)
	for i, c := range []uint8{1, 2, 3, 4} {
		if f.Plot(i, 0, c, ConflictBlock, nil) != PlotOK {
			t.Fatal("free mode")
		}
	}
	if !f.CellClash(0) || f.HiresClashes() != 1 {
		t.Fatal("clash count")
	}
}

func TestHiresFiles(t *testing.T) {
	pal := &Palettes[0]
	p := NewHiresPicture(ModeHires, 6, 14)
	for x := 0; x < 40; x++ {
		p.Plot(x, 3, 1, ConflictBlock, nil)
		p.Plot(x+100, 50, 2, ConflictBlock, nil)
	}
	for _, kind := range []string{"art", "dd"} {
		var data []byte
		var q *Picture
		var err error
		if kind == "art" {
			data = p.SaveART(pal)
			q, err = LoadART(data)
		} else {
			data = p.SaveDD(pal)
			q, err = LoadDD(data)
		}
		if err != nil {
			t.Fatal(kind, err)
		}
		q.ToLayers()
		for _, pt := range [][2]int{{0, 3}, {39, 3}, {100, 50}, {200, 100}, {5, 5}} {
			if q.ColorAt(pt[0], pt[1]) != p.ColorAt(pt[0], pt[1]) {
				t.Fatalf("%s: pixel %v: %d != %d", kind, pt, q.ColorAt(pt[0], pt[1]), p.ColorAt(pt[0], pt[1]))
			}
		}
		if q.BG != 6 {
			t.Fatalf("%s: paper %d", kind, q.BG)
		}
	}
	if len(p.SaveART(pal)) != 9009 || len(p.SaveDD(pal)) != 9218 {
		t.Fatal("file sizes")
	}
	prg := p.HiresPRG(pal)
	if len(prg) != 2+(0x2000-0x0801)+9000 {
		t.Fatalf("prg size %d", len(prg))
	}
	// free mode: reduced to two colors per cell
	f := NewHiresPicture(ModeFree, 0, 0)
	f.Plot(0, 0, 1, ConflictBlock, nil)
	f.Plot(1, 0, 1, ConflictBlock, nil)
	f.Plot(2, 0, 15, ConflictBlock, nil) // light grey: nearer to white than to black
	bm, scr := f.HiresData(pal)
	if scr[0] != 0x10 || bm[0] != 0xE0 {
		t.Fatalf("reduced cell: screen %02x bitmap %08b", scr[0], bm[0])
	}
}

func TestConvertHires(t *testing.T) {
	pal := &Palettes[0]
	px := make([][3]int, HW*H)
	for y := 0; y < H; y++ {
		for x := 0; x < HW; x++ {
			c := pal.Colors[(x/20+y/25)%16]
			px[y*HW+x] = [3]int{int(c >> 16 & 255), int(c >> 8 & 255), int(c & 255)}
		}
	}
	for _, m := range []CellMethod{CellsBestFit, CellsFrequent, CellsFixed} {
		p := ConvertHiresRGB(px, pal, ConvertOptions{BG: -1, Cells: m, Dither: DitherFloyd, Strength: 60}, ModeHires)
		p.ToLayers()
		for cell := 0; cell < Cells; cell++ {
			if n := len(p.hVisibleCellColors(cell)); n > 2 {
				t.Fatalf("method %d: cell %d has %d colors", m, cell, n)
			}
		}
	}
	f := ConvertHiresRGB(px, pal, ConvertOptions{BG: -1}, ModeFree)
	if f.HP[0] != 0 || f.HP[25*HW] != 1 || f.HP[21] != 1 {
		t.Fatalf("free conversion %d %d %d", f.HP[0], f.HP[25*HW], f.HP[21])
	}
}

func TestHiresPRGRuns(t *testing.T) {
	pal := &Palettes[0]
	p := NewHiresPicture(ModeHires, 6, 14)
	for x := 0; x < 320; x += 3 {
		p.Plot(x, x%200, 1, ConflictBlock, nil)
	}
	mem := run6502(t, p.HiresPRG(pal))
	bm, scr := p.HiresData(pal)
	if mem[0xD020] != 14 || mem[0xD011] != 0x3B || mem[0xD016] != 0x08 || mem[0xD018] != 0x18 {
		t.Fatal("VIC registers")
	}
	for i := 0; i < 1000; i++ {
		if mem[0x0400+i] != scr[i] {
			t.Fatalf("screen RAM %d not copied", i)
		}
	}
	for i := 0; i < 8000; i++ {
		if mem[0x2000+i] != bm[i] {
			t.Fatalf("bitmap byte %d not at $2000", i)
		}
	}
	// the multicolor viewer still works with the emulator
	m := NewPicture(6, 14)
	if mem := run6502(t, m.PRG()); mem[0xD016] != 0x18 {
		t.Fatal("multicolor viewer")
	}
}
