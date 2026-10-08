package app

import (
	"time"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
)

// page is one tab of a panel. It draws into the body area it is given and
// may also use the two lines below it (buttons, help text).
type page interface {
	label() string
	draw(a *App, m *Menu, x, y, w, h int)
	key(a *App, m *Menu, ev tty.Event) bool
	hint() string
}

// panel is a window of pages: a group of related settings or a manager.
type panel struct {
	id, title string
	pages     []page
}

// Menu is an open panel. Pages keep their own selection, so reopening a
// panel finds it the way it was left.
type Menu struct {
	panel        *panel
	tab          int
	x, y         int
	w            int // width at the last draw
	placed       bool
	grabX, grabY int
}

// optionsPage lists the options of one group of the option table.
type optionsPage struct {
	group, name string
	sel, scroll int
}

func (p *optionsPage) label() string {
	if p.name != "" {
		return p.name
	}
	return p.group
}

func (p *optionsPage) hint() string {
	return "↑↓ select · ←→ change · Enter edit · Tab next tab · Esc close"
}

func (p *optionsPage) draw(a *App, m *Menu, x, y, w, h int) {
	a.drawFields(a.optFields(p.group), &p.sel, &p.scroll, x, y, w, h)
}

func (p *optionsPage) key(a *App, m *Menu, ev tty.Event) bool {
	return a.fieldsKey(a.optFields(p.group), &p.sel, ev)
}

// fieldsPage lists fields built on the fly (preferences).
type fieldsPage struct {
	name        string
	fields      func(a *App) []field
	sel, scroll int
}

func (p *fieldsPage) label() string { return p.name }
func (p *fieldsPage) hint() string {
	return "↑↓ select · ←→ change · Enter edit · Tab next tab · Esc close"
}
func (p *fieldsPage) draw(a *App, m *Menu, x, y, w, h int) {
	a.drawFields(p.fields(a), &p.sel, &p.scroll, x, y, w, h)
}
func (p *fieldsPage) key(a *App, m *Menu, ev tty.Event) bool {
	return a.fieldsKey(p.fields(a), &p.sel, ev)
}

// touch records an undo point, coalescing bursts of small edits into one.
func (a *App) touch() {
	if time.Since(a.lastTouch) > 800*time.Millisecond {
		a.pushHistory()
	}
	a.lastTouch = time.Now()
}

func (m *Menu) page() page { return m.panel.pages[m.tab] }

func (m *Menu) setTab(t int) {
	n := len(m.panel.pages)
	m.tab = (t%n + n) % n
}

func (m *Menu) draw(a *App) {
	s := a.scr
	w, h := a.dim(66, 22)
	m.w = w
	if !m.placed {
		m.x, m.y, m.placed = (s.W-w)/2, max(0, (s.H-h)/2-1), true
	}
	m.x = max(0, min(s.W-w, m.x))
	m.y = max(0, min(s.H-h, m.y))
	x, y := m.x, m.y
	a.frame(m.panel.title, x, y, w, h)
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
	for i, pg := range m.panel.pages {
		label := " " + pg.label() + " "
		if tx+len([]rune(label)) > x+w {
			break
		}
		fg, bg, attr := cDim, cPanel, uint8(0)
		if i == m.tab {
			fg, bg, attr = cPanel, cAccent, engine.AttrBold
		} else if a.hover(tx, y+1, len([]rune(label)), 1) {
			fg, bg = cFg, cHot
		}
		s.Text(tx, y+1, label, fg, bg, attr, -1)
		i := i
		a.on(tx, y+1, len([]rune(label)), 1, func(ev tty.Event, _, _ int) {
			if clicked(ev) {
				m.setTab(i)
			}
		})
		tx += len([]rune(label))
	}

	bodyY, bodyH := y+3, h-6
	if bodyH < 1 {
		return
	}
	m.page().draw(a, m, x, bodyY, w, bodyH)
	s.Text(x+2, y+h-1, m.page().hint(), cDim, cPanel, 0, w-4)
}

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
	}
	if ev.Key == tty.KeyRune && !ev.Ctrl && !ev.Alt && ev.Rune >= '1' && ev.Rune <= '9' &&
		int(ev.Rune-'1') < len(m.panel.pages) {
		m.setTab(int(ev.Rune - '1'))
		return true
	}
	return m.page().key(a, m, ev)
}

// --- panels ------------------------------------------------------------------

// The pages and panels are wired up in init: they refer to each other
// through the actions they run.
var (
	pgPresets, pgPalettes, pgCharsets, pgThemes, pgSounds *managerPage
	pgEditor                                              = &paletteEditPage{}
	pgPrefs                                               *fieldsPage
	panels                                                []*panel
)

func init() {
	pgPresets, pgPalettes = newPresetManager(), newPaletteManager()
	pgCharsets, pgThemes = newCharsetManager(), newThemeManager()
	pgSounds = newSoundManager()
	audio := func(group string) page { return &optionsPage{group: "Audio: " + group, name: group} }
	pgPrefs = &fieldsPage{name: "Preferences", fields: prefFields}
	panels = []*panel{
		{id: "picture", title: "Picture", pages: []page{
			&optionsPage{group: "Render"}, &optionsPage{group: "Color"}, &optionsPage{group: "Dither"},
			&optionsPage{group: "Adjust"}, &optionsPage{group: "Playback"},
		}},
		{id: "sound", title: "Sound — tone, dynamics, space", pages: []page{
			audio("Tone"), audio("EQ"), audio("Dynamics"), audio("Space"),
		}},
		{id: "soundfx", title: "Sound effects — motion, lo-fi, synth", pages: []page{
			audio("Motion"), audio("Lo-fi"), audio("Synth"),
		}},
		{id: "presets", title: "Presets", pages: []page{pgPresets, pgSounds}},
		{id: "palettes", title: "Palettes & symbols", pages: []page{pgPalettes, pgEditor, pgCharsets}},
		{id: "prefs", title: "Preferences & themes", pages: []page{pgPrefs, pgThemes}},
	}
}

func findPanel(id string) *panel {
	for _, p := range panels {
		if p.id == id {
			return p
		}
	}
	return nil
}

// openPanel shows a panel, or closes it when it is already in front. tab
// selects a page; -1 keeps the one that was open last time.
func (a *App) openPanel(id string, tab int) {
	if a.menu != nil && a.menu.panel.id == id && (tab < 0 || tab == a.menu.tab) {
		a.menu = nil
		return
	}
	p := findPanel(id)
	if p == nil {
		return
	}
	if a.menus == nil {
		a.menus = map[string]*Menu{}
	}
	m := a.menus[id]
	if m == nil {
		m = &Menu{panel: p}
		a.menus[id] = m
	}
	if a.menu != nil {
		// Keep the window where the previous panel was.
		m.x, m.y, m.placed = a.menu.x, a.menu.y, a.menu.placed
	}
	if tab >= 0 {
		m.setTab(tab)
	}
	a.menu = m
}

// showPage opens the panel that holds a page, on that page.
func (a *App) showPage(pg page) {
	for _, p := range panels {
		for i, q := range p.pages {
			if q == pg {
				if a.menu != nil && a.menu.panel == p {
					a.menu.setTab(i)
					return
				}
				a.openPanel(p.id, i)
				return
			}
		}
	}
}

func (a *App) toggleMenu(tab int) { a.openPanel("picture", tab) }
