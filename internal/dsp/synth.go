package dsp

import "math"

// The resynthesiser listens to the sound, works out its pitch and how loud
// it is, and plays an oscillator in its place. With a square wave snapped
// to semitones that is a chiptune cover of whatever is playing; with a saw
// through a resonant filter it is an acid bass line; with slow detuned
// voices it is a drone.

const (
	decim   = 4            // analysis runs at 12 kHz
	anaRate = Rate / decim // 12000
	anaLen  = 1024         // samples kept for pitch detection (85 ms)
	anaWin  = 480          // samples compared at each lag
	minLag  = anaRate / 1000
	maxLag  = anaRate / 55
)

type synth struct {
	ring    [anaLen]float32
	rpos    int
	dacc    float32
	dcount  int
	lp      float32
	diff    [maxLag + 1]float32
	freq    float64 // last detected pitch, Hz
	conf    float32
	env     float32
	cur     float64 // gliding oscillator frequency, Hz
	phase   [3]float64
	arpT    float64
	svfLow  float32
	svfBand float32
	windLow float32
	windBnd float32
	droneA  float32
}

// DetectPitch estimates the fundamental of a mono signal sampled at rate,
// in Hz, with a confidence from 0 (noise) to 1 (a clean tone). It uses the
// cumulative-mean-normalised difference function of the YIN method.
func DetectPitch(x []float32, rate float64) (hz float64, confidence float32) {
	lo, hi := int(rate/1000), int(rate/55)
	w := len(x) - hi
	if w < 32 || lo < 2 {
		return 0, 0
	}
	diff := make([]float32, hi+1)
	return yin(x, diff, lo, hi, w, rate)
}

func yin(x, diff []float32, lo, hi, w int, rate float64) (float64, float32) {
	var sum float32
	best, bestV := -1, float32(1e9)
	diff[0] = 1
	for tau := 1; tau <= hi; tau++ {
		var d float32
		for i := 0; i < w; i++ {
			e := x[i] - x[i+tau]
			d += e * e
		}
		sum += d
		if sum <= 0 {
			diff[tau] = 1
			continue
		}
		diff[tau] = d * float32(tau) / sum
	}
	// The first dip below the threshold is the period; taking the global
	// minimum instead would often land an octave too low.
	for tau := lo; tau <= hi; tau++ {
		if diff[tau] < 0.15 {
			for tau+1 <= hi && diff[tau+1] < diff[tau] {
				tau++
			}
			best, bestV = tau, diff[tau]
			break
		}
		if diff[tau] < bestV {
			best, bestV = tau, diff[tau]
		}
	}
	if best <= 0 {
		return 0, 0
	}
	// Parabolic interpolation between lags refines the estimate.
	t := float64(best)
	if best > 1 && best < hi {
		a, b, c := float64(diff[best-1]), float64(diff[best]), float64(diff[best+1])
		if den := a - 2*b + c; den != 0 {
			t += 0.5 * (a - c) / den
		}
	}
	conf := 1 - bestV
	if conf < 0 {
		conf = 0
	}
	return rate / t, conf
}

var scaleSteps = map[string][]int{
	"major":      {0, 2, 4, 5, 7, 9, 11},
	"minor":      {0, 2, 3, 5, 7, 8, 10},
	"pentatonic": {0, 3, 5, 7, 10},
	"blues":      {0, 3, 5, 6, 7, 10},
}

// Snap moves a frequency to the nearest note of a scale in the given key.
// "free" leaves it alone; "chromatic" snaps to the nearest semitone.
func Snap(hz float64, scale, key string) float64 {
	if hz <= 0 || scale == "free" || scale == "" {
		return hz
	}
	note := 69 + 12*math.Log2(hz/440) // MIDI note number
	n := math.Round(note)
	if steps, ok := scaleSteps[scale]; ok {
		root := 0
		for i, k := range Keys {
			if k == key {
				root = i
			}
		}
		best, bestD := n, math.MaxFloat64
		for oct := -1; oct <= 1; oct++ {
			base := math.Floor((note-float64(root))/12)*12 + float64(root) + float64(oct*12)
			for _, s := range steps {
				c := base + float64(s)
				if d := math.Abs(c - note); d < bestD {
					best, bestD = c, d
				}
			}
		}
		n = best
	}
	return 440 * math.Pow(2, (n-69)/12)
}

var arpPatterns = map[string][]float64{
	"octave": {1, 2},
	"fifth":  {1, 1.5, 2},
	"major":  {1, 1.2599, 1.4983, 2},
	"minor":  {1, 1.1892, 1.4983, 2},
}

func osc(wave string, ph float64) float32 {
	ph -= math.Floor(ph)
	switch wave {
	case "pulse":
		if ph < 0.25 {
			return 1
		}
		return -1
	case "saw":
		return float32(2*ph - 1)
	case "triangle":
		if ph < 0.5 {
			return float32(4*ph - 1)
		}
		return float32(3 - 4*ph)
	case "sine":
		return float32(math.Sin(2 * math.Pi * ph))
	}
	if ph < 0.5 {
		return 1
	}
	return -1
}

func (s *synth) process(p *Processor, l, r []float32, prm *Params) {
	// Analysis: feed a decimated mono copy into the ring buffer and take
	// the loudness of this chunk.
	var energy float32
	for i := range l {
		m := (l[i] + r[i]) * 0.5
		energy += m * m
		s.lp += (m - s.lp) * 0.3 // tame what would alias at 12 kHz
		s.dacc += s.lp
		if s.dcount++; s.dcount == decim {
			s.ring[s.rpos] = s.dacc / decim
			s.rpos = (s.rpos + 1) % anaLen
			s.dacc, s.dcount = 0, 0
		}
	}
	rms := float32(math.Sqrt(float64(energy / float32(len(l)))))
	var lin [anaLen]float32
	for i := range lin {
		lin[i] = s.ring[(s.rpos+i)%anaLen]
	}
	gate := float32(math.Pow(10, clamp(prm.SynthSens, -80, 0)/20))
	if rms > gate {
		if hz, conf := yin(lin[:], s.diff[:], minLag, maxLag, min(anaWin, anaLen-maxLag), anaRate); conf > 0.35 && hz > 50 && hz < 1100 {
			s.freq, s.conf = hz, conf
		}
	}
	if s.freq == 0 {
		s.freq = 110
	}
	target := s.freq * math.Pow(2, float64(max(-3, min(3, prm.SynthOctave))))
	if prm.SynthMode == "drone" {
		// A drone sits low and ignores passing notes: one octave down,
		// always snapped at least to a semitone.
		target /= 2
		for target > 180 {
			target /= 2
		}
		scale := prm.SynthScale
		if scale == "free" {
			scale = "chromatic"
		}
		target = Snap(target, scale, prm.SynthKey)
	} else {
		target = Snap(target, prm.SynthScale, prm.SynthKey)
	}
	if s.cur <= 0 {
		s.cur = target
	}
	glideMs := clamp(prm.SynthGlide, 0, 2000)
	if prm.SynthMode == "drone" {
		glideMs = math.Max(glideMs, 600)
	}
	glide := 1.0
	if glideMs > 0.5 {
		glide = 1 - math.Exp(-1/(glideMs/1000*Rate))
	}
	arp := arpPatterns[prm.Arp]
	arpStep := clamp(prm.ArpRate, 0.5, 60) / Rate
	mix := float32(clamp(prm.SynthMix, 0, 1))
	detune := math.Pow(2, clamp(prm.SynthDetune, 0, 100)/1200)
	cutoff, res, envAmt := clamp(prm.AcidCutoff, 40, 8000), float32(clamp(prm.AcidRes, 0, 0.98)), clamp(prm.AcidEnv, 0, 1)

	for i := range l {
		// Envelope follower: fast up, slower down.
		if rms > s.env {
			s.env += (rms - s.env) * 0.01
		} else {
			s.env += (rms - s.env) * 0.0008
		}
		amp := s.env * 2.2
		if s.env < gate*0.7 {
			amp = 0
		}
		s.cur += (target - s.cur) * glide
		f := s.cur
		if arp != nil {
			s.arpT += arpStep
			f *= arp[int(s.arpT)%len(arp)]
			if s.arpT > 1e6 {
				s.arpT = 0
			}
		}
		var out float32
		switch prm.SynthMode {
		case "acid":
			s.phase[0] += f / Rate
			saw := osc("saw", s.phase[0])
			if prm.SynthWave == "square" || prm.SynthWave == "pulse" {
				saw = osc(prm.SynthWave, s.phase[0])
			}
			// A state-variable low-pass whose cutoff follows the loudness:
			// the squelch of a 303.
			fc := cutoff * (1 + envAmt*float64(min(1, amp*3))*8)
			g := float32(2 * math.Sin(math.Pi*math.Min(fc, 9000)/Rate))
			q := 1.05 - res
			s.svfLow += g * s.svfBand
			high := saw - s.svfLow - q*s.svfBand
			s.svfBand += g * high
			out = float32(math.Tanh(float64(s.svfLow*1.6))) * min(1, amp*1.5)
		case "drone":
			var sum float32
			for v, ratio := range []float64{1, detune, 1.5 / detune} {
				s.phase[v] += f * ratio / Rate
				sum += osc("saw", s.phase[v]) + osc("sine", s.phase[v]*0.5)
			}
			// The drone swells slowly and never quite dies away.
			want := min(1, s.env*6)
			if want < 0.25 && s.env > gate*0.2 {
				want = 0.25
			}
			s.droneA += (want - s.droneA) * 0.00004
			s.svfLow += (sum*0.16 - s.svfLow) * 0.06 // dark low-pass
			out = s.svfLow * s.droneA * 1.8
		case "wind":
			// Noise through a band-pass at the detected pitch: a whistle.
			nz := p.rand()*2 - 1
			g := float32(2 * math.Sin(math.Pi*math.Min(f*2, 9000)/Rate))
			s.windLow += g * s.windBnd
			high := nz - s.windLow - 0.08*s.windBnd
			s.windBnd += g * high
			out = s.windBnd * 0.35 * min(1, amp*1.5)
		default: // chip
			s.phase[0] += f / Rate
			// Sound chips had a 4-bit volume register; so does this one.
			vol := float32(math.Round(float64(min(1, amp*1.4))*15)) / 15
			out = osc(prm.SynthWave, s.phase[0]) * vol * 0.5
		}
		for v := range s.phase {
			if s.phase[v] > 1e6 {
				s.phase[v] -= 1e6
			}
		}
		l[i] += (out - l[i]) * mix
		r[i] += (out - r[i]) * mix
	}
}
