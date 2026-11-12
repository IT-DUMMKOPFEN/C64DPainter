package c64

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func charmapScreen() [][3]int {
	px := make([][3]int, 320*200)
	pal := &Palettes[0]
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			c := uint8(6)
			if (x/8+y/8)%2 == 0 && x%8 < 4 {
				c = 1
			}
			if y > 100 && (x*x+y)%7 == 0 {
				c = 2 // noise: many different characters
			}
			r, g, b := rgbOf(pal.Colors[c])
			px[y*320+x] = [3]int{r, g, b}
		}
	}
	return px
}

func TestCharMap(t *testing.T) {
	pal := &Palettes[0]
	for _, multi := range []bool{false, true} {
		cm := ConvertCharMap(charmapScreen(), pal, CharMapOptions{Multi: multi, BG: -1, MC1: -1, MC2: -1, Limit: 256})
		if cm.Font.BG != 6 {
			t.Fatalf("background %d", cm.Font.BG)
		}
		if cm.Used > 256 || cm.Used < 2 {
			t.Fatalf("used %d", cm.Used)
		}
		// the upper half has only two different cells
		if cm.ColorAt(0, 0) != 1 || cm.ColorAt(9, 0) != 6 {
			t.Fatalf("multi=%v: pixels %d %d", multi, cm.ColorAt(0, 0), cm.ColorAt(9, 0))
		}
		// a smaller limit merges characters
		small := ConvertCharMap(charmapScreen(), pal, CharMapOptions{Multi: multi, BG: -1, MC1: -1, MC2: -1, Limit: 16})
		if small.Used > 16 || small.Unique != cm.Unique {
			t.Fatalf("limit: %d of %d", small.Used, small.Unique)
		}
		prg, err := cm.PRG()
		if err != nil {
			t.Fatal(err)
		}
		if len(prg) < 0x3000-0x0801+2+cm.Used*8 {
			t.Fatalf("prg %d bytes", len(prg))
		}
		// the same as ACME (when it is installed)
		if acme, err := exec.LookPath("acme"); err == nil {
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, "x_data.asm"), []byte(cm.DataACME("x")), 0o644)
			os.WriteFile(filepath.Join(dir, "x.asm"), []byte(cm.ProgramACME("x", "x_data.asm", "")), 0o644)
			cmd := exec.Command(acme, "--cpu", "6510", "--format", "cbm", "--outfile", "x.prg", "x.asm")
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("acme: %v %s", err, out)
			}
			got, _ := os.ReadFile(filepath.Join(dir, "x.prg"))
			if string(got) != string(prg) {
				t.Fatal("different from ACME")
			}
		}
	}
}
