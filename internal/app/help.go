package app

import (
	"strings"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

type helpDialog struct {
	topic   string
	scroll  int
	history []string
	height  int
}

func (a *App) openHelp(topic string) {
	if strings.HasPrefix(topic, "tool_") {
		topic = "tools"
	}
	if topic == "" {
		topic = "start"
		if h, ok := a.modal.(*helpDialog); ok {
			topic = h.topic
		}
	}
	if h, ok := a.modal.(*helpDialog); ok {
		h.go2(topic)
		return
	}
	a.menuOpen = -1
	a.modal = &helpDialog{topic: topic}
}

func (h *helpDialog) go2(t string) {
	if t == h.topic {
		return
	}
	h.history = append(h.history, h.topic)
	h.topic = t
	h.scroll = 0
}

func (h *helpDialog) Title() string          { return "Help – " + AppName }
func (h *helpDialog) Size(a *App) (int, int) { return min(1040, a.W-24), min(760, a.H-24) }
func (h *helpDialog) Key(a *App, k KeyEvent) {
	switch k.Key {
	case KeyUp:
		h.scroll -= 40
	case KeyDown:
		h.scroll += 40
	case KeyPgUp:
		h.scroll -= 400
	case KeyPgDn, KeySpace:
		h.scroll += 400
	case KeyHome:
		h.scroll = 0
	case KeyBackspace, KeyLeft:
		h.back()
	}
}

func (h *helpDialog) back() {
	if n := len(h.history); n > 0 {
		h.topic = h.history[n-1]
		h.history = h.history[:n-1]
		h.scroll = 0
	}
}

func (h *helpDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	// Themenliste
	lw := 230
	lr := gfx.Rect{X: r.X - 4, Y: r.Y - 4, W: lw, H: r.H - 36}
	m.Fill(lr, th.PanelAlt)
	m.Frame(lr, th.Border)
	y := lr.Y + 6
	for _, t := range helpTopics {
		if t.group != "" {
			m.TextL(font.UIB, gfx.Rect{X: lr.X + 10, Y: y + 4, W: lw - 20, H: 20}, t.group, th.TextDim)
			y += 24
		}
		ir := gfx.Rect{X: lr.X + 3, Y: y, W: lw - 6, H: 23}
		tc := th.Text
		if t.id == h.topic {
			m.Fill(ir, th.Accent)
			tc = th.AccentText
		} else if u.hover(ir) {
			m.Fill(ir, th.BtnHover)
		}
		m.TextL(font.UI, gfx.Rect{X: ir.X + 14, Y: ir.Y, W: ir.W - 16, H: ir.H}, t.title, tc)
		if c, _ := u.press("ht_"+t.id, ir, 0); c {
			h.go2(t.id)
		}
		y += 24
	}
	// Inhalt
	cr := gfx.Rect{X: r.X + lw + 10, Y: r.Y - 4, W: r.W - lw - 10, H: r.H - 36}
	m.Fill(cr, th.PanelAlt)
	m.Frame(cr, th.Border)
	inner := gfx.Rect{X: cr.X + 22, Y: cr.Y + 1, W: cr.W - 44, H: cr.H - 2}
	maxS := max(0, h.height-inner.H+30)
	if u.hover(cr) && a.in.Wheel != 0 {
		h.scroll -= a.in.Wheel * 60
	}
	h.scroll = clampI(h.scroll, 0, maxS)
	old := m.SetClip(inner.Intersect(m.Clip))
	topic := findTopic(h.topic)
	h.height = renderHelp(a, h, topic.body, inner.X, inner.Y+16-h.scroll, inner.W) - (inner.Y + 16 - h.scroll) + 16
	m.SetClip(old)
	if h.height > inner.H {
		tr := gfx.Rect{X: cr.Right() - 10, Y: cr.Y + 4, W: 6, H: cr.H - 8}
		m.Fill(tr, th.Track)
		thh := max(24, tr.H*inner.H/h.height)
		ty := tr.Y + (tr.H-thh)*h.scroll/max(1, maxS)
		m.Fill(gfx.Rect{X: tr.X, Y: ty, W: tr.W, H: thh}, th.Accent)
	}
	// Fußzeile
	by := r.Bottom() - 26
	if u.ButtonEx("h_back", gfx.Rect{X: r.X - 4, Y: by, W: 100, H: 28}, "← Back", false, len(h.history) > 0) {
		h.back()
	}
	m.TextL(font.UI, gfx.Rect{X: r.X + 110, Y: by, W: 500, H: 28}, "Page Up/Down scrolls · Backspace = back · Esc closes", th.TextDim)
	if u.ButtonEx("h_close", gfx.Rect{X: r.Right() - 110, Y: by, W: 114, H: 28}, "Close", true, true) {
		a.closeModal(false)
	}
}

// --- Darstellung der Hilfetexte -------------------------------------------------------

type run struct {
	text  string
	style int // 0 normal, 1 fett, 2 Code/Taste
}

func parseInline(s string) []run {
	var out []run
	for len(s) > 0 {
		i := strings.IndexAny(s, "*`")
		if i < 0 {
			out = append(out, run{s, 0})
			break
		}
		if s[i] == '*' && strings.HasPrefix(s[i:], "**") {
			j := strings.Index(s[i+2:], "**")
			if j >= 0 {
				if i > 0 {
					out = append(out, run{s[:i], 0})
				}
				out = append(out, run{s[i+2 : i+2+j], 1})
				s = s[i+2+j+2:]
				continue
			}
		}
		if s[i] == '`' {
			j := strings.Index(s[i+1:], "`")
			if j >= 0 {
				if i > 0 {
					out = append(out, run{s[:i], 0})
				}
				out = append(out, run{s[i+1 : i+1+j], 2})
				s = s[i+1+j+1:]
				continue
			}
		}
		out = append(out, run{s[:i+1], 0})
		s = s[i+1:]
	}
	return out
}

// drawRich zeichnet umbrochenen Text mit Hervorhebungen. Liefert die neue y-Position.
func drawRich(a *App, s string, x, y, w int, base *font.Face, col uint32) int {
	m := a.img
	th := a.th
	lh := base.Height + 5
	cx := x
	for _, r := range parseInline(s) {
		f := base
		if r.style == 1 {
			f = font.BodyB
		}
		if r.style == 2 {
			f = font.Mono
			tw := f.Width(r.text) + 8
			if cx+tw > x+w && cx > x {
				cx = x
				y += lh
			}
			chip := gfx.Rect{X: cx, Y: y - 1, W: tw, H: base.Height + 3}
			m.RoundRect(chip, 2, th.Chip)
			m.Text(f, cx+4, y+(base.Height-f.Height)/2+1, r.text, th.ChipText)
			cx += tw + 2
			continue
		}
		words := strings.SplitAfter(r.text, " ")
		for _, wd := range words {
			if wd == "" {
				continue
			}
			ww := f.Width(strings.TrimRight(wd, " "))
			if cx+ww > x+w && cx > x {
				cx = x
				y += lh
				wd = strings.TrimLeft(wd, " ")
				ww = f.Width(strings.TrimRight(wd, " "))
			}
			m.Text(f, cx, y, wd, col)
			cx += f.Width(wd)
		}
	}
	return y + lh
}

func renderHelp(a *App, h *helpDialog, body string, x, y, w int) int {
	m := a.img
	th := a.th
	u := &a.ui
	lines := strings.Split(strings.TrimSpace(L(body)), "\n")
	para := []string{}
	flush := func() {
		if len(para) > 0 {
			y = drawRich(a, strings.Join(para, " "), x, y, w, font.Body, th.Text) + 6
			para = para[:0]
		}
	}
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		switch {
		case t == "":
			flush()
		case strings.HasPrefix(t, "# "):
			flush()
			m.Text(font.H1, x, y, t[2:], th.Text)
			y += font.H1.Height + 6
			m.Fill(gfx.Rect{X: x, Y: y, W: 48, H: 3}, th.Accent)
			y += 16
		case strings.HasPrefix(t, "## "):
			flush()
			y += 8
			txt := t[3:]
			ix := x
			if strings.HasPrefix(txt, "[") {
				if j := strings.Index(txt, "]"); j > 0 {
					ic := txt[1:j]
					txt = strings.TrimSpace(txt[j+1:])
					box := gfx.Rect{X: x, Y: y - 3, W: 24, H: 24}
					m.Fill(box, th.Btn)
					m.Frame(box, th.BtnBorder)
					if parts := strings.Split(ic, "|"); len(parts) == 2 {
						drawSplitIcon(m, parts[0], parts[1], box.X+4, box.Y+4, th.Text, th.Text)
					} else {
						drawIcon(m, ic, box.X+4, box.Y+4, th.Text, th.Accent)
					}
					ix += 32
				}
			}
			m.Text(font.H2, ix, y, txt, th.Text)
			y += font.H2.Height + 10
		case strings.HasPrefix(t, "- "):
			flush()
			circle(m, x+6, y+font.Body.Height/2+1, 2, th.Accent)
			y = drawRich(a, t[2:], x+18, y, w-18, font.Body, th.Text) + 2
		case strings.HasPrefix(t, "> "):
			flush()
			// Hinweisbox: Höhe zuerst messen
			saved := m.Clip
			m.SetClip(gfx.Rect{})
			hgt := drawRich(a, t[2:], x+14, 0, w-28, font.Body, th.Text)
			m.Clip = saved
			box := gfx.Rect{X: x, Y: y, W: w, H: hgt + 16}
			m.Fill(box, th.Note)
			m.Fill(gfx.Rect{X: x, Y: y, W: 4, H: box.H}, th.NoteBorder)
			drawRich(a, t[2:], x+16, y+9, w-28, font.Body, th.Text)
			y += box.H + 10
		case strings.HasPrefix(t, "= "):
			flush()
			parts := strings.SplitN(t[2:], "|", 2)
			k := strings.TrimSpace(parts[0])
			d := ""
			if len(parts) > 1 {
				d = strings.TrimSpace(parts[1])
			}
			kx := x
			k = strings.ReplaceAll(k, " / ", "\x00/\x00")
			k = strings.ReplaceAll(k, " or ", "\x00or\x00")
			for _, kk := range strings.Split(k, "\x00") {
				if kk == "" {
					continue
				}
				if kk == "/" || kk == "or" {
					kx += m.Text(font.Body, kx+2, y, kk, th.TextDim) + 6
					continue
				}
				tw := font.MonoB.Width(kk) + 12
				chip := gfx.Rect{X: kx, Y: y - 2, W: tw, H: font.Body.Height + 4}
				m.RoundRect(chip, 2, th.Btn)
				m.Frame(chip, th.BtnBorder)
				m.Text(font.MonoB, kx+6, y+1, kk, th.Text)
				kx += tw + 4
			}
			dx := max(190, kx-x+10)
			y = drawRich(a, d, x+dx, y, w-dx, font.Body, th.Text) + 4
		case strings.HasPrefix(t, "-> "):
			flush()
			parts := strings.SplitN(t[3:], "|", 2)
			id := strings.TrimSpace(parts[0])
			txt := "→ " + strings.TrimSpace(parts[len(parts)-1])
			tw := font.BodyB.Width(txt)
			lr := gfx.Rect{X: x, Y: y - 2, W: tw + 4, H: font.Body.Height + 4}
			c := th.Accent
			if u.hover(lr) {
				m.HLine(x, x+tw, y+font.BodyB.Height, c)
			}
			m.Text(font.BodyB, x, y, txt, c)
			if cl, _ := u.press("hl_"+id+txt, lr, 0); cl {
				h.go2(id)
			}
			y += font.Body.Height + 8
		case t == "@cells":
			flush()
			y = drawCellDiagram(a, x, y, w)
		case t == "@palette":
			flush()
			y = drawPaletteTable(a, x, y, w)
		case t == "@tools":
			flush()
			y = drawToolTable(a, x, y, w)
		default:
			para = append(para, t)
		}
	}
	flush()
	return y
}

// drawCellDiagram zeigt, wie eine 4×8-Zelle aus Bitpaaren ihre Farben bezieht.
func drawCellDiagram(a *App, x, y, w int) int {
	m := a.img
	th := a.th
	pal := a.pal
	// Beispielzelle
	cols := [4]uint8{0, 2, 7, 5} // Schwarz, Rot, Gelb, Grün
	pattern := []string{
		"0110",
		"1221",
		"1221",
		"0330",
		"0330",
		"3333",
		"0120",
		"1001",
	}
	ps := 22
	gx, gy := x, y+4
	for yy, row := range pattern {
		for xx, ch := range row {
			s := int(ch - '0')
			r := gfx.Rect{X: gx + xx*ps*2, Y: gy + yy*ps, W: ps * 2, H: ps}
			m.Fill(r, pal.Colors[cols[s]])
			m.Frame(r, th.Border)
			tc := uint32(0xFFFFFF)
			if luma(pal.Colors[cols[s]]) > 140 {
				tc = 0
			}
			m.TextC(font.Mono, r, []string{"00", "01", "10", "11"}[s], tc)
		}
	}
	// Legende
	lx := gx + 4*ps*2 + 30
	ly := gy
	leg := []struct{ bits, src, note string }{
		{"%00", "Background $D021", "the same for the whole picture"},
		{"%01", "Screen RAM bits 4–7", "free choice per cell"},
		{"%10", "Screen RAM bits 0–3", "free choice per cell"},
		{"%11", "Color RAM $D800", "free choice per cell"},
	}
	for i, l := range leg {
		sw := gfx.Rect{X: lx, Y: ly, W: 28, H: 28}
		m.Fill(sw, pal.Colors[cols[i]])
		m.Frame(sw, th.Border)
		m.Text(font.MonoB, lx+38, ly, l.bits, th.Text)
		m.Text(font.BodyB, lx+80, ly, l.src, th.Text)
		m.Text(font.Body, lx+80, ly+16, l.note+" – here: "+c64.ColorNames[cols[i]], th.TextDim)
		ly += 42
	}
	m.Text(font.Body, lx, ly+6, "One cell = 4 × 8 multicolor pixels = 8 × 8 screen pixels.", th.TextDim)
	m.Text(font.Body, lx, ly+24, "40 × 25 cells make up the 160 × 200 pixel picture.", th.TextDim)
	return gy + max(8*ps, 4*42+46) + 18
}

func drawPaletteTable(a *App, x, y, w int) int {
	m := a.img
	th := a.th
	cw := (w - 12) / 4
	for i := 0; i < 16; i++ {
		cx := x + (i%4)*(cw+4)
		cy := y + (i/4)*34
		sw := gfx.Rect{X: cx, Y: cy, W: 40, H: 28}
		m.Fill(sw, a.pal.Colors[i])
		m.Frame(sw, th.Border)
		m.Text(font.MonoB, cx+48, cy+1, itoa(i), th.Text)
		m.Text(font.Body, cx+70, cy+1, c64.ColorNames[i], th.Text)
		hex := strings.ToUpper(strings.TrimPrefix(gfxHex(a.pal.Colors[i]), "0x"))
		m.Text(font.Mono, cx+48, cy+15, "#"+hex, th.TextDim)
	}
	return y + 4*34 + 10
}

func gfxHex(c uint32) string {
	const d = "0123456789abcdef"
	b := make([]byte, 6)
	for i := 5; i >= 0; i-- {
		b[i] = d[c&15]
		c >>= 4
	}
	return string(b)
}

func drawToolTable(a *App, x, y, w int) int {
	m := a.img
	th := a.th
	for _, tb := range a.toolButtons() {
		box := gfx.Rect{X: x, Y: y, W: 26, H: 24}
		m.Fill(box, th.Btn)
		m.Frame(box, th.BtnBorder)
		if tb.alt != "" {
			drawSplitIcon(m, tb.icon, tb.alt, box.X+5, box.Y+4, th.Text, th.Text)
		} else {
			drawIcon(m, tb.icon, box.X+5, box.Y+4, th.Text, th.Accent)
		}
		m.Text(font.Body, x+36, y+4, tb.tip, th.Text)
		y += 28
	}
	return y + 6
}

type helpTopic struct {
	id, title, group, body string
}

func findTopic(id string) helpTopic {
	for _, t := range helpTopics {
		if t.id == id {
			return t
		}
	}
	return helpTopics[0]
}
