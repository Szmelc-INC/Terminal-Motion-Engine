package tty

import (
	"errors"
	"os"
	"sync"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

const (
	enterSeq = "\x1b[?1049h" + // alternate screen
		"\x1b[?25l" + // hide cursor
		"\x1b[?7l" + // no auto-wrap
		"\x1b[?1000h\x1b[?1002h\x1b[?1003h\x1b[?1006h" // mouse: buttons, drag, motion, SGR coords
	leaveSeq = "\x1b[?1006l\x1b[?1003l\x1b[?1002l\x1b[?1000l" +
		"\x1b[?2026l\x1b[0m\x1b[?7h\x1b[?25h\x1b[?1049l"
)

// Terminal owns the raw-mode state of the controlling terminal.
type Terminal struct {
	in, out *os.File
	state   *term.State
	once    sync.Once
}

// Open puts the terminal into raw mode on the alternate screen with mouse
// reporting enabled. Always call Close, including on panic.
func Open() (*Terminal, error) {
	in, out := os.Stdin, os.Stdout
	if !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) {
		return nil, errors.New("termo needs an interactive terminal (stdin and stdout must be a TTY)")
	}
	st, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		return nil, err
	}
	t := &Terminal{in: in, out: out, state: st}
	out.WriteString(enterSeq)
	return t, nil
}

// Close restores the terminal. It is safe to call more than once.
func (t *Terminal) Close() {
	t.once.Do(func() {
		t.out.WriteString(leaveSeq)
		term.Restore(int(t.in.Fd()), t.state)
	})
}

// In returns the input file.
func (t *Terminal) In() *os.File { return t.in }

// Out returns the output file.
func (t *Terminal) Out() *os.File { return t.out }

// Size returns the grid size and the width/height ratio of one cell. The
// ratio is 0 when the terminal does not report its pixel size.
func (t *Terminal) Size() (cols, rows int, cellAspect float64) {
	ws, err := unix.IoctlGetWinsize(int(t.out.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 || ws.Row == 0 {
		return 80, 24, 0
	}
	cols, rows = int(ws.Col), int(ws.Row)
	if ws.Xpixel > 0 && ws.Ypixel > 0 {
		cellAspect = (float64(ws.Xpixel) / float64(ws.Col)) / (float64(ws.Ypixel) / float64(ws.Row))
	}
	return
}
