package app

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"path/filepath"
	"strconv"
	"strings"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
)

func fontUI() *font.Face { return font.UI }
func itoa(v int) string  { return strconv.Itoa(v) }

const (
	exportKLA = iota
	exportPRG
	exportPNG320
	exportPNG160
	exportPNGBorder
)

var openFilters = []Filter{
	{"All supported files", "*.c64dp;*.kla;*.koa;*.png;*.bmp"},
	{"C64 DPainter project with layers (*.c64dp)", "*.c64dp"},
	{"Koala Painter (*.kla, *.koa)", "*.kla;*.koa"},
	{"PNG image (*.png)", "*.png"},
	{"Windows bitmap (*.bmp)", "*.bmp"},
	{"All files", "*.*"},
}

var tplFilters = []Filter{
	{"Images (*.png, *.bmp, *.jpg, *.gif)", "*.png;*.bmp;*.jpg;*.jpeg;*.gif"},
	{"All files", "*.*"},
}

// Template ist die Vorlage-Ebene (PNG/BMP als Hilfe zum Nachzeichnen).
type Template struct {
	src     image.Image
	name    string
	buf     []uint32
	alpha   []uint8
	w, h    int
	visible bool
	opacity int
	under   bool
	fit     c64.FitMode
	hidePic bool
}

const tplScale = 4 // Vorlage wird in 1280×800 vorgehalten

func (t *Template) rebuild() {
	if t.src == nil {
		t.buf, t.alpha = nil, nil
		return
	}
	t.w, t.h = 320*tplScale, 200*tplScale
	t.buf = make([]uint32, t.w*t.h)
	t.alpha = make([]uint8, t.w*t.h)
	b := t.src.Bounds()
	sw, sh := float64(b.Dx()), float64(b.Dy())
	dx, dy, dw, dh := 0.0, 0.0, 320.0, 200.0
	if t.fit != c64.FitStretch {
		sa := sw / sh
		if (t.fit == c64.FitContain) == (sa > 1.6) {
			dw, dh = 320, 320/sa
		} else {
			dh, dw = 200, 200*sa
		}
		dx, dy = (320-dw)/2, (200-dh)/2
	}
	// Quelle als RGBA
	rgba, ok := t.src.(*image.RGBA)
	if !ok || rgba.Bounds().Min != (image.Point{}) {
		rgba = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Bounds(), t.src, b.Min, draw.Src)
	}
	rb := rgba.Bounds()
	for y := 0; y < t.h; y++ {
		fy := (float64(y)/tplScale + 0.5/tplScale - dy) / dh * sh
		if fy < 0 || fy >= sh {
			continue
		}
		iy := int(fy)
		for x := 0; x < t.w; x++ {
			fx := (float64(x)/tplScale + 0.5/tplScale - dx) / dw * sw
			if fx < 0 || fx >= sw {
				continue
			}
			ix := int(fx)
			if iy >= rb.Dy() || ix >= rb.Dx() {
				continue
			}
			o := iy*rgba.Stride + ix*4
			p := rgba.Pix[o : o+4]
			i := y*t.w + x
			t.buf[i] = uint32(p[0])<<16 | uint32(p[1])<<8 | uint32(p[2])
			t.alpha[i] = p[3]
		}
	}
}

func decodeImage(data []byte, path string) (image.Image, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".bmp" || (len(data) > 2 && data[0] == 'B' && data[1] == 'M') {
		return c64.DecodeBMP(data)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("Cannot read image: %v", err)
	}
	return img, nil
}

func (a *App) rememberDir(p string) {
	a.set.LastDir = filepath.Dir(p)
	a.saveSettings()
}

// confirmDiscard fragt bei ungesicherten Änderungen. true = weitermachen.
func (a *App) confirmDiscard() bool {
	if !a.dirty[a.page] {
		return true
	}
	switch a.host.Ask(AppName, "The picture has been changed.\n\nSave changes first?") {
	case 0:
		return a.cmdSave()
	case 1:
		return true
	}
	return false
}

func (a *App) cmdNew() {
	a.commitOrCancelForHistory()
	if !a.confirmDiscard() {
		return
	}
	a.sel.state = selNone
	bg, border := a.pic.BG, a.pic.Border
	a.setFlatPicture(c64.NewPicture(bg, border))
	a.undos[a.page], a.redos[a.page] = nil, nil
	a.dirty[a.page] = false
	a.filePath[a.page] = ""
	a.updateTitle()
	a.setStatus("New picture")
}

func (a *App) cmdOpen() {
	a.commitOrCancelForHistory()
	p, ok := a.host.OpenFile("Open picture", openFilters, a.set.LastDir)
	if !ok {
		return
	}
	a.openPath(p)
}

func (a *App) openPath(p string) {
	a.commitOrCancelForHistory()
	data, err := a.host.ReadFile(p)
	if err != nil {
		a.host.Info(AppName, "Cannot read file:\n"+err.Error())
		return
	}
	a.rememberDir(p)
	ext := strings.ToLower(filepath.Ext(p))
	if ext == ".c64dp" || isProject(data) {
		if a.loadProject(data, p) {
			a.filePath[a.page] = p
			a.dirty[a.page] = false
			a.sel.state = selNone
			a.updateTitle()
			a.setStatus("Project loaded: " + filepath.Base(p))
		}
		return
	}
	if ext == ".kla" || ext == ".koa" || ((len(data) == 10003 || len(data) == 10001) && ext != ".png" && ext != ".bmp") {
		pic, err := c64.LoadKLA(data)
		if err != nil {
			a.host.Info(AppName, err.Error())
			return
		}
		if !a.confirmDiscard() {
			return
		}
		pic.Border = a.pic.Border
		a.pushUndo(a.pic.Clone())
		a.setFlatPicture(pic)
		a.filePath[a.page] = p
		a.dirty[a.page] = false
		a.sel.state = selNone
		a.updateTitle()
		a.setStatus("Loaded: " + filepath.Base(p))
		return
	}
	img, err := decodeImage(data, p)
	if err != nil {
		a.host.Info(AppName, err.Error())
		return
	}
	a.modal = newImportDialog(a, img, filepath.Base(p))
}

func (a *App) loadTemplateDialog() {
	p, ok := a.host.OpenFile("Load template (PNG/BMP)", tplFilters, a.set.LastDir)
	if !ok {
		return
	}
	data, err := a.host.ReadFile(p)
	if err != nil {
		a.host.Info(AppName, err.Error())
		return
	}
	img, err := decodeImage(data, p)
	if err != nil {
		a.host.Info(AppName, err.Error())
		return
	}
	a.rememberDir(p)
	a.setTemplate(img, filepath.Base(p))
}

func (a *App) setTemplate(img image.Image, name string) {
	a.tpl.src = img
	a.tpl.name = name
	a.tpl.visible = true
	a.tpl.rebuild()
	a.setStatus("Template loaded: " + name + " (T shows/hides it)")
}

func (a *App) clearTemplate() {
	a.tpl.src = nil
	a.tpl.name = ""
	a.tpl.rebuild()
}

// cmdSave speichert im Format der aktuellen Datei (Standard: Koala).
func (a *App) cmdSave() bool {
	a.commitOrCancelForHistory()
	p := a.filePath[a.page]
	if p == "" {
		return a.cmdSaveAs()
	}
	return a.saveTo(p)
}

func (a *App) cmdSaveAs() bool {
	a.commitOrCancelForHistory()
	name := "picture.c64dp"
	if a.filePath[a.page] != "" {
		name = filepath.Base(a.filePath[a.page])
	}
	p, ok := a.host.SaveFile("Save as", []Filter{
		{"C64 DPainter project with layers (*.c64dp)", "*.c64dp"},
		{"Koala Painter (*.kla)", "*.kla"},
		{"PNG image 320×200 (*.png)", "*.png"},
		{"C64 program (*.prg)", "*.prg"},
	}, a.set.LastDir, name, "c64dp")
	if !ok {
		return false
	}
	if a.saveTo(p) {
		a.filePath[a.page] = p
		a.updateTitle()
		return true
	}
	return false
}

func (a *App) saveTo(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	if ext == ".c64dp" {
		data, err := a.encodeProject()
		if err == nil {
			err = a.host.WriteFile(p, data)
		}
		if err != nil {
			a.host.Info(AppName, "Saving failed:\n"+err.Error())
			return false
		}
		a.dirty[a.page] = false
		a.rememberDir(p)
		a.updateTitle()
		a.setStatus(fmt.Sprintf("Project saved: %s (%d bytes, all layers)", filepath.Base(p), len(data)))
		return true
	}
	kind := exportKLA
	switch ext {
	case ".png":
		kind = exportPNG320
	case ".prg":
		kind = exportPRG
	}
	if !a.writeExport(p, kind) {
		return false
	}
	a.dirty[a.page] = false
	a.rememberDir(p)
	a.updateTitle()
	if a.usesLayers() {
		a.setStatus("Saved " + filepath.Base(p) + " – visible layers merged. Save as .c64dp to keep the layers.")
	}
	return true
}

// usesLayers reports whether a plain C64 file would lose anything: more than
// one painted layer, a hidden layer or a template.
func (a *App) usesLayers() bool {
	n := 0
	for l := 0; l < c64.LayerCount; l++ {
		if a.pic.LayerPixels(l) > 0 {
			n++
			if a.pic.Hide[l] {
				return true
			}
		}
	}
	return n > 1 || a.tpl.src != nil
}

func (a *App) writeExport(p string, kind int) bool {
	var data []byte
	var err error
	switch kind {
	case exportKLA:
		data = a.pic.SaveKLA()
	case exportPRG:
		data = a.pic.PRG()
	case exportPNG320:
		data, err = a.pic.SavePNG(a.pal, c64.PNG320)
	case exportPNG160:
		data, err = a.pic.SavePNG(a.pal, c64.PNG160)
	case exportPNGBorder:
		data, err = a.pic.SavePNG(a.pal, c64.PNGBorder)
	}
	if err == nil {
		err = a.host.WriteFile(p, data)
	}
	if err != nil {
		a.host.Info(AppName, "Saving failed:\n"+err.Error())
		return false
	}
	a.setStatus(fmt.Sprintf("Saved: %s (%d bytes)", filepath.Base(p), len(data)))
	return true
}

func (a *App) cmdExport(kind int) {
	a.commitOrCancelForHistory()
	base := "picture"
	if a.filePath[a.page] != "" {
		base = trimExt(filepath.Base(a.filePath[a.page]))
	}
	var f []Filter
	var name, ext, title string
	switch kind {
	case exportKLA:
		f, name, ext, title = []Filter{{"Koala Painter (*.kla)", "*.kla"}}, base+".kla", "kla", "Export as Koala"
	case exportPRG:
		f, name, ext, title = []Filter{{"C64 program (*.prg)", "*.prg"}}, base+".prg", "prg", "Export as C64 program"
	case exportPNG320:
		f, name, ext, title = []Filter{{"PNG image (*.png)", "*.png"}}, base+".png", "png", "Export PNG 320×200"
	case exportPNG160:
		f, name, ext, title = []Filter{{"PNG image (*.png)", "*.png"}}, base+"_160.png", "png", "Export PNG 160×200"
	case exportPNGBorder:
		f, name, ext, title = []Filter{{"PNG image (*.png)", "*.png"}}, base+"_border.png", "png", "Export PNG with border"
	}
	p, ok := a.host.SaveFile(title, f, a.set.LastDir, name, ext)
	if !ok {
		return
	}
	if a.writeExport(p, kind) {
		a.rememberDir(p)
		if kind == exportKLA {
			a.dirty[a.page] = false
			a.updateTitle()
		}
	}
}

func (a *App) cmdQuit() {
	if a.CanClose() {
		a.quitting = true
		a.host.Quit()
	}
}

// pngOf ist für Tests: das Bild als PNG.
func (a *App) pngOf() []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, a.pic.ToImage(a.pal, c64.PNG320))
	return buf.Bytes()
}

func baseName(p string) string { return filepath.Base(p) }
