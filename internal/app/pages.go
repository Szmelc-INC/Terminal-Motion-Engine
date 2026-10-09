package app

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/media"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
)

// listPage is a page of two-column text that scrolls: the keys of a layer
// and what they do, or the facts about what is playing. A line without a
// first column is a heading.
type listPage struct {
	name   string
	mode   int // the bind mode the page lists, or -1
	lines  func(a *App) [][2]string
	scroll int
}

func (p *listPage) label() string { return p.name }

// tint is the color of the page's tab: that of the mode it lists.
func (p *listPage) tint() (uint32, bool) {
	if p.mode < 0 {
		return 0, false
	}
	return modeColor(p.mode), true
}

func (p *listPage) hint() string {
	if p.mode >= 0 {
		return "↑↓ scroll · ←→ other pages · Enter use these keys · Esc close"
	}
	return "↑↓ scroll · ←→ other pages · Esc close"
}

func (p *listPage) draw(a *App, m *Menu, x, y, w, h int) {
	s := a.scr
	lines := p.lines(a)
	p.scroll = max(0, min(p.scroll, len(lines)-h))
	a.on(x+1, y, w-2, h, func(ev tty.Event, _, _ int) {
		switch ev.Action {
		case tty.MouseWheelUp:
			p.scroll -= 3
		case tty.MouseWheelDown:
			p.scroll += 3
		}
	})
	head := cAccent
	if c, ok := p.tint(); ok {
		head = c
	}
	kw := 20 + (w-66)/4
	for i := 0; i < h && p.scroll+i < len(lines); i++ {
		l := lines[p.scroll+i]
		if l[0] == "" {
			s.Text(x+2, y+i, tty.Clean(l[1]), head, cPanel, engine.AttrBold, w-4)
			continue
		}
		s.Text(x+2, y+i, l[0], cYellow, cPanel, 0, kw-1)
		s.Text(x+2+kw, y+i, tty.Clean(l[1]), cFg, cPanel, 0, w-4-kw)
	}
	if len(lines) > h {
		s.Text(x+w-12, y+h+1, fmt.Sprintf(" %d/%d ↕ ", min(len(lines), p.scroll+h), len(lines)), cDim, cPanel, 0, -1)
	}
	if p.mode < 0 {
		return
	}
	if p.mode == a.mode {
		s.Text(x+2, y+h, "● these keys are active", modeColor(p.mode), cPanel, engine.AttrBold, w-4)
		return
	}
	a.button(x+2, y+h, "Use these keys", modeColor(p.mode), cBtn, func(ev tty.Event) {
		if clicked(ev) {
			a.setMode(p.mode)
		}
	})
}

func (p *listPage) key(a *App, m *Menu, ev tty.Event) bool {
	switch ev.Key {
	case tty.KeyUp:
		p.scroll--
	case tty.KeyDown:
		p.scroll++
	case tty.KeyPgUp:
		p.scroll -= 10
	case tty.KeyPgDn:
		p.scroll += 10
	case tty.KeyHome:
		p.scroll = 0
	case tty.KeyEnd:
		p.scroll = 1 << 20
	case tty.KeyLeft:
		m.setTab(m.tab - 1)
	case tty.KeyRight:
		m.setTab(m.tab + 1)
	case tty.KeyEnter:
		if p.mode < 0 {
			return false
		}
		a.setMode(p.mode)
	default:
		return false
	}
	return true
}

// bindLines lists the binds of a layer that have a description.
func bindLines(l *layer) [][2]string {
	var out [][2]string
	for _, b := range l.binds {
		if b.desc != "" {
			out = append(out, [2]string{prettyKeys(b), b.desc})
		}
	}
	return out
}

// modeLines is the keys page of a bind mode.
func modeLines(i int) func(a *App) [][2]string {
	return func(a *App) [][2]string {
		m := modes[i]
		out := [][2]string{{"", strings.ToUpper(m.name) + " — " + m.about}}
		out = append(out, bindLines(m)...)
		what := "the settings these keys change go back to normal"
		switch m.name {
		case "play":
			what = "speed, sync, frame rate and picture size back to normal"
		case "fx":
			what = "all effects off"
		case "audio":
			what = "clean sound"
		}
		panel := m.panel
		if p := findPanel(m.panel); p != nil {
			panel = p.title
		}
		return append(out,
			[2]string{"", "The same in every mode"},
			[2]string{"a   A", "a small letter goes up or on, its capital goes back"},
			[2]string{"=   -", "repeat the last setting key: up / down"},
			[2]string{"Enter", "open the panel: " + panel},
			[2]string{"Backspace", what},
			[2]string{"Tab  Shift+Tab", "next / previous mode (also ` and ~)"},
			[2]string{fmt.Sprintf("Alt+%d", i+1), "switch to these keys from anywhere"})
	}
}

func coreLines(*App) [][2]string {
	return append([][2]string{{"", "These keys work in every mode"}}, bindLines(core)...)
}

func mouseLines(*App) [][2]string {
	return [][2]string{
		{"", "Picture"},
		{"click", "play / pause"},
		{"right-click", "the panel of the active mode"},
		{"middle-click", "randomize the look"},
		{"wheel", "volume"},
		{"Ctrl+wheel", "picture size"},
		{"Alt+wheel", "interface size"},
		{"", "Bars"},
		{"mode names", "click to switch the bind mode, wheel to step through them"},
		{"key hints", "click runs the key, right-click runs it backwards, wheel steps"},
		{"F1 … F12 strip", "click opens the window (shown while one is open)"},
		{"buttons", "click = next, right-click = previous; right-click the dice = undo"},
		{"seek bar", "click or drag to seek, wheel to step 5 s"},
		{"", "Windows"},
		{"title bar", "drag to move, ‹ › for the neighbouring windows, ✕ to close"},
		{"sliders", "click or drag; wheel over a row changes it"},
		{"lists", "click selects, double-click runs the first action"},
	}
}

// KeySection is the key list of one layer, for "termo keys".
type KeySection struct {
	Name, Title, About string
	Lines              [][2]string
}

// KeyTable lists every key binding, the bind modes first.
func KeyTable() []KeySection {
	var out []KeySection
	for _, m := range append(append([]*layer{}, modes...), core) {
		out = append(out, KeySection{m.name, m.title, m.about, bindLines(m)})
	}
	return out
}

// --- what is playing -----------------------------------------------------------

func nowLines(a *App) [][2]string {
	if !a.loaded {
		return [][2]string{{"", "Nothing is playing — o opens a file, / finds media on the web."}}
	}
	i := a.info
	onOff := func(b bool, yes, no string) string {
		if b {
			return yes
		}
		return no
	}
	length := "a still picture"
	if !i.Still {
		length = fmtTime(i.Duration)
		if i.Frames > 0 {
			length += fmt.Sprintf(" · %d frames", i.Frames)
		}
	}
	sound := onOff(i.HasAudio, "yes", "none")
	if i.AudioPath != "" {
		sound = "yes, a separate stream"
	}
	out := "no sound in this file"
	switch {
	case a.aud != nil:
		out = a.aud.Name()
	case a.audErr != "":
		out = "off — " + a.audErr
	case a.cfg.NoAudio:
		out = "off (--no-audio)"
	}
	heard := "clean"
	if a.s.Sound.SoundActive() {
		heard = engine.SoundSummary(a.s.Sound)
		if a.soundName != "" {
			heard = a.soundName + " — " + heard
		}
	}
	z := a.s.Zoom
	if z <= 0 {
		z = 1
	}
	return [][2]string{
		{"", "File"},
		{"name", i.Name},
		{"source", i.Path},
		{"playlist", fmt.Sprintf("%d of %d", a.fileIdx+1, len(a.files))},
		{"", "Stream"},
		{"picture", fmt.Sprintf("%d × %d, %.4g fps, %s", i.Width, i.Height, i.FPS, i.Codec)},
		{"length", length},
		{"sound", sound},
		{"", "On screen"},
		{"position", fmtTime(a.pos()) + onOff(a.isPlaying(), "", " (paused)")},
		{"speed", trim(a.s.Speed) + "×" + onOff(a.s.Loop, " · looping", "")},
		{"grid", fmt.Sprintf("%d × %d cells from %d × %d pixels", a.ccols, a.crows, a.key.w, a.key.h)},
		{"picture size", fmt.Sprintf("%.0f %%", z*100)},
		{"render", fmt.Sprintf("%.2f ms a frame · %.1f fps shown · %d dropped", a.renderMs, a.fpsNow, a.dropped)},
		{"look", Summary(a.s.Look)},
		{"", "Sound"},
		{"heard as", heard},
		{"played by", out},
	}
}

// --- the playlist ----------------------------------------------------------------

// fileName is what a playlist entry is called on screen.
func (a *App) fileName(path string) string {
	if m, ok := a.meta[path]; ok && m.name != "" {
		return m.name
	}
	return filepath.Base(path)
}

// moveFile moves a playlist entry up or down and keeps the playing one
// playing.
func (a *App) moveFile(i, d int) bool {
	j := i + d
	if i < 0 || j < 0 || i >= len(a.files) || j >= len(a.files) {
		return false
	}
	a.files[i], a.files[j] = a.files[j], a.files[i]
	switch a.fileIdx {
	case i:
		a.fileIdx = j
	case j:
		a.fileIdx = i
	}
	return true
}

// removeFile takes an entry off the playlist. Removing what is playing
// moves on to the entry after it.
func (a *App) removeFile(i int) {
	if i < 0 || i >= len(a.files) {
		return
	}
	if len(a.files) == 1 {
		a.say("that is the only file — open or find another one first", 2500*time.Millisecond)
		return
	}
	name := a.fileName(a.files[i])
	a.files = append(a.files[:i:i], a.files[i+1:]...)
	switch {
	case i < a.fileIdx:
		a.fileIdx--
	case i == a.fileIdx && a.loaded:
		a.open(i%len(a.files), 0)
	}
	a.fileIdx = min(a.fileIdx, len(a.files)-1)
	a.say("removed "+name+" from the playlist", 1500*time.Millisecond)
}

func newPlaylistManager() *managerPage {
	p := &managerPage{name: "Playlist"}
	p.items = func(a *App) []mItem {
		var out []mItem
		for i, f := range a.files {
			it := mItem{id: i, name: a.fileName(f), tag: "file", info: filepath.Dir(f), active: a.loaded && i == a.fileIdx}
			if media.IsURL(f) {
				it.tag, it.info = "web", f
				if u, err := url.Parse(f); err == nil && u.Host != "" {
					it.info = u.Host
				}
			}
			out = append(out, it)
		}
		return out
	}
	move := func(d int) func(a *App, it mItem, ok bool) {
		return func(a *App, it mItem, ok bool) {
			if ok && a.moveFile(it.id, d) && p.filter == "" {
				p.sel += d
			}
		}
	}
	p.actions = []mAction{
		{'l', "Play", &cGreen, func(a *App, it mItem, ok bool) {
			if ok {
				a.open(it.id, 0)
			}
		}},
		{'o', "Open…", &cAccent, func(a *App, it mItem, ok bool) { a.openBrowser() }},
		{'f', "Find…", &cAccent, func(a *App, it mItem, ok bool) { a.openF(8) }},
		{'k', "▲ Up", nil, move(-1)},
		{'j', "▼ Down", nil, move(1)},
		{'d', "Remove", &cRed, func(a *App, it mItem, ok bool) {
			if ok {
				a.removeFile(it.id)
			}
		}},
		{'c', "Clear", nil, func(a *App, it mItem, ok bool) {
			if len(a.files) < 2 || !a.loaded {
				return
			}
			a.confirm = &Confirm{msg: "Take everything but the playing file off the playlist?", yes: func() {
				a.files, a.fileIdx = []string{a.files[a.fileIdx]}, 0
				a.ok("the playlist is down to one file")
			}}
		}},
	}
	return p
}

// --- downloads -------------------------------------------------------------------

func newDownloadManager() *managerPage {
	p := &managerPage{name: "Downloads"}
	p.items = func(a *App) []mItem {
		var out []mItem
		for i := len(a.downloads) - 1; i >= 0; i-- {
			d := a.downloads[i]
			it := mItem{id: i, name: d.title, active: !d.done}
			switch {
			case d.done && d.err != nil:
				it.tag, it.info = "failed", d.err.Error()
			case d.done:
				it.tag, it.info = "saved", d.path
			default:
				it.tag, it.info = "…", d.p.Stage
				if d.p.Percent >= 0 {
					it.tag = fmt.Sprintf("%.0f %%", d.p.Percent)
				}
				if d.p.Speed != "" {
					it.info += " · " + d.p.Speed
				}
			}
			out = append(out, it)
		}
		return out
	}
	get := func(a *App, it mItem, ok bool) *download {
		if !ok || it.id < 0 || it.id >= len(a.downloads) {
			return nil
		}
		return a.downloads[it.id]
	}
	p.actions = []mAction{
		{'l', "Play", &cGreen, func(a *App, it mItem, ok bool) {
			d := get(a, it, ok)
			switch {
			case d == nil:
			case !d.done:
				a.say("still downloading — it can play when it is saved", 2*time.Second)
			case d.err != nil || d.path == "":
				a.say("that download did not finish", 2*time.Second)
			case strings.Contains(" .mp3 .wav .m4a .opus .ogg .flac .aac ", " "+strings.ToLower(filepath.Ext(d.path))+" "):
				a.say("a sound file: termo plays pictures — it is in "+filepath.Dir(d.path), 3*time.Second)
			default:
				a.addStream(d.path, "", "", true)
			}
		}},
		{'f', "Find…", &cAccent, func(a *App, it mItem, ok bool) { a.openF(8) }},
		{'o', "Folder", nil, func(a *App, it mItem, ok bool) { a.openBrowserAt(a.prefs.Downloads()) }},
		{'x', "Cancel", &cRed, func(a *App, it mItem, ok bool) {
			if d := get(a, it, ok); d != nil && !d.done {
				d.cancel()
			}
		}},
		{'c', "Clear done", nil, func(a *App, it mItem, ok bool) {
			var left []*download
			for _, d := range a.downloads {
				if !d.done {
					left = append(left, d)
				}
			}
			a.downloads, p.sel = left, 0
		}},
	}
	return p
}
