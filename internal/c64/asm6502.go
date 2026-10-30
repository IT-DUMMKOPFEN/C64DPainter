package c64

import (
	"fmt"
	"strconv"
	"strings"
)

// --- A small 6502 assembler for the ACME syntax used by our exports ------------------
//
// The side border viewer is written once as ACME source; the PRG is made by
// assembling that same source here, so the exported .asm and the .prg are
// always the same program. Supported: labels, "* = addr", !byte/!by,
// !word/!wo, !fill, !cpu, comments, numbers ($hex, %bin, decimal), labels and
// "*" with + and -, < and > (low/high byte), and all legal 6502 opcodes.

type asmMode int

const (
	amImp asmMode = iota
	amImm
	amZp
	amZpX
	amZpY
	amAbs
	amAbsX
	amAbsY
	amInd
	amIndX
	amIndY
	amRel
)

var opTable = map[string]map[asmMode]byte{
	"adc": {amImm: 0x69, amZp: 0x65, amZpX: 0x75, amAbs: 0x6D, amAbsX: 0x7D, amAbsY: 0x79, amIndX: 0x61, amIndY: 0x71},
	"and": {amImm: 0x29, amZp: 0x25, amZpX: 0x35, amAbs: 0x2D, amAbsX: 0x3D, amAbsY: 0x39, amIndX: 0x21, amIndY: 0x31},
	"asl": {amImp: 0x0A, amZp: 0x06, amZpX: 0x16, amAbs: 0x0E, amAbsX: 0x1E},
	"bcc": {amRel: 0x90}, "bcs": {amRel: 0xB0}, "beq": {amRel: 0xF0}, "bmi": {amRel: 0x30},
	"bne": {amRel: 0xD0}, "bpl": {amRel: 0x10}, "bvc": {amRel: 0x50}, "bvs": {amRel: 0x70},
	"bit": {amZp: 0x24, amAbs: 0x2C},
	"brk": {amImp: 0x00}, "clc": {amImp: 0x18}, "cld": {amImp: 0xD8}, "cli": {amImp: 0x58}, "clv": {amImp: 0xB8},
	"cmp": {amImm: 0xC9, amZp: 0xC5, amZpX: 0xD5, amAbs: 0xCD, amAbsX: 0xDD, amAbsY: 0xD9, amIndX: 0xC1, amIndY: 0xD1},
	"cpx": {amImm: 0xE0, amZp: 0xE4, amAbs: 0xEC},
	"cpy": {amImm: 0xC0, amZp: 0xC4, amAbs: 0xCC},
	"dec": {amZp: 0xC6, amZpX: 0xD6, amAbs: 0xCE, amAbsX: 0xDE},
	"dex": {amImp: 0xCA}, "dey": {amImp: 0x88},
	"eor": {amImm: 0x49, amZp: 0x45, amZpX: 0x55, amAbs: 0x4D, amAbsX: 0x5D, amAbsY: 0x59, amIndX: 0x41, amIndY: 0x51},
	"inc": {amZp: 0xE6, amZpX: 0xF6, amAbs: 0xEE, amAbsX: 0xFE},
	"inx": {amImp: 0xE8}, "iny": {amImp: 0xC8},
	"jmp": {amAbs: 0x4C, amInd: 0x6C},
	"jsr": {amAbs: 0x20},
	"lda": {amImm: 0xA9, amZp: 0xA5, amZpX: 0xB5, amAbs: 0xAD, amAbsX: 0xBD, amAbsY: 0xB9, amIndX: 0xA1, amIndY: 0xB1},
	"ldx": {amImm: 0xA2, amZp: 0xA6, amZpY: 0xB6, amAbs: 0xAE, amAbsY: 0xBE},
	"ldy": {amImm: 0xA0, amZp: 0xA4, amZpX: 0xB4, amAbs: 0xAC, amAbsX: 0xBC},
	"lsr": {amImp: 0x4A, amZp: 0x46, amZpX: 0x56, amAbs: 0x4E, amAbsX: 0x5E},
	"nop": {amImp: 0xEA},
	"ora": {amImm: 0x09, amZp: 0x05, amZpX: 0x15, amAbs: 0x0D, amAbsX: 0x1D, amAbsY: 0x19, amIndX: 0x01, amIndY: 0x11},
	"pha": {amImp: 0x48}, "php": {amImp: 0x08}, "pla": {amImp: 0x68}, "plp": {amImp: 0x28},
	"rol": {amImp: 0x2A, amZp: 0x26, amZpX: 0x36, amAbs: 0x2E, amAbsX: 0x3E},
	"ror": {amImp: 0x6A, amZp: 0x66, amZpX: 0x76, amAbs: 0x6E, amAbsX: 0x7E},
	"rti": {amImp: 0x40}, "rts": {amImp: 0x60},
	"sbc": {amImm: 0xE9, amZp: 0xE5, amZpX: 0xF5, amAbs: 0xED, amAbsX: 0xFD, amAbsY: 0xF9, amIndX: 0xE1, amIndY: 0xF1},
	"sec": {amImp: 0x38}, "sed": {amImp: 0xF8}, "sei": {amImp: 0x78},
	"sta": {amZp: 0x85, amZpX: 0x95, amAbs: 0x8D, amAbsX: 0x9D, amAbsY: 0x99, amIndX: 0x81, amIndY: 0x91},
	"stx": {amZp: 0x86, amZpY: 0x96, amAbs: 0x8E},
	"sty": {amZp: 0x84, amZpX: 0x94, amAbs: 0x8C},
	"tax": {amImp: 0xAA}, "tay": {amImp: 0xA8}, "tsx": {amImp: 0xBA}, "txa": {amImp: 0x8A}, "txs": {amImp: 0x9A}, "tya": {amImp: 0x98},
}

// Assemble translates ACME source into a C64 program file (load address
// first). The output starts at the first "* =" address; gaps are filled
// with zeros.
func Assemble(src string) ([]byte, error) {
	labels := map[string]int{}
	var mem map[int]byte
	start, end := -1, -1
	for pass := 0; pass < 2; pass++ {
		mem = map[int]byte{}
		pc := -1
		final := pass == 1
		put := func(b byte) {
			mem[pc] = b
			if start < 0 || pc < start {
				start = pc
			}
			if pc+1 > end {
				end = pc + 1
			}
			pc++
		}
		for n, raw := range strings.Split(src, "\n") {
			line := raw
			if i := commentIndex(line); i >= 0 {
				line = line[:i]
			}
			if strings.TrimSpace(line) == "" {
				continue
			}
			fail := func(format string, a ...any) error {
				return fmt.Errorf("line %d: %s: %q", n+1, fmt.Sprintf(format, a...), strings.TrimSpace(raw))
			}
			eval := func(e string) (int, error) {
				v, ok, err := asmEval(e, labels, pc)
				if err != nil {
					return 0, fail("%v", err)
				}
				if !ok && final {
					return 0, fail("unknown label")
				}
				return v, nil
			}
			// label at the start of the line
			if line[0] != ' ' && line[0] != '\t' && line[0] != '*' && line[0] != '!' {
				f := strings.Fields(line)
				name := strings.TrimSuffix(f[0], ":")
				if _, isOp := opTable[strings.ToLower(name)]; !isOp {
					if pc < 0 {
						return nil, fail("label before \"* =\"")
					}
					if !final {
						if _, dup := labels[name]; dup {
							return nil, fail("label defined twice")
						}
					}
					labels[name] = pc
					line = strings.TrimSpace(line[len(f[0]):])
					if line == "" {
						continue
					}
				}
			}
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "*"):
				e := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line[1:]), "="))
				v, err := eval(e)
				if err != nil {
					return nil, err
				}
				pc = v
				continue
			case strings.HasPrefix(line, "!"):
				f := strings.SplitN(line, " ", 2)
				args := ""
				if len(f) > 1 {
					args = f[1]
				}
				switch strings.ToLower(f[0]) {
				case "!cpu", "!to", "!sl":
				case "!byte", "!by", "!8":
					for _, e := range strings.Split(args, ",") {
						v, err := eval(e)
						if err != nil {
							return nil, err
						}
						put(byte(v))
					}
				case "!word", "!wo", "!16":
					for _, e := range strings.Split(args, ",") {
						v, err := eval(e)
						if err != nil {
							return nil, err
						}
						put(byte(v))
						put(byte(v >> 8))
					}
				case "!fill", "!fi":
					p := strings.Split(args, ",")
					cnt, err := eval(p[0])
					if err != nil {
						return nil, err
					}
					val := 0
					if len(p) > 1 {
						if val, err = eval(p[1]); err != nil {
							return nil, err
						}
					}
					for i := 0; i < cnt; i++ {
						put(byte(val))
					}
				default:
					return nil, fail("unknown pseudo opcode")
				}
				continue
			}
			if pc < 0 {
				return nil, fail("code before \"* =\"")
			}
			mn := strings.ToLower(line[:min(3, len(line))])
			modes, ok := opTable[mn]
			if !ok || (len(line) > 3 && line[3] != ' ' && line[3] != '\t') {
				return nil, fail("unknown instruction")
			}
			op := strings.TrimSpace(line[3:])
			mode, expr, err := asmMode2(op, modes)
			if err != nil {
				return nil, fail("%v", err)
			}
			v := 0
			if expr != "" {
				if v, err = eval(expr); err != nil {
					return nil, err
				}
			}
			// zero page when the value is known and small and the opcode has it
			zp := map[asmMode]asmMode{amAbs: amZp, amAbsX: amZpX, amAbsY: amZpY}
			if z, has := zp[mode]; has && v >= 0 && v < 256 && isNumeric(expr) {
				if _, ok := modes[z]; ok {
					mode = z
				}
			}
			code, ok := modes[mode]
			if !ok {
				return nil, fail("addressing mode not available")
			}
			put(code)
			switch mode {
			case amImp:
			case amImm, amZp, amZpX, amZpY, amIndX, amIndY:
				if final && (v < -128 || v > 255) {
					return nil, fail("value out of range")
				}
				put(byte(v))
			case amRel:
				rel := v - (pc + 1)
				if final && (rel < -128 || rel > 127) {
					return nil, fail("branch out of range")
				}
				put(byte(rel))
			default:
				put(byte(v))
				put(byte(v >> 8))
			}
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("no code")
	}
	out := []byte{byte(start), byte(start >> 8)}
	for a := start; a < end; a++ {
		out = append(out, mem[a])
	}
	return out, nil
}

func commentIndex(s string) int {
	inQ := false
	for i, r := range s {
		switch r {
		case '"', '\'':
			inQ = !inQ
		case ';':
			if !inQ {
				return i
			}
		}
	}
	return -1
}

func isNumeric(e string) bool {
	e = strings.TrimSpace(e)
	return e != "" && (e[0] == '$' || e[0] == '%' || (e[0] >= '0' && e[0] <= '9'))
}

func asmMode2(op string, modes map[asmMode]byte) (asmMode, string, error) {
	low := strings.ToLower(strings.ReplaceAll(op, " ", ""))
	if _, rel := modes[amRel]; rel {
		return amRel, op, nil
	}
	switch {
	case op == "" || low == "a":
		return amImp, "", nil
	case strings.HasPrefix(op, "#"):
		return amImm, op[1:], nil
	case strings.HasPrefix(low, "(") && strings.HasSuffix(low, ",x)"):
		return amIndX, op[1:strings.LastIndex(op, ",")], nil
	case strings.HasPrefix(low, "(") && strings.HasSuffix(low, "),y"):
		return amIndY, op[1:strings.Index(op, ")")], nil
	case strings.HasPrefix(low, "(") && strings.HasSuffix(low, ")"):
		return amInd, op[1 : len(op)-1], nil
	case strings.HasSuffix(low, ",x"):
		return amAbsX, op[:strings.LastIndex(op, ",")], nil
	case strings.HasSuffix(low, ",y"):
		return amAbsY, op[:strings.LastIndex(op, ",")], nil
	}
	return amAbs, op, nil
}

// asmEval evaluates term ((+|-) term)*. ok is false when a label is unknown.
func asmEval(e string, labels map[string]int, pc int) (int, bool, error) {
	e = strings.TrimSpace(e)
	if e == "" {
		return 0, false, fmt.Errorf("missing value")
	}
	lohi := byte(0)
	if e[0] == '<' || e[0] == '>' {
		lohi = e[0]
		e = strings.TrimSpace(e[1:])
	}
	total, ok := 0, true
	sign := 1
	i := 0
	for i < len(e) {
		for i < len(e) && e[i] == ' ' {
			i++
		}
		j := i
		if j < len(e) && e[j] == '*' {
			j++
		} else if j < len(e) && e[j] == '\'' && j+2 < len(e) {
			j += 3
		} else {
			for j < len(e) && e[j] != '+' && e[j] != '-' && e[j] != ' ' {
				j++
			}
		}
		t := e[i:j]
		var v int
		switch {
		case t == "*":
			v = pc
		case strings.HasPrefix(t, "'"):
			v = int(t[1])
		case strings.HasPrefix(t, "$"):
			x, err := strconv.ParseInt(t[1:], 16, 32)
			if err != nil {
				return 0, false, err
			}
			v = int(x)
		case strings.HasPrefix(t, "%"):
			x, err := strconv.ParseInt(t[1:], 2, 32)
			if err != nil {
				return 0, false, err
			}
			v = int(x)
		case t != "" && t[0] >= '0' && t[0] <= '9':
			x, err := strconv.Atoi(t)
			if err != nil {
				return 0, false, err
			}
			v = x
		case t == "":
			return 0, false, fmt.Errorf("bad expression")
		default:
			lv, found := labels[t]
			if !found {
				ok = false
			}
			v = lv
		}
		total += sign * v
		for j < len(e) && e[j] == ' ' {
			j++
		}
		if j >= len(e) {
			break
		}
		switch e[j] {
		case '+':
			sign = 1
		case '-':
			sign = -1
		default:
			return 0, false, fmt.Errorf("bad expression")
		}
		i = j + 1
	}
	switch lohi {
	case '<':
		total &= 0xFF
	case '>':
		total = (total >> 8) & 0xFF
	}
	return total, ok, nil
}
