package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- Font editor ---------------------------------------------------------------------
//
// A full-window editor for C64 character sets (256 hires characters of 8 × 8
// pixels), with tiles of 2 × 1, 1 × 2 and 2 × 2 characters, the C64 ROM as
// start and reference, import from a 320 × 200 picture, project files and
// ACME export. Ideas from KANJI (AMID64, MIT) and the C64 BDF font editor
// (garymeg, MIT).

const (
	fntPencil = iota
	fntLine
	fntRect
	fntFill
	fntToolCount
)

var fntToolNames = [fntToolCount]string{"Pencil", "Line", "Rectangle", "Fill"}
var fntToolKeys = [fntToolCount]string{"D", "L", "R", "F"}

type fntEditor struct {
	open       bool
	f          *c64.Font
	undo, redo []c64.Font
	cur        int
	tool       int
	grid       bool
	compare    bool
	clip       []bool
	clipW      int
	clipH      int
	text       string
	textFocus  bool
	path       string
	dirty      bool
	// stroke
	drawing bool
	on      bool
	sx, sy  int
	lx, ly  int
	before  c64.Font
	canvas  gfx.Rect
	z       int
	hx, hy  int
	textBox gfx.Rect
}

func (a *App) openFontEditor() {
	a.commitOrCancelForHistory()
	a.menuOpen = -1
	if a.fnt == nil {
		a.fnt = &fntEditor{f: c64.NewFont(0), cur: 1, grid: true, text: "HELLO C64 DPAINTER 1234"}
	}
	a.fnt.open = true
	a.setStatus("Font editor")
}

func (a *App) closeFontEditor() {
	a.fnt.open = false
	a.fnt.textFocus = false
	a.modal = nil
	a.setStatus("Back to the multicolor editor")
}

func (e *fntEditor) push() {
	e.undo = append(e.undo, *e.f)
	if len(e.undo) > 300 {
		e.undo = e.undo[1:]
	}
	e.redo = nil
	e.dirty = true
}

func (e *fntEditor) doUndo() {
	if n := len(e.undo); n > 0 {
		e.redo = append(e.redo, *e.f)
		*e.f = e.undo[n-1]
		e.undo = e.undo[:n-1]
		e.dirty = true
	}
}

func (e *fntEditor) doRedo() {
	if n := len(e.redo); n > 0 {
		e.undo = append(e.undo, *e.f)
		*e.f = e.redo[n-1]
		e.redo = e.redo[:n-1]
		e.dirty = true
	}
}

// tileSize returns the tile size in pixels.
func (e *fntEditor) tileSize() (int, int) {
	tw, th := e.f.TileSize()
	return tw * 8, th * 8
}

// op applies a pixel operation to the current tile with undo.
func (e *fntEditor) op(f func(get func(x, y int) bool, w, h, x, y int) bool) {
	e.push()
	e.f.TileOp(e.cur, f)
}

func (e *fntEditor) flipH() {
	e.op(func(g func(x, y int) bool, w, h, x, y int) bool { return g(w-1-x, y) })
}
func (e *fntEditor) flipV() {
	e.op(func(g func(x, y int) bool, w, h, x, y int) bool { return g(x, h-1-y) })
}
func (e *fntEditor) rotate() {
	e.op(func(g func(x, y int) bool, w, h, x, y int) bool { return g(y, w-1-x) })
}
func (e *fntEditor) invert() {
	e.op(func(g func(x, y int) bool, w, h, x, y int) bool { return !g(x, y) })
}
func (e *fntEditor) clear() { e.op(func(g func(x, y int) bool, w, h, x, y int) bool { return false }) }
func (e *fntEditor) shift(dx, dy int) {
	e.op(func(g func(x, y int) bool, w, h, x, y int) bool { return g((x-dx+w)%w, (y-dy+h)%h) })
}

// restore copies the tile back from the reference ROM.
func (e *fntEditor) restore() {
	e.push()
	rom := e.f.RefROM()
	tw, th := e.f.TileSize()
	for ty := 0; ty < th; ty++ {
		for tx := 0; tx < tw; tx++ {
			c := e.f.TileChar(e.cur, tx, ty)
			copy(e.f.Data[c*8:c*8+8], rom[c*8:c*8+8])
		}
	}
}

func (e *fntEditor) copyTile() {
	w, h := e.tileSize()
	e.clip = make([]bool, w*h)
	e.clipW, e.clipH = w, h
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			e.clip[y*w+x] = e.f.TilePixel(e.cur, x, y)
		}
	}
}

func (e *fntEditor) pasteTile() {
	if e.clip == nil {
		return
	}
	e.op(func(g func(x, y int) bool, w, h, x, y int) bool {
		if x < e.clipW && y < e.clipH {
			return e.clip[y*e.clipW+x]
		}
		return g(x, y)
	})
}

// moveCur moves the selection in the character grid.
func (e *fntEditor) moveCur(d int) { e.cur = (e.cur + d + 256) & 255 }

// --- input -----------------------------------------------------------------------

func (a *App) fntKeys() {
	e := a.fnt
	for _, k := range a.in.Keys {
		m := k.Mods
		if e.textFocus {
			switch k.Key {
			case KeyBackspace:
				if rs := []rune(e.text); len(rs) > 0 {
					e.text = string(rs[:len(rs)-1])
				}
			case KeyEsc, KeyEnter:
				e.textFocus = false
			}
			continue
		}
		switch {
		case m.Ctrl && k.Key == 'Z' && !m.Shift:
			e.doUndo()
		case m.Ctrl && (k.Key == 'Y' || (k.Key == 'Z' && m.Shift)):
			e.doRedo()
		case m.Ctrl && k.Key == 'S':
			a.later(a.fntSave)
		case m.Ctrl && k.Key == 'O':
			a.later(a.fntOpen)
		case m.Ctrl && k.Key == 'C':
			e.copyTile()
		case m.Ctrl && k.Key == 'V':
			e.pasteTile()
		case m.Alt && k.Key == KeyLeft:
			e.shift(-1, 0)
		case m.Alt && k.Key == KeyRight:
			e.shift(1, 0)
		case m.Alt && k.Key == KeyUp:
			e.shift(0, -1)
		case m.Alt && k.Key == KeyDown:
			e.shift(0, 1)
		case k.Key == KeyLeft:
			e.moveCur(-1)
		case k.Key == KeyRight:
			e.moveCur(1)
		case k.Key == KeyUp:
			e.moveCur(-16)
		case k.Key == KeyDown:
			e.moveCur(16)
		case k.Key == KeyDelete:
			e.clear()
		case k.Key == KeyF1:
			a.openHelp("fonts")
		}
	}
	for _, c := range a.in.Chars {
		if e.textFocus {
			if len([]rune(e.text)) < 120 {
				e.text += string(c)
			}
			continue
		}
		switch c {
		case 'd', 'D':
			e.tool = fntPencil
		case 'l', 'L':
			e.tool = fntLine
		case 'r', 'R':
			e.tool = fntRect
		case 'f', 'F':
			e.tool = fntFill
		case 'g', 'G':
			e.grid = !e.grid
		case 'b', 'B':
			e.compare = !e.compare
		case 'x', 'X':
			e.flipH()
		case 'y', 'Y':
			e.flipV()
		case 't', 'T':
			if w, h := e.tileSize(); w == h {
				e.rotate()
			}
		case 'i', 'I':
			e.invert()
		case 'u':
			e.doUndo()
		case 'U':
			e.doRedo()
		}
	}
}

func (e *fntEditor) pixelAt(x, y int) (int, int, bool) {
	w, h := e.tileSize()
	px := (x - e.canvas.X) / e.z
	py := (y - e.canvas.Y) / e.z
	in := x >= e.canvas.X && y >= e.canvas.Y && px < w && py < h
	return clampI(px, 0, w-1), clampI(py, 0, h-1), in
}

func (e *fntEditor) shape(x0, y0, x1, y1 int, on bool) {
	switch e.tool {
	case fntLine:
		forLine(x0, y0, x1, y1, func(x, y int) { e.f.SetTilePixel(e.cur, x, y, on) })
	case fntRect:
		ax, bx := min(x0, x1), max(x0, x1)
		ay, by := min(y0, y1), max(y0, y1)
		for y := ay; y <= by; y++ {
			for x := ax; x <= bx; x++ {
				if x == ax || x == bx || y == ay || y == by {
					e.f.SetTilePixel(e.cur, x, y, on)
				}
			}
		}
	}
}

func (e *fntEditor) fill(x, y int, on bool) {
	w, h := e.tileSize()
	if e.f.TilePixel(e.cur, x, y) == on {
		return
	}
	stack := [][2]int{{x, y}}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if p[0] < 0 || p[1] < 0 || p[0] >= w || p[1] >= h || e.f.TilePixel(e.cur, p[0], p[1]) == on {
			continue
		}
		e.f.SetTilePixel(e.cur, p[0], p[1], on)
		stack = append(stack, [2]int{p[0] + 1, p[1]}, [2]int{p[0] - 1, p[1]}, [2]int{p[0], p[1] + 1}, [2]int{p[0], p[1] - 1})
	}
}

func (a *App) fntCanvasInput() {
	e := a.fnt
	in := &a.in
	x, y, inside := e.pixelAt(in.MX, in.MY)
	e.hx, e.hy = -1, -1
	if inside {
		e.hx, e.hy = x, y
	}
	if a.ui.blocked {
		return
	}
	if (in.Pressed[0] || in.Pressed[1]) && !e.textBox.Contains(in.MX, in.MY) {
		e.textFocus = false
	}
	pressed := inside && (in.Pressed[0] || in.Pressed[1])
	if pressed {
		e.push()
		e.drawing = true
		e.on = in.Pressed[0]
		// pencil on a set pixel with the left button clears (as in most font editors)
		if e.tool == fntPencil && in.Pressed[0] && e.f.TilePixel(e.cur, x, y) {
			e.on = false
		}
		e.sx, e.sy, e.lx, e.ly = x, y, x, y
		e.before = *e.f
		switch e.tool {
		case fntPencil:
			e.f.SetTilePixel(e.cur, x, y, e.on)
		case fntFill:
			e.fill(x, y, e.on)
			e.drawing = false
		default:
			e.shape(x, y, x, y, e.on)
		}
		return
	}
	if !e.drawing {
		return
	}
	if !in.Down[0] && !in.Down[1] {
		e.drawing = false
		return
	}
	switch e.tool {
	case fntPencil:
		if in.Mods.Shift {
			*e.f = e.before
			e.tool = fntLine
			e.shape(e.sx, e.sy, x, y, e.on)
			e.tool = fntPencil
			return
		}
		if x != e.lx || y != e.ly {
			forLine(e.lx, e.ly, x, y, func(px, py int) { e.f.SetTilePixel(e.cur, px, py, e.on) })
			e.lx, e.ly = x, y
		}
	default:
		*e.f = e.before
		e.shape(e.sx, e.sy, x, y, e.on)
	}
}

// --- drawing -----------------------------------------------------------------------

func (a *App) fntFrame() {
	e := a.fnt
	a.toolFrameStart(a.fntKeys)
	if !e.open {
		return
	}
	a.ui.blocked = a.modal != nil
	m := a.img
	m.Fill(gfx.Rect{X: 0, Y: 0, W: a.W, H: a.H}, a.th.Bg)
	area := gfx.Rect{X: toolLeftW + 10, Y: toolTopH + 10, W: a.W - toolLeftW - toolRightW - 20, H: a.H - toolTopH - toolStatusH - 20}
	prevH := 150
	ca := gfx.Rect{X: area.X, Y: area.Y, W: area.W, H: area.H - prevH - 10}
	w, h := e.tileSize()
	e.z = max(4, min(ca.W/w, ca.H/h))
	e.canvas = gfx.Rect{X: ca.X + (ca.W-w*e.z)/2, Y: ca.Y + (ca.H-h*e.z)/2, W: w * e.z, H: h * e.z}
	a.fntCanvasInput()
	a.fntDrawCanvas()
	a.fntDrawTest(gfx.Rect{X: area.X, Y: area.Bottom() - prevH, W: area.W, H: prevH})
	a.toolTopBar("Font editor", []toolButton{
		{"fe_new", "New …", "Empty character set or the C64 font", func() { a.modal = &fntNewDialog{} }},
		{"fe_imp", "Import …", "Take characters from the picture, a 320 × 200 image or the C64 font", a.fntOpenImport},
		{"fe_open", "Open …", "Open a font project (.c64font) or a raw character set (.bin, .64c) – Ctrl+O", a.fntOpen},
		{"fe_save", "Save …", "Save the font project (.c64font) – Ctrl+S", a.fntSave},
		{},
		{"fe_asm", "Export ASM …", "Source code for the ACME assembler", func() { a.fntExport("asm") }},
		{"fe_bin", "Export BIN …", "Raw character set, 2048 bytes", func() { a.fntExport("bin") }},
		{"fe_png", "Export PNG …", "All 256 characters as a picture", func() { a.fntExport("png") }},
		{},
		{"fe_undo", "Undo", "Ctrl+Z", e.doUndo},
		{"fe_redo", "Redo", "Ctrl+Y", e.doRedo},
		{"fe_help", "Help", "F1", func() { a.openHelp("fonts") }},
	}, "fe_close", "Close font editor", a.closeFontEditor)
	a.fntDrawLeft(gfx.Rect{X: 0, Y: toolTopH, W: toolLeftW, H: a.H - toolTopH - toolStatusH})
	a.fntDrawRight(gfx.Rect{X: a.W - toolRightW, Y: toolTopH, W: toolRightW, H: a.H - toolTopH - toolStatusH})
	status := "Character " + charLabel(e.cur, e.f.Ref == 1)
	if tw, th := e.f.TileSize(); tw*th > 1 {
		var cs []string
		for ty := 0; ty < th; ty++ {
			for tx := 0; tx < tw; tx++ {
				cs = append(cs, fmt.Sprintf("$%02X", e.f.TileChar(e.cur, tx, ty)))
			}
		}
		status = "Tile " + strings.Join(cs, " ")
	}
	if e.hx >= 0 {
		status += fmt.Sprintf("  ·  x %d, y %d", e.hx, e.hy)
	}
	if e.compare {
		status += "  ·  comparing with the C64 ROM (B)"
	}
	a.toolStatus(status, "Left: set (on a set pixel: clear) · right: clear · Shift: straight line · arrows: character · F1: help")
	a.toolFrameEnd()
}

// charLabel describes a screen code.
func charLabel(c int, lower bool) string {
	r := ""
	b := c & 127
	switch {
	case b == 0:
		r = "@"
	case b >= 1 && b <= 26 && lower:
		r = string(rune('a' + b - 1))
	case b >= 1 && b <= 26:
		r = string(rune('A' + b - 1))
	case b >= 33 && b <= 63:
		r = string(rune(b))
	case b >= 65 && b <= 90 && lower:
		r = string(rune('A' + b - 65))
	}
	s := fmt.Sprintf("$%02X (%d)", c, c)
	if r != "" {
		s += "  " + r
	}
	if c >= 128 {
		s += " inverted"
	}
	return s
}

// fntGlyph draws a character of the font (or of rom when not nil).
func (a *App) fntGlyph(data []byte, c, x, y, s int, fg, bg uint32) {
	m := a.img
	if bg != 0xFFFFFFFF {
		m.Fill(gfx.Rect{X: x, Y: y, W: 8 * s, H: 8 * s}, bg)
	}
	for yy := 0; yy < 8; yy++ {
		b := data[(c&255)*8+yy]
		for xx := 0; xx < 8; xx++ {
			if b&(0x80>>xx) != 0 {
				m.Fill(gfx.Rect{X: x + xx*s, Y: y + yy*s, W: s, H: s}, fg)
			}
		}
	}
}

func (a *App) fntDrawCanvas() {
	e := a.fnt
	m := a.img
	f := e.f
	cr := e.canvas
	z := e.z
	pal := a.pal.Colors
	w, h := e.tileSize()
	m.FillA(gfx.Rect{X: cr.Right(), Y: cr.Y + 6, W: 6, H: cr.H}, a.th.Shadow, 80)
	m.FillA(gfx.Rect{X: cr.X + 6, Y: cr.Bottom(), W: cr.W, H: 6}, a.th.Shadow, 80)
	m.Fill(cr, pal[f.BG])
	rom := f.RefROM()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			on := f.TilePixel(e.cur, x, y)
			r := gfx.Rect{X: cr.X + x*z, Y: cr.Y + y*z, W: z, H: z}
			if e.compare {
				c := f.TileChar(e.cur, x/8, y/8)
				orig := rom[c*8+y%8]&(0x80>>(x%8)) != 0
				switch {
				case on && orig:
					m.Fill(r, pal[f.FG])
				case on:
					m.Fill(r, 0x40C040) // new pixel
				case orig:
					m.Fill(r, 0xD04040) // removed pixel
				}
				continue
			}
			if on {
				m.Fill(r, pal[f.FG])
			}
		}
	}
	if e.grid {
		for x := 1; x < w; x++ {
			al := 60
			if x%8 == 0 {
				al = 200
			}
			for y := cr.Y; y < cr.Bottom(); y++ {
				m.BlendPix(cr.X+x*z, y, 0x808080, al)
			}
		}
		for y := 1; y < h; y++ {
			al := 60
			if y%8 == 0 {
				al = 200
			}
			for x := cr.X; x < cr.Right(); x++ {
				m.BlendPix(x, cr.Y+y*z, 0x808080, al)
			}
		}
	}
	m.Frame(cr.Inset(-1), a.th.Border)
	if e.hx >= 0 && a.modal == nil {
		dashedFrame(m, gfx.Rect{X: cr.X + e.hx*z, Y: cr.Y + e.hy*z, W: z, H: z}, 0xFFFFFF, 0, 0)
	}
}

// fntDrawTest draws the test text field and the text in the font.
func (a *App) fntDrawTest(r gfx.Rect) {
	e := a.fnt
	u := &a.ui
	m := a.img
	th := a.th
	f := e.f
	pal := a.pal.Colors
	m.Fill(r, th.Panel)
	m.Frame(r, th.Border)
	x := r.X + 10
	y := r.Y + 8
	m.Text(font.UIB, x, y+5, "Test text", th.Text)
	tf := gfx.Rect{X: x + 80, Y: y, W: r.W - 100, H: 26}
	e.textBox = tf
	m.Fill(tf, th.Popup)
	if e.textFocus {
		m.Frame(tf, th.Accent)
	} else {
		m.Frame(tf, th.BtnBorder)
	}
	tw := m.Text(font.UI, tf.X+6, tf.Y+(tf.H-font.UI.Height)/2, gfx.Elide(font.UI, e.text, tf.W-14), th.Text)
	if e.textFocus && (a.now/500)%2 == 0 {
		m.VLine(tf.X+7+min(tw, tf.W-14), tf.Y+5, tf.Bottom()-6, th.Text)
	}
	if e.textFocus {
		a.needAnim = true
	}
	if c, _ := u.press("fe_tf", tf, 0); c {
		e.textFocus = true
	}
	u.Tip("fe_tf", tf, "Click and type – Enter or Esc ends")
	y += 34
	box := gfx.Rect{X: x, Y: y, W: r.W - 20, H: r.Bottom() - y - 8}
	m.Fill(box, pal[f.BG])
	tw2, th2 := f.TileSize()
	cw, ch := tw2*8, th2*8
	s := max(1, min(3, box.H/(ch+2)/2))
	cx, cy := box.X+4, box.Y+4
	old := m.SetClip(box.Intersect(m.Clip))
	for _, rc := range e.text {
		code := int(c64.ScreenCode(rc, f.Ref == 1))
		if cx+cw*s > box.Right()-4 {
			cx = box.X + 4
			cy += ch*s + 2
		}
		for ty := 0; ty < th2; ty++ {
			for tx := 0; tx < tw2; tx++ {
				a.fntGlyph(f.Data[:], f.TileChar(code, tx, ty), cx+tx*8*s, cy+ty*8*s, s, pal[f.FG], 0xFFFFFFFF)
			}
		}
		cx += cw * s
	}
	m.SetClip(old)
	m.Frame(box.Inset(-1), th.Border)
}

func (a *App) fntDrawLeft(r gfx.Rect) {
	e := a.fnt
	u := &a.ui
	m := a.img
	th := a.th
	f := e.f
	a.toolPanel(r, true)
	x, w := r.X+10, r.W-20
	y := r.Y + 8
	m.Text(font.UIB, x, y, "Tools", th.Text)
	y += 22
	for i := 0; i < fntToolCount; i++ {
		if u.ButtonEx("fe_t"+itoa(i), gfx.Rect{X: x, Y: y, W: w, H: 24}, fntToolKeys[i]+"  "+fntToolNames[i], e.tool == i, true) {
			e.tool = i
		}
		y += 27
	}
	y += 2
	e.grid = u.Checkbox("fe_grid", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Grid (G)", e.grid)
	y += 24
	e.compare = u.Checkbox("fe_cmp", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Compare with ROM (B)", e.compare)
	y += 24
	if e.compare {
		m.Fill(gfx.Rect{X: x + 20, Y: y + 3, W: 10, H: 10}, 0x40C040)
		m.Text(font.UI, x+34, y, "new", th.TextDim)
		m.Fill(gfx.Rect{X: x + 74, Y: y + 3, W: 10, H: 10}, 0xD04040)
		m.Text(font.UI, x+88, y, "removed", th.TextDim)
		y += 20
	}
	y += 6
	m.Text(font.UIB, x, y, "Tile size", th.Text)
	y += 22
	qw := (w - 12) / 4
	for i, n := range c64.TileNames {
		if u.ButtonEx("fe_ts"+itoa(i), gfx.Rect{X: x + i*(qw+4), Y: y, W: qw, H: 22}, n, f.Tile == i, true) {
			f.Tile = i
		}
	}
	y += 26
	bw := (w - 4) / 2
	if f.Tile > 0 {
		for i, n := range []string{"Neighbors", "Quarters"} {
			br := gfx.Rect{X: x + i*(bw+4), Y: y, W: bw, H: 22}
			if u.ButtonEx("fe_tl"+itoa(i), br, n, f.TileLayout == i, true) {
				f.TileLayout = i
			}
			u.Tip("fe_tl"+itoa(i), br, []string{"Next character to the right (+1) and below (+16) in the grid", "Right +64, below +128 (classic 2×2 fonts)"}[i])
		}
		y += 26
	}
	y += 6
	m.Text(font.UIB, x, y, "Edit", th.Text)
	y += 22
	tw, thh := e.tileSize()
	ops := []struct {
		id, label, tip string
		f              func()
		ok             bool
	}{
		{"fe_fh", "Flip ↔", "X", e.flipH, true},
		{"fe_fv", "Flip ↕", "Y", e.flipV, true},
		{"fe_rot", "Rotate ↻", "T (square tiles)", e.rotate, tw == thh},
		{"fe_inv", "Invert", "I", e.invert, true},
		{"fe_sl", "Shift ←", "Alt+←", func() { e.shift(-1, 0) }, true},
		{"fe_sr", "Shift →", "Alt+→", func() { e.shift(1, 0) }, true},
		{"fe_su", "Shift ↑", "Alt+↑", func() { e.shift(0, -1) }, true},
		{"fe_sd", "Shift ↓", "Alt+↓", func() { e.shift(0, 1) }, true},
		{"fe_cp", "Copy", "Ctrl+C", e.copyTile, true},
		{"fe_pa", "Paste", "Ctrl+V", e.pasteTile, e.clip != nil},
		{"fe_clr", "Clear", "Del", e.clear, true},
		{"fe_org", "Original", "Back to the character of the C64 ROM", e.restore, true},
	}
	for i, o := range ops {
		br := gfx.Rect{X: x + (i%2)*(bw+4), Y: y + (i/2)*26, W: bw, H: 22}
		if u.ButtonEx(o.id, br, o.label, false, o.ok) {
			o.f()
		}
		u.Tip(o.id, br, o.tip)
	}
	y += (len(ops)+1)/2*26 + 4
	br := gfx.Rect{X: x, Y: y, W: w, H: 24}
	if u.Button("fe_mkinv", br, "Inverse half (128–255)") {
		e.push()
		f.MakeInverse()
	}
	u.Tip("fe_mkinv", br, "Characters 128–255 become the inverted characters 0–127")
	y += 32
	m.Text(font.UIB, x, y, "Reference ROM", th.Text)
	y += 22
	for i, n := range []string{"Upper case", "Lower case"} {
		if u.ButtonEx("fe_ref"+itoa(i), gfx.Rect{X: x + i*(bw+4), Y: y, W: bw, H: 22}, n, f.Ref == i, true) {
			f.Ref = i
		}
	}
	y += 30
	if e.path != "" {
		for _, ln := range wrapText(font.UI, filepath.Base(e.path), w) {
			m.Text(font.UI, x, y, ln, th.TextDim)
			y += font.UI.Height + 2
		}
	}
}

func (a *App) fntDrawRight(r gfx.Rect) {
	e := a.fnt
	u := &a.ui
	m := a.img
	th := a.th
	f := e.f
	pal := a.pal.Colors
	a.toolPanel(r, false)
	x, w := r.X+12, r.W-24
	y := r.Y + 8
	m.Text(font.UIB, x, y, "Characters", th.Text)
	m.TextR(font.UI, gfx.Rect{X: x, Y: y, W: w, H: 16}, "arrow keys: choose", th.TextDim)
	y += 22
	cell := min(18, w/16)
	gr := gfx.Rect{X: x, Y: y, W: 16 * cell, H: 16 * cell}
	m.Fill(gr, pal[f.BG])
	tw, thh := f.TileSize()
	inTile := map[int]bool{}
	for ty := 0; ty < thh; ty++ {
		for tx := 0; tx < tw; tx++ {
			inTile[f.TileChar(e.cur, tx, ty)] = true
		}
	}
	for c := 0; c < 256; c++ {
		cx, cy := gr.X+(c%16)*cell, gr.Y+(c/16)*cell
		a.fntGlyph(f.Data[:], c, cx+1, cy+1, 2, pal[f.FG], 0xFFFFFFFF)
		br := gfx.Rect{X: cx, Y: cy, W: cell, H: cell}
		if inTile[c] {
			m.Frame(br, 0xFFFFFF)
			if c == e.cur {
				m.Frame(br.Inset(-1), 0x000000)
			}
		}
		if cl, _ := u.press(fmt.Sprintf("fe_c%d", c), br, 0); cl {
			e.cur = c
		}
	}
	m.Frame(gr.Inset(-1), th.Border)
	y += gr.H + 8
	m.Text(font.UI, x, y, charLabel(e.cur, f.Ref == 1), th.Text)
	y += 24
	m.Text(font.UIB, x, y, "Character color", th.Text)
	y += 20
	y += a.toolPalette("fe_fg", x, y, w, int(f.FG), func(c int) { f.FG = uint8(c) }) + 6
	m.Text(font.UIB, x, y, "Background", th.Text)
	y += 20
	a.toolPalette("fe_bg", x, y, w, int(f.BG), func(c int) { f.BG = uint8(c) })
}

// --- new ---------------------------------------------------------------------------

type fntNewDialog struct{}

func (d *fntNewDialog) Title() string          { return "New character set" }
func (d *fntNewDialog) Size(a *App) (int, int) { return 420, 200 }
func (d *fntNewDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	m.Text(font.UI, r.X, r.Y, "Start with:", a.th.Text)
	for i, n := range []string{"The C64 font, upper case / graphics", "The C64 font, lower / upper case", "An empty character set"} {
		if u.Button("fn_n"+itoa(i), gfx.Rect{X: r.X, Y: r.Y + 24 + i*32, W: r.W, H: 28}, n) {
			e := a.fnt
			e.push()
			rom := []int{0, 1, -1}[i]
			nf := c64.NewFont(rom)
			nf.FG, nf.BG, nf.Tile, nf.TileLayout = e.f.FG, e.f.BG, e.f.Tile, e.f.TileLayout
			*e.f = *nf
			e.path = ""
			a.closeModal(false)
			return
		}
	}
	if u.Button("fn_cancel", gfx.Rect{X: r.Right() - 110, Y: r.Bottom() - 28, W: 110, H: 28}, "Cancel") {
		a.closeModal(true)
	}
}

// --- files -------------------------------------------------------------------------

var fntFilters = []Filter{{Name: "C64 DPainter font (*.c64font)", Pattern: "*.c64font"}}

func (a *App) fntName() string {
	if a.fnt.path != "" {
		return strings.TrimSuffix(filepath.Base(a.fnt.path), filepath.Ext(a.fnt.path))
	}
	return "font"
}

func (a *App) fntOpen() {
	p, ok := a.host.OpenFile("Open character set", []Filter{
		{Name: "Character sets (*.c64font, *.bin, *.64c)", Pattern: "*.c64font;*.bin;*.64c;*.chr"}}, a.workDir())
	if !ok {
		return
	}
	data, err := a.host.ReadFile(p)
	if err != nil {
		a.host.Info(AppName, "Cannot open the file:\n"+err.Error())
		return
	}
	a.fntLoad(p, data)
}

// fntLoad reads a font project or a raw character set.
func (a *App) fntLoad(p string, data []byte) {
	project := strings.EqualFold(filepath.Ext(p), ".c64font")
	var f *c64.Font
	var err error
	if project {
		f, err = c64.LoadFont(data)
	} else {
		f, err = c64.LoadRawFont(data)
		if err == nil {
			f.FG, f.BG = a.fnt.f.FG, a.fnt.f.BG
		}
	}
	if err != nil {
		a.host.Info(AppName, "Cannot open the file:\n"+err.Error())
		return
	}
	e := a.fnt
	e.push()
	*e.f = *f
	e.dirty = !project
	e.path = ""
	if project {
		e.path = p
		a.addRecent(p)
	}
}

func (a *App) fntSave() {
	p, ok := a.host.SaveFile("Save character set", fntFilters, a.workDir(), a.fntName()+".c64font", "c64font")
	if !ok {
		return
	}
	p = FixDoubleExt(p)
	data, err := a.fnt.f.Marshal()
	if err == nil {
		err = a.host.WriteFile(p, data)
	}
	if err != nil {
		a.host.Info(AppName, "Saving failed:\n"+err.Error())
		return
	}
	a.fnt.path = p
	a.fnt.dirty = false
	a.setStatus("Saved: " + filepath.Base(p))
}

// fontSheet renders all 256 characters (16 × 16) as a picture.
func (a *App) fontSheet() *gfx.Image {
	f := a.fnt.f
	img := gfx.New(128, 128)
	img.Fill(gfx.Rect{X: 0, Y: 0, W: 128, H: 128}, a.pal.Colors[f.BG])
	for c := 0; c < 256; c++ {
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				if f.Pixel(c, x, y) {
					img.Set((c%16)*8+x, (c/16)*8+y, a.pal.Colors[f.FG])
				}
			}
		}
	}
	return img
}

func (a *App) fntExport(kind string) {
	f := a.fnt.f
	var flt []Filter
	switch kind {
	case "asm":
		flt = []Filter{{Name: "ACME assembler source (*.asm)", Pattern: "*.asm"}}
	case "bin":
		flt = []Filter{{Name: "Raw character set (*.bin)", Pattern: "*.bin"}}
	case "png":
		flt = []Filter{{Name: "PNG image (*.png)", Pattern: "*.png"}}
	}
	p, ok := a.host.SaveFile("Export character set", flt, a.workDir(), a.fntName()+"."+kind, kind)
	if !ok {
		return
	}
	p = FixDoubleExt(p)
	base := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	var data []byte
	var err error
	switch kind {
	case "asm":
		data = []byte(f.ACME(base, 0x3000))
	case "bin":
		data = append([]byte(nil), f.Data[:]...)
	case "png":
		data, err = encodePNG(a.fontSheet())
	}
	if err == nil {
		err = a.host.WriteFile(p, data)
	}
	if err != nil {
		a.host.Info(AppName, "Export failed:\n"+err.Error())
		return
	}
	a.setStatus(fmt.Sprintf("Exported: %s (%d bytes)", filepath.Base(p), len(data)))
}

// --- import dialog -----------------------------------------------------------------

type fntImportDialog struct {
	src  []int8
	name string
	o    c64.FontImport
}

// romPicture lays out a ROM character set as a picture: 40 characters per row.
func romPicture(rom []byte) []int8 {
	px := make([]int8, 320*200)
	for c := 0; c < 256; c++ {
		cx, cy := c%40*8, c/40*8
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				if rom[c*8+y]&(0x80>>x) != 0 {
					px[(cy+y)*320+cx+x] = 1
				}
			}
		}
	}
	return px
}

func (a *App) fntOpenImport() {
	a.modal = &fntImportDialog{src: a.pictureIndexes(), name: "the multicolor picture",
		o: c64.FontImport{Count: 256, BG: int(a.pic.BG)}}
}

func (d *fntImportDialog) Title() string          { return "Import characters" }
func (d *fntImportDialog) Size(a *App) (int, int) { return 1000, min(560, a.H-20) }
func (d *fntImportDialog) Key(a *App, k KeyEvent) {
	if k.Key == KeyEnter {
		d.apply(a)
	}
}

func (d *fntImportDialog) count() int {
	return max(0, min(d.o.Count, 256-d.o.Target, 1000-d.o.StartCell))
}

func (d *fntImportDialog) apply(a *App) {
	e := a.fnt
	e.push()
	o := d.o
	o.Count = d.count()
	e.f.ImportFont(d.src, o)
	e.cur = o.Target
	a.closeModal(false)
	a.setStatus(fmt.Sprintf("Imported %d characters from %s", o.Count, d.name))
}

func (d *fntImportDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	pr := gfx.Rect{X: r.X, Y: r.Y, W: 640, H: 400}
	a.drawIndexPicture(d.src, pr.X, pr.Y, 2)
	// selected cells
	n := d.count()
	for i := 0; i < n; i++ {
		c := d.o.StartCell + i
		cr := gfx.Rect{X: pr.X + c%40*16, Y: pr.Y + c/40*16, W: 16, H: 16}
		m.FillA(cr, 0xFFFFFF, 70)
		m.Frame(cr, 0x000000)
	}
	if n > 0 {
		c := d.o.StartCell
		m.Frame(gfx.Rect{X: pr.X + c%40*16, Y: pr.Y + c/40*16, W: 16, H: 16}, 0xFFFFFF)
	}
	m.Frame(pr.Inset(-1), th.Border)
	if c, _ := u.press("fi_src", pr, 0); c {
		d.o.StartCell = clampI((a.in.MY-pr.Y)/16*40+(a.in.MX-pr.X)/16, 0, 999)
	}
	m.Text(font.UI, r.X, pr.Bottom()+8, "Source: "+d.name+"   ·   click the first cell; cells are read row by row", th.TextDim)
	bx := r.X
	src := func(id, label string, f func()) {
		w := font.UI.Width(label) + 20
		if u.Button(id, gfx.Rect{X: bx, Y: pr.Bottom() + 28, W: w, H: 24}, label) {
			f()
		}
		bx += w + 6
	}
	src("fi_s0", "Multicolor picture", func() {
		d.src, d.name, d.o.BG = a.pictureIndexes(), "the multicolor picture", int(a.pic.BG)
	})
	src("fi_s1", "Load an image (320×200) …", func() {
		a.later(func() {
			if px, name, ok := a.loadIndexImage("Image with characters"); ok {
				d.src, d.name, d.o.BG = px, name, -1
			}
		})
	})
	src("fi_s2", "C64 font upper case", func() {
		d.src, d.name, d.o.BG, d.o.StartCell = romPicture(c64.CharsetUpper), "the C64 font (upper case)", 0, 0
		a.fnt.f.Ref = 0
	})
	src("fi_s3", "C64 font lower case", func() {
		d.src, d.name, d.o.BG, d.o.StartCell = romPicture(c64.CharsetLower), "the C64 font (lower case)", 0, 0
		a.fnt.f.Ref = 1
	})
	x := pr.Right() + 24
	w := r.Right() - x
	y := r.Y
	hdr := func(t string) {
		u.Header(gfx.Rect{X: x, Y: y, W: w, H: 22}, t)
		y += 28
	}
	lbl := func(s string) { m.Text(font.UI, x, y+5, s, th.Text) }
	hdr("Cells → characters")
	lbl("First cell")
	d.o.StartCell = u.Spinner("fi_sc", gfx.Rect{X: x + 120, Y: y, W: w - 120, H: 22}, d.o.StartCell, 0, 999, 1, "")
	y += 26
	m.Text(font.UI, x+120, y, fmt.Sprintf("column %d, row %d", d.o.StartCell%40, d.o.StartCell/40), th.TextDim)
	y += 20
	lbl("Number")
	d.o.Count = u.Spinner("fi_n", gfx.Rect{X: x + 120, Y: y, W: w - 120, H: 22}, d.o.Count, 1, 256, 1, "")
	y += 26
	lbl("To character")
	d.o.Target = u.Spinner("fi_t", gfx.Rect{X: x + 120, Y: y, W: w - 120, H: 22}, d.o.Target, 0, 255, 1, "")
	y += 26
	m.Text(font.UI, x+120, y, fmt.Sprintf("%d characters: $%02X – $%02X", d.count(), d.o.Target, d.o.Target+max(0, d.count()-1)), th.TextDim)
	y += 28
	hdr("Background (not set)")
	a.toolColorChoice("fi_bg", x, y, w, d.o.BG, "Most frequent color", func(c int) { d.o.BG = c })
	y += 34
	for _, ln := range wrapText(font.UI, "Every pixel that is not the background color becomes a set pixel (hires). A multicolor picture is read with its double wide pixels.", w) {
		m.Text(font.UI, x, y, ln, th.TextDim)
		y += font.UI.Height + 2
	}
	by := r.Bottom() - 28
	if u.Button("fi_cancel", gfx.Rect{X: r.Right() - 250, Y: by, W: 120, H: 28}, "Cancel") {
		a.closeModal(true)
		return
	}
	if u.ButtonEx("fi_ok", gfx.Rect{X: r.Right() - 120, Y: by, W: 120, H: 28}, "Import", true, d.count() > 0) {
		d.apply(a)
	}
}
