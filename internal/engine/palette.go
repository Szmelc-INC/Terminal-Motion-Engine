package engine

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
)

// Palette is a fixed set of colors with a precomputed nearest-color table.
// Lookups go through a 5-bit-per-channel LUT so matching a pixel is a single
// array read. "Nearest" compares hue and chroma in OKLab, so colors snap to
// the palette entry that looks closest, but compares brightness as
// gamma-encoded luma: that is the scale dither thresholds and diffused error
// are added on, and using the same scale for both is what keeps a dithered
// picture from coming out brighter or darker than the original.
type Palette struct {
	Colors []RGB
	lut    []uint8
	step   int
}

// spread is the typical distance between neighbouring palette colors; it
// is how far an ordered-dither threshold needs to push a pixel.
func (p *Palette) spread() int { return p.step }

const lutSize = 1 << 15

var (
	bucketOnce sync.Once
	bucketLab  [][3]float32
)

// matchSpace maps a color to the space palette distances are measured in.
func matchSpace(c RGB) [3]float32 {
	_, a, b := ToOklab(c)
	y := (0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)) / 255
	return [3]float32{float32(y), float32(a), float32(b)}
}

func bucketLabs() [][3]float32 {
	bucketOnce.Do(func() {
		bucketLab = make([][3]float32, lutSize)
		for i := range bucketLab {
			r := uint8(i>>10&31)<<3 | 4
			g := uint8(i>>5&31)<<3 | 4
			b := uint8(i&31)<<3 | 4
			bucketLab[i] = matchSpace(RGB{r, g, b})
		}
	})
	return bucketLab
}

// NewPalette builds a palette (at most 256 colors) and its lookup table.
func NewPalette(colors []RGB) *Palette {
	if len(colors) > 256 {
		colors = colors[:256]
	}
	if len(colors) == 0 {
		colors = []RGB{{0, 0, 0}, {255, 255, 255}}
	}
	p := &Palette{Colors: append([]RGB(nil), colors...), lut: make([]uint8, lutSize)}
	labs := make([][3]float32, len(p.Colors))
	for i, c := range p.Colors {
		labs[i] = matchSpace(c)
	}
	var sum float64
	for i, a := range p.Colors {
		best := math.MaxFloat64
		for j, b := range p.Colors {
			if i == j {
				continue
			}
			dr, dg, db := float64(a.R)-float64(b.R), float64(a.G)-float64(b.G), float64(a.B)-float64(b.B)
			best = math.Min(best, math.Sqrt(dr*dr+dg*dg+db*db))
		}
		if len(p.Colors) > 1 {
			sum += best
		}
	}
	step := math.Max(12, math.Min(255, sum/float64(len(p.Colors))))
	buckets := bucketLabs()
	parallel(lutSize, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			q := buckets[i]
			best, bestD := 0, float32(math.MaxFloat32)
			for j, c := range labs {
				dl, da, db := q[0]-c[0], q[1]-c[1], q[2]-c[2]
				if d := dl*dl + da*da + db*db; d < bestD {
					best, bestD = j, d
				}
			}
			p.lut[i] = uint8(best)
		}
	})
	if len(p.Colors) <= 32 {
		// With few colors the threshold must not be able to push the
		// darkest or lightest color over to its neighbour, or solid
		// black and white areas come out speckled.
		lo, hi := 0, 0
		for i, c := range p.Colors {
			if Luma(c.R, c.G, c.B) < Luma(p.Colors[lo].R, p.Colors[lo].G, p.Colors[lo].B) {
				lo = i
			}
			if Luma(c.R, c.G, c.B) > Luma(p.Colors[hi].R, p.Colors[hi].G, p.Colors[hi].B) {
				hi = i
			}
		}
		reach := func(idx, dir int) float64 {
			c := p.Colors[idx]
			for t := 1; t < 256; t++ {
				d := t * dir
				if int(p.Index(clamp8(int(c.R)+d), clamp8(int(c.G)+d), clamp8(int(c.B)+d))) != idx {
					return float64(t)
				}
			}
			return 255
		}
		step = math.Min(step, 2*math.Min(reach(lo, 1), reach(hi, -1)))
	}
	p.step = int(math.Max(4, step))
	return p
}

// Index returns the palette index nearest to the given color.
func (p *Palette) Index(r, g, b uint8) uint8 {
	return p.lut[uint32(r>>3)<<10|uint32(g>>3)<<5|uint32(b>>3)]
}

// Nearest returns the palette color nearest to the given color.
func (p *Palette) Nearest(r, g, b uint8) RGB { return p.Colors[p.Index(r, g, b)] }

func hexes(s string) []RGB {
	f := strings.Fields(s)
	out := make([]RGB, len(f))
	for i, h := range f {
		c, err := ParseHex(h)
		if err != nil {
			panic(err)
		}
		out[i] = c
	}
	return out
}

// NamedPalette is a hand-picked palette that ships with the binary.
type NamedPalette struct {
	Name   string
	Colors []RGB
}

// NamedPalettes lists the built-in palettes in menu order.
var NamedPalettes = []NamedPalette{
	{"1bit", hexes("000000 ffffff")},
	{"paper", hexes("f5f0e1 1c1c1c")},
	{"gameboy", hexes("0f380f 306230 8bac0f 9bbc0f")},
	{"cga", hexes("000000 55ffff ff55ff ffffff")},
	{"cga-hot", hexes("000000 55ff55 ff5555 ffff55")},
	{"amber", hexes("000000 331a00 663300 995200 cc7a00 ffb000 ffd27f")},
	{"matrix", hexes("000000 003b00 008f11 00ff41 afffaf")},
	{"sepia", hexes("1a1208 3d2b17 6b4c2a a07a4a d1ab78 f1dcb3")},
	{"ice", hexes("03045e 023e8a 0077b6 0096c7 00b4d8 48cae4 90e0ef caf0f8")},
	{"sunset", hexes("1b1031 4b1d52 8c2f5b c94f4f f08a4b f9c74f fff3b0")},
	{"vaporwave", hexes("1a1033 2d1b69 7b2cbf ff71ce 01cdfe 05ffa1 b967ff fffb96")},
	{"cyberpunk", hexes("0d0221 261447 541388 d100d1 f706cf 00f0ff fcee0c ffffff")},
	{"pico8", hexes("000000 1d2b53 7e2553 008751 ab5236 5f574f c2c3c7 fff1e8 ff004d ffa300 ffec27 00e436 29adff 83769c ff77a8 ffccaa")},
	{"c64", hexes("000000 ffffff 880000 aaffee cc44cc 00cc55 0000aa eeee77 dd8855 664400 ff7777 333333 777777 aaff66 0088ff bbbbbb")},
	{"zx", hexes("000000 0000d7 d70000 d700d7 00d700 00d7d7 d7d700 d7d7d7 0000ff ff0000 ff00ff 00ff00 00ffff ffff00 ffffff")},
	{"apple2", hexes("000000 6c2940 403578 d93cf0 135740 808080 2697f0 bfb4f8 404b07 d9680f eca8bf 26c30f bfca87 93d6bf ffffff")},
	{"nord", hexes("2e3440 3b4252 434c5e 4c566a d8dee9 e5e9f0 eceff4 8fbcbb 88c0d0 81a1c1 5e81ac bf616a d08770 ebcb8b a3be8c b48ead")},
	{"gruvbox", hexes("282828 cc241d 98971a d79921 458588 b16286 689d6a a89984 928374 fb4934 b8bb26 fabd2f 83a598 d3869b 8ec07c ebdbb2")},
	{"dracula", hexes("282a36 44475a f8f8f2 6272a4 8be9fd 50fa7b ffb86c ff79c6 bd93f9 ff5555 f1fa8c")},
	{"solarized", hexes("002b36 073642 586e75 657b83 839496 93a1a1 eee8d5 fdf6e3 b58900 cb4b16 dc322f d33682 6c71c4 268bd2 2aa198 859900")},
	{"catppuccin", hexes("1e1e2e 313244 45475a 585b70 cdd6f4 f5e0dc f5c2e7 cba6f7 f38ba8 fab387 f9e2af a6e3a1 94e2d5 89dceb 89b4fa b4befe")},
}

// Palette kinds that are generated rather than looked up by name.
const (
	PalOff      = "truecolor"
	PalGray     = "gray"
	PalCube     = "cube"
	PalAnsi16   = "ansi16"
	PalXterm    = "xterm256"
	PalHarmony  = "harmony"
	PalAdaptive = "adaptive"
	PalCustom   = "custom"
)

// PaletteChoices lists every value the palette option accepts.
func PaletteChoices() []string {
	out := []string{PalOff, PalHarmony, PalAdaptive, PalGray, PalCube, PalAnsi16, PalXterm, PalCustom}
	for _, n := range AllPalettes() {
		out = append(out, n.Name)
	}
	return out
}

// Schemes are the color-theory generators available to the harmony palette.
var Schemes = []string{
	"monochromatic", "analogous", "complementary", "split-complementary",
	"triadic", "tetradic", "square", "hexadic", "hue-shift", "golden", "rainbow",
}

func schemeHues(scheme string, n int) []float64 {
	switch scheme {
	case "analogous":
		return []float64{-30, 0, 30}
	case "complementary":
		return []float64{0, 180}
	case "split-complementary":
		return []float64{0, 150, 210}
	case "triadic":
		return []float64{0, 120, 240}
	case "tetradic":
		return []float64{0, 60, 180, 240}
	case "square":
		return []float64{0, 90, 180, 270}
	case "hexadic":
		return []float64{0, 60, 120, 180, 240, 300}
	case "golden":
		out := make([]float64, n)
		for i := range out {
			out[i] = math.Mod(float64(i)*137.50776, 360)
		}
		return out
	case "rainbow":
		out := make([]float64, n)
		for i := range out {
			out[i] = float64(i) * 360 / float64(n)
		}
		return out
	}
	return []float64{0}
}

// Harmony generates n colors in OKLCH. Lightness runs from lmin to lmax so
// the palette always spans dark to light (which is what keeps an image
// readable), while hues are dealt out according to the chosen scheme.
func Harmony(scheme string, hue, chroma, lmin, lmax float64, n int) []RGB {
	if n < 1 {
		n = 1
	}
	if lmin > lmax {
		lmin, lmax = lmax, lmin
	}
	hues := schemeHues(scheme, n)
	out := make([]RGB, n)
	for i := range out {
		t := 0.5
		if n > 1 {
			t = float64(i) / float64(n-1)
		}
		h := hue + hues[i%len(hues)]
		if scheme == "hue-shift" {
			// Classic pixel-art ramp: shadows drift cool, highlights warm.
			h = hue + (t-0.5)*120
		}
		// Chroma peaks in the mid-tones; near black and white there is
		// little room for it inside the sRGB gamut anyway.
		c := chroma * (0.35 + 0.65*math.Sin(math.Pi*t))
		if n == 1 {
			c = chroma
		}
		L := lmin + (lmax-lmin)*t
		if n == 1 {
			L = math.Max(lmax*0.8, lmin)
		}
		out[i] = FromOklch(L, c, h)
	}
	return out
}

// GrayRamp returns n evenly spaced greys from black to white.
func GrayRamp(n int) []RGB {
	if n < 2 {
		return []RGB{{255, 255, 255}}
	}
	out := make([]RGB, n)
	for i := range out {
		v := uint8(i * 255 / (n - 1))
		out[i] = RGB{v, v, v}
	}
	return out
}

// Cube returns a uniform RGB cube with at most n colors. The split favours
// green, then red, then blue, matching the eye's sensitivity.
func Cube(n int) []RGB {
	if n < 2 {
		n = 2
	}
	lv := [3]int{1, 1, 1} // r, g, b
	order := [3]int{1, 0, 2}
	for {
		grew := false
		for _, ch := range order {
			try := lv
			try[ch]++
			if try[0]*try[1]*try[2] <= n {
				lv = try
				grew = true
			}
		}
		if !grew {
			break
		}
	}
	level := func(i, n int) uint8 {
		if n == 1 {
			return 128
		}
		return uint8(i * 255 / (n - 1))
	}
	var out []RGB
	for r := 0; r < lv[0]; r++ {
		for g := 0; g < lv[1]; g++ {
			for b := 0; b < lv[2]; b++ {
				out = append(out, RGB{level(r, lv[0]), level(g, lv[1]), level(b, lv[2])})
			}
		}
	}
	return out
}

// Ansi16 returns the standard xterm 16-color palette.
func Ansi16() []RGB {
	return hexes("000000 cd0000 00cd00 cdcd00 0000ee cd00cd 00cdcd e5e5e5 7f7f7f ff0000 00ff00 ffff00 5c5cff ff00ff 00ffff ffffff")
}

// Xterm256 returns the xterm 256-color palette.
func Xterm256() []RGB {
	out := Ansi16()
	steps := [6]uint8{0, 95, 135, 175, 215, 255}
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				out = append(out, RGB{steps[r], steps[g], steps[b]})
			}
		}
	}
	for i := 0; i < 24; i++ {
		v := uint8(8 + 10*i)
		out = append(out, RGB{v, v, v})
	}
	return out
}

var (
	xtermOnce sync.Once
	xtermPal  *Palette
	ansiOnce  sync.Once
	ansiPal   *Palette
)

// XtermPalette returns the shared xterm-256 palette (with LUT).
func XtermPalette() *Palette {
	xtermOnce.Do(func() { xtermPal = NewPalette(Xterm256()) })
	return xtermPal
}

// AnsiPalette returns the shared 16-color palette (with LUT).
func AnsiPalette() *Palette {
	ansiOnce.Do(func() { ansiPal = NewPalette(Ansi16()) })
	return ansiPal
}

// MedianCut derives an n-color palette from RGB24 pixel data.
func MedianCut(pix []byte, n int) []RGB {
	if n < 1 {
		n = 1
	}
	total := len(pix) / 3
	if total == 0 {
		return GrayRamp(n)
	}
	step := total/20000 + 1
	pts := make([][3]uint8, 0, total/step+1)
	for i := 0; i < total; i += step {
		pts = append(pts, [3]uint8{pix[i*3], pix[i*3+1], pix[i*3+2]})
	}
	type box struct{ lo, hi int }
	span := func(b box) (ch int, width int) {
		var mn, mx [3]uint8
		mn = [3]uint8{255, 255, 255}
		for _, p := range pts[b.lo:b.hi] {
			for c := 0; c < 3; c++ {
				if p[c] < mn[c] {
					mn[c] = p[c]
				}
				if p[c] > mx[c] {
					mx[c] = p[c]
				}
			}
		}
		// Weight channels roughly by perceptual importance.
		w := [3]int{3, 4, 2}
		for c := 0; c < 3; c++ {
			if d := (int(mx[c]) - int(mn[c])) * w[c]; d > width {
				ch, width = c, d
			}
		}
		return
	}
	boxes := []box{{0, len(pts)}}
	for len(boxes) < n {
		best, bestScore, bestCh := -1, 0, 0
		for i, b := range boxes {
			if b.hi-b.lo < 2 {
				continue
			}
			ch, w := span(b)
			if score := w * (b.hi - b.lo); score > bestScore {
				best, bestScore, bestCh = i, score, ch
			}
		}
		if best < 0 {
			break
		}
		b := boxes[best]
		s := pts[b.lo:b.hi]
		sort.Slice(s, func(i, j int) bool { return s[i][bestCh] < s[j][bestCh] })
		mid := b.lo + (b.hi-b.lo)/2
		boxes[best] = box{b.lo, mid}
		boxes = append(boxes, box{mid, b.hi})
	}
	out := make([]RGB, 0, len(boxes))
	for _, b := range boxes {
		var r, g, bl, cnt int
		for _, p := range pts[b.lo:b.hi] {
			r += int(p[0])
			g += int(p[1])
			bl += int(p[2])
			cnt++
		}
		if cnt == 0 {
			continue
		}
		out = append(out, RGB{uint8(r / cnt), uint8(g / cnt), uint8(bl / cnt)})
	}
	sort.Slice(out, func(i, j int) bool {
		return Luma(out[i].R, out[i].G, out[i].B) < Luma(out[j].R, out[j].G, out[j].B)
	})
	return out
}

// PaletteColors resolves the palette described by the look. sample is RGB24
// data used only by the adaptive palette. A nil result means "no palette".
func PaletteColors(l *Look, sample []byte) []RGB {
	n := l.Colors
	if n < 1 {
		n = 1
	}
	if n > 256 {
		n = 256
	}
	switch l.Palette {
	case PalOff, "":
		return nil
	case PalGray:
		return GrayRamp(n)
	case PalCube:
		return Cube(n)
	case PalAnsi16:
		return Ansi16()
	case PalXterm:
		return Xterm256()
	case PalHarmony:
		return Harmony(l.Scheme, l.Hue, l.Chroma, l.LMin, l.LMax, n)
	case PalAdaptive:
		return MedianCut(sample, n)
	case PalCustom:
		var out []RGB
		for _, h := range l.Custom {
			if c, err := ParseHex(h); err == nil {
				out = append(out, c)
			}
		}
		if len(out) == 0 {
			return GrayRamp(2)
		}
		return out
	}
	if c, ok := FindPalette(l.Palette); ok && len(c) > 0 {
		return c
	}
	return nil
}

// paletteKey identifies the palette a look resolves to, so the renderer can
// tell when its cached LUT is stale.
func paletteKey(l *Look) string {
	switch l.Palette {
	case PalGray, PalCube, PalAdaptive:
		return fmt.Sprintf("%s/%d/%d", l.Palette, l.Colors, l.Resample)
	case PalHarmony:
		return fmt.Sprintf("h/%s/%.1f/%.3f/%.3f/%.3f/%d", l.Scheme, l.Hue, l.Chroma, l.LMin, l.LMax, l.Colors)
	case PalCustom:
		return "c/" + strings.Join(l.Custom, ",")
	}
	return fmt.Sprintf("%s/%d", l.Palette, libGen)
}

// RandomHarmony fills in the harmony fields of a look with a random but
// usable color scheme.
func RandomHarmony(l *Look, rng *rand.Rand) {
	l.Palette = PalHarmony
	l.Scheme = Schemes[rng.Intn(len(Schemes))]
	l.Hue = float64(rng.Intn(360))
	l.Chroma = 0.06 + rng.Float64()*0.22
	l.LMin = 0.05 + rng.Float64()*0.2
	l.LMax = 0.8 + rng.Float64()*0.2
	counts := []int{2, 2, 3, 4, 4, 5, 6, 8, 8, 12, 16, 24, 32, 64}
	l.Colors = counts[rng.Intn(len(counts))]
}
