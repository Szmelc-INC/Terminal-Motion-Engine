package app

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/fetch"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/media"
	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/tty"
)

// Config is what the command line hands to the player.
type Config struct {
	Files     []string
	Settings  engine.Settings
	LoopSet   bool // --loop was given explicitly
	NoAudio   bool
	AudioSink string
	HWAccel   bool
	HUD       string // auto, on, off
	Depth     int    // terminal color depth: 24, 8, 4
	Start     float64
	Stats     bool
	Store     *Store
	Preset    string // name of the preset the settings came from
	UIScale   int    // interface size: 0 compact, 1 normal, 2 large, 3 huge
	Lib       *Library
	Prefs     *Prefs
	Find      string   // open the media finder with this search
	FindSite  string   // name of the site to search first
	Streams   []Stream // what is known about web links among Files
}

// Stream describes a playlist entry that plays from the web.
type Stream struct{ Path, Audio, Name string }

type hit struct {
	x, y, w, h int
	fn         func(ev tty.Event, rx, ry int)
}

type decKey struct {
	path         string
	w, h         int
	fps          float64
	cropX, cropY float64
	panX, panY   float64
	loop         bool
}

// App is the running player. Everything except the clock is owned by the
// main loop goroutine.
type App struct {
	cfg   Config
	term  *tty.Terminal
	scr   *tty.Screen
	rend  engine.Renderer
	store *Store
	rng   *rand.Rand

	s         engine.Settings
	applied   engine.Playback
	preset    string
	history   []engine.Look
	lastTouch time.Time
	hudShown  bool

	files   []string
	fileIdx int
	info    media.Info
	loaded  bool
	cellAR  float64

	vid     *media.Video
	aud     *media.Audio
	audErr  string
	key     decKey
	pending *media.Frame
	last    *media.Frame
	eof     bool
	ended   bool
	frameNo int

	// Clock state; read by the audio feeder goroutine.
	mu        sync.Mutex
	basePos   float64
	baseWall  time.Time
	playing   bool
	hold      bool // clock frozen until the next frame is shown
	speed     float64
	needFrame bool

	seekReq  *float64
	lastSeek time.Time

	cells        []engine.Cell
	ccols, crows int
	dirty        bool
	quit         bool

	hud          string
	lastActivity time.Time
	stats        bool
	toast        string
	toastUntil   time.Time
	hits         []hit
	capture      *hit
	mx, my       int
	lastClick    time.Time
	lastClickX   int
	lastClickY   int
	dbl          bool

	// async carries results of background work (searches, downloads…) back
	// to the main loop, which owns every other field.
	async chan func()
	ui    int // interface size, see Config.UIScale

	lib   *Library
	prefs *Prefs

	// meta holds what is known about playlist entries that are web
	// streams: a readable name and, sometimes, a separate sound stream.
	meta       map[string]streamMeta
	finder     *Finder
	finderOpen bool
	downloads  []*download
	menus      map[string]*Menu
	form       *Form

	menu    *Menu
	help    bool
	helpTop int
	browser *Browser
	prompt  *Prompt
	confirm *Confirm

	// Statistics.
	shown, dropped int
	fpsWindow      time.Time
	fpsCount       int
	fpsNow         float64
	renderMs       float64
	flushBytes     float64
}

// Run starts the interactive player and blocks until the user quits.
func Run(cfg Config) (err error) {
	t, err := tty.Open()
	if err != nil {
		return err
	}
	defer t.Close()
	defer func() {
		if r := recover(); r != nil {
			t.Close()
			err = fmt.Errorf("internal error: %v\n%s", r, debug.Stack())
		}
	}()
	a := &App{
		cfg: cfg, term: t, store: cfg.Store, s: cfg.Settings, files: cfg.Files,
		rng: rand.New(rand.NewSource(time.Now().UnixNano())), hud: cfg.HUD, stats: cfg.Stats,
		preset: cfg.Preset, speed: cfg.Settings.Speed, lastActivity: time.Now(), mx: -1, my: -1,
		async: make(chan func(), 64), ui: cfg.UIScale, lib: cfg.Lib, prefs: cfg.Prefs,
	}
	if a.lib == nil {
		a.lib = &Library{Path: filepath.Join(filepath.Dir(a.store.Path), "library.json")}
	}
	if a.prefs == nil {
		a.prefs = &Prefs{Path: filepath.Join(filepath.Dir(a.store.Path), "config.json"), Theme: BuiltinThemes[0].Name}
	}
	if th, ok := a.lib.FindTheme(a.prefs.Theme); ok {
		applyTheme(th)
	}
	if a.hud == "" {
		a.hud = "auto"
	}
	a.applied = a.s.Playback
	a.rend.TermDepth = cfg.Depth
	a.scr = tty.NewScreen(t.Out(), cfg.Depth)
	a.resize()
	defer a.closeMedia()

	events := tty.ReadEvents(t.In())
	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, syscall.SIGWINCH, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
	defer signal.Stop(sigs)

	a.meta = map[string]streamMeta{}
	for _, st := range cfg.Streams {
		a.meta[st.Path] = streamMeta{name: st.Name, audio: st.Audio}
	}
	switch {
	case cfg.Find != "":
		a.openFinder("")
		for i, p := range fetch.Providers {
			if p.Name == cfg.FindSite {
				a.finder.prov = i
			}
		}
		if q := strings.TrimSpace(cfg.Find); q != "" {
			a.openFinder(q)
		}
	case len(a.files) == 0:
		a.openBrowser()
	default:
		a.open(0, cfg.Start)
	}

	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for !a.quit {
		a.tick()
		if a.dirty {
			a.draw()
		}
		var frames chan *media.Frame
		if a.vid != nil && a.pending == nil && !a.eof {
			frames = a.vid.Frames
		}
		timer.Reset(a.nextWake())
		select {
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			a.handle(ev)
			// Mouse motion arrives in bursts; handle what is queued
			// before paying for a redraw.
			for more := true; more; {
				select {
				case ev, ok := <-events:
					if !ok {
						return nil
					}
					a.handle(ev)
				default:
					more = false
				}
			}
		case f, ok := <-frames:
			if !ok {
				a.eof = true
			} else {
				a.pending = f
			}
		case fn := <-a.async:
			fn()
			a.dirty = true
		case sig := <-sigs:
			if sig != syscall.SIGWINCH {
				return nil
			}
			a.resize()
		case <-timer.C:
		}
	}
	return nil
}

func (a *App) closeMedia() {
	if a.vid != nil {
		a.vid.Close()
		a.vid = nil
	}
	if a.aud != nil {
		a.aud.Close()
		a.aud = nil
	}
}

// --- clock ---------------------------------------------------------------

func (a *App) clock() (float64, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.playing || a.hold {
		return a.basePos, false
	}
	return a.basePos + time.Since(a.baseWall).Seconds()*a.speed, true
}

func (a *App) setClock(pos float64, playing, hold bool) {
	a.mu.Lock()
	a.basePos, a.baseWall, a.playing, a.hold = pos, time.Now(), playing, hold
	a.mu.Unlock()
}

func (a *App) isPlaying() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.playing
}

// pos is the position to show the user: wrapped into the clip when looping,
// and tracking the seek target while a seek is in flight.
func (a *App) pos() float64 {
	if a.seekReq != nil {
		return *a.seekReq
	}
	p, _ := a.clock()
	return a.wrap(p)
}

func (a *App) wrap(p float64) float64 {
	d := a.info.Duration
	if d <= 0 {
		return math.Max(p, 0)
	}
	if a.s.Loop {
		p = math.Mod(p, d)
		if p < 0 {
			p += d
		}
		return p
	}
	return math.Max(0, math.Min(p, d))
}

func (a *App) setPlaying(on bool) {
	if !a.loaded {
		return
	}
	if on && a.ended {
		a.ended = false
		a.seekNow(0)
	}
	p, _ := a.clock()
	a.mu.Lock()
	a.basePos, a.baseWall, a.playing = p, time.Now(), on
	a.mu.Unlock()
	a.dirty = true
}

// --- media ---------------------------------------------------------------

func (a *App) open(idx int, start float64) {
	if len(a.files) == 0 {
		return
	}
	idx = (idx%len(a.files) + len(a.files)) % len(a.files)
	info, err := media.Probe(a.files[idx])
	if err != nil {
		a.say(err.Error(), 4*time.Second)
		if !a.loaded {
			a.openBrowser()
		}
		return
	}
	if m, ok := a.meta[a.files[idx]]; ok {
		if m.name != "" {
			info.Name = m.name
		}
		if m.audio != "" {
			info.AudioPath, info.HasAudio = m.audio, true
		}
	}
	a.fileIdx, a.info, a.loaded = idx, info, true
	a.ended, a.eof, a.seekReq = false, false, nil
	if !a.cfg.LoopSet {
		a.s.Loop = !info.HasAudio && !info.Still
	}
	a.applied = a.s.Playback
	a.shown, a.dropped = 0, 0
	if start < 0 || (info.Duration > 0 && start >= info.Duration) {
		start = 0
	}
	if info.HasAudio && !a.cfg.NoAudio && a.aud == nil && a.audErr == "" {
		aud, err := media.NewAudio(a.clock, a.cfg.AudioSink)
		if err != nil {
			a.audErr = err.Error()
			a.say("no sound: "+a.audErr, 4*time.Second)
		} else {
			a.aud = aud
		}
	}
	a.setClock(start, true, true)
	a.restartVideo(start)
	a.startAudio(start)
	a.dirty = true
}

func (a *App) startAudio(pos float64) {
	if a.aud == nil {
		return
	}
	if !a.info.HasAudio {
		a.aud.Stop()
		return
	}
	a.applyVolume()
	a.aud.Play(a.info.AudioSrc(), pos, a.s.Speed, a.s.Loop)
}

func (a *App) applyVolume() {
	if a.aud == nil {
		return
	}
	v := a.s.Volume
	if a.s.Mute {
		v = 0
	}
	a.aud.SetVolume(v)
	a.aud.SetDelay(a.s.AudioDelay)
}

func (a *App) fps() float64 {
	if a.info.Still {
		return 1
	}
	f := a.info.FPS
	if a.s.FPS > 0 && a.s.FPS < f {
		f = a.s.FPS
	}
	return f
}

func (a *App) aspect() float64 {
	if a.s.CellAspect > 0 {
		return a.s.CellAspect
	}
	if a.cellAR > 0.2 && a.cellAR < 1.5 {
		return a.cellAR
	}
	return 0.5
}

// wantKey works out the decode geometry for the current screen and look.
func (a *App) wantKey() decKey {
	g := FitZoom(a.info.Aspect(), a.s.Look, a.scr.W, a.scr.H, a.aspect(), a.s.Zoom)
	k := decKey{path: a.info.Path, fps: a.fps(), w: g.W, h: g.H, cropX: g.CropX, cropY: g.CropY,
		loop: a.s.Loop && !a.info.Still}
	if g.CropX < 0.999 {
		k.panX = a.s.PanX
	}
	if g.CropY < 0.999 {
		k.panY = a.s.PanY
	}
	return k
}

func (a *App) restartVideo(pos float64) {
	if a.vid != nil {
		a.vid.Close()
		a.vid = nil
	}
	a.pending, a.eof = nil, false
	k := a.wantKey()
	v, err := media.StartVideo(media.VideoOpts{
		Path: k.path, PreInput: a.info.PreInput, Start: a.wrap(pos), Base: pos, W: k.w, H: k.h, FPS: k.fps,
		CropX: k.cropX, CropY: k.cropY, PanX: k.panX, PanY: k.panY, Loop: k.loop, Still: a.info.Still,
		HWAccel: a.cfg.HWAccel,
	})
	if err != nil {
		a.say(err.Error(), 4*time.Second)
		return
	}
	a.vid, a.key, a.needFrame = v, k, true
}

// ensureVideo restarts the decoder when the geometry it was started with no
// longer matches (resize, mode, fit, fps cap…). Sound keeps playing.
func (a *App) ensureVideo() {
	if !a.loaded || a.wantKey() == a.key && a.vid != nil {
		return
	}
	p, _ := a.clock()
	if a.ended {
		p = math.Max(0, a.info.Duration-1/a.fps())
	}
	a.restartVideo(p)
}

func (a *App) seekNow(t float64) {
	if !a.loaded || a.info.Still {
		return
	}
	if d := a.info.Duration; d > 0 {
		t = math.Max(0, math.Min(t, d-0.05))
	} else {
		t = math.Max(0, t)
	}
	a.ended = false
	a.setClock(t, a.isPlaying(), true)
	a.restartVideo(t)
	a.startAudio(t)
	a.lastSeek = time.Now()
	a.seekReq = nil
	a.dirty = true
}

// seek queues a seek; bursts (key repeat, dragging the bar) are coalesced
// so ffmpeg is not restarted more often than it can keep up with.
func (a *App) seek(t float64) {
	if !a.loaded || a.info.Still {
		return
	}
	if d := a.info.Duration; d > 0 {
		t = math.Max(0, math.Min(t, d-0.05))
	}
	t = math.Max(0, t)
	a.seekReq = &t
	a.dirty = true
}

func (a *App) seekBy(d float64) { a.seek(a.pos() + d) }

func (a *App) tryRecv() *media.Frame {
	if a.vid == nil || a.eof {
		return nil
	}
	select {
	case f, ok := <-a.vid.Frames:
		if !ok {
			a.eof = true
			return nil
		}
		return f
	default:
		return nil
	}
}

// tick advances playback: performs queued seeks, shows the frame that is
// due (dropping any that are already late) and handles end of stream.
func (a *App) tick() {
	now := time.Now()
	if a.seekReq != nil && now.Sub(a.lastSeek) > 80*time.Millisecond {
		a.seekNow(*a.seekReq)
	}
	if a.toast != "" && now.After(a.toastUntil) {
		a.toast, a.dirty = "", true
	}
	if a.finder != nil {
		a.finder.tick(a)
	}
	if a.hudVisible() != a.hudShown {
		a.dirty = true
	}
	if a.vid == nil {
		return
	}
	if a.pending == nil {
		a.pending = a.tryRecv()
	}
	pos, running := a.clock()
	for a.pending != nil && (a.needFrame || (running && a.pending.PTS <= pos)) {
		f := a.pending
		a.pending = a.tryRecv()
		if a.pending != nil && running && a.pending.PTS <= pos {
			a.vid.Recycle(f)
			a.dropped++
			continue
		}
		a.show(f)
		break
	}
	if a.eof && a.pending == nil && !a.ended {
		a.ended = true
		if err := a.vid.Err(); err != nil {
			a.say(err.Error(), 5*time.Second)
		}
		if a.needFrame {
			a.needFrame = false
		}
		if !a.info.Still {
			p, _ := a.clock()
			if a.info.Duration > 0 {
				p = a.info.Duration
			}
			a.setClock(p, false, false)
		}
		a.dirty = true
	}
}

func (a *App) show(f *media.Frame) {
	if a.last != nil && a.vid != nil {
		a.vid.Recycle(a.last)
	}
	a.last = f
	a.frameNo++
	a.shown++
	if a.needFrame {
		a.needFrame = false
		a.mu.Lock()
		if a.hold || !a.playing {
			// First frame after a seek (or a frame step while paused):
			// anchor the clock to it so the decoder's start-up time
			// does not count as playback.
			a.basePos, a.baseWall, a.hold = f.PTS, time.Now(), false
		}
		a.mu.Unlock()
	}
	now := time.Now()
	a.fpsCount++
	if d := now.Sub(a.fpsWindow); d >= time.Second {
		a.fpsNow = float64(a.fpsCount) / d.Seconds()
		a.fpsCount, a.fpsWindow = 0, now
	}
	a.renderLast()
}

// renderLast re-renders the most recent frame with the current look.
func (a *App) renderLast() {
	f := a.last
	if f == nil {
		return
	}
	sx, sy := engine.SubCells(a.s.Mode)
	cols, rows := f.W/sx, f.H/sy
	if cols*sx != f.W || rows*sy != f.H {
		return // decoded for another mode; a matching frame is on its way
	}
	if len(a.cells) != cols*rows {
		a.cells = make([]engine.Cell, cols*rows)
	}
	a.ccols, a.crows = cols, rows
	t := time.Now()
	a.rend.Render(f.Pix, f.W, f.H, &a.s.Look, a.cells, cols, rows, a.frameNo)
	ms := float64(time.Since(t).Microseconds()) / 1000
	a.renderMs += (ms - a.renderMs) * 0.1
	a.dirty = true
}

// changed is called after any setting was modified.
func (a *App) changed() {
	pb := a.s.Playback
	if a.loaded && (pb.Speed != a.applied.Speed || pb.Loop != a.applied.Loop) {
		// Both change how the timeline maps to the file, so re-anchor
		// the clock and restart sound and picture from the same spot.
		p, _ := a.clock()
		p = a.wrap(p)
		a.mu.Lock()
		a.basePos, a.baseWall, a.speed = p, time.Now(), pb.Speed
		a.mu.Unlock()
		a.startAudio(p)
		a.key = decKey{}
	}
	a.applyVolume()
	a.applied = pb
	a.ensureVideo()
	a.renderLast()
	a.dirty = true
}

func (a *App) resize() {
	cols, rows, ar := a.term.Size()
	a.cellAR = ar
	a.scr.Resize(cols, rows)
	a.cells = nil
	a.ensureVideo()
	a.dirty = true
}

func (a *App) nextWake() time.Duration {
	d := 250 * time.Millisecond
	near := func(t time.Duration) {
		if t < d {
			d = t
		}
	}
	if a.pending != nil {
		if pos, running := a.clock(); running {
			near(time.Duration((a.pending.PTS - pos) / a.speedNow() * float64(time.Second)))
		} else if a.needFrame {
			near(0)
		}
	}
	if a.seekReq != nil {
		near(80*time.Millisecond - time.Since(a.lastSeek))
	}
	if a.toast != "" {
		near(time.Until(a.toastUntil))
	}
	if a.hud == "auto" && a.hudShown {
		near(time.Until(a.lastActivity.Add(2500*time.Millisecond)) + 10*time.Millisecond)
	}
	if a.eof && !a.ended {
		near(0)
	}
	if a.finder != nil {
		near(a.finder.wake())
	}
	if d < 0 {
		d = 0
	}
	return d
}

func (a *App) speedNow() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.speed <= 0 {
		return 1
	}
	return a.speed
}

// --- look changes ----------------------------------------------------------

func (a *App) say(msg string, d time.Duration) {
	a.toast, a.toastUntil, a.dirty = msg, time.Now().Add(d), true
}

func (a *App) pushHistory() {
	a.history = append(a.history, a.s.Look)
	if len(a.history) > 100 {
		a.history = a.history[1:]
	}
}

func (a *App) undo() {
	if len(a.history) == 0 {
		a.say("nothing to undo", time.Second)
		return
	}
	a.s.Look = a.history[len(a.history)-1]
	a.history = a.history[:len(a.history)-1]
	a.s.Look.Resample++
	a.changed()
	a.say("undo → "+Summary(a.s.Look), 1500*time.Millisecond)
}

func (a *App) randomize(paletteOnly bool) {
	a.pushHistory()
	if paletteOnly {
		engine.RandomPalette(&a.s.Look, a.rng)
	} else {
		engine.Randomize(&a.s.Look, a.rng)
	}
	a.preset = ""
	a.changed()
	a.say("random → "+Summary(a.s.Look)+"   (u = undo)", 2500*time.Millisecond)
}

// nudge steps one option and reports the new value.
func (a *App) nudge(key string, dir int) {
	o := engine.FindOption(key)
	if o == nil {
		return
	}
	if o.Group != "Playback" {
		a.touch()
	}
	o.Nudge(&a.s, dir)
	a.changed()
	a.say(o.Label+": "+o.String(&a.s), 1200*time.Millisecond)
}

func (a *App) loadPreset(p engine.Preset) {
	a.pushHistory()
	a.s.Look = p.Look
	a.s.Look.Resample = a.frameNo
	a.preset = p.Name
	a.changed()
	a.say("preset: "+p.Name+" — "+Summary(p.Look), 2*time.Second)
}

func (a *App) cyclePreset(dir int) {
	all := a.store.All()
	if len(all) == 0 {
		return
	}
	idx := -1
	for i, p := range all {
		if p.Name == a.preset {
			idx = i
		}
	}
	if idx < 0 && dir < 0 {
		idx = 0
	}
	a.loadPreset(all[((idx+dir)%len(all)+len(all))%len(all)])
}

func (a *App) title() string {
	if !a.loaded {
		return "termo"
	}
	return tty.Clean(a.info.Name)
}
