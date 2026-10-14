package app

import (
	"fmt"

	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// Tasten (Buchstaben und Ziffern = ASCII-Großbuchstabe, Sondertasten ab 0x1000).
const (
	KeyEsc = 0x1000 + iota
	KeyEnter
	KeyDelete
	KeyBackspace
	KeyLeft
	KeyRight
	KeyUp
	KeyDown
	KeyTab
	KeySpace
	KeyHome
	KeyEnd
	KeyPgUp
	KeyPgDn
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyPlus
	KeyMinus
	KeyInsert
)

// Mods sind die Umschalttasten.
type Mods struct{ Shift, Ctrl, Alt bool }

// KeyEvent ist ein Tastendruck.
type KeyEvent struct {
	Key  int
	Mods Mods
}

// Input sammelt die Eingaben seit dem letzten Frame.
type Input struct {
	MX, MY   int
	Down     [3]bool
	Pressed  [3]bool
	Released [3]bool
	Double   bool
	Wheel    int
	Keys     []KeyEvent
	Chars    []rune
	Mods     Mods
	Space    bool
	Moved    bool
}

func (in *Input) endFrame() {
	in.Pressed = [3]bool{}
	in.Released = [3]bool{}
	in.Wheel = 0
	in.Keys = in.Keys[:0]
	in.Chars = in.Chars[:0]
	in.Double = false
	in.Moved = false
}

// UI ist ein kleines Immediate-Mode-GUI.
type UI struct {
	img     *gfx.Image
	th      *Theme
	in      *Input
	active  string
	blocked bool
	// Tooltip
	tipID, tipText string
	tipRect        gfx.Rect
	tipStart       int64
	tipShown       bool
	now            int64
	hoverID        string
	dragStartVal   int
	dragStartX     int
	wheelUsed      bool
}

func (u *UI) hover(r gfx.Rect) bool {
	return !u.blocked && r.Contains(u.in.MX, u.in.MY) && u.img.Clip.Contains(u.in.MX, u.in.MY)
}

// Tip registriert einen Tooltip für ein Element.
func (u *UI) Tip(id string, r gfx.Rect, text string) {
	if text == "" || !u.hover(r) {
		return
	}
	u.hoverID = id
	if u.tipID != id {
		u.tipID = id
		u.tipStart = u.now
		u.tipShown = false
	}
	u.tipText = L(text)
	u.tipRect = r
}

func (u *UI) bevelBox(r gfx.Rect, fill uint32, down bool) {
	th := u.th
	u.img.Fill(r, fill)
	u.img.Frame(r, th.BtnBorder)
	in := r.Inset(1)
	if down {
		u.img.Bevel(in, th.BevelDark, th.BevelLight)
	} else {
		u.img.Bevel(in, th.BevelLight, th.BevelDark)
	}
}

// press prüft Drücken/Loslassen; liefert (geklickt, gedrückt gehalten).
func (u *UI) press(id string, r gfx.Rect, btn int) (clicked, held bool) {
	h := u.hover(r)
	if h && u.in.Pressed[btn] && u.active == "" {
		u.active = fmt.Sprintf("%s#%d", id, btn)
	}
	key := fmt.Sprintf("%s#%d", id, btn)
	if u.active == key {
		held = h
		if u.in.Released[btn] || !u.in.Down[btn] {
			u.active = ""
			clicked = h
		}
	}
	return
}

// Button zeichnet eine Schaltfläche mit Text.
func (u *UI) Button(id string, r gfx.Rect, label string) bool {
	return u.ButtonEx(id, r, label, false, true)
}

// ButtonEx: selected = hervorgehoben, enabled = bedienbar.
func (u *UI) ButtonEx(id string, r gfx.Rect, label string, selected, enabled bool) bool {
	th := u.th
	clicked, held := false, false
	if enabled {
		clicked, held = u.press(id, r, 0)
	}
	fill := th.Btn
	tc := th.Text
	switch {
	case !enabled:
		tc = th.TextDim
	case selected:
		fill, tc = th.Accent, th.AccentText
	case held:
		fill = th.BtnDown
	case u.hover(r):
		fill = th.BtnHover
	}
	if selected {
		u.img.Fill(r, fill)
		u.img.Frame(r, th.BtnBorder)
	} else {
		u.bevelBox(r, fill, held)
	}
	f := font.UI
	off := 0
	if held {
		off = 1
	}
	u.img.TextC(f, gfx.Rect{X: r.X + off, Y: r.Y + off, W: r.W, H: r.H}, gfx.Elide(f, label, r.W-6), tc)
	return clicked
}

// Checkbox mit Beschriftung.
func (u *UI) Checkbox(id string, r gfx.Rect, label string, on bool) bool {
	th := u.th
	clicked, _ := u.press(id, r, 0)
	box := gfx.Rect{X: r.X, Y: r.Y + (r.H-13)/2, W: 13, H: 13}
	fill := th.PanelAlt
	if u.hover(r) {
		fill = th.BtnHover
	}
	u.img.Fill(box, fill)
	u.img.Frame(box, th.BtnBorder)
	if on {
		in := box.Inset(3)
		u.img.Fill(in, th.Accent)
	}
	u.img.TextL(font.UI, gfx.Rect{X: r.X + 19, Y: r.Y, W: r.W - 19, H: r.H}, gfx.Elide(font.UI, label, r.W-20), th.Text)
	if clicked {
		return !on
	}
	return on
}

// Radio zeichnet einen Auswahlknopf; liefert true bei Klick.
func (u *UI) Radio(id string, r gfx.Rect, label string, on bool) bool {
	th := u.th
	clicked, _ := u.press(id, r, 0)
	cx, cy := r.X+6, r.Y+r.H/2
	fill := th.PanelAlt
	if u.hover(r) {
		fill = th.BtnHover
	}
	circle(u.img, cx, cy, 6, th.BtnBorder)
	circle(u.img, cx, cy, 5, fill)
	if on {
		circle(u.img, cx, cy, 3, th.Accent)
	}
	u.img.TextL(font.UI, gfx.Rect{X: r.X + 18, Y: r.Y, W: r.W - 18, H: r.H}, gfx.Elide(font.UI, label, r.W-18), th.Text)
	return clicked
}

func circle(m *gfx.Image, cx, cy, r int, c uint32) {
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			d := x*x + y*y
			if d <= r*r {
				m.Set(cx+x, cy+y, c)
			} else if d <= r*r+r {
				m.BlendPix(cx+x, cy+y, c, 110)
			}
		}
	}
}

// Slider für ganze Zahlen. Mausrad ändert den Wert ebenfalls.
func (u *UI) Slider(id string, r gfx.Rect, label string, val, lo, hi int, unit string) int {
	th := u.th
	f := font.UI
	lw := 0
	if label != "" {
		lw = 78
		u.img.TextL(f, gfx.Rect{X: r.X, Y: r.Y, W: lw, H: r.H}, gfx.Elide(f, label, lw-4), th.Text)
	}
	vw := 40
	tr := gfx.Rect{X: r.X + lw, Y: r.Y, W: r.W - lw - vw, H: r.H}
	key := id + "#s"
	if u.hover(tr) && u.in.Pressed[0] && u.active == "" {
		u.active = key
	}
	if u.active == key {
		if !u.in.Down[0] {
			u.active = ""
		} else if tr.W > 8 {
			t := float64(u.in.MX-tr.X-4) / float64(tr.W-8)
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			val = lo + int(t*float64(hi-lo)+0.5)
		}
	}
	val = clampI(val, lo, hi)
	// Spur
	cy := tr.Y + tr.H/2
	track := gfx.Rect{X: tr.X + 4, Y: cy - 2, W: tr.W - 8, H: 4}
	u.img.Fill(track, th.Track)
	t := 0.0
	if hi > lo {
		t = float64(val-lo) / float64(hi-lo)
	}
	fx := track.X + int(t*float64(track.W))
	u.img.Fill(gfx.Rect{X: track.X, Y: track.Y, W: fx - track.X, H: track.H}, th.Accent)
	knob := gfx.Rect{X: fx - 4, Y: cy - 7, W: 9, H: 14}
	kf := th.Btn
	if u.active == key || u.hover(knob) {
		kf = th.BtnHover
	}
	u.bevelBox(knob, kf, false)
	u.img.TextR(f, gfx.Rect{X: r.Right() - vw, Y: r.Y, W: vw, H: r.H}, fmt.Sprintf("%d%s", val, unit), th.Text)
	return val
}

// Spinner: [-] Wert [+]
func (u *UI) Spinner(id string, r gfx.Rect, val, lo, hi, step int, unit string) int {
	th := u.th
	bw := 20
	if u.ButtonEx(id+"-", gfx.Rect{X: r.X, Y: r.Y, W: bw, H: r.H}, "−", false, val > lo) {
		val -= step
	}
	if u.ButtonEx(id+"+", gfx.Rect{X: r.Right() - bw, Y: r.Y, W: bw, H: r.H}, "+", false, val < hi) {
		val += step
	}
	mid := gfx.Rect{X: r.X + bw, Y: r.Y, W: r.W - 2*bw, H: r.H}
	u.img.Fill(mid, th.PanelAlt)
	u.img.Frame(mid, th.BtnBorder)
	if u.hover(mid) && u.in.Wheel != 0 && !u.wheelUsed {
		val += sign(u.in.Wheel) * step
		u.wheelUsed = true
	}
	u.img.TextC(font.UI, mid, fmt.Sprintf("%d%s", clampI(val, lo, hi), unit), th.Text)
	return clampI(val, lo, hi)
}

// Header zeichnet eine Abschnittsüberschrift.
func (u *UI) Header(r gfx.Rect, text string) {
	th := u.th
	u.img.Fill(r, th.Header)
	u.img.HLine(r.X, r.Right()-1, r.Bottom()-1, th.Border)
	u.img.TextL(font.UIB, gfx.Rect{X: r.X + 8, Y: r.Y, W: r.W - 8, H: r.H}, text, th.Text)
}

// Label zeichnet einfachen Text.
func (u *UI) Label(r gfx.Rect, text string, dim bool) {
	c := u.th.Text
	if dim {
		c = u.th.TextDim
	}
	u.img.TextL(font.UI, r, gfx.Elide(font.UI, text, r.W), c)
}

// drawTooltip zeichnet den Tooltip (nach kurzer Verzögerung).
func (u *UI) drawTooltip() {
	if u.hoverID == "" || u.tipID != u.hoverID {
		u.tipID = ""
		return
	}
	if u.now-u.tipStart < 550 {
		return
	}
	u.tipShown = true
	th := u.th
	f := font.UI
	w := f.Width(u.tipText) + 14
	h := f.Height + 8
	x := u.tipRect.X
	y := u.tipRect.Bottom() + 4
	if x+w > u.img.W-4 {
		x = u.img.W - 4 - w
	}
	if y+h > u.img.H-4 {
		y = u.tipRect.Y - h - 4
	}
	if x < 2 {
		x = 2
	}
	r := gfx.Rect{X: x, Y: y, W: w, H: h}
	u.img.FillA(gfx.Rect{X: x + 2, Y: y + 2, W: w, H: h}, th.Shadow, 70)
	u.img.Fill(r, th.Popup)
	u.img.Frame(r, th.PopupBorder)
	u.img.Text(f, x+7, y+4, u.tipText, th.Text)
}

func sign(v int) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

func clampI(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
