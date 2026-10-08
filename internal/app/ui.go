package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
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
	pad := 1 + a.pad()
	w := len([]rune(label)) + 2*pad
	if a.hover(x, y, w, 1) {
		bg = cHot
	}
	a.scr.Fill(x, y, w, 1, tty.Cell{Ch: ' ', Bg: bg})
	a.scr.Text(x+pad, y, label, fg, bg, 0, -1)
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
			c.Ch, c.Fg = '●', cFg
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
	paused := !a.isPlaying() && !a.info.Still
	return paused || a.menu != nil || time.Since(a.lastActivity) < 2500*time.Millisecond
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
		if a.ui == 0 {
			a.drawMiniHUD()
		} else {
			a.drawHUD()
		}
	}
	if a.stats {
		a.drawStats()
	}
	if a.menu != nil {
		a.menu.draw(a)
	}
	if a.finder != nil {
		a.finder.draw(a)
	}
	if a.browser != nil {
		a.browser.draw(a)
	}
	if a.help {
		a.drawHelp()
	}
	if a.form != nil {
		a.form.draw(a)
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
		s.Text((s.W-len(msg))/2, y, string(msg), cBar, cYellow, engine.AttrBold, -1)
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
	if n := a.activeDownloads(); n > 0 {
		title += fmt.Sprintf("  ↓ %d downloading", n)
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

	// Seek bar. Large interface sizes put blank rows around it.
	pad := a.pad()
	top := H - 2 - pad
	s.Fill(0, top, W, 2+pad, tty.Cell{Ch: ' ', Bg: cBar})
	a.on(0, top, W, 2+pad, func(tty.Event, int, int) {})
	y := H - 2
	if pad >= 2 {
		y = H - 3
	}
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
		s.Text(tx, y-1, tip, cBar, cAccent, 0, -1)
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
	if a.info.Still {
		play = "■"
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
		if W >= 110 && a.s.Sound.SoundActive() {
			label := "♪ " + engine.SoundSummary(a.s.Sound)
			if a.soundName != "" {
				label = "♪ " + a.soundName
			}
			if r := []rune(label); len(r) > 26 {
				label = string(r[:25]) + "…"
			}
			x += a.button(x, y, label, cYellow, cBar, func(ev tty.Event) {
				if clicked(ev) {
					a.openPanel("presets", 1)
				}
			})
		}
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
				a.help, a.helpTop = !a.help, 0
			}
		}},
	}
	btnW := func(b btn) int { return len([]rune(b.label)) + 2 + 2*a.pad() }
	total := 0
	for _, b := range btns {
		total += btnW(b)
	}
	for len(btns) > 0 && x+total+1 > W {
		total -= btnW(btns[0])
		btns = btns[1:]
	}
	rx := W - total - 1
	for _, b := range btns {
		rx += a.button(rx, y, b.label, b.fg, cBar, b.fn)
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
	s.Text(x+2, y, title, cFg, cSel, engine.AttrBold, w-4)
}

// helpLines lists every binding of the active layers, grouped by layer.
func (a *App) helpLines() [][2]string {
	var out [][2]string
	for _, l := range a.layers() {
		out = append(out, [2]string{"", "── " + l.title + " ──"})
		for _, b := range l.binds {
			if b.desc != "" {
				out = append(out, [2]string{prettyKeys(b.keys), b.desc})
			}
		}
	}
	return append(out,
		[2]string{"", "── Mouse ──"},
		[2]string{"click picture", "play / pause"},
		[2]string{"right-click", "settings menu"},
		[2]string{"middle-click", "randomize the look"},
		[2]string{"wheel", "volume"},
		[2]string{"Ctrl+wheel", "picture size"},
		[2]string{"Alt+wheel", "interface size"},
		[2]string{"drag", "seek bar, sliders, window title bars"})
}

func (a *App) drawHelp() {
	lines := a.helpLines()
	w, h := a.dim(70, 30)
	h = min(h, len(lines)+3)
	x, y := a.window("Keys & mouse — ↑↓ scroll · any other key closes", w, h)
	bodyH := h - 3
	a.helpTop = max(0, min(a.helpTop, len(lines)-bodyH))
	a.on(0, 0, a.scr.W, a.scr.H, func(ev tty.Event, _, _ int) {
		switch ev.Action {
		case tty.MousePress:
			a.help = false
		case tty.MouseWheelUp:
			a.helpTop -= 3
		case tty.MouseWheelDown:
			a.helpTop += 3
		}
	})
	kw := 24 + 4*a.pad()
	for i := 0; i < bodyH && a.helpTop+i < len(lines); i++ {
		l := lines[a.helpTop+i]
		if l[0] == "" {
			a.scr.Text(x+2, y+2+i, l[1], cAccent, cPanel, engine.AttrBold, w-4)
			continue
		}
		a.scr.Text(x+2, y+2+i, l[0], cYellow, cPanel, 0, kw-1)
		a.scr.Text(x+2+kw, y+2+i, l[1], cFg, cPanel, 0, w-4-kw)
	}
	if a.helpTop+bodyH < len(lines) {
		a.scr.Text(x+w-9, y+h-1, " more ↓ ", cDim, cPanel, 0, -1)
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
	case a.form != nil:
		a.form.key(a, ev)
	case a.help:
		switch ev.Key {
		case tty.KeyUp:
			a.helpTop--
		case tty.KeyDown:
			a.helpTop++
		case tty.KeyPgUp:
			a.helpTop -= 10
		case tty.KeyPgDn:
			a.helpTop += 10
		default:
			a.help = false
		}
	case a.browser != nil:
		a.browser.key(a, ev)
	case a.finder != nil && a.finderOpen:
		a.finder.key(a, ev)
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
	case tty.MouseWheelUp, tty.MouseWheelDown:
		d := 1
		if ev.Action == tty.MouseWheelDown {
			d = -1
		}
		switch {
		case ev.Ctrl:
			a.zoom(d)
		case ev.Alt:
			a.setUI(a.ui + d)
		default:
			a.nudge("volume", d)
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
	name = strings.TrimSpace(name)
	if p, ok := a.store.Find(name); ok && !p.Builtin && askOverwrite && !strings.EqualFold(name, a.preset) {
		a.confirm = &Confirm{msg: fmt.Sprintf("Overwrite preset %q?", p.Name), yes: func() { a.savePreset(name, false) }}
		return
	}
	if err := a.store.Save(name, a.s.Look); err != nil {
		a.say("save failed: "+err.Error(), 4*time.Second)
		return
	}
	a.preset = name
	pgPresets.selectName(a, name)
	a.say("saved preset: "+name, 1500*time.Millisecond)
}

// drawMiniHUD is the compact interface size: one line at the bottom.
func (a *App) drawMiniHUD() {
	s := a.scr
	W, y := s.W, s.H-1
	s.Fill(0, y, W, 1, tty.Cell{Ch: ' ', Bg: cBar})
	a.on(0, y, W, 1, func(tty.Event, int, int) {})
	play := "▶"
	if a.isPlaying() {
		play = "▮▮"
	}
	x := a.button(0, y, play, cGreen, cBar, func(ev tty.Event) {
		if clicked(ev) {
			a.setPlaying(!a.isPlaying())
		}
	})
	pos, dur := a.pos(), a.info.Duration
	x += s.Text(x, y, fmtTime(pos)+" ", cFg, cBar, 0, -1)
	right := " " + fmtTime(dur) + " ☰ "
	bw := W - x - len([]rune(right))
	frac := 0.0
	if dur > 0 {
		frac = min(1, pos/dur)
	}
	a.slider(x, y, bw, frac, cAccent, cBar)
	bx := x
	a.on(bx, y, bw, 1, func(ev tty.Event, rx, _ int) {
		if (ev.Action == tty.MousePress || ev.Action == tty.MouseDrag) && ev.Button == tty.ButtonLeft && dur > 0 {
			a.seek(sliderFrac(rx, bw) * dur)
		}
	})
	s.Text(x+bw, y, right, cDim, cBar, 0, -1)
	a.on(W-3, y, 3, 1, func(ev tty.Event, _, _ int) {
		if clicked(ev) {
			a.toggleMenu(-1)
		}
	})
}
