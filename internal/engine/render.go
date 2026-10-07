package engine

import "math"

// Cell is one terminal character cell.
type Cell struct {
	Ch     rune
	Fg, Bg uint32
	Attr   uint8
}

// ColDefault selects the terminal's own foreground/background color.
const ColDefault uint32 = 1 << 24

// Cell attributes.
const (
	AttrBold uint8 = 1 << iota
	AttrDim
	AttrUnderline
	AttrReverse
)

var (
	halfGlyphs    = []rune{' ', '▀', '▄', '█'}
	quadGlyphs    = []rune(" ▘▝▀▖▌▞▛▗▚▐▜▄▙▟█")
	sextantGlyphs = func() []rune {
		out := make([]rune, 64)
		for m := range out {
			switch m {
			case 0:
				out[m] = ' '
			case 21:
				out[m] = '▌'
			case 42:
				out[m] = '▐'
			case 63:
				out[m] = '█'
			default:
				r := rune(0x1FB00 + m - 1)
				if m > 21 {
					r--
				}
				if m > 42 {
					r--
				}
				out[m] = r
			}
		}
		return out
	}()
	brailleGlyphs = func() []rune {
		bits := [8]rune{0x01, 0x08, 0x02, 0x10, 0x04, 0x20, 0x40, 0x80}
		out := make([]rune, 256)
		for m := range out {
			r := rune(0x2800)
			for i, b := range bits {
				if m>>i&1 == 1 {
					r |= b
				}
			}
			out[m] = r
		}
		out[0] = ' '
		return out
	}()
)

func glyphs(mode string) []rune {
	switch mode {
	case ModeHalf:
		return halfGlyphs
	case ModeQuad:
		return quadGlyphs
	case ModeSextant:
		return sextantGlyphs
	}
	return brailleGlyphs
}

// Renderer converts RGB24 frames into cells. It keeps scratch buffers and
// the current palette between frames; it is not safe for concurrent use.
type Renderer struct {
	// TermDepth is the color depth of the output terminal (24, 8 or 4
	// bits). Below 24 the picture is quantised to what the terminal can
	// show, so dithering still applies.
	TermDepth int

	work, small []byte
	lum, lvl    []uint8
	dith        ditherer
	pal         *Palette
	palKey      string
	tone        [256]uint8
	toneKey     [4]float64
}

// Palette returns the palette used for the last rendered frame, or nil when
// the output was truecolor.
func (r *Renderer) Palette() *Palette { return r.pal }

func grow(b []byte, n int) []byte {
	if cap(b) < n {
		return make([]byte, n)
	}
	return b[:n]
}

func (r *Renderer) buildTone(l *Look) {
	inv := 0.0
	if l.Invert {
		inv = 1
	}
	key := [4]float64{l.Brightness, l.Contrast, l.Gamma, inv}
	if key == r.toneKey && r.tone[255] != r.tone[0] {
		return
	}
	r.toneKey = key
	g := l.Gamma
	if g <= 0 {
		g = 1
	}
	for i := range r.tone {
		v := (float64(i)/255-0.5)*l.Contrast + 0.5 + l.Brightness
		v = math.Max(0, math.Min(1, v))
		v = math.Pow(v, 1/g)
		if l.Invert {
			v = 1 - v
		}
		r.tone[i] = uint8(v*255 + 0.5)
	}
}

// colorMatrix returns a 3×3 fixed-point (<<12) matrix combining saturation
// and hue rotation, and whether it differs from identity.
func colorMatrix(sat, hueDeg float64) (m [9]int32, active bool) {
	if sat == 1 && math.Mod(hueDeg, 360) == 0 {
		return m, false
	}
	c := math.Cos(hueDeg*math.Pi/180) * sat
	s := math.Sin(hueDeg*math.Pi/180) * sat
	base := [9]float64{0.213, 0.715, 0.072, 0.213, 0.715, 0.072, 0.213, 0.715, 0.072}
	cm := [9]float64{0.787, -0.715, -0.072, -0.213, 0.285, -0.072, -0.213, -0.715, 0.928}
	sm := [9]float64{-0.213, -0.715, 0.928, 0.143, 0.140, -0.283, -0.787, 0.715, 0.072}
	for i := range m {
		m[i] = int32(math.Round((base[i] + c*cm[i] + s*sm[i]) * 4096))
	}
	return m, true
}

func (r *Renderer) prepare(src []byte, w, h int, l *Look) {
	r.work = grow(r.work, w*h*3)
	r.buildTone(l)
	mat, useMat := colorMatrix(l.Saturation, l.HueShift)
	tone := &r.tone
	work := r.work
	parallel(h, func(lo, hi int) {
		for y := lo; y < hi; y++ {
			sy := y
			if l.FlipY {
				sy = h - 1 - y
			}
			srow := src[sy*w*3 : (sy+1)*w*3]
			drow := work[y*w*3 : (y+1)*w*3]
			for x := 0; x < w; x++ {
				sx := x
				if l.FlipX {
					sx = w - 1 - x
				}
				p := srow[sx*3 : sx*3+3]
				rr, gg, bb := p[0], p[1], p[2]
				if useMat {
					ri, gi, bi := int32(rr), int32(gg), int32(bb)
					rr = clamp8(int((mat[0]*ri + mat[1]*gi + mat[2]*bi) >> 12))
					gg = clamp8(int((mat[3]*ri + mat[4]*gi + mat[5]*bi) >> 12))
					bb = clamp8(int((mat[6]*ri + mat[7]*gi + mat[8]*bi) >> 12))
				}
				d := drow[x*3 : x*3+3]
				d[0], d[1], d[2] = tone[rr], tone[gg], tone[bb]
			}
		}
	})
}

func (r *Renderer) luma(pix []byte, w, h int) []uint8 {
	if cap(r.lum) < w*h {
		r.lum = make([]uint8, w*h)
	}
	r.lum = r.lum[:w*h]
	lum := r.lum
	parallel(h, func(lo, hi int) {
		for i := lo * w; i < hi*w; i++ {
			lum[i] = Luma(pix[i*3], pix[i*3+1], pix[i*3+2])
		}
	})
	return lum
}

func (r *Renderer) edges(w, h int, l *Look) {
	lum := r.luma(r.work, w, h)
	thr := int(l.EdgeThreshold * 255)
	color := l.Edges == "color"
	work := r.work
	at := func(x, y int) int {
		if x < 0 {
			x = 0
		} else if x >= w {
			x = w - 1
		}
		if y < 0 {
			y = 0
		} else if y >= h {
			y = h - 1
		}
		return int(lum[y*w+x])
	}
	parallel(h, func(lo, hi int) {
		for y := lo; y < hi; y++ {
			for x := 0; x < w; x++ {
				gx := at(x+1, y-1) + 2*at(x+1, y) + at(x+1, y+1) - at(x-1, y-1) - 2*at(x-1, y) - at(x-1, y+1)
				gy := at(x-1, y+1) + 2*at(x, y+1) + at(x+1, y+1) - at(x-1, y-1) - 2*at(x, y-1) - at(x+1, y-1)
				if gx < 0 {
					gx = -gx
				}
				if gy < 0 {
					gy = -gy
				}
				m := (gx + gy) / 2
				if m < thr {
					m = 0
				} else if m = m * 2; m > 255 {
					m = 255
				}
				p := work[(y*w+x)*3:]
				if color {
					k := m + 64
					if m == 0 {
						k = 0
					}
					p[0] = clamp8(int(p[0]) * k / 255)
					p[1] = clamp8(int(p[1]) * k / 255)
					p[2] = clamp8(int(p[2]) * k / 255)
				} else {
					p[0], p[1], p[2] = uint8(m), uint8(m), uint8(m)
				}
			}
		}
	})
}

func (r *Renderer) resolvePalette(l *Look, block bool) *Palette {
	key := paletteKey(l)
	if block {
		key += "/b"
	}
	if l.Palette == PalOff || l.Palette == "" {
		switch r.TermDepth {
		case 8:
			r.pal, r.palKey = XtermPalette(), "term8"
		case 4:
			r.pal, r.palKey = AnsiPalette(), "term4"
		default:
			r.pal, r.palKey = nil, ""
		}
		return r.pal
	}
	if key == r.palKey && r.pal != nil {
		return r.pal
	}
	colors := PaletteColors(l, r.work)
	if colors == nil {
		r.pal, r.palKey = nil, ""
		return nil
	}
	if block && len(colors) == 1 {
		// A single color cannot draw a picture out of solid blocks.
		colors = []RGB{{0, 0, 0}, colors[0]}
	}
	r.pal, r.palKey = NewPalette(colors), key
	return r.pal
}

// Render draws one frame. src is RGB24 of size w×h where w = cols·sx and
// h = rows·sy for the look's mode (see SubCells); dst receives cols×rows
// cells in row-major order.
func (r *Renderer) Render(src []byte, w, h int, l *Look, dst []Cell, cols, rows, frame int) {
	sx, sy := SubCells(l.Mode)
	if cols*sx != w || rows*sy != h || len(src) < w*h*3 || len(dst) < cols*rows {
		return
	}
	r.prepare(src, w, h, l)
	if l.Edges == "mono" || l.Edges == "color" {
		r.edges(w, h, l)
	}
	if cap(r.lvl) < w*h {
		r.lvl = make([]uint8, w*h)
	}
	r.lvl = r.lvl[:w*h]

	if !l.Color {
		r.pal, r.palKey = nil, ""
		r.renderMono(w, h, l, dst, cols, rows, frame)
		return
	}
	switch l.Mode {
	case ModeHalf:
		if pal := r.resolvePalette(l, true); pal != nil {
			r.dith.rgb(r.work, w, h, pal, l, frame)
		}
		r.composeHalf(w, dst, cols, rows)
	case ModeQuad, ModeSextant:
		pal := r.resolvePalette(l, true)
		if pal != nil {
			r.dith.rgb(r.work, w, h, pal, l, frame)
		}
		r.composeFit(w, l.Mode, pal, dst, cols, rows)
	case ModeBraille:
		r.composeBraille(w, h, l, dst, cols, rows, frame)
	default:
		r.composeASCII(w, h, l, dst, cols, rows, frame)
	}
}

func (r *Renderer) renderMono(w, h int, l *Look, dst []Cell, cols, rows, frame int) {
	lum := r.luma(r.work, w, h)
	if l.Mode == ModeASCII {
		ramp := l.Ramp()
		r.dith.gray(lum, w, h, len(ramp), l, frame, r.lvl)
		for i := 0; i < cols*rows; i++ {
			dst[i] = Cell{Ch: ramp[r.lvl[i]], Fg: ColDefault, Bg: ColDefault}
		}
		return
	}
	r.dith.gray(lum, w, h, 2, l, frame, r.lvl)
	sx, sy := SubCells(l.Mode)
	table := glyphs(l.Mode)
	lvl := r.lvl
	parallel(rows, func(lo, hi int) {
		for cy := lo; cy < hi; cy++ {
			for cx := 0; cx < cols; cx++ {
				mask, bit := 0, 0
				for j := 0; j < sy; j++ {
					row := (cy*sy+j)*w + cx*sx
					for i := 0; i < sx; i++ {
						mask |= int(lvl[row+i]) << bit
						bit++
					}
				}
				dst[cy*cols+cx] = Cell{Ch: table[mask], Fg: ColDefault, Bg: ColDefault}
			}
		}
	})
}

func (r *Renderer) composeHalf(w int, dst []Cell, cols, rows int) {
	work := r.work
	parallel(rows, func(lo, hi int) {
		for cy := lo; cy < hi; cy++ {
			top := work[cy*2*w*3:]
			bot := work[(cy*2+1)*w*3:]
			out := dst[cy*cols : (cy+1)*cols]
			for cx := range out {
				t := uint32(top[cx*3])<<16 | uint32(top[cx*3+1])<<8 | uint32(top[cx*3+2])
				b := uint32(bot[cx*3])<<16 | uint32(bot[cx*3+1])<<8 | uint32(bot[cx*3+2])
				if t == b {
					out[cx] = Cell{Ch: ' ', Bg: b}
				} else {
					out[cx] = Cell{Ch: '▀', Fg: t, Bg: b}
				}
			}
		}
	})
}

// composeFit reduces each cell's sub-pixels to two colors: it splits them
// along the channel with the widest range and averages each side.
func (r *Renderer) composeFit(w int, mode string, pal *Palette, dst []Cell, cols, rows int) {
	sx, sy := SubCells(mode)
	table := glyphs(mode)
	work := r.work
	parallel(rows, func(lo, hi int) {
		var px [8][3]int
		for cy := lo; cy < hi; cy++ {
			for cx := 0; cx < cols; cx++ {
				n := 0
				mn := [3]int{255, 255, 255}
				mx := [3]int{}
				for j := 0; j < sy; j++ {
					p := work[((cy*sy+j)*w+cx*sx)*3:]
					for i := 0; i < sx; i++ {
						for c := 0; c < 3; c++ {
							v := int(p[i*3+c])
							px[n][c] = v
							if v < mn[c] {
								mn[c] = v
							}
							if v > mx[c] {
								mx[c] = v
							}
						}
						n++
					}
				}
				ch, span := 0, 0
				for c := 0; c < 3; c++ {
					if d := mx[c] - mn[c]; d > span {
						ch, span = c, d
					}
				}
				if span == 0 {
					dst[cy*cols+cx] = Cell{Ch: ' ', Bg: uint32(px[0][0])<<16 | uint32(px[0][1])<<8 | uint32(px[0][2])}
					continue
				}
				thr := (mx[ch] + mn[ch]) / 2
				var fg, bg [3]int
				nf, mask := 0, 0
				for i := 0; i < n; i++ {
					if px[i][ch] > thr {
						mask |= 1 << i
						nf++
						fg[0] += px[i][0]
						fg[1] += px[i][1]
						fg[2] += px[i][2]
					} else {
						bg[0] += px[i][0]
						bg[1] += px[i][1]
						bg[2] += px[i][2]
					}
				}
				nb := n - nf
				f := RGB{uint8(fg[0] / nf), uint8(fg[1] / nf), uint8(fg[2] / nf)}
				b := RGB{uint8(bg[0] / nb), uint8(bg[1] / nb), uint8(bg[2] / nb)}
				if pal != nil {
					f, b = pal.Nearest(f.R, f.G, f.B), pal.Nearest(b.R, b.G, b.B)
				}
				if f == b {
					dst[cy*cols+cx] = Cell{Ch: ' ', Bg: b.U32()}
				} else {
					dst[cy*cols+cx] = Cell{Ch: table[mask], Fg: f.U32(), Bg: b.U32()}
				}
			}
		}
	})
}

func (r *Renderer) cellBg(l *Look, pal *Palette, c RGB) uint32 {
	switch l.Background {
	case "black":
		return 0
	case "tint":
		t := RGB{uint8(int(c.R) * 9 / 25), uint8(int(c.G) * 9 / 25), uint8(int(c.B) * 9 / 25)}
		if pal != nil {
			t = pal.Nearest(t.R, t.G, t.B)
		}
		return t.U32()
	}
	return ColDefault
}

func (r *Renderer) composeBraille(w, h int, l *Look, dst []Cell, cols, rows, frame int) {
	lum := r.luma(r.work, w, h)
	r.dith.gray(lum, w, h, 2, l, frame, r.lvl)
	r.small = grow(r.small, cols*rows*3)
	work, lvl, small := r.work, r.lvl, r.small
	masks := dst // reuse: stash the mask in Attr until colors are known
	parallel(rows, func(lo, hi int) {
		for cy := lo; cy < hi; cy++ {
			for cx := 0; cx < cols; cx++ {
				var on, all [3]int
				mask, bit, non := 0, 0, 0
				for j := 0; j < 4; j++ {
					base := (cy*4+j)*w + cx*2
					for i := 0; i < 2; i++ {
						p := work[(base+i)*3:]
						all[0] += int(p[0])
						all[1] += int(p[1])
						all[2] += int(p[2])
						if lvl[base+i] != 0 {
							mask |= 1 << bit
							non++
							on[0] += int(p[0])
							on[1] += int(p[1])
							on[2] += int(p[2])
						}
						bit++
					}
				}
				s := small[(cy*cols+cx)*3:]
				if non > 0 {
					s[0], s[1], s[2] = uint8(on[0]/non), uint8(on[1]/non), uint8(on[2]/non)
				} else {
					s[0], s[1], s[2] = uint8(all[0]/8), uint8(all[1]/8), uint8(all[2]/8)
				}
				masks[cy*cols+cx] = Cell{Attr: uint8(mask)}
			}
		}
	})
	pal := r.resolvePalette(l, false)
	if pal != nil {
		r.dith.rgb(small, cols, rows, pal, l, frame)
	}
	for i := 0; i < cols*rows; i++ {
		c := RGB{small[i*3], small[i*3+1], small[i*3+2]}
		mask := masks[i].Attr
		cell := Cell{Ch: brailleGlyphs[mask], Fg: c.U32(), Bg: r.cellBg(l, pal, c)}
		if mask == 0 {
			cell.Fg = 0
		}
		dst[i] = cell
	}
}

func (r *Renderer) composeASCII(w, h int, l *Look, dst []Cell, cols, rows, frame int) {
	ramp := l.Ramp()
	lum := r.luma(r.work, w, h)
	r.dith.gray(lum, w, h, len(ramp), l, frame, r.lvl)
	pal := r.resolvePalette(l, false)
	if pal != nil {
		r.dith.rgb(r.work, w, h, pal, l, frame)
	}
	for i := 0; i < cols*rows; i++ {
		c := RGB{r.work[i*3], r.work[i*3+1], r.work[i*3+2]}
		cell := Cell{Ch: ramp[r.lvl[i]], Fg: c.U32(), Bg: r.cellBg(l, pal, c)}
		if cell.Ch == ' ' {
			cell.Fg = 0
		}
		dst[i] = cell
	}
}
