package engine

import (
	"math/rand"
	"testing"
)

func TestParseHex(t *testing.T) {
	for in, want := range map[string]RGB{
		"#ff8800": {255, 136, 0},
		"FF8800":  {255, 136, 0},
		"#f80":    {255, 136, 0},
		" #000 ":  {0, 0, 0},
	} {
		got, err := ParseHex(in)
		if err != nil || got != want {
			t.Errorf("ParseHex(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "#12", "#gggggg", "1234567"} {
		if _, err := ParseHex(bad); err == nil {
			t.Errorf("ParseHex(%q) should fail", bad)
		}
	}
	if h := (RGB{1, 2, 255}).Hex(); h != "#0102ff" {
		t.Errorf("Hex = %q", h)
	}
}

func TestOklabRoundTrip(t *testing.T) {
	for _, c := range []RGB{{0, 0, 0}, {255, 255, 255}, {255, 0, 0}, {12, 200, 90}, {128, 128, 128}} {
		L, a, b := ToOklab(c)
		r, g, bl := oklabToLinear(L, a, b)
		got := RGB{to8(r), to8(g), to8(bl)}
		if got != c {
			t.Errorf("round trip %v -> %v", c, got)
		}
	}
	if L, _, _ := ToOklab(RGB{255, 255, 255}); L < 0.999 || L > 1.001 {
		t.Errorf("white lightness = %v, want 1", L)
	}
}

func TestPaletteNearest(t *testing.T) {
	p := NewPalette(hexes("000000 ffffff ff0000 00ff00 0000ff"))
	cases := map[RGB]RGB{
		{10, 10, 10}:    {0, 0, 0},
		{250, 250, 250}: {255, 255, 255},
		{200, 30, 20}:   {255, 0, 0},
		{20, 220, 30}:   {0, 255, 0},
		{30, 20, 230}:   {0, 0, 255},
	}
	for in, want := range cases {
		if got := p.Nearest(in.R, in.G, in.B); got != want {
			t.Errorf("Nearest(%v) = %v, want %v", in, got, want)
		}
	}
	// Every palette color must map to itself.
	for _, np := range NamedPalettes {
		pal := NewPalette(np.Colors)
		for _, c := range np.Colors {
			got := pal.Nearest(c.R, c.G, c.B)
			dr, dg, db := int(got.R)-int(c.R), int(got.G)-int(c.G), int(got.B)-int(c.B)
			if dr*dr+dg*dg+db*db > 3*24*24 {
				t.Errorf("%s: %v maps to distant %v", np.Name, c, got)
			}
		}
	}
}

func TestGenerators(t *testing.T) {
	for _, n := range []int{1, 2, 3, 7, 16, 64, 256} {
		for _, sc := range Schemes {
			cols := Harmony(sc, 210, 0.15, 0.1, 0.95, n)
			if len(cols) != n {
				t.Fatalf("Harmony(%s, %d) returned %d colors", sc, n, len(cols))
			}
			if n > 1 {
				lo := Luma(cols[0].R, cols[0].G, cols[0].B)
				hi := Luma(cols[n-1].R, cols[n-1].G, cols[n-1].B)
				if lo >= hi {
					t.Errorf("Harmony(%s, %d): not dark→light (%d..%d)", sc, n, lo, hi)
				}
			}
		}
		if got := len(GrayRamp(n)); got != n {
			t.Errorf("GrayRamp(%d) = %d colors", n, got)
		}
		if got := len(Cube(n)); got > max(n, 2) || got < 1 {
			t.Errorf("Cube(%d) = %d colors", n, got)
		}
	}
	if got := len(Cube(8)); got != 8 {
		t.Errorf("Cube(8) = %d colors, want 8", got)
	}
	if got := len(Xterm256()); got != 256 {
		t.Errorf("Xterm256 = %d colors", got)
	}
	// Different schemes must give different hues.
	a := Harmony("complementary", 30, 0.2, 0.3, 0.8, 4)
	b := Harmony("monochromatic", 30, 0.2, 0.3, 0.8, 4)
	if a[1] == b[1] {
		t.Error("complementary and monochromatic produced the same color")
	}
}

func TestMedianCut(t *testing.T) {
	pix := make([]byte, 0, 3000)
	for i := 0; i < 500; i++ {
		pix = append(pix, 250, 10, 10)
		pix = append(pix, 10, 10, 250)
	}
	cols := MedianCut(pix, 2)
	if len(cols) != 2 {
		t.Fatalf("got %d colors", len(cols))
	}
	seenRed, seenBlue := false, false
	for _, c := range cols {
		seenRed = seenRed || (c.R > 200 && c.B < 50)
		seenBlue = seenBlue || (c.B > 200 && c.R < 50)
	}
	if !seenRed || !seenBlue {
		t.Errorf("MedianCut = %v, want red and blue", cols)
	}
	if got := MedianCut(nil, 4); len(got) != 4 {
		t.Errorf("MedianCut(nil) = %v", got)
	}
}

func TestMatrices(t *testing.T) {
	for _, name := range orderedNames {
		m := matrixFor(name)
		if m == nil || len(m.t) != m.n*m.n {
			t.Fatalf("%s: bad matrix", name)
		}
		sum := 0
		for _, v := range m.t {
			if v < -128 || v > 127 {
				t.Fatalf("%s: threshold %d out of range", name, v)
			}
			sum += int(v)
		}
		if avg := sum / len(m.t); avg < -3 || avg > 3 {
			t.Errorf("%s: thresholds are biased (mean %d)", name, avg)
		}
	}
	b := bayer(2)
	if b.t[0] >= b.t[3] || b.t[3] >= b.t[1] || b.t[1] >= b.t[2] {
		t.Errorf("bayer2 order wrong: %v", b.t)
	}
}

// A flat mid-grey dithered to black and white must come out about half on,
// whichever algorithm is used.
func TestDitherPreservesBrightness(t *testing.T) {
	const w, h = 64, 64
	pal := NewPalette(GrayRamp(2))
	for _, name := range DitherNames() {
		if name == "none" {
			continue
		}
		l := DefaultLook()
		l.Dither = name
		var d ditherer
		pix := make([]byte, w*h*3)
		for i := range pix {
			pix[i] = 128
		}
		d.rgb(pix, w, h, pal, &l, 0)
		on := 0
		for i := 0; i < len(pix); i += 3 {
			if pix[i] != 0 && pix[i] != 255 {
				t.Fatalf("%s: produced non-palette value %d", name, pix[i])
			}
			if pix[i] == 255 {
				on++
			}
		}
		if frac := float64(on) / (w * h); frac < 0.4 || frac > 0.6 {
			t.Errorf("%s (rgb): %.2f of pixels on, want ≈0.5", name, frac)
		}

		lum := make([]uint8, w*h)
		for i := range lum {
			lum[i] = 128
		}
		out := make([]uint8, w*h)
		d.gray(lum, w, h, 2, &l, 0, out)
		on = 0
		for _, v := range out {
			on += int(v)
		}
		if frac := float64(on) / (w * h); frac < 0.4 || frac > 0.6 {
			t.Errorf("%s (gray): %.2f of pixels on, want ≈0.5", name, frac)
		}
	}
}

func TestGlyphTables(t *testing.T) {
	if len(quadGlyphs) != 16 || len(sextantGlyphs) != 64 || len(brailleGlyphs) != 256 {
		t.Fatal("glyph table sizes wrong")
	}
	if sextantGlyphs[1] != 0x1FB00 || sextantGlyphs[62] != 0x1FB3B || sextantGlyphs[22] != 0x1FB14 {
		t.Errorf("sextant mapping wrong: %U %U %U", sextantGlyphs[1], sextantGlyphs[62], sextantGlyphs[22])
	}
	if brailleGlyphs[255] != '⣿' || brailleGlyphs[1] != '⠁' || brailleGlyphs[2] != '⠈' {
		t.Errorf("braille mapping wrong")
	}
	seen := map[rune]bool{}
	for _, r := range sextantGlyphs {
		if seen[r] {
			t.Errorf("duplicate sextant glyph %U", r)
		}
		seen[r] = true
	}
}

func solid(w, h int, c RGB) []byte {
	pix := make([]byte, w*h*3)
	for i := 0; i < len(pix); i += 3 {
		pix[i], pix[i+1], pix[i+2] = c.R, c.G, c.B
	}
	return pix
}

func TestRenderModes(t *testing.T) {
	const cols, rows = 8, 4
	for _, mode := range Modes {
		sx, sy := SubCells(mode)
		w, h := cols*sx, rows*sy
		// Top half white, bottom half black.
		pix := solid(w, h, RGB{})
		for i := 0; i < w*(h/2)*3; i++ {
			pix[i] = 255
		}
		for _, color := range []bool{true, false} {
			l := DefaultLook()
			l.Mode, l.Color, l.Dither = mode, color, "none"
			var r Renderer
			r.TermDepth = 24
			cells := make([]Cell, cols*rows)
			r.Render(pix, w, h, &l, cells, cols, rows, 0)
			top, bot := cells[0], cells[(rows-1)*cols]
			if top.Ch == 0 || bot.Ch == 0 {
				t.Fatalf("%s color=%v: cells not written", mode, color)
			}
			if top == bot {
				t.Errorf("%s color=%v: white and black rows render identically (%+v)", mode, color, top)
			}
			if !color && (top.Fg != ColDefault || top.Bg != ColDefault) {
				t.Errorf("%s mono: emitted colors %x/%x", mode, top.Fg, top.Bg)
			}
		}
	}
}

func TestRenderHalfExact(t *testing.T) {
	// 1×1 cell: red over blue.
	pix := []byte{255, 0, 0, 0, 0, 255}
	l := DefaultLook()
	var r Renderer
	r.TermDepth = 24
	cells := make([]Cell, 1)
	r.Render(pix, 1, 2, &l, cells, 1, 1, 0)
	if cells[0] != (Cell{Ch: '▀', Fg: 0xff0000, Bg: 0x0000ff}) {
		t.Errorf("got %+v", cells[0])
	}
	l.FlipY = true
	r.Render(pix, 1, 2, &l, cells, 1, 1, 0)
	if cells[0].Fg != 0x0000ff || cells[0].Bg != 0xff0000 {
		t.Errorf("flip-y: got %+v", cells[0])
	}
	l.FlipY, l.Invert = false, true
	r.Render(pix, 1, 2, &l, cells, 1, 1, 0)
	if cells[0].Fg != 0x00ffff {
		t.Errorf("invert: got fg %06x", cells[0].Fg)
	}
}

func TestRenderPaletteOnly(t *testing.T) {
	const cols, rows = 16, 8
	rng := rand.New(rand.NewSource(1))
	for _, mode := range Modes {
		sx, sy := SubCells(mode)
		w, h := cols*sx, rows*sy
		pix := make([]byte, w*h*3)
		rng.Read(pix)
		for _, d := range []string{"none", "bayer4", "bluenoise", "floyd-steinberg", "atkinson"} {
			l := DefaultLook()
			l.Mode, l.Palette, l.Dither, l.Background = mode, "gameboy", d, "tint"
			var r Renderer
			r.TermDepth = 24
			cells := make([]Cell, cols*rows)
			r.Render(pix, w, h, &l, cells, cols, rows, 0)
			allowed := map[uint32]bool{}
			for _, c := range r.Palette().Colors {
				allowed[c.U32()] = true
			}
			for _, c := range cells {
				if c.Ch != ' ' && !allowed[c.Fg] {
					t.Fatalf("%s/%s: fg %06x not in palette", mode, d, c.Fg)
				}
				if c.Bg != ColDefault && !allowed[c.Bg] {
					t.Fatalf("%s/%s: bg %06x not in palette", mode, d, c.Bg)
				}
			}
		}
	}
}

func TestSingleColorPaletteStaysVisible(t *testing.T) {
	l := DefaultLook()
	l.Palette, l.Colors = PalHarmony, 1
	pix := solid(4, 4, RGB{})
	for i := 0; i < 4*2*3; i++ {
		pix[i] = 255
	}
	var r Renderer
	r.TermDepth = 24
	cells := make([]Cell, 8)
	r.Render(pix, 4, 4, &l, cells, 4, 2, 0)
	if cells[0].Bg == cells[4].Bg {
		t.Error("a 1-color palette rendered a flat picture in block mode")
	}
}

func TestOptions(t *testing.T) {
	seen := map[string]bool{}
	s := DefaultSettings()
	for _, o := range Options {
		if seen[o.Key] {
			t.Errorf("duplicate option key %q", o.Key)
		}
		seen[o.Key] = true
		before := o.String(&s)
		if o.Kind != KText && o.Kind != KList {
			// Step away and back. A value that starts at the top of its
			// range (a filter that is off at 20 kHz, a mix at 100 %) can
			// only step down first.
			dir := 1
			if (o.Kind == KFloat || o.Kind == KInt) && !o.Wrap && o.Frac(&s) >= 1 {
				dir = -1
			}
			o.Nudge(&s, dir)
			o.Nudge(&s, -dir)
			if o.Kind != KBool && o.String(&s) != before {
				t.Errorf("%s: nudge +1/-1 changed %q to %q", o.Key, before, o.String(&s))
			}
			if o.Kind == KBool {
				o.Nudge(&s, 1) // back to the original
			}
		}
		if err := o.Set(&s, before); err != nil {
			t.Errorf("%s: cannot set its own value %q: %v", o.Key, before, err)
		}
	}
	if err := FindOption("mode").Set(&s, "nope"); err == nil {
		t.Error("bad enum accepted")
	}
	if err := FindOption("contrast").Set(&s, "99"); err == nil {
		t.Error("out-of-range number accepted")
	}
	if err := FindOption("custom").Set(&s, "#fff, 000;#12345g"); err == nil {
		t.Error("bad color list accepted")
	}
	if err := FindOption("custom").Set(&s, "#fff, 000"); err != nil || len(s.Custom) != 2 || s.Custom[0] != "#ffffff" {
		t.Errorf("custom list = %v, %v", s.Custom, err)
	}
	hue := FindOption("hue")
	s.Hue = 355
	hue.Nudge(&s, 2)
	if s.Hue != 5 {
		t.Errorf("hue should wrap, got %v", s.Hue)
	}
	col := FindOption("colors")
	col.SetFrac(&s, 0)
	if s.Colors != 1 {
		t.Errorf("colors at frac 0 = %d", s.Colors)
	}
	col.SetFrac(&s, 1)
	if s.Colors != 256 {
		t.Errorf("colors at frac 1 = %d", s.Colors)
	}
}

func TestRandomizeAlwaysValid(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	const cols, rows = 6, 3
	for i := 0; i < 300; i++ {
		l := DefaultLook()
		l.Fit, l.FlipX = "fill", true
		Randomize(&l, rng)
		if l.Fit != "fill" || !l.FlipX {
			t.Fatal("randomize must not touch geometry")
		}
		s := DefaultSettings()
		s.Look = l
		for _, o := range Options {
			if o.Group == "Playback" || o.Kind == KText || o.Kind == KList {
				continue
			}
			if err := o.Set(&s, o.String(&s)); err != nil {
				t.Fatalf("randomized look has invalid %s: %v", o.Key, err)
			}
		}
		sx, sy := SubCells(l.Mode)
		w, h := cols*sx, rows*sy
		pix := make([]byte, w*h*3)
		rng.Read(pix)
		var r Renderer
		r.TermDepth = 24
		cells := make([]Cell, cols*rows)
		r.Render(pix, w, h, &l, cells, cols, rows, i)
		for _, c := range cells {
			if c.Ch == 0 {
				t.Fatalf("look %+v left cells unwritten", l)
			}
		}
	}
}

func BenchmarkRender(b *testing.B) {
	const cols, rows = 240, 67
	rng := rand.New(rand.NewSource(1))
	for _, cfg := range []struct{ mode, pal, dither string }{
		{ModeHalf, PalOff, "none"},
		{ModeHalf, "pico8", "bayer4"},
		{ModeHalf, "pico8", "floyd-steinberg"},
		{ModeSextant, PalXterm, "bluenoise"},
		{ModeBraille, PalOff, "bluenoise"},
		{ModeASCII, PalOff, "none"},
	} {
		b.Run(cfg.mode+"/"+cfg.pal+"/"+cfg.dither, func(b *testing.B) {
			l := DefaultLook()
			l.Mode, l.Palette, l.Dither = cfg.mode, cfg.pal, cfg.dither
			sx, sy := SubCells(l.Mode)
			w, h := cols*sx, rows*sy
			pix := make([]byte, w*h*3)
			rng.Read(pix)
			var r Renderer
			r.TermDepth = 24
			cells := make([]Cell, cols*rows)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.Render(pix, w, h, &l, cells, cols, rows, i)
			}
		})
	}
}
