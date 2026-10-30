package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"c64dpainter/internal/c64"
)

type askHost struct {
	recHost
	answer int
	asked  int
}

func (h *askHost) Ask(string, string) int { h.asked++; return h.answer }

func TestCrashRecovery(t *testing.T) {
	a, _ := setup(t)
	a.pic.L[0][5] = 3
	a.pic.Recomposite()
	a.dirty[0] = true
	a.filePath[0] = "/somewhere/house.c64dp"
	a.now = recoveryInterval + 10
	a.checkRecovery()
	if !hasRecovery() {
		t.Fatal("no recovery file written")
	}
	// saved: the file goes away again
	a.dirty[0] = false
	a.now += recoveryInterval + 10
	a.checkRecovery()
	if hasRecovery() {
		t.Fatal("recovery file should be removed after saving")
	}
	a.dirty[0] = true
	a.now += recoveryInterval + 10
	a.checkRecovery()

	// "crash": a new app with the same settings folder offers the picture
	h := &askHost{answer: 0}
	b := New(h)
	b.Resize(1360, 860)
	step(b)
	if h.asked != 1 {
		t.Fatalf("recovery question asked %d times", h.asked)
	}
	if b.pic.L[0][5] != 3 || !b.dirty[0] || b.filePath[0] != "/somewhere/house.c64dp" {
		t.Fatal("picture not restored")
	}
	if _, ok := b.modal.(*modeDialog); ok {
		t.Fatal("the mode question is not needed after restoring")
	}
	if hasRecovery() {
		t.Fatal("restored files should be removed")
	}
	// a normal quit leaves nothing behind
	b.now += recoveryInterval + 10
	b.checkRecovery()
	b.dirty = [2]bool{}
	if !b.CanClose() || hasRecovery() {
		t.Fatal("normal quit must remove the recovery files")
	}
	// declining discards
	b.dirty[0] = true
	b.writeRecovery()
	h2 := &askHost{answer: 1}
	c := New(h2)
	c.Resize(1360, 860)
	step(c)
	if h2.asked != 1 || hasRecovery() || c.pic.L[0][5] == 3 {
		t.Fatal("declined recovery must discard the files")
	}
	if _, ok := c.modal.(*modeDialog); !ok {
		t.Fatal("after declining the mode question follows")
	}
}

func TestScaledPNGExport(t *testing.T) {
	a, h := setup(t)
	a.pic.BG = 6
	a.pic.Recomposite()
	h.save = filepath.Join(h.dir, "big.png")
	a.cmdExport(exportPNG4x)
	data, err := os.ReadFile(h.save)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 1280 || b.Dy() != 800 {
		t.Fatalf("size %v", b)
	}
	r, g, bl, _ := img.At(1279, 799).RGBA()
	c := a.pal.Colors[6]
	if uint32(r>>8) != c>>16&255 || uint32(g>>8) != c>>8&255 || uint32(bl>>8) != c&255 {
		t.Fatal("wrong color")
	}
}

func TestImportPresetPreviewsAndMemory(t *testing.T) {
	a, _ := setup(t)
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{uint8(x / 2), uint8(y), 120, 255})
		}
	}
	a.modal = newImportDialog(a, img, "photo")
	d := a.modal.(*importDialog)
	for i := 0; i < 8; i++ {
		step(a)
	}
	for i := range importPresets {
		if d.thumbs[i] == nil || len(d.thumbs[i]) != d.thumbW*d.thumbH {
			t.Fatalf("preview %d missing", i)
		}
	}
	// choose "Smooth" and import: the settings are kept for the next start
	d.opts = importPresets[2].opts(d.opts)
	d.opts.Adjust.Brightness = 40
	a.lastImport = d.opts
	a.rememberImport()
	b := New(&recHost{})
	if b.lastImport.Dither != c64.DitherFloyd || b.lastImport.Adjust.Brightness != 0 || b.lastImport.Adjust.AutoLevels != 1 {
		t.Fatalf("import settings not kept: %+v", b.lastImport)
	}
	// a C64-sized image switches automatic contrast off only for itself
	c64img := image.NewRGBA(image.Rect(0, 0, 320, 200))
	d2 := newImportDialog(a, c64img, "c64")
	if d2.opts.Adjust.AutoLevels != 0 || d2.autoWas != 1 {
		t.Fatal("exact size should switch automatic contrast off")
	}
}

func TestWindowMemory(t *testing.T) {
	a, _ := setup(t)
	a.RememberWindow(WindowState{X: 10, Y: 20, W: 900, H: 700})
	b := New(&recHost{})
	b.InitScale(1)
	if st, ok := b.SavedWindow(); !ok || st.W != 900 || st.Y != 20 {
		t.Fatalf("window not kept: %+v %v", st, ok)
	}
	b.set.UIScale = 2
	if _, ok := b.SavedWindow(); ok {
		t.Fatal("another interface size must not use the old window")
	}
}

func TestSideBorderEditor(t *testing.T) {
	a, h := setup(t)
	a.toggleSideBorder()
	if !a.sideOn() {
		t.Fatal("side border should be open")
	}
	// paint with the freehand tool into the right strip
	a.tool = ToolDraw
	a.fg = 2
	x0, y0 := pc(a, c64.W+2, 30)
	x1, y1 := pc(a, c64.W+8, 30)
	tdrag(a, x0, y0, x1, y1, 0)
	if a.pic.SideColorAt(1, 2, 30) != 2 || a.pic.SideColorAt(1, 8, 30) != 2 {
		t.Fatalf("strip not painted: %d", a.pic.SideColorAt(1, 2, 30))
	}
	// fill in the left strip
	a.tool = ToolFill
	a.fg = 5
	fx, fy := pc(a, -5, 100)
	tclick(a, fx, fy, 0)
	if a.pic.SideColorAt(0, 0, 100) != 5 || a.pic.SideColorAt(0, 11, 199) != 5 || a.pic.SideColorAt(0, 4, 0) == 5 {
		t.Fatal("strip fill")
	}
	// undo restores the strip
	a.undo()
	if a.pic.SideColorAt(0, 0, 100) == 5 {
		t.Fatal("undo")
	}
	// project file keeps everything
	h.save = filepath.Join(h.dir, "side.c64dp")
	if !a.saveTo(h.save) {
		t.Fatal("save")
	}
	b, _ := setup(t)
	b.openPath(h.save)
	if !b.sideOn() || b.pic.SideColorAt(1, 8, 30) != 2 || b.pic.Side.Col[1][1] != 2 {
		t.Fatal("project did not keep the side border")
	}
	// export the program
	h.save = filepath.Join(h.dir, "side.prg")
	a.cmdExportSide(false)
	data, err := os.ReadFile(h.save)
	want, _ := a.pic.SideBorderPRG()
	if err != nil || !bytes.Equal(data, want) {
		t.Fatal("prg export")
	}
	h.save = filepath.Join(h.dir, "side.asm")
	a.cmdExportSide(true)
	if src, err := os.ReadFile(h.save); err != nil || !bytes.Contains(src, []byte("sty $d016")) {
		t.Fatal("asm export")
	}
	// PNG with border shows the strips
	img := a.pic.ToImage(a.pal, c64.PNGBorder)
	if img.ColorIndexAt(32+320+2*8, 35+30) != 2 {
		t.Fatal("png with border")
	}
	// hires: not available
	a.newPicture(c64.ModeHires)
	if a.sideOn() {
		t.Fatal("hires has no side border")
	}
}
