package c64

import (
	"fmt"
	"strings"
)

// --- Side border viewer: ACME source and PRG (experimental) ------------------------------
//
// The program shows the multicolor picture with the side border opened and
// the two sprite strips in it (PAL only):
//
//   - VIC bank 1: screen A $4000, screen B $4400 (same colors, other sprite
//     pointers), sprites $4800, bitmap $6000.
//   - Sprite 2 shows the left strip (x = 0), sprite 3 the right strip
//     (x = 344). Both are reused 10 times (every 21 lines) by a multiplexer.
//   - A stable raster (double IRQ) starts a fully unrolled, cycle-exact
//     program for the raster lines 29 to 250. In every line "sty $d016"
//     switches to 38 columns in cycle 56 (after the 40-column check in cycle
//     56, before the 38-column check in cycle 57), "stx $d016" switches back.
//     On bad lines the CPU stops in the middle of "sty $d016" and finishes it
//     in cycles 55/56. The sprite DMA of sprites 2 and 3 stops the CPU from
//     cycle 59 to 2, so every line starts again in cycle 3.
//
// Cycle numbers follow the VIC article by Christian Bauer and VICE (x64sc).

// SideViewerTiming can shift the whole timed part (for tests on other
// emulators); 0 is right for a real PAL C64 and VICE.
var SideViewerTiming = 0

const (
	svFirstLine = 29  // the dummy sprite row starts here (hidden by the top border)
	svLastLine  = 250 // last picture line
	svIRQLine   = 20
	svScreenA   = 0x4000
	svScreenB   = 0x4400
	svSprites   = 0x4800
	svBitmap    = 0x6000
)

func svBadline(l int) bool { return l >= 0x30 && l <= 0xF7 && l&7 == 3 }

// svJob is one register write in the free cycles of a raster line.
type svJob struct {
	val      uint8
	addrs    []int
	minWrite int // earliest cycle of the first write (0 = any)
	maxWrite int // latest cycle of the last write (0 = any)
	comment  string
}

type svLine struct {
	jobs    []svJob
	preload int    // value for A at the end of the line (-1 = none)
	bad     *svJob // store on a bad line: sta addr-$18,x with A preloaded
}

// SideBorderACME returns the complete viewer as ACME source.
func (p *Picture) SideBorderACME(name string) string {
	return p.sideBorderSource(name)
}

// SideBorderPRG returns the runnable program (the same source, assembled).
func (p *Picture) SideBorderPRG() ([]byte, error) {
	return Assemble(p.sideBorderSource("picture"))
}

func (p *Picture) sideBorderSource(name string) string {
	s := &p.Side
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	ptrL := func(seg int) int { return (svSprites-0x4000)/64 + seg }
	ptrR := func(seg int) int { return (svSprites-0x4000)/64 + SideRows + seg }

	w("; %s – multicolor picture with sprites in the opened side border", name)
	w("; made with C64 DPainter (experimental, PAL only)")
	w(";")
	w("; assemble: acme --cpu 6510 --format cbm --outfile %s.prg %s.asm", name, name)
	w("; run: LOAD\"%s\",8 and RUN – Space resets the C64", strings.ToUpper(name))
	w(";")
	w("; memory: VIC bank 1 – screen A $4000, screen B $4400, sprites $4800,")
	w(";         bitmap $6000. Sprites 2 (left) and 3 (right) are reused every")
	w(";         21 lines. Lines %d to %d run cycle-exact code that opens the", svFirstLine, svLastLine)
	w(";         side border in every line (cycle numbers as in VICE x64sc).")
	w("")
	w("!cpu 6510")
	w("* = $0801")
	w("        !byte $0b,$08,$ea,$07,$9e,$32,$30,$36,$31,$00,$00,$00 ; 2026 SYS2061")
	w("")
	w("* = $080d")
	w("start   sei")
	w("        lda #$7f          ; no CIA interrupts")
	w("        sta $dc0d")
	w("        sta $dd0d")
	w("        lda $dc0d")
	w("        lda $dd0d")
	w("        lda #$35          ; RAM under the KERNAL: own IRQ vector at $fffe")
	w("        sta $01")
	// copies
	w("        ldx #0")
	w("copy    lda d_screen,x    ; screen RAM twice (A and B), color RAM")
	w("        sta $4000,x")
	w("        sta $4400,x")
	w("        lda d_screen+$100,x")
	w("        sta $4100,x")
	w("        sta $4500,x")
	w("        lda d_screen+$200,x")
	w("        sta $4200,x")
	w("        sta $4600,x")
	w("        lda d_screen+$2e8,x")
	w("        sta $42e8,x")
	w("        sta $46e8,x")
	w("        lda d_color,x")
	w("        sta $d800,x")
	w("        lda d_color+$100,x")
	w("        sta $d900,x")
	w("        lda d_color+$200,x")
	w("        sta $da00,x")
	w("        lda d_color+$2e8,x")
	w("        sta $dae8,x")
	w("        inx")
	w("        bne copy")
	w("        lda #<d_bitmap    ; bitmap to $6000 (32 pages)")
	w("        sta $fb")
	w("        lda #>d_bitmap")
	w("        sta $fc")
	w("        lda #$00")
	w("        sta $fd")
	w("        lda #$60")
	w("        sta $fe")
	w("        ldx #32")
	w("        ldy #0")
	w("copybm  lda ($fb),y")
	w("        sta ($fd),y")
	w("        iny")
	w("        bne copybm")
	w("        inc $fc")
	w("        inc $fe")
	w("        dex")
	w("        bne copybm")
	// VIC
	w("        lda $dd00         ; VIC bank 1 ($4000-$7fff)")
	w("        and #$fc")
	w("        ora #$02")
	w("        sta $dd00")
	w("        lda #$08          ; screen A $4000, bitmap $6000")
	w("        sta $d018")
	w("        lda #$3b          ; bitmap mode, 25 rows")
	w("        sta $d011")
	w("        lda #$18          ; multicolor, 40 columns")
	w("        sta $d016")
	w("        lda #%d           ; border", p.Border&15)
	w("        sta $d020")
	w("        lda #%d           ; background (also shown in the open border)", p.BG&15)
	w("        sta $d021")
	w("        lda #%%00001100    ; sprites 2 and 3, multicolor")
	w("        sta $d015")
	w("        sta $d01c")
	w("        lda #0")
	w("        sta $d017")
	w("        sta $d01d")
	w("        sta $d01b")
	w("        lda #%d           ; sprite multicolor 1", s.MC1&15)
	w("        sta $d025")
	w("        lda #%d           ; sprite multicolor 2", s.MC2&15)
	w("        sta $d026")
	w("        lda #0            ; sprite 2: x = 0 (left border)")
	w("        sta $d004")
	w("        lda #$58          ; sprite 3: x = 344 (right border)")
	w("        sta $d006")
	w("        lda #%%00001000")
	w("        sta $d010")
	w("        lda #<nmi")
	w("        sta $fffa")
	w("        lda #>nmi")
	w("        sta $fffb")
	w("        jsr frame         ; sprite pointers, positions and colors")
	w("        lda #<irq1")
	w("        sta $fffe")
	w("        lda #>irq1")
	w("        sta $ffff")
	w("        lda #%d", svIRQLine)
	w("        sta $d012")
	w("        lda #$01          ; raster interrupt")
	w("        sta $d01a")
	w("        sta $d019")
	w("        cli")
	w("main")
	for i := 0; i < sideViewerMainNops; i++ {
		w("        nop")
	}
	w("        jmp main")
	w("")
	w("nmi     rti")
	w("")
	w("; values for the start of every frame")
	w("frame   lda #$08          ; screen A")
	w("        sta $d018")
	w("        lda #$%02x         ; pointers for sprite row 0 (screen A)", ptrL(0))
	w("        sta $%04x", svScreenA+0x3FA)
	w("        lda #$%02x", ptrR(0))
	w("        sta $%04x", svScreenA+0x3FB)
	w("        lda #$%02x         ; pointers for sprite row 5 (screen B)", ptrL(5))
	w("        sta $%04x", svScreenB+0x3FA)
	w("        lda #$%02x", ptrR(5))
	w("        sta $%04x", svScreenB+0x3FB)
	w("        lda #%d           ; a hidden first row (lines 30-50) keeps the timing", svFirstLine)
	w("        sta $d005")
	w("        sta $d007")
	w("        lda #%d", s.Col[0][0]&15)
	w("        sta $d029")
	w("        lda #%d", s.Col[1][0]&15)
	w("        sta $d02a")
	w("        rts")
	w("")
	w("; --- stable raster: double interrupt ---")
	w("irq1    lda #<irq2")
	w("        sta $fffe")
	w("        lda #>irq2")
	w("        sta $ffff")
	w("        inc $d012")
	w("        lda #$01")
	w("        sta $d019")
	w("        tsx")
	w("        cli")
	for i := 0; i < 16; i++ {
		w("        nop")
	}
	w("irq2    txs               ; drop the second interrupt")
	w("        ldx #8")
	w("burn    dex")
	w("        bne burn")
	w("        bit $02")
	w("        lda $d012")
	w("        cmp $d012         ; did the line change between the reads?")
	w("        beq sync")
	w("sync    ldx #$18          ; X = 40 columns, Y = 38 columns (multicolor)")
	w("        ldy #$10")

	// from here: exact timing. Compute the delay to line 29, cycle 53.
	// The double interrupt (irq at line 21) leaves "sync" at a fixed cycle;
	// see svSyncLine/svSyncCycle.
	cyc := (svSyncLine*63 + svSyncCycle + 4) // after ldx/ldy
	target := svFirstLine*63 + 53 + SideViewerTiming
	delay := target - cyc
	for _, l := range svDelay(delay) {
		w("%s", l)
	}

	// --- jobs per line ---
	lines := map[int]*svLine{}
	get := func(l int) *svLine {
		if lines[l] == nil {
			lines[l] = &svLine{preload: -1}
		}
		return lines[l]
	}
	addJob := func(l int, j svJob) { get(l).jobs = append(get(l).jobs, j) }
	nonBad := func(l int) int {
		for svBadline(l) {
			l--
		}
		return l
	}
	// row 0 starts at y = 50 after the hidden row
	addJob(40, svJob{val: 50, addrs: []int{0xD005, 0xD007}, comment: "y of sprite row 0"})
	for m := 0; m < SideRows-1; m++ {
		t := 71 + 21*m // last line of row m; row m+1 starts at t+1
		seg := m + 1
		addJob(nonBad(t-10), svJob{val: uint8(t), addrs: []int{0xD005, 0xD007}, comment: fmt.Sprintf("y of sprite row %d", seg)})
		switch {
		case m < 4:
			addJob(t, svJob{val: uint8(ptrL(seg)), addrs: []int{svScreenA + 0x3FA}, comment: fmt.Sprintf("pointer left row %d", seg)})
			addJob(t, svJob{val: uint8(ptrR(seg)), addrs: []int{svScreenA + 0x3FB}, comment: fmt.Sprintf("pointer right row %d", seg)})
		case m == 4:
			// bad line: switch to screen B (pointers of row 5)
			get(t - 1).preload = 0x18
			get(t).bad = &svJob{val: 0x18, addrs: []int{0xD018}, comment: "screen B: pointers of row 5"}
		default:
			addJob(t, svJob{val: uint8(ptrL(seg)), addrs: []int{svScreenB + 0x3FA}, comment: fmt.Sprintf("pointer left row %d", seg)})
			addJob(t, svJob{val: uint8(ptrR(seg)), addrs: []int{svScreenB + 0x3FB}, comment: fmt.Sprintf("pointer right row %d", seg)})
		}
		lc := svJob{val: s.Col[0][seg] & 15, addrs: []int{0xD029}, comment: fmt.Sprintf("color left row %d", seg)}
		if svBadline(t) {
			lc.maxWrite = 12
			addJob(t+1, lc)
		} else {
			lc.minWrite = 17
			addJob(t, lc)
		}
		rc := svJob{val: s.Col[1][seg] & 15, addrs: []int{0xD02A}, comment: fmt.Sprintf("color right row %d", seg)}
		if svBadline(t + 1) {
			get(t).preload = int(rc.val)
			get(t + 1).bad = &rc
		} else {
			addJob(t+1, rc)
		}
	}

	// --- the timed lines ---
	w("")
	w("; --- lines %d-%d: cycle-exact. Each line: free cycles, then", svFirstLine, svLastLine)
	w(";     sty $d016 (38 columns, write in cycle 56) and stx $d016 (40 columns,")
	w(";     the CPU waits for the sprite DMA and writes in cycle 4 of the next line)")
	for l := svFirstLine; l <= svLastLine; l++ {
		ln := get(l)
		bad := svBadline(l)
		kind := ""
		if bad {
			kind = " (bad line)"
		}
		w("l%03d                      ; raster line %d%s", l, l, kind)
		if l == svFirstLine {
			// enters directly before sty (cycle 53)
		} else if bad {
			if ln.bad != nil {
				j := ln.bad
				w("        sta $%04x,x       ; %s (A was loaded before)", j.addrs[0]-0x18, j.comment)
			} else {
				w("        nop")
				w("        bit $02")
			}
			if len(ln.jobs) > 0 {
				panic(fmt.Sprintf("side border: jobs on bad line %d", l))
			}
		} else {
			c := 5
			// jobs that must be early, then the rest, late ones last
			var early, mid, late []svJob
			for _, j := range ln.jobs {
				switch {
				case j.maxWrite > 0:
					early = append(early, j)
				case j.minWrite > 0:
					late = append(late, j)
				default:
					mid = append(mid, j)
				}
			}
			emit := func(j svJob) {
				if j.minWrite > 0 && c+5 < j.minWrite {
					pad := j.minWrite - 5 - c
					if pad%2 == 1 {
						pad++
					}
					for _, x := range svDelay(pad) {
						w("%s", x)
					}
					c += pad
				}
				w("        lda #%d           ; %s", j.val, j.comment)
				c += 2
				for _, a := range j.addrs {
					w("        sta $%04x", a)
					c += 4
				}
				if j.maxWrite > 0 && c-1 > j.maxWrite {
					panic(fmt.Sprintf("side border: line %d: %s too late", l, j.comment))
				}
			}
			for _, j := range early {
				emit(j)
			}
			for _, j := range mid {
				emit(j)
			}
			for _, j := range late {
				emit(j)
			}
			if ln.preload >= 0 {
				w("        lda #%d           ; for the next (bad) line", ln.preload)
				c += 2
			}
			if c > 53 {
				panic(fmt.Sprintf("side border: line %d needs %d cycles", l, c))
			}
			for _, x := range svDelay(53 - c) {
				w("%s", x)
			}
		}
		w("        sty $d016")
		w("        stx $d016")
	}
	w("")
	w("; --- end of the picture: prepare the next frame ---")
	w("        jsr frame")
	w("        lda #$7f          ; Space pressed?")
	w("        sta $dc00")
	w("        lda $dc01")
	w("        and #$10")
	w("        beq quit")
	w("        lda #<irq1")
	w("        sta $fffe")
	w("        lda #>irq1")
	w("        sta $ffff")
	w("        lda #%d", svIRQLine)
	w("        sta $d012")
	w("        lda #$01")
	w("        sta $d019")
	w("        rti")
	w("quit    sei")
	w("        lda #0")
	w("        sta $d015")
	w("        sta $d01a")
	w("        lda #$37")
	w("        sta $01")
	w("        jmp $fce2         ; reset")
	w("")
	w("; delay: jsr dly+24-n waits 12+2n cycles")
	w("dly")
	for i := 0; i < 24; i++ {
		w("        nop")
	}
	w("        rts")
	w("")

	// --- data ---
	dump := func(label string, data []byte, perLine int) {
		w("%s", label)
		for i := 0; i < len(data); i += perLine {
			var parts []string
			for _, v := range data[i:min(len(data), i+perLine)] {
				parts = append(parts, fmt.Sprintf("$%02x", v))
			}
			w("        !byte %s", strings.Join(parts, ","))
		}
	}
	kla := p.KLABytes() // bitmap 8000, screen 1000, color 1000, background
	dump("d_bitmap  ; 8000 bytes bitmap", kla[:8000], 20)
	dump("d_screen  ; 1000 bytes screen RAM (colors %01 and %10)", kla[8000:9000], 20)
	colors := make([]byte, 1000)
	for i := range colors {
		colors[i] = kla[9000+i] & 15
	}
	dump("d_color   ; 1000 bytes color RAM (color %11)", colors, 20)
	w("")
	w("* = $%04x", svSprites)
	w("; 20 sprites: left rows 0-9, right rows 0-9 (multicolor, 21 lines each)")
	for side := 0; side < SideCount; side++ {
		for row := 0; row < SideRows; row++ {
			spr := p.SideSprite(side, row)
			dump(fmt.Sprintf("spr_%s%d", [2]string{"l", "r"}[side], row), spr[:], 21)
		}
	}
	return b.String()
}

// Where the double interrupt leaves "sync" (line and cycle of the next
// instruction), measured in VICE x64sc.
var (
	svSyncLine  = 22
	svSyncCycle = 4
)

// sideViewerMainNops changes the main loop (tests: other interrupt jitter).
var sideViewerMainNops = 0

// svDelay returns code that waits exactly n cycles (n = 0 or n >= 2) and
// keeps X and Y.
func svDelay(n int) []string {
	var out []string
	if n < 0 || n == 1 {
		panic(fmt.Sprintf("side border: cannot wait %d cycles", n))
	}
	for n >= 2*63 {
		// a loop on A: lda #k / loop: sec / sbc #1 / bne loop  = 2 + 7k - 1
		k := min(255, (n-1-2)/7)
		if k < 1 {
			break
		}
		out = append(out, fmt.Sprintf("        lda #%d", k))
		out = append(out, "        sec")
		out = append(out, "        sbc #1")
		out = append(out, "        bne *-3")
		n -= 2 + 7*k - 1
		if n == 1 {
			// take one round less and wait the rest below
			out = out[:len(out)-4]
			n += 2 + 7*k - 1
			k--
			out = append(out, fmt.Sprintf("        lda #%d", k), "        sec", "        sbc #1", "        bne *-3")
			n -= 2 + 7*k - 1
		}
	}
	if n%2 == 1 {
		out = append(out, "        bit $02")
		n -= 3
	}
	if n >= 14 {
		k := (n - 12) / 2
		if k <= 24 {
			out = append(out, fmt.Sprintf("        jsr dly+%d", 24-k))
			return out
		}
	}
	for ; n > 0; n -= 2 {
		out = append(out, "        nop")
	}
	return out
}
