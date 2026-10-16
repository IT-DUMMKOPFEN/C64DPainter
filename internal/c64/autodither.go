package c64

import (
	"math"
	"sort"
)

// DitherStyle selects one of the dithering styles that were reconstructed
// from hand-pixelled C64 pictures (see ditherdata.go).
type DitherStyle int

const (
	StyleArtist    DitherStyle = iota // checker on alternate rows (most common)
	StyleScanlines                    // alternate scanlines
	StyleChecker                      // classic checkerboard
	StyleStripes                      // lines that widen into stripes, parallel to the border
	StyleColor                        // shade by dithering with a darker color of the cell
	StyleCount
)

// StyleNames are the display names of the dither styles.
var StyleNames = [StyleCount]string{
	"Artist", "Scanlines", "Checker", "Stripes", "Color",
}

// StyleInfo describes each style in one sentence.
var StyleInfo = [StyleCount]string{
	"The most common technique in all analyzed pictures: 25 % = checker dots on every second row, 50 % = every second row.",
	"Alternate scanlines with spread dots – fine, calm gradients like in portraits and shiny objects.",
	"The classic checkerboard: 25 % = spread dots, 50 % = checkerboard. Soft and even, good for skin and clouds.",
	"Lines that widen into stripes, always parallel to the border (vertical at vertical borders) – graphic, poster-like.",
	"Shades by darkening: each color is dithered with a darker color that is already in the 4×8 cell (or fits into a free slot). Auto dither darkens areas towards darker neighbors; the brush darkens what you paint over.",
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
//
// The method follows common pixel art practice: only two colors are mixed
// per transition, in a few clean stages (for example 25 % and 50 %), each
// stage a band of the same pattern; outlines, hard edges and small details
// are not dithered, and colors of very different brightness are joined by
// an in-between color from the artists' color ramps instead.
type AutoDitherOptions struct {
	Style  DitherStyle
	Width  int // 2..16: width of the transition on each side, in pixel rows
	Steps  int // 1..3: dither stages per side (50 % · 25/50 % · 12/25/50 %)
	Edges  int // 0..100: keep edges between colors of different brightness
	Detail int // 0..8: never dither areas thinner than this (rows) – outlines, details
	Ramps  int // 0..100: only mix color pairs that artists mix (0 = any pair)
	// Between joins colors that are too far apart with an in-between color
	// from the artists' color ramps (good for gradients, bad for silhouettes).
	Between bool
	Colors  CellColorMode
	// Fill (Color style only): darken every pixel, not only near borders.
	// Used by the dither brush; Steps then means 25 / 50 / 75 % darker color.
	Fill bool `json:"-"`
}

// DefaultAutoDither returns sensible starting values.
func DefaultAutoDither() AutoDitherOptions {
	return AutoDitherOptions{Style: StyleArtist, Width: 8, Steps: 2, Edges: 80, Detail: 3, Ramps: 50, Colors: CellAdd}
}

// Valid reports whether all values are in range.
func (o AutoDitherOptions) Valid() bool {
	return o.Style >= 0 && o.Style < StyleCount && o.Width >= 2 && o.Width <= 16 &&
		o.Steps >= 1 && o.Steps <= 3 && o.Edges >= 0 && o.Edges <= 100 &&
		o.Detail >= 0 && o.Detail <= 8 && o.Ramps >= 0 && o.Ramps <= 100 &&
		o.Colors >= CellKeep && o.Colors <= CellRechoose
}

// lumaLevel is the brightness level (0..8) of the 16 VIC-II colors. Colors on
// the same level have the same luminance on a real C64.
var lumaLevel = [16]int{0, 8, 2, 6, 3, 5, 1, 7, 3, 1, 5, 2, 4, 7, 4, 6}

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

// ditherAffinity[a][b] only counts real dither areas (not plain edges between
// two areas): 1 for black/blue, the pair mixed most, 0 for pairs that were
// never mixed. It decides which borders may be blended at all.
var ditherAffinity = func() (aff [16][16]float64) {
	mx := 0.0
	for a := 0; a < 16; a++ {
		for b := 0; b < 16; b++ {
			mx = math.Max(mx, float64(pairCountDither[a][b]))
		}
	}
	for a := 0; a < 16; a++ {
		for b := 0; b < 16; b++ {
			aff[a][b] = math.Log1p(float64(pairCountDither[a][b])) / math.Log1p(mx)
		}
		aff[a][a] = 1
	}
	return
}()

// stageLevels: share of the other color per stage, nearest to the border first.
var stageLevels = [4][]float64{nil, {0.5}, {0.5, 0.25}, {0.5, 0.25, 0.125}}

type offset struct {
	dx, dy int
	d      float64
}

// offsetsWithin lists all offsets up to radius r (rows; a multicolor pixel
// counts as two rows wide), sorted by distance.
func offsetsWithin(r int) []offset {
	var o []offset
	rx := (r + 1) / 2
	for dy := -r; dy <= r; dy++ {
		for dx := -rx; dx <= rx; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			d := math.Hypot(float64(2*dx), float64(dy))
			if d <= float64(r)+0.01 {
				o = append(o, offset{dx, dy, d})
			}
		}
	}
	sort.SliceStable(o, func(i, j int) bool { return o[i].d < o[j].d })
	return o
}

type ditherPlan struct {
	src        *Picture
	o          AutoDitherOptions
	idx        []uint8
	comp       []int32
	compMax    []float64 // largest distance to another color inside a component
	local      []float64 // local half thickness of the area around each pixel
	compArea   []int
	protected  []bool    // per pixel: part of an outline or a small detail
	near       []int32   // index of the nearest pixel with another color (-1 = none)
	nearD      []float64 // its distance
	maxStep    int
	minAff     float64
	mat        []int
	n          int
	levelCount float64
}

func newPlan(src *Picture, o AutoDitherOptions) *ditherPlan {
	if !o.Valid() {
		def := DefaultAutoDither()
		def.Style, def.Colors = o.Style, o.Colors
		if def.Style < 0 || def.Style >= StyleCount {
			def.Style = StyleArtist
		}
		if def.Colors < CellKeep || def.Colors > CellRechoose {
			def.Colors = CellAdd
		}
		o = def
	}
	p := &ditherPlan{src: src, o: o}
	p.idx = make([]uint8, W*H)
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			p.idx[y*W+x] = src.ColorAt(x, y)
		}
	}
	p.maxStep = 1 + (100-o.Edges)*7/100
	p.minAff = float64(o.Ramps) / 100 * 0.6
	p.mat = styleMatrix[o.Style]
	if p.mat == nil {
		p.mat = styleMatrix[StyleArtist]
	}
	p.n = int(math.Sqrt(float64(len(p.mat))))
	p.levelCount = float64(len(p.mat))
	p.components()
	p.nearest()
	return p
}

// components labels 4-connected areas of the same color.
func (p *ditherPlan) components() {
	p.comp = make([]int32, W*H)
	for i := range p.comp {
		p.comp[i] = -1
	}
	stack := make([]int, 0, 1024)
	for i := range p.idx {
		if p.comp[i] >= 0 {
			continue
		}
		id := int32(len(p.compArea))
		c := p.idx[i]
		p.comp[i] = id
		stack = append(stack[:0], i)
		area := 0
		for len(stack) > 0 {
			j := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			area++
			x, y := j%W, j/W
			for _, nb := range [4][2]int{{x - 1, y}, {x + 1, y}, {x, y - 1}, {x, y + 1}} {
				if nb[0] < 0 || nb[1] < 0 || nb[0] >= W || nb[1] >= H {
					continue
				}
				k := nb[1]*W + nb[0]
				if p.comp[k] < 0 && p.idx[k] == c {
					p.comp[k] = id
					stack = append(stack, k)
				}
			}
		}
		p.compArea = append(p.compArea, area)
	}
}

// nearest finds for every pixel the closest pixel of another color and the
// thickness of every area; thin or small areas are protected.
func (p *ditherPlan) nearest() {
	limit := p.o.Width + 2
	if lim2 := 2*p.o.Detail + 2; lim2 > limit {
		limit = lim2
	}
	offs := offsetsWithin(limit)
	p.near = make([]int32, W*H)
	p.nearD = make([]float64, W*H)
	p.compMax = make([]float64, len(p.compArea))
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			i := y*W + x
			c := p.idx[i]
			p.near[i] = -1
			d := float64(limit) + 1
			for _, o := range offs {
				xx, yy := x+o.dx, y+o.dy
				if xx < 0 || yy < 0 || xx >= W || yy >= H {
					continue
				}
				if p.idx[yy*W+xx] != c {
					p.near[i] = int32(yy*W + xx)
					d = o.d
					break
				}
			}
			p.nearD[i] = d
			if cm := &p.compMax[p.comp[i]]; d > *cm {
				*cm = d
			}
		}
	}
	// local half thickness: the largest distance to another color among the
	// pixels of the same area nearby (the "spine" of the area)
	r := p.o.Width + 1
	if 2*p.o.Detail+1 > r {
		r = 2*p.o.Detail + 1
	}
	loc := offsetsWithin(r)
	p.local = make([]float64, W*H)
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			i := y*W + x
			c := p.comp[i]
			m := p.nearD[i]
			for _, o := range loc {
				xx, yy := x+o.dx, y+o.dy
				if xx < 0 || yy < 0 || xx >= W || yy >= H {
					continue
				}
				j := yy*W + xx
				if p.comp[j] == c && p.nearD[j] > m {
					m = p.nearD[j]
				}
			}
			p.local[i] = m
		}
	}
	p.protected = make([]bool, W*H)
	det := float64(p.o.Detail)
	for i := range p.protected {
		c := p.comp[i]
		p.protected[i] = p.o.Detail > 0 && (p.local[i] < det || p.compArea[c] < 2*p.o.Detail*p.o.Detail)
	}
}

func lumaStep(a, b uint8) int {
	d := lumaLevel[a] - lumaLevel[b]
	if d < 0 {
		return -d
	}
	return d
}

// via returns the color to use between a and b: b itself when the two may
// be mixed directly, an in-between color from the color ramps, or -1.
func (p *ditherPlan) via(a, b uint8) int {
	if lumaStep(a, b) <= p.maxStep && ditherAffinity[a][b] >= p.minAff {
		return int(b)
	}
	if !p.o.Between {
		return -1
	}
	lo, hi := a, b
	if lo > hi {
		lo, hi = hi, lo // the same choice from both sides
	}
	la, lb := lumaLevel[lo], lumaLevel[hi]
	if la > lb {
		la, lb = lb, la
	}
	best, bs := -1, 0.0
	for c := uint8(0); c < 16; c++ {
		if c == lo || c == hi {
			continue
		}
		l := lumaLevel[c]
		if l <= la || l >= lb || lumaStep(lo, c) > p.maxStep || lumaStep(c, hi) > p.maxStep {
			continue
		}
		s := math.Min(ditherAffinity[lo][c], ditherAffinity[c][hi])
		if s >= math.Max(p.minAff, 0.2) && s > bs {
			best, bs = int(c), s
		}
	}
	return best
}

// threshold returns 0..1 from the style's matrix, turned by 90° if asked.
func (p *ditherPlan) threshold(x, y int, turn bool) float64 {
	if turn {
		x, y = y, x
	}
	return (float64(p.mat[(y%p.n)*p.n+x%p.n]) + 0.5) / p.levelCount
}

// desired computes the dithered color of every pixel, without cell limits.
func (p *ditherPlan) desired(mask []bool) []uint8 {
	if p.o.Style == StyleColor {
		return p.darken(mask)
	}
	out := append([]uint8(nil), p.idx...)
	type mix struct {
		lo, hi uint8
		fHi    float64
		turn   bool // use the matrix turned by 90°
	}
	mixes := make([]mix, W*H)
	has := make([]bool, W*H)
	levels := stageLevels[p.o.Steps]
	for i, c := range p.idx {
		if mask != nil && !mask[i] {
			continue
		}
		if p.protected[i] || p.near[i] < 0 {
			continue
		}
		q := p.near[i]
		if p.protected[q] {
			continue // next to an outline or a small detail: stay crisp
		}
		b := p.idx[q]
		v := p.via(c, b)
		if v < 0 {
			continue // hard edge
		}
		// width of the transition: never more than the area allows, so a
		// solid core stays
		reff := math.Min(float64(p.o.Width), p.local[i]-1)
		if reff < 1 {
			continue
		}
		d := p.nearD[i]
		stage := int((d - 1) * float64(len(levels)) / reff)
		if stage >= len(levels) {
			continue
		}
		f := levels[stage] // share of the other color
		other := uint8(v)
		if int(b) != v {
			f = math.Min(1, 2*f) // towards the in-between color
		}
		lo, hi := c, other
		fHi := f
		if lumaLevel[lo] > lumaLevel[hi] || (lumaLevel[lo] == lumaLevel[hi] && lo > hi) {
			lo, hi = hi, lo
			fHi = 1 - f
		}
		qx, qy := int(q)%W-i%W, int(q)/W-i/W
		// Stripes run parallel to the border: their matrix holds vertical
		// lines, turned for horizontal borders. The other styles stay in
		// rows, as C64 artists draw them because of the wide pixels.
		turn := p.o.Style == StyleStripes && 2*absInt(qx) <= absInt(qy)
		mixes[i] = mix{lo, hi, fHi, turn}
		has[i] = true
	}
	for i := range out {
		if !has[i] {
			continue
		}
		m := mixes[i]
		if p.threshold(i%W, i/W, m.turn) < m.fHi {
			out[i] = m.hi
		} else {
			out[i] = m.lo
		}
	}
	return out
}

// AutoDither returns a dithered version of the picture: where two large,
// differently colored areas meet, a stepped transition in the chosen style
// is drawn. Flat areas, outlines and small details stay unchanged. Only
// pixels with mask[i] == true may change (nil = whole picture). The result
// always follows the multicolor rules and keeps the background color.
func AutoDither(src *Picture, pal *Palette, o AutoDitherOptions, mask []bool) *Picture {
	p := newPlan(src, o)
	want := p.desired(mask)
	bg := src.BG
	final := make([]uint8, W*H)
	cellCols := make([][]int, Cells)
	for cell := 0; cell < Cells; cell++ {
		cx, cy := (cell%Cols)*4, (cell/Cols)*8
		var existing []int
		cols, used := src.CellColors(cell)
		for s := 1; s <= 3; s++ {
			if used[s] && cols[s] != bg && !contains(existing, int(cols[s])) {
				existing = append(existing, int(cols[s]))
			}
		}
		allowed := existing
		switch p.o.Colors {
		case CellAdd:
			allowed = append([]int(nil), existing...)
			for _, c := range rankColors(want, cx, cy, bg, existing) {
				if len(allowed) >= 3 {
					break
				}
				allowed = append(allowed, c)
			}
		case CellRechoose:
			// colors of pixels that must stay first, then the most used ones
			var fixed []int
			for y := cy; y < cy+8; y++ {
				for x := cx; x < cx+4; x++ {
					i := y*W + x
					stays := (mask != nil && !mask[i]) || p.protected[i]
					if stays && p.idx[i] != bg && !contains(fixed, int(p.idx[i])) {
						fixed = append(fixed, int(p.idx[i]))
					}
				}
			}
			allowed = fixed
			for _, c := range rankColors(want, cx, cy, bg, fixed) {
				if len(allowed) >= 3 {
					break
				}
				allowed = append(allowed, c)
			}
		}
		for y := cy; y < cy+8; y++ {
			for x := cx; x < cx+4; x++ {
				i := y*W + x
				f := p.idx[i]
				switch {
				case want[i] == bg || contains(allowed, int(want[i])):
					f = want[i]
				case f == bg || contains(allowed, int(f)):
					// keep the original color
				default:
					f = nearestOf(pal, f, append(allowed, int(bg)))
				}
				final[i] = f
				if f != bg && !contains(cellCols[cell], int(f)) {
					cellCols[cell] = append(cellCols[cell], int(f))
				}
			}
		}
	}
	return buildPicture(src, final, cellCols)
}

// rankColors lists the non-background colors wanted in a cell that are not
// in skip, most used first.
func rankColors(want []uint8, cx, cy int, bg uint8, skip []int) []int {
	var cnt [16]int
	for y := cy; y < cy+8; y++ {
		for x := cx; x < cx+4; x++ {
			cnt[want[y*W+x]]++
		}
	}
	var r []int
	for c := 0; c < 16; c++ {
		if cnt[c] > 0 && uint8(c) != bg && !contains(skip, c) {
			r = append(r, c)
		}
	}
	sort.SliceStable(r, func(i, j int) bool { return cnt[r[i]] > cnt[r[j]] })
	return r
}

func nearestOf(pal *Palette, c uint8, set []int) uint8 {
	a := pal.Colors[c]
	best, bd := uint8(set[0]), 1<<62
	for _, k := range set {
		b := pal.Colors[k]
		d := ColorDist(int(a>>16&255), int(a>>8&255), int(a&255), int(b>>16&255), int(b>>8&255), int(b&255))
		if d < bd {
			best, bd = uint8(k), d
		}
	}
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

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// fillLevels: share of the darker color for the brush (Steps 1..3).
var fillLevels = [4]float64{0, 0.25, 0.5, 0.75}

// darkerShade picks the color used to darken c in a cell: a darker color of
// similar brightness that artists mix with c; colors that are already in the
// cell (or the background) come first, otherwise one for a free slot.
func (p *ditherPlan) darkerShade(c uint8, cell int, cellCols []uint8) int {
	best, bs := -1, -1.0
	for k := uint8(0); k < 16; k++ {
		if lumaLevel[k] >= lumaLevel[c] || lumaStep(c, k) > p.maxStep {
			continue
		}
		in := k == p.src.BG
		for _, cc := range cellCols {
			if cc == k {
				in = true
			}
		}
		if p.o.Colors == CellKeep && !in {
			continue
		}
		aff := ditherAffinity[c][k]
		if aff < math.Max(p.minAff, 0.15) && !(in && aff > 0) {
			continue
		}
		s := aff - 0.1*float64(lumaStep(c, k))
		if in {
			s += 1
		}
		if s > bs {
			best, bs = int(k), s
		}
	}
	return best
}

// darken implements the Color style: colors are shaded with a darker color,
// in clean steps towards darker neighbors (or everywhere with Fill).
func (p *ditherPlan) darken(mask []bool) []uint8 {
	out := append([]uint8(nil), p.idx...)
	levels := stageLevels[p.o.Steps]
	shade := make([][16]int, Cells) // cached darker color per cell and color (+1, 0 = unknown)
	for i, c := range p.idx {
		if mask != nil && !mask[i] {
			continue
		}
		if p.protected[i] {
			continue
		}
		f := fillLevels[p.o.Steps]
		if !p.o.Fill {
			q := p.near[i]
			if q < 0 || lumaLevel[p.idx[q]] >= lumaLevel[c] {
				continue // only towards darker neighbors
			}
			reff := math.Min(float64(p.o.Width), p.local[i]-1)
			if reff < 1 {
				continue
			}
			stage := int((p.nearD[i] - 1) * float64(len(levels)) / reff)
			if stage >= len(levels) {
				continue
			}
			f = levels[stage]
		}
		x, y := i%W, i/W
		cell := CellOf(x, y)
		sc := &shade[cell][c]
		if *sc == 0 {
			cols, used := p.src.CellColors(cell)
			var cc []uint8
			for s := 1; s <= 3; s++ {
				if used[s] {
					cc = append(cc, cols[s])
				}
			}
			*sc = p.darkerShade(c, cell, cc) + 2 // -1 → 1 (none)
		}
		d := *sc - 2
		if d < 0 {
			continue
		}
		if p.threshold(x, y, false) < f {
			out[i] = uint8(d)
		}
	}
	return out
}
