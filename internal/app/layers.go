package app

import (
	"fmt"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// Layers in the user interface:
//
//	Layer 1    the template (any image, colors do not matter)
//	Layer 2–4  the paint layers of the picture (c64.Picture.L[0..2]); the
//	           multicolor rules apply to all three together
//
// Visibility and the active layer belong to the view: undo does not change
// them.

// layerName returns the display name of paint layer l (0..2).
func layerName(l int) string { return fmt.Sprintf("Layer %d", l+2) }

// setFlatPicture replaces the picture by a plain C64 picture, which goes into
// layer 2; layers 3 and 4 become empty.
func (a *App) setFlatPicture(p *c64.Picture) {
	q := *p
	q.Layered = false
	q.ToLayers()
	*a.pic = q
}

func (a *App) setActiveLayer(l int) {
	if l < 0 || l >= c64.LayerCount || a.pic.Active == l {
		return
	}
	a.commitOrCancelForHistory()
	a.pic.Active = l
	a.ditherRes = nil
	msg := "Painting on " + layerName(l)
	if a.pic.Hide[l] {
		msg += " (hidden – show it to see what you paint)"
	}
	a.setStatus(msg)
}

func (a *App) toggleLayer(l int) {
	if l < 0 || l >= c64.LayerCount {
		return
	}
	a.commitOrCancelForHistory()
	a.pic.SetLayerHidden(l, !a.pic.Hide[l])
	a.ditherRes = nil
	if a.pic.Hide[l] {
		a.setStatus(layerName(l) + " hidden – its colors still count for the cell rules")
	} else {
		a.setStatus(layerName(l) + " shown")
	}
}

func (a *App) clearActiveLayer() {
	a.commitOrCancelForHistory()
	a.sel.state = selNone
	if a.pic.LayerPixels(a.pic.Active) == 0 {
		a.setStatus(layerName(a.pic.Active) + " is already empty")
		return
	}
	a.pushUndo(a.pic.Clone())
	a.pic.ClearLayer(a.pic.Active)
	a.setStatus(layerName(a.pic.Active) + " cleared")
}

// restoreKeepingView restores a picture (undo/redo) but keeps the current
// visibility of the layers and the active layer.
func (a *App) restoreKeepingView(p *c64.Picture) {
	hide, act := a.pic.Hide, a.pic.Active
	*a.pic = *p
	a.pic.Active = act
	if a.pic.Hide != hide {
		a.pic.Hide = hide
		a.pic.Recomposite()
	}
}

func (a *App) panelLayers(x, y, w int) int {
	u := &a.ui
	m := a.img
	th := a.th
	y = a.section(x, y, w, "Layers")
	row := func(id, name, sub string, visible, active bool, toggle, choose func()) {
		r := gfx.Rect{X: x, Y: y, W: w, H: 34}
		fill := th.PanelAlt
		if active {
			fill = gfx.Mix(th.PanelAlt, th.Accent, 60)
		}
		m.Fill(r, fill)
		m.Frame(r, th.Border)
		if active {
			m.Frame(r, th.Accent)
			m.Frame(r.Inset(1), th.Accent)
		}
		eye := gfx.Rect{X: r.X + 4, Y: r.Y + 7, W: 22, H: 20}
		ec := th.TextDim
		if visible {
			ec = th.Text
		}
		drawIcon(m, "eye", eye.X+3, eye.Y+2, ec, th.Accent)
		if !visible {
			m.Line(eye.X+4, eye.Bottom()-4, eye.Right()-4, eye.Y+4, ec)
		}
		if c, _ := u.press(id+"_eye", eye, 0); c {
			toggle()
		}
		u.Tip(id+"_eye", eye, "Show / hide")
		body := gfx.Rect{X: r.X + 30, Y: r.Y, W: r.W - 30, H: r.H}
		if choose != nil {
			if c, _ := u.press(id, body, 0); c {
				choose()
			}
			u.Tip(id, body, "Click: paint on this layer")
		}
		m.Text(font.UIB, r.X+32, r.Y+3, gfx.Elide(font.UIB, name, r.W-38), th.Text)
		m.Text(font.UI, r.X+32, r.Y+18, gfx.Elide(font.UI, sub, r.W-38), th.TextDim)
		y += 36
	}
	for l := c64.LayerCount - 1; l >= 0; l-- {
		l := l
		n := a.pic.LayerPixels(l)
		sub := "empty"
		if n > 0 {
			sub = fmt.Sprintf("%d pixels", n)
		}
		if a.pic.Active == l {
			sub = "active · " + sub
		}
		row(fmt.Sprintf("lv_l%d", l), layerName(l), sub, !a.pic.Hide[l], a.pic.Active == l,
			func() { a.toggleLayer(l) }, func() { a.setActiveLayer(l) })
	}
	tname := "(none)"
	if a.tpl.src != nil {
		tname = a.tpl.name
	}
	row("lv_tpl", "Layer 1 · Template", tname, a.tpl.visible && a.tpl.src != nil, false,
		func() { a.tpl.visible = !a.tpl.visible }, nil)
	y += 2
	if u.ButtonEx("lyclear", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Clear "+layerName(a.pic.Active), false, a.pic.LayerPixels(a.pic.Active) > 0) {
		a.clearActiveLayer()
	}
	y += 28
	bw := (w - 4) / 2
	if u.Button("tplload", gfx.Rect{X: x, Y: y, W: bw, H: 22}, "Load template …") {
		a.later(a.loadTemplateDialog)
	}
	if u.ButtonEx("tplclear", gfx.Rect{X: x + bw + 4, Y: y, W: bw, H: 22}, "Remove", false, a.tpl.src != nil) {
		a.clearTemplate()
	}
	y += 28
	if a.tpl.src != nil {
		a.tpl.opacity = u.Slider("tplop", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Opacity", a.tpl.opacity, 0, 100, " %")
		y += 26
		if u.Radio("tplover", gfx.Rect{X: x, Y: y, W: bw, H: 20}, "Above picture", !a.tpl.under) {
			a.tpl.under = false
		}
		if u.Radio("tplunder", gfx.Rect{X: x + bw + 4, Y: y, W: bw, H: 20}, "Below picture", a.tpl.under) {
			a.tpl.under = true
		}
		u.Tip("tplunder", gfx.Rect{X: x + bw + 4, Y: y, W: bw, H: 20}, "Template shows only through background pixels (%00)")
		y += 24
		fits := []string{"Stretch", "Fit", "Fill"}
		fw := (w - 8) / 3
		for i, f := range fits {
			if u.ButtonEx("tplfit"+itoa(i), gfx.Rect{X: x + i*(fw+4), Y: y, W: fw, H: 22}, f, int(a.tpl.fit) == i, true) && int(a.tpl.fit) != i {
				a.tpl.fit = c64.FitMode(i)
				a.tpl.rebuild()
			}
		}
		y += 28
		if u.Button("tplconv", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Convert template to C64 picture …") {
			a.modal = newImportDialog(a, a.tpl.src, a.tpl.name)
		}
		y += 28
	}
	return y + 6
}
