package app

import (
	"math"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/gfx"
)

const (
	selNone = iota
	selMarking
	selSelected
	selFloating
)

const handleRotate = 8

// Selection ist der Auswahlrahmen bzw. die schwebende (verschiebbare) Auswahl.
type Selection struct {
	state          int
	x0, y0, x1, y1 int

	src    *CustomBrush
	under  *c64.Picture
	before *c64.Picture
	xf     xform
	out    *CustomBrush
	ox, oy int

	dragMode  int // 0 nichts, 1 markieren, 2 verschieben, 3 Griff
	handle    int
	dsx, dsy  int
	dpx, dpy  float64
	start     xform
	startBox  [4]float64
	stampCopy bool
	transpBG  bool
}

// xform ist die Transformation der schwebenden Auswahl (in Hires-Einheiten).
type xform struct {
	cx, cy   float64 // Mittelpunkt in Bildpunkten
	sx, sy   float64
	angle    float64 // Grad
	shx, shy float64
	flipX    bool
	flipY    bool
}

func identity(cx, cy float64) xform { return xform{cx: cx, cy: cy, sx: 1, sy: 1} }

func (t xform) matrix() (m00, m01, m10, m11 float64) {
	fx, fy := t.sx, t.sy
	if t.flipX {
		fx = -fx
	}
	if t.flipY {
		fy = -fy
	}
	// Sh·S
	a00, a01 := fx, t.shx*fy
	a10, a11 := t.shy*fx, fy
	r := t.angle * math.Pi / 180
	c, s := math.Cos(r), math.Sin(r)
	m00 = c*a00 - s*a10
	m01 = c*a01 - s*a11
	m10 = s*a00 + c*a10
	m11 = s*a01 + c*a11
	return
}

// bounds liefert den umschließenden Rahmen relativ zum Mittelpunkt (Hires-Einheiten).
func (t xform) bounds(w, h int) (minU, minV, maxU, maxV float64) {
	m00, m01, m10, m11 := t.matrix()
	hw, hh := float64(w), float64(h)/2
	minU, minV = math.Inf(1), math.Inf(1)
	maxU, maxV = math.Inf(-1), math.Inf(-1)
	for _, c := range [4][2]float64{{-hw, -hh}, {hw, -hh}, {hw, hh}, {-hw, hh}} {
		u := m00*c[0] + m01*c[1]
		v := m10*c[0] + m11*c[1]
		minU, maxU = math.Min(minU, u), math.Max(maxU, u)
		minV, maxV = math.Min(minV, v), math.Max(maxV, v)
	}
	return
}

// renderXform transformiert einen Pinsel. Ergebnis + linke obere Ecke in Bildpunkten.
func renderXform(src *CustomBrush, t xform) (*CustomBrush, int, int) {
	m00, m01, m10, m11 := t.matrix()
	det := m00*m11 - m01*m10
	if math.Abs(det) < 1e-9 {
		return &CustomBrush{W: 1, H: 1, Pix: []int8{-1}}, int(t.cx), int(t.cy)
	}
	i00, i01, i10, i11 := m11/det, -m01/det, -m10/det, m00/det
	minU, minV, maxU, maxV := t.bounds(src.W, src.H)
	X0 := int(math.Floor(t.cx + minU/2 + 1e-6))
	X1 := int(math.Ceil(t.cx + maxU/2 - 1e-6))
	Y0 := int(math.Floor(t.cy + minV + 1e-6))
	Y1 := int(math.Ceil(t.cy + maxV - 1e-6))
	w, h := X1-X0, Y1-Y0
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if w > 2000 || h > 2000 { // Schutz vor riesigen Skalierungen
		w, h = min(w, 2000), min(h, 2000)
	}
	out := &CustomBrush{W: w, H: h, Pix: make([]int8, w*h)}
	hw, hh := float64(src.W)/2, float64(src.H)/2
	for y := 0; y < h; y++ {
		V := float64(Y0+y) + 0.5 - t.cy
		for x := 0; x < w; x++ {
			U := (float64(X0+x) + 0.5 - t.cx) * 2
			u := i00*U + i01*V
			v := i10*U + i11*V
			si := int(math.Floor(u/2 + hw + 1e-9))
			sj := int(math.Floor(v + hh + 1e-9))
			c := int8(-1)
			if si >= 0 && sj >= 0 && si < src.W && sj < src.H {
				c = src.Pix[sj*src.W+si]
			}
			out.Pix[y*w+x] = c
		}
	}
	return out, X0, Y0
}

func (a *App) selRect() (int, int, int, int) {
	s := &a.sel
	x0, x1 := min(s.x0, s.x1), max(s.x0, s.x1)
	y0, y1 := min(s.y0, s.y1), max(s.y0, s.y1)
	return x0, y0, x1, y1
}

// grab kopiert einen Bildbereich als Pinsel.
func (a *App) grab(p *c64.Picture, x0, y0, x1, y1 int, transp bool) *CustomBrush {
	w, h := x1-x0+1, y1-y0+1
	b := &CustomBrush{W: w, H: h, Pix: make([]int8, w*h)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := p.ColorAt(x0+x, y0+y)
			if transp && c == p.BG {
				b.Pix[y*w+x] = -1
			} else {
				b.Pix[y*w+x] = int8(c)
			}
		}
	}
	return b
}

// selLift hebt die markierte Fläche an (copy = Original bleibt stehen).
func (a *App) selLift(copy bool) {
	s := &a.sel
	if s.state != selSelected {
		return
	}
	x0, y0, x1, y1 := a.selRect()
	s.src = a.grab(a.pic, x0, y0, x1, y1, s.transpBG)
	s.before = a.pic.Clone()
	s.under = a.pic.Clone()
	if !copy {
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				s.under.Bits[y*c64.W+x] = 0 // %00 = Hintergrund ist immer erlaubt
			}
		}
	}
	w, h := x1-x0+1, y1-y0+1
	s.xf = identity(float64(x0)+float64(w)/2, float64(y0)+float64(h)/2)
	s.state = selFloating
	a.selRender()
}

// selRender setzt die schwebende Auswahl (nach Regeln) auf das Bild.
func (a *App) selRender() {
	s := &a.sel
	if s.state != selFloating {
		return
	}
	s.out, s.ox, s.oy = renderXform(s.src, s.xf)
	*a.pic = *s.under
	for y := 0; y < s.out.H; y++ {
		for x := 0; x < s.out.W; x++ {
			c := s.out.Pix[y*s.out.W+x]
			if c >= 0 {
				a.plot(a.pic, s.ox+x, s.oy+y, uint8(c))
			}
		}
	}
}

func (a *App) selCommit() {
	s := &a.sel
	if s.state != selFloating {
		return
	}
	a.pushUndo(s.before)
	s.state = selNone
	s.src, s.under, s.before, s.out = nil, nil, nil, nil
	a.setStatus("Selection applied")
}

func (a *App) selCancel() {
	s := &a.sel
	if s.state == selFloating && s.before != nil {
		*a.pic = *s.before
	}
	s.state = selNone
	s.src, s.under, s.before, s.out = nil, nil, nil, nil
}

// box liefert den Rahmen der Auswahl in Bildpunkten (Gleitkomma).
func (a *App) selBox() (x0, y0, x1, y1 float64, ok bool) {
	s := &a.sel
	switch s.state {
	case selMarking, selSelected:
		ix0, iy0, ix1, iy1 := a.selRect()
		return float64(ix0), float64(iy0), float64(ix1 + 1), float64(iy1 + 1), true
	case selFloating:
		minU, minV, maxU, maxV := s.xf.bounds(s.src.W, s.src.H)
		return s.xf.cx + minU/2, s.xf.cy + minV, s.xf.cx + maxU/2, s.xf.cy + maxV, true
	}
	return 0, 0, 0, 0, false
}

func (a *App) picFToScreen(x, y float64) (int, int) {
	ox, oy := a.origin()
	return ox + int(math.Round(x*2*float64(a.zoom))), oy + int(math.Round(y*float64(a.zoom)))
}

func (a *App) screenToPicF(sx, sy int) (float64, float64) {
	ox, oy := a.origin()
	return float64(sx-ox) / float64(2*a.zoom), float64(sy-oy) / float64(a.zoom)
}

func (a *App) selScreenBox() (gfx.Rect, bool) {
	x0, y0, x1, y1, ok := a.selBox()
	if !ok {
		return gfx.Rect{}, false
	}
	sx0, sy0 := a.picFToScreen(x0, y0)
	sx1, sy1 := a.picFToScreen(x1, y1)
	return gfx.Rect{X: sx0, Y: sy0, W: sx1 - sx0, H: sy1 - sy0}, true
}

func (a *App) handleRects() [9]gfx.Rect {
	var hs [9]gfx.Rect
	r, ok := a.selScreenBox()
	if !ok {
		return hs
	}
	xs := [3]int{r.X, r.X + r.W/2, r.Right()}
	ys := [3]int{r.Y, r.Y + r.H/2, r.Bottom()}
	pos := [8][2]int{{0, 0}, {1, 0}, {2, 0}, {2, 1}, {2, 2}, {1, 2}, {0, 2}, {0, 1}}
	for i, p := range pos {
		hs[i] = gfx.Rect{X: xs[p[0]] - 4, Y: ys[p[1]] - 4, W: 9, H: 9}
	}
	hs[handleRotate] = gfx.Rect{X: xs[1] - 5, Y: r.Y - 26, W: 11, H: 11}
	return hs
}

func (a *App) selHandleAt(sx, sy int) int {
	if a.sel.state != selFloating && a.sel.state != selSelected {
		return -1
	}
	hs := a.handleRects()
	if hs[handleRotate].Inset(-2).Contains(sx, sy) {
		return handleRotate
	}
	// Innen (mehr als 3 Punkte vom Rand) wird verschoben, nicht skaliert
	if r, ok := a.selScreenBox(); ok && r.Inset(4).Contains(sx, sy) {
		return -1
	}
	small := a.selSmall()
	for i := 7; i >= 0; i-- {
		if small && i%2 == 1 {
			continue
		}
		if hs[i].Inset(-2).Contains(sx, sy) {
			return i
		}
	}
	return -1
}

// selSmall: bei kleinen Auswahlen nur Eckgriffe zeigen.
func (a *App) selSmall() bool {
	r, ok := a.selScreenBox()
	return ok && (r.W < 40 || r.H < 40)
}

func (a *App) selContainsScreen(sx, sy int) bool {
	r, ok := a.selScreenBox()
	return ok && r.Contains(sx, sy)
}

// selectInput steuert das Auswahlwerkzeug.
func (a *App) selectInput(px, py int, over bool) {
	in := &a.in
	s := &a.sel
	clampP := func(x, y int) (int, int) { return clampI(x, 0, c64.W-1), clampI(y, 0, c64.H-1) }
	if over && in.Pressed[1] && s.dragMode == 0 {
		// rechte Taste: Auswahl verwerfen
		if s.state == selFloating {
			a.selCommit()
		}
		s.state = selNone
		return
	}
	if over && in.Pressed[0] && s.dragMode == 0 {
		h := a.selHandleAt(in.MX, in.MY)
		switch {
		case h >= 0:
			if s.state == selSelected {
				a.selLift(false)
			}
			s.dragMode = 3
			s.handle = h
			s.start = s.xf
			x0, y0, x1, y1, _ := a.selBox()
			s.startBox = [4]float64{x0, y0, x1, y1}
			s.dpx, s.dpy = a.screenToPicF(in.MX, in.MY)
		case s.state != selNone && a.selContainsScreen(in.MX, in.MY):
			if s.state == selSelected {
				a.selLift(in.Mods.Ctrl)
			} else if in.Mods.Ctrl && s.state == selFloating {
				// Kopie stempeln: aktuelles Ergebnis bleibt im Bild
				s.under = a.pic.Clone()
			}
			s.dragMode = 2
			s.start = s.xf
			s.dsx, s.dsy = px, py
		default:
			if s.state == selFloating {
				a.selCommit()
			}
			x, y := clampP(px, py)
			s.state = selMarking
			s.x0, s.y0, s.x1, s.y1 = x, y, x, y
			s.dragMode = 1
		}
		return
	}
	if s.dragMode == 0 {
		return
	}
	if in.Down[0] {
		switch s.dragMode {
		case 1:
			s.x1, s.y1 = clampP(px, py)
		case 2:
			nx := s.start.cx + float64(px-s.dsx)
			ny := s.start.cy + float64(py-s.dsy)
			if nx != s.xf.cx || ny != s.xf.cy {
				s.xf.cx, s.xf.cy = nx, ny
				a.selRender()
			}
		case 3:
			a.selHandleDrag()
		}
		return
	}
	// losgelassen
	if s.dragMode == 1 {
		if s.x0 == s.x1 && s.y0 == s.y1 {
			s.state = selNone
		} else {
			s.state = selSelected
		}
	}
	s.dragMode = 0
}

func (a *App) selHandleDrag() {
	s := &a.sel
	mx, my := a.screenToPicF(a.in.MX, a.in.MY)
	if s.handle == handleRotate {
		cx, cy := s.start.cx, s.start.cy
		a0 := math.Atan2(s.dpy-cy, (s.dpx-cx)*2)
		a1 := math.Atan2(my-cy, (mx-cx)*2)
		ang := s.start.angle + (a1-a0)*180/math.Pi
		if a.in.Mods.Shift {
			ang = math.Round(ang/15) * 15
		}
		s.xf.angle = normAngle(ang)
		a.selRender()
		return
	}
	bx0, by0, bx1, by1 := s.startBox[0], s.startBox[1], s.startBox[2], s.startBox[3]
	w0, h0 := bx1-bx0, by1-by0
	nx0, ny0, nx1, ny1 := bx0, by0, bx1, by1
	hx := [8]int{-1, 0, 1, 1, 1, 0, -1, -1}[s.handle]
	hy := [8]int{-1, -1, -1, 0, 1, 1, 1, 0}[s.handle]
	if hx < 0 {
		nx0 = math.Min(math.Round(mx), bx1-1)
	} else if hx > 0 {
		nx1 = math.Max(math.Round(mx), bx0+1)
	}
	if hy < 0 {
		ny0 = math.Min(math.Round(my), by1-1)
	} else if hy > 0 {
		ny1 = math.Max(math.Round(my), by0+1)
	}
	fx, fy := (nx1-nx0)/w0, (ny1-ny0)/h0
	if a.in.Mods.Shift && hx != 0 && hy != 0 {
		f := math.Max(fx, fy)
		fx, fy = f, f
		if hx < 0 {
			nx0 = bx1 - w0*f
		} else {
			nx1 = bx0 + w0*f
		}
		if hy < 0 {
			ny0 = by1 - h0*f
		} else {
			ny1 = by0 + h0*f
		}
	}
	s.xf.sx = s.start.sx * fx
	s.xf.sy = s.start.sy * fy
	s.xf.cx = s.start.cx + ((nx0+nx1)/2 - (bx0+bx1)/2)
	s.xf.cy = s.start.cy + ((ny0+ny1)/2 - (by0+by1)/2)
	a.selRender()
}

func normAngle(v float64) float64 {
	for v > 180 {
		v -= 360
	}
	for v <= -180 {
		v += 360
	}
	return v
}

func (a *App) selMove(dx, dy int) {
	s := &a.sel
	switch s.state {
	case selSelected:
		a.selLift(false)
		fallthrough
	case selFloating:
		s.xf.cx += float64(dx)
		s.xf.cy += float64(dy)
		a.selRender()
	}
}

// --- schnelle Transformationen -------------------------------------------------

const (
	qFlipX = iota
	qFlipY
	qRot90
	qRot270
	qHalf
	qDouble
	qReset
)

func applyQuick(t *xform, op int) {
	switch op {
	case qFlipX:
		t.angle, t.shx, t.shy = -t.angle, -t.shx, -t.shy
		t.flipX = !t.flipX
	case qFlipY:
		t.angle, t.shx, t.shy = -t.angle, -t.shx, -t.shy
		t.flipY = !t.flipY
	case qRot90:
		t.angle = normAngle(t.angle + 90)
	case qRot270:
		t.angle = normAngle(t.angle - 90)
	case qHalf:
		t.sx, t.sy = t.sx/2, t.sy/2
	case qDouble:
		t.sx, t.sy = t.sx*2, t.sy*2
	case qReset:
		*t = identity(t.cx, t.cy)
	}
}

var quickNames = []string{"Flipped horizontally", "Flipped vertically", "Rotated 90°", "Rotated −90°", "Half size", "Double size", "Transformation reset"}

// transformQuick wirkt auf die Auswahl oder – ohne Auswahl – auf den eigenen Pinsel.
func (a *App) transformQuick(op int) {
	s := &a.sel
	if s.state == selSelected {
		a.selLift(false)
	}
	if s.state == selFloating {
		applyQuick(&s.xf, op)
		a.selRender()
		a.setStatus("Selection: " + quickNames[op])
		return
	}
	if a.brushShape == BrushCustom && a.custom != nil && op != qReset {
		t := identity(0, 0)
		applyQuick(&t, op)
		nb, _, _ := renderXform(a.custom, t)
		a.custom = trimBrush(nb)
		a.stampCache = nil
		a.setStatus("Brush: " + quickNames[op])
		return
	}
	a.setStatus("No selection and no custom brush active")
}

func trimBrush(b *CustomBrush) *CustomBrush {
	x0, y0, x1, y1 := b.W, b.H, -1, -1
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			if b.Pix[y*b.W+x] >= 0 {
				x0, y0 = min(x0, x), min(y0, y)
				x1, y1 = max(x1, x), max(y1, y)
			}
		}
	}
	if x1 < 0 {
		return &CustomBrush{W: 1, H: 1, Pix: []int8{-1}}
	}
	w, h := x1-x0+1, y1-y0+1
	n := &CustomBrush{W: w, H: h, Pix: make([]int8, w*h)}
	for y := 0; y < h; y++ {
		copy(n.Pix[y*w:y*w+w], b.Pix[(y0+y)*b.W+x0:(y0+y)*b.W+x0+w])
	}
	return n
}

// selEnsureFloating hebt bei Bedarf an (für Regler).
func (a *App) selEnsureFloating() bool {
	if a.sel.state == selSelected {
		a.selLift(false)
	}
	return a.sel.state == selFloating
}

// --- Zwischenablage ---------------------------------------------------------------

func (a *App) selCurrentBrush() *CustomBrush {
	s := &a.sel
	switch s.state {
	case selFloating:
		return trimBrush(s.out)
	case selSelected:
		x0, y0, x1, y1 := a.selRect()
		return a.grab(a.pic, x0, y0, x1, y1, s.transpBG)
	}
	return nil
}

func (a *App) selCopy() {
	if b := a.selCurrentBrush(); b != nil {
		a.clip = b
		a.setStatus("Copied (Ctrl+V pastes)")
	}
}

func (a *App) selCut() {
	if b := a.selCurrentBrush(); b != nil {
		a.clip = b
		a.selDelete()
		a.setStatus("Cut")
	}
}

func (a *App) selDelete() {
	s := &a.sel
	switch s.state {
	case selSelected:
		a.pushUndo(a.pic.Clone())
		x0, y0, x1, y1 := a.selRect()
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				a.pic.Bits[y*c64.W+x] = 0
			}
		}
		s.state = selNone
	case selFloating:
		*a.pic = *s.under
		a.pushUndo(s.before)
		s.state = selNone
	}
}

func (a *App) selPaste() {
	if a.clip == nil {
		a.setStatus("Clipboard is empty")
		return
	}
	a.commitOrCancelForHistory()
	a.setTool(ToolSelect)
	s := &a.sel
	s.src = a.clip.clone()
	s.before = a.pic.Clone()
	s.under = a.pic.Clone()
	cx, cy := float64(c64.W)/2, float64(c64.H)/2
	if a.mouseIn {
		cx, cy = float64(a.mpx), float64(a.mpy)
	}
	s.xf = identity(math.Floor(cx-float64(s.src.W)/2)+float64(s.src.W)/2, math.Floor(cy-float64(s.src.H)/2)+float64(s.src.H)/2)
	s.state = selFloating
	a.selRender()
	a.setStatus("Pasted – drag to place, Enter applies")
}

func (a *App) selectAll() {
	a.commitOrCancelForHistory()
	a.setTool(ToolSelect)
	a.sel.state = selSelected
	a.sel.x0, a.sel.y0, a.sel.x1, a.sel.y1 = 0, 0, c64.W-1, c64.H-1
}

// selToBrush macht aus der Auswahl einen Pinsel.
func (a *App) selToBrush() {
	b := a.selCurrentBrush()
	if b == nil {
		a.setStatus("Select an area first")
		return
	}
	if a.sel.state == selFloating {
		a.selCommit()
	}
	a.sel.state = selNone
	a.custom = b
	a.brushShape = BrushCustom
	a.stampCache = nil
	a.setTool(ToolDraw)
	a.setStatus("The selection is now the brush (Shift+B = standard brush)")
}

func (a *App) restoreBrush() {
	a.brushShape = BrushRound
	a.stampCache = nil
	a.setStatus("Standard brush")
}

// --- Zeichnen ------------------------------------------------------------------------

func (a *App) drawSelection() {
	s := &a.sel
	if s.state == selNone {
		return
	}
	r, ok := a.selScreenBox()
	if !ok {
		return
	}
	m := a.img
	phase := int(a.now/80) % 8
	dashedFrame(m, gfx.Rect{X: r.X - 1, Y: r.Y - 1, W: r.W + 2, H: r.H + 2}, 0xFFFFFF, 0x000000, phase)
	if s.state == selMarking {
		w := absI(s.x1-s.x0) + 1
		h := absI(s.y1-s.y0) + 1
		lbl := itoa(w) + " × " + itoa(h)
		a.drawCanvasLabel(r.Right()+6, r.Bottom()+4, lbl)
		return
	}
	hs := a.handleRects()
	th := a.th
	// Drehgriff
	cx := r.X + r.W/2
	for y := r.Y - 15; y < r.Y-1; y++ {
		m.Set(cx, y, 0xFFFFFF)
		m.Set(cx+1, y, 0x000000)
	}
	hr := hs[handleRotate]
	circle(m, hr.X+5, hr.Y+5, 5, 0x000000)
	circle(m, hr.X+5, hr.Y+5, 4, th.Accent)
	small := a.selSmall()
	for i := 0; i < 8; i++ {
		if small && i%2 == 1 {
			continue
		}
		h := hs[i]
		m.Fill(h, 0x000000)
		m.Fill(h.Inset(1), 0xFFFFFF)
	}
	if s.state == selFloating {
		lbl := itoa(s.out.W) + " × " + itoa(s.out.H)
		if s.xf.angle != 0 {
			lbl += "  " + itoa(int(math.Round(s.xf.angle))) + "°"
		}
		a.drawCanvasLabel(r.Right()+8, r.Bottom()+6, lbl)
	}
}

func (a *App) drawCanvasLabel(x, y int, s string) {
	m := a.img
	w := m.Text(fontUI(), -1000, -1000, s, 0) + 10
	r := gfx.Rect{X: x, Y: y, W: w, H: 18}
	m.FillA(r, 0x000000, 170)
	m.Text(fontUI(), x+5, y+2, s, 0xFFFFFF)
}
