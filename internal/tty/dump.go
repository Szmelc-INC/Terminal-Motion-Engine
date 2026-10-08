package tty

import "unicode/utf8"

// Dump renders a block of cells as plain ANSI text, one line per row, for
// printing a single frame to a pipe or a normal scrolling terminal.
func Dump(cells []Cell, cols, rows, depth int) []byte {
	s := &Screen{Depth: depth}
	var b []byte
	for y := 0; y < rows; y++ {
		fg, bg := ColDefault, ColDefault
		for x := 0; x < cols; x++ {
			c := cells[y*cols+x]
			if c.Ch == 0 {
				c.Ch = ' '
			}
			if c.Ch == ' ' {
				c.Fg = fg
			}
			if c.Fg != fg || c.Bg != bg {
				b = append(b, "\x1b["...)
				if c.Fg != fg {
					b = s.appendColor(b, c.Fg, false)
					if c.Bg != bg {
						b = append(b, ';')
					}
				}
				if c.Bg != bg {
					b = s.appendColor(b, c.Bg, true)
				}
				b = append(b, 'm')
				fg, bg = c.Fg, c.Bg
			}
			b = utf8.AppendRune(b, c.Ch)
		}
		b = append(b, "\x1b[0m\n"...)
	}
	return b
}
