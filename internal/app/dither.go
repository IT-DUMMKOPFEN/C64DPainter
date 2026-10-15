package app

import (
	"fmt"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- shared helpers -------------------------------------------------------------

// picHash is a fast fingerprint of the picture (FNV-1a), used to know when
// the cached dither result is out of date.
func picHash(p *c64.Picture) uint64 {
	h := uint64(14695981039346656037)
	add := func(b uint8) {
		h ^= uint64(b)
		h *= 1099511628211
	}
	for _, b := range p.Bits {
		add(b)
	}
	for i := range p.Screen {
		add(p.Screen[i])
		add(p.ColorRAM[i])
	}
	add(p.BG)
	return h
}

// ditherStyleButtons draws the style choice (two columns).
func (a *App) ditherStyleButtons(id string, x, y, w int, style *c64.DitherStyle) int {
	u := &a.ui
	bw := (w - 4) / 2
	for i := 0; i < int(c64.StyleCount); i++ {
		br := gfx.Rect{X: x + (i%2)*(bw+4), Y: y + (i/2)*25, W: bw, H: 22}
		if i%2 == 1 {
			br.W = x + w - br.X
		}
		if u.ButtonEx(id+"st"+itoa(i), br, c64.StyleNames[i], int(*style) == i, true) {
			*style = c64.DitherStyle(i)
		}
		u.Tip(id+"st"+itoa(i), br, c64.StyleInfo[i])
	}
	return y + ((int(c64.StyleCount)+1)/2)*25 + 4
}

// ditherSliders draws the common settings.
func (a *App) ditherSliders(id string, x, y, w int, o *c64.AutoDitherOptions) int {
	u := &a.ui
	sl := func(sid, label string, v *int, lo, hi int, unit, tip string) {
		r := gfx.Rect{X: x, Y: y, W: w, H: 22}
		*v = u.Slider(id+sid, r, label, *v, lo, hi, unit)
		u.Tip(id+sid, r, tip)
		y += 26
	}
	sl("so", "Softness", &o.Softness, 1, 16, "", "Width of the blend zone where two colors meet (in pixel rows)")
	sl("sr", "Strength", &o.Strength, 0, 100, " %", "How strongly neighboring colors are mixed")
	sl("ed", "Keep edges", &o.Edges, 0, 100, " %", "Protects outlines and hard edges between very different brightness")
	sl("ra", "Color ramps", &o.Ramps, 0, 100, " %", "Prefer the color pairs that C64 artists mix (similar brightness)")
	return y
}

var cellModeTips = [3]string{
	"Every cell keeps exactly its colors. Safe, but no blending across cell borders where a cell lacks the other color.",
	"Existing cell colors always stay; in-between colors are only added in free slots. Never a color conflict. Recommended.",
	"The three colors of every changed cell are chosen anew – the smoothest blend, but small details may change color.",
}

// cellModeRow draws the choice of the cell color mode.
func (a *App) cellModeRow(id string, x, y, w int, mode *c64.CellColorMode) int {
	u := &a.ui
	a.img.Text(font.UI, x, y+3, "Cell colors:", a.th.Text)
	lw := font.UI.Width("Cell colors:") + 8
	n := len(c64.CellModeNames)
	if id == "db" {
		n = 2 // the brush cannot re-choose cell colors
	}
	bw := (w - lw - 4*(n-1)) / n
	for i := 0; i < n; i++ {
		br := gfx.Rect{X: x + lw + i*(bw+4), Y: y, W: bw, H: 22}
		if i == n-1 {
			br.W = x + w - br.X
		}
		if u.ButtonEx(id+"cm"+itoa(i), br, c64.CellModeNames[i], int(*mode) == i, true) {
			*mode = c64.CellColorMode(i)
		}
		u.Tip(id+"cm"+itoa(i), br, cellModeTips[i])
	}
	return y + 30
}

// --- dither brush -----------------------------------------------------------------

// ditherResult returns the auto-dithered version of the current picture with
// the brush settings. It is cached until the picture or the settings change.
func (a *App) ditherResult() *c64.Picture {
	o := a.set.DitherBrush
	if o.Colors == c64.CellRechoose {
		o.Colors = c64.CellAdd // re-choosing cell colors would cause conflicts
	}
	key := fmt.Sprintf("%x/%v/%d", picHash(a.pic), o, a.palIdx)
	if a.ditherRes == nil || a.ditherKey != key {
		a.ditherRes = c64.AutoDither(a.pic, a.pal, o, nil)
		a.ditherKey = key
	}
	return a.ditherRes
}

// ditherSource is the picture the brush copies from: fixed during a stroke.
func (a *App) ditherSource() *c64.Picture {
	if a.drag.active && a.drag.ditherSrc != nil {
		return a.drag.ditherSrc
	}
	return a.ditherResult()
}

// ditherDab copies the dithered pixels under the brush into the picture.
func (a *App) ditherDab(p *c64.Picture, x, y int) {
	src := a.drag.ditherSrc
	if src == nil {
		return
	}
	for _, o := range a.stamp().Offs {
		px, py := x+o.X, y+o.Y
		if px < 0 || py < 0 || px >= c64.W || py >= c64.H {
			continue
		}
		if c := src.ColorAt(px, py); c != p.ColorAt(px, py) {
			a.plot(p, px, py, c)
		}
	}
}

func (a *App) panelDither(x, y, w int) int {
	u := &a.ui
	m := a.img
	th := a.th
	o := &a.set.DitherBrush
	y = a.section(x, y, w, "Dither brush")
	y = a.ditherStyleButtons("db", x, y, w, &o.Style)
	for _, ln := range wrapText(font.UI, c64.StyleInfo[o.Style], w) {
		m.Text(font.UI, x, y, ln, th.TextDim)
		y += font.UI.Height + 2
	}
	y += 6
	y = a.ditherSliders("db", x, y, w, o)
	y = a.cellModeRow("db", x, y, w, &o.Colors)
	if u.Button("db_auto", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Auto dither whole picture …") {
		a.later(a.openAutoDither)
	}
	return y + 32
}

// --- Auto dither dialog -------------------------------------------------------------

type autoDitherDialog struct {
	opts     c64.AutoDitherOptions
	area     int // 0 = whole picture, 1 = selection
	view     int // 0 = dithered, 1 = original, 2 = split, 3 = changes
	hasSel   bool
	sx0, sy0 int
	sx1, sy1 int
	src      *c64.Picture
	preview  *c64.Picture
	key      string
	changed  int
	cells    int
	scroll   int
	colH     int
}

func (a *App) openAutoDither() {
	a.commitOrCancelForHistory()
	a.menuOpen = -1
	d := &autoDitherDialog{opts: a.set.DitherAuto, src: a.pic.Clone()}
	if a.sel.state == selSelected {
		d.hasSel = true
		d.sx0, d.sy0, d.sx1, d.sy1 = a.selRect()
		d.area = 1
	}
	a.modal = d
}

func (d *autoDitherDialog) Title() string          { return "Auto dither" }
func (d *autoDitherDialog) Size(a *App) (int, int) { return 1060, 720 }

func (d *autoDitherDialog) Key(a *App, k KeyEvent) {
	if k.Key == KeyEnter {
		d.apply(a)
	}
}

func (d *autoDitherDialog) mask() []bool {
	if d.area != 1 || !d.hasSel {
		return nil
	}
	m := make([]bool, c64.W*c64.H)
	for y := d.sy0; y <= d.sy1; y++ {
		for x := d.sx0; x <= d.sx1; x++ {
			if x >= 0 && y >= 0 && x < c64.W && y < c64.H {
				m[y*c64.W+x] = true
			}
		}
	}
	return m
}

func (d *autoDitherDialog) update(a *App) {
	key := fmt.Sprintf("%v/%d/%d", d.opts, d.area, a.palIdx)
	if key == d.key && d.preview != nil {
		return
	}
	d.key = key
	d.preview = c64.AutoDither(d.src, a.pal, d.opts, d.mask())
	d.changed = 0
	for y := 0; y < c64.H; y++ {
		for x := 0; x < c64.W; x++ {
			if d.preview.ColorAt(x, y) != d.src.ColorAt(x, y) {
				d.changed++
			}
		}
	}
	d.cells = 0
	for cell := 0; cell < c64.Cells; cell++ {
		a0, u0 := d.src.CellColors(cell)
		b0, u1 := d.preview.CellColors(cell)
		for s := 1; s <= 3; s++ {
			if !u1[s] {
				continue
			}
			found := b0[s] == d.src.BG
			for t := 1; t <= 3; t++ {
				if u0[t] && a0[t] == b0[s] {
					found = true
				}
			}
			if !found {
				d.cells++
				break
			}
		}
	}
}

func (d *autoDitherDialog) apply(a *App) {
	d.update(a)
	a.pushUndo(a.pic.Clone())
	border := a.pic.Border
	*a.pic = *d.preview
	a.pic.Border = border
	a.set.DitherAuto = d.opts
	a.saveSettings()
	a.closeModal(false)
	a.setStatus(fmt.Sprintf("Auto dither (%s): %d pixels changed", c64.StyleNames[d.opts.Style], d.changed))
}

func (d *autoDitherDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	d.update(a)

	// ---- preview (left) ----
	colW := 330
	pw := r.W - colW - 24
	scale := 2
	if pw < 640 || r.H-110 < 400 {
		scale = 1
	}
	tabs := []string{"Dithered", "Original", "Split", "Changes"}
	for i, t := range tabs {
		if u.ButtonEx("ad_v"+itoa(i), gfx.Rect{X: r.X + i*104, Y: r.Y, W: 100, H: 24}, t, d.view == i, true) {
			d.view = i
		}
	}
	pr := gfx.Rect{X: r.X, Y: r.Y + 34, W: 320 * scale, H: 200 * scale}
	m.FillA(gfx.Rect{X: pr.X + 4, Y: pr.Bottom(), W: pr.W, H: 4}, th.Shadow, 50)
	m.FillA(gfx.Rect{X: pr.Right(), Y: pr.Y + 4, W: 4, H: pr.H}, th.Shadow, 50)
	_, holding := u.press("ad_prev", pr, 0)
	view := d.view
	if holding {
		view = 1
	}
	for y := 0; y < c64.H; y++ {
		for x := 0; x < c64.W; x++ {
			o := d.src.ColorAt(x, y)
			n := d.preview.ColorAt(x, y)
			var c uint32
			switch {
			case view == 1 || (view == 2 && x < c64.W/2):
				c = a.pal.Colors[o]
			case view == 3:
				if o == n {
					c = gfx.Mix(a.pal.Colors[o], th.Panel, 170)
				} else {
					c = a.pal.Colors[n]
				}
			default:
				c = a.pal.Colors[n]
			}
			m.Fill(gfx.Rect{X: pr.X + x*2*scale, Y: pr.Y + y*scale, W: 2 * scale, H: scale}, c)
		}
	}
	if view == 2 {
		m.VLine(pr.X+160*scale, pr.Y, pr.Bottom()-1, 0xFFFFFF)
	}
	if d.area == 1 && d.hasSel {
		sr := gfx.Rect{X: pr.X + d.sx0*2*scale, Y: pr.Y + d.sy0*scale, W: (d.sx1 - d.sx0 + 1) * 2 * scale, H: (d.sy1 - d.sy0 + 1) * scale}
		dashedFrame(m, sr.Inset(-1), 0xFFFFFF, 0x000000, int(a.now/120))
		a.needAnim = true
	}
	m.Frame(pr.Inset(-1), th.Border)
	u.Tip("ad_prev", pr, "Hold the mouse button to see the original")
	iy := pr.Bottom() + 14
	m.Text(font.UI, r.X, iy, fmt.Sprintf("Changed pixels: %d  ·  cells with new colors: %d  ·  background stays %s",
		d.changed, d.cells, c64.ColorNames[d.src.BG]), th.Text)
	for i, ln := range wrapText(font.UI, "Where differently colored areas meet, the colors are blended and drawn again with the chosen dither style. Flat areas stay as they are. The styles were reconstructed from hand-pixelled C64 pictures; the result always follows the multicolor rules.", pr.W) {
		m.Text(font.UI, r.X, iy+22+i*(font.UI.Height+3), ln, th.TextDim)
	}

	// ---- settings (right, scrollable) ----
	cx := r.Right() - colW
	colR := gfx.Rect{X: cx - 8, Y: r.Y, W: colW + 8, H: r.H - 40}
	maxS := max(0, d.colH-colR.H)
	if u.hover(colR) && a.in.Wheel != 0 {
		d.scroll -= a.in.Wheel * 40
	}
	d.scroll = clampI(d.scroll, 0, maxS)
	old := m.SetClip(colR.Intersect(m.Clip))
	y := r.Y - d.scroll
	w := colW - 12
	hdr := func(t string) {
		u.Header(gfx.Rect{X: cx, Y: y, W: w, H: 22}, t)
		y += 28
	}
	hdr("Style")
	y = a.ditherStyleButtons("ad", cx, y, w, &d.opts.Style)
	for _, ln := range wrapText(font.UI, c64.StyleInfo[d.opts.Style], w) {
		m.Text(font.UI, cx, y, ln, th.TextDim)
		y += font.UI.Height + 2
	}
	y += 8
	hdr("Settings")
	y = a.ditherSliders("ad", cx, y, w, &d.opts)
	y = a.cellModeRow("ad", cx, y, w, &d.opts.Colors)
	if u.Button("ad_def", gfx.Rect{X: cx, Y: y, W: w, H: 22}, "Default settings") {
		st := d.opts.Style
		d.opts = c64.DefaultAutoDither()
		d.opts.Style = st
	}
	y += 32
	hdr("Area")
	bw := (w - 4) / 2
	if u.ButtonEx("ad_a0", gfx.Rect{X: cx, Y: y, W: bw, H: 24}, "Whole picture", d.area == 0, true) {
		d.area = 0
	}
	if u.ButtonEx("ad_a1", gfx.Rect{X: cx + bw + 4, Y: y, W: w - bw - 4, H: 24}, "Selection", d.area == 1, d.hasSel) {
		d.area = 1
	}
	if !d.hasSel {
		u.Tip("ad_a1", gfx.Rect{X: cx + bw + 4, Y: y, W: w - bw - 4, H: 24}, "Select an area first (Select tool, B)")
	}
	y += 34
	d.colH = y + d.scroll - r.Y
	m.SetClip(old)
	if d.colH > colR.H {
		tr := gfx.Rect{X: r.Right() - 4, Y: colR.Y, W: 4, H: colR.H}
		m.Fill(tr, th.Track)
		thh := max(20, tr.H*colR.H/d.colH)
		ty := tr.Y + (tr.H-thh)*d.scroll/max(1, maxS)
		m.Fill(gfx.Rect{X: tr.X, Y: ty, W: tr.W, H: thh}, th.Accent)
	}

	// ---- buttons ----
	by := r.Bottom() - 28
	if u.Button("ad_brush", gfx.Rect{X: r.X, Y: by, W: 220, H: 28}, "Use settings for the brush") {
		a.set.DitherBrush = d.opts
		if a.set.DitherBrush.Colors == c64.CellRechoose {
			a.set.DitherBrush.Colors = c64.CellAdd // re-choosing would cause conflicts with a brush
		}
		a.saveSettings()
		a.setStatus("Dither brush now uses: " + c64.StyleNames[d.opts.Style])
	}
	u.Tip("ad_brush", gfx.Rect{X: r.X, Y: by, W: 220, H: 28}, "The Dither brush (Shift+D) takes over style and settings")
	if u.Button("ad_cancel", gfx.Rect{X: r.Right() - 250, Y: by, W: 120, H: 28}, "Cancel") {
		a.closeModal(true)
		return
	}
	if u.ButtonEx("ad_ok", gfx.Rect{X: r.Right() - 120, Y: by, W: 120, H: 28}, "Apply", true, d.changed > 0) {
		d.apply(a)
	}
}
