package app

import (
	"strings"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- Text tool -----------------------------------------------------------------------
//
// Text is written in multicolor pixels and becomes the brush: C64 characters
// with every pixel a wide multicolor pixel, twice as high, or an own narrow
// font that fits one cell. Text in one color plus an optional shadow or
// outline in a second color, so a cell never gets more than two of them.

// TextSettings are saved with the settings.
type TextSettings struct {
	Font    int    `json:"font"`
	Lower   bool   `json:"lower"`
	Spacing int    `json:"spacing"`
	Effect  int    `json:"effect"` // 0 none, 1 shadow, 2 outline
	Last    string `json:"last"`   // the last text
}

var textEffects = []string{"None", "Shadow", "Outline"}

type textDialog struct {
	text          string
	color, color2 uint8
}

func (a *App) openTextDialog() {
	a.commitOrCancelForHistory()
	a.menuOpen = -1
	t := a.set.Text.Last
	if t == "" {
		t = "C64 DPAINTER"
	}
	a.modal = &textDialog{text: t, color: a.fg, color2: a.fg2}
}

func (d *textDialog) Title() string          { return "Text" }
func (d *textDialog) Size(a *App) (int, int) { return 760, min(600, a.H-20) }

func (d *textDialog) Char(a *App, r rune) {
	if r >= 32 && len([]rune(d.text)) < 200 {
		d.text += string(r)
	}
}

func (d *textDialog) Key(a *App, k KeyEvent) {
	switch k.Key {
	case KeyBackspace:
		if rs := []rune(d.text); len(rs) > 0 {
			d.text = string(rs[:len(rs)-1])
		}
	case KeyEnter:
		if k.Mods.Shift {
			if strings.Count(d.text, "\n") < 7 {
				d.text += "\n"
			}
			return
		}
		d.use(a)
	}
}

// brush renders the text with the current settings.
func (d *textDialog) brush(a *App) *CustomBrush {
	s := a.set.Text
	gap := 0
	if s.Effect != 0 {
		gap = 2 // room for the shadow or outline between lines
	}
	mask, w, h := c64.RenderText(d.text, c64.TextFont(clampI(s.Font, 0, 2)), s.Lower, clampI(s.Spacing, 0, 4), gap)
	if w == 0 || h == 0 {
		return nil
	}
	pad := 0
	if s.Effect != 0 {
		pad = 1
	}
	bw, bh := w+pad*2, h+pad*2
	if bw > c64.W {
		bw = c64.W
	}
	if bh > c64.H {
		bh = c64.H
	}
	b := &CustomBrush{W: bw, H: bh, Pix: make([]int8, bw*bh)}
	for i := range b.Pix {
		b.Pix[i] = -1
	}
	at := func(x, y int) bool {
		x, y = x-pad, y-pad
		return x >= 0 && y >= 0 && x < w && y < h && mask[y*w+x]
	}
	for y := 0; y < bh; y++ {
		for x := 0; x < bw; x++ {
			switch {
			case at(x, y):
				b.Pix[y*bw+x] = int8(d.color)
			case s.Effect == 1 && at(x-1, y-1):
				b.Pix[y*bw+x] = int8(d.color2)
			case s.Effect == 2 && (at(x-1, y) || at(x+1, y) || at(x, y-1) || at(x, y+1) ||
				at(x-1, y-1) || at(x+1, y-1) || at(x-1, y+1) || at(x+1, y+1)):
				b.Pix[y*bw+x] = int8(d.color2)
			}
		}
	}
	return b
}

func (d *textDialog) use(a *App) {
	b := d.brush(a)
	if b == nil {
		return
	}
	a.set.Text.Last = d.text
	a.saveSettings()
	a.closeModal(false)
	a.useBrush(b, true, "Text brush – click to place it  ·  X / Y flip, H / Shift+H size")
}

func (d *textDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	s := &a.set.Text
	y := r.Y
	// text field
	m.Text(font.UIB, r.X, y+6, "Text", th.Text)
	tf := gfx.Rect{X: r.X + 70, Y: y, W: r.W - 70, H: 28}
	m.Fill(tf, th.Popup)
	m.Frame(tf, th.Accent)
	shown := strings.ReplaceAll(d.text, "\n", " ¶ ")
	tw := m.Text(font.UI, tf.X+6, tf.Y+(tf.H-font.UI.Height)/2, gfx.Elide(font.UI, shown, tf.W-14), th.Text)
	if (a.now/500)%2 == 0 {
		m.VLine(tf.X+7+min(tw, tf.W-14), tf.Y+5, tf.Bottom()-6, th.Text)
	}
	a.needAnim = true
	y += 34
	m.Text(font.UI, r.X+70, y, "Type the text  ·  Shift+Enter: new line  ·  Enter: use as brush", th.TextDim)
	y += 24
	row := func(label string, names []string, cur *int, id string) {
		m.Text(font.UIB, r.X, y+5, label, th.Text)
		bw := min(160, (r.W-90-4*(len(names)-1))/len(names))
		for i, n := range names {
			if u.ButtonEx(id+itoa(i), gfx.Rect{X: r.X + 90 + i*(bw+4), Y: y, W: bw, H: 24}, n, *cur == i, true) {
				*cur = i
			}
		}
		y += 32
	}
	row("Font", c64.TextFontNames[:], &s.Font, "tx_f")
	cs := 0
	if s.Lower {
		cs = 1
	}
	row("Letters", []string{"UPPER CASE", "Lower case"}, &cs, "tx_l")
	s.Lower = cs == 1
	row("Effect", textEffects, &s.Effect, "tx_e")
	m.Text(font.UIB, r.X, y+5, "Spacing", th.Text)
	s.Spacing = u.Spinner("tx_sp", gfx.Rect{X: r.X + 90, Y: y, W: 140, H: 24}, clampI(s.Spacing, 0, 4), 0, 4, 1, " px")
	y += 32
	// colors
	pal := func(label string, cur *uint8, id string) {
		m.Text(font.UIB, r.X, y+5, label, th.Text)
		for i := 0; i < 16; i++ {
			br := gfx.Rect{X: r.X + 90 + i*26, Y: y, W: 24, H: 24}
			m.Fill(br, a.pal.Colors[i])
			m.Frame(br, th.BtnBorder)
			if *cur == uint8(i) {
				m.Frame(br.Inset(-2), th.Accent)
				m.Frame(br.Inset(-1), th.Accent)
			}
			if c, _ := u.press(id+itoa(i), br, 0); c {
				*cur = uint8(i)
			}
			u.Tip(id+itoa(i), br, c64.ColorNames[i])
		}
		y += 32
	}
	pal("Text color", &d.color, "tx_c")
	if s.Effect != 0 {
		pal(textEffects[s.Effect]+" color", &d.color2, "tx_c2")
	} else {
		y += 32
	}
	// preview on the background color
	pr := gfx.Rect{X: r.X, Y: y, W: r.W, H: r.Bottom() - 40 - y}
	m.Fill(pr, a.pal.Colors[a.pic.BG])
	m.Frame(pr, th.Border)
	if b := d.brush(a); b != nil {
		sc := max(1, min(4, min(pr.W/(b.W*2+4), pr.H/(b.H+4))))
		ox := pr.X + (pr.W-b.W*2*sc)/2
		oy := pr.Y + (pr.H-b.H*sc)/2
		old := m.SetClip(pr.Inset(1).Intersect(m.Clip))
		for yy := 0; yy < b.H; yy++ {
			for xx := 0; xx < b.W; xx++ {
				if v := b.Pix[yy*b.W+xx]; v >= 0 {
					m.Fill(gfx.Rect{X: ox + xx*2*sc, Y: oy + yy*sc, W: 2 * sc, H: sc}, a.pal.Colors[v])
				}
			}
		}
		m.SetClip(old)
		m.TextL(font.UI, gfx.Rect{X: r.X, Y: r.Bottom() - 28, W: r.W - 270, H: 28},
			itoa(b.W*2)+" × "+itoa(b.H)+" screen pixels – the text becomes the brush", th.TextDim)
	}
	by := r.Bottom() - 28
	if u.Button("tx_cancel", gfx.Rect{X: r.Right() - 250, Y: by, W: 120, H: 28}, "Cancel") {
		a.closeModal(true)
		return
	}
	if u.ButtonEx("tx_ok", gfx.Rect{X: r.Right() - 120, Y: by, W: 120, H: 28}, "Use as brush", true, strings.TrimSpace(d.text) != "") {
		d.use(a)
	}
}
