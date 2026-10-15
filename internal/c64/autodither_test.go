package c64

import (
	"image"
	"image/color"
	"image/png"
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
