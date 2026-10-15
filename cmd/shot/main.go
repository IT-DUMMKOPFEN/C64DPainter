// Testprogramm (Linux): steuert die Oberfläche per Skript und speichert Bildschirmfotos.
package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"c64dpainter/internal/app"
)

type host struct {
	open, save string
}

func (h *host) OpenFile(title string, f []app.Filter, dir string) (string, bool) {
	return h.open, h.open != ""
}
func (h *host) SaveFile(title string, f []app.Filter, dir, name, ext string) (string, bool) {
	return h.save, h.save != ""
}
func (h *host) Ask(t, s string) int                { return 1 }
func (h *host) Info(t, s string)                   { fmt.Println("INFO:", s) }
func (h *host) Quit()                              {}
func (h *host) SetTitle(s string)                  { fmt.Println("TITLE:", s) }
func (h *host) ReadFile(p string) ([]byte, error)  { return os.ReadFile(p) }
func (h *host) WriteFile(p string, d []byte) error { return os.WriteFile(p, d, 0o644) }

var out string
var now int64

func shot(a *app.App, name string) {
	img := a.Frame()
	f, _ := os.Create(filepath.Join(out, name+".png"))
	png.Encode(f, img.ToRGBA())
	f.Close()
}

func frame(a *app.App) {
	now += 16
	a.Tick(now)
	a.Frame()
	a.RunDeferred()
}

func click(a *app.App, x, y, btn int) {
	a.MouseMove(x, y)
	frame(a)
	a.MouseDown(btn, x, y, false)
	frame(a)
	a.MouseUp(btn, x, y)
	frame(a)
}

func drag(a *app.App, x0, y0, x1, y1, btn int) {
	a.MouseMove(x0, y0)
	frame(a)
	a.MouseDown(btn, x0, y0, false)
	frame(a)
	n := 12
	for i := 1; i <= n; i++ {
		x := x0 + (x1-x0)*i/n
		y := y0 + (y1-y0)*i/n
		a.MouseMove(x, y)
		frame(a)
	}
	a.MouseUp(btn, x1, y1)
	frame(a)
}

func char(a *app.App, c rune) {
	a.Char(c)
	frame(a)
}

func key(a *app.App, k int, m app.Mods) {
	a.KeyDown(k, m)
	frame(a)
	a.KeyUp(k, m)
}

func main() {
	out = os.Args[1]
	os.MkdirAll(out, 0o755)
	cfg := filepath.Join(out, "settings.json")
	os.Remove(cfg)
	app.SettingsPath = func() string { return cfg }
	h := &host{}
	a := app.New(h)
	a.Resize(1280, 800)
	frame(a)
	shot(a, "01_help_start")
	key(a, app.KeyEsc, app.Mods{})
	shot(a, "02_empty")

	// Hintergrund Blau per Strg+Klick (Palette: x≈1215.., y ab ~)
	a.Debug("bg", 6)
	// Gefülltes Rechteck in Hellblau
	a.Debug("fg", 14)
	char(a, 'R')
	p := func(x, y int) (int, int) { return a.PicToScreenCenter(x, y) }
	x0, y0 := p(10, 20)
	x1, y1 := p(70, 90)
	drag(a, x0, y0, x1, y1, 0)
	// Kreis in Gelb
	a.Debug("fg", 7)
	char(a, 'C')
	cx, cy := p(110, 60)
	ex, ey := p(110, 100)
	drag(a, cx, cy, ex, ey, 0)
	// Muster-Ellipse gefüllt, Weiß/Rot zweifarbig
	a.Debug("fg", 1)
	a.Debug("fg2", 2)
	a.Debug("pattern", 5)
	char(a, 'E')
	cx, cy = p(80, 150)
	ex, ey = p(115, 180)
	drag(a, cx, cy, ex, ey, 0)
	a.Debug("pattern", 0)
	// Linien in Grün
	a.Debug("fg", 5)
	char(a, 'v')
	char(a, '+')
	char(a, '+')
	sx, sy := p(5, 195)
	tx, ty := p(155, 110)
	drag(a, sx, sy, tx, ty, 0)
	// Freihand Orange
	a.Debug("fg", 8)
	char(a, 'd')
	sx, sy = p(20, 120)
	tx, ty = p(60, 190)
	drag(a, sx, sy, tx, ty, 0)
	// Sprühdose Hellgrün
	a.Debug("fg", 13)
	char(a, 'a')
	sx, sy = p(140, 30)
	a.MouseMove(sx, sy)
	frame(a)
	a.MouseDown(0, sx, sy, false)
	for i := 0; i < 30; i++ {
		now += 40
		a.Tick(now)
		a.Frame()
	}
	a.MouseUp(0, sx, sy)
	frame(a)
	// Konflikt erzwingen: Rot in volle Zelle
	a.Debug("fg", 10)
	char(a, 'd')
	char(a, '.')
	sx, sy = p(80, 150)
	tx, ty = p(110, 150)
	drag(a, sx, sy, tx, ty, 0)
	shot(a, "03_drawing")

	// Auswahl + drehen
	char(a, 'b')
	sx, sy = p(100, 40)
	tx, ty = p(122, 80)
	drag(a, sx, sy, tx, ty, 0)
	shot(a, "04_selected")
	a.Debug("rotate", 30)
	shot(a, "05_rotated")
	key(a, app.KeyEnter, app.Mods{})

	// Dunkles Thema
	a.Debug("theme", 1)
	a.MouseMove(p(60, 60))
	frame(a)
	shot(a, "06_dark")
	// Menü öffnen
	click(a, 230, 12, 0)
	shot(a, "07_menu")
	key(a, app.KeyEsc, app.Mods{})
	// Palettendialog
	char(a, 'p')
	shot(a, "08_palette")
	key(a, app.KeyDown, app.Mods{})
	key(a, app.KeyEnter, app.Mods{})
	// Gitterdialog
	char(a, 'G')
	shot(a, "09_grid")
	key(a, app.KeyEsc, app.Mods{})
	a.Debug("theme", 0)
	// Hilfe: Regeln
	a.Debug("help", 0)
	shot(a, "10_help_rules")
	key(a, app.KeyEsc, app.Mods{})

	// Speichern
	h.save = filepath.Join(out, "test.kla")
	key(a, 'S', app.Mods{Ctrl: true})
	h.save = filepath.Join(out, "test.prg")
	key(a, 'E', app.Mods{Ctrl: true})
	h.save = filepath.Join(out, "test.png")
	a.Debug("exportpng", 0)
	// Import 320×200
	h.open = os.Args[3]
	key(a, 'I', app.Mods{Ctrl: true})
	shot(a, "11_import")
	a.Debug("importdither", 7)
	a.Debug("importview", 2)
	shot(a, "11b_import_split")
	a.Debug("importok", 0)
	frame(a)
	shot(a, "12_imported")
	// Vorlage
	h.open = os.Args[2]
	a.Debug("loadtpl", 0)
	a.Debug("clear", 0)
	frame(a)
	shot(a, "13_template")
	// Seitenleiste nach unten scrollen, großer Pinsel, Zellinfo
	char(a, 'K')
	a.Debug("fg", 7)
	char(a, 'C')
	cx, cy = p(80, 100)
	ex, ey = p(80, 160)
	drag(a, cx, cy, ex, ey, 0)
	char(a, 'd')
	for i := 0; i < 15; i++ {
		char(a, '+')
	}
	a.Wheel(-20, 100, 500)
	frame(a)
	a.MouseMove(p(40, 60))
	frame(a)
	shot(a, "14_panel_bottom")
	// Hilfe Werkzeuge im dunklen Thema
	a.Debug("theme", 1)
	a.Debug("helptopic", 5)
	shot(a, "15_help_tools_dark")
	a.Debug("helpscroll", 900)
	shot(a, "16_help_tools_scrolled")
	a.Debug("helptopic", 14)
	shot(a, "17_help_keys")
	key(a, app.KeyEsc, app.Mods{})
	a.Debug("theme", 0)
	// kleines Fenster
	a.Resize(1000, 640)
	frame(a)
	shot(a, "18_small_window")
	// Auto dither and the dither brush (optional 4th argument: a KLA file)
	if len(os.Args) > 4 {
		a.Resize(1360, 860)
		frame(a)
		h.open = os.Args[4]
		key(a, 'O', app.Mods{Ctrl: true})
		frame(a)
		char(a, 'A')
		frame(a)
		shot(a, "19_autodither")
		a.Debug("ditherview", 2)
		a.Debug("ditherstyle", 3)
		shot(a, "19b_autodither_split")
		key(a, app.KeyEsc, app.Mods{})
		char(a, 'D')
		for i := 0; i < 15; i++ {
			char(a, '+')
		}
		a.MouseMove(p(60, 100))
		frame(a)
		shot(a, "20_dither_brush")
	}
	fmt.Println("fertig")
}
