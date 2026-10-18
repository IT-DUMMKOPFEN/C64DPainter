// Package app enthält die gesamte, plattformunabhängige Programmlogik von C64 DPainter.
package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

const AppName = "C64 DPainter"
const AppVersion = "0.96 beta"

// AppAuthor and AppLicense appear on the About page.
const AppAuthor = "IT-Dummköpfen"
const AppLicense = "Free for private use."

// Filter für Dateidialoge.
type Filter struct{ Name, Pattern string }

// Host ist die Verbindung zum Betriebssystem (Fenster, Dateidialoge).
type Host interface {
	OpenFile(title string, filters []Filter, dir string) (string, bool)
	SaveFile(title string, filters []Filter, dir, name, defExt string) (string, bool)
	Ask(title, text string) int // 0 = Ja, 1 = Nein, 2 = Abbrechen
	Info(title, text string)
	Quit()
	SetTitle(string)
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte) error
}

// Cursorformen
const (
	CursorArrow = iota
	CursorCross
	CursorHand
	CursorMove
	CursorSize
)

// Werkzeuge
const (
	ToolDots = iota
	ToolDraw
	ToolLine
	ToolCurve
	ToolFill
	ToolSpray
	ToolRect
	ToolRectF
	ToolCircle
	ToolCircleF
	ToolEllipse
	ToolEllipseF
	ToolSelect
	ToolPicker
	ToolHand
	ToolCellSwap
	ToolDither
	ToolSmear
	ToolErase
	ToolGradient
	toolCount
)

var toolNames = [toolCount]string{
	"Dots", "Freehand", "Line", "Curve", "Fill", "Airbrush",
	"Rectangle", "Filled rectangle", "Circle", "Filled circle",
	"Ellipse", "Filled ellipse", "Select", "Color picker", "Pan", "Cell color swap", "Dither brush (experimental)", "Smear", "Eraser", "Gradient fill",
}

var toolHints = [toolCount]string{
	"Click or drag: single brush dabs  ·  Ctrl = straight line",
	"Drag: paint continuously  ·  Ctrl = straight line (Shift = 45°)  ·  right button = color 2",
	"Drag: line  ·  Shift = snap to 45°",
	"Drag: base line, then move the mouse and click to bend",
	"Click: fill an area of the same color (with pattern)",
	"Hold: spray  ·  radius/density in the Brush panel",
	"Drag: rectangle  ·  Shift = square",
	"Drag: filled rectangle  ·  Shift = square",
	"Drag from the center: circle",
	"Drag from the center: filled circle",
	"Drag from the center: ellipse",
	"Drag from the center: filled ellipse",
	"Drag: select  ·  drag inside = move (Ctrl = copy)",
	"Click: pick a color (left = color 1, right = color 2)",
	"Drag: move the view",
	"Paint over cells to swap their colors  ·  right button = other way  ·  any tool or Esc stops",
	"Paint over color borders to dither them  ·  style and softness in the side panel  ·  Ctrl = straight line",
	"Drag: smear the colors along the stroke  ·  strength in the side panel",
	"Drag: erase on the active layer (it becomes transparent)  ·  Ctrl = straight line",
	"Click: fill an area with the gradient from that point  ·  drag: direction and width  ·  Shift+F: choose gradient",
}

// App ist der Programmzustand.
type App struct {
	host Host
	W, H int
	img  *gfx.Image
	in   Input
	ui   UI
	th   *Theme
	set  Settings

	pal    *c64.Palette
	palIdx int

	pages    [2]*c64.Picture
	undos    [2][]*c64.Picture
	redos    [2][]*c64.Picture
	page     int
	pic      *c64.Picture
	dirty    [2]bool
	filePath [2]string

	tool, prevTool int
	fg, fg2        uint8
	brushShape     int
	brushSize      int
	custom         *CustomBrush
	customOwnColor bool
	pattern        int
	sprayRadius    int
	sprayDensity   int
	stampCache     *Stamp
	stampKey       string

	// Ansicht
	zoom       int
	panX, panY int
	fitPending bool
	autoFit    bool
	lastCW     int
	lastCH     int

	// Interaktion
	drag                       dragState
	curve                      curveState
	panning                    bool
	panSX, panSY, panOX, panOY int
	sel                        Selection
	clip                       *CustomBrush
	tpl                        Template
	mouseIn                    bool
	mpx, mpy                   int // Mausposition in Bildpunkten

	menuOpen          int
	menuHover         int
	menus             []*Menu
	modal             Modal
	swapFrom, swapTo  uint8
	swapBoth, swapSet bool
	toolBeforeSwap    int
	swapDone          map[int]bool
	modalOff          map[string][2]int
	modalDragging     bool
	modalGrab         [2]int
	modalTitle        gfx.Rect
	flash             map[int]int64
	status            string
	statusTime        int64
	deferred          []func()
	now               int64
	lastSpray         int64
	lastAnim          int64
	rnd               uint32

	// Layout
	rMenu, rPanel, rTools, rCanvas, rStatus gfx.Rect
	panelScroll, panelContentH              int
	needAnim                                bool
	quitting                                bool
	lastImport                              c64.ConvertOptions

	// Dither brush: result for the current picture (cached) and its key
	ditherRes   *c64.Picture
	ditherKey   string
	extraLayers bool   // layers 3 and 4 switched on
	smearBuf    []int8 // colors carried by the smear brush
	grad        gradientDrag
}

type dragState struct {
	active bool
	btn    int
	sx, sy int
	lx, ly int
	base   *c64.Picture
	moved  bool
	// straight line while Ctrl is held (freehand and dots)
	straight bool
	lineBase *c64.Picture
	lsx, lsy int
	// dither brush: dithered picture taken at the start of the stroke
	ditherSrc *c64.Picture
}

type curveState struct {
	stage int // 0 = nichts, 1 = Linie ziehen, 2 = biegen
	a, b  pt
	btn   int
	base  *c64.Picture
}

// New erzeugt die Anwendung.
func New(host Host) *App {
	a := &App{host: host, menuOpen: -1, flash: map[int]int64{}, rnd: 0x1234567}
	a.set = loadSettings()
	a.applyTheme()
	a.palIdx = c64.FindPalette(a.set.Palette)
	a.pal = &c64.Palettes[a.palIdx]
	for i := range a.pages {
		a.pages[i] = c64.NewPicture(0, 0)
		a.pages[i].ToLayers()
		a.pages[i].Hide[1], a.pages[i].Hide[2] = true, true // layers 3 + 4 off at the start
		a.pages[i].Recomposite()
	}
	a.pic = a.pages[0]
	a.fg, a.fg2 = 1, 0
	a.tool = ToolDraw
	a.prevTool = ToolDraw
	a.brushShape = BrushRound
	a.brushSize = 1
	a.sprayRadius = 12
	a.sprayDensity = 6
	a.customOwnColor = true
	a.zoom = 3
	a.fitPending = true
	a.tpl.opacity = 50
	a.tpl.visible = true
	a.lastImport = c64.ConvertOptions{BG: -1, Fit: c64.FitContain, Strength: 60, Serpentine: true}
	a.ui.in = &a.in
	a.buildMenus()
	a.updateTitle()
	if !a.set.HelpSeen {
		a.set.HelpSeen = true
		a.saveSettings()
		a.openHelp("start")
	}
	a.setStatus("Welcome to " + AppName + " " + AppVersion + "! Press F1 for help.")
	return a
}

func (a *App) applyTheme() {
	if a.set.Theme == "dark" {
		a.th = &themeDark
	} else {
		a.th = &themeLight
	}
	a.ui.th = a.th
}

// --- Ereignisse vom Fenster ---------------------------------------------------

func (a *App) Resize(w, h int) {
	if w < 200 {
		w = 200
	}
	if h < 150 {
		h = 150
	}
	if w == a.W && h == a.H && a.img != nil {
		return
	}
	a.W, a.H = w, h
	a.img = gfx.New(w, h)
	a.ui.img = a.img
}

func (a *App) MouseMove(x, y int) {
	a.in.MX, a.in.MY = x, y
	a.in.Moved = true
}

func (a *App) MouseDown(btn, x, y int, double bool) {
	a.in.MX, a.in.MY = x, y
	a.in.Down[btn] = true
	a.in.Pressed[btn] = true
	if double {
		a.in.Double = true
	}
}

func (a *App) MouseUp(btn, x, y int) {
	a.in.MX, a.in.MY = x, y
	a.in.Down[btn] = false
	a.in.Released[btn] = true
}

func (a *App) Wheel(delta, x, y int) {
	a.in.MX, a.in.MY = x, y
	a.in.Wheel += delta
}

func (a *App) KeyDown(key int, m Mods) {
	a.in.Mods = m
	if key == KeySpace {
		a.in.Space = true
	}
	a.in.Keys = append(a.in.Keys, KeyEvent{key, m})
}

func (a *App) KeyUp(key int, m Mods) {
	a.in.Mods = m
	if key == KeySpace {
		a.in.Space = false
	}
}

func (a *App) SetMods(m Mods) { a.in.Mods = m }

func (a *App) Char(r rune) {
	if r >= 32 {
		a.in.Chars = append(a.in.Chars, r)
	}
}

// Tick wird regelmäßig aufgerufen (Millisekunden). Liefert true, wenn neu gezeichnet werden soll.
func (a *App) Tick(now int64) bool {
	a.now = now
	a.ui.now = now
	redraw := false
	if a.drag.active && (a.tool == ToolSpray) && now-a.lastSpray >= 30 {
		redraw = true
	}
	if a.ui.hoverID != "" && !a.ui.tipShown && now-a.ui.tipStart >= 550 {
		redraw = true
	}
	if a.needAnim && now-a.lastAnim >= 70 {
		a.lastAnim = now
		redraw = true
	}
	return redraw
}

// DropFiles öffnet per Drag & Drop abgelegte Dateien.
func (a *App) DropFiles(paths []string) {
	if len(paths) > 0 {
		p := paths[0]
		a.later(func() { a.openPath(p) })
	}
}

// CanClose fragt bei ungesicherten Änderungen nach.
func (a *App) CanClose() bool {
	if !a.confirmDiscard() {
		return false
	}
	a.saveSettings()
	return true
}

// Cursor liefert die gewünschte Mauszeigerform.
func (a *App) Cursor() int {
	if a.modal != nil && (a.modalDragging || a.modalTitle.Contains(a.in.MX, a.in.MY)) {
		return CursorMove
	}
	if a.modal != nil || a.menuOpen >= 0 {
		return CursorArrow
	}
	if a.panning || (a.rCanvas.Contains(a.in.MX, a.in.MY) && (a.in.Space || a.tool == ToolHand)) {
		return CursorHand
	}
	if a.rCanvas.Contains(a.in.MX, a.in.MY) {
		if a.tool == ToolSelect {
			if h := a.selHandleAt(a.in.MX, a.in.MY); h >= 0 {
				if h == handleRotate {
					return CursorHand
				}
				return CursorSize
			}
			if a.sel.state != selNone && a.selContainsScreen(a.in.MX, a.in.MY) {
				return CursorMove
			}
		}
		return CursorCross
	}
	return CursorArrow
}

func (a *App) later(f func()) { a.deferred = append(a.deferred, f) }

// RunDeferred führt Aktionen aus, die Dialoge des Betriebssystems öffnen.
// Liefert true, wenn etwas ausgeführt wurde.
func (a *App) RunDeferred() bool {
	if len(a.deferred) == 0 {
		return false
	}
	for len(a.deferred) > 0 {
		f := a.deferred[0]
		a.deferred = a.deferred[1:]
		f()
	}
	// Maustasten gelten nach einem Dialog als losgelassen
	a.in.Down = [3]bool{}
	a.drag.active = false
	a.panning = false
	a.ui.active = ""
	return true
}

func (a *App) HasDeferred() bool { return len(a.deferred) > 0 }

// --- Status, Titel -----------------------------------------------------------

func (a *App) setStatus(s string) {
	a.status = s
	a.statusTime = a.now
}

func (a *App) updateTitle() {
	name := "Untitled"
	if a.filePath[a.page] != "" {
		name = filepath.Base(a.filePath[a.page])
	}
	mark := ""
	if a.dirty[a.page] {
		mark = "*"
	}
	pg := ""
	if a.page == 1 {
		pg = " [page 2]"
	}
	a.host.SetTitle(fmt.Sprintf("%s%s%s – %s", mark, name, pg, AppName))
}

// --- Rückgängig ----------------------------------------------------------------

func (a *App) pushUndo(snapshot *c64.Picture) {
	u := &a.undos[a.page]
	*u = append(*u, snapshot)
	if len(*u) > 100 {
		*u = (*u)[1:]
	}
	a.redos[a.page] = nil
	a.markDirty()
}

func (a *App) markDirty() {
	if !a.dirty[a.page] {
		a.dirty[a.page] = true
		a.updateTitle()
	}
}

func (a *App) undo() {
	a.commitOrCancelForHistory()
	u := &a.undos[a.page]
	if len(*u) == 0 {
		a.setStatus("Nothing to undo")
		return
	}
	last := (*u)[len(*u)-1]
	*u = (*u)[:len(*u)-1]
	a.redos[a.page] = append(a.redos[a.page], a.pic.Clone())
	a.restoreKeepingView(last)
	a.markDirty()
	a.setStatus("Undo")
}

func (a *App) redo() {
	a.commitOrCancelForHistory()
	r := &a.redos[a.page]
	if len(*r) == 0 {
		a.setStatus("Nothing to redo")
		return
	}
	last := (*r)[len(*r)-1]
	*r = (*r)[:len(*r)-1]
	a.undos[a.page] = append(a.undos[a.page], a.pic.Clone())
	a.restoreKeepingView(last)
	a.markDirty()
	a.setStatus("Redo")
}

func (a *App) commitOrCancelForHistory() {
	if a.sel.state == selFloating {
		a.selCommit()
	}
	if a.curve.stage != 0 {
		*a.pic = *a.curve.base
		a.curve.stage = 0
	}
	if a.drag.active && a.drag.base != nil {
		*a.pic = *a.drag.base
	}
	a.drag.active = false
}

// --- Malen -------------------------------------------------------------------

func (a *App) colorFor(btn int) uint8 {
	if btn == 1 {
		return a.fg2
	}
	return a.fg
}

func (a *App) otherColor(btn int) uint8 {
	if btn == 1 {
		return a.fg
	}
	return a.fg2
}

// plot setzt einen Bildpunkt nach den Multicolor-Regeln und merkt Konflikte.
func (a *App) plot(p *c64.Picture, x, y int, c uint8) {
	if x < 0 || y < 0 || x >= c64.W || y >= c64.H {
		return
	}
	if r := p.Plot(x, y, c, c64.ConflictMode(a.set.Conflict), a.pal); r != c64.PlotOK {
		a.flash[c64.CellOf(x, y)] = a.now
		a.needAnim = true
	}
}

// plotPat setzt einen Bildpunkt unter Berücksichtigung des Musters.
func (a *App) plotPat(p *c64.Picture, x, y, btn int) {
	if patBit(a.pattern, x, y) {
		a.plot(p, x, y, a.colorFor(btn))
	}
}

func (a *App) stamp() *Stamp {
	key := fmt.Sprintf("%d/%d/%v/%p", a.brushShape, a.brushSize, a.set.AspectBrush, a.custom)
	if a.stampCache == nil || a.stampKey != key {
		shape := a.brushShape
		if shape == BrushCustom && a.custom == nil {
			shape = BrushRound
		}
		a.stampCache = makeStamp(shape, a.brushSize, a.set.AspectBrush, a.custom)
		a.stampKey = key
	}
	return a.stampCache
}

// dab setzt einen Pinselabdruck.
func (a *App) dab(p *c64.Picture, x, y, btn int) {
	if a.tool == ToolDither {
		a.ditherDab(p, x, y)
		return
	}
	if a.tool == ToolErase {
		for _, o := range a.stamp().Offs {
			p.Erase(x+o.X, y+o.Y)
		}
		return
	}
	st := a.stamp()
	for i, o := range st.Offs {
		px, py := x+o.X, y+o.Y
		if st.Cols != nil && a.customOwnColor && btn == 0 {
			a.plot(p, px, py, uint8(st.Cols[i]))
			continue
		}
		a.plotPat(p, px, py, btn)
	}
}

func (a *App) lineDabs(p *c64.Picture, x0, y0, x1, y1, btn int) {
	forLine(x0, y0, x1, y1, func(x, y int) { a.dab(p, x, y, btn) })
}

func forLine(x0, y0, x1, y1 int, f func(x, y int)) {
	dx := absI(x1 - x0)
	dy := -absI(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	e := dx + dy
	for {
		f(x0, y0)
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

func absI(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (a *App) rand() uint32 {
	a.rnd ^= a.rnd << 13
	a.rnd ^= a.rnd >> 17
	a.rnd ^= a.rnd << 5
	return a.rnd
}

// --- Rahmen (Frame) ------------------------------------------------------------

// Frame verarbeitet die Eingaben und zeichnet das Fenster.
func (a *App) Frame() *gfx.Image {
	// Clicks, keys and the wheel change state while the frame is being drawn
	// (e.g. a palette click sets color 1 after the color display was drawn).
	// Draw a second time so the result is visible at once.
	hadInput := a.in.hasEvents()
	img := a.frameOnce()
	if hadInput {
		img = a.frameOnce()
	}
	return img
}

func (in *Input) hasEvents() bool {
	return in.Pressed != [3]bool{} || in.Released != [3]bool{} || in.Wheel != 0 ||
		len(in.Keys) > 0 || len(in.Chars) > 0 || in.Double
}

func (a *App) frameOnce() *gfx.Image {
	if a.img == nil {
		a.Resize(1280, 800)
	}
	a.ui.now = a.now
	a.ui.hoverID = ""
	a.ui.wheelUsed = false
	a.needAnim = false
	a.layout()
	if a.rCanvas.W != a.lastCW || a.rCanvas.H != a.lastCH {
		a.lastCW, a.lastCH = a.rCanvas.W, a.rCanvas.H
		if a.autoFit {
			a.fitPending = true
		} else {
			a.clampPan()
		}
	}
	if a.fitPending && a.rCanvas.W > 0 {
		a.zoomFit()
		a.fitPending = false
	}
	a.img.ResetClip()

	a.handleKeys()

	// Eingabesperre für die Grundfläche, solange Menü oder Dialog offen sind
	a.ui.blocked = a.modal != nil || a.menuOpen >= 0
	if !a.ui.blocked {
		a.canvasInput()
	} else if a.drag.active {
		a.canvasInput()
	}
	a.drawCanvas()
	a.drawPanel()
	a.drawToolbox()
	a.drawStatus()
	a.ui.blocked = a.modal != nil
	a.drawMenuBar()
	if a.menuOpen >= 0 {
		a.drawMenuDropdown()
	}
	if a.modal != nil {
		a.ui.blocked = false
		a.drawModal()
	}
	a.img.ResetClip()
	a.ui.drawTooltip()

	// Abschluss
	if !a.in.Down[0] && !a.in.Down[1] && !a.in.Down[2] {
		a.ui.active = ""
	}
	if a.sel.state != selNone {
		a.needAnim = true
	}
	a.in.endFrame()
	return a.img
}

func (a *App) layout() {
	const menuH, statusH, toolsW = 24, 24, 70
	panelW := 236
	if !a.set.ShowPanel {
		panelW = 0
	}
	a.rMenu = gfx.Rect{X: 0, Y: 0, W: a.W, H: menuH}
	a.rStatus = gfx.Rect{X: 0, Y: a.H - statusH, W: a.W, H: statusH}
	bodyY := menuH
	bodyH := a.H - menuH - statusH
	a.rPanel = gfx.Rect{X: 0, Y: bodyY, W: panelW, H: bodyH}
	a.rTools = gfx.Rect{X: a.W - toolsW, Y: bodyY, W: toolsW, H: bodyH}
	a.rCanvas = gfx.Rect{X: panelW, Y: bodyY, W: a.W - panelW - toolsW, H: bodyH}
}

// --- Statuszeile ----------------------------------------------------------------

func (a *App) drawStatus() {
	th := a.th
	r := a.rStatus
	a.img.Fill(r, th.Header)
	a.img.HLine(r.X, r.Right()-1, r.Y, th.Border)
	f := font.UI
	x := r.X + 8
	text := ""
	if a.status != "" && a.now-a.statusTime < 4000 {
		text = a.status
	} else {
		text = toolNames[a.tool] + ": " + toolHints[a.tool]
	}
	// rechts: Zoom, Palette, Konfliktmodus
	right := fmt.Sprintf("%s   ·   Zoom %d×   ·   %s   ·   %s", layerName(a.pic.Active), a.zoom, a.pal.Name, conflictNames[a.set.Conflict])
	rw := f.Width(right)
	a.img.TextL(f, gfx.Rect{X: r.Right() - rw - 10, Y: r.Y, W: rw, H: r.H}, right, th.TextDim)
	// Mitte: Position
	mid := ""
	if a.mouseIn {
		cell := c64.CellOf(a.mpx, a.mpy)
		bp := a.pic.Bits[a.mpy*c64.W+a.mpx]
		mid = fmt.Sprintf("X %3d  Y %3d   Cell %2d,%2d   %%%02b  %s", a.mpx, a.mpy, cell%c64.Cols, cell/c64.Cols, bp, c64.ColorNames[a.pic.ColorAt(a.mpx, a.mpy)])
	}
	midW := 330
	midX := r.Right() - rw - 24 - midW
	if mid != "" {
		a.img.TextL(font.Mono, gfx.Rect{X: midX, Y: r.Y, W: midW, H: r.H}, mid, th.Text)
	}
	text = L(text)
	a.img.TextL(f, gfx.Rect{X: x, Y: r.Y, W: midX - x - 10, H: r.H}, gfx.Elide(f, text, midX-x-10), th.Text)
}

var conflictNames = []string{"Conflict: block", "Conflict: replace", "Conflict: nearest color"}

// --- Tastatur -------------------------------------------------------------------

func (a *App) handleKeys() {
	if a.modal != nil {
		for _, k := range a.in.Keys {
			if k.Key == KeyEsc {
				a.closeModal(true)
				return
			}
			if h, ok := a.modal.(keyHandler); ok && a.modal != nil {
				h.Key(a, k)
			}
		}
		if h, ok := a.modal.(charHandler); ok {
			for _, c := range a.in.Chars {
				h.Char(a, c)
			}
		}
		return
	}
	for _, k := range a.in.Keys {
		if a.menuOpen >= 0 {
			if k.Key == KeyEsc {
				a.menuOpen = -1
			}
			continue
		}
		m := k.Mods
		switch {
		case m.Ctrl && k.Key == 'Z' && !m.Shift:
			a.undo()
		case m.Ctrl && (k.Key == 'Y' || (k.Key == 'Z' && m.Shift)):
			a.redo()
		case m.Ctrl && k.Key == 'N':
			a.later(a.cmdNew)
		case m.Ctrl && k.Key == 'O':
			a.later(a.cmdOpen)
		case m.Ctrl && k.Key == 'S' && m.Shift:
			a.later(func() { a.cmdSaveAs() })
		case m.Ctrl && k.Key == 'S':
			a.later(func() { a.cmdSave() })
		case m.Ctrl && k.Key == 'I':
			a.later(a.cmdImport)
		case m.Ctrl && k.Key == 'E':
			a.later(func() { a.cmdExport(exportPRG) })
		case m.Ctrl && k.Key == 'C':
			a.selCopy()
		case m.Ctrl && k.Key == 'X':
			a.selCut()
		case m.Ctrl && k.Key == 'V':
			a.selPaste()
		case m.Ctrl && k.Key == 'A':
			a.selectAll()
		case m.Ctrl && k.Key == 'Q':
			a.later(a.cmdQuit)
		case m.Ctrl && k.Key == '0':
			a.zoomFit()
		case m.Ctrl && k.Key == '1':
			a.setZoomCentered(1)
		case m.Ctrl && (k.Key == KeyPlus):
			a.zoomStep(1, -1, -1)
		case m.Ctrl && (k.Key == KeyMinus):
			a.zoomStep(-1, -1, -1)
		case k.Key == KeyF1:
			a.openHelp("")
		case k.Key == KeyEsc:
			a.escape()
		case k.Key == KeyEnter:
			if a.sel.state == selFloating {
				a.selCommit()
			} else if a.curve.stage == 2 {
				a.curveCommit()
			}
		case k.Key == KeyDelete || k.Key == KeyBackspace:
			a.selDelete()
		case k.Key == KeyLeft:
			a.nudge(-1, 0, m)
		case k.Key == KeyRight:
			a.nudge(1, 0, m)
		case k.Key == KeyUp:
			a.nudge(0, -1, m)
		case k.Key == KeyDown:
			a.nudge(0, 1, m)
		case k.Key == KeyTab:
			a.set.ShowPanel = !a.set.ShowPanel
		case k.Key == KeyHome:
			a.zoomFit()
		}
	}
	if a.menuOpen >= 0 {
		return
	}
	for _, c := range a.in.Chars {
		a.charShortcut(c)
	}
}

type keyHandler interface{ Key(a *App, k KeyEvent) }
type charHandler interface{ Char(a *App, r rune) }

func (a *App) charShortcut(c rune) {
	switch c {
	case 's':
		a.setTool(ToolDots)
	case 'd':
		a.setTool(ToolDraw)
	case 'D':
		a.setTool(ToolDither)
	case 'S':
		a.setTool(ToolSmear)
	case 'w':
		a.setTool(ToolErase)
	case 'F':
		a.setTool(ToolGradient)
		a.later(a.openGradientDialog)
	case '0':
		a.zoomFit()
	case '2', '3', '4':
		a.setActiveLayer(int(c - '2'))
	case 'A':
		a.later(a.openAutoDither)
	case 'v':
		a.setTool(ToolLine)
	case 'q':
		a.setTool(ToolCurve)
	case 'f':
		a.setTool(ToolFill)
	case 'a':
		a.setTool(ToolSpray)
	case 'r':
		a.setTool(ToolRect)
	case 'R':
		a.setTool(ToolRectF)
	case 'c':
		a.setTool(ToolCircle)
	case 'C':
		a.setTool(ToolCircleF)
	case 'e':
		a.setTool(ToolEllipse)
	case 'E':
		a.setTool(ToolEllipseF)
	case 'b':
		a.setTool(ToolSelect)
	case 'B':
		a.restoreBrush()
	case ',':
		a.setTool(ToolPicker)
	case 'g':
		a.set.GridOn = !a.set.GridOn
	case 'G':
		a.openGridDialog()
	case 't':
		a.tpl.visible = !a.tpl.visible
	case 'u':
		a.undo()
	case 'U':
		a.redo()
	case 'j':
		a.swapPage()
	case 'K':
		a.clearPicture()
	case '>':
		a.zoomStep(1, -1, -1)
	case '<':
		a.zoomStep(-1, -1, -1)
	case '+', '=':
		a.setBrushSize(a.brushSize + 1)
	case '-':
		a.setBrushSize(a.brushSize - 1)
	case '.':
		a.brushShape = BrushRound
		a.setBrushSize(1)
	case '[':
		a.fg = (a.fg + 15) & 15
	case ']':
		a.fg = (a.fg + 1) & 15
	case 'x':
		a.transformQuick(qFlipX)
	case 'y':
		a.transformQuick(qFlipY)
	case 'z':
		a.transformQuick(qRot90)
	case 'Z':
		a.transformQuick(qRot270)
	case 'h':
		a.transformQuick(qHalf)
	case 'H':
		a.transformQuick(qDouble)
	case 'p':
		a.openPaletteDialog()
	case 'm':
		a.set.PixelGrid = !a.set.PixelGrid
	case '?':
		a.openHelp("keys")
	}
}

func (a *App) escape() {
	switch {
	case a.curve.stage != 0:
		*a.pic = *a.curve.base
		a.curve.stage = 0
	case a.drag.active:
		if a.drag.base != nil {
			*a.pic = *a.drag.base
		}
		a.drag.active = false
	case a.sel.state == selFloating:
		a.selCancel()
	case a.sel.state != selNone:
		a.sel.state = selNone
	case a.tool == ToolCellSwap:
		a.stopCellSwap()
	}
}

func (a *App) nudge(dx, dy int, m Mods) {
	if a.sel.state == selFloating || a.sel.state == selSelected {
		step := 1
		if m.Shift {
			step = 4
		}
		a.selMove(dx*step, dy*step)
		return
	}
	step := 48
	if m.Shift {
		step = 160
	}
	a.panX -= dx * step
	a.panY -= dy * step
}

func (a *App) setTool(t int) {
	if t == a.tool {
		return
	}
	if a.curve.stage != 0 {
		a.curveCommit()
	}
	if a.sel.state == selFloating && t != ToolSelect {
		a.selCommit()
	}
	if t != ToolSelect && a.sel.state == selSelected {
		a.sel.state = selNone
	}
	if t != ToolPicker && t != ToolHand {
		a.prevTool = t
	}
	a.tool = t
}

func (a *App) setBrushSize(s int) {
	a.brushSize = clampI(s, 1, 16)
	if a.brushShape == BrushCustom {
		a.brushShape = BrushRound
	}
}

func (a *App) swapPage() {
	a.commitOrCancelForHistory()
	a.sel.state = selNone
	a.page = 1 - a.page
	a.pic = a.pages[a.page]
	a.syncExtraLayers()
	a.updateTitle()
	a.setStatus(fmt.Sprintf("Page %d (J switches back)", a.page+1))
}

func (a *App) copyToSpare() {
	a.commitOrCancelForHistory()
	other := 1 - a.page
	a.undos[other] = append(a.undos[other], a.pages[other].Clone())
	*a.pages[other] = *a.pic
	a.dirty[other] = true
	a.setStatus(fmt.Sprintf("Picture copied to page %d", other+1))
}

func (a *App) clearPicture() {
	a.commitOrCancelForHistory()
	a.sel.state = selNone
	a.pushUndo(a.pic.Clone())
	a.pic.Clear()
	a.setStatus("All layers cleared (background color kept)")
}

func (a *App) setBG(c uint8) {
	if a.pic.BG == c {
		return
	}
	a.commitOrCancelForHistory()
	a.pushUndo(a.pic.Clone())
	a.pic.SetBG(c)
	a.ditherRes = nil
	a.setStatus("Background color ($D021): " + c64.ColorNames[c])
}

func (a *App) setBorder(c uint8) {
	if a.pic.Border == c {
		return
	}
	a.pushUndo(a.pic.Clone())
	a.pic.Border = c
	a.setStatus("Border color ($D020): " + c64.ColorNames[c])
}

func (a *App) setPalette(i int) {
	if i < 0 || i >= len(c64.Palettes) {
		return
	}
	a.palIdx = i
	a.pal = &c64.Palettes[i]
	a.set.Palette = a.pal.Name
	a.saveSettings()
}

func (a *App) setConflict(m int) {
	a.set.Conflict = m
	a.saveSettings()
	a.setStatus(conflictNames[m])
}

func trimExt(p string) string { return strings.TrimSuffix(p, filepath.Ext(p)) }
