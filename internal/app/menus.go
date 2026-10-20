package app

import (
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// MenuItem ist ein Menüeintrag. Heading = nicht anklickbare Zwischenüberschrift.
type MenuItem struct {
	Label, Key string
	Action     func()
	Check      func() bool
	Enabled    func() bool
	Sep        bool
	Heading    bool
}

type Menu struct {
	Title string
	Items []MenuItem
	x, w  int
}

func sep() MenuItem                       { return MenuItem{Sep: true} }
func head(s string) MenuItem              { return MenuItem{Label: s, Heading: true} }
func item(l, k string, f func()) MenuItem { return MenuItem{Label: l, Key: k, Action: f} }
func check(l, k string, c func() bool, f func()) MenuItem {
	return MenuItem{Label: l, Key: k, Action: f, Check: c}
}

func (a *App) buildMenus() {
	hasSel := func() bool { return a.sel.state == selSelected || a.sel.state == selFloating }
	selOrBrush := func() bool { return hasSel() || a.brushShape == BrushCustom }
	a.menus = []*Menu{
		{Title: "File", Items: append(append([]MenuItem{
			item("New", "Ctrl+N", func() { a.later(a.cmdNew) }),
			item("Open … (C64DP, KLA, OCP, AMI, PNG, BMP)", "Ctrl+O", func() { a.later(a.cmdOpen) }),
			item("Import image (320×200) …", "Ctrl+I", func() { a.later(a.cmdImport) }),
			item("Save", "Ctrl+S", func() { a.later(func() { a.cmdSave() }) }),
			item("Save as …", "Ctrl+Shift+S", func() { a.later(func() { a.cmdSaveAs() }) }),
			item("Quick save PNG (numbered)", "F2", a.quickSave),
			item("Working folder …", "", func() { a.askWorkDir(nil) }),
			sep(),
			head("Recent files"),
		}, a.recentItems()...), []MenuItem{
			sep(),
			head("Export"),
			item("Koala Painter (.kla) …", "", func() { a.later(func() { a.cmdExport(exportKLA) }) }),
			item("C64 program (.prg) …", "Ctrl+E", func() { a.later(func() { a.cmdExport(exportPRG) }) }),
			item("PNG 320×200 …", "", func() { a.later(func() { a.cmdExport(exportPNG320) }) }),
			item("PNG 160×200 (native pixels) …", "", func() { a.later(func() { a.cmdExport(exportPNG160) }) }),
			item("PNG 384×272 with border …", "", func() { a.later(func() { a.cmdExport(exportPNGBorder) }) }),
			sep(),
			item("Load template …", "", func() { a.later(a.loadTemplateDialog) }),
			sep(),
			item("Exit", "Ctrl+Q", func() { a.later(a.cmdQuit) }),
		}...)},
		{Title: "Edit", Items: []MenuItem{
			{Label: "Undo", Key: "Ctrl+Z  U", Action: a.undo, Enabled: func() bool { return len(a.undos[a.page]) > 0 }},
			{Label: "Redo", Key: "Ctrl+Y", Action: a.redo, Enabled: func() bool { return len(a.redos[a.page]) > 0 }},
			sep(),
			{Label: "Cut", Key: "Ctrl+X", Action: a.selCut, Enabled: hasSel},
			{Label: "Copy", Key: "Ctrl+C", Action: a.selCopy, Enabled: hasSel},
			{Label: "Paste", Key: "Ctrl+V", Action: a.selPaste, Enabled: func() bool { return a.clip != nil }},
			{Label: "Delete selection", Key: "Del", Action: a.selDelete, Enabled: hasSel},
			item("Select all", "Ctrl+A", a.selectAll),
			sep(),
			item("Clear picture", "Shift+K", a.clearPicture),
			item("Swap page (spare page)", "J", a.swapPage),
			item("Copy picture to other page", "", a.copyToSpare),
			sep(),
			item("Auto dither … (experimental)", "Shift+A", func() { a.later(a.openAutoDither) }),
			check("Dither brush (experimental)", "Shift+D", func() bool { return a.tool == ToolDither }, func() { a.setTool(ToolDither) }),
			sep(),
			item("Swap color in picture …", "", func() { a.openSwapDialog(false) }),
			check("Swap color in cells (brush) …", "", func() bool { return a.tool == ToolCellSwap }, func() { a.openSwapDialog(true) }),
			sep(),
			item("Clean up unused cell colors", "", func() {
				a.pushUndo(a.pic.Clone())
				a.pic.CleanUnused()
				a.setStatus("Unused bit-pair colors set to black (picture unchanged)")
			}),
		}},
		{Title: "Selection", Items: []MenuItem{
			{Label: "Flip horizontally", Key: "X", Action: func() { a.transformQuick(qFlipX) }, Enabled: selOrBrush},
			{Label: "Flip vertically", Key: "Y", Action: func() { a.transformQuick(qFlipY) }, Enabled: selOrBrush},
			{Label: "Rotate 90° clockwise", Key: "Z", Action: func() { a.transformQuick(qRot90) }, Enabled: selOrBrush},
			{Label: "Rotate 90° counter-clockwise", Key: "Shift+Z", Action: func() { a.transformQuick(qRot270) }, Enabled: selOrBrush},
			{Label: "Half size", Key: "H", Action: func() { a.transformQuick(qHalf) }, Enabled: selOrBrush},
			{Label: "Double size", Key: "Shift+H", Action: func() { a.transformQuick(qDouble) }, Enabled: selOrBrush},
			sep(),
			{Label: "Rotate, scale, shear …", Key: "", Action: a.openTransformDialog, Enabled: hasSel},
			{Label: "Reset transformation", Key: "", Action: func() { a.transformQuick(qReset) }, Enabled: func() bool { return a.sel.state == selFloating }},
			sep(),
			{Label: "Apply", Key: "Enter", Action: a.selCommit, Enabled: func() bool { return a.sel.state == selFloating }},
			{Label: "Cancel", Key: "Esc", Action: a.escape, Enabled: hasSel},
			sep(),
			{Label: "Use selection as brush", Key: "", Action: a.selToBrush, Enabled: hasSel},
			{Label: "Standard brush", Key: "Shift+B", Action: a.restoreBrush, Enabled: func() bool { return a.brushShape == BrushCustom }},
			check("Background color transparent", "", func() bool { return a.sel.transpBG }, func() { a.sel.transpBG = !a.sel.transpBG }),
		}},
		{Title: "Layer", Items: []MenuItem{
			check("Use layers 3 + 4", "", func() bool { return a.extraLayers }, func() { a.setExtraLayers(!a.extraLayers) }),
			sep(),
			head("Paint on"),
			check("Layer 2 (front)", "2", func() bool { return a.pic.Active == 0 }, func() { a.setActiveLayer(0) }),
			check("Layer 3", "3", func() bool { return a.pic.Active == 1 }, func() { a.setActiveLayer(1) }),
			check("Layer 4 (back)", "4", func() bool { return a.pic.Active == 2 }, func() { a.setActiveLayer(2) }),
			sep(),
			head("Show"),
			check("Layer 1 (template)", "T", func() bool { return a.tpl.src != nil && a.tpl.visible }, func() {
				if a.tpl.src == nil {
					a.later(a.loadTemplateDialog)
				} else {
					a.tpl.visible = !a.tpl.visible
				}
			}),
			check("Layer 2", "", func() bool { return !a.pic.Hide[0] }, func() { a.toggleLayer(0) }),
			{Label: "Layer 3", Action: func() { a.toggleLayer(1) }, Check: func() bool { return !a.pic.Hide[1] }, Enabled: func() bool { return a.extraLayers }},
			{Label: "Layer 4", Action: func() { a.toggleLayer(2) }, Check: func() bool { return !a.pic.Hide[2] }, Enabled: func() bool { return a.extraLayers }},
			sep(),
			item("Clear active layer", "", a.clearActiveLayer),
			item("Load template …", "", func() { a.later(a.loadTemplateDialog) }),
		}},
		{Title: "Brushes", Items: []MenuItem{
			item("My brushes …", "", func() { a.later(a.openStamps) }),
			sep(),
			item("Load brush …", "", func() { a.later(a.loadBrushDialog) }),
			{Label: "Save brush …", Action: func() { a.later(a.saveBrushDialog) }, Enabled: func() bool { return a.brushShape == BrushCustom && a.custom != nil }},
			sep(),
			{Label: "Selection as brush", Action: a.selToBrush, Enabled: hasSel},
			item("Standard brush", "Shift+B", a.restoreBrush),
			sep(),
			{Label: "Flip horizontally", Key: "X", Action: func() { a.transformQuick(qFlipX) }, Enabled: selOrBrush},
			{Label: "Flip vertically", Key: "Y", Action: func() { a.transformQuick(qFlipY) }, Enabled: selOrBrush},
			{Label: "Rotate 90°", Key: "Z", Action: func() { a.transformQuick(qRot90) }, Enabled: selOrBrush},
			{Label: "Half size", Key: "H", Action: func() { a.transformQuick(qHalf) }, Enabled: selOrBrush},
			{Label: "Double size", Key: "Shift+H", Action: func() { a.transformQuick(qDouble) }, Enabled: selOrBrush},
			sep(),
			{Label: "Brush keeps its own colors", Check: func() bool { return a.customOwnColor }, Action: func() { a.customOwnColor = !a.customOwnColor }, Enabled: func() bool { return a.brushShape == BrushCustom && a.custom != nil }},
		}},
		{Title: "View", Items: []MenuItem{
			item("Zoom in", ">", func() { a.zoomStep(1, -1, -1) }),
			item("Zoom out", "<", func() { a.zoomStep(-1, -1, -1) }),
			item("Fit to window", "0", a.zoomFit),
			item("Actual size (100 %)", "Ctrl+1", func() { a.setZoomCentered(1) }),
			sep(),
			check("Grid", "G", func() bool { return a.set.GridOn }, func() { a.set.GridOn = !a.set.GridOn; a.saveSettings() }),
			item("Grid settings …", "Shift+G", a.openGridDialog),
			check("Pixel grid (from zoom 4×)", "M", func() bool { return a.set.PixelGrid }, func() { a.set.PixelGrid = !a.set.PixelGrid; a.saveSettings() }),
			check("Highlight cell under the mouse", "", func() bool { return a.set.CellHint }, func() { a.set.CellHint = !a.set.CellHint; a.saveSettings() }),
			check("Show border ($D020)", "", func() bool { return a.set.ShowBorder }, func() { a.set.ShowBorder = !a.set.ShowBorder; a.saveSettings(); a.zoomFit() }),
			check("Side panel", "Tab", func() bool { return a.set.ShowPanel }, func() { a.set.ShowPanel = !a.set.ShowPanel; a.saveSettings() }),
			sep(),
			check("Show template", "T", func() bool { return a.tpl.visible }, func() { a.tpl.visible = !a.tpl.visible }),
			sep(),
			check("Mirrored view", "Shift+M", func() bool { return a.mirrorView }, a.toggleMirrorView),
			check("CRT preview window", "Shift+V", func() bool { return a.crt.open }, a.toggleCRT),
			check("Symmetry (paint mirrored)", "Shift+X", func() bool { return a.set.Sym.On }, a.toggleSymmetry),
		}},
		{Title: "Options", Items: []MenuItem{
			item("Choose palette …", "P", a.openPaletteDialog),
			sep(),
			head("Color conflict (cell full)"),
			check("Block (strict)", "", func() bool { return a.set.Conflict == 0 }, func() { a.setConflict(0) }),
			check("Replace rarest cell color", "", func() bool { return a.set.Conflict == 1 }, func() { a.setConflict(1) }),
			check("Use nearest cell color", "", func() bool { return a.set.Conflict == 2 }, func() { a.setConflict(2) }),
			sep(),
			check("Auto backup (every 3 minutes)", "", func() bool { return a.autoBackup }, a.toggleAutoBackup),
			sep(),
			check("Round brushes on screen (correct 2:1)", "", func() bool { return a.set.AspectBrush }, func() { a.set.AspectBrush = !a.set.AspectBrush; a.saveSettings() }),
			sep(),
			head("Theme"),
			check("Light", "", func() bool { return a.set.Theme != "dark" }, func() { a.set.Theme = "light"; a.applyTheme(); a.saveSettings() }),
			check("Dark", "", func() bool { return a.set.Theme == "dark" }, func() { a.set.Theme = "dark"; a.applyTheme(); a.saveSettings() }),
			sep(),
			head("Interface size"),
			{Label: "100 %", Check: func() bool { return a.UIScale() == 1 }, Action: func() { a.setUIScale(1) }},
			{Label: "200 %", Check: func() bool { return a.UIScale() == 2 }, Action: func() { a.setUIScale(2) }, Enabled: func() bool { return a.scaleFits(2) }},
			{Label: "300 %", Check: func() bool { return a.UIScale() == 3 }, Action: func() { a.setUIScale(3) }, Enabled: func() bool { return a.scaleFits(3) }},
		}},
		{Title: "Help", Items: []MenuItem{
			item("Help contents", "F1", func() { a.openHelp("") }),
			item("Getting started", "", func() { a.openHelp("start") }),
			item("Multicolor rules", "", func() { a.openHelp("rules") }),
			item("Tools", "", func() { a.openHelp("tools") }),
			item("Keyboard shortcuts", "?", func() { a.openHelp("keys") }),
			item("Import & convert", "", func() { a.openHelp("import") }),
			item("File formats", "", func() { a.openHelp("formats") }),
			sep(),
			item("About C64 DPainter", "", a.openAbout),
		}},
	}
}

func (a *App) drawMenuBar() {
	th := a.th
	m := a.img
	r := a.rMenu
	m.Fill(r, th.MenuBar)
	m.HLine(r.X, r.Right()-1, r.Bottom()-1, th.Border)
	// Programmname
	x := r.X + 10
	logo := gfx.Rect{X: x - 1, Y: r.Y + 4, W: 16, H: 16}
	drawLogo(m, logo)
	x += 20
	x += m.Text(font.UIB, x, r.Y+(r.H-font.UIB.Height)/2, AppName, th.MenuText) + 18
	for i, mn := range a.menus {
		w := font.UI.Width(mn.Title) + 20
		mr := gfx.Rect{X: x, Y: r.Y, W: w, H: r.H - 1}
		mn.x, mn.w = x, w
		hov := !a.ui.blocked && mr.Contains(a.in.MX, a.in.MY)
		if hov && a.in.Pressed[0] {
			if a.menuOpen == i {
				a.menuOpen = -1
			} else {
				a.menuOpen = i
				a.menuHover = -1
			}
			a.in.Pressed[0] = false
		} else if hov && a.menuOpen >= 0 && a.menuOpen != i {
			a.menuOpen = i
			a.menuHover = -1
		}
		tc := th.MenuText
		if a.menuOpen == i {
			m.Fill(mr, th.MenuSel)
			tc = th.MenuSelText
		} else if hov {
			m.FillA(mr, th.MenuSel, 40)
		}
		m.TextC(font.UI, mr, mn.Title, tc)
		x += w
	}
	// Hilfe-Knopf rechts
	hr := gfx.Rect{X: r.Right() - 30, Y: r.Y + 2, W: 26, H: r.H - 5}
	if a.ui.hover(hr) && a.menuOpen < 0 {
		m.FillA(hr, th.MenuSel, 40)
	}
	drawIcon(m, "help", hr.X+5, hr.Y+2, th.MenuText, th.Accent)
	if c, _ := a.ui.press("helpbtn", hr, 0); c {
		a.openHelp("")
	}
	a.ui.Tip("helpbtn", hr, "Help (F1)")
	// Seite / Theme-Schalter
	tr := gfx.Rect{X: r.Right() - 60, Y: r.Y + 2, W: 26, H: r.H - 5}
	if a.ui.hover(tr) && a.menuOpen < 0 {
		m.FillA(tr, th.MenuSel, 40)
	}
	drawThemeIcon(m, tr, th.MenuText, th.MenuBar, a.set.Theme == "dark")
	if c, _ := a.ui.press("themebtn", tr, 0); c {
		if a.set.Theme == "dark" {
			a.set.Theme = "light"
		} else {
			a.set.Theme = "dark"
		}
		a.applyTheme()
		a.saveSettings()
	}
	a.ui.Tip("themebtn", tr, "Toggle light / dark")
	if a.page == 1 {
		s := "Page 2"
		w := font.UIB.Width(s) + 12
		pr := gfx.Rect{X: tr.X - w - 8, Y: r.Y + 4, W: w, H: r.H - 9}
		m.Fill(pr, th.Accent)
		m.TextC(font.UIB, pr, s, th.AccentText)
	}
}

func drawThemeIcon(m *gfx.Image, r gfx.Rect, c, bg uint32, dark bool) {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	if dark {
		circle(m, cx, cy, 6, c)
		circle(m, cx+3, cy-3, 5, bg)
		return
	}
	circleOutline(m, cx, cy, 4, c)
	for _, d := range [8][2]int{{0, -7}, {0, 7}, {-7, 0}, {7, 0}, {5, 5}, {-5, -5}, {5, -5}, {-5, 5}} {
		m.Set(cx+d[0], cy+d[1], c)
		m.Set(cx+d[0]*6/7, cy+d[1]*6/7, c)
	}
}

func circleOutline(m *gfx.Image, cx, cy, r int, c uint32) {
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			d := x*x + y*y
			if d <= r*r && d > (r-1)*(r-1) {
				m.Set(cx+x, cy+y, c)
			}
		}
	}
}

func (a *App) drawMenuDropdown() {
	th := a.th
	m := a.img
	mn := a.menus[a.menuOpen]
	f := font.UI
	// Größe
	w := 0
	for _, it := range mn.Items {
		lw := f.Width(it.Label) + 46
		if it.Key != "" {
			lw += f.Width(L(it.Key)) + 30
		}
		if lw > w {
			w = lw
		}
	}
	const ih, sh, hh = 24, 9, 22
	h := 6
	for _, it := range mn.Items {
		switch {
		case it.Sep:
			h += sh
		case it.Heading:
			h += hh
		default:
			h += ih
		}
	}
	x := mn.x
	if x+w > a.W-4 {
		x = a.W - 4 - w
	}
	r := gfx.Rect{X: x, Y: a.rMenu.Bottom() - 1, W: w, H: h}
	m.FillA(gfx.Rect{X: r.X + 4, Y: r.Y + 4, W: r.W, H: r.H}, th.Shadow, 70)
	m.Fill(r, th.Popup)
	m.Frame(r, th.PopupBorder)
	y := r.Y + 3
	clicked := -1
	for i, it := range mn.Items {
		switch {
		case it.Sep:
			m.HLine(r.X+8, r.Right()-9, y+sh/2, th.Border)
			y += sh
			continue
		case it.Heading:
			m.TextL(font.UIB, gfx.Rect{X: r.X + 12, Y: y, W: w - 20, H: hh}, it.Label, th.TextDim)
			y += hh
			continue
		}
		ir := gfx.Rect{X: r.X + 2, Y: y, W: w - 4, H: ih}
		en := it.Enabled == nil || it.Enabled()
		hov := ir.Contains(a.in.MX, a.in.MY)
		tc := th.Text
		kc := th.TextDim
		if hov && en {
			m.Fill(ir, th.MenuSel)
			tc, kc = th.MenuSelText, th.MenuSelText
			if a.in.Released[0] || a.in.Pressed[0] {
				clicked = i
			}
		}
		if !en {
			tc = th.TextDim
		}
		if it.Check != nil && it.Check() {
			// Häkchen
			cx, cy := ir.X+10, ir.Y+ih/2
			for k := 0; k < 3; k++ {
				m.Set(cx+k, cy+k-1, tc)
				m.Set(cx+k, cy+k, tc)
			}
			for k := 0; k < 6; k++ {
				m.Set(cx+3+k, cy+1-k, tc)
				m.Set(cx+3+k, cy-k, tc)
			}
		}
		m.TextL(f, gfx.Rect{X: ir.X + 26, Y: ir.Y, W: ir.W, H: ih}, it.Label, tc)
		if it.Key != "" {
			m.TextR(f, gfx.Rect{X: ir.X, Y: ir.Y, W: ir.W - 12, H: ih}, L(it.Key), kc)
		}
		y += ih
	}
	if clicked >= 0 {
		it := mn.Items[clicked]
		if it.Enabled == nil || it.Enabled() {
			a.menuOpen = -1
			if it.Action != nil {
				it.Action()
			}
		}
		a.in.Pressed[0] = false
		return
	}
	// Klick außerhalb schließt das Menü
	if a.in.Pressed[0] || a.in.Pressed[1] {
		if !r.Contains(a.in.MX, a.in.MY) && !a.rMenu.Contains(a.in.MX, a.in.MY) {
			a.menuOpen = -1
		}
	}
}
