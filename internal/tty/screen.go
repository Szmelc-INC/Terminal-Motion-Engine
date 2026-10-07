// Package tty is a small cell-based terminal layer: a double-buffered
// screen that only emits the cells that changed, and a key/mouse decoder.
package tty

import (
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
)

// Cell re-exports engine.Cell for callers that only draw UI.
type Cell = engine.Cell

// ColDefault is the terminal's default colour.
const ColDefault = engine.ColDefault

var numStr [256]string

func init() {
	for i := range numStr {
		numStr[i] = strconv.Itoa(i)
	}
}

// Screen is a double-buffered cell grid. Draw into it with Set/Text/Fill
// and call Flush once per frame: only the cells that differ from what the
// terminal already shows are written, inside a synchronized-update block,
// in a single write. That is what keeps playback free of flicker.
type Screen struct {
	W, H  int
	Depth int // 24, 8 or 4 bit colour

	cur, prev []Cell
	out       io.Writer
	buf       []byte
	full      bool
}

// NewScreen creates a screen writing to out.
func NewScreen(out io.Writer, depth int) *Screen {
	return &Screen{out: out, Depth: depth, full: true}
}

// DetectDepth guesses the terminal's colour depth from the environment.
func DetectDepth() int {
	ct := strings.ToLower(os.Getenv("COLORTERM"))
	term := os.Getenv("TERM")
	switch {
	case strings.Contains(ct, "truecolor") || strings.Contains(ct, "24bit"):
		return 24
	case strings.Contains(term, "direct") || strings.Contains(term, "kitty") ||
		strings.Contains(term, "ghostty") || strings.Contains(term, "alacritty") ||
		strings.Contains(term, "foot") || strings.Contains(term, "wezterm"):
		return 24
	case strings.Contains(term, "256"):
		return 8
	case term == "linux" || term == "dumb" || term == "":
		return 4
	}
	return 8
}

// Resize changes the grid size and forces a full repaint.
func (s *Screen) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	s.W, s.H = w, h
	s.cur = make([]Cell, w*h)
	s.prev = make([]Cell, w*h)
	s.full = true
	s.Clear()
}

// Invalidate forces the next Flush to repaint every cell.
func (s *Screen) Invalidate() { s.full = true }

// Clear resets the back buffer to blank cells.
func (s *Screen) Clear() {
	blank := Cell{Ch: ' ', Bg: ColDefault}
	for i := range s.cur {
		s.cur[i] = blank
	}
}

// Set writes one cell; out-of-range coordinates are ignored.
func (s *Screen) Set(x, y int, c Cell) {
	if x < 0 || y < 0 || x >= s.W || y >= s.H {
		return
	}
	s.cur[y*s.W+x] = c
}

// Get reads one cell from the back buffer.
func (s *Screen) Get(x, y int) Cell {
	if x < 0 || y < 0 || x >= s.W || y >= s.H {
		return Cell{}
	}
	return s.cur[y*s.W+x]
}

// Blit copies a block of cells to the back buffer, clipping at the edges.
func (s *Screen) Blit(x, y, w, h int, cells []Cell) {
	for row := 0; row < h; row++ {
		dy := y + row
		if dy < 0 || dy >= s.H {
			continue
		}
		x0, x1 := 0, w
		if x < 0 {
			x0 = -x
		}
		if x+x1 > s.W {
			x1 = s.W - x
		}
		if x0 < x1 {
			copy(s.cur[dy*s.W+x+x0:dy*s.W+x+x1], cells[row*w+x0:row*w+x1])
		}
	}
}

// Fill paints a rectangle with one cell.
func (s *Screen) Fill(x, y, w, h int, c Cell) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			s.Set(xx, yy, c)
		}
	}
}

// Dim darkens a rectangle of the back buffer; used for popup shadows.
func (s *Screen) Dim(x, y, w, h int) {
	half := func(c uint32) uint32 {
		if c == ColDefault {
			return c
		}
		return c >> 1 & 0x7f7f7f
	}
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			if xx < 0 || yy < 0 || xx >= s.W || yy >= s.H {
				continue
			}
			c := &s.cur[yy*s.W+xx]
			c.Fg, c.Bg = half(c.Fg), half(c.Bg)
			if c.Bg == ColDefault {
				c.Bg = 0
			}
		}
	}
}

// RuneWidth reports whether r takes two cells. It only needs to be good
// enough to keep file names from breaking the layout.
func wide(r rune) bool {
	return r >= 0x1100 && (r <= 0x115f || (r >= 0x2e80 && r <= 0xa4cf) || (r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) || (r >= 0xfe30 && r <= 0xfe6f) || (r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) || (r >= 0x1f300 && r <= 0x1faff && (r < 0x1fb00)) || (r >= 0x20000 && r <= 0x3fffd))
}

// Clean replaces control and double-width runes so every rune is one cell.
func Clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || wide(r) || r == utf8.RuneError {
			return '?'
		}
		if r >= 0x300 && r <= 0x36f { // combining marks
			return -1
		}
		return r
	}, s)
}

// Text draws a string and returns the number of cells used. Text is clipped
// at max cells when max >= 0.
func (s *Screen) Text(x, y int, str string, fg, bg uint32, attr uint8, max int) int {
	n := 0
	for _, r := range str {
		if max >= 0 && n >= max {
			break
		}
		s.Set(x+n, y, Cell{Ch: r, Fg: fg, Bg: bg, Attr: attr})
		n++
	}
	return n
}

func (s *Screen) appendColor(b []byte, c uint32, bg bool) []byte {
	if c == ColDefault {
		if bg {
			return append(b, "49"...)
		}
		return append(b, "39"...)
	}
	r, g, bl := uint8(c>>16), uint8(c>>8), uint8(c)
	switch s.Depth {
	case 24:
		if bg {
			b = append(b, "48;2;"...)
		} else {
			b = append(b, "38;2;"...)
		}
		b = append(b, numStr[r]...)
		b = append(b, ';')
		b = append(b, numStr[g]...)
		b = append(b, ';')
		return append(b, numStr[bl]...)
	case 8:
		if bg {
			b = append(b, "48;5;"...)
		} else {
			b = append(b, "38;5;"...)
		}
		return append(b, numStr[engine.XtermPalette().Index(r, g, bl)]...)
	}
	idx := int(engine.AnsiPalette().Index(r, g, bl))
	base := 30
	if idx >= 8 {
		base, idx = 90, idx-8
	}
	if bg {
		base += 10
	}
	return append(b, numStr[base+idx]...)
}

// Flush writes the changed cells to the terminal and returns the number of
// bytes written.
func (s *Screen) Flush() (int, error) {
	b := s.buf[:0]
	b = append(b, "\x1b[?2026h\x1b[0m"...)
	fg, bg, attr := ColDefault, ColDefault, uint8(0)
	cx, cy := -1, -1
	for y := 0; y < s.H; y++ {
		row := y * s.W
		for x := 0; x < s.W; x++ {
			c := s.cur[row+x]
			if !s.full && c == s.prev[row+x] {
				continue
			}
			if cy != y || cx != x {
				if cy == y && x > cx && x-cx <= 3 && !s.full {
					// Cheaper to repaint a couple of unchanged cells than
					// to emit a cursor move.
					x = cx
					c = s.cur[row+x]
				} else {
					b = append(b, "\x1b["...)
					b = strconv.AppendInt(b, int64(y+1), 10)
					b = append(b, ';')
					b = strconv.AppendInt(b, int64(x+1), 10)
					b = append(b, 'H')
				}
			}
			if c.Ch == 0 {
				c.Ch = ' '
			}
			wantFg := c.Fg
			if c.Ch == ' ' && c.Attr&(engine.AttrUnderline|engine.AttrReverse) == 0 {
				wantFg = fg // a blank shows no foreground; don't spend bytes on it
			}
			if c.Attr != attr || wantFg != fg || c.Bg != bg {
				b = append(b, "\x1b["...)
				sep := false
				if c.Attr != attr {
					b = append(b, '0')
					sep = true
					if c.Attr&engine.AttrBold != 0 {
						b = append(b, ";1"...)
					}
					if c.Attr&engine.AttrDim != 0 {
						b = append(b, ";2"...)
					}
					if c.Attr&engine.AttrUnderline != 0 {
						b = append(b, ";4"...)
					}
					if c.Attr&engine.AttrReverse != 0 {
						b = append(b, ";7"...)
					}
					attr, fg, bg = c.Attr, ColDefault, ColDefault
					if c.Ch == ' ' && c.Attr&(engine.AttrUnderline|engine.AttrReverse) == 0 {
						wantFg = fg
					}
				}
				if wantFg != fg {
					if sep {
						b = append(b, ';')
					}
					b = s.appendColor(b, wantFg, false)
					fg, sep = wantFg, true
				}
				if c.Bg != bg {
					if sep {
						b = append(b, ';')
					}
					b = s.appendColor(b, c.Bg, true)
					bg = c.Bg
				}
				b = append(b, 'm')
			}
			b = utf8.AppendRune(b, c.Ch)
			cx, cy = x+1, y
		}
	}
	s.buf = b
	s.full = false
	if cy < 0 {
		return 0, nil // nothing changed: leave the terminal alone
	}
	b = append(b, "\x1b[0m\x1b[?2026l"...)
	s.buf = b
	copy(s.prev, s.cur)
	return s.out.Write(b)
}
