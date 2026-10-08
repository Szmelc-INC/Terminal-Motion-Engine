// Package dsp is termo's own audio processor. It runs on the PCM stream
// just before it reaches the sound card, so every parameter takes effect
// at once, without restarting the decoder. It holds the effects ffmpeg has
// no filter for: bit-depth quantisation with dither, sample-and-hold rate
// reduction, ring modulation, stutter, tape noise, a Freeverb reverb, and a
// resynthesiser that replaces the sound with an oscillator following its
// pitch and loudness.
package dsp

import (
	"encoding/binary"
	"math"
)

// Rate is the sample rate the processor works at.
const Rate = 48000

// Params are the settings of the processor. The zero value is not neutral;
// start from Defaults.
type Params struct {
	SynthMode   string  `json:"synth_mode"` // off, chip, acid, drone, wind
	SynthMix    float64 `json:"synth_mix"`
	SynthWave   string  `json:"synth_wave"` // square, pulse, saw, triangle, sine
	SynthOctave int     `json:"synth_octave"`
	SynthScale  string  `json:"synth_scale"` // free, chromatic, major, minor, pentatonic, blues
	SynthKey    string  `json:"synth_key"`
	SynthGlide  float64 `json:"synth_glide"` // ms
	SynthSens   float64 `json:"synth_sens"`  // dB below which the synth stays silent
	SynthDetune float64 `json:"synth_detune"`
	AcidCutoff  float64 `json:"acid_cutoff"`
	AcidRes     float64 `json:"acid_res"`
	AcidEnv     float64 `json:"acid_env"`
	Arp         string  `json:"arp"` // off, octave, major, minor, fifth
	ArpRate     float64 `json:"arp_rate"`

	RingFreq float64 `json:"ring_freq"` // Hz, 0 = off
	RingMix  float64 `json:"ring_mix"`

	StutterMs      float64 `json:"stutter_ms"` // slice length, 0 = off
	StutterRepeats int     `json:"stutter_repeats"`
	StutterChance  float64 `json:"stutter_chance"`
	Reverse        bool    `json:"reverse"`

	Bits         int     `json:"bits"`   // 16 = untouched
	Dither       string  `json:"dither"` // none, rectangular, triangular, shaped
	DitherAmount float64 `json:"dither_amount"`
	Curve        string  `json:"curve"`       // linear, mu-law
	SampleRate   float64 `json:"sample_rate"` // Hz, 48000 = untouched

	Crackle    float64 `json:"crackle"`
	Hiss       float64 `json:"hiss"`
	NoiseColor string  `json:"noise_color"` // white, pink, brown

	ReverbMix   float64 `json:"reverb_mix"`
	ReverbSize  float64 `json:"reverb_size"`
	ReverbDamp  float64 `json:"reverb_damp"`
	ReverbWidth float64 `json:"reverb_width"`
	ReverbPre   float64 `json:"reverb_pre"` // ms

	Balance float64 `json:"balance"` // -1 left … 1 right
}

// Choices for the enumerated parameters.
var (
	SynthModes  = []string{"off", "chip", "acid", "drone", "wind"}
	Waves       = []string{"square", "pulse", "saw", "triangle", "sine"}
	Scales      = []string{"free", "chromatic", "major", "minor", "pentatonic", "blues"}
	Keys        = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	Arps        = []string{"off", "octave", "fifth", "major", "minor"}
	Dithers     = []string{"none", "rectangular", "triangular", "shaped"}
	Curves      = []string{"linear", "mu-law"}
	NoiseColors = []string{"white", "pink", "brown"}
)

// Defaults returns settings that leave the sound untouched.
func Defaults() Params {
	return Params{
		SynthMode: "off", SynthMix: 1, SynthWave: "square", SynthScale: "chromatic", SynthKey: "C", SynthGlide: 30,
		SynthSens: -45, SynthDetune: 12, AcidCutoff: 400, AcidRes: 0.7, AcidEnv: 0.7, Arp: "off", ArpRate: 12,
		RingMix: 1, StutterRepeats: 2, StutterChance: 0.5,
		Bits: 16, Dither: "none", DitherAmount: 1, Curve: "linear", SampleRate: Rate, NoiseColor: "white",
		ReverbSize: 0.6, ReverbDamp: 0.5, ReverbWidth: 1,
	}
}

// Active reports whether the settings change the sound at all.
func (p *Params) Active() bool {
	return (p.SynthMode != "off" && p.SynthMode != "" && p.SynthMix > 0) || (p.RingFreq > 0 && p.RingMix > 0) ||
		p.StutterMs > 0 || p.Reverse || (p.Bits > 0 && p.Bits < 16) || p.SampleRate < Rate-1 || p.Crackle > 0 ||
		p.Hiss > 0 || p.ReverbMix > 0 || p.Balance != 0
}

// Processor carries the state of every effect between chunks. It is not
// safe for concurrent use.
type Processor struct {
	l, r []float32
	rng  uint32

	syn   synth
	ring  float64
	stut  stutter
	crush crusher
	noise noiser
	verb  *reverb
}

// NewProcessor returns a processor with cleared state.
func NewProcessor() *Processor { return &Processor{rng: 0x9e3779b9} }

// rand returns a uniform value in [0,1).
func (p *Processor) rand() float32 {
	p.rng ^= p.rng << 13
	p.rng ^= p.rng >> 17
	p.rng ^= p.rng << 5
	return float32(p.rng>>8) / (1 << 24)
}

// Process applies the effects to interleaved 16-bit little-endian stereo
// PCM, in place.
func (p *Processor) Process(pcm []byte, prm *Params) {
	n := len(pcm) / 4
	if n == 0 || prm == nil || !prm.Active() {
		return
	}
	if cap(p.l) < n {
		p.l, p.r = make([]float32, n), make([]float32, n)
	}
	l, r := p.l[:n], p.r[:n]
	for i := 0; i < n; i++ {
		l[i] = float32(int16(binary.LittleEndian.Uint16(pcm[i*4:]))) / 32768
		r[i] = float32(int16(binary.LittleEndian.Uint16(pcm[i*4+2:]))) / 32768
	}
	if prm.SynthMode != "off" && prm.SynthMode != "" && prm.SynthMix > 0 {
		p.syn.process(p, l, r, prm)
	}
	if prm.RingFreq > 0 && prm.RingMix > 0 {
		mix := float32(clamp(prm.RingMix, 0, 1))
		step := 2 * math.Pi * prm.RingFreq / Rate
		for i := range l {
			c := float32(math.Sin(p.ring))
			p.ring += step
			l[i] += (l[i]*c - l[i]) * mix
			r[i] += (r[i]*c - r[i]) * mix
		}
		p.ring = math.Mod(p.ring, 2*math.Pi)
	}
	if prm.StutterMs > 0 || prm.Reverse {
		p.stut.process(p, l, r, prm)
	}
	if (prm.Bits > 0 && prm.Bits < 16) || prm.SampleRate < Rate-1 {
		p.crush.process(p, l, r, prm)
	}
	if prm.Crackle > 0 || prm.Hiss > 0 {
		p.noise.process(p, l, r, prm)
	}
	if prm.ReverbMix > 0 {
		if p.verb == nil {
			p.verb = newReverb()
		}
		p.verb.process(l, r, prm)
	}
	if b := float32(clamp(prm.Balance, -1, 1)); b != 0 {
		gl, gr := min(1, 1-b), min(1, 1+b)
		for i := range l {
			l[i] *= gl
			r[i] *= gr
		}
	}
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint16(pcm[i*4:], uint16(toS16(l[i])))
		binary.LittleEndian.PutUint16(pcm[i*4+2:], uint16(toS16(r[i])))
	}
}

func toS16(f float32) int16 {
	v := f * 32767
	switch {
	case v != v: // NaN
		return 0
	case v > 32767:
		return 32767
	case v < -32768:
		return -32768
	}
	return int16(v)
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// --- quantisation ------------------------------------------------------------

// crusher reduces the bit depth and the sample rate: the two halves of
// "8-bit" sound. Dither is noise added before rounding so that quiet
// passages turn into hiss instead of into gritty distortion.
type crusher struct {
	phase        float64
	holdL, holdR float32
	errL, errR   float32
}

func muCompress(x float32) float32 {
	const mu = 255
	s := float32(1)
	if x < 0 {
		s, x = -1, -x
	}
	return s * float32(math.Log1p(mu*float64(x))/math.Log1p(mu))
}

func muExpand(y float32) float32 {
	const mu = 255
	s := float32(1)
	if y < 0 {
		s, y = -1, -y
	}
	return s * float32((math.Pow(1+mu, float64(y))-1)/mu)
}

func (c *crusher) process(p *Processor, l, r []float32, prm *Params) {
	bits := prm.Bits
	if bits < 1 || bits > 16 {
		bits = 16
	}
	levels := float32(int(1) << (bits - 1))
	if bits == 1 {
		levels = 0.5 // two output values: -1 and +1
	}
	step := clamp(prm.SampleRate, 100, Rate) / Rate
	amt := float32(clamp(prm.DitherAmount, 0, 4))
	mu := prm.Curve == "mu-law"
	quant := func(x float32, e *float32) float32 {
		if bits == 16 {
			return x
		}
		if mu {
			x = muCompress(x)
		}
		v := x * levels
		switch prm.Dither {
		case "rectangular":
			v += (p.rand() - 0.5) * amt
		case "triangular":
			v += (p.rand() + p.rand() - 1) * amt
		case "shaped":
			// Triangular dither plus first-order error feedback, which
			// pushes the quantisation noise up towards the treble where
			// the ear minds it least.
			v += (p.rand()+p.rand()-1)*amt - *e
		}
		q := float32(math.Floor(float64(v) + 0.5))
		if bits == 1 {
			q = 0.5
			if v < 0 {
				q = -0.5
			}
		}
		if prm.Dither == "shaped" {
			*e = clamp32(q-v, -1, 1)
		}
		q /= levels
		if q > 1 {
			q = 1
		} else if q < -1 {
			q = -1
		}
		if mu {
			q = muExpand(q)
		}
		return q
	}
	for i := range l {
		c.phase += step
		if c.phase >= 1 {
			c.phase -= math.Floor(c.phase)
			c.holdL, c.holdR = quant(l[i], &c.errL), quant(r[i], &c.errR)
		}
		l[i], r[i] = c.holdL, c.holdR
	}
}

func clamp32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// --- stutter -----------------------------------------------------------------

// stutter cuts the sound into slices. Now and then it repeats the slice it
// just heard instead of playing on, like a skipping CD; with Reverse it
// plays each slice backwards.
type stutter struct {
	bufL, bufR []float32 // the slice being recorded
	repL, repR []float32 // the slice being repeated
	pos        int
	left       int // repeats still to play
	rpos       int
}

func (s *stutter) process(p *Processor, l, r []float32, prm *Params) {
	ms := prm.StutterMs
	if ms <= 0 {
		ms = 120 // Reverse alone still needs a slice length
	}
	n := int(clamp(ms, 5, 2000) * Rate / 1000)
	if len(s.bufL) != n {
		s.bufL, s.bufR = make([]float32, n), make([]float32, n)
		s.repL, s.repR = make([]float32, n), make([]float32, n)
		s.pos, s.left, s.rpos = 0, 0, 0
	}
	for i := range l {
		inL, inR := l[i], r[i]
		s.bufL[s.pos], s.bufR[s.pos] = inL, inR
		if s.left > 0 {
			j := s.rpos
			if prm.Reverse {
				j = n - 1 - j
			}
			// A short fade at both ends of the slice hides the clicks.
			g := float32(1)
			if e := min(s.rpos, n-1-s.rpos); e < 64 {
				g = float32(e) / 64
			}
			l[i], r[i] = s.repL[j]*g, s.repR[j]*g
			if s.rpos++; s.rpos >= n {
				s.rpos = 0
				s.left--
			}
		}
		if s.pos++; s.pos >= n {
			s.pos = 0
			if s.left == 0 {
				// A slice just ended: repeat it a few times when the dice
				// say so, or play it backwards once when only Reverse is on.
				trigger := prm.StutterMs > 0 && float64(p.rand()) < prm.StutterChance
				if trigger || prm.Reverse {
					s.repL, s.bufL = s.bufL, s.repL
					s.repR, s.bufR = s.bufR, s.repR
					s.left, s.rpos = 1, 0
					if trigger {
						s.left = max(1, min(16, prm.StutterRepeats))
					}
				}
			}
		}
	}
}

// --- noise -------------------------------------------------------------------

// noiser adds what old media add: steady hiss and the pops of a worn record.
type noiser struct {
	pink        [3]float32
	brown       float32
	popL, popR  float32
	rumblePhase float64
}

func (z *noiser) process(p *Processor, l, r []float32, prm *Params) {
	hiss := float32(clamp(prm.Hiss, 0, 1)) * 0.12
	crackle := clamp(prm.Crackle, 0, 1)
	popRate := float32(crackle * crackle * 40 / Rate) // pops per sample
	for i := range l {
		var nz float32
		if hiss > 0 {
			w := p.rand()*2 - 1
			switch prm.NoiseColor {
			case "pink":
				z.pink[0] = 0.99765*z.pink[0] + w*0.0990460
				z.pink[1] = 0.96300*z.pink[1] + w*0.2965164
				z.pink[2] = 0.57000*z.pink[2] + w*1.0526913
				nz = (z.pink[0] + z.pink[1] + z.pink[2] + w*0.1848) * 0.25
			case "brown":
				z.brown = clamp32(z.brown+w*0.02, -1, 1)
				nz = z.brown * 2.5
			default:
				nz = w
			}
			nz *= hiss
		}
		if popRate > 0 {
			if p.rand() < popRate {
				a := (p.rand()*2 - 1) * float32(0.25+crackle*0.5)
				z.popL, z.popR = a, a*(0.6+p.rand()*0.4)
			}
			// Surface noise: a faint crunch under the pops.
			nz += (p.rand()*2 - 1) * (p.rand() * p.rand()) * float32(crackle) * 0.03
			z.popL *= 0.93
			z.popR *= 0.93
		}
		l[i] += nz + z.popL
		r[i] += nz + z.popR
	}
}

// --- reverb ------------------------------------------------------------------

// reverb is Freeverb: eight parallel damped comb filters per channel feed
// four all-pass filters in series. The delay lengths are the published
// ones, scaled from 44.1 to 48 kHz.
type reverb struct {
	combL, combR [8]comb
	apL, apR     [4]allpass
	pre          []float32
	prePos       int
}

type comb struct {
	buf   []float32
	pos   int
	store float32
}

type allpass struct {
	buf []float32
	pos int
}

func newReverb() *reverb {
	combs := [8]int{1116, 1188, 1277, 1356, 1422, 1491, 1557, 1617}
	aps := [4]int{556, 441, 341, 225}
	const spread = 23
	scale := func(n int) int { return n * Rate / 44100 }
	v := &reverb{pre: make([]float32, Rate/4*2)}
	for i, n := range combs {
		v.combL[i].buf = make([]float32, scale(n))
		v.combR[i].buf = make([]float32, scale(n+spread))
	}
	for i, n := range aps {
		v.apL[i].buf = make([]float32, scale(n))
		v.apR[i].buf = make([]float32, scale(n+spread))
	}
	return v
}

func (c *comb) tick(in, feedback, damp float32) float32 {
	out := c.buf[c.pos]
	c.store = out*(1-damp) + c.store*damp
	c.buf[c.pos] = in + c.store*feedback
	if c.pos++; c.pos >= len(c.buf) {
		c.pos = 0
	}
	return out
}

func (a *allpass) tick(in float32) float32 {
	b := a.buf[a.pos]
	a.buf[a.pos] = in + b*0.5
	if a.pos++; a.pos >= len(a.buf) {
		a.pos = 0
	}
	return b - in
}

func (v *reverb) process(l, r []float32, prm *Params) {
	mix := float32(clamp(prm.ReverbMix, 0, 1))
	feedback := float32(0.7 + clamp(prm.ReverbSize, 0, 1)*0.28)
	damp := float32(clamp(prm.ReverbDamp, 0, 1) * 0.4)
	width := float32(clamp(prm.ReverbWidth, 0, 1))
	wet1, wet2 := mix*3*(width/2+0.5), mix*3*((1-width)/2)
	dry := 1 - mix*0.5
	preN := int(clamp(prm.ReverbPre, 0, 240) * Rate / 1000)
	half := len(v.pre) / 2
	for i := range l {
		in := (l[i] + r[i]) * 0.015
		if preN > 0 {
			v.pre[v.prePos] = in
			in = v.pre[(v.prePos-preN+half)%half]
			if v.prePos++; v.prePos >= half {
				v.prePos = 0
			}
		}
		var outL, outR float32
		for k := range v.combL {
			outL += v.combL[k].tick(in, feedback, damp)
			outR += v.combR[k].tick(in, feedback, damp)
		}
		for k := range v.apL {
			outL = v.apL[k].tick(outL)
			outR = v.apR[k].tick(outR)
		}
		l[i] = l[i]*dry + outL*wet1 + outR*wet2
		r[i] = r[i]*dry + outR*wet1 + outL*wet2
	}
}
