package c64

import (
	"bytes"
	"math/rand"
	"testing"
)

func sameImage(t *testing.T, name string, a, b *Picture, pal *Palette) {
	t.Helper()
	for y := 0; y < H; y++ {
		w := W
		if a.Hires() {
			w = HW
		}
		for x := 0; x < w; x++ {
			if a.ColorAt(x, y) != b.ColorAt(x, y) {
				t.Fatalf("%s: pixel %d,%d differs", name, x, y)
			}
		}
	}
}

func TestMoreFormatsRoundTrip(t *testing.T) {
	pal := &Palettes[0]
	mc := []*Picture{randomPicture(3), NewPicture(0, 0)}
	mc[0].Border = 5
	// a picture with long runs and the escape bytes inside the data
	mc[1].BG = 0xe
	for i := 0; i < 40; i++ {
		mc[1].Screen[i] = 0xfe
		mc[1].Screen[i+40] = 0xc2
	}
	r := rand.New(rand.NewSource(9))
	hi := NewHiresPicture(ModeHires, 6, 14)
	for i := 0; i < 30000; i++ {
		hi.Plot(r.Intn(320), r.Intn(H), uint8(r.Intn(16)), ConflictMode(r.Intn(3)), pal)
	}
	for _, f := range Formats {
		if f.Save == nil {
			continue
		}
		pics := mc
		if f.Hires {
			pics = []*Picture{hi, NewHiresPicture(ModeHires, 0, 0)}
		}
		for k, p := range pics {
			data := f.Save(p, pal)
			q, err := f.Load(data)
			if err != nil {
				t.Fatalf("%s %d: %v", f.Name, k, err)
			}
			sameImage(t, f.Name, p, q, pal)
			if f.Border && q.Border != p.Border {
				t.Fatalf("%s: border", f.Name)
			}
			i, g := FormatByExt(f.Exts[0])
			if g == nil || Formats[i].Name != f.Name {
				t.Fatal("FormatByExt")
			}
			if k == 1 && len(data) > 3000 && f.Name != "Image System" && f.Name != "Vidcom 64" && f.Name != "Blazing Paddles" && f.Name != "Advanced Art Studio" && f.Name != "Hi-Eddi" {
				t.Fatalf("%s: empty picture not packed (%d bytes)", f.Name, len(data))
			}
		}
	}
}

func TestDRZUnpackedAndKoalaLike(t *testing.T) {
	p := randomPicture(5)
	raw := layDRZ.encode(p)
	q, err := LoadDRZ(raw)
	if err != nil {
		t.Fatal(err)
	}
	sameImage(t, "drz raw", p, q, nil)
	if !bytes.Equal(LoadAMIRoundTrip(p), p.KLABytes()) {
		t.Fatal("ami")
	}
	// Run Paint = Koala + 3 bytes
	rpm := append(p.SaveKLA(), 0, 0, 0)
	_, f := FormatByExt(".rpm")
	q, err = f.Load(rpm)
	if err != nil {
		t.Fatal(err)
	}
	sameImage(t, "rpm", p, q, nil)
	// GG starts with the Koala load address
	if gg := p.SaveGG(); gg[0] != 0x00 || gg[1] != 0x60 {
		t.Fatal("gg load address")
	}
	if d := p.SaveDRZ(); string(d[2:15]) != "DRAZPAINT 1.4" || d[1] != 0x58 {
		t.Fatal("drz header")
	}
}

func LoadAMIRoundTrip(p *Picture) []byte {
	q, err := LoadAMI(p.SaveAMI())
	if err != nil {
		return nil
	}
	return q.KLABytes()
}
