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
	return paused || a.menu != nil || a.finderOpen || time.Since(a.lastActivity) < 2500*time.Millisecond
}

func (a *App) draw() {
	s := a.scr
	s.Clear()
	if a.cells != nil {
		s.Blit((s.W-a.ccols)/2, (s.H-a.crows)/2, a.ccols, a.crows, a.cells)
	}
	a.hits = a.hits[:0]
	a.on(0, 0, s.W, s.H, a.videoMouse)
	a.hudShown, a.hintY = a.hudVisible(), -1
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
	if _, bottom := a.desk(); a.hintY >= 0 && a.curF() >= 0 && bottom == a.hintY {
		// After the windows, so that the one in front cannot keep the
		// clicks from it. A terminal too small to keep a row free for it
		// goes without.
		a.drawFKeys(a.hintY)
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
		bg := cYellow
		if a.toastTint {
			bg = a.tint()
		}
		s.Text((s.W-len(msg))/2, y, string(msg), cBar, bg, engine.AttrBold, -1)
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
	a.on(0, 0, W, 1, func(tty.Event, int, int) {})
	tint := a.tint()
	x := 2 + s.Text(1, 0, "termo", tint, cBar, engine.AttrBold, -1)
	x = a.drawModes(x, 0)
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
	// The look on the right, with as much detail as fits.
	base, lead := Summary(a.s.Look), ""
	if a.preset != "" {
		lead = "◆ " + a.preset + "  "
	}
	plain := a.s.Look
	plain.FX = engine.DefaultFX()
	for _, right := range []string{lead + base, lead + Summary(plain), Summary(plain)} {
		if rw := len([]rune(right)); x+rw+3 < W {
			s.Text(W-rw-1, 0, right, cDim, cBar, 0, -1)
			break
		}
	}

	// Seek bar. Large interface sizes put blank rows around it.
	pad := a.pad()
	top := H - 2 - pad
	s.Fill(0, top, W, 2+pad, tty.Cell{Ch: ' ', Bg: cBar})
	a.on(0, top, W, 2+pad, func(tty.Event, int, int) {})
	if a.prefs.Hints && H >= 10 && W >= 40 {
		a.hintY = top - 1
		a.drawHints(a.hintY)
	}
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
	a.slider(bx, y, bw, frac, tint, cBar)
	if a.hover(bx, y, bw, 1) && dur > 0 {
		tip := " " + fmtTime(sliderFrac(a.mx-bx, bw)*dur) + " "
		tx := max(0, min(W-len(tip), a.mx-len(tip)/2))
		s.Text(tx, y-1, tip, cBar, tint, 0, -1)
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
		if W >= 110 && a.s.Sound.SoundActive() && modes[a.mode].name != "audio" {
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

	// Right-aligned buttons: those of the mode, then the two that are always
	// there. They are dropped from the left when space runs out.
	btns := append(a.modeButtons(),
		btn{"☰ menu", tint, func(ev tty.Event) {
			if clicked(ev) {
				a.toggleMenu(-1)
			}
		}},
		btn{"?", cFg, func(ev tty.Event) {
			if clicked(ev) {
				a.openF(0)
			}
		}})
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
	if top, bottom := a.desk(); h <= bottom-top {
		y = top + (bottom-top-h)/2
	}
	a.frame(title, x, y, w, h)
	return
}

// desk is the rows the big windows keep to, top to bottom-1, so that the
// bars stay in sight under them. A small terminal gives them all of it.
func (a *App) desk() (top, bottom int) {
	s := a.scr
	if !a.hudShown || a.ui == 0 || s.H < 6 {
		return 0, s.H
	}
	top, bottom = 1, s.H-2-a.pad()
	if a.hintY >= 0 {
		bottom = a.hintY
	}
	if bottom-top < 18 {
		return 0, s.H
	}
	return top, bottom
}

func (a *App) frame(title string, x, y, w, h int) {
	s := a.scr
	s.Dim(x+2, y+1, w, h)
	s.Fill(x, y, w, h, tty.Cell{Ch: ' ', Fg: cFg, Bg: cPanel})
	s.Fill(x, y, w, 1, tty.Cell{Ch: ' ', Bg: cSel})
	s.Text(x+2, y, title, cFg, cSel, engine.AttrBold, w-4)
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
	case ev.Key >= tty.KeyF1 && ev.Key <= tty.KeyF12 && !ev.Ctrl && !ev.Alt && !ev.Shift:
		a.openF(int(ev.Key - tty.KeyF1)) // the F keys work in every window
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
	// The mode, in its color: click or scroll to change it.
	chip := strings.ToUpper(modes[a.mode].name) + " "
	a.on(x, y, len(chip), 1, a.modeMouse(a.mode+1))
	x += s.Text(x, y, chip, a.tint(), cBar, engine.AttrBold, -1)
	x += s.Text(x, y, fmtTime(pos)+" ", cFg, cBar, 0, -1)
	right := " " + fmtTime(dur) + " ☰ "
	bw := W - x - len([]rune(right))
	frac := 0.0
	if dur > 0 {
		frac = min(1, pos/dur)
	}
	a.slider(x, y, bw, frac, a.tint(), cBar)
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

// --- bind modes on the HUD -----------------------------------------------------

type btn struct {
	label string
	fg    uint32
	fn    func(ev tty.Event)
}

// modeMouse handles the mouse over a mode label: a click switches to mode
// i, the wheel steps through the modes.
func (a *App) modeMouse(i int) func(ev tty.Event, rx, ry int) {
	return func(ev tty.Event, _, _ int) {
		switch {
		case clicked(ev):
			a.setMode(i)
		case ev.Action == tty.MouseWheelDown:
			a.setMode(a.mode + 1)
		case ev.Action == tty.MouseWheelUp:
			a.setMode(a.mode - 1)
		}
	}
}

// drawModes draws the bind modes on the title bar, the active one in its
// color, and returns the column after them. A narrow terminal gets only the
// active one.
func (a *App) drawModes(x, y int) int {
	s := a.scr
	all := s.W >= 76
	for i, m := range modes {
		if i != a.mode && !all {
			continue
		}
		label := " " + m.name + " "
		fg, bg, attr := cDim, cBar, uint8(0)
		switch {
		case i == a.mode:
			label, fg, bg, attr = strings.ToUpper(label), cBar, modeColor(i), engine.AttrBold
		case a.hover(x, y, len(label), 1):
			fg, bg = modeColor(i), cHot
		}
		next := i
		if !all {
			next = a.mode + 1 // the only label on show: a click moves on
		}
		a.on(x, y, len(label), 1, a.modeMouse(next))
		x += s.Text(x, y, label, fg, bg, attr, -1)
	}
	if s.W >= 104 {
		x += s.Text(x, y, " Tab⇄", cDim, cBar, 0, -1)
	}
	return x
}

// drawHints is the line above the seek bar. It lists the keys of the active
// mode; while a window is open it lists the twelve windows behind the F
// keys instead. Everything on it can be clicked.
func (a *App) drawHints(y int) {
	s := a.scr
	W := s.W
	s.Fill(0, y, W, 1, tty.Cell{Ch: ' ', Bg: cBar})
	a.on(0, y, W, 1, func(tty.Event, int, int) {})
	if a.curF() >= 0 {
		return // draw puts the F keys here, over the windows
	}
	const more = " F1 all keys "
	end := W - len(more)
	s.Text(end, y, more, cDim, cBar, 0, -1)
	a.on(end, y, len(more), 1, func(ev tty.Event, _, _ int) {
		if clicked(ev) {
			a.openF(0)
		}
	})
	tint, m, x := a.tint(), modes[a.mode], 1
	for i := range m.binds {
		b := &m.binds[i]
		if b.short == "" {
			continue
		}
		k, label := prettyKey(strings.Fields(b.keys)[0]), b.short
		if b == a.lastBind {
			label += " -/="
		}
		w := len([]rune(k)) + 1 + len([]rune(label))
		if x+w+2 > end {
			s.Text(x, y, "…", cDim, cBar, 0, -1)
			break
		}
		fg, bg := cDim, cBar
		switch {
		case a.hover(x, y, w, 1):
			fg, bg = cFg, cHot
			s.Fill(x-1, y, w+2, 1, tty.Cell{Ch: ' ', Bg: bg})
		case b == a.lastBind:
			fg = cFg
		}
		s.Text(x, y, k, tint, bg, engine.AttrBold, -1)
		s.Text(x+len([]rune(k))+1, y, label, fg, bg, 0, -1)
		a.on(x-1, y, w+2, 1, func(ev tty.Event, _, _ int) {
			dir := 0
			switch {
			case clicked(ev), ev.Action == tty.MouseWheelUp:
				dir = 1
			case ev.Action == tty.MousePress && ev.Button == tty.ButtonRight, ev.Action == tty.MouseWheelDown:
				dir = -1
			}
			if dir == 0 || (dir < 0 && b.rev == "") {
				return
			}
			if b.rev != "" {
				a.lastBind = b
			}
			b.fn(a, dir)
		})
		x += w + 2
	}
}

// drawFKeys lists the windows behind F1 – F12 on one line, the open one
// marked; a click opens one, which helps where the terminal keeps some of
// the F keys to itself.
func (a *App) drawFKeys(y int) {
	s := a.scr
	cur := a.curF()
	s.Fill(0, y, s.W, 1, tty.Cell{Ch: ' ', Bg: cBar})
	a.on(0, y, s.W, 1, func(tty.Event, int, int) {})
	// Names in full, cut short, or left out: whatever fits.
	cut := 0
	for _, c := range []int{99, 4, 0} {
		total := 0
		for i, f := range fkeys {
			total += len(fmt.Sprint(i+1)) + 2 + min(c, len([]rune(f.name))) + min(c, 1)
		}
		if cut = c; total+1 <= s.W {
			break
		}
	}
	x := 1
	for i, f := range fkeys {
		name := []rune(f.name)
		name = name[:min(cut, len(name))]
		k := "F" + fmt.Sprint(i+1)
		w := len(k) + len(name) + min(cut, 1)
		if x+w > s.W {
			break
		}
		fg, bg := cDim, cBar
		switch {
		case i == cur:
			fg, bg = cFg, cSel
		case a.hover(x-1, y, w+1, 1):
			fg, bg = cFg, cHot
		}
		s.Fill(x-1, y, w+1, 1, tty.Cell{Ch: ' ', Bg: bg})
		s.Text(x, y, k, cYellow, bg, engine.AttrBold, -1)
		s.Text(x+len(k)+1, y, string(name), fg, bg, 0, -1)
		i := i
		a.on(x-1, y, w+1, 1, func(ev tty.Event, _, _ int) {
			if clicked(ev) {
				a.openF(i)
			}
		})
		x += w + 1
	}
}

// fxName names the effects in use: the preset they match, none, or custom.
func (a *App) fxName() string {
	for _, p := range a.lib.AllFX() {
		if p.FX == a.s.FX {
			return p.Name
		}
	}
	return "custom"
}

// modeButtons are the buttons of the active mode on the bottom bar: what
// the mode cycles through, and its dice.
func (a *App) modeButtons() []btn {
	cyc := func(fn func(dir int)) func(ev tty.Event) {
		return func(ev tty.Event) {
			switch {
			case clicked(ev), ev.Action == tty.MouseWheelDown:
				fn(1)
			case ev.Action == tty.MousePress && ev.Button == tty.ButtonRight, ev.Action == tty.MouseWheelUp:
				fn(-1)
			}
		}
	}
	option := func(key string) btn {
		return btn{engine.FindOption(key).String(&a.s), cFg, cyc(func(d int) { a.nudge(key, d) })}
	}
	dice := func(fn func()) btn {
		return btn{"⚄ random", cYellow, func(ev tty.Event) {
			if clicked(ev) {
				fn()
			} else if ev.Action == tty.MousePress && ev.Button == tty.ButtonRight {
				a.undo()
			}
		}}
	}
	switch modes[a.mode].name {
	case "video":
		return []btn{option("mode"), option("dither"), option("palette"), dice(func() { a.randomize(false) })}
	case "color":
		return []btn{{"filter: " + a.s.Filter, cFg, cyc(func(d int) { a.nudge("filter", d) })}, dice(a.randomGrade)}
	case "fx":
		return []btn{{"✦ " + a.fxName(), cFg, cyc(a.cycleFX)}, dice(a.randomFX)}
	case "audio":
		name := a.soundName
		switch {
		case name != "":
		case a.s.Sound.SoundActive():
			name = "custom"
		default:
			name = "clean"
		}
		return []btn{{"♪ " + name, cFg, cyc(a.cycleSound)}, dice(a.randomSound)}
	}
	loop := cDim
	if a.s.Loop {
		loop = cGreen
	}
	out := []btn{
		{trim(a.s.Speed) + "×", cFg, cyc(func(d int) { a.nudge("speed", d) })},
		{"loop", loop, func(ev tty.Event) {
			if clicked(ev) {
				a.nudge("loop", 1)
			}
		}},
	}
	if len(a.files) > 1 {
		out = append(out, btn{fmt.Sprintf("file %d/%d", a.fileIdx+1, len(a.files)), cFg, cyc(a.openNext)})
	}
	return append(out, dice(func() { a.randomize(false) }))
}
