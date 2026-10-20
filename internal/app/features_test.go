package app

import (
	"bytes"
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
