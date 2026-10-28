package app

import (
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- Text field ----------------------------------------------------------------------
//
// An editable one-line text with a cursor: click to place the cursor, drag or
// Shift+arrows to select, double-click selects all, Home/End, Backspace,
// Delete and Ctrl+A. Line breaks ("\n") are shown as ¶.

type textField struct {
	text   []rune
	cur    int // cursor position (0..len)
	anchor int // other end of the selection (== cur: no selection)
	focus  bool
	scroll int // horizontal scroll in pixels
	maxLen int // 0 = no limit
	drag   bool
}

func newTextField(s string, maxLen int) *textField {
	t := &textField{text: []rune(s), maxLen: maxLen, focus: true}
	t.SelectAll()
	return t
}

func (t *textField) String() string { return string(t.text) }

func (t *textField) Set(s string) {
	t.text = []rune(s)
	t.cur = min(t.cur, len(t.text))
	t.anchor = t.cur
}

func (t *textField) SelectAll() { t.anchor, t.cur = 0, len(t.text) }

func (t *textField) selRange() (int, int) {
	a, b := min(t.anchor, t.cur), max(t.anchor, t.cur)
	return clampI(a, 0, len(t.text)), clampI(b, 0, len(t.text))
}

func (t *textField) HasSelection() bool { return t.anchor != t.cur }

// deleteSel removes the selected text.
func (t *textField) deleteSel() bool {
	a, b := t.selRange()
	if a == b {
		return false
	}
	t.text = append(t.text[:a:a], t.text[b:]...)
	t.cur, t.anchor = a, a
	return true
}

// Insert types text at the cursor (replacing the selection).
func (t *textField) Insert(s string) {
	t.deleteSel()
	for _, r := range s {
		if t.maxLen > 0 && len(t.text) >= t.maxLen {
			break
		}
		t.text = append(t.text[:t.cur], append([]rune{r}, t.text[t.cur:]...)...)
		t.cur++
	}
	t.anchor = t.cur
}

// Char handles a typed character; returns true when it was used.
func (t *textField) Char(r rune) bool {
	if r < 32 || r == 127 {
		return false
	}
	t.Insert(string(r))
	return true
}

// Key handles editing keys; returns true when the key was used.
func (t *textField) Key(k KeyEvent) bool {
	move := func(to int) {
		t.cur = clampI(to, 0, len(t.text))
		if !k.Mods.Shift {
			t.anchor = t.cur
		}
	}
	switch {
	case k.Key == KeyLeft:
		if !k.Mods.Shift && t.HasSelection() {
			a, _ := t.selRange()
			t.cur, t.anchor = a, a
		} else {
			move(t.cur - 1)
		}
	case k.Key == KeyRight:
		if !k.Mods.Shift && t.HasSelection() {
			_, b := t.selRange()
			t.cur, t.anchor = b, b
		} else {
			move(t.cur + 1)
		}
	case k.Key == KeyHome:
		move(0)
	case k.Key == KeyEnd:
		move(len(t.text))
	case k.Key == KeyBackspace:
		if !t.deleteSel() && t.cur > 0 {
			t.text = append(t.text[:t.cur-1], t.text[t.cur:]...)
			t.cur--
			t.anchor = t.cur
		}
	case k.Key == KeyDelete:
		if !t.deleteSel() && t.cur < len(t.text) {
			t.text = append(t.text[:t.cur], t.text[t.cur+1:]...)
		}
	case k.Mods.Ctrl && k.Key == 'A':
		t.SelectAll()
	default:
		return false
	}
	return true
}

func shownRune(r rune) rune {
	if r == '\n' {
		return '¶'
	}
	return r
}

// xOf returns the pixel offset of position i.
func (t *textField) xOf(f *font.Face, i int) int {
	w := 0
	for _, r := range t.text[:clampI(i, 0, len(t.text))] {
		w += f.Glyph(shownRune(r)).Adv
	}
	return w
}

// posAt returns the cursor position for a pixel offset.
func (t *textField) posAt(f *font.Face, x int) int {
	w := 0
	for i, r := range t.text {
		adv := f.Glyph(shownRune(r)).Adv
		if x < w+adv/2 {
			return i
		}
		w += adv
	}
	return len(t.text)
}

// Draw draws the field and handles the mouse. It returns true when the
// field was clicked (it then has the focus).
func (t *textField) Draw(a *App, id string, r gfx.Rect) bool {
	u := &a.ui
	m := a.img
	th := a.th
	f := font.UI
	m.Fill(r, th.Popup)
	if t.focus {
		m.Frame(r, th.Accent)
	} else {
		m.Frame(r, th.BtnBorder)
	}
	inner := gfx.Rect{X: r.X + 6, Y: r.Y, W: r.W - 12, H: r.H}
	clicked := false
	in := &a.in
	if !u.blocked && in.Pressed[0] && r.Contains(in.MX, in.MY) {
		clicked = true
		t.focus = true
		p := t.posAt(f, in.MX-inner.X+t.scroll)
		if in.Double {
			t.SelectAll()
		} else {
			t.cur = p
			if !in.Mods.Shift {
				t.anchor = p
			}
			t.drag = true
			u.active = id
		}
	}
	if t.drag {
		if in.Down[0] {
			t.cur = t.posAt(f, in.MX-inner.X+t.scroll)
		} else {
			t.drag = false
		}
	}
	// keep the cursor visible
	cx := t.xOf(f, t.cur)
	if cx-t.scroll > inner.W-2 {
		t.scroll = cx - inner.W + 2
	}
	if cx-t.scroll < 0 {
		t.scroll = cx
	}
	if total := t.xOf(f, len(t.text)); total-t.scroll < inner.W-2 {
		t.scroll = max(0, total-inner.W+2)
	}
	old := m.SetClip(inner.Intersect(m.Clip))
	ty := r.Y + (r.H-f.Height)/2
	if t.focus && t.HasSelection() {
		s0, s1 := t.selRange()
		x0, x1 := t.xOf(f, s0), t.xOf(f, s1)
		m.FillA(gfx.Rect{X: inner.X + x0 - t.scroll, Y: r.Y + 4, W: x1 - x0, H: r.H - 8}, th.Accent, 110)
	}
	s := make([]rune, len(t.text))
	for i, rr := range t.text {
		s[i] = shownRune(rr)
	}
	m.Text(f, inner.X-t.scroll, ty, string(s), th.Text)
	if t.focus && (a.now/500)%2 == 0 {
		m.VLine(inner.X+cx-t.scroll, r.Y+5, r.Bottom()-6, th.Text)
	}
	m.SetClip(old)
	if t.focus {
		a.needAnim = true
	}
	return clicked
}
