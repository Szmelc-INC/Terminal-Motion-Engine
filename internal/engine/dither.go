package engine

import (
	"math"
	"math/rand"
	"runtime"
	"sort"
	"sync"
)

var workers = func() int {
	n := runtime.NumCPU()
	if n > 8 {
		n = 8
	}
	if n < 1 {
		n = 1
	}
	return n
}()

// parallel splits [0,n) into bands and runs fn on each concurrently.
func parallel(n int, fn func(lo, hi int)) {
	if n < 16 || workers == 1 {
		fn(0, n)
		return
	}
	var wg sync.WaitGroup
	chunk := (n + workers - 1) / workers
	for lo := 0; lo < n; lo += chunk {
		hi := lo + chunk
		if hi > n {
			hi = n
		}
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			fn(lo, hi)
		}(lo, hi)
	}
	wg.Wait()
}

func clamp8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// matrix is a tileable threshold map with values in -128..127.
type matrix struct {
	n int
	t []int16
}

// fromScores ranks arbitrary scores into an evenly spread threshold matrix.
// Equal scores share a threshold, which is what makes line patterns solid.
func fromScores(n int, score []float64) *matrix {
	uniq := append([]float64(nil), score...)
	sort.Float64s(uniq)
	k := 0
	for i, v := range uniq {
		if i == 0 || v != uniq[k-1] {
			uniq[k] = v
			k++
		}
	}
	uniq = uniq[:k]
	m := &matrix{n: n, t: make([]int16, n*n)}
	for i, v := range score {
		rank := sort.SearchFloat64s(uniq, v)
		m.t[i] = int16(math.Round(((float64(rank)+0.5)/float64(k) - 0.5) * 256))
	}
	return m
}

func bayer(n int) *matrix {
	b := []int{0}
	for size := 1; size < n; size *= 2 {
		nb := make([]int, size*size*4)
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				v := b[y*size+x] * 4
				nb[y*size*2+x] = v
				nb[y*size*2+x+size] = v + 2
				nb[(y+size)*size*2+x] = v + 3
				nb[(y+size)*size*2+x+size] = v + 1
			}
		}
		b = nb
	}
	s := make([]float64, len(b))
	for i, v := range b {
		s[i] = float64(v)
	}
	return fromScores(n, s)
}

func patternMatrix(n int, f func(x, y int) float64) *matrix {
	s := make([]float64, n*n)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			s[y*n+x] = f(x, y)
		}
	}
	return fromScores(n, s)
}

// blueNoise builds a 64×64 blue-noise threshold map by repeatedly dropping a
// point into the emptiest spot of a toroidal energy field (the "void" half
// of void-and-cluster). The result has no visible structure and, unlike
// error diffusion, never moves between frames.
func blueNoise() *matrix {
	const n = 64
	const sigma = 1.9
	kern := make([]float32, n*n)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx, dy := float64(x), float64(y)
			if dx > n/2 {
				dx = n - dx
			}
			if dy > n/2 {
				dy = n - dy
			}
			kern[y*n+x] = float32(math.Exp(-(dx*dx + dy*dy) / (2 * sigma * sigma)))
		}
	}
	energy := make([]float32, n*n)
	used := make([]bool, n*n)
	score := make([]float64, n*n)
	rng := rand.New(rand.NewSource(7))
	pos := rng.Intn(n * n)
	for rank := 0; rank < n*n; rank++ {
		if rank > 0 {
			best := float32(math.MaxFloat32)
			for i, e := range energy {
				if !used[i] && e < best {
					best, pos = e, i
				}
			}
		}
		used[pos] = true
		score[pos] = float64(rank)
		px, py := pos%n, pos/n
		for y := 0; y < n; y++ {
			row := ((y - py + n) % n) * n
			for x := 0; x < n; x++ {
				energy[y*n+x] += kern[row+(x-px+n)%n]
			}
		}
	}
	return fromScores(n, score)
}

func whiteNoise() *matrix {
	const n = 64
	rng := rand.New(rand.NewSource(11))
	s := make([]float64, n*n)
	for i, p := range rng.Perm(n * n) {
		s[i] = float64(p)
	}
	return fromScores(n, s)
}

var (
	matrices   = map[string]*matrix{}
	matricesMu sync.Mutex
)

var orderedNames = []string{"bayer2", "bayer4", "bayer8", "bluenoise", "whitenoise", "grain", "halftone", "hlines", "vlines", "diagonal"}

func matrixFor(name string) *matrix {
	matricesMu.Lock()
	defer matricesMu.Unlock()
	if m, ok := matrices[name]; ok {
		return m
	}
	var m *matrix
	switch name {
	case "bayer2":
		m = bayer(2)
	case "bayer4":
		m = bayer(4)
	case "bayer8":
		m = bayer(8)
	case "bluenoise":
		m = blueNoise()
	case "whitenoise", "grain":
		m = whiteNoise()
	case "halftone":
		m = patternMatrix(8, func(x, y int) float64 {
			return -(math.Cos(2*math.Pi*(float64(x)+0.5)/8) + math.Cos(2*math.Pi*(float64(y)+0.5)/8)) +
				float64(y*8+x)*1e-6
		})
	case "hlines":
		m = patternMatrix(4, func(x, y int) float64 { return float64([]int{0, 2, 1, 3}[y]) })
	case "vlines":
		m = patternMatrix(4, func(x, y int) float64 { return float64([]int{0, 2, 1, 3}[x]) })
	case "diagonal":
		m = patternMatrix(4, func(x, y int) float64 { return float64([]int{0, 2, 1, 3}[(x+y)%4]) })
	}
	matrices[name] = m
	return m
}

type tap struct{ dx, dy, w int }

type kernel struct {
	taps []tap
	div  int
}

var kernels = map[string]*kernel{
	"floyd-steinberg": {[]tap{{1, 0, 7}, {-1, 1, 3}, {0, 1, 5}, {1, 1, 1}}, 16},
	"atkinson":        {[]tap{{1, 0, 1}, {2, 0, 1}, {-1, 1, 1}, {0, 1, 1}, {1, 1, 1}, {0, 2, 1}}, 8},
	"jjn": {[]tap{{1, 0, 7}, {2, 0, 5}, {-2, 1, 3}, {-1, 1, 5}, {0, 1, 7}, {1, 1, 5}, {2, 1, 3},
		{-2, 2, 1}, {-1, 2, 3}, {0, 2, 5}, {1, 2, 3}, {2, 2, 1}}, 48},
	"stucki": {[]tap{{1, 0, 8}, {2, 0, 4}, {-2, 1, 2}, {-1, 1, 4}, {0, 1, 8}, {1, 1, 4}, {2, 1, 2},
		{-2, 2, 1}, {-1, 2, 2}, {0, 2, 4}, {1, 2, 2}, {2, 2, 1}}, 42},
	"burkes": {[]tap{{1, 0, 8}, {2, 0, 4}, {-2, 1, 2}, {-1, 1, 4}, {0, 1, 8}, {1, 1, 4}, {2, 1, 2}}, 32},
	"sierra": {[]tap{{1, 0, 5}, {2, 0, 3}, {-2, 1, 2}, {-1, 1, 4}, {0, 1, 5}, {1, 1, 4}, {2, 1, 2},
		{-1, 2, 2}, {0, 2, 3}, {1, 2, 2}}, 32},
	"sierra2":     {[]tap{{1, 0, 4}, {2, 0, 3}, {-2, 1, 1}, {-1, 1, 2}, {0, 1, 3}, {1, 1, 2}, {2, 1, 1}}, 16},
	"sierra-lite": {[]tap{{1, 0, 2}, {-1, 1, 1}, {0, 1, 1}}, 4},
}

var diffusionNames = []string{"floyd-steinberg", "atkinson", "jjn", "stucki", "burkes", "sierra", "sierra2", "sierra-lite"}

// DitherNames lists every dithering algorithm: "none", the ordered
// (frame-stable) patterns, then the error-diffusion kernels.
func DitherNames() []string {
	out := []string{"none"}
	out = append(out, orderedNames...)
	return append(out, diffusionNames...)
}

// IsDiffusion reports whether the algorithm is an error-diffusion kernel.
func IsDiffusion(name string) bool { return kernels[name] != nil }

// ditherer carries scratch buffers between frames.
type ditherer struct {
	err [3][]int32
}

func (d *ditherer) rows(n int) [3][]int32 {
	for i := range d.err {
		if cap(d.err[i]) < n {
			d.err[i] = make([]int32, n)
		}
		d.err[i] = d.err[i][:n]
		clear(d.err[i])
	}
	return d.err
}

func grainOffset(name string, frame int) (int, int) {
	if name != "grain" {
		return 0, 0
	}
	h := uint32(frame)*2654435761 + 12345
	return int(h >> 8 & 63), int(h >> 16 & 63)
}

// rgb quantises an RGB24 image to the palette in place.
func (d *ditherer) rgb(pix []byte, w, h int, pal *Palette, l *Look, frame int) {
	amount := l.DitherAmount
	if amount <= 0 {
		amount = 0
	}
	if m := matrixFor(l.Dither); m != nil && amount > 0 {
		spread := int(float64(pal.spread()) * amount)
		ox, oy := grainOffset(l.Dither, frame)
		parallel(h, func(lo, hi int) {
			for y := lo; y < hi; y++ {
				trow := m.t[((y+oy)%m.n)*m.n:]
				p := pix[y*w*3 : (y+1)*w*3]
				for x := 0; x < w; x++ {
					t := int(trow[(x+ox)%m.n]) * spread >> 8
					c := pal.Colors[pal.Index(clamp8(int(p[0])+t), clamp8(int(p[1])+t), clamp8(int(p[2])+t))]
					p[0], p[1], p[2] = c.R, c.G, c.B
					p = p[3:]
				}
			}
		})
		return
	}
	if k := kernels[l.Dither]; k != nil && amount > 0 {
		amt := int(amount * 256)
		rows := d.rows((w + 4) * 3)
		for y := 0; y < h; y++ {
			rev := l.Serpentine && y&1 == 1
			cur := rows[0]
			for i := 0; i < w; i++ {
				x, dir := i, 1
				if rev {
					x, dir = w-1-i, -1
				}
				p := pix[(y*w+x)*3:]
				e := cur[(x+2)*3:]
				r := clamp8(int(p[0]) + int(e[0])/k.div)
				g := clamp8(int(p[1]) + int(e[1])/k.div)
				b := clamp8(int(p[2]) + int(e[2])/k.div)
				c := pal.Colors[pal.Index(r, g, b)]
				p[0], p[1], p[2] = c.R, c.G, c.B
				er := int32((int(r) - int(c.R)) * amt >> 8)
				eg := int32((int(g) - int(c.G)) * amt >> 8)
				eb := int32((int(b) - int(c.B)) * amt >> 8)
				for _, t := range k.taps {
					o := rows[t.dy][(x+2+t.dx*dir)*3:]
					o[0] += er * int32(t.w)
					o[1] += eg * int32(t.w)
					o[2] += eb * int32(t.w)
				}
			}
			clear(rows[0])
			rows[0], rows[1], rows[2] = rows[1], rows[2], rows[0]
		}
		return
	}
	parallel(h, func(lo, hi int) {
		p := pix[lo*w*3 : hi*w*3]
		for len(p) >= 3 {
			c := pal.Colors[pal.Index(p[0], p[1], p[2])]
			p[0], p[1], p[2] = c.R, c.G, c.B
			p = p[3:]
		}
	})
}

// gray quantises a luma plane to the given number of levels, writing level
// indices (0..levels-1) to out.
func (d *ditherer) gray(lum []uint8, w, h, levels int, l *Look, frame int, out []uint8) {
	if levels < 2 {
		clear(out[:w*h])
		return
	}
	top := levels - 1
	amount := l.DitherAmount
	if m := matrixFor(l.Dither); m != nil && amount > 0 {
		spread := int(255 / float64(top) * amount)
		ox, oy := grainOffset(l.Dither, frame)
		parallel(h, func(lo, hi int) {
			for y := lo; y < hi; y++ {
				trow := m.t[((y+oy)%m.n)*m.n:]
				for x := 0; x < w; x++ {
					v := int(clamp8(int(lum[y*w+x]) + int(trow[(x+ox)%m.n])*spread>>8))
					out[y*w+x] = uint8((v*top + 127) / 255)
				}
			}
		})
		return
	}
	if k := kernels[l.Dither]; k != nil && amount > 0 {
		amt := int(amount * 256)
		rows := d.rows(w + 4)
		for y := 0; y < h; y++ {
			rev := l.Serpentine && y&1 == 1
			cur := rows[0]
			for i := 0; i < w; i++ {
				x, dir := i, 1
				if rev {
					x, dir = w-1-i, -1
				}
				v := int(clamp8(int(lum[y*w+x]) + int(cur[x+2])/k.div))
				lv := (v*top + 127) / 255
				out[y*w+x] = uint8(lv)
				e := int32((v - lv*255/top) * amt >> 8)
				for _, t := range k.taps {
					rows[t.dy][x+2+t.dx*dir] += e * int32(t.w)
				}
			}
			clear(rows[0])
			rows[0], rows[1], rows[2] = rows[1], rows[2], rows[0]
		}
		return
	}
	for i, v := range lum[:w*h] {
		out[i] = uint8((int(v)*top + 127) / 255)
	}
}
