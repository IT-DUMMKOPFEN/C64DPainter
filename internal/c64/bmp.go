package c64

import (
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"math/bits"
)

// DecodeBMP liest Windows-Bitmaps (1/4/8/16/24/32 Bit, RLE4/RLE8, Bitfields).
func DecodeBMP(d []byte) (image.Image, error) {
	if len(d) < 26 || d[0] != 'B' || d[1] != 'M' {
		return nil, errors.New("not a BMP file")
	}
	le := binary.LittleEndian
	pixOff := int(le.Uint32(d[10:]))
	hs := int(le.Uint32(d[14:]))
	var w, h, bpp, comp, nColors int
	var masks [4]uint32
	if hs == 12 { // OS/2 BITMAPCOREHEADER
		w = int(le.Uint16(d[18:]))
		h = int(int16(le.Uint16(d[20:])))
		bpp = int(le.Uint16(d[24:]))
	} else {
		if len(d) < 14+40 {
			return nil, errors.New("BMP header too short")
		}
		w = int(int32(le.Uint32(d[18:])))
		h = int(int32(le.Uint32(d[22:])))
		bpp = int(le.Uint16(d[28:]))
		comp = int(le.Uint32(d[30:]))
		nColors = int(le.Uint32(d[46:]))
		if comp == 3 || comp == 6 {
			if hs >= 52 {
				masks[0], masks[1], masks[2] = le.Uint32(d[54:]), le.Uint32(d[58:]), le.Uint32(d[62:])
				if hs >= 56 {
					masks[3] = le.Uint32(d[66:])
				}
			} else if len(d) >= 14+hs+12 {
				o := 14 + hs
				masks[0], masks[1], masks[2] = le.Uint32(d[o:]), le.Uint32(d[o+4:]), le.Uint32(d[o+8:])
			}
		}
	}
	topDown := h < 0
	if h < 0 {
		h = -h
	}
	if w <= 0 || h <= 0 || w > 16384 || h > 16384 {
		return nil, errors.New("invalid BMP size")
	}
	// Palette
	var pal []color.RGBA
	if bpp <= 8 {
		n := nColors
		if n == 0 {
			n = 1 << bpp
		}
		entry := 4
		if hs == 12 {
			entry = 3
		}
		po := 14 + hs
		if comp == 3 && hs == 40 {
			po += 12
		}
		for i := 0; i < n && po+i*entry+2 < len(d); i++ {
			b := d[po+i*entry:]
			pal = append(pal, color.RGBA{b[2], b[1], b[0], 255})
		}
		for len(pal) < 256 {
			pal = append(pal, color.RGBA{0, 0, 0, 255})
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	set := func(x, y int, c color.RGBA) {
		if x < 0 || x >= w || y < 0 || y >= h {
			return
		}
		if !topDown {
			y = h - 1 - y
		}
		img.SetRGBA(x, y, c)
	}
	if pixOff >= len(d) {
		return nil, errors.New("BMP without image data")
	}
	px := d[pixOff:]
	switch comp {
	case 1, 2: // RLE8 / RLE4
		x, y, i := 0, 0, 0
		for i+1 < len(px) {
			n, v := int(px[i]), px[i+1]
			i += 2
			if n > 0 {
				for k := 0; k < n; k++ {
					idx := v
					if comp == 2 {
						if k&1 == 0 {
							idx = v >> 4
						} else {
							idx = v & 15
						}
					}
					set(x, y, pal[idx])
					x++
				}
				continue
			}
			switch v {
			case 0:
				x, y = 0, y+1
			case 1:
				i = len(px)
			case 2:
				if i+1 < len(px) {
					x += int(px[i])
					y += int(px[i+1])
					i += 2
				}
			default:
				cnt := int(v)
				nb := cnt
				if comp == 2 {
					nb = (cnt + 1) / 2
				}
				for k := 0; k < cnt; k++ {
					var idx byte
					if comp == 1 {
						if i+k >= len(px) {
							break
						}
						idx = px[i+k]
					} else {
						if i+k/2 >= len(px) {
							break
						}
						b := px[i+k/2]
						if k&1 == 0 {
							idx = b >> 4
						} else {
							idx = b & 15
						}
					}
					set(x, y, pal[idx])
					x++
				}
				i += nb
				if nb&1 == 1 {
					i++
				}
			}
		}
		return img, nil
	case 0, 3, 6:
	default:
		return nil, errors.New("BMP compression is not supported")
	}
	stride := ((w*bpp + 31) / 32) * 4
	if comp == 0 && bpp == 16 {
		masks = [4]uint32{0x7C00, 0x03E0, 0x001F, 0}
	}
	if comp == 0 && bpp == 32 {
		masks = [4]uint32{0xFF0000, 0xFF00, 0xFF, 0}
	}
	ext := func(v, m uint32) uint8 {
		if m == 0 {
			return 0
		}
		sh := bits.TrailingZeros32(m)
		n := bits.OnesCount32(m)
		x := (v & m) >> uint(sh)
		max := uint32(1)<<uint(n) - 1
		return uint8(x * 255 / max)
	}
	for y := 0; y < h; y++ {
		ro := y * stride
		if ro+stride > len(px) {
			break
		}
		row := px[ro : ro+stride]
		for x := 0; x < w; x++ {
			var c color.RGBA
			switch bpp {
			case 1:
				c = pal[row[x/8]>>(7-uint(x%8))&1]
			case 4:
				b := row[x/2]
				if x&1 == 0 {
					c = pal[b>>4]
				} else {
					c = pal[b&15]
				}
			case 8:
				c = pal[row[x]]
			case 16:
				v := uint32(le.Uint16(row[x*2:]))
				c = color.RGBA{ext(v, masks[0]), ext(v, masks[1]), ext(v, masks[2]), 255}
			case 24:
				c = color.RGBA{row[x*3+2], row[x*3+1], row[x*3], 255}
			case 32:
				v := le.Uint32(row[x*4:])
				c = color.RGBA{ext(v, masks[0]), ext(v, masks[1]), ext(v, masks[2]), 255}
			default:
				return nil, errors.New("BMP color depth is not supported")
			}
			set(x, y, c)
		}
	}
	return img, nil
}
