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

func TestColorMatchAndCellMethods(t *testing.T) {
	pal := &Palettes[FindPalette(DefaultPalette)]
	for m := ColorMatch(0); m < matchCount; m++ {
		for c := CellMethod(0); c < cellMethodCount; c++ {
			p := Convert(gradient(), pal, ConvertOptions{Fit: FitStretch, Dither: DitherBayer4, Strength: 50, BG: -1, Match: m, Cells: c})
			cellColorsCheck(t, p)
			if c == CellsFixed {
				set := map[uint8]bool{}
				for cell := 0; cell < Cells; cell++ {
					u := p.Usage(cell, -1, -1)
					for s := 1; s <= 3; s++ {
						if u[s] > 0 {
							set[p.SlotColor(cell, s)] = true
						}
					}
				}
				if len(set) > 3 {
					t.Fatalf("%s: fixed colors use %d colors", MatchNames[m], len(set))
				}
			}
		}
	}
	for m := ColorMatch(0); m < matchCount; m++ {
		d := colorMetric(m)
		if d(10, 20, 30, 10, 20, 30) != 0 || d(0, 0, 0, 255, 255, 255) <= 0 {
			t.Fatalf("%s metric", MatchNames[m])
		}
	}
}

func cellColorsCheck(t *testing.T, p *Picture) {
	for cell := 0; cell < Cells; cell++ {
		seen := map[uint8]bool{}
		cx, cy := (cell%Cols)*4, (cell/Cols)*8
		for y := cy; y < cy+8; y++ {
			for x := cx; x < cx+4; x++ {
				seen[p.ColorAt(x, y)] = true
			}
		}
		if len(seen) > 4 {
			t.Fatalf("cell with %d colors", len(seen))
		}
	}
}

func TestNewDithersAndAutoLevels(t *testing.T) {
	for name, m := range map[string][]int{"blue": blueNoise(), "cell": cellPattern} {
		seen := make([]bool, len(m))
		for _, v := range m {
			if v < 0 || v >= len(m) || seen[v] {
				t.Fatalf("%s: not a permutation", name)
			}
			seen[v] = true
		}
	}
	// auto levels: a dull picture is stretched; each channel and brightness
	px := make([][3]int, 1000)
	for i := range px {
		v := 60 + i%100
		px[i] = [3]int{v, v / 2, v}
	}
	ch := autoLevels(px, 1)
	lo, hi := 255, 0
	for _, c := range ch {
		lo, hi = min(lo, c[0]), max(hi, c[0])
	}
	if lo != 0 || hi != 255 {
		t.Fatalf("each channel: %d..%d", lo, hi)
	}
	br := autoLevels(px, 2)
	if br[0][1] >= br[0][0] && br[99][1] >= br[99][0] {
		t.Fatal("brightness mode must keep the hue (green stays lower)")
	}
	// every new method still follows the rules
	pal := &Palettes[0]
	img := make([][3]int, W*H)
	for i := range img {
		img[i] = [3]int{(i * 7) % 256, (i / W) % 256, (i * 3) % 256}
	}
	for _, d := range []DitherMode{DitherBayer4, DitherBlueNoise, DitherYliluoma, DitherCellPattern, DitherFloyd} {
		p := ConvertRGB(img, pal, ConvertOptions{BG: -1, Dither: d, Strength: 50, Match: MatchYUV})
		for cell := 0; cell < Cells; cell++ {
			u := p.Usage(cell, -1, -1)
			_ = u
		}
		if p == nil {
			t.Fatal("no picture")
		}
		h := ConvertHiresRGB(make([][3]int, HW*H), pal, ConvertOptions{BG: -1, Dither: d, Strength: 50}, ModeHires)
		h.ToLayers()
		if h.HiresClashes() != 0 {
			t.Fatal("hires rules")
		}
	}
}
