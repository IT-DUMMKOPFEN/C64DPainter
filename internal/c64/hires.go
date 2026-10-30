package c64

import (
	"errors"
	"fmt"
	"sort"
)

// --- Hires modes ---------------------------------------------------------------------
//
// Besides multicolor a picture can be in the standard bitmap mode ("hires"):
// 320 × 200 pixels, every 8 × 8 cell has two colors (screen RAM: high nibble
// for the set bits, low nibble for the clear bits) and there is no common
// background. "Hires free" has the same size but no color rules at all; it
// is reduced to two colors per cell when it is exported for the C64.
//
// Hires pictures always use the layers (L). The pixel values are the same as
// in multicolor; pixels where no visible layer paints show the paper color
// (BG). HP holds the composite: the color of every pixel.

// Mode is the graphics mode of a picture.
type Mode uint8

const (
	ModeMulti Mode = iota // multicolor bitmap, 160 × 200, 4 × 8 cells, BG + 3 colors
	ModeHires             // standard bitmap, 320 × 200, 8 × 8 cells, 2 colors
	ModeFree              // 320 × 200 with all colors (no rules)
	modeCount
)

// ModeNames are the display names of the modes.
var ModeNames = [modeCount]string{"Multicolor", "Hires", "Hires free"}

// HW is the width of a hires picture.
const HW = 320

// Hires reports whether the picture is 320 pixels wide.
func (p *Picture) Hires() bool { return p.Mode != ModeMulti }

// Width returns the width in pixels (160 multicolor, 320 hires).
func (p *Picture) Width() int {
	if p.Hires() {
		return HW
	}
	return W
}

// CellW returns the width of a color cell in pixels (4 or 8).
func (p *Picture) CellW() int {
	if p.Hires() {
		return 8
	}
	return 4
}

// Cell returns the cell of a pixel.
func (p *Picture) Cell(x, y int) int {
	if p.Hires() {
		return (y>>3)*Cols + x>>3
	}
	return CellOf(x, y)
}

// CellOrigin returns the top left pixel of a cell.
func (p *Picture) CellOrigin(cell int) (int, int) {
	return (cell % Cols) * p.CellW(), (cell / Cols) * 8
}

// NewHiresPicture returns an empty hires picture with layers.
func NewHiresPicture(mode Mode, paper, border uint8) *Picture {
	p := &Picture{BG: paper & 15, Border: border & 15, Mode: mode}
	if mode == ModeMulti {
		p.Mode = ModeHires
	}
	for i := range p.HP {
		p.HP[i] = p.BG
	}
	p.ToLayers()
	return p
}

// hRecomposite computes the visible colors of a hires picture.
func (p *Picture) hRecomposite() {
	for i := range p.HP {
		p.HP[i] = p.hPixel(i)
	}
}

// hPixel returns the visible color of a pixel from the layers.
func (p *Picture) hPixel(i int) uint8 {
	if !p.Layered {
		return p.HP[i]
	}
	for l := LayerCount - 1; l >= 0; l-- {
		if p.Hide[l] {
			continue
		}
		if c, ok := p.layerColor(p.L[l][i]); ok {
			return c
		}
	}
	return p.BG
}

// hCellColors counts the colors of a hires cell over all layers (as if all
// were visible); positions without any layer pixel count as paper. The pixel
// (ex, ey) of the active layer is taken as value nv.
func (p *Picture) hCellColors(cell, ex, ey int, nv uint8) (cnt [16]int, n int) {
	cx, cy := p.CellOrigin(cell)
	for y := cy; y < cy+8; y++ {
		for x := cx; x < cx+8; x++ {
			i := y*HW + x
			covered := false
			for l := 0; l < LayerCount; l++ {
				v := p.L[l][i]
				if l == p.Active && x == ex && y == ey {
					v = nv
				}
				if c, ok := p.layerColor(v); ok {
					covered = true
					if cnt[c] == 0 {
						n++
					}
					cnt[c]++
				}
			}
			if !covered {
				if cnt[p.BG] == 0 {
					n++
				}
				cnt[p.BG]++
			}
		}
	}
	return
}

// hSet writes a layer pixel and updates the visible pixel.
func (p *Picture) hSet(i int, v uint8) {
	p.L[p.Active][i] = v
	p.HP[i] = p.hPixel(i)
}

// plotHires is Plot for hires pictures.
func (p *Picture) plotHires(x, y int, c uint8, mode ConflictMode, pal *Palette) PlotResult {
	protect := mode&ConflictProtect != 0
	mode &^= ConflictProtect
	i := y*HW + x
	v := c + 1
	if c == p.BG {
		v = LayerBG
	}
	if p.L[p.Active][i] == v {
		return PlotOK
	}
	if p.Mode == ModeFree {
		p.hSet(i, v)
		return PlotOK
	}
	cell := p.Cell(x, y)
	cnt, n := p.hCellColors(cell, x, y, v)
	if n <= 2 {
		p.hSet(i, v)
		return PlotOK
	}
	switch mode {
	case ConflictReplace:
		// replace the rarest other color of the cell (not the paper)
		cx, cy := p.CellOrigin(cell)
		var other [16]int
		if protect {
			for l := 0; l < LayerCount; l++ {
				if l == p.Active {
					continue
				}
				for yy := cy; yy < cy+8; yy++ {
					for xx := cx; xx < cx+8; xx++ {
						if k, ok := p.layerColor(p.L[l][yy*HW+xx]); ok {
							other[k]++
						}
					}
				}
			}
		}
		rare := -1
		for k := 0; k < 16; k++ {
			if k == int(c) || k == int(p.BG) || cnt[k] == 0 || other[k] > 0 {
				continue
			}
			if rare < 0 || cnt[k] < cnt[rare] {
				rare = k
			}
		}
		if rare < 0 {
			return PlotBlocked
		}
		for l := 0; l < LayerCount; l++ {
			for yy := cy; yy < cy+8; yy++ {
				for xx := cx; xx < cx+8; xx++ {
					if p.L[l][yy*HW+xx] == uint8(rare)+1 {
						p.L[l][yy*HW+xx] = v
					}
				}
			}
		}
		p.L[p.Active][i] = v
		p.hRecompositeCell(cell)
		return PlotReplaced
	case ConflictNearest:
		if pal == nil {
			return PlotBlocked
		}
		tr, tg, tb := rgbOf(pal.Colors[c])
		best, bd := -1, 1<<62
		for k := 0; k < 16; k++ {
			if cnt[k] == 0 || k == int(c) {
				continue
			}
			r, g, b := rgbOf(pal.Colors[k])
			if d := ColorDist(tr, tg, tb, r, g, b); d < bd {
				bd, best = d, k
			}
		}
		if best < 0 {
			return PlotBlocked
		}
		nv := uint8(best) + 1
		if uint8(best) == p.BG {
			nv = LayerBG
		}
		if _, n := p.hCellColors(cell, x, y, nv); n > 2 {
			return PlotBlocked
		}
		p.hSet(i, nv)
		return PlotSubstituted
	}
	return PlotBlocked
}

// hRecompositeCell updates the visible pixels of one cell.
func (p *Picture) hRecompositeCell(cell int) {
	cx, cy := p.CellOrigin(cell)
	for y := cy; y < cy+8; y++ {
		for x := cx; x < cx+8; x++ {
			p.HP[y*HW+x] = p.hPixel(y*HW + x)
		}
	}
}

// hErase makes a pixel of the active layer transparent; in hires mode only
// when the paper color then still fits into the cell.
func (p *Picture) hErase(x, y int) {
	i := y*HW + x
	if p.L[p.Active][i] == LayerClear {
		return
	}
	if p.Mode == ModeHires {
		if _, n := p.hCellColors(p.Cell(x, y), x, y, LayerClear); n > 2 {
			return
		}
	}
	p.hSet(i, LayerClear)
}

// hReplaceCellColor changes a color in all layers of a cell (both: exchange).
func (p *Picture) hReplaceCellColor(cell int, from, to uint8, both bool) int {
	cx, cy := p.CellOrigin(cell)
	vf, vt := from+1, to+1
	if from == p.BG {
		vf = LayerBG
	}
	if to == p.BG {
		vt = LayerBG
	}
	changed := 0
	for l := 0; l < LayerCount; l++ {
		for y := cy; y < cy+8; y++ {
			for x := cx; x < cx+8; x++ {
				i := y*HW + x
				switch p.L[l][i] {
				case vf:
					p.L[l][i] = vt
					changed++
				case vt:
					if both {
						p.L[l][i] = vf
						changed++
					}
				}
			}
		}
	}
	if changed > 0 {
		p.hRecompositeCell(cell)
	}
	return changed
}

// HiresClashes counts the cells with more than two visible colors (only in
// "Hires free" pictures).
func (p *Picture) HiresClashes() int {
	n := 0
	for cell := 0; cell < Cells; cell++ {
		if len(p.hVisibleCellColors(cell)) > 2 {
			n++
		}
	}
	return n
}

// CellClash reports whether a cell has more colors than the mode allows
// (only possible in "Hires free").
func (p *Picture) CellClash(cell int) bool {
	return p.Mode == ModeFree && len(p.hVisibleCellColors(cell)) > 2
}

// hVisibleCellColors returns the visible colors of a cell, most frequent first.
func (p *Picture) hVisibleCellColors(cell int) []int {
	var cnt [16]int
	cx, cy := p.CellOrigin(cell)
	for y := cy; y < cy+8; y++ {
		for x := cx; x < cx+8; x++ {
			cnt[p.HP[y*HW+x]]++
		}
	}
	var cols []int
	for k := 0; k < 16; k++ {
		if cnt[k] > 0 {
			cols = append(cols, k)
		}
	}
	sort.SliceStable(cols, func(i, j int) bool { return cnt[cols[i]] > cnt[cols[j]] })
	return cols
}

// HiresData returns the bitmap (8000 bytes) and the screen RAM (1000 bytes)
// of the visible hires picture. Cells with more than two colors ("Hires
// free") keep their two most frequent colors; the other pixels get the
// nearer of the two.
func (p *Picture) HiresData(pal *Palette) (bitmap, screen []byte) {
	bitmap = make([]byte, 8000)
	screen = make([]byte, 1000)
	for cell := 0; cell < Cells; cell++ {
		cols := p.hVisibleCellColors(cell)
		var fg, bg int
		switch len(cols) {
		case 1:
			fg, bg = int(p.BG), cols[0]
			if fg == bg {
				fg = 0
			}
		default:
			// the paper color (if used) stays in the low nibble
			fg, bg = cols[0], cols[1]
			if fg == int(p.BG) {
				fg, bg = bg, fg
			}
		}
		screen[cell] = byte(fg<<4 | bg)
		cx, cy := p.CellOrigin(cell)
		for y := 0; y < 8; y++ {
			var b byte
			for x := 0; x < 8; x++ {
				c := int(p.HP[(cy+y)*HW+cx+x])
				on := c == fg
				if c != fg && c != bg && pal != nil {
					on = colorDistPal(pal, c, fg) < colorDistPal(pal, c, bg)
				}
				if on {
					b |= 0x80 >> x
				}
			}
			bitmap[cell*8+y] = b
		}
	}
	return
}

func colorDistPal(pal *Palette, a, b int) int {
	r1, g1, b1 := rgbOf(pal.Colors[a&15])
	r2, g2, b2 := rgbOf(pal.Colors[b&15])
	return ColorDist(r1, g1, b1, r2, g2, b2)
}

// SetHiresData fills a hires picture (without layers) from a bitmap and the
// screen RAM. The paper color becomes the most frequent color.
func (p *Picture) SetHiresData(bitmap, screen []byte) {
	if p.Mode == ModeMulti {
		p.Mode = ModeHires
	}
	var hist [16]int
	for cell := 0; cell < Cells; cell++ {
		fg, bg := screen[cell]>>4, screen[cell]&15
		cx, cy := p.CellOrigin(cell)
		for y := 0; y < 8; y++ {
			b := bitmap[cell*8+y]
			for x := 0; x < 8; x++ {
				c := bg
				if b&(0x80>>x) != 0 {
					c = fg
				}
				p.HP[(cy+y)*HW+cx+x] = c
				hist[c]++
			}
		}
	}
	best := 0
	for c := range hist {
		if hist[c] > hist[best] {
			best = c
		}
	}
	p.BG = uint8(best)
}

// --- hires file formats ----------------------------------------------------------------

// Art Studio hires (.art, .aas, .hpi): load address $2000, bitmap (8000),
// screen RAM (1000), border color; 9009 bytes with padding.
const artSize = 9009

// LoadART reads an Art Studio hires picture.
func LoadART(data []byte) (*Picture, error) {
	if len(data) < 2+8000+1000 {
		return nil, fmt.Errorf("not an Art Studio hires file: %d bytes (expected %d)", len(data), artSize)
	}
	d := data[2:]
	p := &Picture{Mode: ModeHires}
	p.SetHiresData(d[:8000], d[8000:9000])
	if len(d) > 9000 {
		p.Border = d[9000] & 15
	}
	return p, nil
}

// SaveART writes an Art Studio hires picture (load address $2000).
func (p *Picture) SaveART(pal *Palette) []byte {
	bm, scr := p.HiresData(pal)
	out := []byte{0x00, 0x20}
	out = append(out, bm...)
	out = append(out, scr...)
	out = append(out, p.Border&15)
	for len(out) < artSize {
		out = append(out, 0)
	}
	return out
}

// Doodle (.dd): load address $5C00, screen RAM (1000) at $5C00, bitmap
// (8000) at $6000; 9218 bytes.
const ddSize = 9218

// LoadDD reads a Doodle picture.
func LoadDD(data []byte) (*Picture, error) {
	if len(data) < 2+1024+8000 {
		return nil, fmt.Errorf("not a Doodle file: %d bytes (expected %d)", len(data), ddSize)
	}
	if data[0] != 0x00 || data[1] != 0x5C {
		return nil, errors.New("not a Doodle file (load address is not $5C00)")
	}
	d := data[2:]
	p := &Picture{Mode: ModeHires}
	p.SetHiresData(d[1024:1024+8000], d[:1000])
	return p, nil
}

// SaveDD writes a Doodle picture.
func (p *Picture) SaveDD(pal *Palette) []byte {
	bm, scr := p.HiresData(pal)
	out := make([]byte, ddSize)
	out[0], out[1] = 0x00, 0x5C
	copy(out[2:], scr)
	copy(out[2+1024:], bm)
	return out
}

// HiresPRG returns a runnable C64 program that shows the hires picture
// (space bar = reset).
func (p *Picture) HiresPRG(pal *Palette) []byte {
	const (
		bitmap = 0x2000
		scrSrc = bitmap + 8000 // $3F40
	)
	a := &asm{org: 0x0801, labels: map[string]int{}}
	a.emit(0x0B, 0x08, 0xEA, 0x07, 0x9E, '2', '0', '6', '1', 0x00, 0x00, 0x00) // 2026 SYS2061
	a.emit(0x78)                                                               // SEI
	a.imm(0xA9, p.Border&15)                                                   // LDA #border
	a.abs(0x8D, 0xD020)                                                        // STA $D020
	a.imm(0xA2, 0)                                                             // LDX #0
	a.label("copy")
	for pg := 0; pg < 4; pg++ {
		a.abs(0xBD, scrSrc+pg*256) // LDA scr,X
		a.abs(0x9D, 0x0400+pg*256) // STA $0400,X
	}
	a.emit(0xE8) // INX
	a.bne("copy")
	a.abs(0xAD, 0xDD00) // LDA $DD00
	a.imm(0x29, 0xFC)   // AND #%11111100
	a.imm(0x09, 0x03)   // ORA #%00000011 → VIC bank 0
	a.abs(0x8D, 0xDD00) // STA $DD00
	a.imm(0xA9, 0x3B)   // LDA #%00111011: bitmap mode, screen on, 25 rows
	a.abs(0x8D, 0xD011) // STA $D011
	a.imm(0xA9, 0x08)   // LDA #%00001000: hires (no multicolor), 40 columns
	a.abs(0x8D, 0xD016) // STA $D016
	a.imm(0xA9, 0x18)   // LDA #$18: screen $0400, bitmap $2000
	a.abs(0x8D, 0xD018) // STA $D018
	a.label("wait")
	a.imm(0xA9, 0x7F)   // LDA #$7F: keyboard row 7
	a.abs(0x8D, 0xDC00) // STA $DC00
	a.abs(0xAD, 0xDC01) // LDA $DC01
	a.imm(0x29, 0x10)   // AND #$10: space bar
	a.bne("wait")
	a.abs(0x4C, 0xFCE2) // JMP $FCE2 (reset)
	if err := a.resolve(); err != nil {
		panic(err)
	}
	mem := make([]byte, bitmap-0x0801)
	copy(mem, a.b)
	bm, scr := p.HiresData(pal)
	out := []byte{0x01, 0x08}
	out = append(out, mem...)
	out = append(out, bm...)
	out = append(out, scr...)
	return out
}

// --- conversion ------------------------------------------------------------------------

// ConvertHiresRGB converts a 320 × 200 RGB picture into a hires picture
// (mode ModeHires: two colors per 8 × 8 cell, ModeFree: all colors). It
// works like ConvertRGB: with dithering the picture is first dithered freely,
// the two colors of every cell are chosen from that, then every pixel is
// dithered again with the colors of its cell.
func ConvertHiresRGB(px [][3]int, pal *Palette, o ConvertOptions, mode Mode) *Picture {
	var pr [16][3]int
	for i, c := range pal.Colors {
		pr[i] = [3]int{int(c >> 16 & 255), int(c >> 8 & 255), int(c & 255)}
	}
	if mode == ModeMulti {
		mode = ModeHires
	}
	p := &Picture{Mode: mode}
	setResult := func(idx []uint8) {
		var hist [16]int
		for i, k := range idx {
			p.HP[i] = k
			hist[k]++
		}
		if o.BG >= 0 && o.BG < 16 {
			p.BG = uint8(o.BG)
			return
		}
		best := 0
		for k := range hist {
			if hist[k] > hist[best] {
				best = k
			}
		}
		p.BG = uint8(best)
	}
	if mode == ModeFree {
		setResult(ditherPassW(px, HW, &pr, o, func(x, y int) []int { return all16 }))
		return p
	}
	ref := cellReference(px, HW, &pr, o)
	metric := colorMetric(o.Match)
	dist := make([][16]int, HW*H)
	near := make([]uint8, HW*H)
	for i, c := range ref {
		best, bd := 0, 1<<62
		for k := 0; k < 16; k++ {
			d := metric(c[0], c[1], c[2], pr[k][0], pr[k][1], pr[k][2])
			dist[i][k] = d
			if d < bd {
				bd, best = d, k
			}
		}
		near[i] = uint8(best)
	}
	cellErr := func(cell, a, b int) int {
		cx, cy := (cell%Cols)*8, (cell/Cols)*8
		e := 0
		for y := cy; y < cy+8; y++ {
			for x := cx; x < cx+8; x++ {
				d := &dist[y*HW+x]
				e += min(d[a], d[b])
			}
		}
		return e
	}
	pairs := make([][2]int, Cells)
	switch o.Cells {
	case CellsFixed:
		bestE, bp := 1<<62, [2]int{0, 1}
		for a := 0; a < 16; a++ {
			for b := a + 1; b < 16; b++ {
				e := 0
				for i := range dist {
					e += min(dist[i][a], dist[i][b])
					if e >= bestE {
						break
					}
				}
				if e < bestE {
					bestE, bp = e, [2]int{a, b}
				}
			}
		}
		for i := range pairs {
			pairs[i] = bp
		}
	default:
		for cell := 0; cell < Cells; cell++ {
			cx, cy := (cell%Cols)*8, (cell/Cols)*8
			var cnt [16]int
			var cand [16]bool
			for y := cy; y < cy+8; y++ {
				for x := cx; x < cx+8; x++ {
					i := y*HW + x
					cnt[near[i]]++
					d := &dist[i]
					b1, b2 := -1, -1
					for k := 0; k < 16; k++ {
						if b1 < 0 || d[k] < d[b1] {
							b2, b1 = b1, k
						} else if b2 < 0 || d[k] < d[b2] {
							b2 = k
						}
					}
					cand[b1] = true
					if b2 >= 0 {
						cand[b2] = true
					}
				}
			}
			if o.Cells == CellsFrequent {
				a, b := -1, -1
				for k := 0; k < 16; k++ {
					if a < 0 || cnt[k] > cnt[a] {
						b, a = a, k
					} else if b < 0 || cnt[k] > cnt[b] {
						b = k
					}
				}
				pairs[cell] = [2]int{a, b}
				continue
			}
			var cl []int
			for k := 0; k < 16; k++ {
				if cand[k] {
					cl = append(cl, k)
				}
			}
			if len(cl) == 1 {
				cl = append(cl, (cl[0]+1)%16)
			}
			bestE := 1 << 62
			for i := 0; i < len(cl); i++ {
				for j := i + 1; j < len(cl); j++ {
					if e := cellErr(cell, cl[i], cl[j]); e < bestE {
						bestE, pairs[cell] = e, [2]int{cl[i], cl[j]}
					}
				}
			}
		}
	}
	final := ditherPassW(px, HW, &pr, o, func(x, y int) []int {
		pp := pairs[(y>>3)*Cols+x>>3]
		return pp[:]
	})
	setResult(final)
	return p
}
