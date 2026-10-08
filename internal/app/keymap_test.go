package app

import (
	"strings"
	"testing"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
)

func TestFitZoom(t *testing.T) {
	l := engine.DefaultLook()
	// Zoom 1 is plain Fit.
	if a, b := Fit(16.0/9, l, 80, 24, 0.5), FitZoom(16.0/9, l, 80, 24, 0.5, 1); a != b {
		t.Errorf("zoom 1: %+v != %+v", b, a)
	}
	// Smaller: the picture shrinks, nothing is cropped.
	g := FitZoom(16.0/9, l, 80, 24, 0.5, 0.5)
	if g.Cols != 40 || g.Rows != 11 && g.Rows != 12 || g.CropX != 1 || g.CropY != 1 {
		t.Errorf("zoom 0.5: %+v", g)
	}
	// Bigger: it cannot outgrow the grid, so the source is cropped instead.
	g = FitZoom(16.0/9, l, 80, 24, 0.5, 2)
	if g.Cols != 80 || g.Rows != 24 || g.CropX > 0.51 || g.CropX < 0.49 || g.CropY >= 1 {
		t.Errorf("zoom 2: %+v", g)
	}
	// A tall picture first fills the side bars, then crops.
	g = FitZoom(0.5, l, 80, 24, 0.5, 1.5)
	if g.Cols != 36 || g.Rows != 24 || g.CropX != 1 || g.CropY > 0.67 || g.CropY < 0.66 {
		t.Errorf("zoom 1.5 tall: %+v", g)
	}
}

func TestKeyNames(t *testing.T) {
	cases := []struct {
		ev   tty.Event
		want string
	}{
		{tty.Event{Key: tty.KeyRune, Rune: ' '}, "space"},
		{tty.Event{Key: tty.KeyRune, Rune: 'R'}, "R"},
		{tty.Event{Key: tty.KeyRune, Rune: '=', Ctrl: true}, "ctrl+="},
		{tty.Event{Key: tty.KeyRune, Rune: '=', Ctrl: true, Shift: true}, "ctrl+shift+="},
		{tty.Event{Key: tty.KeyRune, Rune: '-', Alt: true}, "alt+-"},
		{tty.Event{Key: tty.KeyLeft, Shift: true}, "shift+left"},
		{tty.Event{Key: tty.KeyLeft, Alt: true}, "alt+left"},
		{tty.Event{Key: tty.KeyF11}, "f11"},
		{tty.Event{Key: tty.KeyBackTab, Shift: true}, "shift+tab"},
		{tty.Event{Key: tty.KeyCtrlS}, "ctrl+s"},
	}
	for _, c := range cases {
		if got := keyName(c.ev); got != c.want {
			t.Errorf("%+v -> %q, want %q", c.ev, got, c.want)
		}
	}
}

// Every key may be bound once per layer, and every bind that acts must be
// reachable; a duplicate would silently shadow the earlier one.
func TestKeymapHasNoDuplicates(t *testing.T) {
	for _, l := range allLayers() {
		seen := map[string]string{}
		for _, b := range l.binds {
			if b.fn == nil {
				if b.desc == "" {
					t.Errorf("%s: bind %q has neither action nor description", l.name, b.keys)
				}
				continue
			}
			for _, k := range strings.Fields(b.keys) {
				if prev, dup := seen[k]; dup {
					t.Errorf("%s: key %q is bound twice (%q and %q)", l.name, k, prev, b.keys)
				}
				seen[k] = b.keys
			}
		}
	}
}
