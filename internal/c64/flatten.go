package c64

import "sort"

// Flattened returns the visible picture as a plain C64 picture without
// layers, so that C64 files can be written when free layers break the color
// rules. Every cell keeps its most frequent colors (multicolor: background
// plus 3, hires: 2); other pixels take the nearest of these colors. The
// side border sprites stay.
func (p *Picture) Flattened(pal *Palette) *Picture {
	if p.Hires() {
		q := NewHiresPicture(p.Mode, p.BG, p.Border)
		for i := range q.HP {
			q.HP[i] = p.ColorAt(i%HW, i/HW)
		}
		for cell := 0; cell < Cells; cell++ {
			cx, cy := q.CellOrigin(cell)
			var cnt [16]int
			for y := cy; y < cy+8; y++ {
				for x := cx; x < cx+8; x++ {
					cnt[q.HP[y*HW+x]]++
				}
			}
			keep := topColors(cnt, 2, -1)
			for y := cy; y < cy+8; y++ {
				for x := cx; x < cx+8; x++ {
					i := y*HW + x
					q.HP[i] = nearestColorOf(pal, q.HP[i], keep)
				}
			}
		}
		q.SideOn, q.Side = p.SideOn, p.Side
		return q
	}
	q := NewPicture(p.BG, p.Border)
	for cell := 0; cell < Cells; cell++ {
		cx, cy := (cell%Cols)*4, (cell/Cols)*8
		var cnt [16]int
		for y := cy; y < cy+8; y++ {
			for x := cx; x < cx+4; x++ {
				cnt[p.ColorAt(x, y)]++
			}
		}
		keep := topColors(cnt, 3, int(p.BG))
		for s, c := range keep {
			q.SetSlotColor(cell, s+1, c)
		}
		for s := len(keep); s < 3; s++ {
			q.SetSlotColor(cell, s+1, 0)
		}
		all := append([]uint8{p.BG}, keep...)
		for y := cy; y < cy+8; y++ {
			for x := cx; x < cx+4; x++ {
				c := nearestColorOf(pal, p.ColorAt(x, y), all)
				bits := uint8(0)
				for s, k := range keep {
					if k == c && c != p.BG {
						bits = uint8(s + 1)
					}
				}
				q.Bits[y*W+x] = bits
			}
		}
	}
	q.SideOn, q.Side = p.SideOn, p.Side
	return q
}

// topColors returns up to n colors by frequency (skip = a color left out).
func topColors(cnt [16]int, n, skip int) []uint8 {
	var cols []uint8
	for c := 0; c < 16; c++ {
		if cnt[c] > 0 && c != skip {
			cols = append(cols, uint8(c))
		}
	}
	sort.SliceStable(cols, func(i, j int) bool { return cnt[cols[i]] > cnt[cols[j]] })
	if len(cols) > n {
		cols = cols[:n]
	}
	return cols
}

func nearestColorOf(pal *Palette, c uint8, set []uint8) uint8 {
	best, bd := c, -1
	for _, k := range set {
		if k == c {
			return c
		}
		if pal == nil {
			continue
		}
		if d := colorDistPal(pal, int(c), int(k)); bd < 0 || d < bd {
			best, bd = k, d
		}
	}
	if bd < 0 && len(set) > 0 {
		return set[0]
	}
	return best
}
