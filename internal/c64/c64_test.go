package c64

import (
	"bytes"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func randomPicture(seed int64) *Picture {
	r := rand.New(rand.NewSource(seed))
	p := NewPicture(6, 14)
	pal := &Palettes[0]
	for i := 0; i < 20000; i++ {
		p.Plot(r.Intn(W), r.Intn(H), uint8(r.Intn(16)), ConflictMode(r.Intn(3)), pal)
	}
	return p
}

func TestRulesAlwaysHold(t *testing.T) {
	p := randomPicture(1)
	// Jede Zelle darf höchstens 4 sichtbare Farben haben, eine davon global.
	for cell := 0; cell < Cells; cell++ {
		seen := map[uint8]bool{}
		cx, cy := (cell%Cols)*4, (cell/Cols)*8
		for y := cy; y < cy+8; y++ {
			for x := cx; x < cx+4; x++ {
				seen[p.ColorAt(x, y)] = true
			}
		}
		if len(seen) > 4 {
			t.Fatalf("Zelle %d hat %d Farben", cell, len(seen))
		}
	}
}

func TestPlotBlock(t *testing.T) {
	p := NewPicture(0, 0)
	for i, c := range []uint8{2, 3, 4} {
		if r := p.Plot(i, 0, c, ConflictBlock, nil); r != PlotOK {
			t.Fatal("sollte passen")
		}
	}
	if r := p.Plot(3, 0, 5, ConflictBlock, nil); r != PlotBlocked {
		t.Fatal("5. Farbe muss blockiert werden")
	}
	// Bildpunkt mit einziger Nutzung einer Farbe überschreiben → Bitpaar wird frei
	if r := p.Plot(0, 0, 5, ConflictBlock, nil); r != PlotOK || p.ColorAt(0, 0) != 5 {
		t.Fatal("frei gewordenes Bitpaar muss genutzt werden")
	}
	if p.ColorAt(1, 0) != 3 || p.ColorAt(2, 0) != 4 {
		t.Fatal("andere Bildpunkte dürfen sich nicht ändern")
	}
	if r := p.Plot(3, 1, 0, ConflictBlock, nil); r != PlotOK {
		t.Fatal("Hintergrund geht immer")
	}
}

func TestKLARoundTrip(t *testing.T) {
	p := randomPicture(2)
	b := p.SaveKLA()
	if len(b) != 10003 || b[0] != 0x00 || b[1] != 0x60 {
		t.Fatalf("KLA-Größe/Ladeadresse falsch: %d %02x%02x", len(b), b[1], b[0])
	}
	q, err := LoadKLA(b)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			if p.ColorAt(x, y) != q.ColorAt(x, y) {
				t.Fatalf("Unterschied bei %d,%d", x, y)
			}
		}
	}
	// Bitmap-Layout: Bildpunkt (5,9) liegt in Zelle (1,1), Byte-Zeile 1, Bits 5-4
	p2 := NewPicture(0, 0)
	p2.Plot(5, 9, 2, ConflictBlock, nil)
	bm := p2.Bitmap()
	off := (1*40+1)*8 + 1
	if bm[off] != 0b00010000 {
		t.Fatalf("Bitmap-Layout falsch: %08b", bm[off])
	}
	if p2.Screen[41] != 0x20 {
		t.Fatalf("Farbe 2 sollte in Bits 4-7 (Bitpaar %%01) stehen: %02x", p2.Screen[41])
	}
}

// --- Mini-6502 zum Prüfen des PRG-Anzeigers ---------------------------------

type cpu struct {
	a, x, y, sp uint8
	pc          uint16
	z, n, c     bool
	mem         [65536]byte
	writes      map[uint16]byte
	keyPressed  bool
}

func (c *cpu) rd(a uint16) byte {
	if a == 0xDC01 {
		if c.keyPressed {
			return 0xEF
		}
		return 0xFF
	}
	return c.mem[a]
}
func (c *cpu) wr(a uint16, v byte) { c.mem[a] = v; c.writes[a] = v }
func (c *cpu) nz(v uint8)          { c.z = v == 0; c.n = v&0x80 != 0 }
func (c *cpu) w16(a uint16) uint16 { return uint16(c.rd(a)) | uint16(c.rd(a+1))<<8 }

func (c *cpu) step(t *testing.T) bool {
	op := c.rd(c.pc)
	switch op {
	case 0x78: // SEI
		c.pc++
	case 0xA9:
		c.a = c.rd(c.pc + 1)
		c.nz(c.a)
		c.pc += 2
	case 0xA2:
		c.x = c.rd(c.pc + 1)
		c.nz(c.x)
		c.pc += 2
	case 0x8D:
		c.wr(c.w16(c.pc+1), c.a)
		c.pc += 3
	case 0xAD:
		c.a = c.rd(c.w16(c.pc + 1))
		c.nz(c.a)
		c.pc += 3
	case 0xBD:
		c.a = c.rd(c.w16(c.pc+1) + uint16(c.x))
		c.nz(c.a)
		c.pc += 3
	case 0x9D:
		c.wr(c.w16(c.pc+1)+uint16(c.x), c.a)
		c.pc += 3
	case 0xE8:
		c.x++
		c.nz(c.x)
		c.pc++
	case 0xD0:
		off := int8(c.rd(c.pc + 1))
		c.pc += 2
		if !c.z {
			c.pc = uint16(int(c.pc) + int(off))
		}
	case 0x29:
		c.a &= c.rd(c.pc + 1)
		c.nz(c.a)
		c.pc += 2
	case 0x09:
		c.a |= c.rd(c.pc + 1)
		c.nz(c.a)
		c.pc += 2
	case 0x4C:
		c.pc = c.w16(c.pc + 1)
		return false
	default:
		t.Fatalf("unbekannter Opcode %02x bei %04x", op, c.pc)
	}
	return true
}

func TestPRGViewer(t *testing.T) {
	p := randomPicture(3)
	p.Border = 11
	prg := p.PRG()
	if prg[0] != 0x01 || prg[1] != 0x08 {
		t.Fatal("Ladeadresse muss $0801 sein")
	}
	c := &cpu{writes: map[uint16]byte{}}
	c.mem[0xDD00] = 0x97
	copy(c.mem[0x0801:], prg[2:])
	// BASIC-Zeile prüfen: SYS 2061
	if c.mem[0x0805] != 0x9E || string(c.mem[0x0806:0x080A]) != "2061" {
		t.Fatal("BASIC-Startzeile falsch")
	}
	c.pc = 2061
	steps := 0
	for steps < 200000 {
		if c.pc == 0x0800+0x0100 { // nie
			break
		}
		// Nach dem Einschalten der Grafik Taste „drücken“
		if _, ok := c.writes[0xD018]; ok && steps > 0 {
			c.keyPressed = true
		}
		if !c.step(t) {
			break
		}
		steps++
	}
	if c.pc != 0xFCE2 {
		t.Fatalf("Programm endet nicht im Reset: %04x nach %d Schritten", c.pc, steps)
	}
	exp := map[uint16]byte{0xD011: 0x3B, 0xD016: 0x18, 0xD018: 0x18, 0xD020: 11, 0xD021: p.BG}
	for a, v := range exp {
		if c.writes[a] != v {
			t.Fatalf("Register %04x = %02x, erwartet %02x", a, c.writes[a], v)
		}
	}
	if c.writes[0xDD00]&3 != 3 {
		t.Fatal("VIC-Bank 0 nicht gesetzt")
	}
	bm := p.Bitmap()
	if !bytes.Equal(c.mem[0x2000:0x2000+8000], bm) {
		t.Fatal("Bitmap liegt nicht bei $2000")
	}
	for i := 0; i < 1000; i++ {
		if c.mem[0x0400+i] != p.Screen[i] {
			t.Fatalf("Screen RAM falsch bei %d", i)
		}
		if c.mem[0xD800+i]&15 != p.ColorRAM[i]&15 {
			t.Fatalf("Color RAM falsch bei %d", i)
		}
	}
}

func TestConvertAndPNG(t *testing.T) {
	p := randomPicture(4)
	pal := &Palettes[FindPalette(DefaultPalette)]
	data, err := p.SavePNG(pal, PNG320)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 320 || img.Bounds().Dy() != 200 {
		t.Fatal("PNG-Größe falsch")
	}
	// Ein exaktes C64-Bild muss sich verlustfrei zurückwandeln lassen.
	q := Convert(img, pal, ConvertOptions{BG: -1})
	diff := 0
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			if p.ColorAt(x, y) != q.ColorAt(x, y) {
				diff++
			}
		}
	}
	if diff != 0 {
		t.Fatalf("%d Bildpunkte weichen nach PNG→MC ab", diff)
	}
}

func TestBMPFiles(t *testing.T) {
	files, _ := filepath.Glob("testdata/*.bmp")
	for _, f := range files {
		d, _ := os.ReadFile(f)
		img, err := DecodeBMP(d)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		ref, _ := os.ReadFile(f[:len(f)-4] + ".ref.png")
		ri, err := png.Decode(bytes.NewReader(ref))
		if err != nil {
			continue
		}
		b := img.Bounds()
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				r1, g1, b1, _ := img.At(x, y).RGBA()
				r2, g2, b2, _ := ri.At(x, y).RGBA()
				if r1>>8 != r2>>8 || g1>>8 != g2>>8 || b1>>8 != b2>>8 {
					t.Fatalf("%s: Pixel %d,%d weicht ab", f, x, y)
				}
			}
		}
	}
	t.Logf("%d BMP-Dateien geprüft", len(files))
}

func TestReplaceColor(t *testing.T) {
	pal := &Palettes[0]
	p := NewPicture(0, 0)
	// cell 0: red, yellow, green; cell 1: red only; cell 2: red + white
	for i, c := range []uint8{2, 7, 5, 2} {
		p.Plot(i, 0, c, ConflictBlock, pal)
	}
	p.Plot(4, 0, 2, ConflictBlock, pal)
	p.Plot(8, 0, 2, ConflictBlock, pal)
	p.Plot(9, 0, 1, ConflictBlock, pal)
	q := p.Clone()
	px, cells := q.ReplaceColor(2, 1, false)
	if px != 4 || cells != 3 {
		t.Fatalf("replace red->white: %d pixels, %d cells", px, cells)
	}
	if q.ColorAt(0, 0) != 1 || q.ColorAt(3, 0) != 1 || q.ColorAt(4, 0) != 1 || q.ColorAt(8, 0) != 1 || q.ColorAt(1, 0) != 7 {
		t.Fatal("wrong colors after replace")
	}
	// cell 2 had white already: red pixels moved, slot freed
	if u := q.Usage(2, -1, -1); u[1]+u[2]+u[3] != 2 {
		t.Fatalf("usage cell 2: %v", u)
	}
	q2 := p.Clone()
	q2.ReplaceColor(2, 1, true)
	if q2.ColorAt(9, 0) != 2 || q2.ColorAt(8, 0) != 1 {
		t.Fatal("exchange both ways failed")
	}
	// background is never touched
	q3 := p.Clone()
	if n, _ := q3.ReplaceColor(0, 2, false); n != 0 || *q3 != *p {
		t.Fatal("background must not be replaced")
	}
	if n, _ := q3.ReplaceColor(2, 0, true); n != 0 {
		t.Fatal("must not exchange with the background color")
	}
	if n, _ := q3.ReplaceColor(2, 0, false); n != 4 || q3.ColorAt(8, 0) != 0 || q3.ColorAt(9, 0) != 1 {
		t.Fatal("a color can become background")
	}
	// rules still hold
	for cell := 0; cell < Cells; cell++ {
		seen := map[uint8]bool{}
		for y := 0; y < 8; y++ {
			for x := 0; x < 4; x++ {
				seen[q.ColorAt((cell%Cols)*4+x, (cell/Cols)*8+y)] = true
			}
		}
		if len(seen) > 4 {
			t.Fatal("rule broken")
		}
	}
}

func TestLoadOCPAndAMI(t *testing.T) {
	p := NewPicture(6, 14)
	for y := 0; y < H; y += 3 {
		for x := 0; x < W; x += 2 {
			p.Plot(x, y, uint8((x/4+y/8)%16), ConflictReplace, nil)
		}
	}
	// Advanced Art Studio layout
	ocp := make([]byte, ocpSize)
	ocp[0], ocp[1] = 0x00, 0x20
	copy(ocp[2:], p.Bitmap())
	copy(ocp[0x1f42:], p.Screen[:])
	for i := 0; i < Cells; i++ {
		ocp[0x233a+i] = p.ColorRAM[i]
	}
	ocp[0x232a], ocp[0x232b] = p.Border, p.BG
	q, err := LoadOCP(ocp)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			if q.ColorAt(x, y) != p.ColorAt(x, y) {
				t.Fatalf("OCP pixel %d,%d", x, y)
			}
		}
	}
	if q.Border != 14 || q.BG != 6 {
		t.Fatal("OCP colors")
	}
	ami := p.SaveAMI()
	if len(ami) >= klaSize {
		t.Fatalf("AMI not packed: %d", len(ami))
	}
	r, err := LoadAMI(ami)
	if err != nil {
		t.Fatal(err)
	}
	if string(r.KLABytes()) != string(p.KLABytes()) {
		t.Fatal("AMI round trip")
	}
	if _, err := LoadAMI([]byte{0, 0x40, 1, 2, 3}); err == nil {
		t.Fatal("short AMI must fail")
	}
}
