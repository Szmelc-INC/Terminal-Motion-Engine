package engine

import (
	"strings"
	"testing"
)

func near(a, b RGB) bool {
	d := func(x, y uint8) int { return max(int(x), int(y)) - min(int(x), int(y)) }
	return d(a.R, b.R) <= 1 && d(a.G, b.G) <= 1 && d(a.B, b.B) <= 1
}

func TestBuiltinPalettes(t *testing.T) {
	if len(NamedPalettes) < 100 {
		t.Errorf("only %d built-in palettes", len(NamedPalettes))
	}
	reserved := map[string]bool{PalOff: true, PalGray: true, PalCube: true, PalAnsi16: true, PalXterm: true,
		PalHarmony: true, PalAdaptive: true, PalCustom: true}
	seen := map[string]bool{}
	var s Settings
	for _, p := range NamedPalettes {
		key := strings.ToLower(p.Name)
		if seen[key] || reserved[key] || p.Name == "" || p.Name != key || strings.ContainsAny(p.Name, " ,") {
			t.Errorf("palette name %q is taken, reserved or malformed", p.Name)
		}
		seen[key] = true
		if len(p.Colors) < 2 || len(p.Colors) > 256 {
			t.Errorf("%s has %d colors", p.Name, len(p.Colors))
		}
		colors := map[RGB]bool{}
		for _, c := range p.Colors {
			colors[c] = true
		}
		if len(colors) < 2 {
			t.Errorf("%s has fewer than two different colors", p.Name)
		}
		if PaletteTag(p.Name) == "" {
			t.Errorf("%s has no family", p.Name)
		}
		if err := FindOption("palette").Set(&s, p.Name); err != nil {
			t.Errorf("--palette %s: %v", p.Name, err)
		}
		if got, ok := FindPalette(p.Name); !ok || len(got) != len(p.Colors) {
			t.Errorf("%s cannot be looked up", p.Name)
		}
	}
	// Palettes built by blending must have been computed after the color
	// tables they depend on: the ends of a ramp are its first and last stop.
	for name, ends := range map[string][2]string{
		"thermal":   {"000004", "ffffe0"},
		"viridis":   {"440154", "fde725"},
		"blueprint": {"041733", "ffffff"},
	} {
		c, _ := FindPalette(name)
		lo, _ := ParseHex(ends[0])
		hi, _ := ParseHex(ends[1])
		if len(c) < 3 || !near(c[0], lo) || !near(c[len(c)-1], hi) {
			t.Errorf("%s runs %v … %v, want %v … %v", name, c[0], c[len(c)-1], lo, hi)
		}
		for i := 1; i < len(c); i++ {
			if c[i] == c[i-1] {
				t.Errorf("%s: steps %d and %d are the same color", name, i-1, i)
			}
		}
	}
	if c, _ := FindPalette("mastersystem"); len(c) != 64 {
		t.Errorf("mastersystem has %d colors, want 64", len(c))
	}
	if c, _ := FindPalette("rainbow12"); len(c) != 14 || c[0] != (RGB{}) || c[13] != (RGB{255, 255, 255}) {
		t.Errorf("rainbow12 = %v", c)
	}
	if PaletteTag("no-such-palette") != "" || len(PaletteFamilies()) < 6 {
		t.Error("palette families are wrong")
	}
}

// wide reports whether a terminal draws the glyph two cells wide, which
// would break the picture.
func wide(r rune) bool {
	return (r >= 0x1100 && r <= 0x115f) || (r >= 0x2e80 && r <= 0xa4cf) || (r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) || (r >= 0xfe30 && r <= 0xfe4f) || (r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) || r >= 0x1f000
}

func TestBuiltinCharsets(t *testing.T) {
	if len(Charsets) < 30 {
		t.Errorf("only %d built-in charsets", len(Charsets))
	}
	if last := Charsets[len(Charsets)-1]; last.Name != "custom" || last.Runes != nil {
		t.Fatalf("custom must be the last charset, got %q", last.Name)
	}
	seen := map[string]bool{}
	var s Settings
	for _, c := range Charsets[:len(Charsets)-1] {
		if seen[c.Name] || c.Name == "" || c.Name != strings.ToLower(c.Name) || strings.ContainsAny(c.Name, " ,") {
			t.Errorf("charset name %q is taken or malformed", c.Name)
		}
		seen[c.Name] = true
		if len(c.Runes) < 2 || c.Runes[0] != ' ' {
			t.Errorf("%s must have at least two glyphs and start with a space: %q", c.Name, string(c.Runes))
		}
		glyphs := map[rune]bool{}
		for _, r := range c.Runes {
			if glyphs[r] {
				t.Errorf("%s repeats %q", c.Name, r)
			}
			glyphs[r] = true
			if wide(r) || r < ' ' {
				t.Errorf("%s contains %q (U+%04X), which is not one cell wide", c.Name, r, r)
			}
		}
		if err := FindOption("charset").Set(&s, c.Name); err != nil {
			t.Errorf("--charset %s: %v", c.Name, err)
		}
		l := DefaultLook()
		l.Mode, l.Charset = ModeASCII, c.Name
		if got := l.Ramp(); len(got) != len(c.Runes) {
			t.Errorf("%s: the look uses %q", c.Name, string(got))
		}
	}
	// Ramps of plain ASCII and block glyphs have a measurable density, so
	// they can be checked to run from empty to full.
	for _, name := range []string{"blocks", "blocks-fine", "bars-v", "bars-h", "quadrants", "dots",
		"braille-fill", "numbers", "hex", "alphabet", "caps", "code"} {
		l := Look{Charset: name}
		r := l.Ramp()
		for i := 1; i < len(r); i++ {
			if Density(r[i]) < Density(r[i-1]) {
				t.Errorf("%s: %q is emptier than %q before it", name, r[i], r[i-1])
			}
		}
	}
}

func TestBuiltinPresets(t *testing.T) {
	if len(BuiltinPresets) < 40 {
		t.Errorf("only %d built-in looks", len(BuiltinPresets))
	}
	seen := map[string]bool{}
	for _, p := range BuiltinPresets {
		if seen[p.Name] || p.Name == "" || !p.Builtin {
			t.Errorf("look %q: duplicate, unnamed or not marked built-in", p.Name)
		}
		seen[p.Name] = true
		inRange(t, p.Name, p.Look)
		l := p.Look
		if l.Charset == "custom" || l.Palette == PalCustom {
			t.Errorf("%s relies on custom glyphs or colors", p.Name)
		}
		film(t, l, 16, 6, 3)
	}
	if BuiltinPresets[0].Name != "default" {
		t.Error(`the first look must be "default"`)
	}
}
