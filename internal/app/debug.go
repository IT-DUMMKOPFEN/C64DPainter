package app

import "c64dpainter/internal/c64"

// Hilfsfunktionen für das automatische Testprogramm.

// PicToScreenCenter liefert die Bildschirmmitte eines Bildpunkts.
func (a *App) PicToScreenCenter(x, y int) (int, int) {
	sx, sy := a.picToScreen(x, y)
	return sx + a.zoom, sy + a.zoom/2
}

// Debug setzt interne Zustände für Tests.
func (a *App) Debug(what string, v int) {
	switch what {
	case "bg":
		a.setBG(uint8(v))
	case "fg":
		a.fg = uint8(v)
	case "fg2":
		a.fg2 = uint8(v)
	case "pattern":
		a.pattern = v
	case "rotate":
		if a.selEnsureFloating() {
			a.sel.xf.angle = float64(v)
			a.selRender()
		}
	case "theme":
		if v == 1 {
			a.set.Theme = "dark"
		} else {
			a.set.Theme = "light"
		}
		a.applyTheme()
	case "help":
		a.openHelp("rules")
	case "exportpng":
		a.later(func() { a.cmdExport(exportPNG320) })
		a.RunDeferred()
	case "loadtpl":
		a.later(a.loadTemplateDialog)
		a.RunDeferred()
	case "helptopic":
		a.openHelp(helpTopics[v].id)
	case "helpscroll":
		if h, ok := a.modal.(*helpDialog); ok {
			h.scroll = v
		}
	case "importdither":
		if d, ok := a.modal.(*importDialog); ok {
			d.opts.Dither = c64.DitherMode(v)
			d.opts.Strength = 70
			d.opts.Adjust.Contrast = 15
			d.opts.Adjust.Saturation = 20
		}
	case "importview":
		if d, ok := a.modal.(*importDialog); ok {
			d.view = v
		}
	case "ditherview":
		if d, ok := a.modal.(*autoDitherDialog); ok {
			d.view = v
		}
	case "ditherstyle":
		if d, ok := a.modal.(*autoDitherDialog); ok {
			d.opts.Style = c64.DitherStyle(v)
		}
	case "importok":
		if d, ok := a.modal.(*importDialog); ok {
			*a.pic = *d.preview
			a.closeModal(false)
		}
	case "clear":
		a.clearPicture()
	case "open":
		// v unused: opens the file the host returns
		a.later(a.cmdOpen)
		a.RunDeferred()
	case "swapdlg": // v = from<<4 | to
		a.openSwapDialog(false)
		if d, ok := a.modal.(*swapDialog); ok {
			d.from, d.to = uint8(v>>4), uint8(v&15)
		}
	case "transformdlg":
		a.openTransformDialog()
	case "palettedlg":
		a.openPaletteDialog()
	case "about":
		a.openAbout()
	case "helpdither":
		a.openHelp("dither")
	case "dithersettings": // brush: v = style
		a.set.DitherBrush.Style = c64.DitherStyle(v)
	case "smearstrength":
		a.set.SmearStrength = v
	case "panelscroll":
		a.panelScroll = v
	}
}
