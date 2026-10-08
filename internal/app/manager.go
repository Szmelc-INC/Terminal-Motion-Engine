package app

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
)

// mItem is one row of a manager list.
type mItem struct {
	name, tag, info string
	user, active    bool
	swatch          []engine.RGB
}

// mAction is something a manager can do with the selected item. The first
// action of a manager is the one Enter runs.
type mAction struct {
	key   rune
	label string
	fg    *uint32
	fn    func(a *App, it mItem, ok bool)
}

// managerPage is a list of named things (presets, palettes, charsets,
// themes…) with a row of actions. Every manager in termo is one of these.
type managerPage struct {
	name        string
	items       func(a *App) []mItem
	actions     []mAction
	sel, scroll int
}

func (p *managerPage) label() string { return p.name }

func (p *managerPage) hint() string {
	parts := []string{"Enter " + strings.ToLower(p.actions[0].label)}
	for _, ac := range p.actions[1:] {
		parts = append(parts, string(ac.key)+" "+strings.ToLower(ac.label))
	}
	return strings.Join(parts, " · ")
}

func (p *managerPage) selected(a *App) (mItem, bool) {
	items := p.items(a)
	if len(items) == 0 {
		return mItem{}, false
	}
	p.sel = max(0, min(p.sel, len(items)-1))
	return items[p.sel], true
}

func (p *managerPage) run(a *App, key rune) bool {
	for _, ac := range p.actions {
		if ac.key == key {
			it, ok := p.selected(a)
			ac.fn(a, it, ok)
			return true
		}
	}
	return false
}

// selectName moves the selection to the item with the given name.
func (p *managerPage) selectName(a *App, name string) {
	for i, it := range p.items(a) {
		if strings.EqualFold(it.name, name) {
			p.sel = i
		}
	}
}

func (p *managerPage) draw(a *App, m *Menu, x, y, w, h int) {
	s := a.scr
	items := p.items(a)
	listH := h - 1
	scrollInto(&p.sel, &p.scroll, len(items), listH)
	wheel := func(ev tty.Event) {
		switch ev.Action {
		case tty.MouseWheelUp:
			p.sel = max(0, p.sel-1)
		case tty.MouseWheelDown:
			p.sel = min(len(items)-1, p.sel+1)
		}
	}
	a.on(x+1, y, w-2, listH, func(ev tty.Event, _, _ int) { wheel(ev) })
	nw := 18 + (w-66)/5
	for i := p.scroll; i < len(items) && i-p.scroll < listH; i++ {
		it, i, ry := items[i], i, y+i-p.scroll
		bg := cPanel
		if i == p.sel {
			bg = cSel
		} else if a.hover(x+1, ry, w-2, 1) {
			bg = cHot
		}
		s.Fill(x+1, ry, w-2, 1, tty.Cell{Ch: ' ', Bg: bg})
		if it.active {
			s.Text(x+2, ry, "◆", cGreen, bg, 0, -1)
		}
		s.Text(x+4, ry, tty.Clean(it.name), cFg, bg, engine.AttrBold, nw)
		tag, tfg := it.tag, cDim
		if tag == "" {
			tag = "built-in"
			if it.user {
				tag, tfg = "user", cGreen
			}
		}
		s.Text(x+5+nw, ry, tag, tfg, bg, 0, 8)
		rx := x + 14 + nw
		if rw := x + w - 2 - rx; rw > 0 {
			if len(it.swatch) > 0 {
				a.swatchRow(rx, ry, min(rw, max(len(it.swatch), min(rw, 16))), it.swatch)
			} else {
				s.Text(rx, ry, tty.Clean(it.info), cDim, bg, 0, rw)
			}
		}
		a.on(x+1, ry, w-2, 1, func(ev tty.Event, _, _ int) {
			if ev.Action != tty.MousePress {
				wheel(ev)
				return
			}
			if ev.Button != tty.ButtonLeft {
				return
			}
			again := p.sel == i
			p.sel = i
			if a.dbl || again {
				p.actions[0].fn(a, it, true)
			}
		})
	}
	if len(items) == 0 {
		s.Text(x+2, y, "Nothing here yet.", cDim, cPanel, 0, w-4)
	}
	bx := x + 2
	for _, ac := range p.actions {
		lw := len([]rune(ac.label)) + 2 + 2*a.pad()
		if bx+lw > x+w-1 {
			break
		}
		fg := cFg
		if ac.fg != nil {
			fg = *ac.fg
		}
		key := ac.key
		bx += 1 + a.button(bx, y+h, ac.label, fg, cBtn, func(ev tty.Event) {
			if clicked(ev) {
				p.run(a, key)
			}
		})
	}
	if len(items) > listH {
		s.Text(x+w-12, y+h+1, fmt.Sprintf(" %d/%d ↕ ", p.sel+1, len(items)), cDim, cPanel, 0, -1)
	}
}

func (p *managerPage) key(a *App, m *Menu, ev tty.Event) bool {
	n := len(p.items(a))
	switch ev.Key {
	case tty.KeyUp:
		p.sel = ((p.sel-1)%max(n, 1) + max(n, 1)) % max(n, 1)
	case tty.KeyDown:
		p.sel = (p.sel + 1) % max(n, 1)
	case tty.KeyPgUp:
		p.sel = max(0, p.sel-8)
	case tty.KeyPgDn:
		p.sel = min(n-1, p.sel+8)
	case tty.KeyHome:
		p.sel = 0
	case tty.KeyEnd:
		p.sel = n - 1
	case tty.KeyLeft:
		m.setTab(m.tab - 1)
	case tty.KeyRight:
		m.setTab(m.tab + 1)
	case tty.KeyEnter:
		p.run(a, p.actions[0].key)
	case tty.KeyDelete:
		p.run(a, 'd')
	case tty.KeyRune:
		if ev.Ctrl || ev.Alt {
			return false
		}
		if p.run(a, ev.Rune) {
			return true
		}
		// Type a letter that is not an action to jump to a name.
		r := unicode.ToLower(ev.Rune)
		items := p.items(a)
		for k := 1; k <= len(items); k++ {
			i := (p.sel + k) % len(items)
			if nr := []rune(strings.ToLower(items[i].name)); len(nr) > 0 && nr[0] == r {
				p.sel = i
				return true
			}
		}
		return false
	default:
		return false
	}
	return true
}

// --- small dialog helpers ------------------------------------------------------

// ask opens a one-line prompt.
func (a *App) ask(title, hint, initial string, done func(v string)) {
	a.prompt = &Prompt{title: title, hint: hint, buf: []rune(initial), cur: len([]rune(initial)),
		done: func(v string) { done(strings.TrimSpace(v)) }}
}

// fail reports an error as a toast and says whether there was one.
func (a *App) fail(err error) bool {
	if err != nil {
		a.say(err.Error(), 4*time.Second)
		return true
	}
	return false
}

func (a *App) ok(msg string) { a.say(msg, 1800*time.Millisecond) }

// userOnly guards actions that cannot touch built-in items.
func (a *App) userOnly(it mItem, ok bool, what string) bool {
	if !ok {
		return false
	}
	if !it.user {
		a.say("built-in items are read-only — press c to "+what+" a copy", 3*time.Second)
		return false
	}
	return true
}
