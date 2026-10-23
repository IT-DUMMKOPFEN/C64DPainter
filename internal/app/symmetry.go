package app

import (
	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- Symmetry (mirror while painting) ------------------------------------------------
//
// Everything painted is also painted mirrored at a vertical axis (left ↔
// right), a horizontal axis (top ↔ bottom) or both (four-fold). The axes lie
// between two pixels; by default in the middle of the picture.

// SymSettings are saved with the settings.
type SymSettings struct {
	On   bool `json:"on"`
	Mode int  `json:"mode"` // 0 = left ↔ right, 1 = top ↔ bottom, 2 = both
	AX   int  `json:"axisX"`
	AY   int  `json:"axisY"`
}

var symModes = []string{"Left ↔ right", "Top ↔ bottom", "Both"}

func (s *SymSettings) clamp() {
	s.Mode = clampI(s.Mode, 0, 2)
	if s.AX <= 0 || s.AX >= c64.W {
		s.AX = c64.W / 2
	}
	if s.AY <= 0 || s.AY >= c64.H {
		s.AY = c64.H / 2
	}
}

// symActive reports whether painting is mirrored right now.
func (a *App) symActive() bool {
	if !a.set.Sym.On || a.noSym {
		return false
	}
	switch a.tool {
	case ToolSelect, ToolPicker, ToolHand, ToolCellSwap, ToolSmear:
		return false
	}
	return true
}

// symMirrors returns the mirrored positions of a pixel (without the pixel).
func (a *App) symMirrors(x, y int) []pt {
	s := &a.set.Sym
	s.clamp()
	mx, my := 2*s.AX-1-x, 2*s.AY-1-y
	switch s.Mode {
	case 1:
		return []pt{{x, my}}
	case 2:
		return []pt{{mx, y}, {x, my}, {mx, my}}
	}
	return []pt{{mx, y}}
}

func (a *App) toggleSymmetry() {
	a.set.Sym.On = !a.set.Sym.On
	a.set.Sym.clamp()
	a.saveSettings()
	if a.set.Sym.On {
		a.setStatus("Symmetry on: " + symModes[a.set.Sym.Mode] + " (Shift+X switches it off)")
	} else {
		a.setStatus("Symmetry off")
	}
}

// drawSymAxes shows the mirror axes on the canvas.
func (a *App) drawSymAxes() {
	if !a.set.Sym.On {
		return
	}
	s := &a.set.Sym
	s.clamp()
	m := a.img
	z := a.zoom
	ox, oy := a.origin()
	ph := int(a.now/120) % 8
	dash := func(i int) uint32 {
		if ((i+ph)/4)&1 == 0 {
			return 0xFF40C0
		}
		return 0xFFFFFF
	}
	if s.Mode != 1 {
		x := ox + s.AX*2*z
		for i, y := 0, oy; y < oy+c64.H*z; i, y = i+1, y+1 {
			m.Set(x, y, dash(i))
			m.Set(x-1, y, dash(i))
		}
	}
	if s.Mode != 0 {
		y := oy + s.AY*z
		for i, x := 0, ox; x < ox+c64.W*2*z; i, x = i+1, x+1 {
			m.Set(x, y, dash(i))
			m.Set(x, y-1, dash(i))
		}
	}
	a.needAnim = true
}

func (a *App) panelSymmetry(x, y, w int) int {
	u := &a.ui
	m := a.img
	s := &a.set.Sym
	s.clamp()
	hy := y
	y = a.section(x, y, w, "Symmetry")
	if u.Button("sym_off", gfx.Rect{X: x + w - 44, Y: hy + 3, W: 44, H: 18}, "Off") {
		a.toggleSymmetry()
		return y
	}
	if a.collapsed["Symmetry"] {
		return y
	}
	bw := (w - 8) / 3
	for i, n := range symModes {
		r := gfx.Rect{X: x + i*(bw+4), Y: y, W: bw, H: 22}
		if u.ButtonEx("sym_m"+itoa(i), r, []string{"↔", "↕", "↔ ↕"}[i], s.Mode == i, true) {
			s.Mode = i
			a.saveSettings()
		}
		u.Tip("sym_m"+itoa(i), r, "Mirror "+n)
	}
	y += 28
	half := (w - 4) / 2
	if s.Mode != 1 {
		m.Text(font.UI, x, y+4, "Axis X", a.th.Text)
		s.AX = u.Spinner("sym_ax", gfx.Rect{X: x + half - 40, Y: y, W: w - half + 40, H: 22}, s.AX, 1, c64.W-1, 1, "")
		y += 26
	}
	if s.Mode != 0 {
		m.Text(font.UI, x, y+4, "Axis Y", a.th.Text)
		s.AY = u.Spinner("sym_ay", gfx.Rect{X: x + half - 40, Y: y, W: w - half + 40, H: 22}, s.AY, 1, c64.H-1, 1, "")
		y += 26
	}
	if u.Button("sym_center", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Axis to the center") {
		s.AX, s.AY = c64.W/2, c64.H/2
	}
	y += 28
	for _, ln := range wrapText(font.UI, "Everything you paint is mirrored at the dashed axis. Shift+X switches symmetry on and off.", w) {
		m.Text(font.UI, x, y, ln, a.th.TextDim)
		y += font.UI.Height + 2
	}
	return y + 8
}

// --- Mirrored view (Shift+M) ---------------------------------------------------------
//
// Shows the picture mirrored to look at it with fresh eyes. Only the view
// changes; painting keeps working at the mirrored positions.

func (a *App) toggleMirrorView() {
	a.mirrorView = !a.mirrorView
	if a.mirrorView {
		a.setStatus("Mirrored view – the picture itself is not changed (Shift+M switches back)")
	} else {
		a.setStatus("Normal view")
	}
}

// mirrorX maps a screen x coordinate on the canvas to the mirrored view.
func (a *App) mirrorX(sx int) int {
	cr := a.rCanvas
	return cr.X + cr.Right() - 1 - sx
}

// flipCanvas mirrors the drawn canvas area.
func (a *App) flipCanvas() {
	cr := a.rCanvas.Intersect(gfx.Rect{X: 0, Y: 0, W: a.img.W, H: a.img.H})
	m := a.img
	for y := cr.Y; y < cr.Bottom(); y++ {
		row := m.Pix[y*m.W+cr.X : y*m.W+cr.Right()]
		for i, j := 0, len(row)-1; i < j; i, j = i+1, j-1 {
			row[i], row[j] = row[j], row[i]
		}
	}
	// a small note in the corner (drawn after the flip, so it is readable)
	s := "MIRRORED VIEW"
	w := font.UIB.Width(s) + 12
	r := gfx.Rect{X: cr.Right() - w - 8, Y: cr.Y + 8, W: w, H: 20}
	m.FillA(r, 0x000000, 160)
	m.TextC(font.UIB, r, s, 0xFFFFFF)
}

// --- Shade brush ---------------------------------------------------------------------
//
// Lightens (left button) or darkens (right button) the colors under the
// brush by one step. Each pixel changes only once per stroke.

// lumaOrder: the 16 C64 colors from dark to light.
var lumaOrder = []uint8{0, 6, 9, 11, 2, 4, 8, 12, 14, 5, 10, 3, 15, 7, 13, 1}

// shadeStep returns the color one step lighter (dir > 0) or darker.
func (a *App) shadeStep(c uint8, dir int) uint8 {
	seq := lumaOrder
	rs := shadeRamps()
	if r := a.set.ShadeRamp; r >= 0 && r < len(rs) {
		// ramp colors are light → dark; walk them dark → light
		cols := rs[r].cols
		seq = make([]uint8, len(cols))
		for i, v := range cols {
			seq[len(cols)-1-i] = v
		}
	}
	for i, v := range seq {
		if v == c {
			return seq[clampI(i+dir, 0, len(seq)-1)]
		}
	}
	return c
}

func (a *App) shadeDab(p *c64.Picture, x, y, btn int) {
	if len(a.shadeDone) != c64.W*c64.H {
		a.shadeDone = make([]bool, c64.W*c64.H)
	}
	dir := 1
	if btn == 1 {
		dir = -1
	}
	for _, o := range a.stamp().Offs {
		px, py := x+o.X, y+o.Y
		if px < 0 || py < 0 || px >= c64.W || py >= c64.H || a.shadeDone[py*c64.W+px] {
			continue
		}
		a.shadeDone[py*c64.W+px] = true
		c := p.ColorAt(px, py)
		if n := a.shadeStep(c, dir); n != c {
			a.plot(p, px, py, n)
		}
	}
}

func (a *App) panelShade(x, y, w int) int {
	u := &a.ui
	m := a.img
	y = a.section(x, y, w, "Shade")
	if a.collapsed["Shade"] {
		return y
	}
	m.Text(font.UI, x, y, "Steps (3, 4 or 5 colors)", a.th.Text)
	y += 18
	r := gfx.Rect{X: x, Y: y, W: w, H: 22}
	if u.ButtonEx("sh_all", r, "All colors by brightness", a.set.ShadeRamp < 0, true) {
		a.set.ShadeRamp = -1
	}
	y += 26
	a.drawRampSwatch(gfx.Rect{X: x, Y: y, W: w, H: 8}, lumaOrder)
	y += 14
	cw := (w - 4) / 2
	rs := shadeRamps()
	n3, n4 := len(groupRamps(0)), len(groupRamps(1))
	for i, rp := range rs {
		k := i
		switch {
		case i == n3:
			y += ((n3+1)/2)*26 + 2
			m.Text(font.UI, x, y, "4 colors", a.th.TextDim)
			y += 16
		case i == n3+n4:
			y += ((n4+1)/2)*26 + 2
			m.Text(font.UI, x, y, "5 colors", a.th.TextDim)
			y += 16
		}
		if i >= n3+n4 {
			k = i - n3 - n4
		} else if i >= n3 {
			k = i - n3
		}
		br := gfx.Rect{X: x + (k%2)*(cw+4), Y: y + (k/2)*26, W: cw, H: 24}
		if u.ButtonEx("sh_r"+itoa(i), br, "", a.set.ShadeRamp == i, true) {
			a.set.ShadeRamp = i
		}
		cols := rp.cols
		rev := make([]uint8, len(cols))
		for j, v := range cols {
			rev[len(cols)-1-j] = v
		}
		a.drawRampSwatch(gfx.Rect{X: br.X + 4, Y: br.Y + 4, W: br.W - 8, H: br.H - 8}, rev)
		u.Tip("sh_r"+itoa(i), br, rp.name)
	}
	y += ((len(groupRamps(2))+1)/2)*26 + 4
	for _, ln := range wrapText(font.UI, "Left button lightens, right button darkens by one step. Colors outside the chosen ramp stay as they are.", w) {
		m.Text(font.UI, x, y, ln, a.th.TextDim)
		y += font.UI.Height + 2
	}
	return y + 8
}
