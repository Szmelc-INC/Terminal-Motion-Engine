package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
)

const (
	tabPresets = 5
	tabPalette = 6
)

var menuTabs = []string{"Render", "Color", "Dither", "Adjust", "Playback", "Presets", "Palette"}

// Menu is the settings popup: five tabs generated from the option table,
// plus the preset manager and the palette editor.
type Menu struct {
	tab          int
	sel          [7]int
	x, y         int
	placed       bool
	grabX, grabY int
	scroll       int
}

func newMenu() *Menu { return &Menu{} }

func optionsFor(group string) []*engine.Option {
	var out []*engine.Option
	for _, o := range engine.Options {
		if o.Group == group {
			out = append(out, o)
		}
	}
	return out
}

// touch records an undo point, coalescing bursts of small edits into one.
func (a *App) touch() {
	if time.Since(a.lastTouch) > 800*time.Millisecond {
		a.pushHistory()
	}
	a.lastTouch = time.Now()
}

func (m *Menu) rows(a *App) int {
	switch m.tab {
	case tabPresets:
		return len(a.store.All())
	case tabPalette:
		if p := a.rend.Palette(); p != nil {
			return len(p.Colors)
		}
		return 0
	}
	return len(optionsFor(menuTabs[m.tab]))
}

func (m *Menu) move(a *App, d int) {
	n := m.rows(a)
	if n == 0 {
		m.sel[m.tab] = 0
		return
	}
	m.sel[m.tab] = ((m.sel[m.tab]+d)%n + n) % n
}

func (m *Menu) setTab(t int) {
	n := len(menuTabs)
	m.tab = (t%n + n) % n
	m.scroll = 0
}

func (m *Menu) draw(a *App) {
	s := a.scr
	w, h := min(66, s.W), min(22, s.H)
	if !m.placed {
		m.x, m.y, m.placed = (s.W-w)/2, max(0, (s.H-h)/2-1), true
	}
	m.x = max(0, min(s.W-w, m.x))
	m.y = max(0, min(s.H-h, m.y))
	x, y := m.x, m.y
	a.frame("Settings", x, y, w, h)
	a.on(x, y, w, h, func(tty.Event, int, int) {})

	// Title bar: drag to move, ✕ to close.
	a.on(x, y, w, 1, func(ev tty.Event, _, _ int) {
		switch ev.Action {
		case tty.MousePress:
			m.grabX, m.grabY = ev.X-m.x, ev.Y-m.y
		case tty.MouseDrag:
			m.x, m.y = ev.X-m.grabX, ev.Y-m.grabY
		}
	})
	s.Text(x+w-4, y, " ✕ ", cRed, cSel, engine.AttrBold, -1)
	a.on(x+w-4, y, 3, 1, func(ev tty.Event, _, _ int) {
		if clicked(ev) {
			a.menu = nil
		}
	})

	// Tabs.
	tx := x + 1
	for i, name := range menuTabs {
		label := " " + name + " "
		if tx+len(label) > x+w {
			break
		}
		fg, bg, attr := cDim, cPanel, uint8(0)
		if i == m.tab {
			fg, bg, attr = 0xffffff, cAccent, engine.AttrBold
		} else if a.hover(tx, y+1, len(label), 1) {
			fg, bg = cFg, cHot
		}
		s.Text(tx, y+1, label, fg, bg, attr, -1)
		i := i
		a.on(tx, y+1, len(label), 1, func(ev tty.Event, _, _ int) {
			if clicked(ev) {
				m.setTab(i)
			}
		})
		tx += len(label)
	}

	bodyY, bodyH := y+3, h-6
	if bodyH < 1 {
		return
	}
	hint := "↑↓ select · ←→ change · Enter edit · Tab next tab · r random · Esc close"
	switch m.tab {
	case tabPresets:
		m.drawPresets(a, x, bodyY, w, bodyH)
		hint = "Enter load · n new · s save over · e rename · d delete · Tab next tab"
	case tabPalette:
		m.drawPalette(a, x, bodyY, w, bodyH)
		hint = "←→↑↓ pick · t make custom · a add · Enter edit · d delete · R random"
	default:
		m.drawOptions(a, x, bodyY, w, bodyH)
	}
	s.Text(x+2, y+h-1, hint, cDim, cPanel, 0, w-4)
}

func (m *Menu) visible(n, bodyH int) (first int) {
	sel := m.sel[m.tab]
	if sel < m.scroll {
		m.scroll = sel
	}
	if sel >= m.scroll+bodyH {
		m.scroll = sel - bodyH + 1
	}
	m.scroll = max(0, min(m.scroll, max(0, n-bodyH)))
	return m.scroll
}

func (m *Menu) drawOptions(a *App, x, y, w, bodyH int) {
	s := a.scr
	opts := optionsFor(menuTabs[m.tab])
	if m.sel[m.tab] >= len(opts) {
		m.sel[m.tab] = 0
	}
	first := m.visible(len(opts), bodyH)
	wx, ww := x+18, w-20
	for i := first; i < len(opts) && i-first < bodyH; i++ {
		o, i, ry := opts[i], i, y+i-first
		bg := cPanel
		if i == m.sel[m.tab] {
			bg = cSel
		}
		s.Fill(x+1, ry, w-2, 1, tty.Cell{Ch: ' ', Bg: bg})
		fg := cFg
		if !o.IsActive(&a.s) {
			fg = cDim
		}
		s.Text(x+2, ry, o.Label, fg, bg, 0, 15)
		a.on(x+1, ry, w-2, 1, func(ev tty.Event, _, _ int) {
			switch ev.Action {
			case tty.MouseMove, tty.MousePress:
				m.sel[m.tab] = i
			case tty.MouseWheelUp:
				m.sel[m.tab] = i
				m.change(a, o, 1)
			case tty.MouseWheelDown:
				m.sel[m.tab] = i
				m.change(a, o, -1)
			}
		})
		val := o.String(&a.s)
		switch o.Kind {
		case engine.KEnum:
			s.Text(wx, ry, "◀", cAccent, bg, 0, -1)
			s.Text(wx+ww-1, ry, "▶", cAccent, bg, 0, -1)
			vr := []rune(val)
			if len(vr) > ww-4 {
				vr = vr[:ww-4]
			}
			s.Text(wx+(ww-len(vr))/2, ry, string(vr), fg, bg, engine.AttrBold, -1)
			a.on(wx-1, ry, ww+2, 1, func(ev tty.Event, rx, _ int) {
				m.sel[m.tab] = i
				switch {
				case ev.Action == tty.MouseWheelUp:
					m.change(a, o, -1)
				case ev.Action == tty.MouseWheelDown:
					m.change(a, o, 1)
				case ev.Action != tty.MousePress:
				case ev.Button == tty.ButtonRight || (ev.Button == tty.ButtonLeft && rx < 4):
					m.change(a, o, -1)
				case ev.Button == tty.ButtonLeft:
					m.change(a, o, 1)
				}
			})
		case engine.KInt, engine.KFloat:
			sw := ww - 8
			a.slider(wx, ry, sw, o.Frac(&a.s), cAccent, bg)
			s.Text(wx+sw+1, ry, fmt.Sprintf("%7s", val), fg, bg, engine.AttrBold, 7)
			a.on(wx, ry, sw, 1, func(ev tty.Event, rx, _ int) {
				m.sel[m.tab] = i
				switch ev.Action {
				case tty.MousePress, tty.MouseDrag:
					if ev.Button == tty.ButtonLeft {
						if ev.Action == tty.MousePress {
							a.touch()
						}
						o.SetFrac(&a.s, sliderFrac(rx, sw))
						a.changed()
					}
				case tty.MouseWheelUp:
					m.change(a, o, 1)
				case tty.MouseWheelDown:
					m.change(a, o, -1)
				}
			})
		case engine.KBool:
			mark, mfg := "○ off", cDim
			if val == "on" {
				mark, mfg = "● on", cGreen
			}
			s.Text(wx, ry, mark, mfg, bg, engine.AttrBold, -1)
			a.on(wx-1, ry, 8, 1, func(ev tty.Event, _, _ int) {
				m.sel[m.tab] = i
				if clicked(ev) {
					m.change(a, o, 1)
				}
			})
		default:
			if val == "" {
				val = "(empty — click to edit)"
			}
			s.Text(wx, ry, tty.Clean(val), fg, bg, 0, ww)
			a.on(wx, ry, ww, 1, func(ev tty.Event, _, _ int) {
				m.sel[m.tab] = i
				if clicked(ev) {
					m.edit(a, o)
				}
			})
		}
	}
	if sel := m.sel[m.tab]; sel < len(opts) {
		help := opts[sel].Help
		if help == "" {
			help = "--" + opts[sel].Key
		} else {
			help += "  (--" + opts[sel].Key + ")"
		}
		s.Text(x+2, y+bodyH+1, help, cYellow, cPanel, 0, w-4)
	}
}

func (m *Menu) change(a *App, o *engine.Option, dir int) {
	if o.Kind == engine.KText || o.Kind == engine.KList {
		return
	}
	if o.Group != "Playback" {
		a.touch()
	}
	o.Nudge(&a.s, dir)
	a.changed()
}

func (m *Menu) edit(a *App, o *engine.Option) {
	cur := o.String(&a.s)
	a.prompt = &Prompt{title: o.Label, hint: o.Help, buf: []rune(cur), cur: len([]rune(cur)), done: func(v string) {
		a.touch()
		if err := o.Set(&a.s, strings.TrimSpace(v)); err != nil {
			a.say(strings.TrimPrefix(err.Error(), "--"), 3*time.Second)
			return
		}
		a.changed()
	}}
}

// --- presets -----------------------------------------------------------------

func (m *Menu) selectedPreset(a *App) (engine.Preset, bool) {
	all := a.store.All()
	if len(all) == 0 {
		return engine.Preset{}, false
	}
	m.sel[tabPresets] = max(0, min(m.sel[tabPresets], len(all)-1))
	return all[m.sel[tabPresets]], true
}

func (m *Menu) drawPresets(a *App, x, y, w, bodyH int) {
	s := a.scr
	all := a.store.All()
	m.sel[tabPresets] = max(0, min(m.sel[tabPresets], len(all)-1))
	listH := bodyH - 1
	first := m.visible(len(all), listH)
	a.on(x+1, y, w-2, listH, func(ev tty.Event, _, _ int) {
		switch ev.Action {
		case tty.MouseWheelUp:
			m.move(a, -1)
		case tty.MouseWheelDown:
			m.move(a, 1)
		}
	})
	for i := first; i < len(all) && i-first < listH; i++ {
		p, i, ry := all[i], i, y+i-first
		bg := cPanel
		if i == m.sel[tabPresets] {
			bg = cSel
		} else if a.hover(x+1, ry, w-2, 1) {
			bg = cHot
		}
		s.Fill(x+1, ry, w-2, 1, tty.Cell{Ch: ' ', Bg: bg})
		if p.Name == a.preset {
			s.Text(x+2, ry, "◆", cGreen, bg, 0, -1)
		}
		s.Text(x+4, ry, tty.Clean(p.Name), cFg, bg, engine.AttrBold, 18)
		tag, tfg := "user", cGreen
		if p.Builtin {
			tag, tfg = "built-in", cDim
		}
		s.Text(x+23, ry, tag, tfg, bg, 0, 8)
		s.Text(x+32, ry, Summary(p.Look), cDim, bg, 0, w-34)
		a.on(x+1, ry, w-2, 1, func(ev tty.Event, _, _ int) {
			switch ev.Action {
			case tty.MousePress:
				if ev.Button != tty.ButtonLeft {
					return
				}
				again := m.sel[tabPresets] == i
				m.sel[tabPresets] = i
				if a.dbl || again {
					a.loadPreset(p)
				}
			case tty.MouseWheelUp:
				m.move(a, -1)
			case tty.MouseWheelDown:
				m.move(a, 1)
			}
		})
	}
	bx := x + 2
	for _, b := range []struct {
		label string
		fg    uint32
		fn    func()
	}{
		{"Load", cGreen, func() { m.presetAction(a, 'l') }},
		{"New", cAccent, func() { m.presetAction(a, 'n') }},
		{"Save over", cFg, func() { m.presetAction(a, 's') }},
		{"Rename", cFg, func() { m.presetAction(a, 'e') }},
		{"Delete", cRed, func() { m.presetAction(a, 'd') }},
	} {
		fn := b.fn
		bx += 1 + a.button(bx, y+bodyH, b.label, b.fg, cBtn, func(ev tty.Event) {
			if clicked(ev) {
				fn()
			}
		})
	}
}

func (m *Menu) presetAction(a *App, act rune) {
	p, ok := m.selectedPreset(a)
	switch act {
	case 'l':
		if ok {
			a.loadPreset(p)
		}
	case 'n':
		a.prompt = &Prompt{title: "New preset from current look", hint: "name", done: func(v string) {
			a.savePreset(strings.TrimSpace(v), true)
		}}
	case 's':
		if !ok {
			return
		}
		if p.Builtin {
			a.say("built-in presets are read-only — press n to save a new one", 3*time.Second)
			return
		}
		a.confirm = &Confirm{msg: fmt.Sprintf("Overwrite %q with the current look?", p.Name), yes: func() {
			a.savePreset(p.Name, false)
		}}
	case 'e':
		if !ok {
			return
		}
		if p.Builtin {
			a.say("built-in presets cannot be renamed — press n to save a copy", 3*time.Second)
			return
		}
		a.prompt = &Prompt{title: "Rename preset", hint: "new name", buf: []rune(p.Name), cur: len([]rune(p.Name)),
			done: func(v string) {
				if err := a.store.Rename(p.Name, v); err != nil {
					a.say(err.Error(), 3*time.Second)
					return
				}
				if a.preset == p.Name {
					a.preset = strings.TrimSpace(v)
				}
				a.say("renamed to "+strings.TrimSpace(v), 1500*time.Millisecond)
			}}
	case 'd':
		if !ok {
			return
		}
		if p.Builtin {
			a.say("built-in presets cannot be deleted", 2*time.Second)
			return
		}
		a.confirm = &Confirm{msg: fmt.Sprintf("Delete preset %q?", p.Name), yes: func() {
			if err := a.store.Delete(p.Name); err != nil {
				a.say(err.Error(), 3*time.Second)
				return
			}
			if a.preset == p.Name {
				a.preset = ""
			}
			a.say("deleted "+p.Name, 1500*time.Millisecond)
		}}
	}
}

// --- palette -----------------------------------------------------------------

func (m *Menu) swatchLayout(n, w int) (sw, perRow int) {
	sw = 2
	if n <= 16 {
		sw = 4
	} else if n <= 64 {
		sw = 3
	}
	return sw, max(1, (w-4)/sw)
}

func (m *Menu) drawPalette(a *App, x, y, w, bodyH int) {
	s := a.scr
	pal := a.rend.Palette()
	if pal == nil {
		msg := "Truecolor output — there is no palette to show."
		if !a.s.Color {
			msg = "Color is off — the picture uses the terminal's own colours."
		}
		s.Text(x+2, y, msg, cFg, cPanel, 0, w-4)
		s.Text(x+2, y+2, "Press R (or click below) for a random colour-theory palette,", cDim, cPanel, 0, w-4)
		s.Text(x+2, y+3, "or choose one on the Color tab.", cDim, cPanel, 0, w-4)
		a.button(x+2, y+5, "⚄ Random palette", cYellow, cHot, func(ev tty.Event) {
			if clicked(ev) {
				a.randomize(true)
			}
		})
		return
	}
	n := len(pal.Colors)
	m.sel[tabPalette] = max(0, min(m.sel[tabPalette], n-1))
	sel := m.sel[tabPalette]
	name := a.s.Palette
	if name == engine.PalOff {
		name = fmt.Sprintf("terminal %d-bit", a.rend.TermDepth)
	}
	if a.s.Palette == engine.PalHarmony {
		name += " / " + a.s.Scheme
	}
	s.Text(x+2, y, fmt.Sprintf("%s — %d colors", name, n), cFg, cPanel, engine.AttrBold, w-4)
	sw, per := m.swatchLayout(n, w)
	gridH := bodyH - 4
	for i, c := range pal.Colors {
		row, col := i/per, i%per
		if row >= gridH {
			break
		}
		cx, cy, i := x+2+col*sw, y+2+row, i
		fg := uint32(0xffffff)
		if engine.Luma(c.R, c.G, c.B) > 140 {
			fg = 0
		}
		s.Fill(cx, cy, sw, 1, tty.Cell{Ch: ' ', Bg: c.U32()})
		if i == sel {
			s.Set(cx+(sw-1)/2, cy, tty.Cell{Ch: '◆', Fg: fg, Bg: c.U32()})
		}
		a.on(cx, cy, sw, 1, func(ev tty.Event, _, _ int) {
			if clicked(ev) {
				m.sel[tabPalette] = i
				if a.dbl {
					m.paletteAction(a, 'e')
				}
			}
		})
	}
	c := pal.Colors[sel]
	s.Text(x+2, y+bodyH-1, fmt.Sprintf("#%d  %s", sel+1, c.Hex()), cFg, cPanel, 0, 16)
	s.Fill(x+19, y+bodyH-1, 4, 1, tty.Cell{Ch: ' ', Bg: c.U32()})

	bx := x + 2
	type pb struct {
		label string
		fg    uint32
		act   rune
	}
	btns := []pb{{"⚄ Random", cYellow, 'R'}}
	if a.s.Palette == engine.PalAdaptive {
		btns = append(btns, pb{"Resample", cFg, 'm'})
	}
	if a.s.Palette == engine.PalCustom {
		btns = append(btns, pb{"Add", cGreen, 'a'}, pb{"Edit", cFg, 'e'}, pb{"Delete", cRed, 'd'})
	} else {
		btns = append(btns, pb{"Make custom", cAccent, 't'})
	}
	for _, b := range btns {
		act := b.act
		bx += 1 + a.button(bx, y+bodyH, b.label, b.fg, cBtn, func(ev tty.Event) {
			if clicked(ev) {
				m.paletteAction(a, act)
			}
		})
	}
}

func (m *Menu) paletteAction(a *App, act rune) {
	pal := a.rend.Palette()
	custom := a.s.Palette == engine.PalCustom
	needCustom := func() bool {
		if !custom {
			a.say("press t to turn this palette into an editable custom one", 3*time.Second)
		}
		return custom
	}
	switch act {
	case 'R':
		a.randomize(true)
	case 'm':
		a.touch()
		a.s.Resample++
		a.changed()
		a.say("adaptive palette resampled from this frame", 1500*time.Millisecond)
	case 't':
		if pal == nil || custom {
			return
		}
		a.touch()
		a.s.Custom = a.s.Custom[:0:0]
		for _, c := range pal.Colors {
			a.s.Custom = append(a.s.Custom, c.Hex())
		}
		a.s.Palette = engine.PalCustom
		a.changed()
		a.say("palette copied to custom — a add · Enter edit · d delete", 3*time.Second)
	case 'a':
		if pal != nil && !custom {
			m.paletteAction(a, 't')
		} else if pal == nil {
			a.touch()
			a.s.Color, a.s.Palette, a.s.Custom = true, engine.PalCustom, nil
		}
		a.prompt = &Prompt{title: "Add color", hint: "hex, e.g. #ff8800", buf: []rune("#"), cur: 1, done: func(v string) {
			c, err := engine.ParseHex(v)
			if err != nil {
				a.say(err.Error(), 2*time.Second)
				return
			}
			a.touch()
			a.s.Custom = append(append([]string(nil), a.s.Custom...), c.Hex())
			m.sel[tabPalette] = len(a.s.Custom) - 1
			a.changed()
		}}
	case 'e':
		if pal == nil || !needCustom() {
			return
		}
		i := m.sel[tabPalette]
		if i >= len(a.s.Custom) {
			return
		}
		cur := a.s.Custom[i]
		a.prompt = &Prompt{title: fmt.Sprintf("Edit color #%d", i+1), hint: "hex, e.g. #ff8800", buf: []rune(cur), cur: len(cur),
			done: func(v string) {
				c, err := engine.ParseHex(v)
				if err != nil {
					a.say(err.Error(), 2*time.Second)
					return
				}
				a.touch()
				cp := append([]string(nil), a.s.Custom...)
				cp[i] = c.Hex()
				a.s.Custom = cp
				a.changed()
			}}
	case 'd':
		if pal == nil || !needCustom() {
			return
		}
		i := m.sel[tabPalette]
		if len(a.s.Custom) <= 1 || i >= len(a.s.Custom) {
			a.say("a palette needs at least one color", 2*time.Second)
			return
		}
		a.touch()
		cp := append([]string(nil), a.s.Custom[:i]...)
		a.s.Custom = append(cp, a.s.Custom[i+1:]...)
		a.changed()
	}
}

// --- keyboard ----------------------------------------------------------------

// key handles a key press; it returns false to let the player handle it.
func (m *Menu) key(a *App, ev tty.Event) bool {
	switch ev.Key {
	case tty.KeyEsc:
		a.menu = nil
		return true
	case tty.KeyTab:
		m.setTab(m.tab + 1)
		return true
	case tty.KeyBackTab:
		m.setTab(m.tab - 1)
		return true
	case tty.KeyUp, tty.KeyDown:
		d := 1
		if ev.Key == tty.KeyUp {
			d = -1
		}
		if m.tab == tabPalette {
			if p := a.rend.Palette(); p != nil {
				_, per := m.swatchLayout(len(p.Colors), min(66, a.scr.W))
				if t := m.sel[m.tab] + d*per; t >= 0 && t < len(p.Colors) {
					m.sel[m.tab] = t
				}
			}
			return true
		}
		m.move(a, d)
		return true
	case tty.KeyPgUp:
		m.move(a, -5)
		return true
	case tty.KeyPgDn:
		m.move(a, 5)
		return true
	case tty.KeyHome:
		m.sel[m.tab] = 0
		return true
	case tty.KeyEnd:
		m.sel[m.tab] = max(0, m.rows(a)-1)
		return true
	case tty.KeyDelete:
		if m.tab == tabPresets {
			m.presetAction(a, 'd')
		} else if m.tab == tabPalette {
			m.paletteAction(a, 'd')
		}
		return true
	}
	if ev.Key == tty.KeyRune && ev.Rune >= '1' && ev.Rune <= '7' {
		m.setTab(int(ev.Rune - '1'))
		return true
	}
	if m.tab == tabPresets {
		switch {
		case ev.Key == tty.KeyEnter:
			m.presetAction(a, 'l')
		case ev.Key == tty.KeyRune && strings.ContainsRune("nsed", ev.Rune):
			m.presetAction(a, ev.Rune)
		case ev.Key == tty.KeyLeft:
			m.setTab(m.tab - 1)
		case ev.Key == tty.KeyRight:
			m.setTab(m.tab + 1)
		default:
			return false
		}
		return true
	}
	if m.tab == tabPalette {
		switch {
		case ev.Key == tty.KeyLeft:
			m.move(a, -1)
		case ev.Key == tty.KeyRight:
			m.move(a, 1)
		case ev.Key == tty.KeyEnter:
			m.paletteAction(a, 'e')
		case ev.Key == tty.KeyRune && strings.ContainsRune("taed", ev.Rune):
			m.paletteAction(a, ev.Rune)
		default:
			return false
		}
		return true
	}
	opts := optionsFor(menuTabs[m.tab])
	if len(opts) == 0 {
		return false
	}
	o := opts[min(m.sel[m.tab], len(opts)-1)]
	step := 1
	if ev.Shift || ev.Ctrl {
		step = 5
	}
	switch {
	case ev.Key == tty.KeyLeft:
		m.change(a, o, -step)
	case ev.Key == tty.KeyRight:
		m.change(a, o, step)
	case ev.Key == tty.KeyEnter:
		if o.Kind == engine.KBool || o.Kind == engine.KEnum {
			m.change(a, o, 1)
		} else {
			m.edit(a, o)
		}
	case ev.Key == tty.KeyRune && ev.Rune == ' ':
		if o.Kind == engine.KText || o.Kind == engine.KList {
			m.edit(a, o)
		} else {
			m.change(a, o, 1)
		}
	default:
		return false
	}
	return true
}
