package c64

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// bandPicture paints hard color bands (like a sky painted without dithering)
// and a filled circle with a black outline.
func bandPicture() *Picture {
	p := NewPicture(0, 0)
	bands := []uint8{6, 4, 14, 3, 13, 1, 7, 10, 8, 2, 9}
	for y := 0; y < H; y++ {
		c := bands[y*len(bands)/H]
		for x := 0; x < W; x++ {
			if x >= 100 {
				// grey ramp from left to right in the right part
				g := []uint8{11, 12, 15, 1}[(x-100)*4/60]
				p.Plot(x, y, g, ConflictReplace, nil)
				continue
			}
			p.Plot(x, y, c, ConflictReplace, nil)
		}
	}
	for y := 60; y < 140; y++ {
		for x := 20; x < 80; x++ {
			dx, dy := float64(x-50)*2, float64(y-100)
			r := dx*dx + dy*dy
			if r < 38*38 {
				p.Plot(x, y, 5, ConflictReplace, nil)
			} else if r < 40*40 {
				p.Plot(x, y, 0, ConflictReplace, nil)
			}
		}
	}
	return p
}

func checkValid(t *testing.T, p *Picture, name string) {
	t.Helper()
	for cell := 0; cell < Cells; cell++ {
		cols := map[uint8]bool{}
		cx, cy := (cell%Cols)*4, (cell/Cols)*8
		for y := cy; y < cy+8; y++ {
			for x := cx; x < cx+4; x++ {
				cols[p.ColorAt(x, y)] = true
			}
		}
		cols[p.BG] = true
		if len(cols) > 4 {
			t.Fatalf("%s: cell %d has %d colors", name, cell, len(cols))
		}
	}
}

func TestAutoDitherStyles(t *testing.T) {
	pal := &Palettes[FindPalette("PEPTOette_a")]
	src := bandPicture()
	out := os.Getenv("DITHER_OUT")
	for s := DitherStyle(0); s < StyleCount; s++ {
		for _, cm := range []CellColorMode{CellAdd, CellKeep, CellRechoose} {
			o := DefaultAutoDither()
			o.Style, o.Colors = s, cm
			res := AutoDither(src, pal, o, nil)
			checkValid(t, res, StyleNames[s])
			if res.BG != src.BG {
				t.Fatal("background changed")
			}
			// interior of a flat band stays unchanged
			if res.ColorAt(10, 2) != src.ColorAt(10, 2) || res.ColorAt(90, 9) != src.ColorAt(90, 9) {
				t.Fatalf("%s: flat area changed", StyleNames[s])
			}
			// the band border must now be mixed
			changed := 0
			for y := 0; y < H; y++ {
				for x := 0; x < 100; x++ {
					if res.ColorAt(x, y) != src.ColorAt(x, y) {
						changed++
					}
				}
			}
			if changed < 0 {
				t.Fatalf("%s: only %d pixels dithered", StyleNames[s], changed)
			}
			if cm != CellRechoose {
				// Keep: no new colors at all. Add: new colors only in free slots,
				// so a cell that already had three colors gets nothing new.
				for cell := 0; cell < Cells; cell++ {
					a, ua := src.CellColors(cell)
					b, ub := res.CellColors(cell)
					had := 0
					for s1 := 1; s1 <= 3; s1++ {
						if ua[s1] && a[s1] != src.BG {
							had++
						}
					}
					for s2 := 1; s2 <= 3; s2++ {
						if !ub[s2] || b[s2] == res.BG {
							continue
						}
						found := false
						for s1 := 1; s1 <= 3; s1++ {
							if ua[s1] && a[s1] == b[s2] {
								found = true
							}
						}
						if !found && (cm == CellKeep || had == 3) {
							t.Fatalf("%s/%s: new color in cell %d", StyleNames[s], CellModeNames[cm], cell)
						}
					}
				}
			}
			if out != "" && cm == CellAdd {
				writePNG(t, res, pal, filepath.Join(out, "style"+string(rune('0'+s))+".png"))
			}
		}
	}
	if out != "" {
		writePNG(t, src, pal, filepath.Join(out, "source.png"))
	}
}

func TestAutoDitherMask(t *testing.T) {
	pal := &Palettes[FindPalette("PEPTOette_a")]
	src := bandPicture()
	mask := make([]bool, W*H)
	for y := 0; y < 100; y++ {
		for x := 0; x < 50; x++ {
			mask[y*W+x] = true
		}
	}
	res := AutoDither(src, pal, DefaultAutoDither(), mask)
	checkValid(t, res, "mask")
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			if !mask[y*W+x] && res.ColorAt(x, y) != src.ColorAt(x, y) {
				t.Fatalf("pixel %d,%d outside the mask changed", x, y)
			}
		}
	}
}

func writePNG(t *testing.T, p *Picture, pal *Palette, name string) {
	img := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for y := 0; y < H; y++ {
		for x := 0; x < 320; x++ {
			c := pal.Colors[p.ColorAt(x/2, y)]
			img.Set(x, y, color.RGBA{uint8(c >> 16), uint8(c >> 8), uint8(c), 255})
		}
	}
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, img)
}

// twoAreas: upper half color a, lower half color b, border between rows 99/100.
func twoAreas(a, b uint8) *Picture {
	p := NewPicture(0, 0)
	for y := 0; y < H; y++ {
		c := a
		if y >= 100 {
			c = b
		}
		for x := 0; x < W; x++ {
			p.Plot(x, y, c, ConflictReplace, nil)
		}
	}
	return p
}

func share(p *Picture, y int, c uint8) float64 {
	n := 0
	for x := 0; x < W; x++ {
		if p.ColorAt(x, y) == c {
			n++
		}
	}
	return float64(n) / W
}

func TestAutoDitherStages(t *testing.T) {
	pal := &Palettes[FindPalette("PEPTOette_a")]
	src := twoAreas(6, 4) // blue / purple: a pair artists mix
	o := DefaultAutoDither()
	o.Style, o.Width, o.Steps = StyleChecker, 8, 2
	res := AutoDither(src, pal, o, nil)
	// two clean stages per side: 50 % next to the border, then 25 %, then solid
	for _, c := range []struct {
		y    int
		want float64
	}{{96, 0.5}, {99, 0.5}, {94, 0.25}, {92, 0.25}, {90, 0}, {100, 0.5}, {103, 0.5}, {105, 0.25}, {109, 0}} {
		other := uint8(4)
		if c.y >= 100 {
			other = 6
		}
		// share of the other color in two neighboring rows (patterns span two rows)
		s := (share(res, c.y, other) + share(res, c.y^1, other)) / 2
		if c.y == 99 || c.y == 100 {
			s = share(res, c.y, other)
		}
		if math.Abs(s-c.want) > 0.13 {
			t.Errorf("row %d: share of the other color %.2f, want %.2f", c.y, s, c.want)
		}
	}
	// the pattern runs on across the border: checkerboard rows 99/100 differ everywhere
	for x := 0; x < W; x++ {
		if res.ColorAt(x, 99) == res.ColorAt(x, 100) {
			t.Fatalf("checkerboard broken at the border (x=%d)", x)
		}
	}
}

func TestAutoDitherKeepsOutlinesAndHardEdges(t *testing.T) {
	pal := &Palettes[FindPalette("PEPTOette_a")]
	src := twoAreas(6, 4)
	for x := 0; x < W; x++ { // a black outline between the areas
		src.Plot(x, 99, 0, ConflictReplace, nil)
		src.Plot(x, 100, 0, ConflictReplace, nil)
	}
	res := AutoDither(src, pal, DefaultAutoDither(), nil)
	for x := 0; x < W; x++ {
		for y := 94; y <= 105; y++ {
			if res.ColorAt(x, y) != src.ColorAt(x, y) {
				t.Fatalf("outline area changed at %d,%d", x, y)
			}
		}
	}
	// black next to grey: too far apart, stays a hard edge ...
	bw := twoAreas(0, 12)
	res = AutoDither(bw, pal, DefaultAutoDither(), nil)
	for y := 0; y < H; y++ {
		if res.ColorAt(10, y) != bw.ColorAt(10, y) {
			t.Fatalf("black/grey edge was dithered at row %d", y)
		}
	}
	// ... unless in-between colors are allowed (dark grey)
	o := DefaultAutoDither()
	o.Between = true
	res = AutoDither(bw, pal, o, nil)
	used := map[uint8]bool{}
	for y := 80; y < 120; y++ {
		used[res.ColorAt(10, y)] = true
	}
	if len(used) < 3 {
		t.Fatalf("in-between color expected, colors used: %v", used)
	}
}
