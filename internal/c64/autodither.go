package c64

import (
	"math"
)

// DitherStyle selects one of the dithering styles that were reconstructed
// from hand-pixelled C64 pictures (see ditherdata.go).
type DitherStyle int

const (
	StyleArtist    DitherStyle = iota // checker on alternate rows (most common)
	StyleScanlines                    // alternate scanlines
	StyleHatching                     // horizontal dashes / strokes
	StyleChecker                      // classic checkerboard
	StyleColumns                      // vertical lines and stripes
	StyleStipple                      // scattered single pixels
	StyleOrganic                      // irregular, error diffusion along color ramps
	StyleCount
)

// StyleNames are the display names of the dither styles.
var StyleNames = [StyleCount]string{
	"Artist", "Scanlines", "Hatching", "Checker", "Columns", "Stipple", "Organic",
}

// StyleInfo describes each style in one sentence.
var StyleInfo = [StyleCount]string{
	"The most common technique in all analyzed pictures: a checkerboard on every second row first, then full rows, then the rows in between.",
	"Alternate scanlines with spread dots – fine, calm gradients like in portraits and shiny objects.",
	"Horizontal dashes that grow into strokes – a drawn, sketchy look.",
	"The classic checkerboard mix – soft and even, good for skin and clouds.",
	"Vertical lines that widen into stripes – graphic, poster-like gradients.",
	"Scattered single pixels (blue noise) – a grainy, natural look for skies and textures.",
	"Irregular texture from error diffusion along the artists' color ramps – lively and hand-made.",
}

// CellColorMode decides which colors a cell may use after dithering.
type CellColorMode int

const (
	CellKeep     CellColorMode = iota // only the colors the cell already has
	CellAdd                           // keep them and add in-between colors in free slots
	CellRechoose                      // choose the best three colors anew (may change details)
)

// CellModeNames are short display names of the cell color modes.
var CellModeNames = [3]string{"Keep", "Add", "Re-choose"}

// AutoDitherOptions controls AutoDither.
type AutoDitherOptions struct {
	Style    DitherStyle
	Softness int // 1..16: radius of the blend zone in rows
	Strength int // 0..100: how much the colors blend
	Edges    int // 0..100: keep hard edges and outlines
	Ramps    int // 0..100: follow the color pairs artists mix
	Colors   CellColorMode
}

// DefaultAutoDither returns sensible starting values.
func DefaultAutoDither() AutoDitherOptions {
	return AutoDitherOptions{Style: StyleArtist, Softness: 6, Strength: 100, Edges: 50, Ramps: 60, Colors: CellAdd}
}

// pairAffinity[a][b] is 1 for the pair artists mix most and 0 for pairs that
// are never mixed. Built from the measured tables in ditherdata.go.
var pairAffinity = func() (aff [16][16]float64) {
	mx := 0.0
	var v [16][16]float64
	for a := 0; a < 16; a++ {
		for b := 0; b < 16; b++ {
			v[a][b] = math.Log1p(4*float64(pairCountDither[a][b]) + float64(pairCountAll[a][b]))
			if v[a][b] > mx {
				mx = v[a][b]
			}
		}
	}
	for a := 0; a < 16; a++ {
		for b := 0; b < 16; b++ {
			aff[a][b] = v[a][b] / mx
		}
		aff[a][a] = 1
	}
	return
}()

type ditherer struct {
	pr     [16][3]int
	style  DitherStyle
	mat    []int
	n      int // matrix side
	levels float64
	rampK  float64
}

func newDitherer(pal *Palette, o AutoDitherOptions) *ditherer {
	d := &ditherer{style: o.Style}
	for i, c := range pal.Colors {
		d.pr[i] = [3]int{int(c >> 16 & 255), int(c >> 8 & 255), int(c & 255)}
	}
	d.mat = styleMatrix[o.Style]
	if d.mat == nil {
		d.mat = styleMatrix[StyleArtist]
	}
	d.n = int(math.Sqrt(float64(len(d.mat))))
	d.levels = float64(len(d.mat))
	d.rampK = float64(clamp100(o.Ramps)) / 100 * 6
	return d
}

// threshold returns 0..1 for the ordered styles.
func (d *ditherer) threshold(x, y int) float64 {
	return (float64(d.mat[(y%d.n)*d.n+x%d.n]) + 0.5) / d.levels
}

// bestPair finds the two allowed colors whose mix comes closest to the target,
// preferring pairs of similar brightness that artists actually mix.
func (d *ditherer) bestPair(t [3]float64, allowed []int) (int, int, float64) {
	r, g, b := clamp(int(t[0]+0.5)), clamp(int(t[1]+0.5)), clamp(int(t[2]+0.5))
	best, bestB, bestR, bestE := allowed[0], allowed[0], 0.0, math.Inf(1)
	for i, ka := range allowed {
		A := d.pr[ka]
		eA := float64(ColorDist(r, g, b, A[0], A[1], A[2]))
		if eA < bestE {
			best, bestB, bestR, bestE = ka, ka, 0, eA
		}
		for _, kb := range allowed[i+1:] {
			B := d.pr[kb]
			dr, dg, db := float64(B[0]-A[0]), float64(B[1]-A[1]), float64(B[2]-A[2])
			ll := dr*dr + dg*dg + db*db
			if ll == 0 {
				continue
			}
			ratio := ((float64(r-A[0]))*dr + (float64(g-A[1]))*dg + (float64(b-A[2]))*db) / ll
			if ratio <= 0 || ratio >= 1 {
				continue
			}
			ratio = math.Round(ratio*d.levels) / d.levels
			mr, mg, mb := float64(A[0])+ratio*dr, float64(A[1])+ratio*dg, float64(A[2])+ratio*db
			// mixing far-apart colors looks noisy, and pairs artists never
			// mix get an extra penalty (Ramps)
			e := float64(ColorDist(r, g, b, int(mr+0.5), int(mg+0.5), int(mb+0.5)))
			k := 0.035 * (1 + d.rampK*(1-pairAffinity[ka][kb]))
			e += k * float64(ColorDist(A[0], A[1], A[2], B[0], B[1], B[2])) * ratio * (1 - ratio)
			if e < bestE {
				best, bestB, bestR, bestE = ka, kb, ratio, e
			}
		}
	}
	return best, bestB, bestR
}

// pass dithers the target picture with the allowed colors of every pixel.
// keep[i] >= 0 forces a pixel to that color (pixels outside the mask).
func (d *ditherer) pass(target [][3]float64, allowed func(x, y int) []int, keep []int) []uint8 {
	out := make([]uint8, W*H)
	if d.style != StyleOrganic {
		for y := 0; y < H; y++ {
			for x := 0; x < W; x++ {
				i := y*W + x
				if keep != nil && keep[i] >= 0 {
					out[i] = uint8(keep[i])
					continue
				}
				a, b, ratio := d.bestPair(target[i], allowed(x, y))
				if ratio > 0 && d.threshold(x, y) < ratio {
					a = b
				}
				out[i] = uint8(a)
			}
		}
		return out
	}
	// Organic: serpentine Sierra Lite error diffusion, but every pixel only
	// chooses between the two colors of its best (ramp-friendly) pair.
	work := make([][3]float64, len(target))
	copy(work, target)
	for y := 0; y < H; y++ {
		rev := y&1 == 1
		for i := 0; i < W; i++ {
			x := i
			if rev {
				x = W - 1 - i
			}
			idx := y*W + x
			if keep != nil && keep[idx] >= 0 {
				out[idx] = uint8(keep[idx])
				continue
			}
			a, b, ratio := d.bestPair(target[idx], allowed(x, y))
			c := work[idx]
			pick := a
			if ratio > 0 {
				A, B := d.pr[a], d.pr[b]
				da := ColorDist(clamp(int(c[0])), clamp(int(c[1])), clamp(int(c[2])), A[0], A[1], A[2])
				db := ColorDist(clamp(int(c[0])), clamp(int(c[1])), clamp(int(c[2])), B[0], B[1], B[2])
				if db < da {
					pick = b
				}
			}
			out[idx] = uint8(pick)
			P := d.pr[pick]
			er := [3]float64{
				math.Max(-80, math.Min(80, c[0]-float64(P[0]))),
				math.Max(-80, math.Min(80, c[1]-float64(P[1]))),
				math.Max(-80, math.Min(80, c[2]-float64(P[2]))),
			}
			for _, tp := range [][3]float64{{1, 0, 0.5}, {-1, 1, 0.25}, {0, 1, 0.25}} {
				dx := int(tp[0])
				if rev {
					dx = -dx
				}
				xx, yy := x+dx, y+int(tp[1])
				if xx < 0 || xx >= W || yy >= H {
					continue
				}
				w := &work[yy*W+xx]
				for ch := 0; ch < 3; ch++ {
					w[ch] += er[ch] * tp[2]
				}
			}
		}
	}
	return out
}

// softTarget blurs the picture where colors meet (bilateral blur on the
// palette colors, 2:1 pixel aspect) and blends it with the original.
func softTarget(src *Picture, pr *[16][3]int, o AutoDitherOptions) [][3]float64 {
	R := o.Softness
	if R < 1 {
		R = 1
	}
	if R > 16 {
		R = 16
	}
	Rx := (R + 1) / 2
	sigma := float64(R)/2 + 0.5
	sw := make([]float64, (2*R+1)*(2*Rx+1))
	for dy := -R; dy <= R; dy++ {
		for dx := -Rx; dx <= Rx; dx++ {
			sx := float64(dx * 2)
			sw[(dy+R)*(2*Rx+1)+dx+Rx] = math.Exp(-(sx*sx + float64(dy*dy)) / (2 * sigma * sigma))
		}
	}
	// range weights between palette colors (keep edges)
	var rw [16][16]float64
	var luma [16]float64
	for i := 0; i < 16; i++ {
		luma[i] = 0.299*float64(pr[i][0]) + 0.587*float64(pr[i][1]) + 0.114*float64(pr[i][2])
	}
	e := clamp100(o.Edges)
	for a := 0; a < 16; a++ {
		for b := 0; b < 16; b++ {
			rw[a][b] = 1
			if e > 0 {
				f := float64(100-e) / 100
				sr := 6 + 255*f*f
				dl := luma[a] - luma[b]
				rw[a][b] = math.Exp(-dl * dl / (2 * sr * sr))
			}
		}
	}
	idx := make([]uint8, W*H)
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			idx[y*W+x] = src.ColorAt(x, y)
		}
	}
	s := float64(clamp100(o.Strength)) / 100
	out := make([][3]float64, W*H)
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			c := idx[y*W+x]
			var acc [3]float64
			wsum := 0.0
			uniform := true
			for dy := -R; dy <= R; dy++ {
				yy := y + dy
				if yy < 0 || yy >= H {
					continue
				}
				row := (dy + R) * (2*Rx + 1)
				for dx := -Rx; dx <= Rx; dx++ {
					xx := x + dx
					if xx < 0 || xx >= W {
						continue
					}
					n := idx[yy*W+xx]
					if n != c {
						uniform = false
					}
					w := sw[row+dx+Rx] * rw[c][n]
					acc[0] += w * float64(pr[n][0])
					acc[1] += w * float64(pr[n][1])
					acc[2] += w * float64(pr[n][2])
					wsum += w
				}
			}
			o := [3]float64{float64(pr[c][0]), float64(pr[c][1]), float64(pr[c][2])}
			if !uniform && wsum > 0 {
				for ch := 0; ch < 3; ch++ {
					o[ch] += s * (acc[ch]/wsum - o[ch])
				}
			}
			out[y*W+x] = o
		}
	}
	return out
}

// AutoDither returns a dithered version of the picture: where differently
// colored areas meet, the colors are blended and re-drawn with the chosen
// dither style. Flat areas stay unchanged. Only pixels with mask[i] == true
// may change (nil = whole picture). The result always follows the multicolor
// rules and keeps the background color.
func AutoDither(src *Picture, pal *Palette, o AutoDitherOptions, mask []bool) *Picture {
	d := newDitherer(pal, o)
	target := softTarget(src, &d.pr, o)
	bg := int(src.BG)
	keep := make([]int, W*H)
	for i := range keep {
		keep[i] = -1
		if mask != nil && !mask[i] {
			keep[i] = int(src.ColorAt(i%W, i/W))
		}
	}
	// colors per cell
	cellCols := make([][]int, Cells) // non-background colors, max 3
	for cell := 0; cell < Cells; cell++ {
		cols, used := src.CellColors(cell)
		for s := 1; s <= 3; s++ {
			if used[s] && int(cols[s]) != bg && !contains(cellCols[cell], int(cols[s])) {
				cellCols[cell] = append(cellCols[cell], int(cols[s]))
			}
		}
	}
	if o.Colors != CellKeep {
		free := d.pass(target, func(x, y int) []int { return all16 }, keep)
		for cell := 0; cell < Cells; cell++ {
			var fixed []int
			if o.Colors == CellAdd {
				fixed = cellCols[cell]
			}
			cellCols[cell] = chooseCellColors(cell, free, target, keep, fixed, bg, &d.pr)
		}
	}
	allowed := make([][]int, Cells)
	for cell := range allowed {
		allowed[cell] = append([]int{bg}, cellCols[cell]...)
	}
	final := d.pass(target, func(x, y int) []int { return allowed[CellOf(x, y)] }, keep)
	return buildPicture(src, final, cellCols)
}

// chooseCellColors picks up to three non-background colors for a cell: the
// fixed colors, all colors of pixels that must stay, plus the colors that
// represent the freely dithered result best.
func chooseCellColors(cell int, free []uint8, target [][3]float64, keep []int, fixed0 []int, bg int, pr *[16][3]int) []int {
	cx, cy := (cell%Cols)*4, (cell/Cols)*8
	fixed := append([]int(nil), fixed0...)
	var cnt [16]int
	for y := cy; y < cy+8; y++ {
		for x := cx; x < cx+4; x++ {
			i := y*W + x
			if keep[i] >= 0 {
				if keep[i] != bg && !contains(fixed, keep[i]) {
					fixed = append(fixed, keep[i])
				}
				continue
			}
			cnt[free[i]]++
		}
	}
	if len(fixed) >= 3 {
		return fixed[:3]
	}
	var cand []int
	for k := 0; k < 16; k++ {
		if cnt[k] > 0 && k != bg && !contains(fixed, k) {
			cand = append(cand, k)
		}
	}
	need := 3 - len(fixed)
	if len(cand) <= need {
		return append(fixed, cand...)
	}
	// try all combinations of the candidates, keep the one with the smallest error
	bestE := math.Inf(1)
	var best []int
	var rec func(start int, cur []int)
	rec = func(start int, cur []int) {
		if len(cur) == need {
			set := append(append([]int{bg}, fixed...), cur...)
			e := 0.0
			for y := cy; y < cy+8 && e < bestE; y++ {
				for x := cx; x < cx+4; x++ {
					i := y*W + x
					if keep[i] >= 0 {
						continue
					}
					f := pr[free[i]]
					t := target[i]
					m := math.Inf(1)
					for _, k := range set {
						c := pr[k]
						// distance to the free result and (weaker) to the soft target
						v := float64(ColorDist(f[0], f[1], f[2], c[0], c[1], c[2])) +
							0.5*float64(ColorDist(clamp(int(t[0])), clamp(int(t[1])), clamp(int(t[2])), c[0], c[1], c[2]))
						if v < m {
							m = v
						}
					}
					e += m
				}
			}
			if e < bestE {
				bestE = e
				best = append(append([]int{}, fixed...), cur...)
			}
			return
		}
		for k := start; k < len(cand); k++ {
			rec(k+1, append(cur, cand[k]))
		}
	}
	rec(0, nil)
	return best
}

// buildPicture turns palette indices into a valid multicolor picture. Colors
// that stay in a cell keep their slot, so screen and color RAM change little.
func buildPicture(src *Picture, px []uint8, cellCols [][]int) *Picture {
	p := src.Clone()
	bg := int(src.BG)
	for cell := 0; cell < Cells; cell++ {
		var slotOf [16]int
		for i := range slotOf {
			slotOf[i] = -1
		}
		slotOf[bg] = 0
		var taken [4]bool
		cols, used := src.CellColors(cell)
		need := cellCols[cell]
		for s := 1; s <= 3; s++ {
			c := int(cols[s])
			if used[s] && c != bg && contains(need, c) && slotOf[c] < 0 {
				slotOf[c] = s
				taken[s] = true
			}
		}
		for _, c := range need {
			if slotOf[c] >= 0 {
				continue
			}
			for s := 1; s <= 3; s++ {
				if !taken[s] {
					slotOf[c] = s
					taken[s] = true
					p.SetSlotColor(cell, s, uint8(c))
					break
				}
			}
		}
		cx, cy := (cell%Cols)*4, (cell/Cols)*8
		for y := cy; y < cy+8; y++ {
			for x := cx; x < cx+4; x++ {
				i := y*W + x
				s := slotOf[px[i]]
				if s < 0 {
					s = 0 // cannot happen: every pixel uses an allowed color
				}
				p.Bits[i] = uint8(s)
			}
		}
	}
	p.CleanUnused()
	return p
}
