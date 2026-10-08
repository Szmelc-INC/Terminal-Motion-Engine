// Package engine turns raw RGB frames into terminal cells: color
// adjustment, palette quantisation, dithering and glyph composition.
package engine

import (
	"fmt"
	"math"
	"strings"
)

// RGB is an 8-bit sRGB color.
type RGB struct{ R, G, B uint8 }

// U32 packs the color as 0xRRGGBB.
func (c RGB) U32() uint32 { return uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B) }

// Hex formats the color as #rrggbb.
func (c RGB) Hex() string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// FromU32 unpacks a 0xRRGGBB value.
func FromU32(v uint32) RGB { return RGB{uint8(v >> 16), uint8(v >> 8), uint8(v)} }

// ParseHex accepts "#rgb", "rgb", "#rrggbb" and "rrggbb".
func ParseHex(s string) (RGB, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return RGB{}, fmt.Errorf("invalid color %q (want #rrggbb)", s)
	}
	var v uint32
	for _, ch := range []byte(s) {
		var d byte
		switch {
		case ch >= '0' && ch <= '9':
			d = ch - '0'
		case ch >= 'a' && ch <= 'f':
			d = ch - 'a' + 10
		case ch >= 'A' && ch <= 'F':
			d = ch - 'A' + 10
		default:
			return RGB{}, fmt.Errorf("invalid color %q (want #rrggbb)", s)
		}
		v = v<<4 | uint32(d)
	}
	return FromU32(v), nil
}

var srgbLin [256]float64

func init() {
	for i := range srgbLin {
		c := float64(i) / 255
		if c <= 0.04045 {
			srgbLin[i] = c / 12.92
		} else {
			srgbLin[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
}

func linToSrgb(f float64) float64 {
	if f <= 0.0031308 {
		return f * 12.92
	}
	return 1.055*math.Pow(f, 1/2.4) - 0.055
}

// ToOklab converts sRGB to OKLab.
func ToOklab(c RGB) (L, a, b float64) {
	r, g, bl := srgbLin[c.R], srgbLin[c.G], srgbLin[c.B]
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*bl)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*bl)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*bl)
	return 0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		0.0259040371*l + 0.7827717662*m - 0.8086757660*s
}

// oklabToLinear returns linear sRGB components, possibly out of gamut.
func oklabToLinear(L, a, b float64) (r, g, bl float64) {
	l := L + 0.3963377774*a + 0.2158037573*b
	m := L - 0.1055613458*a - 0.0638541728*b
	s := L - 0.0894841775*a - 1.2914855480*b
	l, m, s = l*l*l, m*m*m, s*s*s
	return 4.0767416621*l - 3.3077115913*m + 0.2309699292*s,
		-1.2684380046*l + 2.6097574011*m - 0.3413193965*s,
		-0.0041960863*l - 0.7034186147*m + 1.7076147010*s
}

func to8(f float64) uint8 {
	v := linToSrgb(f)*255 + 0.5
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// FromOklch builds an sRGB color from lightness (0..1), chroma (≈0..0.37)
// and hue in degrees. Out-of-gamut colors are pulled in by reducing chroma,
// which keeps hue and lightness intact.
func FromOklch(L, C, hue float64) RGB {
	if L < 0 {
		L = 0
	}
	if L > 1 {
		L = 1
	}
	h := hue * math.Pi / 180
	ca, sa := math.Cos(h), math.Sin(h)
	const eps = 1e-4
	in := func(c float64) (float64, float64, float64, bool) {
		r, g, b := oklabToLinear(L, c*ca, c*sa)
		ok := r >= -eps && r <= 1+eps && g >= -eps && g <= 1+eps && b >= -eps && b <= 1+eps
		return r, g, b, ok
	}
	r, g, b, ok := in(C)
	if !ok {
		lo, hi := 0.0, C
		for i := 0; i < 16; i++ {
			mid := (lo + hi) / 2
			if _, _, _, ok := in(mid); ok {
				lo = mid
			} else {
				hi = mid
			}
		}
		r, g, b, _ = in(lo)
	}
	return RGB{to8(r), to8(g), to8(b)}
}

// Luma returns the Rec.601 luma of a color in 0..255.
func Luma(r, g, b uint8) uint8 {
	return uint8((uint32(r)*77 + uint32(g)*150 + uint32(b)*29 + 128) >> 8)
}
