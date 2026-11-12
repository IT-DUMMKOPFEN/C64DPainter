package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- Font editor: picture → character set + screen map (1.1) -----------------------
//
// A 320 × 200 picture (an image file or the current picture) becomes a
// character set plus a 40 × 25 map and the cell colors. Equal characters are
// stored once; when more characters are needed than allowed, the most
// similar ones are merged. Brightness, contrast, colors … of the image can be
// changed first, which often saves characters. Export: the program (.asm),
// the data (_data.asm with !byte lines) and a PRG to look at the result.

type fntPicDialog struct {
	src     [][3]int // 320 × 200
	name    string
	adj     c64.Adjust
	o       c64.CharMapOptions
	fit     c64.FitMode
	cm      *c64.CharMap
	key     string
	view    int // 0 result, 1 original, 2 split
	bgAuto  bool
	mcAuto  bool
	tooMany bool
}

func (a *App) fntOpenPicImport() {
	d := &fntPicDialog{name: "picture", bgAuto: true, mcAuto: true, fit: c64.FitContain}
	d.o = c64.CharMapOptions{Multi: a.fnt.f.Multi, BG: -1, MC1: -1, MC2: -1, Limit: 256}
	// start with the current picture
	d.src = c64.ResampleScreen(a.pic.ToImage(a.pal, c64.PNG320), c64.FitStretch)
	a.modal = d
}

func (d *fntPicDialog) loadImage(a *App) {
	p, ok := a.host.OpenFile("Picture for the character set (320 × 200)", []Filter{{Name: "Images (*.png, *.bmp, *.jpg, *.gif)", Pattern: "*.png;*.bmp;*.jpg;*.jpeg;*.gif"}}, a.workDir())
	if !ok {
		return
	}
	data, err := a.host.ReadFile(p)
	if err == nil {
		img, err2 := decodeImage(data, p)
		if err2 == nil {
			d.src = c64.ResampleScreen(img, d.fit)
			d.name = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
			if b := img.Bounds(); b.Dx() != 320 || b.Dy() != 200 {
				d.adj.AutoLevels = 1
			}
			d.key = ""
			return
		}
		err = err2
	}
	a.host.Info(AppName, "Cannot read the image:\n"+err.Error())
}

func (d *fntPicDialog) update(a *App) {
	o := d.o
	key := fmt.Sprintf("%v/%v/%d/%p", o, d.adj, a.palIdx, d.src)
	if key == d.key && d.cm != nil {
		return
	}
	d.key = key
	d.cm = c64.ConvertCharMap(c64.ApplyAdjust(d.src, d.adj), a.pal, o)
	d.tooMany = d.cm.Unique > o.Limit
}

func (d *fntPicDialog) Title() string          { return "Picture to character set + map" }
func (d *fntPicDialog) Size(a *App) (int, int) { return 1060, min(740, a.H-30) }

func (d *fntPicDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	d.update(a)
	// preview
	for i, l := range []string{"Result", "Original", "Split"} {
		if u.ButtonEx("fp_v"+itoa(i), gfx.Rect{X: r.X + i*104, Y: r.Y, W: 100, H: 24}, l, d.view == i, true) {
			d.view = i
		}
	}
	pr := gfx.Rect{X: r.X, Y: r.Y + 32, W: 640, H: 400}
	adjusted := c64.ApplyAdjust(d.src, d.adj)
	for py := 0; py < pr.H; py++ {
		y := py / 2
		for px := 0; px < pr.W; px++ {
			x := px / 2
			orig := d.view == 1 || (d.view == 2 && px >= pr.W/2)
			var c uint32
			if orig {
				s := adjusted[y*320+x]
				c = gfx.RGB(uint8(s[0]), uint8(s[1]), uint8(s[2]))
			} else {
				c = a.pal.Colors[d.cm.ColorAt(x, y)]
			}
			m.Set(pr.X+px, pr.Y+py, c)
		}
	}
	m.Frame(pr.Inset(-1), th.Border)
	if d.view == 2 {
		m.Line(pr.X+pr.W/2, pr.Y, pr.X+pr.W/2, pr.Bottom()-1, 0xFFFFFF)
	}
	y := pr.Bottom() + 10
	stat := fmt.Sprintf("%d different characters · %d in the set (of 256) · map 40 × 25", d.cm.Unique, d.cm.Used)
	m.Text(font.UIB, r.X, y, stat, th.Text)
	y += 20
	if d.tooMany {
		msg := fmt.Sprintf("More characters than allowed (%d > %d): similar characters were merged.", d.cm.Unique, d.o.Limit)
		if d.cm.Unique > 256 {
			msg = fmt.Sprintf("Warning: the picture needs %d characters – a character set has only 256. Similar characters were merged.", d.cm.Unique)
		}
		m.Text(font.UI, r.X, y, msg, gfx.RGB(240, 140, 40))
		y += 18
	}
	m.Text(font.UI, r.X, y, "Tip: less contrast, less saturation or fewer details need fewer characters.", th.TextDim)
	// settings
	x := pr.Right() + 20
	w := r.Right() - x
	y = r.Y
	hdr := func(s string) {
		m.Text(font.UIB, x, y+4, s, th.Text)
		y += 24
	}
	bw := (w - 4) / 2
	if u.Button("fp_load", gfx.Rect{X: x, Y: y, W: bw, H: 24}, "Load image …") {
		a.later(func() { d.loadImage(a) })
	}
	if u.Button("fp_pic", gfx.Rect{X: x + bw + 4, Y: y, W: bw, H: 24}, "Current picture") {
		d.src = c64.ResampleScreen(a.pic.ToImage(a.pal, c64.PNG320), c64.FitStretch)
		d.name = "picture"
		d.key = ""
	}
	y += 30
	m.Text(font.UI, x, y, gfx.Elide(font.UI, "Source: "+d.name, w), th.TextDim)
	y += 20
	hdr("Mode")
	for i, l := range []string{"Hires", "Multicolor"} {
		if u.ButtonEx("fp_m"+itoa(i), gfx.Rect{X: x + i*(bw+4), Y: y, W: bw, H: 24}, l, d.o.Multi == (i == 1), true) {
			d.o.Multi = i == 1
		}
	}
	y += 30
	hdr("Characters")
	d.o.Limit = u.Slider("fp_lim", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Max chars", d.o.Limit, 16, 256, "")
	u.Tip("fp_lim", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Fewer characters leave room for text characters – similar characters are merged")
	y += 26
	d.o.Similar = u.Slider("fp_sim", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Similar", d.o.Similar, 0, 16, " px")
	u.Tip("fp_sim", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Characters that differ in at most this many pixels become one")
	y += 30
	hdr("Colors")
	swatches := func(id, label string, val *int) {
		m.Text(font.UI, x, y+3, label, th.Text)
		sx := x + 84
		sw := (w - 84 - 46) / 16
		ar := gfx.Rect{X: sx, Y: y, W: 44, H: 18}
		if u.ButtonEx(id+"a", ar, "Auto", *val < 0, true) {
			*val = -1
		}
		for c := 0; c < 16; c++ {
			cr := gfx.Rect{X: sx + 46 + c*sw, Y: y, W: sw - 1, H: 18}
			m.Fill(cr, a.pal.Colors[c])
			if *val == c {
				m.Frame(cr.Inset(-1), th.Accent)
				m.Frame(cr, 0xFFFFFF)
			}
			if cl, _ := u.press(fmt.Sprintf("%s%d", id, c), cr, 0); cl {
				*val = c
			}
		}
		y += 24
	}
	swatches("fp_bg", "Background", &d.o.BG)
	if d.o.Multi {
		swatches("fp_m1", "$D022", &d.o.MC1)
		swatches("fp_m2", "$D023", &d.o.MC2)
	}
	hdr("Adjust the image")
	sl := func(id, name string, v *int, lo, hi int, unit string) {
		*v = u.Slider(id, gfx.Rect{X: x, Y: y, W: w, H: 22}, name, *v, lo, hi, unit)
		y += 25
	}
	als := c64.AutoLevelNames
	m.Text(font.UI, x, y+3, "Automatic contrast", th.Text)
	y += 22
	for i, l := range als {
		bw3 := (w - 8) / 3
		if u.ButtonEx("fp_al"+itoa(i), gfx.Rect{X: x + i*(bw3+4), Y: y, W: bw3, H: 22}, l, d.adj.AutoLevels == i, true) {
			d.adj.AutoLevels = i
		}
	}
	y += 26
	sl("fp_br", "Brightness", &d.adj.Brightness, -100, 100, "")
	sl("fp_co", "Contrast", &d.adj.Contrast, -100, 100, "")
	sl("fp_sa", "Saturation", &d.adj.Saturation, -100, 100, "")
	sl("fp_hu", "Hue", &d.adj.Hue, -180, 180, "°")
	sl("fp_ga", "Gamma", &d.adj.Gamma, -100, 100, "")
	if u.Button("fp_reset", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Reset adjustments") {
		d.adj = c64.Adjust{}
	}
	y += 36
	// what the export writes
	by0 := r.Bottom() - 30 - 10
	if by0-y >= 120 {
		box := gfx.Rect{X: x, Y: y, W: w, H: min(by0-y, 176)}
		m.Frame(box, th.Border)
		tx, ty := box.X+10, box.Y+8
		line := func(f *font.Face, s string, c uint32) {
			if ty+16 <= box.Bottom() {
				m.Text(f, tx, ty, gfx.Elide(f, s, box.W-20), c)
			}
			ty += 18
		}
		mode := "hires"
		if d.o.Multi {
			mode = "multicolor"
		}
		line(font.UIB, "Export ASM + PRG saves", th.Text)
		line(font.UI, fmt.Sprintf("• Font: %d characters, %d bytes (%s)", d.cm.Used, d.cm.Used*8, mode), th.Text)
		line(font.UI, "• Map: 40 × 25 screen codes, 1000 bytes", th.Text)
		line(font.UI, "• Color map: 1000 bytes for $D800", th.Text)
		if d.o.Multi {
			line(font.UI, "• Colors: $D021, $D022, $D023", th.Text)
		} else {
			line(font.UI, "• Colors: $D021", th.Text)
		}
		line(font.UI, "name.asm · name_data.asm · name.prg", th.TextDim)
	}
	// buttons
	by := r.Bottom() - 30
	if u.Button("fp_cancel", gfx.Rect{X: r.Right() - 470, Y: by, W: 110, H: 30}, "Cancel") {
		a.closeModal(true)
	}
	if u.Button("fp_exp", gfx.Rect{X: r.Right() - 350, Y: by, W: 160, H: 30}, "Export ASM + PRG …") {
		cm := d.cm
		a.later(func() { a.fntExportCharMap(cm, d.name) })
	}
	if u.ButtonEx("fp_use", gfx.Rect{X: r.Right() - 180, Y: by, W: 180, H: 30}, "Use as character set", true, true) {
		a.closeModal(false)
		e := a.fnt
		e.undo = append(e.undo, *e.f)
		e.redo = nil
		f := *d.cm.Font
		f.Ref, f.Tile, f.TileLayout = e.f.Ref, 0, 0
		*e.f = f
		e.cmap = d.cm
		e.cmapName = d.name
		a.setStatus(fmt.Sprintf("Character set from %s: %d characters – File \"Export picture\" saves the map too", d.name, d.cm.Used))
	}
}

// fntExportCharMap writes name.asm (program), name_data.asm (!byte data)
// and name.prg (to look at it).
func (a *App) fntExportCharMap(cm *c64.CharMap, name string) {
	if cm == nil {
		return
	}
	base := strings.Map(func(r rune) rune {
		if r == ' ' {
			return '_'
		}
		return r
	}, name)
	p, ok := a.host.SaveFile("Export picture as characters (program .asm, data, PRG)", []Filter{{"ACME source (*.asm)", "*.asm"}}, a.workDir(), base+".asm", "asm")
	if !ok {
		return
	}
	p = FixDoubleExt(p)
	stem := strings.TrimSuffix(p, filepath.Ext(p))
	label := filepath.Base(stem)
	dataFile := label + "_data.asm"
	data := cm.DataACME(label)
	prog := cm.ProgramACME(label, dataFile, "")
	prg, err := c64.Assemble(cm.ProgramACME(label, "", data))
	for _, f := range []struct {
		path string
		b    []byte
	}{{stem + ".asm", []byte(prog)}, {stem + "_data.asm", []byte(data)}, {stem + ".prg", prg}} {
		if err != nil {
			break
		}
		err = a.host.WriteFile(f.path, f.b)
	}
	if err != nil {
		a.host.Info(AppName, "Saving failed:\n"+err.Error())
		return
	}
	a.rememberDir(p)
	a.setStatus(fmt.Sprintf("Saved %s.asm, %s and %s.prg (%d characters)", label, dataFile, label, cm.Used))
}
