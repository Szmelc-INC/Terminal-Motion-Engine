package engine

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
)

// FX is the effects part of a look: a color-grade filter, detail controls
// and the retro / glitch effects. It is embedded in Look, and it can also
// be saved and applied on its own as an effect preset, on top of any look.
type FX struct {
	Filter       string  `json:"filter"`
	FilterAmount float64 `json:"filter_amount"`

	Blur      float64 `json:"blur"`
	Sharpen   float64 `json:"sharpen"`
	Glow      float64 `json:"glow"`
	Pixelate  int     `json:"pixelate"`
	Posterize int     `json:"posterize"`
	Solarize  float64 `json:"solarize"`
	Grain     float64 `json:"grain"`
	Vignette  float64 `json:"vignette"`
	VigSize   float64 `json:"vignette_size"`
	Mirror    string  `json:"mirror"`

	Scanlines  float64 `json:"scanlines"`
	ScanGap    int     `json:"scan_gap"`
	Curvature  float64 `json:"curvature"`
	Mask       string  `json:"mask"`
	Split      float64 `json:"split"`
	SplitMode  string  `json:"split_mode"`
	VHS        float64 `json:"vhs"`
	Bleed      float64 `json:"bleed"`
	Tracking   float64 `json:"tracking"`
	Glitch     float64 `json:"glitch"`
	Jitter     float64 `json:"jitter"`
	Wave       float64 `json:"wave"`
	WaveFreq   float64 `json:"wave_freq"`
	Interlace  bool    `json:"interlace"`
	MotionBlur float64 `json:"motion_blur"`
	HueCycle   float64 `json:"hue_cycle"`
}

// Choices for the enumerated effect settings.
var (
	Mirrors    = []string{"off", "left", "right", "top", "bottom", "quad", "kaleidoscope"}
	Masks      = []string{"off", "aperture", "shadow", "slot"}
	SplitModes = []string{"rgb", "anaglyph", "phase"}
)

// DefaultFX has every effect off.
func DefaultFX() FX {
	return FX{Filter: "none", FilterAmount: 1, Pixelate: 1, VigSize: 0.5, Mirror: "off", ScanGap: 2, Mask: "off",
		SplitMode: "rgb", WaveFreq: 3}
}

// Active reports whether any effect changes the picture.
func (f *FX) Active() bool {
	return f.spatial() || f.Blur > 0 || f.Sharpen > 0 || f.Glow > 0 || f.Pixelate > 1 || f.Split != 0 ||
		(f.Filter != "none" && f.Filter != "" && f.FilterAmount > 0) || f.Posterize > 1 || f.Solarize > 0 ||
		f.VHS > 0 || f.Bleed > 0 || f.Grain > 0 || f.Scanlines > 0 || (f.Mask != "off" && f.Mask != "") ||
		f.Interlace || f.Vignette > 0 || f.MotionBlur > 0 || f.HueCycle != 0
}

// spatial reports whether pixels move (mirror, curvature, row shifts).
func (f *FX) spatial() bool {
	return (f.Mirror != "off" && f.Mirror != "") || f.Curvature > 0 || f.Wave > 0 || f.Jitter > 0 || f.Glitch > 0 ||
		f.VHS > 0 || f.Tracking > 0
}

// FXSummary names the effects in use.
func FXSummary(f FX) string {
	var p []string
	add := func(on bool, name string) {
		if on {
			p = append(p, name)
		}
	}
	add(f.Filter != "none" && f.Filter != "" && f.FilterAmount > 0, f.Filter)
	add(f.VHS > 0 || f.Tracking > 0, "vhs")
	add(f.Bleed > 0 && f.VHS == 0, "bleed")
	add(f.Curvature > 0 || (f.Mask != "off" && f.Mask != ""), "crt")
	add(f.Scanlines > 0, "scanlines")
	add(f.Glitch > 0, "glitch")
	add(f.Jitter > 0 && f.Glitch == 0, "jitter")
	splitName := "rgb split"
	switch f.SplitMode {
	case "anaglyph":
		splitName = "3d anaglyph"
	case "phase":
		splitName = "phase split"
	}
	add(f.Split != 0, splitName)
	add(f.Posterize > 1, "posterize")
	add(f.Solarize > 0, "solarize")
	add(f.Pixelate > 1, "pixelate")
	add(f.Blur > 0, "blur")
	add(f.Sharpen > 0, "sharpen")
	add(f.Glow > 0, "glow")
	add(f.MotionBlur > 0, "trails")
	add(f.Wave > 0, "wave")
	add(f.Mirror != "off" && f.Mirror != "", "mirror")
	add(f.Grain > 0, "grain")
	add(f.Vignette > 0, "vignette")
	add(f.Interlace, "interlace")
	add(f.HueCycle != 0, "hue cycle")
	if len(p) == 0 {
		return "none"
	}
	if len(p) > 4 {
		p = append(p[:4], fmt.Sprintf("+%d", len(p)-4))
	}
	return strings.Join(p, " · ")
}

// --- color grades -------------------------------------------------------------

// grade is a color-grade filter: a color matrix, then a curve per channel,
// or a gradient that replaces the colors according to brightness.
type grade struct {
	name  string
	mat   []float64                       // 3×3 row-major, plus optional 3 offsets (0..1)
	curve func(ch int, v float64) float64 // applied after the matrix
	ramp  []RGB                           // gradient map over luma
}

func sCurve(v, k float64) float64 { // k > 0 adds contrast, < 0 flattens
	return clampF(v+k*math.Sin(2*math.Pi*v)*-0.16, 0, 1)
}

func desat(amount float64) []float64 { // 1 = fully grey
	a := 1 - amount
	return []float64{
		0.299*amount + a, 0.587 * amount, 0.114 * amount,
		0.299 * amount, 0.587*amount + a, 0.114 * amount,
		0.299 * amount, 0.587 * amount, 0.114*amount + a,
	}
}

var grades = []grade{
	{name: "mono", mat: desat(1)},
	{name: "noir", mat: desat(1), curve: func(_ int, v float64) float64 { return sCurve(clampF(v*1.08-0.04, 0, 1), 1.6) }},
	{name: "sepia", mat: []float64{0.393, 0.769, 0.189, 0.349, 0.686, 0.168, 0.272, 0.534, 0.131}},
	{name: "vintage", mat: desat(0.35), curve: func(ch int, v float64) float64 {
		return clampF(0.07+v*[]float64{0.95, 0.9, 0.78}[ch], 0, 1)
	}},
	{name: "faded", mat: desat(0.25), curve: func(_ int, v float64) float64 { return 0.12 + v*0.78 }},
	{name: "polaroid", mat: []float64{1.438, -0.062, -0.062, -0.122, 1.378, -0.122, -0.016, -0.016, 1.483, -0.03, 0.05, -0.02}},
	{name: "kodachrome", mat: []float64{1.128, -0.396, -0.039, -0.164, 1.083, -0.054, -0.167, -0.560, 1.601, 0.249, 0.097, 0.139},
		curve: func(_ int, v float64) float64 { return sCurve(v, 0.6) }},
	{name: "technicolor", mat: []float64{1.912, -0.854, -0.091, -0.308, 1.765, -0.106, -0.231, -0.750, 1.847, 0.046, -0.275, 0.121}},
	{name: "xpro", curve: func(ch int, v float64) float64 {
		switch ch {
		case 0:
			return sCurve(v, 1.5)
		case 1:
			return sCurve(clampF(v*1.05, 0, 1), 0.8)
		}
		return 0.14 + v*0.62 // blue: lifted shadows, held-back highlights
	}},
	{name: "lomo", curve: func(ch int, v float64) float64 { return sCurve(clampF(v*[]float64{1.08, 1.02, 0.94}[ch], 0, 1), 1.8) }},
	{name: "bleach", mat: desat(0.55), curve: func(_ int, v float64) float64 { return sCurve(v, 1.7) }},
	{name: "teal-orange", curve: func(ch int, v float64) float64 {
		switch ch {
		case 0:
			return clampF(v+0.18*v*v-0.05*(1-v), 0, 1)
		case 1:
			return clampF(v+0.04*(1-v)*v, 0, 1)
		}
		return clampF(v+0.14*(1-v)*(1-v)-0.16*v*v, 0, 1)
	}},
	{name: "cinema", mat: desat(0.12), curve: func(ch int, v float64) float64 {
		v = sCurve(v, 0.9)
		return clampF(v+[]float64{0.05, 0, -0.05}[ch]*(2*v-1), 0, 1)
	}},
	{name: "cool", curve: func(ch int, v float64) float64 { return clampF(v*[]float64{0.88, 0.98, 1.14}[ch], 0, 1) }},
	{name: "warm", curve: func(ch int, v float64) float64 { return clampF(v*[]float64{1.14, 1.02, 0.84}[ch], 0, 1) }},
	{name: "golden-hour", mat: desat(-0.2), curve: func(ch int, v float64) float64 {
		return clampF(v*[]float64{1.16, 1.0, 0.72}[ch]+[]float64{0.03, 0.01, 0}[ch], 0, 1)
	}},
	{name: "infrared", mat: []float64{0.2, 1.3, -0.4, 1.1, 0.2, -0.2, -0.1, 0.4, 0.9}},
	{name: "negative", curve: func(_ int, v float64) float64 { return 1 - v }},
	{name: "cyanotype", ramp: hexes("04111f 0c2f52 16639a 5aa7d0 c4e6f3 f4fbfd")},
	{name: "thermal", ramp: hexes("000004 2d0a5a 8a1c7c d43d51 f98e09 f9d423 ffffe0")},
	{name: "night-vision", ramp: hexes("000300 062a08 12701a 3ed44a b8ffbe")},
	{name: "x-ray", ramp: hexes("e8f6ff 9fc4dc 4d7691 1b3345 050b12")},
	{name: "gold", ramp: hexes("120a02 5a3a08 a8760f e3b73b fff3b8")},
	{name: "redscale", ramp: hexes("050000 5a0500 c22a00 ff8a1c ffe08a")},
	{name: "duotone", ramp: hexes("1b0b3a 6d1f7a e0457b ffb347 fff7c2")},
	{name: "acid", ramp: hexes("0b0033 7a00ff ff00aa ffea00 00ffb3 e8fff8")},
}

// FilterNames lists the color-grade filters, "none" first.
func FilterNames() []string {
	out := []string{"none"}
	for _, g := range grades {
		out = append(out, g.name)
	}
	return out
}

// gradeLUT is a grade compiled for the pixel loop.
type gradeLUT struct {
	useMat bool
	mat    [9]int32
	off    [3]int32
	lut    [3][256]uint8
	useLUT bool
	ramp   [256][3]uint8
	isRamp bool
}

var (
	gradeCache   = map[string]*gradeLUT{}
	gradeCacheMu sync.Mutex
)

func compileGrade(name string) *gradeLUT {
	gradeCacheMu.Lock()
	defer gradeCacheMu.Unlock()
	if g, ok := gradeCache[name]; ok {
		return g
	}
	var out *gradeLUT
	for _, g := range grades {
		if g.name != name {
			continue
		}
		out = &gradeLUT{}
		if len(g.mat) >= 9 {
			out.useMat = true
			for i := 0; i < 9; i++ {
				out.mat[i] = int32(math.Round(g.mat[i] * 4096))
			}
			if len(g.mat) >= 12 {
				for i := 0; i < 3; i++ {
					out.off[i] = int32(math.Round(g.mat[9+i] * 255))
				}
			}
		}
		if g.curve != nil {
			out.useLUT = true
			for ch := 0; ch < 3; ch++ {
				for i := 0; i < 256; i++ {
					out.lut[ch][i] = uint8(clampF(g.curve(ch, float64(i)/255), 0, 1)*255 + 0.5)
				}
			}
		}
		if len(g.ramp) > 0 {
			out.isRamp = true
			for i, c := range Gradient(g.ramp, 256) {
				out.ramp[i] = [3]uint8{c.R, c.G, c.B}
			}
		}
	}
	gradeCache[name] = out
	return out
}

func hash32(a, b, c uint32) uint32 {
	h := a*0x9e3779b1 ^ b*0x85ebca6b ^ c*0xc2b2ae35
	h ^= h >> 15
	h *= 0x2c1b3c6d
	h ^= h >> 12
	h *= 0x297a2d39
	h ^= h >> 15
	return h
}

// noise returns a repeatable value in [-1,1) for a position and frame.
func noise(x, y, frame int) float64 {
	return float64(hash32(uint32(x), uint32(y), uint32(frame))>>8)/(1<<23) - 1
}

// --- the effect chain ---------------------------------------------------------

// applyFX runs the effects over the working picture, in the order light
// would meet them: geometry first, then optics, color, the tape, the tube.
func (r *Renderer) applyFX(w, h int, l *Look, frame int) {
	f := &l.FX
	if !f.Active() {
		r.prevOK = false
		return
	}
	if f.spatial() {
		r.remap(w, h, f, frame)
	}
	if f.Pixelate > 1 {
		r.pixelate(w, h, min(f.Pixelate, 64))
	}
	if f.Blur > 0 {
		r.blur(r.work, w, h, f.Blur)
	}
	if f.Sharpen > 0 || f.Glow > 0 {
		r.sharpenGlow(w, h, f)
	}
	if f.Split != 0 {
		r.split(w, h, f, frame)
	}
	if f.VHS > 0 || f.Bleed > 0 {
		r.tape(w, h, f, frame)
	}
	r.colorFX(w, h, f, frame)
	if f.Interlace || f.MotionBlur > 0 {
		r.temporal(w, h, f, frame)
	} else {
		r.prevOK = false
	}
}

// scratch returns a copy of the working picture to read from while the
// original is rewritten.
func (r *Renderer) scratch(n int) []byte {
	r.tmp = grow(r.tmp, n)
	copy(r.tmp, r.work[:n])
	return r.tmp
}

// remap moves pixels: mirrors, the bulge of a picture tube, and the
// sideways row shifts of wave, jitter, glitch and tape-tracking errors.
func (r *Renderer) remap(w, h int, f *FX, frame int) {
	src := r.scratch(w * h * 3)
	dst := r.work
	if cap(r.shift) < h {
		r.shift = make([]int, h)
	}
	shift := r.shift[:h]
	fw := float64(w)
	// Glitch: a few bands jump sideways, on some frames only.
	type band struct{ y0, y1, dx int }
	var bands []band
	if f.Glitch > 0 {
		step := frame / 3 // a glitch lasts a few frames
		if float64(hash32(7, 0, uint32(step))%1000)/1000 < 0.25+f.Glitch*0.6 {
			n := 1 + int(f.Glitch*5)
			for i := 0; i < n; i++ {
				hv := hash32(uint32(i), 11, uint32(step))
				y0 := int(hv % uint32(h))
				bh := 1 + int(hv>>8%uint32(max(2, h/6)))
				dx := int((float64(hv>>16%2000)/1000 - 1) * f.Glitch * fw * 0.35)
				bands = append(bands, band{y0, min(h, y0+bh), dx})
			}
		}
	}
	// Tape tracking: a noisy band that rolls up the picture.
	trackY, trackH := -1, 0
	track := math.Max(f.Tracking, f.VHS*0.5)
	if track > 0 {
		trackH = max(1, int(float64(h)*(0.03+0.1*track)))
		period := 90 + int(160*(1-track))
		trackY = h - (frame*max(1, h/period+1))%(h+trackH*4)
	}
	headRows := 0
	if f.VHS > 0 {
		headRows = max(1, int(float64(h)*0.035*f.VHS+0.5)) // head-switching noise at the bottom
	}
	for y := 0; y < h; y++ {
		dx := 0.0
		if f.Wave > 0 {
			dx += f.Wave * fw * 0.08 * math.Sin(2*math.Pi*(float64(y)/float64(h)*f.WaveFreq+float64(frame)*0.02))
		}
		if f.Jitter > 0 {
			dx += noise(3, y, frame) * f.Jitter * fw * 0.02
		}
		if f.VHS > 0 {
			dx += noise(5, y/2, frame) * f.VHS * fw * 0.006
			if y >= h-headRows {
				dx += (0.5 + noise(9, y, frame)*0.5) * fw * 0.12 * f.VHS
			}
		}
		if trackY >= 0 && y >= trackY && y < trackY+trackH {
			dx += (noise(13, y, frame)*0.5 + 0.5) * fw * 0.08 * track
		}
		for _, b := range bands {
			if y >= b.y0 && y < b.y1 {
				dx += float64(b.dx)
			}
		}
		shift[y] = int(math.Round(dx))
	}
	k := f.Curvature * 0.35
	parallel(h, func(lo, hi int) {
		for y := lo; y < hi; y++ {
			drow := dst[y*w*3 : (y+1)*w*3]
			for x := 0; x < w; x++ {
				sx, sy := x, y
				switch f.Mirror {
				case "left":
					if sx >= w/2 {
						sx = w - 1 - sx
					}
				case "right":
					if sx < w/2 {
						sx = w - 1 - sx
					}
				case "top":
					if sy >= h/2 {
						sy = h - 1 - sy
					}
				case "bottom":
					if sy < h/2 {
						sy = h - 1 - sy
					}
				case "quad", "kaleidoscope":
					if sx >= w/2 {
						sx = w - 1 - sx
					}
					if sy >= h/2 {
						sy = h - 1 - sy
					}
					if f.Mirror == "kaleidoscope" {
						// Fold along the diagonal as well: eight-fold symmetry.
						u, v := float64(sx)/float64(w), float64(sy)/float64(h)
						if v > u {
							sx, sy = int(v*float64(w)), int(u*float64(h))
						}
					}
				}
				if k > 0 {
					u := (float64(sx)+0.5)/float64(w)*2 - 1
					v := (float64(sy)+0.5)/float64(h)*2 - 1
					s := (1 + k*(u*u+v*v)) / (1 + k) * (1 + k*0.9)
					u, v = u*s, v*s
					if u < -1 || u >= 1 || v < -1 || v >= 1 {
						drow[x*3], drow[x*3+1], drow[x*3+2] = 0, 0, 0
						continue
					}
					sx, sy = int((u+1)/2*float64(w)), int((v+1)/2*float64(h))
				}
				sx -= shift[y]
				if sx < 0 || sx >= w {
					sx = ((sx % w) + w) % w // shifted rows wrap around, as on tape
				}
				sy = max(0, min(h-1, sy))
				p := src[(sy*w+sx)*3:]
				drow[x*3], drow[x*3+1], drow[x*3+2] = p[0], p[1], p[2]
			}
		}
	})
}

func (r *Renderer) pixelate(w, h, n int) {
	work := r.work
	parallel((h+n-1)/n, func(lo, hi int) {
		for by := lo; by < hi; by++ {
			y0, y1 := by*n, min(h, by*n+n)
			for x0 := 0; x0 < w; x0 += n {
				x1 := min(w, x0+n)
				var sr, sg, sb, cnt int
				for y := y0; y < y1; y++ {
					p := work[(y*w+x0)*3:]
					for x := x0; x < x1; x++ {
						sr += int(p[0])
						sg += int(p[1])
						sb += int(p[2])
						p = p[3:]
						cnt++
					}
				}
				cr, cg, cb := uint8(sr/cnt), uint8(sg/cnt), uint8(sb/cnt)
				for y := y0; y < y1; y++ {
					p := work[(y*w+x0)*3:]
					for x := x0; x < x1; x++ {
						p[0], p[1], p[2] = cr, cg, cb
						p = p[3:]
					}
				}
			}
		}
	})
}

// blur is two passes of a box blur in each direction, which is close to a
// Gaussian. pix is blurred in place.
func (r *Renderer) blur(pix []byte, w, h int, radius float64) {
	rad := int(math.Round(radius))
	if rad < 1 {
		rad = 1
	}
	r.tmp2 = grow(r.tmp2, w*h*3)
	tmp := r.tmp2
	for pass := 0; pass < 2; pass++ {
		// Horizontal: pix -> tmp.
		parallel(h, func(lo, hi int) {
			for y := lo; y < hi; y++ {
				row := pix[y*w*3 : (y+1)*w*3]
				out := tmp[y*w*3 : (y+1)*w*3]
				for c := 0; c < 3; c++ {
					sum, n := 0, 0
					for x := 0; x <= rad && x < w; x++ {
						sum += int(row[x*3+c])
						n++
					}
					for x := 0; x < w; x++ {
						out[x*3+c] = uint8(sum / n)
						if a := x + rad + 1; a < w {
							sum += int(row[a*3+c])
							n++
						}
						if b := x - rad; b >= 0 {
							sum -= int(row[b*3+c])
							n--
						}
					}
				}
			}
		})
		// Vertical: tmp -> pix.
		parallel(w, func(lo, hi int) {
			for x := lo; x < hi; x++ {
				for c := 0; c < 3; c++ {
					sum, n := 0, 0
					for y := 0; y <= rad && y < h; y++ {
						sum += int(tmp[(y*w+x)*3+c])
						n++
					}
					for y := 0; y < h; y++ {
						pix[(y*w+x)*3+c] = uint8(sum / n)
						if a := y + rad + 1; a < h {
							sum += int(tmp[(a*w+x)*3+c])
							n++
						}
						if b := y - rad; b >= 0 {
							sum -= int(tmp[(b*w+x)*3+c])
							n--
						}
					}
				}
			}
		})
	}
}

// sharpenGlow does unsharp masking (add back the difference from a blurred
// copy) and bloom (add a blurred copy of the highlights).
func (r *Renderer) sharpenGlow(w, h int, f *FX) {
	n := w * h * 3
	r.soft = grow(r.soft, n)
	soft, work := r.soft, r.work
	copy(soft, work[:n])
	rad := 1.0
	if f.Glow > 0 {
		rad = math.Max(2, float64(min(w, h))/40)
	}
	r.blur(soft, w, h, rad)
	sharp := int(f.Sharpen * 256)
	glow := int(f.Glow * 256)
	parallel(h, func(lo, hi int) {
		for i := lo * w * 3; i < hi*w*3; i++ {
			v, s := int(work[i]), int(soft[i])
			out := v + (v-s)*sharp>>8
			if glow > 0 && s > 110 {
				out += (s - 110) * glow >> 7
			}
			work[i] = clamp8(out)
		}
	})
}

// split pulls the color channels apart sideways: chromatic aberration, a
// red-cyan 3D anaglyph, or a split that swings with time and row.
func (r *Renderer) split(w, h int, f *FX, frame int) {
	src := r.scratch(w * h * 3)
	work := r.work
	base := f.Split * float64(w) * 0.012
	parallel(h, func(lo, hi int) {
		for y := lo; y < hi; y++ {
			d := base
			if f.SplitMode == "phase" {
				d = base * math.Sin(float64(frame)*0.13+float64(y)/float64(h)*5)
			}
			di := int(math.Round(d))
			if di == 0 && d != 0 {
				di = 1
				if d < 0 {
					di = -1
				}
			}
			row := src[y*w*3 : (y+1)*w*3]
			out := work[y*w*3 : (y+1)*w*3]
			for x := 0; x < w; x++ {
				xa := max(0, min(w-1, x+di))
				xb := max(0, min(w-1, x-di))
				if f.SplitMode == "anaglyph" {
					// Left eye in red, right eye in cyan, both from luma so
					// the picture reads as depth rather than as fringes.
					a, b := row[xa*3:], row[xb*3:]
					out[x*3] = Luma(a[0], a[1], a[2])
					lb := Luma(b[0], b[1], b[2])
					out[x*3+1], out[x*3+2] = lb, lb
					continue
				}
				out[x*3] = row[xa*3]
				out[x*3+2] = row[xb*3+2]
			}
		}
	})
}

// tape gives the picture what a worn VHS gives it: color that smears
// sideways and lags behind the brightness, washed-out contrast, noise in
// the tracking band and at the bottom edge.
func (r *Renderer) tape(w, h int, f *FX, frame int) {
	src := r.scratch(w * h * 3)
	work := r.work
	amt := math.Max(f.Bleed, f.VHS*0.7)
	rad := max(1, int(amt*float64(w)*0.02+0.5))
	lag := max(1, rad/2)
	vhs := f.VHS
	headRows := 0
	if vhs > 0 {
		headRows = max(1, int(float64(h)*0.035*vhs+0.5))
	}
	parallel(h, func(lo, hi int) {
		for y := lo; y < hi; y++ {
			row := src[y*w*3 : (y+1)*w*3]
			out := work[y*w*3 : (y+1)*w*3]
			// Running average of the color-difference signals, taken from a
			// window that sits lag pixels to the left: the color trails to
			// the right of the edges that cause it.
			var su, sv, n int
			for x := -lag - rad; x <= -lag+rad; x++ {
				if x >= 0 && x < w {
					p := row[x*3:]
					yy := int(Luma(p[0], p[1], p[2]))
					su += int(p[0]) - yy
					sv += int(p[2]) - yy
					n++
				}
			}
			for x := 0; x < w; x++ {
				p := row[x*3:]
				yy := int(Luma(p[0], p[1], p[2]))
				cu, cv := su/max(1, n), sv/max(1, n)
				if vhs > 0 {
					yy = yy*(256-int(vhs*40))>>8 + int(vhs*14)                // lifted blacks, lower whites
					yy += int(noise(x, y, frame) * vhs * 18)                  // luma noise
					cu, cv = cu*(256-int(vhs*50))>>8, cv*(256-int(vhs*50))>>8 // weaker color
					if y >= h-headRows && hash32(uint32(x/3), uint32(y), uint32(frame))%3 == 0 {
						yy = 200 + int(noise(x, y, frame+1)*55)
						cu, cv = 0, 0
					}
				}
				rr := yy + cu
				bb := yy + cv
				gg := (yy*256 - 77*rr - 29*bb) / 150
				out[x*3], out[x*3+1], out[x*3+2] = clamp8(rr), clamp8(gg), clamp8(bb)
				if a := x + 1 - lag + rad; a >= 0 && a < w {
					q := row[a*3:]
					qy := int(Luma(q[0], q[1], q[2]))
					su += int(q[0]) - qy
					sv += int(q[2]) - qy
					n++
				}
				if b := x - lag - rad; b >= 0 && b < w {
					q := row[b*3:]
					qy := int(Luma(q[0], q[1], q[2]))
					su -= int(q[0]) - qy
					sv -= int(q[2]) - qy
					n--
				}
			}
		}
	})
}

// colorFX is everything that needs only the pixel itself and where it is:
// the grade filter, posterize, solarize, grain, scanlines, the shadow mask
// and the vignette.
func (r *Renderer) colorFX(w, h int, f *FX, frame int) {
	var g *gradeLUT
	amt := int(clampF(f.FilterAmount, 0, 1) * 256)
	if f.Filter != "none" && f.Filter != "" && amt > 0 {
		g = compileGrade(f.Filter)
	}
	var levels [256]uint8
	useLevels := f.Posterize > 1 || f.Solarize > 0
	if useLevels {
		n := f.Posterize
		thr := int((1 - clampF(f.Solarize, 0, 1)) * 255)
		for i := range levels {
			v := i
			if f.Solarize > 0 && v > thr {
				v = 255 - v
			}
			if n > 1 {
				v = (v*(n-1) + 127) / 255 * 255 / (n - 1)
			}
			levels[i] = uint8(v)
		}
	}
	grain := int(f.Grain * 70)
	scan := int(clampF(f.Scanlines, 0, 1) * 220)
	gap := max(2, f.ScanGap)
	vig := f.Vignette
	inner := clampF(f.VigSize, 0, 1.2)
	maskOn := f.Mask != "off" && f.Mask != ""
	if g == nil && !useLevels && grain == 0 && scan == 0 && vig <= 0 && !maskOn {
		return
	}
	work := r.work
	parallel(h, func(lo, hi int) {
		for y := lo; y < hi; y++ {
			row := work[y*w*3 : (y+1)*w*3]
			rowGain := 256
			if scan > 0 && y%gap == gap-1 {
				rowGain = 256 - scan
			}
			v := (float64(y)+0.5)/float64(h)*2 - 1
			for x := 0; x < w; x++ {
				p := row[x*3 : x*3+3]
				cr, cg, cb := int(p[0]), int(p[1]), int(p[2])
				if g != nil {
					nr, ng, nb := cr, cg, cb
					if g.isRamp {
						c := g.ramp[Luma(p[0], p[1], p[2])]
						nr, ng, nb = int(c[0]), int(c[1]), int(c[2])
					} else {
						if g.useMat {
							ri, gi, bi := int32(cr), int32(cg), int32(cb)
							nr = int(clamp8(int((g.mat[0]*ri+g.mat[1]*gi+g.mat[2]*bi)>>12 + g.off[0])))
							ng = int(clamp8(int((g.mat[3]*ri+g.mat[4]*gi+g.mat[5]*bi)>>12 + g.off[1])))
							nb = int(clamp8(int((g.mat[6]*ri+g.mat[7]*gi+g.mat[8]*bi)>>12 + g.off[2])))
						}
						if g.useLUT {
							nr, ng, nb = int(g.lut[0][nr]), int(g.lut[1][ng]), int(g.lut[2][nb])
						}
					}
					cr += (nr - cr) * amt >> 8
					cg += (ng - cg) * amt >> 8
					cb += (nb - cb) * amt >> 8
				}
				if useLevels {
					cr, cg, cb = int(levels[clamp8(cr)]), int(levels[clamp8(cg)]), int(levels[clamp8(cb)])
				}
				if grain > 0 {
					nz := int(noise(x, y, frame) * float64(grain))
					cr, cg, cb = cr+nz, cg+nz, cb+nz
				}
				gain := rowGain
				if vig > 0 {
					u := (float64(x)+0.5)/float64(w)*2 - 1
					d := math.Sqrt(u*u+v*v) / math.Sqrt2 // 0 centre … 1 corner
					if d > inner {
						t := (d - inner) / math.Max(0.05, 1.05-inner)
						t = t * t * (3 - 2*math.Min(1, t))
						gain = gain * int(256*(1-vig*math.Min(1, t))) >> 8
					}
				}
				if gain != 256 {
					cr, cg, cb = cr*gain>>8, cg*gain>>8, cb*gain>>8
				}
				if maskOn {
					// The colored phosphor stripes of a picture tube.
					xx := x
					switch f.Mask {
					case "shadow":
						xx = x + (y%2)*1
					case "slot":
						xx = x + (y/2%2)*1
						if y%4 == 3 {
							cr, cg, cb = cr*200>>8, cg*200>>8, cb*200>>8
						}
					}
					switch xx % 3 {
					case 0:
						cg, cb = cg*180>>8, cb*180>>8
					case 1:
						cr, cb = cr*180>>8, cb*180>>8
					default:
						cr, cg = cr*180>>8, cg*180>>8
					}
					cr, cg, cb = cr*300>>8, cg*300>>8, cb*300>>8 // win back the brightness the mask costs
				}
				p[0], p[1], p[2] = clamp8(cr), clamp8(cg), clamp8(cb)
			}
		}
	})
}

// temporal mixes in the previous frame: as trails (motion blur) or as the
// comb teeth of an interlaced picture.
func (r *Renderer) temporal(w, h int, f *FX, frame int) {
	n := w * h * 3
	work := r.work
	if !r.prevOK || len(r.prev) != n {
		r.prev = grow(r.prev, n)
		copy(r.prev, work[:n])
		r.prevOK = true
		return
	}
	prev := r.prev
	if f.Interlace {
		for y := frame & 1; y < h; y += 2 {
			a, b := work[y*w*3:(y+1)*w*3], prev[y*w*3:(y+1)*w*3]
			for i := range a {
				a[i], b[i] = b[i], a[i] // show the old line, remember the new one
			}
		}
		for y := 1 - frame&1; y < h; y += 2 {
			copy(prev[y*w*3:(y+1)*w*3], work[y*w*3:(y+1)*w*3])
		}
	}
	if mb := int(clampF(f.MotionBlur, 0, 0.97) * 256); mb > 0 {
		parallel(h, func(lo, hi int) {
			for i := lo * w * 3; i < hi*w*3; i++ {
				v := (int(prev[i])*mb + int(work[i])*(256-mb)) >> 8
				work[i], prev[i] = uint8(v), uint8(v)
			}
		})
	}
}

// --- options and presets ------------------------------------------------------

func init() {
	num := func(group, key, label, help string, lo, hi, step float64, p func(*Look) *float64) {
		Options = append(Options, &Option{Key: key, Label: label, Group: group, Kind: KFloat, Min: lo, Max: hi, Step: step,
			Help: help, ptr: func(s *Settings) any { return p(&s.Look) }})
	}
	whole := func(group, key, label, help string, lo, hi float64, p func(*Look) *int) {
		Options = append(Options, &Option{Key: key, Label: label, Group: group, Kind: KInt, Min: lo, Max: hi, Step: 1,
			Help: help, ptr: func(s *Settings) any { return p(&s.Look) }})
	}
	flag := func(group, key, label, help string, p func(*Look) *bool) {
		Options = append(Options, &Option{Key: key, Label: label, Group: group, Kind: KBool, Help: help,
			ptr: func(s *Settings) any { return p(&s.Look) }})
	}
	enum := func(group, key, label, help string, choices []string, p func(*Look) *string) {
		Options = append(Options, &Option{Key: key, Label: label, Group: group, Kind: KEnum, Choices: choices, Help: help,
			ptr: func(s *Settings) any { return p(&s.Look) }})
	}

	g := "Adjust"
	num(g, "exposure", "Exposure", "brighten or darken in photographic stops", -3, 3, 0.1, func(l *Look) *float64 { return &l.Exposure })
	num(g, "black", "Black point", "everything darker than this becomes black", 0, 0.5, 0.01, func(l *Look) *float64 { return &l.Black })
	num(g, "white", "White point", "everything brighter than this becomes white", 0.5, 1, 0.01, func(l *Look) *float64 { return &l.White })
	num(g, "shadows", "Shadows", "lift or deepen the dark areas only", -1, 1, 0.05, func(l *Look) *float64 { return &l.Shadows })
	num(g, "highlights", "Highlights", "lift or tame the bright areas only", -1, 1, 0.05, func(l *Look) *float64 { return &l.Highlights })
	num(g, "fade", "Fade", "wash the blacks out, like an old print", 0, 1, 0.05, func(l *Look) *float64 { return &l.Fade })
	num(g, "vibrance", "Vibrance", "boost the dull colors and leave the vivid ones", -1, 1, 0.05, func(l *Look) *float64 { return &l.Vibrance })
	num(g, "temperature", "Temperature", "cooler (blue) to warmer (orange)", -1, 1, 0.05, func(l *Look) *float64 { return &l.Temperature })
	num(g, "tint", "Tint", "green to magenta", -1, 1, 0.05, func(l *Look) *float64 { return &l.Tint })

	g = "Filter"
	enum(g, "filter", "Filter", "a color grade: mono, sepia, cross-process, film stocks, heat and night cameras…", FilterNames(), func(l *Look) *string { return &l.Filter })
	num(g, "filter-amount", "Filter amount", "blend between the original colors and the filter", 0, 1, 0.05, func(l *Look) *float64 { return &l.FilterAmount })
	num(g, "blur", "Blur", "soften the picture, radius in pixels", 0, 12, 0.5, func(l *Look) *float64 { return &l.Blur })
	num(g, "sharpen", "Sharpness", "bring out the edges", 0, 4, 0.1, func(l *Look) *float64 { return &l.Sharpen })
	num(g, "glow", "Glow", "let the highlights bloom", 0, 2, 0.05, func(l *Look) *float64 { return &l.Glow })
	whole(g, "pixelate", "Pixelate", "merge pixels into blocks this many pixels wide (1 = off)", 1, 32, func(l *Look) *int { return &l.Pixelate })
	whole(g, "posterize", "Posterize", "keep only this many levels per color channel (0 = off)", 0, 16, func(l *Look) *int { return &l.Posterize })
	num(g, "solarize", "Solarize", "invert the tones above a threshold, the darkroom accident (0 = off)", 0, 1, 0.05, func(l *Look) *float64 { return &l.Solarize })
	num(g, "grain", "Grain", "film grain that changes every frame", 0, 1, 0.05, func(l *Look) *float64 { return &l.Grain })
	num(g, "vignette", "Vignette", "darken the corners", 0, 1, 0.05, func(l *Look) *float64 { return &l.Vignette })
	num(g, "vignette-size", "Vignette size", "how far from the centre the darkening starts", 0, 1, 0.05, func(l *Look) *float64 { return &l.VigSize })
	enum(g, "mirror", "Mirror", "reflect one half onto the other; kaleidoscope folds it eight ways", Mirrors, func(l *Look) *string { return &l.Mirror })

	g = "Effects"
	num(g, "vhs", "VHS tape", "worn tape: color smear, jitter, noise, a tracking band, a torn bottom edge", 0, 1, 0.05, func(l *Look) *float64 { return &l.VHS })
	num(g, "bleed", "Color bleed", "smear the colors sideways, as composite video does", 0, 1, 0.05, func(l *Look) *float64 { return &l.Bleed })
	num(g, "tracking", "Tracking error", "a band of displaced lines rolling up the picture", 0, 1, 0.05, func(l *Look) *float64 { return &l.Tracking })
	num(g, "scanlines", "Scanlines", "darken every n-th line, like a picture tube", 0, 1, 0.05, func(l *Look) *float64 { return &l.Scanlines })
	whole(g, "scanline-gap", "Scanline spacing", "one dark line every this many pixel rows", 2, 8, func(l *Look) *int { return &l.ScanGap })
	num(g, "curvature", "CRT curvature", "bulge the picture like the glass of an old tube", 0, 1, 0.05, func(l *Look) *float64 { return &l.Curvature })
	enum(g, "mask", "CRT mask", "the red, green and blue phosphor pattern of a tube", Masks, func(l *Look) *string { return &l.Mask })
	num(g, "split", "RGB split", "pull the color channels apart sideways", -4, 4, 0.1, func(l *Look) *float64 { return &l.Split })
	enum(g, "split-mode", "Split mode", "rgb = color fringes · anaglyph = red-cyan 3D · phase = a split that swings in time", SplitModes, func(l *Look) *string { return &l.SplitMode })
	num(g, "glitch", "Glitch", "bands of the picture jump sideways now and then", 0, 1, 0.05, func(l *Look) *float64 { return &l.Glitch })
	num(g, "jitter", "Line jitter", "every line shakes sideways a little", 0, 1, 0.05, func(l *Look) *float64 { return &l.Jitter })
	num(g, "wave", "Wave", "bend the picture in a travelling sine wave", 0, 1, 0.05, func(l *Look) *float64 { return &l.Wave })
	num(g, "wave-freq", "Wave count", "how many waves fit the height", 0.5, 12, 0.5, func(l *Look) *float64 { return &l.WaveFreq })
	flag(g, "interlace", "Interlace", "show odd and even lines from different frames: combing on motion", func(l *Look) *bool { return &l.Interlace })
	num(g, "motion-blur", "Motion blur", "blend each frame into the next: trails and ghosts", 0, 0.95, 0.05, func(l *Look) *float64 { return &l.MotionBlur })
	num(g, "hue-cycle", "Hue cycle", "rotate all hues a little more every frame, degrees", 0, 30, 0.5, func(l *Look) *float64 { return &l.HueCycle })

	// Filter and Effects sit right after Adjust in the menus.
	var groups []string
	for _, name := range Groups {
		groups = append(groups, name)
		if name == "Adjust" {
			groups = append(groups, "Filter", "Effects")
		}
	}
	Groups = groups
}

// FXPreset is a named set of effects.
type FXPreset struct {
	Name    string `json:"name"`
	FX      FX     `json:"fx"`
	Builtin bool   `json:"-"`
}

func fx(name string, edit func(*FX)) FXPreset {
	f := DefaultFX()
	edit(&f)
	return FXPreset{Name: name, FX: f, Builtin: true}
}

// BuiltinFX ship with termo.
var BuiltinFX = []FXPreset{
	fx("none", func(f *FX) {}),
	fx("vhs-tape", func(f *FX) { f.VHS, f.Scanlines, f.Vignette = 0.6, 0.15, 0.2 }),
	fx("vhs-worn", func(f *FX) {
		f.VHS, f.Tracking, f.Bleed, f.Jitter, f.Grain, f.Filter, f.FilterAmount = 1, 0.7, 0.8, 0.15, 0.2, "faded", 0.6
	}),
	fx("vhs-pause", func(f *FX) { f.VHS, f.Tracking, f.Jitter, f.Interlace = 0.8, 1, 0.5, true }),
	fx("crt-tv", func(f *FX) {
		f.Scanlines, f.Curvature, f.Mask, f.Vignette, f.Glow, f.Bleed = 0.45, 0.35, "aperture", 0.35, 0.3, 0.25
	}),
	fx("crt-arcade", func(f *FX) {
		f.Scanlines, f.ScanGap, f.Curvature, f.Mask, f.Glow, f.Sharpen = 0.6, 2, 0.2, "slot", 0.5, 0.6
	}),
	fx("crt-terminal", func(f *FX) {
		f.Filter, f.Scanlines, f.Curvature, f.Glow, f.Vignette = "night-vision", 0.5, 0.4, 0.6, 0.4
	}),
	fx("retro-scanlines", func(f *FX) { f.Scanlines, f.ScanGap = 0.55, 2 }),
	fx("old-tv-static", func(f *FX) {
		f.Filter, f.Grain, f.Scanlines, f.Jitter, f.Curvature, f.Vignette = "mono", 0.55, 0.3, 0.2, 0.3, 0.4
	}),
	fx("anaglyph-3d", func(f *FX) { f.Split, f.SplitMode = 1.2, "anaglyph" }),
	fx("rgb-split", func(f *FX) { f.Split, f.SplitMode = 1.5, "rgb" }),
	fx("phase-glitch", func(f *FX) { f.Split, f.SplitMode, f.Jitter, f.Glitch = 2.2, "phase", 0.2, 0.35 }),
	fx("glitch-light", func(f *FX) { f.Glitch, f.Split, f.SplitMode = 0.35, 0.6, "rgb" }),
	fx("glitch-heavy", func(f *FX) {
		f.Glitch, f.Jitter, f.Split, f.SplitMode, f.Posterize, f.Grain = 1, 0.5, 2.5, "phase", 5, 0.15
	}),
	fx("security-cam", func(f *FX) {
		f.Filter, f.Grain, f.Scanlines, f.Vignette, f.Interlace, f.Sharpen = "mono", 0.3, 0.3, 0.3, true, 0.5
	}),
	fx("night-vision", func(f *FX) {
		f.Filter, f.Grain, f.Glow, f.Vignette, f.VigSize, f.Scanlines = "night-vision", 0.45, 0.6, 0.8, 0.3, 0.25
	}),
	fx("thermal-cam", func(f *FX) { f.Filter, f.Blur, f.Posterize = "thermal", 1.5, 12 }),
	fx("x-ray", func(f *FX) { f.Filter, f.Glow, f.Sharpen = "x-ray", 0.4, 1 }),
	fx("old-film", func(f *FX) { f.Filter, f.Grain, f.Vignette, f.Jitter, f.FilterAmount = "sepia", 0.4, 0.55, 0.04, 0.85 }),
	fx("silent-movie", func(f *FX) { f.Filter, f.Grain, f.Vignette, f.VigSize, f.Blur = "noir", 0.5, 0.7, 0.35, 0.5 }),
	fx("noir", func(f *FX) { f.Filter, f.Vignette, f.Grain = "noir", 0.45, 0.15 }),
	fx("polaroid", func(f *FX) { f.Filter, f.Vignette, f.Glow = "polaroid", 0.3, 0.15 }),
	fx("lomo", func(f *FX) { f.Filter, f.Vignette, f.VigSize = "lomo", 0.75, 0.35 }),
	fx("cross-process", func(f *FX) { f.Filter, f.Vignette = "xpro", 0.3 }),
	fx("blockbuster", func(f *FX) { f.Filter, f.Vignette, f.Sharpen = "teal-orange", 0.25, 0.4 }),
	fx("dream", func(f *FX) {
		f.Glow, f.Blur, f.Vignette, f.MotionBlur, f.Filter, f.FilterAmount = 1.1, 1, 0.3, 0.4, "warm", 0.6
	}),
	fx("ghost-trails", func(f *FX) { f.MotionBlur = 0.85 }),
	fx("poster", func(f *FX) { f.Posterize, f.Sharpen = 4, 0.8 }),
	fx("pop-comic", func(f *FX) { f.Posterize, f.Sharpen, f.Filter, f.FilterAmount = 3, 1.5, "kodachrome", 0.7 }),
	fx("solarized-print", func(f *FX) { f.Solarize, f.Filter = 0.5, "mono" }),
	fx("mosaic", func(f *FX) { f.Pixelate = 6 }),
	fx("lofi-webcam", func(f *FX) { f.Pixelate, f.Bleed, f.Grain, f.Filter, f.FilterAmount = 2, 0.4, 0.2, "cool", 0.5 }),
	fx("underwater", func(f *FX) { f.Wave, f.WaveFreq, f.Filter, f.Blur, f.FilterAmount = 0.25, 4, "cool", 0.5, 0.9 }),
	fx("heat-haze", func(f *FX) { f.Wave, f.WaveFreq, f.Filter, f.FilterAmount = 0.12, 9, "golden-hour", 0.7 }),
	fx("trip", func(f *FX) {
		f.HueCycle, f.Wave, f.WaveFreq, f.MotionBlur, f.Filter, f.FilterAmount = 6, 0.3, 2.5, 0.6, "acid", 0.5
	}),
	fx("kaleidoscope", func(f *FX) { f.Mirror, f.HueCycle = "kaleidoscope", 2 }),
	fx("mirror-world", func(f *FX) { f.Mirror = "left" }),
	fx("interlaced", func(f *FX) { f.Interlace, f.Scanlines = true, 0.2 }),
}

// RandomFX rolls a small, watchable set of effects.
func RandomFX(rng *rand.Rand) FX {
	f := DefaultFX()
	r := func(lo, hi, step float64) float64 {
		v := math.Round((lo+rng.Float64()*(hi-lo))/step) * step
		return math.Round(v*100) / 100 // no 0.15000000000000002
	}
	names := FilterNames()
	rolls := []func(){
		func() { f.Filter, f.FilterAmount = names[1+rng.Intn(len(names)-1)], r(0.5, 1, 0.05) },
		func() { f.Filter, f.FilterAmount = names[1+rng.Intn(len(names)-1)], r(0.5, 1, 0.05) },
		func() { f.VHS = r(0.3, 1, 0.05) },
		func() { f.Scanlines, f.ScanGap = r(0.2, 0.6, 0.05), 2+rng.Intn(2) },
		func() { f.Curvature, f.Mask = r(0.15, 0.5, 0.05), Masks[rng.Intn(len(Masks))] },
		func() { f.Split, f.SplitMode = r(0.5, 2.5, 0.1), SplitModes[rng.Intn(len(SplitModes))] },
		func() { f.Glitch = r(0.2, 0.8, 0.05) },
		func() { f.Posterize = 3 + rng.Intn(5) },
		func() { f.Vignette = r(0.25, 0.7, 0.05) },
		func() { f.Grain = r(0.1, 0.45, 0.05) },
		func() { f.Glow = r(0.3, 1, 0.05) },
		func() { f.Sharpen = r(0.4, 1.5, 0.1) },
		func() { f.MotionBlur = r(0.3, 0.8, 0.05) },
		func() { f.Wave, f.WaveFreq = r(0.05, 0.3, 0.05), r(1, 6, 0.5) },
		func() { f.Pixelate = 2 + rng.Intn(4) },
		func() { f.Mirror = Mirrors[1+rng.Intn(len(Mirrors)-1)] },
		func() { f.HueCycle = r(1, 8, 0.5) },
	}
	n := 2 + rng.Intn(3)
	for _, i := range rng.Perm(len(rolls))[:n] {
		rolls[i]()
	}
	return f
}
