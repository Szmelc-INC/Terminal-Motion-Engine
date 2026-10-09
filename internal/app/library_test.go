package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
)

func TestLibraryCRUD(t *testing.T) {
	defer engine.SetUserPalettes(nil)
	defer engine.SetUserCharsets(nil)
	dir := t.TempDir()
	l, err := LoadLibrary(dir)
	if err != nil {
		t.Fatal(err)
	}
	cols := []engine.RGB{{R: 1, G: 2, B: 3}, {R: 250, G: 240, B: 230}}
	if err := l.SavePalette("  dusk ", cols); err != nil {
		t.Fatal(err)
	}
	if err := l.SavePalette("harmony", cols); err == nil {
		t.Error("a palette named after a generator must be refused")
	}
	if err := l.SavePalette("a/b", cols); err == nil {
		t.Error("a name with a slash must be refused")
	}
	if err := l.SavePalette("empty", nil); err == nil {
		t.Error("an empty palette must be refused")
	}
	if err := l.SaveCharset("thin", " .:"); err != nil {
		t.Fatal(err)
	}
	if err := l.SaveCharset("one", "x"); err == nil {
		t.Error("a one-glyph charset must be refused")
	}
	if err := l.SaveTheme(GenTheme("teal", 190, 0.12, false)); err != nil {
		t.Fatal(err)
	}
	// Saving registers with the engine straight away.
	if c, ok := engine.FindPalette("dusk"); !ok || len(c) != 2 {
		t.Errorf("palette not registered: %v %v", c, ok)
	}

	// Everything survives a reload.
	l2, err := LoadLibrary(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(l2.Palettes) != 1 || l2.Palettes[0].Name != "dusk" || len(l2.Charsets) != 1 || len(l2.Themes) != 1 {
		t.Fatalf("reloaded library: %+v", l2)
	}
	if th, ok := l2.FindTheme("TEAL"); !ok || th.Builtin || len(th.Colors) != len(ThemeSlots) {
		t.Errorf("theme lookup: %+v %v", th, ok)
	}

	if err := l2.Rename("palette", "dusk", "dawn"); err != nil {
		t.Fatal(err)
	}
	if err := l2.Rename("palette", "pico8", "x"); err == nil {
		t.Error("renaming a built-in palette must fail")
	}
	if err := l2.SavePalette("other", cols); err != nil {
		t.Fatal(err)
	}
	if err := l2.Rename("palette", "other", "DAWN"); err == nil {
		t.Error("renaming onto an existing name must fail")
	}
	if err := l2.Delete("palette", "dawn"); err != nil {
		t.Fatal(err)
	}
	if err := l2.Delete("palette", "dawn"); err == nil {
		t.Error("deleting twice must fail")
	}
	if err := l2.Delete("theme", "tokyo-night"); err == nil {
		t.Error("deleting a built-in theme must fail")
	}
	if _, ok := engine.FindPalette("dawn"); ok {
		t.Error("deleted palette is still registered")
	}
}

func TestThemes(t *testing.T) {
	for _, th := range BuiltinThemes {
		if len(th.Colors) != len(ThemeSlots) {
			t.Errorf("theme %s has %d colors, want %d", th.Name, len(th.Colors), len(ThemeSlots))
		}
		for _, h := range th.Colors {
			if _, err := engine.ParseHex(h); err != nil {
				t.Errorf("theme %s: %v", th.Name, err)
			}
		}
	}
	// Generated themes must keep text readable on the panel, dark or light.
	for _, light := range []bool{false, true} {
		for hue := 0.0; hue < 360; hue += 45 {
			c := GenTheme("g", hue, 0.15, light).RGBs()
			panel, text := int(engine.Luma(c[1].R, c[1].G, c[1].B)), int(engine.Luma(c[2].R, c[2].G, c[2].B))
			if d := text - panel; (light && d > -110) || (!light && d < 110) {
				t.Errorf("hue %.0f light=%v: text %d on panel %d has too little contrast", hue, light, text, panel)
			}
		}
	}
	// A broken user theme falls back slot by slot instead of going black.
	c := Theme{Colors: []string{"#112233", "junk"}}.RGBs()
	if c[0] != (engine.RGB{R: 0x11, G: 0x22, B: 0x33}) || c[1].Hex() != BuiltinThemes[0].Colors[1] {
		t.Errorf("fallback: %v", c[:2])
	}
	defer applyTheme(BuiltinThemes[0])
	applyTheme(BuiltinThemes[1])
	if want, _ := engine.ParseHex(BuiltinThemes[1].Colors[4]); cAccent != want.U32() {
		t.Errorf("applyTheme did not set the accent color")
	}
}

func TestPrefs(t *testing.T) {
	dir := t.TempDir()
	p, err := LoadPrefs(dir)
	if err != nil || p.UI != "normal" || p.Theme != "tokyo-night" || p.HUD != "auto" {
		t.Fatalf("defaults: %+v %v", p, err)
	}
	p.UI, p.YouTubeKey = "large", "secret"
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(dir, "config.json"))
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("config.json must be private (0600): %v %v", st.Mode(), err)
	}
	q, _ := LoadPrefs(dir)
	if q.UI != "large" || q.YouTubeKey != "secret" {
		t.Errorf("reloaded: %+v", q)
	}
	// An unknown size from a hand-edited file falls back.
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"ui":"gigantic"}`), 0o600)
	if q, _ := LoadPrefs(dir); q.UI != "normal" {
		t.Errorf("bad ui size kept: %q", q.UI)
	}
}

func TestLibraryEffects(t *testing.T) {
	dir := t.TempDir()
	l, err := LoadLibrary(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(l.AllFX()); n != len(engine.BuiltinFX) {
		t.Fatalf("an empty library lists %d effect presets, want the %d built-in ones", n, len(engine.BuiltinFX))
	}
	f := engine.DefaultFX()
	f.VHS, f.Mask, f.Interlace = 0.5, "slot", true
	if err := l.SaveFX("mine", f); err != nil {
		t.Fatal(err)
	}
	if err := l.SaveFX("a/b", f); err == nil {
		t.Error("a name with a slash must be refused")
	}
	// A user preset may shadow a built-in one; it is then listed once.
	if err := l.SaveFX("VHS-Tape", f); err != nil {
		t.Fatal(err)
	}
	if n := len(l.AllFX()); n != len(engine.BuiltinFX)+1 {
		t.Errorf("%d effect presets listed, want %d", n, len(engine.BuiltinFX)+1)
	}
	if p, ok := l.FindFX("vhs-tape"); !ok || p.Builtin || p.FX != f {
		t.Errorf("the user preset must win over the built-in one: %+v", p)
	}

	l2, err := LoadLibrary(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := l2.FindFX("MINE"); !ok || p.Builtin || p.FX != f {
		t.Fatalf("reloaded effect preset: %+v %v", p, ok)
	}
	if err := l2.Rename("fx", "mine", "yours"); err != nil {
		t.Fatal(err)
	}
	if err := l2.Rename("fx", "crt-tv", "x"); err == nil {
		t.Error("renaming a built-in effect preset must fail")
	}
	if err := l2.Rename("fx", "yours", "vhs-tape"); err == nil {
		t.Error("renaming onto an existing name must fail")
	}
	if err := l2.Delete("fx", "yours"); err != nil {
		t.Fatal(err)
	}
	if err := l2.Delete("fx", "yours"); err == nil {
		t.Error("deleting twice must fail")
	}
	if err := l2.Delete("fx", "crt-tv"); err == nil {
		t.Error("deleting a built-in effect preset must fail")
	}

	// A preset written before a setting existed gets that setting's default.
	os.WriteFile(filepath.Join(dir, "library.json"), []byte(`{"effects":[{"name":"old","fx":{"vhs":0.4}}]}`), 0o644)
	l3, err := LoadLibrary(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := engine.DefaultFX()
	want.VHS = 0.4
	if p, ok := l3.FindFX("old"); !ok || p.FX != want {
		t.Errorf("old effect preset decoded to %+v", p.FX)
	}
}

func TestManagerFilter(t *testing.T) {
	p := &managerPage{name: "Things", items: func(*App) []mItem {
		return []mItem{
			{name: "gameboy", tag: "handheld", info: "4 colors"},
			{name: "pico8", tag: "pixelart", info: "16 colors"},
			{name: "dusk", user: true, info: "4 colors"},
		}
	}}
	names := func() string {
		out := ""
		for _, it := range p.list(nil) {
			out += it.name + " "
		}
		return out
	}
	for filter, want := range map[string]string{
		"":           "gameboy pico8 dusk ",
		"PICO":       "pico8 ",
		"handheld":   "gameboy ",
		"user":       "dusk ",
		"4 colors":   "gameboy dusk ",
		"4 game":     "gameboy ",
		"no-such":    "",
		"  pixel   ": "pico8 ",
	} {
		if p.filter = filter; names() != want {
			t.Errorf("filter %q lists %q, want %q", filter, names(), want)
		}
	}
	// Selecting something the filter hides drops the filter.
	p.filter = "pico"
	p.selectName(nil, "dusk")
	if p.filter != "" || p.sel != 2 {
		t.Errorf("selectName under a filter: filter %q, selection %d", p.filter, p.sel)
	}
}
