package app

import (
	"fmt"
	"path/filepath"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- Side border sprites (experimental, multicolor only) -------------------------------
//
// Layer → "Side border sprites" opens the side border: left and right of the
// picture there is a strip of 12 × 200 multicolor pixels made of sprites
// (10 sprites of 12 × 21 per side). Every 12 × 21 block has its own color,
// multicolor 1 and 2 are shared; transparent pixels show the background.
// The tools that paint pixel by pixel work there (dots, freehand, lines,
// shapes, fill, airbrush, eraser, color picker).

func (a *App) sideOn() bool { return a.pic.SideActive() }

// stripAt reports whether a picture position lies in a side strip.
func (a *App) stripAt(x, y int) (side, sx int, ok bool) {
	if !a.sideOn() || y < 0 || y >= c64.H {
		return 0, 0, false
	}
	return c64.SideAt(x)
}

// paintable: inside the picture or a side strip.
func (a *App) paintable(x, y int) bool {
	if x >= 0 && y >= 0 && x < picW && y < c64.H {
		return true
	}
	_, _, ok := a.stripAt(x, y)
	return ok
}

func (a *App) toggleSideBorder() {
	if a.hires() {
		a.host.Info(AppName, "Side border sprites work in multicolor pictures only.")
		return
	}
	a.commitOrCancelForHistory()
	a.pushUndo(a.pic.Clone())
	a.pic.SideOn = !a.pic.SideOn
	a.markDirty()
	if a.pic.SideOn {
		a.setStatus("Side border open (experimental): paint left and right of the picture – 10 sprites per side")
		if !a.set.ShowBorder {
			a.fitPending = true
		}
	} else {
		a.setStatus("Side border closed (the sprites are kept in the project)")
	}
}

func (a *App) clearSideBorder() {
	a.commitOrCancelForHistory()
	a.pushUndo(a.pic.Clone())
	a.pic.ClearSide()
	a.markDirty()
	a.setStatus("Side border sprites cleared")
}

// setSideMC changes a shared sprite color; its pixels change with it.
func (a *App) setSideMC(k int, c uint8) {
	s := &a.pic.Side
	old := s.MC1
	if k == 2 {
		old = s.MC2
	}
	if old == c {
		return
	}
	a.pushUndo(a.pic.Clone())
	for sd := range s.Pix {
		for i, v := range s.Pix[sd] {
			if v != 0 && v-1 == old {
				s.Pix[sd][i] = c + 1
			}
		}
	}
	if k == 2 {
		s.MC2 = c
	} else {
		s.MC1 = c
	}
	a.markDirty()
	a.setStatus(fmt.Sprintf("Sprite multicolor %d: %s", k, c64.ColorNames[c]))
}

func (a *App) plotSide(p *c64.Picture, side, sx, y int, c uint8) {
	mode := c64.ConflictMode(a.set.Conflict)
	if r := p.PlotSide(side, sx, y, c, mode, a.pal); r != c64.PlotOK {
		if a.sideFlash == nil {
			a.sideFlash = map[[2]int]int64{}
		}
		a.sideFlash[[2]int{side, y / c64.SpriteH}] = a.now
		a.needAnim = true
	}
}

// stripFill fills the connected area of one strip.
func (a *App) stripFill(side, sx, y, btn int) bool {
	p := a.pic
	target := p.SideColorAt(side, sx, y)
	if target == a.colorFor(btn) && a.pattern == 0 {
		return false
	}
	seen := make([]bool, c64.SideW*c64.H)
	stack := []int{y*c64.SideW + sx}
	seen[stack[0]] = true
	var region []int
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		region = append(region, i)
		cx, cy := i%c64.SideW, i/c64.SideW
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := cx+d[0], cy+d[1]
			if nx < 0 || ny < 0 || nx >= c64.SideW || ny >= c64.H || c64.SideHidden(side, ny) {
				continue
			}
			j := ny*c64.SideW + nx
			if !seen[j] && p.SideColorAt(side, nx, ny) == target {
				seen[j] = true
				stack = append(stack, j)
			}
		}
	}
	x0 := -c64.SideW
	if side == 1 {
		x0 = c64.W
	}
	for _, i := range region {
		px, py := x0+i%c64.SideW, i/c64.SideW
		if patBit(a.pattern, px, py) {
			a.plot(p, px, py, a.colorFor(btn))
		}
	}
	return true
}

// drawSideStrips draws the opened side border with the sprite strips.
func (a *App) drawSideStrips(imgR gfx.Rect, z int) {
	m := a.img
	p := a.pic
	bg := a.pal.Colors[p.BG]
	pw := asp * z
	for y := 0; y <= c64.H; y++ {
		sy := imgR.Y + y*z
		if y < c64.H {
			// right: open border in the background color, then the strip
			m.Fill(gfx.Rect{X: imgR.Right(), Y: sy, W: 32 * z, H: z}, bg)
			for sx := 0; sx < c64.SideW; sx++ {
				m.Fill(gfx.Rect{X: imgR.Right() + sx*pw, Y: sy, W: pw, H: z}, a.pal.Colors[p.SideColorAt(1, sx, y)])
			}
		}
		if y >= 1 {
			m.Fill(gfx.Rect{X: imgR.X - 32*z, Y: sy, W: 32 * z, H: z}, bg)
			if y < c64.H {
				for sx := 0; sx < c64.SideW; sx++ {
					m.Fill(gfx.Rect{X: imgR.X - 24*z + sx*pw, Y: sy, W: pw, H: z}, a.pal.Colors[p.SideColorAt(0, sx, y)])
				}
			}
		}
		if y == c64.H {
			// the line below the picture stays open up to the right border
			m.Fill(gfx.Rect{X: imgR.X - 32*z, Y: sy, W: (32 + 320) * z, H: z}, bg)
		}
	}
	// sprite blocks (with the grid)
	if a.set.GridOn {
		al := a.set.GridAlpha * 256 / 100
		for side := 0; side < 2; side++ {
			x0 := imgR.X - 24*z
			if side == 1 {
				x0 = imgR.Right()
			}
			r := gfx.Rect{X: x0, Y: imgR.Y, W: 24 * z, H: 200 * z}.Intersect(m.Clip)
			for row := 0; row <= c64.SideRows; row++ {
				y := imgR.Y + min(row*c64.SpriteH, c64.H)*z
				if y >= r.Y && y < r.Bottom() {
					for x := r.X; x < r.Right(); x++ {
						i := y*m.W + x
						m.Pix[i] = gfx.Mix(m.Pix[i], m.Pix[i]^0xFFFFFF, al)
					}
				}
			}
			for _, x := range []int{x0, x0 + 24*z} {
				if x >= r.X && x < r.Right() {
					for y := r.Y; y < r.Bottom(); y++ {
						i := y*m.W + x
						m.Pix[i] = gfx.Mix(m.Pix[i], m.Pix[i]^0xFFFFFF, al)
					}
				}
			}
		}
	}
	// blocked pixels flash the sprite block red
	for k, t := range a.sideFlash {
		age := a.now - t
		if age > 900 || age < 0 {
			delete(a.sideFlash, k)
			continue
		}
		a.needAnim = true
		r := a.sideBlockRect(imgR, z, k[0], k[1])
		al := int(200 - age*200/900)
		m.FillA(r, gfx.RGB(255, 40, 40), al/2)
		if al > 40 {
			m.Frame(r, gfx.RGB(255, 40, 40))
		}
	}
	// block under the mouse
	if a.mouseSide && a.set.CellHint && a.modal == nil && a.menuOpen < 0 {
		if side, _, ok := a.stripAt(a.mpx, a.mpy); ok {
			dashedFrame(m, a.sideBlockRect(imgR, z, side, a.mpy/c64.SpriteH), 0xFFFFFF, 0x000000, 0)
		}
	}
}

func (a *App) sideBlockRect(imgR gfx.Rect, z, side, row int) gfx.Rect {
	x := imgR.X - 24*z
	if side == 1 {
		x = imgR.Right()
	}
	y0 := row * c64.SpriteH
	y1 := min(c64.H, y0+c64.SpriteH)
	return gfx.Rect{X: x, Y: imgR.Y + y0*z, W: 24 * z, H: (y1 - y0) * z}
}

// panelSide: the shared colors and the block under the mouse.
func (a *App) panelSide(x, y, w int) int {
	if !a.sideOn() {
		return y
	}
	u := &a.ui
	m := a.img
	th := a.th
	y = a.section(x, y, w, "Side border sprites")
	if a.collapsed["Side border sprites"] {
		return y
	}
	s := &a.pic.Side
	sw := func(id string, k int, c uint8, label string) {
		r := gfx.Rect{X: x, Y: y, W: 18, H: 15}
		m.Fill(r, a.pal.Colors[c])
		m.Frame(r, th.BtnBorder)
		m.Text(font.UI, x+24, y+1, label+": "+c64.ColorNames[c], th.Text)
		hit := gfx.Rect{X: x, Y: y - 1, W: w, H: 17}
		if cl, _ := u.press(id, hit, 0); cl {
			a.setSideMC(k, a.fg)
		}
		if cl, _ := u.press(id+"r", hit, 1); cl {
			a.setSideMC(k, a.fg2)
		}
		u.Tip(id, hit, "Click: set to color 1 (right click: color 2). All its pixels change with it.")
		y += 19
	}
	sw("sd_mc1", 1, s.MC1, "Multicolor 1 ($D025)")
	sw("sd_mc2", 2, s.MC2, "Multicolor 2 ($D026)")
	if side, _, ok := a.stripAt(a.mpx, a.mpy); ok && a.mouseSide {
		row := a.mpy / c64.SpriteH
		c := s.Col[side][row]
		r := gfx.Rect{X: x, Y: y, W: 18, H: 15}
		m.Fill(r, a.pal.Colors[c])
		m.Frame(r, th.BtnBorder)
		m.Text(font.UI, x+24, y+1, fmt.Sprintf("%s sprite %d: %s", [2]string{"Left", "Right"}[side], row+1, c64.ColorNames[c]), th.Text)
		y += 19
	} else {
		u.Label(gfx.Rect{X: x, Y: y, W: w, H: 18}, "Each 12×21 block: own color + MC 1/2", true)
		y += 20
	}
	if u.Button("sd_clear", gfx.Rect{X: x, Y: y, W: w, H: 22}, "Clear side border") {
		a.clearSideBorder()
	}
	y += 28
	m.Text(font.UI, x, y, "Experimental · PAL only", th.TextDim)
	return y + 22
}

// sideStatus is the status bar text over a strip.
func (a *App) sideStatus() string {
	side, sx, ok := a.stripAt(a.mpx, a.mpy)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s border X %2d  Y %3d   Sprite %d   %s", [2]string{"Left", "Right"}[side], sx, a.mpy, a.mpy/c64.SpriteH+1, c64.ColorNames[a.pic.SideColorAt(side, sx, a.mpy)])
}

// cmdExportSide saves the side border program or its ACME source.
func (a *App) cmdExportSide(source bool) {
	a.commitOrCancelForHistory()
	if !a.sideOn() {
		return
	}
	base := "picture"
	if a.filePath[a.page] != "" {
		base = trimExt(filepath.Base(a.filePath[a.page]))
	}
	var f []Filter
	var name, ext, title string
	if source {
		f, name, ext, title = []Filter{{"ACME assembler source (*.asm)", "*.asm"}}, base+"_border.asm", "asm", "Export side border program as ACME source"
	} else {
		f, name, ext, title = []Filter{{"C64 program (*.prg)", "*.prg"}}, base+"_border.prg", "prg", "Export C64 program with side border"
	}
	p, ok := a.host.SaveFile(title, f, a.set.LastDir, name, ext)
	if !ok {
		return
	}
	p = FixDoubleExt(p)
	var data []byte
	var err error
	if source {
		data = []byte(a.pic.SideBorderACME(trimExt(filepath.Base(p))))
	} else {
		data, err = a.pic.SideBorderPRG()
	}
	if err == nil {
		err = a.host.WriteFile(p, data)
	}
	if err != nil {
		a.host.Info(AppName, "Saving failed:\n"+err.Error())
		return
	}
	a.rememberDir(p)
	a.setStatus(fmt.Sprintf("Saved: %s (%d bytes) – PAL only, experimental", filepath.Base(p), len(data)))
}
