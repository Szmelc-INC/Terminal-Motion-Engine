package engine

import (
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/dsp"
)

// Sound is everything that shapes the audio, the counterpart of Look. Most
// of it becomes an ffmpeg filter chain (see Chain); the embedded dsp.Params
// are applied by termo's own processor and change without a restart.
type Sound struct {
	// Tone.
	Preamp   float64     `json:"preamp"` // dB
	SubBass  float64     `json:"sub_bass"`
	Bass     float64     `json:"bass"`
	Mid      float64     `json:"mid"`
	Presence float64     `json:"presence"`
	Treble   float64     `json:"treble"`
	Lowpass  float64     `json:"lowpass"`  // Hz, 20000 = off
	Highpass float64     `json:"highpass"` // Hz, 0 = off
	Bandpass float64     `json:"bandpass"` // Hz, 0 = off
	BandQ    float64     `json:"band_q"`
	EQ       [10]float64 `json:"eq"` // dB at 31 Hz … 16 kHz

	// Dynamics.
	Comp          bool    `json:"comp"`
	CompThreshold float64 `json:"comp_threshold"` // dB
	CompRatio     float64 `json:"comp_ratio"`
	CompAttack    float64 `json:"comp_attack"` // ms
	CompRelease   float64 `json:"comp_release"`
	CompMakeup    float64 `json:"comp_makeup"` // dB
	Limiter       bool    `json:"limiter"`
	Limit         float64 `json:"limit"` // dB
	Gate          bool    `json:"gate"`
	GateThreshold float64 `json:"gate_threshold"`
	Normalize     bool    `json:"normalize"`
	Crystal       float64 `json:"crystal"`
	Exciter       float64 `json:"exciter"`
	Contrast      float64 `json:"contrast"`
	Clip          string  `json:"clip"`
	Drive         float64 `json:"drive"` // dB

	// Space.
	EchoDelay    float64 `json:"echo_delay"` // ms, 0 = off
	EchoFeedback float64 `json:"echo_feedback"`
	EchoMix      float64 `json:"echo_mix"`
	Width        float64 `json:"width"` // 1 = unchanged
	PanRate      float64 `json:"pan_rate"`
	PanDepth     float64 `json:"pan_depth"`
	Karaoke      bool    `json:"karaoke"`
	Downmix      bool    `json:"downmix"`
	Swap         bool    `json:"swap"`

	// Modulation and pitch.
	TremRate     float64 `json:"trem_rate"`
	TremDepth    float64 `json:"trem_depth"`
	VibRate      float64 `json:"vib_rate"`
	VibDepth     float64 `json:"vib_depth"`
	ChorusMix    float64 `json:"chorus_mix"`
	ChorusDepth  float64 `json:"chorus_depth"`
	ChorusRate   float64 `json:"chorus_rate"`
	FlangerDepth float64 `json:"flanger_depth"`
	FlangerRate  float64 `json:"flanger_rate"`
	FlangerRegen float64 `json:"flanger_regen"`
	PhaserRate   float64 `json:"phaser_rate"`
	PhaserDecay  float64 `json:"phaser_decay"`
	FreqShift    float64 `json:"freq_shift"`
	Pitch        float64 `json:"pitch"` // semitones
	Formant      bool    `json:"formant"`
	Wow          float64 `json:"wow"`
	Flutter      float64 `json:"flutter"`

	dsp.Params
}

// ClipTypes are the soft-clipping curves of the drive effect.
var ClipTypes = []string{"tanh", "atan", "cubic", "exp", "alg", "quintic", "sin", "erf", "hard"}

// EQBands are the centre frequencies of the graphic equaliser, in Hz.
var EQBands = [10]int{31, 62, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}

// DefaultSound leaves the audio untouched.
func DefaultSound() Sound {
	return Sound{
		Lowpass: 20000, BandQ: 1,
		CompThreshold: -18, CompRatio: 4, CompAttack: 20, CompRelease: 250, Limit: -1, GateThreshold: -40,
		Clip: "tanh", EchoFeedback: 0.4, EchoMix: 0.5, Width: 1, PanDepth: 1,
		TremDepth: 0.6, VibDepth: 0.5, ChorusDepth: 2, ChorusRate: 0.25, FlangerRate: 0.5, PhaserDecay: 0.4,
		Params: dsp.Defaults(),
	}
}

func db(v float64) float64 { return math.Pow(10, v/20) }

func ff(v float64) string { return trimFloat(v) }

// Chain builds the ffmpeg audio filter chain for the settings, or "" when
// none of the ffmpeg effects is in use. rubberband says whether ffmpeg has
// the rubberband filter; without it pitch shifting falls back to
// resampling, which also moves the formants.
func (s Sound) Chain(rubberband bool) string {
	var f []string
	add := func(format string, a ...any) { f = append(f, fmt.Sprintf(format, a...)) }
	switch {
	case s.Karaoke:
		// What sits in the middle of the stereo picture (usually the
		// voice) cancels out when one channel is subtracted from the other.
		add("pan=stereo|c0=c0-c1|c1=c1-c0")
	case s.Downmix:
		add("pan=stereo|c0=0.5*c0+0.5*c1|c1=0.5*c0+0.5*c1")
	case s.Swap:
		add("pan=stereo|c0=c1|c1=c0")
	}
	if s.Highpass >= 10 {
		add("highpass=f=%s", ff(s.Highpass))
	}
	if s.Lowpass > 0 && s.Lowpass < 19500 {
		add("lowpass=f=%s", ff(s.Lowpass))
	}
	if s.Bandpass >= 20 {
		add("bandpass=f=%s:width_type=q:w=%s", ff(s.Bandpass), ff(math.Max(0.1, s.BandQ)))
	}
	if s.SubBass != 0 {
		add("bass=g=%s:f=55:width_type=q:w=0.9", ff(s.SubBass))
	}
	if s.Bass != 0 {
		add("bass=g=%s:f=120", ff(s.Bass))
	}
	if s.Mid != 0 {
		add("equalizer=f=1000:width_type=q:w=0.8:g=%s", ff(s.Mid))
	}
	if s.Presence != 0 {
		add("equalizer=f=3500:width_type=q:w=1:g=%s", ff(s.Presence))
	}
	if s.Treble != 0 {
		add("treble=g=%s:f=6500", ff(s.Treble))
	}
	for i, g := range s.EQ {
		if g != 0 {
			add("equalizer=f=%d:width_type=o:w=1:g=%s", EQBands[i], ff(g))
		}
	}
	if s.Preamp != 0 {
		add("volume=%sdB", ff(s.Preamp))
	}
	if s.Pitch != 0 {
		r := math.Pow(2, s.Pitch/12)
		switch {
		case rubberband && s.Formant:
			add("rubberband=pitch=%s:formant=preserved", ff(r))
		case rubberband:
			add("rubberband=pitch=%s", ff(r))
		default:
			// Resample to shift the pitch, then stretch back to length.
			add("asetrate=%d,aresample=48000", int(48000*r+0.5))
			for t := 1 / r; ; {
				if t < 0.5 {
					add("atempo=0.5")
					t /= 0.5
				} else if t > 2 {
					add("atempo=2.0")
					t /= 2
				} else {
					add("atempo=%s", ff(t))
					break
				}
			}
		}
	}
	if s.FreqShift != 0 {
		add("afreqshift=shift=%s", ff(s.FreqShift))
	}
	if s.Gate {
		add("agate=threshold=%s:ratio=4:attack=5:release=120", ff(clampF(db(s.GateThreshold), 0.000001, 1)))
	}
	if s.Comp {
		add("acompressor=threshold=%s:ratio=%s:attack=%s:release=%s:makeup=%s",
			ff(clampF(db(s.CompThreshold), 0.000977, 1)), ff(clampF(s.CompRatio, 1, 20)), ff(clampF(s.CompAttack, 0.01, 2000)),
			ff(clampF(s.CompRelease, 0.01, 9000)), ff(clampF(db(s.CompMakeup), 1, 64)))
	}
	if s.Drive > 0 {
		clip := s.Clip
		if clip == "" {
			clip = "tanh"
		}
		// Push the level into the curve, then bring most of it back.
		add("volume=%sdB,asoftclip=type=%s,volume=%sdB", ff(s.Drive), clip, ff(-s.Drive*0.55))
	}
	if s.Crystal != 0 {
		add("crystalizer=i=%s", ff(s.Crystal))
	}
	if s.Exciter > 0 {
		add("aexciter=amount=%s:drive=8.5:blend=0", ff(s.Exciter))
	}
	if s.Contrast > 0 {
		add("acontrast=contrast=%s", ff(s.Contrast))
	}
	if s.ChorusMix > 0 {
		d, sp := clampF(s.ChorusDepth, 0.1, 9), clampF(s.ChorusRate, 0.1, 5)
		add("chorus=%s:%s:45|60|75:0.4|0.32|0.3:%s|%s|%s:%s|%s|%s", ff(1-s.ChorusMix*0.4), ff(clampF(s.ChorusMix, 0.01, 1)),
			ff(sp), ff(sp*1.6), ff(sp*0.7), ff(d), ff(d*1.15), ff(d*0.65))
	}
	if s.FlangerDepth > 0 {
		add("flanger=delay=2:depth=%s:regen=%s:width=75:speed=%s", ff(clampF(s.FlangerDepth, 0, 10)),
			ff(clampF(s.FlangerRegen, -95, 95)), ff(clampF(s.FlangerRate, 0.1, 10)))
	}
	if s.PhaserRate > 0 {
		add("aphaser=in_gain=0.6:out_gain=0.8:delay=3:decay=%s:speed=%s", ff(clampF(s.PhaserDecay, 0, 0.95)),
			ff(clampF(s.PhaserRate, 0.1, 2)))
	}
	if s.TremRate > 0 && s.TremDepth > 0 {
		add("tremolo=f=%s:d=%s", ff(clampF(s.TremRate, 0.1, 200)), ff(clampF(s.TremDepth, 0, 1)))
	}
	if s.VibRate > 0 && s.VibDepth > 0 {
		add("vibrato=f=%s:d=%s", ff(clampF(s.VibRate, 0.1, 200)), ff(clampF(s.VibDepth, 0, 1)))
	}
	if s.Wow > 0 {
		add("vibrato=f=0.6:d=%s", ff(clampF(s.Wow, 0, 1)))
	}
	if s.Flutter > 0 {
		add("vibrato=f=8.5:d=%s", ff(clampF(s.Flutter*0.4, 0, 1)))
	}
	if s.PanRate > 0 && s.PanDepth > 0 {
		add("apulsator=mode=sine:hz=%s:amount=%s", ff(clampF(s.PanRate, 0.01, 100)), ff(clampF(s.PanDepth, 0, 1)))
	}
	if s.EchoDelay > 0 && s.EchoMix > 0 {
		// Three taps, each quieter by the feedback factor, stand in for a
		// feedback loop.
		d, fb := clampF(s.EchoDelay, 1, 3000), clampF(s.EchoFeedback, 0.01, 0.95)
		delays, decays := []string{}, []string{}
		for k := 1.0; k <= 3 && d*k <= 9000; k++ {
			delays = append(delays, ff(d*k))
			decays = append(decays, ff(clampF(s.EchoMix*math.Pow(fb, k-1), 0.001, 1)))
		}
		add("aecho=0.9:0.9:%s:%s", strings.Join(delays, "|"), strings.Join(decays, "|"))
	}
	if s.Width != 1 {
		add("extrastereo=m=%s:c=0", ff(clampF(s.Width, 0, 4)))
	}
	if s.Normalize {
		add("dynaudnorm=f=250:g=15")
	}
	if s.Limiter {
		add("alimiter=limit=%s:attack=3:release=60:level=0", ff(clampF(db(s.Limit), 0.0625, 1)))
	}
	return strings.Join(f, ",")
}

func clampF(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// SoundActive reports whether any effect, ffmpeg's or termo's own, is on.
func (s Sound) SoundActive() bool { return s.Chain(true) != "" || s.Params.Active() }

// SoundSummary names the effects in use, for the HUD and preset lists.
func SoundSummary(s Sound) string {
	var p []string
	add := func(on bool, name string) {
		if on {
			p = append(p, name)
		}
	}
	eq := s.SubBass != 0 || s.Bass != 0 || s.Mid != 0 || s.Presence != 0 || s.Treble != 0 || s.EQ != [10]float64{}
	add(s.SynthMode != "off" && s.SynthMode != "", s.SynthMode+" synth")
	add(s.Bits < 16, fmt.Sprintf("%d-bit", s.Bits))
	add(s.SampleRate < dsp.Rate-1, fmt.Sprintf("%.0f Hz", s.SampleRate))
	add(s.Pitch != 0, fmt.Sprintf("pitch %+g", s.Pitch))
	add(s.RingFreq > 0, "ring mod")
	add(s.FreqShift != 0, "freq shift")
	add(eq, "eq")
	add((s.Lowpass > 0 && s.Lowpass < 19500) || s.Highpass >= 10 || s.Bandpass >= 20, "filter")
	add(s.Drive > 0, "drive")
	add(s.Comp, "compressor")
	add(s.ReverbMix > 0, "reverb")
	add(s.EchoDelay > 0, "echo")
	add(s.ChorusMix > 0, "chorus")
	add(s.FlangerDepth > 0, "flanger")
	add(s.PhaserRate > 0, "phaser")
	add(s.TremRate > 0, "tremolo")
	add(s.VibRate > 0 || s.Wow > 0 || s.Flutter > 0, "wobble")
	add(s.StutterMs > 0 || s.Reverse, "stutter")
	add(s.Hiss > 0 || s.Crackle > 0, "noise")
	add(s.Karaoke, "karaoke")
	add(s.PanRate > 0, "auto-pan")
	if len(p) == 0 {
		if s.SoundActive() {
			return "tweaked"
		}
		return "clean"
	}
	if len(p) > 4 {
		p = append(p[:4], fmt.Sprintf("+%d", len(p)-4))
	}
	return strings.Join(p, " · ")
}

// --- option table -------------------------------------------------------------

// SoundGroups lists the option groups of the audio settings, in menu order.
var SoundGroups = []string{"Audio: Tone", "Audio: EQ", "Audio: Dynamics", "Audio: Space", "Audio: Motion",
	"Audio: Lo-fi", "Audio: Synth"}

func init() {
	num := func(group, key, label, help string, lo, hi, step float64, p func(*Sound) *float64) {
		Options = append(Options, &Option{Key: key, Label: label, Group: group, Kind: KFloat, Min: lo, Max: hi, Step: step,
			Help: help, ptr: func(s *Settings) any { return p(&s.Sound) }})
	}
	whole := func(group, key, label, help string, lo, hi float64, p func(*Sound) *int) {
		Options = append(Options, &Option{Key: key, Label: label, Group: group, Kind: KInt, Min: lo, Max: hi, Step: 1,
			Help: help, ptr: func(s *Settings) any { return p(&s.Sound) }})
	}
	flag := func(group, key, label, help string, p func(*Sound) *bool) {
		Options = append(Options, &Option{Key: key, Label: label, Group: group, Kind: KBool, Help: help,
			ptr: func(s *Settings) any { return p(&s.Sound) }})
	}
	enum := func(group, key, label, help string, choices []string, p func(*Sound) *string) {
		Options = append(Options, &Option{Key: key, Label: label, Group: group, Kind: KEnum, Choices: choices, Help: help,
			ptr: func(s *Settings) any { return p(&s.Sound) }})
	}

	g := "Audio: Tone"
	num(g, "preamp", "Preamp", "overall gain before the effects, dB", -24, 24, 1, func(s *Sound) *float64 { return &s.Preamp })
	num(g, "sub-bass", "Sub bass", "boost or cut around 55 Hz, dB", -20, 20, 1, func(s *Sound) *float64 { return &s.SubBass })
	num(g, "bass", "Bass", "boost or cut the lows, dB", -20, 20, 1, func(s *Sound) *float64 { return &s.Bass })
	num(g, "mid", "Mid", "boost or cut around 1 kHz, dB", -20, 20, 1, func(s *Sound) *float64 { return &s.Mid })
	num(g, "presence", "Presence", "boost or cut around 3.5 kHz, where voices cut through, dB", -20, 20, 1, func(s *Sound) *float64 { return &s.Presence })
	num(g, "treble", "Treble", "boost or cut the highs, dB", -20, 20, 1, func(s *Sound) *float64 { return &s.Treble })
	num(g, "lowpass", "Low-pass", "cut everything above this frequency, Hz (20000 = off): muffled, underwater", 100, 20000, 100, func(s *Sound) *float64 { return &s.Lowpass })
	num(g, "highpass", "High-pass", "cut everything below this frequency, Hz (0 = off): thin, tinny", 0, 8000, 20, func(s *Sound) *float64 { return &s.Highpass })
	num(g, "bandpass", "Band-pass", "keep only a band around this frequency, Hz (0 = off): telephone, radio", 0, 8000, 50, func(s *Sound) *float64 { return &s.Bandpass })
	num(g, "band-q", "Band width", "how narrow the band-pass is (higher = narrower)", 0.1, 10, 0.1, func(s *Sound) *float64 { return &s.BandQ })

	g = "Audio: EQ"
	for i, hz := range EQBands {
		i := i
		label := fmt.Sprintf("%d Hz", hz)
		if hz >= 1000 {
			label = fmt.Sprintf("%d kHz", hz/1000)
		}
		num(g, fmt.Sprintf("eq%d", hz), label, "graphic equaliser band, dB", -15, 15, 1, func(s *Sound) *float64 { return &s.EQ[i] })
	}

	g = "Audio: Dynamics"
	flag(g, "comp", "Compressor", "even out loud and quiet passages", func(s *Sound) *bool { return &s.Comp })
	num(g, "comp-threshold", "Threshold", "level above which the compressor works, dB", -60, 0, 1, func(s *Sound) *float64 { return &s.CompThreshold })
	num(g, "comp-ratio", "Ratio", "how hard it squeezes (4 = 4:1)", 1, 20, 0.5, func(s *Sound) *float64 { return &s.CompRatio })
	num(g, "comp-attack", "Attack", "how fast it reacts, ms", 1, 200, 1, func(s *Sound) *float64 { return &s.CompAttack })
	num(g, "comp-release", "Release", "how fast it lets go, ms", 10, 2000, 10, func(s *Sound) *float64 { return &s.CompRelease })
	num(g, "comp-makeup", "Make-up gain", "gain added after compressing, dB", 0, 24, 1, func(s *Sound) *float64 { return &s.CompMakeup })
	flag(g, "limiter", "Limiter", "stop peaks from going above the limit", func(s *Sound) *bool { return &s.Limiter })
	num(g, "limit", "Limit", "ceiling of the limiter, dB", -24, 0, 0.5, func(s *Sound) *float64 { return &s.Limit })
	flag(g, "gate", "Noise gate", "mute what is quieter than the gate threshold", func(s *Sound) *bool { return &s.Gate })
	num(g, "gate-threshold", "Gate threshold", "level below which the gate closes, dB", -80, 0, 1, func(s *Sound) *float64 { return &s.GateThreshold })
	flag(g, "normalize", "Normalize", "keep the loudness even over time", func(s *Sound) *bool { return &s.Normalize })
	num(g, "drive", "Drive", "overdrive into a clipping curve, dB (0 = off): warm to broken", 0, 40, 1, func(s *Sound) *float64 { return &s.Drive })
	enum(g, "clip", "Clip curve", "shape of the distortion", ClipTypes, func(s *Sound) *string { return &s.Clip })
	num(g, "crystal", "Crystalizer", "sharpen (or, below 0, soften) the transients", -10, 10, 0.5, func(s *Sound) *float64 { return &s.Crystal })
	num(g, "exciter", "Exciter", "add synthetic harmonics to the highs", 0, 10, 0.5, func(s *Sound) *float64 { return &s.Exciter })
	num(g, "audio-contrast", "Contrast", "exaggerate the difference between loud and quiet", 0, 100, 5, func(s *Sound) *float64 { return &s.Contrast })

	g = "Audio: Space"
	num(g, "reverb", "Reverb", "amount of reverb (live)", 0, 1, 0.05, func(s *Sound) *float64 { return &s.ReverbMix })
	num(g, "reverb-size", "Room size", "from a closet to a cathedral (live)", 0, 1, 0.05, func(s *Sound) *float64 { return &s.ReverbSize })
	num(g, "reverb-damp", "Damping", "how fast the highs die in the room (live)", 0, 1, 0.05, func(s *Sound) *float64 { return &s.ReverbDamp })
	num(g, "reverb-width", "Reverb width", "stereo spread of the reverb (live)", 0, 1, 0.05, func(s *Sound) *float64 { return &s.ReverbWidth })
	num(g, "reverb-pre", "Pre-delay", "gap before the reverb starts, ms (live)", 0, 240, 5, func(s *Sound) *float64 { return &s.ReverbPre })
	num(g, "echo-delay", "Echo", "delay between echoes, ms (0 = off)", 0, 2000, 10, func(s *Sound) *float64 { return &s.EchoDelay })
	num(g, "echo-feedback", "Echo decay", "how much quieter each echo is", 0, 0.95, 0.05, func(s *Sound) *float64 { return &s.EchoFeedback })
	num(g, "echo-mix", "Echo level", "loudness of the first echo", 0, 1, 0.05, func(s *Sound) *float64 { return &s.EchoMix })
	num(g, "width", "Stereo width", "0 = mono, 1 = unchanged, above = wider", 0, 4, 0.1, func(s *Sound) *float64 { return &s.Width })
	num(g, "balance", "Balance", "left / right (live)", -1, 1, 0.05, func(s *Sound) *float64 { return &s.Balance })
	num(g, "pan-rate", "Auto-pan", "sweep the sound between the speakers, Hz (0 = off): \"8D audio\"", 0, 10, 0.05, func(s *Sound) *float64 { return &s.PanRate })
	num(g, "pan-depth", "Auto-pan depth", "how far the sweep goes", 0, 1, 0.05, func(s *Sound) *float64 { return &s.PanDepth })
	flag(g, "karaoke", "Karaoke", "cancel what is in the centre, usually the voice", func(s *Sound) *bool { return &s.Karaoke })
	flag(g, "downmix", "Mono", "play both channels on both speakers", func(s *Sound) *bool { return &s.Downmix })
	flag(g, "swap", "Swap channels", "left becomes right", func(s *Sound) *bool { return &s.Swap })

	g = "Audio: Motion"
	num(g, "pitch", "Pitch", "shift the pitch without changing the speed, semitones", -12, 12, 1, func(s *Sound) *float64 { return &s.Pitch })
	flag(g, "formant", "Keep formants", "keep voices sounding like the same person when the pitch moves", func(s *Sound) *bool { return &s.Formant })
	num(g, "freq-shift", "Frequency shift", "move every frequency by the same number of Hz: metallic, alien", -1000, 1000, 10, func(s *Sound) *float64 { return &s.FreqShift })
	num(g, "ring-freq", "Ring modulator", "multiply by a tone of this frequency, Hz (0 = off): robots, Daleks (live)", 0, 2000, 5, func(s *Sound) *float64 { return &s.RingFreq })
	num(g, "ring-mix", "Ring mix", "blend of the ring modulator (live)", 0, 1, 0.05, func(s *Sound) *float64 { return &s.RingMix })
	num(g, "tremolo", "Tremolo", "pulse the volume, Hz (0 = off)", 0, 30, 0.5, func(s *Sound) *float64 { return &s.TremRate })
	num(g, "tremolo-depth", "Tremolo depth", "how deep the pulse cuts", 0, 1, 0.05, func(s *Sound) *float64 { return &s.TremDepth })
	num(g, "vibrato", "Vibrato", "wobble the pitch, Hz (0 = off)", 0, 20, 0.5, func(s *Sound) *float64 { return &s.VibRate })
	num(g, "vibrato-depth", "Vibrato depth", "how far the pitch wobbles", 0, 1, 0.05, func(s *Sound) *float64 { return &s.VibDepth })
	num(g, "chorus", "Chorus", "thicken the sound with detuned copies (0 = off)", 0, 1, 0.05, func(s *Sound) *float64 { return &s.ChorusMix })
	num(g, "chorus-depth", "Chorus depth", "how far the copies drift, ms", 0.1, 8, 0.1, func(s *Sound) *float64 { return &s.ChorusDepth })
	num(g, "chorus-rate", "Chorus rate", "how fast they drift, Hz", 0.1, 5, 0.05, func(s *Sound) *float64 { return &s.ChorusRate })
	num(g, "flanger", "Flanger", "jet-plane sweep, depth in ms (0 = off)", 0, 10, 0.5, func(s *Sound) *float64 { return &s.FlangerDepth })
	num(g, "flanger-rate", "Flanger rate", "speed of the sweep, Hz", 0.1, 10, 0.1, func(s *Sound) *float64 { return &s.FlangerRate })
	num(g, "flanger-regen", "Flanger feedback", "how resonant the sweep is, %", -95, 95, 5, func(s *Sound) *float64 { return &s.FlangerRegen })
	num(g, "phaser", "Phaser", "swirling notches, Hz (0 = off)", 0, 2, 0.05, func(s *Sound) *float64 { return &s.PhaserRate })
	num(g, "phaser-decay", "Phaser depth", "how pronounced the swirl is", 0, 0.95, 0.05, func(s *Sound) *float64 { return &s.PhaserDecay })

	g = "Audio: Lo-fi"
	whole(g, "bits", "Bit depth", "quantise the samples to this many bits (16 = off): 8 is a Game Boy, 4 is a toy (live)", 1, 16, func(s *Sound) *int { return &s.Bits })
	enum(g, "audio-dither", "Dither", "noise added before quantising, so quiet parts turn to hiss instead of grit (live)", dsp.Dithers, func(s *Sound) *string { return &s.Dither })
	num(g, "audio-dither-amount", "Dither amount", "strength of the dither, in quantisation steps (live)", 0, 4, 0.1, func(s *Sound) *float64 { return &s.DitherAmount })
	enum(g, "curve", "Quantise curve", "linear, or mu-law like old telephone lines: finer steps for quiet sounds (live)", dsp.Curves, func(s *Sound) *string { return &s.Curve })
	num(g, "sample-rate", "Sample rate", "hold each sample to fake a low sample rate, Hz (48000 = off) (live)", 500, 48000, 500, func(s *Sound) *float64 { return &s.SampleRate })
	num(g, "stutter", "Stutter", "length of the slices that get repeated, ms (0 = off) (live)", 0, 1000, 10, func(s *Sound) *float64 { return &s.StutterMs })
	whole(g, "stutter-repeats", "Repeats", "how many times a slice repeats (live)", 1, 16, func(s *Sound) *int { return &s.StutterRepeats })
	num(g, "stutter-chance", "Stutter chance", "how often a slice gets caught (live)", 0, 1, 0.05, func(s *Sound) *float64 { return &s.StutterChance })
	flag(g, "reverse", "Reverse slices", "play each slice backwards (live)", func(s *Sound) *bool { return &s.Reverse })
	num(g, "wow", "Tape wow", "slow pitch drift of a tired tape", 0, 1, 0.05, func(s *Sound) *float64 { return &s.Wow })
	num(g, "flutter", "Tape flutter", "fast pitch shake of a cheap cassette deck", 0, 1, 0.05, func(s *Sound) *float64 { return &s.Flutter })
	num(g, "hiss", "Hiss", "steady background noise (live)", 0, 1, 0.05, func(s *Sound) *float64 { return &s.Hiss })
	enum(g, "noise-color", "Hiss color", "white is bright, pink is even, brown is a rumble (live)", dsp.NoiseColors, func(s *Sound) *string { return &s.NoiseColor })
	num(g, "crackle", "Vinyl crackle", "pops and surface noise of a worn record (live)", 0, 1, 0.05, func(s *Sound) *float64 { return &s.Crackle })

	g = "Audio: Synth"
	enum(g, "synth", "Instrument", "replace the sound with an oscillator that follows its pitch and loudness (live)", dsp.SynthModes, func(s *Sound) *string { return &s.SynthMode })
	num(g, "synth-mix", "Synth mix", "0 = original sound, 1 = synth only (live)", 0, 1, 0.05, func(s *Sound) *float64 { return &s.SynthMix })
	enum(g, "synth-wave", "Waveform", "shape of the chip oscillator (live)", dsp.Waves, func(s *Sound) *string { return &s.SynthWave })
	whole(g, "synth-octave", "Octave", "play octaves above or below what is heard (live)", -3, 3, func(s *Sound) *int { return &s.SynthOctave })
	enum(g, "synth-scale", "Snap to scale", "force the notes onto a scale; chromatic = semitones, like MIDI (live)", dsp.Scales, func(s *Sound) *string { return &s.SynthScale })
	enum(g, "synth-key", "Key", "root note of the scale (live)", dsp.Keys, func(s *Sound) *string { return &s.SynthKey })
	num(g, "synth-glide", "Glide", "time to slide from one note to the next, ms (live)", 0, 1000, 10, func(s *Sound) *float64 { return &s.SynthGlide })
	enum(g, "arp", "Arpeggio", "cycle through a chord on every note, the chiptune way (live)", dsp.Arps, func(s *Sound) *string { return &s.Arp })
	num(g, "arp-rate", "Arpeggio rate", "notes per second (live)", 1, 40, 1, func(s *Sound) *float64 { return &s.ArpRate })
	num(g, "acid-cutoff", "Filter cutoff", "base cutoff of the acid filter, Hz (live)", 60, 4000, 20, func(s *Sound) *float64 { return &s.AcidCutoff })
	num(g, "acid-res", "Resonance", "how much the acid filter squelches (live)", 0, 0.98, 0.02, func(s *Sound) *float64 { return &s.AcidRes })
	num(g, "acid-env", "Filter envelope", "how far loud notes open the filter (live)", 0, 1, 0.05, func(s *Sound) *float64 { return &s.AcidEnv })
	num(g, "synth-detune", "Detune", "spread between the drone's voices, cents (live)", 0, 60, 1, func(s *Sound) *float64 { return &s.SynthDetune })
	num(g, "synth-sens", "Sensitivity", "the synth stays silent below this level, dB (live)", -70, -10, 1, func(s *Sound) *float64 { return &s.SynthSens })

	Groups = append(Groups, SoundGroups...)
}

// --- presets ------------------------------------------------------------------

// SoundPreset is a named sound.
type SoundPreset struct {
	Name    string `json:"name"`
	Sound   Sound  `json:"sound"`
	Builtin bool   `json:"-"`
}

func sound(name string, edit func(*Sound)) SoundPreset {
	s := DefaultSound()
	edit(&s)
	return SoundPreset{Name: name, Sound: s, Builtin: true}
}

// BuiltinSounds ship with termo.
var BuiltinSounds = []SoundPreset{
	sound("clean", func(s *Sound) {}),
	sound("bass-boost", func(s *Sound) { s.SubBass, s.Bass, s.Limiter = 6, 5, true }),
	sound("loudness", func(s *Sound) { s.Bass, s.Treble, s.Comp, s.CompMakeup, s.Limiter = 5, 4, true, 4, true }),
	sound("sparkle", func(s *Sound) { s.Treble, s.Exciter, s.Crystal = 4, 3, 2 }),
	sound("warm-tube", func(s *Sound) { s.Drive, s.Clip, s.Bass, s.Treble = 8, "tanh", 2, -3 }),
	sound("telephone", func(s *Sound) {
		s.Bandpass, s.BandQ, s.Drive, s.Downmix, s.Bits, s.Curve, s.Hiss = 1800, 1.2, 6, true, 8, "mu-law", 0.08
	}),
	sound("am-radio", func(s *Sound) {
		s.Highpass, s.Lowpass, s.Downmix, s.Drive, s.Hiss, s.NoiseColor, s.Comp = 300, 3500, true, 5, 0.2, "pink", true
	}),
	sound("megaphone", func(s *Sound) { s.Bandpass, s.BandQ, s.Drive, s.Clip, s.Downmix = 1500, 0.7, 20, "hard", true }),
	sound("walkie-talkie", func(s *Sound) {
		s.Bandpass, s.BandQ, s.Drive, s.Clip, s.Downmix, s.Hiss, s.Gate, s.GateThreshold = 2200, 2, 14, "hard", true, 0.25, true, -30
	}),
	sound("underwater", func(s *Sound) {
		s.Lowpass, s.ChorusMix, s.ChorusDepth, s.ReverbMix, s.ReverbSize = 500, 0.5, 4, 0.35, 0.8
	}),
	sound("next-room", func(s *Sound) { s.Lowpass, s.SubBass, s.ReverbMix, s.ReverbSize, s.Width = 350, 6, 0.25, 0.5, 0.4 }),
	sound("small-room", func(s *Sound) { s.ReverbMix, s.ReverbSize, s.ReverbDamp = 0.25, 0.3, 0.7 }),
	sound("cathedral", func(s *Sound) { s.ReverbMix, s.ReverbSize, s.ReverbDamp, s.ReverbPre = 0.6, 0.95, 0.2, 40 }),
	sound("cave", func(s *Sound) {
		s.ReverbMix, s.ReverbSize, s.EchoDelay, s.EchoFeedback, s.EchoMix, s.Lowpass = 0.45, 0.9, 380, 0.5, 0.4, 5000
	}),
	sound("stadium", func(s *Sound) {
		s.EchoDelay, s.EchoFeedback, s.EchoMix, s.ReverbMix, s.ReverbSize, s.Width = 180, 0.35, 0.35, 0.3, 0.7, 1.6
	}),
	sound("canyon-echo", func(s *Sound) { s.EchoDelay, s.EchoFeedback, s.EchoMix = 520, 0.6, 0.6 }),
	sound("8d-audio", func(s *Sound) { s.PanRate, s.PanDepth, s.ReverbMix, s.ReverbSize = 0.12, 1, 0.25, 0.7 }),
	sound("karaoke", func(s *Sound) { s.Karaoke = true }),
	sound("lofi-tape", func(s *Sound) {
		s.Lowpass, s.Highpass, s.Wow, s.Flutter, s.Hiss, s.NoiseColor, s.Drive = 6500, 60, 0.25, 0.15, 0.12, "pink", 4
	}),
	sound("cassette", func(s *Sound) { s.Lowpass, s.Wow, s.Flutter, s.Hiss, s.Comp, s.Width = 9000, 0.15, 0.3, 0.1, true, 0.8 }),
	sound("vinyl", func(s *Sound) {
		s.Crackle, s.Hiss, s.NoiseColor, s.Lowpass, s.Highpass, s.Wow = 0.55, 0.04, "brown", 12000, 40, 0.1
	}),
	sound("gramophone", func(s *Sound) {
		s.Bandpass, s.BandQ, s.Downmix, s.Crackle, s.Hiss, s.Wow, s.Drive = 1400, 0.6, true, 0.9, 0.15, 0.4, 6
	}),
	sound("8-bit", func(s *Sound) { s.Bits, s.SampleRate, s.Dither = 8, 11025, "triangular" }),
	sound("4-bit-crunch", func(s *Sound) { s.Bits, s.SampleRate = 4, 8000 }),
	sound("1-bit-speaker", func(s *Sound) { s.Bits, s.SampleRate, s.Highpass, s.Downmix = 1, 16000, 200, true }),
	sound("chiptune", func(s *Sound) {
		s.SynthMode, s.SynthWave, s.SynthScale, s.SynthGlide, s.Bits = "chip", "square", "chromatic", 0, 8
	}),
	sound("nes-arpeggio", func(s *Sound) {
		s.SynthMode, s.SynthWave, s.SynthScale, s.SynthGlide, s.Arp, s.ArpRate, s.SynthOctave = "chip", "pulse", "minor", 0, "minor", 16, 1
	}),
	sound("gameboy-lead", func(s *Sound) {
		s.SynthMode, s.SynthWave, s.SynthScale, s.SynthGlide, s.SynthMix, s.SampleRate = "chip", "triangle", "pentatonic", 20, 0.85, 22000
	}),
	sound("acid-bass", func(s *Sound) {
		s.SynthMode, s.SynthOctave, s.SynthScale, s.SynthGlide, s.AcidCutoff, s.AcidRes, s.AcidEnv, s.SubBass, s.Drive = "acid", -1, "minor", 60, 300, 0.9, 0.85, 4, 6
	}),
	sound("acid-lead", func(s *Sound) {
		s.SynthMode, s.SynthWave, s.SynthScale, s.SynthGlide, s.AcidCutoff, s.AcidRes, s.EchoDelay, s.EchoMix = "acid", "square", "blues", 90, 700, 0.85, 240, 0.35
	}),
	sound("drone", func(s *Sound) {
		s.SynthMode, s.SynthScale, s.SynthDetune, s.ReverbMix, s.ReverbSize, s.ReverbDamp = "drone", "minor", 18, 0.6, 0.95, 0.3
	}),
	sound("drone-under", func(s *Sound) {
		s.SynthMode, s.SynthMix, s.SynthScale, s.ReverbMix, s.ReverbSize, s.Lowpass = "drone", 0.5, "pentatonic", 0.4, 0.9, 4000
	}),
	sound("wind", func(s *Sound) {
		s.SynthMode, s.SynthScale, s.SynthGlide, s.ReverbMix, s.ReverbSize = "wind", "free", 200, 0.4, 0.8
	}),
	sound("robot", func(s *Sound) { s.RingFreq, s.RingMix, s.Downmix, s.Bits, s.Comp = 50, 1, true, 10, true }),
	sound("dalek", func(s *Sound) {
		s.RingFreq, s.RingMix, s.Drive, s.Clip, s.Bandpass, s.BandQ = 30, 1, 14, "hard", 1600, 0.5
	}),
	sound("sci-fi-computer", func(s *Sound) {
		s.RingFreq, s.RingMix, s.FlangerDepth, s.FlangerRate, s.FlangerRegen, s.Pitch, s.Formant, s.EchoDelay, s.EchoMix = 120, 0.7, 4, 0.3, 60, -2, true, 90, 0.3
	}),
	sound("alien", func(s *Sound) {
		s.FreqShift, s.PhaserRate, s.PhaserDecay, s.VibRate, s.VibDepth = 260, 0.6, 0.7, 6, 0.4
	}),
	sound("chipmunk", func(s *Sound) { s.Pitch = 7 }),
	sound("helium", func(s *Sound) { s.Pitch, s.Treble = 12, 3 }),
	sound("demon", func(s *Sound) { s.Pitch, s.ReverbMix, s.ReverbSize, s.SubBass, s.Drive = -7, 0.35, 0.85, 5, 6 }),
	sound("giant", func(s *Sound) { s.Pitch, s.Formant, s.Bass, s.ReverbMix = -5, false, 4, 0.2 }),
	sound("nightcore", func(s *Sound) { s.Pitch, s.Treble, s.Comp = 4, 2, true }),
	sound("vaporwave", func(s *Sound) {
		s.Pitch, s.ReverbMix, s.ReverbSize, s.ChorusMix, s.Lowpass, s.Wow, s.Width = -4, 0.4, 0.85, 0.4, 7000, 0.2, 1.5
	}),
	sound("dream", func(s *Sound) {
		s.ChorusMix, s.ChorusDepth, s.ReverbMix, s.ReverbSize, s.Treble, s.Width = 0.6, 3, 0.5, 0.9, -2, 1.8
	}),
	sound("psychedelic", func(s *Sound) {
		s.PhaserRate, s.PhaserDecay, s.FlangerDepth, s.FlangerRate, s.PanRate, s.EchoDelay, s.EchoMix = 0.4, 0.8, 5, 0.2, 0.2, 300, 0.4
	}),
	sound("jet-flanger", func(s *Sound) { s.FlangerDepth, s.FlangerRate, s.FlangerRegen = 8, 0.25, 80 }),
	sound("tremolo-surf", func(s *Sound) { s.TremRate, s.TremDepth, s.ReverbMix, s.ReverbSize = 6, 0.8, 0.3, 0.6 }),
	sound("helicopter", func(s *Sound) { s.TremRate, s.TremDepth = 14, 1 }),
	sound("broken-speaker", func(s *Sound) { s.Drive, s.Clip, s.Highpass, s.Lowpass, s.Bits = 30, "hard", 250, 4000, 6 }),
	sound("glitch", func(s *Sound) {
		s.StutterMs, s.StutterRepeats, s.StutterChance, s.Bits, s.SampleRate = 90, 3, 0.35, 10, 16000
	}),
	sound("skipping-cd", func(s *Sound) { s.StutterMs, s.StutterRepeats, s.StutterChance = 60, 6, 0.2 }),
	sound("backmasking", func(s *Sound) { s.Reverse, s.StutterMs, s.StutterChance, s.ReverbMix = true, 400, 0, 0.2 }),
	sound("club-next-door", func(s *Sound) { s.Lowpass, s.SubBass, s.Comp, s.CompMakeup, s.ReverbMix = 180, 8, true, 6, 0.15 }),
	sound("podcast-voice", func(s *Sound) {
		s.Highpass, s.Presence, s.Comp, s.CompThreshold, s.CompRatio, s.CompMakeup, s.Gate, s.Limiter = 90, 3, true, -24, 3, 5, true, true
	}),
}

// RandomSound rolls a sound that is strange but still listenable: a few
// effects at moderate settings rather than everything at once.
func RandomSound(rng *rand.Rand) Sound {
	s := DefaultSound()
	r := func(lo, hi, step float64) float64 { return math.Round((lo+rng.Float64()*(hi-lo))/step) * step }
	rolls := []func(){
		func() { s.Bass, s.Treble = r(-4, 9, 1), r(-6, 6, 1) },
		func() { s.Lowpass = r(400, 6000, 100) },
		func() { s.Highpass = r(200, 1500, 20) },
		func() { s.Bandpass, s.BandQ = r(600, 3000, 50), r(0.5, 2, 0.1) },
		func() { s.Drive, s.Clip = r(4, 22, 1), ClipTypes[rng.Intn(len(ClipTypes))] },
		func() { s.ReverbMix, s.ReverbSize = r(0.2, 0.6, 0.05), r(0.3, 0.95, 0.05) },
		func() { s.EchoDelay, s.EchoFeedback, s.EchoMix = r(80, 600, 10), r(0.2, 0.6, 0.05), r(0.25, 0.6, 0.05) },
		func() { s.ChorusMix, s.ChorusDepth = r(0.3, 0.7, 0.05), r(1, 5, 0.1) },
		func() { s.FlangerDepth, s.FlangerRate, s.FlangerRegen = r(2, 8, 0.5), r(0.1, 1.5, 0.1), r(0, 80, 5) },
		func() { s.PhaserRate, s.PhaserDecay = r(0.2, 1.2, 0.05), r(0.4, 0.8, 0.05) },
		func() { s.TremRate, s.TremDepth = r(2, 12, 0.5), r(0.4, 1, 0.05) },
		func() { s.Pitch = float64(rng.Intn(15) - 7) },
		func() { s.RingFreq, s.RingMix = r(20, 400, 5), r(0.5, 1, 0.05) },
		func() { s.FreqShift = r(-300, 300, 10) },
		func() { s.Bits, s.Dither = 3+rng.Intn(8), dsp.Dithers[rng.Intn(len(dsp.Dithers))] },
		func() { s.SampleRate = r(3000, 16000, 500) },
		func() {
			s.StutterMs, s.StutterRepeats, s.StutterChance = r(40, 200, 10), 2+rng.Intn(4), r(0.15, 0.5, 0.05)
		},
		func() { s.Wow, s.Flutter, s.Hiss = r(0.1, 0.4, 0.05), r(0, 0.3, 0.05), r(0.03, 0.15, 0.01) },
		func() { s.Crackle = r(0.3, 0.8, 0.05) },
		func() { s.PanRate = r(0.05, 0.5, 0.05) },
		func() {
			s.SynthMode = dsp.SynthModes[1+rng.Intn(len(dsp.SynthModes)-1)]
			s.SynthWave = dsp.Waves[rng.Intn(len(dsp.Waves))]
			s.SynthScale = dsp.Scales[1+rng.Intn(len(dsp.Scales)-1)]
			s.SynthOctave, s.SynthMix = rng.Intn(3)-1, r(0.6, 1, 0.05)
			if rng.Intn(3) == 0 {
				s.Arp, s.ArpRate = dsp.Arps[1+rng.Intn(len(dsp.Arps)-1)], r(8, 20, 1)
			}
		},
	}
	n := 2 + rng.Intn(3)
	for _, i := range rng.Perm(len(rolls))[:n] {
		rolls[i]()
	}
	s.Limiter = true // whatever came out, keep it from clipping
	return s
}
