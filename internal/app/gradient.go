package app

import (
	"math"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- Gradient fill -------------------------------------------------------------------
//
// Fills the connected area under the click (like Fill) with a color ramp.
// A single click starts the gradient at the click point (linear: in the
// chosen direction, circle/diamond: around the point) and reaches to the
// far edge of the area; dragging sets direction and width. The ramps are
// three-color C64 ramps (light → dark) and the two five-color versions made
// from them, so a gradient never uses more than three colors.

type gradientRamp struct {
	name  string
	light uint8
	mid   uint8
	dark  uint8
}

// gradientRamps: 7 basic and 8 extended C64 ramps, light → mid → dark.
var gradientRamps = []gradientRamp{
	{"Grey (contrast)", 1, 12, 0},
	{"Grey (fine)", 15, 12, 11},
	{"Blue", 3, 14, 6},
	{"Red", 10, 2, 9},
	{"Yellow/Brown", 7, 8, 9},
	{"Green", 13, 5, 0},
	{"Purple", 10, 4, 0},
	{"Copper/Gold", 7, 8, 2},
	{"Fire", 7, 2, 9},
	{"Ice/Sky", 1, 3, 6},
	{"Pastel blue", 15, 14, 6},
	{"Neon pink", 10, 4, 6},
	{"Forest/Earth", 13, 5, 9},
	{"Mono soft", 1, 15, 12},
	{"Dusk", 8, 2, 4},
}

const gradientBasic = 7 // the first 7 ramps are the basic ones

var gradientGroups = []string{"3 colors: light → dark", "5 colors: dark → light → dark", "5 colors: light → dark → light"}
var gradientShapes = []string{"Linear", "Circle", "Diamond"}

// direction presets for a single click with the linear shape (screen space)
var gradientDirs = []struct {
	dx, dy float64
	tip    string
}{
	{0, 1, "top to bottom"}, {0, -1, "bottom to top"},
	{1, 0, "left to right"}, {-1, 0, "right to left"},
	{1, 1, "from top left"}, {-1, 1, "from top right"},
	{1, -1, "from bottom left"}, {-1, -1, "from bottom right"},
}

// GradientSettings are saved with the settings.
type GradientSettings struct {
	Group   int  `json:"group"`
	Ramp    int  `json:"ramp"`
	Shape   int  `json:"shape"`
	Dir     int  `json:"dir"`
	Reverse bool `json:"reverse"`
	Dither  bool `json:"dither"`
}

func (g *GradientSettings) clamp() {
	g.Group = clampI(g.Group, 0, len(gradientGroups)-1)
	g.Ramp = clampI(g.Ramp, 0, len(gradientRamps)-1)
	g.Shape = clampI(g.Shape, 0, len(gradientShapes)-1)
	g.Dir = clampI(g.Dir, 0, len(gradientDirs)-1)
}

// colors returns the ramp colors of a group, in fill order.
func gradientColors(group, ramp int, reverse bool) []uint8 {
	r := gradientRamps[ramp]
	var c []uint8
	switch group {
	case 1:
		c = []uint8{r.dark, r.mid, r.light, r.mid, r.dark}
	case 2:
		c = []uint8{r.light, r.mid, r.dark, r.mid, r.light}
	default:
		c = []uint8{r.light, r.mid, r.dark}
	}
	if reverse {
		for i, j := 0, len(c)-1; i < j; i, j = i+1, j-1 {
			c[i], c[j] = c[j], c[i]
		}
	}
	return c
}

var gradBayer = [16]float64{0, 8, 2, 10, 12, 4, 14, 6, 3, 11, 1, 9, 15, 7, 13, 5}

// gradientColorAt maps a position t (0..1) to a ramp color. With dither the
// band borders get clean 25 / 50 / 75 % transitions.
func gradientColorAt(cols []uint8, t float64, dither bool, x, y int) uint8 {
	n := len(cols)
	t = math.Max(0, math.Min(1, t))
	if !dither {
		return cols[clampI(int(t*float64(n)), 0, n-1)]
	}
	v := t*float64(n) - 0.5
	i := int(math.Floor(v))
	if i < 0 {
		return cols[0]
	}
	if i >= n-1 {
		return cols[n-1]
	}
	f := v - float64(i)
	share := 0.0
	switch {
	case f < 0.25:
		return cols[i]
	case f > 0.75:
		return cols[i+1]
	case f < 0.42:
		share = 0.25
	case f < 0.58:
		share = 0.5
	default:
		share = 0.75
	}
	if (gradBayer[(y&3)*4+(x&3)]+0.5)/16 < share {
		return cols[i+1]
	}
	return cols[i]
}

// gradientRegion returns the connected area of the same (visible) color.
func gradientRegion(p *c64.Picture, x, y int) []int {
	if x < 0 || y < 0 || x >= c64.W || y >= c64.H {
		return nil
	}
	target := p.ColorAt(x, y)
	seen := make([]bool, c64.W*c64.H)
	stack := []int{y*c64.W + x}
	seen[y*c64.W+x] = true
	var region []int
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		region = append(region, i)
		cx, cy := i%c64.W, i/c64.W
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := cx+d[0], cy+d[1]
			if nx < 0 || ny < 0 || nx >= c64.W || ny >= c64.H {
				continue
			}
			j := ny*c64.W + nx
			if !seen[j] && p.ColorAt(nx, ny) == target {
				seen[j] = true
				stack = append(stack, j)
			}
		}
	}
	return region
}

// gradientFill paints the region. Coordinates are in screen units (a
// multicolor pixel is two units wide). drag = false: single click at s.
func (a *App) gradientFill(region []int, sx, sy, ex, ey float64, drag bool) {
	g := a.set.Gradient
	cols := gradientColors(g.Group, g.Ramp, g.Reverse)
	pos := func(i int) (float64, float64) {
		return float64(i%c64.W)*2 + 1, float64(i/c64.W) + 0.5
	}
	var ux, uy, length float64
	metric := func(px, py float64) float64 { // distance along the gradient
		dx, dy := px-sx, py-sy
		switch g.Shape {
		case 1:
			return math.Hypot(dx, dy)
		case 2:
			return math.Abs(dx) + math.Abs(dy)
		}
		return dx*ux + dy*uy
	}
	if g.Shape == 0 {
		if drag {
			ux, uy = ex-sx, ey-sy
		} else {
			ux, uy = gradientDirs[g.Dir].dx, gradientDirs[g.Dir].dy
		}
		if l := math.Hypot(ux, uy); l > 0 {
			ux, uy = ux/l, uy/l
		}
	}
	if drag {
		length = metric(ex, ey)
	} else {
		for _, i := range region {
			if m := metric(pos(i)); m > length {
				length = m
			}
		}
	}
	if length < 1 {
		length = 1
	}
	for _, i := range region {
		px, py := pos(i)
		t := metric(px, py) / length
		x, y := i%c64.W, i/c64.W
		a.plot(a.pic, x, y, gradientColorAt(cols, t, g.Dither, x, y))
	}
}

// gradientDrag holds the state while the gradient tool is used.
type gradientDrag struct {
	region   []int
	sx, sy   int // start (picture pixels)
	ex, ey   int // current end
	dragging bool
}

func gradPos(x, y int) (float64, float64) { return float64(x)*2 + 1, float64(y) + 0.5 }

// gradientPress starts a gradient fill at a picture pixel (single click result).
func (a *App) gradientPress(px, py int) {
	d := &a.drag
	d.base = a.pic.Clone()
	a.grad = gradientDrag{region: gradientRegion(d.base, px, py), sx: px, sy: py, ex: px, ey: py}
	a.gradientRedraw()
}

// gradientMove follows the mouse while the button is held.
func (a *App) gradientMove(px, py int) {
	g := &a.grad
	if a.drag.base == nil || (px == g.ex && py == g.ey) {
		return
	}
	g.ex, g.ey = px, py
	dx, dy := float64(px-g.sx)*2, float64(py-g.sy)
	g.dragging = math.Hypot(dx, dy) >= 3
	a.gradientRedraw()
}

func (a *App) gradientRedraw() {
	g := &a.grad
	*a.pic = *a.drag.base
	sx, sy := gradPos(g.sx, g.sy)
	ex, ey := gradPos(g.ex, g.ey)
	a.gradientFill(g.region, sx, sy, ex, ey, g.dragging)
}

// drawGradientLine shows start, direction and width while dragging.
func (a *App) drawGradientLine() {
	g := &a.grad
	if a.tool != ToolGradient || !a.drag.active || !g.dragging {
		return
	}
	z := a.zoom
	sx, sy := a.picToScreen(g.sx, g.sy)
	ex, ey := a.picToScreen(g.ex, g.ey)
	sx, sy, ex, ey = sx+z, sy+z/2, ex+z, ey+z/2
	m := a.img
	m.Line(sx+1, sy+1, ex+1, ey+1, 0x000000)
	m.Line(sx, sy, ex, ey, 0xFFFFFF)
	for _, p := range [][2]int{{sx, sy}, {ex, ey}} {
		r := gfx.Rect{X: p[0] - 3, Y: p[1] - 3, W: 7, H: 7}
		m.Fill(r, 0xFFFFFF)
		m.Frame(r, 0x000000)
	}
}

// --- window ------------------------------------------------------------------------

type gradientDialog struct {
	g GradientSettings
}

func (a *App) openGradientDialog() {
	a.commitOrCancelForHistory()
	a.menuOpen = -1
	a.set.Gradient.clamp()
	a.modal = &gradientDialog{g: a.set.Gradient}
}

func (d *gradientDialog) Title() string          { return "Gradient fill" }
func (d *gradientDialog) Size(a *App) (int, int) { return 760, 530 }

func (d *gradientDialog) Key(a *App, k KeyEvent) {
	if k.Key == KeyEnter {
		d.apply(a)
	}
}

func (d *gradientDialog) apply(a *App) {
	a.set.Gradient = d.g
	a.saveSettings()
	a.closeModal(false)
	if a.tool != ToolGradient {
		a.setTool(ToolGradient)
	}
	a.setStatus("Gradient: " + gradientRamps[d.g.Ramp].name + " – click into an area, or drag for direction and width")
}

// drawRampSwatch draws the colors of a ramp as a strip.
func (a *App) drawRampSwatch(r gfx.Rect, cols []uint8) {
	n := len(cols)
	for i, c := range cols {
		x0 := r.X + r.W*i/n
		x1 := r.X + r.W*(i+1)/n
		a.img.Fill(gfx.Rect{X: x0, Y: r.Y, W: x1 - x0, H: r.H}, a.pal.Colors[c])
	}
	a.img.Frame(r, a.th.BtnBorder)
}

func (d *gradientDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	y := r.Y
	// tabs: the three ramp versions
	tw := (r.W - 8) / 3
	for i, name := range gradientGroups {
		if u.ButtonEx("gr_tab"+itoa(i), gfx.Rect{X: r.X + i*(tw+4), Y: y, W: tw, H: 26}, name, d.g.Group == i, true) {
			d.g.Group = i
		}
	}
	y += 36
	// ramps: three columns
	cols := 3
	cw := (r.W - 2*12) / cols
	for i, rp := range gradientRamps {
		col, row := i%cols, i/cols
		br := gfx.Rect{X: r.X + col*(cw+12), Y: y + row*44, W: cw, H: 40}
		sel := d.g.Ramp == i
		if sel {
			m.Fill(br, gfx.Mix(th.PanelAlt, th.Accent, 70))
			m.Frame(br, th.Accent)
			m.Frame(br.Inset(1), th.Accent)
		} else if u.hover(br) {
			m.Fill(br, th.BtnHover)
		}
		name := rp.name
		if i >= gradientBasic {
			name += "  ·  extended"
		}
		m.Text(font.UI, br.X+6, br.Y+3, name, th.Text)
		a.drawRampSwatch(gfx.Rect{X: br.X + 6, Y: br.Y + 20, W: br.W - 12, H: 14}, gradientColors(d.g.Group, i, d.g.Reverse))
		if c, _ := u.press("gr_r"+itoa(i), br, 0); c {
			d.g.Ramp = i
		}
	}
	y += ((len(gradientRamps)+cols-1)/cols)*44 + 10
	// shape and options (left), preview (right)
	lx := r.X
	lw := 360
	m.Text(font.UIB, lx, y+4, "Shape", th.Text)
	sw := (lw - 80 - 8) / 3
	for i, s := range gradientShapes {
		if u.ButtonEx("gr_s"+itoa(i), gfx.Rect{X: lx + 80 + i*(sw+4), Y: y, W: sw, H: 24}, s, d.g.Shape == i, true) {
			d.g.Shape = i
		}
	}
	y += 32
	m.Text(font.UIB, lx, y+4, "Direction", th.Text)
	dw := (lw - 80 - 7*3) / 8
	for i, dir := range gradientDirs {
		br := gfx.Rect{X: lx + 80 + i*(dw+3), Y: y, W: dw, H: 24}
		if u.ButtonEx("gr_d"+itoa(i), br, "", d.g.Dir == i, d.g.Shape == 0) {
			d.g.Dir = i
		}
		ac := th.Text
		if d.g.Dir == i {
			ac = th.TextInv
		}
		if d.g.Shape != 0 {
			ac = th.TextDim
		}
		drawArrow(m, br.X+br.W/2, br.Y+br.H/2, dir.dx, dir.dy, ac)
		u.Tip("gr_d"+itoa(i), br, "Single click: "+dir.tip+" (dragging sets the direction freely)")
	}
	y += 32
	d.g.Reverse = u.Checkbox("gr_rev", gfx.Rect{X: lx, Y: y, W: lw, H: 20}, "Reverse colors", d.g.Reverse)
	y += 24
	d.g.Dither = u.Checkbox("gr_dit", gfx.Rect{X: lx, Y: y, W: lw, H: 20}, "Dithered transitions (25 / 50 / 75 %)", d.g.Dither)
	y += 28
	for _, ln := range wrapText(font.UI, "Click into an area: the gradient starts at the click point. Click and drag: the drag sets direction and width.", lw) {
		m.Text(font.UI, lx, y, ln, th.TextDim)
		y += font.UI.Height + 2
	}
	// preview
	pr := gfx.Rect{X: r.X + lw + 24, Y: r.Y + 36 + ((len(gradientRamps)+cols-1)/cols)*44 + 10, W: r.W - lw - 24, H: 150}
	d.drawPreview(a, pr)
	by := r.Bottom() - 28
	if u.Button("gr_cancel", gfx.Rect{X: r.Right() - 250, Y: by, W: 120, H: 28}, "Cancel") {
		a.closeModal(true)
		return
	}
	if u.ButtonEx("gr_ok", gfx.Rect{X: r.Right() - 120, Y: by, W: 120, H: 28}, "OK", true, true) {
		d.apply(a)
	}
}

// drawPreview shows the gradient with the current settings on a sample area.
func (d *gradientDialog) drawPreview(a *App, r gfx.Rect) {
	m := a.img
	cols := gradientColors(d.g.Group, d.g.Ramp, d.g.Reverse)
	w, h := r.W/4, r.H/2 // multicolor pixels of 4×2 screen pixels
	var sx, sy float64
	var ux, uy float64
	switch d.g.Shape {
	case 0:
		ux, uy = gradientDirs[d.g.Dir].dx, gradientDirs[d.g.Dir].dy
		l := math.Hypot(ux, uy)
		ux, uy = ux/l, uy/l
		// start at the corner the direction comes from
		if ux < 0 {
			sx = float64(w * 2)
		}
		if uy < 0 {
			sy = float64(h)
		}
	default:
		sx, sy = float64(w), float64(h)/2
	}
	maxm := 0.0
	metric := func(px, py float64) float64 {
		dx, dy := px-sx, py-sy
		switch d.g.Shape {
		case 1:
			return math.Hypot(dx, dy)
		case 2:
			return math.Abs(dx) + math.Abs(dy)
		}
		return dx*ux + dy*uy
	}
	for _, c := range [][2]float64{{0, 0}, {float64(w * 2), 0}, {0, float64(h)}, {float64(w * 2), float64(h)}} {
		maxm = math.Max(maxm, metric(c[0], c[1]))
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			t := metric(float64(x)*2+1, float64(y)+0.5) / math.Max(1, maxm)
			c := gradientColorAt(cols, t, d.g.Dither, x, y)
			m.Fill(gfx.Rect{X: r.X + x*4, Y: r.Y + y*2, W: 4, H: 2}, a.pal.Colors[c])
		}
	}
	m.Frame(gfx.Rect{X: r.X - 1, Y: r.Y - 1, W: w*4 + 2, H: h*2 + 2}, a.th.Border)
}

// drawArrow draws a small arrow (about 13 pixels) pointing in direction dx, dy.
func drawArrow(m *gfx.Image, cx, cy int, dx, dy float64, c uint32) {
	l := math.Hypot(dx, dy)
	dx, dy = dx/l, dy/l
	x0, y0 := cx-int(math.Round(dx*6)), cy-int(math.Round(dy*6))
	x1, y1 := cx+int(math.Round(dx*6)), cy+int(math.Round(dy*6))
	for o := 0; o < 2; o++ { // two pixels thick
		ox, oy := 0, 0
		if math.Abs(dx) >= math.Abs(dy) {
			oy = o
		} else {
			ox = o
		}
		m.Line(x0+ox, y0+oy, x1+ox, y1+oy, c)
		for _, s := range []float64{1, -1} {
			hx := -dx*5 + s*-dy*4
			hy := -dy*5 + s*dx*4
			m.Line(x1+ox, y1+oy, x1+ox+int(math.Round(hx)), y1+oy+int(math.Round(hy)), c)
		}
	}
}

// --- side panel ----------------------------------------------------------------------

func (a *App) panelGradient(x, y, w int) int {
	u := &a.ui
	m := a.img
	g := &a.set.Gradient
	g.clamp()
	y = a.section(x, y, w, "Gradient fill")
	m.Text(font.UIB, x, y, gradientRamps[g.Ramp].name, a.th.Text)
	m.Text(font.UI, x, y+16, gradientGroups[g.Group], a.th.TextDim)
	y += 36
	a.drawRampSwatch(gfx.Rect{X: x, Y: y, W: w, H: 18}, gradientColors(g.Group, g.Ramp, g.Reverse))
	y += 26
	sw := (w - 8) / 3
	for i, s := range gradientShapes {
		if u.ButtonEx("gp_s"+itoa(i), gfx.Rect{X: x + i*(sw+4), Y: y, W: sw, H: 22}, s, g.Shape == i, true) {
			g.Shape = i
		}
	}
	y += 28
	g.Reverse = u.Checkbox("gp_rev", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Reverse colors", g.Reverse)
	y += 24
	g.Dither = u.Checkbox("gp_dit", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Dithered transitions", g.Dither)
	y += 26
	if u.Button("gp_open", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Choose gradient …") {
		a.later(a.openGradientDialog)
	}
	y += 30
	for _, ln := range wrapText(font.UI, "Click into an area to fill it from that point. Click and drag to set direction and width.", w) {
		m.Text(font.UI, x, y, ln, a.th.TextDim)
		y += font.UI.Height + 2
	}
	return y + 8
}
