package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- Sprite editor --------------------------------------------------------------------
//
// A full-window editor for up to 64 C64 sprites, hires or multicolor per
// sprite, with the shared multicolors, overlay, animation preview, import
// from a 320 × 200 picture, project files and ACME export. Ideas from
// Spritemate (Ingo Hinterding, MIT), MagSpriteEd (Magnar Harestad, MIT) and
// Sprite Studio 64 (Tobiasz Stamborski, MIT).

const (
	sprPencil = iota
	sprLine
	sprRect
	sprBox
	sprFill
	sprToolCount
)

var sprToolNames = [sprToolCount]string{"Pencil", "Line", "Rectangle", "Filled box", "Fill"}
var sprToolKeys = [sprToolCount]string{"D", "L", "R", "B", "F"}

// sprSlotNames: the drawing colors.
var sprSlotNames = [4]string{"Transparent", "Sprite color", "Multicolor 1", "Multicolor 2"}
var sprSlotRegs = [4]string{"$D021", "$D027+n", "$D025", "$D026"}

type sprState struct {
	bank *c64.SpriteBank
	cur  int
}

type sprEditor struct {
	open       bool
	bank       *c64.SpriteBank
	cur        int
	undo, redo []sprState
	tool       int
	slot       int // 0 transparent, 1 sprite color, 2 MC1, 3 MC2
	mirror     bool
	grid       bool
	overlayOn  bool
	overlay    int
	clip       *c64.Sprite
	path       string
	dirty      bool
	playing    bool
	playStart  int64
	thumbScrl  int
	// stroke
	drawing bool
	btnSlot int
	sx, sy  int
	lx, ly  int
	before  c64.Sprite
	canvas  gfx.Rect
	z       int // hires pixel size
	hx, hy  int // hovered pixel, -1 = none
}

func (a *App) openSprites() {
	a.commitOrCancelForHistory()
	a.menuOpen = -1
	if a.spr == nil {
		a.spr = &sprEditor{bank: c64.NewSpriteBank(), slot: 1, grid: true, overlay: 1}
	}
	a.spr.open = true
	a.setStatus("Sprite editor")
}

func (a *App) closeSprites() {
	a.spr.open = false
	a.modal = nil
	a.setStatus("Back to the picture")
}

func (e *sprEditor) sp() *c64.Sprite { return &e.bank.Sprites[e.cur] }

func (e *sprEditor) push() {
	e.undo = append(e.undo, sprState{e.bank.Clone(), e.cur})
	if len(e.undo) > 300 {
		e.undo = e.undo[1:]
	}
	e.redo = nil
	e.dirty = true
}

func (e *sprEditor) doUndo() {
	if n := len(e.undo); n > 0 {
		e.redo = append(e.redo, sprState{e.bank.Clone(), e.cur})
		s := e.undo[n-1]
		e.undo = e.undo[:n-1]
		e.bank, e.cur = s.bank, s.cur
		e.dirty = true
	}
}

func (e *sprEditor) doRedo() {
	if n := len(e.redo); n > 0 {
		e.undo = append(e.undo, sprState{e.bank.Clone(), e.cur})
		s := e.redo[n-1]
		e.redo = e.redo[:n-1]
		e.bank, e.cur = s.bank, s.cur
		e.dirty = true
	}
}

// edit runs a change of the current sprite with undo.
func (e *sprEditor) edit(f func(s *c64.Sprite)) {
	e.push()
	f(e.sp())
}

func (e *sprEditor) selectSprite(i int) {
	e.cur = clampI(i, 0, len(e.bank.Sprites)-1)
}

func (e *sprEditor) addSprite(dup bool) {
	if len(e.bank.Sprites) >= c64.MaxSprites {
		return
	}
	e.push()
	s := c64.Sprite{Multi: e.sp().Multi, Color: e.sp().Color}
	if dup {
		s = *e.sp()
	}
	b := e.bank
	b.Sprites = append(b.Sprites[:e.cur+1], append([]c64.Sprite{s}, b.Sprites[e.cur+1:]...)...)
	e.cur++
	b.ClampAnim()
}

func (e *sprEditor) deleteSprite() {
	e.push()
	b := e.bank
	if len(b.Sprites) == 1 {
		b.Sprites[0] = c64.Sprite{Multi: b.Sprites[0].Multi, Color: b.Sprites[0].Color}
		return
	}
	b.Sprites = append(b.Sprites[:e.cur], b.Sprites[e.cur+1:]...)
	e.cur = min(e.cur, len(b.Sprites)-1)
	b.ClampAnim()
}

func (e *sprEditor) moveSprite(d int) {
	j := e.cur + d
	if j < 0 || j >= len(e.bank.Sprites) {
		return
	}
	e.push()
	s := e.bank.Sprites
	s[e.cur], s[j] = s[j], s[e.cur]
	e.cur = j
}

// slotOK reports whether a slot can be used in the current sprite.
func (e *sprEditor) slotOK(slot int) bool { return e.sp().Multi || slot < 2 }

// --- input -----------------------------------------------------------------------

func (a *App) sprKeys() {
	e := a.spr
	for _, k := range a.in.Keys {
		m := k.Mods
		switch {
		case m.Ctrl && k.Key == 'Z' && !m.Shift:
			e.doUndo()
		case m.Ctrl && (k.Key == 'Y' || (k.Key == 'Z' && m.Shift)):
			e.doRedo()
		case m.Ctrl && k.Key == 'S':
			a.later(a.sprSave)
		case m.Ctrl && k.Key == 'O':
			a.later(a.sprOpen)
		case m.Ctrl && k.Key == 'C':
			c := *e.sp()
			e.clip = &c
		case m.Ctrl && k.Key == 'V':
			if e.clip != nil {
				e.edit(func(s *c64.Sprite) { *s = *e.clip })
			}
		case m.Alt && k.Key == KeyLeft:
			e.edit(func(s *c64.Sprite) { s.Shift(-1, 0) })
		case m.Alt && k.Key == KeyRight:
			e.edit(func(s *c64.Sprite) { s.Shift(1, 0) })
		case m.Alt && k.Key == KeyUp:
			e.edit(func(s *c64.Sprite) { s.Shift(0, -1) })
		case m.Alt && k.Key == KeyDown:
			e.edit(func(s *c64.Sprite) { s.Shift(0, 1) })
		case k.Key == KeyPgUp || k.Key == KeyLeft:
			e.selectSprite(e.cur - 1)
		case k.Key == KeyPgDn || k.Key == KeyRight:
			e.selectSprite(e.cur + 1)
		case k.Key == KeyDelete:
			e.edit(func(s *c64.Sprite) { s.Data = [c64.SpriteBytes]byte{} })
		case k.Key == KeySpace:
			e.togglePlay(a)
		case k.Key == KeyF1:
			a.openHelp("sprites")
		}
	}
	for _, c := range a.in.Chars {
		switch c {
		case '0', '1', '2', '3':
			if e.slotOK(int(c - '0')) {
				e.slot = int(c - '0')
			}
		case 'd', 'D':
			e.tool = sprPencil
		case 'l', 'L':
			e.tool = sprLine
		case 'r', 'R':
			e.tool = sprRect
		case 'b', 'B':
			e.tool = sprBox
		case 'f', 'F':
			e.tool = sprFill
		case 'g', 'G':
			e.grid = !e.grid
		case 'm', 'M':
			e.mirror = !e.mirror
		case 'o', 'O':
			e.overlayOn = !e.overlayOn
		case 'x', 'X':
			e.edit(func(s *c64.Sprite) { s.FlipH() })
		case 'y', 'Y':
			e.edit(func(s *c64.Sprite) { s.FlipV() })
		case 'i', 'I':
			e.edit(func(s *c64.Sprite) { s.Invert() })
		case 'c', 'C':
			e.edit(func(s *c64.Sprite) { s.Multi = !s.Multi })
			if !e.slotOK(e.slot) {
				e.slot = 1
			}
		case 'u':
			e.doUndo()
		case 'U':
			e.doRedo()
		case ',':
			e.selectSprite(e.cur - 1)
		case '.':
			e.selectSprite(e.cur + 1)
		}
	}
}

func (e *sprEditor) togglePlay(a *App) {
	e.playing = !e.playing
	e.playStart = a.now
}

// pixelAt returns the editable pixel under a screen position (clamped) and
// whether it is inside.
func (e *sprEditor) pixelAt(x, y int) (int, int, bool) {
	pw := e.sp().PixelW()
	sz := e.z * c64.SpriteW / pw
	px := (x - e.canvas.X) / sz
	py := (y - e.canvas.Y) / e.z
	in := x >= e.canvas.X && y >= e.canvas.Y && px < pw && py < c64.SpriteH
	return clampI(px, 0, pw-1), clampI(py, 0, c64.SpriteH-1), in
}

// plot sets a pixel, mirrored as well when mirror is on.
func (e *sprEditor) plot(s *c64.Sprite, x, y, slot int) {
	s.Set(x, y, slot)
	if e.mirror {
		s.Set(s.PixelW()-1-x, y, slot)
	}
}

func (e *sprEditor) shape(s *c64.Sprite, x0, y0, x1, y1, slot int) {
	switch e.tool {
	case sprLine:
		forLine(x0, y0, x1, y1, func(x, y int) { e.plot(s, x, y, slot) })
	case sprRect, sprBox:
		ax, bx := min(x0, x1), max(x0, x1)
		ay, by := min(y0, y1), max(y0, y1)
		for y := ay; y <= by; y++ {
			for x := ax; x <= bx; x++ {
				if e.tool == sprBox || x == ax || x == bx || y == ay || y == by {
					e.plot(s, x, y, slot)
				}
			}
		}
	}
}

// fill does a flood fill of the area with the same slot.
func (e *sprEditor) fill(s *c64.Sprite, x, y, slot int) {
	from := s.Get(x, y)
	if from == slot {
		return
	}
	w := s.PixelW()
	stack := [][2]int{{x, y}}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if p[0] < 0 || p[1] < 0 || p[0] >= w || p[1] >= c64.SpriteH || s.Get(p[0], p[1]) != from {
			continue
		}
		e.plot(s, p[0], p[1], slot)
		stack = append(stack, [2]int{p[0] + 1, p[1]}, [2]int{p[0] - 1, p[1]}, [2]int{p[0], p[1] + 1}, [2]int{p[0], p[1] - 1})
	}
}

func (a *App) sprCanvasInput() {
	e := a.spr
	in := &a.in
	x, y, inside := e.pixelAt(in.MX, in.MY)
	e.hx, e.hy = -1, -1
	if inside {
		e.hx, e.hy = x, y
	}
	if a.ui.blocked {
		return
	}
	s := e.sp()
	pressed := inside && (in.Pressed[0] || in.Pressed[1])
	if pressed && in.Mods.Alt {
		if v := s.Get(x, y); e.slotOK(v) {
			e.slot = v
		}
		return
	}
	if pressed {
		e.push()
		e.drawing = true
		e.btnSlot = e.slot
		if in.Pressed[1] {
			e.btnSlot = 0
		}
		e.sx, e.sy, e.lx, e.ly = x, y, x, y
		e.before = *s
		switch e.tool {
		case sprPencil:
			e.plot(s, x, y, e.btnSlot)
		case sprFill:
			e.fill(s, x, y, e.btnSlot)
			e.drawing = false
		default:
			e.shape(s, x, y, x, y, e.btnSlot)
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
	case sprPencil:
		if in.Mods.Shift { // straight line from the start
			*s = e.before
			e.tool = sprLine
			e.shape(s, e.sx, e.sy, x, y, e.btnSlot)
			e.tool = sprPencil
			return
		}
		if x != e.lx || y != e.ly {
			forLine(e.lx, e.ly, x, y, func(px, py int) { e.plot(s, px, py, e.btnSlot) })
			e.lx, e.ly = x, y
		}
	default:
		*s = e.before
		e.shape(s, e.sx, e.sy, x, y, e.btnSlot)
	}
}

// --- drawing -----------------------------------------------------------------------

func (a *App) sprFrame() {
	e := a.spr
	a.toolFrameStart(a.sprKeys)
	if !e.open {
		return
	}
	a.ui.blocked = a.modal != nil
	e.cur = clampI(e.cur, 0, len(e.bank.Sprites)-1)
	if !e.slotOK(e.slot) {
		e.slot = 1
	}
	m := a.img
	m.Fill(gfx.Rect{X: 0, Y: 0, W: a.W, H: a.H}, a.th.Bg)
	area := gfx.Rect{X: toolLeftW + 10, Y: toolTopH + 10, W: a.W - toolLeftW - toolRightW - 20, H: a.H - toolTopH - toolStatusH - 20}
	prevH := 170
	ca := gfx.Rect{X: area.X, Y: area.Y, W: area.W, H: area.H - prevH - 10}
	e.z = max(2, min(ca.W/c64.SpriteW, ca.H/c64.SpriteH))
	e.canvas = gfx.Rect{X: ca.X + (ca.W-c64.SpriteW*e.z)/2, Y: ca.Y + (ca.H-c64.SpriteH*e.z)/2, W: c64.SpriteW * e.z, H: c64.SpriteH * e.z}
	a.sprCanvasInput()
	a.sprDrawCanvas()
	a.sprDrawPreview(gfx.Rect{X: area.X, Y: area.Bottom() - prevH, W: area.W, H: prevH})
	a.toolTopBar("Sprite editor", []toolButton{
		{"sp_new", "New", "Empty sprite bank", a.sprNew},
		{"sp_imp", "Import …", "Take sprites from the multicolor picture or a 320 × 200 image", a.sprOpenImport},
		{"sp_open", "Open …", "Open a sprite project (.c64sprites) – Ctrl+O", a.sprOpen},
		{"sp_save", "Save …", "Save the sprite project (.c64sprites) – Ctrl+S", a.sprSave},
		{},
		{"sp_asm", "Export ASM …", "Source code for the ACME assembler with all colors and settings", func() { a.sprExport("asm") }},
		{"sp_bin", "Export BIN …", "Raw sprite data, 64 bytes per sprite", func() { a.sprExport("bin") }},
		{"sp_png", "Export PNG …", "All sprites as a picture sheet", func() { a.sprExport("png") }},
		{},
		{"sp_undo", "Undo", "Ctrl+Z", e.doUndo},
		{"sp_redo", "Redo", "Ctrl+Y", e.doRedo},
		{"sp_help", "Help", "F1", func() { a.openHelp("sprites") }},
	}, "sp_close", "Close sprite editor", a.closeSprites)
	a.sprDrawLeft(gfx.Rect{X: 0, Y: toolTopH, W: toolLeftW, H: a.H - toolTopH - toolStatusH})
	a.sprDrawRight(gfx.Rect{X: a.W - toolRightW, Y: toolTopH, W: toolRightW, H: a.H - toolTopH - toolStatusH})
	s := e.sp()
	status := fmt.Sprintf("Sprite %d of %d  ·  %s", e.cur, len(e.bank.Sprites), map[bool]string{true: "multicolor 12 × 21", false: "hires 24 × 21"}[s.Multi])
	if e.hx >= 0 {
		v := s.Get(e.hx, e.hy)
		status += fmt.Sprintf("  ·  x %d, y %d  ·  %s", e.hx, e.hy, sprSlotNames[v])
	}
	a.toolStatus(status, "Alt+click: pick · Shift: straight line · right button: transparent · F1: help")
	a.toolFrameEnd()
}

// spriteColor returns the RGB of a slot of sprite s (ok = false: transparent).
func (a *App) spriteColor(b *c64.SpriteBank, s *c64.Sprite, slot int) (uint32, bool) {
	c := b.SlotColor(s, slot)
	if c < 0 {
		return 0, false
	}
	return a.pal.Colors[c], true
}

// drawSprite draws a sprite at (x, y): one hires pixel = zx × zy screen pixels.
func (a *App) drawSprite(b *c64.SpriteBank, s *c64.Sprite, x, y, zx, zy int, alpha int) {
	m := a.img
	for py := 0; py < c64.SpriteH; py++ {
		for px := 0; px < c64.SpriteW; px++ {
			c := b.ColorAt(s, px, py)
			if c < 0 {
				continue
			}
			r := gfx.Rect{X: x + px*zx, Y: y + py*zy, W: zx, H: zy}
			if alpha >= 255 {
				m.Fill(r, a.pal.Colors[c])
			} else {
				m.FillA(r, a.pal.Colors[c], alpha)
			}
		}
	}
}

func (a *App) sprDrawCanvas() {
	e := a.spr
	m := a.img
	b := e.bank
	s := e.sp()
	cr := e.canvas
	z := e.z
	m.FillA(gfx.Rect{X: cr.Right(), Y: cr.Y + 6, W: 6, H: cr.H}, a.th.Shadow, 80)
	m.FillA(gfx.Rect{X: cr.X + 6, Y: cr.Bottom(), W: cr.W, H: 6}, a.th.Shadow, 80)
	m.Fill(cr, a.pal.Colors[b.BG])
	// overlay sprite, faded
	if o := e.overlay; e.overlayOn && o >= 0 && o < len(b.Sprites) && o != e.cur {
		a.drawSprite(b, &b.Sprites[o], cr.X, cr.Y, z, z, 110)
	}
	pw := s.PixelW()
	sz := z * c64.SpriteW / pw
	for py := 0; py < c64.SpriteH; py++ {
		for px := 0; px < pw; px++ {
			if c, ok := a.spriteColor(b, s, s.Get(px, py)); ok {
				m.Fill(gfx.Rect{X: cr.X + px*sz, Y: cr.Y + py*z, W: sz, H: z}, c)
			}
		}
	}
	if e.grid {
		for px := 1; px < pw; px++ {
			x := cr.X + px*sz
			al := 60
			if (px*c64.SpriteW/pw)%8 == 0 {
				al = 150
			}
			for y := cr.Y; y < cr.Bottom(); y++ {
				m.BlendPix(x, y, 0x808080, al)
			}
		}
		for py := 1; py < c64.SpriteH; py++ {
			y := cr.Y + py*z
			for x := cr.X; x < cr.Right(); x++ {
				m.BlendPix(x, y, 0x808080, 60)
			}
		}
	}
	if e.mirror {
		x := cr.X + cr.W/2
		for y := cr.Y; y < cr.Bottom(); y += 3 {
			m.Fill(gfx.Rect{X: x - 1, Y: y, W: 2, H: 2}, a.th.Accent)
		}
	}
	m.Frame(cr.Inset(-1), a.th.Border)
	if e.hx >= 0 && a.modal == nil {
		r := gfx.Rect{X: cr.X + e.hx*sz, Y: cr.Y + e.hy*z, W: sz, H: z}
		dashedFrame(m, r, 0xFFFFFF, 0, 0)
	}
}

// animFrames returns the sprite numbers of the animation.
func (e *sprEditor) animFrames() []int {
	b := e.bank
	b.ClampAnim()
	var f []int
	for i := b.AnimFrom; i <= b.AnimTo; i++ {
		f = append(f, i)
	}
	if b.AnimPingPong {
		for i := b.AnimTo - 1; i > b.AnimFrom; i-- {
			f = append(f, i)
		}
	}
	return f
}

// drawSpriteComposite draws a sprite with its overlay as on the C64 (lower
// sprite numbers are in front) and the expand flags.
func (a *App) drawSpriteComposite(n, x, y, z int) {
	e := a.spr
	b := e.bank
	s := &b.Sprites[n]
	draw := func(sp *c64.Sprite) {
		zx, zy := z, z
		if sp.ExpandX {
			zx *= 2
		}
		if sp.ExpandY {
			zy *= 2
		}
		a.drawSprite(b, sp, x, y, zx, zy, 255)
	}
	o := e.overlay
	if e.overlayOn && o >= 0 && o < len(b.Sprites) && o != n {
		if o > n {
			draw(&b.Sprites[o])
			draw(s)
		} else {
			draw(s)
			draw(&b.Sprites[o])
		}
		return
	}
	draw(s)
}

func (a *App) sprDrawPreview(r gfx.Rect) {
	e := a.spr
	u := &a.ui
	m := a.img
	th := a.th
	b := e.bank
	m.Fill(r, th.Panel)
	m.Frame(r, th.Border)
	x := r.X + 10
	y := r.Y + 8
	m.Text(font.UIB, x, y, "Preview", th.Text)
	// 1× and 2× (with expand)
	box := gfx.Rect{X: x, Y: y + 22, W: 24 + 96 + 20, H: 84 + 10}
	m.Fill(box, a.pal.Colors[b.BG])
	a.drawSpriteComposite(e.cur, box.X+5, box.Y+5, 1)
	a.drawSpriteComposite(e.cur, box.X+39, box.Y+5, 2)
	m.Frame(box.Inset(-1), th.Border)
	m.Text(font.UI, x, box.Bottom()+6, "1× and 2× (expand X/Y)", th.TextDim)
	// animation
	ax := box.Right() + 24
	m.Text(font.UIB, ax, y, "Animation", th.Text)
	ab := gfx.Rect{X: ax, Y: y + 22, W: 96 + 10, H: 84 + 10}
	m.Fill(ab, a.pal.Colors[b.BG])
	frames := e.animFrames()
	fi := 0
	if e.playing && len(frames) > 0 {
		fi = int((a.now-e.playStart)*int64(b.AnimFPS)/1000) % len(frames)
		a.needAnim = true
	}
	if len(frames) > 0 {
		a.drawSpriteComposite(frames[fi], ab.X+5, ab.Y+5, 2)
	}
	m.Frame(ab.Inset(-1), th.Border)
	cx := ab.Right() + 16
	cw := min(260, r.Right()-cx-10)
	cy := y + 22
	lbl := func(s string) { m.Text(font.UI, cx, cy+5, s, th.Text) }
	n := len(b.Sprites)
	lbl("From sprite")
	b.AnimFrom = u.Spinner("sp_af", gfx.Rect{X: cx + 100, Y: cy, W: cw - 100, H: 22}, b.AnimFrom, 0, n-1, 1, "")
	cy += 26
	lbl("To sprite")
	b.AnimTo = u.Spinner("sp_at", gfx.Rect{X: cx + 100, Y: cy, W: cw - 100, H: 22}, b.AnimTo, 0, n-1, 1, "")
	cy += 26
	lbl("Speed")
	b.AnimFPS = u.Spinner("sp_fps", gfx.Rect{X: cx + 100, Y: cy, W: cw - 100, H: 22}, b.AnimFPS, 1, 50, 1, " fps")
	cy += 26
	b.AnimPingPong = u.Checkbox("sp_pp", gfx.Rect{X: cx, Y: cy, W: 110, H: 22}, "Ping-pong", b.AnimPingPong)
	label := "Play (Space)"
	if e.playing {
		label = "Stop (Space)"
	}
	if u.ButtonEx("sp_play", gfx.Rect{X: cx + 110, Y: cy, W: cw - 110, H: 22}, label, e.playing, true) {
		e.togglePlay(a)
	}
	if e.playing && len(frames) > 0 {
		m.Text(font.UI, ab.X, ab.Bottom()+6, fmt.Sprintf("Sprite %d", frames[fi]), th.TextDim)
	}
}

func (a *App) sprDrawLeft(r gfx.Rect) {
	e := a.spr
	u := &a.ui
	m := a.img
	th := a.th
	a.toolPanel(r, true)
	x, w := r.X+10, r.W-20
	y := r.Y + 8
	m.Text(font.UIB, x, y, "Tools", th.Text)
	y += 22
	for i := 0; i < sprToolCount; i++ {
		br := gfx.Rect{X: x, Y: y, W: w, H: 24}
		if u.ButtonEx("sp_t"+itoa(i), br, sprToolKeys[i]+"  "+sprToolNames[i], e.tool == i, true) {
			e.tool = i
		}
		y += 27
	}
	y += 2
	e.mirror = u.Checkbox("sp_mir", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Mirror left/right (M)", e.mirror)
	y += 24
	e.grid = u.Checkbox("sp_grid", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Grid (G)", e.grid)
	y += 24
	e.overlayOn = u.Checkbox("sp_ov", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Overlay sprite (O)", e.overlayOn)
	y += 24
	if e.overlayOn {
		e.overlay = u.Spinner("sp_ovn", gfx.Rect{X: x + 20, Y: y, W: w - 20, H: 22}, clampI(e.overlay, 0, len(e.bank.Sprites)-1), 0, len(e.bank.Sprites)-1, 1, "")
		y += 28
	}
	y += 4
	s := e.sp()
	m.Text(font.UIB, x, y, "Sprite "+itoa(e.cur), th.Text)
	y += 22
	bw := (w - 4) / 2
	for i, n := range []string{"Hires", "Multicolor"} {
		if u.ButtonEx("sp_mode"+itoa(i), gfx.Rect{X: x + i*(bw+4), Y: y, W: bw, H: 22}, n, s.Multi == (i == 1), true) && s.Multi != (i == 1) {
			e.edit(func(s *c64.Sprite) { s.Multi = i == 1 })
		}
	}
	u.Tip("sp_mode0", gfx.Rect{X: x, Y: y, W: bw, H: 22}, "C: switch (the bits stay, as on the C64)")
	y += 28
	if v := u.Checkbox("sp_ex", gfx.Rect{X: x, Y: y, W: bw, H: 20}, "Expand X", s.ExpandX); v != s.ExpandX {
		e.edit(func(s *c64.Sprite) { s.ExpandX = v })
	}
	if v := u.Checkbox("sp_ey", gfx.Rect{X: x + bw + 4, Y: y, W: bw, H: 20}, "Expand Y", s.ExpandY); v != s.ExpandY {
		e.edit(func(s *c64.Sprite) { s.ExpandY = v })
	}
	y += 30
	m.Text(font.UIB, x, y, "Edit", th.Text)
	y += 22
	ops := []struct {
		id, label, tip string
		f              func()
		ok             bool
	}{
		{"sp_fh", "Flip ↔", "X", func() { e.edit(func(s *c64.Sprite) { s.FlipH() }) }, true},
		{"sp_fv", "Flip ↕", "Y", func() { e.edit(func(s *c64.Sprite) { s.FlipV() }) }, true},
		{"sp_sl", "Shift ←", "Alt+←", func() { e.edit(func(s *c64.Sprite) { s.Shift(-1, 0) }) }, true},
		{"sp_sr", "Shift →", "Alt+→", func() { e.edit(func(s *c64.Sprite) { s.Shift(1, 0) }) }, true},
		{"sp_su", "Shift ↑", "Alt+↑", func() { e.edit(func(s *c64.Sprite) { s.Shift(0, -1) }) }, true},
		{"sp_sd", "Shift ↓", "Alt+↓", func() { e.edit(func(s *c64.Sprite) { s.Shift(0, 1) }) }, true},
		{"sp_inv", "Invert", "I", func() { e.edit(func(s *c64.Sprite) { s.Invert() }) }, true},
		{"sp_clr", "Clear", "Del", func() { e.edit(func(s *c64.Sprite) { s.Data = [c64.SpriteBytes]byte{} }) }, true},
		{"sp_cp", "Copy", "Ctrl+C", func() { c := *e.sp(); e.clip = &c }, true},
		{"sp_pa", "Paste", "Ctrl+V", func() { e.edit(func(s *c64.Sprite) { *s = *e.clip }) }, e.clip != nil},
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
	if u.ButtonEx("sp_brush", br, "Use as brush", false, s.Multi || a.hires()) {
		a.later(a.sprUseAsBrush)
	}
	u.Tip("sp_brush", br, "Paint with this sprite in the picture (hires sprites only in hires pictures)")
	y += 32
	if e.path != "" {
		for _, ln := range wrapText(font.UI, filepath.Base(e.path), w) {
			m.Text(font.UI, x, y, ln, th.TextDim)
			y += font.UI.Height + 2
		}
	}
}

func (a *App) sprDrawRight(r gfx.Rect) {
	e := a.spr
	u := &a.ui
	m := a.img
	th := a.th
	b := e.bank
	a.toolPanel(r, false)
	x, w := r.X+12, r.W-24
	y := r.Y + 8
	m.Text(font.UIB, x, y, "Sprites", th.Text)
	m.TextR(font.UI, gfx.Rect{X: x, Y: y, W: w, H: 16}, fmt.Sprintf("%d / %d", len(b.Sprites), c64.MaxSprites), th.TextDim)
	y += 22
	bw := (w - 8) / 3
	full := len(b.Sprites) >= c64.MaxSprites
	if u.ButtonEx("sp_add", gfx.Rect{X: x, Y: y, W: bw, H: 22}, "Add", false, !full) {
		e.addSprite(false)
	}
	if u.ButtonEx("sp_dup", gfx.Rect{X: x + bw + 4, Y: y, W: bw, H: 22}, "Duplicate", false, !full) {
		e.addSprite(true)
	}
	if u.Button("sp_del", gfx.Rect{X: x + 2*(bw+4), Y: y, W: bw, H: 22}, "Delete") {
		e.deleteSprite()
	}
	y += 26
	hw := (w - 4) / 2
	if u.ButtonEx("sp_ml", gfx.Rect{X: x, Y: y, W: hw, H: 22}, "← Move", false, e.cur > 0) {
		e.moveSprite(-1)
	}
	if u.ButtonEx("sp_mr", gfx.Rect{X: x + hw + 4, Y: y, W: hw, H: 22}, "Move →", false, e.cur < len(b.Sprites)-1) {
		e.moveSprite(1)
	}
	y += 30
	// thumbnails, 4 per row
	colorsH := 4*26 + 22 + 70
	cols := 4
	tw := w / cols
	th2 := 42 + 14
	rows := (len(b.Sprites) + cols - 1) / cols
	gh := max(th2, min(rows*th2, r.Bottom()-y-colorsH-16))
	gr := gfx.Rect{X: x, Y: y, W: w, H: gh}
	maxScroll := max(0, rows*th2-gh)
	if u.hover(gr) && a.in.Wheel != 0 && !u.wheelUsed {
		e.thumbScrl -= a.in.Wheel * th2
		u.wheelUsed = true
	}
	e.thumbScrl = clampI(e.thumbScrl, 0, maxScroll)
	m.Fill(gr, th.Popup)
	old := m.SetClip(gr.Intersect(m.Clip))
	for i := range b.Sprites {
		cx := gr.X + (i%cols)*tw
		cy := gr.Y + (i/cols)*th2 - e.thumbScrl
		if cy+th2 < gr.Y || cy > gr.Bottom() {
			continue
		}
		br := gfx.Rect{X: cx + 2, Y: cy + 2, W: tw - 4, H: th2 - 4}
		inner := gfx.Rect{X: br.X + (br.W-48)/2, Y: br.Y + 1, W: 48, H: 42}
		m.Fill(inner, a.pal.Colors[b.BG])
		a.drawSprite(b, &b.Sprites[i], inner.X, inner.Y, 2, 2, 255)
		m.Text(font.UI, br.X+2, inner.Bottom()-2, itoa(i), th.TextDim)
		if i == e.cur {
			m.Frame(br, th.Accent)
			m.Frame(br.Inset(1), th.Accent)
		}
		if b.AnimFrom <= i && i <= b.AnimTo && b.AnimTo > b.AnimFrom {
			m.Fill(gfx.Rect{X: br.Right() - 7, Y: br.Bottom() - 7, W: 5, H: 5}, th.Accent)
		}
		if gr.Contains(a.in.MX, a.in.MY) {
			if c, _ := u.press(fmt.Sprintf("sp_th%d", i), br, 0); c {
				e.selectSprite(i)
			}
		}
	}
	m.SetClip(old)
	m.Frame(gr.Inset(-1), th.Border)
	y += gh + 10
	// colors
	m.Text(font.UIB, x, y, "Colors", th.Text)
	m.TextR(font.UI, gfx.Rect{X: x, Y: y, W: w, H: 16}, "0–3: choose", th.TextDim)
	y += 20
	s := e.sp()
	for slot := 0; slot < 4; slot++ {
		br := gfx.Rect{X: x, Y: y, W: w, H: 23}
		ok := e.slotOK(slot)
		c := int(b.BG)
		if slot > 0 {
			c = b.SlotColor(s, slot)
		}
		label := fmt.Sprintf("%d  %s", slot, sprSlotNames[slot])
		if slot == 0 {
			label = "0  Transparent (background)"
		}
		if u.ButtonEx("sp_slot"+itoa(slot), br, "", e.slot == slot, ok) && ok {
			e.slot = slot
		}
		sw := gfx.Rect{X: br.X + 4, Y: br.Y + 4, W: 26, H: br.H - 8}
		m.Fill(sw, a.pal.Colors[c])
		m.Frame(sw, th.BtnBorder)
		tc := th.Text
		if !ok {
			tc = th.TextDim
		}
		m.TextL(font.UI, gfx.Rect{X: br.X + 38, Y: br.Y, W: br.W - 40, H: br.H}, label, tc)
		m.TextR(font.UI, gfx.Rect{X: br.X, Y: br.Y, W: br.W - 6, H: br.H}, sprSlotRegs[slot], th.TextDim)
		y += 26
	}
	y += 4
	cur := int(b.BG)
	if e.slot > 0 {
		cur = b.SlotColor(s, e.slot)
	}
	a.toolPalette("sp_pal", x, y, w, cur, func(c int) {
		e.push()
		switch e.slot {
		case 0:
			b.BG = uint8(c)
		case 1:
			e.sp().Color = uint8(c)
		case 2:
			b.MC1 = uint8(c)
		case 3:
			b.MC2 = uint8(c)
		}
	})
}

// sprUseAsBrush hands the current multicolor sprite to the painter as brush.
func (a *App) sprUseAsBrush() {
	e := a.spr
	s := e.sp()
	if !s.Multi && !a.hires() {
		return
	}
	// multicolor picture: 12 wide pixels; hires picture: 24 pixels
	w := 24 / asp
	br := &CustomBrush{W: w, H: c64.SpriteH, Pix: make([]int8, w*c64.SpriteH)}
	for y := 0; y < c64.SpriteH; y++ {
		for x := 0; x < w; x++ {
			br.Pix[y*w+x] = int8(e.bank.ColorAt(s, x*asp, y))
		}
	}
	a.closeSprites()
	a.useBrush(br, true, fmt.Sprintf("Sprite %d as brush – click to place it", e.cur))
}

// --- files -------------------------------------------------------------------------

var sprFilters = []Filter{{Name: "C64 DPainter sprites (*.c64sprites)", Pattern: "*.c64sprites"}}

func (a *App) sprNew() {
	e := a.spr
	e.push()
	e.bank = c64.NewSpriteBank()
	e.cur = 0
	e.path = ""
}

func (a *App) sprName() string {
	if a.spr.path != "" {
		return strings.TrimSuffix(filepath.Base(a.spr.path), filepath.Ext(a.spr.path))
	}
	return "sprites"
}

func (a *App) sprOpen() {
	p, ok := a.host.OpenFile("Open sprites", sprFilters, a.workDir())
	if !ok {
		return
	}
	data, err := a.host.ReadFile(p)
	if err != nil {
		a.host.Info(AppName, "Cannot open the file:\n"+err.Error())
		return
	}
	a.sprLoad(p, data)
}

func (a *App) sprLoad(p string, data []byte) {
	b, err := c64.LoadSpriteBank(data)
	if err != nil {
		a.host.Info(AppName, "Cannot open the file:\n"+err.Error())
		return
	}
	e := a.spr
	e.push()
	e.bank, e.cur, e.path, e.dirty = b, 0, p, false
	a.addRecent(p)
}

func (a *App) sprSave() {
	p, ok := a.host.SaveFile("Save sprites", sprFilters, a.workDir(), a.sprName()+".c64sprites", "c64sprites")
	if !ok {
		return
	}
	p = FixDoubleExt(p)
	data, err := a.spr.bank.Marshal()
	if err == nil {
		err = a.host.WriteFile(p, data)
	}
	if err != nil {
		a.host.Info(AppName, "Saving failed:\n"+err.Error())
		return
	}
	a.spr.path = p
	a.spr.dirty = false
	a.setStatus("Saved: " + filepath.Base(p))
}

// spriteSheet renders all sprites as a picture (8 per row, 1 pixel gap).
func (a *App) spriteSheet() *gfx.Image {
	b := a.spr.bank
	cols := min(8, len(b.Sprites))
	rows := (len(b.Sprites) + 7) / 8
	img := gfx.New(cols*(c64.SpriteW+1)+1, rows*(c64.SpriteH+1)+1)
	img.Fill(gfx.Rect{X: 0, Y: 0, W: img.W, H: img.H}, a.pal.Colors[b.BG])
	for i := range b.Sprites {
		s := &b.Sprites[i]
		ox, oy := 1+(i%8)*(c64.SpriteW+1), 1+(i/8)*(c64.SpriteH+1)
		for y := 0; y < c64.SpriteH; y++ {
			for x := 0; x < c64.SpriteW; x++ {
				if c := b.ColorAt(s, x, y); c >= 0 {
					img.Set(ox+x, oy+y, a.pal.Colors[c])
				}
			}
		}
	}
	return img
}

func (a *App) sprExport(kind string) {
	b := a.spr.bank
	name := a.sprName()
	var f []Filter
	switch kind {
	case "asm":
		f = []Filter{{Name: "ACME assembler source (*.asm)", Pattern: "*.asm"}}
	case "bin":
		f = []Filter{{Name: "Raw sprite data (*.bin)", Pattern: "*.bin"}}
	case "png":
		f = []Filter{{Name: "PNG image (*.png)", Pattern: "*.png"}}
	}
	p, ok := a.host.SaveFile("Export sprites", f, a.workDir(), name+"."+kind, kind)
	if !ok {
		return
	}
	p = FixDoubleExt(p)
	base := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	var data []byte
	var err error
	switch kind {
	case "asm":
		data = []byte(b.ACME(base, 0x2000))
	case "bin":
		data = b.RawData()
	case "png":
		data, err = encodePNG(a.spriteSheet())
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

type sprImportDialog struct {
	src      []int8
	name     string
	o        c64.SpriteImport
	snap     bool
	appendTo bool
	result   []c64.Sprite
	bg       uint8
	mc1, mc2 uint8
	key      string
}

func (a *App) sprOpenImport() {
	d := &sprImportDialog{src: a.pictureIndexes(), name: "the multicolor picture", snap: true,
		o: c64.SpriteImport{Cols: 1, Rows: 1, Multi: true, BG: int(a.pic.BG)}}
	a.modal = d
}

func (d *sprImportDialog) Title() string          { return "Import sprites" }
func (d *sprImportDialog) Size(a *App) (int, int) { return 1000, min(660, a.H-20) }
func (d *sprImportDialog) Key(a *App, k KeyEvent) {
	switch k.Key {
	case KeyEnter:
		d.apply(a)
	case KeyLeft:
		d.o.X = max(0, d.o.X-d.stepX())
	case KeyRight:
		d.o.X = min(319, d.o.X+d.stepX())
	case KeyUp:
		d.o.Y = max(0, d.o.Y-1)
	case KeyDown:
		d.o.Y = min(199, d.o.Y+1)
	}
}

func (d *sprImportDialog) stepX() int {
	if d.o.Multi {
		return 2
	}
	return 1
}

// count returns how many sprites will be imported.
func (d *sprImportDialog) count(a *App) int {
	n := d.o.Cols * d.o.Rows
	e := a.spr
	start := e.cur
	if d.appendTo {
		start = len(e.bank.Sprites)
	}
	return max(0, min(n, c64.MaxSprites-start))
}

func (d *sprImportDialog) apply(a *App) {
	n := d.count(a)
	if n == 0 || len(d.result) == 0 {
		return
	}
	e := a.spr
	e.push()
	b := e.bank
	start := e.cur
	if d.appendTo {
		start = len(b.Sprites)
	}
	for i := 0; i < n; i++ {
		if start+i < len(b.Sprites) {
			b.Sprites[start+i] = d.result[i]
		} else {
			b.Sprites = append(b.Sprites, d.result[i])
		}
	}
	b.BG, b.MC1, b.MC2 = d.bg, d.mc1, d.mc2
	e.cur = start
	b.ClampAnim()
	a.closeModal(false)
	a.setStatus(fmt.Sprintf("Imported %d sprites from %s", n, d.name))
}

func (d *sprImportDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	e := a.spr
	if d.o.Multi {
		d.o.X &^= 1
	}
	key := fmt.Sprintf("%+v/%p/%d/%d/%d", d.o, d.src, a.palIdx, e.bank.MC1, e.bank.MC2)
	if key != d.key {
		d.key = key
		pr := a.pal.Colors
		dist := func(i, j int) int {
			ci, cj := pr[i&15], pr[j&15]
			return c64.ColorDist(int(ci>>16&255), int(ci>>8&255), int(ci&255), int(cj>>16&255), int(cj>>8&255), int(cj&255))
		}
		d.result, d.bg, d.mc1, d.mc2 = c64.ImportSprites(d.src, e.bank, d.o, dist)
	}
	// source at 2×
	pr := gfx.Rect{X: r.X, Y: r.Y, W: 640, H: 400}
	a.drawIndexPicture(d.src, pr.X, pr.Y, 2)
	old := m.SetClip(pr.Intersect(m.Clip))
	n := d.o.Cols * d.o.Rows
	for i := 0; i < n; i++ {
		sx, sy := d.o.Rect(i)
		fr := gfx.Rect{X: pr.X + sx*2, Y: pr.Y + sy*2, W: c64.SpriteW * 2, H: c64.SpriteH * 2}
		col := uint32(0xFFFFFF)
		if i >= d.count(a) {
			col = 0xFF4040
		}
		dashedFrame(m, fr, col, 0, int(a.now/80)%8)
	}
	m.SetClip(old)
	a.needAnim = true
	m.Frame(pr.Inset(-1), th.Border)
	// click or drag on the source: position
	if c, held := u.press("si_src", pr, 0); c || held {
		px, py := (a.in.MX-pr.X)/2, (a.in.MY-pr.Y)/2
		if d.snap {
			px, py = px/c64.SpriteW*c64.SpriteW, py/c64.SpriteH*c64.SpriteH
		}
		d.o.X, d.o.Y = clampI(px, 0, 319), clampI(py, 0, 199)
	}
	m.Text(font.UI, r.X, pr.Bottom()+8, "Source: "+d.name+"   ·   click or drag to place the frame, arrow keys move it", th.TextDim)
	if u.Button("si_s0", gfx.Rect{X: r.X, Y: pr.Bottom() + 28, W: 200, H: 24}, "Use the multicolor picture") {
		d.src, d.name, d.key = a.pictureIndexes(), "the multicolor picture", ""
		d.o.BG = int(a.pic.BG)
	}
	if u.Button("si_s1", gfx.Rect{X: r.X + 206, Y: pr.Bottom() + 28, W: 200, H: 24}, "Load an image (320×200) …") {
		a.later(func() {
			if px, name, ok := a.loadIndexImage("Image with sprites"); ok {
				d.src, d.name, d.key = px, name, ""
				d.o.BG = -1
			}
		})
	}
	// result
	ry := pr.Bottom() + 62
	m.Text(font.UIB, r.X, ry, "Result", th.Text)
	ry += 20
	rx := r.X
	for i, s := range d.result {
		if i >= d.count(a) || rx+52 > pr.Right() {
			break
		}
		box := gfx.Rect{X: rx, Y: ry, W: 52, H: 46}
		m.Fill(box, a.pal.Colors[d.bg])
		tmp := &c64.SpriteBank{BG: d.bg, MC1: d.mc1, MC2: d.mc2}
		a.drawSprite(tmp, &s, box.X+2, box.Y+2, 2, 2, 255)
		m.Frame(box, th.Border)
		rx += 56
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
	b2i := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	hdr("Sprite format")
	if v := row("si_m", []string{"Hires 24×21", "Multicolor 12×21"}, b2i(d.o.Multi)); v >= 0 {
		d.o.Multi = v == 1
	}
	hdr("Position and raster")
	lbl := func(s string) { m.Text(font.UI, x, y+5, s, th.Text) }
	half := w/2 - 2
	lbl("X")
	d.o.X = u.Spinner("si_x", gfx.Rect{X: x + 24, Y: y, W: half - 24, H: 22}, d.o.X, 0, 319, d.stepX(), "")
	m.Text(font.UI, x+half+4, y+5, "Y", th.Text)
	d.o.Y = u.Spinner("si_y", gfx.Rect{X: x + half + 28, Y: y, W: half - 24, H: 22}, d.o.Y, 0, 199, 1, "")
	y += 28
	lbl("Columns")
	d.o.Cols = u.Spinner("si_c", gfx.Rect{X: x + 90, Y: y, W: w - 90, H: 22}, d.o.Cols, 1, 13, 1, "")
	y += 26
	lbl("Rows")
	d.o.Rows = u.Spinner("si_r", gfx.Rect{X: x + 90, Y: y, W: w - 90, H: 22}, d.o.Rows, 1, 9, 1, "")
	y += 26
	lbl("Gap X / Y")
	d.o.GapX = u.Spinner("si_gx", gfx.Rect{X: x + 90, Y: y, W: (w - 94) / 2, H: 22}, d.o.GapX, 0, 32, d.stepX(), "")
	d.o.GapY = u.Spinner("si_gy", gfx.Rect{X: x + 94 + (w-94)/2, Y: y, W: (w - 94) / 2, H: 22}, d.o.GapY, 0, 32, 1, "")
	y += 28
	d.snap = u.Checkbox("si_snap", gfx.Rect{X: x, Y: y, W: w, H: 20}, "Clicks snap to the sprite grid (24×21)", d.snap)
	y += 28
	hdr("Background (transparent)")
	a.toolColorChoice("si_bg", x, y, w, d.o.BG, "Most frequent color", func(c int) { d.o.BG = c })
	y += 30
	if d.o.Multi {
		hdr("Multicolor 1 and 2 (shared)")
		if v := row("si_sh", []string{"Find automatically", "Keep current"}, b2i(d.o.KeepShared)); v >= 0 {
			d.o.KeepShared = v == 1
		}
		for i, c := range []uint8{d.mc1, d.mc2} {
			sw := gfx.Rect{X: x + i*(w/2), Y: y, W: 22, H: 18}
			m.Fill(sw, a.pal.Colors[c])
			m.Frame(sw, th.BtnBorder)
			m.Text(font.UI, sw.Right()+6, y+2, fmt.Sprintf("MC%d %s", i+1, c64.ColorNames[c]), th.Text)
		}
		y += 26
	}
	hdr("Target")
	if v := row("si_t", []string{fmt.Sprintf("From sprite %d", e.cur), "Append"}, b2i(d.appendTo)); v >= 0 {
		d.appendTo = v == 1
	}
	if c := d.count(a); c < n {
		m.Text(font.UI, x, y, fmt.Sprintf("Only %d of %d fit (max. %d sprites)", c, n, c64.MaxSprites), th.TextDim)
	}
	by := r.Bottom() - 28
	if u.Button("si_cancel", gfx.Rect{X: r.Right() - 250, Y: by, W: 120, H: 28}, "Cancel") {
		a.closeModal(true)
		return
	}
	if u.ButtonEx("si_ok", gfx.Rect{X: r.Right() - 120, Y: by, W: 120, H: 28}, "Import", true, d.count(a) > 0) {
		d.apply(a)
	}
}
