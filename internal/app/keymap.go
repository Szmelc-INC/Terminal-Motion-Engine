package app

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
)

// bind ties keys to an action. The keys in keys run it forwards; those in
// rev — the Shift twin of a letter, as a rule — run it backwards. Both are
// space-separated lists of key names as keyName produces them.
type bind struct {
	keys, rev string
	short     string // label on the hint row; empty keeps the bind off it
	desc      string // line on the keys page; empty keeps the bind off it
	opt       string // the option the bind steps, if it steps one
	fn        func(a *App, dir int)
}

// layer is a named set of key bindings. The core layer always applies. The
// others are bind modes: one is active at a time, it is asked first, and so
// the same letters do different work in each of them.
type layer struct {
	name, title string
	about       string       // what the mode is for, in a few words
	turn        float64      // hue of the mode: degrees away from the theme's accent
	panel       string       // the panel that belongs to the mode; Enter opens it
	reset       func(a *App) // puts what the mode controls back to normal
	binds       []bind
	index       map[string]target
}

// target is what a key leads to: a bind and the direction to run it in.
type target struct {
	b   *bind
	dir int
}

func (l *layer) find(key string) (target, bool) {
	if l.index == nil {
		l.index = map[string]target{}
		for i := range l.binds {
			b := &l.binds[i]
			if b.fn == nil {
				continue // a line of help
			}
			for _, k := range strings.Fields(b.keys) {
				l.index[k] = target{b, 1}
			}
			for _, k := range strings.Fields(b.rev) {
				l.index[k] = target{b, -1}
			}
		}
	}
	t, ok := l.index[key]
	return t, ok
}

// key binds keys to an action that has no direction.
func key(keys, short, desc string, fn func(a *App)) bind {
	return bind{keys: keys, short: short, desc: desc, fn: func(a *App, _ int) { fn(a) }}
}

// step binds keys to an action that goes two ways.
func step(keys, rev, short, desc string, fn func(a *App, dir int)) bind {
	return bind{keys: keys, rev: rev, short: short, desc: desc, fn: fn}
}

// note is a line of help without an action.
func note(keys, desc string) bind { return bind{keys: keys, desc: desc} }

// opt binds keys to an entry of the option table: a number moves by n of its
// steps, a list goes to its next or previous choice, a switch flips. A
// negative n turns the keys around, for settings where less is more.
func opt(keys, rev, option, short string, n int) bind {
	o := engine.FindOption(option)
	if o == nil {
		panic("keymap: no option named " + option)
	}
	desc := o.Label
	switch {
	case o.Kind == engine.KBool:
		desc += " on / off"
	case o.Kind == engine.KEnum && rev == "":
		desc += ": next choice"
	case o.Kind == engine.KEnum:
		desc += ": next / previous"
	case n < 0:
		desc += " down / up"
	default:
		desc += " up / down"
	}
	return bind{keys: keys, rev: rev, short: short, desc: desc, opt: option,
		fn: func(a *App, dir int) { a.nudge(option, dir*n) }}
}

// named gives a bind another line on the keys page, where the label of its
// option says too little out of its panel.
func named(b bind, desc string) bind { b.desc = desc; return b }

// geo binds keys to a number that is better moved by a ratio than by a step:
// a frequency. factor is what the forward keys multiply it by.
func geo(keys, rev, option, short, desc string, factor float64) bind {
	o := engine.FindOption(option)
	if o == nil {
		panic("keymap: no option named " + option)
	}
	return bind{keys: keys, rev: rev, short: short, desc: desc, opt: option, fn: func(a *App, dir int) {
		span := o.Max - o.Min
		v := o.Min + o.Frac(&a.s)*span
		nv := v * math.Pow(factor, float64(dir))
		// Near zero a ratio gets nowhere: move by at least three steps.
		if least := 3 * o.Step; math.Abs(nv-v) < least {
			nv = v + least*float64(dir)*math.Copysign(1, factor-1)
		}
		a.touch()
		o.SetFrac(&a.s, (nv-o.Min)/span)
		a.changed()
		a.say(o.Label+": "+o.String(&a.s), 1200*time.Millisecond)
	}}
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

var keyPretty = strings.NewReplacer("left", "←", "right", "→", "up", "↑", "down", "↓", "space", "Space",
	"ctrl+", "Ctrl+", "alt+", "Alt+", "shift+", "Shift+", "esc", "Esc", "tab", "Tab", "home", "Home",
	"enter", "Enter", "pgup", "PgUp", "pgdn", "PgDn", "backspace", "Backspace")

// prettyKey formats one key name for the screen.
func prettyKey(k string) string {
	if len(k) > 1 && k[0] == 'f' && k[1] >= '1' && k[1] <= '9' {
		return "F" + k[1:]
	}
	return keyPretty.Replace(k)
}

// prettyKeys formats a bind's keys for the keys page: the first two of a
// bind without a direction, or the first of each direction.
func prettyKeys(b bind) string {
	f, r := strings.Fields(b.keys), strings.Fields(b.rev)
	switch {
	case len(r) > 0 && len(f) > 0:
		f = []string{f[0], r[0]}
	case len(f) > 2 && b.fn != nil:
		f = f[:2]
	}
	for i := range f {
		f[i] = prettyKey(f[i])
	}
	return strings.Join(f, " ")
}

// --- the core layer -----------------------------------------------------------

// core holds the keys that work in every mode. It owns no plain letter that
// a mode would want: the transport, the windows and the handful of letters
// that had better mean the same thing everywhere.
var core = &layer{name: "core", title: "Everywhere"}

// modes are the bind modes, in the order Tab steps through them.
var modes []*layer

func init() {
	core.binds = []bind{
		key("space", "", "play / pause", func(a *App) { a.setPlaying(!a.isPlaying()) }),
		step("right", "left", "", "seek 5 s forward / back", func(a *App, d int) { a.seekBy(float64(5 * d)) }),
		step("shift+right", "shift+left", "", "seek 30 s forward / back", func(a *App, d int) { a.seekBy(float64(30 * d)) }),
		step(".", ",", "", "next / previous frame", func(a *App, d int) { a.stepFrame(d) }),
		key("home", "", "jump to the start", func(a *App) { a.seek(0) }),
		note("0 … 9", "jump to 0 – 90 %"),
		step("up", "down", "", "volume up / down", func(a *App, d int) { a.nudge("volume", d) }),
		key("m", "", "mute", func(a *App) { a.nudge("mute", 1) }),
		step("]", "[", "", "speed up / down", func(a *App, d int) { a.nudge("speed", d) }),

		step("tab `", "shift+tab ~", "", "next / previous bind mode", func(a *App, d int) { a.setMode(a.mode + d) }),
		note("alt+1 … alt+5", "bind mode: play · video · color · fx · audio"),
		key("enter", "", "open the panel of the active mode", func(a *App) { a.openPanel(modes[a.mode].panel, -1) }),
		key("backspace", "", "reset what the active mode controls", func(a *App) { a.resetMode() }),
		step("= +", "- _", "", "repeat the last setting key: up / down", func(a *App, d int) { a.again(d) }),

		key("f1 ?", "", "keys: this list, a page for every mode", func(a *App) { a.openF(0) }),
		key("f2", "", "picture: render mode, colors, dither", func(a *App) { a.openF(1) }),
		key("f3", "", "adjust & filters: light, color, grade", func(a *App) { a.openF(2) }),
		key("f4", "", "effects: VHS, CRT, glitch, 3D + presets", func(a *App) { a.openF(3) }),
		key("f5", "", "sound: tone, equaliser, dynamics, space", func(a *App) { a.openF(4) }),
		key("f6", "", "sound effects: pitch, lo-fi, synth", func(a *App) { a.openF(5) }),
		key("f7", "", "presets: looks, sounds and effects", func(a *App) { a.openF(6) }),
		key("f8", "", "palettes & symbols: manage, generate", func(a *App) { a.openF(7) }),
		key("f9 /", "", "find media on the web: search, download", func(a *App) { a.openF(8) }),
		key("f10", "", "media: playlist and downloads", func(a *App) { a.openF(9) }),
		key("f11", "", "playback: speed, size, sync, what is playing", func(a *App) { a.openF(10) }),
		key("f12", "", "preferences & themes", func(a *App) { a.openF(11) }),
		step(">", "<", "", "next / previous panel (when F keys are taken)", func(a *App, d int) { a.stepPanel(d) }),

		key("ctrl+= ctrl++ ctrl+shift+= ctrl+shift++ z", "", "picture bigger (interface unchanged)", func(a *App) { a.zoom(1) }),
		key("ctrl+- ctrl+_ ctrl+shift+- ctrl+shift+_ Z", "", "picture smaller", func(a *App) { a.zoom(-1) }),
		key("ctrl+0 \\", "", "picture size back to 100 %", func(a *App) { a.zoom(0) }),
		note("alt+← alt+→ alt+↑ alt+↓", "pan a zoomed picture"),
		key("alt+= alt++ alt+shift+= alt+shift++", "", "interface bigger (picture unchanged)", func(a *App) { a.setUI(a.ui + 1) }),
		key("alt+- alt+_ alt+shift+- alt+shift+_", "", "interface smaller", func(a *App) { a.setUI(a.ui - 1) }),
		key("alt+0", "", "interface size back to normal", func(a *App) { a.setUI(1) }),

		key("u U", "", "undo the last change", func(a *App) { a.undo() }),
		key("ctrl+s", "", "save the look as a preset", func(a *App) { a.savePresetPrompt() }),
		key("h", "", "HUD auto / on / off", func(a *App) {
			a.hud = map[string]string{"auto": "on", "on": "off", "off": "auto"}[a.hud]
			a.savePrefs()
			a.say("HUD: "+a.hud, time.Second)
		}),
		key("o", "", "open a file", func(a *App) { a.openBrowser() }),
		key("esc", "", "close the panel", func(a *App) { a.menu = nil }),
		note("ctrl+l", "redraw the screen"),
		key("q Q", "", "quit (Ctrl+C too)", func(a *App) { a.quit = true }),

		step("alt+right", "alt+left", "", "", func(a *App, d int) { a.pan(d, 0) }),
		step("alt+down", "alt+up", "", "", func(a *App, d int) { a.pan(0, d) }),
	}
	for d := 0; d <= 9; d++ {
		d := d
		core.binds = append(core.binds, key(fmt.Sprint(d), "", "", func(a *App) {
			if a.info.Duration > 0 {
				a.seek(a.info.Duration * float64(d) / 10)
			}
		}))
	}

	play := &layer{name: "play", title: "Playback", turn: 0, panel: "playback",
		about: "transport, playlist, speed and sync; r and p work on the whole look"}
	play.reset = func(a *App) {
		a.s.Speed, a.s.AudioDelay, a.s.FPS, a.s.Zoom, a.s.PanX, a.s.PanY = 1, 0, 0, 1, 0, 0
		a.changed()
		a.say("playback: speed, sync, frame rate and picture size back to normal", 2*time.Second)
	}
	play.binds = []bind{
		step("n", "N", "next", "next / previous file", func(a *App, d int) { a.openNext(d) }),
		opt("s", "S", "speed", "speed", 1),
		opt("l", "", "loop", "loop", 1),
		key("k", "", "play / pause", func(a *App) { a.setPlaying(!a.isPlaying()) }),
		key("x", "shuffle", "play a random file of the playlist", func(a *App) { a.shuffle() }),
		opt("a", "A", "audio-delay", "sync", 5),
		opt("f", "F", "fps", "fps cap", 1),
		step("p", "P", "look", "next / previous look preset", func(a *App, d int) { a.cyclePreset(d) }),
		key("r", "random", "RANDOMIZE the whole look", func(a *App) { a.randomize(false) }),
		key("R", "", "randomize the palette only", func(a *App) { a.randomize(true) }),
		key("i", "info", "what is playing: size, rate, codec", func(a *App) { a.showPage(pgNow) }),
		key("e", "list", "the playlist", func(a *App) { a.showPage(pgPlaylist) }),
		key("g", "stats", "performance statistics", func(a *App) { a.stats = !a.stats; a.savePrefs() }),
	}

	video := &layer{name: "video", title: "Picture", turn: 150, panel: "picture",
		about: "how the picture is drawn: glyphs, palette, dither, geometry"}
	video.binds = []bind{
		opt("v", "V", "mode", "mode", 1),
		opt("c", "C", "palette", "palette", 1),
		opt("d", "D", "dither", "dither", 1),
		opt("a", "A", "charset", "charset", 1),
		step("p", "P", "preset", "next / previous look preset", func(a *App, d int) { a.cyclePreset(d) }),
		key("r", "random", "RANDOMIZE the whole look", func(a *App) { a.randomize(false) }),
		key("R", "", "randomize the palette only", func(a *App) { a.randomize(true) }),
		opt("k", "K", "colors", "colors", 1),
		named(opt("w", "W", "dither-amount", "dither amt", 1), "Dither amount up / down"),
		opt("s", "S", "scheme", "scheme", 1),
		opt("j", "J", "hue", "hue", 3),
		opt("t", "", "color", "color", 1),
		opt("b", "B", "background", "backgr.", 1),
		opt("f", "F", "fit", "fit", 1),
		opt("x", "", "flip-x", "flip x", 1),
		opt("y", "", "flip-y", "flip y", 1),
		opt("g", "G", "mirror", "mirror", 1),
		opt("e", "E", "edges", "edges", 1),
		opt("i", "", "invert", "invert", 1),
	}

	color := &layer{name: "color", title: "Color & light", turn: 290, panel: "adjust",
		about: "tone and grade: light, contrast, color balance, filters"}
	color.binds = []bind{
		opt("b", "B", "brightness", "bright", 2),
		opt("c", "C", "contrast", "contrast", 1),
		opt("g", "G", "gamma", "gamma", 1),
		opt("s", "S", "saturation", "satur.", 1),
		opt("p", "P", "filter", "filter", 1),
		opt("f", "F", "filter-amount", "filter amt", 2),
		key("r", "random", "a random filter", func(a *App) { a.randomGrade() }),
		opt("e", "E", "exposure", "exposure", 1),
		opt("t", "T", "temperature", "temp.", 1),
		opt("i", "I", "tint", "tint", 1),
		opt("j", "J", "hue-shift", "hue", 3),
		opt("v", "V", "vibrance", "vibrance", 1),
		opt("d", "D", "shadows", "shadows", 1),
		opt("l", "L", "highlights", "lights", 1),
		opt("k", "K", "black", "black pt", 2),
		opt("w", "W", "white", "white pt", -2),
		opt("a", "A", "fade", "fade", 1),
		opt("x", "X", "sharpen", "sharp", 2),
		opt("y", "Y", "vignette", "vignette", 2),
		opt("n", "N", "glow", "glow", 2),
	}

	fx := &layer{name: "fx", title: "Effects", turn: 75, panel: "effects",
		about: "tape, tube, glitch and time effects, and their presets"}
	fx.reset = func(a *App) { a.setFX(engine.BuiltinFX[0]) }
	fx.binds = []bind{
		step("p", "P", "preset", "next / previous effect preset", func(a *App, d int) { a.cycleFX(d) }),
		key("r", "random", "random effects", func(a *App) { a.randomFX() }),
		opt("v", "V", "vhs", "vhs", 2),
		opt("s", "S", "scanlines", "scanlines", 2),
		opt("c", "C", "curvature", "crt", 2),
		opt("g", "G", "glitch", "glitch", 2),
		opt("a", "A", "split", "split", 5),
		opt("d", "D", "split-mode", "3d", 1),
		step("e", "E", "poster", "Posterize: fewer / more levels", func(a *App, d int) { a.posterize(d) }),
		opt("f", "F", "blur", "blur", 1),
		opt("l", "L", "motion-blur", "trails", 2),
		opt("x", "X", "pixelate", "pixels", 1),
		opt("k", "K", "mask", "mask", 1),
		opt("t", "T", "tracking", "tracking", 2),
		opt("b", "B", "bleed", "bleed", 2),
		opt("j", "J", "jitter", "jitter", 2),
		opt("w", "W", "wave", "wave", 2),
		opt("n", "N", "grain", "grain", 2),
		opt("y", "Y", "hue-cycle", "hue spin", 2),
		opt("i", "", "interlace", "interlace", 1),
	}

	audio := &layer{name: "audio", title: "Sound", turn: 215, panel: "sound",
		about: "tone, space, dirt and synth, and the sound presets"}
	audio.reset = func(a *App) { a.setSound(engine.BuiltinSounds[0]) }
	audio.binds = []bind{
		step("p", "P", "preset", "next / previous sound preset", func(a *App, d int) { a.cycleSound(d) }),
		key("r", "random", "a random sound", func(a *App) { a.randomSound() }),
		opt("b", "B", "bass", "bass", 2),
		opt("t", "T", "treble", "treble", 2),
		opt("d", "D", "mid", "mid", 2),
		opt("g", "G", "preamp", "gain", 2),
		opt("f", "F", "pitch", "pitch", 1),
		opt("v", "V", "reverb", "reverb", 2),
		opt("e", "E", "echo-delay", "echo", 5),
		geo("l", "L", "lowpass", "low-pass", "Low-pass: darker / brighter", 0.8),
		geo("k", "K", "highpass", "high-pass", "High-pass: thinner / fuller", 1.25),
		opt("a", "A", "drive", "drive", 3),
		opt("x", "X", "bits", "crush", -1),
		geo("s", "S", "sample-rate", "rate", "Sample rate: lower / higher", 0.8),
		opt("j", "J", "tremolo", "tremolo", 2),
		opt("w", "W", "width", "width", 2),
		opt("y", "Y", "synth", "synth", 1),
		opt("c", "", "comp", "compress", 1),
		opt("i", "", "limiter", "limiter", 1),
		opt("n", "", "normalize", "normalize", 1),
	}

	modes = []*layer{play, video, color, fx, audio}
	for i := range modes {
		i := i
		core.binds = append(core.binds, key(fmt.Sprintf("alt+%d", i+1), "", "", func(a *App) { a.setMode(i) }))
	}
}

// allLayers lists every layer; layers lists those that apply right now, the
// one that is asked first in front.
func allLayers() []*layer { return append([]*layer{core}, modes...) }

func (a *App) layers() []*layer { return []*layer{modes[a.mode], core} }

// findMode returns the index of the mode with that name, or -1.
func findMode(name string) int {
	for i, m := range modes {
		if strings.EqualFold(m.name, name) {
			return i
		}
	}
	return -1
}

// ModeNames lists the bind modes.
func ModeNames() []string {
	var out []string
	for _, m := range modes {
		out = append(out, m.name)
	}
	return out
}

// playerKey runs the action bound to a key press: the active mode is asked
// first, then the core layer.
func (a *App) playerKey(ev tty.Event) {
	name := keyName(ev)
	if name == "" {
		return
	}
	for _, l := range a.layers() {
		if t, ok := l.find(name); ok {
			if t.b.rev != "" && l != core {
				a.lastBind = t.b
			}
			t.b.fn(a, t.dir)
			return
		}
	}
}

// --- modes ---------------------------------------------------------------------

// modeTints caches the mode colors of one accent color.
var modeTints struct {
	accent uint32
	set    bool
	colors []uint32
}

// modeColor is the color that stands for a bind mode: the accent of the
// theme with its hue turned, so the five keep their distance on any theme.
// It follows the theme, hence it is worked out when asked for.
func modeColor(i int) uint32 {
	if !modeTints.set || modeTints.accent != cAccent || len(modeTints.colors) != len(modes) {
		L, ca, cb := engine.ToOklab(engine.FromU32(cAccent))
		C, hue := math.Hypot(ca, cb), math.Atan2(cb, ca)*180/math.Pi
		modeTints.accent, modeTints.set, modeTints.colors = cAccent, true, nil
		for _, m := range modes {
			if m.turn == 0 {
				modeTints.colors = append(modeTints.colors, cAccent)
				continue
			}
			// Enough chroma to tell the hues apart, and not so light or so
			// dark that the hue drowns: a gray or a white accent still
			// gives five different colors.
			c := engine.FromOklch(math.Max(0.52, math.Min(0.8, L)), math.Max(0.12, math.Min(0.17, C)), hue+m.turn)
			modeTints.colors = append(modeTints.colors, c.U32())
		}
	}
	return modeTints.colors[(i%len(modes)+len(modes))%len(modes)]
}

// tint is the color of the active mode.
func (a *App) tint() uint32 { return modeColor(a.mode) }

// setMode switches the bind mode and says what its keys are for.
func (a *App) setMode(i int) {
	n := len(modes)
	a.mode = (i%n + n) % n
	a.lastBind = nil
	m := modes[a.mode]
	a.sayTint(strings.ToUpper(m.name)+" keys — "+m.about+"  ·  F1 lists them", 2500*time.Millisecond)
	if a.prefs != nil && a.prefs.Keys == "last" {
		a.savePrefs()
	}
}

// follow makes the mode of a panel the active one, quietly: the keys go
// where the user is looking.
func (a *App) follow(mode string) {
	if i := findMode(mode); i >= 0 && i != a.mode {
		a.mode, a.lastBind = i, nil
	}
}

// again repeats the last two-way key of the mode, so a setting picked with
// its letter can be walked up and down with = and -.
func (a *App) again(dir int) {
	if a.lastBind == nil {
		a.say("- and = repeat a setting key: press one first ("+a.someKeys()+")", 2500*time.Millisecond)
		return
	}
	a.lastBind.fn(a, dir)
}

// someKeys names a few two-way keys of the active mode, as examples.
func (a *App) someKeys() string {
	var out []string
	for _, b := range modes[a.mode].binds {
		if b.rev != "" && b.short != "" && len(out) < 3 {
			out = append(out, prettyKey(strings.Fields(b.keys)[0])+" "+b.short)
		}
	}
	return strings.Join(out, ", ")
}

// resetMode puts what the active mode controls back to normal: the options
// its keys step, unless the mode has a reset of its own.
func (a *App) resetMode() {
	m := modes[a.mode]
	if m.reset != nil {
		m.reset(a)
		return
	}
	a.pushHistory()
	def := engine.DefaultSettings()
	n := 0
	for _, b := range m.binds {
		o := engine.FindOption(b.opt)
		if o == nil || o.Key == "loop" {
			continue
		}
		if want := o.String(&def); o.String(&a.s) != want && o.Set(&a.s, want) == nil {
			n++
		}
	}
	if n == 0 {
		a.history = a.history[:len(a.history)-1]
		a.say(strings.ToLower(m.title)+": nothing to reset", 1500*time.Millisecond)
		return
	}
	a.preset = ""
	a.changed()
	a.say(fmt.Sprintf("%s: %d settings back to normal   (u = undo)", strings.ToLower(m.title), n), 2*time.Second)
}

// --- actions that only the keymap uses ----------------------------------------

func (a *App) openNext(d int) {
	if len(a.files) > 1 {
		a.open(a.fileIdx+d, 0)
	}
}

func (a *App) shuffle() {
	if len(a.files) < 2 {
		a.say("the playlist has one file — o opens more, / finds some", 2*time.Second)
		return
	}
	a.open(a.fileIdx+1+a.rng.Intn(len(a.files)-1), 0)
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

// posterize steps the number of levels per channel. Forwards means a
// stronger effect: fewer levels, starting from eight; one level would be a
// blank picture, so two is where it stops and after the last it is off.
func (a *App) posterize(dir int) {
	a.touch()
	n := a.s.Posterize
	switch {
	case dir > 0 && n == 0:
		n = 8
	case dir > 0:
		n = max(2, n-1)
	case n == 0:
	case n >= 16:
		n = 0
	default:
		n++
	}
	a.s.Posterize = n
	a.changed()
	o := engine.FindOption("posterize")
	a.say(o.Label+": "+o.String(&a.s), 1200*time.Millisecond)
}

// randomGrade picks a color filter at random.
func (a *App) randomGrade() {
	a.touch()
	names := engine.FilterNames()[1:]
	a.s.Filter, a.s.FilterAmount = names[a.rng.Intn(len(names))], 1
	a.changed()
	a.say("filter: "+a.s.Filter+"   (u = undo)", 2*time.Second)
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
	a.savePrefs()
	a.say("interface: "+uiNames[a.ui], 1200*time.Millisecond)
}

// dim scales a window size to the interface size and clips it to the screen.
func (a *App) dim(w, h int) (int, int) {
	f := []float64{0.82, 1, 1.3, 1.7}[max(0, min(3, a.ui))]
	return min(a.scr.W, int(float64(w)*f+0.5)), min(a.scr.H, int(float64(h)*f+0.5))
}

// pad is the extra space, in cells, that controls get at large sizes.
func (a *App) pad() int { return max(0, a.ui-1) }
