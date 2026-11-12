package app

import (
	"math"

	"c64dpainter/internal/gfx"
)

// --- Bend ----------------------------------------------------------------------------
//
// The floating selection (or a brush made from it) can be bent as a whole.
// Horizontal: the columns move up or down along a curve, so the shape bends
// like an arch; the two handles sit on the top and bottom edge at the
// highest point of the curve. Dragging a handle up/down sets the strength,
// left/right moves the highest point. Vertical works the same way with the
// rows moving left/right and the handles on the left and right edge.

const (
	hbA = 40 + iota // top (horizontal) or left (vertical)
	hbB             // bottom (horizontal) or right (vertical)
)

type bend struct {
	on     bool
	vert   bool
	amount float64 // shift at the highest point (pixels / rows)
	apex   float64 // position of the highest point, 0..1
}

func (b *bend) active() bool { return b.amount != 0 }

func (b *bend) reset() { b.amount, b.apex = 0, 0.5 }

// shift returns the movement at position t (0..1 across the shape).
func (b *bend) shift(t float64) float64 {
	c := b.apex
	if c <= 0 || c >= 1 {
		c = 0.5
	}
	var u float64
	if t < c {
		u = (c - t) / c
	} else {
		u = (t - c) / (1 - c)
	}
	return b.amount * (1 - u*u)
}

// warpBend bends a brush; it returns the new brush and how far its top left
// corner moved.
func warpBend(src *CustomBrush, b *bend) (*CustomBrush, int, int) {
	if !b.active() {
		return src, 0, 0
	}
	ext := int(math.Ceil(math.Abs(b.amount)))
	neg := b.amount < 0
	if b.vert {
		out := &CustomBrush{W: src.W + ext, H: src.H, Pix: make([]int8, (src.W+ext)*src.H)}
		for i := range out.Pix {
			out.Pix[i] = -1
		}
		for y := 0; y < src.H; y++ {
			t := 0.5
			if src.H > 1 {
				t = float64(y) / float64(src.H-1)
			}
			d := int(math.Round(b.shift(t)))
			if neg {
				d += ext
			}
			for x := 0; x < src.W; x++ {
				if v := src.Pix[y*src.W+x]; v >= 0 {
					out.Pix[y*out.W+x+d] = v
				}
			}
		}
		if neg {
			return out, -ext, 0
		}
		return out, 0, 0
	}
	out := &CustomBrush{W: src.W, H: src.H + ext, Pix: make([]int8, src.W*(src.H+ext))}
	for i := range out.Pix {
		out.Pix[i] = -1
	}
	for x := 0; x < src.W; x++ {
		t := 0.5
		if src.W > 1 {
			t = float64(x) / float64(src.W-1)
		}
		d := int(math.Round(b.shift(t)))
		if neg {
			d += ext
		}
		for y := 0; y < src.H; y++ {
			if v := src.Pix[y*src.W+x]; v >= 0 {
				out.Pix[(y+d)*out.W+x] = v
			}
		}
	}
	if neg {
		return out, 0, -ext
	}
	return out, 0, 0
}

// bendHandles returns the screen positions of the two bend handles.
func (a *App) bendHandles() [2][2]int {
	var out [2][2]int
	s := &a.sel
	if s.state != selFloating || s.out == nil {
		return out
	}
	x0, y0, x1, y1 := a.perspBase()
	b := &s.bend
	if b.vert {
		y := y0 + (y1-y0)*b.apex
		out[0][0], out[0][1] = a.picFToScreen(x0+b.amount, y)
		out[1][0], out[1][1] = a.picFToScreen(x1+b.amount, y)
	} else {
		x := x0 + (x1-x0)*b.apex
		out[0][0], out[0][1] = a.picFToScreen(x, y0+b.amount)
		out[1][0], out[1][1] = a.picFToScreen(x, y1+b.amount)
	}
	return out
}

func (a *App) bendHandleAt(sx, sy int) int {
	if !a.sel.bend.on || a.sel.state != selFloating {
		return -1
	}
	for i, p := range a.bendHandles() {
		if absI(sx-p[0]) <= 7 && absI(sy-p[1]) <= 7 {
			return hbA + i
		}
	}
	return -1
}

// bendDrag: dx/dy since the start of the drag (picture units).
func (a *App) bendDrag(start bend, dx, dy float64) {
	s := &a.sel
	x0, y0, x1, y1 := a.perspBase()
	b := start
	if b.vert {
		b.amount = math.Round(start.amount + dx)
		if y1 > y0 {
			b.apex = start.apex + dy/(y1-y0)
		}
	} else {
		b.amount = math.Round(start.amount + dy)
		if x1 > x0 {
			b.apex = start.apex + dx/(x1-x0)
		}
	}
	b.apex = math.Max(0.05, math.Min(0.95, b.apex))
	if b != s.bend {
		s.bend = b
		a.selRender()
	}
}

// drawBend draws the curve through the handles and the handles.
func (a *App) drawBend() {
	m := a.img
	s := &a.sel
	x0, y0, x1, y1 := a.perspBase()
	b := &s.bend
	edge := func(fixed float64) {
		var px, py int
		for i := 0; i <= 40; i++ {
			t := float64(i) / 40
			var sx, sy int
			if b.vert {
				sx, sy = a.picFToScreen(fixed+b.shift(t), y0+(y1-y0)*t)
			} else {
				sx, sy = a.picFToScreen(x0+(x1-x0)*t, fixed+b.shift(t))
			}
			if i > 0 {
				m.Line(px, py, sx, sy, a.th.Accent)
			}
			px, py = sx, sy
		}
	}
	if b.vert {
		edge(x0)
		edge(x1)
	} else {
		edge(y0)
		edge(y1)
	}
	for _, p := range a.bendHandles() {
		r := gfx.Rect{X: p[0] - 5, Y: p[1] - 5, W: 11, H: 11}
		m.Fill(r, 0x000000)
		m.Fill(r.Inset(1), a.th.Accent)
		m.Fill(r.Inset(3), 0xFFFFFF)
	}
}
