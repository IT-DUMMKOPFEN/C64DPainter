package c64

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// --- Character sets (font editor) ---------------------------------------------------
//
// A C64 character set: 256 characters of 8 × 8 hires pixels, 2048 bytes. In
// multicolor character mode two bits make one double wide pixel (4 × 8):
// %00 background ($D021), %01 multicolor 1 ($D022), %10 multicolor 2
// ($D023), %11 the character color of the color RAM (0–7, bit 3 switches the
// character to multicolor).

const CharsetSize = 2048

// Font is one character set with the settings of the editor.
type Font struct {
	Data       [CharsetSize]byte
	FG, BG     uint8 // character color and background
	Multi      bool  // multicolor character mode
	MC1, MC2   uint8 // $D022, $D023 (multicolor)
	Ref        int   // reference ROM for "original": 0 upper case, 1 lower case
	Tile       int   // 0 = 1×1, 1 = 2×1, 2 = 1×2, 3 = 2×2
	TileLayout int   // 0 = neighbors in the grid (+1 / +16), 1 = quarters (+64 / +128 / +192)
}

// TileSizes: width × height of the tile formats.
var TileSizes = [4][2]int{{1, 1}, {2, 1}, {1, 2}, {2, 2}}

// TileNames are the display names of the tile formats.
var TileNames = [4]string{"1×1", "2×1", "1×2", "2×2"}

// NewFont returns a font: rom 0 = C64 upper case, 1 = lower case, -1 = empty.
func NewFont(rom int) *Font {
	f := &Font{FG: 14, BG: 6, MC1: 11, MC2: 12}
	switch rom {
	case 0:
		copy(f.Data[:], CharsetUpper)
	case 1:
		copy(f.Data[:], CharsetLower)
		f.Ref = 1
	}
	return f
}

// RefROM returns the ROM used to compare and restore characters.
func (f *Font) RefROM() []byte {
	if f.Ref == 1 {
		return CharsetLower
	}
	return CharsetUpper
}

// Pixel reports whether pixel (x, y) of character c is set.
func (f *Font) Pixel(c, x, y int) bool {
	return f.Data[(c&255)*8+y]&(0x80>>x) != 0
}

// SetPixel changes one pixel.
func (f *Font) SetPixel(c, x, y int, on bool) {
	i := (c&255)*8 + y
	if on {
		f.Data[i] |= 0x80 >> x
	} else {
		f.Data[i] &^= 0x80 >> x
	}
}

// PixW returns the width of a character in editable pixels (8 or 4).
func (f *Font) PixW() int {
	if f.Multi {
		return 4
	}
	return 8
}

// Get returns the value of an editable pixel: hires 0/1, multicolor the bit
// pair 0..3.
func (f *Font) Get(c, x, y int) int {
	b := f.Data[(c&255)*8+y]
	if f.Multi {
		return int(b>>(6-2*x)) & 3
	}
	return int(b>>(7-x)) & 1
}

// Set changes an editable pixel.
func (f *Font) Set(c, x, y, v int) {
	i := (c&255)*8 + y
	if f.Multi {
		sh := uint(6 - 2*x)
		f.Data[i] = f.Data[i]&^(3<<sh) | byte(v&3)<<sh
		return
	}
	f.SetPixel(c, x, y, v != 0)
}

// SlotColor returns the C64 color of a pixel value.
func (f *Font) SlotColor(v int) uint8 {
	if !f.Multi {
		if v != 0 {
			return f.FG & 15
		}
		return f.BG & 15
	}
	switch v {
	case 1:
		return f.MC1 & 15
	case 2:
		return f.MC2 & 15
	case 3:
		return f.FG & 7
	}
	return f.BG & 15
}

// ColorAt returns the color of the hires position (x 0..7) of a character.
func (f *Font) ColorAt(c, x, y int) uint8 {
	if f.Multi {
		return f.SlotColor(f.Get(c, x/2, y))
	}
	return f.SlotColor(f.Get(c, x, y))
}

// TileChar returns the character at tile position (tx, ty) for the start
// character c.
func (f *Font) TileChar(c, tx, ty int) int {
	if f.TileLayout == 1 {
		return (c + tx*64 + ty*128) & 255
	}
	return (c + tx + ty*16) & 255
}

// TileSize returns the size of the tile in characters.
func (f *Font) TileSize() (int, int) {
	t := TileSizes[min(max(f.Tile, 0), 3)]
	return t[0], t[1]
}

// TilePixel reads a pixel of the tile that starts at character c (x in
// editable pixels: 8 or 4 per character).
func (f *Font) TilePixel(c, x, y int) int {
	pw := f.PixW()
	return f.Get(f.TileChar(c, x/pw, y/8), x%pw, y%8)
}

// SetTilePixel changes a pixel of the tile.
func (f *Font) SetTilePixel(c, x, y, v int) {
	pw := f.PixW()
	f.Set(f.TileChar(c, x/pw, y/8), x%pw, y%8, v)
}

// TilePixels returns the size of the tile in editable pixels.
func (f *Font) TilePixels() (int, int) {
	tw, th := f.TileSize()
	return tw * f.PixW(), th * 8
}

// TileOp applies a pixel operation to the whole tile at c: get(x, y) is
// called on a copy and the result is written back.
func (f *Font) TileOp(c int, op func(get func(x, y int) int, w, h, x, y int) int) {
	w, h := f.TilePixels()
	old := *f
	get := func(x, y int) int {
		if x < 0 || y < 0 || x >= w || y >= h {
			return 0
		}
		return old.TilePixel(c, x, y)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			f.SetTilePixel(c, x, y, op(get, w, h, x, y))
		}
	}
}

// MakeInverse writes the inverted characters 0..127 into 128..255.
func (f *Font) MakeInverse() {
	for i := 0; i < 1024; i++ {
		f.Data[1024+i] = f.Data[i] ^ 0xFF
	}
}

// --- import ------------------------------------------------------------------------

// FontImport describes how characters are read from a picture.
type FontImport struct {
	StartCell  int  // first 8 × 8 cell of the picture (0..999, row by row)
	Count      int  // number of characters
	Target     int  // first character to fill
	BG         int  // -1 = most frequent color
	Multi      bool // multicolor: pixel pairs become the four colors
	KeepColors bool // multicolor: keep MC1, MC2 and the character color
}

// ImportFont reads characters from a 320 × 200 picture of C64 colors. Hires:
// every pixel that is not the background color is set. Multicolor: every
// pixel pair gets the nearest of the four colors (dist measures the distance
// of two C64 colors); the colors are found automatically or kept.
func (f *Font) ImportFont(px []int8, o FontImport, dist func(a, b int) int) (bg int) {
	cellOK := func(n int) bool { return o.StartCell+n < 1000 && o.Target+n < 256 }
	var h [16]int
	for n := 0; n < o.Count && cellOK(n); n++ {
		cell := o.StartCell + n
		cx, cy := cell%40*8, cell/40*8
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				if c := px[(cy+y)*320+cx+x]; c >= 0 {
					h[c]++
				}
			}
		}
	}
	bg = o.BG
	if bg < 0 {
		bg = 0
		for c := range h {
			if h[c] > h[bg] {
				bg = c
			}
		}
	}
	if o.Multi {
		f.Multi = true
		f.BG = uint8(bg)
		if !o.KeepColors {
			// the most frequent colors: one of 0..7 for the character color
			type cc struct{ c, n int }
			var list []cc
			for c := 0; c < 16; c++ {
				if c != bg && h[c] > 0 {
					list = append(list, cc{c, h[c]})
				}
			}
			for i := 1; i < len(list); i++ {
				for j := i; j > 0 && list[j].n > list[j-1].n; j-- {
					list[j], list[j-1] = list[j-1], list[j]
				}
			}
			fgSet := false
			var rest []int
			for _, e := range list {
				if !fgSet && e.c < 8 {
					f.FG, fgSet = uint8(e.c), true
				} else {
					rest = append(rest, e.c)
				}
			}
			if len(rest) > 0 {
				f.MC1 = uint8(rest[0])
			}
			if len(rest) > 1 {
				f.MC2 = uint8(rest[1])
			}
		}
		slots := [4]int{bg, int(f.MC1), int(f.MC2), int(f.FG & 7)}
		for n := 0; n < o.Count && cellOK(n); n++ {
			cell := o.StartCell + n
			cx, cy := cell%40*8, cell/40*8
			ch := o.Target + n
			for y := 0; y < 8; y++ {
				for x := 0; x < 4; x++ {
					c := int(px[(cy+y)*320+cx+x*2])
					best, bd := 0, 1<<62
					if c >= 0 {
						for k, sc := range slots {
							if d := dist(c, sc); d < bd {
								best, bd = k, d
							}
						}
					}
					f.Set(ch, x, y, best)
				}
			}
		}
		return bg
	}
	f.Multi = false
	for n := 0; n < o.Count; n++ {
		cell := o.StartCell + n
		if cell >= 1000 {
			break
		}
		ch := o.Target + n
		if ch > 255 {
			break
		}
		cx, cy := cell%40*8, cell/40*8
		for y := 0; y < 8; y++ {
			var v byte
			for x := 0; x < 8; x++ {
				if c := int(px[(cy+y)*320+cx+x]); c >= 0 && c != bg {
					v |= 0x80 >> x
				}
			}
			f.Data[ch*8+y] = v
		}
	}
	return bg
}

// LoadRawFont reads a raw character set: 2048 or 4096 bytes, optionally with
// a 2 byte load address (only the first set of a 4096 byte file is used).
func LoadRawFont(data []byte) (*Font, error) {
	switch len(data) {
	case 2050, 4098:
		data = data[2:]
	case 2048, 4096:
	default:
		if len(data) < 8 || len(data) > 4098 {
			return nil, fmt.Errorf("unexpected size %d bytes (expected 2048 or 4096)", len(data))
		}
		if len(data)%8 == 2 {
			data = data[2:]
		}
	}
	f := NewFont(-1)
	copy(f.Data[:], data)
	return f, nil
}

// --- project file ------------------------------------------------------------------

type fontFile struct {
	Format      string `json:"format"`
	Version     int    `json:"version"`
	Data        string `json:"data"` // 2048 bytes as hex
	Color       int    `json:"color"`
	Background  int    `json:"background"`
	Multicolor  bool   `json:"multicolor,omitempty"`
	Multicolor1 int    `json:"multicolor1"`
	Multicolor2 int    `json:"multicolor2"`
	Reference   string `json:"reference"`
	Tile        string `json:"tile"`
	TileLayout  string `json:"tileLayout"`
}

const fontFormat = "c64dpainter-font"

var tileLayoutNames = [2]string{"neighbors", "quarters"}

// Marshal writes the project file (JSON).
func (f *Font) Marshal() ([]byte, error) {
	ref := "upper"
	if f.Ref == 1 {
		ref = "lower"
	}
	return json.MarshalIndent(fontFile{Format: fontFormat, Version: 1, Data: hex.EncodeToString(f.Data[:]),
		Color: int(f.FG), Background: int(f.BG), Multicolor: f.Multi, Multicolor1: int(f.MC1), Multicolor2: int(f.MC2), Reference: ref,
		Tile: TileNames[min(max(f.Tile, 0), 3)], TileLayout: tileLayoutNames[f.TileLayout&1]}, "", " ")
}

// LoadFont reads a project file.
func LoadFont(data []byte) (*Font, error) {
	var ff fontFile
	if err := json.Unmarshal(data, &ff); err != nil || ff.Format != fontFormat {
		return nil, errors.New("not a C64 DPainter font file")
	}
	d, err := hex.DecodeString(ff.Data)
	if err != nil {
		return nil, err
	}
	f := &Font{FG: uint8(ff.Color) & 15, BG: uint8(ff.Background) & 15, Multi: ff.Multicolor,
		MC1: uint8(ff.Multicolor1) & 15, MC2: uint8(ff.Multicolor2) & 15}
	copy(f.Data[:], d)
	if ff.Reference == "lower" {
		f.Ref = 1
	}
	for i, n := range TileNames {
		if n == ff.Tile {
			f.Tile = i
		}
	}
	if ff.TileLayout == tileLayoutNames[1] {
		f.TileLayout = 1
	}
	return f, nil
}

// --- ACME export ---------------------------------------------------------------------

// charComment describes a screen code for the comments.
func charComment(c int, lower bool) string {
	inv := ""
	if c >= 128 {
		inv = " inverted"
	}
	b := c & 127
	var s string
	switch {
	case b == 0:
		s = "@"
	case b >= 1 && b <= 26:
		if lower {
			s = string(rune('a' + b - 1))
		} else {
			s = string(rune('A' + b - 1))
		}
	case b >= 32 && b <= 63:
		s = string(rune(b))
		if b == 32 {
			s = "space"
		}
	case lower && b >= 65 && b <= 90:
		s = string(rune('A' + b - 65))
	}
	if s == "" {
		return fmt.Sprintf("$%02x%s", c, inv)
	}
	return fmt.Sprintf("$%02x %s%s", c, s, inv)
}

// ACME returns assembler source with the character set and its settings.
func (f *Font) ACME(name string, addr int) string {
	var w strings.Builder
	label := asmLabel(name)
	tw, th := f.TileSize()
	fmt.Fprintf(&w, "; Character set %q – made with C64 DPainter\n", name)
	if f.Multi {
		w.WriteString("; 256 characters, multicolor (4 × 8 double wide pixels), 2048 bytes\n")
		w.WriteString("; bit pairs: %00 $d021, %01 $d022, %10 $d023, %11 character color (color RAM 8-15)\n")
	} else {
		w.WriteString("; 256 characters, 8 × 8 hires pixels, 2048 bytes\n")
	}
	fmt.Fprintf(&w, "; assemble: acme --cpu 6510 %s.asm  (writes %s.prg with load address)\n", name, name)
	w.WriteString("; the address must be a multiple of $800 inside the VIC bank;\n")
	w.WriteString("; $d018 bits 1-3 = (address & $3fff) / $800\n")
	if tw*th > 1 {
		lay := "neighbors: +1 right, +16 down"
		if f.TileLayout == 1 {
			lay = "quarters: +64 right, +128 down"
		}
		fmt.Fprintf(&w, "; drawn as %d×%d tiles (%s)\n", tw, th, lay)
	}
	fmt.Fprintf(&w, "\n!to \"%s.prg\", cbm\n\n", name)
	multi := 0
	if f.Multi {
		multi = 1
	}
	fmt.Fprintf(&w, "%s_MULTICOLOR = %d  ; 1 = multicolor: set bit 4 of $d016\n", label, multi)
	fmt.Fprintf(&w, "%s_BACKGROUND = %-2d ; $d021 (%s)\n", label, f.BG, ColorNames[f.BG&15])
	if f.Multi {
		fmt.Fprintf(&w, "%s_MC1        = %-2d ; $d022 (%s, bit pair %%01)\n", label, f.MC1, ColorNames[f.MC1&15])
		fmt.Fprintf(&w, "%s_MC2        = %-2d ; $d023 (%s, bit pair %%10)\n", label, f.MC2, ColorNames[f.MC2&15])
		fmt.Fprintf(&w, "%s_COLOR      = %-2d ; color RAM value: character color %d (%s) + 8 = multicolor\n", label, (f.FG&7)|8, f.FG&7, ColorNames[f.FG&7])
	} else {
		fmt.Fprintf(&w, "%s_COLOR      = %-2d ; character color (%s)\n", label, f.FG, ColorNames[f.FG&15])
	}
	fmt.Fprintf(&w, "%s_D018       = ((%s_data & $3fff) / $800) * 2 ; or with the screen bits\n\n", label, strings.ToLower(label))
	fmt.Fprintf(&w, "* = $%04x\n\n", addr)
	fmt.Fprintf(&w, "%s_data\n", strings.ToLower(label))
	lower := f.Ref == 1
	for c := 0; c < 256; c++ {
		fmt.Fprintf(&w, "        ; %s\n", charComment(c, lower))
		for y := 0; y < 8; y++ {
			v := f.Data[c*8+y]
			var sb strings.Builder
			for bit := 7; bit >= 0; bit-- {
				on := v>>bit&1 != 0
				switch {
				case f.Multi && on:
					sb.WriteByte('1')
				case f.Multi:
					sb.WriteByte('0')
				case on:
					sb.WriteByte('#')
				default:
					sb.WriteByte('.')
				}
			}
			fmt.Fprintf(&w, "        !byte %%%s\n", sb.String())
		}
	}
	return w.String()
}
