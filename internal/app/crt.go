package app

import (
	"math"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- CRT preview window --------------------------------------------------------------
//
// A floating window that shows the picture with its border as on a TV:
// 100 % (384 × 272 screen pixels, the size of a real C64 picture), 200 % or
// 300 %, with or without the CRT look (scanlines, soft picture and the PAL
// color bleed). It does not block the program: you keep painting while it
// is open and it follows every change.

// CRTSettings are saved with the settings.
type CRTSettings struct {
	Scale int  `json:"scale"` // 1, 2 or 3
	On    bool `json:"on"`
	X     int  `json:"x"` // window position (0, 0 = not placed yet)
	Y     int  `json:"y"`
}

const crtW, crtH = 384, 272 // picture with border in screen pixels
const crtTitleH, crtBarH = 28, 32

type crtWindow struct {
	open     bool
	dragging bool
	grab     [2]int
	src      []uint32 // the source picture of the cached result
	key      [2]int   // scale, crt on
	out      []uint32 // rendered picture
}

func (a *App) toggleCRT() {
	a.crt.open = !a.crt.open
	if a.crt.open {
		a.setStatus("CRT preview opened (Shift+V closes it)")
	}
}

func (a *App) crtScale() int { return clampI(a.set.CRT.Scale, 1, 3) }

// crtRect returns the window rectangle.
func (a *App) crtRect() gfx.Rect {
	s := a.crtScale()
	w := min(crtW*s+2, a.W-8)
	h := min(crtH*s+crtTitleH+crtBarH+2, a.H-8)
	w = max(w, 360)
	x, y := a.set.CRT.X, a.set.CRT.Y
	if x == 0 && y == 0 {
		x, y = a.rCanvas.Right()-w-12, a.rCanvas.Y+12
	}
	x = clampI(x, 0, max(0, a.W-w))
	y = clampI(y, 0, max(0, a.H-h))
	return gfx.Rect{X: x, Y: y, W: w, H: h}
}

// crtHit reports whether the mouse is over the CRT window (it then gets
// the mouse instead of the canvas and panels below).
func (a *App) crtHit() bool {
	if !a.crt.open {
		return false
	}
	return a.crt.dragging || a.crtRect().Contains(a.in.MX, a.in.MY)
}

// crtSource renders the picture with its border at 384 × 272.
func (a *App) crtSource() []uint32 {
	src := make([]uint32, crtW*crtH)
	bc := a.pal.Colors[a.pic.Border]
	for i := range src {
		src[i] = bc
	}
	const bx, by = (crtW - 320) / 2, (crtH - 200) / 2
	for y := 0; y < c64.H; y++ {
		row := src[(by+y)*crtW+bx:]
		for x := 0; x < c64.W; x++ {
			c := a.pal.Colors[a.pic.ColorAt(x, y)]
			row[x*2] = c
			row[x*2+1] = c
		}
	}
	return src
}

func sameColors(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// crtRender builds the scaled picture, with or without the CRT look.
func crtRender(src []uint32, s int, on bool) []uint32 {
	W, H := crtW*s, crtH*s
	out := make([]uint32, W*H)
	if !on {
		for y := 0; y < H; y++ {
			for x := 0; x < W; x++ {
				out[y*W+x] = src[(y/s)*crtW+x/s]
			}
		}
		return out
	}
	// luma / chroma per source pixel; the chroma is blurred more (PAL)
	n := crtW * crtH
	Y, U, V := make([]float64, n), make([]float64, n), make([]float64, n)
	for i, c := range src {
		r, g, b := float64(c>>16&255), float64(c>>8&255), float64(c&255)
		Y[i] = 0.299*r + 0.587*g + 0.114*b
		U[i] = b - Y[i]
		V[i] = r - Y[i]
	}
	R, G, B := make([]float64, n), make([]float64, n), make([]float64, n)
	at := func(a []float64, y, x int) float64 { return a[y*crtW+clampI(x, 0, crtW-1)] }
	for y := 0; y < crtH; y++ {
		for x := 0; x < crtW; x++ {
			yy := 0.2*at(Y, y, x-1) + 0.6*at(Y, y, x) + 0.2*at(Y, y, x+1)
			u := (at(U, y, x-2) + 2*at(U, y, x-1) + 3*at(U, y, x) + 2*at(U, y, x+1) + at(U, y, x+2)) / 9
			v := (at(V, y, x-2) + 2*at(V, y, x-1) + 3*at(V, y, x) + 2*at(V, y, x+1) + at(V, y, x+2)) / 9
			i := y*crtW + x
			R[i] = yy + v
			B[i] = yy + u
			G[i] = (yy - 0.299*R[i] - 0.114*B[i]) / 0.587
		}
	}
	// scanline profile inside one source row
	prof := []float64{1.08}
	switch s {
	case 2:
		prof = []float64{1.12, 0.62}
	case 3:
		prof = []float64{1.05, 1.15, 0.55}
	}
	cl := func(v float64) uint32 { return uint32(math.Max(0, math.Min(255, v))) }
	for oy := 0; oy < H; oy++ {
		sy := oy / s
		f := prof[oy%s]
		vy := (float64(oy)+0.5)/float64(H)*2 - 1
		for ox := 0; ox < W; ox++ {
			// soft horizontal interpolation between source pixels
			fx := (float64(ox)+0.5)/float64(s) - 0.5
			x0 := int(math.Floor(fx))
			t := fx - float64(x0)
			i0 := sy*crtW + clampI(x0, 0, crtW-1)
			i1 := sy*crtW + clampI(x0+1, 0, crtW-1)
			vx := (float64(ox)+0.5)/float64(W)*2 - 1
			vig := f * (1 - 0.10*(vx*vx+vy*vy))
			r := (R[i0]*(1-t) + R[i1]*t) * vig
			g := (G[i0]*(1-t) + G[i1]*t) * vig
			b := (B[i0]*(1-t) + B[i1]*t) * vig
			out[oy*W+ox] = cl(r)<<16 | cl(g)<<8 | cl(b)
		}
	}
	return out
}

func (a *App) drawCRT() {
	if !a.crt.open {
		return
	}
	u := &a.ui
	m := a.img
	th := a.th
	in := &a.in
	r := a.crtRect()
	m.ResetClip()
	m.FillA(gfx.Rect{X: r.X + 6, Y: r.Y + 6, W: r.W, H: r.H}, th.Shadow, 90)
	m.Fill(r, th.Panel)
	m.Frame(r, th.PopupBorder)
	// title bar (drag to move)
	tb := gfx.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: crtTitleH}
	m.Fill(tb, th.MenuBar)
	m.TextL(font.UIB, gfx.Rect{X: tb.X + 10, Y: tb.Y, W: tb.W - 40, H: tb.H}, "CRT preview", th.MenuText)
	xr := gfx.Rect{X: tb.Right() - 28, Y: tb.Y + 2, W: 26, H: tb.H - 4}
	if u.hover(xr) {
		m.Fill(xr, th.Danger)
	}
	m.TextC(font.UIB, xr, "✕", th.MenuText)
	if c, _ := u.press("crt_x", xr, 0); c {
		a.crt.open = false
		return
	}
	drag := gfx.Rect{X: tb.X, Y: tb.Y, W: tb.W - 30, H: tb.H}
	if !a.crt.dragging && !u.blocked && drag.Contains(in.MX, in.MY) && in.Pressed[0] && u.active == "" {
		a.crt.dragging = true
		a.crt.grab = [2]int{in.MX - r.X, in.MY - r.Y}
		in.Pressed[0] = false
	}
	if a.crt.dragging {
		u.active = "crtdrag"
		if in.Down[0] {
			a.set.CRT.X = clampI(in.MX-a.crt.grab[0], 0, max(0, a.W-r.W))
			a.set.CRT.Y = clampI(in.MY-a.crt.grab[1], 0, max(0, a.H-r.H))
			if a.set.CRT.X == 0 && a.set.CRT.Y == 0 {
				a.set.CRT.Y = 1
			}
			r = a.crtRect()
		} else {
			a.crt.dragging = false
			u.active = ""
			a.saveSettings()
		}
	}
	// controls
	by := r.Y + 1 + crtTitleH + 4
	for i, s := range []int{1, 2, 3} {
		br := gfx.Rect{X: r.X + 8 + i*64, Y: by, W: 60, H: 22}
		if u.ButtonEx("crt_s"+itoa(s), br, itoa(s*100)+" %", a.crtScale() == s, true) && a.set.CRT.Scale != s {
			a.set.CRT.Scale = s
			a.saveSettings()
			return
		}
	}
	on := u.Checkbox("crt_on", gfx.Rect{X: r.X + 8 + 3*64 + 8, Y: by + 1, W: 70, H: 20}, "CRT", a.set.CRT.On)
	if on != a.set.CRT.On {
		a.set.CRT.On = on
		a.saveSettings()
	}
	// picture (cached until something changes)
	s := a.crtScale()
	src := a.crtSource()
	key := [2]int{s, 0}
	if a.set.CRT.On {
		key[1] = 1
	}
	if a.crt.out == nil || a.crt.key != key || !sameColors(src, a.crt.src) {
		a.crt.out = crtRender(src, s, a.set.CRT.On)
		a.crt.src = src
		a.crt.key = key
	}
	ir := gfx.Rect{X: r.X + 1, Y: r.Y + 1 + crtTitleH + crtBarH, W: r.W - 2, H: r.H - crtTitleH - crtBarH - 2}
	m.Fill(ir, 0x000000)
	W, H := crtW*s, crtH*s
	// center the picture; if the window is smaller, show the middle part
	dx, dy := ir.X+(ir.W-W)/2, ir.Y+(ir.H-H)/2
	vis := ir.Intersect(gfx.Rect{X: dx, Y: dy, W: W, H: H}).Intersect(gfx.Rect{X: 0, Y: 0, W: m.W, H: m.H})
	for y := vis.Y; y < vis.Bottom(); y++ {
		copy(m.Pix[y*m.W+vis.X:y*m.W+vis.Right()], a.crt.out[(y-dy)*W+vis.X-dx:])
	}
}
