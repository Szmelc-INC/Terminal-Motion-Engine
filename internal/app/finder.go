package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/fetch"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/media"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
)

type streamMeta struct{ name, audio string }

// download is one transfer, shown at the bottom of the finder.
type download struct {
	title  string
	path   string
	p      fetch.Progress
	done   bool
	err    error
	cancel context.CancelFunc
}

// Finder is the media search window: pick a site, type a query, browse the
// results with a thumbnail preview, play one straight from the web or
// download it in the format of your choice.
type Finder struct {
	prov             int
	query            []rune
	cur              int
	typing           bool
	kind, dur, order string

	items       []fetch.Item
	sel, scroll int
	busy        bool
	err         string
	exhausted   bool
	gen         int
	cancel      context.CancelFunc

	// Thumbnail of the selected result, rendered with the current look.
	thumbs    map[string]*media.Picture
	thumbBusy map[string]bool
	thumbAt   time.Time
	thumbKey  string
	rend      engine.Renderer
	cells     []engine.Cell

	showInfo   bool
	info       map[string]*fetch.Detail
	infoErr    map[string]string
	infoScroll int

	playAfter bool
	spin      int
}

func (a *App) keys() fetch.Keys {
	k := fetch.Keys{YouTube: a.prefs.YouTubeKey, Giphy: a.prefs.GiphyKey}
	if k.YouTube == "" {
		for _, env := range []string{"TERMO_YOUTUBE_KEY", "YOUTUBE_API_KEY"} {
			if v := os.Getenv(env); v != "" {
				k.YouTube = v
				break
			}
		}
	}
	if k.Giphy == "" {
		k.Giphy = os.Getenv("GIPHY_API_KEY")
	}
	return k
}

// openFinder shows the finder; a non-empty query is searched for at once.
func (a *App) openFinder(query string) {
	if a.finder == nil {
		a.finder = &Finder{kind: "any", dur: "any", order: "relevance", typing: true, playAfter: true,
			thumbs: map[string]*media.Picture{}, thumbBusy: map[string]bool{}, info: map[string]*fetch.Detail{},
			infoErr: map[string]string{}}
		a.finder.rend.TermDepth = a.cfg.Depth
	}
	a.menu = nil
	a.finderOpen = true
	if query != "" {
		a.finder.query, a.finder.cur = []rune(query), len([]rune(query))
		a.finder.search(a, false)
	}
}

func (f *Finder) provider() *fetch.Provider { return fetch.Providers[f.prov] }

func (f *Finder) current() (fetch.Item, bool) {
	if f.sel < 0 || f.sel >= len(f.items) {
		return fetch.Item{}, false
	}
	return f.items[f.sel], true
}

func itemKey(it fetch.Item) string { return it.Provider + "\x00" + it.ID + "\x00" + it.URL }

// search runs the query in the background; more appends the next batch.
func (f *Finder) search(a *App, more bool) {
	text := strings.TrimSpace(string(f.query))
	if text == "" {
		f.typing = true
		return
	}
	if miss := f.provider().Missing(); miss != "" {
		f.err, f.busy = fmt.Sprintf("%s search needs %s, which is not installed", f.provider().Name, miss), false
		return
	}
	if f.cancel != nil {
		f.cancel()
	}
	if !more {
		f.items, f.sel, f.scroll, f.exhausted = nil, 0, 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	f.cancel, f.busy, f.err, f.typing, f.showInfo = cancel, true, "", false, false
	f.gen++
	gen, p, keys := f.gen, f.provider(), a.keys()
	q := fetch.Query{Text: text, Kind: f.kind, Duration: f.dur, Sort: f.order, Offset: len(f.items)}
	go func() {
		items, err := p.Search(ctx, q, keys)
		cancel()
		a.async <- func() {
			if f.gen != gen {
				return // a newer search replaced this one
			}
			f.busy = false
			if err != nil {
				f.err = err.Error()
				return
			}
			seen := map[string]bool{}
			for _, it := range f.items {
				seen[itemKey(it)] = true
			}
			added := 0
			for _, it := range items {
				if !seen[itemKey(it)] {
					f.items = append(f.items, it)
					added++
				}
			}
			if added == 0 {
				f.exhausted = true
				if len(f.items) == 0 {
					f.err = "nothing found — try other words, another site (←→) or looser filters"
				} else {
					a.say("no more results", 1500*time.Millisecond)
				}
			}
			f.thumbAt = time.Now()
		}
	}()
}

// tick loads the thumbnail of the selected result once the selection has
// rested for a moment, so scrolling does not start a fetch per row.
func (f *Finder) tick(a *App) {
	if f.busy || len(f.thumbBusy) > 0 {
		a.dirty = true
	}
	it, ok := f.current()
	if !ok || !a.finderOpen || it.Thumb == "" || time.Since(f.thumbAt) < 180*time.Millisecond {
		return
	}
	key := it.Thumb
	if f.thumbs[key] != nil || f.thumbBusy[key] || len(f.thumbBusy) >= 2 {
		return
	}
	f.thumbBusy[key] = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		pic, err := media.Thumb(ctx, key, 320)
		cancel()
		a.async <- func() {
			delete(f.thumbBusy, key)
			if err != nil {
				pic = &media.Picture{} // remember the failure; do not retry
			}
			if len(f.thumbs) > 80 {
				f.thumbs = map[string]*media.Picture{}
			}
			f.thumbs[key] = pic
			f.thumbKey = ""
		}
	}()
}

func (f *Finder) wake() time.Duration {
	if f.busy || len(f.thumbBusy) > 0 {
		return 120 * time.Millisecond
	}
	if d := time.Until(f.thumbAt.Add(190 * time.Millisecond)); d > 0 {
		return d
	}
	return time.Hour
}

func (f *Finder) move(d int) {
	if len(f.items) == 0 {
		return
	}
	f.sel = max(0, min(len(f.items)-1, f.sel+d))
	f.thumbAt, f.infoScroll = time.Now(), 0
}

func (f *Finder) setProvider(a *App, i int) {
	n := len(fetch.Providers)
	f.prov = ((i % n) + n) % n
	f.items, f.sel, f.scroll, f.err, f.exhausted = nil, 0, 0, "", false
	if strings.TrimSpace(string(f.query)) != "" {
		if f.provider().Name == "URL" && !fetch.IsURL(strings.TrimSpace(string(f.query))) {
			f.query, f.cur, f.typing = nil, 0, true
			return
		}
		f.search(a, false)
	}
}

// --- actions -----------------------------------------------------------------

// playItem streams a result: it is resolved to its stream URLs in the
// background and then joins the playlist.
func (a *App) playItem(it fetch.Item, now bool) {
	a.say("opening "+it.Title+" …", 30*time.Second)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		st, err := fetch.Resolve(ctx, it, 480)
		cancel()
		a.async <- func() {
			if a.fail(err) {
				return
			}
			a.addStream(st.Video, st.Audio, it.Title, now)
		}
	}()
}

// addStream puts a web stream or a downloaded file on the playlist.
func (a *App) addStream(path, audio, name string, now bool) {
	if a.meta == nil {
		a.meta = map[string]streamMeta{}
	}
	if name != "" || audio != "" {
		a.meta[path] = streamMeta{name: name, audio: audio}
	}
	idx := -1
	for i, f := range a.files {
		if f == path {
			idx = i
		}
	}
	if idx < 0 {
		a.files = append(a.files, path)
		idx = len(a.files) - 1
	}
	if now || !a.loaded {
		a.toast = ""
		a.finderOpen = false
		a.open(idx, 0)
		return
	}
	a.say(fmt.Sprintf("added to the playlist as %d/%d — n plays the next file", idx+1, len(a.files)), 3*time.Second)
}

func (a *App) startDownload(it fetch.Item, f fetch.Format, playAfter bool) {
	ctx, cancel := context.WithCancel(context.Background())
	d := &download{title: it.Title, cancel: cancel, p: fetch.Progress{Stage: "starting", Percent: -1}}
	a.downloads = append(a.downloads, d)
	dir := a.prefs.Downloads()
	last := time.Now()
	go func() {
		path, err := fetch.Download(ctx, it, f, dir, func(p fetch.Progress) {
			if time.Since(last) < 120*time.Millisecond {
				return
			}
			last = time.Now()
			select {
			case a.async <- func() { d.p = p }:
			default: // the interface is busy; the next report will do
			}
		})
		cancel()
		a.async <- func() {
			d.done, d.err, d.path = true, err, path
			if err != nil {
				if err == context.Canceled {
					d.err = fmt.Errorf("cancelled")
					return
				}
				a.say("download failed: "+err.Error(), 6*time.Second)
				return
			}
			a.say("saved "+path, 5*time.Second)
			if f.Audio {
				return // termo plays pictures; the sound file is just saved
			}
			a.addStream(path, "", "", playAfter && !a.finderOpen)
		}
	}()
}

func (a *App) activeDownloads() int {
	n := 0
	for _, d := range a.downloads {
		if !d.done {
			n++
		}
	}
	return n
}

func (f *Finder) pickFormat(a *App, it fetch.Item) {
	var fields []field
	fields = append(fields, boolField("Play when done", "open the file as soon as it is saved (when the finder is closed)", &f.playAfter))
	for _, fm := range fetch.Formats {
		fm := fm
		help := "saved in " + a.prefs.Downloads()
		if fm.Ext == "" && !it.Direct {
			help = "the best single file the site offers · " + help
		}
		fields = append(fields, action(fm.Label, help, nil, func() {
			a.form = nil
			a.startDownload(it, fm, f.playAfter)
			a.say("downloading "+it.Title+" as "+fm.ID, 2*time.Second)
		}))
	}
	a.form = &Form{title: "Download — " + it.Title, sel: 1, fields: func() []field { return fields }}
}

func (f *Finder) loadInfo(a *App, it fetch.Item) {
	key := itemKey(it)
	if f.info[key] != nil || f.infoErr[key] == "…" {
		return
	}
	f.infoErr[key] = "…"
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		d, err := fetch.Info(ctx, it)
		cancel()
		a.async <- func() {
			delete(f.infoErr, key)
			if err != nil && len(d.Lines) == 0 {
				f.infoErr[key] = err.Error()
				return
			}
			f.info[key] = &d
		}
	}()
}

// --- drawing -----------------------------------------------------------------

func (f *Finder) draw(a *App) {
	if !a.finderOpen {
		return
	}
	s := a.scr
	w, h := a.dim(112, 34)
	top, bottom := a.desk()
	h = min(h, bottom-top)
	x, y := a.window("F9 · Find media", w, h)
	a.on(0, 0, s.W, s.H, func(tty.Event, int, int) {})
	s.Text(x+w-4, y, " ✕ ", cRed, cSel, engine.AttrBold, -1)
	a.on(x+w-4, y, 3, 1, func(ev tty.Event, _, _ int) {
		if clicked(ev) {
			a.finderOpen = false
		}
	})

	// Sites.
	tx := x + 1
	for i, p := range fetch.Providers {
		label := " " + p.Name + " "
		fg, bg, attr := cDim, cPanel, uint8(0)
		if i == f.prov {
			fg, bg, attr = cPanel, cAccent, engine.AttrBold
		} else if a.hover(tx, y+1, len(label), 1) {
			fg, bg = cFg, cHot
		}
		s.Text(tx, y+1, label, fg, bg, attr, -1)
		i := i
		a.on(tx, y+1, len(label), 1, func(ev tty.Event, _, _ int) {
			if clicked(ev) {
				f.setProvider(a, i)
			}
		})
		tx += len(label)
	}
	s.Text(tx+1, y+1, f.provider().Hint, cDim, cPanel, 0, x+w-tx-3)

	// Search box and filters.
	filters := fmt.Sprintf(" kind:%s  length:%s  sort:%s ", f.kind, f.dur, f.order)
	qw := w - 14 - len(filters)
	s.Text(x+2, y+3, "Search ", cFg, cPanel, engine.AttrBold, -1)
	qx := x + 9
	s.Fill(qx, y+3, qw, 1, tty.Cell{Ch: ' ', Bg: cBar})
	first := max(0, f.cur-qw+1)
	for i := 0; i < qw; i++ {
		c := tty.Cell{Ch: ' ', Fg: cFg, Bg: cBar}
		if first+i < len(f.query) {
			c.Ch = f.query[first+i]
		}
		if f.typing && first+i == f.cur {
			c.Attr = engine.AttrReverse
		}
		s.Set(qx+i, y+3, c)
	}
	if len(f.query) == 0 && !f.typing {
		s.Text(qx+1, y+3, "press / and type what you are looking for", cDim, cBar, 0, qw-2)
	}
	a.on(qx, y+3, qw, 1, func(ev tty.Event, rx, _ int) {
		if clicked(ev) {
			f.typing, f.cur = true, min(len(f.query), first+rx)
		}
	})
	s.Text(qx+qw+1, y+3, filters, cYellow, cPanel, 0, -1)
	a.on(qx+qw+1, y+3, len(filters), 1, func(ev tty.Event, rx, _ int) {
		if !clicked(ev) {
			return
		}
		switch {
		case rx < 3+len("kind:"+f.kind):
			f.filter(a, 'f')
		case rx < 5+len("kind:"+f.kind+"length:"+f.dur):
			f.filter(a, 't')
		default:
			f.filter(a, 'o')
		}
	})

	nd := min(3, len(a.downloads))
	bodyY, bodyH := y+5, h-8-nd
	if bodyH < 3 {
		return
	}
	listW := (w - 4) * 55 / 100
	px, pw := x+3+listW, w-5-listW
	f.drawList(a, x+1, bodyY, listW, bodyH)
	if pw >= 16 {
		f.drawPreview(a, px, bodyY, pw, bodyH)
	}

	// Transfers.
	for i := 0; i < nd; i++ {
		f.drawDownload(a, a.downloads[len(a.downloads)-nd+i], x+2, y+h-2-nd+i, w-4)
	}
	hint := "/ search · ←→ site · ↑↓ pick · Enter play · a queue · d download · i info · f t o filters · m more · l files · Esc"
	if f.typing {
		hint = "type your search · Enter search · ←→ move · Ctrl+U clear · Esc stop typing"
	}
	s.Text(x+2, y+h-1, hint, cDim, cPanel, 0, w-4)
}

func (f *Finder) drawList(a *App, x, y, w, h int) {
	s := a.scr
	spinner := []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
	f.spin++
	status := fmt.Sprintf("%d results", len(f.items))
	switch {
	case f.busy:
		status = string(spinner[f.spin/2%len(spinner)]) + " searching " + f.provider().Name + " …"
	case len(f.items) == 0:
		status = ""
	}
	s.Text(x+1, y, status, cDim, cPanel, 0, w-2)
	listY, listH := y+1, h-1
	if f.err != "" && !f.busy {
		for i, line := range wrap(f.err, w-4) {
			if i < listH {
				s.Text(x+1, listY+1+i, line, cRed, cPanel, 0, w-2)
			}
		}
		if len(f.items) == 0 {
			return
		}
	}
	if len(f.items) == 0 {
		if !f.busy {
			for i, l := range []string{
				"Search " + f.provider().Name + " for GIFs, clips, films and memes.",
				"",
				"Enter plays a result straight from the web;",
				"d downloads it as mp4, webm, gif, mp3, wav and more.",
				"",
				"←→ switches between " + providerNames() + ".",
			} {
				s.Text(x+1, listY+1+i, l, cDim, cPanel, 0, w-2)
			}
		}
		return
	}
	scrollInto(&f.sel, &f.scroll, len(f.items), listH)
	a.on(x, listY, w, listH, func(ev tty.Event, _, _ int) {
		switch ev.Action {
		case tty.MouseWheelUp:
			f.move(-2)
		case tty.MouseWheelDown:
			f.move(2)
		}
	})
	for i := f.scroll; i < len(f.items) && i-f.scroll < listH; i++ {
		it, i, ry := f.items[i], i, listY+i-f.scroll
		bg := cPanel
		if i == f.sel {
			bg = cSel
		} else if a.hover(x, ry, w, 1) {
			bg = cHot
		}
		s.Fill(x, ry, w, 1, tty.Cell{Ch: ' ', Bg: bg})
		meta := fetch.Clock(it.Duration)
		if v := fetch.Count(it.Views); v != "" {
			meta = strings.TrimSpace(meta + "  " + v)
		}
		if meta == "" {
			meta = it.Kind
		}
		mw := len([]rune(meta))
		s.Text(x+1, ry, tty.Clean(it.Title), cFg, bg, 0, w-mw-4)
		s.Text(x+w-mw-1, ry, meta, cDim, bg, 0, -1)
		a.on(x, ry, w, 1, func(ev tty.Event, _, _ int) {
			switch ev.Action {
			case tty.MousePress:
				if ev.Button == tty.ButtonLeft {
					again := f.sel == i
					f.sel, f.typing, f.thumbAt = i, false, time.Now()
					if a.dbl || again {
						a.playItem(it, true)
					}
				}
			case tty.MouseWheelUp:
				f.move(-2)
			case tty.MouseWheelDown:
				f.move(2)
			}
		})
	}
}

func providerNames() string {
	var n []string
	for _, p := range fetch.Providers {
		n = append(n, p.Name)
	}
	return strings.Join(n, ", ")
}

// wrap breaks text into lines of at most w cells, at spaces where it can.
func wrap(text string, w int) []string {
	if w < 4 {
		return []string{text}
	}
	var out []string
	for _, para := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			for len([]rune(word)) > w {
				// A word longer than the line (a URL): cut it into pieces.
				if line != "" {
					out, line = append(out, line), ""
				}
				r := []rune(word)
				out, word = append(out, string(r[:w])), string(r[w:])
			}
			if line != "" && len([]rune(line))+1+len([]rune(word)) > w {
				out, line = append(out, line), ""
			}
			if line != "" {
				line += " "
			}
			line += word
		}
		out = append(out, line)
	}
	return out
}

func (f *Finder) drawPreview(a *App, x, y, w, h int) {
	s := a.scr
	it, ok := f.current()
	if !ok {
		return
	}
	if f.showInfo {
		f.drawInfo(a, it, x, y, w, h)
		return
	}
	// Picture on top, facts below.
	facts := [][2]string{{"", it.Title}}
	add := func(k, v string) {
		if v != "" {
			facts = append(facts, [2]string{k, v})
		}
	}
	add("by", it.Author)
	add("length", fetch.Clock(it.Duration))
	add("views", fetch.Count(it.Views))
	add("date", it.Date)
	if it.Width > 0 {
		add("size", fmt.Sprintf("%d×%d", it.Width, it.Height))
	}
	if it.Size > 0 {
		add("file", fmt.Sprintf("%.1f MiB", float64(it.Size)/(1<<20)))
	}
	add("kind", strings.TrimSpace(it.Kind+" "+strings.TrimPrefix(it.Ext, ".")))
	picH := max(3, h-len(facts)-2)
	pic := f.thumbs[it.Thumb]
	switch {
	case it.Thumb == "":
		s.Text(x, y+picH/2, "no preview picture for this result", cDim, cPanel, 0, w)
	case pic == nil:
		s.Text(x, y+picH/2, "loading preview …", cDim, cPanel, 0, w)
	case pic.W == 0:
		s.Text(x, y+picH/2, "the preview picture could not be loaded", cDim, cPanel, 0, w)
	default:
		g := FitZoom(float64(pic.W)/float64(pic.H), a.s.Look, w, picH, a.aspect(), 1)
		key := fmt.Sprintf("%s/%d/%d/%v", it.Thumb, g.W, g.H, a.s.Look) // redrawn when any setting changes
		if f.thumbKey != key || len(f.cells) != g.Cols*g.Rows {
			f.cells = make([]engine.Cell, g.Cols*g.Rows)
			look := a.s.Look
			f.rend.Render(pic.Resize(g.W, g.H), g.W, g.H, &look, f.cells, g.Cols, g.Rows, 0)
			f.thumbKey = key
		}
		s.Blit(x+(w-g.Cols)/2, y+(picH-g.Rows)/2, g.Cols, g.Rows, f.cells)
	}
	fy := y + picH + 1
	for i, l := range facts {
		if fy+i >= y+h {
			break
		}
		if l[0] == "" {
			s.Text(x, fy+i, tty.Clean(l[1]), cFg, cPanel, engine.AttrBold, w)
			continue
		}
		s.Text(x, fy+i, l[0], cDim, cPanel, 0, 7)
		s.Text(x+8, fy+i, tty.Clean(l[1]), cFg, cPanel, 0, w-8)
	}
}

func (f *Finder) drawInfo(a *App, it fetch.Item, x, y, w, h int) {
	s := a.scr
	key := itemKey(it)
	d := f.info[key]
	if d == nil {
		msg := "reading the details …"
		if e := f.infoErr[key]; e != "" && e != "…" {
			msg = e
		}
		for i, l := range wrap(msg, w) {
			s.Text(x, y+1+i, l, cDim, cPanel, 0, w)
		}
		return
	}
	type line struct {
		text string
		fg   uint32
	}
	var lines []line
	for _, l := range d.Lines {
		for i, part := range wrap(l[1], w-14) {
			label := ""
			if i == 0 {
				label = l[0]
			}
			lines = append(lines, line{fmt.Sprintf("%-13s %s", label, part), cFg})
		}
	}
	if len(d.Formats) > 0 {
		lines = append(lines, line{}, line{fmt.Sprintf("Available streams (%d)", len(d.Formats)), cAccent})
		for _, fl := range d.Formats {
			lines = append(lines, line{fl, cDim})
		}
	}
	if d.Desc != "" {
		lines = append(lines, line{}, line{"Description", cAccent})
		for _, l := range wrap(d.Desc, w) {
			lines = append(lines, line{l, cFg})
		}
	}
	f.infoScroll = max(0, min(f.infoScroll, len(lines)-h))
	for i := 0; i < h && f.infoScroll+i < len(lines); i++ {
		l := lines[f.infoScroll+i]
		s.Text(x, y+i, tty.Clean(l.text), l.fg, cPanel, 0, w)
	}
	if f.infoScroll+h < len(lines) {
		s.Text(x+w-18, y+h-1, " PgDn for more ↓ ", cYellow, cPanel, 0, -1)
	}
	a.on(x, y, w, h, func(ev tty.Event, _, _ int) {
		switch ev.Action {
		case tty.MouseWheelUp:
			f.infoScroll -= 3
		case tty.MouseWheelDown:
			f.infoScroll += 3
		}
	})
}

func (f *Finder) drawDownload(a *App, d *download, x, y, w int) {
	s := a.scr
	name := d.title
	switch {
	case d.done && d.err != nil:
		s.Text(x, y, "✗ "+tty.Clean(name)+" — "+d.err.Error(), cRed, cPanel, 0, w)
	case d.done:
		s.Text(x, y, "✓ saved "+tty.Clean(filepath.Base(d.path)), cGreen, cPanel, 0, w)
	default:
		tw := max(10, w/3)
		s.Text(x, y, "↓ "+tty.Clean(name), cFg, cPanel, 0, tw)
		bx, bw := x+tw+1, max(8, w/4)
		frac := max(0, d.p.Percent) / 100
		if d.p.Percent < 0 {
			frac = float64(f.spin%40) / 40
		}
		for i := 0; i < bw; i++ {
			c := tty.Cell{Ch: '━', Fg: cGreen, Bg: cPanel}
			if float64(i) >= frac*float64(bw) {
				c.Ch, c.Fg = '─', cTrack
			}
			s.Set(bx+i, y, c)
		}
		st := d.p.Stage
		if d.p.Percent >= 0 {
			st += fmt.Sprintf(" %3.0f%%", d.p.Percent)
		}
		if d.p.Speed != "" {
			st += "  " + d.p.Speed
		}
		if d.p.ETA != "" && d.p.ETA != "Unknown" && d.p.ETA != "NA" {
			st += "  eta " + d.p.ETA
		}
		s.Text(bx+bw+1, y, st+"  (x cancels)", cDim, cPanel, 0, x+w-bx-bw-1)
	}
}

// --- input -------------------------------------------------------------------

func (f *Finder) filter(a *App, which rune) {
	switch which {
	case 'f':
		f.kind = cycle(fetch.Kinds, f.kind, 1)
	case 't':
		f.dur = cycle(fetch.Durations, f.dur, 1)
	case 'o':
		f.order = cycle(fetch.Sorts, f.order, 1)
	}
	if strings.TrimSpace(string(f.query)) != "" {
		f.search(a, false)
	}
}

func (f *Finder) key(a *App, ev tty.Event) {
	if !a.finderOpen {
		a.playerKey(ev)
		return
	}
	if f.typing {
		f.typeKey(a, ev)
		return
	}
	it, ok := f.current()
	switch ev.Key {
	case tty.KeyEsc:
		if f.showInfo {
			f.showInfo = false
			return
		}
		a.finderOpen = false
	case tty.KeyUp:
		f.move(-1)
	case tty.KeyDown:
		f.move(1)
	case tty.KeyPgUp:
		if f.showInfo {
			f.infoScroll -= 8
		} else {
			f.move(-10)
		}
	case tty.KeyPgDn:
		if f.showInfo {
			f.infoScroll += 8
		} else {
			f.move(10)
		}
	case tty.KeyHome:
		f.move(-len(f.items))
	case tty.KeyEnd:
		f.move(len(f.items))
	case tty.KeyLeft, tty.KeyBackTab:
		f.setProvider(a, f.prov-1)
	case tty.KeyRight, tty.KeyTab:
		f.setProvider(a, f.prov+1)
	case tty.KeyEnter:
		if ok {
			a.playItem(it, true)
		}
	case tty.KeyRune:
		if ev.Ctrl || ev.Alt || ev.Rune == '<' || ev.Rune == '>' {
			a.playerKey(ev) // picture and interface size, and the other windows
			return
		}
		switch ev.Rune {
		case '/', 's':
			f.typing, f.cur = true, len(f.query)
		case 'q':
			a.finderOpen = false
		case 'a':
			if ok {
				a.playItem(it, false)
			}
		case 'd':
			if ok {
				f.pickFormat(a, it)
			}
		case 'i':
			if ok {
				if f.showInfo = !f.showInfo; f.showInfo {
					f.infoScroll = 0
					f.loadInfo(a, it)
				}
			}
		case 'f', 't', 'o':
			f.filter(a, ev.Rune)
		case 'm':
			if f.exhausted {
				a.say("no more results", 1500*time.Millisecond)
			} else if !f.busy && len(f.items) > 0 {
				f.search(a, true)
			}
		case 'l':
			a.openBrowserAt(a.prefs.Downloads())
		case 'x':
			n := 0
			for _, d := range a.downloads {
				if !d.done {
					d.cancel()
					n++
				}
			}
			if n == 0 {
				a.say("no download is running", 1500*time.Millisecond)
			}
		case 'j':
			f.move(1)
		case 'k':
			f.move(-1)
		}
	}
	if f.showInfo {
		if cur, ok := f.current(); ok {
			f.loadInfo(a, cur)
		}
	}
}

func (f *Finder) typeKey(a *App, ev tty.Event) {
	switch ev.Key {
	case tty.KeyEsc:
		if len(f.items) == 0 && len(f.query) == 0 {
			a.finderOpen = false
			return
		}
		f.typing = false
	case tty.KeyEnter:
		f.search(a, false)
	case tty.KeyDown, tty.KeyTab:
		if len(f.items) > 0 {
			f.typing = false
		} else if ev.Key == tty.KeyTab {
			f.setProvider(a, f.prov+1)
			f.typing = true
		}
	case tty.KeyBackTab:
		f.setProvider(a, f.prov-1)
		f.typing = true
	case tty.KeyLeft:
		f.cur = max(0, f.cur-1)
	case tty.KeyRight:
		f.cur = min(len(f.query), f.cur+1)
	case tty.KeyHome:
		f.cur = 0
	case tty.KeyEnd:
		f.cur = len(f.query)
	case tty.KeyBackspace:
		if f.cur > 0 {
			f.query = append(f.query[:f.cur-1], f.query[f.cur:]...)
			f.cur--
		}
	case tty.KeyDelete:
		if f.cur < len(f.query) {
			f.query = append(f.query[:f.cur], f.query[f.cur+1:]...)
		}
	case tty.KeyRune:
		if ev.Ctrl && ev.Rune == 'u' {
			f.query, f.cur = nil, 0
			return
		}
		if ev.Ctrl || ev.Alt || !unicode.IsPrint(ev.Rune) || len(f.query) >= 400 {
			return
		}
		f.query = append(f.query[:f.cur], append([]rune{ev.Rune}, f.query[f.cur:]...)...)
		f.cur++
	}
}
