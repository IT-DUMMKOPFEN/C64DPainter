package c64

import (
	"image"
	"image/color"
	"testing"
	"time"
)

func gradient() image.Image {
	im := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			im.Set(x, y, color.RGBA{uint8(x * 255 / 320), uint8(y * 255 / 200), uint8(255 - x*255/320), 255})
		}
	}
	return im
}

func TestBayer8Permutation(t *testing.T) {
	seen := map[int]bool{}
	for _, v := range bayer8 {
		if v < 0 || v > 63 || seen[v] {
			t.Fatalf("bayer8 is not a permutation: %v", bayer8)
		}
		seen[v] = true
	}
}

func TestAllDitherModes(t *testing.T) {
	pal := &Palettes[FindPalette(DefaultPalette)]
	src := gradient()
	px := ResamplePairs(src, FitStretch, PairAverage)
	for d := DitherNone; d < ditherCount; d++ {
		start := time.Now()
		p := ConvertRGB(ApplyAdjust(px, Adjust{Brightness: 10, Contrast: 20, Saturation: 30, Hue: 15, Gamma: -20}), pal, ConvertOptions{Dither: d, Strength: 80, Serpentine: true, BG: -1})
		el := time.Since(start)
		for cell := 0; cell < Cells; cell++ {
			seen := map[uint8]bool{}
			cx, cy := (cell%Cols)*4, (cell/Cols)*8
			for y := cy; y < cy+8; y++ {
				for x := cx; x < cx+4; x++ {
					seen[p.ColorAt(x, y)] = true
				}
			}
			if len(seen) > 4 {
				t.Fatalf("%s: cell with %d colors", DitherNames[d], len(seen))
			}
		}
		t.Logf("%-20s %v", DitherNames[d], el)
	}
}

func TestPairModes(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x += 2 {
			im.Set(x, y, color.RGBA{255, 0, 0, 255})
			im.Set(x+1, y, color.RGBA{0, 0, 255, 255})
		}
	}
	l := ResamplePairs(im, FitStretch, PairLeft)
	r := ResamplePairs(im, FitStretch, PairRight)
	a := ResamplePairs(im, FitStretch, PairAverage)
	if l[5] != [3]int{255, 0, 0} || r[5] != [3]int{0, 0, 255} || a[5] != [3]int{127, 0, 127} {
		t.Fatalf("pair modes wrong: %v %v %v", l[5], r[5], a[5])
	}
}

func TestAdjustIdentity(t *testing.T) {
	px := [][3]int{{10, 20, 30}, {200, 100, 50}}
	out := ApplyAdjust(px, Adjust{})
	if out[0] != px[0] || out[1] != px[1] {
		t.Fatal("zero adjust must not change pixels")
	}
	out = ApplyAdjust(px, Adjust{Saturation: -100})
	if out[1][0] != out[1][1] || out[1][1] != out[1][2] {
		t.Fatalf("saturation -100 must give grey: %v", out[1])
	}
}
