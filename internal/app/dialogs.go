package app

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
)

// Prompt is a one-line text input dialog.
type Prompt struct {
	title, hint string
	buf         []rune
	cur         int
	done        func(string)
}

func (p *Prompt) draw(a *App) {
	s := a.scr
	w, _ := a.dim(56, 6)
	x, y := a.window(p.title, w, 6)
	a.on(0, 0, s.W, s.H, func(tty.Event, int, int) {})
	if p.hint != "" {
		s.Text(x+2, y+1, p.hint, cDim, cPanel, 0, w-4)
	}
	fw := w - 4
	s.Fill(x+2, y+2, fw, 1, tty.Cell{Ch: ' ', Bg: cBar})
	first := 0
	if p.cur >= fw {
		first = p.cur - fw + 1
	}
	for i := 0; i < fw; i++ {
		c := tty.Cell{Ch: ' ', Fg: cFg, Bg: cBar}
		if first+i < len(p.buf) {
			c.Ch = p.buf[first+i]
		}
		if first+i == p.cur {
			c.Attr = engine.AttrReverse
		}
		s.Set(x+2+i, y+2, c)
	}
	bx := x + 2
	bx += 1 + a.button(bx, y+4, "OK ⏎", cGreen, cBtn, func(ev tty.Event) {
		if clicked(ev) {
			p.submit(a)
		}
	})
	a.button(bx, y+4, "Cancel", cFg, cBtn, func(ev tty.Event) {
		if clicked(ev) {
			a.prompt = nil
		}
	})
}

func (p *Prompt) submit(a *App) {
	a.prompt = nil
	p.done(string(p.buf))
}

func (p *Prompt) key(a *App, ev tty.Event) {
	switch ev.Key {
	case tty.KeyEsc:
		a.prompt = nil
	case tty.KeyEnter:
		p.submit(a)
	case tty.KeyLeft:
		p.cur = max(0, p.cur-1)
	case tty.KeyRight:
		p.cur = min(len(p.buf), p.cur+1)
	case tty.KeyHome:
		p.cur = 0
	case tty.KeyEnd:
		p.cur = len(p.buf)
	case tty.KeyBackspace:
		if p.cur > 0 {
			p.buf = append(p.buf[:p.cur-1], p.buf[p.cur:]...)
			p.cur--
		}
	case tty.KeyDelete:
		if p.cur < len(p.buf) {
			p.buf = append(p.buf[:p.cur], p.buf[p.cur+1:]...)
		}
	case tty.KeyRune:
		if ev.Ctrl || ev.Alt || !unicode.IsPrint(ev.Rune) || len(p.buf) >= 512 {
			return
		}
		p.buf = append(p.buf[:p.cur], append([]rune{ev.Rune}, p.buf[p.cur:]...)...)
		p.cur++
	}
}

// Confirm is a yes/no dialog.
type Confirm struct {
	msg string
	yes func()
}

func (c *Confirm) draw(a *App) {
	s := a.scr
	w := min(max(34, len([]rune(c.msg))+6), s.W)
	x, y := a.window("Confirm", w, 5)
	a.on(0, 0, s.W, s.H, func(tty.Event, int, int) {})
	s.Text(x+2, y+1, tty.Clean(c.msg), cFg, cPanel, 0, w-4)
	bx := x + 2
	bx += 1 + a.button(bx, y+3, "Yes (y)", cGreen, cBtn, func(ev tty.Event) {
		if clicked(ev) {
			c.answer(a, true)
		}
	})
	a.button(bx, y+3, "No (n)", cRed, cBtn, func(ev tty.Event) {
		if clicked(ev) {
			c.answer(a, false)
		}
	})
}

func (c *Confirm) answer(a *App, yes bool) {
	a.confirm = nil
	if yes {
		c.yes()
	}
}

func (c *Confirm) key(a *App, ev tty.Event) {
	switch {
	case ev.Key == tty.KeyEnter, ev.Key == tty.KeyRune && (ev.Rune == 'y' || ev.Rune == 'Y'):
		c.answer(a, true)
	case ev.Key == tty.KeyEsc, ev.Key == tty.KeyRune && (ev.Rune == 'n' || ev.Rune == 'N' || ev.Rune == 'q'):
		c.answer(a, false)
	}
}

// MediaExts are the file extensions the file browser lists.
var MediaExts = map[string]bool{
	".mp4": true, ".mkv": true, ".webm": true, ".avi": true, ".mov": true, ".wmv": true, ".flv": true,
	".mpg": true, ".mpeg": true, ".m4v": true, ".ts": true, ".mts": true, ".m2ts": true, ".ogv": true,
	".3gp": true, ".gif": true, ".apng": true, ".png": true, ".jpg": true, ".jpeg": true, ".bmp": true,
	".webp": true, ".m3u8": true,
}

type bitem struct {
	name string
	dir  bool
}

// Browser is the file-open dialog.
type Browser struct {
	dir    string
	items  []bitem
	sel    int
	scroll int
	err    string
}

// openBrowserAt opens the file browser on a folder.
func (a *App) openBrowserAt(dir string) {
	b := &Browser{}
	b.load(dir)
	a.browser = b
}

func (a *App) openBrowser() {
	dir, _ := os.Getwd()
	cur := ""
	if a.loaded && a.fileIdx < len(a.files) {
		cur = a.files[a.fileIdx]
		if abs, err := filepath.Abs(cur); err == nil {
			dir = filepath.Dir(abs)
		}
	}
	b := &Browser{}
	b.load(dir)
	if cur != "" {
		base := filepath.Base(cur)
		for i, it := range b.items {
			if it.name == base {
				b.sel = i
			}
		}
	}
	a.browser = b
}

func (b *Browser) load(dir string) {
	b.dir, b.sel, b.scroll, b.err = dir, 0, 0, ""
	b.items = []bitem{{"..", true}}
	ents, err := os.ReadDir(dir)
	if err != nil {
		b.err = err.Error()
		return
	}
	var dirs, files []bitem
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(dir, name)); err == nil {
				isDir = st.IsDir()
			}
		}
		if isDir {
			dirs = append(dirs, bitem{name, true})
		} else if MediaExts[strings.ToLower(filepath.Ext(name))] {
			files = append(files, bitem{name, false})
		}
	}
	less := func(l []bitem) func(i, j int) bool {
		return func(i, j int) bool { return strings.ToLower(l[i].name) < strings.ToLower(l[j].name) }
	}
	sort.Slice(dirs, less(dirs))
	sort.Slice(files, less(files))
	b.items = append(append(b.items, dirs...), files...)
}

func (b *Browser) move(d int) {
	b.sel = max(0, min(len(b.items)-1, b.sel+d))
}

func (b *Browser) activate(a *App) {
	if b.sel >= len(b.items) {
		return
	}
	it := b.items[b.sel]
	if it.dir {
		prev := filepath.Base(b.dir)
		b.load(filepath.Clean(filepath.Join(b.dir, it.name)))
		if it.name == ".." {
			for i, x := range b.items {
				if x.name == prev {
					b.sel = i
				}
			}
		}
		return
	}
	// Opening a file queues every media file in its folder, so n / N
	// step through the directory.
	var files []string
	idx := 0
	for _, x := range b.items {
		if x.dir {
			continue
		}
		if x.name == it.name {
			idx = len(files)
		}
		files = append(files, filepath.Join(b.dir, x.name))
	}
	a.files = files
	a.browser, a.finderOpen = nil, false
	a.open(idx, 0)
}

func (b *Browser) draw(a *App) {
	s := a.scr
	w, h := a.dim(72, 24)
	x, y := a.window("Open — "+tty.Clean(b.dir), w, h)
	a.on(0, 0, s.W, s.H, func(tty.Event, int, int) {})
	listH := h - 3
	if b.sel < b.scroll {
		b.scroll = b.sel
	}
	if b.sel >= b.scroll+listH {
		b.scroll = b.sel - listH + 1
	}
	a.on(x, y+1, w, listH, func(ev tty.Event, _, _ int) {
		switch ev.Action {
		case tty.MouseWheelUp:
			b.move(-3)
		case tty.MouseWheelDown:
			b.move(3)
		}
	})
	if b.err != "" {
		s.Text(x+2, y+3, tty.Clean(b.err), cRed, cPanel, 0, w-4)
	}
	for i := b.scroll; i < len(b.items) && i-b.scroll < listH; i++ {
		it, i, ry := b.items[i], i, y+1+i-b.scroll
		bg := cPanel
		if i == b.sel {
			bg = cSel
		} else if a.hover(x+1, ry, w-2, 1) {
			bg = cHot
		}
		s.Fill(x+1, ry, w-2, 1, tty.Cell{Ch: ' ', Bg: bg})
		name, fg := tty.Clean(it.name), cFg
		if it.dir {
			name, fg = name+"/", cAccent
		}
		s.Text(x+2, ry, name, fg, bg, 0, w-4)
		a.on(x+1, ry, w-2, 1, func(ev tty.Event, _, _ int) {
			switch ev.Action {
			case tty.MousePress:
				if ev.Button != tty.ButtonLeft {
					return
				}
				again := b.sel == i
				b.sel = i
				if a.dbl || again {
					b.activate(a)
				}
			case tty.MouseWheelUp:
				b.move(-3)
			case tty.MouseWheelDown:
				b.move(3)
			}
		})
	}
	hint := "↑↓ select · Enter open · Backspace parent folder · Esc close"
	if !a.loaded {
		hint = "↑↓ select · Enter open · Backspace parent folder · Esc quit"
	}
	s.Text(x+2, y+h-1, hint, cDim, cPanel, 0, w-4)
}

func (b *Browser) key(a *App, ev tty.Event) {
	switch ev.Key {
	case tty.KeyEsc:
		a.browser = nil
		if !a.loaded {
			a.quit = true
		}
	case tty.KeyUp:
		b.move(-1)
	case tty.KeyDown:
		b.move(1)
	case tty.KeyPgUp:
		b.move(-10)
	case tty.KeyPgDn:
		b.move(10)
	case tty.KeyHome:
		b.sel = 0
	case tty.KeyEnd:
		b.sel = len(b.items) - 1
	case tty.KeyEnter, tty.KeyRight:
		b.activate(a)
	case tty.KeyBackspace, tty.KeyLeft:
		b.sel = 0
		b.activate(a)
	case tty.KeyRune:
		if ev.Rune == 'q' && !a.loaded {
			a.quit = true
			return
		}
		// Type a letter to jump to the next entry starting with it.
		r := unicode.ToLower(ev.Rune)
		for k := 1; k <= len(b.items); k++ {
			i := (b.sel + k) % len(b.items)
			if n := []rune(strings.ToLower(b.items[i].name)); len(n) > 0 && n[0] == r {
				b.sel = i
				break
			}
		}
	}
}
