package c64

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSpritePixels(t *testing.T) {
	s := Sprite{Multi: true, Color: 2}
	for slot := 0; slot < 4; slot++ {
		s.Set(slot, 0, slot)
	}
	for slot := 0; slot < 4; slot++ {
		if g := s.Get(slot, 0); g != slot {
			t.Fatalf("multicolor slot %d read as %d", slot, g)
		}
	}
	// %00 %10 %01 %11 = sprite color is %10, MC1 %01
	if s.Data[0] != 0b00100111 {
		t.Fatalf("multicolor byte %08b", s.Data[0])
	}
	h := Sprite{Color: 1}
	h.Set(0, 0, 1)
	h.Set(23, 20, 1)
	if h.Data[0] != 0x80 || h.Data[62] != 0x01 {
		t.Fatal("hires bits")
	}
	h.FlipH()
	if !(h.Get(23, 0) == 1 && h.Get(0, 20) == 1 && h.Get(0, 0) == 0) {
		t.Fatal("flip")
	}
	h.Shift(1, 0)
	if h.Get(0, 0) != 1 {
		t.Fatal("shift wraps")
	}
}

func TestSpriteImport(t *testing.T) {
	px := make([]int8, 320*200)
	// background blue; sprite 0 at (0,0) a red block with white and black parts
	for i := range px {
		px[i] = 6
	}
	for y := 0; y < 10; y++ {
		for x := 0; x < 12; x++ {
			px[y*320+x] = 2
			px[y*320+x+30] = 2 // sprite 1 (gap 6)
		}
	}
	for x := 0; x < 24; x++ {
		px[15*320+x] = 1
		px[18*320+x] = 0
		px[15*320+x+30] = 1
		px[18*320+x+30] = 7
	}
	b := NewSpriteBank()
	dist := func(a, b int) int {
		if a == b {
			return 0
		}
		return 1
	}
	out, bg, mc1, mc2 := ImportSprites(px, b, SpriteImport{Cols: 2, Rows: 1, GapX: 6, Multi: true, BG: -1}, dist)
	if len(out) != 2 || bg != 6 {
		t.Fatalf("got %d sprites, bg %d", len(out), bg)
	}
	// red and white are in both sprites → shared colors
	if !((mc1 == 2 && mc2 == 1) || (mc1 == 1 && mc2 == 2)) {
		t.Fatalf("shared colors %d %d", mc1, mc2)
	}
	if out[0].Color != 0 || out[1].Color != 7 {
		t.Fatalf("sprite colors %d %d", out[0].Color, out[1].Color)
	}
	if out[0].Get(0, 0) != 2 || out[0].Get(0, 12) != 0 {
		t.Fatalf("pixel mapping %d %d %d %d", out[0].Get(0, 0), out[0].Get(0, 12), mc1, mc2)
	}
	hi, _, _, _ := ImportSprites(px, b, SpriteImport{Cols: 1, Rows: 1, BG: 6}, dist)
	if hi[0].Multi || hi[0].Get(23, 15) != 1 || hi[0].Get(23, 0) != 0 || hi[0].Get(5, 0) != 1 {
		t.Fatal("hires import")
	}
}

func TestSpriteFileAndACME(t *testing.T) {
	b := NewSpriteBank()
	b.Sprites[0].Set(3, 4, 2)
	b.Sprites = append(b.Sprites, Sprite{Color: 7, ExpandX: true})
	b.Sprites[1].Set(10, 10, 1)
	b.AnimTo = 1
	data, err := b.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	c, err := LoadSpriteBank(data)
	if err != nil || len(c.Sprites) != 2 || c.Sprites[1] != b.Sprites[1] || c.Sprites[0] != b.Sprites[0] || c.AnimTo != 1 {
		t.Fatal("round trip", err)
	}
	acme := os.Getenv("ACME")
	if acme == "" {
		acme, _ = exec.LookPath("acme")
	}
	if acme == "" {
		t.Skip("acme not found")
	}
	dir := t.TempDir()
	src := b.ACME("my sprites", 0x2000)
	os.WriteFile(filepath.Join(dir, "s.asm"), []byte(src), 0o644)
	cmd := exec.Command(acme, "--cpu", "6510", "s.asm")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("acme: %v\n%s", err, out)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "my sprites.prg"))
	if len(got) < 2+128 || got[0] != 0 || got[1] != 0x20 || !bytes.Equal(got[2:2+128], b.RawData()) {
		t.Fatalf("assembled sprite data differs (%d bytes)", len(got))
	}
}

func TestFontACMEAndFile(t *testing.T) {
	f := NewFont(0)
	f.SetPixel(1, 0, 0, false)
	f.Tile = 3
	f.TileLayout = 1
	if f.TileChar(1, 1, 1) != 193 {
		t.Fatal("tile layout")
	}
	f.SetTilePixel(1, 15, 15, 1)
	if !f.Pixel(193, 7, 7) {
		t.Fatal("tile pixel")
	}
	data, _ := f.Marshal()
	g, err := LoadFont(data)
	if err != nil || g.Data != f.Data || g.Tile != 3 || g.TileLayout != 1 {
		t.Fatal("font round trip", err)
	}
	raw, err := LoadRawFont(append([]byte{0, 0x30}, f.Data[:]...))
	if err != nil || raw.Data != f.Data {
		t.Fatal("raw font")
	}
	px := make([]int8, 320*200)
	px[0] = 1
	px[8*320+8+7] = 1 // cell 41, pixel (7,0)
	imp := NewFont(-1)
	imp.ImportFont(px, FontImport{StartCell: 0, Count: 42, Target: 0, BG: -1}, nil)
	if imp.Data[0] != 0x80 || imp.Data[41*8] != 0x01 {
		t.Fatal("font import")
	}
	acme := os.Getenv("ACME")
	if acme == "" {
		acme, _ = exec.LookPath("acme")
	}
	if acme == "" {
		t.Skip("acme not found")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "f.asm"), []byte(f.ACME("font", 0x3000)), 0o644)
	cmd := exec.Command(acme, "--cpu", "6510", "f.asm")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("acme: %v\n%s", err, out)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "font.prg"))
	if len(got) != 2050 || got[1] != 0x30 || !bytes.Equal(got[2:], f.Data[:]) {
		t.Fatalf("assembled font differs (%d bytes)", len(got))
	}
}

func TestFontMulticolor(t *testing.T) {
	f := NewFont(-1)
	f.Multi = true
	f.BG, f.MC1, f.MC2, f.FG = 0, 2, 5, 7
	for v := 0; v < 4; v++ {
		f.Set(1, v, 0, v)
	}
	if f.Data[8] != 0b00011011 {
		t.Fatalf("multicolor byte %08b", f.Data[8])
	}
	if f.ColorAt(1, 2, 0) != 2 || f.ColorAt(1, 5, 0) != 5 || f.ColorAt(1, 7, 0) != 7 {
		t.Fatal("multicolor colors")
	}
	f.FG = 15 // only 0..7 in multicolor
	if f.SlotColor(3) != 7 {
		t.Fatal("character color 0..7")
	}
	data, _ := f.Marshal()
	g, err := LoadFont(data)
	if err != nil || !g.Multi || g.MC1 != 2 || g.MC2 != 5 {
		t.Fatal("multicolor project", err)
	}
	// import: pixel pairs, colors found automatically
	px := make([]int8, 320*200)
	for i := range px {
		px[i] = 6
	}
	for x := 0; x < 8; x++ {
		px[x] = 2 // row 0 red (most frequent after blue)
		px[320+x] = 2
		px[2*320+x] = 5
	}
	px[3*320] = 1
	px[3*320+1] = 1
	dist := func(a, b int) int {
		if a == b {
			return 0
		}
		return 1
	}
	h := NewFont(-1)
	h.ImportFont(px, FontImport{Count: 1, BG: -1, Multi: true}, dist)
	if !h.Multi || h.BG != 6 || h.FG != 2 || h.MC1 != 5 || h.MC2 != 1 {
		t.Fatalf("import colors bg %d fg %d mc %d %d", h.BG, h.FG, h.MC1, h.MC2)
	}
	if h.Get(0, 0, 0) != 3 || h.Get(0, 0, 2) != 1 || h.Get(0, 0, 3) != 2 || h.Get(0, 1, 3) != 0 {
		t.Fatal("import pixels")
	}
}
