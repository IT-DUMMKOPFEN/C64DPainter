package app

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- Brush files ---------------------------------------------------------------------
//
// Custom brushes are saved as PNG with a transparent background. Every
// multicolor pixel is two pixels wide, as on screen, so the files look right
// in other programs. Loading accepts both double-wide and single pixels.

var brushFilters = []Filter{{Name: "PNG brush (*.png)", Pattern: "*.png"}}

// brushesDir is the "brushes" folder in the working folder.
func (a *App) brushesDir() string {
	return filepath.Join(a.workDir(), "brushes")
}

func encodeBrushPNG(b *CustomBrush, pal *c64.Palette) ([]byte, error) {
	img := image.NewNRGBA(image.Rect(0, 0, b.W*asp, b.H))
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			v := b.Pix[y*b.W+x]
			if v < 0 {
				continue
			}
			c := pal.Colors[v&15]
			col := color.NRGBA{uint8(c >> 16), uint8(c >> 8), uint8(c), 255}
			for k := 0; k < asp; k++ {
				img.SetNRGBA(x*asp+k, y, col)
			}
		}
	}
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	return buf.Bytes(), err
}

// decodeBrush turns an image into a brush: transparent pixels stay
// transparent, colors become the nearest C64 color.
func decodeBrush(img image.Image, pal *c64.Palette) (*CustomBrush, error) {
	r := img.Bounds()
	w, h := r.Dx(), r.Dy()
	if w < 1 || h < 1 {
		return nil, fmt.Errorf("empty image")
	}
	px := func(x, y int) (int8, bool) {
		cr, cg, cb, ca := img.At(r.Min.X+x, r.Min.Y+y).RGBA()
		if ca < 0x8000 {
			return -1, true
		}
		// undo premultiplied alpha
		cr, cg, cb = cr*0xffff/ca, cg*0xffff/ca, cb*0xffff/ca
		return int8(pal.Nearest(int(cr>>8), int(cg>>8), int(cb>>8))), false
	}
	// double-wide pixels (as saved by this program)?
	double := w%2 == 0 && asp == 2 // hires pictures use single pixels
	for y := 0; y < h && double; y++ {
		for x := 0; x+1 < w; x += 2 {
			a, _ := px(x, y)
			b, _ := px(x+1, y)
			if a != b {
				double = false
				break
			}
		}
	}
	bw := w
	if double {
		bw = w / 2
	}
	if bw > picW || h > c64.H {
		return nil, fmt.Errorf("the brush is too large (at most 320 × 200 pixels)")
	}
	b := &CustomBrush{W: bw, H: h, Pix: make([]int8, bw*h)}
	for y := 0; y < h; y++ {
		for x := 0; x < bw; x++ {
			sx := x
			if double {
				sx = x * 2
			}
			b.Pix[y*bw+x], _ = px(sx, y)
		}
	}
	return b, nil
}

func (a *App) useBrush(b *CustomBrush, ownColors bool, status string) {
	if a.sel.state == selFloating {
		a.selCommit()
	}
	a.sel.state = selNone
	a.custom = b
	a.brushShape = BrushCustom
	a.customOwnColor = ownColors
	a.stampCache = nil
	a.setTool(ToolDots) // every click stamps once
	a.setStatus(status)
}

func (a *App) loadBrushDialog() {
	dir := a.brushesDir()
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		dir = a.workDir()
	}
	p, ok := a.host.OpenFile("Load brush", brushFilters, dir)
	if !ok {
		return
	}
	a.loadBrushFile(p)
}

func (a *App) loadBrushFile(p string) bool {
	data, err := a.host.ReadFile(p)
	if err != nil {
		a.host.Info(AppName, "Cannot read the brush:\n"+err.Error())
		return false
	}
	img, err := decodeImage(data, p)
	if err == nil {
		var b *CustomBrush
		if b, err = decodeBrush(img, a.pal); err == nil {
			a.useBrush(b, true, "Brush loaded: "+filepath.Base(p))
			return true
		}
	}
	a.host.Info(AppName, "Cannot use this image as a brush:\n"+err.Error())
	return false
}

func (a *App) saveBrushDialog() {
	if a.brushShape != BrushCustom || a.custom == nil {
		a.setStatus("No custom brush – make one from a selection first")
		return
	}
	dir := a.brushesDir()
	_ = os.MkdirAll(dir, 0o755)
	p, ok := a.host.SaveFile("Save brush", brushFilters, dir, "brush.png", "png")
	if !ok {
		return
	}
	p = FixDoubleExt(p)
	b := a.custom
	if !a.customOwnColor {
		// stamps and one-color brushes are saved in color 1
		b = b.clone()
		for i, v := range b.Pix {
			if v >= 0 {
				b.Pix[i] = int8(a.fg)
			}
		}
	}
	data, err := encodeBrushPNG(b, a.pal)
	if err == nil {
		err = a.host.WriteFile(p, data)
	}
	if err != nil {
		a.host.Info(AppName, "Saving the brush failed:\n"+err.Error())
		return
	}
	a.setStatus("Brush saved: " + filepath.Base(p))
}

// --- My brushes ---------------------------------------------------------------------
//
// A window with the brushes saved in the "brushes" folder of the working
// folder. The chosen brush becomes the current brush.

type myBrush struct {
	name string
	b    *CustomBrush
}

type myBrushesDialog struct {
	sel    int
	scroll int
	mine   []myBrush
}

func (a *App) openStamps() {
	a.commitOrCancelForHistory()
	a.menuOpen = -1
	d := &myBrushesDialog{sel: -1}
	d.loadMine(a)
	a.modal = d
}

// loadMine reads the PNG brushes of the brushes folder.
func (d *myBrushesDialog) loadMine(a *App) {
	d.mine = nil
	dir := a.brushesDir()
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".png") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		if len(d.mine) >= 300 {
			break
		}
		data, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			continue
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		if b, err := decodeBrush(img, a.pal); err == nil {
			d.mine = append(d.mine, myBrush{strings.TrimSuffix(n, filepath.Ext(n)), b})
		}
	}
}

func (d *myBrushesDialog) Title() string          { return "My brushes" }
func (d *myBrushesDialog) Size(a *App) (int, int) { return 900, min(640, a.H-30) }

func (d *myBrushesDialog) Key(a *App, k KeyEvent) {
	if k.Key == KeyEnter {
		d.use(a)
	}
}

func (d *myBrushesDialog) use(a *App) {
	if d.sel < 0 || d.sel >= len(d.mine) {
		return
	}
	a.closeModal(false)
	m := d.mine[d.sel]
	a.useBrush(m.b.clone(), true, "Brush: "+m.name+" – click to stamp")
}

// drawBrushThumb draws a brush scaled into a box.
func (a *App) drawBrushThumb(b *CustomBrush, r gfx.Rect) {
	a.drawPixelsFit(r, b.W, b.H, 8, func(x, y int) (uint32, bool) {
		if v := b.Pix[y*b.W+x]; v >= 0 {
			return a.pal.Colors[v&15], true
		}
		return 0, false
	})
}

// drawPixelsFit draws w × h picture pixels (asp screen pixels wide) centered
// into r: enlarged by a whole factor (at most maxScale) when they fit, else
// reduced smoothly so the whole brush is always visible.
func (a *App) drawPixelsFit(r gfx.Rect, w, h, maxScale int, get func(x, y int) (uint32, bool)) {
	if w <= 0 || h <= 0 || r.W <= 0 || r.H <= 0 {
		return
	}
	m := a.img
	old := m.SetClip(r.Intersect(m.Clip))
	defer m.SetClip(old)
	sw := w * asp // width in screen pixels
	if s := min(maxScale, r.W/sw, r.H/h); s >= 1 {
		ox, oy := r.X+(r.W-sw*s)/2, r.Y+(r.H-h*s)/2
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if c, ok := get(x, y); ok {
					m.Fill(gfx.Rect{X: ox + x*asp*s, Y: oy + y*s, W: asp * s, H: s}, c)
				}
			}
		}
		return
	}
	// too large: every target pixel averages the area it covers
	f := min(float64(r.W)/float64(sw), float64(r.H)/float64(h))
	tw, th := max(1, int(float64(sw)*f)), max(1, int(float64(h)*f))
	ox, oy := r.X+(r.W-tw)/2, r.Y+(r.H-th)/2
	for ty := 0; ty < th; ty++ {
		y0, y1 := ty*h/th, max((ty+1)*h/th, ty*h/th+1)
		for tx := 0; tx < tw; tx++ {
			x0, x1 := tx*sw/tw, max((tx+1)*sw/tw, tx*sw/tw+1)
			var rr, gg, bb, n, all int
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					all++
					if c, ok := get(x/asp, y); ok {
						rr += int(c>>16) & 255
						gg += int(c>>8) & 255
						bb += int(c) & 255
						n++
					}
				}
			}
			if n == 0 {
				continue
			}
			c := gfx.RGB(uint8(rr/n), uint8(gg/n), uint8(bb/n))
			if n*2 < all {
				// mostly transparent: blend with what is below
				m.BlendPix(ox+tx, oy+ty, c, n*256/all)
			} else {
				m.Set(ox+tx, oy+ty, c)
			}
		}
	}
}

func (d *myBrushesDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	y := r.Y
	m.TextL(font.UI, gfx.Rect{X: r.X, Y: y, W: r.W - 130, H: 24}, gfx.Elide(font.UI, "Folder: "+a.brushesDir(), r.W-140), th.TextDim)
	if u.Button("mb_reload", gfx.Rect{X: r.Right() - 120, Y: y, W: 120, H: 24}, "Reload") {
		d.loadMine(a)
		d.sel = -1
	}
	y += 32
	gr := gfx.Rect{X: r.X, Y: y, W: r.W, H: r.Bottom() - 40 - y}
	m.Fill(gr, th.PanelAlt)
	m.Frame(gr, th.Border)
	const cell = 84
	cols := max(1, (gr.W-8)/(cell+6))
	rows := (len(d.mine) + cols - 1) / cols
	maxScroll := max(0, rows*(cell+22)+8-gr.H)
	if u.hover(gr) && a.in.Wheel != 0 && !u.wheelUsed {
		d.scroll -= a.in.Wheel * 60
		u.wheelUsed = true
	}
	d.scroll = clampI(d.scroll, 0, maxScroll)
	old := m.SetClip(gr.Inset(1))
	for k, mb := range d.mine {
		col, row := k%cols, k/cols
		br := gfx.Rect{X: gr.X + 6 + col*(cell+6), Y: gr.Y + 6 + row*(cell+22) - d.scroll, W: cell, H: cell}
		if br.Bottom()+18 < gr.Y || br.Y > gr.Bottom() {
			continue
		}
		if d.sel == k {
			m.Fill(br.Inset(-3), th.Accent)
		}
		m.Fill(br, a.pal.Colors[a.pic.BG])
		a.drawBrushThumb(mb.b, br.Inset(4))
		m.TextC(font.UI, gfx.Rect{X: br.X - 3, Y: br.Bottom() + 2, W: br.W + 6, H: 16}, gfx.Elide(font.UI, mb.name, br.W+6), th.Text)
		if gr.Contains(a.in.MX, a.in.MY) {
			if c, _ := u.press("mb_i"+itoa(k), br, 0); c {
				d.sel = k
				if a.in.Double {
					d.use(a)
					m.SetClip(old)
					return
				}
			}
		}
	}
	m.SetClip(old)
	if len(d.mine) == 0 {
		m.TextC(font.UI, gr, "No brushes yet – save brushes with Brushes → Save brush …", th.TextDim)
	}
	if maxScroll > 0 {
		tr := gfx.Rect{X: gr.Right() - 7, Y: gr.Y + 2, W: 5, H: gr.H - 4}
		th0 := max(20, tr.H*gr.H/(gr.H+maxScroll))
		ty := tr.Y + (tr.H-th0)*d.scroll/maxScroll
		m.Fill(gfx.Rect{X: tr.X, Y: ty, W: tr.W, H: th0}, th.Track)
	}
	by := r.Bottom() - 28
	info := fmt.Sprintf("%d brushes", len(d.mine))
	if d.sel >= 0 && d.sel < len(d.mine) {
		b := d.mine[d.sel].b
		info = fmt.Sprintf("%s · %d × %d screen pixels", d.mine[d.sel].name, b.W*asp, b.H)
	}
	m.TextL(font.UI, gfx.Rect{X: r.X, Y: by, W: r.W - 260, H: 28}, gfx.Elide(font.UI, info, r.W-260), th.TextDim)
	if u.Button("mb_cancel", gfx.Rect{X: r.Right() - 250, Y: by, W: 120, H: 28}, "Cancel") {
		a.closeModal(true)
		return
	}
	if u.ButtonEx("mb_ok", gfx.Rect{X: r.Right() - 120, Y: by, W: 120, H: 28}, "Use as brush", true, d.sel >= 0) {
		d.use(a)
	}
}
