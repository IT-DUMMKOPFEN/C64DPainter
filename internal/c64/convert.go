package c64

import (
	"image"
	"image/draw"
	"math"
	"sort"
	"sync"
)

// FitMode controls how an image is mapped onto the 320×200 screen.
type FitMode int

const (
	FitStretch FitMode = iota // stretch to 320×200
	FitContain                // keep aspect ratio, black borders
	FitCover                  // keep aspect ratio, crop the overflow
)

// PairMode controls how two horizontal screen pixels become one multicolor pixel.
type PairMode int

const (
	PairAverage PairMode = iota // average of both pixels
	PairLeft                    // take the left pixel
	PairRight                   // take the right pixel
)

// DitherMode selects the dithering method.
type DitherMode int

const (
	DitherNone DitherMode = iota
	DitherBayer2
	DitherBayer4
	DitherBayer8
	DitherCluster
	DitherLines
	DitherNoise
	DitherFloyd
	DitherAtkinson
	DitherJarvis
	DitherStucki
	DitherBurkes
	DitherSierra
	DitherSierraLite
	DitherBlueNoise   // ordered, void-and-cluster blue noise (no visible grid)
	DitherYliluoma    // pattern dithering that mixes two palette colors
	DitherCellPattern // ordered pattern aligned to the color cells
	ditherCount
)

// DitherNames are the display names of the dithering methods.
var DitherNames = [ditherCount]string{
	"None", "Bayer 2×2", "Bayer 4×4", "Bayer 8×8", "Clustered dot", "Line pattern", "Noise",
	"Floyd–Steinberg", "Atkinson", "Jarvis-Judice-Ninke", "Stucki", "Burkes", "Sierra", "Sierra Lite",
	"Blue noise", "Yliluoma (2 colors)", "Cell pattern",
}

// DitherOrdered reports whether a method is an ordered (pattern) dither.
func (d DitherMode) Ordered() bool {
	return (d >= DitherBayer2 && d <= DitherNoise) || d == DitherBlueNoise || d == DitherCellPattern
}

// Diffusion reports whether a method is an error diffusion.
func (d DitherMode) Diffusion() bool { return d >= DitherFloyd && d <= DitherSierraLite }

// Adjust holds image adjustments. The zero value changes nothing.
type Adjust struct {
	Brightness int // -100..100
	Contrast   int // -100..100
	Saturation int // -100..100 (-100 = grey, +100 = double)
	Hue        int // -180..180 degrees
	Gamma      int // -100..100 (positive = brighter mid-tones)
	// AutoLevels stretches the picture to the full range first (as
	// "normalize" in Retropixels): 0 off, 1 each color channel, 2 brightness.
	AutoLevels int
}

// AutoLevelNames are the display names of the automatic contrast modes.
var AutoLevelNames = []string{"Off", "Each channel", "Brightness"}

// autoLevels stretches the colors to the full range.
func autoLevels(px [][3]int, mode int) [][3]int {
	if mode <= 0 || len(px) == 0 {
		return px
	}
	bounds := func(val func(c [3]int) int, cut int) (float64, float64) {
		var h [256]int
		for _, c := range px {
			h[clamp(val(c))]++
		}
		lo, hi, n := 0, 255, 0
		for lo < 255 && n+h[lo] <= cut {
			n += h[lo]
			lo++
		}
		n = 0
		for hi > 0 && n+h[hi] <= cut {
			n += h[hi]
			hi--
		}
		if hi-lo < 16 { // nearly flat: leave it
			return 0, 255
		}
		return float64(lo), float64(hi)
	}
	out := make([][3]int, len(px))
	if mode == 1 {
		var lo, hi [3]float64
		for ch := 0; ch < 3; ch++ {
			ch := ch
			// each channel from its darkest to its brightest value (as the
			// "normalize" of Retropixels); ignoring a few pixels here would
			// shift the colors of pictures with a color cast
			lo[ch], hi[ch] = bounds(func(c [3]int) int { return c[ch] }, 0)
		}
		for i, c := range px {
			for ch := 0; ch < 3; ch++ {
				out[i][ch] = clamp(int(math.Round((float64(c[ch]) - lo[ch]) * 255 / (hi[ch] - lo[ch]))))
			}
		}
		return out
	}
	// brightness: the darkest and brightest 0.1 % are ignored, so single
	// pixels do not spoil it; the colors keep their hue
	lo, hi := bounds(func(c [3]int) int { return (c[0]*299 + c[1]*587 + c[2]*114) / 1000 }, len(px)/1000)
	for i, c := range px {
		for ch := 0; ch < 3; ch++ {
			out[i][ch] = clamp(int(math.Round((float64(c[ch]) - lo) * 255 / (hi - lo))))
		}
	}
	return out
}

// ConvertOptions controls converting PNG/BMP images to multicolor.
type ConvertOptions struct {
	Fit        FitMode
	Pair       PairMode
	Dither     DitherMode
	Strength   int // dithering strength 0..100
	Serpentine bool
	BG         int // -1 = automatic
	Adjust     Adjust
	Match      ColorMatch // how similar colors are measured
	Cells      CellMethod // how the three colors of a cell are chosen
}

// CellMethod selects how the colors of the 4×8 cells are chosen.
type CellMethod int

const (
	CellsBestFit  CellMethod = iota // the three colors that fit the cell best
	CellsFrequent                   // the three most frequent colors (Retropixels)
	CellsFixed                      // the same three colors in all cells
	cellMethodCount
)

// CellMethodNames are the display names of the cell methods.
var CellMethodNames = [cellMethodCount]string{"Best fit", "Most frequent", "Same in all cells"}

// Resample maps any image onto 160×200 multicolor pixels (RGB, box filter).
// The image is placed on a 320×200 screen like on the C64.
func Resample(src image.Image, fit FitMode) [][3]int {
	return ResamplePairs(src, fit, PairAverage)
}

// ResamplePairs is Resample with a choice of how pixel pairs are merged.
func ResamplePairs(src image.Image, fit FitMode, pair PairMode) [][3]int {
	b := src.Bounds()
	sw, sh := float64(b.Dx()), float64(b.Dy())
	out := make([][3]int, W*H)
	// target area on the 320×200 screen
	dx, dy, dw, dh := 0.0, 0.0, 320.0, 200.0
	// 160×200 images already are multicolor pixels
	isFat := b.Dx() == 160 && b.Dy() == 200
	if !isFat && fit != FitStretch {
		sa := sw / sh
		ta := 320.0 / 200.0
		if (fit == FitContain) == (sa > ta) {
			dw = 320
			dh = 320 / sa
		} else {
			dh = 200
			dw = 200 * sa
		}
		dx = (320 - dw) / 2
		dy = (200 - dh) / 2
	}
	// source as premultiplied RGBA (= blended over black)
	src2 := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(src2, src2.Bounds(), src, b.Min, draw.Src)
	rgba := make([][3]int, b.Dx()*b.Dy())
	for i := range rgba {
		p := src2.Pix[i*4 : i*4+3]
		rgba[i] = [3]int{int(p[0]), int(p[1]), int(p[2])}
	}
	for ty := 0; ty < H; ty++ {
		for tx := 0; tx < W; tx++ {
			// the target pixel covers (2tx..2tx+2, ty..ty+1) on the 320×200 screen
			hx0, hx1 := float64(tx*2), float64(tx*2+2)
			if pair == PairLeft && !isFat {
				hx1 = hx0 + 1
			} else if pair == PairRight && !isFat {
				hx0 = hx0 + 1
			}
			x0 := (hx0 - dx) / dw * sw
			x1 := (hx1 - dx) / dw * sw
			y0 := (float64(ty) - dy) / dh * sh
			y1 := (float64(ty+1) - dy) / dh * sh
			if x1 <= 0 || y1 <= 0 || x0 >= sw || y0 >= sh {
				continue // outside → black
			}
			ix0, ix1 := int(math.Floor(x0+1e-6)), int(math.Ceil(x1-1e-6))
			iy0, iy1 := int(math.Floor(y0+1e-6)), int(math.Ceil(y1-1e-6))
			if ix1 <= ix0 {
				ix1 = ix0 + 1
			}
			if iy1 <= iy0 {
				iy1 = iy0 + 1
			}
			var sr, sg, sb, n int
			for y := iy0; y < iy1; y++ {
				if y < 0 || y >= b.Dy() {
					continue
				}
				for x := ix0; x < ix1; x++ {
					if x < 0 || x >= b.Dx() {
						continue
					}
					c := rgba[y*b.Dx()+x]
					sr += c[0]
					sg += c[1]
					sb += c[2]
					n++
				}
			}
			if n > 0 {
				out[ty*W+tx] = [3]int{sr / n, sg / n, sb / n}
			}
		}
	}
	return out
}

// ApplyAdjust applies brightness, contrast, saturation, hue and gamma.
func ApplyAdjust(px [][3]int, a Adjust) [][3]int {
	px = autoLevels(px, a.AutoLevels)
	a.AutoLevels = 0
	out := make([][3]int, len(px))
	if a == (Adjust{}) {
		copy(out, px)
		return out
	}
	// lookup table for gamma, brightness and contrast
	var lut [256]float64
	gamma := math.Pow(2, float64(a.Gamma)/100)
	C := float64(a.Contrast) * 2.55
	cf := (259 * (C + 255)) / (255 * (259 - C))
	for i := range lut {
		v := math.Pow(float64(i)/255, 1/gamma) * 255
		v += float64(a.Brightness) * 1.28
		v = cf*(v-128) + 128
		lut[i] = v
	}
	sat := 1 + float64(a.Saturation)/100
	if sat < 0 {
		sat = 0
	}
	// hue rotation around the grey axis
	h := float64(a.Hue) * math.Pi / 180
	cosA, sinA := math.Cos(h), math.Sin(h)
	m := [3][3]float64{
		{0.213 + cosA*0.787 - sinA*0.213, 0.715 - cosA*0.715 - sinA*0.715, 0.072 - cosA*0.072 + sinA*0.928},
		{0.213 - cosA*0.213 + sinA*0.143, 0.715 + cosA*0.285 + sinA*0.140, 0.072 - cosA*0.072 - sinA*0.283},
		{0.213 - cosA*0.213 - sinA*0.787, 0.715 - cosA*0.715 + sinA*0.715, 0.072 + cosA*0.928 + sinA*0.072},
	}
	for i, c := range px {
		r, g, b := lut[clamp(c[0])], lut[clamp(c[1])], lut[clamp(c[2])]
		if a.Hue != 0 {
			r, g, b = m[0][0]*r+m[0][1]*g+m[0][2]*b, m[1][0]*r+m[1][1]*g+m[1][2]*b, m[2][0]*r+m[2][1]*g+m[2][2]*b
		}
		if sat != 1 {
			y := 0.299*r + 0.587*g + 0.114*b
			r, g, b = y+(r-y)*sat, y+(g-y)*sat, y+(b-y)*sat
		}
		out[i] = [3]int{clamp(int(math.Round(r))), clamp(int(math.Round(g))), clamp(int(math.Round(b)))}
	}
	return out
}

// --- ordered dithering matrices (values 0..n-1) -------------------------------------

var bayer2 = []int{0, 2, 3, 1}
var bayer4 = []int{0, 8, 2, 10, 12, 4, 14, 6, 3, 11, 1, 9, 15, 7, 13, 5}
var cluster4 = []int{12, 5, 6, 13, 4, 0, 1, 7, 11, 3, 2, 8, 15, 10, 9, 14}
var bayer8 = func() []int {
	m := make([]int, 64)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			v := 0
			xc, yc := x^y, y
			for bit := 0; bit < 3; bit++ {
				v = v<<2 | ((xc>>uint(2-bit))&1)<<1 | (yc>>uint(2-bit))&1
			}
			m[y*8+x] = v
		}
	}
	return m
}()

// threshold returns the ordered-dither offset in -0.5..0.5 for a pixel.
func threshold(d DitherMode, x, y int) float64 {
	switch d {
	case DitherBayer2:
		return (float64(bayer2[(y&1)*2+(x&1)])+0.5)/4 - 0.5
	case DitherBayer4:
		return (float64(bayer4[(y&3)*4+(x&3)])+0.5)/16 - 0.5
	case DitherBayer8:
		return (float64(bayer8[(y&7)*8+(x&7)])+0.5)/64 - 0.5
	case DitherCluster:
		return (float64(cluster4[(y&3)*4+(x&3)])+0.5)/16 - 0.5
	case DitherLines:
		return (float64([]int{0, 2, 1, 3}[y&3])+0.5)/4 - 0.5
	case DitherNoise:
		h := uint32(x)*374761393 + uint32(y)*668265263
		h = (h ^ (h >> 13)) * 1274126177
		h ^= h >> 16
		return float64(h&0xFFFF)/65536 - 0.5
	case DitherBlueNoise:
		m := blueNoise()
		return (float64(m[(y&31)*32+(x&31)])+0.5)/1024 - 0.5
	case DitherCellPattern:
		return (float64(cellPattern[(y&7)*4+(x&3)])+0.5)/32 - 0.5
	}
	return 0
}

// cellPattern: a 4 × 8 ordered matrix (one multicolor cell): the even
// columns of the 8 × 8 Bayer matrix, ranked 0..31.
var cellPattern = func() []int {
	vals := make([]int, 32)
	for y := 0; y < 8; y++ {
		for x := 0; x < 4; x++ {
			vals[y*4+x] = bayer8[y*8+x*2]
		}
	}
	idx := make([]int, 32)
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return vals[idx[a]] < vals[idx[b]] })
	out := make([]int, 32)
	for r, i := range idx {
		out[i] = r
	}
	return out
}()

var (
	blueOnce sync.Once
	blueMat  []int
)

// blueNoise returns a 32 × 32 blue noise matrix (ranks 0..1023), made with
// the void-and-cluster method (Ulichney 1993).
func blueNoise() []int {
	blueOnce.Do(func() { blueMat = voidAndCluster(32, 1.5) })
	return blueMat
}

func voidAndCluster(n int, sigma float64) []int {
	size := n * n
	// toroidal Gaussian kernel
	kern := make([]float64, size)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx, dy := min(x, n-x), min(y, n-y)
			kern[y*n+x] = math.Exp(-float64(dx*dx+dy*dy) / (2 * sigma * sigma))
		}
	}
	energy := make([]float64, size)
	on := make([]bool, size)
	add := func(i int, sgn float64) {
		ix, iy := i%n, i/n
		for y := 0; y < n; y++ {
			for x := 0; x < n; x++ {
				energy[y*n+x] += sgn * kern[((y-iy+n)%n)*n+(x-ix+n)%n]
			}
		}
	}
	// initial pattern: ~10 % points from a fixed pseudo random sequence
	h := uint32(12345)
	count := 0
	for count < size/10 {
		h = h*1664525 + 1013904223
		i := int(h>>8) % size
		if !on[i] {
			on[i] = true
			add(i, 1)
			count++
		}
	}
	tightest := func() int {
		best, be := -1, -1.0
		for i := 0; i < size; i++ {
			if on[i] && energy[i] > be {
				best, be = i, energy[i]
			}
		}
		return best
	}
	largestVoid := func() int {
		best, be := -1, math.Inf(1)
		for i := 0; i < size; i++ {
			if !on[i] && energy[i] < be {
				best, be = i, energy[i]
			}
		}
		return best
	}
	// relax: move points from clusters into voids until stable
	for iter := 0; iter < size; iter++ {
		c := tightest()
		on[c] = false
		add(c, -1)
		v := largestVoid()
		if v == c {
			on[c] = true
			add(c, 1)
			break
		}
		on[v] = true
		add(v, 1)
	}
	rank := make([]int, size)
	proto := append([]bool(nil), on...)
	protoE := append([]float64(nil), energy...)
	// phase 1: remove points from the tightest clusters
	r := count - 1
	for r >= 0 {
		c := tightest()
		on[c] = false
		add(c, -1)
		rank[c] = r
		r--
	}
	// phase 2 + 3: fill the largest voids
	copy(on, proto)
	copy(energy, protoE)
	for r = count; r < size; r++ {
		v := largestVoid()
		on[v] = true
		add(v, 1)
		rank[v] = r
	}
	return rank
}

// error diffusion kernels: dx, dy, weight; divisor
type kernel struct {
	taps [][3]int
	div  int
}

var kernels = map[DitherMode]kernel{
	DitherFloyd:      {[][3]int{{1, 0, 7}, {-1, 1, 3}, {0, 1, 5}, {1, 1, 1}}, 16},
	DitherAtkinson:   {[][3]int{{1, 0, 1}, {2, 0, 1}, {-1, 1, 1}, {0, 1, 1}, {1, 1, 1}, {0, 2, 1}}, 8},
	DitherJarvis:     {[][3]int{{1, 0, 7}, {2, 0, 5}, {-2, 1, 3}, {-1, 1, 5}, {0, 1, 7}, {1, 1, 5}, {2, 1, 3}, {-2, 2, 1}, {-1, 2, 3}, {0, 2, 5}, {1, 2, 3}, {2, 2, 1}}, 48},
	DitherStucki:     {[][3]int{{1, 0, 8}, {2, 0, 4}, {-2, 1, 2}, {-1, 1, 4}, {0, 1, 8}, {1, 1, 4}, {2, 1, 2}, {-2, 2, 1}, {-1, 2, 2}, {0, 2, 4}, {1, 2, 2}, {2, 2, 1}}, 42},
	DitherBurkes:     {[][3]int{{1, 0, 8}, {2, 0, 4}, {-2, 1, 2}, {-1, 1, 4}, {0, 1, 8}, {1, 1, 4}, {2, 1, 2}}, 32},
	DitherSierra:     {[][3]int{{1, 0, 5}, {2, 0, 3}, {-2, 1, 2}, {-1, 1, 4}, {0, 1, 5}, {1, 1, 4}, {2, 1, 2}, {-1, 2, 2}, {0, 2, 3}, {1, 2, 2}}, 32},
	DitherSierraLite: {[][3]int{{1, 0, 2}, {-1, 1, 1}, {0, 1, 1}}, 4},
}

// Convert converts an image while strictly following the multicolor rules.
func Convert(src image.Image, pal *Palette, o ConvertOptions) *Picture {
	px := ResamplePairs(src, o.Fit, o.Pair)
	return ConvertRGB(ApplyAdjust(px, o.Adjust), pal, o)
}

// ditherPass gives every pixel one of the allowed palette colors, using the
// selected dithering method. It returns palette indices.
func ditherPass(px [][3]int, pr *[16][3]int, o ConvertOptions, allowed func(x, y int) []int) []uint8 {
	return ditherPassW(px, W, pr, o, allowed)
}

// ditherPassW is ditherPass for pictures of width W (160) or HW (320).
func ditherPassW(px [][3]int, W int, pr *[16][3]int, o ConvertOptions, allowed func(x, y int) []int) []uint8 {
	out := make([]uint8, W*H)
	dist := colorMetric(o.Match)
	strength := float64(clamp100(o.Strength)) / 100
	if o.Dither == DitherNone {
		strength = 0
	}
	k, diffuse := kernels[o.Dither]
	work := make([][3]float64, len(px))
	for i, c := range px {
		work[i] = [3]float64{float64(c[0]), float64(c[1]), float64(c[2])}
	}
	for y := 0; y < H; y++ {
		rev := o.Serpentine && diffuse && y&1 == 1
		for i := 0; i < W; i++ {
			x := i
			if rev {
				x = W - 1 - i
			}
			c := work[y*W+x]
			// keep accumulated error in a sane range
			for ch := 0; ch < 3; ch++ {
				c[ch] = math.Max(-64, math.Min(319, c[ch]))
			}
			cr, cg, cb := clamp(int(c[0]+0.5)), clamp(int(c[1]+0.5)), clamp(int(c[2]+0.5))
			if o.Dither.Ordered() && strength > 0 {
				// as in Retropixels: the pattern shifts the brightness, then the
				// nearest allowed color is taken – calm, regular patterns
				off := int(math.Round(threshold(o.Dither, x, y) * strength * 64))
				cr, cg, cb = clamp(cr+off), clamp(cg+off), clamp(cb+off)
			} else if o.Dither == DitherYliluoma && strength > 0 {
				out[y*W+x] = uint8(mixPick(cr, cg, cb, pr, allowed(x, y), threshold(DitherBayer4, x, y)+0.5, strength, dist))
				continue
			}
			best, bd := 0, 1<<62
			for _, kk := range allowed(x, y) {
				d := dist(cr, cg, cb, pr[kk][0], pr[kk][1], pr[kk][2])
				if d < bd {
					bd, best = d, kk
				}
			}
			out[y*W+x] = uint8(best)
			if diffuse && strength > 0 {
				// the error is limited, so single far-off pixels do not throw
				// stray colors into the neighbourhood
				lim := 24 + 40*strength
				er := [3]float64{
					math.Max(-lim, math.Min(lim, (math.Max(0, math.Min(255, c[0]))-float64(pr[best][0]))*strength)),
					math.Max(-lim, math.Min(lim, (math.Max(0, math.Min(255, c[1]))-float64(pr[best][1]))*strength)),
					math.Max(-lim, math.Min(lim, (math.Max(0, math.Min(255, c[2]))-float64(pr[best][2]))*strength)),
				}
				for _, tp := range k.taps {
					dx := tp[0]
					if rev {
						dx = -dx
					}
					xx, yy := x+dx, y+tp[1]
					if xx < 0 || xx >= W || yy >= H {
						continue
					}
					f := float64(tp[2]) / float64(k.div)
					w := &work[yy*W+xx]
					w[0] += er[0] * f
					w[1] += er[1] * f
					w[2] += er[2] * f
				}
			}
		}
	}
	return out
}

// mixPick implements ordered dithering as a mix of two colors: it finds the
// pair of allowed colors whose blend comes closest to the target, then uses the
// threshold t (0..1) of the dither pattern to choose one of the two. A higher
// strength allows mixing colors that are further apart.
func mixPick(r, g, b int, pr *[16][3]int, allowed []int, t, strength float64, dist func(r1, g1, b1, r2, g2, b2 int) int) int {
	lambda := 0.02 + (1-strength)*0.8
	if len(allowed) > 6 {
		// only the six nearest colors are worth mixing
		var near [16]int
		var nd [16]int
		n := 0
		for _, k := range allowed {
			c := pr[k]
			d := dist(r, g, b, c[0], c[1], c[2])
			j := n
			for j > 0 && nd[j-1] > d {
				near[j], nd[j] = near[j-1], nd[j-1]
				j--
			}
			near[j], nd[j] = k, d
			n++
		}
		allowed = append([]int(nil), near[:6]...)
	}
	best, bestB := allowed[0], allowed[0]
	bestR, bestE := 0.0, math.Inf(1)
	for i, ka := range allowed {
		A := pr[ka]
		for _, kb := range allowed[i:] {
			B := pr[kb]
			dr, dg, db := float64(B[0]-A[0]), float64(B[1]-A[1]), float64(B[2]-A[2])
			ll := dr*dr + dg*dg + db*db
			ratio := 0.0
			if ll > 0 {
				ratio = ((float64(r-A[0]))*dr + (float64(g-A[1]))*dg + (float64(b-A[2]))*db) / ll
				ratio = math.Max(0, math.Min(1, ratio))
				// quantize to the pattern resolution so flat areas stay calm
				ratio = math.Round(ratio*16) / 16
			}
			mr := float64(A[0]) + ratio*dr
			mg := float64(A[1]) + ratio*dg
			mb := float64(A[2]) + ratio*db
			e := float64(dist(r, g, b, int(mr+0.5), int(mg+0.5), int(mb+0.5)))
			e += lambda * float64(dist(A[0], A[1], A[2], B[0], B[1], B[2])) * ratio * (1 - ratio)
			if e < bestE {
				bestE, best, bestB, bestR = e, ka, kb, ratio
			}
		}
	}
	if t < bestR {
		return bestB
	}
	return best
}

var all16 = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

// cellReference returns the picture the cell colors are chosen from:
//   - ordered dithering: the picture with the pattern added (as Retropixels
//     does), so the colors of the pattern win the cells;
//   - Yliluoma: the picture dithered freely with all 16 colors (mixed colors
//     survive);
//   - error diffusion and no dithering: the picture itself – choosing the
//     cells from a freely diffused picture brought stray colors into them.
func cellReference(px [][3]int, w int, pr *[16][3]int, o ConvertOptions) [][3]int {
	if o.Dither == DitherNone || o.Strength <= 0 {
		return px
	}
	switch {
	case o.Dither.Ordered():
		s := float64(clamp100(o.Strength)) / 100
		ref := make([][3]int, len(px))
		for i, c := range px {
			off := int(math.Round(threshold(o.Dither, i%w, i/w) * s * 64))
			ref[i] = [3]int{clamp(c[0] + off), clamp(c[1] + off), clamp(c[2] + off)}
		}
		return ref
	case o.Dither == DitherYliluoma:
		free := ditherPassW(px, w, pr, o, func(x, y int) []int { return all16 })
		ref := make([][3]int, len(px))
		for i, k := range free {
			ref[i] = pr[k]
		}
		return ref
	}
	return px
}

// ConvertRGB converts 160×200 RGB values into a multicolor picture.
//
// With dithering, the picture is first dithered freely with all 16 colors; the
// colors of every 4×8 cell are then chosen from that result, so mixed colors
// (for example green made of light green and dark pixels) survive. Finally
// every pixel is dithered again, this time only with the background color and
// the three colors of its cell. The result always follows the multicolor rules.
func ConvertRGB(px [][3]int, pal *Palette, o ConvertOptions) *Picture {
	var pr [16][3]int
	for i, c := range pal.Colors {
		pr[i] = [3]int{int(c >> 16 & 255), int(c >> 8 & 255), int(c & 255)}
	}
	ref := cellReference(px, W, &pr, o)
	metric := colorMetric(o.Match)
	dist := make([][16]int, W*H)
	near := make([]uint8, W*H)
	hist := [16]int{}
	for i, c := range ref {
		best, bd := 0, 1<<62
		for k := 0; k < 16; k++ {
			d := metric(c[0], c[1], c[2], pr[k][0], pr[k][1], pr[k][2])
			dist[i][k] = d
			if d < bd {
				bd, best = d, k
			}
		}
		hist[best]++
		near[i] = uint8(best)
	}
	// background color candidates
	var bgCands []int
	if o.BG >= 0 && o.BG < 16 {
		bgCands = []int{o.BG}
	} else {
		idx := make([]int, 16)
		for i := range idx {
			idx[i] = i
		}
		sort.SliceStable(idx, func(a, b int) bool { return hist[idx[a]] > hist[idx[b]] })
		for _, k := range idx[:4] {
			if hist[k] > 0 || len(bgCands) == 0 {
				bgCands = append(bgCands, k)
			}
		}
	}
	bestTotal := 1 << 62
	var bestBG int
	var bestCells [][3]int
	for _, bg := range bgCands {
		total := 0
		cells := make([][3]int, Cells)
		if o.Cells == CellsFixed {
			tri, e := fixedTriple(dist, hist, bg)
			for i := range cells {
				cells[i] = tri
			}
			total = e
		}
		for cell := 0; cell < Cells && o.Cells != CellsFixed; cell++ {
			if o.Cells == CellsFrequent {
				tri, e := frequentTriple(dist, near, cell, bg)
				cells[cell] = tri
				total += e
				continue
			}
			cx, cy := (cell%Cols)*4, (cell/Cols)*8
			// candidates: nearest and second nearest color of every pixel
			var cand [16]bool
			for y := cy; y < cy+8; y++ {
				for x := cx; x < cx+4; x++ {
					d := &dist[y*W+x]
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
			var cl []int
			for k := 0; k < 16; k++ {
				if cand[k] && k != bg {
					cl = append(cl, k)
				}
			}
			for k := 0; k < 16 && len(cl) < 3; k++ {
				if k != bg && !contains(cl, k) {
					cl = append(cl, k)
				}
			}
			bestE := 1 << 62
			var bc [3]int
			for a := 0; a < len(cl); a++ {
				for b := a + 1; b < len(cl); b++ {
					for c := b + 1; c < len(cl); c++ {
						e := 0
						for y := cy; y < cy+8 && e < bestE; y++ {
							for x := cx; x < cx+4; x++ {
								d := &dist[y*W+x]
								m := d[bg]
								if d[cl[a]] < m {
									m = d[cl[a]]
								}
								if d[cl[b]] < m {
									m = d[cl[b]]
								}
								if d[cl[c]] < m {
									m = d[cl[c]]
								}
								e += m
							}
						}
						if e < bestE {
							bestE = e
							bc = [3]int{cl[a], cl[b], cl[c]}
						}
					}
				}
			}
			cells[cell] = bc
			total += bestE
		}
		if total < bestTotal {
			bestTotal, bestBG, bestCells = total, bg, cells
		}
	}
	p := &Picture{BG: uint8(bestBG)}
	allowed := make([][]int, Cells)
	for cell := range allowed {
		allowed[cell] = []int{bestBG, bestCells[cell][0], bestCells[cell][1], bestCells[cell][2]}
	}
	final := ditherPass(px, &pr, o, func(x, y int) []int { return allowed[CellOf(x, y)] })
	for cell := 0; cell < Cells; cell++ {
		for s := 0; s < 3; s++ {
			p.SetSlotColor(cell, s+1, uint8(bestCells[cell][s]))
		}
	}
	for i, k := range final {
		cell := CellOf(i%W, i/W)
		bits := uint8(0)
		for s := 1; s <= 3; s++ {
			if int(k) == bestCells[cell][s-1] && int(k) != bestBG {
				bits = uint8(s)
			}
		}
		p.Bits[i] = bits
	}
	p.CleanUnused()
	return p
}

func clamp100(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func contains(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// ResampleScreen maps an image onto the 320×200 screen (RGB, box filter).
// A 160×200 image is treated as multicolor pixels and doubled horizontally.
func ResampleScreen(src image.Image, fit FitMode) [][3]int {
	b := src.Bounds()
	if b.Dx() == 160 && b.Dy() == 200 {
		fat := ResamplePairs(src, fit, PairAverage)
		out := make([][3]int, 320*200)
		for y := 0; y < 200; y++ {
			for x := 0; x < 320; x++ {
				out[y*320+x] = fat[y*W+x/2]
			}
		}
		return out
	}
	// 320×200 = treat each screen pixel as two "fat" halves of a 640-wide image
	sw, sh := float64(b.Dx()), float64(b.Dy())
	dx, dy, dw, dh := 0.0, 0.0, 320.0, 200.0
	if fit != FitStretch {
		sa := sw / sh
		if (fit == FitContain) == (sa > 1.6) {
			dw, dh = 320, 320/sa
		} else {
			dh, dw = 200, 200*sa
		}
		dx, dy = (320-dw)/2, (200-dh)/2
	}
	src2 := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(src2, src2.Bounds(), src, b.Min, draw.Src)
	out := make([][3]int, 320*200)
	for ty := 0; ty < 200; ty++ {
		y0 := (float64(ty) - dy) / dh * sh
		y1 := (float64(ty+1) - dy) / dh * sh
		for tx := 0; tx < 320; tx++ {
			x0 := (float64(tx) - dx) / dw * sw
			x1 := (float64(tx+1) - dx) / dw * sw
			if x1 <= 0 || y1 <= 0 || x0 >= sw || y0 >= sh {
				continue
			}
			ix0, ix1 := int(math.Floor(x0+1e-6)), int(math.Ceil(x1-1e-6))
			iy0, iy1 := int(math.Floor(y0+1e-6)), int(math.Ceil(y1-1e-6))
			if ix1 <= ix0 {
				ix1 = ix0 + 1
			}
			if iy1 <= iy0 {
				iy1 = iy0 + 1
			}
			var sr, sg, sb, n int
			for y := max(iy0, 0); y < min(iy1, b.Dy()); y++ {
				row := src2.Pix[y*src2.Stride:]
				for x := max(ix0, 0); x < min(ix1, b.Dx()); x++ {
					sr += int(row[x*4])
					sg += int(row[x*4+1])
					sb += int(row[x*4+2])
					n++
				}
			}
			if n > 0 {
				out[ty*320+tx] = [3]int{sr / n, sg / n, sb / n}
			}
		}
	}
	return out
}

// MergePairs turns a 320×200 screen into 160×200 multicolor pixels.
func MergePairs(screen [][3]int, pair PairMode) [][3]int {
	out := make([][3]int, W*H)
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			l, r := screen[y*320+2*x], screen[y*320+2*x+1]
			switch pair {
			case PairLeft:
				out[y*W+x] = l
			case PairRight:
				out[y*W+x] = r
			default:
				out[y*W+x] = [3]int{(l[0] + r[0]) / 2, (l[1] + r[1]) / 2, (l[2] + r[2]) / 2}
			}
		}
	}
	return out
}

// tripleError sums, over the pixels of a cell, the distance to the nearest of
// the background and the three cell colors.
func tripleError(dist [][16]int, cell, bg int, tri [3]int) int {
	cx, cy := (cell%Cols)*4, (cell/Cols)*8
	e := 0
	for y := cy; y < cy+8; y++ {
		for x := cx; x < cx+4; x++ {
			d := &dist[y*W+x]
			m := d[bg]
			for _, k := range tri {
				if d[k] < m {
					m = d[k]
				}
			}
			e += m
		}
	}
	return e
}

// frequentTriple: the three colors that occur most often in the cell (as
// nearest colors), like Retropixels does it.
func frequentTriple(dist [][16]int, near []uint8, cell, bg int) ([3]int, int) {
	cx, cy := (cell%Cols)*4, (cell/Cols)*8
	var cnt [16]int
	for y := cy; y < cy+8; y++ {
		for x := cx; x < cx+4; x++ {
			cnt[near[y*W+x]]++
		}
	}
	var tri [3]int
	var used [16]bool
	used[bg] = true
	for s := 0; s < 3; s++ {
		best := -1
		for k := 0; k < 16; k++ {
			if !used[k] && (best < 0 || cnt[k] > cnt[best]) {
				best = k
			}
		}
		used[best] = true
		tri[s] = best
	}
	return tri, tripleError(dist, cell, bg, tri)
}

// fixedTriple: the same three colors for the whole picture.
func fixedTriple(dist [][16]int, hist [16]int, bg int) ([3]int, int) {
	idx := make([]int, 0, 16)
	for k := 0; k < 16; k++ {
		if k != bg {
			idx = append(idx, k)
		}
	}
	sort.SliceStable(idx, func(a, b int) bool { return hist[idx[a]] > hist[idx[b]] })
	if len(idx) > 8 {
		idx = idx[:8]
	}
	best, bestE := [3]int{idx[0], idx[1], idx[2]}, 1<<62
	for a := 0; a < len(idx); a++ {
		for b := a + 1; b < len(idx); b++ {
			for c := b + 1; c < len(idx); c++ {
				tri := [3]int{idx[a], idx[b], idx[c]}
				e := 0
				for cell := 0; cell < Cells && e < bestE; cell++ {
					e += tripleError(dist, cell, bg, tri)
				}
				if e < bestE {
					best, bestE = tri, e
				}
			}
		}
	}
	return best, bestE
}
