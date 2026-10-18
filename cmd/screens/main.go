// Screenshot tool: loads a KLA picture, drives the user interface with a
// script and saves screenshots of the main functions (e.g. for a web page).
//
//	go run ./cmd/screens <out-folder> <picture.kla> <image320x200.png>
package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"c64dpainter/internal/app"
)

type host struct{ open string }

func (h *host) OpenFile(title string, f []app.Filter, dir string) (string, bool) {
	return h.open, h.open != ""
}
func (h *host) SaveFile(title string, f []app.Filter, dir, name, ext string) (string, bool) {
	return "", false
}
func (h *host) Ask(t, s string) int                { return 1 }
func (h *host) Info(t, s string)                   { fmt.Println("INFO:", s) }
func (h *host) Quit()                              {}
func (h *host) SetTitle(s string)                  {}
func (h *host) ReadFile(p string) ([]byte, error)  { return os.ReadFile(p) }
func (h *host) WriteFile(p string, d []byte) error { return os.WriteFile(p, d, 0o644) }

var (
	out string
	now int64
)

func frame(a *app.App) {
	now += 16
	a.Tick(now)
	a.Frame()
	a.RunDeferred()
}

func shot(a *app.App, name string) {
	now += 8000 // let old status messages expire
	frame(a)
	img := a.Frame()
	f, err := os.Create(filepath.Join(out, name+".png"))
	if err != nil {
		panic(err)
	}
	png.Encode(f, img.ToRGBA())
	f.Close()
	fmt.Println("saved", name)
}

func char(a *app.App, c rune) {
	a.Char(c)
	frame(a)
}

func key(a *app.App, k int, m app.Mods) {
	a.KeyDown(k, m)
	frame(a)
	a.KeyUp(k, m)
	frame(a)
}

func move(a *app.App, x, y int) {
	a.MouseMove(x, y)
	frame(a)
}

// stroke drags through picture coordinates.
func stroke(a *app.App, btn int, pts ...[2]int) {
	x, y := a.PicToScreenCenter(pts[0][0], pts[0][1])
	move(a, x, y)
	a.MouseDown(btn, x, y, false)
	frame(a)
	for _, p := range pts[1:] {
		x2, y2 := a.PicToScreenCenter(p[0], p[1])
		for i := 1; i <= 8; i++ {
			move(a, x+(x2-x)*i/8, y+(y2-y)*i/8)
		}
		x, y = x2, y2
	}
	a.MouseUp(btn, x, y)
	frame(a)
}

func hover(a *app.App, px, py int) {
	x, y := a.PicToScreenCenter(px, py)
	move(a, x, y)
}

func main() {
	out = os.Args[1]
	os.MkdirAll(out, 0o755)
	cfg := filepath.Join(out, ".settings.json")
	os.Remove(cfg)
	app.SettingsPath = func() string { return cfg }
	h := &host{}
	a := app.New(h)
	a.Resize(1360, 860)
	frame(a)
	key(a, app.KeyEsc, app.Mods{}) // first-start help
	h.open = os.Args[2]
	a.Debug("open", 0)
	frame(a)
	a.Debug("fg", 7)
	a.Debug("fg2", 9)

	char(a, 'g') // grid off for a clean view
	// 1 main window
	hover(a, 82, 60)
	shot(a, "01_main_window")

	// 1b layers: paint on layer 3 over the picture, panel scrolled to the layers
	char(a, '3')
	a.Debug("fg", 13)
	for i := 0; i < 4; i++ {
		char(a, '+')
	}
	stroke(a, 0, [2]int{20, 150}, [2]int{40, 140}, [2]int{60, 150}, [2]int{80, 140})
	a.Debug("fg", 1)
	stroke(a, 0, [2]int{24, 170}, [2]int{70, 166})
	a.Debug("panelscroll", 520)
	hover(a, 60, 150)
	shot(a, "01b_layers")
	a.Debug("panelscroll", 0)
	char(a, 'u')
	char(a, 'u')
	char(a, '2')
	for i := 0; i < 4; i++ {
		char(a, '-')
	}
	a.Debug("fg", 7)

	// 2 zoomed in: cells, grid and the cell info
	cx, cy := a.PicToScreenCenter(84, 62)
	for i := 0; i < 3; i++ {
		a.Wheel(1, cx, cy)
		frame(a)
	}
	char(a, 'g') // cell grid
	char(a, 'm') // pixel grid
	move(a, cx+8, cy+6)
	shot(a, "02_zoom_cells_and_grid")
	char(a, 'm')
	char(a, 'g')
	for i := 0; i < 3; i++ {
		a.Wheel(-1, cx, cy)
		frame(a)
	}

	// 3/4 Auto dither
	char(a, 'A')
	a.Debug("ditherview", 2)
	a.Debug("ditherstyle", 0)
	move(a, 1355, 855) // park the mouse: no tooltips
	shot(a, "03_auto_dither_artist")
	a.Debug("ditherstyle", 4)
	a.Debug("ditherview", 0)
	move(a, 1355, 855) // park the mouse: no tooltips
	shot(a, "04_auto_dither_color")
	key(a, app.KeyEsc, app.Mods{})

	// 5 dither brush, style Color: darken the floor
	char(a, 'D')
	a.Debug("dithersettings", 4)
	for i := 0; i < 8; i++ {
		char(a, '+')
	}
	zx, zy := a.PicToScreenCenter(70, 182)
	a.Wheel(1, zx, zy)
	frame(a)
	stroke(a, 0, [2]int{30, 186}, [2]int{60, 190}, [2]int{90, 188}, [2]int{112, 186})
	stroke(a, 0, [2]int{40, 176}, [2]int{70, 178})
	hover(a, 96, 176)
	shot(a, "05_dither_brush_color")
	char(a, 'u')
	char(a, 'u')
	a.Wheel(-1, zx, zy)
	frame(a)

	// 6 smear
	char(a, 'S')
	a.Debug("smearstrength", 70)
	zx, zy = a.PicToScreenCenter(118, 96)
	a.Wheel(1, zx, zy)
	frame(a)
	stroke(a, 0, [2]int{112, 82}, [2]int{124, 92}, [2]int{136, 104})
	stroke(a, 0, [2]int{104, 98}, [2]int{118, 106}, [2]int{132, 118})
	stroke(a, 0, [2]int{126, 80}, [2]int{134, 88}, [2]int{140, 98})
	hover(a, 132, 112)
	shot(a, "06_smear")
	char(a, 'u')
	char(a, 'u')
	char(a, 'u')
	a.Wheel(-1, zx, zy)
	frame(a)

	// 7 selection: rotate, scale and shear
	char(a, 'b')
	stroke(a, 0, [2]int{98, 70}, [2]int{136, 120})
	a.Debug("rotate", 25)
	frame(a)
	hover(a, 140, 60)
	shot(a, "07_rotate_scale_shear")
	key(a, app.KeyEsc, app.Mods{})
	char(a, 'd')

	// 8 swap colors
	a.Debug("swapdlg", 9<<4|6)
	move(a, 1355, 855) // park the mouse: no tooltips
	shot(a, "08_swap_colors")
	key(a, app.KeyEsc, app.Mods{})

	// 9 palettes
	a.Debug("palettedlg", 0)
	move(a, 1355, 855) // park the mouse: no tooltips
	shot(a, "09_palettes")
	key(a, app.KeyEsc, app.Mods{})

	// 10 import & convert
	h.open = os.Args[3]
	key(a, 'I', app.Mods{Ctrl: true})
	a.Debug("importdither", 3)
	a.Debug("importview", 2)
	move(a, 1355, 855) // park the mouse: no tooltips
	shot(a, "10_import_convert")
	key(a, app.KeyEsc, app.Mods{})

	// 11/12 dark theme and help
	a.Debug("theme", 1)
	hover(a, 82, 60)
	shot(a, "11_dark_theme")
	a.Debug("helpdither", 0)
	move(a, 1355, 855) // park the mouse: no tooltips
	shot(a, "12_help")
	key(a, app.KeyEsc, app.Mods{})
	a.Debug("theme", 0)
	a.Debug("about", 0)
	move(a, 1355, 855) // park the mouse: no tooltips
	shot(a, "13_about")
	key(a, app.KeyEsc, app.Mods{})

	// 14/15 gradient fill on the spare page
	char(a, 'j')
	a.Debug("gradientdlg", 1)
	move(a, 1355, 855)
	shot(a, "14_gradient_window")
	key(a, app.KeyEnter, app.Mods{})
	a.Debug("fg", 1)
	char(a, '.')
	char(a, '+')
	char(a, 'c')
	stroke(a, 0, [2]int{48, 100}, [2]int{78, 100})
	char(a, 'r')
	stroke(a, 0, [2]int{100, 30}, [2]int{150, 170})
	char(a, 'F')
	key(a, app.KeyEsc, app.Mods{})
	a.Debug("gradient", 1<<4|2<<8|2<<12|0<<20) // blue, light → dark → light, linear ↓, dithered
	stroke(a, 0, [2]int{10, 10})
	a.Debug("gradient", 1|1<<4|1<<8|8<<12) // fire, circle, dithered
	stroke(a, 0, [2]int{48, 100})
	a.Debug("gradient", 2|0<<8|12<<12) // forest, diamond
	stroke(a, 0, [2]int{125, 100})
	hover(a, 140, 20)
	shot(a, "15_gradient_fill")

	// 16 symmetry and the new shapes on the spare page
	for i := 0; i < 12; i++ {
		char(a, 'u')
	}
	char(a, 'X')
	a.Debug("symmode", 2)
	char(a, '.')
	char(a, '+')
	a.Debug("fg", 13)
	char(a, 'L')
	stroke(a, 0, [2]int{40, 50}, [2]int{40, 30})
	a.Debug("fg", 14)
	char(a, 'O')
	stroke(a, 0, [2]int{60, 90}, [2]int{60, 76})
	a.Debug("fg", 7)
	char(a, 'i')
	stroke(a, 0, [2]int{20, 85}, [2]int{20, 72})
	a.Debug("fg", 10)
	char(a, 'n')
	stroke(a, 0, [2]int{66, 20}, [2]int{72, 32})
	a.Debug("fg", 1)
	char(a, 'd')
	stroke(a, 0, [2]int{10, 10}, [2]int{30, 40}, [2]int{50, 60}, [2]int{75, 95})
	char(a, 'n')
	hover(a, 50, 60)
	shot(a, "16_symmetry_shapes")
	char(a, 'X')

	// 17 shade brush and CRT preview on the picture
	char(a, 'j')
	char(a, 'W')
	for i := 0; i < 4; i++ {
		char(a, '+')
	}
	stroke(a, 0, [2]int{90, 60}, [2]int{110, 70})
	char(a, 'V')
	hover(a, 30, 150)
	shot(a, "17_shade_and_crt")
	char(a, 'V')
	char(a, 'u')

	// 18 mirrored view
	char(a, 'M')
	move(a, 1355, 855)
	shot(a, "18_mirrored_view")
	char(a, 'M')
	fmt.Println("done")
}
