package c64

import (
	"fmt"
	"math/bits"
	"sort"
	"strings"
)

// --- Picture → character set + screen map (1.1) -----------------------------------------
//
// A 320 × 200 picture is cut into 40 × 25 cells of 8 × 8 pixels. Every cell
// becomes a character; equal characters are stored once. When more than the
// allowed number of characters remain, the most similar ones are merged
// (the rarer character takes the shape of the more common one). The result
// is a character set, a 40 × 25 screen map and the colors of the cells.
//
// Hires: background $D021 plus one color per cell (color RAM).
// Multicolor: background $D021, $D022 and $D023 for all cells plus one color
// per cell (color RAM, colors 0–7; the cell is shown in multicolor).

// CharMapOptions control the conversion.
type CharMapOptions struct {
	Multi   bool
	BG      int // -1 = automatic (most frequent color)
	MC1     int // -1 = automatic
	MC2     int // -1 = automatic
	Limit   int // most characters (1..256)
	Similar int // merge characters that differ in at most this many pixels
}

// CharMap is the result.
type CharMap struct {
	Font    *Font
	Map     [Cells]byte
	Colors  [Cells]byte // color RAM (multicolor: with bit 3 set)
	Unique  int         // different characters before merging
	Used    int         // characters in the set
	Opts    CharMapOptions
	cellPix [Cells][64]uint8
}

type cmChar struct {
	data  [8]byte
	uses  int
	alive bool
	to    int // merged into
}

func charDist(a, b *[8]byte, multi bool) int {
	d := 0
	for i := 0; i < 8; i++ {
		x := a[i] ^ b[i]
		if multi {
			// count multicolor pixels (bit pairs), not bits
			x = (x | x>>1) & 0x55
		}
		d += bits.OnesCount8(x)
	}
	return d
}

// ConvertCharMap converts a 320 × 200 picture (RGB, row by row).
func ConvertCharMap(px [][3]int, pal *Palette, o CharMapOptions) *CharMap {
	if o.Limit < 1 || o.Limit > 256 {
		o.Limit = 256
	}
	near := func(c [3]int, set []uint8) uint8 {
		best, bd := set[0], -1
		for _, k := range set {
			r, g, b := rgbOf(pal.Colors[k])
			if d := ColorDist(c[0], c[1], c[2], r, g, b); bd < 0 || d < bd {
				best, bd = k, d
			}
		}
		return best
	}
	all := make([]uint8, 16)
	for i := range all {
		all[i] = uint8(i)
	}
	// source pixels: multicolor uses pairs of pixels
	pw := 320
	src := px
	if o.Multi {
		pw = 160
		src = make([][3]int, 160*200)
		for i := range src {
			l, r := px[(i/160)*320+(i%160)*2], px[(i/160)*320+(i%160)*2+1]
			src[i] = [3]int{(l[0] + r[0]) / 2, (l[1] + r[1]) / 2, (l[2] + r[2]) / 2}
		}
	}
	q := make([]uint8, len(src))
	var freq [16]int
	for i, c := range src {
		q[i] = near(c, all)
		freq[q[i]]++
	}
	order := make([]int, 16)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return freq[order[i]] > freq[order[j]] })
	pick := func(v int, skip ...int) uint8 {
		if v >= 0 {
			return uint8(v & 15)
		}
	next:
		for _, c := range order {
			for _, s := range skip {
				if c == s {
					continue next
				}
			}
			return uint8(c)
		}
		return 0
	}
	bg := pick(o.BG)
	var mc1, mc2 uint8
	if o.Multi {
		mc1 = pick(o.MC1, int(bg))
		mc2 = pick(o.MC2, int(bg), int(mc1))
	}
	f := &Font{FG: 1, BG: bg, Multi: o.Multi, MC1: mc1, MC2: mc2}
	cm := &CharMap{Font: f, Opts: o}
	cw := 8
	if o.Multi {
		cw = 4
	}
	var chars []*cmChar
	index := map[[8]byte]int{}
	var cellChar [Cells]int
	for cell := 0; cell < Cells; cell++ {
		cx, cy := (cell%Cols)*cw, (cell/Cols)*8
		// the cell color: the one that fits the cell best
		var cands []uint8
		if o.Multi {
			for c := uint8(0); c < 8; c++ {
				cands = append(cands, c)
			}
		} else {
			for c := uint8(0); c < 16; c++ {
				cands = append(cands, c)
			}
		}
		best, bestErr := uint8(1), -1
		for _, c := range cands {
			set := []uint8{bg, c}
			if o.Multi {
				set = []uint8{bg, mc1, mc2, c}
			}
			e := 0
			for y := 0; y < 8; y++ {
				for x := 0; x < cw; x++ {
					s := src[(cy+y)*pw+cx+x]
					k := near(s, set)
					r, g, b := rgbOf(pal.Colors[k])
					e += ColorDist(s[0], s[1], s[2], r, g, b)
				}
			}
			if bestErr < 0 || e < bestErr {
				best, bestErr = c, e
			}
		}
		var data [8]byte
		for y := 0; y < 8; y++ {
			var b byte
			for x := 0; x < cw; x++ {
				s := src[(cy+y)*pw+cx+x]
				if o.Multi {
					k := near(s, []uint8{bg, mc1, mc2, best})
					v := 0
					switch k {
					case bg:
						v = 0
					case mc1:
						v = 1
					case mc2:
						v = 2
					default:
						v = 3
					}
					b |= byte(v) << (6 - 2*x)
				} else if near(s, []uint8{bg, best}) != bg {
					b |= 0x80 >> x
				}
			}
			data[y] = b
		}
		if o.Multi {
			cm.Colors[cell] = best | 8
		} else {
			cm.Colors[cell] = best
		}
		k, ok := index[data]
		if !ok {
			k = len(chars)
			index[data] = k
			chars = append(chars, &cmChar{data: data, alive: true, to: -1})
		}
		chars[k].uses++
		cellChar[cell] = k
	}
	cm.Unique = len(chars)
	// merge similar characters (and as many as needed for the limit)
	alive := len(chars)
	type pair struct{ a, b, d int }
	var pairs []pair
	if alive > o.Limit || o.Similar > 0 {
		for i := range chars {
			for j := i + 1; j < len(chars); j++ {
				pairs = append(pairs, pair{i, j, charDist(&chars[i].data, &chars[j].data, o.Multi)})
			}
		}
		sort.Slice(pairs, func(i, j int) bool {
			if pairs[i].d != pairs[j].d {
				return pairs[i].d < pairs[j].d
			}
			return min(chars[pairs[i].a].uses, chars[pairs[i].b].uses) < min(chars[pairs[j].a].uses, chars[pairs[j].b].uses)
		})
	}
	root := func(k int) int {
		for chars[k].to >= 0 {
			k = chars[k].to
		}
		return k
	}
	for _, p := range pairs {
		if alive <= o.Limit && p.d > o.Similar {
			break
		}
		a, b := root(p.a), root(p.b)
		if a == b || !chars[a].alive || !chars[b].alive {
			continue
		}
		// the rarer character becomes the more common one
		if chars[a].uses < chars[b].uses {
			a, b = b, a
		}
		if charDist(&chars[a].data, &chars[b].data, o.Multi) != p.d && p.d <= o.Similar {
			// already merged into something else: check the real distance
			if charDist(&chars[a].data, &chars[b].data, o.Multi) > o.Similar && alive <= o.Limit {
				continue
			}
		}
		chars[b].alive = false
		chars[b].to = a
		chars[a].uses += chars[b].uses
		alive--
	}
	// number the characters
	num := map[int]int{}
	n := 0
	for k, c := range chars {
		if c.alive {
			num[k] = n
			copy(f.Data[n*8:n*8+8], c.data[:])
			n++
		}
	}
	cm.Used = n
	for cell := 0; cell < Cells; cell++ {
		cm.Map[cell] = byte(num[root(cellChar[cell])])
	}
	return cm
}

// ColorAt returns the color of a pixel of the converted screen (320 × 200).
func (cm *CharMap) ColorAt(x, y int) uint8 {
	cell := (y/8)*Cols + x/8
	ch := int(cm.Map[cell])
	f := cm.Font
	b := f.Data[ch*8+y%8]
	col := cm.Colors[cell] & 15
	if f.Multi {
		v := (b >> (6 - 2*((x%8)/2))) & 3
		switch v {
		case 0:
			return f.BG
		case 1:
			return f.MC1
		case 2:
			return f.MC2
		}
		return col & 7
	}
	if b&(0x80>>(x%8)) != 0 {
		return col
	}
	return f.BG
}

func asmBytes(w *strings.Builder, label, comment string, data []byte, per int) {
	fmt.Fprintf(w, "%s ; %s\n", label, comment)
	for i := 0; i < len(data); i += per {
		var parts []string
		for _, v := range data[i:min(len(data), i+per)] {
			parts = append(parts, fmt.Sprintf("$%02x", v))
		}
		fmt.Fprintf(w, "        !byte %s\n", strings.Join(parts, ","))
	}
}

// DataACME returns the data file: map, colors and the character set (at
// $3000).
func (cm *CharMap) DataACME(name string) string {
	var w strings.Builder
	f := cm.Font
	fmt.Fprintf(&w, "; %s – character set, screen map and colors (made with C64 DPainter)\n", name)
	mode := "hires"
	if f.Multi {
		mode = "multicolor"
	}
	fmt.Fprintf(&w, "; %s, %d characters, background %d", mode, cm.Used, f.BG)
	if f.Multi {
		fmt.Fprintf(&w, ", $d022 = %d, $d023 = %d", f.MC1, f.MC2)
	}
	w.WriteString("\n\n")
	fmt.Fprintf(&w, "cm_bg = %d\ncm_mc1 = %d\ncm_mc2 = %d\ncm_multi = %d\n\n", f.BG, f.MC1, f.MC2, map[bool]int{false: 0, true: 1}[f.Multi])
	asmBytes(&w, "cm_map", "40 × 25 screen codes (character numbers)", cm.Map[:], 40)
	w.WriteString("\n")
	asmBytes(&w, "cm_colors", "40 × 25 colors for the color RAM", cm.Colors[:], 40)
	w.WriteString("\n* = $3000\n")
	asmBytes(&w, "cm_charset", fmt.Sprintf("character set, %d characters × 8 bytes", cm.Used), f.Data[:max(8, cm.Used*8)], 16)
	return w.String()
}

// ProgramACME returns the viewer program; it includes the data file with
// !source (inline: the data text is put in its place).
func (cm *CharMap) ProgramACME(name, dataFile string, inline string) string {
	var w strings.Builder
	fmt.Fprintf(&w, "; %s – shows a picture made of characters (made with C64 DPainter)\n", name)
	fmt.Fprintf(&w, "; assemble: acme --cpu 6510 --format cbm --outfile %s.prg %s.asm\n", name, name)
	w.WriteString("; Space resets the C64.\n\n!cpu 6510\n* = $0801\n")
	w.WriteString("        !byte $0b,$08,$ea,$07,$9e,$32,$30,$36,$31,$00,$00,$00 ; 2026 SYS2061\n\n* = $080d\n")
	w.WriteString(`start   sei
        lda #cm_bg
        sta $d020
        sta $d021
        lda #cm_mc1
        sta $d022
        lda #cm_mc2
        sta $d023
        lda #$1b          ; text mode
        sta $d011
        lda #$08
        ldx #cm_multi
        beq hires
        lda #$18          ; multicolor characters
hires   sta $d016
        lda #$1c          ; screen $0400, characters $3000
        sta $d018
        ldx #0
copy    lda cm_map,x
        sta $0400,x
        lda cm_map+$100,x
        sta $0500,x
        lda cm_map+$200,x
        sta $0600,x
        lda cm_map+$2e8,x
        sta $06e8,x
        lda cm_colors,x
        sta $d800,x
        lda cm_colors+$100,x
        sta $d900,x
        lda cm_colors+$200,x
        sta $da00,x
        lda cm_colors+$2e8,x
        sta $dae8,x
        inx
        bne copy
        cli
wait    lda #$7f          ; Space pressed?
        sta $dc00
        lda $dc01
        and #$10
        bne wait
        jmp $fce2         ; reset

`)
	if inline != "" {
		w.WriteString(inline)
	} else {
		fmt.Fprintf(&w, "        !source \"%s\"\n", dataFile)
	}
	return w.String()
}

// PRG returns the runnable viewer.
func (cm *CharMap) PRG() ([]byte, error) {
	return Assemble(cm.ProgramACME("charmap", "", cm.DataACME("charmap")))
}
