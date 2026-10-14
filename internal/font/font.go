// Package font lädt vorgerenderte Alpha-Glyphen (DejaVu-Schriften, siehe tools/genfont.py).
package font

import (
	_ "embed"
	"encoding/binary"
)

//go:embed ui.bin
var uiBin []byte

//go:embed uib.bin
var uibBin []byte

//go:embed body.bin
var bodyBin []byte

//go:embed bodyb.bin
var bodybBin []byte

//go:embed h1.bin
var h1Bin []byte

//go:embed h2.bin
var h2Bin []byte

//go:embed mono.bin
var monoBin []byte

//go:embed monob.bin
var monobBin []byte

// Glyph ist ein einzelnes Zeichen als Alpha-Maske.
type Glyph struct {
	Adv    int
	BX, BY int // Versatz der Maske relativ zum Ursprung auf der Grundlinie
	W, H   int
	A      []byte
}

// Face ist eine Schrift in fester Größe.
type Face struct {
	Ascent, Descent, Height int
	glyphs                  map[rune]*Glyph
	ascii                   [128]*Glyph
}

func load(b []byte) *Face {
	f := &Face{glyphs: map[rune]*Glyph{}}
	f.Ascent = int(binary.LittleEndian.Uint16(b[4:]))
	f.Descent = int(binary.LittleEndian.Uint16(b[6:]))
	f.Height = int(binary.LittleEndian.Uint16(b[8:]))
	n := int(binary.LittleEndian.Uint16(b[10:]))
	p := 12
	for i := 0; i < n; i++ {
		c := rune(binary.LittleEndian.Uint32(b[p:]))
		g := &Glyph{
			Adv: int(int16(binary.LittleEndian.Uint16(b[p+4:]))),
			BX:  int(int16(binary.LittleEndian.Uint16(b[p+6:]))),
			BY:  int(int16(binary.LittleEndian.Uint16(b[p+8:]))),
			W:   int(b[p+10]),
			H:   int(b[p+11]),
		}
		p += 12
		sz := g.W * g.H
		g.A = b[p : p+sz]
		p += sz
		f.glyphs[c] = g
		if c < 128 {
			f.ascii[c] = g
		}
	}
	return f
}

// Glyph liefert das Zeichen oder ein Ersatzzeichen.
func (f *Face) Glyph(r rune) *Glyph {
	if r >= 0 && r < 128 {
		if g := f.ascii[r]; g != nil {
			return g
		}
	} else if g, ok := f.glyphs[r]; ok {
		return g
	}
	return f.ascii['?']
}

// Width misst einen Text.
func (f *Face) Width(s string) int {
	w := 0
	for _, r := range s {
		w += f.Glyph(r).Adv
	}
	return w
}

var (
	UI    = load(uiBin)
	UIB   = load(uibBin)
	Body  = load(bodyBin)
	BodyB = load(bodybBin)
	H1    = load(h1Bin)
	H2    = load(h2Bin)
	Mono  = load(monoBin)
	MonoB = load(monobBin)
)
