package app

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
)

// bind ties one or more keys to an action. keys is a space-separated list of
// key names as produced by keyName.
type bind struct {
	keys string
	desc string
	fn   func(a *App)
}

// layer is a named set of key bindings. The "global" layer is always active;
// the others are bind modes, of which one is active at a time.
type layer struct {
	name, title string
	color       uint32
	binds       []bind
	index       map[string]*bind
}

func (l *layer) find(key string) *bind {
	if l.index == nil {
		l.index = map[string]*bind{}
		for i := range l.binds {
			if l.binds[i].fn == nil {
				continue // a help-only line
			}
			for _, k := range strings.Fields(l.binds[i].keys) {
				l.index[k] = &l.binds[i]
			}
		}
	}
	return l.index[key]
}

// pair adds two binds that act in opposite directions; their shared help
// line is a separate entry without an action.
func (l *layer) pair(fwd, back string, f, b func(*App)) {
	l.binds = append(l.binds, bind{fwd, "", f}, bind{back, "", b})
}

var keyNames = map[tty.Key]string{
	tty.KeyEnter: "enter", tty.KeyEsc: "esc", tty.KeyTab: "tab", tty.KeyBackTab: "shift+tab",
	tty.KeyBackspace: "backspace", tty.KeyDelete: "delete", tty.KeyUp: "up", tty.KeyDown: "down",
	tty.KeyLeft: "left", tty.KeyRight: "right", tty.KeyHome: "home", tty.KeyEnd: "end",
	tty.KeyPgUp: "pgup", tty.KeyPgDn: "pgdn", tty.KeyF1: "f1", tty.KeyF2: "f2", tty.KeyF3: "f3",
	tty.KeyF4: "f4", tty.KeyF5: "f5", tty.KeyF6: "f6", tty.KeyF7: "f7", tty.KeyF8: "f8", tty.KeyF9: "f9",
	tty.KeyF10: "f10", tty.KeyF11: "f11", tty.KeyF12: "f12",
	tty.KeyCtrlC: "ctrl+c", tty.KeyCtrlS: "ctrl+s", tty.KeyCtrlL: "ctrl+l",
}

// keyName gives an event its canonical name: "space", "R", "ctrl+=",
// "shift+left", "f5"… A shifted letter is named by the letter it types;
// shift is only spelled out next to another modifier or a named key.
func keyName(ev tty.Event) string {
	var b strings.Builder
	if ev.Ctrl {
		b.WriteString("ctrl+")
	}
	if ev.Alt {
		b.WriteString("alt+")
	}
	if ev.Key == tty.KeyRune {
		if ev.Shift && (ev.Ctrl || ev.Alt) {
			b.WriteString("shift+")
		}
		if ev.Rune == ' ' {
			b.WriteString("space")
		} else {
			b.WriteRune(ev.Rune)
		}
		return b.String()
	}
	name, ok := keyNames[ev.Key]
	if !ok {
		return ""
	}
	if ev.Shift && ev.Key != tty.KeyBackTab {
		b.WriteString("shift+")
	}
	b.WriteString(name)
	return b.String()
}

// prettyKeys formats a bind's key list for the help screen.
func prettyKeys(keys string) string {
	r := strings.NewReplacer("left", "←", "right", "→", "up", "↑", "down", "↓", "space", "Space",
		"ctrl+", "Ctrl+", "alt+", "Alt+", "shift+", "Shift+", "esc", "Esc", "tab", "Tab", "home", "Home",
		"enter", "Enter", "pgup", "PgUp", "pgdn", "PgDn")
	f := strings.Fields(keys)
	if len(f) > 2 {
		f = f[:2]
	}
	for i, k := range f {
		if len(k) > 1 && k[0] == 'f' && k[1] >= '1' && k[1] <= '9' {
			f[i] = "F" + k[1:]
			continue
		}
		f[i] = r.Replace(k)
	}
	return strings.Join(f, " ")
}

func nudger(key string, dir int) func(*App) { return func(a *App) { a.nudge(key, dir) } }

var globalLayer = &layer{name: "global", title: "Everywhere", color: cAccent}

func init() {
	g := globalLayer
	g.binds = []bind{
		{"space k", "play / pause", func(a *App) { a.setPlaying(!a.isPlaying()) }},
		{"left", "seek back 5 s", func(a *App) { a.seekBy(-5) }},
		{"right", "seek forward 5 s", func(a *App) { a.seekBy(5) }},
		{"shift+left", "seek back 30 s", func(a *App) { a.seekBy(-30) }},
		{"shift+right", "seek forward 30 s", func(a *App) { a.seekBy(30) }},
		{",", "previous frame", func(a *App) { a.stepFrame(-1) }},
		{".", "next frame", func(a *App) { a.stepFrame(1) }},
		{"home", "jump to the start", func(a *App) { a.seek(0) }},
		{"up", "volume up", nudger("volume", 1)},
		{"down", "volume down", nudger("volume", -1)},
		{"m", "mute", nudger("mute", 1)},
		{"] [", "speed up / down", nil},
		{"l", "loop", nudger("loop", 1)},

		{"ctrl+= ctrl++ ctrl+shift+= ctrl+shift++ z", "picture bigger (interface unchanged)", func(a *App) { a.zoom(1) }},
		{"ctrl+- ctrl+_ ctrl+shift+- ctrl+shift+_ Z", "picture smaller", func(a *App) { a.zoom(-1) }},
		{"ctrl+0 \\", "picture size back to 100 %", func(a *App) { a.zoom(0) }},
		{"alt+left alt+right alt+up alt+down", "pan a zoomed picture", nil},
		{"alt+= alt++ alt+shift+= alt+shift++", "interface bigger (picture unchanged)", func(a *App) { a.setUI(a.ui + 1) }},
		{"alt+- alt+_ alt+shift+- alt+shift+_", "interface smaller", func(a *App) { a.setUI(a.ui - 1) }},
		{"alt+0", "interface size back to normal", func(a *App) { a.setUI(1) }},

		{"r", "RANDOMIZE the whole look", func(a *App) { a.randomize(false) }},
		{"R", "randomize the palette only", func(a *App) { a.randomize(true) }},
		{"u U", "undo the last change", func(a *App) { a.undo() }},
		{"tab f2 s S", "settings menu", func(a *App) { a.toggleMenu(-1) }},
		{"p P", "next / previous preset", nil},
		{"ctrl+s", "save the look as a preset", func(a *App) { a.savePresetPrompt() }},

		{"v V", "cycle mode", nil},
		{"d D", "cycle dither", nil},
		{"c C", "cycle palette", nil},
		{"a A", "cycle charset", nil},
		{"= -", "more / fewer palette colors", nil},
		{"f", "fit / fill / stretch", nudger("fit", 1)},
		{"x", "flip X", nudger("flip-x", 1)},
		{"y", "flip Y", nudger("flip-y", 1)},
		{"e", "edges", nudger("edges", 1)},
		{"i", "invert", nudger("invert", 1)},

		{"h", "HUD auto / on / off", func(a *App) {
			a.hud = map[string]string{"auto": "on", "on": "off", "off": "auto"}[a.hud]
			a.say("HUD: "+a.hud, time.Second)
		}},
		{"g", "performance stats", func(a *App) { a.stats = !a.stats }},
		{"o", "open a file", func(a *App) { a.openBrowser() }},
		{"n N", "next / previous file", nil},
		{"f1 ?", "this help", func(a *App) { a.help, a.helpTop = true, 0 }},
		{"esc", "close the menu", func(a *App) { a.menu = nil }},
		{"q Q", "quit", func(a *App) { a.quit = true }},
	}
	g.pair("]", "[", nudger("speed", 1), nudger("speed", -1))
	g.pair("p", "P", func(a *App) { a.cyclePreset(1) }, func(a *App) { a.cyclePreset(-1) })
	g.pair("v", "V", nudger("mode", 1), nudger("mode", -1))
	g.pair("d", "D", nudger("dither", 1), nudger("dither", -1))
	g.pair("c", "C", nudger("palette", 1), nudger("palette", -1))
	g.pair("a", "A", nudger("charset", 1), nudger("charset", -1))
	g.pair("= +", "- _", nudger("colors", 1), nudger("colors", -1))
	g.pair("n", "N", func(a *App) { a.openNext(1) }, func(a *App) { a.openNext(-1) })
	g.pair("alt+right", "alt+left", func(a *App) { a.pan(1, 0) }, func(a *App) { a.pan(-1, 0) })
	g.pair("alt+down", "alt+up", func(a *App) { a.pan(0, 1) }, func(a *App) { a.pan(0, -1) })
	for d := 0; d <= 9; d++ {
		d := d
		g.binds = append(g.binds, bind{fmt.Sprint(d), "", func(a *App) {
			if a.info.Duration > 0 {
				a.seek(a.info.Duration * float64(d) / 10)
			}
		}})
	}
	g.binds = append(g.binds, bind{"0 … 9", "jump to 0 – 90 %", nil})
}

func allLayers() []*layer { return []*layer{globalLayer} }

// layers lists the layers that apply right now, most specific first.
func (a *App) layers() []*layer { return []*layer{globalLayer} }

// playerKey runs the action bound to a key press.
func (a *App) playerKey(ev tty.Event) {
	name := keyName(ev)
	if name == "" {
		return
	}
	for _, l := range a.layers() {
		if b := l.find(name); b != nil {
			b.fn(a)
			return
		}
	}
}

// --- actions that only the keymap uses ----------------------------------------

func (a *App) openNext(d int) {
	if len(a.files) > 1 {
		a.open(a.fileIdx+d, 0)
	}
}

func (a *App) stepFrame(d int) {
	if !a.loaded || a.info.Still {
		return
	}
	if a.isPlaying() {
		a.setPlaying(false)
	}
	if d > 0 {
		a.needFrame = true
	} else {
		a.seek(a.pos() - 1/a.fps())
	}
}

// zoom changes the size of the picture and leaves the interface alone.
// dir 0 resets it.
func (a *App) zoom(dir int) {
	z := a.s.Zoom
	if z <= 0 {
		z = 1
	}
	switch {
	case dir == 0:
		z, a.s.PanX, a.s.PanY = 1, 0, 0
	default:
		z = math.Round(z*math.Pow(1.1, float64(dir))*100) / 100
		if math.Abs(z-1) < 0.04 {
			z = 1
		}
	}
	a.s.Zoom = math.Max(0.1, math.Min(8, z))
	a.changed()
	a.say(fmt.Sprintf("picture %.0f %%", a.s.Zoom*100), 1200*time.Millisecond)
}

func (a *App) pan(dx, dy int) {
	a.s.PanX = math.Max(-1, math.Min(1, a.s.PanX+float64(dx)*0.1))
	a.s.PanY = math.Max(-1, math.Min(1, a.s.PanY+float64(dy)*0.1))
	a.changed()
	if a.key.cropX >= 0.999 && a.key.cropY >= 0.999 {
		a.say("nothing to pan — zoom in first (z or Ctrl +)", 1500*time.Millisecond)
	}
}

var uiNames = []string{"compact", "normal", "large", "huge"}

// UIScale turns an interface size name into its level, or -1.
func UIScale(name string) int {
	for i, n := range uiNames {
		if strings.EqualFold(n, name) {
			return i
		}
	}
	return -1
}

// setUI changes the size of the interface and leaves the picture alone.
func (a *App) setUI(n int) {
	a.ui = max(0, min(len(uiNames)-1, n))
	if a.menu != nil {
		a.menu.placed = false
	}
	a.say("interface: "+uiNames[a.ui], 1200*time.Millisecond)
}

// dim scales a window size to the interface size and clips it to the screen.
func (a *App) dim(w, h int) (int, int) {
	f := []float64{0.82, 1, 1.3, 1.7}[max(0, min(3, a.ui))]
	return min(a.scr.W, int(float64(w)*f+0.5)), min(a.scr.H, int(float64(h)*f+0.5))
}

// pad is the extra space, in cells, that controls get at large sizes.
func (a *App) pad() int { return max(0, a.ui-1) }
