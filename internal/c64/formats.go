package c64

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// Koala-Layout (ohne Ladeadresse):
//
//	$0000–$1F3F Bitmap (8000 Byte)
//	$1F40–$2327 Bildschirmspeicher (1000 Byte)
//	$2328–$270F Farbspeicher (1000 Byte)
//	$2710       Hintergrundfarbe
const klaSize = 10001

// KoalaLoadAddr ist die übliche Ladeadresse von Koala-Dateien.
const KoalaLoadAddr = 0x6000

// LoadKLA liest eine Koala-Painter-Datei (.kla/.koa).
func LoadKLA(data []byte) (*Picture, error) {
	var off int
	switch {
	case len(data) == klaSize:
		off = 0
	case len(data) >= klaSize+2:
		off = 2
	default:
		return nil, fmt.Errorf("not a Koala file: %d bytes (expected 10003)", len(data))
	}
	d := data[off:]
	p := &Picture{}
	p.SetBitmap(d[0:8000])
	copy(p.Screen[:], d[8000:9000])
	for i := 0; i < Cells; i++ {
		p.ColorRAM[i] = d[9000+i] & 15
	}
	p.BG = d[10000] & 15
	return p, nil
}

// KLABytes liefert die Koala-Daten ohne Ladeadresse (10001 Byte).
func (p *Picture) KLABytes() []byte {
	out := make([]byte, 0, klaSize)
	out = append(out, p.Bitmap()...)
	out = append(out, p.Screen[:]...)
	for i := 0; i < Cells; i++ {
		out = append(out, p.ColorRAM[i]&15)
	}
	out = append(out, p.BG&15)
	return out
}

// SaveKLA liefert eine Koala-Datei mit Ladeadresse $6000 (10003 Byte).
func (p *Picture) SaveKLA() []byte {
	return append([]byte{KoalaLoadAddr & 0xFF, KoalaLoadAddr >> 8}, p.KLABytes()...)
}

// --- PRG ---------------------------------------------------------------

type asm struct {
	org    int
	b      []byte
	labels map[string]int
	fix    []struct {
		pos   int
		label string
	}
}

func (a *asm) pc() int             { return a.org + len(a.b) }
func (a *asm) emit(bs ...byte)     { a.b = append(a.b, bs...) }
func (a *asm) label(n string)      { a.labels[n] = a.pc() }
func (a *asm) abs(op byte, v int)  { a.emit(op, byte(v), byte(v>>8)) }
func (a *asm) imm(op byte, v byte) { a.emit(op, v) }
func (a *asm) bne(l string) {
	a.emit(0xD0, 0)
	a.fix = append(a.fix, struct {
		pos   int
		label string
	}{len(a.b) - 1, l})
}
func (a *asm) resolve() error {
	for _, f := range a.fix {
		target := a.labels[f.label]
		rel := target - (a.org + f.pos + 1)
		if rel < -128 || rel > 127 {
			return errors.New("branch out of range")
		}
		a.b[f.pos] = byte(int8(rel))
	}
	return nil
}

// PRG erzeugt ein startfähiges C64-Programm (LOAD"BILD",8,1 bzw. ,8 + RUN),
// das das Bild im Multicolor-Bitmap-Modus anzeigt. Leertaste = Reset.
func (p *Picture) PRG() []byte {
	const (
		bitmap = 0x2000
		scrSrc = bitmap + 8000 // $3F40
		colSrc = scrSrc + 1000 // $4328
	)
	a := &asm{org: 0x0801, labels: map[string]int{}}
	// BASIC-Zeile: 2026 SYS2061
	a.emit(0x0B, 0x08, 0xEA, 0x07, 0x9E, '2', '0', '6', '1', 0x00, 0x00, 0x00)
	// $080D
	a.emit(0x78)             // SEI (keine Cursor-IRQs, die in $0400 schreiben)
	a.imm(0xA9, p.Border&15) // LDA #rahmen
	a.abs(0x8D, 0xD020)      // STA $D020
	a.imm(0xA9, p.BG&15)     // LDA #hintergrund
	a.abs(0x8D, 0xD021)      // STA $D021
	a.imm(0xA2, 0)           // LDX #0
	a.label("copy")
	for pg := 0; pg < 4; pg++ {
		a.abs(0xBD, scrSrc+pg*256) // LDA scr,X
		a.abs(0x9D, 0x0400+pg*256) // STA $0400,X
		a.abs(0xBD, colSrc+pg*256) // LDA col,X
		a.abs(0x9D, 0xD800+pg*256) // STA $D800,X
	}
	a.emit(0xE8) // INX
	a.bne("copy")
	a.abs(0xAD, 0xDD00) // LDA $DD00
	a.imm(0x29, 0xFC)   // AND #%11111100
	a.imm(0x09, 0x03)   // ORA #%00000011 → VIC-Bank 0 ($0000–$3FFF)
	a.abs(0x8D, 0xDD00) // STA $DD00
	a.imm(0xA9, 0x3B)   // LDA #%00111011: Bitmap an, Bildschirm an, 25 Zeilen
	a.abs(0x8D, 0xD011) // STA $D011
	a.imm(0xA9, 0x18)   // LDA #%00011000: Multicolor an, 40 Spalten
	a.abs(0x8D, 0xD016) // STA $D016
	a.imm(0xA9, 0x18)   // LDA #$18: Screen $0400, Bitmap $2000
	a.abs(0x8D, 0xD018) // STA $D018
	a.label("wait")
	a.imm(0xA9, 0x7F)   // LDA #$7F: Tastaturzeile 7
	a.abs(0x8D, 0xDC00) // STA $DC00
	a.abs(0xAD, 0xDC01) // LDA $DC01
	a.imm(0x29, 0x10)   // AND #$10: Leertaste
	a.bne("wait")
	a.abs(0x4C, 0xFCE2) // JMP $FCE2 (Reset)
	if err := a.resolve(); err != nil {
		panic(err)
	}
	mem := make([]byte, bitmap-0x0801)
	copy(mem, a.b)
	out := []byte{0x01, 0x08}
	out = append(out, mem...)
	out = append(out, p.KLABytes()...)
	return out
}

// --- PNG ---------------------------------------------------------------

// PNGMode bestimmt die Ausgabegröße beim PNG-Export.
type PNGMode int

const (
	PNG320    PNGMode = iota // 320×200, Bildpunkte doppelt breit (wie am Bildschirm)
	PNG160                   // 160×200, ein Bildpunkt pro Bitpaar
	PNGBorder                // 384×272 mit Rahmen (PAL-Sichtbereich)
)

// ToImage erzeugt ein indiziertes Bild mit der aktuellen Palette.
func (p *Picture) ToImage(pal *Palette, mode PNGMode) *image.Paletted {
	cp := make(color.Palette, 16)
	for i, c := range pal.Colors {
		cp[i] = color.RGBA{uint8(c >> 16), uint8(c >> 8), uint8(c), 255}
	}
	var w, h, ox, oy, sx int
	switch mode {
	case PNG160:
		w, h, sx = 160, 200, 1
	case PNGBorder:
		w, h, ox, oy, sx = 384, 272, 32, 35, 2
	default:
		w, h, sx = 320, 200, 2
	}
	im := image.NewPaletted(image.Rect(0, 0, w, h), cp)
	for i := range im.Pix {
		im.Pix[i] = p.Border & 15
	}
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			c := p.ColorAt(x, y)
			for k := 0; k < sx; k++ {
				im.Pix[(y+oy)*w+ox+x*sx+k] = c
			}
		}
	}
	return im
}

// SavePNG kodiert das Bild als PNG.
func (p *Picture) SavePNG(pal *Palette, mode PNGMode) ([]byte, error) {
	var buf bytes.Buffer
	err := png.Encode(&buf, p.ToImage(pal, mode))
	return buf.Bytes(), err
}

// Advanced Art Studio (.ocp, multicolor), 10018 bytes, load address $2000:
//
//	$0002 bitmap (8000), $1F42 screen RAM (1000), $232A border,
//	$232B background, $233A color RAM (1000)
//
// Offsets as documented by RECOIL (Piotr Fusik).
const ocpSize = 10018

// LoadOCP reads an Advanced Art Studio multicolor picture.
func LoadOCP(data []byte) (*Picture, error) {
	if len(data) != ocpSize {
		return nil, fmt.Errorf("not an Advanced Art Studio file: %d bytes (expected %d)", len(data), ocpSize)
	}
	p := &Picture{}
	p.SetBitmap(data[2:8002])
	copy(p.Screen[:], data[0x1f42:0x1f42+1000])
	for i := 0; i < Cells; i++ {
		p.ColorRAM[i] = data[0x233a+i] & 15
	}
	p.Border = data[0x232a] & 15
	p.BG = data[0x232b] & 15
	return p, nil
}

// Amica Paint (.ami): load address, then the Koala layout (10001 bytes)
// packed with a simple RLE: $C2 count value = value repeated count times,
// any other byte is a literal. (Format as documented by RECOIL.)
const amiEscape = 0xc2

// LoadAMI reads an Amica Paint picture.
func LoadAMI(data []byte) (*Picture, error) {
	if len(data) < 3 {
		return nil, errors.New("not an Amica Paint file")
	}
	out := make([]byte, 0, klaSize)
	for i := 2; i < len(data) && len(out) < klaSize; {
		b := data[i]
		if b != amiEscape {
			out = append(out, b)
			i++
			continue
		}
		if i+2 >= len(data) {
			break
		}
		n, v := int(data[i+1]), data[i+2]
		for k := 0; k < n && len(out) < klaSize; k++ {
			out = append(out, v)
		}
		i += 3
	}
	if len(out) < klaSize {
		return nil, fmt.Errorf("not an Amica Paint file (only %d of %d bytes unpacked)", len(out), klaSize)
	}
	return LoadKLA(out)
}

// SaveAMI packs a picture as Amica Paint file (used by the tests).
func (p *Picture) SaveAMI() []byte {
	raw := p.KLABytes()
	out := []byte{0x00, 0x40}
	for i := 0; i < len(raw); {
		j := i
		for j < len(raw) && raw[j] == raw[i] && j-i < 255 {
			j++
		}
		n := j - i
		if n > 3 || raw[i] == amiEscape {
			out = append(out, amiEscape, byte(n), raw[i])
		} else {
			out = append(out, raw[i:j]...)
		}
		i = j
	}
	return out
}
