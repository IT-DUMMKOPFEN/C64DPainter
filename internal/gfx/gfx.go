// Package gfx ist ein kleiner Software-Renderer (32-Bit-Puffer 0x00RRGGBB).
package gfx

import (
	"image"
	"image/color"

	"c64dpainter/internal/font"
)

// Rect ist ein Rechteck mit X,Y,W,H.
type Rect struct{ X, Y, W, H int }

func R(x, y, w, h int) Rect { return Rect{x, y, w, h} }

func (r Rect) Contains(x, y int) bool {
	return x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H
}
func (r Rect) Inset(d int) Rect { return Rect{r.X + d, r.Y + d, r.W - 2*d, r.H - 2*d} }
func (r Rect) Right() int       { return r.X + r.W }
func (r Rect) Bottom() int      { return r.Y + r.H }
func (r Rect) Empty() bool      { return r.W <= 0 || r.H <= 0 }

func (r Rect) Intersect(o Rect) Rect {
	x0, y0 := max(r.X, o.X), max(r.Y, o.Y)
	x1, y1 := min(r.Right(), o.Right()), min(r.Bottom(), o.Bottom())
	if x1 <= x0 || y1 <= y0 {
		return Rect{x0, y0, 0, 0}
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

// Image ist der Zeichenpuffer.
type Image struct {
	W, H int
	Pix  []uint32
	Clip Rect
}

func New(w, h int) *Image {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return &Image{W: w, H: h, Pix: make([]uint32, w*h), Clip: Rect{0, 0, w, h}}
}

func (m *Image) SetClip(r Rect) Rect {
	old := m.Clip
	m.Clip = r.Intersect(Rect{0, 0, m.W, m.H})
	return old
}
func (m *Image) ResetClip() { m.Clip = Rect{0, 0, m.W, m.H} }

// RGB baut eine Farbe.
func RGB(r, g, b uint8) uint32 { return uint32(r)<<16 | uint32(g)<<8 | uint32(b) }

func Split(c uint32) (r, g, b uint32) { return c >> 16 & 255, c >> 8 & 255, c & 255 }

// Mix mischt a und b mit Anteil t (0..256) von b.
func Mix(a, b uint32, t int) uint32 {
	if t <= 0 {
		return a
	}
	if t >= 256 {
		return b
	}
	ar, ag, ab := Split(a)
	br, bg, bb := Split(b)
	u := uint32(t)
	v := 256 - u
	return ((ar*v+br*u)>>8)<<16 | ((ag*v+bg*u)>>8)<<8 | (ab*v+bb*u)>>8
}

func (m *Image) Set(x, y int, c uint32) {
	if m.Clip.Contains(x, y) {
		m.Pix[y*m.W+x] = c
	}
}

func (m *Image) Get(x, y int) uint32 {
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return 0
	}
	return m.Pix[y*m.W+x]
}

func (m *Image) BlendPix(x, y int, c uint32, a int) {
	if m.Clip.Contains(x, y) {
		i := y*m.W + x
		m.Pix[i] = Mix(m.Pix[i], c, a)
	}
}

func (m *Image) Fill(r Rect, c uint32) {
	r = r.Intersect(m.Clip)
	if r.Empty() {
		return
	}
	for y := r.Y; y < r.Bottom(); y++ {
		row := m.Pix[y*m.W+r.X : y*m.W+r.Right()]
		for i := range row {
			row[i] = c
		}
	}
}

// FillA füllt mit Transparenz a (0..256).
func (m *Image) FillA(r Rect, c uint32, a int) {
	r = r.Intersect(m.Clip)
	if r.Empty() {
		return
	}
	for y := r.Y; y < r.Bottom(); y++ {
		row := m.Pix[y*m.W+r.X : y*m.W+r.Right()]
		for i := range row {
			row[i] = Mix(row[i], c, a)
		}
	}
}

func (m *Image) HLine(x0, x1, y int, c uint32) {
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	m.Fill(Rect{x0, y, x1 - x0 + 1, 1}, c)
}

func (m *Image) VLine(x, y0, y1 int, c uint32) {
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	m.Fill(Rect{x, y0, 1, y1 - y0 + 1}, c)
}

// Frame zeichnet einen 1-Pixel-Rahmen innen.
func (m *Image) Frame(r Rect, c uint32) {
	if r.Empty() {
		return
	}
	m.HLine(r.X, r.Right()-1, r.Y, c)
	m.HLine(r.X, r.Right()-1, r.Bottom()-1, c)
	m.VLine(r.X, r.Y, r.Bottom()-1, c)
	m.VLine(r.Right()-1, r.Y, r.Bottom()-1, c)
}

// Bevel zeichnet einen 3D-Rahmen (hell oben links, dunkel unten rechts).
func (m *Image) Bevel(r Rect, light, dark uint32) {
	m.HLine(r.X, r.Right()-1, r.Y, light)
	m.VLine(r.X, r.Y, r.Bottom()-1, light)
	m.HLine(r.X, r.Right()-1, r.Bottom()-1, dark)
	m.VLine(r.Right()-1, r.Y, r.Bottom()-1, dark)
}

// Line zeichnet eine Linie (Bresenham).
func (m *Image) Line(x0, y0, x1, y1 int, c uint32) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	e := dx + dy
	for {
		m.Set(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			x0 += sx
		}
		if e2 <= dx {
			e += dx
			y0 += sy
		}
	}
}

// RoundRect füllt ein Rechteck mit abgerundeten Ecken (Radius 2 oder 3).
func (m *Image) RoundRect(r Rect, rad int, c uint32) {
	if r.Empty() {
		return
	}
	for y := 0; y < r.H; y++ {
		inset := 0
		if y < rad {
			inset = rad - y
		} else if y >= r.H-rad {
			inset = y - (r.H - rad - 1)
		}
		if rad >= 3 && inset > 0 {
			inset = (inset*inset + rad - 1) / rad
		}
		m.HLine(r.X+inset, r.Right()-1-inset, r.Y+y, c)
	}
}

// HeartRed is the color of the heart character ❤ (U+2764).
const HeartRed uint32 = 0xE02030

// Text zeichnet s mit Oberkante bei y. Gibt die Breite zurück.
func (m *Image) Text(f *font.Face, x, y int, s string, c uint32) int {
	ox := x
	base := y + f.Ascent
	col := c
	for _, r := range s {
		g := f.Glyph(r)
		c = col
		if r == '\u2764' { // the heart is always red
			c = HeartRed
		}
		if g.W > 0 {
			gx, gy := x+g.BX, base+g.BY
			// schneller Test gegen Clip
			if gx+g.W > m.Clip.X && gx < m.Clip.Right() && gy+g.H > m.Clip.Y && gy < m.Clip.Bottom() {
				for yy := 0; yy < g.H; yy++ {
					py := gy + yy
					if py < m.Clip.Y || py >= m.Clip.Bottom() {
						continue
					}
					row := g.A[yy*g.W : yy*g.W+g.W]
					for xx, a := range row {
						if a == 0 {
							continue
						}
						px := gx + xx
						if px < m.Clip.X || px >= m.Clip.Right() {
							continue
						}
						i := py*m.W + px
						if a == 255 {
							m.Pix[i] = c
						} else {
							m.Pix[i] = Mix(m.Pix[i], c, int(a)+1)
						}
					}
				}
			}
		}
		x += g.Adv
	}
	return x - ox
}

// TextC zentriert Text in r.
func (m *Image) TextC(f *font.Face, r Rect, s string, c uint32) {
	w := f.Width(s)
	m.Text(f, r.X+(r.W-w)/2, r.Y+(r.H-f.Height)/2, s, c)
}

// TextL zeichnet Text linksbündig, vertikal zentriert.
func (m *Image) TextL(f *font.Face, r Rect, s string, c uint32) {
	m.Text(f, r.X, r.Y+(r.H-f.Height)/2, s, c)
}

// TextR zeichnet Text rechtsbündig, vertikal zentriert.
func (m *Image) TextR(f *font.Face, r Rect, s string, c uint32) {
	m.Text(f, r.Right()-f.Width(s), r.Y+(r.H-f.Height)/2, s, c)
}

// Elide kürzt einen Text mit „…“ auf maximale Breite.
func Elide(f *font.Face, s string, w int) string {
	if f.Width(s) <= w {
		return s
	}
	rs := []rune(s)
	for len(rs) > 0 {
		rs = rs[:len(rs)-1]
		t := string(rs) + "…"
		if f.Width(t) <= w {
			return t
		}
	}
	return ""
}

// ToRGBA wandelt in ein image.RGBA (für PNG-Tests).
func (m *Image) ToRGBA() *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, m.W, m.H))
	for y := 0; y < m.H; y++ {
		for x := 0; x < m.W; x++ {
			r, g, b := Split(m.Pix[y*m.W+x])
			im.SetRGBA(x, y, color.RGBA{uint8(r), uint8(g), uint8(b), 255})
		}
	}
	return im
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
