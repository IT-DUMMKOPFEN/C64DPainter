package app

import (
	"math"

	"c64dpainter/internal/gfx"
)

// --- Perspective ---------------------------------------------------------------------
//
// The floating selection can be distorted in perspective. Horizontal: the
// corners of the top and bottom edge move left/right; vertical: the corners
// of the left and right edge move up/down. "Opposite" moves the other
// corner of the same edge the other way (a symmetric trapezoid), "Free"
// moves every corner on its own. The handle in the middle of an edge moves
// the whole edge.

// handle numbers of the perspective handles
const (
	hpTL = 20 + iota
	hpTR
	hpBR
	hpBL
	hpMidA // top (horizontal) or left (vertical)
	hpMidB // bottom (horizontal) or right (vertical)
)

// perspective is the distortion of the selection: offsets of the corners
// TL, TR, BR, BL (multicolor pixels across, rows down).
type perspective struct {
	off  [4]float64
	on   bool // perspective handles are shown
	free bool // corners move freely (else opposite)
	vert bool // vertical: corners move up/down
}

func (p *perspective) active() bool {
	return p.off != [4]float64{}
}

// quad returns the corners TL, TR, BR, BL for the rectangle x0..x1, y0..y1.
func (p *perspective) quad(x0, y0, x1, y1 float64) [4][2]float64 {
	o := p.off
	if p.vert {
		return [4][2]float64{{x0, y0 + o[0]}, {x1, y0 + o[1]}, {x1, y1 + o[2]}, {x0, y1 + o[3]}}
	}
	return [4][2]float64{{x0 + o[0], y0}, {x1 + o[1], y0}, {x1 + o[2], y1}, {x0 + o[3], y1}}
}

// homography returns the 3×3 matrix that maps the unit square (0,0),(1,0),
// (1,1),(0,1) onto the quad (Heckbert, "Fundamentals of Texture Mapping").
func homography(q [4][2]float64) [9]float64 {
	x0, y0 := q[0][0], q[0][1]
	x1, y1 := q[1][0], q[1][1]
	x2, y2 := q[2][0], q[2][1]
	x3, y3 := q[3][0], q[3][1]
	dx1, dx2, dx3 := x1-x2, x3-x2, x0-x1+x2-x3
	dy1, dy2, dy3 := y1-y2, y3-y2, y0-y1+y2-y3
	var g, h float64
	if math.Abs(dx3) > 1e-9 || math.Abs(dy3) > 1e-9 {
		den := dx1*dy2 - dx2*dy1
		if math.Abs(den) < 1e-12 {
			den = 1e-12
		}
		g = (dx3*dy2 - dx2*dy3) / den
		h = (dx1*dy3 - dx3*dy1) / den
	}
	return [9]float64{
		x1 - x0 + g*x1, x3 - x0 + h*x3, x0,
		y1 - y0 + g*y1, y3 - y0 + h*y3, y0,
		g, h, 1,
	}
}

func invert3(m [9]float64) ([9]float64, bool) {
	a, b, c := m[0], m[1], m[2]
	d, e, f := m[3], m[4], m[5]
	g, h, i := m[6], m[7], m[8]
	A := e*i - f*h
	B := -(d*i - f*g)
	C := d*h - e*g
	det := a*A + b*B + c*C
	if math.Abs(det) < 1e-12 {
		return m, false
	}
	return [9]float64{
		A / det, -(b*i - c*h) / det, (b*f - c*e) / det,
		B / det, (a*i - c*g) / det, -(a*f - c*d) / det,
		C / det, -(a*h - b*g) / det, (a*e - b*d) / det,
	}, true
}

// warpPerspective distorts a brush: the brush rectangle (0..W, 0..H) becomes
// the quad of p. It returns the new brush and its offset to the old one.
func warpPerspective(src *CustomBrush, p *perspective) (*CustomBrush, int, int) {
	W, H := float64(src.W), float64(src.H)
	q := p.quad(0, 0, W, H)
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, c := range q {
		minX, maxX = math.Min(minX, c[0]), math.Max(maxX, c[0])
		minY, maxY = math.Min(minY, c[1]), math.Max(maxY, c[1])
	}
	X0, Y0 := int(math.Floor(minX+1e-6)), int(math.Floor(minY+1e-6))
	X1, Y1 := int(math.Ceil(maxX-1e-6)), int(math.Ceil(maxY-1e-6))
	w, h := min(max(1, X1-X0), 2000), min(max(1, Y1-Y0), 2000)
	out := &CustomBrush{W: w, H: h, Pix: make([]int8, w*h)}
	for i := range out.Pix {
		out.Pix[i] = -1
	}
	inv, ok := invert3(homography(q))
	if !ok {
		return out, X0, Y0
	}
	for y := 0; y < h; y++ {
		py := float64(Y0+y) + 0.5
		for x := 0; x < w; x++ {
			px := float64(X0+x) + 0.5
			sw := inv[6]*px + inv[7]*py + inv[8]
			if math.Abs(sw) < 1e-12 {
				continue
			}
			s := (inv[0]*px + inv[1]*py + inv[2]) / sw
			t := (inv[3]*px + inv[4]*py + inv[5]) / sw
			if s < 0 || t < 0 || s >= 1 || t >= 1 {
				continue
			}
			si, sj := int(s*W), int(t*H)
			out.Pix[y*w+x] = src.Pix[sj*src.W+si]
		}
	}
	return out, X0, Y0
}

// perspBase returns the rectangle of the selection before the perspective
// (picture units).
func (a *App) perspBase() (x0, y0, x1, y1 float64) {
	s := &a.sel
	minU, minV, maxU, maxV := s.xf.bounds(s.src.W, s.src.H)
	ap := s.xf.aspect()
	return s.xf.cx + minU/ap, s.xf.cy + minV, s.xf.cx + maxU/ap, s.xf.cy + maxV
}

// perspHandles returns the screen positions of the six perspective handles
// (TL, TR, BR, BL, middle A, middle B).
func (a *App) perspHandles() [6][2]int {
	var out [6][2]int
	s := &a.sel
	if s.state != selFloating || s.src == nil {
		return out
	}
	q := s.persp.quad(a.perspBase())
	for i, c := range q {
		out[i][0], out[i][1] = a.picFToScreen(c[0], c[1])
	}
	mid := func(i, j int) [2]int {
		return [2]int{(out[i][0] + out[j][0]) / 2, (out[i][1] + out[j][1]) / 2}
	}
	if s.persp.vert {
		out[4], out[5] = mid(0, 3), mid(1, 2)
	} else {
		out[4], out[5] = mid(0, 1), mid(3, 2)
	}
	return out
}

// perspHandleAt returns the perspective handle under the mouse or -1.
func (a *App) perspHandleAt(sx, sy int) int {
	s := &a.sel
	if !s.persp.on || s.state != selFloating {
		return -1
	}
	for i, p := range a.perspHandles() {
		if absI(sx-p[0]) <= 6 && absI(sy-p[1]) <= 6 {
			return hpTL + i
		}
	}
	return -1
}

// perspDrag moves a perspective handle; start holds the offsets at the
// start of the drag, d the movement (multicolor pixels or rows).
func (p *perspective) drag(handle int, start [4]float64, d float64) {
	p.off = start
	// the corner on the same edge
	partner := map[int]int{0: 1, 1: 0, 2: 3, 3: 2}
	if p.vert {
		partner = map[int]int{0: 3, 3: 0, 1: 2, 2: 1}
	}
	switch handle {
	case hpMidA, hpMidB:
		var i, j int
		switch {
		case !p.vert && handle == hpMidA:
			i, j = 0, 1
		case !p.vert:
			i, j = 3, 2
		case handle == hpMidA:
			i, j = 0, 3
		default:
			i, j = 1, 2
		}
		p.off[i] += d
		p.off[j] += d
	default:
		c := handle - hpTL
		p.off[c] += d
		if !p.free {
			p.off[partner[c]] -= d
		}
	}
}

// drawPerspective draws the outline of the quad and the handles.
func (a *App) drawPerspective() {
	m := a.img
	hs := a.perspHandles()
	for i := 0; i < 4; i++ {
		p, q := hs[i], hs[(i+1)%4]
		m.Line(p[0], p[1], q[0], q[1], a.th.Accent)
	}
	for i, p := range hs {
		r := gfx.Rect{X: p[0] - 4, Y: p[1] - 4, W: 9, H: 9}
		m.Fill(r, 0x000000)
		if i >= 4 {
			m.Fill(r.Inset(1), a.th.Accent)
		} else {
			m.Fill(r.Inset(1), 0xFFFFFF)
		}
	}
}
