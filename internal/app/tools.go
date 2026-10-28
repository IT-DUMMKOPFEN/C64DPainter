package app

import (
	"math"

	"c64dpainter/internal/c64"
)

// canvasInput verarbeitet Maus-Eingaben auf der Zeichenfläche.
func (a *App) canvasInput() {
	in := &a.in
	if a.mirrorView {
		mx := in.MX
		in.MX = a.mirrorX(mx)
		defer func() { in.MX = mx }()
	}
	cr := a.rCanvas
	over := cr.Contains(in.MX, in.MY)
	px, py := a.screenToPic(in.MX, in.MY)
	a.mouseIn = over && px >= 0 && py >= 0 && px < picW && py < c64.H
	if a.mouseIn {
		a.mpx, a.mpy = px, py
	}
	// Mausrad: Zoom (Umschalt = waagerecht verschieben)
	if over && in.Wheel != 0 {
		if in.Mods.Shift {
			a.panX += sign(in.Wheel) * 64
		} else if in.Mods.Ctrl {
			a.panY += sign(in.Wheel) * 64
		} else {
			a.zoomStep(sign(in.Wheel), in.MX, in.MY)
		}
		a.clampPan()
	}
	// Verschieben der Ansicht
	if !a.panning && over && !a.drag.active && (in.Pressed[2] || (in.Pressed[0] && (in.Space || a.tool == ToolHand))) {
		a.panning = true
		a.panSX, a.panSY, a.panOX, a.panOY = in.MX, in.MY, a.panX, a.panY
		in.Pressed[0] = false
	}
	if a.panning {
		a.panX = a.panOX + in.MX - a.panSX
		a.panY = a.panOY + in.MY - a.panSY
		a.clampPan()
		if !in.Down[0] && !in.Down[2] {
			a.panning = false
		}
		return
	}
	// Pipette mit Alt
	if over && in.Mods.Alt && (in.Pressed[0] || in.Pressed[1]) && !a.drag.active {
		a.pickColor(px, py, btnOf(in))
		return
	}
	a.toolInput(px, py, over)
}

func btnOf(in *Input) int {
	if in.Pressed[1] || (in.Down[1] && !in.Down[0]) {
		return 1
	}
	return 0
}

func (a *App) pickColor(px, py, btn int) {
	if px < 0 || py < 0 || px >= picW || py >= c64.H {
		return
	}
	c := a.pic.ColorAt(px, py)
	if btn == 1 {
		a.fg2 = c
		a.setStatus("Color 2: " + c64.ColorNames[c])
	} else {
		a.fg = c
		a.setStatus("Color 1: " + c64.ColorNames[c])
	}
}

func (a *App) toolInput(px, py int, over bool) {
	in := &a.in
	d := &a.drag
	// Kurve, Stufe 2: Biegen und Klick zum Festlegen
	if a.curve.stage == 2 {
		if in.Moved || in.Pressed[0] || in.Pressed[1] {
			*a.pic = *a.curve.base
			a.drawCurve(a.pic, a.curve.a, a.curve.b, pt{px, py}, a.curve.btn)
		}
		if over && (in.Pressed[0] || in.Pressed[1]) {
			a.curveCommit()
			in.Pressed = [3]bool{}
		}
		return
	}
	if a.tool == ToolSelect {
		a.selectInput(px, py, over)
		return
	}
	if a.tool == ToolCellSwap {
		a.cellSwapInput(px, py, over)
		return
	}
	pressed := over && (in.Pressed[0] || in.Pressed[1]) && !d.active
	if pressed {
		btn := btnOf(in)
		if a.tool == ToolPicker {
			a.pickColor(px, py, btn)
			a.tool = a.prevTool
			return
		}
		d.active = true
		d.btn = btn
		d.sx, d.sy, d.lx, d.ly = px, py, px, py
		d.moved = false
		d.straight = false
		d.lineBase = nil
		switch a.tool {
		case ToolDraw, ToolDots, ToolSpray, ToolDither, ToolErase, ToolShade:
			a.shadeDone = nil
			if a.tool == ToolDither {
				d.ditherSrc = a.ditherResult()
			}
			a.pushUndo(a.pic.Clone())
			d.base = nil
			if a.tool == ToolSpray {
				a.spray(px, py)
			} else {
				a.dab(a.pic, px, py, btn)
			}
		case ToolSmear:
			a.pushUndo(a.pic.Clone())
			d.base = nil
			a.smearStart(px, py)
		case ToolGradient:
			a.gradientPress(px, py)
		case ToolFill:
			snap := a.pic.Clone()
			if a.floodFill(px, py, btn) {
				a.pushUndo(snap)
			}
			d.active = false
		default:
			d.base = a.pic.Clone()
			a.shapePreview(px, py)
		}
		return
	}
	if !d.active {
		return
	}
	if in.Down[d.btn] {
		if a.tool == ToolDraw || a.tool == ToolDots || a.tool == ToolDither || a.tool == ToolErase || a.tool == ToolShade {
			if a.straightLine(px, py) {
				return
			}
		}
		if px != d.lx || py != d.ly {
			d.moved = true
			switch a.tool {
			case ToolDraw, ToolDither, ToolErase, ToolShade:
				a.lineDabs(a.pic, d.lx, d.ly, px, py, d.btn)
			case ToolDots:
				a.dab(a.pic, px, py, d.btn)
			case ToolSmear:
				forLine(d.lx, d.ly, px, py, func(x, y int) {
					if x != d.lx || y != d.ly {
						a.smearStep(x, y)
					}
				})
			case ToolSpray:
			case ToolGradient:
				a.gradientMove(px, py)
			default:
				a.shapePreview(px, py)
			}
			d.lx, d.ly = px, py
		}
		if a.tool == ToolSpray {
			a.spray(px, py)
		}
		if (a.tool == ToolRect || a.tool == ToolRectF || a.tool == ToolLine || isShapeTool(a.tool)) && (in.Mods.Shift || isShapeTool(a.tool)) {
			a.shapePreview(px, py)
		}
		return
	}
	// losgelassen
	switch a.tool {
	case ToolDraw, ToolDots, ToolSpray, ToolErase:
	case ToolShade:
		a.shadeDone = nil
	case ToolDither:
		d.ditherSrc = nil
	case ToolSmear:
		a.smearBuf = nil
	case ToolGradient:
		a.gradientMove(px, py)
		a.pushUndo(d.base)
		a.grad = gradientDrag{}
	case ToolCurve:
		if d.sx == px && d.sy == py {
			*a.pic = *d.base
			a.dab(a.pic, px, py, d.btn)
			a.pushUndo(d.base)
		} else {
			a.curve = curveState{stage: 2, a: pt{d.sx, d.sy}, b: pt{px, py}, btn: d.btn, base: d.base}
		}
	default:
		a.shapePreview(px, py)
		a.pushUndo(d.base)
	}
	d.active = false
	d.base = nil
}

func (a *App) curveCommit() {
	if a.curve.stage == 0 {
		return
	}
	a.pushUndo(a.curve.base)
	a.curve.stage = 0
	a.curve.base = nil
}

// shapePreview zeichnet die aktuelle Form neu auf die Grundlage.
func (a *App) shapePreview(px, py int) {
	d := &a.drag
	if d.base == nil {
		return
	}
	*a.pic = *d.base
	x0, y0 := d.sx, d.sy
	x1, y1 := px, py
	shift := a.in.Mods.Shift
	switch a.tool {
	case ToolLine, ToolCurve:
		if shift {
			x1, y1 = constrain45(x0, y0, x1, y1)
		}
		a.lineDabs(a.pic, x0, y0, x1, y1, d.btn)
	case ToolRect, ToolRectF:
		if shift {
			x1, y1 = constrainSquare(x0, y0, x1, y1)
		}
		if a.tool == ToolRectF {
			a.fillRect(x0, y0, x1, y1, d.btn)
		} else {
			a.lineDabs(a.pic, x0, y0, x1, y0, d.btn)
			a.lineDabs(a.pic, x1, y0, x1, y1, d.btn)
			a.lineDabs(a.pic, x1, y1, x0, y1, d.btn)
			a.lineDabs(a.pic, x0, y1, x0, y0, d.btn)
		}
	case ToolCircle, ToolCircleF:
		dxh := float64(x1-x0) * float64(asp)
		dy := float64(y1 - y0)
		r := math.Sqrt(dxh*dxh + dy*dy)
		a.ellipse(x0, y0, r/float64(asp), r, a.tool == ToolCircleF, d.btn)
	case ToolEllipse, ToolEllipseF:
		rx := math.Abs(float64(x1 - x0))
		ry := math.Abs(float64(y1 - y0))
		if shift {
			// Umschalt: Kreis am Bildschirm
			m := math.Max(rx*float64(asp), ry)
			rx, ry = m/float64(asp), m
		}
		a.ellipse(x0, y0, rx, ry, a.tool == ToolEllipseF, d.btn)
	default:
		if isShapeTool(a.tool) {
			a.drawShape(x0, y0, x1, y1, d.btn, shift)
		}
	}
}

func constrain45(x0, y0, x1, y1 int) (int, int) {
	dx := float64(x1-x0) * float64(asp)
	dy := float64(y1 - y0)
	ax, ay := math.Abs(dx), math.Abs(dy)
	switch {
	case ay < ax*0.414:
		return x1, y0
	case ax < ay*0.414:
		return x0, y1
	default:
		m := math.Max(ax, ay)
		sx, sy := 1.0, 1.0
		if dx < 0 {
			sx = -1
		}
		if dy < 0 {
			sy = -1
		}
		return x0 + int(math.Round(sx*m/float64(asp))), y0 + int(math.Round(sy*m))
	}
}

func constrainSquare(x0, y0, x1, y1 int) (int, int) {
	dx := float64(x1-x0) * float64(asp)
	dy := float64(y1 - y0)
	m := math.Max(math.Abs(dx), math.Abs(dy))
	sx, sy := 1.0, 1.0
	if dx < 0 {
		sx = -1
	}
	if dy < 0 {
		sy = -1
	}
	return x0 + int(math.Round(sx*m/float64(asp))), y0 + int(math.Round(sy*m))
}

func (a *App) fillRect(x0, y0, x1, y1, btn int) {
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			a.plotPat(a.pic, x, y, btn)
		}
	}
}

// ellipse zeichnet eine Ellipse um (cx,cy) mit Radien in Bildpunkten.
func (a *App) ellipse(cx, cy int, rx, ry float64, filled bool, btn int) {
	if rx < 0.5 && ry < 0.5 {
		a.dab(a.pic, cx, cy, btn)
		return
	}
	ex, ey := rx+0.5, ry+0.5
	inside := func(x, y int) bool {
		fx := float64(x-cx) / ex
		fy := float64(y-cy) / ey
		return fx*fx+fy*fy <= 1.0
	}
	irx, iry := int(math.Ceil(rx))+1, int(math.Ceil(ry))+1
	for y := cy - iry; y <= cy+iry; y++ {
		for x := cx - irx; x <= cx+irx; x++ {
			if !inside(x, y) {
				continue
			}
			if filled {
				a.plotPat(a.pic, x, y, btn)
				continue
			}
			if !inside(x-1, y) || !inside(x+1, y) || !inside(x, y-1) || !inside(x, y+1) {
				a.dab(a.pic, x, y, btn)
			}
		}
	}
}

// drawCurve zeichnet eine quadratische Bézierkurve durch m.
func (a *App) drawCurve(p *c64.Picture, A, B, M pt, btn int) {
	cx := 2*float64(M.X) - float64(A.X+B.X)/2
	cy := 2*float64(M.Y) - float64(A.Y+B.Y)/2
	l := math.Hypot(float64(A.X-M.X)*float64(asp), float64(A.Y-M.Y)) + math.Hypot(float64(B.X-M.X)*float64(asp), float64(B.Y-M.Y))
	n := int(l/3) + 8
	lx, ly := A.X, A.Y
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		u := 1 - t
		x := u*u*float64(A.X) + 2*u*t*cx + t*t*float64(B.X)
		y := u*u*float64(A.Y) + 2*u*t*cy + t*t*float64(B.Y)
		ix, iy := int(math.Round(x)), int(math.Round(y))
		if i == 1 {
			a.dab(p, lx, ly, btn)
		}
		if ix != lx || iy != ly {
			a.lineDabs(p, lx, ly, ix, iy, btn)
			lx, ly = ix, iy
		}
	}
}

// floodFill füllt die zusammenhängende Fläche gleicher Farbe.
func (a *App) floodFill(x, y, btn int) bool {
	p := a.pic
	if x < 0 || y < 0 || x >= picW || y >= c64.H {
		return false
	}
	target := p.ColorAt(x, y)
	if target == a.colorFor(btn) && a.pattern == 0 {
		return false
	}
	seen := make([]bool, picW*c64.H)
	stack := []int{y*picW + x}
	seen[y*picW+x] = true
	var region []int
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		region = append(region, i)
		cx, cy := i%picW, i/picW
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := cx+d[0], cy+d[1]
			if nx < 0 || ny < 0 || nx >= picW || ny >= c64.H {
				continue
			}
			j := ny*picW + nx
			if !seen[j] && p.ColorAt(nx, ny) == target {
				seen[j] = true
				stack = append(stack, j)
			}
		}
	}
	for _, i := range region {
		px, py := i%picW, i/picW
		if patBit(a.pattern, px, py) {
			a.plot(p, px, py, a.colorFor(btn))
		}
	}
	return true
}

// spray sprüht Pinselabdrücke in einen Kreis (am Bildschirm rund).
func (a *App) spray(x, y int) {
	if a.now-a.lastSpray < 30 && a.lastSpray != 0 {
		return
	}
	a.lastSpray = a.now
	R := float64(a.sprayRadius)
	for i := 0; i < a.sprayDensity; i++ {
		var u, v float64
		for k := 0; k < 8; k++ {
			u = float64(a.rand()%2001)/1000 - 1
			v = float64(a.rand()%2001)/1000 - 1
			if u*u+v*v <= 1 {
				break
			}
		}
		px := x + int(math.Round(u*R/float64(asp)))
		py := y + int(math.Round(v*R))
		a.dab(a.pic, px, py, a.drag.btn)
	}
}

// cellSwapInput: one undo step per stroke, every cell changed once per stroke.
func (a *App) cellSwapInput(px, py int, over bool) {
	in := &a.in
	d := &a.drag
	if over && (in.Pressed[0] || in.Pressed[1]) && !d.active {
		d.active = true
		d.btn = btnOf(in)
		d.base = a.pic.Clone()
		d.lx, d.ly = px, py
		a.swapDone = map[int]bool{}
		a.swapAt(px, py, d.btn)
		return
	}
	if !d.active {
		return
	}
	if in.Down[d.btn] {
		if px != d.lx || py != d.ly {
			forLine(d.lx, d.ly, px, py, func(x, y int) { a.swapAt(x, y, d.btn) })
			d.lx, d.ly = px, py
		}
		return
	}
	if *a.pic != *d.base {
		a.pushUndo(d.base)
	}
	d.active = false
	d.base = nil
}

// straightLine: while Ctrl is held during a freehand or dots stroke, a
// straight line is drawn from the point where Ctrl was pressed (or where the
// stroke began) to the mouse. Shift additionally snaps it to 45°. Releasing
// Ctrl keeps the line and continues painting freehand from its end.
func (a *App) straightLine(px, py int) bool {
	d := &a.drag
	if !a.in.Mods.Ctrl {
		if d.straight {
			d.straight = false
			d.lineBase = nil
		}
		return false
	}
	if !d.straight {
		d.straight = true
		d.lineBase = a.pic.Clone()
		d.lsx, d.lsy = d.lx, d.ly
	}
	ex, ey := px, py
	if a.in.Mods.Shift {
		ex, ey = constrain45(d.lsx, d.lsy, px, py)
	}
	*a.pic = *d.lineBase
	a.lineDabs(a.pic, d.lsx, d.lsy, ex, ey, d.btn)
	d.lx, d.ly = ex, ey
	d.moved = true
	return true
}
