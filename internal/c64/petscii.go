package c64

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
)

// --- PETSCII (standard character mode) ------------------------------------------------
//
// 40 × 25 characters of 8 × 8 pixels from the character ROM. Every character
// has its own color (color RAM); the background ($D021) is shared.

const (
	PetCols  = 40
	PetRows  = 25
	PetCells = PetCols * PetRows
)

// PetsciiScreen is one PETSCII picture.
type PetsciiScreen struct {
	Chars  [PetCells]uint8 // screen codes
	Colors [PetCells]uint8
	BG     uint8
	Border uint8
	Lower  bool // lower case character set
}

// NewPetscii returns an empty screen (spaces).
func NewPetscii(bg, border uint8) *PetsciiScreen {
	s := &PetsciiScreen{BG: bg & 15, Border: border & 15}
	for i := range s.Chars {
		s.Chars[i] = 32
		s.Colors[i] = 14
	}
	return s
}

// Charset returns the 2048 bytes of the character set in use.
func (s *PetsciiScreen) Charset() []byte {
	if s.Lower {
		return CharsetLower
	}
	return CharsetUpper
}

// PixelOn reports whether the hires pixel (x 0..319, y 0..199) is set.
func (s *PetsciiScreen) PixelOn(x, y int) bool {
	c := s.Chars[(y/8)*PetCols+x/8]
	return s.Charset()[int(c)*8+y%8]&(0x80>>(x%8)) != 0
}

// ColorAt returns the C64 color of a hires pixel.
func (s *PetsciiScreen) ColorAt(x, y int) uint8 {
	if s.PixelOn(x, y) {
		return s.Colors[(y/8)*PetCols+x/8]
	}
	return s.BG
}

// PetsciiOptions control the conversion of an image into PETSCII.
type PetsciiOptions struct {
	BG      int        // -1 = automatic
	Lower   bool       // lower case character set
	Inverse bool       // allow the inverted characters 128..255
	Chars   int        // 0 = all characters, 1 = no letters/digits, 2 = blocks only
	Match   ColorMatch // color distance
	Colors  [16]bool   // allowed foreground colors (all false = all allowed)
}

// petsciiBlockChars: block and shade characters of the upper case set.
var petsciiBlockChars = []int{32, 97, 98, 108, 123, 124, 126, 127, 92, 102, 104, 116, 117, 118, 119, 120, 121, 106, 111, 101, 103, 99, 100}

// petsciiAllowedChars returns the characters a conversion may use.
func petsciiAllowedChars(o PetsciiOptions) []int {
	var base []int
	switch o.Chars {
	case 1: // no letters, digits and punctuation
		base = append(base, 32)
		for c := 64; c < 128; c++ {
			if o.Lower && c >= 65 && c <= 90 {
				continue // upper case letters in the lower case set
			}
			base = append(base, c)
		}
	case 2:
		base = append(base, petsciiBlockChars...)
	default:
		for c := 0; c < 128; c++ {
			base = append(base, c)
		}
	}
	if !o.Inverse {
		return base
	}
	out := append([]int(nil), base...)
	for _, c := range base {
		out = append(out, c+128)
	}
	return out
}

// ConvertPetscii turns a 320 × 200 RGB screen into PETSCII: for every 8 × 8
// cell the character and color that match the cell best are chosen.
func ConvertPetscii(px [][3]int, pal *Palette, o PetsciiOptions) *PetsciiScreen {
	var pr [16][3]int
	for i, c := range pal.Colors {
		pr[i] = [3]int{int(c >> 16 & 255), int(c >> 8 & 255), int(c & 255)}
	}
	metric := colorMetric(o.Match)
	allowedFG := []int{}
	for k := 0; k < 16; k++ {
		if o.Colors[k] {
			allowedFG = append(allowedFG, k)
		}
	}
	if len(allowedFG) == 0 {
		allowedFG = all16
	}
	// distance of every pixel to every color
	dist := make([][16]int32, 320*200)
	var hist [16]int
	for i, c := range px {
		best, bd := 0, int32(1<<30)
		for k := 0; k < 16; k++ {
			d := int32(min(metric(c[0], c[1], c[2], pr[k][0], pr[k][1], pr[k][2]), 1<<30))
			dist[i][k] = d
			if d < bd {
				bd, best = d, k
			}
		}
		hist[best]++
	}
	s := &PetsciiScreen{Lower: o.Lower}
	rom := s.Charset()
	chars := petsciiAllowedChars(o)
	// set pixels of every character as bit indices 0..63
	setBits := make([][]uint8, 256)
	for _, ch := range chars {
		for y := 0; y < 8; y++ {
			b := rom[ch*8+y]
			for x := 0; x < 8; x++ {
				if b&(0x80>>x) != 0 {
					setBits[ch] = append(setBits[ch], uint8(y*8+x))
				}
			}
		}
	}
	bgs := []int{}
	if o.BG >= 0 && o.BG < 16 {
		bgs = []int{o.BG}
	} else {
		// the three most frequent colors are tried as background
		for n := 0; n < 3; n++ {
			best := -1
			for k := 0; k < 16; k++ {
				if !contains(bgs, k) && (best < 0 || hist[k] > hist[best]) {
					best = k
				}
			}
			bgs = append(bgs, best)
		}
	}
	bestTotal := int64(1) << 62
	for _, bg := range bgs {
		var chs, cols [PetCells]uint8
		total := int64(0)
		var cell [64][16]int32
		for ci := 0; ci < PetCells; ci++ {
			cx, cy := (ci%PetCols)*8, (ci/PetCols)*8
			base := int64(0)
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					d := dist[(cy+y)*320+cx+x]
					cell[y*8+x] = d
					base += int64(d[bg])
				}
			}
			bestE, bestCh, bestCol := int64(1)<<62, 32, 14
			for _, fg := range allowedFG {
				var diff [64]int32
				for i := 0; i < 64; i++ {
					diff[i] = cell[i][fg] - cell[i][bg]
				}
				for _, ch := range chars {
					e := base
					for _, b := range setBits[ch] {
						e += int64(diff[b])
					}
					if e < bestE {
						bestE, bestCh, bestCol = e, ch, fg
					}
				}
			}
			chs[ci], cols[ci] = uint8(bestCh), uint8(bestCol)
			total += bestE
		}
		if total < bestTotal {
			bestTotal = total
			s.Chars, s.Colors, s.BG = chs, cols, uint8(bg)
		}
	}
	return s
}

// PNG returns the screen as 320 × 200 (or 384 × 272 with border) image.
func (s *PetsciiScreen) PNG(pal *Palette, border bool) ([]byte, error) {
	w, h, ox, oy := 320, 200, 0, 0
	if border {
		w, h, ox, oy = 384, 272, 32, 35
	}
	img := image.NewPaletted(image.Rect(0, 0, w, h), nil)
	for _, c := range pal.Colors {
		img.Palette = append(img.Palette, color.RGBA{uint8(c >> 16), uint8(c >> 8), uint8(c), 255})
	}
	for i := range img.Pix {
		img.Pix[i] = s.Border
	}
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			img.Pix[(y+oy)*w+x+ox] = s.ColorAt(x, y)
		}
	}
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	return buf.Bytes(), err
}

// --- PRG and ACME assembler export -------------------------------------------

// petsciiViewer: the 6502 viewer that is put in front of the data. It sets
// the colors and character set, copies screen and colors and waits for a
// key, then resets the C64. Assembled for $080D; the data follows at
// petViewerEnd.
func petsciiViewer(s *PetsciiScreen, dataAddr int) []byte {
	charset := byte(0x15)
	if s.Lower {
		charset = 0x17
	}
	scr, col := dataAddr, dataAddr+PetCells
	lo := func(a int) byte { return byte(a) }
	hi := func(a int) byte { return byte(a >> 8) }
	code := []byte{
		0xA9, s.Border & 15, 0x8D, 0x20, 0xD0, // lda #border : sta $d020
		0xA9, s.BG & 15, 0x8D, 0x21, 0xD0, // lda #bg : sta $d021
		0xA9, charset, 0x8D, 0x18, 0xD0, // lda #charset : sta $d018
		0xA9, 0x80, 0x8D, 0x91, 0x02, // lda #$80 : sta $0291 (no case switching)
		0xA2, 0x00, // ldx #0
	}
	for _, off := range []int{0, 0x100, 0x200, 0x2E8} {
		code = append(code, 0xBD, lo(scr+off), hi(scr+off), 0x9D, lo(0x0400+off), hi(0x0400+off))
		code = append(code, 0xBD, lo(col+off), hi(col+off), 0x9D, lo(0xD800+off), hi(0xD800+off))
	}
	code = append(code,
		0xE8,       // inx
		0xD0, 0x00, // bne loop (patched below)
		0x20, 0xE4, 0xFF, // wait: jsr $ffe4
		0xF0, 0xFB, // beq wait
		0x4C, 0xE2, 0xFC, // jmp $fce2 (reset)
	)
	// branch back to the copy loop start (after ldx #0)
	loopStart := 22
	bne := len(code) - 10
	code[bne+1] = byte(loopStart - (bne + 2))
	return code
}

var basicStub = []byte{0x0B, 0x08, 0x0A, 0x00, 0x9E, 0x32, 0x30, 0x36, 0x31, 0x00, 0x00, 0x00} // 10 SYS2061

// PRG returns a runnable C64 program showing the screen.
func (s *PetsciiScreen) PRG() []byte {
	codeLen := len(petsciiViewer(s, 0))
	dataAddr := 0x080D + codeLen
	out := []byte{0x01, 0x08}
	out = append(out, basicStub...)
	out = append(out, petsciiViewer(s, dataAddr)...)
	out = append(out, s.Chars[:]...)
	for _, c := range s.Colors {
		out = append(out, c&15)
	}
	return out
}

// ACME returns assembler source for the ACME cross assembler.
func (s *PetsciiScreen) ACME(name string) string {
	var b strings.Builder
	charset := "$15 ; upper case / graphics"
	if s.Lower {
		charset = "$17 ; lower case / upper case"
	}
	fmt.Fprintf(&b, `; PETSCII picture %q – made with C64 DPainter
; assemble: acme --cpu 6510 --format cbm --outfile %s.prg %s.asm

!cpu 6510
* = $0801
        !byte $0b,$08,$0a,$00,$9e,$32,$30,$36,$31,$00,$00,$00 ; 10 SYS2061

* = $080d
start   lda #%d          ; border color
        sta $d020
        lda #%d          ; background color
        sta $d021
        lda #%s
        sta $d018
        lda #$80         ; no case switching with Shift+C=
        sta $0291
        ldx #0
copy    lda screen,x
        sta $0400,x
        lda colors,x
        sta $d800,x
        lda screen+$100,x
        sta $0500,x
        lda colors+$100,x
        sta $d900,x
        lda screen+$200,x
        sta $0600,x
        lda colors+$200,x
        sta $da00,x
        lda screen+$2e8,x
        sta $06e8,x
        lda colors+$2e8,x
        sta $dae8,x
        inx
        bne copy
wait    jsr $ffe4        ; wait for a key
        beq wait
        jmp $fce2        ; reset

`, name, name, name, s.Border&15, s.BG&15, charset)
	dump := func(label string, data []uint8) {
		fmt.Fprintf(&b, "%s\n", label)
		for row := 0; row < PetRows; row++ {
			b.WriteString("        !byte ")
			for c := 0; c < PetCols; c++ {
				if c > 0 {
					b.WriteString(",")
				}
				fmt.Fprintf(&b, "$%02x", data[row*PetCols+c])
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	dump("screen  ; 1000 screen codes", s.Chars[:])
	cols := make([]uint8, PetCells)
	for i, c := range s.Colors {
		cols[i] = c & 15
	}
	dump("colors  ; 1000 colors", cols)
	return b.String()
}

// --- Petmate workspace (.petmate) ---------------------------------------------

type petmatePixel struct {
	Code  int `json:"code"`
	Color int `json:"color"`
}

type petmateFramebuf struct {
	Width           int              `json:"width"`
	Height          int              `json:"height"`
	BackgroundColor int              `json:"backgroundColor"`
	BorderColor     int              `json:"borderColor"`
	Charset         string           `json:"charset"`
	Name            string           `json:"name,omitempty"`
	Framebuf        [][]petmatePixel `json:"framebuf"`
}

type petmateFile struct {
	Version     int               `json:"version"`
	Screens     []int             `json:"screens"`
	Framebufs   []petmateFramebuf `json:"framebufs"`
	CustomFonts map[string]any    `json:"customFonts"`
}

// Petmate returns the screen as Petmate workspace file (JSON).
func (s *PetsciiScreen) Petmate(name string) ([]byte, error) {
	fb := petmateFramebuf{Width: PetCols, Height: PetRows, BackgroundColor: int(s.BG), BorderColor: int(s.Border), Charset: "upper", Name: name}
	if s.Lower {
		fb.Charset = "lower"
	}
	for r := 0; r < PetRows; r++ {
		row := make([]petmatePixel, PetCols)
		for c := range row {
			row[c] = petmatePixel{int(s.Chars[r*PetCols+c]), int(s.Colors[r*PetCols+c])}
		}
		fb.Framebuf = append(fb.Framebuf, row)
	}
	return json.Marshal(petmateFile{Version: 2, Screens: []int{0}, Framebufs: []petmateFramebuf{fb}, CustomFonts: map[string]any{}})
}

// LoadPetmate reads the first screen of a Petmate workspace file. Screens of
// another size are cut or filled with spaces.
func LoadPetmate(data []byte) (*PetsciiScreen, error) {
	var f petmateFile
	if err := json.Unmarshal(data, &f); err != nil || len(f.Framebufs) == 0 {
		return nil, errors.New("not a Petmate file")
	}
	fb := f.Framebufs[0]
	s := NewPetscii(uint8(fb.BackgroundColor), uint8(fb.BorderColor))
	s.Lower = fb.Charset == "lower"
	for r := 0; r < PetRows && r < len(fb.Framebuf); r++ {
		for c := 0; c < PetCols && c < len(fb.Framebuf[r]); c++ {
			p := fb.Framebuf[r][c]
			s.Chars[r*PetCols+c] = uint8(p.Code)
			s.Colors[r*PetCols+c] = uint8(p.Color) & 15
		}
	}
	return s, nil
}
