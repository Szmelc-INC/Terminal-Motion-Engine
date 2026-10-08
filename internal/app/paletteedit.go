package app

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
)

func newRand(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }

// paletteEditPage shows the colors the picture is drawn with and edits them
// one by one once they are a custom palette.
type paletteEditPage struct {
	sel    int
	w      int
	source string // user palette being edited, if any: s saves back to it
}

func (p *paletteEditPage) label() string { return "Editor" }

func (p *paletteEditPage) hint() string {
	return "←→↑↓ pick · t make custom · a add · Enter edit · d delete · s save · R random"
}

func (p *paletteEditPage) swatchLayout(n, w int) (sw, perRow int) {
	sw = 2
	if n <= 16 {
		sw = 4
	} else if n <= 64 {
		sw = 3
	}
	return sw, max(1, (w-4)/sw)
}

func (p *paletteEditPage) draw(a *App, m *Menu, x, y, w, bodyH int) {
	s := a.scr
	p.w = w
	pal := a.rend.Palette()
	if pal == nil {
		msg := "Truecolor output — there is no palette to show."
		if !a.s.Color {
			msg = "Color is off — the picture uses the terminal's own colors."
		}
		s.Text(x+2, y, msg, cFg, cPanel, 0, w-4)
		s.Text(x+2, y+2, "Press R (or click below) for a random color-theory palette,", cDim, cPanel, 0, w-4)
		s.Text(x+2, y+3, "or pick one on the Palettes tab.", cDim, cPanel, 0, w-4)
		a.button(x+2, y+5, "⚄ Random palette", cYellow, cHot, func(ev tty.Event) {
			if clicked(ev) {
				a.randomize(true)
			}
		})
		return
	}
	n := len(pal.Colors)
	p.sel = max(0, min(p.sel, n-1))
	name := a.s.Palette
	if name == engine.PalOff {
		name = fmt.Sprintf("terminal %d-bit", a.rend.TermDepth)
	}
	if a.s.Palette == engine.PalHarmony {
		name += " / " + a.s.Scheme
	}
	if a.s.Palette == engine.PalCustom && p.source != "" {
		name += " (editing " + p.source + ")"
	}
	s.Text(x+2, y, fmt.Sprintf("%s — %d colors", name, n), cFg, cPanel, engine.AttrBold, w-4)
	sw, per := p.swatchLayout(n, w)
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
		if i == p.sel {
			s.Set(cx+(sw-1)/2, cy, tty.Cell{Ch: '◆', Fg: fg, Bg: c.U32()})
		}
		a.on(cx, cy, sw, 1, func(ev tty.Event, _, _ int) {
			if clicked(ev) {
				p.sel = i
				if a.dbl {
					p.action(a, 'e')
				}
			}
		})
	}
	c := pal.Colors[p.sel]
	s.Text(x+2, y+bodyH-1, fmt.Sprintf("#%d  %s", p.sel+1, c.Hex()), cFg, cPanel, 0, 16)
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
	btns = append(btns, pb{"Save as…", cGreen, 's'})
	for _, b := range btns {
		act := b.act
		bx += 1 + a.button(bx, y+bodyH, b.label, b.fg, cBtn, func(ev tty.Event) {
			if clicked(ev) {
				p.action(a, act)
			}
		})
	}
}

func (p *paletteEditPage) action(a *App, act rune) {
	pal := a.rend.Palette()
	custom := a.s.Palette == engine.PalCustom
	needCustom := func() bool {
		if !custom {
			a.say("press t to turn this palette into an editable custom one", 3*time.Second)
		}
		return custom
	}
	editColor := func(title, cur string, done func(c engine.RGB)) {
		a.prompt = &Prompt{title: title, hint: "hex, e.g. #ff8800", buf: []rune(cur), cur: len(cur), done: func(v string) {
			c, err := engine.ParseHex(v)
			if a.fail(err) {
				return
			}
			a.touch()
			done(c)
			a.changed()
		}}
	}
	switch act {
	case 'R':
		a.randomize(true)
	case 'm':
		a.touch()
		a.s.Resample++
		a.changed()
		a.say("adaptive palette resampled from this frame", 1500*time.Millisecond)
	case 's':
		a.savePaletteAs(a.currentColors(), p.source, func(n string) { p.source = ""; a.setPalette(n) })
	case 't':
		if pal == nil || custom {
			return
		}
		a.touch()
		a.s.Custom = a.s.Custom[:0:0]
		for _, c := range pal.Colors {
			a.s.Custom = append(a.s.Custom, c.Hex())
		}
		a.s.Palette, p.source = engine.PalCustom, ""
		a.changed()
		a.say("palette copied to custom — a add · Enter edit · d delete · s save", 3*time.Second)
	case 'a':
		if pal != nil && !custom {
			p.action(a, 't')
		} else if pal == nil {
			a.touch()
			a.s.Color, a.s.Palette, a.s.Custom = true, engine.PalCustom, nil
		}
		editColor("Add color", "#", func(c engine.RGB) {
			a.s.Custom = append(append([]string(nil), a.s.Custom...), c.Hex())
			p.sel = len(a.s.Custom) - 1
		})
	case 'e':
		if pal == nil || !needCustom() || p.sel >= len(a.s.Custom) {
			return
		}
		i := p.sel
		editColor(fmt.Sprintf("Edit color #%d", i+1), a.s.Custom[i], func(c engine.RGB) {
			cp := append([]string(nil), a.s.Custom...)
			cp[i] = c.Hex()
			a.s.Custom = cp
		})
	case 'd':
		if pal == nil || !needCustom() {
			return
		}
		i := p.sel
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

func (p *paletteEditPage) key(a *App, m *Menu, ev tty.Event) bool {
	n := 0
	if pal := a.rend.Palette(); pal != nil {
		n = len(pal.Colors)
	}
	_, per := p.swatchLayout(n, max(p.w, 20))
	step := func(d int) {
		if t := p.sel + d; t >= 0 && t < n {
			p.sel = t
		}
	}
	switch {
	case ev.Key == tty.KeyUp:
		step(-per)
	case ev.Key == tty.KeyDown:
		step(per)
	case ev.Key == tty.KeyLeft:
		step(-1)
	case ev.Key == tty.KeyRight:
		step(1)
	case ev.Key == tty.KeyHome:
		p.sel = 0
	case ev.Key == tty.KeyEnd:
		p.sel = max(0, n-1)
	case ev.Key == tty.KeyEnter:
		p.action(a, 'e')
	case ev.Key == tty.KeyDelete:
		p.action(a, 'd')
	case ev.Key == tty.KeyRune && !ev.Ctrl && !ev.Alt && strings.ContainsRune("taedsRm", ev.Rune):
		p.action(a, ev.Rune)
	default:
		return false
	}
	return true
}
