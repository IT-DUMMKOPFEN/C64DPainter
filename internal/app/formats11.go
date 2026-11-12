package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- More file formats (1.2) ---------------------------------------------------------

// writeRaw saves name.asm plus name_bitmap.bin, name_screen.bin and (for
// multicolor) name_color.bin – plain data for programmers.
func (a *App) writeRaw(pic *c64.Picture, p string) bool {
	stem := strings.TrimSuffix(p, filepath.Ext(p))
	label := acmeLabel(filepath.Base(stem))
	bm, scr, col, src := pic.RawParts(a.pal, label)
	dir := filepath.Dir(p)
	files := []struct {
		name string
		data []byte
	}{{label + "_bitmap.bin", bm}, {label + "_screen.bin", scr}}
	if col != nil {
		files = append(files, struct {
			name string
			data []byte
		}{label + "_color.bin", col})
	}
	for _, f := range files {
		if err := a.host.WriteFile(filepath.Join(dir, f.name), f.data); err != nil {
			a.host.Info(AppName, "Saving failed:\n"+err.Error())
			return false
		}
	}
	if err := a.host.WriteFile(filepath.Join(dir, label+".asm"), []byte(src)); err != nil {
		a.host.Info(AppName, "Saving failed:\n"+err.Error())
		return false
	}
	a.setStatus(fmt.Sprintf("Saved %s.asm and %d .bin files", label, len(files)))
	return true
}

// acmeLabel turns a file name into a valid ACME label.
func acmeLabel(s string) string {
	var b strings.Builder
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			if i == 0 {
				b.WriteRune('_')
			}
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "picture"
	}
	return b.String()
}

// formatsDialog lists the other C64 formats for the current mode.
type formatsDialog struct{}

func (a *App) openFormatsDialog() { a.modal = &formatsDialog{} }

func (d *formatsDialog) Title() string { return "Export in another C64 format" }
func (d *formatsDialog) Size(a *App) (int, int) {
	n := 1
	for _, f := range c64.Formats {
		if f.Save != nil && f.Hires == a.hires() {
			n++
		}
	}
	return 540, 120 + n*34
}

func (d *formatsDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	mode := "multicolor"
	if a.hires() {
		mode = "hires"
	}
	m.Text(font.UI, r.X, r.Y, "Formats for "+mode+" pictures:", a.th.TextDim)
	y := r.Y + 24
	row := func(id, label, info string, f func()) {
		br := gfx.Rect{X: r.X, Y: y, W: 230, H: 28}
		if u.Button(id, br, label) {
			a.closeModal(false)
			a.later(f)
		}
		m.Text(font.UI, br.Right()+12, y+7, info, a.th.TextDim)
		y += 34
	}
	for i, f := range c64.Formats {
		if f.Save == nil || f.Hires != a.hires() {
			continue
		}
		i := i
		row("fx"+itoa(i), f.Name+" …", "*"+strings.Join(f.Exts, ", *")+fmtInfo(f.Name), func() { a.cmdExport(exportFmt + i) })
	}
	row("fxraw", "Raw data + ACME source …", ".bin files for your own programs", func() { a.cmdExport(exportRaw) })
	if u.Button("fx_close", gfx.Rect{X: r.Right() - 100, Y: r.Bottom() - 28, W: 100, H: 28}, "Close") {
		a.closeModal(false)
	}
}

func fmtInfo(name string) string {
	switch name {
	case "Amica Paint", "Koala Painter compressed", "Drazpaint", "Doodle compressed":
		return " – packed"
	case "Advanced Art Studio":
		return " – with border color"
	}
	return ""
}
