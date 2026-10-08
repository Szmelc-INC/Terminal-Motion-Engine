package dsp

import (
	"encoding/binary"
	"math"
	"testing"
)

// tone returns seconds of a stereo sine wave as s16le PCM.
func tone(hz, seconds, amp float64) []byte {
	n := int(seconds * Rate)
	out := make([]byte, n*4)
	for i := 0; i < n; i++ {
		v := uint16(int16(math.Sin(2*math.Pi*hz*float64(i)/Rate) * amp * 32767))
		binary.LittleEndian.PutUint16(out[i*4:], v)
		binary.LittleEndian.PutUint16(out[i*4+2:], v)
	}
	return out
}

func samples(pcm []byte, ch int) []float64 {
	out := make([]float64, len(pcm)/4)
	for i := range out {
		out[i] = float64(int16(binary.LittleEndian.Uint16(pcm[i*4+ch*2:]))) / 32768
	}
	return out
}

func rms(x []float64) float64 {
	var s float64
	for _, v := range x {
		s += v * v
	}
	return math.Sqrt(s / float64(len(x)))
}

// run processes pcm in 20 ms chunks, the way the player feeds it.
func run(p *Processor, pcm []byte, prm Params) []byte {
	out := append([]byte(nil), pcm...)
	const chunk = Rate / 50 * 4
	for i := 0; i < len(out); i += chunk {
		p.Process(out[i:min(len(out), i+chunk)], &prm)
	}
	return out
}

// goertzel measures how much of one frequency a signal holds.
func goertzel(x []float64, hz float64) float64 {
	w := 2 * math.Pi * hz / Rate
	c := 2 * math.Cos(w)
	var s1, s2 float64
	for _, v := range x {
		s1, s2 = v+c*s1-s2, s1
	}
	return math.Sqrt(s1*s1+s2*s2-c*s1*s2) / float64(len(x)) * 2
}

func TestDefaultsAreTransparent(t *testing.T) {
	d := Defaults()
	if d.Active() {
		t.Fatal("the default settings must not change the sound")
	}
	in := tone(440, 0.2, 0.5)
	out := run(NewProcessor(), in, d)
	for i := range in {
		if in[i] != out[i] {
			t.Fatalf("byte %d changed with default settings", i)
		}
	}
}

func TestBitDepth(t *testing.T) {
	in := tone(440, 0.5, 0.9)
	for _, bits := range []int{1, 2, 4, 8} {
		prm := Defaults()
		prm.Bits = bits
		got := samples(run(NewProcessor(), in, prm), 0)
		levels := map[float64]bool{}
		for _, v := range got {
			levels[math.Round(v*32768)] = true
		}
		// b bits give 2^b output values, plus one because the scale is
		// symmetrical around zero.
		if want := 1<<bits + 1; len(levels) > want || len(levels) < 2 {
			t.Errorf("%d bits: %d distinct levels, want at most %d", bits, len(levels), want)
		}
		if r := rms(got); r < 0.3 || r > 1 {
			t.Errorf("%d bits: level changed too much (rms %.2f)", bits, r)
		}
	}
}

// Dither trades distortion for noise: a tone too quiet for the bit depth
// vanishes without it and survives with it.
func TestDitherKeepsQuietSignals(t *testing.T) {
	in := tone(1000, 1, 0.02) // below half a step at 4 bits
	prm := Defaults()
	prm.Bits = 4
	plain := samples(run(NewProcessor(), in, prm), 0)
	if rms(plain) != 0 {
		t.Fatalf("without dither a tone below the first step should round to silence (rms %g)", rms(plain))
	}
	for _, kind := range []string{"rectangular", "triangular", "shaped"} {
		prm.Dither = kind
		got := samples(run(NewProcessor(), in, prm), 0)
		if g := goertzel(got, 1000); g < 0.01 {
			t.Errorf("%s dither: the 1 kHz tone is gone (%.4f)", kind, g)
		}
	}
}

func TestSampleRateReduction(t *testing.T) {
	in := tone(200, 0.2, 0.8)
	prm := Defaults()
	prm.SampleRate = 4800 // every value is held for 10 samples
	got := samples(run(NewProcessor(), in, prm), 0)
	changes := 0
	for i := 1; i < len(got); i++ {
		if got[i] != got[i-1] {
			changes++
		}
	}
	if max := len(got)/10 + 2; changes > max {
		t.Errorf("%d value changes in %d samples, want at most %d", changes, len(got), max)
	}
	if changes < len(got)/20 {
		t.Errorf("only %d value changes: the signal is frozen", changes)
	}
}

func TestMuLawRoundTrip(t *testing.T) {
	for _, x := range []float32{-1, -0.5, -0.01, 0, 0.001, 0.3, 1} {
		if y := muExpand(muCompress(x)); math.Abs(float64(y-x)) > 1e-4 {
			t.Errorf("mu-law round trip of %v gives %v", x, y)
		}
	}
	// Companding spends the steps on quiet sounds: a quiet tone keeps more
	// of its shape at 8 bits than with a linear scale.
	in := tone(500, 0.3, 0.01)
	prm := Defaults()
	prm.Bits = 8
	lin := goertzel(samples(run(NewProcessor(), in, prm), 0), 500)
	prm.Curve = "mu-law"
	mu := goertzel(samples(run(NewProcessor(), in, prm), 0), 500)
	if mu < 0.008 || mu > 0.012 {
		t.Errorf("mu-law changed a quiet tone's level: %.4f (linear %.4f, input 0.0100)", mu, lin)
	}
}

func TestRingModulator(t *testing.T) {
	// Multiplying by a carrier replaces the tone with the sum and
	// difference frequencies.
	in := tone(1000, 0.5, 0.5)
	prm := Defaults()
	prm.RingFreq, prm.RingMix = 300, 1
	got := samples(run(NewProcessor(), in, prm), 0)
	if g := goertzel(got, 1000); g > 0.02 {
		t.Errorf("the original 1 kHz tone is still there (%.3f)", g)
	}
	for _, hz := range []float64{700, 1300} {
		if g := goertzel(got, hz); g < 0.2 {
			t.Errorf("sideband at %v Hz is missing (%.3f)", hz, g)
		}
	}
}

func TestReverbTail(t *testing.T) {
	in := append(tone(440, 0.2, 0.8), make([]byte, Rate*4)...) // a burst, then silence
	prm := Defaults()
	prm.ReverbMix, prm.ReverbSize = 0.5, 0.8
	got := samples(run(NewProcessor(), in, prm), 0)
	tail := rms(got[Rate/2 : Rate*3/4]) // 0.3 s after the burst ended
	late := rms(got[len(got)-Rate/10:])
	if tail < 0.002 {
		t.Errorf("no reverb tail after the sound stopped (rms %.5f)", tail)
	}
	if late >= tail {
		t.Errorf("the tail does not decay: %.5f then %.5f", tail, late)
	}
	for i, v := range got {
		if math.IsNaN(v) || math.Abs(v) > 1 {
			t.Fatalf("sample %d is out of range: %v", i, v)
		}
	}
}

func TestBalance(t *testing.T) {
	in := tone(440, 0.1, 0.5)
	prm := Defaults()
	prm.Balance = -1
	out := run(NewProcessor(), in, prm)
	if l, r := rms(samples(out, 0)), rms(samples(out, 1)); l < 0.3 || r != 0 {
		t.Errorf("balance fully left: left %.3f right %.3f", l, r)
	}
}

func TestStutterRepeatsSlices(t *testing.T) {
	// A rising ramp makes repeats easy to see: wherever the output steps
	// back, a slice is being replayed.
	n := Rate
	in := make([]byte, n*4)
	for i := 0; i < n; i++ {
		v := uint16(int16(i * 30000 / n))
		binary.LittleEndian.PutUint16(in[i*4:], v)
		binary.LittleEndian.PutUint16(in[i*4+2:], v)
	}
	prm := Defaults()
	prm.StutterMs, prm.StutterChance, prm.StutterRepeats = 50, 1, 2
	got := samples(run(NewProcessor(), in, prm), 0)
	// Slice edges are faded, so compare across the fade, not sample to
	// sample.
	back := 0
	for i := Rate / 10; i < len(got); i++ {
		if got[i] < got[i-200]-0.02 {
			back++
		}
	}
	if back < 1000 {
		t.Errorf("the ramp steps back for only %d samples in a second of 50 ms stutter", back)
	}
	prm.StutterChance = 0
	plain := samples(run(NewProcessor(), in, prm), 0)
	for i := 1; i < len(plain); i++ {
		if plain[i] < plain[i-1] {
			t.Fatalf("with 0 %% chance the sound must pass straight through (sample %d)", i)
		}
	}
}

func TestDetectPitch(t *testing.T) {
	for _, hz := range []float64{82.4, 110, 220, 440, 880} {
		x := make([]float32, 1024)
		for i := range x {
			// A tone with harmonics, like a voice or an instrument.
			ph := 2 * math.Pi * hz * float64(i) / 12000
			x[i] = float32(0.6*math.Sin(ph) + 0.3*math.Sin(2*ph) + 0.1*math.Sin(3*ph))
		}
		got, conf := DetectPitch(x, 12000)
		if math.Abs(got-hz)/hz > 0.02 || conf < 0.8 {
			t.Errorf("%v Hz detected as %.1f Hz (confidence %.2f)", hz, got, conf)
		}
	}
	noise := make([]float32, 1024)
	p := NewProcessor()
	for i := range noise {
		noise[i] = p.rand()*2 - 1
	}
	if _, conf := DetectPitch(noise, 12000); conf > 0.6 {
		t.Errorf("noise was taken for a tone (confidence %.2f)", conf)
	}
}

func TestSnap(t *testing.T) {
	cents := func(a, b float64) float64 { return math.Abs(1200 * math.Log2(a/b)) }
	if got := Snap(450, "chromatic", "C"); cents(got, 440) > 1 {
		t.Errorf("450 Hz should snap to A (440), got %.1f", got)
	}
	if got := Snap(450, "free", "C"); got != 450 {
		t.Errorf("free must not move the pitch, got %.1f", got)
	}
	// C# is not in C major: it moves to C or D.
	got := Snap(277.18, "major", "C")
	if cents(got, 261.63) > 1 && cents(got, 293.66) > 1 {
		t.Errorf("C# in C major snapped to %.1f Hz", got)
	}
	// Every snapped note of a scale is a member of that scale.
	for hz := 60.0; hz < 1000; hz *= 1.037 {
		n := int(math.Round(69+12*math.Log2(Snap(hz, "pentatonic", "A")/440))) - 69 // semitones from A
		ok := false
		for _, s := range scaleSteps["pentatonic"] {
			ok = ok || ((n%12)+12)%12 == s
		}
		if !ok {
			t.Fatalf("%.1f Hz snapped to a note %d semitones from A, outside A pentatonic", hz, n)
		}
	}
}

// The chip synth must play the note it hears, as a square wave.
func TestChipSynthFollowsPitch(t *testing.T) {
	in := tone(220, 1.5, 0.5)
	prm := Defaults()
	prm.SynthMode, prm.SynthGlide = "chip", 0
	got := samples(run(NewProcessor(), in, prm), 0)[Rate/2:] // let the tracker settle
	if r := rms(got); r < 0.1 {
		t.Fatalf("the synth is silent (rms %.3f)", r)
	}
	fund, off := goertzel(got, 220), goertzel(got, 300)
	if fund < 0.2 || off > fund/5 {
		t.Errorf("energy at 220 Hz %.3f, at 300 Hz %.3f: the synth is off pitch", fund, off)
	}
	// A square wave has a strong third harmonic and no second.
	if h3, h2 := goertzel(got, 660), goertzel(got, 440); h3 < fund/5 || h2 > fund/5 {
		t.Errorf("not a square wave: 2nd harmonic %.3f, 3rd %.3f, fundamental %.3f", h2, h3, fund)
	}
	// One octave up.
	prm.SynthOctave = 1
	up := samples(run(NewProcessor(), in, prm), 0)[Rate/2:]
	if a, b := goertzel(up, 440), goertzel(up, 220); a < 0.2 || b > a/5 {
		t.Errorf("octave +1: 440 Hz %.3f, 220 Hz %.3f", a, b)
	}
}

func TestEverySynthModeIsSane(t *testing.T) {
	in := append(tone(196, 1, 0.6), make([]byte, Rate/2*4)...)
	for _, mode := range SynthModes[1:] {
		for _, arp := range Arps {
			prm := Defaults()
			prm.SynthMode, prm.Arp = mode, arp
			got := samples(run(NewProcessor(), in, prm), 0)
			loud := rms(got[Rate/2 : Rate])
			if loud < 0.01 {
				t.Errorf("%s/%s: silent while the input plays (rms %.4f)", mode, arp, loud)
			}
			for i, v := range got {
				if math.IsNaN(v) || math.Abs(v) > 1 {
					t.Fatalf("%s/%s: sample %d out of range: %v", mode, arp, i, v)
				}
			}
		}
	}
	// Silence in, silence out: the synth is gated by the input's loudness.
	prm := Defaults()
	prm.SynthMode = "chip"
	if r := rms(samples(run(NewProcessor(), make([]byte, Rate*4), prm), 0)); r != 0 {
		t.Errorf("the chip synth makes sound out of silence (rms %.4f)", r)
	}
}

func TestNoiseIsAddedOnTop(t *testing.T) {
	silence := make([]byte, Rate*4)
	for _, color := range NoiseColors {
		prm := Defaults()
		prm.Hiss, prm.NoiseColor = 0.5, color
		if r := rms(samples(run(NewProcessor(), silence, prm), 0)); r < 0.001 || r > 0.2 {
			t.Errorf("%s hiss at 50 %%: rms %.4f", color, r)
		}
	}
	prm := Defaults()
	prm.Crackle = 1
	got := samples(run(NewProcessor(), silence, prm), 0)
	pops := 0
	for i := 1; i < len(got); i++ {
		if math.Abs(got[i]) > 0.15 && math.Abs(got[i-1]) <= 0.15 {
			pops++
		}
	}
	if pops < 5 || pops > 200 {
		t.Errorf("%d pops in one second of full crackle", pops)
	}
}

func BenchmarkProcessEverything(b *testing.B) {
	prm := Defaults()
	prm.SynthMode, prm.RingFreq, prm.StutterMs, prm.Bits, prm.SampleRate = "acid", 30, 80, 8, 12000
	prm.Hiss, prm.Crackle, prm.ReverbMix, prm.Dither = 0.2, 0.3, 0.4, "shaped"
	p := NewProcessor()
	chunk := tone(220, 0.02, 0.5)
	b.SetBytes(int64(len(chunk)))
	for i := 0; i < b.N; i++ {
		p.Process(chunk, &prm)
	}
}
