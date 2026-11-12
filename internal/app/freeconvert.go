package app

import (
	"fmt"
	"path/filepath"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// freeConvertDialog: the picture has free layers, which break the color
// rules. Before a C64 file is written, all layers are merged and every cell
// keeps its most frequent colors. The dialog shows the result first.
type freeConvertDialog struct {
	path    string
	kind    int
	save    bool // Save (marks the page saved) instead of Export
	flat    *c64.Picture
	compare int // 0 = converted, 1 = picture, 2 = split
}

func (a *App) openFreeConvert(path string, kind int, save bool) {
	a.modal = &freeConvertDialog{path: path, kind: kind, save: save, flat: a.pic.Flattened(a.pal)}
}

func (d *freeConvertDialog) Title() string          { return "Free layers: convert for the C64" }
func (d *freeConvertDialog) Size(a *App) (int, int) { return 760, min(640, a.H-30) }

func (d *freeConvertDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	y := r.Y
	for _, ln := range []string{
		"This picture has free layers – they do not follow the C64 color rules.",
		"To save " + filepath.Base(d.path) + ", all visible layers are merged and every cell",
		"keeps its most frequent colors; other pixels take the nearest of them.",
		"Your picture and its layers stay as they are.",
	} {
		m.Text(font.UI, r.X, y, ln, th.Text)
		y += 18
	}
	y += 8
	bw := 120
	for i, l := range []string{"Converted", "Picture", "Split"} {
		if u.ButtonEx("fc_v"+itoa(i), gfx.Rect{X: r.X + i*(bw+6), Y: y, W: bw, H: 24}, l, d.compare == i, true) {
			d.compare = i
		}
	}
	y += 32
	// preview (2× multicolor pixel aspect)
	pw := 640
	ph := 400
	pr := gfx.Rect{X: r.X + (r.W-pw)/2, Y: y, W: pw, H: ph}
	if pr.Bottom() > r.Bottom()-40 {
		ph = r.Bottom() - 40 - y
		pw = ph * 8 / 5
		pr = gfx.Rect{X: r.X + (r.W-pw)/2, Y: y, W: pw, H: ph}
	}
	src := [2]*c64.Picture{d.flat, a.pic}
	for py := 0; py < ph; py++ {
		yy := py * c64.H / ph
		for px := 0; px < pw; px++ {
			k := d.compare
			if k == 2 {
				k = 0
				if px >= pw/2 {
					k = 1
				}
			}
			p := src[k]
			xx := px * p.Width() / pw
			m.Set(pr.X+px, pr.Y+py, a.pal.Colors[p.ColorAt(xx, yy)])
		}
	}
	m.Frame(pr.Inset(-1), th.Border)
	if d.compare == 2 {
		m.Line(pr.X+pw/2, pr.Y, pr.X+pw/2, pr.Bottom()-1, 0xFFFFFF)
	}
	by := r.Bottom() - 30
	if u.Button("fc_cancel", gfx.Rect{X: r.Right() - 290, Y: by, W: 120, H: 30}, "Cancel") {
		a.closeModal(true)
		a.setStatus("Not saved – the picture has free layers")
	}
	if u.ButtonEx("fc_ok", gfx.Rect{X: r.Right() - 160, Y: by, W: 160, H: 30}, "Save converted", true, true) {
		a.closeModal(false)
		if a.writeExportOf(d.flat, d.path, d.kind) {
			a.rememberDir(d.path)
			if d.save {
				a.dirty[a.page] = false
				a.updateTitle()
			}
			a.setStatus(fmt.Sprintf("Saved converted: %s (all layers merged, free layers fitted to the color rules)", filepath.Base(d.path)))
		}
	}
}
