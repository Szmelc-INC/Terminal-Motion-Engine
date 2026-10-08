// Command termo plays GIFs and videos in the terminal as ASCII / ANSI art.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/app"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/media"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
)

const version = "2.0.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "termo:", err)
		os.Exit(1)
	}
}

// extra describes a flag that is not part of the option table.
type extra struct {
	value bool // takes a value
	help  string
}

var extras = map[string]extra{
	"preset":     {true, "start from a saved or built-in preset"},
	"random":     {false, "start with a randomized look"},
	"seed":       {true, "seed for --random (default: time)"},
	"no-audio":   {false, "do not play sound"},
	"audio-sink": {true, "shell command that plays raw s16le 48 kHz stereo PCM from stdin"},
	"hwaccel":    {false, "let ffmpeg use hardware decoding"},
	"hud":        {true, "HUD visibility: auto, on, off"},
	"ui":         {true, "interface size: compact, normal, large, huge"},
	"stats":      {false, "show the performance overlay"},
	"depth":      {true, "terminal color depth: 24, 8 or 4 (default: auto-detect)"},
	"start":      {true, "start position in seconds"},
	"at":         {true, "snap: time of the frame to print, seconds"},
	"size":       {true, "snap/bench: grid size as COLSxROWS"},
	"frames":     {true, "bench: number of frames to measure"},
	"all":        {false, "bench: measure every mode and dither"},
	"presets":    {true, "path to the preset file"},
	"sort":       {false, "charsets add: order the characters from empty to full"},
	"sound":      {true, "start with a sound preset (termo sounds lists them)"},
	"site":       {true, "find/search/get: youtube, giphy, tenor, pinterest, archive, wikimedia, url"},
	"kind":       {true, "search filter: any, video, gif, image"},
	"length":     {true, "search filter: any, short, medium, long"},
	"sort-by":    {true, "search order: relevance, date, views"},
	"count":      {true, "search: number of results"},
	"pick":       {true, "get: which search result to take (default 1)"},
	"as":         {true, "get: mp4, mp4-720, mp4-480, webm, mkv, gif, mp3, m4a, opus, flac, wav, original"},
	"dir":        {true, "get: folder to save into"},
	"help":       {false, "show help"},
	"version":    {false, "show version"},
}

var shorts = map[string]string{
	"m": "mode", "p": "palette", "d": "dither", "c": "colors", "P": "preset",
	"f": "fps", "r": "random", "h": "help", "v": "version", "s": "start",
}

// library is the user's library, loaded once in run.
var library *app.Library

type flagVal struct{ key, val string }

type cli struct {
	flags []flagVal
	args  []string
}

func (c *cli) get(key string) (string, bool) {
	for i := len(c.flags) - 1; i >= 0; i-- {
		if c.flags[i].key == key {
			return c.flags[i].val, true
		}
	}
	return "", false
}

func (c *cli) has(key string) bool { _, ok := c.get(key); return ok }

func parse(argv []string) (*cli, error) {
	c := &cli{}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "--" {
			c.args = append(c.args, argv[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			c.args = append(c.args, arg)
			continue
		}
		name := strings.TrimLeft(arg, "-")
		val, hasVal := "", false
		if k, v, ok := strings.Cut(name, "="); ok {
			name, val, hasVal = k, v, true
		}
		if !strings.HasPrefix(arg, "--") {
			long, ok := shorts[name]
			if !ok {
				return nil, fmt.Errorf("unknown option -%s (see termo --help)", name)
			}
			name = long
		}
		negated := false
		opt := engine.FindOption(name)
		ex, isExtra := extras[name]
		if opt == nil && !isExtra && strings.HasPrefix(name, "no-") {
			if o := engine.FindOption(strings.TrimPrefix(name, "no-")); o != nil && o.Kind == engine.KBool {
				opt, negated = o, true
				name = o.Key
			}
		}
		if opt == nil && !isExtra {
			return nil, fmt.Errorf("unknown option --%s (see termo --help)", name)
		}
		takesValue := (opt != nil && opt.Kind != engine.KBool) || (isExtra && ex.value)
		switch {
		case takesValue && !hasVal:
			if i+1 >= len(argv) {
				return nil, fmt.Errorf("option --%s needs a value", name)
			}
			i++
			val = argv[i]
		case !takesValue && !hasVal:
			val = "on"
		}
		if negated {
			val = "off"
		}
		c.flags = append(c.flags, flagVal{name, val})
	}
	return c, nil
}

// settings builds the settings from defaults, an optional preset, an
// optional random roll and then the explicit flags, in that order.
func (c *cli) settings(store *app.Store) (engine.Settings, string, error) {
	s := engine.DefaultSettings()
	preset := ""
	if name, ok := c.get("preset"); ok {
		p, found := store.Find(name)
		if !found {
			var names []string
			for _, p := range store.All() {
				names = append(names, p.Name)
			}
			return s, "", fmt.Errorf("no preset named %q (available: %s)", name, strings.Join(names, ", "))
		}
		s.Look, preset = p.Look, p.Name
	} else if p, found := store.Find("default"); found && !p.Builtin {
		s.Look, preset = p.Look, p.Name
	}
	if c.has("random") {
		seed := time.Now().UnixNano()
		if v, ok := c.get("seed"); ok {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return s, "", fmt.Errorf("--seed: %q is not a whole number", v)
			}
			seed = n
		}
		engine.Randomize(&s.Look, rand.New(rand.NewSource(seed)))
		preset = ""
	}
	if name, ok := c.get("sound"); ok && library != nil {
		sp, found := library.FindSound(name)
		if !found {
			return s, "", fmt.Errorf("no sound named %q (termo sounds lists them)", name)
		}
		s.Sound = sp.Sound
	}
	for _, f := range c.flags {
		if o := engine.FindOption(f.key); o != nil {
			if err := o.Set(&s, f.val); err != nil {
				return s, "", err
			}
			if o.Key == "custom" && !c.has("palette") {
				s.Palette = engine.PalCustom
			}
			if o.Key == "chars" && !c.has("charset") {
				s.Charset = "custom"
			}
		}
	}
	return s, preset, nil
}

func (c *cli) depth() (int, error) {
	v, ok := c.get("depth")
	if !ok {
		return tty.DetectDepth(), nil
	}
	switch v {
	case "24", "truecolor":
		return 24, nil
	case "8", "256":
		return 8, nil
	case "4", "16":
		return 4, nil
	}
	return 0, fmt.Errorf("--depth: %q is not one of 24, 8, 4", v)
}

func (c *cli) float(key string, def float64) (float64, error) {
	v, ok := c.get(key)
	if !ok {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 {
		return 0, fmt.Errorf("--%s: %q is not a non-negative number", key, v)
	}
	return f, nil
}

func (c *cli) size(defCols, defRows int) (int, int, error) {
	v, ok := c.get("size")
	if !ok {
		return defCols, defRows, nil
	}
	a, b, found := strings.Cut(strings.ToLower(v), "x")
	cols, e1 := strconv.Atoi(a)
	rows, e2 := strconv.Atoi(b)
	if !found || e1 != nil || e2 != nil || cols < 1 || rows < 1 || cols > 2000 || rows > 1000 {
		return 0, 0, fmt.Errorf("--size: %q is not COLSxROWS (e.g. 120x40)", v)
	}
	return cols, rows, nil
}

func run(argv []string) error {
	c, err := parse(argv)
	if err != nil {
		return err
	}
	if c.has("help") || (len(c.args) > 0 && c.args[0] == "help") {
		usage(os.Stdout)
		return nil
	}
	if c.has("version") {
		fmt.Println("termo", version)
		return nil
	}
	path := app.StorePath()
	if v, ok := c.get("presets"); ok {
		path = v
	}
	store, err := app.LoadStore(path)
	if err != nil {
		return fmt.Errorf("cannot load presets: %v", err)
	}
	// The library registers user palettes and charsets with the engine, so
	// every command (and every flag that names one) can use them.
	lib, err := app.LoadLibrary(filepath.Dir(store.Path))
	if err != nil {
		return fmt.Errorf("cannot load the library: %v", err)
	}
	library = lib
	cmd := ""
	if len(c.args) > 0 {
		switch c.args[0] {
		case "play", "snap", "bench", "info", "presets", "palettes", "charsets", "themes", "options",
			"find", "search", "get", "sounds":
			// A file that happens to share a command's name still plays.
			if _, err := os.Stat(c.args[0]); err != nil || c.args[0] == "play" {
				cmd, c.args = c.args[0], c.args[1:]
			}
		}
	}
	switch cmd {
	case "presets":
		return cmdPresets(c, store)
	case "palettes":
		return cmdPalettes(c, lib)
	case "charsets":
		return cmdCharsets(c, lib)
	case "themes":
		return cmdThemes(c, lib)
	case "sounds":
		for _, s := range lib.AllSounds() {
			tag := "user"
			if s.Builtin {
				tag = "built-in"
			}
			fmt.Printf("  %-16s %-9s %s\n", s.Name, tag, engine.SoundSummary(s.Sound))
		}
		return nil
	case "search", "get":
		prefs, err := app.LoadPrefs(filepath.Dir(store.Path))
		if err != nil {
			return fmt.Errorf("cannot load the preferences: %v", err)
		}
		if cmd == "search" {
			return cmdSearch(c, prefs)
		}
		if err := media.CheckTools(); err != nil {
			return err
		}
		return cmdGet(c, prefs)
	case "options":
		return cmdOptions()
	}
	if err := media.CheckTools(); err != nil {
		return err
	}
	switch cmd {
	case "info":
		return cmdInfo(c)
	case "snap":
		return cmdSnap(c, store)
	case "bench":
		return cmdBench(c, store)
	}
	return cmdPlay(c, store, lib, cmd == "find")
}

func cmdPlay(c *cli, store *app.Store, lib *app.Library, find bool) error {
	s, preset, err := c.settings(store)
	if err != nil {
		return err
	}
	depth, err := c.depth()
	if err != nil {
		return err
	}
	start, err := c.float("start", 0)
	if err != nil {
		return err
	}
	hud, _ := c.get("hud")
	switch hud {
	case "", "auto", "on", "off":
	default:
		return fmt.Errorf("--hud: %q is not one of auto, on, off", hud)
	}
	files, findQuery, findSite := c.args, "", ""
	var streams []app.Stream
	if find {
		// "termo find cats" opens the finder with that search already run.
		files, findQuery = nil, strings.Join(c.args, " ")
		if findQuery == "" {
			findQuery = " "
		}
		site, _ := c.get("site")
		p, err := findProvider(site)
		if err != nil {
			return err
		}
		findSite = p.Name
	} else {
		for _, f := range c.args {
			if _, err := os.Stat(f); err != nil && !media.IsURL(f) {
				return fmt.Errorf("cannot open %s: no such file or folder", f)
			}
		}
		if files, streams, err = resolveArgs(c.args); err != nil {
			return err
		}
	}
	prefs, err := app.LoadPrefs(filepath.Dir(store.Path))
	if err != nil {
		return fmt.Errorf("cannot load the preferences: %v", err)
	}
	if hud == "" {
		hud = prefs.HUD
	}
	ui := app.UIScale(prefs.UI)
	if v, ok := c.get("ui"); ok {
		if ui = app.UIScale(v); ui < 0 {
			return fmt.Errorf("--ui: %q is not one of compact, normal, large, huge", v)
		}
	}
	sink, _ := c.get("audio-sink")
	soundName, _ := c.get("sound")
	return app.Run(app.Config{
		Sound: soundName, UIScale: ui, Lib: lib, Prefs: prefs, Find: findQuery, FindSite: findSite, Streams: streams,
		Files: files, Settings: s, LoopSet: c.has("loop"), NoAudio: c.has("no-audio"), AudioSink: sink,
		HWAccel: c.has("hwaccel"), HUD: hud, Depth: depth, Start: start, Stats: c.has("stats") || prefs.Stats,
		Store: store, Preset: preset,
	})
}

func cmdInfo(c *cli) error {
	if len(c.args) == 0 {
		return errors.New("usage: termo info <file>")
	}
	for _, f := range c.args {
		info, err := media.Probe(f)
		if err != nil {
			return err
		}
		audio := "no"
		if info.HasAudio {
			audio = "yes"
		}
		fmt.Printf("%s\n  video    %dx%d %s, %.4g fps\n  duration %.2f s\n  audio    %s\n",
			f, info.Width, info.Height, info.Codec, info.FPS, info.Duration, audio)
	}
	return nil
}

func cellAspect(s engine.Settings) float64 {
	if s.CellAspect > 0 {
		return s.CellAspect
	}
	return 0.5
}

// grab decodes n frames at the geometry the look needs for a cols×rows grid.
func grab(info media.Info, s engine.Settings, cols, rows int, at float64, n int) ([]*media.Frame, app.Geometry, time.Duration, error) {
	g := app.Fit(info.Aspect(), s.Look, cols, rows, cellAspect(s))
	fps := info.FPS
	if s.FPS > 0 && s.FPS < fps {
		fps = s.FPS
	}
	v, err := media.StartVideo(media.VideoOpts{
		Path: info.Path, PreInput: info.PreInput, Start: at, Base: at, W: g.W, H: g.H, FPS: fps,
		CropX: g.CropX, CropY: g.CropY, Still: info.Still, Loop: n > 1 && !info.Still,
	})
	if err != nil {
		return nil, g, 0, err
	}
	defer v.Close()
	var frames []*media.Frame
	t := time.Now()
	for f := range v.Frames {
		frames = append(frames, f)
		if len(frames) >= n {
			break
		}
	}
	el := time.Since(t)
	if len(frames) == 0 {
		if err := v.Err(); err != nil {
			return nil, g, 0, err
		}
		return nil, g, 0, fmt.Errorf("%s: no frame at %.2f s", info.Path, at)
	}
	return frames, g, el, nil
}

func cmdSnap(c *cli, store *app.Store) error {
	if len(c.args) != 1 {
		return errors.New("usage: termo snap [options] [--at SEC] [--size COLSxROWS] <file>")
	}
	s, _, err := c.settings(store)
	if err != nil {
		return err
	}
	depth, err := c.depth()
	if err != nil {
		return err
	}
	at, err := c.float("at", 0)
	if err != nil {
		return err
	}
	dc, dr := 80, 24
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 && h > 1 {
		dc, dr = w, h-1
	}
	cols, rows, err := c.size(dc, dr)
	if err != nil {
		return err
	}
	info, err := media.Probe(c.args[0])
	if err != nil {
		return err
	}
	frames, g, _, err := grab(info, s, cols, rows, at, 1)
	if err != nil {
		return err
	}
	r := engine.Renderer{TermDepth: depth}
	cells := make([]engine.Cell, g.Cols*g.Rows)
	r.Render(frames[0].Pix, g.W, g.H, &s.Look, cells, g.Cols, g.Rows, 0)
	_, err = os.Stdout.Write(tty.Dump(cells, g.Cols, g.Rows, depth))
	return err
}

func cmdBench(c *cli, store *app.Store) error {
	if len(c.args) != 1 {
		return errors.New("usage: termo bench [options] [--size COLSxROWS] [--frames N] [--all] <file>")
	}
	base, _, err := c.settings(store)
	if err != nil {
		return err
	}
	depth, err := c.depth()
	if err != nil {
		return err
	}
	cols, rows, err := c.size(240, 67)
	if err != nil {
		return err
	}
	nf, err := c.float("frames", 240)
	if err != nil {
		return err
	}
	n := max(2, int(nf))
	info, err := media.Probe(c.args[0])
	if err != nil {
		return err
	}
	fmt.Printf("%s: %dx%d %.4g fps → grid %dx%d, %d frames, %d-bit color\n\n",
		c.args[0], info.Width, info.Height, info.FPS, cols, rows, n, depth)
	fmt.Printf("%-9s %-17s %-12s %9s %9s %9s %10s %9s\n",
		"mode", "dither", "palette", "decode", "render", "encode", "KiB/frame", "max fps")

	one := func(s engine.Settings) error {
		frames, g, decTime, err := grab(info, s, cols, rows, 0, n)
		if err != nil {
			return err
		}
		r := engine.Renderer{TermDepth: depth}
		scr := tty.NewScreen(io.Discard, depth)
		scr.Resize(cols, rows)
		cells := make([]engine.Cell, g.Cols*g.Rows)
		var render, encode time.Duration
		var bytes int
		for i, f := range frames {
			t := time.Now()
			r.Render(f.Pix, g.W, g.H, &s.Look, cells, g.Cols, g.Rows, i)
			render += time.Since(t)
			t = time.Now()
			scr.Blit((cols-g.Cols)/2, (rows-g.Rows)/2, g.Cols, g.Rows, cells)
			nb, _ := scr.Flush()
			encode += time.Since(t)
			bytes += nb
		}
		k := float64(len(frames))
		ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 / k }
		fmt.Printf("%-9s %-17s %-12s %7.2fms %7.2fms %7.2fms %10.1f %9.0f\n",
			s.Mode, s.Dither, s.Palette, ms(decTime), ms(render), ms(encode),
			float64(bytes)/k/1024, 1000/(ms(render)+ms(encode)))
		return nil
	}
	if !c.has("all") {
		return one(base)
	}
	for _, mode := range engine.Modes {
		for _, d := range []string{"none", "bayer4", "bluenoise", "floyd-steinberg", "jjn"} {
			for _, pal := range []string{engine.PalOff, "pico8", engine.PalXterm} {
				if pal == engine.PalOff && d != "none" && mode != engine.ModeBraille && mode != engine.ModeASCII {
					continue // dithering is a no-op on truecolor block modes
				}
				s := base
				s.Mode, s.Dither, s.Palette = mode, d, pal
				if err := one(s); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func swatches(colors []engine.RGB, depth int) string {
	cells := make([]engine.Cell, 0, len(colors)*2)
	for _, col := range colors {
		cells = append(cells, engine.Cell{Ch: ' ', Bg: col.U32()}, engine.Cell{Ch: ' ', Bg: col.U32()})
	}
	return strings.TrimRight(string(tty.Dump(cells, len(cells), 1, depth)), "\n")
}

// cmdPalettes lists the palettes, or manages the user's own:
// add NAME COLORS · import SOURCE [NAME] · rm NAME · export NAME
func cmdPalettes(c *cli, lib *app.Library) error {
	if len(c.args) > 0 && c.args[0] != "list" && c.args[0] != "ls" {
		arg := func(i int) string {
			if len(c.args) > i {
				return c.args[i]
			}
			return ""
		}
		switch sub := c.args[0]; sub {
		case "add", "import":
			name, src := arg(1), arg(2)
			if sub == "import" {
				src, name = arg(1), arg(2)
				if name == "" {
					name = strings.TrimSuffix(filepath.Base(strings.TrimPrefix(src, "lospec:")), filepath.Ext(src))
				}
			}
			if name == "" || src == "" {
				return errors.New("usage: termo palettes add NAME '#hex,#hex,…'  |  termo palettes import FILE|URL|lospec:NAME [NAME]")
			}
			text := src
			if sub == "import" {
				t, err := app.FetchText(src)
				if err != nil {
					return err
				}
				text = t
			}
			cols, err := engine.ParsePaletteText(text)
			if err != nil {
				return err
			}
			if err := lib.SavePalette(name, cols); err != nil {
				return err
			}
			fmt.Printf("saved palette %s (%d colors)\n", name, len(cols))
		case "rm", "delete", "del":
			if err := lib.Delete("palette", arg(1)); err != nil {
				return err
			}
			fmt.Println("deleted", arg(1))
		case "export", "show":
			cols, ok := engine.FindPalette(arg(1))
			if !ok {
				return fmt.Errorf("no palette named %q", arg(1))
			}
			for _, col := range cols {
				fmt.Println(strings.TrimPrefix(col.Hex(), "#"))
			}
		default:
			return fmt.Errorf("unknown palettes command %q (list, add, import, rm, export)", sub)
		}
		return nil
	}
	return listPalettes(c)
}

// cmdCharsets lists the ASCII ramps, or manages the user's own:
// add NAME CHARS · gen NAME POOL [COUNT] · rm NAME
func cmdCharsets(c *cli, lib *app.Library) error {
	arg := func(i int) string {
		if len(c.args) > i {
			return c.args[i]
		}
		return ""
	}
	switch sub := arg(0); sub {
	case "", "list", "ls":
		for _, cs := range engine.AllCharsets() {
			if cs.Name != "custom" {
				fmt.Printf("  %-14s %3d  |%s|\n", cs.Name, len(cs.Runes), string(cs.Runes))
			}
		}
		fmt.Println("\nGenerator pools (termo charsets gen NAME POOL [COUNT]):", strings.Join(engine.GlyphSetNames(), ", "))
	case "add":
		if arg(1) == "" || arg(2) == "" {
			return errors.New("usage: termo charsets add NAME 'CHARS'   (emptiest character first; --sort orders them for you)")
		}
		chars := arg(2)
		if c.has("sort") {
			chars = engine.SortByDensity(chars)
		}
		if err := lib.SaveCharset(arg(1), chars); err != nil {
			return err
		}
		fmt.Printf("saved charset %s |%s|\n", arg(1), chars)
	case "gen":
		n := 12
		if v, err := strconv.Atoi(arg(3)); err == nil {
			n = v
		}
		if arg(1) == "" || arg(2) == "" {
			return errors.New("usage: termo charsets gen NAME POOL [COUNT]")
		}
		chars := engine.GenCharset(arg(2), n, c.has("random"), rand.New(rand.NewSource(time.Now().UnixNano())))
		if err := lib.SaveCharset(arg(1), chars); err != nil {
			return err
		}
		fmt.Printf("saved charset %s |%s|\n", arg(1), chars)
	case "rm", "delete", "del":
		if err := lib.Delete("charset", arg(1)); err != nil {
			return err
		}
		fmt.Println("deleted", arg(1))
	default:
		return fmt.Errorf("unknown charsets command %q (list, add, gen, rm)", sub)
	}
	return nil
}

func cmdThemes(c *cli, lib *app.Library) error {
	depth, err := c.depth()
	if err != nil {
		return err
	}
	for _, t := range lib.AllThemes() {
		tag := "user"
		if t.Builtin {
			tag = "built-in"
		}
		fmt.Printf("  %-14s %-9s %s\n", t.Name, tag, swatches(t.RGBs(), depth))
	}
	return nil
}

func listPalettes(c *cli) error {
	depth, err := c.depth()
	if err != nil {
		return err
	}
	show := term.IsTerminal(int(os.Stdout.Fd()))
	line := func(name string, colors []engine.RGB) {
		if show && len(colors) <= 32 {
			fmt.Printf("  %-12s %3d  %s\n", name, len(colors), swatches(colors, depth))
			return
		}
		var hex []string
		for i, col := range colors {
			if i == 16 {
				hex = append(hex, "…")
				break
			}
			hex = append(hex, col.Hex())
		}
		fmt.Printf("  %-12s %3d  %s\n", name, len(colors), strings.Join(hex, " "))
	}
	fmt.Println("Named palettes (--palette NAME):")
	for _, p := range engine.AllPalettes() {
		line(p.Name, p.Colors)
	}
	fmt.Println("\nGenerated palettes:")
	fmt.Println("  truecolor         no palette, full 24-bit color")
	fmt.Println("  harmony           color-theory palette: --scheme, --hue, --chroma, --lmin, --lmax, --colors")
	fmt.Println("  adaptive          --colors most representative colors of the picture (median cut)")
	fmt.Println("  gray / cube       --colors greys / uniform RGB cube")
	fmt.Println("  ansi16 / xterm256 the terminal's standard palettes")
	fmt.Println("  custom            your own: --custom '#1a1c2c,#f4f4f4,#ef7d57'")
	fmt.Println("\nHarmony schemes (--scheme), shown at --hue 200 --colors 8:")
	for _, sc := range engine.Schemes {
		line(sc, engine.Harmony(sc, 200, 0.16, 0.12, 0.95, 8))
	}
	return nil
}

func cmdOptions() error {
	for _, g := range engine.Groups {
		fmt.Printf("%s:\n", g)
		for _, o := range engine.Options {
			if o.Group != g {
				continue
			}
			var kind string
			switch o.Kind {
			case engine.KEnum:
				kind = strings.Join(o.Choices, "|")
			case engine.KBool:
				kind = "(flag; --no-" + o.Key + " to turn off)"
			case engine.KInt, engine.KFloat:
				kind = fmt.Sprintf("%s..%s", strconv.FormatFloat(o.Min, 'f', -1, 64), strconv.FormatFloat(o.Max, 'f', -1, 64))
			case engine.KList:
				kind = "#hex,#hex,…"
			default:
				kind = "TEXT"
			}
			fmt.Printf("  --%-15s %s\n", o.Key, kind)
			if o.Help != "" {
				fmt.Printf("  %-17s %s\n", "", o.Help)
			}
		}
		fmt.Println()
	}
	return nil
}

func cmdPresets(c *cli, store *app.Store) error {
	sub := "list"
	if len(c.args) > 0 {
		sub = c.args[0]
	}
	need := func() (string, error) {
		if len(c.args) < 2 || strings.TrimSpace(c.args[1]) == "" {
			return "", fmt.Errorf("usage: termo presets %s <name>", sub)
		}
		return c.args[1], nil
	}
	switch sub {
	case "list", "ls":
		for _, p := range store.All() {
			tag := "user"
			if p.Builtin {
				tag = "built-in"
			}
			fmt.Printf("%-20s %-9s %s\n", p.Name, tag, app.Summary(p.Look))
		}
	case "path":
		fmt.Println(store.Path)
	case "show":
		name, err := need()
		if err != nil {
			return err
		}
		p, ok := store.Find(name)
		if !ok {
			return fmt.Errorf("no preset named %q", name)
		}
		out, _ := json.MarshalIndent(p.Look, "", "  ")
		fmt.Println(string(out))
	case "save", "new", "edit":
		// Start from the preset when it exists, so "edit" changes only
		// the settings named on the command line.
		name, err := need()
		if err != nil {
			return err
		}
		if _, ok := store.Find(name); ok && !c.has("preset") {
			c.flags = append([]flagVal{{"preset", name}}, c.flags...)
		} else if sub == "edit" && !ok {
			return fmt.Errorf("no preset named %q", name)
		}
		s, _, err := c.settings(store)
		if err != nil {
			return err
		}
		if err := store.Save(name, s.Look); err != nil {
			return err
		}
		fmt.Printf("saved %s: %s\n", strings.TrimSpace(name), app.Summary(s.Look))
	case "rm", "delete", "del":
		name, err := need()
		if err != nil {
			return err
		}
		if err := store.Delete(name); err != nil {
			return err
		}
		fmt.Println("deleted", name)
	case "rename", "mv":
		if len(c.args) < 3 {
			return errors.New("usage: termo presets rename <old> <new>")
		}
		if err := store.Rename(c.args[1], c.args[2]); err != nil {
			return err
		}
		fmt.Printf("renamed %s → %s\n", c.args[1], strings.TrimSpace(c.args[2]))
	default:
		return fmt.Errorf("unknown presets command %q (list, show, save, edit, rename, rm, path)", sub)
	}
	return nil
}

func usage(w io.Writer) {
	fmt.Fprintf(w, `termo %s — play GIFs and videos in the terminal as ASCII / ANSI art

Usage:
  termo [options] [file...]         play files or folders of frames (no file: file browser)
  termo snap  [options] <file>      print a single frame (--at SEC, --size COLSxROWS)
  termo bench [options] <file>      measure render speed (--frames N, --size, --all)
  termo info  <file>                show stream information
  termo find  [words…]              search the web for media: browse, preview, play, download
  termo search [--site S] words…    print search results (youtube, giphy, tenor, pinterest, archive, wikimedia)
  termo get [--as FORMAT] <link | words…>   download a link or the best match (mp4, webm, gif, mp3, wav…)
  termo <link>                      play a YouTube / web link without saving it
  termo presets [list|show|save|edit|rename|rm|path] [name]
  termo palettes [add|import|rm|export]   list palettes, or manage your own
  termo charsets [add|gen|rm]       list ASCII ramps, or manage your own
  termo themes                      list interface themes
  termo sounds                      list sound presets (--sound NAME starts with one)
  termo options                     list every look option with its values

Common options (termo options lists all of them):
  -m, --mode MODE        half | quad | sextant | braille | ascii
  -p, --palette NAME     truecolor | harmony | adaptive | gray | cube | custom | gameboy | pico8 | …
  -c, --colors N         palette size, 1-256
      --scheme NAME      color-theory scheme for --palette harmony
  -d, --dither NAME      none | bayer4 | bluenoise | halftone | floyd-steinberg | atkinson | …
  -P, --preset NAME      start from a preset
  -r, --random           start with a randomized look (--seed N to repeat it)
  -f, --fps N            cap the frame rate (default: source rate)
  -s, --start SEC        start position
      --loop / --no-loop loop playback (default: on for clips without sound)
      --volume V  --mute  --no-audio  --speed X
      --hud auto|on|off  --stats  --depth 24|8|4  --hwaccel

In the player:
  Space pause · ←/→ seek · ↑/↓ volume · r RANDOMIZE look · R random palette · u undo
  Tab or s settings menu (presets: save, load, edit, rename, delete) · p/P cycle presets
  v/d/c cycle mode/dither/palette · ? all keys · q quit
  Mouse: click = pause, right-click = menu, wheel = volume, drag the seek bar and sliders.

Presets are stored in %s
`, version, app.StorePath())
}
