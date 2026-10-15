// Writes example pictures when DITHER_OUT is set: DITHER_OUT=dir go test ./internal/c64 -run DitherScene
package c64

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func scene() *Picture {
	p := NewPicture(0, 0)
	sky := []uint8{6, 6, 4, 14, 4, 10, 8, 10, 7, 7}
	for y := 0; y < 120; y++ {
		c := sky[y*len(sky)/120]
		for x := 0; x < W; x++ {
			p.Plot(x, y, c, ConflictReplace, nil)
		}
	}
	// sun
	for y := 50; y < 110; y++ {
		for x := 50; x < 110; x++ {
			dx, dy := float64(x-80)*2, float64(y-85)
			r := math.Hypot(dx, dy)
			if r < 22 {
				p.Plot(x, y, 1, ConflictReplace, nil)
			} else if r < 30 {
				p.Plot(x, y, 7, ConflictReplace, nil)
			}
		}
	}
	// mountains and ground
	for x := 0; x < W; x++ {
		h1 := 100 + int(18*math.Sin(float64(x)/13)+10*math.Sin(float64(x)/5.3))
		h2 := 130 + int(8*math.Sin(float64(x)/9+1))
		for y := h1; y < H; y++ {
			c := uint8(11)
			if y > h1+12 {
				c = 12
			}
			if y >= h2 {
				c = 9
				if y > 160 {
					c = 8
				}
				if y > 182 {
					c = 2
				}
			}
			p.Plot(x, y, c, ConflictReplace, nil)
		}
	}
	return p
}

func TestDitherScenePNG(t *testing.T) {
	out := os.Getenv("DITHER_OUT")
	if out == "" {
		t.Skip()
	}
	pal := &Palettes[FindPalette("PEPTOette_a")]
	src := scene()
	writePNG(t, src, pal, filepath.Join(out, "scene_src.png"))
	for s := DitherStyle(0); s < StyleCount; s++ {
		o := DefaultAutoDither()
		o.Style = s
		t0 := time.Now()
		res := AutoDither(src, pal, o, nil)
		fmt.Printf("%-10s %v\n", StyleNames[s], time.Since(t0))
		writePNG(t, res, pal, filepath.Join(out, fmt.Sprintf("scene_%d.png", s)))
	}
}

func TestDitherSceneKLA(t *testing.T) {
	out := os.Getenv("DITHER_OUT")
	if out == "" {
		t.Skip()
	}
	os.WriteFile(filepath.Join(out, "scene.kla"), scene().SaveKLA(), 0o644)
}
