package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"path/filepath"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- Shared parts of the full-window tools (PETSCII, sprite and font editor) ---

const toolTopH, toolLeftW, toolRightW, toolStatusH = 38, 190, 300, 24

// toolButton is a button of the top bar; an empty id adds a gap.
type toolButton struct {
	id, label, tip string
	f              func()
}

// toolTopBar draws the top bar with the title, the buttons and the close button.
func (a *App) toolTopBar(title string, btns []toolButton, closeID, closeLabel string, close func()) {
	u := &a.ui
	m := a.img
	th := a.th
	h := toolTopH
	m.Fill(gfx.Rect{X: 0, Y: 0, W: a.W, H: h}, th.MenuBar)
	m.HLine(0, a.W-1, h-1, th.Border)
	m.Text(font.UIB, 10, (h-font.UIB.Height)/2, title, th.MenuText)
	x := font.UIB.Width(title) + 30
	for _, b := range btns {
		if b.id == "" {
			x += 8
			continue
		}
		w := font.UI.Width(b.label) + 18
		br := gfx.Rect{X: x, Y: 6, W: w, H: h - 12}
		if u.Button(b.id, br, b.label) {
			a.later(b.f)
		}
		u.Tip(b.id, br, b.tip)
		x += w + 4
	}
	cw := font.UI.Width(closeLabel) + 24
	cr := gfx.Rect{X: a.W - cw - 10, Y: 6, W: cw, H: h - 12}
	if u.ButtonEx(closeID, cr, closeLabel, true, true) {
		a.later(close)
	}
}

// toolStatus draws the status line.
func (a *App) toolStatus(left, right string) {
	m := a.img
	th := a.th
	sr := gfx.Rect{X: 0, Y: a.H - toolStatusH, W: a.W, H: toolStatusH}
	m.Fill(sr, th.Panel)
	m.HLine(0, a.W-1, sr.Y, th.Border)
	m.TextL(font.UI, gfx.Rect{X: 10, Y: sr.Y, W: a.W - 20, H: sr.H}, left, th.Text)
	m.TextR(font.UI, gfx.Rect{X: 10, Y: sr.Y, W: a.W - 20, H: sr.H}, right, th.TextDim)
}

// toolPanel fills a side panel.
func (a *App) toolPanel(r gfx.Rect, left bool) {
	m := a.img
	m.Fill(r, a.th.Panel)
	if left {
		m.VLine(r.Right()-1, r.Y, r.Bottom()-1, a.th.Border)
	} else {
		m.VLine(r.X, r.Y, r.Bottom()-1, a.th.Border)
	}
}

// toolPalette draws the 16 colors in two rows of eight and returns the height.
// pick is called with the clicked color.
func (a *App) toolPalette(id string, x, y, w, cur int, pick func(int)) int {
	u := &a.ui
	m := a.img
	th := a.th
	sw := w / 8
	for i := 0; i < 16; i++ {
		br := gfx.Rect{X: x + (i%8)*sw, Y: y + (i/8)*(sw-4), W: sw - 2, H: sw - 6}
		m.Fill(br, a.pal.Colors[i])
		m.Frame(br, th.BtnBorder)
		if cur == i {
			m.Frame(br.Inset(-1), th.Accent)
			m.Frame(br.Inset(-2), th.Accent)
		}
		if c, _ := u.press(id+itoa(i), br, 0); c {
			pick(i)
		}
		u.Tip(id+itoa(i), br, c64.ColorNames[i])
	}
	return 2*(sw-4) + 6
}

// toolColorChoice draws "A" plus the 16 colors in one row (A = -1).
func (a *App) toolColorChoice(id string, x, y, w, cur int, autoTip string, pick func(int)) {
	u := &a.ui
	m := a.img
	th := a.th
	sw := (w - 16) / 17
	for i := -1; i < 16; i++ {
		sr := gfx.Rect{X: x + (i+1)*(sw+1), Y: y, W: sw, H: 22}
		if i < 0 {
			m.Fill(sr, th.Btn)
			m.TextC(font.UIB, sr, "A", th.Text)
		} else {
			m.Fill(sr, a.pal.Colors[i])
		}
		if cur == i {
			m.Frame(sr.Inset(-1), th.Accent)
			m.Frame(sr, th.Accent)
		}
		if c, _ := u.press(id+itoa(i), sr, 0); c {
			pick(i)
		}
		if i < 0 {
			u.Tip(id+itoa(i), sr, autoTip)
		} else {
			u.Tip(id+itoa(i), sr, c64.ColorNames[i])
		}
	}
}

// pictureIndexes returns the multicolor picture as 320 × 200 C64 colors.
func (a *App) pictureIndexes() []int8 {
	out := make([]int8, 320*200)
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			out[y*320+x] = int8(a.pic.ColorAt(x/2, y))
		}
	}
	return out
}

// loadIndexImage asks for an image and returns it as 320 × 200 C64 colors
// (an image of exactly 320 × 200 pixels is taken 1:1).
func (a *App) loadIndexImage(title string) ([]int8, string, bool) {
	p, ok := a.host.OpenFile(title, []Filter{{Name: "Images (*.png, *.bmp, *.jpg, *.gif)", Pattern: "*.png;*.bmp;*.jpg;*.jpeg;*.gif"}}, a.workDir())
	if !ok {
		return nil, "", false
	}
	data, err := a.host.ReadFile(p)
	if err == nil {
		img, err2 := decodeImage(data, p)
		if err2 == nil {
			rgb := c64.ResampleScreen(img, c64.FitContain)
			out := make([]int8, 320*200)
			for i, c := range rgb {
				out[i] = int8(a.pal.Nearest(c[0], c[1], c[2]))
			}
			return out, filepath.Base(p), true
		}
		err = err2
	}
	a.host.Info(AppName, "Cannot read the image:\n"+err.Error())
	return nil, "", false
}

// drawIndexPicture draws a 320 × 200 picture of C64 colors with pixel size z.
func (a *App) drawIndexPicture(px []int8, x0, y0, z int) {
	m := a.img
	pal := a.pal.Colors
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			if c := px[y*320+x]; c >= 0 {
				m.Fill(gfx.Rect{X: x0 + x*z, Y: y0 + y*z, W: z, H: z}, pal[c])
			}
		}
	}
}

// fullEditorFrame runs the frame of an open full-window tool. It returns
// false when none is open.
func (a *App) fullEditorFrame() bool {
	switch {
	case a.pet != nil && a.pet.open:
		a.petFrame()
	case a.spr != nil && a.spr.open:
		a.sprFrame()
	case a.fnt != nil && a.fnt.open:
		a.fntFrame()
	default:
		return false
	}
	return true
}

// toolFrameStart handles keys and the input block of a full-window tool.
func (a *App) toolFrameStart(keys func()) {
	if a.modal != nil {
		a.handleKeys()
	} else {
		keys()
	}
}

// toolFrameEnd draws an open dialog on top.
func (a *App) toolFrameEnd() {
	if a.modal != nil {
		a.ui.blocked = false
		a.drawModal()
	}
}

// encodePNG writes a gfx image as PNG.
func encodePNG(m *gfx.Image) ([]byte, error) {
	img := image.NewNRGBA(image.Rect(0, 0, m.W, m.H))
	for y := 0; y < m.H; y++ {
		for x := 0; x < m.W; x++ {
			c := m.Pix[y*m.W+x]
			img.SetNRGBA(x, y, color.NRGBA{uint8(c >> 16), uint8(c >> 8), uint8(c), 255})
		}
	}
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	return buf.Bytes(), err
}
