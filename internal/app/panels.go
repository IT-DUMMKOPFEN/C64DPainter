package app

import (
	"fmt"
	"math"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- Werkzeugkasten (rechts, im Stil von Deluxe Paint) -----------------------------

type toolBtn struct {
	id         string
	icon, alt  string // alt = gefüllte Variante (geteilter Knopf)
	tool, alt2 int
	tip        string
	action     func()
	active     func() bool
	right      func()
}

func (a *App) toolButtons() []toolBtn {
	return []toolBtn{
		{id: "t_dots", icon: "dots", tool: ToolDots, alt2: -1, tip: "Dots (S)"},
		{id: "t_draw", icon: "draw", tool: ToolDraw, alt2: -1, tip: "Freehand (D)"},
		{id: "t_line", icon: "line", tool: ToolLine, alt2: -1, tip: "Line (V)"},
		{id: "t_curve", icon: "curve", tool: ToolCurve, alt2: -1, tip: "Curve (Q)"},
		{id: "t_fill", icon: "fill", tool: ToolFill, alt2: -1, tip: "Fill (F)"},
		{id: "t_spray", icon: "spray", tool: ToolSpray, alt2: -1, tip: "Airbrush (A)"},
		{id: "t_dith", icon: "dither", tool: ToolDither, alt2: -1, tip: "Dither brush, experimental (Shift+D) · right-click: Auto dither", right: func() { a.setTool(ToolDither); a.later(a.openAutoDither) }},
		{id: "t_smear", icon: "smear", tool: ToolSmear, alt2: -1, tip: "Smear (Shift+S)"},
		{id: "t_erase", icon: "eraser", tool: ToolErase, alt2: -1, tip: "Eraser (W) – makes the active layer transparent"},
		{id: "t_shade", icon: "shade", tool: ToolShade, alt2: -1, tip: "Shade (Shift+W): left = lighten, right = darken"},
		{id: "t_grad", icon: "gradient", tool: ToolGradient, alt2: -1, tip: "Gradient fill (Shift+F) – opens the gradient window",
			action: func() { a.later(a.openGradientDialog) }, right: func() { a.setTool(ToolGradient); a.later(a.openGradientDialog) }},
		{id: "t_rect", icon: "rect", alt: "rectf", tool: ToolRect, alt2: ToolRectF, tip: "Rectangle (R) / filled (Shift+R)"},
		{id: "t_circ", icon: "circle", alt: "circlef", tool: ToolCircle, alt2: ToolCircleF, tip: "Circle (C) / filled (Shift+C)"},
		{id: "t_ell", icon: "ellipse", alt: "ellipsef", tool: ToolEllipse, alt2: ToolEllipseF, tip: "Ellipse (E) / filled (Shift+E)"},
		{id: "t_sq", icon: "square", alt: "squaref", tool: ToolSquare, alt2: ToolSquareF, tip: "Square (N) / filled (Shift+N)"},
		{id: "t_tri", icon: "triangle", alt: "trianglef", tool: ToolTriangle, alt2: ToolTriangleF, tip: "Triangle (I) / filled (Shift+I)"},
		{id: "t_hex", icon: "hexagon", alt: "hexagonf", tool: ToolHexagon, alt2: ToolHexagonF, tip: "Hexagon (O) / filled (Shift+O)"},
		{id: "t_star", icon: "star", alt: "starf", tool: ToolStar, alt2: ToolStarF, tip: "Star (L) / filled (Shift+L)"},
		{id: "t_sel", icon: "select", tool: ToolSelect, alt2: -1, tip: "Select (B)"},
		{id: "t_pick", icon: "picker", tool: ToolPicker, alt2: -1, tip: "Color picker (,) – also Alt+click"},
		{id: "t_hand", icon: "hand", tool: ToolHand, alt2: -1, tip: "Pan view (hold Space)"},
		{id: "t_stamp", icon: "stamp", tool: -1, alt2: -1, tip: "Stamps: silhouettes of people, animals and objects as brush",
			action: func() { a.later(a.openStamps) }},
		{id: "t_sym", icon: "symmetry", tool: -1, alt2: -1, tip: "Symmetry on/off (Shift+X): paint mirrored",
			action: a.toggleSymmetry, active: func() bool { return a.set.Sym.On }},
		{id: "t_crt", icon: "crt", tool: -1, alt2: -1, tip: "CRT preview window (Shift+V)",
			action: a.toggleCRT, active: func() bool { return a.crt.open }},
		{id: "t_grid", icon: "grid", tool: -1, alt2: -1, tip: "Grid on/off (G) · right-click: settings",
			action: func() { a.set.GridOn = !a.set.GridOn; a.saveSettings() }, active: func() bool { return a.set.GridOn }, right: a.openGridDialog},
		{id: "t_tpl", icon: "layer", tool: -1, alt2: -1, tip: "Template on/off (T) · right-click: load",
			action: func() {
				if a.tpl.src == nil {
					a.later(a.loadTemplateDialog)
				} else {
					a.tpl.visible = !a.tpl.visible
				}
			}, active: func() bool { return a.tpl.src != nil && a.tpl.visible }, right: func() { a.later(a.loadTemplateDialog) }},
		{id: "t_zin", icon: "zoomin", tool: -1, alt2: -1, tip: "Zoom in (>) · mouse wheel", action: func() { a.zoomStep(1, -1, -1) }},
		{id: "t_zout", icon: "zoomout", tool: -1, alt2: -1, tip: "Zoom out (<) · right-click: fit", action: func() { a.zoomStep(-1, -1, -1) }, right: a.zoomFit},
		{id: "t_undo", icon: "undo", tool: -1, alt2: -1, tip: "Undo (U, Ctrl+Z)", action: a.undo},
		{id: "t_redo", icon: "redo", tool: -1, alt2: -1, tip: "Redo (Shift+U, Ctrl+Y)", action: a.redo},
	}
}

var brushPresets = []struct{ shape, size int }{
	{BrushRound, 1}, {BrushRound, 3}, {BrushRound, 5}, {BrushRound, 8},
	{BrushSquare, 2}, {BrushSquare, 4}, {BrushSquare, 8}, {BrushSquare, 16},
	{BrushDiamond, 3}, {BrushDiamond, 5}, {BrushDiamond, 9}, {BrushDiamond, 16},
}

var brushShapeNames = map[int]string{BrushRound: "round", BrushSquare: "square", BrushDiamond: "diamond", BrushCustom: "custom"}

func (a *App) drawToolbox() {
	th := a.th
	m := a.img
	r := a.rTools
	m.Fill(r, th.Panel)
	m.VLine(r.X, r.Y, r.Bottom()-1, th.Border)
	old := m.SetClip(r)
	defer m.SetClip(old)
	x0 := r.X + 5
	y := r.Y + 6
	const bw, bh = 30, 26
	// Pinsel-Vorgaben
	for i, p := range brushPresets {
		br := gfx.Rect{X: x0 + (i%4)*15, Y: y + (i/4)*15, W: 14, H: 14}
		sel := a.brushShape == p.shape && a.brushSize == p.size
		fill := th.Btn
		if sel {
			fill = th.Accent
		} else if a.ui.hover(br) {
			fill = th.BtnHover
		}
		m.Fill(br, fill)
		m.Frame(br, th.BtnBorder)
		fc := th.Text
		if sel {
			fc = th.AccentText
		}
		// Miniatur: Größe proportional
		d := 2 + p.size*8/16
		if d > 10 {
			d = 10
		}
		cx, cy := br.X+7, br.Y+7
		if p.shape == BrushRound {
			if d <= 2 {
				m.Fill(gfx.Rect{X: cx - 1, Y: cy - 1, W: 2, H: 2}, fc)
			} else {
				circle(m, cx, cy, d/2, fc)
			}
		} else if p.shape == BrushDiamond {
			rr := (d + 1) / 2
			for dy := -rr; dy <= rr; dy++ {
				hw := rr - absI(dy)
				m.Fill(gfx.Rect{X: cx - hw, Y: cy + dy, W: 2*hw + 1, H: 1}, fc)
			}
		} else {
			m.Fill(gfx.Rect{X: cx - d/2, Y: cy - d/2, W: d, H: d}, fc)
		}
		if c, _ := a.ui.press(fmt.Sprintf("bp%d", i), br, 0); c {
			a.brushShape, a.brushSize = p.shape, p.size
			if a.tool == ToolSelect || a.tool == ToolPicker || a.tool == ToolFill || a.tool == ToolHand {
				a.setTool(ToolDraw)
			}
		}
		a.ui.Tip(fmt.Sprintf("bp%d", i), br, fmt.Sprintf("Brush %s %d (+/− changes size)", brushShapeNames[p.shape], p.size))
	}
	y += ((len(brushPresets)+3)/4)*15 + 4
	// Werkzeuge (flatter buttons when the window is low)
	rows := (len(a.toolButtons()) + 1) / 2
	tbh := bh
	if free := r.Bottom() - y - 8 - 40 - 8*16 - 6; rows*(tbh+1) > free {
		tbh = clampI(free/rows-1, 18, bh)
	}
	for i, tb := range a.toolButtons() {
		br := gfx.Rect{X: x0 + (i%2)*(bw+1), Y: y + (i/2)*(tbh+1), W: bw, H: tbh}
		a.toolButton(tb, br)
	}
	n := len(a.toolButtons())
	y += ((n+1)/2)*(tbh+1) + 8
	// Farbanzeige: Farbe 1 / Farbe 2
	cr := gfx.Rect{X: x0, Y: y, W: 61, H: 34}
	m.Fill(cr, th.PanelAlt)
	m.Frame(cr, th.BtnBorder)
	f2 := gfx.Rect{X: cr.X + 24, Y: cr.Y + 10, W: 32, H: 20}
	m.Fill(f2, a.pal.Colors[a.fg2])
	m.Frame(f2, th.Text)
	f1 := gfx.Rect{X: cr.X + 5, Y: cr.Y + 4, W: 32, H: 20}
	m.Fill(f1, a.pal.Colors[a.fg])
	m.Frame(f1, th.Text)
	m.Frame(f1.Inset(1), th.Panel)
	a.ui.Tip("fgbg", cr, fmt.Sprintf("Color 1: %s · Color 2: %s", c64.ColorNames[a.fg], c64.ColorNames[a.fg2]))
	if c, _ := a.ui.press("fgbgswap", cr, 0); c {
		a.fg, a.fg2 = a.fg2, a.fg
	}
	y += 40
	// Palette 2×8
	avail := r.Bottom() - y - 6
	sh := clampI(avail/8, 10, 26)
	for i := 0; i < 16; i++ {
		col := i / 8
		row := i % 8
		sr := gfx.Rect{X: x0 + col*31, Y: y + row*sh, W: 30, H: sh - 1}
		m.Fill(sr, a.pal.Colors[i])
		m.Frame(sr, th.BtnBorder)
		// Markierungen
		lum := luma(a.pal.Colors[i])
		mc := uint32(0xFFFFFF)
		if lum > 140 {
			mc = 0x000000
		}
		if uint8(i) == a.fg {
			m.Frame(sr.Inset(1), mc)
			m.Frame(sr.Inset(2), mc)
		} else if uint8(i) == a.fg2 {
			dashedFrame(m, sr.Inset(2), mc, a.pal.Colors[i], 0)
		}
		if uint8(i) == a.pic.BG {
			m.Text(font.MonoB, sr.X+3, sr.Y+(sr.H-font.MonoB.Height)/2, "B", mc)
		}
		id := fmt.Sprintf("pal%d", i)
		if c, _ := a.ui.press(id, sr, 0); c {
			if a.in.Mods.Ctrl {
				a.setBG(uint8(i))
			} else {
				a.fg = uint8(i)
			}
		}
		if c, _ := a.ui.press(id, sr, 1); c {
			a.fg2 = uint8(i)
		}
		a.ui.Tip(id, sr, fmt.Sprintf("%d %s – left: color 1, right: color 2, Ctrl+click: background", i, c64.ColorNames[i]))
	}
}

func luma(c uint32) int {
	r, g, b := gfx.Split(c)
	return int(r*299+g*587+b*114) / 1000
}

func (a *App) toolButton(tb toolBtn, br gfx.Rect) {
	th := a.th
	m := a.img
	selected := false
	if tb.tool >= 0 {
		selected = a.tool == tb.tool || (tb.alt2 >= 0 && a.tool == tb.alt2)
	} else if tb.active != nil {
		selected = tb.active()
	}
	cl, held := a.ui.press(tb.id, br, 0)
	cr, _ := a.ui.press(tb.id, br, 1)
	fill := th.Btn
	ic := th.Text
	if selected {
		fill, ic = th.Accent, th.AccentText
	} else if held {
		fill = th.BtnDown
	} else if a.ui.hover(br) {
		fill = th.BtnHover
	}
	if selected {
		m.Fill(br, fill)
		m.Frame(br, th.BtnBorder)
	} else {
		a.ui.bevelBox(br, fill, held)
	}
	ix, iy := br.X+(br.W-16)/2, br.Y+(br.H-16)/2
	if tb.alt != "" {
		drawSplitIcon(m, tb.icon, tb.alt, ix, iy, ic, ic)
		// Diagonale andeuten
		for k := 0; k < 6; k++ {
			m.BlendPix(br.Right()-3-k, br.Y+2+k, ic, 60)
		}
	} else {
		drawIcon(m, tb.icon, ix, iy, ic, th.Accent)
		if selected {
			drawIcon(m, tb.icon, ix, iy, ic, ic)
		}
	}
	a.ui.Tip(tb.id, br, tb.tip)
	if cl {
		if tb.tool >= 0 {
			t := tb.tool
			if tb.alt2 >= 0 {
				// geteilter Knopf: unten rechts = gefüllt
				rx := float64(a.in.MX-br.X) / float64(br.W)
				ry := float64(a.in.MY-br.Y) / float64(br.H)
				if rx+ry > 1 {
					t = tb.alt2
				}
			}
			a.setTool(t)
			if tb.action != nil {
				tb.action()
			}
		} else if tb.action != nil {
			tb.action()
		}
	}
	if cr {
		if tb.right != nil {
			tb.right()
		} else if tb.tool >= 0 {
			a.setTool(tb.tool)
			a.openHelp("tool_" + itoa(tb.tool))
		}
	}
}

// --- Seitenleiste (links) ------------------------------------------------------------

func (a *App) drawPanel() {
	r := a.rPanel
	if r.W <= 0 {
		return
	}
	th := a.th
	m := a.img
	m.Fill(r, th.Panel)
	m.VLine(r.Right()-1, r.Y, r.Bottom()-1, th.Border)
	old := m.SetClip(r)
	defer m.SetClip(old)
	// Mausrad scrollt die Leiste (wenn kein Regler es benutzt hat)
	maxScroll := max(0, a.panelContentH-r.H)
	a.panelScroll = clampI(a.panelScroll, 0, maxScroll)
	y := r.Y - a.panelScroll
	x := r.X + 10
	w := r.W - 22
	if a.tool == ToolSelect && (a.sel.state == selSelected || a.sel.state == selFloating) {
		y = a.panelSelection(x, y, w)
	}
	if a.tool == ToolCellSwap {
		y = a.panelCellSwap(x, y, w)
	}
	if a.tool == ToolDither {
		y = a.panelDither(x, y, w)
	}
	if a.tool == ToolSmear {
		y = a.panelSmear(x, y, w)
	}
	if a.set.Sym.On {
		y = a.panelSymmetry(x, y, w)
	}
	if a.tool == ToolShade {
		y = a.panelShade(x, y, w)
	}
	if a.tool == ToolGradient {
		y = a.panelGradient(x, y, w)
	} else {
		y = a.panelBrush(x, y, w)
	}
	if a.tool != ToolDither && a.tool != ToolSmear && a.tool != ToolErase && a.tool != ToolGradient && a.tool != ToolShade {
		y = a.panelPattern(x, y, w)
	}
	y = a.panelColors(x, y, w)
	y = a.panelLayers(x, y, w)
	y = a.panelCell(x, y, w)
	y = a.panelNavigator(x, y, w)
	a.panelContentH = y + a.panelScroll - r.Y + 8
	// Scrollbalken
	if a.panelContentH > r.H {
		track := gfx.Rect{X: r.Right() - 8, Y: r.Y + 2, W: 5, H: r.H - 4}
		m.Fill(track, th.PanelAlt)
		th0 := track.H * r.H / a.panelContentH
		ty := track.Y + (track.H-th0)*a.panelScroll/max(1, a.panelContentH-r.H)
		m.Fill(gfx.Rect{X: track.X, Y: ty, W: track.W, H: th0}, th.Track)
	}
	if a.ui.hover(r) && a.in.Wheel != 0 && !a.ui.wheelUsed {
		a.panelScroll -= a.in.Wheel * 40
		a.panelScroll = clampI(a.panelScroll, 0, max(0, a.panelContentH-r.H))
		a.ui.wheelUsed = true
	}
}

func (a *App) section(x, y, w int, title string) int {
	a.ui.Header(gfx.Rect{X: a.rPanel.X, Y: y, W: a.rPanel.W - 1, H: 24}, title)
	return y + 30
}

func (a *App) panelBrush(x, y, w int) int {
	u := &a.ui
	y = a.section(x, y, w, "Brush")
	bw := (w - 4) / 2
	shapes := []string{"Round", "Square", "Diamond", "Custom"}
	shapeRect := func(i int) gfx.Rect {
		r := gfx.Rect{X: x + (i%2)*(bw+4), Y: y + (i/2)*26, W: bw, H: 22}
		if i%2 == 1 {
			r.W = x + w - r.X
		}
		return r
	}
	for i, s := range shapes {
		en := i != BrushCustom || a.custom != nil
		if u.ButtonEx("bshape"+itoa(i), shapeRect(i), s, a.brushShape == i, en) {
			a.brushShape = i
			a.stampCache = nil
		}
	}
	u.Tip("bshape"+itoa(BrushDiamond), shapeRect(BrushDiamond), "Diamond: a square rotated by 45°")
	u.Tip("bshape"+itoa(BrushCustom), shapeRect(BrushCustom), "Custom brush: selection → \"Use as brush\"")
	y += 54
	if a.brushShape != BrushCustom {
		ns := u.Slider("bsize", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Size", a.brushSize, 1, 16, " px")
		if ns != a.brushSize {
			a.brushSize = ns
		}
		y += 26
		// Schnellwahl
		sizes := []int{1, 2, 3, 4, 6, 8, 12, 16}
		qw := (w - 7) / 8
		for i, s := range sizes {
			br := gfx.Rect{X: x + i*(qw+1), Y: y, W: qw, H: 20}
			if u.ButtonEx("bq"+itoa(s), br, itoa(s), a.brushSize == s, true) {
				a.brushSize = s
			}
		}
		y += 26
	} else if a.custom != nil {
		u.Label(gfx.Rect{X: x, Y: y, W: w, H: 18}, fmt.Sprintf("Custom brush: %d × %d pixels", a.custom.W, a.custom.H), true)
		y += 20
		a.customOwnColor = u.Checkbox("bowncol", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Brush keeps its own colors", a.customOwnColor)
		y += 24
	}
	// Vorschau
	pr := gfx.Rect{X: x, Y: y, W: w, H: 54}
	a.img.Fill(pr, a.pal.Colors[a.pic.BG])
	a.img.Frame(pr, a.th.BtnBorder)
	st := a.stamp()
	ps := 3
	for st.W*2*ps > pr.W-8 || st.H*ps > pr.H-8 {
		ps--
		if ps == 1 {
			break
		}
	}
	cx := pr.X + pr.W/2 - st.W*ps
	cy := pr.Y + pr.H/2 - st.H*ps/2
	for i, o := range st.Offs {
		c := a.pal.Colors[a.fg]
		if st.Cols != nil && a.customOwnColor {
			c = a.pal.Colors[st.Cols[i]]
		}
		a.img.Fill(gfx.Rect{X: cx + (o.X+st.HX)*2*ps, Y: cy + (o.Y+st.HY)*ps, W: 2 * ps, H: ps}, c)
	}
	y += 60
	a.set.AspectBrush = u.Checkbox("baspect", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Round on screen (2:1)", a.set.AspectBrush)
	u.Tip("baspect", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Multicolor pixels are twice as wide as high")
	y += 24
	if a.tool == ToolSpray {
		a.sprayRadius = u.Slider("sprad", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Spray radius", a.sprayRadius, 2, 64, "")
		y += 26
		a.sprayDensity = u.Slider("spden", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Density", a.sprayDensity, 1, 40, "")
		y += 26
	}
	return y + 6
}

func (a *App) panelPattern(x, y, w int) int {
	u := &a.ui
	m := a.img
	y = a.section(x, y, w, "Patterns")
	cols := 7
	cw := (w - (cols - 1)) / cols
	ch := 20
	for i := range patterns {
		br := gfx.Rect{X: x + (i%cols)*(cw+1), Y: y + (i/cols)*(ch+1), W: cw, H: ch}
		c1 := a.pal.Colors[a.fg]
		c0 := a.pal.Colors[a.pic.BG]
		for yy := 0; yy < ch-2; yy++ {
			for xx := 0; xx < cw-2; xx++ {
				c := c0
				if patBit(i, xx/2, yy) {
					c = c1
				}
				m.Set(br.X+1+xx, br.Y+1+yy, c)
			}
		}
		if i == a.pattern {
			m.Frame(br, a.th.Accent)
			m.Frame(br.Inset(-1), a.th.Accent)
		} else {
			m.Frame(br, a.th.BtnBorder)
		}
		id := "pat" + itoa(i)
		if c, _ := u.press(id, br, 0); c {
			a.pattern = i
		}
		u.Tip(id, br, "Pattern: "+patternNames[i])
	}
	return y + ((len(patterns)+cols-1)/cols)*(ch+1) + 8
}

func (a *App) panelColors(x, y, w int) int {
	u := &a.ui
	m := a.img
	th := a.th
	y = a.section(x, y, w, "Colors")
	// Farbe 1/2
	for k, c := range []uint8{a.fg, a.fg2} {
		r := gfx.Rect{X: x, Y: y, W: 18, H: 16}
		m.Fill(r, a.pal.Colors[c])
		m.Frame(r, th.BtnBorder)
		u.Label(gfx.Rect{X: x + 24, Y: y - 1, W: w - 24, H: 18}, fmt.Sprintf("Color %d: %d %s", k+1, c, c64.ColorNames[c]), false)
		y += 20
	}
	y += 4
	sw := (w - 15) / 16
	row := func(id, label string, cur uint8, set func(uint8)) {
		u.Label(gfx.Rect{X: x, Y: y, W: w, H: 16}, label, true)
		y += 18
		for i := 0; i < 16; i++ {
			r := gfx.Rect{X: x + i*(sw+1), Y: y, W: sw, H: 14}
			m.Fill(r, a.pal.Colors[i])
			if uint8(i) == cur {
				m.Frame(r.Inset(-1), th.Text)
				m.Frame(r, th.Panel)
			}
			if c, _ := u.press(id+itoa(i), r, 0); c {
				set(uint8(i))
			}
			u.Tip(id+itoa(i), r, fmt.Sprintf("%d %s", i, c64.ColorNames[i]))
		}
		y += 20
	}
	row("bgc", "Background ($D021, bit pair %00)", a.pic.BG, a.setBG)
	row("brc", "Border ($D020)", a.pic.Border, a.setBorder)
	u.Label(gfx.Rect{X: x, Y: y, W: w, H: 16}, "When a cell is full (4 colors):", true)
	y += 18
	names := []string{"Block (strict)", "Replace rarest color", "Use nearest color"}
	tips := []string{
		"The pixel is not set, the cell flashes red",
		"The least used cell color is recolored",
		"The most similar existing cell color is painted",
	}
	for i, n := range names {
		r := gfx.Rect{X: x, Y: y, W: w, H: 20}
		if u.Radio("conf"+itoa(i), r, n, a.set.Conflict == i) {
			a.setConflict(i)
		}
		u.Tip("conf"+itoa(i), r, tips[i])
		y += 21
	}
	return y + 8
}

func (a *App) panelCell(x, y, w int) int {
	u := &a.ui
	m := a.img
	th := a.th
	y = a.section(x, y, w, "Cell under the mouse")
	if !a.mouseIn {
		u.Label(gfx.Rect{X: x, Y: y, W: w, H: 18}, "Move the mouse over the picture", true)
		y += 22
	} else {
		cell := c64.CellOf(a.mpx, a.mpy)
		cols, used := a.pic.CellColors(cell)
		hidden := a.pic.HiddenOnly(cell)
		u.Label(gfx.Rect{X: x, Y: y, W: w, H: 18}, fmt.Sprintf("Column %d, row %d  (4×8 pixels)", cell%c64.Cols, cell/c64.Cols), false)
		y += 20
		src := []string{"%00 $D021", "%01 Screen hi", "%10 Screen lo", "%11 Color RAM"}
		for s := 0; s < 4; s++ {
			r := gfx.Rect{X: x, Y: y, W: 18, H: 15}
			m.Fill(r, a.pal.Colors[cols[s]])
			m.Frame(r, th.BtnBorder)
			m.Text(font.Mono, x+24, y, src[s], th.Text)
			st := c64.ColorNames[cols[s]]
			c := th.Text
			if hidden[s] {
				st += " (hidden layer)"
			}
			if !used[s] {
				st = "free"
				c = th.TextDim
				if s == 0 {
					st = c64.ColorNames[cols[s]]
				}
			}
			m.Text(font.UI, x+130, y+1, gfx.Elide(font.UI, st, w-130), c)
			y += 18
		}
	}
	full := a.pic.FullCells()
	u.Label(gfx.Rect{X: x, Y: y, W: w, H: 18}, fmt.Sprintf("Full cells (4 colors): %d of 1000", full), true)
	return y + 26
}

func (a *App) panelNavigator(x, y, w int) int {
	m := a.img
	th := a.th
	y = a.section(x, y, w, "Navigator")
	nh := w * 200 / 320
	nr := gfx.Rect{X: x, Y: y, W: w, H: nh}
	for yy := 0; yy < nh; yy++ {
		py := yy * 200 / nh
		for xx := 0; xx < w; xx++ {
			px := xx * 160 / w
			m.Set(nr.X+xx, nr.Y+yy, a.pal.Colors[a.pic.ColorAt(px, py)])
		}
	}
	m.Frame(nr.Inset(-1), th.BtnBorder)
	// sichtbarer Ausschnitt
	ox, oy := a.origin()
	vx0 := float64(a.rCanvas.X-ox) / float64(320*a.zoom)
	vy0 := float64(a.rCanvas.Y-oy) / float64(200*a.zoom)
	vx1 := float64(a.rCanvas.Right()-ox) / float64(320*a.zoom)
	vy1 := float64(a.rCanvas.Bottom()-oy) / float64(200*a.zoom)
	if vx0 > 0 || vy0 > 0 || vx1 < 1 || vy1 < 1 {
		cl := func(v float64) float64 { return math.Max(0, math.Min(1, v)) }
		vr := gfx.Rect{X: nr.X + int(cl(vx0)*float64(w)), Y: nr.Y + int(cl(vy0)*float64(nh))}
		vr.W = nr.X + int(cl(vx1)*float64(w)) - vr.X
		vr.H = nr.Y + int(cl(vy1)*float64(nh)) - vr.Y
		dashedFrame(m, vr, 0xFFFFFF, 0x000000, 0)
	}
	_, held := a.ui.press("nav", nr, 0)
	if held || (a.ui.active == "nav#0" && a.in.Down[0]) {
		fx := float64(a.in.MX-nr.X) / float64(w)
		fy := float64(a.in.MY-nr.Y) / float64(nh)
		a.panX = a.rCanvas.W/2 - int(fx*float64(320*a.zoom))
		a.panY = a.rCanvas.H/2 - int(fy*float64(200*a.zoom))
		a.clampPan()
	}
	a.ui.Tip("nav", nr, "Click or drag: choose the visible area")
	return y + nh + 12
}

// panelSelection zeigt die Transformationsregler der Auswahl.
func (a *App) panelSelection(x, y, w int) int {
	y = a.section(x, y, w, "Selection")
	return a.transformControls(x, y, w, "p") + 6
}

// transformControls: Regler für Drehen, Skalieren, Neigen (Seitenleiste und Dialog).
func (a *App) transformControls(x, y, w int, idp string) int {
	u := &a.ui
	s := &a.sel
	qb := []struct {
		l, tip string
		op     int
	}{
		{"↔", "Flip horizontally (X)", qFlipX}, {"↕", "Flip vertically (Y)", qFlipY},
		{"↻", "Rotate 90° (Z)", qRot90}, {"↺", "Rotate −90° (Shift+Z)", qRot270},
		{"½", "Half size (H)", qHalf}, {"2×", "Double size (Shift+H)", qDouble},
	}
	bw := (w - 5*3) / 6
	for i, q := range qb {
		r := gfx.Rect{X: x + i*(bw+3), Y: y, W: bw, H: 24}
		if u.Button(idp+"q"+itoa(i), r, q.l) {
			a.transformQuick(q.op)
		}
		u.Tip(idp+"q"+itoa(i), r, q.tip)
	}
	y += 30
	xf := s.xf
	if s.state != selFloating {
		xf = identity(0, 0)
	}
	ang := int(math.Round(xf.angle))
	wpc := int(math.Round(xf.sx * 100))
	hpc := int(math.Round(xf.sy * 100))
	shx := int(math.Round(xf.shx * 100))
	shy := int(math.Round(xf.shy * 100))
	nang := u.Slider(idp+"ang", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Rotate", ang, -180, 180, "°")
	y += 26
	nw := u.Slider(idp+"sw", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Width", wpc, 5, 400, " %")
	y += 26
	nh := u.Slider(idp+"sh", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Height", hpc, 5, 400, " %")
	y += 26
	nsx := u.Slider(idp+"shx", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Shear ↔", shx, -200, 200, " %")
	y += 26
	nsy := u.Slider(idp+"shy", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Shear ↕", shy, -200, 200, " %")
	y += 28
	if nang != ang || nw != wpc || nh != hpc || nsx != shx || nsy != shy {
		if a.selEnsureFloating() {
			if nang != ang {
				s.xf.angle = float64(nang)
			}
			if nw != wpc {
				s.xf.sx = float64(nw) / 100
			}
			if nh != hpc {
				s.xf.sy = float64(nh) / 100
			}
			if nsx != shx {
				s.xf.shx = float64(nsx) / 100
			}
			if nsy != shy {
				s.xf.shy = float64(nsy) / 100
			}
			a.selRender()
		}
	}
	bw2 := (w - 4) / 2
	if u.ButtonEx(idp+"reset", gfx.Rect{X: x, Y: y, W: bw2, H: 22}, "Reset", false, s.state == selFloating) {
		a.transformQuick(qReset)
	}
	if u.ButtonEx(idp+"apply", gfx.Rect{X: x + bw2 + 4, Y: y, W: bw2, H: 22}, "Apply", s.state == selFloating, s.state == selFloating) {
		a.selCommit()
	}
	y += 26
	if u.Button(idp+"brush", gfx.Rect{X: x, Y: y, W: bw2, H: 22}, "Use as brush") {
		a.selToBrush()
	}
	if u.Button(idp+"cancel", gfx.Rect{X: x + bw2 + 4, Y: y, W: bw2, H: 22}, "Cancel") {
		a.escape()
	}
	y += 26
	s.transpBG = u.Checkbox(idp+"transp", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Background color transparent", s.transpBG)
	u.Tip(idp+"transp", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Applies when lifting and for \"Use as brush\"")
	return y + 24
}
