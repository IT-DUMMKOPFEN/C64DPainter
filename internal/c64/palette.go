package c64

import (
	_ "embed"
	"strconv"
	"strings"
)

//go:embed palettes.txt
var paletteText string

// Palette ist eine Interpretation der 16 C64-Farben.
type Palette struct {
	Name   string
	Colors [16]uint32 // 0x00RRGGBB
	Note   string
}

// ColorNames sind die deutschen Namen der 16 C64-Farben.
var ColorNames = [16]string{
	"Black", "White", "Red", "Cyan", "Purple", "Green", "Blue", "Yellow",
	"Orange", "Brown", "Light red", "Dark grey", "Grey", "Light green", "Light blue", "Light grey",
}

// Palettes enthält alle Paletten (Quelle: c64gfx.com/palettes).
var Palettes []Palette

// DefaultPalette ist der Name der voreingestellten Palette.
const DefaultPalette = "PIXCEN_Colodore"

func init() {
	var fallback [16]uint32
	for _, line := range strings.Split(paletteText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, vals, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		parts := strings.Split(strings.TrimSpace(vals), ",")
		if len(parts) != 16 {
			continue
		}
		p := Palette{Name: strings.TrimSpace(name)}
		var missing []string
		for i, h := range parts {
			h = strings.TrimSpace(h)
			if strings.HasPrefix(h, "badba") { // Platzhalter auf der Quellseite
				missing = append(missing, ColorNames[i])
				p.Colors[i] = 0xFFFFFFFF
				continue
			}
			v, err := strconv.ParseUint(h, 16, 32)
			if err != nil {
				continue
			}
			p.Colors[i] = uint32(v)
		}
		if len(Palettes) == 0 {
			fallback = p.Colors
		}
		if len(missing) > 0 {
			if len(missing) > 2 {
				continue // unvollständige Palette (z. B. 8-Farben-Ladebild) weglassen
			}
			for i := range p.Colors {
				if p.Colors[i] == 0xFFFFFFFF {
					p.Colors[i] = fallback[i]
				}
			}
			p.Note = "Incomplete source: " + strings.Join(missing, ", ") + " taken from Pepto"
		}
		Palettes = append(Palettes, p)
	}
}

// FindPalette sucht eine Palette nach Name (oder liefert die Standardpalette).
func FindPalette(name string) int {
	for i, p := range Palettes {
		if p.Name == name {
			return i
		}
	}
	for i, p := range Palettes {
		if p.Name == DefaultPalette {
			return i
		}
	}
	return 0
}

// Nearest liefert den Index der nächstgelegenen Palettenfarbe (gewichtete RGB-Distanz).
func (p *Palette) Nearest(r, g, b int) int {
	best, bd := 0, 1<<62
	for i, c := range p.Colors {
		d := ColorDist(r, g, b, int(c>>16&255), int(c>>8&255), int(c&255))
		if d < bd {
			bd, best = d, i
		}
	}
	return best
}

// ColorDist ist eine einfache wahrnehmungsgewichtete Farbdistanz („redmean“).
func ColorDist(r1, g1, b1, r2, g2, b2 int) int {
	rm := (r1 + r2) / 2
	dr, dg, db := r1-r2, g1-g2, b1-b2
	return ((512+rm)*dr*dr)>>8 + 4*dg*dg + ((767-rm)*db*db)>>8
}
