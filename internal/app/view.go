package app

import (
	"math"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/gfx"
)

var zoomLevels = []int{1, 2, 3, 4, 5, 6, 8, 10, 12, 16, 20, 24, 32}

// Bildschirm-Ursprung des Bildes.
func (a *App) origin() (int, int) { return a.rCanvas.X + a.panX, a.rCanvas.Y + a.panY }

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// screenToPic rechnet Bildschirm- in Bildpunktkoordinaten um.
func (a *App) screenToPic(sx, sy int) (int, int) {
	ox, oy := a.origin()
	return floorDiv(sx-ox, 2*a.zoom), floorDiv(sy-oy, a.zoom)
}

// picToScreen liefert die linke obere Ecke eines Bildpunkts.
func (a *App) picToScreen(px, py int) (int, int) {
	ox, oy := a.origin()
	return ox + px*2*a.zoom, oy + py*a.zoom
}

func (a *App) borderSize() (l, t, r, b int) {
	if !a.set.ShowBorder {
		return 0, 0, 0, 0
	}
	return 32, 35, 32, 37
}

func (a *App) zoomFit() {
	l, t, r, b := a.borderSize()
	tw, thh := 320+l+r, 200+t+b
	z := 1
	for _, lv := range zoomLevels {
		if tw*lv <= a.rCanvas.W-8 && thh*lv <= a.rCanvas.H-8 {
			z = lv
		}
	}
	a.zoom = z
	a.centerView()
	a.autoFit = true
}

func (a *App) centerView() {
	a.panX = (a.rCanvas.W - 320*a.zoom) / 2
	a.panY = (a.rCanvas.H - 200*a.zoom) / 2
}

func (a *App) setZoomCentered(z int) {
	cx, cy := a.rCanvas.X+a.rCanvas.W/2, a.rCanvas.Y+a.rCanvas.H/2
	a.setZoomAt(z, cx, cy)
}

func (a *App) setZoomAt(z, cx, cy int) {
	if z == a.zoom {
		return
	}
	ox, oy := a.origin()
	u := float64(cx-ox) / float64(a.zoom)
	v := float64(cy-oy) / float64(a.zoom)
	a.zoom = z
	a.panX = cx - int(math.Round(u*float64(z))) - a.rCanvas.X
	a.panY = cy - int(math.Round(v*float64(z))) - a.rCanvas.Y
	a.clampPan()
}

// zoomStep vergrößert (dir>0) oder verkleinert; (cx,cy) = Fixpunkt (-1 = Mitte).
func (a *App) zoomStep(dir, cx, cy int) {
	if cx < 0 {
		cx, cy = a.rCanvas.X+a.rCanvas.W/2, a.rCanvas.Y+a.rCanvas.H/2
	}
	idx := 0
	for i, lv := range zoomLevels {
		if lv <= a.zoom {
			idx = i
		}
	}
	idx = clampI(idx+dir, 0, len(zoomLevels)-1)
	a.setZoomAt(zoomLevels[idx], cx, cy)
}

func (a *App) clampPan() {
	a.autoFit = false
	w, h := 320*a.zoom, 200*a.zoom
	m := 48
	a.panX = clampI(a.panX, m-w, a.rCanvas.W-m)
	a.panY = clampI(a.panY, m-h, a.rCanvas.H-m)
}

// --- Zeichnen der Zeichenfläche ---------------------------------------------------

func (a *App) drawCanvas() {
	th := a.th
	cr := a.rCanvas
	if cr.Empty() {
		return
	}
	m := a.img
	old := m.SetClip(cr)
	defer m.SetClip(old)
	z := a.zoom
	ox, oy := a.origin()
	imgR := gfx.Rect{X: ox, Y: oy, W: 320 * z, H: 200 * z}
	frameR := imgR
	if bl, bt, br, bb := a.borderSize(); bl > 0 {
		frameR = gfx.Rect{X: ox - bl*z, Y: oy - bt*z, W: (320 + bl + br) * z, H: (200 + bt + bb) * z}
	}
	// dezentes Karomuster nur um das Bild herum
	checker := func(r gfx.Rect) {
		r = r.Intersect(cr)
		if r.Empty() {
			return
		}
		old := m.SetClip(r)
		for y := cr.Y + ((r.Y-cr.Y)/16)*16; y < r.Bottom(); y += 16 {
			for x := cr.X + ((r.X-cr.X)/16)*16; x < r.Right(); x += 16 {
				c := th.CanvasBg
				if ((x-cr.X)/16+(y-cr.Y)/16)&1 == 1 {
					c = th.CanvasBg2
				}
				m.Fill(gfx.Rect{X: x, Y: y, W: 16, H: 16}, c)
			}
		}
		m.SetClip(old)
	}
	checker(gfx.Rect{X: cr.X, Y: cr.Y, W: cr.W, H: frameR.Y - cr.Y})
	checker(gfx.Rect{X: cr.X, Y: frameR.Bottom(), W: cr.W, H: cr.Bottom() - frameR.Bottom()})
	checker(gfx.Rect{X: cr.X, Y: frameR.Y, W: frameR.X - cr.X, H: frameR.H})
	checker(gfx.Rect{X: frameR.Right(), Y: frameR.Y, W: cr.Right() - frameR.Right(), H: frameR.H})
	// Schatten (nur rechts und unten)
	m.FillA(gfx.Rect{X: frameR.Right(), Y: frameR.Y + 4, W: 4, H: frameR.H}, th.Shadow, 60)
	m.FillA(gfx.Rect{X: frameR.X + 4, Y: frameR.Bottom(), W: frameR.W - 4, H: 4}, th.Shadow, 60)
	// Rahmen ($D020)
	if frameR != imgR {
		bc := a.pal.Colors[a.pic.Border]
		m.Fill(gfx.Rect{X: frameR.X, Y: frameR.Y, W: frameR.W, H: imgR.Y - frameR.Y}, bc)
		m.Fill(gfx.Rect{X: frameR.X, Y: imgR.Bottom(), W: frameR.W, H: frameR.Bottom() - imgR.Bottom()}, bc)
		m.Fill(gfx.Rect{X: frameR.X, Y: imgR.Y, W: imgR.X - frameR.X, H: imgR.H}, bc)
		m.Fill(gfx.Rect{X: imgR.Right(), Y: imgR.Y, W: frameR.Right() - imgR.Right(), H: imgR.H}, bc)
	}
	// Bild
	pic := a.pic
	var cols [c64.W * c64.H]uint32
	showPic := !a.tpl.hidePic
	for y := 0; y < c64.H; y++ {
		for x := 0; x < c64.W; x++ {
			cols[y*c64.W+x] = a.pal.Colors[pic.ColorAt(x, y)]
		}
	}
	vis := imgR.Intersect(cr)
	tplOn := a.tpl.buf != nil && a.tpl.visible
	alpha := a.tpl.opacity * 256 / 100
	bgCol := a.pal.Colors[pic.BG]
	var pxMap, txMap []int
	if tplOn {
		pxMap = make([]int, vis.W)
		txMap = make([]int, vis.W)
		for i := range pxMap {
			sx := vis.X + i
			pxMap[i] = (sx - ox) / (2 * z)
			txMap[i] = (sx - ox) * a.tpl.w / (320 * z)
		}
	}
	for sy := vis.Y; sy < vis.Bottom(); sy++ {
		py := (sy - oy) / z
		row := m.Pix[sy*m.W : sy*m.W+m.W]
		if !tplOn {
			for sx := vis.X; sx < vis.Right(); {
				px := (sx - ox) / (2 * z)
				c := cols[py*c64.W+px]
				end := ox + (px+1)*2*z
				if end > vis.Right() {
					end = vis.Right()
				}
				if !showPic {
					c = bgCol
				}
				for ; sx < end; sx++ {
					row[sx] = c
				}
			}
			continue
		}
		ty := (sy - oy) * a.tpl.h / (200 * z)
		for sx := vis.X; sx < vis.Right(); sx++ {
			px := pxMap[sx-vis.X]
			c := cols[py*c64.W+px]
			if !showPic {
				c = bgCol
			}
			ti := ty*a.tpl.w + txMap[sx-vis.X]
			ta := int(a.tpl.alpha[ti])
			if ta > 0 {
				if a.tpl.under {
					if pic.Bits[py*c64.W+px] == 0 || !showPic {
						c = gfx.Mix(c, a.tpl.buf[ti], alpha*ta/255)
					}
				} else {
					c = gfx.Mix(c, a.tpl.buf[ti], alpha*ta/255)
				}
			}
			row[sx] = c
		}
	}
	// Bildpunktraster
	if a.set.PixelGrid && z >= 4 {
		for px := 1; px < c64.W; px++ {
			x := ox + px*2*z
			if x >= vis.X && x < vis.Right() {
				for sy := vis.Y; sy < vis.Bottom(); sy++ {
					i := sy*m.W + x
					m.Pix[i] = gfx.Mix(m.Pix[i], m.Pix[i]^0xFFFFFF, 50)
				}
			}
		}
		for py := 1; py < c64.H; py++ {
			y := oy + py*z
			if y >= vis.Y && y < vis.Bottom() {
				for sx := vis.X; sx < vis.Right(); sx++ {
					i := y*m.W + sx
					m.Pix[i] = gfx.Mix(m.Pix[i], m.Pix[i]^0xFFFFFF, 50)
				}
			}
		}
	}
	// Gitter
	if a.set.GridOn {
		a.drawGrid(vis, ox, oy, z)
	}
	// Konflikt-Aufblitzen
	for cell, t := range a.flash {
		age := a.now - t
		if age > 900 || age < 0 {
			delete(a.flash, cell)
			continue
		}
		a.needAnim = true
		cx, cy := (cell%c64.Cols)*4, (cell/c64.Cols)*8
		sx, sy := a.picToScreen(cx, cy)
		r := gfx.Rect{X: sx, Y: sy, W: 8 * z, H: 8 * z}
		al := int(200 - age*200/900)
		m.FillA(r, gfx.RGB(255, 40, 40), al/2)
		if al > 40 {
			m.Frame(r, gfx.RGB(255, 40, 40))
		}
	}
	// Zellenhinweis unter der Maus
	if a.mouseIn && a.set.CellHint && a.modal == nil && a.menuOpen < 0 {
		cell := c64.CellOf(a.mpx, a.mpy)
		cx, cy := (cell%c64.Cols)*4, (cell/c64.Cols)*8
		sx, sy := a.picToScreen(cx, cy)
		r := gfx.Rect{X: sx, Y: sy, W: 8 * z, H: 8 * z}
		dashedFrame(m, r, 0xFFFFFF, 0x000000, 0)
	}
	// Pinselvorschau
	if a.mouseIn && !a.drag.active && !a.panning && a.modal == nil && a.menuOpen < 0 && !a.in.Space {
		a.drawBrushPreview()
	}
	a.drawGradientLine()
	a.drawSymAxes()
	// Auswahl
	a.drawSelection()
	if a.mirrorView {
		a.flipCanvas()
	}
}

func (a *App) drawGrid(vis gfx.Rect, ox, oy, z int) {
	m := a.img
	gw, gh := a.set.GridW, a.set.GridH
	al := a.set.GridAlpha * 256 / 100
	var gc uint32
	auto := a.set.GridColor == 0
	if !auto {
		gc = a.pal.Colors[(a.set.GridColor-1)&15]
	}
	blend := func(i int) {
		if auto {
			m.Pix[i] = gfx.Mix(m.Pix[i], m.Pix[i]^0xFFFFFF, al)
		} else {
			m.Pix[i] = gfx.Mix(m.Pix[i], gc, al)
		}
	}
	// Gitter in Hires-Bildpunkten (320×200), Versatz berücksichtigt
	for hx := ((a.set.GridOX % gw) + gw) % gw; hx <= 320; hx += gw {
		x := ox + hx*z
		if x >= vis.X && x < vis.Right() {
			for sy := vis.Y; sy < vis.Bottom(); sy++ {
				blend(sy*m.W + x)
			}
		}
	}
	for hy := ((a.set.GridOY % gh) + gh) % gh; hy <= 200; hy += gh {
		y := oy + hy*z
		if y >= vis.Y && y < vis.Bottom() {
			for sx := vis.X; sx < vis.Right(); sx++ {
				blend(y*m.W + sx)
			}
		}
	}
}

// dashedFrame zeichnet einen gestrichelten Rahmen (Ameisen), phase verschiebt das Muster.
func dashedFrame(m *gfx.Image, r gfx.Rect, c1, c2 uint32, phase int) {
	p := 0
	put := func(x, y int) {
		if ((p+phase)/4)&1 == 0 {
			m.Set(x, y, c1)
		} else {
			m.Set(x, y, c2)
		}
		p++
	}
	for x := r.X; x < r.Right(); x++ {
		put(x, r.Y)
	}
	for y := r.Y + 1; y < r.Bottom(); y++ {
		put(r.Right()-1, y)
	}
	for x := r.Right() - 2; x >= r.X; x-- {
		put(x, r.Bottom()-1)
	}
	for y := r.Bottom() - 2; y > r.Y; y-- {
		put(r.X, y)
	}
}

func (a *App) drawBrushPreview() {
	if a.tool == ToolCellSwap {
		a.drawCellSwapPreview()
		return
	}
	switch a.tool {
	case ToolSelect, ToolPicker, ToolHand, ToolFill:
		return
	case ToolGradient:
		a.drawGradientLine()
		return
	}
	if a.curve.stage == 2 {
		return
	}
	m := a.img
	z := a.zoom
	st := a.stamp()
	col := a.pal.Colors[a.fg]
	var dsrc *c64.Picture
	if a.tool == ToolDither {
		dsrc = a.ditherSource()
	}
	for i, o := range st.Offs {
		px, py := a.mpx+o.X, a.mpy+o.Y
		if px < 0 || py < 0 || px >= c64.W || py >= c64.H {
			continue
		}
		c := col
		if a.tool == ToolSmear || a.tool == ToolErase || a.tool == ToolShade {
			continue // only the outline
		}
		if dsrc != nil {
			// dither brush: show what would change
			n := dsrc.ColorAt(px, py)
			if n == a.pic.ColorAt(px, py) {
				continue
			}
			c = a.pal.Colors[n]
		} else if st.Cols != nil && a.customOwnColor {
			c = a.pal.Colors[st.Cols[i]]
		} else if !patBit(a.pattern, px, py) {
			continue
		}
		sx, sy := a.picToScreen(px, py)
		m.FillA(gfx.Rect{X: sx, Y: sy, W: 2 * z, H: z}, c, 190)
	}
	// Umriss des Pinsels
	sx, sy := a.picToScreen(a.mpx-st.HX, a.mpy-st.HY)
	r := gfx.Rect{X: sx - 1, Y: sy - 1, W: st.W*2*z + 2, H: st.H*z + 2}
	if st.W*st.H > 1 || z >= 3 {
		dashedFrame(m, r, 0xFFFFFF, 0x000000, 0)
	}
	// mirrored positions (symmetry)
	if a.symActive() {
		for _, q := range a.symMirrors(a.mpx, a.mpy) {
			qx, qy := a.picToScreen(q.X, q.Y)
			c := gfx.Rect{X: qx + z - 4, Y: qy + z/2 - 4, W: 9, H: 9}
			dashedFrame(m, gfx.Rect{X: qx + z - st.W*z - 1, Y: qy + z/2 - st.H*z/2 - 1, W: st.W*2*z + 2, H: st.H*z + 2}, 0xFF40C0, 0xFFFFFF, 0)
			m.HLine(c.X, c.Right()-1, c.Y+4, 0xFF40C0)
			m.VLine(c.X+4, c.Y, c.Bottom()-1, 0xFF40C0)
		}
	}
}
