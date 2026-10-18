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
	img := image.NewNRGBA(image.Rect(0, 0, b.W*2, b.H))
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			v := b.Pix[y*b.W+x]
			if v < 0 {
				continue
			}
			c := pal.Colors[v&15]
			col := color.NRGBA{uint8(c >> 16), uint8(c >> 8), uint8(c), 255}
			img.SetNRGBA(x*2, y, col)
			img.SetNRGBA(x*2+1, y, col)
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
	double := w%2 == 0
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
	if bw > c64.W || h > c64.H {
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
		a.setStatus("No custom brush – make one from a selection or a stamp first")
		return
	}
	dir := a.brushesDir()
	_ = os.MkdirAll(dir, 0o755)
	p, ok := a.host.SaveFile("Save brush", brushFilters, dir, "brush.png", "png")
	if !ok {
		return
	}
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

// --- Stamps --------------------------------------------------------------------------
//
// Ready-made black silhouettes (people, animals, objects) in multicolor
// pixels with a transparent background, plus the brushes saved in the
// "brushes" folder. The chosen stamp becomes the brush; it paints in
// color 1 (left button) or color 2 (right button).

type stampDef struct {
	name, cat string
	w, h      int      // multicolor pixels
	rows      []string // '#' = set, '.' = transparent
}

func (s *stampDef) brush() *CustomBrush {
	b := &CustomBrush{W: s.w, H: s.h, Pix: make([]int8, s.w*s.h)}
	for y, row := range s.rows {
		for x := 0; x < s.w; x++ {
			b.Pix[y*s.w+x] = -1
			if x < len(row) && row[x] == '#' {
				b.Pix[y*s.w+x] = 0
			}
		}
	}
	return b
}

var stampTabs = []string{"People", "Animals", "Objects", "My brushes"}
var stampSizes = []struct {
	label string
	w, h  int
}{{"All sizes", 0, 0}, {"32 × 32", 16, 32}, {"32 × 64", 16, 64}, {"64 × 32", 32, 32}, {"64 × 64", 32, 64}}

type myBrush struct {
	name string
	b    *CustomBrush
}

type stampDialog struct {
	tab, size int
	sel       int
	scroll    int
	mine      []myBrush
}

func (a *App) openStamps() {
	a.commitOrCancelForHistory()
	a.menuOpen = -1
	d := &stampDialog{tab: clampI(a.set.StampTab, 0, len(stampTabs)-1), size: clampI(a.set.StampSize, 0, len(stampSizes)-1), sel: -1}
	d.loadMine(a)
	a.modal = d
}

// loadMine reads the PNG brushes of the brushes folder.
func (d *stampDialog) loadMine(a *App) {
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
		if len(d.mine) >= 200 {
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

func (d *stampDialog) Title() string          { return "Stamps" }
func (d *stampDialog) Size(a *App) (int, int) { return 900, min(680, a.H-30) }

// items returns the stamps of the current tab and size filter (indices
// into stampDefs or d.mine).
func (d *stampDialog) items() []int {
	var out []int
	if d.tab == 3 {
		for i, m := range d.mine {
			if d.fits(m.b.W, m.b.H) {
				out = append(out, i)
			}
		}
		return out
	}
	cat := stampTabs[d.tab]
	for i := range stampDefs {
		s := &stampDefs[i]
		if s.cat == cat && d.fits(s.w, s.h) {
			out = append(out, i)
		}
	}
	return out
}

func (d *stampDialog) fits(w, h int) bool {
	sz := stampSizes[d.size]
	if sz.w == 0 || d.tab == 3 { // own brushes have any size
		return true
	}
	return w == sz.w && h == sz.h
}

func (d *stampDialog) Key(a *App, k KeyEvent) {
	if k.Key == KeyEnter {
		d.use(a)
	}
}

func (d *stampDialog) use(a *App) {
	items := d.items()
	if d.sel < 0 || d.sel >= len(items) {
		return
	}
	a.set.StampTab, a.set.StampSize = d.tab, d.size
	a.saveSettings()
	a.closeModal(false)
	i := items[d.sel]
	if d.tab == 3 {
		m := d.mine[i]
		a.useBrush(m.b.clone(), true, "Brush: "+m.name+" – click to stamp")
		return
	}
	s := &stampDefs[i]
	a.useBrush(s.brush(), false, "Stamp: "+s.name+" – left click stamps in color 1, right click in color 2")
}

// drawBrushThumb draws a brush scaled into a box (stamps in black).
func (a *App) drawBrushThumb(b *CustomBrush, r gfx.Rect, black bool) {
	s := max(1, min(r.W/(b.W*2), r.H/b.H))
	w, h := b.W*2*s, b.H*s
	ox, oy := r.X+(r.W-w)/2, r.Y+(r.H-h)/2
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			v := b.Pix[y*b.W+x]
			if v < 0 {
				continue
			}
			c := uint32(0x000000)
			if !black {
				c = a.pal.Colors[v&15]
			}
			a.img.Fill(gfx.Rect{X: ox + x*2*s, Y: oy + y*s, W: 2 * s, H: s}, c)
		}
	}
}

func (d *stampDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	y := r.Y
	tw := (r.W - 3*4) / len(stampTabs)
	for i, t := range stampTabs {
		if u.ButtonEx("st_tab"+itoa(i), gfx.Rect{X: r.X + i*(tw+4), Y: y, W: tw, H: 26}, t, d.tab == i, true) && d.tab != i {
			d.tab, d.sel, d.scroll = i, -1, 0
		}
	}
	y += 34
	sw := 100
	for i, s := range stampSizes {
		if u.ButtonEx("st_sz"+itoa(i), gfx.Rect{X: r.X + i*(sw+4), Y: y, W: sw, H: 22}, s.label, d.size == i, true) && d.size != i {
			d.size, d.sel, d.scroll = i, -1, 0
		}
	}
	m.TextR(font.UI, gfx.Rect{X: r.X, Y: y, W: r.W, H: 22}, "sizes in screen pixels (320 × 200)", th.TextDim)
	y += 30
	// grid of thumbnails
	items := d.items()
	gr := gfx.Rect{X: r.X, Y: y, W: r.W, H: r.Bottom() - 40 - y}
	m.Fill(gr, th.PanelAlt)
	m.Frame(gr, th.Border)
	const cell = 84
	cols := max(1, (gr.W-8)/(cell+6))
	rows := (len(items) + cols - 1) / cols
	maxScroll := max(0, rows*(cell+22)+8-gr.H)
	if u.hover(gr) && a.in.Wheel != 0 && !u.wheelUsed {
		d.scroll -= a.in.Wheel * 60
		u.wheelUsed = true
	}
	d.scroll = clampI(d.scroll, 0, maxScroll)
	old := m.SetClip(gr.Inset(1))
	for k, idx := range items {
		col, row := k%cols, k/cols
		br := gfx.Rect{X: gr.X + 6 + col*(cell+6), Y: gr.Y + 6 + row*(cell+22) - d.scroll, W: cell, H: cell}
		if br.Bottom()+18 < gr.Y || br.Y > gr.Bottom() {
			continue
		}
		bg := uint32(0xD8D8D8)
		if d.sel == k {
			m.Fill(br.Inset(-3), th.Accent)
		} else if u.hover(br) {
			bg = 0xEFEFEF
		}
		m.Fill(br, bg)
		var name string
		if d.tab == 3 {
			mb := d.mine[idx]
			a.drawBrushThumb(mb.b, br.Inset(4), false)
			name = mb.name
		} else {
			s := &stampDefs[idx]
			a.drawBrushThumb(s.brush(), br.Inset(4), true)
			name = s.name
		}
		m.TextC(font.UI, gfx.Rect{X: br.X - 3, Y: br.Bottom() + 2, W: br.W + 6, H: 16}, gfx.Elide(font.UI, name, br.W+6), th.Text)
		if gr.Contains(a.in.MX, a.in.MY) {
			if c, _ := u.press("st_i"+itoa(k), br, 0); c {
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
	if len(items) == 0 {
		msg := "No stamps in this size"
		if d.tab == 3 {
			msg = "No brushes yet – save brushes with Brushes → Save brush … into " + a.brushesDir()
		}
		m.TextC(font.UI, gr, gfx.Elide(font.UI, msg, gr.W-20), th.TextDim)
	}
	// scrollbar
	if maxScroll > 0 {
		tr := gfx.Rect{X: gr.Right() - 7, Y: gr.Y + 2, W: 5, H: gr.H - 4}
		th0 := max(20, tr.H*gr.H/(gr.H+maxScroll))
		ty := tr.Y + (tr.H-th0)*d.scroll/maxScroll
		m.Fill(gfx.Rect{X: tr.X, Y: ty, W: tr.W, H: th0}, th.Track)
	}
	by := r.Bottom() - 28
	info := fmt.Sprintf("%d stamps", len(items))
	if d.sel >= 0 && d.sel < len(items) {
		if d.tab == 3 {
			b := d.mine[items[d.sel]].b
			info = fmt.Sprintf("%s · %d × %d screen pixels", d.mine[items[d.sel]].name, b.W*2, b.H)
		} else {
			s := &stampDefs[items[d.sel]]
			info = fmt.Sprintf("%s · %d × %d screen pixels · stamps in color 1 (left) or color 2 (right button)", s.name, s.w*2, s.h)
		}
	}
	m.TextL(font.UI, gfx.Rect{X: r.X, Y: by, W: r.W - 260, H: 28}, gfx.Elide(font.UI, info, r.W-260), th.TextDim)
	if u.Button("st_cancel", gfx.Rect{X: r.Right() - 250, Y: by, W: 120, H: 28}, "Cancel") {
		a.closeModal(true)
		return
	}
	if u.ButtonEx("st_ok", gfx.Rect{X: r.Right() - 120, Y: by, W: 120, H: 28}, "Use as brush", true, d.sel >= 0) {
		d.use(a)
	}
}
