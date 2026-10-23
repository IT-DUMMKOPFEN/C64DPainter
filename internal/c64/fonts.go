package c64

import (
	_ "embed"
	"strings"
	"unicode"
)

// The C64 character ROM (upper case/graphics and lower case), 256 × 8 bytes
// each. See charset/NOTICE.txt.
//
//go:embed charset/upper.bin
var CharsetUpper []byte

//go:embed charset/lower.bin
var CharsetLower []byte

// ScreenCode converts a character into a C64 screen code of the upper case
// (lower == false) or the lower case character set. Unknown characters
// become a space.
func ScreenCode(r rune, lower bool) uint8 {
	switch {
	case r == '@':
		return 0
	case r >= 'A' && r <= 'Z':
		if lower {
			return uint8(r-'A') + 65
		}
		return uint8(r-'A') + 1
	case r >= 'a' && r <= 'z':
		return uint8(r-'a') + 1
	case r >= ' ' && r <= '?':
		return uint8(r)
	case r == '[':
		return 27
	case r == '£':
		return 28
	case r == ']':
		return 29
	case r == '↑' || r == '^':
		return 30
	case r == '←' || r == '_':
		return 31
	}
	return 32
}

// narrowFont: an own font of 3 × 7 pixels for one multicolor cell (4 × 8 with
// spacing). Upper case only; lower case letters use the same glyphs.
var narrowFont = map[rune][7]string{
	'A':  {".#.", "#.#", "#.#", "###", "#.#", "#.#", "#.#"},
	'B':  {"##.", "#.#", "#.#", "##.", "#.#", "#.#", "##."},
	'C':  {".##", "#..", "#..", "#..", "#..", "#..", ".##"},
	'D':  {"##.", "#.#", "#.#", "#.#", "#.#", "#.#", "##."},
	'E':  {"###", "#..", "#..", "##.", "#..", "#..", "###"},
	'F':  {"###", "#..", "#..", "##.", "#..", "#..", "#.."},
	'G':  {".##", "#..", "#..", "#.#", "#.#", "#.#", ".##"},
	'H':  {"#.#", "#.#", "#.#", "###", "#.#", "#.#", "#.#"},
	'I':  {"###", ".#.", ".#.", ".#.", ".#.", ".#.", "###"},
	'J':  {"..#", "..#", "..#", "..#", "..#", "#.#", ".#."},
	'K':  {"#.#", "#.#", "##.", "#..", "##.", "#.#", "#.#"},
	'L':  {"#..", "#..", "#..", "#..", "#..", "#..", "###"},
	'M':  {"#.#", "###", "###", "#.#", "#.#", "#.#", "#.#"},
	'N':  {"#.#", "#.#", "###", "###", "###", "#.#", "#.#"},
	'O':  {".#.", "#.#", "#.#", "#.#", "#.#", "#.#", ".#."},
	'P':  {"##.", "#.#", "#.#", "##.", "#..", "#..", "#.."},
	'Q':  {".#.", "#.#", "#.#", "#.#", "#.#", "##.", ".##"},
	'R':  {"##.", "#.#", "#.#", "##.", "#.#", "#.#", "#.#"},
	'S':  {".##", "#..", "#..", ".#.", "..#", "..#", "##."},
	'T':  {"###", ".#.", ".#.", ".#.", ".#.", ".#.", ".#."},
	'U':  {"#.#", "#.#", "#.#", "#.#", "#.#", "#.#", "###"},
	'V':  {"#.#", "#.#", "#.#", "#.#", "#.#", ".#.", ".#."},
	'W':  {"#.#", "#.#", "#.#", "#.#", "###", "###", "#.#"},
	'X':  {"#.#", "#.#", ".#.", ".#.", ".#.", "#.#", "#.#"},
	'Y':  {"#.#", "#.#", "#.#", ".#.", ".#.", ".#.", ".#."},
	'Z':  {"###", "..#", "..#", ".#.", "#..", "#..", "###"},
	'0':  {".#.", "#.#", "#.#", "#.#", "#.#", "#.#", ".#."},
	'1':  {".#.", "##.", ".#.", ".#.", ".#.", ".#.", "###"},
	'2':  {"##.", "..#", "..#", ".#.", "#..", "#..", "###"},
	'3':  {"##.", "..#", "..#", ".#.", "..#", "..#", "##."},
	'4':  {"#.#", "#.#", "#.#", "###", "..#", "..#", "..#"},
	'5':  {"###", "#..", "#..", "##.", "..#", "..#", "##."},
	'6':  {".##", "#..", "#..", "##.", "#.#", "#.#", ".#."},
	'7':  {"###", "..#", "..#", ".#.", ".#.", ".#.", ".#."},
	'8':  {".#.", "#.#", "#.#", ".#.", "#.#", "#.#", ".#."},
	'9':  {".#.", "#.#", "#.#", ".##", "..#", "..#", "##."},
	' ':  {"...", "...", "...", "...", "...", "...", "..."},
	'.':  {"...", "...", "...", "...", "...", "...", ".#."},
	',':  {"...", "...", "...", "...", "...", ".#.", "#.."},
	':':  {"...", "...", ".#.", "...", "...", ".#.", "..."},
	';':  {"...", "...", ".#.", "...", "...", ".#.", "#.."},
	'!':  {".#.", ".#.", ".#.", ".#.", ".#.", "...", ".#."},
	'?':  {"##.", "..#", "..#", ".#.", ".#.", "...", ".#."},
	'-':  {"...", "...", "...", "###", "...", "...", "..."},
	'+':  {"...", "...", ".#.", "###", ".#.", "...", "..."},
	'=':  {"...", "...", "###", "...", "###", "...", "..."},
	'/':  {"..#", "..#", ".#.", ".#.", ".#.", "#..", "#.."},
	'\'': {".#.", ".#.", "...", "...", "...", "...", "..."},
	'"':  {"#.#", "#.#", "...", "...", "...", "...", "..."},
	'(':  {"..#", ".#.", "#..", "#..", "#..", ".#.", "..#"},
	')':  {"#..", ".#.", "..#", "..#", "..#", ".#.", "#.."},
	'*':  {"...", "#.#", ".#.", "###", ".#.", "#.#", "..."},
	'#':  {"#.#", "#.#", "###", "#.#", "###", "#.#", "#.#"},
	'&':  {".#.", "#.#", "#.#", ".#.", "#.#", "#.#", ".##"},
	'%':  {"#.#", "..#", ".#.", ".#.", ".#.", "#..", "#.#"},
	'<':  {"...", "..#", ".#.", "#..", ".#.", "..#", "..."},
	'>':  {"...", "#..", ".#.", "..#", ".#.", "#..", "..."},
	'_':  {"...", "...", "...", "...", "...", "...", "###"},
	'@':  {".#.", "#.#", "###", "###", "#..", "#..", ".##"},
	'$':  {".#.", ".##", "#..", ".#.", "..#", "##.", ".#."},
	'[':  {"##.", "#..", "#..", "#..", "#..", "#..", "##."},
	']':  {".##", "..#", "..#", "..#", "..#", "..#", ".##"},
}

// TextFont selects a font of the text tool.
type TextFont int

const (
	FontWide   TextFont = iota // C64 characters, every pixel a multicolor pixel (16 × 8 screen pixels)
	FontTall                   // the same, twice as high (16 × 16)
	FontNarrow                 // own 3 × 7 font in one cell (8 × 8 screen pixels)
	textFontCount
)

// TextFontNames are the display names of the text fonts.
var TextFontNames = [textFontCount]string{"C64 wide (8×8)", "C64 tall (8×16)", "Narrow (4×8)"}

// RenderText draws a text in multicolor pixels: the result is a mask of
// w × h pixels (true = set). Lines are separated by "\n".
func RenderText(text string, f TextFont, lower bool, spacing, lineGap int) (mask []bool, w, h int) {
	lines := strings.Split(text, "\n")
	cw, ch := 8, 8
	switch f {
	case FontTall:
		ch = 16
	case FontNarrow:
		cw = 4
	}
	cw += spacing
	ch += lineGap
	maxLen := 0
	for _, l := range lines {
		if n := len([]rune(l)); n > maxLen {
			maxLen = n
		}
	}
	if maxLen == 0 {
		return nil, 0, 0
	}
	w, h = maxLen*cw-spacing, len(lines)*ch-lineGap
	if f == FontNarrow {
		w-- // no empty column after the last glyph
	}
	mask = make([]bool, w*h)
	set := func(x, y int) {
		if x >= 0 && y >= 0 && x < w && y < h {
			mask[y*w+x] = true
		}
	}
	rom := CharsetUpper
	if lower {
		rom = CharsetLower
	}
	for li, l := range lines {
		for ci, r := range []rune(l) {
			ox, oy := ci*cw, li*ch
			if f == FontNarrow {
				g, ok := narrowFont[unicode.ToUpper(r)]
				if !ok {
					g = narrowFont['?']
				}
				for y := 0; y < 7; y++ {
					for x := 0; x < 3; x++ {
						if g[y][x] == '#' {
							set(ox+x, oy+y)
						}
					}
				}
				continue
			}
			code := int(ScreenCode(r, lower))
			for y := 0; y < 8; y++ {
				b := rom[code*8+y]
				for x := 0; x < 8; x++ {
					if b&(0x80>>x) != 0 {
						if f == FontTall {
							set(ox+x, oy+y*2)
							set(ox+x, oy+y*2+1)
						} else {
							set(ox+x, oy+y)
						}
					}
				}
			}
		}
	}
	return mask, w, h
}
