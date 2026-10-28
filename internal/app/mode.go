package app

import (
	"fmt"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// The picture is either multicolor (160 × 200, pixels twice as wide) or
// hires (320 × 200). picW and asp follow the mode of the current picture;
// syncMode sets them (every frame and after a new picture).
var (
	picW = c64.W // width of the picture in pixels
	asp  = 2     // screen pixels per picture pixel across
)

func (a *App) syncMode() {
	picW = a.pic.Width()
	asp = 320 / picW
}

// hires reports whether the current picture is a hires picture.
func (a *App) hires() bool { return a.pic.Hires() }

// --- choosing the mode (start and File → New) ------------------------------

type modeDialog struct {
	start bool   // at program start: closing keeps multicolor
	then  func() // runs after a choice (or after closing at the start)
}

// askMode shows the mode window. At the start closing it keeps the
// multicolor picture; with File → New closing cancels.
func (a *App) askMode(start bool, then func()) {
	a.menuOpen = -1
	a.modal = &modeDialog{start: start, then: then}
}

func (d *modeDialog) Title() string {
	if d.start {
		return "Welcome – which mode do you want to paint in?"
	}
	return "New picture – choose the mode"
}

func (d *modeDialog) Size(a *App) (int, int) { return 780, 360 }

func (d *modeDialog) Cancel(a *App) {
	if d.start && d.then != nil {
		d.then()
	}
}

func (d *modeDialog) Key(a *App, k KeyEvent) {
	if k.Key == KeyEnter {
		d.choose(a, c64.ModeMulti)
	}
}

func (d *modeDialog) Char(a *App, r rune) {
	if r >= '1' && r <= '3' {
		d.choose(a, c64.Mode(r-'1'))
	}
}

func (d *modeDialog) choose(a *App, m c64.Mode) {
	if a.modal != Modal(d) {
		return // already chosen
	}
	a.closeModal(false)
	a.newPicture(m)
	if d.then != nil {
		d.then()
	}
}

var modeInfo = [3][]string{
	{"160 × 200 pixels, twice as wide", "4 × 8 cells: background + 3 colors", "Koala, Art Studio (OCP), PRG"},
	{"320 × 200 pixels", "8 × 8 cells: 2 colors each", "Art Studio hires, Doodle, PRG"},
	{"320 × 200 pixels", "all 16 colors everywhere", "for drawing; cells are reduced to", "2 colors when exported for the C64"},
}

func (d *modeDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	m.Text(font.UI, r.X, r.Y, "The mode decides the size of the pixels and the color rules. You can choose again with File → New.", th.TextDim)
	cw := (r.W - 2*16) / 3
	for i := 0; i < 3; i++ {
		cr := gfx.Rect{X: r.X + i*(cw+16), Y: r.Y + 28, W: cw, H: r.H - 28}
		m.Fill(cr, th.PanelAlt)
		m.Frame(cr, th.Border)
		if u.hover(cr) {
			m.Frame(cr, th.Accent)
			m.Frame(cr.Inset(1), th.Accent)
		}
		// little sample: pixels of the mode
		sr := gfx.Rect{X: cr.X + 14, Y: cr.Y + 14, W: cw - 28, H: 84}
		a.drawModeSample(c64.Mode(i), sr)
		y := sr.Bottom() + 12
		m.Text(font.H2, cr.X+14, y, fmt.Sprintf("%d  %s", i+1, c64.ModeNames[i]), th.Text)
		y += font.H2.Height + 8
		for _, ln := range modeInfo[i] {
			m.Text(font.UI, cr.X+14, y, ln, th.TextDim)
			y += font.UI.Height + 4
		}
		if c, _ := u.press("md_"+itoa(i), cr, 0); c {
			d.choose(a, c64.Mode(i))
			return
		}
	}
}

// drawModeSample draws a small example of the pixel shape and the colors.
func (a *App) drawModeSample(mode c64.Mode, r gfx.Rect) {
	m := a.img
	pal := a.pal.Colors
	px := 5 // screen pixel size of the sample
	cols := r.W / (8 * px)
	r = gfx.Rect{X: r.X + (r.W-cols*8*px)/2, Y: r.Y, W: cols * 8 * px, H: 16 * px}
	for cy := 0; cy < 2; cy++ {
		for cx := 0; cx < cols; cx++ {
			ox, oy := r.X+cx*8*px, r.Y+cy*8*px
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					var c uint32
					switch mode {
					case c64.ModeMulti:
						set := [4]int{6, 14, 3, 1}
						v := ((x/2 + y + cx*3 + cy) / 2) % 4
						c = pal[set[v]]
					case c64.ModeHires:
						pair := [][2]int{{6, 14}, {6, 1}, {0, 7}, {2, 10}}[(cx+cy)%4]
						if (x+y+cx)%3 == 0 {
							c = pal[pair[1]]
						} else {
							c = pal[pair[0]]
						}
					default:
						c = pal[(x/2+y/2+cx*2+cy*3)%16]
					}
					w := px
					if mode == c64.ModeMulti {
						if x%2 == 1 {
							continue
						}
						w = 2 * px
					}
					m.Fill(gfx.Rect{X: ox + x*px, Y: oy + y*px, W: w, H: px}, c)
				}
			}
			// cell frame
			m.Frame(gfx.Rect{X: ox, Y: oy, W: 8 * px, H: 8 * px}, 0x000000)
		}
	}
	m.Frame(r.Inset(-1), a.th.Border)
}

// newPicture replaces the current page by an empty picture of a mode.
func (a *App) newPicture(mode c64.Mode) {
	a.commitOrCancelForHistory()
	a.sel.state = selNone
	bg, border := a.pic.BG, a.pic.Border
	if mode == c64.ModeMulti {
		a.setFlatPicture(c64.NewPicture(bg, border))
	} else {
		a.setFlatPicture(c64.NewHiresPicture(mode, bg, border))
	}
	a.syncMode()
	a.undos[a.page], a.redos[a.page] = nil, nil
	a.dirty[a.page] = false
	a.filePath[a.page] = ""
	a.stampCache = nil
	a.set.Sym.AX = picW / 2
	a.updateTitle()
	a.setStatus("New picture: " + c64.ModeNames[mode])
}
