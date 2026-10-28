package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- PETSCII editor ------------------------------------------------------------------
//
// A full-window editor for PETSCII pictures (C64 standard character mode:
// 40 × 25 characters, one color per character, a common background). It
// converts the current picture or an image into PETSCII, lets you edit it and
// exports PRG, ACME assembler, PNG and Petmate files. Ideas from Petmate
// (nurpax, MIT), petsciirender (gnacu), PETSCII editor (Marq/ventti, WTFPL)
// and Image-to-PETSCII (amb).

const (
	petDraw = iota
	petColorize
	petCharOnly
	petErase
	petBrush
	petText
	petToolCount
)

var petToolNames = [petToolCount]string{"Draw", "Colorize", "Character only", "Eraser", "Brush", "Text"}
var petToolTips = [petToolCount]string{
	"Draw the character in the color (1)",
	"Change only the color of characters (2)",
	"Change only the characters, keep their colors (3)",
	"Erase: space characters (4)",
	"Drag a rectangle to copy it as brush, then click to stamp it (5)",
	"Click a cell and type (6)",
}

type petBrushData struct {
	w, h   int
	chars  []uint8
	colors []uint8
}

type petEditor struct {
	open       bool
	scr        c64.PetsciiScreen
	undo, redo []c64.PetsciiScreen
	tool       int
	ch, color  uint8
	arrange    int
	dropOpen   bool
	grid       bool
	brush      *petBrushData
	selecting  bool
	sx, sy     int // selection start cell
	stroke     bool
	lastCell   int
	lockAxis   int // 0 = free, 1 = horizontal, 2 = vertical (Shift)
	startCell  int
	cursor     int // text cursor (cell), -1 = none
	path       string
	dirty      bool
	charScroll int
	// layout
	canvas    gfx.Rect
	zoom      int
	hoverCell int
}

func (a *App) openPetscii() {
	a.commitOrCancelForHistory()
	a.menuOpen = -1
	if a.pet == nil {
		s := c64.NewPetscii(a.pic.BG, a.pic.Border)
		a.pet = &petEditor{scr: *s, ch: 81, color: 1, cursor: -1, grid: false}
	}
	a.pet.open = true
	a.setStatus("PETSCII editor")
}

func (a *App) closePetscii() {
	a.pet.open = false
	a.pet.dropOpen = false
	a.modal = nil
	a.setStatus("Back to the picture")
}

func (e *petEditor) push() {
	e.undo = append(e.undo, e.scr)
	if len(e.undo) > 200 {
		e.undo = e.undo[1:]
	}
	e.redo = nil
	e.dirty = true
}

func (e *petEditor) doUndo() {
	if n := len(e.undo); n > 0 {
		e.redo = append(e.redo, e.scr)
		e.scr = e.undo[n-1]
		e.undo = e.undo[:n-1]
		e.dirty = true
	}
}

func (e *petEditor) doRedo() {
	if n := len(e.redo); n > 0 {
		e.undo = append(e.undo, e.scr)
		e.scr = e.redo[n-1]
		e.redo = e.redo[:n-1]
		e.dirty = true
	}
}

// codes returns the characters of the selector.
func (e *petEditor) codes() []int {
	return petArrangements[clampI(e.arrange, 0, len(petArrangements)-1)].codes(e.scr.Lower)
}

// set changes a cell with the current tool.
func (e *petEditor) set(cell int, erase bool) {
	if cell < 0 || cell >= c64.PetCells {
		return
	}
	if erase || e.tool == petErase {
		e.scr.Chars[cell] = 32
		return
	}
	switch e.tool {
	case petDraw:
		e.scr.Chars[cell], e.scr.Colors[cell] = e.ch, e.color
	case petColorize:
		e.scr.Colors[cell] = e.color
	case petCharOnly:
		e.scr.Chars[cell] = e.ch
	}
}

func (e *petEditor) stamp(cell int) {
	b := e.brush
	if b == nil {
		return
	}
	cx, cy := cell%c64.PetCols, cell/c64.PetCols
	for y := 0; y < b.h; y++ {
		for x := 0; x < b.w; x++ {
			tx, ty := cx+x, cy+y
			if tx < 0 || ty < 0 || tx >= c64.PetCols || ty >= c64.PetRows {
				continue
			}
			i := ty*c64.PetCols + tx
			e.scr.Chars[i], e.scr.Colors[i] = b.chars[y*b.w+x], b.colors[y*b.w+x]
		}
	}
}

// shift moves the whole screen by one cell (wrapping around).
func (e *petEditor) shift(dx, dy int) {
	e.push()
	old := e.scr
	for y := 0; y < c64.PetRows; y++ {
		for x := 0; x < c64.PetCols; x++ {
			sx := (x - dx + c64.PetCols) % c64.PetCols
			sy := (y - dy + c64.PetRows) % c64.PetRows
			e.scr.Chars[y*c64.PetCols+x] = old.Chars[sy*c64.PetCols+sx]
			e.scr.Colors[y*c64.PetCols+x] = old.Colors[sy*c64.PetCols+sx]
		}
	}
}

// --- input -----------------------------------------------------------------------

func (a *App) petKeys() {
	e := a.pet
	for _, k := range a.in.Keys {
		m := k.Mods
		switch {
		case m.Ctrl && k.Key == 'Z' && !m.Shift:
			e.doUndo()
		case m.Ctrl && (k.Key == 'Y' || (k.Key == 'Z' && m.Shift)):
			e.doRedo()
		case m.Ctrl && k.Key == 'S':
			a.later(a.petSave)
		case m.Ctrl && k.Key == 'O':
			a.later(a.petOpen)
		case m.Alt && k.Key == KeyLeft:
			e.shift(-1, 0)
		case m.Alt && k.Key == KeyRight:
			e.shift(1, 0)
		case m.Alt && k.Key == KeyUp:
			e.shift(0, -1)
		case m.Alt && k.Key == KeyDown:
			e.shift(0, 1)
		case k.Key == KeyEsc:
			switch {
			case e.dropOpen:
				e.dropOpen = false
			case e.brush != nil || e.selecting:
				e.brush, e.selecting = nil, false
			case e.cursor >= 0:
				e.cursor = -1
			}
		case k.Key == KeyF1:
			a.openHelp("petscii")
		case e.tool == petText && e.cursor >= 0:
			a.petTextKey(k)
		}
	}
	for _, c := range a.in.Chars {
		if e.tool == petText && e.cursor >= 0 {
			if e.cursor < c64.PetCells {
				if !e.stroke {
					e.push()
				}
				e.scr.Chars[e.cursor] = c64.ScreenCode(c, e.scr.Lower)
				e.scr.Colors[e.cursor] = e.color
				e.cursor = min(e.cursor+1, c64.PetCells-1)
			}
			continue
		}
		switch c {
		case '1', '2', '3', '4', '5', '6':
			e.tool = int(c - '1')
			e.selecting = false
		case 'g', 'G':
			e.grid = !e.grid
		case 'f', 'F':
			e.ch ^= 0x80
		case 'q', 'Q':
			e.color = (e.color + 15) & 15
		case 'e', 'E':
			e.color = (e.color + 1) & 15
		case 'u':
			e.doUndo()
		case 'U':
			e.doRedo()
		case 'a', 'd', 'w', 's', 'A', 'D', 'W', 'S':
			codes := e.codes()
			idx := 0
			for i, v := range codes {
				if v == int(e.ch) {
					idx = i
				}
			}
			step := map[rune]int{'a': -1, 'd': 1, 'w': -16, 's': 16, 'A': -1, 'D': 1, 'W': -16, 'S': 16}[c]
			idx = clampI(idx+step, 0, len(codes)-1)
			e.ch = uint8(codes[idx])
		}
	}
}

func (a *App) petTextKey(k KeyEvent) {
	e := a.pet
	switch k.Key {
	case KeyBackspace:
		if e.cursor > 0 {
			e.push()
			e.cursor--
			e.scr.Chars[e.cursor] = 32
		}
	case KeyEnter:
		e.cursor = min((e.cursor/c64.PetCols+1)*c64.PetCols, c64.PetCells-1)
	case KeyLeft:
		e.cursor = max(e.cursor-1, 0)
	case KeyRight:
		e.cursor = min(e.cursor+1, c64.PetCells-1)
	case KeyUp:
		if e.cursor >= c64.PetCols {
			e.cursor -= c64.PetCols
		}
	case KeyDown:
		if e.cursor+c64.PetCols < c64.PetCells {
			e.cursor += c64.PetCols
		}
	}
}

func (a *App) petCanvasInput() {
	e := a.pet
	in := &a.in
	r := e.canvas
	z := e.zoom
	inside := func(x, y int) (int, bool) {
		cx := (x - r.X) / (8 * z)
		cy := (y - r.Y) / (8 * z)
		if x < r.X || y < r.Y || cx >= c64.PetCols || cy >= c64.PetRows {
			return -1, false
		}
		return cy*c64.PetCols + cx, true
	}
	cell, ok := inside(in.MX, in.MY)
	e.hoverCell = -1
	if ok {
		e.hoverCell = cell
	}
	if a.ui.blocked {
		return
	}
	pressed := ok && (in.Pressed[0] || in.Pressed[1])
	btn := 0
	if in.Pressed[1] || (in.Down[1] && !in.Down[0]) {
		btn = 1
	}
	// Alt+click: pick character and color
	if pressed && in.Mods.Alt {
		e.ch, e.color = e.scr.Chars[cell], e.scr.Colors[cell]
		return
	}
	switch e.tool {
	case petText:
		if pressed {
			e.cursor = cell
		}
		return
	case petBrush:
		if e.brush != nil {
			if pressed && btn == 0 {
				e.push()
				e.stamp(cell)
			} else if pressed && btn == 1 {
				e.brush = nil
			}
			return
		}
		if pressed && btn == 0 {
			e.selecting = true
			e.sx, e.sy = cell%c64.PetCols, cell/c64.PetCols
		}
		if e.selecting && !in.Down[0] {
			e.selecting = false
			ex, ey := e.hoverCellXY(in.MX, in.MY)
			x0, x1 := min(e.sx, ex), max(e.sx, ex)
			y0, y1 := min(e.sy, ey), max(e.sy, ey)
			b := &petBrushData{w: x1 - x0 + 1, h: y1 - y0 + 1}
			for y := y0; y <= y1; y++ {
				for x := x0; x <= x1; x++ {
					i := y*c64.PetCols + x
					b.chars = append(b.chars, e.scr.Chars[i])
					b.colors = append(b.colors, e.scr.Colors[i])
				}
			}
			e.brush = b
		}
		return
	}
	if pressed {
		e.push()
		e.stroke = true
		e.lastCell = cell
		e.startCell = cell
		e.lockAxis = 0
		e.set(cell, btn == 1)
		return
	}
	if e.stroke {
		if !in.Down[0] && !in.Down[1] {
			e.stroke = false
			return
		}
		cx, cy := e.hoverCellXY(in.MX, in.MY)
		// Shift: straight horizontal or vertical line from the start
		if in.Mods.Shift {
			sx, sy := e.startCell%c64.PetCols, e.startCell/c64.PetCols
			if e.lockAxis == 0 && (cx != sx || cy != sy) {
				if abs(cx-sx) >= abs(cy-sy) {
					e.lockAxis = 1
				} else {
					e.lockAxis = 2
				}
			}
			if e.lockAxis == 1 {
				cy = sy
			} else if e.lockAxis == 2 {
				cx = sx
			}
		}
		c := cy*c64.PetCols + cx
		if c != e.lastCell {
			// fill the gap between the last and this cell
			lx, ly := e.lastCell%c64.PetCols, e.lastCell/c64.PetCols
			forLine(lx, ly, cx, cy, func(x, y int) { e.set(y*c64.PetCols+x, in.Down[1] && !in.Down[0]) })
			e.lastCell = c
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// hoverCellXY returns the (clamped) cell under a screen position.
func (e *petEditor) hoverCellXY(x, y int) (int, int) {
	z := e.zoom
	cx := clampI((x-e.canvas.X)/(8*z), 0, c64.PetCols-1)
	cy := clampI((y-e.canvas.Y)/(8*z), 0, c64.PetRows-1)
	if x < e.canvas.X {
		cx = 0
	}
	if y < e.canvas.Y {
		cy = 0
	}
	return cx, cy
}

// --- drawing -----------------------------------------------------------------------

func (a *App) petFrame() {
	e := a.pet
	if a.modal != nil {
		a.handleKeys()
	} else {
		a.petKeys()
	}
	if !e.open {
		return
	}
	a.ui.blocked = a.modal != nil || e.dropOpen
	m := a.img
	th := a.th
	m.Fill(gfx.Rect{X: 0, Y: 0, W: a.W, H: a.H}, th.Bg)
	const topH, leftW, rightW, statusH = 38, 190, 300, 24
	// canvas
	area := gfx.Rect{X: leftW + 10, Y: topH + 10, W: a.W - leftW - rightW - 20, H: a.H - topH - statusH - 20}
	z := max(1, min(area.W/384, area.H/272))
	e.zoom = z
	fr := gfx.Rect{X: area.X + (area.W-384*z)/2, Y: area.Y + (area.H-272*z)/2, W: 384 * z, H: 272 * z}
	e.canvas = gfx.Rect{X: fr.X + 32*z, Y: fr.Y + 35*z, W: 320 * z, H: 200 * z}
	if !e.dropOpen {
		a.petCanvasInput()
	}
	a.petDrawCanvas(fr)
	a.petDrawTop(topH)
	a.petDrawLeft(gfx.Rect{X: 0, Y: topH, W: leftW, H: a.H - topH - statusH})
	a.petDrawRight(gfx.Rect{X: a.W - rightW, Y: topH, W: rightW, H: a.H - topH - statusH})
	// status
	sr := gfx.Rect{X: 0, Y: a.H - statusH, W: a.W, H: statusH}
	m.Fill(sr, th.Panel)
	m.HLine(0, a.W-1, sr.Y, th.Border)
	status := petToolTips[e.tool]
	if e.hoverCell >= 0 {
		c := e.hoverCell
		status = fmt.Sprintf("Column %2d, row %2d   ·   character $%02X (%d)   ·   color %s", c%c64.PetCols, c/c64.PetCols,
			e.scr.Chars[c], e.scr.Chars[c], c64.ColorNames[e.scr.Colors[c]])
	}
	m.TextL(font.UI, gfx.Rect{X: 10, Y: sr.Y, W: a.W - 20, H: sr.H}, status, th.Text)
	m.TextR(font.UI, gfx.Rect{X: 10, Y: sr.Y, W: a.W - 20, H: sr.H}, "Alt+click: pick · Shift: straight line · right button: erase · F1: help", th.TextDim)
	if e.dropOpen {
		a.ui.blocked = a.modal != nil
		a.petDrawDropdown()
	}
	if a.modal != nil {
		a.ui.blocked = false
		a.drawModal()
	}
}

// petGlyph draws one character at (x, y) with pixel size s.
func (a *App) petGlyph(x, y, s int, code uint8, fg, bg uint32, lower bool) {
	rom := c64.CharsetUpper
	if lower {
		rom = c64.CharsetLower
	}
	m := a.img
	if bg != 0xFFFFFFFF {
		m.Fill(gfx.Rect{X: x, Y: y, W: 8 * s, H: 8 * s}, bg)
	}
	for yy := 0; yy < 8; yy++ {
		b := rom[int(code)*8+yy]
		for xx := 0; xx < 8; xx++ {
			if b&(0x80>>xx) != 0 {
				m.Fill(gfx.Rect{X: x + xx*s, Y: y + yy*s, W: s, H: s}, fg)
			}
		}
	}
}

func (a *App) petDrawCanvas(fr gfx.Rect) {
	e := a.pet
	m := a.img
	z := e.zoom
	pal := a.pal.Colors
	m.Fill(fr, pal[e.scr.Border])
	m.FillA(gfx.Rect{X: fr.Right(), Y: fr.Y + 6, W: 6, H: fr.H}, a.th.Shadow, 80)
	m.FillA(gfx.Rect{X: fr.X + 6, Y: fr.Bottom(), W: fr.W, H: 6}, a.th.Shadow, 80)
	cr := e.canvas
	bg := pal[e.scr.BG]
	rom := e.scr.Charset()
	for y := 0; y < 200; y++ {
		row := m.Pix[(cr.Y+y*z)*m.W:]
		_ = row
		for x := 0; x < 320; x++ {
			ci := (y/8)*c64.PetCols + x/8
			c := bg
			if rom[int(e.scr.Chars[ci])*8+y%8]&(0x80>>(x%8)) != 0 {
				c = pal[e.scr.Colors[ci]]
			}
			m.Fill(gfx.Rect{X: cr.X + x*z, Y: cr.Y + y*z, W: z, H: z}, c)
		}
	}
	if e.grid {
		for cx := 1; cx < c64.PetCols; cx++ {
			x := cr.X + cx*8*z
			for y := cr.Y; y < cr.Bottom(); y += 2 {
				m.BlendPix(x, y, 0x808080, 140)
			}
		}
		for cy := 1; cy < c64.PetRows; cy++ {
			y := cr.Y + cy*8*z
			for x := cr.X; x < cr.Right(); x += 2 {
				m.BlendPix(x, y, 0x808080, 140)
			}
		}
	}
	cellRect := func(cx, cy, w, h int) gfx.Rect {
		return gfx.Rect{X: cr.X + cx*8*z, Y: cr.Y + cy*8*z, W: w * 8 * z, H: h * 8 * z}
	}
	// previews
	if e.hoverCell >= 0 && a.modal == nil && !e.dropOpen {
		hx, hy := e.hoverCell%c64.PetCols, e.hoverCell/c64.PetCols
		switch {
		case e.tool == petBrush && e.brush != nil:
			b := e.brush
			for y := 0; y < b.h; y++ {
				for x := 0; x < b.w; x++ {
					if hx+x < c64.PetCols && hy+y < c64.PetRows {
						a.petGlyph(cr.X+(hx+x)*8*z, cr.Y+(hy+y)*8*z, z, b.chars[y*b.w+x], pal[b.colors[y*b.w+x]], bg, e.scr.Lower)
					}
				}
			}
			dashedFrame(m, cellRect(hx, hy, min(b.w, c64.PetCols-hx), min(b.h, c64.PetRows-hy)), 0xFFFFFF, 0, 0)
		case e.tool == petBrush && e.selecting:
			x0, x1 := min(e.sx, hx), max(e.sx, hx)
			y0, y1 := min(e.sy, hy), max(e.sy, hy)
			dashedFrame(m, cellRect(x0, y0, x1-x0+1, y1-y0+1), 0xFFFFFF, 0, int(a.now/80)%8)
			a.needAnim = true
		case e.tool == petDraw && !e.stroke:
			a.petGlyph(cr.X+hx*8*z, cr.Y+hy*8*z, z, e.ch, pal[e.color], bg, e.scr.Lower)
			dashedFrame(m, cellRect(hx, hy, 1, 1), 0xFFFFFF, 0, 0)
		default:
			dashedFrame(m, cellRect(hx, hy, 1, 1), 0xFFFFFF, 0, 0)
		}
	}
	if e.tool == petText && e.cursor >= 0 && (a.now/400)%2 == 0 {
		r := cellRect(e.cursor%c64.PetCols, e.cursor/c64.PetCols, 1, 1)
		m.FillA(r, pal[e.color], 160)
	}
	if e.tool == petText {
		a.needAnim = true
	}
}

func (a *App) petDrawTop(h int) {
	e := a.pet
	u := &a.ui
	m := a.img
	th := a.th
	r := gfx.Rect{X: 0, Y: 0, W: a.W, H: h}
	m.Fill(r, th.MenuBar)
	m.HLine(0, a.W-1, h-1, th.Border)
	m.Text(font.UIB, 10, (h-font.UIB.Height)/2, "PETSCII editor", th.MenuText)
	x := 140
	btn := func(id, label, tip string, f func()) {
		w := font.UI.Width(label) + 18
		br := gfx.Rect{X: x, Y: 6, W: w, H: h - 12}
		if u.Button(id, br, label) {
			a.later(f)
		}
		u.Tip(id, br, tip)
		x += w + 4
	}
	btn("pt_new", "New", "Empty screen", a.petNew)
	btn("pt_conv", "Convert …", "Convert the multicolor picture or an image into PETSCII", a.petOpenConvert)
	btn("pt_open", "Open …", "Open a Petmate file (.petmate)", a.petOpen)
	btn("pt_save", "Save …", "Save as Petmate file (.petmate) – Ctrl+S", a.petSave)
	x += 8
	btn("pt_prg", "Export PRG …", "Runnable C64 program", func() { a.petExport("prg") })
	btn("pt_asm", "Export ASM …", "Source code for the ACME assembler", func() { a.petExport("asm") })
	btn("pt_png", "Export PNG …", "Picture with border (384 × 272)", func() { a.petExport("png") })
	x += 8
	btn("pt_undo", "Undo", "Ctrl+Z", e.doUndo)
	btn("pt_redo", "Redo", "Ctrl+Y", e.doRedo)
	btn("pt_help", "Help", "F1", func() { a.openHelp("petscii") })
	cr := gfx.Rect{X: a.W - 170, Y: 6, W: 160, H: h - 12}
	if u.ButtonEx("pt_close", cr, "Close PETSCII editor", true, true) {
		a.later(a.closePetscii)
	}
}

func (a *App) petDrawLeft(r gfx.Rect) {
	e := a.pet
	u := &a.ui
	m := a.img
	th := a.th
	m.Fill(r, th.Panel)
	m.VLine(r.Right()-1, r.Y, r.Bottom()-1, th.Border)
	x, w := r.X+10, r.W-20
	y := r.Y + 8
	m.Text(font.UIB, x, y, "Tools", th.Text)
	y += 22
	for i := 0; i < petToolCount; i++ {
		br := gfx.Rect{X: x, Y: y, W: w, H: 24}
		if u.ButtonEx("pt_t"+itoa(i), br, fmt.Sprintf("%d  %s", i+1, petToolNames[i]), e.tool == i, true) {
			e.tool = i
			e.selecting = false
		}
		u.Tip("pt_t"+itoa(i), br, petToolTips[i])
		y += 28
	}
	if e.tool == petBrush && e.brush != nil {
		if u.Button("pt_bclr", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Drop brush (Esc)") {
			e.brush = nil
		}
		y += 28
	}
	y += 6
	m.Text(font.UIB, x, y, "Character", th.Text)
	y += 20
	pal := a.pal.Colors
	big := gfx.Rect{X: x, Y: y, W: 64, H: 64}
	a.petGlyph(big.X, big.Y, 8, e.ch, pal[e.color], pal[e.scr.BG], e.scr.Lower)
	m.Frame(big.Inset(-1), th.Border)
	m.Text(font.UI, x+74, y+4, fmt.Sprintf("$%02X  (%d)", e.ch, e.ch), th.Text)
	if u.Button("pt_inv", gfx.Rect{X: x + 74, Y: y + 26, W: w - 74, H: 22}, "Invert (F)") {
		e.ch ^= 0x80
	}
	y += 74
	e.grid = u.Checkbox("pt_grid", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Grid (G)", e.grid)
	y += 28
	m.Text(font.UIB, x, y, "Character set", th.Text)
	y += 20
	bw := (w - 4) / 2
	for i, n := range []string{"Upper", "Lower"} {
		if u.ButtonEx("pt_cs"+itoa(i), gfx.Rect{X: x + i*(bw+4), Y: y, W: bw, H: 22}, n, e.scr.Lower == (i == 1), true) && e.scr.Lower != (i == 1) {
			e.push()
			e.scr.Lower = i == 1
		}
	}
	y += 30
	colRow := func(label string, cur *uint8, id string) {
		m.Text(font.UIB, x, y, label, th.Text)
		y += 20
		sw := w / 8
		for i := 0; i < 16; i++ {
			br := gfx.Rect{X: x + (i%8)*sw, Y: y + (i/8)*(sw-2), W: sw - 2, H: sw - 4}
			m.Fill(br, pal[i])
			m.Frame(br, th.BtnBorder)
			if *cur == uint8(i) {
				m.Frame(br.Inset(-1), th.Accent)
				m.Frame(br.Inset(-2), th.Accent)
			}
			if c, _ := u.press(id+itoa(i), br, 0); c && *cur != uint8(i) {
				e.push()
				*cur = uint8(i)
			}
			u.Tip(id+itoa(i), br, c64.ColorNames[i])
		}
		y += 2*(sw-2) + 8
	}
	colRow("Background ($D021)", &e.scr.BG, "pt_bg")
	colRow("Border ($D020)", &e.scr.Border, "pt_bo")
	if e.path != "" {
		for _, ln := range wrapText(font.UI, filepath.Base(e.path), w) {
			m.Text(font.UI, x, y, ln, th.TextDim)
			y += font.UI.Height + 2
		}
	}
}

func (a *App) petDrawRight(r gfx.Rect) {
	e := a.pet
	u := &a.ui
	m := a.img
	th := a.th
	pal := a.pal.Colors
	m.Fill(r, th.Panel)
	m.VLine(r.X, r.Y, r.Bottom()-1, th.Border)
	x, w := r.X+12, r.W-24
	y := r.Y + 8
	m.Text(font.UIB, x, y, "Characters", th.Text)
	y += 22
	// arrangement dropdown
	dr := gfx.Rect{X: x, Y: y, W: w, H: 24}
	label := petArrangements[clampI(e.arrange, 0, len(petArrangements)-1)].name + "  ▾"
	if u.Button("pt_drop", dr, label) {
		e.dropOpen = !e.dropOpen
	}
	u.Tip("pt_drop", dr, "Arrangement of the characters")
	y += 32
	// character grid: 16 columns
	codes := e.codes()
	cs := 2
	cell := min(8*cs+2, w/16)
	cols := 16
	rows := (len(codes) + cols - 1) / cols
	gh := min(rows*cell, r.Bottom()-y-150)
	gr := gfx.Rect{X: x, Y: y, W: cols * cell, H: gh}
	maxScroll := max(0, rows*cell-gh)
	if u.hover(gr) && a.in.Wheel != 0 && !u.wheelUsed {
		e.charScroll -= a.in.Wheel * cell * 2
		u.wheelUsed = true
	}
	e.charScroll = clampI(e.charScroll, 0, maxScroll)
	m.Fill(gr, pal[e.scr.BG])
	old := m.SetClip(gr.Intersect(m.Clip))
	for i, code := range codes {
		cx := gr.X + (i%cols)*cell
		cy := gr.Y + (i/cols)*cell - e.charScroll
		if cy+cell < gr.Y || cy > gr.Bottom() {
			continue
		}
		a.petGlyph(cx+1, cy+1, cs, uint8(code), pal[e.color], 0xFFFFFFFF, e.scr.Lower)
		br := gfx.Rect{X: cx, Y: cy, W: cell, H: cell}
		if uint8(code) == e.ch {
			m.Frame(br, 0xFFFFFF)
			m.Frame(br.Inset(-1), 0x000000)
		}
		if gr.Contains(a.in.MX, a.in.MY) {
			if c, _ := u.press(fmt.Sprintf("pt_c%d", i), br, 0); c {
				e.ch = uint8(code)
				if e.tool == petErase || e.tool == petColorize || e.tool == petBrush {
					e.tool = petDraw
				}
			}
		}
	}
	m.SetClip(old)
	m.Frame(gr.Inset(-1), th.Border)
	y += gh + 8
	m.Text(font.UI, x, y, "W A S D: move in the characters", th.TextDim)
	y += 22
	m.Text(font.UIB, x, y, "Color", th.Text)
	m.TextR(font.UI, gfx.Rect{X: x, Y: y, W: w, H: 16}, "Q / E: previous / next", th.TextDim)
	y += 20
	sw := w / 8
	for i := 0; i < 16; i++ {
		br := gfx.Rect{X: x + (i%8)*sw, Y: y + (i/8)*(sw-4), W: sw - 2, H: sw - 6}
		m.Fill(br, pal[i])
		m.Frame(br, th.BtnBorder)
		if e.color == uint8(i) {
			m.Frame(br.Inset(-1), th.Accent)
			m.Frame(br.Inset(-2), th.Accent)
		}
		if c, _ := u.press("pt_col"+itoa(i), br, 0); c {
			e.color = uint8(i)
		}
		u.Tip("pt_col"+itoa(i), br, c64.ColorNames[i])
	}
}

func (a *App) petDrawDropdown() {
	e := a.pet
	u := &a.ui
	m := a.img
	th := a.th
	x := a.W - 300 + 12
	y := 38 + 8 + 22 + 26
	w := 300 - 24
	r := gfx.Rect{X: x, Y: y, W: w, H: len(petArrangements)*26 + 4}
	m.FillA(gfx.Rect{X: r.X + 4, Y: r.Y + 4, W: r.W, H: r.H}, th.Shadow, 90)
	m.Fill(r, th.Popup)
	m.Frame(r, th.PopupBorder)
	for i, p := range petArrangements {
		br := gfx.Rect{X: r.X + 2, Y: r.Y + 2 + i*26, W: r.W - 4, H: 24}
		if u.ButtonEx("pt_ar"+itoa(i), br, p.name, e.arrange == i, true) {
			e.arrange = i
			e.charScroll = 0
			e.dropOpen = false
		}
	}
	if a.in.Pressed[0] && !r.Contains(a.in.MX, a.in.MY) {
		e.dropOpen = false
		a.in.Pressed[0] = false
	}
}

// --- files -------------------------------------------------------------------------

var petFilters = []Filter{{Name: "Petmate file (*.petmate)", Pattern: "*.petmate"}}

func (a *App) petNew() {
	e := a.pet
	e.push()
	e.scr = *c64.NewPetscii(e.scr.BG, e.scr.Border)
	e.path = ""
}

func (a *App) petName() string {
	if a.pet.path != "" {
		return strings.TrimSuffix(filepath.Base(a.pet.path), filepath.Ext(a.pet.path))
	}
	return "petscii"
}

func (a *App) petOpen() {
	p, ok := a.host.OpenFile("Open PETSCII (Petmate file)", petFilters, a.workDir())
	if !ok {
		return
	}
	data, err := a.host.ReadFile(p)
	if err != nil {
		a.host.Info(AppName, "Cannot open the file:\n"+err.Error())
		return
	}
	a.petLoad(p, data)
}

func (a *App) petLoad(p string, data []byte) {
	s, err := c64.LoadPetmate(data)
	if err != nil {
		a.host.Info(AppName, "Cannot open the file:\n"+err.Error())
		return
	}
	a.pet.push()
	a.pet.scr = *s
	a.pet.path = p
	a.pet.dirty = false
	a.addRecent(p)
}

func (a *App) petSave() {
	p, ok := a.host.SaveFile("Save PETSCII (Petmate file)", petFilters, a.workDir(), a.petName()+".petmate", "petmate")
	if !ok {
		return
	}
	p = FixDoubleExt(p)
	data, err := a.pet.scr.Petmate(a.petName())
	if err == nil {
		err = a.host.WriteFile(p, data)
	}
	if err != nil {
		a.host.Info(AppName, "Saving failed:\n"+err.Error())
		return
	}
	a.pet.path = p
	a.pet.dirty = false
	a.setStatus("Saved: " + filepath.Base(p))
}

func (a *App) petExport(kind string) {
	s := &a.pet.scr
	var f []Filter
	var data []byte
	var err error
	name := a.petName()
	switch kind {
	case "prg":
		f = []Filter{{Name: "C64 program (*.prg)", Pattern: "*.prg"}}
		data = s.PRG()
	case "asm":
		f = []Filter{{Name: "ACME assembler source (*.asm)", Pattern: "*.asm"}}
		data = []byte(s.ACME(name))
	case "png":
		f = []Filter{{Name: "PNG image (*.png)", Pattern: "*.png"}}
		data, err = s.PNG(a.pal, true)
	}
	p, ok := a.host.SaveFile("Export PETSCII", f, a.workDir(), name+"."+kind, kind)
	if !ok {
		return
	}
	p = FixDoubleExt(p)
	if kind == "asm" {
		// the source names its output after the file
		data = []byte(s.ACME(strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))))
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

// --- convert dialog ----------------------------------------------------------------

type petConvertDialog struct {
	src     [][3]int // 320 × 200, before adjustments
	name    string
	opts    c64.PetsciiOptions
	adj     c64.Adjust
	preview *c64.PetsciiScreen
	key     string
}

// pictureRGB renders the current multicolor picture as 320 × 200 RGB.
func (a *App) pictureRGB() [][3]int {
	out := make([][3]int, 320*200)
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			c := a.pal.Colors[a.pic.ColorAt(x/asp, y)]
			out[y*320+x] = [3]int{int(c >> 16 & 255), int(c >> 8 & 255), int(c & 255)}
		}
	}
	return out
}

func (a *App) petOpenConvert() {
	o := a.set.Petscii
	o.BG = clampI(o.BG, -1, 15)
	a.modal = &petConvertDialog{src: a.pictureRGB(), name: "the multicolor picture", opts: o}
}

func (d *petConvertDialog) Title() string          { return "Convert to PETSCII" }
func (d *petConvertDialog) Size(a *App) (int, int) { return 1000, min(640, a.H-20) }
func (d *petConvertDialog) Key(a *App, k KeyEvent) {
	if k.Key == KeyEnter {
		d.apply(a)
	}
}

func (d *petConvertDialog) apply(a *App) {
	if d.preview == nil {
		return
	}
	a.set.Petscii = d.opts
	a.saveSettings()
	e := a.pet
	e.push()
	e.scr = *d.preview
	e.scr.Border = a.pic.Border
	a.closeModal(false)
	a.setStatus("Converted " + d.name + " to PETSCII")
}

func (d *petConvertDialog) loadImage(a *App) {
	p, ok := a.host.OpenFile("Image for PETSCII", []Filter{{Name: "Images (*.png, *.bmp, *.jpg, *.gif)", Pattern: "*.png;*.bmp;*.jpg;*.jpeg;*.gif"}}, a.workDir())
	if !ok {
		return
	}
	data, err := a.host.ReadFile(p)
	if err == nil {
		img, err2 := decodeImage(data, p)
		if err2 == nil {
			d.src = c64.ResampleScreen(img, c64.FitContain)
			d.name = filepath.Base(p)
			d.key = ""
			return
		}
		err = err2
	}
	a.host.Info(AppName, "Cannot read the image:\n"+err.Error())
}

func (d *petConvertDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	key := fmt.Sprintf("%v/%v/%p/%d", d.opts, d.adj, d.src, a.palIdx)
	if key != d.key {
		d.key = key
		d.preview = c64.ConvertPetscii(c64.ApplyAdjust(d.src, d.adj), a.pal, d.opts)
		if d.preview != nil {
			d.preview.Border = a.pic.Border
		}
	}
	// preview (2×)
	pr := gfx.Rect{X: r.X, Y: r.Y, W: 640, H: 400}
	if d.preview != nil {
		pal := a.pal.Colors
		for y := 0; y < 200; y++ {
			for x := 0; x < 320; x++ {
				m.Fill(gfx.Rect{X: pr.X + x*2, Y: pr.Y + y*2, W: 2, H: 2}, pal[d.preview.ColorAt(x, y)])
			}
		}
	}
	m.Frame(pr.Inset(-1), th.Border)
	m.Text(font.UI, r.X, pr.Bottom()+10, "Source: "+d.name, th.Text)
	m.Text(font.UI, r.X, pr.Bottom()+28, "Every 8 × 8 cell gets the character and color that fit it best.", th.TextDim)
	if u.Button("pc_src0", gfx.Rect{X: r.X, Y: pr.Bottom() + 52, W: 200, H: 24}, "Use the multicolor picture") {
		d.src, d.name, d.key = a.pictureRGB(), "the multicolor picture", ""
	}
	if u.Button("pc_src1", gfx.Rect{X: r.X + 206, Y: pr.Bottom() + 52, W: 160, H: 24}, "Load an image …") {
		a.later(func() { d.loadImage(a) })
	}
	// settings
	x := pr.Right() + 24
	w := r.Right() - x
	y := r.Y
	hdr := func(t string) {
		u.Header(gfx.Rect{X: x, Y: y, W: w, H: 22}, t)
		y += 28
	}
	row := func(id string, labels []string, sel int) int {
		bw := (w - 4*(len(labels)-1)) / len(labels)
		res := -1
		for i, l := range labels {
			if u.ButtonEx(id+itoa(i), gfx.Rect{X: x + i*(bw+4), Y: y, W: bw, H: 22}, l, sel == i, true) {
				res = i
			}
		}
		y += 28
		return res
	}
	hdr("Character set")
	cs := 0
	if d.opts.Lower {
		cs = 1
	}
	if v := row("pc_cs", []string{"Upper case", "Lower case"}, cs); v >= 0 {
		d.opts.Lower = v == 1
	}
	hdr("Characters")
	if v := row("pc_ch", []string{"All", "No letters", "Blocks"}, d.opts.Chars); v >= 0 {
		d.opts.Chars = v
	}
	d.opts.Inverse = u.Checkbox("pc_inv", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Also inverted characters", d.opts.Inverse)
	y += 28
	hdr("Color matching")
	mw := (w - 8) / 3
	for i := 0; i < len(c64.MatchNames); i++ {
		br := gfx.Rect{X: x + (i%3)*(mw+4), Y: y + (i/3)*25, W: mw, H: 22}
		if u.ButtonEx("pc_m"+itoa(i), br, c64.MatchNames[i], int(d.opts.Match) == i, true) {
			d.opts.Match = c64.ColorMatch(i)
		}
		u.Tip("pc_m"+itoa(i), br, c64.MatchInfo[i])
	}
	y += 56
	hdr("Adjustments")
	d.adj.Brightness = u.Slider("pc_br", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Brightness", d.adj.Brightness, -100, 100, "")
	y += 26
	d.adj.Contrast = u.Slider("pc_co", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Contrast", d.adj.Contrast, -100, 100, "")
	y += 30
	hdr("Background ($D021)")
	sw := (w - 16) / 17
	for i := -1; i < 16; i++ {
		sr := gfx.Rect{X: x + (i+1)*(sw+1), Y: y, W: sw, H: 22}
		if i < 0 {
			m.Fill(sr, th.Btn)
			m.TextC(font.UIB, sr, "A", th.Text)
		} else {
			m.Fill(sr, a.pal.Colors[i])
		}
		if d.opts.BG == i {
			m.Frame(sr.Inset(-1), th.Accent)
			m.Frame(sr, th.Accent)
		}
		if c, _ := u.press("pc_bg"+itoa(i), sr, 0); c {
			d.opts.BG = i
		}
		if i < 0 {
			u.Tip("pc_bg"+itoa(i), sr, "Choose automatically")
		} else {
			u.Tip("pc_bg"+itoa(i), sr, c64.ColorNames[i])
		}
	}
	by := r.Bottom() - 28
	if u.Button("pc_cancel", gfx.Rect{X: r.Right() - 250, Y: by, W: 120, H: 28}, "Cancel") {
		a.closeModal(true)
		return
	}
	if u.ButtonEx("pc_ok", gfx.Rect{X: r.Right() - 120, Y: by, W: 120, H: 28}, "Convert", true, d.preview != nil) {
		d.apply(a)
	}
}
