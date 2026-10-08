package engine

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
)

// Look is everything that decides how a frame is drawn. It is what a preset
// stores.
type Look struct {
	Mode       string `json:"mode"`
	Charset    string `json:"charset"`
	Chars      string `json:"chars,omitempty"`
	Color      bool   `json:"color"`
	Background string `json:"background"`
	Fit        string `json:"fit"`
	FlipX      bool   `json:"flip_x"`
	FlipY      bool   `json:"flip_y"`

	Palette string   `json:"palette"`
	Colors  int      `json:"colors"`
	Scheme  string   `json:"scheme"`
	Hue     float64  `json:"hue"`
	Chroma  float64  `json:"chroma"`
	LMin    float64  `json:"lmin"`
	LMax    float64  `json:"lmax"`
	Custom  []string `json:"custom,omitempty"`

	Dither       string  `json:"dither"`
	DitherAmount float64 `json:"dither_amount"`
	Serpentine   bool    `json:"serpentine"`

	Brightness    float64 `json:"brightness"`
	Contrast      float64 `json:"contrast"`
	Gamma         float64 `json:"gamma"`
	Saturation    float64 `json:"saturation"`
	HueShift      float64 `json:"hue_shift"`
	Invert        bool    `json:"invert"`
	Edges         string  `json:"edges"`
	EdgeThreshold float64 `json:"edge_threshold"`

	// Resample is bumped to make the adaptive palette re-read the picture.
	Resample int `json:"-"`
}

// Playback holds per-session settings that presets do not touch.
type Playback struct {
	FPS        float64
	Speed      float64
	Volume     float64
	Mute       bool
	Loop       bool
	CellAspect float64
	AudioDelay float64

	// Zoom is the size of the picture relative to the terminal: below 1 it
	// shrinks, above 1 it grows and is cropped once it fills the screen.
	// It is independent of the interface size.
	Zoom       float64
	PanX, PanY float64 // -1..1: which part of a cropped picture is shown
}

// Settings is the full set of user-adjustable state.
type Settings struct {
	Look
	Playback
	Sound Sound
}

// Render modes.
const (
	ModeHalf    = "half"
	ModeQuad    = "quad"
	ModeSextant = "sextant"
	ModeBraille = "braille"
	ModeASCII   = "ascii"
)

// Modes lists the render modes in menu order.
var Modes = []string{ModeHalf, ModeQuad, ModeSextant, ModeBraille, ModeASCII}

// SubCells returns how many source pixels one terminal cell covers.
func SubCells(mode string) (sx, sy int) {
	switch mode {
	case ModeHalf:
		return 1, 2
	case ModeQuad:
		return 2, 2
	case ModeSextant:
		return 2, 3
	case ModeBraille:
		return 2, 4
	}
	return 1, 1
}

// Charset is a brightness ramp, darkest glyph first.
type Charset struct {
	Name  string
	Runes []rune
}

// Charsets lists the built-in ASCII ramps.
var Charsets = []Charset{
	{"standard", []rune(" .:-=+*#%@")},
	{"detailed", []rune(" .'`^\",:;Il!i><~+_-?][}{1)(|\\/tfjrxnuvczXYUJCLQ0OZmwqpdbkhao*#MW&8%B@$")},
	{"minimal", []rune(" .oO@")},
	{"blocks", []rune(" ░▒▓█")},
	{"dots", []rune(" ⠁⠃⠇⠏⠟⠿⡿⣿")},
	{"lines", []rune(" ˙-~=≡#")},
	{"slashes", []rune(" ./\\|X#")},
	{"binary", []rune(" 01")},
	{"katakana", []rune(" ･ｰｧｨｩｱｲｳｴｵｶｷｸｹｻｼｽｾﾀﾁﾂﾃﾄﾅﾆﾇﾈﾊﾋﾌﾍﾎﾏﾐﾑﾒﾓﾔﾕﾖﾗﾘﾙﾚﾛﾜﾝ")},
	{"custom", nil},
}

func charsetNames() []string {
	all := AllCharsets()
	out := make([]string, len(all))
	for i, c := range all {
		out[i] = c.Name
	}
	return out
}

// Ramp returns the glyph ramp selected by the look.
func (l *Look) Ramp() []rune {
	if l.Charset == "custom" {
		if r := []rune(l.Chars); len(r) >= 2 {
			return r
		}
		return Charsets[0].Runes
	}
	for _, c := range AllCharsets() {
		if c.Name == l.Charset && len(c.Runes) >= 2 {
			return c.Runes
		}
	}
	return Charsets[0].Runes
}

// DefaultLook is the look termo starts with.
func DefaultLook() Look {
	return Look{
		Mode: ModeHalf, Charset: "standard", Color: true, Background: "default", Fit: "fit",
		Palette: PalOff, Colors: 8, Scheme: "analogous", Hue: 200, Chroma: 0.14, LMin: 0.1, LMax: 0.95,
		Dither: "bayer4", DitherAmount: 1, Serpentine: true,
		Contrast: 1, Gamma: 1, Saturation: 1, Edges: "off", EdgeThreshold: 0.15,
	}
}

// DefaultSettings returns the default look and playback settings.
func DefaultSettings() Settings {
	return Settings{Look: DefaultLook(), Playback: Playback{Speed: 1, Volume: 1, Zoom: 1}, Sound: DefaultSound()}
}

// Kind is the value type of an Option.
type Kind int

// Option kinds.
const (
	KEnum Kind = iota
	KInt
	KFloat
	KBool
	KText
	KList
)

// Option describes one setting. The same table drives the command-line
// flags, the settings menu and the randomiser.
type Option struct {
	Key, Label, Group, Help string
	Kind                    Kind
	Choices                 []string
	Min, Max, Step          float64
	Wrap                    bool
	ptr                     func(*Settings) any
	Active                  func(*Settings) bool
}

func isHarmony(s *Settings) bool { return s.Palette == PalHarmony }
func usesCount(s *Settings) bool {
	switch s.Palette {
	case PalHarmony, PalGray, PalCube, PalAdaptive:
		return true
	}
	return false
}
func usesDither(s *Settings) bool { return s.Dither != "none" }

// Options is the table of every setting, in menu order.
var Options = []*Option{
	{Key: "mode", Label: "Mode", Group: "Render", Kind: KEnum, Choices: Modes,
		Help: "glyph set: half blocks, quadrants, sextants, braille dots or ASCII",
		ptr:  func(s *Settings) any { return &s.Mode }},
	{Key: "charset", Label: "Charset", Group: "Render", Kind: KEnum, Choices: charsetNames(),
		Help:   "brightness ramp used by ascii mode",
		ptr:    func(s *Settings) any { return &s.Charset },
		Active: func(s *Settings) bool { return s.Mode == ModeASCII }},
	{Key: "chars", Label: "Custom chars", Group: "Render", Kind: KText,
		Help:   "custom ramp, darkest first (used when charset=custom)",
		ptr:    func(s *Settings) any { return &s.Chars },
		Active: func(s *Settings) bool { return s.Mode == ModeASCII && s.Charset == "custom" }},
	{Key: "background", Label: "Background", Group: "Render", Kind: KEnum, Choices: []string{"default", "black", "tint"},
		Help:   "cell background in ascii/braille modes",
		ptr:    func(s *Settings) any { return &s.Background },
		Active: func(s *Settings) bool { return s.Mode == ModeASCII || s.Mode == ModeBraille }},
	{Key: "fit", Label: "Fit", Group: "Render", Kind: KEnum, Choices: []string{"fit", "fill", "stretch"},
		Help: "fit keeps aspect, fill crops, stretch ignores aspect",
		ptr:  func(s *Settings) any { return &s.Fit }},
	{Key: "flip-x", Label: "Flip X", Group: "Render", Kind: KBool, Help: "mirror horizontally",
		ptr: func(s *Settings) any { return &s.FlipX }},
	{Key: "flip-y", Label: "Flip Y", Group: "Render", Kind: KBool, Help: "mirror vertically",
		ptr: func(s *Settings) any { return &s.FlipY }},

	{Key: "color", Label: "Color", Group: "Color", Kind: KBool,
		Help: "off = 1-bit art in the terminal's own colors",
		ptr:  func(s *Settings) any { return &s.Color }},
	{Key: "palette", Label: "Palette", Group: "Color", Kind: KEnum, Choices: PaletteChoices(),
		Help: "truecolor, a generated palette, or a named one",
		ptr:  func(s *Settings) any { return &s.Palette }},
	{Key: "colors", Label: "Colors", Group: "Color", Kind: KInt, Min: 1, Max: 256, Step: 1,
		Help: "palette size for harmony/adaptive/gray/cube",
		ptr:  func(s *Settings) any { return &s.Colors }, Active: usesCount},
	{Key: "scheme", Label: "Scheme", Group: "Color", Kind: KEnum, Choices: Schemes,
		Help: "color-theory scheme for the harmony palette",
		ptr:  func(s *Settings) any { return &s.Scheme }, Active: isHarmony},
	{Key: "hue", Label: "Base hue", Group: "Color", Kind: KFloat, Min: 0, Max: 360, Step: 5, Wrap: true,
		Help: "base hue of the harmony palette, degrees",
		ptr:  func(s *Settings) any { return &s.Hue }, Active: isHarmony},
	{Key: "chroma", Label: "Chroma", Group: "Color", Kind: KFloat, Min: 0, Max: 0.37, Step: 0.01,
		Help: "colorfulness of the harmony palette",
		ptr:  func(s *Settings) any { return &s.Chroma }, Active: isHarmony},
	{Key: "lmin", Label: "Darkest", Group: "Color", Kind: KFloat, Min: 0, Max: 1, Step: 0.01,
		Help: "lightness of the darkest harmony color",
		ptr:  func(s *Settings) any { return &s.LMin }, Active: isHarmony},
	{Key: "lmax", Label: "Lightest", Group: "Color", Kind: KFloat, Min: 0, Max: 1, Step: 0.01,
		Help: "lightness of the lightest harmony color",
		ptr:  func(s *Settings) any { return &s.LMax }, Active: isHarmony},
	{Key: "custom", Label: "Custom colors", Group: "Color", Kind: KList,
		Help:   "comma-separated hex colors (used when palette=custom)",
		ptr:    func(s *Settings) any { return &s.Custom },
		Active: func(s *Settings) bool { return s.Palette == PalCustom }},

	{Key: "dither", Label: "Dither", Group: "Dither", Kind: KEnum, Choices: DitherNames(),
		Help: "ordered patterns are rock-steady; error diffusion may shimmer on video",
		ptr:  func(s *Settings) any { return &s.Dither }},
	{Key: "dither-amount", Label: "Amount", Group: "Dither", Kind: KFloat, Min: 0, Max: 2, Step: 0.05,
		Help: "dither strength", ptr: func(s *Settings) any { return &s.DitherAmount }, Active: usesDither},
	{Key: "serpentine", Label: "Serpentine", Group: "Dither", Kind: KBool,
		Help:   "alternate scan direction for error diffusion",
		ptr:    func(s *Settings) any { return &s.Serpentine },
		Active: func(s *Settings) bool { return IsDiffusion(s.Dither) }},

	{Key: "brightness", Label: "Brightness", Group: "Adjust", Kind: KFloat, Min: -1, Max: 1, Step: 0.02,
		ptr: func(s *Settings) any { return &s.Brightness }},
	{Key: "contrast", Label: "Contrast", Group: "Adjust", Kind: KFloat, Min: 0, Max: 3, Step: 0.05,
		ptr: func(s *Settings) any { return &s.Contrast }},
	{Key: "gamma", Label: "Gamma", Group: "Adjust", Kind: KFloat, Min: 0.2, Max: 3, Step: 0.05,
		ptr: func(s *Settings) any { return &s.Gamma }},
	{Key: "saturation", Label: "Saturation", Group: "Adjust", Kind: KFloat, Min: 0, Max: 3, Step: 0.05,
		ptr: func(s *Settings) any { return &s.Saturation }},
	{Key: "hue-shift", Label: "Hue shift", Group: "Adjust", Kind: KFloat, Min: 0, Max: 360, Step: 5, Wrap: true,
		Help: "rotate source hues, degrees", ptr: func(s *Settings) any { return &s.HueShift }},
	{Key: "invert", Label: "Invert", Group: "Adjust", Kind: KBool,
		ptr: func(s *Settings) any { return &s.Invert }},
	{Key: "edges", Label: "Edges", Group: "Adjust", Kind: KEnum, Choices: []string{"off", "mono", "color"},
		Help: "Sobel edge detection", ptr: func(s *Settings) any { return &s.Edges }},
	{Key: "edge-threshold", Label: "Edge threshold", Group: "Adjust", Kind: KFloat, Min: 0, Max: 1, Step: 0.01,
		ptr:    func(s *Settings) any { return &s.EdgeThreshold },
		Active: func(s *Settings) bool { return s.Edges != "off" }},

	{Key: "zoom", Label: "Picture size", Group: "Playback", Kind: KFloat, Min: 0.1, Max: 8, Step: 0.05,
		Help: "size of the picture only, not of the interface (1 = fit the terminal)",
		ptr:  func(s *Settings) any { return &s.Zoom }},
	{Key: "pan-x", Label: "Pan X", Group: "Playback", Kind: KFloat, Min: -1, Max: 1, Step: 0.05,
		Help: "which part of a cropped picture is shown, left to right",
		ptr:  func(s *Settings) any { return &s.PanX }},
	{Key: "pan-y", Label: "Pan Y", Group: "Playback", Kind: KFloat, Min: -1, Max: 1, Step: 0.05,
		Help: "which part of a cropped picture is shown, top to bottom",
		ptr:  func(s *Settings) any { return &s.PanY }},
	{Key: "fps", Label: "FPS cap", Group: "Playback", Kind: KFloat, Min: 0, Max: 240, Step: 5,
		Help: "frame rate to render at (0 = source rate)", ptr: func(s *Settings) any { return &s.FPS }},
	{Key: "speed", Label: "Speed", Group: "Playback", Kind: KFloat, Min: 0.25, Max: 4, Step: 0.25,
		ptr: func(s *Settings) any { return &s.Speed }},
	{Key: "volume", Label: "Volume", Group: "Playback", Kind: KFloat, Min: 0, Max: 1.5, Step: 0.05,
		ptr: func(s *Settings) any { return &s.Volume }},
	{Key: "mute", Label: "Mute", Group: "Playback", Kind: KBool,
		ptr: func(s *Settings) any { return &s.Mute }},
	{Key: "loop", Label: "Loop", Group: "Playback", Kind: KBool,
		Help: "default: on for clips without audio", ptr: func(s *Settings) any { return &s.Loop }},
	{Key: "audio-delay", Label: "Audio delay", Group: "Playback", Kind: KFloat, Min: -1, Max: 1, Step: 0.01,
		Help: "seconds to delay sound by (negative = earlier) to fix lip-sync",
		ptr:  func(s *Settings) any { return &s.AudioDelay }},
	{Key: "cell-aspect", Label: "Cell aspect", Group: "Playback", Kind: KFloat, Min: 0, Max: 1.5, Step: 0.01,
		Help: "width/height of one terminal cell (0 = auto-detect)",
		ptr:  func(s *Settings) any { return &s.CellAspect }},
}

// Groups lists option groups in menu order.
var Groups = []string{"Render", "Color", "Dither", "Adjust", "Playback"}

// FindOption looks an option up by its key.
func FindOption(key string) *Option {
	for _, o := range Options {
		if o.Key == key {
			return o
		}
	}
	return nil
}

// IsActive reports whether the option has any effect with the current settings.
func (o *Option) IsActive(s *Settings) bool { return o.Active == nil || o.Active(s) }

func trimFloat(f float64) string {
	return strconv.FormatFloat(math.Round(f*1000)/1000, 'f', -1, 64)
}

// String formats the option's current value.
func (o *Option) String(s *Settings) string {
	switch p := o.ptr(s).(type) {
	case *string:
		return *p
	case *int:
		return strconv.Itoa(*p)
	case *float64:
		return trimFloat(*p)
	case *bool:
		if *p {
			return "on"
		}
		return "off"
	case *[]string:
		return strings.Join(*p, ",")
	}
	return ""
}

func (o *Option) clamp(v float64) float64 {
	if o.Wrap {
		span := o.Max - o.Min
		v = math.Mod(v-o.Min, span)
		if v < 0 {
			v += span
		}
		return v + o.Min
	}
	return math.Max(o.Min, math.Min(o.Max, v))
}

// Set parses and stores a value.
func (o *Option) Set(s *Settings, v string) error {
	switch p := o.ptr(s).(type) {
	case *string:
		if o.Kind == KEnum {
			for _, c := range o.Choices {
				if strings.EqualFold(c, v) {
					*p = c
					return nil
				}
			}
			return fmt.Errorf("--%s: unknown value %q (choices: %s)", o.Key, v, strings.Join(o.Choices, ", "))
		}
		*p = v
	case *int:
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("--%s: %q is not a whole number", o.Key, v)
		}
		*p = int(o.clamp(float64(n)))
	case *float64:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("--%s: %q is not a number", o.Key, v)
		}
		if !o.Wrap && (f < o.Min || f > o.Max) {
			return fmt.Errorf("--%s: %s is out of range %s..%s", o.Key, v, trimFloat(o.Min), trimFloat(o.Max))
		}
		*p = o.clamp(f)
	case *bool:
		switch strings.ToLower(v) {
		case "1", "true", "on", "yes", "":
			*p = true
		case "0", "false", "off", "no":
			*p = false
		default:
			return fmt.Errorf("--%s: %q is not on/off", o.Key, v)
		}
	case *[]string:
		var out []string
		for _, f := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
			c, err := ParseHex(f)
			if err != nil {
				return fmt.Errorf("--%s: %v", o.Key, err)
			}
			out = append(out, c.Hex())
		}
		*p = out
	}
	return nil
}

// Nudge moves the value by dir steps: enums cycle, numbers step, bools flip.
func (o *Option) Nudge(s *Settings, dir int) {
	switch p := o.ptr(s).(type) {
	case *string:
		if o.Kind != KEnum {
			return
		}
		idx := 0
		for i, c := range o.Choices {
			if c == *p {
				idx = i
			}
		}
		n := len(o.Choices)
		*p = o.Choices[((idx+dir)%n+n)%n]
	case *int:
		*p = int(o.clamp(float64(*p) + float64(dir)*o.Step))
	case *float64:
		*p = o.clamp(math.Round((*p+float64(dir)*o.Step)/o.Step) * o.Step)
	case *bool:
		*p = !*p
	}
}

// Frac returns the value's position within its range, for sliders.
func (o *Option) Frac(s *Settings) float64 {
	var v float64
	switch p := o.ptr(s).(type) {
	case *int:
		v = float64(*p)
	case *float64:
		v = *p
	default:
		return 0
	}
	if o.Max == o.Min {
		return 0
	}
	return math.Max(0, math.Min(1, (v-o.Min)/(o.Max-o.Min)))
}

// SetFrac sets a numeric option from a 0..1 slider position.
func (o *Option) SetFrac(s *Settings, f float64) {
	f = math.Max(0, math.Min(1, f))
	v := o.Min + f*(o.Max-o.Min)
	switch p := o.ptr(s).(type) {
	case *int:
		*p = int(math.Round(v))
	case *float64:
		if o.Step > 0 {
			v = math.Round(v/o.Step) * o.Step
		}
		*p = math.Max(o.Min, math.Min(o.Max, v))
	}
}

func pick(rng *rand.Rand, list []string) string { return list[rng.Intn(len(list))] }

// RandomPalette picks a new palette and leaves the rest of the look alone.
func RandomPalette(l *Look, rng *rand.Rand) {
	l.Color = true
	switch r := rng.Float64(); {
	case r < 0.55:
		RandomHarmony(l, rng)
	case r < 0.85:
		all := AllPalettes()
		l.Palette = all[rng.Intn(len(all))].Name
	case r < 0.95:
		l.Palette = PalAdaptive
		l.Colors = []int{2, 3, 4, 6, 8, 12, 16, 32}[rng.Intn(8)]
		l.Resample++
	default:
		l.Palette = pick(rng, []string{PalGray, PalCube})
		l.Colors = []int{2, 3, 4, 8, 16, 27, 64}[rng.Intn(7)]
	}
}

// Randomize rolls a whole new look. Geometry (fit, flips) is kept, and the
// tone controls stay in a range where the picture remains recognisable.
func Randomize(l *Look, rng *rand.Rand) {
	l.Mode = pick(rng, []string{ModeHalf, ModeHalf, ModeQuad, ModeSextant, ModeBraille, ModeBraille, ModeASCII, ModeASCII})
	names := charsetNames()
	l.Charset = names[rng.Intn(len(names)-1)] // never "custom"
	l.Background = pick(rng, []string{"default", "default", "default", "black", "tint"})
	if rng.Float64() < 0.15 {
		l.Palette = PalOff
		l.Color = rng.Float64() < 0.8
	} else {
		RandomPalette(l, rng)
	}
	dn := DitherNames()
	l.Dither = dn[rng.Intn(len(dn))]
	l.DitherAmount = math.Round((0.5+rng.Float64()*0.9)*20) / 20
	l.Serpentine = rng.Intn(2) == 0
	l.Brightness = math.Round((rng.Float64()*0.2-0.1)*50) / 50
	l.Contrast = math.Round((0.9+rng.Float64()*0.6)*20) / 20
	l.Gamma = math.Round((0.8+rng.Float64()*0.45)*20) / 20
	l.Saturation = math.Round((0.6+rng.Float64()*1.0)*20) / 20
	l.HueShift = 0
	if rng.Float64() < 0.2 {
		l.HueShift = float64(rng.Intn(72) * 5)
	}
	l.Invert = rng.Float64() < 0.08
	l.Edges = "off"
	if rng.Float64() < 0.12 {
		l.Edges = pick(rng, []string{"mono", "color"})
		l.EdgeThreshold = math.Round((0.08+rng.Float64()*0.25)*100) / 100
	}
}

// Preset is a named look.
type Preset struct {
	Name    string `json:"name"`
	Look    Look   `json:"look"`
	Builtin bool   `json:"-"`
}

func variant(name string, edit func(*Look)) Preset {
	l := DefaultLook()
	edit(&l)
	return Preset{Name: name, Look: l, Builtin: true}
}

// BuiltinPresets are always available and cannot be deleted.
var BuiltinPresets = []Preset{
	variant("default", func(l *Look) {}),
	variant("sextant-hd", func(l *Look) { l.Mode = ModeSextant }),
	variant("ascii-classic", func(l *Look) { l.Mode = ModeASCII; l.Color = false; l.Dither = "none" }),
	variant("ascii-color", func(l *Look) { l.Mode = ModeASCII; l.Charset = "detailed"; l.Dither = "none" }),
	variant("braille-mono", func(l *Look) { l.Mode = ModeBraille; l.Color = false; l.Dither = "bluenoise" }),
	variant("gameboy", func(l *Look) { l.Palette = "gameboy"; l.Dither = "bayer4"; l.Contrast = 1.15 }),
	variant("newsprint", func(l *Look) {
		l.Mode = ModeSextant
		l.Palette = "paper"
		l.Dither = "halftone"
		l.Contrast = 1.2
	}),
	variant("matrix", func(l *Look) {
		l.Mode = ModeASCII
		l.Charset = "katakana"
		l.Palette = "matrix"
		l.Dither = "none"
		l.Background = "black"
	}),
	variant("amber-crt", func(l *Look) { l.Palette = "amber"; l.Dither = "hlines"; l.Contrast = 1.2 }),
	variant("cga", func(l *Look) { l.Palette = "cga"; l.Dither = "bayer2" }),
	variant("vaporwave", func(l *Look) { l.Palette = "vaporwave"; l.Dither = "bluenoise"; l.Saturation = 1.3 }),
	variant("blueprint", func(l *Look) {
		l.Mode = ModeBraille
		l.Edges = "mono"
		l.Palette = "ice"
		l.Background = "tint"
		l.Dither = "none"
	}),
	variant("pop-art", func(l *Look) {
		l.Palette = PalHarmony
		l.Scheme = "triadic"
		l.Hue = 20
		l.Chroma = 0.3
		l.Colors = 6
		l.Dither = "halftone"
		l.Contrast = 1.3
	}),
}
