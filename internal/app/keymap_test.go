package app

import (
	"fmt"
	"io"
	"math"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
)

// newTestApp builds a player without a terminal and without media: enough
// to press keys and to draw the interface into a screen buffer.
func newTestApp(t *testing.T, w, h int) *App {
	t.Helper()
	dir := t.TempDir()
	store, err := LoadStore(filepath.Join(dir, "presets.json"))
	if err != nil {
		t.Fatal(err)
	}
	lib, err := LoadLibrary(dir)
	if err != nil {
		t.Fatal(err)
	}
	prefs, err := LoadPrefs(dir)
	if err != nil {
		t.Fatal(err)
	}
	applyTheme(BuiltinThemes[0])
	a := &App{
		store: store, lib: lib, prefs: prefs, s: engine.DefaultSettings(), rng: rand.New(rand.NewSource(1)),
		hud: "on", ui: 1, mx: -1, my: -1, async: make(chan func(), 64), meta: map[string]streamMeta{},
		files: []string{"a.gif", "b.gif", "c.gif", "d.gif"},
	}
	a.scr = tty.NewScreen(io.Discard, 24)
	a.scr.Resize(w, h)
	return a
}

func press(a *App, names ...string) {
	for _, n := range names {
		a.handle(keyEvent(n))
	}
}

// keyEvent is the reverse of keyName, for the names the tests use.
func keyEvent(name string) tty.Event {
	ev := tty.Event{Type: tty.EvKey}
	for {
		switch {
		case strings.HasPrefix(name, "ctrl+"):
			ev.Ctrl, name = true, name[5:]
			continue
		case strings.HasPrefix(name, "alt+"):
			ev.Alt, name = true, name[4:]
			continue
		case strings.HasPrefix(name, "shift+") && name != "shift+tab":
			ev.Shift, name = true, name[6:]
			continue
		}
		break
	}
	for k, n := range keyNames {
		if n == name {
			ev.Key = k
			return ev
		}
	}
	ev.Key, ev.Rune = tty.KeyRune, []rune(name)[0]
	if name == "space" {
		ev.Rune = ' '
	}
	return ev
}

func row(a *App, y int) string {
	var b strings.Builder
	for x := 0; x < a.scr.W; x++ {
		c := a.scr.Get(x, y).Ch
		if c == 0 {
			c = ' '
		}
		b.WriteRune(c)
	}
	return b.String()
}

func screen(a *App) string {
	var b strings.Builder
	for y := 0; y < a.scr.H; y++ {
		b.WriteString(row(a, y) + "\n")
	}
	return b.String()
}

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
	for name, ev := range map[string]tty.Event{
		"alt+1":       {Type: tty.EvKey, Key: tty.KeyRune, Rune: '1', Alt: true},
		"shift+tab":   {Type: tty.EvKey, Key: tty.KeyBackTab},
		"tab":         {Type: tty.EvKey, Key: tty.KeyTab},
		"R":           {Type: tty.EvKey, Key: tty.KeyRune, Rune: 'R', Shift: true},
		"`":           {Type: tty.EvKey, Key: tty.KeyRune, Rune: '`'},
		"f10":         {Type: tty.EvKey, Key: tty.KeyF10},
		"shift+right": {Type: tty.EvKey, Key: tty.KeyRight, Shift: true},
		"backspace":   {Type: tty.EvKey, Key: tty.KeyBackspace},
	} {
		if got := keyName(ev); got != name {
			t.Errorf("keyName = %q, want %q", got, name)
		}
		if got := keyName(keyEvent(name)); got != name {
			t.Errorf("keyEvent(%q) names itself %q", name, got)
		}
	}
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

func TestLayers(t *testing.T) {
	if len(modes) != 5 {
		t.Fatalf("%d bind modes", len(modes))
	}
	coreKeys := map[string]bool{}
	for _, l := range allLayers() {
		seen := map[string]string{}
		for _, b := range l.binds {
			if b.fn == nil {
				continue
			}
			if b.opt != "" && engine.FindOption(b.opt) == nil {
				t.Errorf("%s: bind steps a missing option %q", l.name, b.opt)
			}
			for _, k := range append(strings.Fields(b.keys), strings.Fields(b.rev)...) {
				if prev, dup := seen[k]; dup {
					t.Errorf("%s: key %q is bound twice (%q and %q)", l.name, k, prev, b.desc+b.short)
				}
				seen[k] = b.desc + b.short
				if l == core {
					coreKeys[k] = true
				} else if coreKeys[k] {
					t.Errorf("mode %s takes %q away from the core layer", l.name, k)
				}
			}
		}
	}
	for _, k := range []string{"esc", "q", "space", "tab", "shift+tab", "`", "~", "enter", "backspace", "=", "-",
		"u", "h", "o", "m", "/", "?", "<", ">", "alt+1", "alt+5", "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9",
		"f10", "f11", "f12", "0", "9", "ctrl+=", "alt+=", "alt+0", "left", "up"} {
		if !coreKeys[k] {
			t.Errorf("the core layer has no %q", k)
		}
	}
	names := map[string]bool{}
	for _, m := range modes {
		if names[m.name] || m.title == "" || m.about == "" {
			t.Errorf("mode %q: duplicate or unnamed", m.name)
		}
		names[m.name] = true
		// The grammar every mode shares.
		for _, k := range []string{"r", "p", "P"} {
			if _, ok := m.find(k); !ok {
				t.Errorf("mode %s has no %q", m.name, k)
			}
		}
		p := findPanel(m.panel)
		if p == nil {
			t.Errorf("mode %s: no panel %q", m.name, m.panel)
		} else if p.mode != m.name {
			t.Errorf("panel %s belongs to mode %q, mode %s opens it", p.id, p.mode, m.name)
		}
		hints := 0
		for _, b := range m.binds {
			if b.short != "" {
				hints++
			}
			if b.fn != nil && b.desc == "" {
				t.Errorf("mode %s: %q has no description", m.name, b.keys)
			}
			// A small letter goes up, its capital goes back.
			f, r := strings.Fields(b.keys), strings.Fields(b.rev)
			if len(r) > 0 && (len(f) != 1 || len(r) != 1 || strings.ToUpper(f[0]) != r[0] || f[0] == r[0]) {
				t.Errorf("mode %s: %q / %q is not a letter and its capital", m.name, b.keys, b.rev)
			}
		}
		if hints < 5 {
			t.Errorf("mode %s shows %d hints", m.name, hints)
		}
	}
}

func TestFKeys(t *testing.T) {
	seen := map[string]bool{}
	for i, f := range fkeys {
		if f.name == "" || seen[f.name] {
			t.Errorf("F%d: name %q", i+1, f.name)
		}
		seen[f.name] = true
		if f.panel != "" && findPanel(f.panel) == nil {
			t.Errorf("F%d: no panel %q", i+1, f.panel)
		}
		if _, ok := core.find(fmt.Sprintf("f%d", i+1)); !ok {
			t.Errorf("F%d is not bound", i+1)
		}
	}
	for _, p := range panels {
		found := false
		for _, f := range fkeys {
			found = found || f.panel == p.id
		}
		if !found {
			t.Errorf("panel %s is behind no F key", p.id)
		}
		if len(p.pages) == 0 {
			t.Errorf("panel %s is empty", p.id)
		}
	}

	a := newTestApp(t, 120, 40)
	for i, f := range fkeys {
		press(a, fmt.Sprintf("f%d", i+1))
		if a.curF() != i {
			t.Fatalf("F%d opened window %d", i+1, a.curF())
		}
		a.draw() // must not panic, whatever the page
		if i > 0 && f.panel != "" && !strings.Contains(screen(a), fmt.Sprintf("F%d · ", i+1)) {
			t.Errorf("F%d: the window title does not name the key\n%s", i+1, screen(a))
		}
		if p := findPanel(f.panel); p != nil && p.mode != "" && modes[a.mode].name != p.mode {
			t.Errorf("F%d: mode is %s, the panel belongs to %s", i+1, modes[a.mode].name, p.mode)
		}
	}
	// The same key closes its window; < and > walk through all twelve.
	press(a, "f12")
	if a.curF() != -1 || a.menu != nil {
		t.Error("F12 twice did not close the window")
	}
	for i := range fkeys {
		press(a, ">")
		if i == 8 {
			a.finder.typing = false // the search box would take the next key
		}
		if a.curF() != i {
			t.Fatalf("> %d times shows window %d", i+1, a.curF())
		}
	}
	press(a, "<")
	if a.curF() != 10 {
		t.Errorf("< from F12 shows window %d", a.curF())
	}
	// F keys get through from the finder and the file browser.
	press(a, "f9", "f4")
	if a.finderOpen || a.menu == nil || a.menu.panel.id != "effects" {
		t.Error("F4 from the finder did not open the effects")
	}
	a.openBrowser()
	press(a, "f5")
	if a.browser != nil || a.menu == nil || a.menu.panel.id != "sound" {
		t.Error("F5 from the file browser did not open the sound panel")
	}
}

func TestModeSwitch(t *testing.T) {
	a := newTestApp(t, 100, 30)
	if a.mode != 0 {
		t.Fatalf("starts in mode %d", a.mode)
	}
	for i := 1; i <= len(modes); i++ {
		press(a, "tab")
		if a.mode != i%len(modes) {
			t.Fatalf("Tab %d times: mode %d", i, a.mode)
		}
	}
	press(a, "shift+tab")
	if a.mode != len(modes)-1 {
		t.Errorf("Shift+Tab from the first mode: %d", a.mode)
	}
	press(a, "`", "~", "~")
	if a.mode != len(modes)-2 {
		t.Errorf("` ~ ~: mode %d", a.mode)
	}
	for i, m := range modes {
		press(a, fmt.Sprintf("alt+%d", i+1))
		if a.mode != i {
			t.Errorf("Alt+%d: mode %d", i+1, a.mode)
		}
		if !strings.Contains(a.toast, strings.ToUpper(m.name)) || !a.toastTint {
			t.Errorf("Alt+%d: toast %q (tinted %v)", i+1, a.toast, a.toastTint)
		}
		press(a, "enter")
		if a.menu == nil || a.menu.panel.id != m.panel {
			t.Errorf("Enter in mode %s did not open %s", m.name, m.panel)
		}
		press(a, "esc")
	}
}

// The same letter does different work in each mode, and the mode is asked
// before the core layer.
func TestModeKeys(t *testing.T) {
	a := newTestApp(t, 100, 30)
	def := engine.DefaultSettings()

	press(a, "alt+2", "v")
	if a.s.Mode == def.Mode || a.s.VHS != 0 {
		t.Errorf("video v: mode %s, vhs %v", a.s.Mode, a.s.VHS)
	}
	press(a, "V", "alt+4", "v")
	if a.s.Mode != def.Mode || a.s.VHS <= 0 {
		t.Errorf("fx v: mode %s, vhs %v", a.s.Mode, a.s.VHS)
	}
	vhs := a.s.VHS
	// = and - repeat the last setting key.
	press(a, "=", "=")
	if a.s.VHS <= vhs {
		t.Errorf("= did not raise vhs: %v", a.s.VHS)
	}
	press(a, "-", "-", "-")
	if a.s.VHS != 0 {
		t.Errorf("- did not lower vhs to 0: %v", a.s.VHS)
	}
	press(a, "alt+5", "v")
	if a.s.Sound.ReverbMix <= 0 || a.s.VHS != 0 {
		t.Errorf("audio v: reverb %v", a.s.Sound.ReverbMix)
	}
	// Switching the mode forgets the key to repeat.
	press(a, "alt+3", "=")
	if a.s.Sound.ReverbMix != engine.FindOption("reverb").Step*2 || a.lastBind != nil {
		t.Errorf("= after a mode switch changed something: reverb %v", a.s.Sound.ReverbMix)
	}
	press(a, "b")
	if a.s.Brightness <= 0 {
		t.Errorf("color b: brightness %v", a.s.Brightness)
	}
	// Posterize skips the blank picture one level would give.
	press(a, "alt+4", "e")
	if a.s.Posterize != 8 {
		t.Errorf("fx e: posterize %d", a.s.Posterize)
	}
	for i := 0; i < 20; i++ {
		press(a, "e")
	}
	if a.s.Posterize != 2 {
		t.Errorf("fx e many times: posterize %d", a.s.Posterize)
	}
	for i := 0; i < 20; i++ {
		press(a, "E")
	}
	if a.s.Posterize != 0 {
		t.Errorf("fx E many times: posterize %d", a.s.Posterize)
	}
	// Frequencies move by a ratio, and the small letter is the stronger effect.
	press(a, "alt+5", "l")
	if lp := a.s.Sound.Lowpass; lp >= 20000 || lp < 10000 {
		t.Errorf("audio l: low-pass %v", lp)
	}
	press(a, "L", "L", "x")
	if a.s.Sound.Lowpass != 20000 || a.s.Sound.Bits != 15 {
		t.Errorf("audio L L x: low-pass %v, bits %d", a.s.Sound.Lowpass, a.s.Sound.Bits)
	}
	// Core keys work in every mode.
	for i := range modes {
		a.s.Mute = false
		press(a, fmt.Sprintf("alt+%d", i+1), "m")
		if !a.s.Mute {
			t.Errorf("m does not mute in mode %s", modes[i].name)
		}
	}
}

func TestModeReset(t *testing.T) {
	a := newTestApp(t, 100, 30)
	def := engine.DefaultSettings()
	press(a, "alt+2", "v", "d", "c", "x", "alt+3", "b", "c", "p", "alt+4", "v", "g", "alt+5", "v", "b")
	a.s.Speed = 2

	press(a, "alt+3", "backspace")
	if a.s.Brightness != 0 || a.s.Contrast != 1 || a.s.Filter != def.Filter {
		t.Errorf("color reset left %v %v %s", a.s.Brightness, a.s.Contrast, a.s.Filter)
	}
	if a.s.Mode == def.Mode || a.s.VHS == 0 || a.s.Sound.ReverbMix == 0 {
		t.Error("color reset touched another mode's settings")
	}
	press(a, "alt+2", "backspace")
	if a.s.Mode != def.Mode || a.s.Dither != def.Dither || a.s.Palette != def.Palette || a.s.FlipX {
		t.Errorf("video reset left %s %s %s", a.s.Mode, a.s.Dither, a.s.Palette)
	}
	press(a, "alt+4", "backspace")
	if a.s.FX != engine.DefaultFX() {
		t.Errorf("fx reset left %+v", a.s.FX)
	}
	press(a, "alt+5", "backspace")
	if a.s.Sound.SoundActive() {
		t.Error("audio reset left the sound changed")
	}
	press(a, "alt+1", "backspace")
	if a.s.Speed != 1 {
		t.Errorf("play reset left speed %v", a.s.Speed)
	}
	// Every reset can be undone, the sound one included.
	a.lastTouch = a.lastTouch.Add(-2 * time.Second)
	press(a, "alt+5", "r")
	if !a.s.Sound.SoundActive() {
		t.Fatal("audio r changed nothing")
	}
	press(a, "u")
	if a.s.Sound.SoundActive() {
		t.Error("undo did not bring the clean sound back")
	}
}

// A sound setting changed in a panel is an undo step of its own: undo takes
// it back and leaves the look change before it alone.
func TestUndoPanelSound(t *testing.T) {
	a := newTestApp(t, 100, 30)
	press(a, "alt+3", "b")
	bright := a.s.Brightness
	if bright <= 0 {
		t.Fatal("b changed nothing")
	}
	a.lastTouch = a.lastTouch.Add(-2 * time.Second)
	a.optField(engine.FindOption("bass")).nudge(3)
	if a.s.Sound.Bass != 3 {
		t.Fatalf("bass %v after the panel raised it", a.s.Sound.Bass)
	}
	press(a, "u")
	if a.s.Sound.Bass != 0 || a.s.Brightness != bright {
		t.Errorf("first undo: bass %v, brightness %v (want 0, %v)", a.s.Sound.Bass, a.s.Brightness, bright)
	}
	press(a, "u")
	if a.s.Brightness != 0 {
		t.Errorf("second undo: brightness %v", a.s.Brightness)
	}
	// The same through a typed value.
	a.lastTouch = a.lastTouch.Add(-2 * time.Second)
	if err := a.optField(engine.FindOption("reverb")).set("0.5"); err != nil {
		t.Fatal(err)
	}
	press(a, "u")
	if a.s.Sound.ReverbMix != 0 {
		t.Errorf("undo left reverb at %v", a.s.Sound.ReverbMix)
	}
}

func TestModeColors(t *testing.T) {
	defer applyTheme(BuiltinThemes[0])
	for _, th := range BuiltinThemes {
		applyTheme(th)
		if modeColor(0) != cAccent {
			t.Errorf("%s: the first mode does not wear the accent", th.Name)
		}
		bl, _, _ := engine.ToOklab(engine.FromU32(cBar))
		for i := range modes {
			l1, a1, b1 := engine.ToOklab(engine.FromU32(modeColor(i)))
			// Readable as text on the bar and as a background under it.
			if i > 0 && math.Abs(l1-bl) < 0.25 {
				t.Errorf("%s: mode %s (%06x) is too close to the bar in lightness", th.Name, modes[i].name, modeColor(i))
			}
			for j := i + 1; j < len(modes); j++ {
				l2, a2, b2 := engine.ToOklab(engine.FromU32(modeColor(j)))
				if d := math.Sqrt((l1-l2)*(l1-l2) + (a1-a2)*(a1-a2) + (b1-b2)*(b1-b2)); d < 0.09 {
					t.Errorf("%s: modes %s (%06x) and %s (%06x) look alike, ΔE %.3f", th.Name,
						modes[i].name, modeColor(i), modes[j].name, modeColor(j), d)
				}
			}
		}
	}
}

// The bars name the mode, show it in its color, and fit at any width.
func TestHUDModes(t *testing.T) {
	for _, size := range [][2]int{{160, 45}, {100, 30}, {80, 24}, {60, 20}, {44, 12}, {30, 8}} {
		for ui := 0; ui < len(uiNames); ui++ {
			a := newTestApp(t, size[0], size[1])
			a.ui = ui
			for i, m := range modes {
				a.setMode(i)
				a.toast = ""
				a.draw()
				name := strings.ToUpper(m.name)
				y := 0
				if ui == 0 {
					y = a.scr.H - 1 // the compact interface is one line at the bottom
				}
				x := strings.Index(row(a, y), name)
				if x < 0 {
					t.Errorf("%dx%d ui %d: the bar does not name mode %s: %q", size[0], size[1], ui, m.name, row(a, y))
					continue
				}
				x = len([]rune(row(a, y)[:x]))
				if c := a.scr.Get(x, y); c.Fg != modeColor(i) && c.Bg != modeColor(i) {
					t.Errorf("%dx%d ui %d: mode %s is not drawn in its color", size[0], size[1], ui, m.name)
				}
				if ui == 0 || size[0] < 40 || size[1] < 10 {
					continue
				}
				// The hint row: keys of this mode, in its color.
				hints := ""
				for y := 0; y < a.scr.H; y++ {
					if strings.Contains(row(a, y), "F1 all keys") {
						hints = row(a, y)
						if c := a.scr.Get(1, y); c.Fg != modeColor(i) {
							t.Errorf("%dx%d: the first hint key of %s is not in the mode color", size[0], size[1], m.name)
						}
					}
				}
				first := m.binds[0]
				if want := prettyKey(strings.Fields(first.keys)[0]) + " " + first.short; !strings.Contains(hints, want) {
					t.Errorf("%dx%d ui %d: no hint %q for mode %s in %q", size[0], size[1], ui, want, m.name, hints)
				}
				// With a window open the row lists the F keys instead.
				a.openF(6)
				a.draw()
				if _, bottom := a.desk(); bottom != a.hintY {
					a.menu = nil
					continue // too small to keep a row free for the strip
				}
				if s := row(a, a.hintY); !strings.Contains(s, "F1") || !strings.Contains(s, "F12") || strings.Contains(s, "F1 all keys") {
					t.Errorf("%dx%d ui %d: no F key strip with a window open\n%s", size[0], size[1], ui, s)
				}
				a.menu = nil
			}
		}
	}
	// The hint row can be switched off.
	a := newTestApp(t, 100, 30)
	a.prefs.Hints = false
	a.draw()
	if strings.Contains(screen(a), "F1 all keys") {
		t.Error("key hints are drawn with the preference off")
	}
}

// Clicking a mode name switches to it; clicking a hint runs its key.
func TestHUDClicks(t *testing.T) {
	a := newTestApp(t, 120, 30)
	a.draw()
	x := len([]rune(row(a, 0)[:strings.Index(row(a, 0), " fx ")])) + 1
	click := func(x, y int, b int) {
		a.handle(tty.Event{Type: tty.EvMouse, Action: tty.MousePress, Button: b, X: x, Y: y})
		a.handle(tty.Event{Type: tty.EvMouse, Action: tty.MouseRelease, Button: b, X: x, Y: y})
		a.draw()
	}
	click(x, 0, tty.ButtonLeft)
	if modes[a.mode].name != "fx" {
		t.Fatalf("a click on fx gave mode %s", modes[a.mode].name)
	}
	for y := 0; y < a.scr.H; y++ {
		if i := strings.Index(row(a, y), "v vhs"); i >= 0 {
			hx := len([]rune(row(a, y)[:i]))
			click(hx, y, tty.ButtonLeft)
			if a.s.VHS <= 0 {
				t.Error("a click on the vhs hint did nothing")
			}
			click(hx, y, tty.ButtonRight)
			if a.s.VHS != 0 {
				t.Errorf("a right-click on the vhs hint left vhs at %v", a.s.VHS)
			}
			return
		}
	}
	t.Errorf("no vhs hint on screen\n%s", screen(a))
}

func TestPlaylistEdits(t *testing.T) {
	a := newTestApp(t, 100, 30)
	a.fileIdx = 2 // c.gif; nothing is loaded, so nothing is reopened
	if !a.moveFile(2, -1) || a.fileIdx != 1 || a.files[1] != "c.gif" || a.files[2] != "b.gif" {
		t.Errorf("moving the current file up: idx %d, %v", a.fileIdx, a.files)
	}
	if !a.moveFile(0, 1) || a.fileIdx != 0 || a.files[0] != "c.gif" {
		t.Errorf("moving a file onto the current one: idx %d, %v", a.fileIdx, a.files)
	}
	if a.moveFile(0, -1) || a.moveFile(3, 1) {
		t.Error("a file moved off the end of the list")
	}
	a.fileIdx = 2
	cur := a.files[2]
	a.removeFile(0)
	if a.fileIdx != 1 || a.files[a.fileIdx] != cur || len(a.files) != 3 {
		t.Errorf("removing an earlier file: idx %d, %v", a.fileIdx, a.files)
	}
	a.removeFile(2)
	if a.fileIdx != 1 || a.files[a.fileIdx] != cur || len(a.files) != 2 {
		t.Errorf("removing a later file: idx %d, %v", a.fileIdx, a.files)
	}
	a.removeFile(0)
	a.removeFile(0)
	if len(a.files) != 1 || a.files[0] != cur || a.fileIdx != 0 {
		t.Errorf("the last file was removed: %v", a.files)
	}
	// Removing the entry the index points at, at the end of the list.
	a.files, a.fileIdx = []string{"a.gif", "b.gif"}, 1
	a.removeFile(1)
	if a.fileIdx != 0 || len(a.files) != 1 {
		t.Errorf("removing the last, current file: idx %d, %v", a.fileIdx, a.files)
	}
	// The pages draw and list what is there.
	a.files = []string{"/tmp/x/one.gif", "https://example.com/two.mp4"}
	a.showPage(pgPlaylist)
	a.draw()
	if s := screen(a); !strings.Contains(s, "one.gif") || !strings.Contains(s, "example.com") {
		t.Errorf("the playlist page does not list the files\n%s", s)
	}
	a.showPage(pgNow)
	a.draw()
	if !strings.Contains(screen(a), "Nothing is playing") {
		t.Errorf("the now playing page with nothing loaded\n%s", screen(a))
	}
}

func TestPrefsKeys(t *testing.T) {
	dir := t.TempDir()
	p, err := LoadPrefs(dir)
	if err != nil || !p.Hints || p.startMode() != "play" {
		t.Fatalf("default prefs: %+v, %v", p, err)
	}
	p.Keys, p.Mode, p.Hints = "last", "fx", false
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	p, _ = LoadPrefs(dir)
	if p.Hints || p.startMode() != "fx" {
		t.Errorf("prefs did not round-trip: %+v", p)
	}
	if got := KeyTable(); len(got) != len(modes)+1 || got[0].Name != "play" || len(got[len(got)-1].Lines) < 30 {
		t.Errorf("KeyTable: %d sections", len(got))
	}
}
