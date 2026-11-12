package app

import (
	"math"
	"sort"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- Dithering of gradients (1.1) ----------------------------------------------------
//
// Between two colors of a ramp there is a transition zone; its width is set
// in percent of the band width (0 = hard edges, 100 = mixed all the way).
// In the zone the colors are mixed with the chosen method: ordered patterns
// (Bayer, blue noise, lines, checker, the artist styles from Auto dither) or
// error diffusion (Floyd-Steinberg, Stucki, Sierra, Atkinson, Jarvis).

type gradMethod struct {
	name    string
	ordered func(x, y int) float64 // threshold -0.5 .. 0.5
	diff    c64.DitherMode         // error diffusion
	steps   bool                   // classic 25 / 50 / 75 %
}

var gradMethods = []gradMethod{
	{name: "None"},
	{name: "Classic 25/50/75 %", steps: true, ordered: func(x, y int) float64 { return c64.Threshold(c64.DitherBayer4, x, y) }},
	{name: "Bayer 2×2", ordered: func(x, y int) float64 { return c64.Threshold(c64.DitherBayer2, x, y) }},
	{name: "Bayer 4×4", ordered: func(x, y int) float64 { return c64.Threshold(c64.DitherBayer4, x, y) }},
	{name: "Bayer 8×8", ordered: func(x, y int) float64 { return c64.Threshold(c64.DitherBayer8, x, y) }},
	{name: "Blue noise", ordered: func(x, y int) float64 { return c64.Threshold(c64.DitherBlueNoise, x, y) }},
	{name: "Lines", ordered: func(x, y int) float64 { return c64.Threshold(c64.DitherLines, x, y) }},
	{name: "Cluster dots", ordered: func(x, y int) float64 { return c64.Threshold(c64.DitherCluster, x, y) }},
	{name: "Artist", ordered: func(x, y int) float64 { return c64.StyleThreshold(c64.StyleArtist, x, y) }},
	{name: "Scanlines", ordered: func(x, y int) float64 { return c64.StyleThreshold(c64.StyleScanlines, x, y) }},
	{name: "Checker", ordered: func(x, y int) float64 { return c64.StyleThreshold(c64.StyleChecker, x, y) }},
	{name: "Stripes", ordered: func(x, y int) float64 { return c64.StyleThreshold(c64.StyleStripes, x, y) }},
	{name: "Floyd-Steinberg", diff: c64.DitherFloyd},
	{name: "Stucki", diff: c64.DitherStucki},
	{name: "Sierra", diff: c64.DitherSierra},
	{name: "Atkinson", diff: c64.DitherAtkinson},
	{name: "Jarvis", diff: c64.DitherJarvis},
}

// gradIndex maps t (0..1) to a continuous ramp index: whole numbers inside a
// band, a ramp from one band to the next in the transition zone.
func gradIndex(n int, t float64, width int) float64 {
	t = math.Max(0, math.Min(1, t))
	v := t*float64(n) - 0.5
	i := math.Floor(v)
	if i < 0 {
		return 0
	}
	if int(i) >= n-1 {
		return float64(n - 1)
	}
	f := v - i
	half := float64(clampI(width, 0, 100)) / 200
	switch {
	case half <= 0:
		if f < 0.5 {
			return i
		}
		return i + 1
	case f <= 0.5-half:
		return i
	case f >= 0.5+half:
		return i + 1
	}
	return i + (f-(0.5-half))/(2*half)
}

// gradientRender computes the colors of the pixels pts (indexes into a
// picture of width w) whose ramp positions are ts.
func gradientRender(cols []uint8, method, width int, pts []int, ts []float64, w int) []uint8 {
	n := len(cols)
	out := make([]uint8, len(pts))
	if n == 0 {
		return out
	}
	method = clampI(method, 0, len(gradMethods)-1)
	gm := gradMethods[method]
	if gm.diff == 0 {
		for k, i := range pts {
			v := gradIndex(n, ts[k], width)
			b := math.Floor(v)
			f := v - b
			idx := int(b)
			if f > 0 {
				switch {
				case gm.ordered == nil:
					if f >= 0.5 {
						idx++
					}
				default:
					if gm.steps {
						f = math.Round(f*4) / 4
					}
					if f > gm.ordered(i%w, i/w)+0.5 {
						idx++
					}
				}
			}
			out[k] = cols[clampI(idx, 0, n-1)]
		}
		return out
	}
	// error diffusion in raster order over the area
	taps, div, _ := c64.DiffusionKernel(gm.diff)
	order := make([]int, len(pts))
	for k := range order {
		order[k] = k
	}
	sort.Slice(order, func(a, b int) bool { return pts[order[a]] < pts[order[b]] })
	pos := make(map[int]int, len(pts))
	for k, i := range pts {
		pos[i] = k
	}
	vals := make([]float64, len(pts))
	for k := range pts {
		vals[k] = gradIndex(n, ts[k], width)
	}
	for _, k := range order {
		v := vals[k]
		q := math.Round(v)
		q = math.Max(0, math.Min(float64(n-1), q))
		out[k] = cols[int(q)]
		base := gradIndex(n, ts[k], width)
		if base == math.Floor(base) {
			continue // inside a band: no error to spread
		}
		e := v - q
		x, y := pts[k]%w, pts[k]/w
		for _, tp := range taps {
			if j, ok := pos[(y+tp[1])*w+x+tp[0]]; ok && x+tp[0] >= 0 && x+tp[0] < w {
				vals[j] += e * float64(tp[2]) / float64(div)
			}
		}
	}
	return out
}

// gradDitherUI: method (◀ name ▶) and the width of the transitions.
func (a *App) gradDitherUI(id string, x, y, w int, g *GradientSettings) int {
	u := &a.ui
	m := a.img
	m.Text(font.UI, x, y+3, "Dither", a.th.Text)
	bx := x + 52
	if u.Button(id+"_mp", gfx.Rect{X: bx, Y: y, W: 22, H: 22}, "<") {
		g.Method = (g.Method + len(gradMethods) - 1) % len(gradMethods)
	}
	nr := gfx.Rect{X: bx + 24, Y: y, W: w - 52 - 48, H: 22}
	m.Fill(nr, a.th.PanelAlt)
	m.Frame(nr, a.th.Border)
	m.TextC(font.UI, nr, gradMethods[g.Method].name, a.th.Text)
	if u.Button(id+"_mn", gfx.Rect{X: nr.Right() + 2, Y: y, W: 22, H: 22}, ">") {
		g.Method = (g.Method + 1) % len(gradMethods)
	}
	u.Tip(id+"_mn", nr, "How the colors mix in the transitions")
	y += 26
	g.Width = u.Slider(id+"_w", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Mix width", g.Width, 0, 100, " %")
	u.Tip(id+"_w", gfx.Rect{X: x, Y: y, W: w, H: 22}, "How wide the mixed zone between two colors is: 0 = hard edges, 100 = mixed all the way")
	return y + 26
}
