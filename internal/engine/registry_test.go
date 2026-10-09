package engine

import (
	"math/rand"
	"strings"
	"testing"
)

func TestParsePaletteText(t *testing.T) {
	cases := map[string]int{
		"1a1c2c\n5d275d\nb13e53\n":  3, // .hex
		"#ff0000, #00ff00;0x0000ff": 3,
		"GIMP Palette\nName: x\nColumns: 4\n#\n255 0 0\tred\n  0 255   0 green\n": 2,
		`["#112233", "#445566"]`:                             2,
		`{"name":"p","colors":["aabbcc","ddeeff","001122"]}`: 3,
	}
	for in, want := range cases {
		got, err := ParsePaletteText(in)
		if err != nil || len(got) != want {
			t.Errorf("%q -> %d colors (%v), want %d", in, len(got), err, want)
		}
	}
	if got, _ := ParsePaletteText("GIMP Palette\n#\n255 0 0\n"); len(got) != 1 || got[0] != (RGB{255, 0, 0}) {
		t.Errorf("gpl color: %v", got)
	}
	if _, err := ParsePaletteText("no colors here"); err == nil {
		t.Error("text without colors should be an error")
	}
}

func TestGradient(t *testing.T) {
	g := Gradient([]RGB{{0, 0, 0}, {255, 255, 255}}, 5)
	if len(g) != 5 || g[0] != (RGB{0, 0, 0}) || g[4] != (RGB{255, 255, 255}) {
		t.Fatalf("gradient ends: %v", g)
	}
	for i := 1; i < len(g); i++ {
		if Luma(g[i].R, g[i].G, g[i].B) <= Luma(g[i-1].R, g[i-1].G, g[i-1].B) {
			t.Errorf("gradient is not monotonic at %d: %v", i, g)
		}
	}
	if g := Gradient([]RGB{{10, 20, 30}}, 3); len(g) != 3 || g[1] != (RGB{10, 20, 30}) {
		t.Errorf("single stop: %v", g)
	}
	if g := Gradient([]RGB{{255, 0, 0}, {0, 255, 0}, {0, 0, 255}}, 3); g[1] != (RGB{0, 255, 0}) {
		t.Errorf("three stops, middle: %v", g)
	}
}

func TestDensityOrder(t *testing.T) {
	// The well-known ramp must come out in its usual order.
	if got := SortByDensity("@#*+=-:. "); got != " .-:=+*#@" {
		t.Errorf("SortByDensity = %q", got)
	}
	if got := SortByDensity("█ ▓░▒"); got != " ░▒▓█" {
		t.Errorf("shades = %q", got)
	}
	if got := SortByDensity("aab a"); got != " ab" && got != " ba" {
		t.Errorf("duplicates kept: %q", got)
	}
	if Density('⣿') <= Density('⠁') || Density(' ') != 0 || Density('█') != 1 {
		t.Error("density of braille / space / full block is off")
	}
}

func TestGenCharset(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, set := range GlyphSetNames() {
		for _, n := range []int{2, 5, 16, 500} {
			for _, shuffle := range []bool{false, true} {
				got := []rune(GenCharset(set, n, shuffle, rng))
				if len(got) < 2 || got[0] != ' ' {
					t.Errorf("%s/%d: %q must start with a space and have 2+ glyphs", set, n, string(got))
				}
				seen := map[rune]bool{}
				for i, r := range got {
					if seen[r] {
						t.Errorf("%s/%d: duplicate %q in %q", set, n, r, string(got))
					}
					seen[r] = true
					if i > 0 && Density(r) < Density(got[i-1]) {
						t.Errorf("%s/%d: %q is not ordered by density", set, n, string(got))
						break
					}
				}
			}
		}
	}
}

func TestUserPalettesAndCharsets(t *testing.T) {
	defer SetUserPalettes(nil)
	defer SetUserCharsets(nil)
	SetUserPalettes([]NamedPalette{{"mine", []RGB{{1, 2, 3}, {200, 100, 50}}}, {"gameboy", []RGB{{9, 9, 9}}}})
	SetUserCharsets([]Charset{{"dots2", []rune(" .:")}})

	l := DefaultLook()
	l.Palette = "mine"
	if got := PaletteColors(&l, nil); len(got) != 2 || got[1] != (RGB{200, 100, 50}) {
		t.Errorf("user palette not resolved: %v", got)
	}
	// A user palette shadows a built-in one of the same name, once.
	n := 0
	for _, p := range AllPalettes() {
		if p.Name == "gameboy" {
			n++
			if len(p.Colors) != 1 {
				t.Error("user gameboy should shadow the built-in")
			}
		}
	}
	if n != 1 || IsBuiltinPalette("gameboy") || !IsBuiltinPalette("pico8") {
		t.Errorf("shadowing: %d entries", n)
	}
	// The option table follows the registry, so flags and menus see them.
	s := DefaultSettings()
	if err := FindOption("palette").Set(&s, "mine"); err != nil {
		t.Errorf("--palette mine: %v", err)
	}
	if err := FindOption("charset").Set(&s, "dots2"); err != nil {
		t.Errorf("--charset dots2: %v", err)
	}
	s.Mode = ModeASCII
	if string(s.Ramp()) != " .:" {
		t.Errorf("ramp = %q", string(s.Ramp()))
	}
	// Editing a palette under the same name must invalidate cached LUTs.
	k1 := paletteKey(&l)
	SetUserPalettes([]NamedPalette{{"mine", []RGB{{4, 5, 6}}}})
	if paletteKey(&l) == k1 {
		t.Error("palette key did not change after the palette was edited")
	}
	names := strings.Join(charsetNames(), ",")
	if !strings.HasSuffix(names, "custom") {
		t.Errorf("custom must stay the last charset: %s", names)
	}
}
