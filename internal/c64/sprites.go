package c64

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// --- Sprites --------------------------------------------------------------------------
//
// A C64 sprite is 24 × 21 pixels in 63 bytes (3 bytes per row). In hires
// every bit is a pixel in the sprite color. In multicolor two bits make one
// double wide pixel (12 × 21):
//
//	%00 transparent, %01 multicolor 1 ($D025), %10 sprite color ($D027+n),
//	%11 multicolor 2 ($D026).
//
// The editor slots follow the order a painter thinks in: 0 transparent,
// 1 sprite color, 2 multicolor 1, 3 multicolor 2.

const (
	SpriteW     = 24
	SpriteH     = 21
	SpriteBytes = 63
	MaxSprites  = 64
)

// Sprite is one sprite with its own settings.
type Sprite struct {
	Data    [SpriteBytes]byte
	Multi   bool  // multicolor
	Color   uint8 // sprite color ($D027+n)
	ExpandX bool  // $D01D
	ExpandY bool  // $D017
}

// SpriteBank is the whole sprite project.
type SpriteBank struct {
	Sprites  []Sprite
	BG       uint8 // background ($D021), for display
	MC1, MC2 uint8 // $D025, $D026 – common to all sprites
	// animation preview
	AnimFrom, AnimTo int
	AnimFPS          int
	AnimPingPong     bool
}

// NewSpriteBank returns a bank with one empty multicolor sprite.
func NewSpriteBank() *SpriteBank {
	return &SpriteBank{Sprites: []Sprite{{Multi: true, Color: 1}}, BG: 6, MC1: 11, MC2: 12, AnimFPS: 8}
}

// Clone returns a deep copy.
func (b *SpriteBank) Clone() *SpriteBank {
	c := *b
	c.Sprites = append([]Sprite(nil), b.Sprites...)
	return &c
}

// PixelW returns the editable width (24 hires, 12 multicolor).
func (s *Sprite) PixelW() int {
	if s.Multi {
		return 12
	}
	return 24
}

// Get returns the slot of a pixel (x in editable pixels): 0 transparent,
// 1 sprite color, 2 multicolor 1, 3 multicolor 2.
func (s *Sprite) Get(x, y int) int {
	if x < 0 || y < 0 || y >= SpriteH || x >= s.PixelW() {
		return 0
	}
	if !s.Multi {
		return int(s.Data[y*3+x/8]>>(7-x%8)) & 1
	}
	v := int(s.Data[y*3+x/4]>>(6-2*(x%4))) & 3
	return [4]int{0, 2, 1, 3}[v]
}

// Set changes the slot of a pixel.
func (s *Sprite) Set(x, y, slot int) {
	if x < 0 || y < 0 || y >= SpriteH || x >= s.PixelW() {
		return
	}
	if !s.Multi {
		i, bit := y*3+x/8, byte(0x80>>(x%8))
		if slot != 0 {
			s.Data[i] |= bit
		} else {
			s.Data[i] &^= bit
		}
		return
	}
	v := byte([4]int{0, 2, 1, 3}[slot&3])
	i, sh := y*3+x/4, uint(6-2*(x%4))
	s.Data[i] = s.Data[i]&^(3<<sh) | v<<sh
}

// SlotColor returns the C64 color of a slot (-1 = transparent).
func (b *SpriteBank) SlotColor(s *Sprite, slot int) int {
	switch slot {
	case 1:
		return int(s.Color)
	case 2:
		return int(b.MC1)
	case 3:
		return int(b.MC2)
	}
	return -1
}

// ColorAt returns the C64 color of a hires pixel (x 0..23), -1 = transparent.
func (b *SpriteBank) ColorAt(s *Sprite, x, y int) int {
	if s.Multi {
		return b.SlotColor(s, s.Get(x/2, y))
	}
	return b.SlotColor(s, s.Get(x, y))
}

// Pointer byte: the 64th byte as SpritePad writes it (bit 7 multicolor,
// bits 0–3 sprite color).
func (s *Sprite) infoByte() byte {
	v := s.Color & 15
	if s.Multi {
		v |= 0x80
	}
	return v
}

// FlipH mirrors a sprite horizontally (in editable pixels).
func (s *Sprite) FlipH() {
	o := *s
	w := s.PixelW()
	for y := 0; y < SpriteH; y++ {
		for x := 0; x < w; x++ {
			s.Set(x, y, o.Get(w-1-x, y))
		}
	}
}

// FlipV mirrors a sprite vertically.
func (s *Sprite) FlipV() {
	o := *s
	for y := 0; y < SpriteH; y++ {
		copy(s.Data[y*3:y*3+3], o.Data[(SpriteH-1-y)*3:(SpriteH-1-y)*3+3])
	}
}

// Shift moves the pixels by one (wrapping around).
func (s *Sprite) Shift(dx, dy int) {
	o := *s
	w := s.PixelW()
	for y := 0; y < SpriteH; y++ {
		for x := 0; x < w; x++ {
			s.Set(x, y, o.Get((x-dx+w)%w, (y-dy+SpriteH)%SpriteH))
		}
	}
}

// Invert flips all bits (hires: set ↔ clear, multicolor: %00 ↔ %11, %01 ↔ %10).
func (s *Sprite) Invert() {
	for i := range s.Data {
		s.Data[i] ^= 0xFF
	}
}

// Empty reports whether no pixel is set.
func (s *Sprite) Empty() bool {
	for _, v := range s.Data {
		if v != 0 {
			return false
		}
	}
	return true
}

// --- import from a 320 × 200 picture ------------------------------------------------

// SpriteImport describes how sprites are taken from a picture of C64 colors
// (320 × 200 color indexes).
type SpriteImport struct {
	X, Y       int  // top left (screen pixels)
	Cols, Rows int  // raster of sprites
	GapX, GapY int  // space between the sprites
	Multi      bool // multicolor or hires
	BG         int  // -1 = most frequent color of the area
	KeepShared bool // keep MC1/MC2 of the bank (multicolor)
}

// Rect returns the area of sprite n of the raster.
func (o SpriteImport) Rect(n int) (x, y int) {
	c, r := n%max(1, o.Cols), n/max(1, o.Cols)
	return o.X + c*(SpriteW+o.GapX), o.Y + r*(SpriteH+o.GapY)
}

// ImportSprites cuts sprites out of a picture (C64 color indexes, 320 × 200,
// -1 outside). It returns the sprites and the colors to use: background,
// multicolor 1 and 2. dist gives the distance between two C64 colors.
func ImportSprites(px []int8, b *SpriteBank, o SpriteImport, dist func(a, b int) int) (out []Sprite, bg, mc1, mc2 uint8) {
	n := max(1, o.Cols) * max(1, o.Rows)
	at := func(x, y int) int {
		if x < 0 || y < 0 || x >= 320 || y >= 200 {
			return -1
		}
		return int(px[y*320+x])
	}
	// histograms: all sprites and each one
	var all [16]int
	hist := make([][16]int, n)
	for i := 0; i < n; i++ {
		sx, sy := o.Rect(i)
		for y := 0; y < SpriteH; y++ {
			for x := 0; x < SpriteW; x++ {
				if c := at(sx+x, sy+y); c >= 0 {
					all[c]++
					hist[i][c]++
				}
			}
		}
	}
	order := func(h [16]int, skip ...int) []int {
		idx := make([]int, 0, 16)
		for c := 0; c < 16; c++ {
			if h[c] > 0 && !containsInt(skip, c) {
				idx = append(idx, c)
			}
		}
		sort.SliceStable(idx, func(i, j int) bool { return h[idx[i]] > h[idx[j]] })
		return idx
	}
	if o.BG >= 0 {
		bg = uint8(o.BG)
	} else if f := order(all); len(f) > 0 {
		bg = uint8(f[0])
	} else {
		bg = b.BG
	}
	mc1, mc2 = b.MC1, b.MC2
	if o.Multi && !o.KeepShared {
		// the two colors used in the most sprites become the shared colors
		var spread [16]int
		for i := range hist {
			for c := 0; c < 16; c++ {
				if hist[i][c] > 0 {
					spread[c] += 1000 + hist[i][c]
				}
			}
		}
		f := order(spread, int(bg))
		if len(f) > 0 {
			mc1 = uint8(f[0])
		}
		if len(f) > 1 {
			mc2 = uint8(f[1])
		}
		// with only one sprite the most frequent color is better as sprite color
		if n == 1 && len(f) > 2 {
			mc1, mc2 = uint8(f[1]), uint8(f[2])
		}
	}
	for i := 0; i < n; i++ {
		s := Sprite{Multi: o.Multi}
		sx, sy := o.Rect(i)
		skip := []int{int(bg)}
		if o.Multi {
			skip = append(skip, int(mc1), int(mc2))
		}
		if f := order(hist[i], skip...); len(f) > 0 {
			s.Color = uint8(f[0])
		} else if o.Multi {
			s.Color = 1
		} else if f := order(hist[i], int(bg)); len(f) > 0 {
			s.Color = uint8(f[0])
		} else {
			s.Color = 1
		}
		if !o.Multi {
			for y := 0; y < SpriteH; y++ {
				for x := 0; x < SpriteW; x++ {
					if c := at(sx+x, sy+y); c >= 0 && c != int(bg) {
						s.Set(x, y, 1)
					}
				}
			}
		} else {
			slots := [4]int{int(bg), int(s.Color), int(mc1), int(mc2)}
			for y := 0; y < SpriteH; y++ {
				for x := 0; x < 12; x++ {
					c := at(sx+x*2, sy+y)
					if c < 0 {
						continue
					}
					best, bd := 0, 1<<62
					for k, sc := range slots {
						if d := dist(c, sc); d < bd {
							best, bd = k, d
						}
					}
					s.Set(x, y, best)
				}
			}
		}
		out = append(out, s)
	}
	return out, bg, mc1, mc2
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// --- project file ------------------------------------------------------------------

type spriteFileSprite struct {
	Data    string `json:"data"` // 63 bytes as hex
	Multi   bool   `json:"multicolor"`
	Color   int    `json:"color"`
	ExpandX bool   `json:"expandX,omitempty"`
	ExpandY bool   `json:"expandY,omitempty"`
}

type spriteFile struct {
	Format       string             `json:"format"`
	Version      int                `json:"version"`
	Background   int                `json:"background"`
	Multicolor1  int                `json:"multicolor1"`
	Multicolor2  int                `json:"multicolor2"`
	AnimFrom     int                `json:"animFrom"`
	AnimTo       int                `json:"animTo"`
	AnimFPS      int                `json:"animFPS"`
	AnimPingPong bool               `json:"animPingPong"`
	Sprites      []spriteFileSprite `json:"sprites"`
}

const spriteFormat = "c64dpainter-sprites"

// Marshal writes the project file (JSON).
func (b *SpriteBank) Marshal() ([]byte, error) {
	f := spriteFile{Format: spriteFormat, Version: 1, Background: int(b.BG), Multicolor1: int(b.MC1), Multicolor2: int(b.MC2),
		AnimFrom: b.AnimFrom, AnimTo: b.AnimTo, AnimFPS: b.AnimFPS, AnimPingPong: b.AnimPingPong}
	for _, s := range b.Sprites {
		f.Sprites = append(f.Sprites, spriteFileSprite{hex.EncodeToString(s.Data[:]), s.Multi, int(s.Color), s.ExpandX, s.ExpandY})
	}
	return json.MarshalIndent(f, "", " ")
}

// LoadSpriteBank reads a project file.
func LoadSpriteBank(data []byte) (*SpriteBank, error) {
	var f spriteFile
	if err := json.Unmarshal(data, &f); err != nil || f.Format != spriteFormat {
		return nil, errors.New("not a C64 DPainter sprite file")
	}
	b := &SpriteBank{BG: uint8(f.Background) & 15, MC1: uint8(f.Multicolor1) & 15, MC2: uint8(f.Multicolor2) & 15,
		AnimFrom: f.AnimFrom, AnimTo: f.AnimTo, AnimFPS: f.AnimFPS, AnimPingPong: f.AnimPingPong}
	for i, fs := range f.Sprites {
		if i >= MaxSprites {
			break
		}
		s := Sprite{Multi: fs.Multi, Color: uint8(fs.Color) & 15, ExpandX: fs.ExpandX, ExpandY: fs.ExpandY}
		d, err := hex.DecodeString(fs.Data)
		if err != nil {
			return nil, fmt.Errorf("sprite %d: %v", i, err)
		}
		copy(s.Data[:], d)
		b.Sprites = append(b.Sprites, s)
	}
	if len(b.Sprites) == 0 {
		b.Sprites = []Sprite{{Multi: true, Color: 1}}
	}
	b.ClampAnim()
	return b, nil
}

// ClampAnim keeps the animation settings valid.
func (b *SpriteBank) ClampAnim() {
	n := len(b.Sprites)
	b.AnimFrom = min(max(b.AnimFrom, 0), n-1)
	b.AnimTo = min(max(b.AnimTo, b.AnimFrom), n-1)
	b.AnimFPS = min(max(b.AnimFPS, 1), 50)
}

// --- ACME export ---------------------------------------------------------------------

// spriteRowSource writes one sprite row in binary: hires with . and #,
// multicolor with the bit pairs as 0/1 (ACME understands both).
func spriteRowSource(s *Sprite, y int) string {
	parts := make([]string, 3)
	for i := 0; i < 3; i++ {
		v := s.Data[y*3+i]
		var sb strings.Builder
		sb.WriteByte('%')
		for bit := 7; bit >= 0; bit-- {
			on := v>>bit&1 != 0
			switch {
			case s.Multi && on:
				sb.WriteByte('1')
			case s.Multi:
				sb.WriteByte('0')
			case on:
				sb.WriteByte('#')
			default:
				sb.WriteByte('.')
			}
		}
		parts[i] = sb.String()
	}
	return strings.Join(parts, ",")
}

// ACME returns assembler source with the sprite data and all settings. The
// sprites are aligned to 64 bytes; the 64th byte holds the multicolor flag
// (bit 7) and the sprite color (bits 0–3), as in SpritePad files.
func (b *SpriteBank) ACME(name string, addr int) string {
	var w strings.Builder
	cn := func(c uint8) string { return ColorNames[c&15] }
	label := asmLabel(name)
	fmt.Fprintf(&w, "; Sprites %q – made with C64 DPainter\n", name)
	fmt.Fprintf(&w, "; %d sprites, 64 bytes each (63 bytes data + 1 byte info)\n", len(b.Sprites))
	fmt.Fprintf(&w, "; assemble: acme --cpu 6510 %s.asm  (writes %s.prg with load address)\n", name, name)
	w.WriteString("; to include it into your own source, remove the !to line and set * to\n")
	w.WriteString("; a multiple of 64 inside the VIC bank; pointer = (address & $3fff) / 64\n\n")
	fmt.Fprintf(&w, "!to \"%s.prg\", cbm\n\n", name)
	fmt.Fprintf(&w, "%s_COUNT       = %d\n", label, len(b.Sprites))
	fmt.Fprintf(&w, "%s_BACKGROUND  = %-2d ; $d021  %s\n", label, b.BG, cn(b.BG))
	fmt.Fprintf(&w, "%s_MULTICOLOR1 = %-2d ; $d025  %s (bit pair %%01)\n", label, b.MC1, cn(b.MC1))
	fmt.Fprintf(&w, "%s_MULTICOLOR2 = %-2d ; $d026  %s (bit pair %%11)\n", label, b.MC2, cn(b.MC2))
	fmt.Fprintf(&w, "%s_ANIM_FROM   = %d\n", label, b.AnimFrom)
	fmt.Fprintf(&w, "%s_ANIM_TO     = %d\n", label, b.AnimTo)
	fmt.Fprintf(&w, "%s_ANIM_FPS    = %d\n", label, b.AnimFPS)
	pp := 0
	if b.AnimPingPong {
		pp = 1
	}
	fmt.Fprintf(&w, "%s_ANIM_PINGPONG = %d\n\n", label, pp)
	fmt.Fprintf(&w, "* = $%04x\n\n", addr)
	fmt.Fprintf(&w, "%s_data\n", strings.ToLower(label))
	for i := range b.Sprites {
		s := &b.Sprites[i]
		mode := "hires"
		if s.Multi {
			mode = "multicolor"
		}
		var ex []string
		if s.ExpandX {
			ex = append(ex, "expand X")
		}
		if s.ExpandY {
			ex = append(ex, "expand Y")
		}
		exs := ""
		if len(ex) > 0 {
			exs = ", " + strings.Join(ex, ", ")
		}
		fmt.Fprintf(&w, "; sprite %d – %s, color %d (%s)%s, pointer +%d\n", i, mode, s.Color, cn(s.Color), exs, i)
		fmt.Fprintf(&w, "%s_%d\n", strings.ToLower(label), i)
		for y := 0; y < SpriteH; y++ {
			fmt.Fprintf(&w, "        !byte %s\n", spriteRowSource(s, y))
		}
		fmt.Fprintf(&w, "        !byte $%02x ; info: bit 7 multicolor, bits 0-3 color\n\n", s.infoByte())
	}
	w.WriteString("; per sprite tables (index = sprite number)\n")
	table := func(lbl, comment string, f func(s *Sprite) int) {
		fmt.Fprintf(&w, "%s_%s ; %s\n", strings.ToLower(label), lbl, comment)
		for i := 0; i < len(b.Sprites); i += 16 {
			var v []string
			for j := i; j < min(i+16, len(b.Sprites)); j++ {
				v = append(v, fmt.Sprintf("%d", f(&b.Sprites[j])))
			}
			fmt.Fprintf(&w, "        !byte %s\n", strings.Join(v, ","))
		}
	}
	bool01 := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	table("colors", "$d027+n", func(s *Sprite) int { return int(s.Color) })
	table("multicolor", "1 = multicolor ($d01c bit)", func(s *Sprite) int { return bool01(s.Multi) })
	table("expand_x", "1 = double width ($d01d bit)", func(s *Sprite) int { return bool01(s.ExpandX) })
	table("expand_y", "1 = double height ($d017 bit)", func(s *Sprite) int { return bool01(s.ExpandY) })
	return w.String()
}

// RawData returns the sprites as 64 byte blocks (as SpritePad and most tools
// use them).
func (b *SpriteBank) RawData() []byte {
	out := make([]byte, 0, 64*len(b.Sprites))
	for i := range b.Sprites {
		out = append(out, b.Sprites[i].Data[:]...)
		out = append(out, b.Sprites[i].infoByte())
	}
	return out
}

// asmLabel makes an upper case assembler label out of a file name.
func asmLabel(name string) string {
	var sb strings.Builder
	for i, r := range strings.ToUpper(name) {
		switch {
		case r >= 'A' && r <= 'Z', r == '_':
			sb.WriteRune(r)
		case r >= '0' && r <= '9':
			if i == 0 {
				sb.WriteByte('_')
			}
			sb.WriteRune(r)
		default:
			sb.WriteByte('_')
		}
	}
	if sb.Len() == 0 {
		return "DATA"
	}
	return sb.String()
}
