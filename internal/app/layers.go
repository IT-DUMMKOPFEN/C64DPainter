package app

import (
	"fmt"
	"strings"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// Layers in the user interface (listed from 1 at the top):
//
//	Layer 1     the template (any image, colors do not matter)
//	Layer 2 ... the paint layers of the picture (c64.Picture.L[0..]); layer 2
//	            is at the bottom, the last one on top. Up to 15 paint layers.
//	            The color rules apply to all normal layers together; a
//	            "free" layer ignores them (it does not change the others).
//
// The switch next to "Layers" turns the layers above layer 2 on or off (off
// at the start). While it is off they are hidden and cannot be chosen; their
// colors still count for the cell rules. Visibility and the active layer
// belong to the view: undo does not change them (unless layers were added
// or removed).

// layerNameOf returns the display name of paint layer l.
func layerNameOf(p *c64.Picture, l int) string {
	if l >= 0 && l < len(p.Names) && p.Names[l] != "" {
		return p.Names[l]
	}
	return fmt.Sprintf("Layer %d", l+2)
}

func (a *App) layerName(l int) string { return layerNameOf(a.pic, l) }

// usesExtraLayers: a picture that needs the layers above layer 2.
func usesExtraLayers(p *c64.Picture) bool {
	if p.Active > 0 || p.LayerCount() != c64.DefaultLayers {
		return true
	}
	for l := 1; l < p.LayerCount(); l++ {
		if p.LayerPixels(l) > 0 && !p.Hide[l] {
			return true
		}
	}
	return false
}

// setFlatPicture replaces the picture by a plain C64 picture, which goes into
// layer 2; the other layers become empty.
func (a *App) setFlatPicture(p *c64.Picture) {
	q := p.Clone()
	q.Layered = false
	q.ToLayers()
	a.pic.CopyFrom(q)
	a.syncExtraLayers()
}

// syncExtraLayers applies the switch for the upper layers to the picture.
func (a *App) syncExtraLayers() {
	if a.extraLayers {
		return
	}
	if a.pic.Active != 0 {
		a.pic.Active = 0
	}
	changed := false
	for l := 1; l < a.pic.LayerCount(); l++ {
		if !a.pic.Hide[l] {
			a.pic.Hide[l] = true
			changed = true
		}
	}
	if changed {
		a.pic.Recomposite()
	}
	a.ditherRes = nil
}

// setExtraLayers switches the layers above layer 2 on or off.
func (a *App) setExtraLayers(on bool) {
	if a.extraLayers == on {
		return
	}
	a.commitOrCancelForHistory()
	a.extraLayers = on
	if on {
		for l := 1; l < a.pic.LayerCount(); l++ {
			a.pic.Hide[l] = false
		}
		a.pic.Recomposite()
		a.ditherRes = nil
		a.setStatus("Upper layers switched on")
		return
	}
	a.syncExtraLayers()
	a.setStatus("Upper layers switched off – their colors still count for the cell rules")
}

func (a *App) setActiveLayer(l int) {
	if l < 0 || l >= a.pic.LayerCount() || a.pic.Active == l {
		return
	}
	if l > 0 && !a.extraLayers {
		a.setExtraLayers(true)
	}
	a.commitOrCancelForHistory()
	a.pic.Active = l
	a.ditherRes = nil
	msg := "Painting on " + a.layerName(l)
	if a.pic.IsFree(l) {
		msg += " (free layer – no color rules)"
	}
	if a.pic.Hide[l] {
		msg += " (hidden – show it to see what you paint)"
	}
	a.setStatus(msg)
}

func (a *App) toggleLayer(l int) {
	if l < 0 || l >= a.pic.LayerCount() {
		return
	}
	if l > 0 && !a.extraLayers {
		a.setStatus("The upper layers are switched off – use the switch next to \"Layers\"")
		return
	}
	a.commitOrCancelForHistory()
	a.pic.SetLayerHidden(l, !a.pic.Hide[l])
	a.ditherRes = nil
	if a.pic.Hide[l] {
		a.setStatus(a.layerName(l) + " hidden – its colors still count for the cell rules")
	} else {
		a.setStatus(a.layerName(l) + " shown")
	}
}

func (a *App) clearActiveLayer() {
	a.commitOrCancelForHistory()
	a.sel.state = selNone
	if a.pic.LayerPixels(a.pic.Active) == 0 {
		a.setStatus(a.layerName(a.pic.Active) + " is already empty")
		return
	}
	a.pushUndo(a.pic.Clone())
	a.pic.ClearLayer(a.pic.Active)
	a.setStatus(a.layerName(a.pic.Active) + " cleared")
}

// layerOp runs a change of the layer list with undo.
func (a *App) layerOp(name string, f func(p *c64.Picture) bool) {
	a.commitOrCancelForHistory()
	a.sel.state = selNone
	before := a.pic.Clone()
	if !f(a.pic) {
		return
	}
	a.pushUndo(before)
	if !a.extraLayers && usesExtraLayers(a.pic) {
		a.extraLayers = true
		for l := 1; l < a.pic.LayerCount(); l++ {
			a.pic.Hide[l] = false
		}
		a.pic.Recomposite()
	}
	a.ditherRes = nil
	a.setStatus(name)
}

func (a *App) addLayer() {
	if a.pic.LayerCount() >= c64.MaxLayers {
		a.setStatus(fmt.Sprintf("At most %d paint layers (plus the template)", c64.MaxLayers))
		return
	}
	a.layerOp("New layer – it is the active layer now", func(p *c64.Picture) bool {
		ok := p.InsertLayer(p.Active + 1)
		p.Recomposite()
		return ok
	})
}

func (a *App) deleteLayer() {
	if a.pic.LayerCount() <= 1 {
		return
	}
	name := a.layerName(a.pic.Active)
	if a.pic.LayerPixels(a.pic.Active) > 0 && a.host.Ask(AppName, "Delete "+name+" with all its pixels?") != 0 {
		return
	}
	a.layerOp(name+" deleted", func(p *c64.Picture) bool { return p.DeleteLayer(p.Active) })
}

func (a *App) moveLayer(dir int) {
	a.layerOp(a.layerName(a.pic.Active)+map[int]string{1: " moved up", -1: " moved down"}[dir], func(p *c64.Picture) bool {
		return p.MoveLayer(p.Active, dir)
	})
}

func (a *App) duplicateLayer() {
	if a.pic.LayerCount() >= c64.MaxLayers {
		a.setStatus(fmt.Sprintf("At most %d paint layers (plus the template)", c64.MaxLayers))
		return
	}
	a.layerOp(a.layerName(a.pic.Active)+" duplicated", func(p *c64.Picture) bool { return p.DuplicateLayer(p.Active) })
}

func (a *App) mergeLayerDown() {
	a.layerOp(a.layerName(a.pic.Active)+" merged into the layer below", func(p *c64.Picture) bool { return p.MergeDown(p.Active, a.pal) })
}

func (a *App) toggleFreeLayer() {
	if a.pic.Mode == c64.ModeFree {
		a.setStatus("In hires free every layer is free anyway")
		return
	}
	l := a.pic.Active
	free := !a.pic.IsFree(l)
	msg := a.layerName(l) + " is free now: it ignores the color rules (C64 files convert the picture when saving)"
	if !free {
		msg = a.layerName(l) + " follows the color rules again – colors that did not fit took the nearest color of their cell"
	}
	a.layerOp(msg, func(p *c64.Picture) bool { p.SetFree(l, free, a.pal); return true })
}

func (a *App) renameLayer() {
	l := a.pic.Active
	a.modal = &promptDialog{title: "Rename layer", label: "Name", tf: newTextField(a.layerName(l), 24), ok: func(s string) {
		a.layerOp("Layer renamed", func(p *c64.Picture) bool {
			if l >= p.LayerCount() {
				return false
			}
			p.Names[l] = strings.TrimSpace(s)
			if p.Names[l] == fmt.Sprintf("Layer %d", l+2) {
				p.Names[l] = ""
			}
			return true
		})
	}}
	a.modal.(*promptDialog).tf.SelectAll()
}

// restoreKeepingView restores a picture (undo/redo) but keeps the current
// visibility of the layers and the active layer (when the list is the same).
func (a *App) restoreKeepingView(p *c64.Picture) {
	if p.LayerCount() != a.pic.LayerCount() {
		a.pic.CopyFrom(p)
		return
	}
	hide := append([]bool(nil), a.pic.Hide...)
	act := a.pic.Active
	a.pic.CopyFrom(p)
	a.pic.Active = act
	same := true
	for l := range hide {
		if a.pic.Hide[l] != hide[l] {
			same = false
		}
	}
	if !same {
		copy(a.pic.Hide, hide)
		a.pic.Recomposite()
	}
}

// promptDialog asks for one line of text.
type promptDialog struct {
	title, label string
	tf           *textField
	ok           func(string)
}

func (d *promptDialog) Title() string          { return d.title }
func (d *promptDialog) Size(a *App) (int, int) { return 420, 150 }
func (d *promptDialog) Char(a *App, r rune)    { d.tf.Char(r) }
func (d *promptDialog) Key(a *App, k KeyEvent) {
	if k.Key == KeyEnter {
		a.closeModal(false)
		d.ok(d.tf.String())
		return
	}
	d.tf.Key(k)
}
func (d *promptDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	a.img.Text(font.UI, r.X, r.Y+6, d.label, a.th.Text)
	d.tf.focus = true
	d.tf.Draw(a, "pd_field", gfx.Rect{X: r.X + 60, Y: r.Y, W: r.W - 60, H: 28})
	by := r.Bottom() - 28
	if u.Button("pd_cancel", gfx.Rect{X: r.Right() - 210, Y: by, W: 100, H: 28}, "Cancel") {
		a.closeModal(false)
	}
	if u.ButtonEx("pd_ok", gfx.Rect{X: r.Right() - 100, Y: by, W: 100, H: 28}, "OK", true, true) {
		a.closeModal(false)
		d.ok(d.tf.String())
	}
}

// drawSwitch draws a small on/off switch and returns the new state.
func (a *App) drawSwitch(id string, r gfx.Rect, on bool, tip string) bool {
	m := a.img
	th := a.th
	track := th.Track
	if on {
		track = th.Accent
	}
	m.RoundRect(r, r.H/2, track)
	k := r.H - 4
	kx := r.X + 2
	if on {
		kx = r.Right() - 2 - k
	}
	m.RoundRect(gfx.Rect{X: kx, Y: r.Y + 2, W: k, H: k}, k/2, 0xFFFFFF)
	c, _ := a.ui.press(id, r.Inset(-3), 0)
	a.ui.Tip(id, r.Inset(-3), tip)
	if c {
		return !on
	}
	return on
}

func (a *App) panelLayers(x, y, w int) int {
	u := &a.ui
	m := a.img
	th := a.th
	hy := y
	y = a.section(x, y, w, "Layers")
	// switch for layers 3 and 4 next to the title
	sw := gfx.Rect{X: x + w - 34, Y: hy + 5, W: 30, H: 14}
	lbl := "Layers 3+"
	m.TextR(font.UI, gfx.Rect{X: sw.X - 110, Y: hy, W: 104, H: 24}, lbl, th.Text)
	if on := a.drawSwitch("ly_extra", sw, a.extraLayers, "Switch the layers above layer 2 on or off"); on != a.extraLayers {
		a.setExtraLayers(on)
	}
	if a.collapsed["Layers"] {
		return y
	}
	row := func(id, name, sub string, visible, active, enabled bool, toggle, choose func()) {
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
		tc, dc := th.Text, th.TextDim
		if !enabled {
			tc = th.TextDim
		}
		eye := gfx.Rect{X: r.X + 4, Y: r.Y + 7, W: 22, H: 20}
		ec := th.TextDim
		if visible && enabled {
			ec = th.Text
		}
		drawIcon(m, "eye", eye.X+3, eye.Y+2, ec, th.Accent)
		if !visible {
			m.Line(eye.X+4, eye.Bottom()-4, eye.Right()-4, eye.Y+4, ec)
		}
		if c, _ := u.press(id+"_eye", eye, 0); c && enabled {
			toggle()
		}
		if enabled {
			u.Tip(id+"_eye", eye, "Show / hide")
		} else {
			u.Tip(id+"_eye", eye, "Switch on the upper layers first (switch next to \"Layers\")")
		}
		body := gfx.Rect{X: r.X + 30, Y: r.Y, W: r.W - 30, H: r.H}
		if choose != nil {
			if c, _ := u.press(id, body, 0); c {
				choose()
			}
			u.Tip(id, body, "Click: paint on this layer")
		}
		m.Text(font.UIB, r.X+32, r.Y+3, gfx.Elide(font.UIB, name, r.W-38), tc)
		m.Text(font.UI, r.X+32, r.Y+18, gfx.Elide(font.UI, sub, r.W-38), dc)
		y += 36
	}
	tname := "(none)"
	if a.tpl.src != nil {
		tname = a.tpl.name
	}
	row("lv_tpl", "Layer 1 · Template", tname, a.tpl.visible && a.tpl.src != nil, false, true,
		func() { a.tpl.visible = !a.tpl.visible }, nil)
	for l := 0; l < a.pic.LayerCount(); l++ {
		l := l
		n := a.pic.LayerPixels(l)
		sub := "empty"
		if n > 0 {
			sub = fmt.Sprintf("%d pixels", n)
		}
		if l == 0 {
			sub += " · bottom"
		} else if l == a.pic.LayerCount()-1 {
			sub += " · top"
		}
		if a.pic.IsFree(l) {
			sub = "FREE · " + sub
		}
		if a.pic.Active == l {
			sub = "active · " + sub
		}
		enabled := l == 0 || a.extraLayers
		if !enabled {
			sub = "switched off"
		}
		row(fmt.Sprintf("lv_l%d", l), a.layerName(l), sub, !a.pic.Hide[l], a.pic.Active == l, enabled,
			func() { a.toggleLayer(l) }, func() { a.setActiveLayer(l) })
		// free switch per layer (not in hires free, where everything is free)
		if a.pic.Mode != c64.ModeFree && enabled {
			fr := gfx.Rect{X: x + w - 30, Y: y - 36 + 8, W: 24, H: 18}
			free := a.pic.IsFree(l)
			col := a.th.TextDim
			if free {
				m.RoundRect(fr, 4, a.th.Accent)
				col = 0xFFFFFF
			} else {
				m.Frame(fr, a.th.Border)
			}
			m.TextC(font.UIB, fr, "F", col)
			if c, _ := u.press(fmt.Sprintf("lv_free%d", l), fr.Inset(-2), 0); c {
				a.setActiveLayer(l)
				a.toggleFreeLayer()
			}
			u.Tip(fmt.Sprintf("lv_free%d", l), fr.Inset(-2), "Free layer: ignores the multicolor/hires color rules (the other layers keep them). C64 files then convert the picture.")
		}
	}
	// layer list tools
	bw3 := (w - 8) / 3
	lb := func(id string, i, row int, label, tip string, enabled bool, f func()) {
		r := gfx.Rect{X: x + i*(bw3+4), Y: y + row*26, W: bw3, H: 22}
		if u.ButtonEx(id, r, label, false, enabled) {
			f()
		}
		u.Tip(id, r, tip)
	}
	cnt, act := a.pic.LayerCount(), a.pic.Active
	lb("ly_add", 0, 0, "+ New", "Add an empty layer above the active one", cnt < c64.MaxLayers, a.addLayer)
	lb("ly_del", 1, 0, "Delete", "Delete the active layer", cnt > 1, a.deleteLayer)
	lb("ly_dup", 2, 0, "Duplicate", "Copy the active layer", cnt < c64.MaxLayers, a.duplicateLayer)
	lb("ly_up", 0, 1, "↑ Up", "Move the active layer up", act < cnt-1, func() { a.moveLayer(1) })
	lb("ly_down", 1, 1, "↓ Down", "Move the active layer down", act > 0, func() { a.moveLayer(-1) })
	lb("ly_merge", 2, 1, "Merge ↓", "Merge the active layer into the layer below", act > 0, a.mergeLayerDown)
	lb("ly_ren", 0, 2, "Rename …", "Give the active layer a name", true, a.renameLayer)
	y += 3*26 + 2
	y += 2
	pr := gfx.Rect{X: x, Y: y, W: w, H: 20}
	if on := u.Checkbox("ly_protect", pr, "Protect other layers on clashes", a.set.ProtectLayers); on != a.set.ProtectLayers {
		a.set.ProtectLayers = on
		a.saveSettings()
	}
	u.Tip("ly_protect", pr, "When a cell is full, only colors of the active layer may be replaced – the other layers stay as they are")
	y += 26
	if u.ButtonEx("lyclear", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Clear "+a.layerName(a.pic.Active), false, a.pic.LayerPixels(a.pic.Active) > 0) {
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
