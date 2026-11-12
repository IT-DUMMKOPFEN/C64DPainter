package app

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/gfx"
)

func TestLayersUIAndFreeExport(t *testing.T) {
	a, h := setup(t)
	a.addLayer()
	a.addLayer()
	if a.pic.LayerCount() != 5 || !a.extraLayers {
		t.Fatalf("layers: %d", a.pic.LayerCount())
	}
	a.toggleFreeLayer()
	if !a.pic.IsFree(a.pic.Active) {
		t.Fatal("free")
	}
	// 4 colors in one cell: 3 on layer 2, the 4th on the free layer
	act := a.pic.Active
	a.pic.Active = 0
	for i, c := range []uint8{2, 5, 7} {
		a.plot(a.pic, i, 0, c)
	}
	a.pic.Active = act
	a.plot(a.pic, 3, 0, 1)
	if a.pic.ColorAt(3, 0) != 1 {
		t.Fatal("free pixel not shown")
	}
	// project keeps 5 layers, the free flag and names
	a.renameLayer()
	d := a.modal.(*promptDialog)
	d.tf.Set("Sky")
	a.closeModal(false)
	d.ok(d.tf.String())
	h.save = filepath.Join(h.dir, "l.c64dp")
	if !a.saveTo(h.save) {
		t.Fatal("save")
	}
	b, _ := setup(t)
	b.openPath(h.save)
	if b.pic.LayerCount() != 5 || !b.pic.IsFree(act) || b.layerName(act) != "Sky" || b.pic.ColorAt(3, 0) != 1 {
		t.Fatalf("project: %d layers", b.pic.LayerCount())
	}
	// KLA export: the conversion dialog appears and writes a valid picture
	h.save = filepath.Join(h.dir, "x.kla")
	a.cmdExport(exportKLA)
	fc, ok := a.modal.(*freeConvertDialog)
	if !ok {
		t.Fatal("no conversion dialog")
	}
	if !a.writeExportOf(fc.flat, h.save, exportKLA) {
		t.Fatal("write")
	}
	data, _ := os.ReadFile(h.save)
	q, err := c64.LoadKLA(data)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[uint8]bool{}
	for x := 0; x < 4; x++ {
		if c := q.ColorAt(x, 0); c != q.BG {
			seen[c] = true
		}
	}
	if len(seen) != 3 {
		t.Fatalf("converted cell has %d colors", len(seen))
	}
	// undo of a layer operation restores the list
	n := a.pic.LayerCount()
	a.closeModal(true)
	a.deleteLayer()
	a.undo()
	if a.pic.LayerCount() != n {
		t.Fatal("undo of delete")
	}
}

func TestBend(t *testing.T) {
	src := &CustomBrush{W: 21, H: 4, Pix: make([]int8, 84)}
	for i := range src.Pix {
		src.Pix[i] = 1
	}
	b := bend{amount: -6, apex: 0.5}
	out, dx, dy := warpBend(src, &b)
	if out.W != 21 || out.H != 10 || dx != 0 || dy != -6 {
		t.Fatalf("size %dx%d move %d,%d", out.W, out.H, dx, dy)
	}
	// edges stay, the middle column moves up by 6
	if out.Pix[6*21+0] != 1 || out.Pix[0*21+10] != 1 || out.Pix[0*21+0] != -1 {
		t.Fatal("arch shape")
	}
	// apex to the right: the highest column moves right
	b.apex = 0.8
	out, _, _ = warpBend(src, &b)
	if out.Pix[0*21+16] != 1 {
		t.Fatal("apex")
	}
	// vertical
	b = bend{vert: true, amount: 3, apex: 0.5}
	src2 := &CustomBrush{W: 2, H: 11, Pix: make([]int8, 22)}
	for i := range src2.Pix {
		src2.Pix[i] = 1
	}
	out, dx, _ = warpBend(src2, &b)
	if out.W != 5 || dx != 0 || out.Pix[5*5+3] != 1 || out.Pix[0*5+0] != 1 {
		t.Fatal("vertical bend")
	}
	// in the app: selection with bend handles, dragging a handle bends it
	a, _ := setup(t)
	a.pic.Plot(10, 10, 2, 0, a.pal)
	a.sel.x0, a.sel.y0, a.sel.x1, a.sel.y1 = 0, 0, 40, 30
	a.sel.state = selSelected
	a.selLift(false)
	a.sel.bend.on = true
	a.sel.bend.reset()
	a.bendDrag(a.sel.bend, 0, -8)
	if !a.sel.bend.active() || a.sel.out.H != 31+8 {
		t.Fatalf("bend in the app: %d", a.sel.out.H)
	}
}

func TestPatterns11(t *testing.T) {
	a, h := setup(t)
	if patternCount() < len(patterns)+40 {
		t.Fatalf("only %d patterns", patternCount())
	}
	for i := len(patterns); i < patternCount(); i++ {
		b := bigPatternAt(i)
		if b == nil || (b.S != 16 && b.S != 32 && b.S != 64) || len(b.Bits) != b.S*b.S {
			t.Fatalf("pattern %d", i)
		}
	}
	// square on screen: a multicolor pixel covers two pattern pixels
	i := len(patterns) + 12 // checker 8
	if patBit(i, 3, 0) != bigPatternAt(i).at(6, 0) {
		t.Fatal("multicolor sampling")
	}
	// editor: save asks for the folder once and remembers it
	a.set.PatternDir = filepath.Join(h.dir, "pats")
	os.MkdirAll(a.set.PatternDir, 0o755)
	a.openPatternEditor(nil)
	e := a.modal.(*patternEditor)
	e.p.Bits[0], e.p.Bits[17] = true, true
	e.name.Set("Mine")
	e.save(a, false)
	if len(userPatterns) != 1 || !userPatterns[0].Bits[17] || a.pattern != patternCount()-1 {
		t.Fatal("own pattern not saved/loaded")
	}
	// pattern from a selection
	a.pic.Plot(0, 0, 2, 0, a.pal)
	a.sel.x0, a.sel.y0, a.sel.x1, a.sel.y1 = 0, 0, 7, 15
	a.sel.state = selSelected
	a.selectionAsPattern()
	pe, ok := a.modal.(*patternEditor)
	if !ok || pe.p.S != 16 || !pe.p.Bits[0] || !pe.p.Bits[1] || pe.p.Bits[2] {
		t.Fatal("selection as pattern")
	}
	step(a)
	userPatterns = nil
}

func TestGradientDither(t *testing.T) {
	cols := []uint8{1, 2}
	w := 64
	var pts []int
	var ts []float64
	for y := 0; y < 8; y++ {
		for x := 0; x < w; x++ {
			pts = append(pts, y*w+x)
			ts = append(ts, float64(x)/float64(w-1))
		}
	}
	count := func(out []uint8, x0, x1 int) (n1, n2 int) {
		for k, c := range out {
			if x := pts[k] % w; x >= x0 && x < x1 {
				if c == 1 {
					n1++
				} else {
					n2++
				}
			}
		}
		return
	}
	for m := range gradMethods {
		// width 0: hard edge in the middle
		out := gradientRender(cols, m, 0, pts, ts, w)
		if a, b := count(out, 0, 30); b != 0 || a == 0 {
			t.Fatalf("%s: width 0 must be a hard edge", gradMethods[m].name)
		}
		// width 100: both colors mixed around the middle, pure at the ends
		out = gradientRender(cols, m, 100, pts, ts, w)
		a1, b1 := count(out, 26, 38)
		if m > 0 && (a1 == 0 || b1 == 0) {
			t.Fatalf("%s: no mix in the middle", gradMethods[m].name)
		}
		if a, b := count(out, 0, 2); b > a {
			t.Fatalf("%s: left end", gradMethods[m].name)
		}
	}
	// settings from 1.0: the dither switch becomes the classic method
	g := GradientSettings{Dither: true}
	g.clamp()
	if g.Method != 1 || g.Width != 50 {
		t.Fatal("migration")
	}
}

func TestCopyIntoOtherPageAndSnap(t *testing.T) {
	a, _ := setup(t)
	// page 2: a full pattern
	a.swapPage()
	for y := 0; y < 16; y++ {
		for x := 0; x < 8; x++ {
			a.pic.Plot(x, y, 5, 0, a.pal)
		}
	}
	a.swapPage()
	// page 1: a "logo" with a hole (background in the middle)
	for y := 0; y < 16; y++ {
		for x := 0; x < 8; x++ {
			if x == 0 || x == 7 || y == 0 || y == 15 {
				a.pic.Plot(x, y, 2, 0, a.pal)
			}
		}
	}
	a.sel.x0, a.sel.y0, a.sel.x1, a.sel.y1 = 0, 0, 7, 15
	a.sel.state = selSelected
	a.copyIntoOtherPage()
	o := a.pages[1]
	if o.ColorAt(0, 0) != 2 || o.ColorAt(3, 5) != 5 {
		t.Fatalf("copy: frame %d, hole %d", o.ColorAt(0, 0), o.ColorAt(3, 5))
	}
	if len(a.undos[1]) == 0 {
		t.Fatal("undo on the other page")
	}
	// snap: a drag from (5,5) to (9,10) covers the 4×8 cells
	a.set.SelSnap = true
	a.set.GridW, a.set.GridH = 8, 8
	if x, y := a.snapSel(5, 5, true); x != 4 || y != 0 {
		t.Fatalf("snap start %d,%d", x, y)
	}
	if x, y := a.snapSel(9, 10, false); x != 11 || y != 15 {
		t.Fatalf("snap end %d,%d", x, y)
	}
}

func TestFontPictureImport(t *testing.T) {
	a, h := setup(t)
	for y := 0; y < 200; y++ {
		for x := 0; x < 160; x++ {
			if (x/4+y/8)%3 == 0 {
				a.plot(a.pic, x, y, 6)
			}
		}
	}
	a.openFontEditor()
	a.fntOpenPicImport()
	d, ok := a.modal.(*fntPicDialog)
	if !ok {
		t.Fatal("dialog")
	}
	step(a)
	if d.cm == nil || d.cm.Used < 2 || d.cm.Used > 256 {
		t.Fatalf("charmap: %+v", d.cm)
	}
	d.o.Limit = 16
	step(a)
	if d.cm.Used > 16 {
		t.Fatalf("limit: %d", d.cm.Used)
	}
	h.save = filepath.Join(h.dir, "logo.asm")
	a.fntExportCharMap(d.cm, d.name)
	for _, f := range []string{"logo.asm", "logo_data.asm", "logo.prg"} {
		if st, err := os.Stat(filepath.Join(h.dir, f)); err != nil || st.Size() == 0 {
			t.Fatalf("%s missing", f)
		}
	}
	cm := d.cm
	a.closeModal(false)
	a.fnt.cmap = cm
	if a.fnt.f.Multi != cm.Font.Multi {
		t.Log("mode differs (ok)")
	}
}

func TestMoreFormatsApp(t *testing.T) {
	a, h := setup(t)
	for y := 0; y < 200; y++ {
		for x := 0; x < 160; x++ {
			if (x/4+y/8)%3 == 0 {
				a.plot(a.pic, x, y, uint8(2+x%5))
			}
		}
	}
	want := a.pic.KLABytes()
	for _, ext := range []string{".drz", ".gg", ".ocp", ".ami", ".ism", ".vid", ".pi"} {
		p := filepath.Join(h.dir, "m"+ext)
		if !a.saveTo(p) {
			t.Fatalf("save %s", ext)
		}
		a.pic = c64.NewPicture(0, 0)
		a.openPath(p)
		if !bytes.Equal(a.pic.KLABytes(), want) {
			t.Fatalf("%s: picture differs after loading", ext)
		}
		if a.filePath[a.page] != p {
			t.Fatalf("%s: file path", ext)
		}
	}
	// hires format refused for a multicolor picture
	if a.saveTo(filepath.Join(h.dir, "x.hed")) {
		t.Fatal("hed saved from multicolor")
	}
	// raw data
	h.save = filepath.Join(h.dir, "my pic.asm")
	a.cmdExport(exportRaw)
	for _, f := range []string{"my_pic.asm", "my_pic_bitmap.bin", "my_pic_screen.bin", "my_pic_color.bin"} {
		if st, err := os.Stat(filepath.Join(h.dir, f)); err != nil || st.Size() == 0 {
			t.Fatalf("%s missing", f)
		}
	}
	if st, _ := os.Stat(filepath.Join(h.dir, "my_pic_bitmap.bin")); st.Size() != 8000 {
		t.Fatal("bitmap size")
	}
	// the formats dialog draws
	a.openFormatsDialog()
	step(a)
	a.closeModal(false)
}

func TestImportIntoFormat(t *testing.T) {
	a, h := setup(t)
	img := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 3 / 4), uint8(y), uint8((x + y) / 3), 255})
		}
	}
	tg := importTargets()
	pick := func(name string) int {
		for i, t := range tg {
			if strings.HasPrefix(t.label, name) {
				return i
			}
		}
		t.Fatalf("no target %s", name)
		return 0
	}
	for _, c := range []struct {
		target, file string
		hires        bool
	}{{"Drazpaint", "a.drz", false}, {"Hi-Eddi", "b.hed", true}, {"Koala", "c.kla", false}} {
		a.modal = newImportDialog(a, img, "photo")
		d := a.modal.(*importDialog)
		step(a)
		// choose the target with the arrow button until it is selected
		want := pick(c.target)
		d.saveAs = want
		if t2 := tg[d.saveAs]; !t2.fits(d.mode) {
			if t2.mode == 1 {
				d.mode = c64.ModeHires
			} else {
				d.mode = c64.ModeMulti
			}
		}
		step(a)
		h.save = filepath.Join(h.dir, c.file)
		w, hh := d.Size(a)
		r := a.modalRect(d, min(w, a.W-16), min(hh, a.H-16))
		content := gfx.Rect{X: r.X + 16, Y: r.Y + 31 + 12, W: r.W - 32, H: r.H - 31 - 24}
		tclick(a, content.Right()-60, content.Bottom()-14, 0)
		step(a)
		if a.modal != nil {
			t.Fatalf("%s: dialog still open", c.target)
		}
		if a.hires() != c.hires {
			t.Fatalf("%s: mode", c.target)
		}
		if a.filePath[a.page] != h.save {
			t.Fatalf("%s: file not adopted: %q", c.target, a.filePath[a.page])
		}
		if st, err := os.Stat(h.save); err != nil || st.Size() < 1000 {
			t.Fatalf("%s: file missing", c.target)
		}
	}
	// ACME program + data
	h.save = filepath.Join(h.dir, "show.asm")
	a.cmdExport(exportACME)
	for _, f := range []string{"show.asm", "show_data.asm", "show.prg"} {
		if _, err := os.Stat(filepath.Join(h.dir, f)); err != nil {
			t.Fatalf("%s missing", f)
		}
	}
}
