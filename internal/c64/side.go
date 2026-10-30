package c64

// --- Side border sprites (experimental, multicolor only) -------------------------------
//
// With the side border opened, sprites are visible left and right of the
// bitmap. The editor keeps one strip per side: 12 multicolor pixels (one
// sprite width, 24 screen pixels) × 200 lines, split into 10 sprites of
// 21 lines that are shown one below the other (a sprite multiplexer in the
// C64 program).
//
// Every 12 × 21 block has its own sprite color; multicolor 1 ($D025) and
// multicolor 2 ($D026) are shared by all blocks. Transparent pixels show the
// background color ($D021), as the open border does on the C64.

const (
	SideW     = 12 // multicolor pixels per strip
	SideRows  = 10 // sprites per strip
	SideCount = 2  // left, right
)

// SideSprites holds the two strips. Pixels: 0 = transparent, 1..16 = color+1.
type SideSprites struct {
	MC1, MC2 uint8
	Col      [SideCount][SideRows]uint8
	Pix      [SideCount][SideW * H]uint8
}

// SideActive reports whether the side border strips are in use.
func (p *Picture) SideActive() bool { return p.SideOn && p.Mode == ModeMulti }

// SideAt maps a picture x coordinate to a strip: left strip x = -12..-1,
// right strip x = 160..171.
func SideAt(x int) (side, sx int, ok bool) {
	switch {
	case x >= -SideW && x < 0:
		return 0, x + SideW, true
	case x >= W && x < W+SideW:
		return 1, x - W, true
	}
	return 0, 0, false
}

// SideHidden reports pixels the C64 cannot show: the first line of the left
// strip lies left of the first picture line, where the border is still
// closed.
func SideHidden(side, y int) bool { return side == 0 && y == 0 }

// SideColorAt returns the visible color of a strip pixel.
func (p *Picture) SideColorAt(side, sx, y int) uint8 {
	if y < 0 || y >= H || sx < 0 || sx >= SideW {
		return p.BG
	}
	v := p.Side.Pix[side][y*SideW+sx]
	if v == 0 {
		return p.BG
	}
	return v - 1
}

// sideUsage counts the pixels of a block that need its sprite color (not
// MC1/MC2) and, over all blocks, the pixels that need MC1 and MC2. The pixel
// (side, ex, ey) is not counted (ex < 0: count all).
func (p *Picture) sideUsage(side, row, ex, ey int) (own, mc1, mc2 int) {
	s := &p.Side
	for sd := 0; sd < SideCount; sd++ {
		for i, v := range s.Pix[sd] {
			if v == 0 {
				continue
			}
			x, y := i%SideW, i/SideW
			if sd == side && x == ex && y == ey {
				continue
			}
			c := v - 1
			switch {
			case c == s.MC1:
				mc1++
			case c == s.MC2:
				mc2++
			case sd == side && y/SpriteH == row:
				own++
			}
		}
	}
	return
}

// sideColorElsewhere reports whether another sprite uses c as its own color.
func (p *Picture) sideColorElsewhere(side, row int, c uint8) bool {
	s := &p.Side
	for sd := 0; sd < SideCount; sd++ {
		for i, v := range s.Pix[sd] {
			if v == c+1 && (sd != side || (i/SideW)/SpriteH != row) {
				return true
			}
		}
	}
	return false
}

// PlotSide sets a strip pixel by the sprite rules: the block may use MC1,
// MC2 and its own sprite color. A free sprite color or a free shared color
// is taken automatically; otherwise the conflict mode decides.
func (p *Picture) PlotSide(side, sx, y int, c uint8, mode ConflictMode, pal *Palette) PlotResult {
	if y < 0 || y >= H || sx < 0 || sx >= SideW || SideHidden(side, y) {
		return PlotOK
	}
	s := &p.Side
	i := y*SideW + sx
	c &= 15
	if c == p.BG {
		s.Pix[side][i] = 0
		return PlotOK
	}
	row := y / SpriteH
	own, mc1, mc2 := p.sideUsage(side, row, sx, y)
	switch {
	case c == s.MC1 || c == s.MC2 || c == s.Col[side][row]:
	case mc1 == 0 && p.sideColorElsewhere(side, row, c):
		// a color used in several sprites becomes a shared color
		s.MC1 = c
	case mc2 == 0 && p.sideColorElsewhere(side, row, c):
		s.MC2 = c
	case own == 0:
		s.Col[side][row] = c
	case mc1 == 0:
		s.MC1 = c
	case mc2 == 0:
		s.MC2 = c
	default:
		switch mode &^ ConflictProtect {
		case ConflictReplace:
			// the block's own color changes (MC1/MC2 belong to all blocks)
			old := s.Col[side][row]
			for yy := row * SpriteH; yy < min(H, (row+1)*SpriteH); yy++ {
				for xx := 0; xx < SideW; xx++ {
					j := yy*SideW + xx
					if v := s.Pix[side][j]; v != 0 && v-1 == old && old != s.MC1 && old != s.MC2 {
						s.Pix[side][j] = c + 1
					}
				}
			}
			s.Col[side][row] = c
			s.Pix[side][i] = c + 1
			return PlotReplaced
		case ConflictNearest:
			best, bd := s.Col[side][row], -1
			for _, k := range []uint8{s.MC1, s.MC2, s.Col[side][row], p.BG} {
				if d := colorDistPal(pal, int(c), int(k)); bd < 0 || d < bd {
					best, bd = k, d
				}
			}
			if best == p.BG {
				s.Pix[side][i] = 0
			} else {
				s.Pix[side][i] = best + 1
			}
			return PlotReplaced
		default:
			return PlotBlocked
		}
	}
	s.Pix[side][i] = c + 1
	return PlotOK
}

// EraseSide makes a strip pixel transparent.
func (p *Picture) EraseSide(side, sx, y int) {
	if y >= 0 && y < H && sx >= 0 && sx < SideW {
		p.Side.Pix[side][y*SideW+sx] = 0
	}
}

// ClearSide empties both strips.
func (p *Picture) ClearSide() {
	p.Side.Pix = [SideCount][SideW * H]uint8{}
}

// SidePixels counts the painted strip pixels.
func (p *Picture) SidePixels() int {
	n := 0
	for sd := range p.Side.Pix {
		for _, v := range p.Side.Pix[sd] {
			if v != 0 {
				n++
			}
		}
	}
	return n
}

// SideSprite returns the 63 bytes of one sprite (multicolor bit pairs:
// %00 transparent, %01 MC1, %10 sprite color, %11 MC2).
func (p *Picture) SideSprite(side, row int) [64]byte {
	var out [64]byte
	s := &p.Side
	for ly := 0; ly < SpriteH; ly++ {
		y := row*SpriteH + ly
		for sx := 0; sx < SideW; sx++ {
			var bits byte
			if y < H && !SideHidden(side, y) {
				if v := s.Pix[side][y*SideW+sx]; v != 0 {
					switch c := v - 1; {
					case c == s.MC1:
						bits = 1
					case c == s.MC2:
						bits = 3
					default:
						bits = 2
					}
				}
			}
			out[ly*3+sx/4] |= bits << (6 - 2*(sx%4))
		}
	}
	return out
}
