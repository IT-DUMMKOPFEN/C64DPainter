package app

import (
	"os"
	"path/filepath"
	"testing"

	"c64dpainter/internal/c64"
)

type recHost struct {
	nullHost
	save, open string
	dir        string
}

func (h *recHost) SaveFile(string, []Filter, string, string, string) (string, bool) {
	return h.save, h.save != ""
}
func (h *recHost) OpenFile(string, []Filter, string) (string, bool) { return h.open, h.open != "" }
func (h *recHost) WriteFile(p string, d []byte) error               { return os.WriteFile(p, d, 0o644) }

var tnow int64

func step(a *App) {
	tnow += 16
	a.Tick(tnow)
	a.Frame()
	a.RunDeferred()
}

func tclick(a *App, x, y, b int) {
	a.MouseMove(x, y)
	step(a)
	a.MouseDown(b, x, y, false)
	step(a)
	a.MouseUp(b, x, y)
	step(a)
}

func tdrag(a *App, x0, y0, x1, y1, b int) {
	a.MouseMove(x0, y0)
	step(a)
	a.MouseDown(b, x0, y0, false)
	step(a)
	for i := 1; i <= 8; i++ {
		a.MouseMove(x0+(x1-x0)*i/8, y0+(y1-y0)*i/8)
		step(a)
	}
	a.MouseUp(b, x1, y1)
	step(a)
}

func pc(a *App, x, y int) (int, int) { return a.PicToScreenCenter(x, y) }

func setup(t *testing.T) (*App, *recHost) {
	dir := t.TempDir()
	SettingsPath = func() string { return filepath.Join(dir, "s.json") }
	h := &recHost{dir: dir}
	a := New(h)
	a.Resize(1360, 860)
	step(a)
	a.closeModal(false) // Hilfe beim ersten Start
	step(a)
	return a, h
}

func toolRect(a *App, i int) (int, int) {
	x := a.rTools.X + 5 + (i%2)*31 + 15
	y := a.rTools.Y + 6 + presetsH() + (i/2)*27 + 13
	return x, y
}

func TestToolboxAndPalette(t *testing.T) {
	a, _ := setup(t)
	// Werkzeug „Linie“ ist Knopf 2
	x, y := toolRect(a, 2)
	tclick(a, x, y, 0)
	if a.tool != ToolLine {
		t.Fatalf("Linie nicht gewählt: %d", a.tool)
	}
	// geteilter Knopf Rechteck (Index 6): unten rechts = gefüllt
	x, y = toolRect(a, 6)
	tclick(a, x+10, y+9, 0)
	if a.tool != ToolRectF {
		t.Fatalf("gefülltes Rechteck erwartet, %d", a.tool)
	}
	tclick(a, x-10, y-9, 0)
	if a.tool != ToolRect {
		t.Fatalf("Rechteck erwartet, %d", a.tool)
	}
	// Palette: Spalte 1 Zeile 0 = Farbe 8
	n := len(a.toolButtons())
	py := a.rTools.Y + 6 + presetsH() + ((n+1)/2)*27 + 8 + 40
	px := a.rTools.X + 5 + 31 + 10
	tclick(a, px, py+5, 0)
	if a.fg != 8 {
		t.Fatalf("Farbe 1 sollte 8 sein: %d", a.fg)
	}
	tclick(a, px, py+5, 1)
	if a.fg2 != 8 {
		t.Fatalf("Farbe 2 sollte 8 sein: %d", a.fg2)
	}
	a.SetMods(Mods{Ctrl: true})
	a.MouseMove(px, py+5)
	step(a)
	a.MouseDown(0, px, py+5, false)
	step(a)
	a.MouseUp(0, px, py+5)
	step(a)
	a.SetMods(Mods{})
	if a.pic.BG != 8 {
		t.Fatalf("Hintergrund sollte 8 sein: %d", a.pic.BG)
	}
}

func TestDrawUndoRedo(t *testing.T) {
	a, _ := setup(t)
	a.fg = 7
	a.Char('R')
	step(a)
	x0, y0 := pc(a, 10, 10)
	x1, y1 := pc(a, 20, 30)
	tdrag(a, x0, y0, x1, y1, 0)
	if a.pic.ColorAt(15, 20) != 7 || a.pic.ColorAt(9, 9) != 0 {
		t.Fatal("gefülltes Rechteck fehlt")
	}
	a.Char('u')
	step(a)
	if a.pic.ColorAt(15, 20) != 0 {
		t.Fatal("Rückgängig wirkungslos")
	}
	a.KeyDown('Y', Mods{Ctrl: true})
	step(a)
	if a.pic.ColorAt(15, 20) != 7 {
		t.Fatal("Wiederholen wirkungslos")
	}
	// Rechte Maustaste malt Farbe 2
	a.fg2 = 2
	a.Char('d')
	step(a)
	x0, y0 = pc(a, 50, 50)
	x1, y1 = pc(a, 60, 50)
	tdrag(a, x0, y0, x1, y1, 1)
	if a.pic.ColorAt(55, 50) != 2 {
		t.Fatal("rechte Taste sollte Farbe 2 malen")
	}
}

func TestCurveAndFill(t *testing.T) {
	a, _ := setup(t)
	a.fg = 5
	a.Char('q')
	step(a)
	x0, y0 := pc(a, 10, 100)
	x1, y1 := pc(a, 100, 100)
	tdrag(a, x0, y0, x1, y1, 0)
	if a.curve.stage != 2 {
		t.Fatal("Kurve sollte im Biegen-Modus sein")
	}
	mx, my := pc(a, 55, 60)
	tclick(a, mx, my, 0)
	if a.curve.stage != 0 {
		t.Fatal("Kurve nicht festgelegt")
	}
	if a.pic.ColorAt(55, 60) != 5 {
		t.Fatal("Kurve geht nicht durch den Biegepunkt")
	}
	// Füllen
	a.fg = 1
	a.Char('f')
	step(a)
	fx, fy := pc(a, 150, 190)
	tclick(a, fx, fy, 0)
	if a.pic.ColorAt(0, 0) != 1 || a.pic.ColorAt(159, 199) != 1 {
		t.Fatal("Füllen fehlgeschlagen")
	}
}

func TestSelectionMoveFlipBrush(t *testing.T) {
	a, _ := setup(t)
	// Muster malen: links Gelb, rechts Rot
	for y := 10; y < 20; y++ {
		for x := 10; x < 14; x++ {
			a.pic.Plot(x, y, 7, 0, a.pal)
		}
		for x := 14; x < 18; x++ {
			a.pic.Plot(x, y, 2, 0, a.pal)
		}
	}
	a.Char('b')
	step(a)
	x0, y0 := pc(a, 10, 10)
	x1, y1 := pc(a, 17, 19)
	tdrag(a, x0, y0, x1, y1, 0)
	if a.sel.state != selSelected {
		t.Fatal("Auswahl fehlt")
	}
	a.Char('x')
	step(a)
	if a.sel.state != selFloating {
		t.Fatal("Spiegeln sollte anheben")
	}
	if a.pic.ColorAt(10, 10) != 2 || a.pic.ColorAt(17, 10) != 7 {
		t.Fatalf("Spiegeln falsch: %d %d", a.pic.ColorAt(10, 10), a.pic.ColorAt(17, 10))
	}
	// verschieben um 20 nach rechts
	sx, sy := pc(a, 12, 12)
	tx, ty := pc(a, 32, 12)
	tdrag(a, sx, sy, tx, ty, 0)
	a.KeyDown(KeyEnter, Mods{})
	step(a)
	if a.sel.state != selNone {
		t.Fatal("Übernehmen fehlgeschlagen")
	}
	if a.pic.ColorAt(30, 10) != 2 || a.pic.ColorAt(37, 10) != 7 || a.pic.ColorAt(12, 12) != 0 {
		t.Fatalf("Verschieben falsch: %d %d %d", a.pic.ColorAt(30, 10), a.pic.ColorAt(37, 10), a.pic.ColorAt(12, 12))
	}
	// als Pinsel
	x0, y0 = pc(a, 30, 10)
	x1, y1 = pc(a, 37, 19)
	tdrag(a, x0, y0, x1, y1, 0)
	a.selToBrush()
	step(a)
	if a.brushShape != BrushCustom || a.custom.W != 8 || a.custom.H != 10 {
		t.Fatalf("eigener Pinsel falsch")
	}
	px, py := pc(a, 100, 100)
	tclick(a, px, py, 0)
	if a.pic.ColorAt(100-4, 100-5) != 2 || a.pic.ColorAt(100+3, 100-5) != 7 {
		t.Fatalf("Pinsel stempelt falsch: %d %d", a.pic.ColorAt(96, 95), a.pic.ColorAt(103, 95))
	}
	// Kopieren & Einfügen
	a.Char('b')
	step(a)
	tdrag(a, x0, y0, x1, y1, 0)
	a.KeyDown('C', Mods{Ctrl: true})
	step(a)
	a.MouseMove(pc(a, 140, 180))
	step(a)
	a.KeyDown('V', Mods{Ctrl: true})
	step(a)
	if a.sel.state != selFloating {
		t.Fatal("Einfügen erzeugt keine schwebende Auswahl")
	}
	a.KeyDown(KeyEnter, Mods{})
	step(a)
	if a.pic.ColorAt(136, 175) != 2 {
		t.Fatalf("Einfügen an Mausposition falsch: %d", a.pic.ColorAt(136, 175))
	}
	// Drehen um 90° und rückgängig
	snap := a.pic.Clone()
	tdrag(a, x0, y0, x1, y1, 0)
	a.Char('z')
	step(a)
	a.KeyDown(KeyEnter, Mods{})
	step(a)
	a.Char('u')
	step(a)
	if *a.pic != *snap {
		t.Fatal("Rückgängig nach Drehen stellt nicht her")
	}
}

func TestConflictModes(t *testing.T) {
	a, _ := setup(t)
	for i, c := range []uint8{2, 3, 4} {
		a.pic.Plot(i, 0, c, 0, a.pal)
	}
	a.fg = 5
	a.Char('d')
	step(a)
	x, y := pc(a, 3, 0)
	tclick(a, x, y, 0)
	if a.pic.ColorAt(3, 0) != 0 {
		t.Fatal("Blockieren: Punkt darf nicht gesetzt werden")
	}
	if len(a.flash) == 0 {
		t.Fatal("Konfliktzelle sollte blinken")
	}
	a.setConflict(1)
	tclick(a, x, y, 0)
	if a.pic.ColorAt(3, 0) != 5 {
		t.Fatal("Ersetzen: Punkt sollte gesetzt werden")
	}
	a.setConflict(2)
	a.fg = 10 // Hellrot → ähnlich Rot
	x, y = pc(a, 3, 1)
	tclick(a, x, y, 0)
	c := a.pic.ColorAt(3, 1)
	if c == 10 || c == 0 {
		t.Fatalf("Ähnlichste: unerwartete Farbe %d", c)
	}
}

func TestMenuSpareHelpPanel(t *testing.T) {
	a, _ := setup(t)
	// Menü „Bearbeiten“ öffnen
	var bx int
	for _, m := range a.menus {
		if m.Title == "Edit" {
			bx = m.x + m.w/2
		}
	}
	tclick(a, bx, 12, 0)
	if a.menuOpen != 1 {
		t.Fatal("Menü nicht offen")
	}
	// Eintrag „Seite wechseln“ suchen
	y := a.rMenu.Bottom() - 1 + 3
	for _, it := range a.menus[1].Items {
		if it.Sep {
			y += 9
			continue
		}
		if it.Heading {
			y += 22
			continue
		}
		if it.Label == "Swap page (spare page)" {
			break
		}
		y += 24
	}
	tclick(a, bx+20, y+12, 0)
	if a.page != 1 || a.menuOpen != -1 {
		t.Fatalf("Seitenwechsel per Menü fehlgeschlagen: page %d menu %d", a.page, a.menuOpen)
	}
	a.KeyDown(KeyF1, Mods{})
	step(a)
	if _, ok := a.modal.(*helpDialog); !ok {
		t.Fatal("F1 öffnet keine Hilfe")
	}
	a.KeyDown(KeyEsc, Mods{})
	step(a)
	if a.modal != nil {
		t.Fatal("Esc schließt Hilfe nicht")
	}
	// Seitenleiste scrollen
	a.Wheel(-3, 100, 400)
	step(a)
	if a.panelScroll == 0 {
		t.Fatal("Seitenleiste scrollt nicht")
	}
}

func TestSaveLoadKLA(t *testing.T) {
	a, h := setup(t)
	a.fg = 7
	a.Char('C')
	step(a)
	x0, y0 := pc(a, 80, 100)
	x1, y1 := pc(a, 80, 140)
	tdrag(a, x0, y0, x1, y1, 0)
	h.save = filepath.Join(h.dir, "t.kla")
	a.KeyDown('S', Mods{Ctrl: true})
	step(a)
	d, err := os.ReadFile(h.save)
	if err != nil || len(d) != 10003 {
		t.Fatalf("KLA nicht gespeichert: %v %d", err, len(d))
	}
	if a.dirty[0] {
		t.Fatal("nach Speichern nicht mehr geändert")
	}
	q, _ := c64.LoadKLA(d)
	if q.ColorAt(80, 100) != 7 {
		t.Fatal("Inhalt falsch")
	}
	a.cmdNew()
	h.open = h.save
	a.KeyDown('O', Mods{Ctrl: true})
	step(a)
	if a.pic.ColorAt(80, 100) != 7 {
		t.Fatal("Laden fehlgeschlagen")
	}
}

func TestDialogDrag(t *testing.T) {
	a, _ := setup(t)
	a.openAbout()
	step(a)
	r0 := a.modalTitle
	// drag the title bar 200 px left and 150 px up
	tdrag(a, r0.X+60, r0.Y+12, r0.X+60-200, r0.Y+12-150, 0)
	if a.modalTitle.X != r0.X-200 || a.modalTitle.Y != r0.Y-150 {
		t.Fatalf("dialog not moved: %v -> %v", r0, a.modalTitle)
	}
	if a.modal == nil {
		t.Fatal("dialog closed by dragging")
	}
	// dragging far outside keeps it inside the window
	r1 := a.modalTitle
	tdrag(a, r1.X+60, r1.Y+12, -2000, -2000, 0)
	if a.modalTitle.X != 0 || a.modalTitle.Y != 0 {
		t.Fatalf("dialog left the window: %v", a.modalTitle)
	}
	// position is remembered when reopened
	a.closeModal(false)
	step(a)
	a.openAbout()
	step(a)
	if a.modalTitle.X != 0 || a.modalTitle.Y != 0 {
		t.Fatalf("position not remembered: %v", a.modalTitle)
	}
	// double-click centers it again
	a.MouseMove(60, 12)
	step(a)
	a.MouseDown(0, 60, 12, true)
	step(a)
	a.MouseUp(0, 60, 12)
	step(a)
	if a.modalTitle != r0 {
		t.Fatalf("double-click did not center: %v vs %v", a.modalTitle, r0)
	}
	// buttons still work after moving
	tdrag(a, r0.X+60, r0.Y+12, r0.X+160, r0.Y+62, 0)
	r := a.modalTitle
	tclick(a, r.X+540-16-55, r.Y+380-12-14, 0) // "Close"
	if a.modal != nil {
		t.Fatal("Close button does not work after moving")
	}
}

func TestSwapColorPicture(t *testing.T) {
	a, _ := setup(t)
	a.pic.BG = 6
	for x := 0; x < 40; x++ {
		a.pic.Plot(x, 10, 2, 0, a.pal) // red line
		a.pic.Plot(x, 11, 7, 0, a.pal) // yellow line
	}
	a.fg, a.fg2 = 2, 5
	a.openSwapDialog(false)
	step(a)
	d := a.modal.(*swapDialog)
	if d.from != 2 || d.to != 5 {
		t.Fatalf("dialog defaults: %d %d", d.from, d.to)
	}
	a.KeyDown(KeyEnter, Mods{})
	step(a)
	if a.pic.ColorAt(5, 10) != 5 || a.pic.ColorAt(5, 11) != 7 || a.pic.BG != 6 || a.pic.ColorAt(5, 50) != 6 {
		t.Fatal("whole picture swap wrong")
	}
	a.undo()
	if a.pic.ColorAt(5, 10) != 2 {
		t.Fatal("undo after swap")
	}
	// background color cannot be chosen
	a.fg = 6
	a.openSwapDialog(false)
	if d := a.modal.(*swapDialog); d.from == 6 || d.to == 6 {
		t.Fatal("background color offered for swapping")
	}
	a.closeModal(true)
}

func TestCellSwapBrush(t *testing.T) {
	a, _ := setup(t)
	for x := 0; x < 160; x++ {
		a.pic.Plot(x, 20, 2, 0, a.pal)
	}
	a.fg, a.fg2 = 2, 7
	a.Char('v') // start from the line tool
	step(a)
	a.openSwapDialog(true)
	step(a)
	a.KeyDown(KeyEnter, Mods{})
	step(a)
	if a.tool != ToolCellSwap {
		t.Fatal("cell swap brush not active")
	}
	// paint over cells in the left half only
	x0, y0 := pc(a, 2, 20)
	x1, y1 := pc(a, 30, 20)
	tdrag(a, x0, y0, x1, y1, 0)
	if a.pic.ColorAt(0, 20) != 7 || a.pic.ColorAt(31, 20) != 7 {
		t.Fatalf("cells under the brush not swapped: %d %d", a.pic.ColorAt(0, 20), a.pic.ColorAt(31, 20))
	}
	if a.pic.ColorAt(100, 20) != 2 {
		t.Fatal("cells outside the stroke must stay")
	}
	// one undo step for the whole stroke
	a.undo()
	if a.pic.ColorAt(0, 20) != 2 || a.pic.ColorAt(31, 20) != 2 {
		t.Fatal("undo of stroke")
	}
	// choosing another tool in the toolbox ends the brush
	x, y := toolRect(a, 1) // Freehand
	tclick(a, x, y, 0)
	if a.tool != ToolDraw {
		t.Fatalf("tool after toolbox click: %d", a.tool)
	}
	// Esc ends it too and returns to the previous tool
	a.Char('v')
	step(a)
	a.openSwapDialog(true)
	step(a)
	a.KeyDown(KeyEnter, Mods{})
	step(a)
	a.KeyDown(KeyEsc, Mods{})
	step(a)
	if a.tool != ToolLine {
		t.Fatalf("Esc should return to the line tool: %d", a.tool)
	}
}

// A click must be visible in the very frame that handles it (no extra
// mouse movement needed).
func TestClickShowsImmediately(t *testing.T) {
	a, _ := setup(t)
	n := len(a.toolButtons())
	cy := a.rTools.Y + 6 + presetsH() + ((n+1)/2)*27 + 8 // color 1/2 display
	py := cy + 40                                        // palette
	px := a.rTools.X + 5 + 31 + 10                       // right column
	sh := clampI((a.rTools.Bottom()-py-6)/8, 10, 26)
	x, y := px, py+3*sh+5 // colour 11 (row 3, right)
	a.MouseMove(x, y)
	step(a)
	a.MouseDown(0, x, y, false)
	step(a)
	a.MouseUp(0, x, y)
	tnow += 16
	a.Tick(tnow)
	img := a.Frame() // the frame that handles the click
	want := a.pal.Colors[a.fg]
	got := img.Get(a.rTools.X+5+5+16, cy+4+10)
	if a.fg != 11 {
		t.Fatalf("color 1 should be 11, is %d", a.fg)
	}
	if got != want {
		t.Fatalf("color 1 display not updated in the same frame: %06x, want %06x", got, want)
	}
}

func TestCtrlStraightLine(t *testing.T) {
	a, _ := setup(t)
	a.fg = 7
	a.Char('d')
	step(a)
	// press at (20,100), wiggle around with Ctrl held, end at (100,60)
	sx, sy := pc(a, 20, 100)
	a.SetMods(Mods{Ctrl: true})
	a.MouseMove(sx, sy)
	step(a)
	a.MouseDown(0, sx, sy, false)
	step(a)
	for _, p := range [][2]int{{40, 150}, {80, 20}, {130, 180}, {100, 60}} {
		x, y := pc(a, p[0], p[1])
		a.MouseMove(x, y)
		step(a)
	}
	ex, ey := pc(a, 100, 60)
	a.MouseUp(0, ex, ey)
	step(a)
	a.SetMods(Mods{})
	// only the straight line remains, not the wiggles
	if a.pic.ColorAt(60, 80) != 7 {
		t.Fatal("straight line missing in the middle")
	}
	if a.pic.ColorAt(40, 150) != 0 || a.pic.ColorAt(80, 20) != 0 || a.pic.ColorAt(130, 180) != 0 {
		t.Fatal("preview positions must not stay painted")
	}
	// one undo removes it
	a.undo()
	if a.pic.ColorAt(60, 80) != 0 || a.pic.ColorAt(20, 100) != 0 {
		t.Fatal("undo should remove the whole stroke")
	}
	// Ctrl released mid-stroke: freehand continues from the line end
	a.MouseMove(sx, sy)
	step(a)
	a.MouseDown(0, sx, sy, false)
	step(a)
	a.SetMods(Mods{Ctrl: true})
	mx, my := pc(a, 60, 100)
	a.MouseMove(mx, my)
	step(a)
	a.SetMods(Mods{})
	fx, fy := pc(a, 60, 140)
	a.MouseMove(fx, fy)
	step(a)
	a.MouseUp(0, fx, fy)
	step(a)
	if a.pic.ColorAt(40, 100) != 7 || a.pic.ColorAt(60, 120) != 7 {
		t.Fatal("line + freehand continuation missing")
	}
}

func TestDiamondBrush(t *testing.T) {
	for _, asp := range []bool{false, true} {
		s := makeStamp(BrushDiamond, 16, asp, nil)
		rows := make([]int, s.H)
		for _, o := range s.Offs {
			rows[o.Y+s.HY]++
		}
		for y := 0; y < s.H; y++ {
			if rows[y] == 0 {
				t.Fatalf("aspect=%v: row %d empty", asp, y)
			}
			if rows[y] != rows[s.H-1-y] {
				t.Fatalf("aspect=%v: rows not symmetric: %v", asp, rows)
			}
		}
		mid := rows[s.H/2]
		if mid != s.W || rows[0] >= mid/2 {
			t.Fatalf("aspect=%v: no diamond shape: %v (w=%d)", asp, rows, s.W)
		}
		sq := makeStamp(BrushSquare, 16, asp, nil)
		if len(s.Offs) >= len(sq.Offs)*2/3 {
			t.Fatalf("aspect=%v: diamond has %d pixels, square %d", asp, len(s.Offs), len(sq.Offs))
		}
	}
}

// presetsH is the height of the brush preset rows at the top of the toolbox.
func presetsH() int { return ((len(brushPresets)+3)/4)*15 + 4 }
