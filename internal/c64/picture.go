// Package c64 bildet den Multicolor-Bitmap-Modus des C64 exakt nach.
//
// Ein Bild besteht aus 160×200 Bildpunkten mit je einem Bitpaar:
//
//	%00 → Hintergrundfarbe ($D021, global)
//	%01 → Bits 4–7 des Bildschirmspeichers (Screen RAM) der Zelle
//	%10 → Bits 0–3 des Bildschirmspeichers der Zelle
//	%11 → Farbspeicher (Color RAM, $D800) der Zelle
//
// Eine Zelle ist 4×8 Bildpunkte groß (40×25 Zellen). Pro Zelle sind also
// höchstens vier Farben möglich, davon eine global.
package c64

const (
	W     = 160
	H     = 200
	Cols  = 40
	Rows  = 25
	Cells = Cols * Rows
)

// Picture enthält genau die Daten, die der C64 anzeigt.
type Picture struct {
	Bits     [W * H]uint8 // Bitpaar je Bildpunkt (0..3)
	Screen   [Cells]uint8 // Bildschirmspeicher: hohes Nibble = %01, niedriges = %10
	ColorRAM [Cells]uint8 // Farbspeicher (nur Bits 0–3) = %11
	BG       uint8        // Hintergrundfarbe $D021 = %00
	Border   uint8        // Rahmenfarbe $D020 (nur für PRG/Anzeige)

	// Paint layers of the editor (see layers.go). Bits, Screen and ColorRAM
	// then hold the composite of the visible layers.
	Layered bool
	L       [LayerCount][W * H]uint8 // 0 = transparent, 1..16 = color+1, 17 = background
	Hide    [LayerCount]bool
	Active  int // layer that tools paint into
}

// ConflictMode bestimmt, was passiert, wenn eine Zelle schon vier Farben hat.
type ConflictMode int

const (
	ConflictBlock   ConflictMode = iota // Bildpunkt wird nicht gesetzt
	ConflictReplace                     // seltenste Zellfarbe wird ersetzt
	ConflictNearest                     // ähnlichste vorhandene Zellfarbe wird genutzt
)

// PlotResult beschreibt das Ergebnis eines Setzvorgangs.
type PlotResult int

const (
	PlotOK PlotResult = iota
	PlotBlocked
	PlotReplaced
	PlotSubstituted
)

func NewPicture(bg, border uint8) *Picture {
	p := &Picture{BG: bg & 15, Border: border & 15}
	return p
}

func (p *Picture) Clone() *Picture {
	q := *p
	return &q
}

// CellOf liefert den Zellindex eines Bildpunkts.
func CellOf(x, y int) int { return (y>>3)*Cols + (x >> 2) }

// SlotColor liefert die Farbe eines Bitpaars in einer Zelle.
func (p *Picture) SlotColor(cell, slot int) uint8 {
	switch slot {
	case 0:
		return p.BG
	case 1:
		return p.Screen[cell] >> 4
	case 2:
		return p.Screen[cell] & 15
	default:
		return p.ColorRAM[cell] & 15
	}
}

// SetSlotColor setzt die Farbe eines Bitpaars (1..3) einer Zelle.
func (p *Picture) SetSlotColor(cell, slot int, c uint8) {
	c &= 15
	switch slot {
	case 1:
		p.Screen[cell] = p.Screen[cell]&0x0F | c<<4
	case 2:
		p.Screen[cell] = p.Screen[cell]&0xF0 | c
	case 3:
		p.ColorRAM[cell] = c
	}
}

// ColorAt liefert die sichtbare Farbe eines Bildpunkts.
func (p *Picture) ColorAt(x, y int) uint8 {
	if x < 0 || y < 0 || x >= W || y >= H {
		return p.BG
	}
	return p.SlotColor(CellOf(x, y), int(p.Bits[y*W+x]))
}

// Usage zählt, wie viele Bildpunkte einer Zelle jedes Bitpaar nutzen.
// Der Bildpunkt (ex,ey) wird nicht mitgezählt (ex<0: alle zählen).
func (p *Picture) Usage(cell, ex, ey int) [4]int {
	if p.Layered {
		return p.usageLayers(cell, ex, ey)
	}
	var u [4]int
	cx, cy := (cell%Cols)*4, (cell/Cols)*8
	for y := cy; y < cy+8; y++ {
		row := p.Bits[y*W+cx : y*W+cx+4]
		for i, b := range row {
			if cx+i == ex && y == ey {
				continue
			}
			u[b]++
		}
	}
	return u
}

// CellColors liefert die 4 Farben einer Zelle und ob das Bitpaar benutzt wird.
func (p *Picture) CellColors(cell int) (cols [4]uint8, used [4]bool) {
	u := p.Usage(cell, -1, -1)
	for s := 0; s < 4; s++ {
		cols[s] = p.SlotColor(cell, s)
		used[s] = u[s] > 0
	}
	return
}

// Plot setzt einen Bildpunkt auf Farbe c und hält dabei die Multicolor-Regeln ein.
func (p *Picture) Plot(x, y int, c uint8, mode ConflictMode, pal *Palette) PlotResult {
	if x < 0 || y < 0 || x >= W || y >= H {
		return PlotOK
	}
	c &= 15
	if p.Layered {
		return p.plotLayer(x, y, c, mode, pal)
	}
	i := y*W + x
	cell := CellOf(x, y)
	if c == p.BG {
		p.Bits[i] = 0
		return PlotOK
	}
	cur := int(p.Bits[i])
	if cur != 0 && p.SlotColor(cell, cur) == c {
		return PlotOK
	}
	// 1. Farbe liegt schon in einem Bitpaar der Zelle
	for s := 1; s <= 3; s++ {
		if p.SlotColor(cell, s) == c {
			p.Bits[i] = uint8(s)
			return PlotOK
		}
	}
	// 2. freies Bitpaar (von keinem anderen Bildpunkt der Zelle benutzt)
	u := p.Usage(cell, x, y)
	for s := 1; s <= 3; s++ {
		if u[s] == 0 {
			p.SetSlotColor(cell, s, c)
			p.Bits[i] = uint8(s)
			return PlotOK
		}
	}
	// 3. Konflikt: die Zelle hat bereits Hintergrund + 3 Farben
	switch mode {
	case ConflictReplace:
		best := 1
		for s := 2; s <= 3; s++ {
			if u[s] <= u[best] {
				best = s
			}
		}
		p.SetSlotColor(cell, best, c)
		p.Bits[i] = uint8(best)
		return PlotReplaced
	case ConflictNearest:
		if pal == nil {
			return PlotBlocked
		}
		tr, tg, tb := rgbOf(pal.Colors[c])
		best, bd := 0, 1<<62
		for s := 0; s <= 3; s++ {
			if s > 0 && u[s] == 0 {
				continue
			}
			r, g, b := rgbOf(pal.Colors[p.SlotColor(cell, s)])
			if d := ColorDist(tr, tg, tb, r, g, b); d < bd {
				bd, best = d, s
			}
		}
		p.Bits[i] = uint8(best)
		return PlotSubstituted
	}
	return PlotBlocked
}

func rgbOf(c uint32) (int, int, int) { return int(c >> 16 & 255), int(c >> 8 & 255), int(c & 255) }

// Clear löscht das Bild (alle Bildpunkte = Hintergrund).
func (p *Picture) Clear() {
	for l := range p.L {
		p.L[l] = [W * H]uint8{}
	}
	p.Bits = [W * H]uint8{}
	p.Screen = [Cells]uint8{}
	p.ColorRAM = [Cells]uint8{}
}

// CleanUnused setzt unbenutzte Bitpaar-Farben auf 0 (rein kosmetisch, ändert nichts Sichtbares).
func (p *Picture) CleanUnused() {
	if p.Layered {
		p.Recomposite() // assigns only used colors
		return
	}
	for cell := 0; cell < Cells; cell++ {
		u := p.Usage(cell, -1, -1)
		for s := 1; s <= 3; s++ {
			if u[s] == 0 {
				p.SetSlotColor(cell, s, 0)
			}
		}
	}
}

// Bitmap liefert die 8000 Byte Bitmap im C64-Speicherlayout.
func (p *Picture) Bitmap() []byte {
	out := make([]byte, 8000)
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			off := ((y>>3)*Cols+(x>>2))*8 + (y & 7)
			shift := uint(6 - 2*(x&3))
			out[off] |= (p.Bits[y*W+x] & 3) << shift
		}
	}
	return out
}

// SetBitmap übernimmt 8000 Byte Bitmap im C64-Speicherlayout.
func (p *Picture) SetBitmap(b []byte) {
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			off := ((y>>3)*Cols+(x>>2))*8 + (y & 7)
			shift := uint(6 - 2*(x&3))
			p.Bits[y*W+x] = b[off] >> shift & 3
		}
	}
}

// Render schreibt das Bild als RGB in dst (160×200).
func (p *Picture) Render(pal *Palette, dst []uint32) {
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			dst[y*W+x] = pal.Colors[p.ColorAt(x, y)]
		}
	}
}

// Validate prüft die Regeln (immer erfüllt, da das Modell sie erzwingt) und
// zählt die Zellen mit 4 belegten Farben.
func (p *Picture) FullCells() int {
	n := 0
	for cell := 0; cell < Cells; cell++ {
		u := p.Usage(cell, -1, -1)
		if u[1] > 0 && u[2] > 0 && u[3] > 0 {
			n++
		}
	}
	return n
}

// ReplaceCellColor changes color "from" into color "to" inside one cell by
// changing the color sources of the cell, never the number of colors, so the
// multicolor rules always hold. With both=true the two colors are exchanged.
// The background color ($D021, bit pair %00) is never changed. It returns
// the number of pixels whose color changed.
func (p *Picture) ReplaceCellColor(cell int, from, to uint8, both bool) int {
	from &= 15
	to &= 15
	if from == to || from == p.BG || cell < 0 || cell >= Cells || (to == p.BG && both) {
		return 0
	}
	if p.Layered {
		return p.replaceCellColorLayers(cell, from, to, both)
	}
	u := p.Usage(cell, -1, -1)
	if to == p.BG {
		// the pixels become background pixels (%00)
		changed := 0
		cx, cy := (cell%Cols)*4, (cell/Cols)*8
		for y := cy; y < cy+8; y++ {
			for x := cx; x < cx+4; x++ {
				if b := p.Bits[y*W+x]; b != 0 && p.SlotColor(cell, int(b)) == from {
					p.Bits[y*W+x] = 0
					changed++
				}
			}
		}
		return changed
	}
	var fromSlots, toSlots []int
	for s := 1; s <= 3; s++ {
		if u[s] == 0 {
			continue
		}
		switch p.SlotColor(cell, s) {
		case from:
			fromSlots = append(fromSlots, s)
		case to:
			toSlots = append(toSlots, s)
		}
	}
	changed := 0
	if both {
		for _, s := range fromSlots {
			p.SetSlotColor(cell, s, to)
			changed += u[s]
		}
		for _, s := range toSlots {
			p.SetSlotColor(cell, s, from)
			changed += u[s]
		}
		return changed
	}
	if len(fromSlots) == 0 {
		return 0
	}
	if len(toSlots) > 0 {
		// "to" is already in the cell: move the pixels over and free the slot
		t := uint8(toSlots[0])
		cx, cy := (cell%Cols)*4, (cell/Cols)*8
		for y := cy; y < cy+8; y++ {
			for x := cx; x < cx+4; x++ {
				b := int(p.Bits[y*W+x])
				for _, s := range fromSlots {
					if b == s {
						p.Bits[y*W+x] = t
						changed++
					}
				}
			}
		}
		return changed
	}
	for _, s := range fromSlots {
		p.SetSlotColor(cell, s, to)
		changed += u[s]
	}
	return changed
}

// ReplaceColor runs ReplaceCellColor for every cell. It returns the number of
// changed pixels and cells.
func (p *Picture) ReplaceColor(from, to uint8, both bool) (pixels, cells int) {
	for c := 0; c < Cells; c++ {
		if n := p.ReplaceCellColor(c, from, to, both); n > 0 {
			pixels += n
			cells++
		}
	}
	return
}
