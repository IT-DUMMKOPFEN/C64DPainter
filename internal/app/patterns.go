package app

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- Square patterns (1.1) -----------------------------------------------------------
//
// Besides the classic 8 × 8 patterns there are square patterns of 16, 32 and
// 64 screen pixels. They are stored in hires pixels; in multicolor every
// picture pixel covers two of them, so a pattern is 8 × 16, 16 × 32 or
// 32 × 64 multicolor pixels and looks square on screen.
//
// Own patterns are 1-bit PNG files (white = paint) in the pattern folder.
// The folder is asked for when the first pattern is saved and kept in the
// settings (Options → Pattern folder …).

type bigPattern struct {
	S    int
	Bits []bool
	Name string
	File string // own pattern: file name in the pattern folder
}

func (p *bigPattern) at(x, y int) bool {
	return p.Bits[((y%p.S+p.S)%p.S)*p.S+(x%p.S+p.S)%p.S]
}

func newBigPattern(s int, name string, f func(x, y, s int) bool) *bigPattern {
	p := &bigPattern{S: s, Bits: make([]bool, s*s), Name: name}
	for y := 0; y < s; y++ {
		for x := 0; x < s; x++ {
			p.Bits[y*s+x] = f(x, y, s)
		}
	}
	return p
}

var bayer8 = [8][8]int{
	{0, 32, 8, 40, 2, 34, 10, 42}, {48, 16, 56, 24, 50, 18, 58, 26},
	{12, 44, 4, 36, 14, 46, 6, 38}, {60, 28, 52, 20, 62, 30, 54, 22},
	{3, 35, 11, 43, 1, 33, 9, 41}, {51, 19, 59, 27, 49, 17, 57, 25},
	{15, 47, 7, 39, 13, 45, 5, 37}, {63, 31, 55, 23, 61, 29, 53, 21},
}

// dith: ordered dither of a level 0..1 at (x, y)
func dith(level float64, x, y int) bool {
	return float64(bayer8[y&7][x&7])+0.5 < level*64
}

func iabs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// builtInBig are the square patterns that come with the program.
var builtInBig = func() []*bigPattern {
	var out []*bigPattern
	add := func(s int, name string, f func(x, y, s int) bool) {
		out = append(out, newBigPattern(s, name, f))
	}
	ring := func(x, y, cx, cy int, r0, r1 float64) bool {
		d := math.Hypot(float64(x-cx)+0.5, float64(y-cy)+0.5)
		return d >= r0 && d < r1
	}
	// 16 × 16
	add(16, "Bricks", func(x, y, s int) bool {
		if y%8 == 7 {
			return true
		}
		off := 0
		if (y/8)%2 == 1 {
			off = 8
		}
		return (x+off)%16 == 15
	})
	add(16, "Herringbone", func(x, y, s int) bool { return ((x+y)/2+(x/4))%4 == 0 })
	add(16, "Scales", func(x, y, s int) bool {
		cy := 8
		if y >= 8 {
			cy = 16
		}
		ox := 0
		if y >= 8 {
			ox = 8
		}
		return ring((x+ox)%16, y, 8, cy, 6.5, 8)
	})
	add(16, "Big waves", func(x, y, s int) bool {
		return iabs(y%8-int(3.5+3.5*math.Sin(float64(x)*math.Pi/8))) < 1
	})
	add(16, "Diamonds", func(x, y, s int) bool { return iabs(x-8)+iabs(y-8) == 7 })
	add(16, "Filled diamonds", func(x, y, s int) bool { return iabs(x-8)+iabs(y-8) < 6 })
	add(16, "Big dots", func(x, y, s int) bool { return ring(x, y, 8, 8, 0, 4.5) })
	add(16, "Rings 16", func(x, y, s int) bool { return ring(x, y, 8, 8, 4.5, 6.5) })
	add(16, "Plus", func(x, y, s int) bool {
		return (x >= 6 && x < 10 && y >= 2 && y < 14) || (y >= 6 && y < 10 && x >= 2 && x < 14)
	})
	add(16, "Hearts", func(x, y, s int) bool {
		fx, fy := (float64(x)-7.5)/6, (7.5-float64(y))/6
		v := fx*fx + fy*fy - 1
		return v*v*v-fx*fx*fy*fy*fy < 0
	})
	add(16, "Stars", func(x, y, s int) bool {
		dx, dy := float64(x)-7.5, float64(y)-7.5
		a := math.Atan2(dy, dx)
		r := math.Hypot(dx, dy)
		return r < 3+3*math.Pow(math.Abs(math.Cos(2.5*a)), 3)
	})
	add(16, "Triangles", func(x, y, s int) bool { return iabs(x-8) <= y/2 && y < 15 })
	add(16, "Checker 8", func(x, y, s int) bool { return (x/8+y/8)%2 == 0 })
	add(16, "Thick diagonal", func(x, y, s int) bool { return (x+y)%16 < 5 })
	add(16, "Grid 16", func(x, y, s int) bool { return x == 0 || y == 0 })
	add(16, "Basket", func(x, y, s int) bool {
		if (x/8+y/8)%2 == 0 {
			return y%4 == 0
		}
		return x%4 == 0
	})
	add(16, "Honeycomb", func(x, y, s int) bool {
		yy := y % 8
		switch {
		case yy == 0:
			return x%16 < 5 || x%16 > 11
		case yy < 4:
			return x%16 == 5+yy || x%16 == 11-yy+5-5
		case yy == 4:
			return x%16 > 7 && x%16 < 13
		}
		return x%16 == 4+8-yy || x%16 == 12+yy-4
	})
	add(16, "Zigzag big", func(x, y, s int) bool { return iabs((x%8)-4)+4 == y%8+0 || iabs((x%8)-4) == (y%8)-4 })
	add(16, "Crosses 16", func(x, y, s int) bool { return x == y || x == 15-y })
	add(16, "Weave 16", func(x, y, s int) bool {
		if (x/4+y/4)%2 == 0 {
			return y%4 != 3
		}
		return x%4 != 3
	})
	// 32 × 32
	add(32, "Big bricks", func(x, y, s int) bool {
		if y%16 == 15 || y%16 == 14 {
			return true
		}
		off := 0
		if (y/16)%2 == 1 {
			off = 16
		}
		return (x+off)%32 >= 30
	})
	add(32, "Big scales", func(x, y, s int) bool {
		cy := 16
		ox := 0
		if y >= 16 {
			cy, ox = 32, 16
		}
		return ring((x+ox)%32, y, 16, cy, 12, 15)
	})
	add(32, "Target", func(x, y, s int) bool {
		d := math.Hypot(float64(x)-15.5, float64(y)-15.5)
		return int(d)%6 < 3 && d < 15
	})
	add(32, "Wood", func(x, y, s int) bool {
		v := float64(y) + 3*math.Sin(float64(x)*math.Pi/16) + 2*math.Sin(float64(x)*math.Pi/5)
		return int(math.Floor(v))%6 == 0
	})
	add(32, "Grass", func(x, y, s int) bool {
		h := 10 + int(8*math.Abs(math.Sin(float64(x)*1.7)))
		return y > 31-h && (x+y/4)%3 == 0
	})
	add(32, "Arches", func(x, y, s int) bool { return ring(x%16, y%16, 8, 16, 6, 8) || y%16 == 15 })
	add(32, "Tiles 32", func(x, y, s int) bool { return x%16 == 0 || y%16 == 0 || (x%16 == 8 && y%16 == 8) })
	add(32, "Dot grid 32", func(x, y, s int) bool { return ring(x, y, 16, 16, 0, 3) })
	add(32, "Diagonal weave", func(x, y, s int) bool { return (x+y)%16 < 3 || (x-y+32)%16 < 3 })
	add(32, "Sine stripes", func(x, y, s int) bool {
		return int(float64(y)+4*math.Sin(float64(x)*2*math.Pi/32))%8 < 3
	})
	add(32, "Clouds", func(x, y, s int) bool {
		v := math.Sin(float64(x)*2*math.Pi/32) + math.Sin(float64(y)*2*math.Pi/32) + 0.6*math.Sin(float64(x+y)*2*math.Pi/16)
		return dith((v+2.6)/5.2, x, y)
	})
	add(32, "Chain", func(x, y, s int) bool { return ring(x, y%16, 10, 8, 4, 6) || ring(x, y%16, 26, 8, 4, 6) })
	// 64 × 64
	add(64, "Fade down", func(x, y, s int) bool { return dith(float64(y)/63, x, y) })
	add(64, "Fade right", func(x, y, s int) bool { return dith(float64(x)/63, x, y) })
	add(64, "Fade from center", func(x, y, s int) bool {
		return dith(math.Hypot(float64(x)-31.5, float64(y)-31.5)/45, x, y)
	})
	add(64, "Spot", func(x, y, s int) bool {
		return dith(1-math.Hypot(float64(x)-31.5, float64(y)-31.5)/45, x, y)
	})
	add(64, "Big circles", func(x, y, s int) bool { return ring(x, y, 32, 32, 24, 30) })
	add(64, "Big checker", func(x, y, s int) bool { return (x/32+y/32)%2 == 0 })
	add(64, "Big waves", func(x, y, s int) bool {
		return int(float64(y)+8*math.Sin(float64(x)*2*math.Pi/64))%16 < 5
	})
	add(64, "Large bricks", func(x, y, s int) bool {
		if y%32 >= 29 {
			return true
		}
		off := 0
		if (y/32)%2 == 1 {
			off = 32
		}
		return (x+off)%64 >= 61
	})
	add(64, "Rays", func(x, y, s int) bool {
		a := math.Atan2(float64(y)-31.5, float64(x)-31.5)
		return int((a+math.Pi)/(2*math.Pi)*24)%2 == 0
	})
	add(64, "Diagonal fade", func(x, y, s int) bool { return dith(float64(x+y)/126, x, y) })
	return out
}()

// user patterns (loaded from the pattern folder)
var userPatterns []*bigPattern

func patternCount() int { return len(patterns) + len(builtInBig) + len(userPatterns) }

func bigPatternAt(i int) *bigPattern {
	i -= len(patterns)
	if i < 0 {
		return nil
	}
	if i < len(builtInBig) {
		return builtInBig[i]
	}
	i -= len(builtInBig)
	if i < len(userPatterns) {
		return userPatterns[i]
	}
	return nil
}

func patternName(i int) string {
	if i >= 0 && i < len(patternNames) {
		return patternNames[i]
	}
	if b := bigPatternAt(i); b != nil {
		n := b.Name
		if b.File != "" {
			n = "My pattern: " + n
		}
		return fmt.Sprintf("%s (%d×%d)", n, b.S, b.S)
	}
	return ""
}

// patternDir is the folder for own patterns ("" = not chosen yet).
func (a *App) patternDir() string { return a.set.PatternDir }

func (a *App) loadUserPatterns() {
	userPatterns = nil
	dir := a.patternDir()
	if dir == "" {
		return
	}
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
		if len(userPatterns) >= 200 {
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
		if p := patternFromImage(img); p != nil {
			p.Name = strings.TrimSuffix(n, filepath.Ext(n))
			p.File = n
			userPatterns = append(userPatterns, p)
		}
	}
}

// patternFromImage reads a pattern PNG (light pixels = paint); other sizes
// are scaled to the nearest of 16, 32 and 64.
func patternFromImage(img image.Image) *bigPattern {
	r := img.Bounds()
	if r.Dx() < 2 || r.Dy() < 2 {
		return nil
	}
	s := 16
	for _, k := range []int{32, 64} {
		if max(r.Dx(), r.Dy()) > s+s/2 {
			s = k
		}
	}
	p := &bigPattern{S: s, Bits: make([]bool, s*s)}
	for y := 0; y < s; y++ {
		for x := 0; x < s; x++ {
			c := color.NRGBAModel.Convert(img.At(r.Min.X+x*r.Dx()/s, r.Min.Y+y*r.Dy()/s)).(color.NRGBA)
			p.Bits[y*s+x] = c.A > 127 && int(c.R)+int(c.G)+int(c.B) > 3*127
		}
	}
	return p
}

func encodePatternPNG(p *bigPattern) ([]byte, error) {
	img := image.NewGray(image.Rect(0, 0, p.S, p.S))
	for i, b := range p.Bits {
		if b {
			img.Pix[i] = 255
		}
	}
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	return buf.Bytes(), err
}

// ensurePatternDir asks for the pattern folder the first time.
func (a *App) ensurePatternDir() bool {
	if a.set.PatternDir != "" {
		return true
	}
	a.choosePatternDir()
	return a.set.PatternDir != ""
}

func (a *App) choosePatternDir() {
	start := a.set.PatternDir
	if start == "" {
		start = filepath.Join(a.workDir(), "patterns")
	}
	if fc, ok := a.host.(FolderChooser); ok {
		if d, ok := fc.ChooseFolder("Choose the folder for your patterns (used by all your pictures)", start); ok && d != "" {
			a.set.PatternDir = d
		}
	} else {
		a.set.PatternDir = start
	}
	if a.set.PatternDir != "" {
		_ = os.MkdirAll(a.set.PatternDir, 0o755)
		a.saveSettings()
		a.loadUserPatterns()
		a.setStatus("Pattern folder: " + a.set.PatternDir)
	}
}

// --- Pattern editor ------------------------------------------------------------------

type patternEditor struct {
	p      *bigPattern
	name   *textField
	file   string // editing an own pattern
	paint  int    // -1 = not painting, 0 = clear, 1 = set
	nameOn bool
}

func (a *App) openPatternEditor(p *bigPattern) {
	a.commitOrCancelForHistory()
	a.menuOpen = -1
	e := &patternEditor{paint: -1}
	if p == nil {
		p = &bigPattern{S: 16, Bits: make([]bool, 256), Name: "pattern"}
	}
	q := &bigPattern{S: p.S, Bits: append([]bool(nil), p.Bits...), Name: p.Name}
	e.p = q
	e.file = p.File
	e.name = newTextField(p.Name, 40)
	a.modal = e
}

// openPatternEditorFor edits the pattern with index i (classic 8 × 8
// patterns are turned into 16 × 16 ones).
func (a *App) openPatternEditorFor(i int) {
	if b := bigPatternAt(i); b != nil {
		a.openPatternEditor(b)
		return
	}
	p := &bigPattern{S: 16, Bits: make([]bool, 256), Name: "pattern"}
	if i > 0 && i < len(patterns) {
		p.Name = strings.ToLower(patternNames[i])
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				p.Bits[y*16+x] = patterns[i][y&7]>>(7-uint((x/2)&7))&1 == 1
			}
		}
	}
	a.openPatternEditor(p)
}

func (e *patternEditor) Title() string          { return "Pattern editor" }
func (e *patternEditor) Size(a *App) (int, int) { return 900, min(640, a.H-30) }
func (e *patternEditor) Char(a *App, r rune) {
	if e.nameOn {
		e.name.Char(r)
	}
}
func (e *patternEditor) Key(a *App, k KeyEvent) {
	if e.nameOn {
		e.name.Key(k)
	}
}

func (e *patternEditor) resize(s int) {
	if s == e.p.S {
		return
	}
	nb := make([]bool, s*s)
	for y := 0; y < s; y++ {
		for x := 0; x < s; x++ {
			nb[y*s+x] = e.p.Bits[(y*e.p.S/s)*e.p.S+x*e.p.S/s]
		}
	}
	e.p.S, e.p.Bits = s, nb
}

func (e *patternEditor) edit(f func(x, y int) bool) {
	s := e.p.S
	nb := make([]bool, s*s)
	for y := 0; y < s; y++ {
		for x := 0; x < s; x++ {
			nb[y*s+x] = f(x, y)
		}
	}
	e.p.Bits = nb
}

func (e *patternEditor) save(a *App, asNew bool) {
	if !a.ensurePatternDir() {
		return
	}
	name := strings.TrimSpace(e.name.String())
	if name == "" {
		name = "pattern"
	}
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
	file := e.file
	if asNew || file == "" || !strings.EqualFold(strings.TrimSuffix(file, filepath.Ext(file)), name) {
		file = name + ".png"
		for k := 2; ; k++ {
			if _, err := os.Stat(filepath.Join(a.patternDir(), file)); err != nil {
				break
			}
			file = fmt.Sprintf("%s %d.png", name, k)
		}
	}
	data, err := encodePatternPNG(e.p)
	if err == nil {
		err = os.WriteFile(filepath.Join(a.patternDir(), file), data, 0o644)
	}
	if err != nil {
		a.host.Info(AppName, "Saving the pattern failed:\n"+err.Error())
		return
	}
	e.file = file
	a.loadUserPatterns()
	for i := 0; i < patternCount(); i++ {
		if b := bigPatternAt(i); b != nil && b.File == file {
			a.pattern = i
		}
	}
	a.setStatus("Pattern saved: " + filepath.Join(a.patternDir(), file))
}

func (e *patternEditor) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	in := &a.in
	s := e.p.S
	// canvas
	cell := max(1, min(512/s, (r.H-10)/s))
	cr := gfx.Rect{X: r.X, Y: r.Y, W: s * cell, H: s * cell}
	on, off := a.pal.Colors[a.fg], a.pal.Colors[a.pic.BG]
	for y := 0; y < s; y++ {
		for x := 0; x < s; x++ {
			c := off
			if e.p.Bits[y*s+x] {
				c = on
			}
			m.Fill(gfx.Rect{X: cr.X + x*cell, Y: cr.Y + y*cell, W: cell, H: cell}, c)
		}
	}
	if cell >= 6 {
		gc := gfx.Mix(off, 0x808080, 90)
		for k := 0; k <= s; k++ {
			m.Fill(gfx.Rect{X: cr.X + k*cell, Y: cr.Y, W: 1, H: cr.H}, gc)
			m.Fill(gfx.Rect{X: cr.X, Y: cr.Y + k*cell, W: cr.W, H: 1}, gc)
		}
	}
	m.Frame(cr.Inset(-1), th.Border)
	// painting: left sets, right clears; a click on a set pixel clears
	if cr.Contains(in.MX, in.MY) && !a.modalDragging {
		x, y := (in.MX-cr.X)/cell, (in.MY-cr.Y)/cell
		if in.Pressed[0] {
			e.paint = 1
			if e.p.Bits[y*s+x] {
				e.paint = 0
			}
		} else if in.Pressed[1] {
			e.paint = 0
		}
		if e.paint >= 0 && (in.Down[0] || in.Down[1]) {
			e.p.Bits[y*s+x] = e.paint == 1
		}
	}
	if !in.Down[0] && !in.Down[1] {
		e.paint = -1
	}
	// right column
	x := cr.Right() + 24
	w := r.Right() - x
	y := r.Y
	m.Text(font.UIB, x, y+4, "Size (screen pixels)", th.Text)
	y += 24
	bw := (w - 8) / 3
	for i, k := range []int{16, 32, 64} {
		lbl := fmt.Sprintf("%d×%d", k, k)
		if u.ButtonEx("pe_s"+itoa(k), gfx.Rect{X: x + i*(bw+4), Y: y, W: bw, H: 24}, lbl, s == k, true) {
			e.resize(k)
		}
	}
	y += 28
	m.Text(font.UI, x, y+2, fmt.Sprintf("Multicolor: %d×%d pixels · hires: %d×%d", s/2, s, s, s), th.TextDim)
	y += 24
	ops := []struct {
		l string
		f func(x, y int) bool
	}{
		{"Clear", func(x, y int) bool { return false }},
		{"Invert", func(x, y int) bool { return !e.p.at(x, y) }},
		{"Flip ↔", func(x, y int) bool { return e.p.at(s-1-x, y) }},
		{"Flip ↕", func(x, y int) bool { return e.p.at(x, s-1-y) }},
		{"Shift ←", func(x, y int) bool { return e.p.at(x+1, y) }},
		{"Shift →", func(x, y int) bool { return e.p.at(x-1, y) }},
		{"Shift ↑", func(x, y int) bool { return e.p.at(x, y+1) }},
		{"Shift ↓", func(x, y int) bool { return e.p.at(x, y-1) }},
		{"Rotate ↻", func(x, y int) bool { return e.p.at(y, s-1-x) }},
	}
	for i, op := range ops {
		if u.Button("pe_op"+itoa(i), gfx.Rect{X: x + (i%3)*(bw+4), Y: y + (i/3)*28, W: bw, H: 24}, op.l) {
			e.edit(op.f)
		}
	}
	y += 3*28 + 8
	// preview
	m.Text(font.UIB, x, y, "Preview", th.Text)
	y += 20
	pr := gfx.Rect{X: x, Y: y, W: w, H: 120}
	for py := 0; py < pr.H; py++ {
		for px := 0; px < pr.W; px++ {
			// multicolor look: every picture pixel is two screen pixels wide
			c := off
			if e.p.at((px/2)*2, py) {
				c = on
			}
			m.Set(pr.X+px, pr.Y+py, c)
		}
	}
	m.Frame(pr.Inset(-1), th.Border)
	y = pr.Bottom() + 12
	m.Text(font.UI, x, y+6, "Name", th.Text)
	e.name.focus = e.nameOn
	nr := gfx.Rect{X: x + 50, Y: y, W: w - 50, H: 26}
	if in.Pressed[0] && nr.Contains(in.MX, in.MY) {
		e.nameOn = true
	} else if in.Pressed[0] && !nr.Contains(in.MX, in.MY) {
		e.nameOn = false
	}
	e.name.Draw(a, "pe_name", nr)
	y += 34
	dir := a.patternDir()
	if dir == "" {
		dir = "(asked when you save)"
	}
	m.Text(font.UI, x, y, gfx.Elide(font.UI, "Folder: "+dir, w), th.TextDim)
	by := r.Bottom() - 30
	bw2 := (w - 8) / 3
	if u.Button("pe_cancel", gfx.Rect{X: x, Y: by, W: bw2, H: 30}, "Close") {
		a.closeModal(true)
	}
	if u.ButtonEx("pe_new", gfx.Rect{X: x + bw2 + 4, Y: by, W: bw2, H: 30}, "Save as new", false, true) {
		e.save(a, true)
	}
	lbl := "Save"
	if e.file != "" {
		lbl = "Save (replace)"
	}
	if u.ButtonEx("pe_save", gfx.Rect{X: x + 2*(bw2+4), Y: by, W: bw2, H: 30}, lbl, true, true) {
		e.save(a, false)
		a.closeModal(false)
	}
}

// selectionAsPattern opens the pattern editor with the selected area
// (picture pixels that are not the background color become set pixels).
func (a *App) selectionAsPattern() {
	b := a.selCurrentBrush()
	if b == nil {
		a.setStatus("Select an area first")
		return
	}
	w, h := b.W*asp, b.H
	s := 16
	for _, k := range []int{32, 64} {
		if max(w, h) > s {
			s = k
		}
	}
	p := &bigPattern{S: s, Bits: make([]bool, s*s), Name: "from selection"}
	for y := 0; y < min(h, s); y++ {
		for x := 0; x < min(w, s); x++ {
			v := b.Pix[y*b.W+x/asp]
			p.Bits[y*s+x] = v >= 0 && uint8(v) != a.pic.BG
		}
	}
	a.openPatternEditor(p)
}
