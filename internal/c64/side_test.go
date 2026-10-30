package c64

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func sidePicture() *Picture {
	p := NewPicture(0, 14)
	p.ToLayers()
	p.SideOn = true
	return p
}

func TestSidePlotRules(t *testing.T) {
	p := sidePicture()
	pal := &Palettes[0]
	// first color of a block becomes its sprite color
	if r := p.PlotSide(0, 3, 30, 2, ConflictBlock, pal); r != PlotOK || p.Side.Col[0][1] != 2 {
		t.Fatalf("own color: %v %d", r, p.Side.Col[0][1])
	}
	// next colors take the shared colors
	p.Side.MC1, p.Side.MC2 = 15, 15
	p.PlotSide(0, 4, 30, 5, ConflictBlock, pal)
	p.PlotSide(0, 5, 30, 7, ConflictBlock, pal)
	if p.Side.MC1 != 5 || p.Side.MC2 != 7 {
		t.Fatalf("shared colors %d %d", p.Side.MC1, p.Side.MC2)
	}
	// a fourth color is blocked, replaced or matched
	if r := p.PlotSide(0, 6, 30, 8, ConflictBlock, pal); r != PlotBlocked || p.SideColorAt(0, 6, 30) != p.BG {
		t.Fatal("block")
	}
	if r := p.PlotSide(0, 6, 30, 8, ConflictReplace, pal); r != PlotReplaced || p.Side.Col[0][1] != 8 || p.SideColorAt(0, 3, 30) != 8 {
		t.Fatal("replace changes the sprite color of the block")
	}
	p.PlotSide(0, 7, 30, 9, ConflictNearest, pal)
	if c := p.SideColorAt(0, 7, 30); c != 8 && c != 5 && c != 7 && c != p.BG {
		t.Fatalf("nearest gave %d", c)
	}
	// another block may use its own color again
	if r := p.PlotSide(0, 3, 50, 12, ConflictBlock, pal); r != PlotOK || p.Side.Col[0][2] != 12 {
		t.Fatal("blocks are independent")
	}
	// background = transparent, hidden first line of the left strip
	p.PlotSide(0, 3, 50, 0, ConflictBlock, pal)
	if p.Side.Pix[0][50*SideW+3] != 0 {
		t.Fatal("background must be transparent")
	}
	p.PlotSide(0, 3, 0, 2, ConflictBlock, pal)
	if p.Side.Pix[0][3] != 0 {
		t.Fatal("first line of the left strip is hidden")
	}
	// ColorAt reaches the strips through x -12..-1 and 160..171
	p.PlotSide(1, 0, 10, 3, ConflictBlock, pal)
	if p.ColorAt(W, 10) != 3 || p.ColorAt(-SideW+3, 30) != 8 {
		t.Fatal("ColorAt")
	}
	// sprite data: %10 = sprite color, %01 = MC1, %11 = MC2
	spr := p.SideSprite(0, 1)
	ly := 30 - SpriteH
	got := []byte{spr[ly*3], spr[ly*3+1]}
	// pixels 3..7 of the line: own(8) at 3, MC1 at 4, MC2 at 5, 8 at 6, nearest at 7
	if (got[0]&3) != 2 || (got[1]>>6) != 1 || (got[1]>>4)&3 != 3 {
		t.Fatalf("sprite bits %08b %08b", got[0], got[1])
	}
	// closing keeps the data, hires ignores it
	p.SideOn = false
	if p.ColorAt(W, 10) != p.BG || p.SidePixels() == 0 {
		t.Fatal("closed side border")
	}
}

// cycles of the instructions the viewer uses in its timed lines
func svCycles(t *testing.T, ins string) int {
	f := strings.Fields(ins)
	switch {
	case f[0] == "nop":
		return 2
	case f[0] == "bit":
		return 3
	case f[0] == "lda" && strings.HasPrefix(f[1], "#"):
		return 2
	case f[0] == "sta" && strings.HasSuffix(f[1], ",x"):
		return 5
	case f[0] == "sta":
		return 4
	case f[0] == "jsr" && strings.HasPrefix(f[1], "dly+"):
		k, _ := strconv.Atoi(strings.TrimPrefix(f[1], "dly+"))
		return 12 + 2*(24-k)
	}
	t.Fatalf("unexpected instruction in a timed line: %q", ins)
	return 0
}

func TestSideViewerTiming(t *testing.T) {
	p := sidePicture()
	pal := &Palettes[0]
	for s := 0; s < 2; s++ {
		for row := 0; row < SideRows; row++ {
			for y := row * SpriteH; y < min(H, row*SpriteH+SpriteH); y++ {
				p.PlotSide(s, (y+s)%SideW, y, uint8(row+1), ConflictReplace, pal)
			}
		}
	}
	src := p.SideBorderACME("test")
	prg, err := p.SideBorderPRG()
	if err != nil {
		t.Fatal(err)
	}
	if prg[0] != 0x01 || prg[1] != 0x08 {
		t.Fatal("load address")
	}
	// the sprites sit at $4800, 20 × 64 bytes
	off := 2 + svSprites - 0x0801
	if len(prg) != off+20*64 {
		t.Fatalf("length %d", len(prg))
	}
	want := p.SideSprite(1, 3)
	if !bytes.Equal(prg[off+13*64:off+13*64+63], want[:63]) {
		t.Fatal("sprite data of right row 3")
	}
	// every timed line: free cycles 48 (normal) or 5 (bad line), then sty/stx
	re := regexp.MustCompile(`^l(\d{3})`)
	lines := strings.Split(src, "\n")
	seen := 0
	for i := 0; i < len(lines); i++ {
		m := re.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		l, _ := strconv.Atoi(m[1])
		seen++
		cyc := 0
		j := i + 1
		for ; !strings.HasPrefix(strings.TrimSpace(lines[j]), "sty $d016"); j++ {
			ins := strings.TrimSpace(strings.SplitN(lines[j], ";", 2)[0])
			cyc += svCycles(t, ins)
		}
		if strings.TrimSpace(lines[j+1]) != "stx $d016" {
			t.Fatalf("line %d: stx missing", l)
		}
		want := 48
		if svBadline(l) {
			want = 5
		}
		if l == svFirstLine {
			want = 0
		}
		if cyc != want {
			t.Fatalf("line %d: %d free cycles, want %d", l, cyc, want)
		}
	}
	if seen != svLastLine-svFirstLine+1 {
		t.Fatalf("%d timed lines", seen)
	}
	// the multiplexer: 9 row changes with pointers, positions and colors
	for m := 0; m < SideRows-1; m++ {
		if !strings.Contains(src, fmt.Sprintf("y of sprite row %d", m+1)) ||
			!strings.Contains(src, fmt.Sprintf("color left row %d", m+1)) ||
			!strings.Contains(src, fmt.Sprintf("color right row %d", m+1)) {
			t.Fatalf("row %d not prepared", m+1)
		}
	}
}

func TestAssemblerBasics(t *testing.T) {
	prg, err := Assemble(`!cpu 6510
* = $1000
start   lda #$01
        sta $d020
        lda ($fb),y
        sta $fd
loop    dex
        bne loop
        jmp start
        !byte 1,2,$ff
        !word $1234
`)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x00, 0x10, 0xA9, 0x01, 0x8D, 0x20, 0xD0, 0xB1, 0xFB, 0x85, 0xFD, 0xCA, 0xD0, 0xFD, 0x4C, 0x00, 0x10, 1, 2, 0xFF, 0x34, 0x12}
	if !bytes.Equal(prg, want) {
		t.Fatalf("got % x", prg)
	}
	if _, err := Assemble("* = $1000\n        lda nowhere\n"); err == nil {
		t.Fatal("unknown label must fail")
	}
}
