package c64

import (
	"bytes"
	"errors"
	"fmt"
)

// --- More C64 picture formats (1.2) --------------------------------------------------
//
// The layouts follow RECOIL by Piotr Fusik (recoil.sourceforge.net); all
// offsets include the two bytes of the load address.

// Format describes a C64 picture file format that can be read and maybe saved.
type Format struct {
	Name  string
	Exts  []string // lower case, with dot; the first one is used for saving
	Hires bool
	Load  func(data []byte) (*Picture, error)
	Save  func(p *Picture, pal *Palette) []byte // nil: read only
	// Border is true when the file keeps the border color.
	Border bool
}

// layout of an uncompressed multicolor file
type mcLayout struct {
	name                   string
	size                   int
	load                   int
	bm, scr, col, bg, bord int // bord < 0: none
}

func (l mcLayout) decode(d []byte) (*Picture, error) {
	if len(d) < l.size {
		return nil, fmt.Errorf("not a %s file: %d bytes (expected %d)", l.name, len(d), l.size)
	}
	p := &Picture{}
	p.SetBitmap(d[l.bm : l.bm+8000])
	copy(p.Screen[:], d[l.scr:l.scr+1000])
	for i := 0; i < Cells; i++ {
		p.ColorRAM[i] = d[l.col+i] & 15
	}
	p.BG = d[l.bg] & 15
	if l.bord >= 0 {
		p.Border = d[l.bord] & 15
	}
	return p, nil
}

func (l mcLayout) encode(p *Picture) []byte {
	out := make([]byte, l.size)
	out[0], out[1] = byte(l.load), byte(l.load>>8)
	copy(out[l.bm:], p.Bitmap())
	copy(out[l.scr:], p.Screen[:])
	for i := 0; i < Cells; i++ {
		out[l.col+i] = p.ColorRAM[i] & 15
	}
	out[l.bg] = p.BG & 15
	if l.bord >= 0 {
		out[l.bord] = p.Border & 15
	}
	return out
}

func (l mcLayout) loader() func([]byte) (*Picture, error) {
	return func(d []byte) (*Picture, error) {
		if len(d) != l.size {
			return nil, fmt.Errorf("not a %s file: %d bytes (expected %d)", l.name, len(d), l.size)
		}
		return l.decode(d)
	}
}

func (l mcLayout) saver() func(*Picture, *Palette) []byte {
	return func(p *Picture, _ *Palette) []byte { return l.encode(p) }
}

// layout of an uncompressed hires file
type hiLayout struct {
	name          string
	size          int
	load, bm, scr int
}

func (l hiLayout) loader() func([]byte) (*Picture, error) {
	return func(d []byte) (*Picture, error) {
		if len(d) != l.size {
			return nil, fmt.Errorf("not a %s file: %d bytes (expected %d)", l.name, len(d), l.size)
		}
		p := &Picture{Mode: ModeHires}
		p.SetHiresData(d[l.bm:l.bm+8000], d[l.scr:l.scr+1000])
		return p, nil
	}
}

func (l hiLayout) saver() func(*Picture, *Palette) []byte {
	return func(p *Picture, pal *Palette) []byte {
		bm, scr := p.HiresData(pal)
		out := make([]byte, l.size)
		out[0], out[1] = byte(l.load), byte(l.load>>8)
		copy(out[l.bm:], bm)
		copy(out[l.scr:], scr)
		return out
	}
}

var (
	layOCP = mcLayout{"Advanced Art Studio", 10018, 0x2000, 2, 0x1f42, 0x233a, 0x232b, 0x232a}
	layDRZ = mcLayout{"Drazpaint", 10051, 0x5800, 0x802, 0x402, 2, 0x2742, -1}
	layVID = mcLayout{"Vidcom 64", 10050, 0x5800, 0x802, 0x402, 2, 0x7ea, -1}
	layISM = mcLayout{"Image System", 10218, 0x3c00, 0x402, 0x2402, 2, 0x2401, -1}
	layBPL = mcLayout{"Blazing Paddles", 10242, 0xa000, 2, 0x2002, 0x2402, 0x1f82, -1}
	laySAR = mcLayout{"Saracen Paint", 10219, 0x7800, 0x402, 2, 0x2402, 0x3f2, -1}
	layHED = hiLayout{"Hi-Eddi", 9218, 0x2000, 2, 0x2002}
	layISH = hiLayout{"Image System hires", 9194, 0x4000, 2, 0x2002}
	layIPH = hiLayout{"Interpaint hires", 9002, 0x4000, 2, 0x1f42}
)

// koalaLike loads files with the Koala layout behind a load address
// (Run Paint, Interpaint, Face Painter …).
func koalaLike(name string, sizes ...int) func([]byte) (*Picture, error) {
	return func(d []byte) (*Picture, error) {
		for _, s := range sizes {
			if len(d) == s {
				return LoadKLA(d[:klaSize+2])
			}
		}
		return nil, fmt.Errorf("not a %s file: %d bytes", name, len(d))
	}
}

// Formats lists the formats beyond Koala, Art Studio hires and Doodle.
var Formats = []Format{
	// multicolor, read and write
	{Name: "Advanced Art Studio", Exts: []string{".ocp", ".mpic"}, Load: layOCP.loader(), Save: layOCP.saver(), Border: true},
	{Name: "Amica Paint", Exts: []string{".ami"}, Load: LoadAMI, Save: func(p *Picture, _ *Palette) []byte { return p.SaveAMI() }},
	{Name: "Koala Painter compressed", Exts: []string{".gg"}, Load: LoadGG, Save: func(p *Picture, _ *Palette) []byte { return p.SaveGG() }},
	{Name: "Drazpaint", Exts: []string{".drz", ".drp"}, Load: LoadDRZ, Save: func(p *Picture, _ *Palette) []byte { return p.SaveDRZ() }},
	{Name: "Image System", Exts: []string{".ism"}, Load: layISM.loader(), Save: layISM.saver()},
	{Name: "Vidcom 64", Exts: []string{".vid"}, Load: layVID.loader(), Save: layVID.saver()},
	{Name: "Blazing Paddles", Exts: []string{".pi", ".bpl"}, Load: layBPL.loader(), Save: layBPL.saver()},
	// multicolor, read only
	{Name: "Saracen Paint", Exts: []string{".sar"}, Load: laySAR.loader()},
	{Name: "Run Paint", Exts: []string{".rpm"}, Load: koalaLike("Run Paint", 10006, 10003)},
	{Name: "Interpaint", Exts: []string{".ipt", ".lre"}, Load: koalaLike("Interpaint", 10003)},
	{Name: "Face Painter", Exts: []string{".fpt", ".fcp"}, Load: koalaLike("Face Painter", 10004)},
	// hires, read and write
	{Name: "Hi-Eddi", Exts: []string{".hed"}, Hires: true, Load: layHED.loader(), Save: layHED.saver()},
	{Name: "Doodle compressed", Exts: []string{".jj"}, Hires: true, Load: LoadJJ, Save: func(p *Picture, pal *Palette) []byte { return p.SaveJJ(pal) }},
	// hires, read only
	{Name: "Image System hires", Exts: []string{".ish"}, Hires: true, Load: layISH.loader()},
	{Name: "Interpaint hires", Exts: []string{".iph"}, Hires: true, Load: layIPH.loader()},
	{Name: "Run Paint hires", Exts: []string{".rph"}, Hires: true, Load: hiLayout{"Run Paint hires", 9026, 0x5c00, 0x402, 2}.loader()},
}

// FormatByExt finds a format by its file extension (".drz").
func FormatByExt(ext string) (int, *Format) {
	for i := range Formats {
		for _, e := range Formats[i].Exts {
			if e == ext {
				return i, &Formats[i]
			}
		}
	}
	return -1, nil
}

// --- Koala compressed (.gg) and Doodle compressed (.jj) ------------------------------
//
// RLE: $FE value count = value repeated count times, any other byte is a
// literal.

const ggEscape = 0xfe

func unpackFE(d []byte, n int) ([]byte, error) {
	out := make([]byte, 0, n)
	for i := 2; i < len(d) && len(out) < n; {
		b := d[i]
		if b != ggEscape {
			out = append(out, b)
			i++
			continue
		}
		if i+2 >= len(d) {
			break
		}
		v, c := d[i+1], int(d[i+2])
		for k := 0; k < c && len(out) < n; k++ {
			out = append(out, v)
		}
		i += 3
	}
	if len(out) < n {
		return nil, fmt.Errorf("packed data too short (%d of %d bytes)", len(out), n)
	}
	return out, nil
}

func packFE(load int, raw []byte) []byte {
	out := []byte{byte(load), byte(load >> 8)}
	for i := 0; i < len(raw); {
		j := i
		for j < len(raw) && raw[j] == raw[i] && j-i < 255 {
			j++
		}
		if n := j - i; n > 3 || raw[i] == ggEscape {
			out = append(out, ggEscape, raw[i], byte(n))
		} else {
			out = append(out, raw[i:j]...)
		}
		i = j
	}
	return out
}

// LoadGG reads a compressed Koala picture.
func LoadGG(d []byte) (*Picture, error) {
	raw, err := unpackFE(d, klaSize)
	if err != nil {
		return nil, errors.New("not a compressed Koala file: " + err.Error())
	}
	return LoadKLA(raw)
}

// SaveGG writes a compressed Koala picture (load address $6000).
func (p *Picture) SaveGG() []byte { return packFE(KoalaLoadAddr, p.KLABytes()) }

const jjSize = 0x400 + 8000

// LoadJJ reads a compressed Doodle picture.
func LoadJJ(d []byte) (*Picture, error) {
	raw, err := unpackFE(d, jjSize)
	if err != nil {
		return nil, errors.New("not a compressed Doodle file: " + err.Error())
	}
	p := &Picture{Mode: ModeHires}
	p.SetHiresData(raw[0x400:], raw[:1000])
	return p, nil
}

// SaveJJ writes a compressed Doodle picture (load address $5C00).
func (p *Picture) SaveJJ(pal *Palette) []byte {
	bm, scr := p.HiresData(pal)
	raw := make([]byte, jjSize)
	copy(raw, scr)
	copy(raw[0x400:], bm)
	return packFE(0x5c00, raw)
}

// --- Drazpaint (.drz) ----------------------------------------------------------------
//
// Uncompressed: 10051 bytes, load address $5800, color RAM, screen RAM,
// bitmap, background. Compressed: load address, "DRAZPAINT 1.4", the
// escape byte, then the data from offset 2 packed as escape count value.

const drzSig = "DRAZPAINT 1.4"

// LoadDRZ reads a Drazpaint picture (compressed or not).
func LoadDRZ(d []byte) (*Picture, error) {
	if len(d) > 16 && bytes.Equal(d[2:2+len(drzSig)], []byte(drzSig)) {
		esc := d[15]
		raw := make([]byte, 2, layDRZ.size)
		for i := 16; i < len(d) && len(raw) < layDRZ.size; {
			b := d[i]
			if b != esc {
				raw = append(raw, b)
				i++
				continue
			}
			if i+2 >= len(d) {
				break
			}
			c, v := int(d[i+1]), d[i+2]
			for k := 0; k < c && len(raw) < layDRZ.size; k++ {
				raw = append(raw, v)
			}
			i += 3
		}
		if len(raw) < layDRZ.size {
			return nil, errors.New("not a Drazpaint file (packed data too short)")
		}
		return layDRZ.decode(raw)
	}
	return layDRZ.loader()(d)
}

// SaveDRZ writes a compressed Drazpaint picture.
func (p *Picture) SaveDRZ() []byte {
	raw := layDRZ.encode(p)[2:]
	// the rarest byte becomes the escape byte
	var hist [256]int
	for _, b := range raw {
		hist[b]++
	}
	esc := 0
	for b := range hist {
		if hist[b] < hist[esc] {
			esc = b
		}
	}
	out := []byte{byte(layDRZ.load), byte(layDRZ.load >> 8)}
	out = append(out, drzSig...)
	out = append(out, byte(esc))
	for i := 0; i < len(raw); {
		j := i
		for j < len(raw) && raw[j] == raw[i] && j-i < 255 {
			j++
		}
		if n := j - i; n > 3 || int(raw[i]) == esc {
			out = append(out, byte(esc), byte(n), raw[i])
		} else {
			out = append(out, raw[i:j]...)
		}
		i = j
	}
	return out
}

// --- Raw data for programmers --------------------------------------------------------

// RawParts returns the bitmap (8000), the screen RAM (1000) and, for
// multicolor, the color RAM (1000) without load addresses, plus an ACME
// source that includes them.
func (p *Picture) RawParts(pal *Palette, name string) (bm, scr, col []byte, src string) {
	var b bytes.Buffer
	if p.Hires() {
		bm, scr = p.HiresData(pal)
	} else {
		bm = p.Bitmap()
		scr = append([]byte(nil), p.Screen[:]...)
		col = make([]byte, Cells)
		for i := range col {
			col[i] = p.ColorRAM[i] & 15
		}
	}
	mode := "multicolor"
	if p.Hires() {
		mode = "hires"
	}
	fmt.Fprintf(&b, "; %s – %s bitmap, saved by C64 DPainter\n", name, mode)
	b.WriteString("; Include this file in your ACME program (!source) or use the .bin files directly.\n\n")
	fmt.Fprintf(&b, "%s_border = $%02x\n", name, p.Border&15)
	if !p.Hires() {
		fmt.Fprintf(&b, "%s_background = $%02x  ; $d021\n", name, p.BG&15)
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "%s_bitmap  !binary \"%s_bitmap.bin\"   ; 8000 bytes, copy to a $2000 boundary\n", name, name)
	fmt.Fprintf(&b, "%s_screen  !binary \"%s_screen.bin\"   ; 1000 bytes, screen RAM\n", name, name)
	if col != nil {
		fmt.Fprintf(&b, "%s_color   !binary \"%s_color.bin\"    ; 1000 bytes, copy to $d800\n", name, name)
	}
	return bm, scr, col, b.String()
}
