package c64

import "math"

// ColorMatch selects how "the most similar color" is measured when an image
// is converted (idea from Retropixels by Michel de Bree, MIT license).
type ColorMatch int

const (
	MatchRGB   ColorMatch = iota // weighted RGB ("redmean"), the default
	MatchYUV                     // luma + color difference (TV)
	MatchXYZ                     // CIE XYZ
	MatchLab                     // CIE L*a*b*
	MatchOkLab                   // OKLab
	MatchLuma                    // brightness only
	matchCount
)

// MatchNames are the display names of the color matching methods.
var MatchNames = [matchCount]string{"RGB", "YUV", "XYZ", "Lab", "OkLab", "Brightness"}

// MatchInfo explains the methods (tooltips).
var MatchInfo = [matchCount]string{
	"Weighted RGB distance – quick and good for most images",
	"Brightness and color difference as on a TV – keeps brightness steps",
	"CIE XYZ – physical light, good for photos",
	"CIE Lab – close to how the eye sees differences",
	"OkLab – modern, even perception of hue and brightness",
	"Only the brightness counts – the colors follow the light and dark parts",
}

var srgbLinear [256]float64

func init() {
	for i := range srgbLinear {
		v := float64(i) / 255
		if v > 0.04045 {
			v = math.Pow((v+0.055)/1.055, 2.4)
		} else {
			v /= 12.92
		}
		srgbLinear[i] = v
	}
}

func clampByte(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// space converts a color into the coordinates of the method, scaled so that
// the squared distances are in a similar range for all methods.
func (m ColorMatch) space(r, g, b int) [3]float64 {
	r, g, b = clampByte(r), clampByte(g), clampByte(b)
	switch m {
	case MatchYUV:
		fr, fg, fb := float64(r), float64(g), float64(b)
		return [3]float64{
			fr*0.299 + fg*0.587 + fb*0.114,
			fr*-0.14713 + fg*-0.28886 + fb*0.436,
			fr*0.615 + fg*-0.51499 + fb*-0.10001,
		}
	case MatchLuma:
		return [3]float64{float64(r)*0.299 + float64(g)*0.587 + float64(b)*0.114, 0, 0}
	}
	lr, lg, lb := srgbLinear[r], srgbLinear[g], srgbLinear[b]
	switch m {
	case MatchXYZ:
		return [3]float64{
			(lr*41.24 + lg*35.76 + lb*18.05) * 2.5,
			(lr*21.26 + lg*71.52 + lb*7.22) * 2.5,
			(lr*1.93 + lg*11.92 + lb*95.05) * 2.5,
		}
	case MatchLab:
		x := (lr*41.24 + lg*35.76 + lb*18.05) / 95.047
		y := (lr*21.26 + lg*71.52 + lb*7.22) / 100
		z := (lr*1.93 + lg*11.92 + lb*95.05) / 108.883
		f := func(t float64) float64 {
			if t > 0.008856 {
				return math.Cbrt(t)
			}
			return 7.787*t + 16.0/116
		}
		fx, fy, fz := f(x), f(y), f(z)
		return [3]float64{(116*fy - 16) * 2.5, 500 * (fx - fy) * 2.5, 200 * (fy - fz) * 2.5}
	case MatchOkLab:
		l := math.Cbrt(0.4122214708*lr + 0.5363325363*lg + 0.0514459929*lb)
		mm := math.Cbrt(0.2119034982*lr + 0.6806995451*lg + 0.1073969566*lb)
		s := math.Cbrt(0.0883024619*lr + 0.2817188376*lg + 0.6299787005*lb)
		return [3]float64{
			(0.2104542553*l + 0.7936177850*mm - 0.0040720468*s) * 255,
			(1.9779984951*l - 2.4285922050*mm + 0.4505937099*s) * 255,
			(0.0259040371*l + 0.7827717662*mm - 0.8086757660*s) * 255,
		}
	}
	return [3]float64{float64(r), float64(g), float64(b)}
}

// colorMetric returns the distance function of a method (squared distance,
// comparable within one method).
func colorMetric(m ColorMatch) func(r1, g1, b1, r2, g2, b2 int) int {
	if m <= MatchRGB || m >= matchCount {
		return ColorDist
	}
	cache := make(map[int32][3]float64, 4096)
	conv := func(r, g, b int) [3]float64 {
		key := int32(clampByte(r))<<16 | int32(clampByte(g))<<8 | int32(clampByte(b))
		if v, ok := cache[key]; ok {
			return v
		}
		v := m.space(r, g, b)
		if len(cache) < 1<<18 {
			cache[key] = v
		}
		return v
	}
	return func(r1, g1, b1, r2, g2, b2 int) int {
		a, b := conv(r1, g1, b1), conv(r2, g2, b2)
		d0, d1, d2 := a[0]-b[0], a[1]-b[1], a[2]-b[2]
		return int((d0*d0 + d1*d1 + d2*d2) * 4)
	}
}
