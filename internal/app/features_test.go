package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"c64dpainter/internal/c64"
)

func TestSquareAndPolygonTools(t *testing.T) {
	a, _ := setup(t)
	a.fg = 1
	a.brushShape, a.brushSize = BrushRound, 1
	a.Char('N') // filled square
	step(a)
	if a.tool != ToolSquareF {
		t.Fatalf("Shift+N: tool %d", a.tool)
	}
	x0, y0 := pc(a, 10, 10)
	x1, y1 := pc(a, 40, 40) // square on screen: 30 × 60 pixels
	tdrag(a, x0, y0, x1, y1, 0)
	if a.pic.ColorAt(35, 65) != 1 || a.pic.ColorAt(45, 30) == 1 || a.pic.ColorAt(35, 75) == 1 {
		t.Fatal("filled square wrong")
	}
	for _, c := range []struct {
		key  rune
		tool int
	}{{'i', ToolTriangle}, {'I', ToolTriangleF}, {'o', ToolHexagon}, {'O', ToolHexagonF}, {'l', ToolStar}, {'L', ToolStarF}, {'n', ToolSquare}} {
		a.Char(c.key)
		step(a)
		if a.tool != c.tool {
			t.Fatalf("key %c: tool %d", c.key, a.tool)
		}
	}
	// filled hexagon around (100, 100), upright with Shift
	a.Char('O')
	step(a)
	a.fg = 2
	cx, cy := pc(a, 100, 100)
	ex, ey := pc(a, 100, 130)
	tdrag(a, cx, cy, ex, ey, 0)
	if a.pic.ColorAt(100, 100) != 2 || a.pic.ColorAt(100, 125) != 2 || a.pic.ColorAt(100, 140) == 2 {
		t.Fatal("filled hexagon wrong")
	}
	// outlined star: the center stays empty
	a.Char('l')
	step(a)
	a.fg = 7
	cx, cy = pc(a, 40, 150)
	ex, ey = pc(a, 40, 120)
	tdrag(a, cx, cy, ex, ey, 0)
	if a.pic.ColorAt(40, 150) == 7 || a.pic.ColorAt(40, 121) != 7 {
		t.Fatalf("star outline wrong: %d %d", a.pic.ColorAt(40, 150), a.pic.ColorAt(40, 121))
	}
	// every toolbox button has an icon
	for _, tb := range a.toolButtons() {
		if iconData[tb.icon] == nil || (tb.alt != "" && iconData[tb.alt] == nil) {
			t.Fatalf("icon missing for %s", tb.id)
		}
	}
	if iconData["rect"][3] == iconData["square"][3] {
		t.Fatal("rectangle and square icons must differ")
	}
}

func TestSymmetryAndShade(t *testing.T) {
	a, _ := setup(t)
	a.Char('X')
	step(a)
	if !a.set.Sym.On {
		t.Fatal("Shift+X switches symmetry on")
	}
	a.fg = 5
	a.brushShape, a.brushSize = BrushRound, 1
	a.Char('s')
	step(a)
	x, y := pc(a, 10, 20)
	tclick(a, x, y, 0)
	if a.pic.ColorAt(10, 20) != 5 || a.pic.ColorAt(149, 20) != 5 {
		t.Fatal("left ↔ right symmetry")
	}
	a.set.Sym.Mode = 2
	x, y = pc(a, 30, 40)
	tclick(a, x, y, 0)
	for _, p := range [][2]int{{30, 40}, {129, 40}, {30, 159}, {129, 159}} {
		if a.pic.ColorAt(p[0], p[1]) != 5 {
			t.Fatalf("four-fold symmetry missing at %v", p)
		}
	}
	a.Char('X')
	step(a)
	// shade: grey area, left lightens one step, right darkens
	for yy := 60; yy < 80; yy++ {
		for xx := 60; xx < 100; xx++ {
			a.pic.Plot(xx, yy, 12, 0, a.pal)
		}
	}
	a.Char('W')
	step(a)
	if a.tool != ToolShade {
		t.Fatal("Shift+W selects the shade brush")
	}
	a.set.ShadeRamp = 0 // grey (contrast): black – grey – white
	a.brushShape, a.brushSize = BrushSquare, 4
	x0, y0 := pc(a, 64, 64)
	x1, y1 := pc(a, 64, 74)
	tdrag(a, x0, y0, x1, y1, 0) // one stroke over the same pixels: only one step
	if a.pic.ColorAt(64, 70) != 1 {
		t.Fatalf("lighten: %d", a.pic.ColorAt(64, 70))
	}
	x0, y0 = pc(a, 90, 64)
	x1, y1 = pc(a, 90, 74)
	tdrag(a, x0, y0, x1, y1, 1)
	if a.pic.ColorAt(90, 70) != 0 {
		t.Fatalf("darken: %d", a.pic.ColorAt(90, 70))
	}
	a.set.ShadeRamp = -1
	if a.shadeStep(0, 1) != 6 || a.shadeStep(1, 1) != 1 {
		t.Fatal("brightness order")
	}
}

func TestMirrorViewAndCRT(t *testing.T) {
	a, _ := setup(t)
	a.Char('M')
	step(a)
	if !a.mirrorView {
		t.Fatal("Shift+M mirrors the view")
	}
	a.fg = 2
	a.brushShape, a.brushSize = BrushRound, 1
	a.Char('s')
	step(a)
	// click where pixel 20 appears in the mirrored view
	x, y := pc(a, 20, 50)
	tclick(a, a.mirrorX(x), y, 0)
	if a.pic.ColorAt(20, 50) != 2 {
		t.Fatal("painting in the mirrored view")
	}
	a.Char('M')
	step(a)
	// CRT window: open, the canvas under it gets no clicks
	a.Char('V')
	step(a)
	if !a.crt.open {
		t.Fatal("Shift+V opens the CRT preview")
	}
	r := a.crtRect()
	before := a.pic.Clone()
	tclick(a, r.X+r.W/2, r.Y+r.H-20, 0)
	if countDiff(before, a.pic, 0, 0, 159, 199) != 0 {
		t.Fatal("click into the CRT window painted on the picture")
	}
	if a.crt.out == nil || len(a.crt.out) != 384*272*a.crtScale()*a.crtScale() {
		t.Fatal("CRT picture not rendered")
	}
	for _, on := range []bool{false, true} {
		for s := 1; s <= 3; s++ {
			if len(crtRender(a.crtSource(), s, on)) != 384*272*s*s {
				t.Fatal("crtRender size")
			}
		}
	}
	a.Char('V')
	step(a)
}

func TestSelectionCopiesByDefault(t *testing.T) {
	a, _ := setup(t)
	for y := 10; y < 20; y++ {
		for x := 10; x < 18; x++ {
			a.pic.Plot(x, y, 7, 0, a.pal)
		}
	}
	a.Char('b')
	step(a)
	x0, y0 := pc(a, 10, 10)
	x1, y1 := pc(a, 17, 19)
	tdrag(a, x0, y0, x1, y1, 0)
	sx, sy := pc(a, 12, 12)
	tx, ty := pc(a, 52, 12)
	tdrag(a, sx, sy, tx, ty, 0)
	a.KeyDown(KeyEnter, Mods{})
	step(a)
	if a.pic.ColorAt(12, 12) != 7 || a.pic.ColorAt(52, 12) != 7 {
		t.Fatal("dragging a selection keeps the original")
	}
	// with Shift the original is cut out
	x0, y0 = pc(a, 50, 10)
	x1, y1 = pc(a, 57, 19)
	tdrag(a, x0, y0, x1, y1, 0)
	sx, sy = pc(a, 52, 12)
	tx, ty = pc(a, 92, 12)
	a.MouseMove(sx, sy)
	step(a)
	a.in.Mods.Shift = true
	a.MouseDown(0, sx, sy, false)
	step(a)
	a.in.Mods.Shift = false
	for i := 1; i <= 8; i++ {
		a.MouseMove(sx+(tx-sx)*i/8, sy)
		step(a)
	}
	a.MouseUp(0, tx, ty)
	step(a)
	a.KeyDown(KeyEnter, Mods{})
	step(a)
	if a.pic.ColorAt(52, 12) == 7 || a.pic.ColorAt(92, 12) != 7 {
		t.Fatal("Shift+drag moves the selection")
	}
}

func TestQuickSaveNumbered(t *testing.T) {
	a, _ := setup(t)
	dir := t.TempDir()
	a.set.WorkDir = dir
	os.WriteFile(filepath.Join(dir, "picture_0007.png"), []byte("x"), 0o644)
	a.KeyDown(KeyF2, Mods{})
	step(a)
	a.KeyDown(KeyF2, Mods{})
	step(a)
	for _, n := range []string{"picture_0008.png", "picture_0009.png"} {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil || len(b) < 100 {
			t.Fatalf("%s not saved", n)
		}
	}
	a.filePath[a.page] = filepath.Join(dir, "castle.kla")
	if p := nextQuickPath(dir, a.quickBase()); filepath.Base(p) != "castle_0001.png" {
		t.Fatalf("name: %s", p)
	}
	_ = c64.W
}

func TestBrushFilesAndMyBrushes(t *testing.T) {
	a, h := setup(t)
	// a two-color brush from a selection
	for y := 10; y < 20; y++ {
		for x := 10; x < 18; x++ {
			c := uint8(2)
			if x >= 14 {
				c = 7
			}
			a.pic.Plot(x, y, c, 0, a.pal)
		}
	}
	a.Char('b')
	step(a)
	x0, y0 := pc(a, 10, 10)
	x1, y1 := pc(a, 17, 19)
	tdrag(a, x0, y0, x1, y1, 0)
	a.selToBrush()
	step(a)
	dir := t.TempDir()
	a.set.WorkDir = dir
	h.save = filepath.Join(dir, "brushes", "mine.png")
	a.saveBrushDialog()
	data, err := os.ReadFile(h.save)
	if err != nil {
		t.Fatal("brush not saved")
	}
	img, _ := png.Decode(bytes.NewReader(data))
	if img.Bounds().Dx() != a.custom.W*2 {
		t.Fatal("brush PNG must have double-wide pixels")
	}
	b, err := decodeBrush(img, a.pal)
	if err != nil || b.W != a.custom.W || b.H != a.custom.H {
		t.Fatal("brush round trip")
	}
	for i, v := range b.Pix {
		if v != a.custom.Pix[i] {
			t.Fatal("brush pixels")
		}
	}
	a.restoreBrush()
	a.openStamps()
	step(a)
	d := a.modal.(*myBrushesDialog)
	if len(d.mine) != 1 || d.mine[0].name != "mine" {
		t.Fatal("My brushes lists the saved brush")
	}
	d.sel = 0
	a.KeyDown(KeyEnter, Mods{})
	step(a)
	if a.modal != nil || a.brushShape != BrushCustom || !a.customOwnColor || a.tool != ToolDots {
		t.Fatal("my brush must become the brush")
	}
	_ = strings.Contains
	// the gradient tabs
	for g := range gradientGroups {
		for i := range groupRamps(g) {
			c := gradientColors(g, i, false)
			if len(c) < 3 || len(c) > 5 {
				t.Fatal("ramp length")
			}
		}
	}
	if len(gradientRamps4[0].cols) != 4 || len(gradientRamps5[0].cols) != 5 {
		t.Fatal("4 and 5 color ramps")
	}
}

func TestAutoBackupAndRecent(t *testing.T) {
	a, h := setup(t)
	dir := t.TempDir()
	a.set.WorkDir = dir
	if a.autoBackup {
		t.Fatal("auto backup is off at the start")
	}
	a.toggleAutoBackup()
	a.pic.Plot(5, 5, 2, 0, a.pal)
	a.Tick(a.now + backupInterval + 1)
	p := filepath.Join(dir, "Backup.c64dp")
	data, err := os.ReadFile(p)
	if err != nil || !isProject(data) {
		t.Fatal("Backup.c64dp not written")
	}
	st, _ := os.Stat(p)
	// no change: no new backup
	os.Remove(p)
	a.Tick(a.now + backupInterval + 1)
	if _, err := os.Stat(p); err == nil {
		t.Fatal("backup without changes")
	}
	_ = st
	// recent files
	h.save = filepath.Join(dir, "pic.kla")
	a.cmdSaveAs()
	if len(a.set.Recent) == 0 || filepath.Base(a.set.Recent[0]) != "pic.kla" {
		t.Fatalf("recent after save: %v", a.set.Recent)
	}
	found := false
	for _, it := range a.menus[0].Items {
		if strings.HasPrefix(it.Label, "pic.kla") {
			found = true
		}
	}
	if !found {
		t.Fatal("recent file in the File menu")
	}
	a.openRecent(filepath.Join(dir, "gone.kla"))
	for _, r := range a.set.Recent {
		if filepath.Base(r) == "gone.kla" {
			t.Fatal("missing file stays in the list")
		}
	}
}

func TestFixDoubleExt(t *testing.T) {
	for in, want := range map[string]string{
		"/a/pic.kla.kla": "/a/pic.kla", "/a/pic.KLA.kla": "/a/pic.KLA", "/a/pic.kla": "/a/pic.kla",
		"/a/pic.c64dp.kla": "/a/pic.c64dp.kla", "/a/pic": "/a/pic", "/a/x.png.png.png": "/a/x.png",
	} {
		if got := FixDoubleExt(in); got != want {
			t.Fatalf("%s: %s", in, got)
		}
	}
}

func TestCollapsibleSections(t *testing.T) {
	a, _ := setup(t)
	before := a.panelContentH
	// the header of "Patterns": find it by folding through the API path
	a.collapsed = map[string]bool{"Patterns": true, "Colors": true, "Layers": true, "Cell under the mouse": true, "Navigator": true}
	step(a)
	if a.panelContentH >= before-300 {
		t.Fatalf("folded sections must save space: %d → %d", before, a.panelContentH)
	}
	// a click on a header opens it again
	a.collapsed = nil
	step(a)
	full := a.panelContentH
	y := a.rPanel.Y
	// Brush is the first section: click its header
	tclick(a, a.rPanel.X+40, y+12, 0)
	if !a.collapsed["Brush"] || a.panelContentH >= full {
		t.Fatal("clicking the header folds the section")
	}
	tclick(a, a.rPanel.X+40, y+12, 0)
	if a.collapsed["Brush"] {
		t.Fatal("second click opens it again")
	}
}

func TestSwapToBackground(t *testing.T) {
	a, _ := setup(t)
	for x := 0; x < 20; x++ {
		a.pic.Plot(x, 10, 2, 0, a.pal)
	}
	a.openSwapDialog(false)
	d := a.modal.(*swapDialog)
	d.from, d.to, d.both = 2, a.pic.BG, true
	d.apply(a)
	if a.pic.ColorAt(5, 10) != a.pic.BG {
		t.Fatal("red must become background")
	}
	a.setBG(6)
	if a.pic.ColorAt(5, 10) != 6 {
		t.Fatal("the swapped pixels follow the background color")
	}
	a.openSwapDialog(false)
	d = a.modal.(*swapDialog)
	d.from = a.pic.BG
	if d.ok(a) {
		t.Fatal("the background must not be replaced")
	}
}

func TestCustomGradient(t *testing.T) {
	a, _ := setup(t)
	a.openGradientDialog()
	d := a.modal.(*gradientDialog)
	d.g.Group = 1 // 4 colors
	d.g.Ramp = len(groupRamps(1)) - 1
	step(a)
	if _, ok := customIndex(1, d.g.Ramp); !ok {
		t.Fatal("custom is the last ramp")
	}
	// set the custom 4-color ramp to 7, 3, 14, 6
	for i, c := range []int{7, 3, 14, 6} {
		d.cslot = i
		customRamps[1][i] = uint8(c)
	}
	a.set.Gradient.storeCustomRamps()
	a.saveSettings()
	b := New(&recHost{dir: t.TempDir()})
	_ = b
	got := gradientColors(1, len(groupRamps(1))-1, false)
	if len(got) != 4 || got[0] != 7 || got[3] != 6 {
		t.Fatalf("custom ramp: %v", got)
	}
	s := loadSettings()
	if len(s.Gradient.Custom4) != 4 || s.Gradient.Custom4[1] != 3 {
		t.Fatal("custom ramp must be saved in the settings")
	}
	if len(groupRamps(0)) != len(gradientRamps)+1 || len(groupRamps(3)) != len(gradientRamps)+1 {
		t.Fatal("custom 3-color ramp in the three-color based tabs")
	}
}

func TestTextBrush(t *testing.T) {
	a, _ := setup(t)
	a.openTextDialog()
	d := a.modal.(*textDialog)
	d.tf.Set("")
	for _, r := range "Hi 64" {
		a.Char(r)
	}
	step(a)
	if d.tf.String() != "Hi 64" {
		t.Fatalf("typed text: %q", d.tf.String())
	}
	// cursor editing: Home, then type at the start
	a.KeyDown(KeyHome, Mods{})
	a.Char('X')
	step(a)
	if d.tf.String() != "XHi 64" {
		t.Fatalf("insert at the start: %q", d.tf.String())
	}
	a.KeyDown(KeyBackspace, Mods{})
	step(a)
	for f := 0; f < 3; f++ {
		for e := 0; e < 3; e++ {
			a.set.Text.Font, a.set.Text.Effect = f, e
			b := d.brush(a)
			if b == nil || b.W < 10 {
				t.Fatalf("font %d effect %d: no brush", f, e)
			}
			colors := map[int8]bool{}
			for _, v := range b.Pix {
				if v >= 0 {
					colors[v] = true
				}
			}
			if e == 0 && len(colors) != 1 || e > 0 && len(colors) != 2 {
				t.Fatalf("font %d effect %d: %d colors", f, e, len(colors))
			}
		}
	}
	a.set.Text.Font, a.set.Text.Effect = 0, 1
	a.KeyDown(KeyEnter, Mods{})
	step(a)
	if a.modal != nil || a.brushShape != BrushCustom || a.set.Text.Last != "Hi 64" {
		t.Fatal("Enter uses the text as brush")
	}
	if a.custom.W != 5*8+2 || a.custom.H != 10 {
		t.Fatalf("wide font size: %d×%d", a.custom.W, a.custom.H)
	}
}

func TestPetsciiEditor(t *testing.T) {
	a, h := setup(t)
	for x := 0; x < 80; x++ {
		for y := 0; y < 100; y++ {
			a.pic.Plot(x, y, 7, 0, a.pal)
		}
	}
	a.openPetscii()
	step(a)
	e := a.pet
	if !e.open {
		t.Fatal("editor must be open")
	}
	// convert the picture
	a.petOpenConvert()
	step(a)
	d := a.modal.(*petConvertDialog)
	if d.preview == nil {
		t.Fatal("no preview")
	}
	d.apply(a)
	if e.scr.ColorAt(10, 10) != 7 || e.scr.ColorAt(300, 190) != a.pic.BG {
		t.Fatalf("converted picture: %d %d", e.scr.ColorAt(10, 10), e.scr.ColorAt(300, 190))
	}
	// draw a character with the mouse
	e.tool, e.ch, e.color = petDraw, 81, 2
	step(a)
	cx := e.canvas.X + 30*8*e.zoom + 4
	cy := e.canvas.Y + 20*8*e.zoom + 4
	tclick(a, cx, cy, 0)
	if e.scr.Chars[20*40+30] != 81 || e.scr.Colors[20*40+30] != 2 {
		t.Fatal("draw tool")
	}
	e.doUndo()
	if e.scr.Chars[20*40+30] == 81 {
		t.Fatal("undo")
	}
	e.doRedo()
	// text tool
	e.tool = petText
	tclick(a, e.canvas.X+2, e.canvas.Y+24*8*e.zoom+2, 0)
	for _, r := range "HI" {
		a.Char(r)
	}
	step(a)
	if e.scr.Chars[24*40] != 8 || e.scr.Chars[24*40+1] != 9 {
		t.Fatalf("text tool: %d %d", e.scr.Chars[24*40], e.scr.Chars[24*40+1])
	}
	// keys do not reach the paint program while the editor is open
	tool := a.tool
	e.tool = petDraw
	e.cursor = -1
	a.Char('c')
	step(a)
	if a.tool != tool {
		t.Fatal("paint shortcuts must not work in the PETSCII editor")
	}
	// exports
	dir := t.TempDir()
	a.set.WorkDir = dir
	for _, k := range []string{"prg", "asm", "png"} {
		h.save = filepath.Join(dir, "pic."+k)
		a.petExport(k)
		if b, err := os.ReadFile(h.save); err != nil || len(b) < 100 {
			t.Fatalf("export %s", k)
		}
	}
	h.save = filepath.Join(dir, "pic.petmate")
	a.petSave()
	h.open = h.save
	e.scr.Chars[0] = 1
	a.petOpen()
	if e.scr.Chars[0] == 1 || e.path == "" {
		t.Fatal("open petmate file")
	}
	a.closePetscii()
	step(a)
	if e.open {
		t.Fatal("close")
	}
}

func TestSpriteEditor(t *testing.T) {
	a, h := setup(t)
	// two "sprites" side by side in the picture: yellow blocks with a red line
	for y := 0; y < 21; y++ {
		for x := 0; x < 12; x++ {
			a.pic.Plot(x, y, 7, 0, a.pal)
			a.pic.Plot(x+12, y, 7, 0, a.pal)
		}
	}
	for x := 0; x < 24; x++ {
		a.pic.Plot(x, 5, 2, 0, a.pal)
	}
	a.toggleSpriteGrid()
	if a.set.GridW != 24 || a.set.GridH != 21 {
		t.Fatal("sprite grid")
	}
	a.openSprites()
	step(a)
	e := a.spr
	if !e.open {
		t.Fatal("editor must be open")
	}
	a.sprOpenImport()
	step(a)
	d := a.modal.(*sprImportDialog)
	d.o.Cols = 2
	step(a)
	d.apply(a)
	if len(e.bank.Sprites) != 2 || !e.bank.Sprites[0].Multi {
		t.Fatalf("import: %d sprites", len(e.bank.Sprites))
	}
	if c := e.bank.ColorAt(&e.bank.Sprites[1], 0, 0); c != 7 {
		t.Fatalf("imported color %d", c)
	}
	if c := e.bank.ColorAt(&e.bank.Sprites[1], 0, 5); c != 2 {
		t.Fatalf("imported line color %d", c)
	}
	// draw with the mouse: slot 0 (transparent) with the right button
	e.cur = 0
	e.tool = sprPencil
	step(a)
	sz := e.z * 2
	tclick(a, e.canvas.X+3*sz+1, e.canvas.Y+10*e.z+1, 1)
	if e.sp().Get(3, 10) != 0 {
		t.Fatal("erase with the right button")
	}
	e.doUndo()
	if e.sp().Get(3, 10) == 0 {
		t.Fatal("undo")
	}
	// hires and operations
	a.Char('c')
	step(a)
	if e.sp().Multi {
		t.Fatal("C switches to hires")
	}
	a.Char('x')
	step(a)
	e.addSprite(true)
	if len(e.bank.Sprites) != 3 || e.cur != 1 {
		t.Fatal("duplicate")
	}
	// files
	dir := t.TempDir()
	a.set.WorkDir = dir
	for _, k := range []string{"asm", "bin", "png"} {
		h.save = filepath.Join(dir, "spr."+k)
		a.sprExport(k)
		if b, err := os.ReadFile(h.save); err != nil || len(b) < 100 {
			t.Fatalf("export %s", k)
		}
	}
	h.save = filepath.Join(dir, "spr.c64sprites")
	a.sprSave()
	e.bank.Sprites = e.bank.Sprites[:1]
	a.openPath(h.save)
	if len(e.bank.Sprites) != 3 || e.path == "" || !e.open {
		t.Fatal("open sprite project")
	}
	// use a multicolor sprite as brush
	e.cur = 2
	e.sp().Multi = true
	a.sprUseAsBrush()
	if e.open || a.custom == nil || a.custom.W != 12 {
		t.Fatal("use as brush")
	}
}

func TestFontEditor(t *testing.T) {
	a, h := setup(t)
	a.openFontEditor()
	step(a)
	e := a.fnt
	if !e.open || e.f.Data[8] != c64.CharsetUpper[8] {
		t.Fatal("editor must start with the C64 font")
	}
	// draw: clicking a set pixel clears it
	e.cur = 1 // A
	step(a)
	z := e.z
	set := e.f.Pixel(1, 3, 1)
	tclick(a, e.canvas.X+3*z+1, e.canvas.Y+1*z+1, 0)
	if e.f.Pixel(1, 3, 1) == set {
		t.Fatal("pencil")
	}
	e.doUndo()
	if e.f.Pixel(1, 3, 1) != set {
		t.Fatal("undo")
	}
	// typing in the test text does not trigger shortcuts
	e.tf.focus = true
	e.tf.cur = len(e.tf.text)
	a.Char('x')
	step(a)
	if !strings.HasSuffix(e.tf.String(), "x") {
		t.Fatal("test text")
	}
	e.tf.focus = false
	// 2×2 tile and import from the picture
	e.f.Tile = 3
	step(a)
	for x := 0; x < 4; x++ {
		a.pic.Plot(x, 0, 1, 0, a.pal)
	}
	a.fntOpenImport()
	step(a)
	d := a.modal.(*fntImportDialog)
	d.o.Count, d.o.Target = 1, 200
	d.apply(a)
	if e.f.Data[200*8] != 0xFF || e.cur != 200 {
		t.Fatalf("import: %02x", e.f.Data[200*8])
	}
	dir := t.TempDir()
	a.set.WorkDir = dir
	for _, k := range []string{"asm", "bin", "png"} {
		h.save = filepath.Join(dir, "font."+k)
		a.fntExport(k)
		if b, err := os.ReadFile(h.save); err != nil || len(b) < 100 {
			t.Fatalf("export %s", k)
		}
	}
	h.save = filepath.Join(dir, "font.c64font")
	a.fntSave()
	e.f.Data[200*8] = 0
	h.open = h.save
	a.fntOpen()
	if e.f.Data[200*8] != 0xFF || e.f.Tile != 3 {
		t.Fatal("open font project")
	}
	h.open = filepath.Join(dir, "font.bin")
	a.fntOpen()
	if e.f.Data[200*8] != 0xFF || e.path != "" {
		t.Fatal("open raw font")
	}
	a.closeFontEditor()
	step(a)
	if e.open {
		t.Fatal("close")
	}
}

func TestSelectionBrushColor2Transparent(t *testing.T) {
	a, _ := setup(t)
	// a red square on a yellow area
	for y := 0; y < 20; y++ {
		for x := 0; x < 10; x++ {
			c := uint8(7)
			if x >= 3 && x < 7 && y >= 5 && y < 15 {
				c = 2
			}
			a.pic.Plot(x, y, c, 0, a.pal)
		}
	}
	a.fg, a.fg2 = 1, 7 // color 2 = yellow
	a.setTool(ToolSelect)
	a.sel.state = selSelected
	a.sel.x0, a.sel.y0, a.sel.x1, a.sel.y1 = 0, 0, 9, 19
	a.selToBrush()
	b := a.custom
	if b == nil || b.W != 10 || b.H != 20 {
		t.Fatal("brush")
	}
	if b.Pix[0] != -1 || b.Pix[10*10+4] != 2 {
		t.Fatalf("color 2 must be transparent: %d %d", b.Pix[0], b.Pix[10*10+4])
	}
	// stamping keeps the picture around the motif
	for y := 100; y < 130; y++ {
		for x := 100; x < 120; x++ {
			a.pic.Plot(x, y, 5, 0, a.pal)
		}
	}
	step(a)
	px, py := pc(a, 110, 115)
	tclick(a, px, py, 0)
	if a.pic.ColorAt(105, 105) != 5 || a.pic.ColorAt(110, 115) != 2 {
		t.Fatalf("stamp: %d %d", a.pic.ColorAt(105, 105), a.pic.ColorAt(110, 115))
	}
}

func TestPerspective(t *testing.T) {
	src := &CustomBrush{W: 20, H: 10, Pix: make([]int8, 200)}
	for i := range src.Pix {
		src.Pix[i] = 2
	}
	p := &perspective{}
	p.drag(hpTL, p.off, 5) // opposite: TR moves 5 to the left
	if p.off != [4]float64{5, -5, 0, 0} {
		t.Fatalf("opposite drag: %v", p.off)
	}
	out, dx, dy := warpPerspective(src, p)
	if dx != 0 || dy != 0 || out.W != 20 || out.H != 10 {
		t.Fatalf("warp size %d×%d at %d,%d", out.W, out.H, dx, dy)
	}
	row := func(y int) (int, int) {
		x0, x1 := -1, -1
		for x := 0; x < out.W; x++ {
			if out.Pix[y*out.W+x] >= 0 {
				if x0 < 0 {
					x0 = x
				}
				x1 = x
			}
		}
		return x0, x1
	}
	if x0, x1 := row(0); x0 < 4 || x0 > 6 || x1 < 13 || x1 > 15 {
		t.Fatalf("top row %d..%d", x0, x1)
	}
	if x0, x1 := row(9); x0 != 0 || x1 != 19 {
		t.Fatalf("bottom row %d..%d", x0, x1)
	}
	// free + middle handle; vertical mode
	p = &perspective{free: true}
	p.drag(hpBR, p.off, 3)
	p.drag(hpMidA, p.off, -2)
	if p.off != [4]float64{-2, -2, 3, 0} {
		t.Fatalf("free drag: %v", p.off)
	}
	p = &perspective{vert: true}
	p.drag(hpTR, p.off, 4)
	if p.off != [4]float64{0, 4, -4, 0} {
		t.Fatalf("vertical opposite: %v", p.off)
	}

	// in the program: selection, perspective handles, drag
	a, _ := setup(t)
	for y := 20; y < 60; y++ {
		for x := 20; x < 60; x++ {
			a.pic.Plot(x, y, 7, 0, a.pal)
		}
	}
	a.setTool(ToolSelect)
	a.sel.state = selSelected
	a.sel.x0, a.sel.y0, a.sel.x1, a.sel.y1 = 20, 20, 59, 59
	a.sel.persp.on = true
	a.selEnsureFloating()
	step(a)
	hs := a.perspHandles()
	if a.selHandleAt(hs[0][0], hs[0][1]) != hpTL {
		t.Fatal("perspective handle")
	}
	tx, ty := hs[0][0], hs[0][1]
	tdrag(a, tx, ty, tx+10*2*a.zoom, ty, 0)
	if a.sel.persp.off[0] != 10 || a.sel.persp.off[1] != -10 {
		t.Fatalf("drag in the program: %v", a.sel.persp.off)
	}
	a.selCommit()
	if a.pic.ColorAt(22, 20) == 7 || a.pic.ColorAt(40, 20) != 7 || a.pic.ColorAt(21, 59) != 7 {
		t.Fatalf("result: %d %d %d", a.pic.ColorAt(22, 20), a.pic.ColorAt(40, 20), a.pic.ColorAt(21, 59))
	}
}

func TestFontEditorMulticolor(t *testing.T) {
	a, _ := setup(t)
	a.openFontEditor()
	step(a)
	e := a.fnt
	e.cur = 0
	a.Char('c') // multicolor
	step(a)
	if !e.f.Multi || e.f.FG > 7 {
		t.Fatal("C switches to multicolor")
	}
	e.f.Data = [c64.CharsetSize]byte{}
	a.Char('2')
	step(a)
	if e.slot != 2 {
		t.Fatal("color key")
	}
	tclick(a, e.canvas.X+1*e.zx+1, e.canvas.Y+3*e.z+1, 0)
	if e.f.Get(0, 1, 3) != 2 {
		t.Fatalf("draw multicolor pixel: %d", e.f.Get(0, 1, 3))
	}
	tclick(a, e.canvas.X+1*e.zx+1, e.canvas.Y+3*e.z+1, 1)
	if e.f.Get(0, 1, 3) != 0 {
		t.Fatal("right button: background")
	}
	if e.canRotate() {
		t.Fatal("no rotation in multicolor")
	}
}

func TestHiresMode(t *testing.T) {
	a, h := setup(t)
	a.cmdNew()
	step(a)
	d, ok := a.modal.(*modeDialog)
	if !ok {
		t.Fatal("New asks for the mode")
	}
	d.choose(a, c64.ModeHires)
	step(a)
	if a.pic.Mode != c64.ModeHires || picW != 320 || asp != 1 {
		t.Fatal("hires picture")
	}
	if a.modal != nil {
		a.closeModal(false) // working folder (not shown with this host)
	}
	// draw a line with the mouse at hires pixels
	a.fg = 2
	a.setTool(ToolLine)
	step(a)
	x0, y0 := pc(a, 100, 50)
	x1, y1 := pc(a, 140, 50)
	tdrag(a, x0, y0, x1, y1, 0)
	if a.pic.ColorAt(100, 50) != 2 || a.pic.ColorAt(140, 50) != 2 || a.pic.ColorAt(141, 50) == 2 {
		t.Fatalf("hires line: %d %d %d", a.pic.ColorAt(100, 50), a.pic.ColorAt(140, 50), a.pic.ColorAt(141, 50))
	}
	// a third color in a cell is blocked (cell 12,6: paper + red)
	a.fg = 5
	a.setTool(ToolDraw)
	step(a)
	px, py := pc(a, 101, 51)
	tclick(a, px, py, 0)
	if a.pic.ColorAt(101, 51) == 5 {
		t.Fatal("hires rule: third color must be blocked")
	}
	// fill, selection and perspective work in hires coordinates
	a.fg = 7
	a.setTool(ToolFill)
	step(a)
	px, py = pc(a, 300, 190)
	tclick(a, px, py, 0)
	if a.pic.ColorAt(319, 199) != 7 {
		t.Fatal("hires fill")
	}
	// files
	dir := t.TempDir()
	a.set.WorkDir = dir
	for _, k := range []struct {
		kind int
		ext  string
		size int
	}{{exportART, "art", 9009}, {exportDD, "dd", 9218}, {exportPRG, "prg", 2 + 0x2000 - 0x0801 + 9000}} {
		p := filepath.Join(dir, "h."+k.ext)
		if !a.writeExport(p, k.kind) {
			t.Fatal("export", k.ext)
		}
		if b, _ := os.ReadFile(p); len(b) != k.size {
			t.Fatalf("%s: %d bytes", k.ext, len(b))
		}
	}
	a.openPath(filepath.Join(dir, "h.art"))
	if a.pic.Mode != c64.ModeHires || a.pic.ColorAt(120, 50) != 2 {
		t.Fatal("open art")
	}
	h.save = filepath.Join(dir, "h.c64dp")
	if !a.saveTo(h.save) {
		t.Fatal("save project")
	}
	a.newPicture(c64.ModeMulti)
	if picW != 160 {
		t.Fatal("back to multicolor")
	}
	a.openPath(h.save)
	step(a)
	if a.pic.Mode != c64.ModeHires || a.pic.ColorAt(120, 50) != 2 || picW != 320 {
		t.Fatal("hires project")
	}
	// a hires picture cannot be saved as Koala
	if a.saveTo(filepath.Join(dir, "x.kla")) {
		t.Fatal("hires as Koala")
	}
}

func TestHiresFreeMode(t *testing.T) {
	a, _ := setup(t)
	a.newPicture(c64.ModeFree)
	step(a)
	a.setTool(ToolDraw)
	for i, c := range []uint8{1, 2, 3, 4} {
		a.fg = c
		step(a)
		px, py := pc(a, i, 0)
		tclick(a, px, py, 0)
	}
	for i, c := range []uint8{1, 2, 3, 4} {
		if a.pic.ColorAt(i, 0) != c {
			t.Fatal("free mode allows all colors")
		}
	}
	if a.pic.HiresClashes() != 1 {
		t.Fatal("clash")
	}
	// import converts to the mode of the picture
	img := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			c := a.pal.Colors[(x/7+y/9)%16]
			img.Set(x, y, color.RGBA{uint8(c >> 16), uint8(c >> 8), uint8(c), 255})
		}
	}
	d := newImportDialog(a, img, "test")
	d.opts.Dither = c64.DitherNone
	d.update(a)
	if d.preview.Mode != c64.ModeFree || d.preview.ColorAt(0, 0) != 0 || d.preview.ColorAt(7, 0) != 1 {
		t.Fatalf("free import: %d %d", d.preview.ColorAt(0, 0), d.preview.ColorAt(7, 0))
	}
	a.newPicture(c64.ModeHires)
	d = newImportDialog(a, img, "test")
	d.update(a)
	if d.preview.Mode != c64.ModeHires {
		t.Fatal("hires import")
	}
	d.preview.ToLayers()
	if d.preview.HiresClashes() != 0 {
		t.Fatal("hires import must follow the rules")
	}
}
