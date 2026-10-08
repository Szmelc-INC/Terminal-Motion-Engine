package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
)

func (a *App) exportDir() string { return filepath.Join(filepath.Dir(a.lib.Path), "exports") }

func (a *App) writeExport(name string, data []byte) (string, error) {
	dir := a.exportDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r < 0x20 {
			return '_'
		}
		return r
	}, name))
	return path, os.WriteFile(path, data, 0o644)
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// --- presets -----------------------------------------------------------------

func newPresetManager() *managerPage {
	p := &managerPage{name: "Looks"}
	p.items = func(a *App) []mItem {
		var out []mItem
		for _, pr := range a.store.All() {
			out = append(out, mItem{name: pr.Name, user: !pr.Builtin, info: Summary(pr.Look), active: pr.Name == a.preset})
		}
		return out
	}
	find := func(a *App, it mItem) (engine.Preset, bool) { return a.store.Find(it.name) }
	p.actions = []mAction{
		{'l', "Load", &cGreen, func(a *App, it mItem, ok bool) {
			if pr, found := find(a, it); ok && found {
				a.loadPreset(pr)
			}
		}},
		{'n', "New", &cAccent, func(a *App, it mItem, ok bool) {
			a.ask("New preset from the current look", "name", "", func(v string) { a.savePreset(v, true) })
		}},
		{'s', "Update", nil, func(a *App, it mItem, ok bool) {
			if a.userOnly(it, ok, "save") {
				a.confirm = &Confirm{msg: fmt.Sprintf("Overwrite %q with the current look?", it.name),
					yes: func() { a.savePreset(it.name, false) }}
			}
		}},
		{'e', "Rename", nil, func(a *App, it mItem, ok bool) {
			if !a.userOnly(it, ok, "save") {
				return
			}
			a.ask("Rename preset", "new name", it.name, func(v string) {
				if a.fail(a.store.Rename(it.name, v)) {
					return
				}
				if a.preset == it.name {
					a.preset = v
				}
				a.ok("renamed to " + v)
			})
		}},
		{'c', "Copy", nil, func(a *App, it mItem, ok bool) {
			pr, found := find(a, it)
			if !ok || !found {
				return
			}
			a.ask("Copy preset "+it.name, "name of the copy", it.name+"-copy", func(v string) {
				if _, exists := a.store.Find(v); exists {
					a.say("a preset named "+v+" already exists", 3*time.Second)
					return
				}
				if !a.fail(a.store.Save(v, pr.Look)) {
					p.selectName(a, v)
					a.ok("copied to " + v)
				}
			})
		}},
		{'d', "Delete", &cRed, func(a *App, it mItem, ok bool) {
			if !a.userOnly(it, ok, "make") {
				return
			}
			a.confirm = &Confirm{msg: fmt.Sprintf("Delete preset %q?", it.name), yes: func() {
				if a.fail(a.store.Delete(it.name)) {
					return
				}
				if a.preset == it.name {
					a.preset = ""
				}
				a.ok("deleted " + it.name)
			}}
		}},
		{'f', "Default", nil, func(a *App, it mItem, ok bool) {
			pr, found := find(a, it)
			if !ok || !found {
				return
			}
			a.confirm = &Confirm{msg: fmt.Sprintf("Start termo with %q from now on?", it.name), yes: func() {
				if !a.fail(a.store.Save("default", pr.Look)) {
					a.ok("termo now starts with " + it.name + " (saved as preset \"default\")")
				}
			}}
		}},
		{'x', "Export", nil, func(a *App, it mItem, ok bool) {
			pr, found := find(a, it)
			if !ok || !found {
				return
			}
			data, _ := json.MarshalIndent(pr, "", "  ")
			path, err := a.writeExport("preset-"+it.name+".json", append(data, '\n'))
			if !a.fail(err) {
				a.say("exported to "+path, 4*time.Second)
			}
		}},
		{'i', "Import", nil, func(a *App, it mItem, ok bool) {
			a.ask("Import a preset", "path to a .json file exported by termo", "", func(v string) {
				data, err := os.ReadFile(expandHome(v))
				if a.fail(err) {
					return
				}
				var raw struct {
					Name string          `json:"name"`
					Look json.RawMessage `json:"look"`
				}
				l := engine.DefaultLook()
				if err := json.Unmarshal(data, &raw); err != nil || raw.Look == nil || json.Unmarshal(raw.Look, &l) != nil {
					a.say("that file is not a termo preset", 3*time.Second)
					return
				}
				name := raw.Name
				if name == "" {
					name = strings.TrimSuffix(filepath.Base(v), filepath.Ext(v))
				}
				base := name
				for k := 2; ; k++ {
					if _, exists := a.store.Find(name); !exists {
						break
					}
					name = fmt.Sprintf("%s-%d", base, k)
				}
				if !a.fail(a.store.Save(name, l)) {
					p.selectName(a, name)
					a.ok("imported as " + name)
				}
			})
		}},
	}
	return p
}

// --- color palettes ----------------------------------------------------------

// currentColors returns the colors the picture is drawn with right now.
func (a *App) currentColors() []engine.RGB {
	if p := a.rend.Palette(); p != nil {
		return append([]engine.RGB(nil), p.Colors...)
	}
	return nil
}

func (a *App) setPalette(name string) {
	a.touch()
	a.s.Color, a.s.Palette = true, name
	a.changed()
}

func (a *App) savePaletteAs(colors []engine.RGB, suggest string, then func(name string)) {
	if len(colors) == 0 {
		a.say("there is no palette to save — the picture is in truecolor", 3*time.Second)
		return
	}
	a.ask("Save palette as", "name", suggest, func(v string) {
		save := func() {
			if a.fail(a.lib.SavePalette(v, colors)) {
				return
			}
			pgPalettes.selectName(a, v)
			a.ok(fmt.Sprintf("saved palette %s (%d colors)", v, len(colors)))
			if then != nil {
				then(v)
			}
		}
		if _, exists := engine.FindPalette(v); exists && !engine.IsBuiltinPalette(v) {
			a.confirm = &Confirm{msg: fmt.Sprintf("Overwrite palette %q?", v), yes: save}
			return
		}
		save()
	})
}

// FetchText reads a palette source: a file, a URL, or lospec:NAME.
func FetchText(src string) (string, error) {
	if s, ok := strings.CutPrefix(src, "lospec:"); ok {
		slug := strings.ToLower(strings.Join(strings.Fields(s), "-"))
		src = "https://lospec.com/palette-list/" + slug + ".hex"
	}
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		c := http.Client{Timeout: 15 * time.Second}
		resp, err := c.Get(src)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return "", fmt.Errorf("%s: %s", src, resp.Status)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return string(data), err
	}
	data, err := os.ReadFile(expandHome(src))
	return string(data), err
}

func gpl(name string, colors []engine.RGB) string {
	var b strings.Builder
	fmt.Fprintf(&b, "GIMP Palette\nName: %s\nColumns: 8\n#\n", name)
	for _, c := range colors {
		fmt.Fprintf(&b, "%3d %3d %3d\t%s\n", c.R, c.G, c.B, c.Hex())
	}
	return b.String()
}

func newPaletteManager() *managerPage {
	p := &managerPage{name: "Palettes"}
	p.items = func(a *App) []mItem {
		var out []mItem
		for _, np := range engine.AllPalettes() {
			user := !engine.IsBuiltinPalette(np.Name)
			it := mItem{name: np.Name, user: user, swatch: np.Colors, active: a.s.Color && a.s.Palette == np.Name,
				info: fmt.Sprintf("%d colors", len(np.Colors))}
			if !user {
				it.tag = engine.PaletteTag(np.Name)
			}
			out = append(out, it)
		}
		return out
	}
	p.actions = []mAction{
		{'l', "Use", &cGreen, func(a *App, it mItem, ok bool) {
			if ok {
				a.setPalette(it.name)
				a.ok(fmt.Sprintf("palette: %s (%d colors)", it.name, len(it.swatch)))
			}
		}},
		{'g', "Generate", &cAccent, func(a *App, it mItem, ok bool) { a.openPaletteGen() }},
		{'s', "Save current", nil, func(a *App, it mItem, ok bool) {
			a.savePaletteAs(a.currentColors(), "", func(n string) { a.setPalette(n) })
		}},
		{'e', "Edit", nil, func(a *App, it mItem, ok bool) {
			if !ok {
				return
			}
			a.touch()
			a.s.Custom = nil
			for _, c := range it.swatch {
				a.s.Custom = append(a.s.Custom, c.Hex())
			}
			a.s.Color, a.s.Palette = true, engine.PalCustom
			pgEditor.source = ""
			if it.user {
				pgEditor.source = it.name
			}
			a.changed()
			a.showPage(pgEditor)
			a.say("editing a copy as the custom palette — press s there to save it", 3*time.Second)
		}},
		{'c', "Copy", nil, func(a *App, it mItem, ok bool) {
			if ok {
				a.savePaletteAs(it.swatch, it.name+"-copy", nil)
			}
		}},
		{'r', "Rename", nil, func(a *App, it mItem, ok bool) {
			if !a.userOnly(it, ok, "make") {
				return
			}
			a.ask("Rename palette", "new name", it.name, func(v string) {
				if a.fail(a.lib.Rename("palette", it.name, v)) {
					return
				}
				if a.s.Palette == it.name {
					a.s.Palette = v
					a.changed()
				}
				a.ok("renamed to " + v)
			})
		}},
		{'d', "Delete", &cRed, func(a *App, it mItem, ok bool) {
			if !a.userOnly(it, ok, "make") {
				return
			}
			a.confirm = &Confirm{msg: fmt.Sprintf("Delete palette %q?", it.name), yes: func() {
				if a.fail(a.lib.Delete("palette", it.name)) {
					return
				}
				if a.s.Palette == it.name {
					a.s.Palette = engine.PalOff
				}
				a.changed()
				a.ok("deleted " + it.name)
			}}
		}},
		{'i', "Import", nil, func(a *App, it mItem, ok bool) {
			a.ask("Import a palette", "file (.hex .gpl .json .txt), URL, or lospec:NAME", "lospec:", func(v string) {
				if v == "" || v == "lospec:" {
					return
				}
				a.say("importing "+v+" …", 20*time.Second)
				go func() {
					text, err := FetchText(v)
					var cols []engine.RGB
					if err == nil {
						cols, err = engine.ParsePaletteText(text)
					}
					a.async <- func() {
						if a.fail(err) {
							return
						}
						a.toast = ""
						name := strings.TrimPrefix(v, "lospec:")
						name = strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
						a.savePaletteAs(cols, name, func(n string) { a.setPalette(n) })
					}
				}()
			})
		}},
		{'x', "Export", nil, func(a *App, it mItem, ok bool) {
			if !ok {
				return
			}
			var hex strings.Builder
			for _, c := range it.swatch {
				hex.WriteString(strings.TrimPrefix(c.Hex(), "#") + "\n")
			}
			path, err := a.writeExport(it.name+".hex", []byte(hex.String()))
			if err == nil {
				_, err = a.writeExport(it.name+".gpl", []byte(gpl(it.name, it.swatch)))
			}
			if !a.fail(err) {
				a.say("exported "+path+" and .gpl", 4*time.Second)
			}
		}},
	}
	return p
}

// palGen is the state of the palette generator form.
type palGen struct {
	kind                    string
	n                       int
	scheme                  string
	hue, chroma, lmin, lmax float64
	from, via, to           string
	sorted                  bool
	seed                    int64
	frame                   []engine.RGB
	stops                   []engine.RGB
}

var palGenKinds = []string{"harmony", "gradient", "from picture", "random"}

func (g *palGen) colors(a *App) []engine.RGB {
	var out []engine.RGB
	switch g.kind {
	case "gradient":
		var stops []engine.RGB
		for _, h := range []string{g.from, g.via, g.to} {
			if c, err := engine.ParseHex(h); err == nil && strings.TrimSpace(h) != "" {
				stops = append(stops, c)
			}
		}
		out = engine.Gradient(stops, g.n)
	case "from picture":
		if a.last != nil {
			out = engine.MedianCut(a.last.Pix, g.n)
		}
	case "random":
		out = engine.RandomColors(g.n, newRand(g.seed))
	default:
		out = engine.Harmony(g.scheme, g.hue, g.chroma, g.lmin, g.lmax, g.n)
	}
	if g.sorted {
		out = engine.SortByLuma(out)
	}
	return out
}

func cycle(list []string, cur string, d int) string {
	idx := 0
	for i, v := range list {
		if v == cur {
			idx = i
		}
	}
	n := len(list)
	return list[((idx+d)%n+n)%n]
}

// enumField, numField, boolField and textField build form fields over plain
// variables.
func enumField(label, help string, v *string, choices []string) field {
	return field{label: label, help: help, kind: fEnum, str: func() string { return *v },
		nudge: func(d int) { *v = cycle(choices, *v, d) }}
}

func numField(label, help string, v *float64, lo, hi, step float64, wrap bool) field {
	clamp := func(x float64) float64 {
		if wrap {
			x = math.Mod(x-lo, hi-lo)
			if x < 0 {
				x += hi - lo
			}
			return x + lo
		}
		return math.Max(lo, math.Min(hi, x))
	}
	return field{label: label, help: help, kind: fNum,
		str:     func() string { return trim(*v) },
		nudge:   func(d int) { *v = clamp(math.Round((*v+float64(d)*step)/step) * step) },
		frac:    func() float64 { return (*v - lo) / (hi - lo) },
		setFrac: func(f float64) { *v = math.Max(lo, math.Min(hi, math.Round((lo+f*(hi-lo))/step)*step)) },
		set: func(s string) error {
			var x float64
			if _, err := fmt.Sscanf(strings.TrimSpace(s), "%g", &x); err != nil || x < lo || x > hi {
				return fmt.Errorf("%s: enter a number from %s to %s", label, trim(lo), trim(hi))
			}
			*v = x
			return nil
		}}
}

func intField(label, help string, v *int, lo, hi int) field {
	f := float64(*v)
	nf := numField(label, help, &f, float64(lo), float64(hi), 1, false)
	wrap := func(fn func()) { f = float64(*v); fn(); *v = int(f) }
	out := nf
	out.str = func() string { return fmt.Sprint(*v) }
	out.nudge = func(d int) { wrap(func() { nf.nudge(d) }) }
	out.frac = func() float64 { return float64(*v-lo) / float64(max(1, hi-lo)) }
	out.setFrac = func(x float64) { wrap(func() { nf.setFrac(x) }) }
	out.set = func(s string) error {
		var err error
		wrap(func() { err = nf.set(s) })
		return err
	}
	return out
}

func boolField(label, help string, v *bool) field {
	return field{label: label, help: help, kind: fBool, nudge: func(int) { *v = !*v },
		str: func() string {
			if *v {
				return "on"
			}
			return "off"
		}}
}

func hexField(label, help string, v *string, optional bool) field {
	return field{label: label, help: help, kind: fText, str: func() string { return *v },
		set: func(s string) error {
			s = strings.TrimSpace(s)
			if s == "" && optional {
				*v = ""
				return nil
			}
			c, err := engine.ParseHex(s)
			if err != nil {
				return err
			}
			*v = c.Hex()
			return nil
		}}
}

func trim(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", f), "0"), ".")
}

func (a *App) openPaletteGen() {
	g := &palGen{kind: "harmony", n: 8, scheme: a.s.Scheme, hue: a.s.Hue, chroma: a.s.Chroma, lmin: a.s.LMin,
		lmax: a.s.LMax, from: "#0b1026", via: "", to: "#ffd9a0", seed: time.Now().UnixNano()}
	if g.chroma == 0 {
		g.chroma = 0.14
	}
	a.form = &Form{title: "Palette generator", previewH: 2,
		preview: func(a *App, x, y, w int) {
			cols := g.colors(a)
			a.swatchRow(x, y, w, cols)
			a.swatchRow(x, y+1, w, cols)
			if len(cols) == 0 {
				a.scr.Text(x, y, "nothing to show — open a picture first, or fix the colors below", cDim, cPanel, 0, w)
			}
		},
		fields: func() []field {
			fs := []field{
				enumField("Source", "where the colors come from", &g.kind, palGenKinds),
				intField("Colors", "how many colors to make", &g.n, 1, 64),
			}
			switch g.kind {
			case "harmony":
				fs = append(fs,
					enumField("Scheme", "color-theory relation between the hues", &g.scheme, engine.Schemes),
					numField("Base hue", "degrees around the color wheel", &g.hue, 0, 360, 5, true),
					numField("Chroma", "colorfulness", &g.chroma, 0, 0.37, 0.01, false),
					numField("Darkest", "lightness of the darkest color", &g.lmin, 0, 1, 0.01, false),
					numField("Lightest", "lightness of the lightest color", &g.lmax, 0, 1, 0.01, false))
			case "gradient":
				fs = append(fs,
					hexField("From", "first color, hex", &g.from, false),
					hexField("Through", "optional middle color, hex (empty = none)", &g.via, true),
					hexField("To", "last color, hex", &g.to, false))
			case "from picture":
				fs = append(fs, field{label: "Picture", kind: fText, help: "the most representative colors of the frame on screen",
					str: func() string { return a.title() }})
			case "random":
				fs = append(fs, action("⚄ Roll again", "a new random scheme or gradient", &cYellow,
					func() { g.seed = time.Now().UnixNano() }))
			}
			return append(fs,
				boolField("Sort", "order the colors from dark to light", &g.sorted),
				action("Try it on the picture", "use these colors now, without saving", nil, func() {
					cols := g.colors(a)
					if len(cols) == 0 {
						return
					}
					a.touch()
					a.s.Custom = nil
					for _, c := range cols {
						a.s.Custom = append(a.s.Custom, c.Hex())
					}
					a.s.Color, a.s.Palette = true, engine.PalCustom
					a.changed()
				}),
				action("Save as…", "store these colors as a named palette and use it", &cGreen, func() {
					a.savePaletteAs(g.colors(a), "", func(n string) { a.setPalette(n); a.form = nil })
				}))
		}}
}

// --- glyph ramps -------------------------------------------------------------

func (a *App) setCharset(name string) {
	a.touch()
	a.s.Mode, a.s.Charset = engine.ModeASCII, name
	a.changed()
}

func (a *App) saveCharsetAs(chars, suggest string, then func(name string)) {
	if len([]rune(chars)) < 2 {
		a.say("a charset needs at least two characters", 3*time.Second)
		return
	}
	a.ask("Save charset as", "name", suggest, func(v string) {
		for _, c := range engine.Charsets {
			if strings.EqualFold(c.Name, v) {
				a.say(v+" is a built-in charset, pick another name", 3*time.Second)
				return
			}
		}
		if a.fail(a.lib.SaveCharset(v, chars)) {
			return
		}
		pgCharsets.selectName(a, v)
		a.ok("saved charset " + v)
		if then != nil {
			then(v)
		}
	})
}

func isUserCharset(a *App, name string) bool {
	for _, c := range a.lib.Charsets {
		if strings.EqualFold(c.Name, name) {
			return true
		}
	}
	return false
}

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

func newCharsetManager() *managerPage {
	p := &managerPage{name: "Symbols"}
	p.items = func(a *App) []mItem {
		var out []mItem
		for _, c := range engine.AllCharsets() {
			if c.Name == "custom" {
				continue
			}
			out = append(out, mItem{name: c.Name, user: isUserCharset(a, c.Name), info: "▕" + string(c.Runes) + "▏",
				active: a.s.Mode == engine.ModeASCII && a.s.Charset == c.Name})
		}
		return out
	}
	chars := func(it mItem) string { return strings.TrimSuffix(strings.TrimPrefix(it.info, "▕"), "▏") }
	rewrite := func(a *App, it mItem, ok bool, what string, fn func(string) string) {
		if a.userOnly(it, ok, "make") && !a.fail(a.lib.SaveCharset(it.name, fn(chars(it)))) {
			a.changed()
			a.ok(it.name + ": " + what)
		}
	}
	p.actions = []mAction{
		{'l', "Use", &cGreen, func(a *App, it mItem, ok bool) {
			if ok {
				a.setCharset(it.name)
				a.ok("ascii mode with charset " + it.name)
			}
		}},
		{'n', "New", &cAccent, func(a *App, it mItem, ok bool) {
			a.ask("New charset", "type the characters, emptiest first (o sorts them later)", " ", func(v string) {
				a.saveCharsetAs(v, "", func(n string) { a.setCharset(n) })
			})
		}},
		{'g', "Generate", &cAccent, func(a *App, it mItem, ok bool) { a.openCharsetGen() }},
		{'e', "Edit", nil, func(a *App, it mItem, ok bool) {
			if !a.userOnly(it, ok, "edit") {
				return
			}
			a.prompt = &Prompt{title: "Edit charset " + it.name, hint: "emptiest character first", buf: []rune(chars(it)),
				cur: len([]rune(chars(it))), done: func(v string) {
					if !a.fail(a.lib.SaveCharset(it.name, v)) {
						a.changed()
					}
				}}
		}},
		{'o', "Sort", nil, func(a *App, it mItem, ok bool) {
			rewrite(a, it, ok, "sorted from empty to full", engine.SortByDensity)
		}},
		{'v', "Reverse", nil, func(a *App, it mItem, ok bool) { rewrite(a, it, ok, "reversed", reverse) }},
		{'c', "Copy", nil, func(a *App, it mItem, ok bool) {
			if ok {
				a.saveCharsetAs(chars(it), it.name+"-copy", nil)
			}
		}},
		{'r', "Rename", nil, func(a *App, it mItem, ok bool) {
			if !a.userOnly(it, ok, "make") {
				return
			}
			a.ask("Rename charset", "new name", it.name, func(v string) {
				if a.fail(a.lib.Rename("charset", it.name, v)) {
					return
				}
				if a.s.Charset == it.name {
					a.s.Charset = v
					a.changed()
				}
				a.ok("renamed to " + v)
			})
		}},
		{'d', "Delete", &cRed, func(a *App, it mItem, ok bool) {
			if !a.userOnly(it, ok, "make") {
				return
			}
			a.confirm = &Confirm{msg: fmt.Sprintf("Delete charset %q?", it.name), yes: func() {
				if a.fail(a.lib.Delete("charset", it.name)) {
					return
				}
				if a.s.Charset == it.name {
					a.s.Charset = engine.Charsets[0].Name
				}
				a.changed()
				a.ok("deleted " + it.name)
			}}
		}},
	}
	return p
}

func (a *App) openCharsetGen() {
	pool, extra := "ascii", ""
	n, shuffle, seed := 12, false, time.Now().UnixNano()
	gen := func() string {
		out := engine.GenCharset(pool, n, shuffle, newRand(seed))
		if extra != "" {
			out = engine.SortByDensity(out + extra)
		}
		return out
	}
	a.form = &Form{title: "Charset generator", previewH: 2,
		preview: func(a *App, x, y, w int) {
			ramp := gen()
			a.scr.Text(x, y, "▕"+ramp+"▏", cFg, cBar, 0, w)
			a.scr.Text(x, y+1, fmt.Sprintf("%d glyphs · brightness range %.0f %%", len([]rune(ramp)),
				engine.RampContrast(ramp)*100), cDim, cPanel, 0, w)
		},
		fields: func() []field {
			return []field{
				enumField("Glyphs from", "the pool of characters to pick from", &pool, engine.GlyphSetNames()),
				intField("How many", "length of the ramp; more glyphs = finer shading", &n, 2, 64),
				boolField("Shuffle", "pick random glyphs of each brightness instead of fixed ones", &shuffle),
				action("⚄ Roll again", "a new random pick (with Shuffle on)", &cYellow, func() { seed = time.Now().UnixNano() }),
				{label: "Add your own", kind: fText, help: "extra characters to mix in; they are sorted by density for you",
					str: func() string { return extra }, set: func(v string) error { extra = v; return nil }},
				action("Try it on the picture", "switch to ascii mode with this ramp, without saving", nil, func() {
					a.touch()
					a.s.Mode, a.s.Charset, a.s.Chars = engine.ModeASCII, "custom", gen()
					a.changed()
				}),
				action("Save as…", "store this ramp as a named charset and use it", &cGreen, func() {
					a.saveCharsetAs(gen(), "", func(name string) { a.setCharset(name); a.form = nil })
				}),
			}
		}}
}

// --- themes ------------------------------------------------------------------

func (a *App) setTheme(t Theme) {
	applyTheme(t)
	a.prefs.Theme = t.Name
	a.savePrefs()
	a.scr.Invalidate()
}

func (a *App) saveThemeAs(t Theme, suggest string, then func(Theme)) {
	a.ask("Save theme as", "name", suggest, func(v string) {
		for _, b := range BuiltinThemes {
			if strings.EqualFold(b.Name, v) {
				a.say(v+" is a built-in theme, pick another name", 3*time.Second)
				return
			}
		}
		t.Name = v
		if a.fail(a.lib.SaveTheme(t)) {
			return
		}
		pgThemes.selectName(a, v)
		a.ok("saved theme " + v)
		if then != nil {
			then(t)
		}
	})
}

func newThemeManager() *managerPage {
	p := &managerPage{name: "Themes"}
	p.items = func(a *App) []mItem {
		var out []mItem
		for _, t := range a.lib.AllThemes() {
			out = append(out, mItem{name: t.Name, user: !t.Builtin, swatch: t.RGBs(), active: a.prefs.Theme == t.Name})
		}
		return out
	}
	find := func(a *App, it mItem) Theme { t, _ := a.lib.FindTheme(it.name); return t }
	p.actions = []mAction{
		{'l', "Use", &cGreen, func(a *App, it mItem, ok bool) {
			if ok {
				a.setTheme(find(a, it))
				a.ok("theme: " + it.name)
			}
		}},
		{'g', "Generate", &cAccent, func(a *App, it mItem, ok bool) { a.openThemeGen() }},
		{'e', "Edit", nil, func(a *App, it mItem, ok bool) {
			if !ok {
				return
			}
			if !it.user {
				a.saveThemeAs(find(a, it), it.name+"-mine", func(t Theme) { a.setTheme(t); a.openThemeEdit(t) })
				return
			}
			a.setTheme(find(a, it))
			a.openThemeEdit(find(a, it))
		}},
		{'c', "Copy", nil, func(a *App, it mItem, ok bool) {
			if ok {
				a.saveThemeAs(find(a, it), it.name+"-copy", nil)
			}
		}},
		{'r', "Rename", nil, func(a *App, it mItem, ok bool) {
			if !a.userOnly(it, ok, "make") {
				return
			}
			a.ask("Rename theme", "new name", it.name, func(v string) {
				if a.fail(a.lib.Rename("theme", it.name, v)) {
					return
				}
				if a.prefs.Theme == it.name {
					a.prefs.Theme = v
					a.savePrefs()
				}
				a.ok("renamed to " + v)
			})
		}},
		{'d', "Delete", &cRed, func(a *App, it mItem, ok bool) {
			if !a.userOnly(it, ok, "make") {
				return
			}
			a.confirm = &Confirm{msg: fmt.Sprintf("Delete theme %q?", it.name), yes: func() {
				if a.fail(a.lib.Delete("theme", it.name)) {
					return
				}
				if a.prefs.Theme == it.name {
					a.setTheme(BuiltinThemes[0])
				}
				a.ok("deleted " + it.name)
			}}
		}},
	}
	return p
}

func (a *App) openThemeGen() {
	hue, chroma, light := 250.0, 0.14, false
	before, _ := a.lib.FindTheme(a.prefs.Theme)
	gen := func() Theme { return GenTheme("generated", hue, chroma, light) }
	live := func(f field) field {
		// Show the result on the interface itself while the sliders move.
		wrap := func(fn func()) { fn(); applyTheme(gen()); a.scr.Invalidate() }
		if n := f.nudge; n != nil {
			f.nudge = func(d int) { wrap(func() { n(d) }) }
		}
		if sf := f.setFrac; sf != nil {
			f.setFrac = func(x float64) { wrap(func() { sf(x) }) }
		}
		if st := f.set; st != nil {
			f.set = func(s string) (err error) { wrap(func() { err = st(s) }); return }
		}
		return f
	}
	applyTheme(gen())
	a.form = &Form{title: "Theme generator — the interface previews it live", previewH: 1,
		preview: func(a *App, x, y, w int) { a.swatchRow(x, y, w, gen().RGBs()) },
		fields: func() []field {
			return []field{
				live(numField("Hue", "the color the whole theme is built around, degrees", &hue, 0, 360, 5, true)),
				live(numField("Chroma", "how colorful the accents are", &chroma, 0.02, 0.3, 0.01, false)),
				live(boolField("Light", "a light theme instead of a dark one", &light)),
				action("⚄ Random", "roll a random theme", &cYellow, func() {
					r := newRand(time.Now().UnixNano())
					hue, chroma, light = float64(r.Intn(72)*5), 0.08+float64(r.Intn(15))/100, r.Intn(6) == 0
					applyTheme(gen())
					a.scr.Invalidate()
				}),
				action("Save as…", "keep this theme and use it", &cGreen, func() {
					a.saveThemeAs(gen(), "", func(t Theme) { a.setTheme(t); a.form = nil })
				}),
				action("Cancel", "go back to the theme you had", nil, func() {
					applyTheme(before)
					a.scr.Invalidate()
					a.form = nil
				}),
			}
		}}
}

func (a *App) openThemeEdit(t Theme) {
	cols := make([]string, len(ThemeSlots))
	for i, c := range t.RGBs() {
		cols[i] = c.Hex()
	}
	store := func() {
		t.Colors = append([]string(nil), cols...)
		applyTheme(t)
		a.scr.Invalidate()
		a.fail(a.lib.SaveTheme(t))
	}
	a.form = &Form{title: "Edit theme " + t.Name, previewH: 1,
		preview: func(a *App, x, y, w int) { a.swatchRow(x, y, w, Theme{Colors: cols}.RGBs()) },
		fields: func() []field {
			var fs []field
			for i, slot := range ThemeSlots {
				f := hexField(slot, "hex color of the "+slot+" parts of the interface", &cols[i], false)
				set := f.set
				f.set = func(s string) error {
					if err := set(s); err != nil {
						return err
					}
					store()
					return nil
				}
				fs = append(fs, f)
			}
			return append(fs, action("Done", "", &cGreen, func() { a.form = nil }))
		}}
}

// --- preferences -------------------------------------------------------------

func (a *App) savePrefs() {
	if a.prefs == nil {
		return
	}
	a.prefs.UI, a.prefs.HUD, a.prefs.Stats = uiNames[max(0, min(len(uiNames)-1, a.ui))], a.hud, a.stats
	if err := a.prefs.Save(); err != nil && !errors.Is(err, os.ErrNotExist) {
		a.say("could not save preferences: "+err.Error(), 4*time.Second)
	}
}

func prefFields(a *App) []field {
	p := a.prefs
	themes := func() []string {
		var out []string
		for _, t := range a.lib.AllThemes() {
			out = append(out, t.Name)
		}
		return out
	}
	text := func(label, help string, v *string, secret bool) field {
		return field{label: label, help: help, kind: fText, secret: secret, str: func() string { return *v },
			set: func(s string) error { *v = strings.TrimSpace(s); a.savePrefs(); return nil }}
	}
	return []field{
		header("Interface"),
		{label: "Interface size", kind: fEnum, help: "size of menus and controls; the picture is not affected (Alt +/-)",
			str: func() string { return uiNames[a.ui] }, nudge: func(d int) { a.setUI(a.ui + d) }},
		{label: "Theme", kind: fEnum, help: "interface colors; the Themes tab has a generator and an editor",
			str: func() string { return p.Theme },
			nudge: func(d int) {
				if t, ok := a.lib.FindTheme(cycle(themes(), p.Theme, d)); ok {
					a.setTheme(t)
				}
			}},
		{label: "HUD", kind: fEnum, help: "auto hides the bars while playing", str: func() string { return a.hud },
			nudge: func(d int) { a.hud = cycle([]string{"auto", "on", "off"}, a.hud, d); a.savePrefs() }},
		{label: "Statistics", kind: fBool, help: "frame rate and render time overlay",
			str: func() string {
				if a.stats {
					return "on"
				}
				return "off"
			}, nudge: func(int) { a.stats = !a.stats; a.savePrefs() }},
		header("Media"),
		text("Download folder", "where downloaded media is stored (empty = ~/Videos/termo)", &p.DownloadDir, false),
		text("YouTube API key", "YouTube Data API v3 key for searching; without one yt-dlp does the search", &p.YouTubeKey, true),
		text("Giphy API key", "API key for GIF search on giphy.com", &p.GiphyKey, true),
		header("Files"),
		action("Where are my settings?", "", nil, func() {
			a.say("settings, presets and library live in "+filepath.Dir(p.Path), 6*time.Second)
		}),
		action("Use the current look at start-up", "saves the look as the preset named \"default\"", nil, func() {
			if !a.fail(a.store.Save("default", a.s.Look)) {
				a.ok("termo now starts with this look")
			}
		}),
	}
}

// --- sound presets -----------------------------------------------------------

func (a *App) setSound(sp engine.SoundPreset) {
	a.s.Sound, a.soundName = sp.Sound, sp.Name
	a.changed()
	a.say("sound: "+sp.Name+" — "+engine.SoundSummary(sp.Sound), 2*time.Second)
}

// randomSound rolls a few audio effects at random.
func (a *App) randomSound() {
	a.s.Sound, a.soundName = engine.RandomSound(a.rng), ""
	a.changed()
	a.say("random sound → "+engine.SoundSummary(a.s.Sound), 2500*time.Millisecond)
}

func (a *App) cycleSound(dir int) {
	all := a.lib.AllSounds()
	idx := -1
	for i, s := range all {
		if s.Name == a.soundName {
			idx = i
		}
	}
	if idx < 0 && dir < 0 {
		idx = 0
	}
	a.setSound(all[((idx+dir)%len(all)+len(all))%len(all)])
}

func (a *App) saveSoundAs(s engine.Sound, suggest string) {
	a.ask("Save the sound as", "name", suggest, func(v string) {
		for _, b := range engine.BuiltinSounds {
			if strings.EqualFold(b.Name, v) {
				a.say(v+" is a built-in sound, pick another name", 3*time.Second)
				return
			}
		}
		if a.fail(a.lib.SaveSound(v, s)) {
			return
		}
		a.soundName = v
		pgSounds.selectName(a, v)
		a.ok("saved sound " + v)
	})
}

func newSoundManager() *managerPage {
	p := &managerPage{name: "Sounds"}
	p.items = func(a *App) []mItem {
		var out []mItem
		for _, s := range a.lib.AllSounds() {
			out = append(out, mItem{name: s.Name, user: !s.Builtin, info: engine.SoundSummary(s.Sound), active: s.Name == a.soundName})
		}
		return out
	}
	find := func(a *App, it mItem) engine.SoundPreset { s, _ := a.lib.FindSound(it.name); return s }
	p.actions = []mAction{
		{'l', "Use", &cGreen, func(a *App, it mItem, ok bool) {
			if ok {
				a.setSound(find(a, it))
			}
		}},
		{'n', "New", &cAccent, func(a *App, it mItem, ok bool) { a.saveSoundAs(a.s.Sound, "") }},
		{'g', "⚄ Random", &cYellow, func(a *App, it mItem, ok bool) { a.randomSound() }},
		{'s', "Update", nil, func(a *App, it mItem, ok bool) {
			if a.userOnly(it, ok, "save") {
				a.confirm = &Confirm{msg: fmt.Sprintf("Overwrite %q with the current sound?", it.name), yes: func() {
					if !a.fail(a.lib.SaveSound(it.name, a.s.Sound)) {
						a.soundName = it.name
						a.ok("saved " + it.name)
					}
				}}
			}
		}},
		{'c', "Copy", nil, func(a *App, it mItem, ok bool) {
			if ok {
				a.saveSoundAs(find(a, it).Sound, it.name+"-copy")
			}
		}},
		{'r', "Rename", nil, func(a *App, it mItem, ok bool) {
			if !a.userOnly(it, ok, "make") {
				return
			}
			a.ask("Rename sound", "new name", it.name, func(v string) {
				if a.fail(a.lib.Rename("sound", it.name, v)) {
					return
				}
				if a.soundName == it.name {
					a.soundName = v
				}
				a.ok("renamed to " + v)
			})
		}},
		{'d', "Delete", &cRed, func(a *App, it mItem, ok bool) {
			if !a.userOnly(it, ok, "make") {
				return
			}
			a.confirm = &Confirm{msg: fmt.Sprintf("Delete sound %q?", it.name), yes: func() {
				if !a.fail(a.lib.Delete("sound", it.name)) {
					a.ok("deleted " + it.name)
				}
			}}
		}},
		{'0', "Clean", nil, func(a *App, it mItem, ok bool) { a.setSound(engine.BuiltinSounds[0]) }},
	}
	return p
}

// --- effect presets ----------------------------------------------------------

// setFX puts a set of effects on top of the current look.
func (a *App) setFX(p engine.FXPreset) {
	a.touch()
	a.s.FX = p.FX
	a.changed()
	a.say("effects: "+p.Name+" — "+engine.FXSummary(p.FX), 2*time.Second)
}

// randomFX rolls a few effects at random.
func (a *App) randomFX() {
	a.touch()
	a.s.FX = engine.RandomFX(a.rng)
	a.changed()
	a.say("random effects → "+engine.FXSummary(a.s.FX)+"   (u = undo)", 2500*time.Millisecond)
}

func (a *App) cycleFX(dir int) {
	all := a.lib.AllFX()
	idx := -1
	for i, p := range all {
		if p.FX == a.s.FX {
			idx = i
			break
		}
	}
	if idx < 0 && dir < 0 {
		idx = 0
	}
	a.setFX(all[((idx+dir)%len(all)+len(all))%len(all)])
}

func (a *App) saveFXAs(f engine.FX, suggest string) {
	a.ask("Save the effects as", "name", suggest, func(v string) {
		for _, b := range engine.BuiltinFX {
			if strings.EqualFold(b.Name, v) {
				a.say(v+" is a built-in effect preset, pick another name", 3*time.Second)
				return
			}
		}
		if a.fail(a.lib.SaveFX(v, f)) {
			return
		}
		pgFX.selectName(a, v)
		a.ok("saved effects " + v)
	})
}

func newFXManager() *managerPage {
	p := &managerPage{name: "Effect presets"}
	p.items = func(a *App) []mItem {
		var out []mItem
		for _, f := range a.lib.AllFX() {
			out = append(out, mItem{name: f.Name, user: !f.Builtin, info: engine.FXSummary(f.FX), active: f.FX == a.s.FX})
		}
		return out
	}
	find := func(a *App, it mItem) engine.FXPreset { f, _ := a.lib.FindFX(it.name); return f }
	p.actions = []mAction{
		{'l', "Use", &cGreen, func(a *App, it mItem, ok bool) {
			if ok {
				a.setFX(find(a, it))
			}
		}},
		{'n', "New", &cAccent, func(a *App, it mItem, ok bool) { a.saveFXAs(a.s.FX, "") }},
		{'g', "⚄ Random", &cYellow, func(a *App, it mItem, ok bool) { a.randomFX() }},
		{'s', "Update", nil, func(a *App, it mItem, ok bool) {
			if a.userOnly(it, ok, "save") {
				a.confirm = &Confirm{msg: fmt.Sprintf("Overwrite %q with the current effects?", it.name), yes: func() {
					if !a.fail(a.lib.SaveFX(it.name, a.s.FX)) {
						a.ok("saved " + it.name)
					}
				}}
			}
		}},
		{'c', "Copy", nil, func(a *App, it mItem, ok bool) {
			if ok {
				a.saveFXAs(find(a, it).FX, it.name+"-copy")
			}
		}},
		{'r', "Rename", nil, func(a *App, it mItem, ok bool) {
			if !a.userOnly(it, ok, "make") {
				return
			}
			a.ask("Rename effect preset", "new name", it.name, func(v string) {
				if !a.fail(a.lib.Rename("fx", it.name, v)) {
					p.selectName(a, v)
					a.ok("renamed to " + v)
				}
			})
		}},
		{'d', "Delete", &cRed, func(a *App, it mItem, ok bool) {
			if !a.userOnly(it, ok, "make") {
				return
			}
			a.confirm = &Confirm{msg: fmt.Sprintf("Delete effect preset %q?", it.name), yes: func() {
				if !a.fail(a.lib.Delete("fx", it.name)) {
					a.ok("deleted " + it.name)
				}
			}}
		}},
		{'0', "Off", nil, func(a *App, it mItem, ok bool) { a.setFX(engine.BuiltinFX[0]) }},
	}
	return p
}
