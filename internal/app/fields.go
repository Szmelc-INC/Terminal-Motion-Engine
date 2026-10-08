package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
)

type fkind int

const (
	fEnum fkind = iota
	fNum
	fBool
	fText
	fAction
	fHeader
)

// field is one editable row: a setting from the option table, a preference,
// or a parameter of a generator. Menus and forms draw and drive them alike.
type field struct {
	label, help string
	kind        fkind
	dim         bool
	secret      bool
	str         func() string
	nudge       func(dir int)
	frac        func() float64
	setFrac     func(f float64)
	set         func(v string) error
	act         func()
	fg          *uint32 // label color of an action
}

func header(title string) field { return field{label: title, kind: fHeader} }

func action(label, help string, fg *uint32, fn func()) field {
	return field{label: label, help: help, kind: fAction, act: fn, fg: fg}
}

// optField adapts an entry of the option table.
func (a *App) optField(o *engine.Option) field {
	f := field{label: o.Label, dim: !o.IsActive(&a.s), str: func() string { return o.String(&a.s) }}
	f.help = "--" + o.Key
	if o.Help != "" {
		f.help = o.Help + "  (--" + o.Key + ")"
	}
	// The look and the sound take part in undo; the transport does not.
	pre := func() {
		if o.Group != "Playback" {
			a.touch()
		}
	}
	f.nudge = func(d int) { pre(); o.Nudge(&a.s, d); a.changed() }
	f.set = func(v string) error {
		pre()
		if err := o.Set(&a.s, strings.TrimSpace(v)); err != nil {
			return fmt.Errorf("%s", strings.TrimPrefix(err.Error(), "--"))
		}
		a.changed()
		return nil
	}
	switch o.Kind {
	case engine.KEnum:
		f.kind = fEnum
	case engine.KInt, engine.KFloat:
		f.kind = fNum
		f.frac = func() float64 { return o.Frac(&a.s) }
		f.setFrac = func(v float64) { pre(); o.SetFrac(&a.s, v); a.changed() }
	case engine.KBool:
		f.kind = fBool
	default:
		f.kind, f.nudge = fText, nil
	}
	return f
}

func (a *App) optFields(group string) []field {
	var out []field
	for _, o := range engine.Options {
		if o.Group == group {
			out = append(out, a.optField(o))
		}
	}
	return out
}

func scrollInto(sel, scroll *int, n, h int) {
	*sel = max(0, min(*sel, n-1))
	if *sel < *scroll {
		*scroll = *sel
	}
	if *sel >= *scroll+h {
		*scroll = *sel - h + 1
	}
	*scroll = max(0, min(*scroll, max(0, n-h)))
}

// editField opens a text prompt for a field's value.
func (a *App) editField(f field) {
	if f.set == nil {
		return
	}
	cur := ""
	if !f.secret {
		cur = f.str()
	}
	a.prompt = &Prompt{title: f.label, hint: f.help, buf: []rune(cur), cur: len([]rune(cur)), done: func(v string) {
		if err := f.set(v); err != nil {
			a.say(err.Error(), 3*time.Second)
		}
	}}
}

// drawFields draws rows of fields in h lines starting at y, with the help
// text of the selected one on the line after a blank one.
func (a *App) drawFields(fs []field, sel, scroll *int, x, y, w, h int) {
	s := a.scr
	scrollInto(sel, scroll, len(fs), h)
	lw := 16 + (w-66)/4
	wx, ww := x+lw+2, w-lw-4
	for i := *scroll; i < len(fs) && i-*scroll < h; i++ {
		f, i, ry := fs[i], i, y+i-*scroll
		if f.kind == fHeader {
			s.Text(x+2, ry, "── "+f.label+" "+strings.Repeat("─", max(0, w-9-len([]rune(f.label)))), cDim, cPanel, 0, w-4)
			continue
		}
		bg := cPanel
		if i == *sel {
			bg = cSel
		}
		s.Fill(x+1, ry, w-2, 1, tty.Cell{Ch: ' ', Bg: bg})
		fg := cFg
		if f.dim {
			fg = cDim
		}
		pick := func() { *sel = i }
		if f.kind == fAction {
			afg := cAccent
			if f.fg != nil {
				afg = *f.fg
			}
			s.Text(x+2, ry, "▸ "+f.label, afg, bg, engine.AttrBold, w-4)
			a.on(x+1, ry, w-2, 1, func(ev tty.Event, _, _ int) {
				switch ev.Action {
				case tty.MouseMove:
					pick()
				case tty.MousePress:
					pick()
					if ev.Button == tty.ButtonLeft {
						f.act()
					}
				}
			})
			continue
		}
		s.Text(x+2, ry, f.label, fg, bg, 0, lw-1)
		a.on(x+1, ry, w-2, 1, func(ev tty.Event, _, _ int) {
			switch ev.Action {
			case tty.MouseMove, tty.MousePress:
				pick()
			case tty.MouseWheelUp:
				pick()
				if f.nudge != nil {
					f.nudge(1)
				}
			case tty.MouseWheelDown:
				pick()
				if f.nudge != nil {
					f.nudge(-1)
				}
			}
		})
		val := f.str()
		switch f.kind {
		case fEnum:
			s.Text(wx, ry, "◀", cAccent, bg, 0, -1)
			s.Text(wx+ww-1, ry, "▶", cAccent, bg, 0, -1)
			vr := []rune(tty.Clean(val))
			if len(vr) > ww-4 {
				vr = vr[:max(0, ww-4)]
			}
			s.Text(wx+(ww-len(vr))/2, ry, string(vr), fg, bg, engine.AttrBold, -1)
			a.on(wx-1, ry, ww+2, 1, func(ev tty.Event, rx, _ int) {
				pick()
				switch {
				case ev.Action == tty.MouseWheelUp:
					f.nudge(-1)
				case ev.Action == tty.MouseWheelDown:
					f.nudge(1)
				case ev.Action != tty.MousePress:
				case ev.Button == tty.ButtonRight || (ev.Button == tty.ButtonLeft && rx < 4):
					f.nudge(-1)
				case ev.Button == tty.ButtonLeft:
					f.nudge(1)
				}
			})
		case fNum:
			sw := ww - 8
			a.slider(wx, ry, sw, f.frac(), cAccent, bg)
			s.Text(wx+sw+1, ry, fmt.Sprintf("%7s", val), fg, bg, engine.AttrBold, 7)
			a.on(wx, ry, sw, 1, func(ev tty.Event, rx, _ int) {
				pick()
				switch ev.Action {
				case tty.MousePress, tty.MouseDrag:
					if ev.Button == tty.ButtonLeft {
						f.setFrac(sliderFrac(rx, sw))
					}
				case tty.MouseWheelUp:
					f.nudge(1)
				case tty.MouseWheelDown:
					f.nudge(-1)
				}
			})
			a.on(wx+sw+1, ry, 7, 1, func(ev tty.Event, _, _ int) {
				pick()
				if clicked(ev) {
					a.editField(f)
				}
			})
		case fBool:
			mark, mfg := "○ off", cDim
			if val == "on" {
				mark, mfg = "● on", cGreen
			}
			s.Text(wx, ry, mark, mfg, bg, engine.AttrBold, -1)
			a.on(wx-1, ry, 8, 1, func(ev tty.Event, _, _ int) {
				pick()
				if clicked(ev) {
					f.nudge(1)
				}
			})
		default:
			switch {
			case val == "":
				val, fg = "(empty — Enter to edit)", cDim
			case f.secret:
				val = strings.Repeat("•", min(12, len(val))) + "  (set)"
			}
			s.Text(wx, ry, tty.Clean(val), fg, bg, 0, ww)
			a.on(wx, ry, ww, 1, func(ev tty.Event, _, _ int) {
				pick()
				if clicked(ev) {
					a.editField(f)
				}
			})
		}
	}
	if *sel < len(fs) && fs[*sel].help != "" {
		s.Text(x+2, y+h+1, fs[*sel].help, cYellow, cPanel, 0, w-4)
	}
	if len(fs) > h {
		s.Text(x+w-12, y+h, fmt.Sprintf(" %d/%d ↕ ", *sel+1, len(fs)), cDim, cPanel, 0, -1)
	}
}

// fieldsKey handles navigation and editing keys for a list of fields.
func (a *App) fieldsKey(fs []field, sel *int, ev tty.Event) bool {
	n := len(fs)
	if n == 0 {
		return false
	}
	*sel = max(0, min(*sel, n-1))
	move := func(d int) {
		// Step over headers, which cannot be selected.
		for k := 0; k < n; k++ {
			*sel = ((*sel+d)%n + n) % n
			if fs[*sel].kind != fHeader {
				return
			}
			if d > 1 || d < -1 {
				d = d / max(d, -d)
			}
		}
	}
	if fs[*sel].kind == fHeader {
		move(1)
	}
	f := fs[*sel]
	step := 1
	if ev.Shift || ev.Ctrl {
		step = 5
	}
	switch {
	case ev.Key == tty.KeyUp:
		move(-1)
	case ev.Key == tty.KeyDown:
		move(1)
	case ev.Key == tty.KeyPgUp:
		*sel = max(0, *sel-8)
		if fs[*sel].kind == fHeader {
			move(1)
		}
	case ev.Key == tty.KeyPgDn:
		*sel = min(n-1, *sel+8)
		if fs[*sel].kind == fHeader {
			move(-1)
		}
	case ev.Key == tty.KeyHome:
		*sel = 0
		if fs[0].kind == fHeader {
			move(1)
		}
	case ev.Key == tty.KeyEnd:
		*sel = n - 1
	case ev.Key == tty.KeyLeft && f.nudge != nil:
		f.nudge(-step)
	case ev.Key == tty.KeyRight && f.nudge != nil:
		f.nudge(step)
	case ev.Key == tty.KeyEnter, ev.Key == tty.KeyRune && ev.Rune == ' ':
		switch {
		case f.kind == fAction:
			f.act()
		case f.kind == fBool, f.kind == fEnum, f.kind == fNum && ev.Key != tty.KeyEnter:
			f.nudge(1)
		default:
			a.editField(f)
		}
	default:
		return false
	}
	return true
}

// Form is a modal window of fields with an optional live preview on top:
// the palette, charset and theme generators are forms.
type Form struct {
	title       string
	fields      func() []field
	preview     func(a *App, x, y, w int) // draws into previewH lines
	previewH    int
	sel, scroll int
}

func (f *Form) draw(a *App) {
	s := a.scr
	fs := f.fields()
	w, h := a.dim(64, min(len(fs), 14)+f.previewH+6)
	x, y := a.window(f.title, w, h)
	a.on(0, 0, s.W, s.H, func(tty.Event, int, int) {})
	s.Text(x+w-4, y, " ✕ ", cRed, cSel, engine.AttrBold, -1)
	a.on(x+w-4, y, 3, 1, func(ev tty.Event, _, _ int) {
		if clicked(ev) {
			a.form = nil
		}
	})
	top := y + 2
	if f.preview != nil {
		f.preview(a, x+2, top, w-4)
		top += f.previewH + 1
	}
	bodyH := y + h - 3 - top
	if bodyH < 1 {
		return
	}
	a.drawFields(fs, &f.sel, &f.scroll, x, top, w, bodyH)
	s.Text(x+2, y+h-1, "↑↓ select · ←→ change · Enter edit / run · Esc close", cDim, cPanel, 0, w-4)
}

func (f *Form) key(a *App, ev tty.Event) {
	if ev.Key == tty.KeyEsc {
		a.form = nil
		return
	}
	a.fieldsKey(f.fields(), &f.sel, ev)
}

// swatchRow paints colors as a strip w cells wide.
func (a *App) swatchRow(x, y, w int, colors []engine.RGB) {
	if len(colors) == 0 || w < 1 {
		return
	}
	for i := 0; i < w; i++ {
		c := colors[i*len(colors)/w]
		a.scr.Set(x+i, y, tty.Cell{Ch: ' ', Bg: c.U32()})
	}
}
