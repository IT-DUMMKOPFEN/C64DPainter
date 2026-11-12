package app

import (
	"fmt"
	"math"

	"c64dpainter/internal/c64"
)

// --- Copy into the other page (1.1) ---------------------------------------------------
//
// The selection (or the whole picture) of the active layer is copied into
// the other page at exactly the same position, into the layer with the same
// number. Transparent pixels and pixels in the background color are not
// copied, so a logo with a hole lands over a pattern on the other page. The
// other page keeps its color rules.

func (a *App) copyIntoOtherPage() {
	x0, y0, x1, y1 := 0, 0, picW, c64.H
	what := "picture"
	if bx0, by0, bx1, by1, ok := a.selBox(); ok && a.sel.state != selNone {
		x0, y0 = int(math.Floor(bx0)), int(math.Floor(by0))
		x1, y1 = int(math.Ceil(bx1)), int(math.Ceil(by1))
		what = "selection"
	}
	a.commitOrCancelForHistory()
	src := a.pic
	other := 1 - a.page
	dst := a.pages[other]
	if dst.Mode != src.Mode {
		a.host.Info(AppName, "The other page has another graphics mode – copying works between pictures of the same mode.")
		return
	}
	l := src.Active
	undo := dst.Clone()
	for dst.LayerCount() <= l {
		dst.InsertLayer(dst.LayerCount())
	}
	act := dst.Active
	dst.Active = l
	mode := c64.ConflictMode(a.set.Conflict)
	if a.set.ProtectLayers {
		mode |= c64.ConflictProtect
	}
	n, blocked := 0, 0
	w := src.Width()
	for y := max(0, y0); y < min(c64.H, y1); y++ {
		for x := max(0, x0); x < min(w, x1); x++ {
			c, ok := src.LayerColorAt(l, x, y)
			if !ok || c == src.BG {
				continue // transparent
			}
			if r := dst.Plot(x, y, c, mode, a.pal); r == c64.PlotBlocked {
				blocked++
			} else {
				n++
			}
		}
	}
	dst.Active = act
	dst.Recomposite()
	if n == 0 && blocked == 0 {
		a.setStatus("Nothing to copy – the " + what + " of " + a.layerName(l) + " is empty or background")
		return
	}
	a.undos[other] = append(a.undos[other], undo)
	a.redos[other] = nil
	a.dirty[other] = true
	msg := fmt.Sprintf("%s of %s copied into page %d (%d pixels, background left out)", capitalize(what), a.layerName(l), other+1, n)
	if blocked > 0 {
		msg += fmt.Sprintf(" – %d pixels did not fit the color rules there", blocked)
	}
	a.setStatus(msg)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}

// snapSel moves a selection corner onto the grid (Selection → Snap to grid).
// start: the first corner (rounded down), else the second (to the end of
// its grid cell).
func (a *App) snapSel(x, y int, start bool) (int, int) {
	if !a.set.SelSnap {
		return x, y
	}
	gw := max(1, a.set.GridW/asp)
	gh := max(1, a.set.GridH)
	ox := ((a.set.GridOX/asp)%gw + gw) % gw
	oy := (a.set.GridOY%gh + gh) % gh
	fx := int(math.Floor(float64(x-ox)/float64(gw)))*gw + ox
	fy := int(math.Floor(float64(y-oy)/float64(gh)))*gh + oy
	if !start {
		fx += gw - 1
		fy += gh - 1
	}
	return clampI(fx, 0, picW-1), clampI(fy, 0, c64.H-1)
}
