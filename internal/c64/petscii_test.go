package c64

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func testScreen() *PetsciiScreen {
	s := NewPetscii(6, 14)
	for i := range s.Chars {
		s.Chars[i] = uint8(i * 7)
		s.Colors[i] = uint8(i % 16)
	}
	return s
}

// run6502 executes the few opcodes the viewer uses until it calls $FFE4.
func run6502(t *testing.T, prg []byte) []byte {
	mem := make([]byte, 65536)
	load := int(prg[0]) | int(prg[1])<<8
	copy(mem[load:], prg[2:])
	pc, a, x := 0x080D, byte(0), byte(0)
	z := false
	abs := func() int { return int(mem[pc+1]) | int(mem[pc+2])<<8 }
	for steps := 0; steps < 100000; steps++ {
		switch mem[pc] {
		case 0xA9:
			a, z = mem[pc+1], mem[pc+1] == 0
			pc += 2
		case 0xA2:
			x, z = mem[pc+1], mem[pc+1] == 0
			pc += 2
		case 0x8D:
			mem[abs()] = a
			pc += 3
		case 0xBD:
			a = mem[abs()+int(x)]
			z = a == 0
			pc += 3
		case 0x9D:
			mem[abs()+int(x)] = a
			pc += 3
		case 0xE8:
			x++
			z = x == 0
			pc++
		case 0xD0:
			if !z {
				pc += 2 + int(int8(mem[pc+1]))
			} else {
				pc += 2
			}
		case 0x20:
			if abs() == 0xFFE4 {
				return mem
			}
			t.Fatalf("unexpected jsr $%04x", abs())
		default:
			t.Fatalf("unknown opcode $%02x at $%04x", mem[pc], pc)
		}
	}
	t.Fatal("viewer does not reach the key wait")
	return nil
}

func TestPetsciiPRG(t *testing.T) {
	s := testScreen()
	mem := run6502(t, s.PRG())
	if mem[0xD020] != 14 || mem[0xD021] != 6 || mem[0xD018] != 0x15 {
		t.Fatal("colors / charset registers")
	}
	for i := 0; i < PetCells; i++ {
		if mem[0x0400+i] != s.Chars[i] || mem[0xD800+i] != s.Colors[i] {
			t.Fatalf("cell %d not copied", i)
		}
	}
	s.Lower = true
	if run6502(t, s.PRG())[0xD018] != 0x17 {
		t.Fatal("lower case charset")
	}
}

// TestPetsciiACME assembles the ACME source (when acme is available) and
// compares it with the PRG export.
func TestPetsciiACME(t *testing.T) {
	acme, err := exec.LookPath("acme")
	if err != nil {
		if p := os.Getenv("ACME"); p != "" {
			acme = p
		} else {
			t.Skip("acme not installed")
		}
	}
	s := testScreen()
	dir := t.TempDir()
	src := filepath.Join(dir, "pic.asm")
	os.WriteFile(src, []byte(s.ACME("pic")), 0o644)
	out := filepath.Join(dir, "pic.prg")
	cmd := exec.Command(acme, "--cpu", "6510", "--format", "cbm", "--outfile", out, src)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("acme: %v\n%s", err, b)
	}
	got, _ := os.ReadFile(out)
	if string(got) != string(s.PRG()) {
		t.Fatalf("ACME output (%d bytes) differs from the PRG export (%d bytes)", len(got), len(s.PRG()))
	}
}

func TestPetsciiConvertAndPetmate(t *testing.T) {
	pal := &Palettes[FindPalette(DefaultPalette)]
	// a screen rendered from a known PETSCII screen converts back to it
	s := testScreen()
	for i := range s.Chars {
		s.Chars[i] = uint8(i % 128)
		if s.Colors[i] == s.BG {
			s.Colors[i] = 1
		}
	}
	px := make([][3]int, 320*200)
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			c := pal.Colors[s.ColorAt(x, y)]
			px[y*320+x] = [3]int{int(c >> 16 & 255), int(c >> 8 & 255), int(c & 255)}
		}
	}
	q := ConvertPetscii(px, pal, PetsciiOptions{BG: -1})
	wrong := 0
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			if q.ColorAt(x, y) != s.ColorAt(x, y) {
				wrong++
			}
		}
	}
	if wrong > 0 {
		t.Fatalf("%d pixels differ after converting back", wrong)
	}
	data, err := q.Petmate("test")
	if err != nil {
		t.Fatal(err)
	}
	r, err := LoadPetmate(data)
	if err != nil || r.Chars != q.Chars || r.Colors != q.Colors || r.BG != q.BG {
		t.Fatal("Petmate round trip")
	}
}
