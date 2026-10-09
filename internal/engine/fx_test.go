package engine

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"
)

// moving returns a test picture that differs from frame to frame: a color
// gradient with a bright bar that travels across it.
func moving(w, h, frame int) []byte {
	pix := make([]byte, w*h*3)
	bar := (frame * 3) % w
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := pix[(y*w+x)*3:]
			p[0] = uint8(x * 255 / max(1, w-1))
			p[1] = uint8(y * 255 / max(1, h-1))
			p[2] = uint8((x + y + frame*5) * 255 / (w + h + 40) % 256)
			if x >= bar && x < bar+3 {
				p[0], p[1], p[2] = 255, 250, 240
			}
		}
	}
	return pix
}

// film renders a few frames of the moving picture with a fresh renderer.
func film(t testing.TB, l Look, cols, rows, frames int) [][]Cell {
	sx, sy := SubCells(l.Mode)
	w, h := cols*sx, rows*sy
	var r Renderer
	r.TermDepth = 24
	out := make([][]Cell, frames)
	for f := range out {
		cells := make([]Cell, cols*rows)
		r.Render(moving(w, h, f), w, h, &l, cells, cols, rows, f)
		for i, c := range cells {
			if c.Ch == 0 {
				t.Fatalf("frame %d: cell %d not written", f, i)
			}
		}
		out[f] = cells
	}
	return out
}

func TestToneCurveIdentity(t *testing.T) {
	var r Renderer
	l := DefaultLook()
	r.buildTone(&l)
	for ch := 0; ch < 3; ch++ {
		for i := 0; i < 256; i++ {
			if int(r.tone[ch][i]) != i {
				t.Fatalf("default look changes channel %d: %d -> %d", ch, i, r.tone[ch][i])
			}
		}
	}
}

func TestToneControls(t *testing.T) {
	curve := func(edit func(*Look)) [3][256]uint8 {
		var r Renderer
		l := DefaultLook()
		edit(&l)
		r.buildTone(&l)
		return r.tone
	}
	if c := curve(func(l *Look) { l.Exposure = 1 }); c[0][64] < 120 || c[0][64] > 136 {
		t.Errorf("one stop of exposure should double 64, got %d", c[0][64])
	}
	if c := curve(func(l *Look) { l.Black, l.White = 0.25, 0.75 }); c[1][63] != 0 || c[1][192] != 255 || c[1][128] < 120 || c[1][128] > 136 {
		t.Errorf("levels: 63->%d 128->%d 192->%d", c[1][63], c[1][128], c[1][192])
	}
	// Shadows move the dark end and leave the bright end nearly alone.
	c := curve(func(l *Look) { l.Shadows = 1 })
	if c[0][50] <= 60 || c[0][230] > 236 || c[0][0] != 0 || c[0][255] != 255 {
		t.Errorf("shadows: 0->%d 50->%d 230->%d 255->%d", c[0][0], c[0][50], c[0][230], c[0][255])
	}
	c = curve(func(l *Look) { l.Highlights = -1 })
	if c[0][200] >= 195 || c[0][30] < 24 || c[0][255] != 255 {
		t.Errorf("highlights: 30->%d 200->%d 255->%d", c[0][30], c[0][200], c[0][255])
	}
	if c := curve(func(l *Look) { l.Fade = 1 }); c[0][0] < 40 || c[0][255] > 235 {
		t.Errorf("fade should lift black and lower white, got %d..%d", c[0][0], c[0][255])
	}
	c = curve(func(l *Look) { l.Temperature = 1 })
	if c[0][128] <= 128 || c[2][128] >= 128 {
		t.Errorf("warm should raise red (%d) and lower blue (%d)", c[0][128], c[2][128])
	}
	// A white point at or below the black point must not divide by zero.
	c = curve(func(l *Look) { l.Black, l.White = 0.5, 0.5 })
	if c[0][0] != 0 || c[0][255] != 255 {
		t.Errorf("degenerate levels: %d..%d", c[0][0], c[0][255])
	}
}

func TestDefaultFXIsOff(t *testing.T) {
	f := DefaultFX()
	if f.Active() {
		t.Fatal("the default effects must be inactive")
	}
	if DefaultLook().FX != f {
		t.Fatal("the default look must carry the default effects")
	}
	if FXSummary(f) != "none" {
		t.Errorf("summary of nothing = %q", FXSummary(f))
	}
	if BuiltinFX[0].Name != "none" || BuiltinFX[0].FX != f {
		t.Error(`the first effect preset must be "none"`)
	}
}

// every sets each field of the effects to a value that is not its default.
func everyFX() FX {
	f := DefaultFX()
	v := reflect.ValueOf(&f).Elem()
	for i := 0; i < v.NumField(); i++ {
		switch fl := v.Field(i); fl.Kind() {
		case reflect.Float64:
			fl.SetFloat(fl.Float() + 0.25)
		case reflect.Int:
			fl.SetInt(fl.Int() + 3)
		case reflect.Bool:
			fl.SetBool(true)
		}
	}
	f.Filter, f.Mirror, f.Mask, f.SplitMode = "sepia", "quad", "slot", "phase"
	return f
}

func TestLookJSON(t *testing.T) {
	// No two fields of a look may share a JSON name: the embedded effects
	// would silently lose theirs.
	seen := map[string]string{}
	var walk func(reflect.Type)
	walk = func(tp reflect.Type) {
		for i := 0; i < tp.NumField(); i++ {
			f := tp.Field(i)
			if f.Anonymous {
				walk(f.Type)
				continue
			}
			tag := f.Tag.Get("json")
			if tag == "-" {
				continue
			}
			if tag == "" {
				t.Errorf("%s has no json name", f.Name)
			}
			if prev, dup := seen[tag]; dup {
				t.Errorf("%s and %s share the json name %q", prev, f.Name, tag)
			}
			seen[tag] = f.Name
		}
	}
	walk(reflect.TypeOf(Look{}))

	l := DefaultLook()
	l.FX = everyFX()
	l.Exposure, l.Black, l.White, l.Shadows, l.Highlights = 0.5, 0.1, 0.9, 0.2, -0.3
	l.Fade, l.Vibrance, l.Temperature, l.Tint = 0.4, 0.5, -0.6, 0.7
	data, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	back := DefaultLook()
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(l, back) {
		t.Errorf("round trip changed the look:\n%+v\n%+v", l, back)
	}

	// A look saved before the effects existed keeps them all off.
	old := DefaultLook()
	if err := json.Unmarshal([]byte(`{"mode":"ascii","contrast":1.2}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.White != 1 || old.FX != DefaultFX() || old.Contrast != 1.2 {
		t.Errorf("old look decoded to %+v", old)
	}
}

// inRange checks every option of a look against its own limits.
func inRange(t *testing.T, what string, l Look) {
	t.Helper()
	s := DefaultSettings()
	s.Look = l
	for _, o := range Options {
		if o.Group == "Playback" || o.Kind == KText || o.Kind == KList {
			continue
		}
		if err := o.Set(&s, o.String(&s)); err != nil {
			t.Errorf("%s: invalid %s: %v", what, o.Key, err)
		}
	}
	if s.Look.FX != l.FX {
		t.Errorf("%s: setting every option to its own value changed the effects:\n%+v\n%+v", what, l.FX, s.Look.FX)
	}
}

func TestBuiltinFX(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range BuiltinFX {
		if seen[p.Name] || p.Name == "" || !p.Builtin {
			t.Errorf("effect preset %q: duplicate, unnamed or not marked built-in", p.Name)
		}
		seen[p.Name] = true
		if p.Name != "none" && !p.FX.Active() && p.FX.HueCycle == 0 {
			t.Errorf("%s does nothing", p.Name)
		}
		l := DefaultLook()
		l.FX = p.FX
		inRange(t, p.Name, l)
		a, b := film(t, l, 24, 10, 5), film(t, l, 24, 10, 5)
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s: the same frames rendered differently twice", p.Name)
		}
		if p.Name != "none" {
			if plain := film(t, DefaultLook(), 24, 10, 5); reflect.DeepEqual(a, plain) {
				t.Errorf("%s: the picture is unchanged", p.Name)
			}
		}
	}
}

func TestEveryEffectChangesThePicture(t *testing.T) {
	// Wide enough for the effects that move lines by a fraction of the width.
	const cols, rows = 96, 12
	plain := film(t, DefaultLook(), cols, rows, 6)
	for _, o := range Options {
		if o.Group != "Filter" && o.Group != "Effects" && o.Group != "Adjust" {
			continue
		}
		if o.Key == "hue-shift" || o.Key == "edge-threshold" {
			continue // a full turn of hue is no change; the threshold needs edges
		}
		// Settings that only shape another effect are tested with it on.
		base := DefaultSettings()
		switch o.Key {
		case "filter-amount":
			continue // covered below
		case "vignette-size":
			base.Vignette = 1
		case "scanline-gap":
			base.Scanlines = 1
		case "split-mode":
			base.Split = 2
		case "wave-freq":
			base.Wave = 0.5
		}
		ref := plain
		if base.FX != DefaultFX() {
			ref = film(t, base.Look, cols, rows, 6)
		}
		var values []string
		switch o.Kind {
		case KEnum:
			for _, c := range o.Choices {
				if c != o.String(&base) {
					values = append(values, c)
				}
			}
		case KBool:
			values = []string{"true"}
		default:
			s := base
			o.SetFrac(&s, 1)
			if o.String(&s) == o.String(&base) {
				o.SetFrac(&s, 0)
			}
			values = []string{o.String(&s)}
			if o.Min < 0 { // two-sided: try the other end as well
				o.SetFrac(&s, 0)
				values = append(values, o.String(&s))
			}
		}
		for _, v := range values {
			s := base
			if err := o.Set(&s, v); err != nil {
				t.Fatalf("%s=%s: %v", o.Key, v, err)
			}
			if got := film(t, s.Look, cols, rows, 6); reflect.DeepEqual(got, ref) {
				t.Errorf("%s=%s leaves the picture unchanged", o.Key, v)
			}
		}
	}
	// A filter at amount 0 is the original picture.
	l := DefaultLook()
	l.Filter, l.FilterAmount = "sepia", 0
	if !reflect.DeepEqual(film(t, l, cols, rows, 6), plain) {
		t.Error("filter-amount 0 should leave the picture alone")
	}
	for _, name := range FilterNames()[1:] {
		if compileGrade(name) == nil {
			t.Errorf("filter %q does not compile", name)
		}
	}
	if compileGrade("no-such-filter") != nil {
		t.Error("unknown filters should compile to nothing")
	}
	l.Filter, l.FilterAmount = "no-such-filter", 1
	film(t, l, 24, 10, 2) // must not crash
}

func TestFilterLooks(t *testing.T) {
	// One mid-grey-ish orange pixel through a few classics.
	grade := func(name string) (r, g, b uint8) {
		l := DefaultLook()
		l.Filter = name
		pix := []byte{200, 120, 40, 200, 120, 40}
		var rn Renderer
		rn.TermDepth = 24
		cells := make([]Cell, 1)
		rn.Render(pix, 1, 2, &l, cells, 1, 1, 0)
		c := FromU32(cells[0].Bg) // both halves alike: a space on a background
		return c.R, c.G, c.B
	}
	if r, g, b := grade("mono"); r != g || g != b {
		t.Errorf("mono is not grey: %d %d %d", r, g, b)
	}
	if r, g, b := grade("sepia"); !(r > g && g > b) {
		t.Errorf("sepia is not brown: %d %d %d", r, g, b)
	}
	if r, g, b := grade("negative"); r != 55 || g != 135 || b != 215 {
		t.Errorf("negative: %d %d %d", r, g, b)
	}
	if r, g, b := grade("night-vision"); !(g > r && g > b) {
		t.Errorf("night vision is not green: %d %d %d", r, g, b)
	}
}

func TestPosterizeLevels(t *testing.T) {
	l := DefaultLook()
	l.Posterize = 3
	levels := map[uint8]bool{}
	for _, cells := range film(t, l, 24, 10, 2) {
		for _, c := range cells {
			colors := []RGB{FromU32(c.Bg)}
			if c.Ch != ' ' {
				colors = append(colors, FromU32(c.Fg))
			}
			for _, v := range colors {
				levels[v.R], levels[v.G], levels[v.B] = true, true, true
			}
		}
	}
	for v := range levels {
		if v != 0 && v != 127 && v != 255 {
			t.Fatalf("posterize 3 produced level %d (have %v)", v, levels)
		}
	}
	if len(levels) != 3 {
		t.Errorf("want 3 levels, got %v", levels)
	}
}

func TestMirrorIsSymmetric(t *testing.T) {
	l := DefaultLook()
	l.Mirror = "left"
	const cols, rows = 24, 10
	cells := film(t, l, cols, rows, 1)[0]
	for y := 0; y < rows; y++ {
		for x := 0; x < cols/2; x++ {
			if cells[y*cols+x] != cells[y*cols+cols-1-x] {
				t.Fatalf("row %d: column %d is not mirrored", y, x)
			}
		}
	}
}

// TestFXSurviveResize renders consecutive frames at changing sizes with
// everything turned up: the buffers that carry state between frames must
// cope with a picture that is zoomed while it plays.
func TestFXSurviveResize(t *testing.T) {
	s := DefaultSettings()
	for _, o := range Options {
		if o.Group != "Filter" && o.Group != "Effects" {
			continue
		}
		switch o.Kind {
		case KEnum:
			o.Set(&s, o.Choices[len(o.Choices)-1])
		case KBool:
			o.Set(&s, "true")
		default:
			o.SetFrac(&s, 1)
		}
	}
	var r Renderer
	r.TermDepth = 24
	frame := 0
	for _, mode := range Modes {
		s.Mode = mode
		sx, sy := SubCells(mode)
		for _, size := range [][2]int{{40, 12}, {7, 3}, {1, 1}, {80, 30}, {2, 9}, {33, 1}, {40, 12}} {
			cols, rows := size[0], size[1]
			w, h := cols*sx, rows*sy
			cells := make([]Cell, cols*rows)
			for i := 0; i < 3; i++ {
				r.Render(moving(w, h, frame), w, h, &s.Look, cells, cols, rows, frame)
				frame++
			}
			for i, c := range cells {
				if c.Ch == 0 {
					t.Fatalf("%s %dx%d: cell %d not written", mode, cols, rows, i)
				}
			}
		}
	}
}

func TestRandomFX(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 400; i++ {
		f := RandomFX(rng)
		if !f.Active() && f.HueCycle == 0 {
			t.Fatalf("roll %d does nothing: %+v", i, f)
		}
		l := DefaultLook()
		l.FX = f
		inRange(t, "random effects", l)
		if t.Failed() {
			t.Fatalf("roll %d: %+v", i, f)
		}
		if i < 40 {
			film(t, l, 12, 5, 3)
		}
	}
}

func BenchmarkFX(b *testing.B) {
	const cols, rows = 240, 67
	for _, p := range BuiltinFX {
		switch p.Name {
		case "none", "vhs-worn", "crt-tv", "crt-arcade", "glitch-heavy", "dream", "night-vision", "kaleidoscope", "old-film":
		default:
			continue
		}
		b.Run(p.Name, func(b *testing.B) {
			l := DefaultLook()
			l.FX = p.FX
			w, h := cols, rows*2
			pix := moving(w, h, 0)
			var r Renderer
			r.TermDepth = 24
			cells := make([]Cell, cols*rows)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.Render(pix, w, h, &l, cells, cols, rows, i)
			}
		})
	}
}
