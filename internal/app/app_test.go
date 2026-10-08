package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
)

func TestStoreCRUD(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "presets.json")
	s, err := LoadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.All()) != len(engine.BuiltinPresets) {
		t.Fatalf("empty store lists %d presets", len(s.All()))
	}

	l := engine.DefaultLook()
	l.Mode, l.Palette, l.Custom, l.Hue = engine.ModeBraille, engine.PalCustom, []string{"#112233", "#ffffff"}, 123
	if err := s.Save("  mine ", l); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("", l); err == nil {
		t.Error("empty name accepted")
	}

	// Reload from disk: everything must survive.
	s2, err := LoadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := s2.Find("MINE")
	if !ok || p.Builtin || p.Name != "mine" {
		t.Fatalf("Find after reload = %+v, %v", p, ok)
	}
	if p.Look.Mode != engine.ModeBraille || p.Look.Hue != 123 || len(p.Look.Custom) != 2 || p.Look.Custom[0] != "#112233" {
		t.Errorf("look did not round-trip: %+v", p.Look)
	}

	// Overwrite (edit).
	l.Dither = "atkinson"
	if err := s2.Save("mine", l); err != nil {
		t.Fatal(err)
	}
	if len(s2.User) != 1 {
		t.Errorf("overwrite duplicated the preset: %d", len(s2.User))
	}

	// Rename.
	if err := s2.Rename("mine", "theirs"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s2.Find("mine"); ok {
		t.Error("old name still present after rename")
	}
	if err := s2.Rename("default", "x"); err == nil {
		t.Error("renaming a built-in should fail")
	}
	if err := s2.Save("other", l); err != nil {
		t.Fatal(err)
	}
	if err := s2.Rename("other", "theirs"); err == nil {
		t.Error("rename onto an existing name should fail")
	}

	// A user preset may shadow a built-in; deleting it brings the built-in back.
	if err := s2.Save("gameboy", l); err != nil {
		t.Fatal(err)
	}
	if p, _ := s2.Find("gameboy"); p.Builtin || p.Look.Dither != "atkinson" {
		t.Error("user preset does not shadow the built-in")
	}
	if err := s2.Delete("gameboy"); err != nil {
		t.Fatal(err)
	}
	if p, ok := s2.Find("gameboy"); !ok || !p.Builtin {
		t.Error("built-in did not return after deleting its shadow")
	}
	if err := s2.Delete("gameboy"); err == nil {
		t.Error("deleting a built-in should fail")
	}
	if err := s2.Delete("nope"); err == nil {
		t.Error("deleting a missing preset should fail")
	}

	// Delete persists.
	if err := s2.Delete("theirs"); err != nil {
		t.Fatal(err)
	}
	s3, _ := LoadStore(path)
	if _, ok := s3.Find("theirs"); ok {
		t.Error("deleted preset came back after reload")
	}
}

func TestStoreOldFileGetsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "presets.json")
	os.WriteFile(path, []byte(`{"presets":[{"name":"old","look":{"mode":"ascii"}}]}`), 0o644)
	s, err := LoadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.Find("old")
	if p.Look.Mode != "ascii" || p.Look.Contrast != 1 || p.Look.Gamma != 1 || p.Look.Dither == "" {
		t.Errorf("missing fields were not defaulted: %+v", p.Look)
	}
	os.WriteFile(path, []byte(`{not json`), 0o644)
	if _, err := LoadStore(path); err == nil {
		t.Error("corrupt file should be reported")
	}
}

func TestFit(t *testing.T) {
	l := engine.DefaultLook() // half blocks: 1×2 sub-cells
	// 16:9 picture on an 80×24 grid of 1:2 cells (view is 40:24 = 1.67).
	g := Fit(16.0/9, l, 80, 24, 0.5)
	if g.Cols != 80 || g.Rows != 23 || g.W != 80 || g.H != 46 || g.CropX != 1 || g.CropY != 1 {
		t.Errorf("fit wide: %+v", g)
	}
	// Tall picture: limited by height.
	g = Fit(0.5, l, 80, 24, 0.5)
	if g.Rows != 24 || g.Cols != 24 {
		t.Errorf("fit tall: %+v", g)
	}
	l.Fit = "fill"
	g = Fit(16.0/9, l, 80, 24, 0.5)
	if g.Cols != 80 || g.Rows != 24 || g.CropX >= 1 || g.CropY != 1 {
		t.Errorf("fill: %+v", g)
	}
	l.Fit, l.Mode = "stretch", engine.ModeBraille
	g = Fit(16.0/9, l, 80, 24, 0.5)
	if g.W != 160 || g.H != 96 || g.CropX != 1 {
		t.Errorf("stretch braille: %+v", g)
	}
	// Never zero-sized, even on absurd inputs.
	g = Fit(100, engine.DefaultLook(), 3, 40, 0.5)
	if g.Cols < 1 || g.Rows < 1 {
		t.Errorf("degenerate: %+v", g)
	}
}
