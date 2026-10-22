package app

import (
	"fmt"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- swap color dialog -------------------------------------------------------------
//
// Mode "picture": replace one color by another in the whole picture.
// Mode "brush":   choose the two colors for the cell color swap brush.
// The background color ($D021) is never changed in either mode.

type swapDialog struct {
	brush    bool
	from, to uint8
	both     bool
}

func (a *App) validSwapColors(from, to uint8) (uint8, uint8) {
	bg := a.pic.BG
	if from == bg {
		from = 255
	}
	if to == bg || to == from {
		to = 255
	}
	for c := uint8(0); from == 255 && c < 16; c++ {
		if c != bg {
			from = c
		}
	}
	for c := uint8(0); to == 255 && c < 16; c++ {
		if c != bg && c != from {
			to = c
		}
	}
	return from, to
}

func (a *App) openSwapDialog(brush bool) {
	a.commitOrCancelForHistory()
	f, t := a.fg, a.fg2
	if a.tool == ToolCellSwap || a.swapSet {
		f, t = a.swapFrom, a.swapTo
	}
	f, t = a.validSwapColors(f, t)
	a.menuOpen = -1
	a.modal = &swapDialog{brush: brush, from: f, to: t, both: a.swapBoth}
}

func (d *swapDialog) Title() string {
	if d.brush {
		return "Swap color in cells (brush)"
	}
	return "Swap color in picture"
}

func (d *swapDialog) Size(a *App) (int, int) { return 560, 380 }

func (d *swapDialog) Key(a *App, k KeyEvent) {
	if k.Key == KeyEnter {
		d.apply(a)
	}
}

func (d *swapDialog) ok(a *App) bool {
	return d.from != d.to && d.from != a.pic.BG && d.to != a.pic.BG
}

func (d *swapDialog) apply(a *App) {
	if !d.ok(a) {
		return
	}
	a.swapFrom, a.swapTo, a.swapBoth, a.swapSet = d.from, d.to, d.both, true
	a.closeModal(false)
	if d.brush {
		a.startCellSwap()
		return
	}
	snap := a.pic.Clone()
	px, cells := a.pic.ReplaceColor(d.from, d.to, d.both)
	if px == 0 {
		a.setStatus("Nothing to swap: " + c64.ColorNames[d.from] + " is not used in the picture")
		return
	}
	a.pushUndo(snap)
	arrow := " → "
	if d.both {
		arrow = " ↔ "
	}
	a.setStatus(fmt.Sprintf("Swapped %s%s%s: %d pixels in %d cells", c64.ColorNames[d.from], arrow, c64.ColorNames[d.to], px, cells))
}

// countColor counts the pixels (not background) of a color and the cells using it.
func countColor(p *c64.Picture, col uint8) (pixels, cells int) {
	for cell := 0; cell < c64.Cells; cell++ {
		n := 0
		cx, cy := (cell%c64.Cols)*4, (cell/c64.Cols)*8
		for y := cy; y < cy+8; y++ {
			for x := cx; x < cx+4; x++ {
				if p.Bits[y*c64.W+x] != 0 && p.ColorAt(x, y) == col {
					n++
				}
			}
		}
		if n > 0 {
			pixels += n
			cells++
		}
	}
	return
}

func (d *swapDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	y := r.Y
	intro := "Replaces one color by another in every cell of the picture."
	if d.brush {
		intro = "Paint over cells with the brush: in every cell you touch, the first color is replaced by the second."
	}
	for _, ln := range wrapText(font.UI, intro, r.W) {
		m.Text(font.UI, r.X, y, ln, th.TextDim)
		y += font.UI.Height + 3
	}
	y += 10
	row := func(id, label string, cur *uint8, other uint8) {
		m.Text(font.UIB, r.X, y, label, th.Text)
		y += 22
		sw := (r.W - 15) / 16
		for i := 0; i < 16; i++ {
			c := uint8(i)
			sr := gfx.Rect{X: r.X + i*(sw+1), Y: y, W: sw, H: 30}
			m.Fill(sr, a.pal.Colors[i])
			m.Frame(sr, th.BtnBorder)
			isBG := c == a.pic.BG
			if isBG {
				mc := uint32(0xFFFFFF)
				if luma(a.pal.Colors[i]) > 140 {
					mc = 0
				}
				m.FillA(sr, th.Panel, 150)
				m.TextC(font.MonoB, sr, "B", mc)
				u.Tip(id+itoa(i), sr, c64.ColorNames[i]+" is the background color – it is never changed")
				continue
			}
			if c == *cur {
				m.Frame(sr.Inset(-2), th.Accent)
				m.Frame(sr.Inset(-1), th.Accent)
				m.Frame(sr.Inset(1), th.Panel)
			}
			if cl, _ := u.press(id+itoa(i), sr, 0); cl {
				*cur = c
			}
			u.Tip(id+itoa(i), sr, fmt.Sprintf("%d %s", i, c64.ColorNames[i]))
		}
		y += 36
	}
	row("sw_f", "Replace this color:", &d.from, d.to)
	row("sw_t", "with this color:", &d.to, d.from)
	// summary line with both swatches
	sr := gfx.Rect{X: r.X, Y: y, W: 30, H: 20}
	m.Fill(sr, a.pal.Colors[d.from])
	m.Frame(sr, th.BtnBorder)
	arrow := "→"
	if d.both {
		arrow = "↔"
	}
	m.TextC(font.H2, gfx.Rect{X: r.X + 32, Y: y, W: 30, H: 20}, arrow, th.Text)
	tr := gfx.Rect{X: r.X + 64, Y: y, W: 30, H: 20}
	m.Fill(tr, a.pal.Colors[d.to])
	m.Frame(tr, th.BtnBorder)
	m.TextL(font.UI, gfx.Rect{X: r.X + 104, Y: y, W: r.W - 104, H: 20}, c64.ColorNames[d.from]+" "+arrow+" "+c64.ColorNames[d.to], th.Text)
	y += 30
	d.both = u.Checkbox("sw_both", gfx.Rect{X: r.X, Y: y, W: r.W, H: 20}, "Exchange both colors (A ↔ B)", d.both)
	u.Tip("sw_both", gfx.Rect{X: r.X, Y: y, W: r.W, H: 20}, "Off: only A becomes B.  On: A becomes B and B becomes A.")
	y += 28
	info := "The background color ($D021) is never changed."
	if !d.brush {
		n, cells := countColor(a.pic, d.from)
		info = fmt.Sprintf("%s is used by %d pixels in %d cells. ", c64.ColorNames[d.from], n, cells) + info
	}
	if d.from == d.to {
		info = "Please choose two different colors."
	}
	for _, ln := range wrapText(font.UI, info, r.W) {
		m.Text(font.UI, r.X, y, ln, th.TextDim)
		y += font.UI.Height + 3
	}
	by := r.Bottom() - 28
	if u.Button("sw_cancel", gfx.Rect{X: r.Right() - 260, Y: by, W: 120, H: 28}, "Cancel") {
		a.closeModal(true)
		return
	}
	label := "Swap"
	if d.brush {
		label = "Start brush"
	}
	if u.ButtonEx("sw_ok", gfx.Rect{X: r.Right() - 130, Y: by, W: 130, H: 28}, label, true, d.ok(a)) {
		d.apply(a)
	}
}

// --- cell color swap brush -----------------------------------------------------------

func (a *App) startCellSwap() {
	if a.tool != ToolCellSwap {
		a.toolBeforeSwap = a.tool
	}
	a.setTool(ToolCellSwap)
	arrow := " → "
	if a.swapBoth {
		arrow = " ↔ "
	}
	a.setStatus("Cell swap brush: " + c64.ColorNames[a.swapFrom] + arrow + c64.ColorNames[a.swapTo] + " – choose any tool to stop")
}

func (a *App) stopCellSwap() {
	t := a.toolBeforeSwap
	if t == ToolCellSwap || t == ToolSelect || t == ToolPicker || t == ToolHand {
		t = ToolDraw
	}
	a.setTool(t)
}

// cellsUnderStamp returns the cells touched by the brush at (x, y).
func (a *App) cellsUnderStamp(x, y int) []int {
	st := a.stamp()
	seen := map[int]bool{}
	var out []int
	for _, o := range st.Offs {
		px, py := x+o.X, y+o.Y
		if px < 0 || py < 0 || px >= c64.W || py >= c64.H {
			continue
		}
		c := c64.CellOf(px, py)
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

// swapAt swaps the colors in all cells under the brush (each cell once per stroke).
func (a *App) swapAt(x, y, btn int) {
	from, to := a.swapFrom, a.swapTo
	if btn == 1 && !a.swapBoth { // right button: the other way round
		from, to = to, from
	}
	for _, c := range a.cellsUnderStamp(x, y) {
		if a.swapDone[c] {
			continue
		}
		a.swapDone[c] = true
		a.pic.ReplaceCellColor(c, from, to, a.swapBoth)
	}
}

func (a *App) drawCellSwapPreview() {
	m := a.img
	z := a.zoom
	for _, c := range a.cellsUnderStamp(a.mpx, a.mpy) {
		cx, cy := (c%c64.Cols)*4, (c/c64.Cols)*8
		sx, sy := a.picToScreen(cx, cy)
		r := gfx.Rect{X: sx, Y: sy, W: 8 * z, H: 8 * z}
		m.FillA(r, a.pal.Colors[a.swapTo], 70)
		m.Frame(r, a.th.Accent)
		m.Frame(r.Inset(1), 0xFFFFFF)
	}
}

// panelCellSwap shows the brush colors in the side panel.
func (a *App) panelCellSwap(x, y, w int) int {
	u := &a.ui
	m := a.img
	th := a.th
	y = a.section(x, y, w, "Cell color swap brush")
	if a.collapsed["Cell color swap brush"] {
		return y
	}
	for i, c := range []uint8{a.swapFrom, a.swapTo} {
		sr := gfx.Rect{X: x + i*70, Y: y, W: 40, H: 26}
		m.Fill(sr, a.pal.Colors[c])
		m.Frame(sr, th.BtnBorder)
	}
	arrow := "→"
	if a.swapBoth {
		arrow = "↔"
	}
	m.TextC(font.H2, gfx.Rect{X: x + 40, Y: y, W: 30, H: 26}, arrow, th.Text)
	m.TextL(font.UI, gfx.Rect{X: x + 118, Y: y, W: w - 118, H: 26}, gfx.Elide(font.UI, c64.ColorNames[a.swapFrom]+" "+arrow+" "+c64.ColorNames[a.swapTo], w-118), th.Text)
	y += 32
	for _, ln := range wrapText(font.UI, "Paint over cells. Each cell you touch is changed once per stroke. Right button: the other way round.", w) {
		m.Text(font.UI, x, y, ln, th.TextDim)
		y += font.UI.Height + 2
	}
	y += 6
	bw := (w - 4) / 2
	if u.Button("cs_change", gfx.Rect{X: x, Y: y, W: bw, H: 22}, "Change colors …") {
		a.openSwapDialog(true)
	}
	if u.Button("cs_stop", gfx.Rect{X: x + bw + 4, Y: y, W: bw, H: 22}, "Stop") {
		a.stopCellSwap()
	}
	return y + 32
}
