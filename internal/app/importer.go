package app

import (
	"fmt"
	"image"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// importPreset is a ready-made combination of the converter settings.
type importPreset struct {
	name, tip string
	opts      func(c64.ConvertOptions) c64.ConvertOptions
}

// importPresets: the first one is the default.
var importPresets = []importPreset{
	{"Recommended", "Bayer 4×4, YUV, best fit, automatic contrast – calm patterns, true colors", func(o c64.ConvertOptions) c64.ConvertOptions {
		o.Dither, o.Strength, o.Match, o.Cells, o.Serpentine = c64.DitherBayer4, 50, c64.MatchYUV, c64.CellsBestFit, true
		o.Adjust.AutoLevels = 1
		return o
	}},
	{"Retropixels look", "As the web version of Retropixels: Bayer 2×2, YUV, most frequent colors, automatic contrast", func(o c64.ConvertOptions) c64.ConvertOptions {
		o.Dither, o.Strength, o.Match, o.Cells, o.Serpentine = c64.DitherBayer2, 50, c64.MatchYUV, c64.CellsFrequent, true
		o.Adjust.AutoLevels = 1
		return o
	}},
	{"Smooth", "Floyd–Steinberg error diffusion, YUV, best fit, automatic contrast – soft transitions", func(o c64.ConvertOptions) c64.ConvertOptions {
		o.Dither, o.Strength, o.Match, o.Cells, o.Serpentine = c64.DitherFloyd, 60, c64.MatchYUV, c64.CellsBestFit, true
		o.Adjust.AutoLevels = 1
		return o
	}},
	{"Sharp", "No dithering, YUV, best fit, automatic contrast – flat areas, good for graphics and logos", func(o c64.ConvertOptions) c64.ConvertOptions {
		o.Dither, o.Strength, o.Match, o.Cells = c64.DitherNone, 0, c64.MatchYUV, c64.CellsBestFit
		o.Adjust.AutoLevels = 1
		return o
	}},
}

// importDialog converts PNG/BMP images (ideally 320×200) to multicolor with a
// live preview, image adjustments and a choice of dithering methods.
type importDialog struct {
	img    image.Image
	name   string
	opts   c64.ConvertOptions
	target int // 0 = picture, 1 = template only, 2 = both
	view   int // 0 = multicolor, 1 = original, 2 = split

	screen    [][3]int // 320×200 resampled source
	screenFit c64.FitMode
	adjusted  [][3]int // adjusted 320×200 (for the original view)
	adjKey    string
	preview   *c64.Picture
	key       string
	scroll    int
	colH      int

	thumbs   [4][]uint32 // preset previews (thumbW × thumbH)
	thumbKey [4]string
	thumbW   int
	thumbH   int
	autoWas  int // automatic contrast before it was switched off for a C64-sized image
}

func newImportDialog(a *App, img image.Image, name string) *importDialog {
	d := &importDialog{img: img, name: name, opts: a.lastImport}
	if b := img.Bounds(); (b.Dx() == 320 || b.Dx() == 160) && b.Dy() == 200 {
		// most likely already a C64 picture: do not change its colors
		d.autoWas = d.opts.Adjust.AutoLevels
		d.opts.Adjust.AutoLevels = 0
	}
	d.screenFit = -1
	return d
}

// openImportDialog asks for a file and opens the converter.
func (a *App) cmdImport() {
	a.commitOrCancelForHistory()
	p, ok := a.host.OpenFile("Import image (320×200)", []Filter{
		{"Images (*.png, *.bmp, *.jpg, *.gif)", "*.png;*.bmp;*.jpg;*.jpeg;*.gif"},
		{"All files", "*.*"},
	}, a.set.LastDir)
	if !ok {
		return
	}
	data, err := a.host.ReadFile(p)
	if err != nil {
		a.host.Info(AppName, "Cannot read file:\n"+err.Error())
		return
	}
	img, err := decodeImage(data, p)
	if err != nil {
		a.host.Info(AppName, err.Error())
		return
	}
	a.rememberDir(p)
	a.modal = newImportDialog(a, img, baseName(p))
}

func (d *importDialog) Title() string          { return "Import & convert: " + d.name }
func (d *importDialog) Size(a *App) (int, int) { return 1060, 740 }

func (d *importDialog) update(a *App) {
	if d.screen == nil || d.screenFit != d.opts.Fit {
		d.screen = c64.ResampleScreen(d.img, d.opts.Fit)
		d.screenFit = d.opts.Fit
		d.adjKey = ""
		d.key = ""
	}
	ak := fmt.Sprint(d.opts.Adjust)
	if d.adjKey != ak {
		d.adjKey = ak
		d.adjusted = c64.ApplyAdjust(d.screen, d.opts.Adjust)
	}
	key := d.keyOf(a, d.opts)
	if d.key != key {
		d.key = key
		d.preview = d.convert(a, d.opts)
	}
}

func (d *importDialog) keyOf(a *App, o c64.ConvertOptions) string {
	return fmt.Sprintf("%v/%d/%d", o, a.palIdx, a.pic.Mode)
}

// convert turns the resampled source into a picture with the given options.
func (d *importDialog) convert(a *App, o c64.ConvertOptions) *c64.Picture {
	if a.hires() {
		// hires: the picture keeps its mode (two colors per 8×8 cell or free)
		return c64.ConvertHiresRGB(c64.ApplyAdjust(d.screen, o.Adjust), a.pal, o, a.pic.Mode)
	}
	px := c64.ApplyAdjust(c64.MergePairs(d.screen, o.Pair), o.Adjust)
	return c64.ConvertRGB(px, a.pal, o)
}

// updateThumbs builds the preset previews: at most one conversion per frame,
// so the dialog stays responsive while they are made.
func (d *importDialog) updateThumbs(a *App, tw, th int) {
	if tw != d.thumbW || th != d.thumbH {
		d.thumbW, d.thumbH = tw, th
		d.thumbKey = [4]string{}
	}
	for i, ps := range importPresets {
		if i >= len(d.thumbs) {
			break
		}
		o := ps.opts(d.opts)
		key := d.keyOf(a, o)
		if d.thumbKey[i] == key {
			continue
		}
		pic := d.preview
		if key != d.key {
			pic = d.convert(a, o)
		}
		d.thumbs[i] = thumbOf(pic, a.pal, tw, th)
		d.thumbKey[i] = key
		a.needAnim = true // the next frame makes the next preview
		return
	}
}

// thumbOf scales a picture down to tw × th (box filter over 320 × 200).
func thumbOf(p *c64.Picture, pal *c64.Palette, tw, th int) []uint32 {
	out := make([]uint32, tw*th)
	pw := p.Width()
	for ty := 0; ty < th; ty++ {
		y0, y1 := ty*200/th, max((ty+1)*200/th, ty*200/th+1)
		for tx := 0; tx < tw; tx++ {
			x0, x1 := tx*320/tw, max((tx+1)*320/tw, tx*320/tw+1)
			var r, g, b, n int
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					c := pal.Colors[p.ColorAt(x*pw/320, y)]
					r += int(c>>16) & 255
					g += int(c>>8) & 255
					b += int(c) & 255
					n++
				}
			}
			out[ty*tw+tx] = gfx.RGB(uint8(r/n), uint8(g/n), uint8(b/n))
		}
	}
	return out
}

func (d *importDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	d.update(a)
	b := d.img.Bounds()

	// ---- preview (left) ----
	colW := 330
	pw := r.W - colW - 24
	scale := 2
	if pw < 640 || r.H-110 < 400 {
		scale = 1
	}
	vx := r.X
	tabs := []string{c64.ModeNames[a.pic.Mode], "Original", "Split"}
	for i, t := range tabs {
		if u.ButtonEx("im_v"+itoa(i), gfx.Rect{X: vx + i*104, Y: r.Y, W: 100, H: 24}, t, d.view == i, true) {
			d.view = i
		}
	}
	pr := gfx.Rect{X: r.X, Y: r.Y + 34, W: 320 * scale, H: 200 * scale}
	m.FillA(gfx.Rect{X: pr.X + 4, Y: pr.Bottom(), W: pr.W, H: 4}, th.Shadow, 50)
	m.FillA(gfx.Rect{X: pr.Right(), Y: pr.Y + 4, W: 4, H: pr.H}, th.Shadow, 50)
	_, holding := u.press("im_prev", pr, 0)
	view := d.view
	if holding {
		view = 1
	}
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			var c uint32
			showOrig := view == 1 || (view == 2 && x < 160)
			if d.target == 1 {
				showOrig = true
			}
			if showOrig {
				p := d.adjusted[y*320+x]
				c = gfx.RGB(uint8(p[0]), uint8(p[1]), uint8(p[2]))
			} else {
				c = a.pal.Colors[d.preview.ColorAt(x*d.preview.Width()/320, y)]
			}
			m.Fill(gfx.Rect{X: pr.X + x*scale, Y: pr.Y + y*scale, W: scale, H: scale}, c)
		}
	}
	if view == 2 && d.target != 1 {
		m.VLine(pr.X+160*scale, pr.Y, pr.Bottom()-1, 0xFFFFFF)
	}
	m.Frame(pr.Inset(-1), th.Border)
	u.Tip("im_prev", pr, "Hold the mouse button to see the original")
	iy := pr.Bottom() + 14
	info := fmt.Sprintf("Source: %d × %d pixels", b.Dx(), b.Dy())
	if b.Dx() == 320 && b.Dy() == 200 {
		info += " – exact C64 screen size, imported 1:1"
	} else if b.Dx() == 160 && b.Dy() == 200 {
		info += " – multicolor pixels, imported 1:1"
	} else {
		info += " – will be scaled to 320 × 200"
	}
	m.Text(font.UI, r.X, iy, info, th.Text)
	if d.target != 1 && !a.hires() {
		m.Text(font.UI, r.X, iy+18, fmt.Sprintf("Background: %s  ·  cells using all 4 colors: %d of 1000  ·  palette: %s",
			c64.ColorNames[d.preview.BG], d.preview.FullCells(), a.pal.Name), th.TextDim)
	} else if d.target != 1 {
		m.Text(font.UI, r.X, iy+18, fmt.Sprintf("Mode: %s  ·  paper color: %s  ·  palette: %s",
			c64.ModeNames[a.pic.Mode], c64.ColorNames[d.preview.BG], a.pal.Name), th.TextDim)
	}
	cellText := []string{
		"Every 4×8 cell gets the background color plus the three colors that fit best.",
		"Every 4×8 cell gets the background color plus its three most frequent colors.",
		"The whole picture uses the background color plus the same three colors.",
	}[clampI(int(d.opts.Cells), 0, 2)]
	rules := "The result always follows the multicolor rules."
	if a.pic.Mode == c64.ModeHires {
		cellText = []string{
			"Every 8×8 cell gets the two colors that fit best.",
			"Every 8×8 cell gets its two most frequent colors.",
			"The whole picture uses the same two colors.",
		}[clampI(int(d.opts.Cells), 0, 2)]
		rules = "The result always follows the hires rules (2 colors per cell)."
	} else if a.pic.Mode == c64.ModeFree {
		cellText = "Hires free: every pixel gets the nearest of all 16 colors."
		rules = "Cells may have more than 2 colors; they are reduced when you export for the C64."
	}
	m.Text(font.UI, r.X, iy+40, cellText, th.TextDim)
	m.Text(font.UI, r.X, iy+56, rules, th.TextDim)

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
	row3 := func(id string, labels []string, sel int, enabled bool) int {
		bw := (w - 8) / len(labels)
		res := -1
		for i, l := range labels {
			if u.ButtonEx(id+itoa(i), gfx.Rect{X: cx + i*(bw+4), Y: y, W: bw, H: 24}, l, sel == i, enabled) {
				res = i
			}
		}
		y += 30
		return res
	}
	hdr("Presets – click a preview")
	tw := (w - 8) / 2
	tht := tw * 200 / 320
	d.updateThumbs(a, tw, tht)
	for i, ps := range importPresets {
		cell := gfx.Rect{X: cx + (i%2)*(tw+8), Y: y + (i/2)*(tht+28), W: tw, H: tht + 22}
		tr := gfx.Rect{X: cell.X, Y: cell.Y, W: tw, H: tht}
		cur := ps.opts(d.opts) == d.opts
		enabled := d.target != 1
		clicked, _ := u.press("im_ps"+itoa(i), cell, 0)
		if clicked && enabled {
			d.opts = ps.opts(d.opts)
			cur = true
		}
		if t := d.thumbs[i]; t != nil && len(t) == tw*tht {
			for py := 0; py < tht; py++ {
				for px := 0; px < tw; px++ {
					m.Set(tr.X+px, tr.Y+py, t[py*tw+px])
				}
			}
			if !enabled {
				m.FillA(tr, th.Panel, 160)
			}
		} else {
			m.Fill(tr, th.Track)
			m.TextC(font.UI, tr, "…", th.TextDim)
		}
		switch {
		case cur && enabled:
			m.Frame(tr.Inset(-1), th.Accent)
			m.Frame(tr.Inset(-2), th.Accent)
		case enabled && u.hover(cell):
			m.Frame(tr.Inset(-1), th.Text)
		default:
			m.Frame(tr.Inset(-1), th.Border)
		}
		lc := th.Text
		if !enabled {
			lc = th.TextDim
		}
		m.TextC(font.UI, gfx.Rect{X: cell.X, Y: tr.Bottom() + 3, W: tw, H: 18}, ps.name, lc)
		u.Tip("im_ps"+itoa(i), cell, ps.tip)
	}
	y += ((len(importPresets)+1)/2)*(tht+28) + 4
	hdr("Use as")
	if v := row3("im_t", []string{"C64 picture", "Template", "Both"}, d.target, true); v >= 0 {
		d.target = v
	}
	hdr("Size and pixel pairs")
	exact := (b.Dx() == 320 || b.Dx() == 160) && b.Dy() == 200
	if v := row3("im_f", []string{"Stretch", "Fit", "Fill"}, int(d.opts.Fit), !exact); v >= 0 {
		d.opts.Fit = c64.FitMode(v)
	}
	if v := row3("im_p", []string{"Average", "Left pixel", "Right pixel"}, int(d.opts.Pair), d.target != 1 && !a.hires()); v >= 0 {
		d.opts.Pair = c64.PairMode(v)
	}
	u.Tip("im_p0", gfx.Rect{X: cx, Y: y - 30, W: w, H: 24}, "Two screen pixels become one wide multicolor pixel")
	hdr("Adjustments")
	sl := func(id, label string, v *int, lo, hi int, unit string) {
		*v = u.Slider(id, gfx.Rect{X: cx, Y: y, W: w, H: 22}, label, *v, lo, hi, unit)
		y += 26
	}
	ad := &d.opts.Adjust
	m.Text(font.UI, cx, y+5, "Automatic contrast", th.Text)
	y += 22
	if v := row3("im_al", c64.AutoLevelNames, ad.AutoLevels, true); v >= 0 {
		ad.AutoLevels = v
	}
	alTips := []string{"No automatic contrast", "Stretch every color channel from its darkest to its brightest value (as Retropixels) – best for dull photos", "Stretch only the brightness – the colors keep their hue"}
	for i, tip := range alTips {
		bw := (w - 8) / 3
		u.Tip("im_al"+itoa(i), gfx.Rect{X: cx + i*(bw+4), Y: y - 30, W: bw, H: 24}, tip)
	}
	sl("im_br", "Brightness", &ad.Brightness, -100, 100, "")
	sl("im_co", "Contrast", &ad.Contrast, -100, 100, "")
	sl("im_sa", "Saturation", &ad.Saturation, -100, 100, "")
	sl("im_hu", "Hue", &ad.Hue, -180, 180, "°")
	sl("im_ga", "Gamma", &ad.Gamma, -100, 100, "")
	if u.ButtonEx("im_rst", gfx.Rect{X: cx, Y: y, W: w, H: 22}, "Reset adjustments", false, *ad != (c64.Adjust{AutoLevels: ad.AutoLevels})) {
		*ad = c64.Adjust{AutoLevels: ad.AutoLevels}
	}
	y += 30
	hdr("Dithering")
	cols := 2
	bw := (w - 4) / cols
	for i := 0; i < len(c64.DitherNames); i++ {
		br := gfx.Rect{X: cx + (i%cols)*(bw+4), Y: y + (i/cols)*25, W: bw, H: 22}
		if u.ButtonEx("im_d"+itoa(i), br, c64.DitherNames[i], int(d.opts.Dither) == i, d.target != 1) {
			d.opts.Dither = c64.DitherMode(i)
			if d.opts.Strength == 0 && i > 0 {
				d.opts.Strength = 60
			}
		}
	}
	y += ((len(c64.DitherNames)+cols-1)/cols)*25 + 6
	d.opts.Strength = u.Slider("im_ds", gfx.Rect{X: cx, Y: y, W: w, H: 22}, "Strength", d.opts.Strength, 0, 100, " %")
	y += 24
	dinfo := ""
	switch {
	case d.opts.Dither.Ordered():
		dinfo = "Pattern shifts the brightness (50 % = Retropixels)"
	case d.opts.Dither.Diffusion():
		dinfo = "Error diffusion, limited so no stray colors appear"
	case d.opts.Dither == c64.DitherYliluoma:
		dinfo = "Mixes the two best palette colors in a pattern"
	}
	if dinfo != "" {
		m.Text(font.UI, cx, y, gfx.Elide(font.UI, dinfo, w), th.TextDim)
		y += 18
	}
	y += 2
	if d.opts.Dither >= c64.DitherFloyd {
		d.opts.Serpentine = u.Checkbox("im_serp", gfx.Rect{X: cx, Y: y, W: w, H: 20}, "Serpentine scanning (fewer streaks)", d.opts.Serpentine)
		y += 24
	}
	y += 4
	hdr("Color matching")
	mw := (w - 8) / 3
	for i := 0; i < len(c64.MatchNames); i++ {
		br := gfx.Rect{X: cx + (i%3)*(mw+4), Y: y + (i/3)*25, W: mw, H: 22}
		if u.ButtonEx("im_m"+itoa(i), br, c64.MatchNames[i], int(d.opts.Match) == i, d.target != 1) {
			d.opts.Match = c64.ColorMatch(i)
		}
		u.Tip("im_m"+itoa(i), br, c64.MatchInfo[i])
	}
	y += ((len(c64.MatchNames)+2)/3)*25 + 6
	hdr("Cell colors")
	if v := row3("im_c", c64.CellMethodNames[:], int(d.opts.Cells), d.target != 1); v >= 0 {
		d.opts.Cells = c64.CellMethod(v)
	}
	cellTips := []string{
		"Every cell gets the three colors that fit it best (default)",
		"Every cell gets its three most frequent colors – often calmer (as in Retropixels)",
		"The whole picture uses the background plus the same three colors – good for game graphics",
	}
	for i, tip := range cellTips {
		bw := (w - 8) / 3
		u.Tip("im_c"+itoa(i), gfx.Rect{X: cx + i*(bw+4), Y: y - 30, W: bw, H: 24}, tip)
	}
	y += 4
	if a.hires() {
		hdr("Paper color (empty pixels)")
	} else {
		hdr("Background color ($D021)")
	}
	sw := (w - 16) / 17
	for i := -1; i < 16; i++ {
		sr := gfx.Rect{X: cx + (i+1)*(sw+1), Y: y, W: sw, H: 22}
		if i < 0 {
			m.Fill(sr, th.Btn)
			m.TextC(font.UIB, sr, "A", th.Text)
		} else {
			m.Fill(sr, a.pal.Colors[i])
		}
		if d.opts.BG == i {
			m.Frame(sr.Inset(-1), th.Accent)
			m.Frame(sr, th.Accent)
		}
		if c, _ := u.press("im_bg"+itoa(i), sr, 0); c && d.target != 1 {
			d.opts.BG = i
		}
		if i < 0 {
			u.Tip("im_bg"+itoa(i), sr, "Choose automatically")
		} else {
			u.Tip("im_bg"+itoa(i), sr, c64.ColorNames[i])
		}
	}
	y += 30
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
	if u.Button("im_cancel", gfx.Rect{X: r.Right() - 250, Y: by, W: 120, H: 28}, "Cancel") {
		a.closeModal(true)
		return
	}
	if u.ButtonEx("im_ok", gfx.Rect{X: r.Right() - 120, Y: by, W: 120, H: 28}, "Import", true, true) {
		a.lastImport = d.opts
		if d.autoWas != 0 && d.opts.Adjust.AutoLevels == 0 {
			// keep automatic contrast for the next photo
			a.lastImport.Adjust.AutoLevels = d.autoWas
		}
		a.rememberImport()
		if d.target != 1 {
			a.commitOrCancelForHistory()
			a.sel.state = selNone
			a.pushUndo(a.pic.Clone())
			border := a.pic.Border
			sideOn, side := a.pic.SideOn, a.pic.Side
			a.setFlatPicture(d.preview)
			a.pic.Border = border
			if !a.pic.Hires() {
				// the side border sprites stay
				a.pic.SideOn, a.pic.Side = sideOn, side
			}
			a.syncMode()
			a.filePath[a.page] = ""
			a.updateTitle()
			a.setStatus("Image converted: " + d.name)
		}
		if d.target >= 1 {
			a.setTemplate(d.img, d.name)
		}
		a.closeModal(false)
	}
}

// rememberImport stores the converter settings for the next start. The
// picture adjustments (brightness, contrast, …) and the background color
// belong to one image and are not kept.
func (a *App) rememberImport() {
	o := a.lastImport
	o.BG = -1
	o.Adjust = c64.Adjust{AutoLevels: o.Adjust.AutoLevels}
	a.set.Import = &o
	a.saveSettings()
}

// savedImport returns the stored converter settings, or the default preset.
func (a *App) savedImport() c64.ConvertOptions {
	def := importPresets[0].opts(c64.ConvertOptions{BG: -1, Fit: c64.FitContain})
	o := a.set.Import
	if o == nil || !o.Valid() {
		return def
	}
	r := *o
	r.BG = -1
	return r
}
