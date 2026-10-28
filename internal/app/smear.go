package app

import (
	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- Smear brush ------------------------------------------------------------------
//
// Like the smudge tool of other paint programs: the brush picks up the colors
// under it and drags them along the stroke. Strength decides how long the
// picked-up colors last: 100 % drags them far, low values blend quickly with
// what is under the brush. With indexed C64 colors "blending" means that each
// pixel randomly takes either the carried or the underlying color.
// In full cells the nearest cell color is used, so smearing never fails.

func (a *App) smearStrength() int {
	return clampI(a.set.SmearStrength, 10, 100)
}

// smearStart picks up the colors under the brush.
func (a *App) smearStart(x, y int) {
	st := a.stamp()
	a.smearBuf = make([]int8, len(st.Offs))
	for k, o := range st.Offs {
		px, py := x+o.X, y+o.Y
		if px < 0 || py < 0 || px >= picW || py >= c64.H {
			a.smearBuf[k] = -1
			continue
		}
		a.smearBuf[k] = int8(a.pic.ColorAt(px, py))
	}
}

// smearStep moves the brush to (x, y): carried colors are put down, then the
// brush picks up some of the colors that are there now.
func (a *App) smearStep(x, y int) {
	st := a.stamp()
	if len(a.smearBuf) != len(st.Offs) {
		a.smearStart(x, y)
		return
	}
	s := uint32(a.smearStrength())
	for k, o := range st.Offs {
		px, py := x+o.X, y+o.Y
		if px < 0 || py < 0 || px >= picW || py >= c64.H {
			continue
		}
		cur := a.pic.ColorAt(px, py)
		if b := a.smearBuf[k]; b >= 0 && uint8(b) != cur && a.rand()%100 < s {
			a.pic.Plot(px, py, uint8(b), c64.ConflictNearest, a.pal)
			cur = a.pic.ColorAt(px, py)
		}
		// pick up: the weaker the smear, the sooner the brush takes the new color
		if a.smearBuf[k] < 0 || a.rand()%100 >= s {
			a.smearBuf[k] = int8(cur)
		}
	}
}

func (a *App) panelSmear(x, y, w int) int {
	u := &a.ui
	m := a.img
	y = a.section(x, y, w, "Smear")
	if a.collapsed["Smear"] {
		return y
	}
	r := gfx.Rect{X: x, Y: y, W: w, H: 22}
	a.set.SmearStrength = u.Slider("sm_str", r, "Strength", a.smearStrength(), 10, 100, " %")
	u.Tip("sm_str", r, "How far the colors are dragged: high = long streaks, low = soft blending")
	y += 28
	for _, ln := range wrapText(font.UI, "Drag to smear the colors along the stroke. Brush shape and size from the Brush panel. In full cells the nearest cell color is used.", w) {
		m.Text(font.UI, x, y, ln, a.th.TextDim)
		y += font.UI.Height + 2
	}
	return y + 8
}
