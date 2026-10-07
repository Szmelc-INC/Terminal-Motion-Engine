package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
)

// Theme.
const (
	cBar    uint32 = 0x16161e
	cPanel  uint32 = 0x1a1b26
	cFg     uint32 = 0xc0caf5
	cDim    uint32 = 0x565f89
	cAccent uint32 = 0x7aa2f7
	cSel    uint32 = 0x283457
	cHot    uint32 = 0x3b4261
	cGreen  uint32 = 0x9ece6a
	cYellow uint32 = 0xe0af68
	cRed    uint32 = 0xf7768e
	cTrack  uint32 = 0x414868
)

func fmtTime(t float64) string {
	if t < 0 {
		t = 0
	}
	s := int(t)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func (a *App) on(x, y, w, h int, fn func(ev tty.Event, rx, ry int)) {
	a.hits = append(a.hits, hit{x, y, w, h, fn})
}

func (a *App) hover(x, y, w, h int) bool {
	return a.mx >= x && a.mx < x+w && a.my >= y && a.my < y+h
}

func clicked(ev tty.Event) bool {
	return ev.Action == tty.MousePress && ev.Button == tty.ButtonLeft
}

// button draws a clickable label and returns its width.
func (a *App) button(x, y int, label string, fg, bg uint32, fn func(ev tty.Event)) int {
	w := len([]rune(label)) + 2
	if a.hover(x, y, w, 1) {
		bg = cHot
	}
	a.scr.Fill(x, y, w, 1, tty.Cell{Ch: ' ', Bg: bg})
	a.scr.Text(x+1, y, label, fg, bg, 0, -1)
	a.on(x, y, w, 1, func(ev tty.Event, _, _ int) { fn(ev) })
	return w
}

// slider draws a horizontal track with a knob at frac (0..1).
func (a *App) slider(x, y, w int, frac float64, fg, bg uint32) {
	if w < 1 {
		return
	}
	knob := int(frac*float64(w-1) + 0.5)
	for i := 0; i < w; i++ {
		c := tty.Cell{Ch: '━', Fg: fg, Bg: bg}
		if i > knob {
			c.Ch, c.Fg = '─', cTrack
		}
		if i == knob {
			c.Ch, c.Fg = '●', 0xffffff
		}
		a.scr.Set(x+i, y, c)
	}
}

func sliderFrac(rx, w int) float64 {
	if w <= 1 {
		return 0
	}
	f := float64(rx) / float64(w-1)
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func (a *App) hudVisible() bool {
	switch a.hud {
	case "on":
		return true
	case "off":
		return false
	}
	return !a.isPlaying() || a.menu != nil || time.Since(a.lastActivity) < 2500*time.Millisecond
}

func (a *App) draw() {
	s := a.scr
	s.Clear()
	if a.cells != nil {
		s.Blit((s.W-a.ccols)/2, (s.H-a.crows)/2, a.ccols, a.crows, a.cells)
	}
	a.hits = a.hits[:0]
	a.on(0, 0, s.W, s.H, a.videoMouse)
	a.hudShown = a.hudVisible()
	if a.hudShown && s.H >= 6 {
		a.drawHUD()
	}
	if a.stats {
		a.drawStats()
	}
	if a.menu != nil {
		a.menu.draw(a)
	}
	if a.browser != nil {
		a.browser.draw(a)
	}
	if a.help {
		a.drawHelp()
	}
	if a.prompt != nil {
		a.prompt.draw(a)
	}
	if a.confirm != nil {
		a.confirm.draw(a)
	}
	if a.toast != "" {
		msg := []rune(" " + tty.Clean(a.toast) + " ")
		if len(msg) > s.W {
			msg = msg[:s.W]
		}
		y := 1
		if s.H < 4 {
			y = 0
		}
		s.Text((s.W-len(msg))/2, y, string(msg), 0x1a1b26, cYellow, engine.AttrBold, -1)
	}
	n, _ := s.Flush()
	a.flushBytes += (float64(n) - a.flushBytes) * 0.1
	a.dirty = false
}

func (a *App) drawHUD() {
	s := a.scr
	W, H := s.W, s.H

	// Title bar.
	s.Fill(0, 0, W, 1, tty.Cell{Ch: ' ', Bg: cBar})
	x := 1 + s.Text(1, 0, "termo", cAccent, cBar, engine.AttrBold, -1)
	title := " · " + a.title()
	if len(a.files) > 1 {
		title += fmt.Sprintf("  [%d/%d]", a.fileIdx+1, len(a.files))
	}
	if a.loaded {
		title += fmt.Sprintf("  %dx%d %.4g fps", a.info.Width, a.info.Height, a.info.FPS)
	}
	x += s.Text(x, 0, title, cFg, cBar, 0, W-x-1)
	right := Summary(a.s.Look)
	if a.preset != "" {
		right = "◆ " + a.preset + "  " + right
	}
	if rw := len([]rune(right)); x+rw+3 < W {
		s.Text(W-rw-1, 0, right, cDim, cBar, 0, -1)
	}
	a.on(0, 0, W, 1, func(tty.Event, int, int) {})

	// Seek bar.
	y := H - 2
	s.Fill(0, y, W, 2, tty.Cell{Ch: ' ', Bg: cBar})
	pos, dur := a.pos(), a.info.Duration
	left := " " + fmtTime(pos) + " "
	rightT := " " + fmtTime(dur) + " "
	if dur <= 0 {
		rightT = " --:-- "
	}
	lw := s.Text(0, y, left, cFg, cBar, 0, -1)
	s.Text(W-len(rightT), y, rightT, cDim, cBar, 0, -1)
	bx, bw := lw, W-lw-len(rightT)
	frac := 0.0
	if dur > 0 {
		frac = pos / dur
	}
	if frac > 1 {
		frac = 1
	}
	a.slider(bx, y, bw, frac, cAccent, cBar)
	if a.hover(bx, y, bw, 1) && dur > 0 {
		tip := " " + fmtTime(sliderFrac(a.mx-bx, bw)*dur) + " "
		tx := max(0, min(W-len(tip), a.mx-len(tip)/2))
		s.Text(tx, y-1, tip, 0x1a1b26, cAccent, 0, -1)
	}
	a.on(0, y, W, 1, func(ev tty.Event, rx, _ int) {
		switch ev.Action {
		case tty.MousePress, tty.MouseDrag:
			if ev.Button == tty.ButtonLeft && dur > 0 {
				a.seek(sliderFrac(rx-bx, bw) * dur)
			}
		case tty.MouseWheelUp:
			a.seekBy(5)
		case tty.MouseWheelDown:
			a.seekBy(-5)
		}
	})

	// Controls.
	y = H - 1
	a.on(0, y, W, 1, func(tty.Event, int, int) {})
	x = 1
	play := "▶"
	if a.isPlaying() {
		play = "▮▮"
	}
	if a.ended && !a.info.Still {
		play = "↺"
	}
	x += a.button(x, y, play, cGreen, cBar, func(ev tty.Event) {
		if clicked(ev) {
			a.setPlaying(!a.isPlaying())
		}
	})
	x += a.button(x, y, "«5s", cFg, cBar, func(ev tty.Event) {
		if clicked(ev) {
			a.seekBy(-5)
		}
	})
	x += a.button(x, y, "5s»", cFg, cBar, func(ev tty.Event) {
		if clicked(ev) {
			a.seekBy(5)
		}
	})
	if W >= 70 {
		vol := "vol"
		vfg := cFg
		if a.s.Mute || !a.info.HasAudio || a.aud == nil {
			vol, vfg = "mute", cDim
		}
		if !a.info.HasAudio && a.loaded {
			vol = "no audio"
		}
		x += a.button(x, y, vol, vfg, cBar, func(ev tty.Event) {
			if clicked(ev) {
				a.nudge("mute", 1)
			}
		})
		const vw = 8
		vx := x
		a.slider(vx, y, vw, a.s.Volume/1.5, vfg, cBar)
		a.on(vx, y, vw, 1, func(ev tty.Event, rx, _ int) {
			switch ev.Action {
			case tty.MousePress, tty.MouseDrag:
				if ev.Button == tty.ButtonLeft {
					engine.FindOption("volume").SetFrac(&a.s, sliderFrac(rx, vw))
					a.changed()
				}
			case tty.MouseWheelUp:
				a.nudge("volume", 1)
			case tty.MouseWheelDown:
				a.nudge("volume", -1)
			}
		})
		x += vw + 1
		x += s.Text(x, y, fmt.Sprintf("%3.0f%%", a.s.Volume*100), cDim, cBar, 0, -1) + 1
	}

	// Right-aligned buttons; dropped from the left when space runs out.
	type btn struct {
		label string
		fg    uint32
		fn    func(ev tty.Event)
	}
	cycle := func(key string) func(ev tty.Event) {
		return func(ev tty.Event) {
			switch {
			case clicked(ev), ev.Action == tty.MouseWheelDown:
				a.nudge(key, 1)
			case ev.Action == tty.MousePress && ev.Button == tty.ButtonRight, ev.Action == tty.MouseWheelUp:
				a.nudge(key, -1)
			}
		}
	}
	btns := []btn{
		{a.s.Mode, cFg, cycle("mode")},
		{a.s.Dither, cFg, cycle("dither")},
		{a.s.Palette, cFg, cycle("palette")},
		{"⚄ random", cYellow, func(ev tty.Event) {
			if clicked(ev) {
				a.randomize(false)
			} else if ev.Action == tty.MousePress && ev.Button == tty.ButtonRight {
				a.undo()
			}
		}},
		{"☰ menu", cAccent, func(ev tty.Event) {
			if clicked(ev) {
				a.toggleMenu(-1)
			}
		}},
		{"?", cFg, func(ev tty.Event) {
			if clicked(ev) {
				a.help = !a.help
			}
		}},
	}
	total := 0
	for _, b := range btns {
		total += len([]rune(b.label)) + 2
	}
	for len(btns) > 0 && x+total+1 > W {
		total -= len([]rune(btns[0].label)) + 2
		btns = btns[1:]
	}
	bx = W - total - 1
	for _, b := range btns {
		bx += a.button(bx, y, b.label, b.fg, cBar, b.fn)
	}
}

func (a *App) drawStats() {
	lines := []string{
		fmt.Sprintf("%5.1f fps (target %.4g)", a.fpsNow, a.fps()),
		fmt.Sprintf("render %.2f ms", a.renderMs),
		fmt.Sprintf("output %.1f KiB/frame", a.flushBytes/1024),
		fmt.Sprintf("grid %dx%d  src %dx%d", a.ccols, a.crows, a.key.w, a.key.h),
		fmt.Sprintf("dropped %d / %d", a.dropped, a.shown+a.dropped),
	}
	if a.aud != nil {
		lines = append(lines, "audio via "+a.aud.Name())
	} else if a.audErr != "" {
		lines = append(lines, "audio: off")
	}
	w := 0
	for _, l := range lines {
		w = max(w, len([]rune(l)))
	}
	x, y := a.scr.W-w-3, 2
	for i, l := range lines {
		a.scr.Fill(x, y+i, w+2, 1, tty.Cell{Ch: ' ', Bg: cBar})
		a.scr.Text(x+1, y+i, l, cGreen, cBar, 0, -1)
	}
}

var helpText = [][2]string{
	{"Space / k", "play / pause"},
	{"← →   Shift+← →", "seek 5 s / 30 s"},
	{", .", "previous / next frame"},
	{"0-9   Home", "jump to 0-90 % / start"},
	{"↑ ↓   m", "volume / mute"},
	{"[ ]   l", "speed / loop"},
	{"", ""},
	{"r", "RANDOMIZE the whole look"},
	{"R", "randomize palette only"},
	{"u", "undo last change"},
	{"Tab / s / F2", "settings menu"},
	{"p P", "next / previous preset"},
	{"Ctrl+S", "save look as preset"},
	{"", ""},
	{"v d c a", "cycle mode / dither / palette / charset"},
	{"V D C A", "… backwards"},
	{"- =", "fewer / more palette colors"},
	{"f x y", "fit / flip X / flip Y"},
	{"e i", "edges / invert"},
	{"", ""},
	{"h   g", "HUD auto·on·off / stats"},
	{"o   n N", "open file / next · previous file"},
	{"q  Ctrl+C", "quit"},
	{"", ""},
	{"Mouse", "click video = pause · right-click = menu"},
	{"", "wheel = volume · drag bars and sliders"},
	{"", "middle-click = randomize"},
}

func (a *App) window(title string, w, h int) (x, y int) {
	s := a.scr
	x, y = (s.W-w)/2, (s.H-h)/2
	a.frame(title, x, y, w, h)
	return
}

func (a *App) frame(title string, x, y, w, h int) {
	s := a.scr
	s.Dim(x+2, y+1, w, h)
	s.Fill(x, y, w, h, tty.Cell{Ch: ' ', Fg: cFg, Bg: cPanel})
	s.Fill(x, y, w, 1, tty.Cell{Ch: ' ', Bg: cSel})
	s.Text(x+2, y, title, 0xffffff, cSel, engine.AttrBold, w-4)
}

func (a *App) drawHelp() {
	w, h := 62, len(helpText)+3
	if w > a.scr.W {
		w = a.scr.W
	}
	if h > a.scr.H {
		h = a.scr.H
	}
	x, y := a.window("Keys & mouse — press any key to close", w, h)
	a.on(0, 0, a.scr.W, a.scr.H, func(ev tty.Event, _, _ int) {
		if ev.Action == tty.MousePress {
			a.help = false
		}
	})
	for i, l := range helpText {
		if i+2 >= h {
			break
		}
		a.scr.Text(x+2, y+2+i, l[0], cYellow, cPanel, 0, 18)
		a.scr.Text(x+21, y+2+i, l[1], cFg, cPanel, 0, w-23)
	}
}

// --- input -----------------------------------------------------------------

func (a *App) handle(ev tty.Event) {
	a.dirty = true
	if ev.Type == tty.EvMouse {
		a.mouse(ev)
		return
	}
	a.lastActivity = time.Now()
	switch {
	case ev.Key == tty.KeyCtrlC:
		a.quit = true
	case ev.Key == tty.KeyCtrlL:
		a.scr.Invalidate()
	case a.confirm != nil:
		a.confirm.key(a, ev)
	case a.prompt != nil:
		a.prompt.key(a, ev)
	case a.help:
		a.help = false
	case a.browser != nil:
		a.browser.key(a, ev)
	case a.menu != nil && a.menu.key(a, ev):
	default:
		a.playerKey(ev)
	}
}

func (a *App) mouse(ev tty.Event) {
	if ev.Action != tty.MouseMove || ev.X != a.mx || ev.Y != a.my {
		a.lastActivity = time.Now()
	}
	a.mx, a.my = ev.X, ev.Y
	a.dbl = false
	if ev.Action == tty.MousePress && ev.Button == tty.ButtonLeft {
		now := time.Now()
		a.dbl = now.Sub(a.lastClick) < 400*time.Millisecond && ev.X == a.lastClickX && ev.Y == a.lastClickY
		a.lastClick, a.lastClickX, a.lastClickY = now, ev.X, ev.Y
		if a.dbl {
			a.lastClick = time.Time{}
		}
	}
	if a.capture != nil && (ev.Action == tty.MouseDrag || ev.Action == tty.MouseRelease) {
		h := a.capture
		if ev.Action == tty.MouseRelease {
			a.capture = nil
		}
		h.fn(ev, ev.X-h.x, ev.Y-h.y)
		return
	}
	for i := len(a.hits) - 1; i >= 0; i-- {
		h := a.hits[i]
		if ev.X >= h.x && ev.X < h.x+h.w && ev.Y >= h.y && ev.Y < h.y+h.h {
			if ev.Action == tty.MousePress {
				hc := h
				a.capture = &hc
			}
			h.fn(ev, ev.X-h.x, ev.Y-h.y)
			return
		}
	}
}

func (a *App) videoMouse(ev tty.Event, _, _ int) {
	switch ev.Action {
	case tty.MousePress:
		switch ev.Button {
		case tty.ButtonLeft:
			if a.menu != nil {
				a.menu = nil
			} else {
				a.setPlaying(!a.isPlaying())
			}
		case tty.ButtonRight:
			a.toggleMenu(-1)
		case tty.ButtonMiddle:
			a.randomize(false)
		}
	case tty.MouseWheelUp:
		a.nudge("volume", 1)
	case tty.MouseWheelDown:
		a.nudge("volume", -1)
	}
}

func (a *App) toggleMenu(tab int) {
	if a.menu != nil && (tab < 0 || tab == a.menu.tab) {
		a.menu = nil
		return
	}
	if a.menu == nil {
		a.menu = newMenu()
	}
	if tab >= 0 {
		a.menu.tab = tab
	}
}

func (a *App) playerKey(ev tty.Event) {
	switch ev.Key {
	case tty.KeyLeft, tty.KeyRight:
		d := 5.0
		if ev.Shift || ev.Ctrl {
			d = 30
		}
		if ev.Key == tty.KeyLeft {
			d = -d
		}
		a.seekBy(d)
	case tty.KeyUp:
		a.nudge("volume", 1)
	case tty.KeyDown:
		a.nudge("volume", -1)
	case tty.KeyHome:
		a.seek(0)
	case tty.KeyTab, tty.KeyF2:
		a.toggleMenu(-1)
	case tty.KeyF1:
		a.help = true
	case tty.KeyCtrlS:
		a.savePresetPrompt()
	case tty.KeyEsc:
		if a.menu != nil {
			a.menu = nil
		}
	case tty.KeyRune:
		a.playerRune(ev.Rune)
	}
}

func (a *App) playerRune(r rune) {
	switch r {
	case ' ', 'k':
		a.setPlaying(!a.isPlaying())
	case 'q', 'Q':
		a.quit = true
	case 'r':
		a.randomize(false)
	case 'R':
		a.randomize(true)
	case 'u', 'U':
		a.undo()
	case 's', 'S':
		a.toggleMenu(-1)
	case 'p':
		a.cyclePreset(1)
	case 'P':
		a.cyclePreset(-1)
	case 'v':
		a.nudge("mode", 1)
	case 'V':
		a.nudge("mode", -1)
	case 'd':
		a.nudge("dither", 1)
	case 'D':
		a.nudge("dither", -1)
	case 'c':
		a.nudge("palette", 1)
	case 'C':
		a.nudge("palette", -1)
	case 'a':
		a.nudge("charset", 1)
	case 'A':
		a.nudge("charset", -1)
	case '=', '+':
		a.nudge("colors", 1)
	case '-', '_':
		a.nudge("colors", -1)
	case 'f':
		a.nudge("fit", 1)
	case 'x':
		a.nudge("flip-x", 1)
	case 'y':
		a.nudge("flip-y", 1)
	case 'e':
		a.nudge("edges", 1)
	case 'i':
		a.nudge("invert", 1)
	case 'm':
		a.nudge("mute", 1)
	case 'l':
		a.nudge("loop", 1)
	case ']':
		a.nudge("speed", 1)
	case '[':
		a.nudge("speed", -1)
	case 'h':
		a.hud = map[string]string{"auto": "on", "on": "off", "off": "auto"}[a.hud]
		a.say("HUD: "+a.hud, time.Second)
	case 'g':
		a.stats = !a.stats
	case '?':
		a.help = true
	case 'o':
		a.openBrowser()
	case 'n':
		if len(a.files) > 1 {
			a.open(a.fileIdx+1, 0)
		}
	case 'N':
		if len(a.files) > 1 {
			a.open(a.fileIdx-1, 0)
		}
	case '.':
		if a.loaded && !a.info.Still {
			if a.isPlaying() {
				a.setPlaying(false)
			}
			a.needFrame = true
		}
	case ',':
		if a.loaded {
			if a.isPlaying() {
				a.setPlaying(false)
			}
			a.seek(a.pos() - 1/a.fps())
		}
	default:
		if r >= '0' && r <= '9' && a.info.Duration > 0 {
			a.seek(a.info.Duration * float64(r-'0') / 10)
		}
	}
}

func (a *App) savePresetPrompt() {
	name := a.preset
	if p, ok := a.store.Find(name); ok && p.Builtin {
		name = ""
	}
	a.prompt = &Prompt{title: "Save look as preset", hint: "name", buf: []rune(name), cur: len([]rune(name)),
		done: func(v string) { a.savePreset(strings.TrimSpace(v), true) }}
}

func (a *App) savePreset(name string, askOverwrite bool) {
	if err := ValidName(name); err != nil {
		a.say(err.Error(), 2*time.Second)
		return
	}
	if p, ok := a.store.Find(name); ok && !p.Builtin && askOverwrite && !strings.EqualFold(name, a.preset) {
		a.confirm = &Confirm{msg: fmt.Sprintf("Overwrite preset %q?", p.Name), yes: func() { a.savePreset(name, false) }}
		return
	}
	if err := a.store.Save(name, a.s.Look); err != nil {
		a.say("save failed: "+err.Error(), 4*time.Second)
		return
	}
	a.preset = name
	a.say("saved preset: "+name, 1500*time.Millisecond)
}
