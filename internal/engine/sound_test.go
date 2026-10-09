package engine

import (
	"encoding/json"
	"math/rand"
	"os/exec"
	"strings"
	"testing"
)

func TestOptionKeysAreUnique(t *testing.T) {
	seen := map[string]string{}
	groups := map[string]bool{}
	for _, g := range Groups {
		groups[g] = true
	}
	for _, o := range Options {
		if prev, dup := seen[o.Key]; dup {
			t.Errorf("option key %q is used by both %q and %q", o.Key, prev, o.Group+"/"+o.Label)
		}
		seen[o.Key] = o.Group + "/" + o.Label
		if !groups[o.Group] {
			t.Errorf("option %q is in group %q, which is not listed in Groups", o.Key, o.Group)
		}
		if (o.Kind == KFloat || o.Kind == KInt) && (o.Max <= o.Min || o.Step <= 0) {
			t.Errorf("option %q has a bad range %v..%v step %v", o.Key, o.Min, o.Max, o.Step)
		}
	}
}

func TestDefaultSoundIsClean(t *testing.T) {
	s := DefaultSound()
	if c := s.Chain(true); c != "" {
		t.Errorf("the default sound builds a filter chain: %s", c)
	}
	if s.SoundActive() || SoundSummary(s) != "clean" {
		t.Errorf("the default sound is reported as active (%q)", SoundSummary(s))
	}
	// Every audio option must sit inside its own range at the defaults, or
	// the first nudge in the menu would jump.
	set := DefaultSettings()
	for _, o := range Options {
		if !strings.HasPrefix(o.Group, "Audio") || (o.Kind != KFloat && o.Kind != KInt) {
			continue
		}
		before := o.String(&set)
		if err := o.Set(&set, before); err != nil {
			t.Errorf("default of %q is outside its range: %v", o.Key, err)
		}
	}
}

func TestSoundJSONKeepsDefaults(t *testing.T) {
	// A preset saved before a setting existed must load with that setting
	// at its neutral value, not at zero.
	s := DefaultSound()
	if err := json.Unmarshal([]byte(`{"bass": 6, "synth_mode": "chip"}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Bass != 6 || s.SynthMode != "chip" {
		t.Errorf("fields from the file were lost: %+v", s)
	}
	if s.Lowpass != 20000 || s.Bits != 16 || s.SampleRate != 48000 || s.Width != 1 || s.SynthMix != 1 {
		t.Errorf("missing fields did not keep their defaults: lowpass %v bits %v rate %v width %v mix %v",
			s.Lowpass, s.Bits, s.SampleRate, s.Width, s.SynthMix)
	}
	data, _ := json.Marshal(DefaultSound())
	var back Sound
	if err := json.Unmarshal(data, &back); err != nil || back != DefaultSound() {
		t.Errorf("round trip changed the sound: %v", err)
	}
}

func TestChainPieces(t *testing.T) {
	s := DefaultSound()
	s.Bass, s.Lowpass, s.Pitch, s.Karaoke, s.Limiter = 6, 800, 12, true, true
	c := s.Chain(true)
	for _, want := range []string{"bass=g=6", "lowpass=f=800", "rubberband=pitch=2", "pan=stereo|c0=c0-c1", "alimiter="} {
		if !strings.Contains(c, want) {
			t.Errorf("chain %q lacks %q", c, want)
		}
	}
	if strings.Index(c, "pan=") > strings.Index(c, "lowpass") || !strings.HasSuffix(c, "level=0") {
		t.Errorf("order is off — channel mixing first, limiter last: %s", c)
	}
	// Without rubberband the pitch is shifted by resampling, and the
	// length is put back with atempo so sound and picture stay in step.
	c = s.Chain(false)
	if !strings.Contains(c, "asetrate=96000,aresample=48000,atempo=0.5") {
		t.Errorf("fallback pitch shift: %s", c)
	}
	s = DefaultSound()
	s.Pitch = -24 // far outside what one atempo stage can undo
	if c := s.Chain(false); !strings.Contains(c, "atempo=2.0,atempo=2") {
		t.Errorf("large pitch shift must chain atempo stages: %s", c)
	}
}

// Every chain termo can build must be one ffmpeg accepts. This runs the
// real ffmpeg on a fraction of a second of silence per chain.
func TestChainsAreValidFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	check := func(name, chain string) {
		if chain == "" {
			return
		}
		out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i",
			"anullsrc=r=48000:cl=stereo", "-t", "0.2", "-af", chain, "-f", "null", "-").CombinedOutput()
		if err != nil {
			t.Errorf("%s: ffmpeg rejects the chain\n  %s\n  %s", name, chain, strings.TrimSpace(string(out)))
		}
	}
	for _, p := range BuiltinSounds {
		check(p.Name, p.Sound.Chain(true))
		check(p.Name+" (no rubberband)", p.Sound.Chain(false))
	}
	// Each ffmpeg-side option on its own, at both ends of its range.
	for _, o := range Options {
		if !strings.HasPrefix(o.Group, "Audio") {
			continue
		}
		for _, end := range []int{0, 1} {
			set := DefaultSettings()
			switch o.Kind {
			case KFloat, KInt:
				o.SetFrac(&set, float64(end))
			case KBool:
				if end == 0 {
					continue
				}
				o.Nudge(&set, 1)
			case KEnum:
				for i := range o.Choices {
					o.Nudge(&set, 1)
					drv := withDrive(set.Sound)
					check(o.Key+"="+o.Choices[i], drv.Chain(true))
				}
				continue
			}
			check(o.Key+" at "+o.String(&set), set.Sound.Chain(true))
		}
	}
	// Dependent settings only show up once their effect is on.
	all := DefaultSound()
	all.Comp, all.Gate, all.Limiter, all.Normalize, all.Drive = true, true, true, true, 10
	all.EchoDelay, all.ChorusMix, all.FlangerDepth, all.PhaserRate, all.TremRate, all.VibRate, all.PanRate = 300, 0.5, 4, 0.5, 5, 5, 0.3
	all.Bandpass, all.Pitch, all.FreqShift, all.Wow, all.Flutter, all.Width = 1000, 3, 100, 0.5, 0.5, 2
	check("everything on", all.Chain(true))
	rng := rand.New(rand.NewSource(5))
	for i := 0; i < 40; i++ {
		check("random sound", RandomSound(rng).Chain(true))
	}
}

func withDrive(s Sound) Sound {
	s.Drive = 10 // the clip curve only matters with drive on
	return s
}

func TestBuiltinSounds(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range BuiltinSounds {
		if seen[p.Name] || !p.Builtin || p.Name == "" {
			t.Errorf("sound preset %q is duplicated or malformed", p.Name)
		}
		seen[p.Name] = true
		if p.Name != "clean" && !p.Sound.SoundActive() {
			t.Errorf("sound preset %q does nothing", p.Name)
		}
		// Presets must stay inside the ranges the menu offers.
		set := DefaultSettings()
		set.Sound = p.Sound
		for _, o := range Options {
			if strings.HasPrefix(o.Group, "Audio") && o.Kind != KBool {
				if err := o.Set(&set, o.String(&set)); err != nil {
					t.Errorf("sound preset %q: %v", p.Name, err)
				}
			}
		}
	}
	if len(BuiltinSounds) < 40 {
		t.Errorf("only %d built-in sounds", len(BuiltinSounds))
	}
}
