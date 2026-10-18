package app

import (
	"fmt"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// Modal ist ein Dialogfenster innerhalb des Programms.
type Modal interface {
	Title() string
	Size(a *App) (int, int)
	Draw(a *App, r gfx.Rect)
}

type canceler interface{ Cancel(a *App) }

func (a *App) closeModal(cancel bool) {
	a.modalDragging = false
	a.modalTitle = gfx.Rect{}
	if cancel {
		if c, ok := a.modal.(canceler); ok {
			c.Cancel(a)
		}
	}
	a.modal = nil
}

func (a *App) drawModal() {
	th := a.th
	m := a.img
	md := a.modal
	m.ResetClip()
	if _, light := md.(noDimmer); !light {
		m.FillA(gfx.Rect{X: 0, Y: 0, W: a.W, H: a.H}, 0x000000, 90)
	}
	w, h := md.Size(a)
	w = min(w, a.W-16)
	h = min(h, a.H-16)
	r := a.modalRect(md, w, h)
	m.FillA(gfx.Rect{X: r.X + 6, Y: r.Y + 6, W: r.W, H: r.H}, th.Shadow, 90)
	m.Fill(r, th.Panel)
	m.Frame(r, th.PopupBorder)
	tb := gfx.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: 30}
	m.Fill(tb, th.MenuBar)
	m.HLine(tb.X, tb.Right()-1, tb.Bottom(), th.Border)
	m.TextL(font.UIB, gfx.Rect{X: tb.X + 12, Y: tb.Y, W: tb.W - 50, H: tb.H}, gfx.Elide(font.UIB, md.Title(), tb.W-56), th.MenuText)
	xr := gfx.Rect{X: tb.Right() - 30, Y: tb.Y + 3, W: 26, H: 24}
	if a.ui.hover(xr) {
		m.Fill(xr, th.Danger)
	}
	m.TextC(font.UIB, xr, "✕", th.MenuText)
	if c, _ := a.ui.press("modalx", xr, 0); c {
		a.closeModal(true)
		return
	}
	if a.modalDragging {
		dashedFrame(m, r.Inset(-1), th.Accent, th.Panel, int(a.now/80)%8)
		a.needAnim = true
	}
	content := gfx.Rect{X: r.X + 16, Y: tb.Bottom() + 12, W: r.W - 32, H: r.H - tb.H - 24}
	old := m.SetClip(gfx.Rect{X: r.X + 1, Y: tb.Bottom() + 1, W: r.W - 2, H: r.H - tb.H - 2})
	md.Draw(a, content)
	m.SetClip(old)
}

// noDimmer: dialogs that leave the picture undimmed (to watch live changes).
type noDimmer interface{ NoDim() }

// modalRect places a dialog: centered, plus the offset the user dragged it
// to (remembered per dialog type for this session), always kept inside the
// program window. Dragging the title bar moves it, a double-click centers it.
func (a *App) modalRect(md Modal, w, h int) gfx.Rect {
	key := fmt.Sprintf("%T", md)
	if a.modalOff == nil {
		a.modalOff = map[string][2]int{}
	}
	cx, cy := (a.W-w)/2, (a.H-h)/2
	off := a.modalOff[key]
	clampR := func(x, y int) (int, int) {
		return clampI(x, 0, max(0, a.W-w)), clampI(y, 0, max(0, a.H-h))
	}
	x, y := clampR(cx+off[0], cy+off[1])
	title := gfx.Rect{X: x, Y: y, W: w - 34, H: 31}
	a.modalTitle = title
	in := &a.in
	if !a.modalDragging && title.Contains(in.MX, in.MY) && in.Pressed[0] && a.ui.active == "" {
		if in.Double {
			a.modalOff[key] = [2]int{}
			x, y = clampR(cx, cy)
		} else {
			a.modalDragging = true
			a.modalGrab = [2]int{in.MX - x, in.MY - y}
			a.ui.active = "modaldrag"
		}
		in.Pressed[0] = false
	}
	if a.modalDragging {
		if in.Down[0] {
			x, y = clampR(in.MX-a.modalGrab[0], in.MY-a.modalGrab[1])
			a.ui.active = "modaldrag"
		} else {
			a.modalDragging = false
			a.ui.active = ""
		}
		a.modalOff[key] = [2]int{x - cx, y - cy}
		a.modalTitle = gfx.Rect{X: x, Y: y, W: w - 34, H: 31}
	}
	return gfx.Rect{X: x, Y: y, W: w, H: h}
}

// --- Gitter ----------------------------------------------------------------------

type gridDialog struct{}

func (a *App) openGridDialog() { a.modal = &gridDialog{} }

func (d *gridDialog) Title() string          { return "Grid settings" }
func (d *gridDialog) Size(a *App) (int, int) { return 460, 470 }
func (d *gridDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	x, y, w := r.X, r.Y, r.W
	m.Text(font.UI, x, y, "Sizes in screen pixels (320×200). A multicolor pixel is", th.TextDim)
	y += 16
	m.Text(font.UI, x, y, "2 screen pixels wide. 8×8 = exactly one color cell.", th.TextDim)
	y += 26
	a.set.GridOn = u.Checkbox("g_on", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Show grid (G)", a.set.GridOn)
	y += 30
	lbl := func(s string) { u.Label(gfx.Rect{X: x, Y: y, W: 160, H: 24}, s, false) }
	lbl("Width")
	a.set.GridW = u.Spinner("g_w", gfx.Rect{X: x + 170, Y: y, W: 140, H: 24}, a.set.GridW, 2, 320, 2, " px")
	y += 30
	lbl("Height")
	a.set.GridH = u.Spinner("g_h", gfx.Rect{X: x + 170, Y: y, W: 140, H: 24}, a.set.GridH, 1, 200, 1, " px")
	y += 30
	lbl("Horizontal offset")
	a.set.GridOX = u.Spinner("g_ox", gfx.Rect{X: x + 170, Y: y, W: 140, H: 24}, a.set.GridOX, 0, 318, 2, " px")
	y += 30
	lbl("Vertical offset")
	a.set.GridOY = u.Spinner("g_oy", gfx.Rect{X: x + 170, Y: y, W: 140, H: 24}, a.set.GridOY, 0, 199, 1, " px")
	y += 34
	a.set.GridAlpha = u.Slider("g_a", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Opacity", a.set.GridAlpha, 5, 100, " %")
	y += 30
	u.Label(gfx.Rect{X: x, Y: y, W: w, H: 18}, "Color (A = automatic, inverted)", false)
	y += 20
	sw := (w - 17) / 17
	for i := 0; i <= 16; i++ {
		cr := gfx.Rect{X: x + i*(sw+1), Y: y, W: sw, H: 20}
		if i == 0 {
			m.Fill(cr, th.Btn)
			m.TextC(font.UIB, cr, "A", th.Text)
		} else {
			m.Fill(cr, a.pal.Colors[i-1])
		}
		if a.set.GridColor == i {
			m.Frame(cr.Inset(-1), th.Accent)
			m.Frame(cr, th.Accent)
		}
		if c, _ := u.press("g_c"+itoa(i), cr, 0); c {
			a.set.GridColor = i
		}
	}
	y += 32
	u.Label(gfx.Rect{X: x, Y: y, W: w, H: 18}, "Presets:", true)
	y += 20
	pre := []struct {
		n    string
		w, h int
	}{{"8×8 cell", 8, 8}, {"16×16", 16, 16}, {"32×32", 32, 32}, {"2×1 pixel", 2, 1}, {"4×4", 4, 4}}
	pw := (w - 4*4) / 5
	for i, p := range pre {
		if u.Button("g_p"+itoa(i), gfx.Rect{X: x + i*(pw+4), Y: y, W: pw, H: 24}, p.n) {
			a.set.GridW, a.set.GridH, a.set.GridOX, a.set.GridOY = p.w, p.h, 0, 0
			a.set.GridOn = true
		}
	}
	if u.Button("g_ok", gfx.Rect{X: r.Right() - 110, Y: r.Bottom() - 28, W: 110, H: 28}, "Close") {
		a.saveSettings()
		a.closeModal(false)
	}
}

// --- Palette ----------------------------------------------------------------------

type paletteDialog struct {
	orig   int
	scroll int
}

func (a *App) openPaletteDialog() {
	d := &paletteDialog{orig: a.palIdx}
	d.scroll = max(0, a.palIdx-4) * 28
	a.modal = d
}

func (d *paletteDialog) Title() string {
	return fmt.Sprintf("Choose palette (%d palettes)", len(c64.Palettes))
}
func (d *paletteDialog) Size(a *App) (int, int) { return 640, min(640, a.H-40) }
func (d *paletteDialog) Cancel(a *App)          { a.setPalette(d.orig) }
func (d *paletteDialog) Key(a *App, k KeyEvent) {
	switch k.Key {
	case KeyUp:
		a.setPalette(max(0, a.palIdx-1))
	case KeyDown:
		a.setPalette(min(len(c64.Palettes)-1, a.palIdx+1))
	case KeyEnter:
		a.closeModal(false)
	}
}
func (d *paletteDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	m.Text(font.UI, r.X, r.Y, "Interpretations of the 16 C64 colors (source: c64gfx.com/palettes).", th.TextDim)
	m.Text(font.UI, r.X, r.Y+16, "The palette only changes the display and PNG export – KLA/PRG stay the same.", th.TextDim)
	lr := gfx.Rect{X: r.X, Y: r.Y + 40, W: r.W, H: r.H - 40 - 40}
	m.Fill(lr, th.PanelAlt)
	m.Frame(lr, th.Border)
	const rh = 28
	total := len(c64.Palettes) * rh
	d.scroll = clampI(d.scroll, 0, max(0, total-lr.H+2))
	if u.hover(lr) && a.in.Wheel != 0 {
		d.scroll = clampI(d.scroll-a.in.Wheel*rh*2, 0, max(0, total-lr.H+2))
	}
	old := m.SetClip(lr.Inset(1))
	sw := 16
	for i, p := range c64.Palettes {
		y := lr.Y + 1 + i*rh - d.scroll
		if y+rh < lr.Y || y > lr.Bottom() {
			continue
		}
		row := gfx.Rect{X: lr.X + 1, Y: y, W: lr.W - 12, H: rh}
		tc := th.Text
		if i == a.palIdx {
			m.Fill(row, th.Accent)
			tc = th.AccentText
		} else if u.hover(row) {
			m.Fill(row, th.BtnHover)
		}
		m.TextL(font.UI, gfx.Rect{X: row.X + 8, Y: row.Y, W: 250, H: rh}, gfx.Elide(font.UI, p.Name, 250), tc)
		for k := 0; k < 16; k++ {
			m.Fill(gfx.Rect{X: row.X + 270 + k*(sw+1), Y: row.Y + 5, W: sw, H: rh - 10}, p.Colors[k])
		}
		if p.Note != "" {
			m.TextL(font.UIB, gfx.Rect{X: row.Right() - 18, Y: row.Y, W: 14, H: rh}, "*", tc)
			u.Tip("paln"+itoa(i), row, p.Note)
		}
		if c, _ := u.press("pal_"+itoa(i), row, 0); c {
			a.setPalette(i)
			if a.in.Double {
				a.closeModal(false)
			}
		}
	}
	m.SetClip(old)
	// Scrollbalken
	if total > lr.H {
		tr := gfx.Rect{X: lr.Right() - 9, Y: lr.Y + 2, W: 6, H: lr.H - 4}
		m.Fill(tr, th.Track)
		thh := max(20, tr.H*lr.H/total)
		ty := tr.Y + (tr.H-thh)*d.scroll/max(1, total-lr.H+2)
		m.Fill(gfx.Rect{X: tr.X, Y: ty, W: tr.W, H: thh}, th.Accent)
	}
	by := r.Bottom() - 28
	m.TextL(font.UI, gfx.Rect{X: r.X, Y: by, W: 300, H: 28}, "Current: "+a.pal.Name, th.Text)
	if u.Button("pal_cancel", gfx.Rect{X: r.Right() - 230, Y: by, W: 110, H: 28}, "Cancel") {
		a.closeModal(true)
		return
	}
	if u.ButtonEx("pal_ok", gfx.Rect{X: r.Right() - 110, Y: by, W: 110, H: 28}, "OK", true, true) {
		a.closeModal(false)
	}
}

// --- Transformieren ------------------------------------------------------------------

type transformDialog struct{}

func (a *App) openTransformDialog() {
	if a.sel.state == selNone {
		return
	}
	a.modal = &transformDialog{}
}
func (d *transformDialog) Title() string          { return "Rotate, scale and shear selection" }
func (d *transformDialog) NoDim()                 {}
func (d *transformDialog) Size(a *App) (int, int) { return 420, 380 }
func (d *transformDialog) Draw(a *App, r gfx.Rect) {
	m := a.img
	m.Text(font.UI, r.X, r.Y, "Changes are shown in the picture right away. The color", a.th.TextDim)
	m.Text(font.UI, r.X, r.Y+16, "cell rules also apply to the result.", a.th.TextDim)
	a.transformControls(r.X, r.Y+40, r.W, "d")
	if a.ui.ButtonEx("td_ok", gfx.Rect{X: r.Right() - 110, Y: r.Bottom() - 28, W: 110, H: 28}, "Close", true, true) {
		a.closeModal(false)
	}
}

// --- About ------------------------------------------------------------------------------

type aboutDialog struct{}

func (a *App) openAbout() {
	a.menuOpen = -1
	a.modal = &aboutDialog{}
}

func (d *aboutDialog) Title() string          { return "About " + AppName }
func (d *aboutDialog) Size(a *App) (int, int) { return 640, min(690, a.H-40) }
func (d *aboutDialog) Key(a *App, k KeyEvent) {
	if k.Key == KeyEnter {
		a.closeModal(false)
	}
}

// aboutSources: sources and thanks (also in the help page "About").
var aboutSources = [][2]string{
	{"Palettes", "c64gfx.com palette collection (two missing colors from Pepto's palette)"},
	{"C64 basics", "C64-Wiki and retro-programming.de"},
	{"Font", "DejaVu Sans (Bitstream Vera / DejaVu fonts license)"},
	{"macOS", "purego by Ebitengine (Apache 2.0)"},
	{"Linux", "xgb by BurntSushi and jezek (BSD)"},
	{"Windows", "rsrc by akavel (MIT) for the icon resource"},
	{"Language", "Go and its standard library (BSD), The Go Authors"},
	{"Dithering", "pixelparmesan.com, pixel-editor.com, pixnote.net, divoom.com, pixie.haus"},
	{"Dither styles", "16 hand-pixelled C64 scene pictures, analyzed pattern by pattern"},
	{"Gradients", "color ramp table by IT-Dummköpfen, extended"},
	{"Inspiration", "Deluxe Paint (Electronic Arts, 1985), PyDPainter and Multipaint (Marq/Kameli)"},
}

func (d *aboutDialog) Draw(a *App, r gfx.Rect) {
	m := a.img
	th := a.th
	// logo: the program icon as big pixel art
	lr := gfx.Rect{X: r.X + (r.W-64)/2, Y: r.Y + 4, W: 64, H: 64}
	drawBigLogo(m, lr)
	y := lr.Bottom() + 12
	center := func(f *font.Face, s string, c uint32, gap int) {
		m.Text(f, r.X+(r.W-f.Width(s))/2, y, s, c)
		y += f.Height + gap
	}
	center(font.H1, AppName, th.Text, 6)
	center(font.BodyB, "Version "+AppVersion, th.Accent, 10)
	center(font.Body, "Created by "+AppAuthor, th.Text, 4)
	center(font.Body, AppLicense, th.Text, 12)
	center(font.UI, "A paint program for the C64 multicolor bitmap mode for", th.TextDim, 4)
	center(font.UI, "Windows 11 (Windows 10 may crash) · macOS (M chips only) · Linux", th.TextDim, 10)
	center(font.UI, "Inspired by Deluxe Paint (Electronic Arts, 1985), PyDPainter and Multipaint.", th.Text, 4)
	center(font.UI, "I was also inspired by many comments from users. Thank you so much! \u2764", th.Text, 14)
	// sources and thanks
	m.HLine(r.X, r.Right()-1, y, th.Border)
	y += 8
	center(font.UIB, "Sources & thanks", th.Text, 4)
	center(font.UI, "Thank you all for making your work available and letting us use it.", th.TextDim, 8)
	kw := 0
	for _, s := range aboutSources {
		kw = max(kw, font.UIB.Width(s[0]))
	}
	for _, s := range aboutSources {
		if y+font.UI.Height > r.Bottom()-36 {
			break
		}
		m.Text(font.UIB, r.X+8, y, s[0], th.Text)
		m.Text(font.UI, r.X+8+kw+12, y, gfx.Elide(font.UI, s[1], r.W-kw-28), th.TextDim)
		y += font.UI.Height + 4
	}
	by := r.Bottom() - 28
	if a.ui.Button("ab_help", gfx.Rect{X: r.X, Y: by, W: 120, H: 28}, "More in Help") {
		a.modal = nil
		a.openHelp("about")
		return
	}
	if a.ui.ButtonEx("ab_ok", gfx.Rect{X: r.Right() - 110, Y: by, W: 110, H: 28}, "Close", true, true) {
		a.closeModal(false)
	}
}
