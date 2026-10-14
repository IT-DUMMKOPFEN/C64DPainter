package app

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	"image/png"

	"c64dpainter/internal/gfx"
)

//go:embed logo16.png
var logo16PNG []byte

//go:embed logo64.png
var logo64PNG []byte

var logo16, logo64 *image.NRGBA

func decodeLogo(b []byte) *image.NRGBA {
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	n := image.NewNRGBA(img.Bounds())
	draw.Draw(n, n.Bounds(), img, img.Bounds().Min, draw.Src)
	return n
}

// blitLogo draws a logo with alpha at (x, y), each pixel scale× scale large.
func blitLogo(m *gfx.Image, l *image.NRGBA, x, y, scale int) {
	b := l.Bounds()
	for yy := 0; yy < b.Dy(); yy++ {
		for xx := 0; xx < b.Dx(); xx++ {
			o := yy*l.Stride + xx*4
			al := int(l.Pix[o+3])
			if al == 0 {
				continue
			}
			c := gfx.RGB(l.Pix[o], l.Pix[o+1], l.Pix[o+2])
			for sy := 0; sy < scale; sy++ {
				for sx := 0; sx < scale; sx++ {
					if al == 255 {
						m.Set(x+xx*scale+sx, y+yy*scale+sy, c)
					} else {
						m.BlendPix(x+xx*scale+sx, y+yy*scale+sy, c, al+1)
					}
				}
			}
		}
	}
}

// drawLogo draws the small program icon (menu bar).
func drawLogo(m *gfx.Image, r gfx.Rect) {
	if logo16 == nil {
		logo16 = decodeLogo(logo16PNG)
	}
	blitLogo(m, logo16, r.X+(r.W-16)/2, r.Y+(r.H-16)/2, 1)
}

// drawBigLogo draws the program icon in the About window.
func drawBigLogo(m *gfx.Image, r gfx.Rect) {
	if logo64 == nil {
		logo64 = decodeLogo(logo64PNG)
	}
	blitLogo(m, logo64, r.X+(r.W-64)/2, r.Y+(r.H-64)/2, 1)
}
