package tty

import (
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Key identifies a non-printable key.
type Key int

// Keys. KeyRune means Event.Rune holds a printable character.
const (
	KeyNone Key = iota
	KeyRune
	KeyEnter
	KeyEsc
	KeyTab
	KeyBackTab
	KeyBackspace
	KeyDelete
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyPgUp
	KeyPgDn
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyCtrlC
	KeyCtrlS
	KeyCtrlL
)

// EventType distinguishes keyboard from mouse input.
type EventType int

// Event types.
const (
	EvKey EventType = iota
	EvMouse
)

// MouseAction says what the mouse did.
type MouseAction int

// Mouse actions.
const (
	MousePress MouseAction = iota
	MouseRelease
	MouseDrag
	MouseMove
	MouseWheelUp
	MouseWheelDown
)

// Mouse buttons.
const (
	ButtonLeft = iota
	ButtonMiddle
	ButtonRight
	ButtonNone
)

// Event is one decoded key press or mouse action.
type Event struct {
	Type   EventType
	Key    Key
	Rune   rune
	Shift  bool
	Alt    bool
	Ctrl   bool
	Action MouseAction
	Button int
	X, Y   int // zero-based cell coordinates
}

// ReadEvents decodes terminal input from r until it fails, then closes the
// returned channel.
func ReadEvents(r io.Reader) <-chan Event {
	raw := make(chan []byte, 16)
	go func() {
		defer close(raw)
		for {
			buf := make([]byte, 1024)
			n, err := r.Read(buf)
			if n > 0 {
				raw <- buf[:n]
			}
			if err != nil {
				return
			}
		}
	}()
	out := make(chan Event, 256)
	go func() {
		defer close(out)
		var pend []byte
		timer := time.NewTimer(time.Hour)
		for {
			var wait <-chan time.Time
			if len(pend) > 0 {
				// An unfinished sequence: give the rest a moment to
				// arrive before treating a lone ESC as the Escape key.
				timer.Reset(30 * time.Millisecond)
				wait = timer.C
			}
			select {
			case chunk, ok := <-raw:
				if !ok {
					return
				}
				pend = append(pend, chunk...)
				pend = parse(pend, out, false)
			case <-wait:
				pend = parse(pend, out, true)
			}
		}
	}()
	return out
}

// parse consumes as many complete events as possible and returns the rest.
// With flush set, an incomplete sequence is emitted as-is.
func parse(b []byte, out chan<- Event, flush bool) []byte {
	for len(b) > 0 {
		ev, n := decode(b, flush)
		if n == 0 {
			return b
		}
		b = b[n:]
		if ev.Type == EvKey && ev.Key == KeyNone {
			continue
		}
		out <- ev
	}
	return b[:0]
}

func key(k Key) Event { return Event{Type: EvKey, Key: k} }

func decode(b []byte, flush bool) (Event, int) {
	c := b[0]
	switch {
	case c == 0x1b:
		if len(b) == 1 {
			if flush {
				return key(KeyEsc), 1
			}
			return Event{}, 0
		}
		switch b[1] {
		case '[':
			return decodeCSI(b, flush)
		case 'O':
			if len(b) < 3 {
				if flush {
					return key(KeyNone), len(b)
				}
				return Event{}, 0
			}
			switch b[2] {
			case 'A':
				return key(KeyUp), 3
			case 'B':
				return key(KeyDown), 3
			case 'C':
				return key(KeyRight), 3
			case 'D':
				return key(KeyLeft), 3
			case 'H':
				return key(KeyHome), 3
			case 'F':
				return key(KeyEnd), 3
			case 'P':
				return key(KeyF1), 3
			case 'Q':
				return key(KeyF2), 3
			case 'R':
				return key(KeyF3), 3
			case 'S':
				return key(KeyF4), 3
			}
			return key(KeyNone), 3
		case 0x1b:
			return key(KeyEsc), 1
		}
		ev, n := decode(b[1:], flush)
		if n == 0 {
			return Event{}, 0
		}
		ev.Alt = true
		return ev, n + 1
	case c == '\r' || c == '\n':
		return key(KeyEnter), 1
	case c == '\t':
		return key(KeyTab), 1
	case c == 0x7f || c == 0x08:
		return key(KeyBackspace), 1
	case c == 0x03:
		return key(KeyCtrlC), 1
	case c == 0x13:
		return key(KeyCtrlS), 1
	case c == 0x0c:
		return key(KeyCtrlL), 1
	case c == 0x00:
		return Event{Type: EvKey, Key: KeyRune, Rune: ' ', Ctrl: true}, 1
	case c >= 0x1c && c < 0x20:
		// Ctrl+\ ] ^ and Ctrl+- (which terminals send as Ctrl+_).
		return Event{Type: EvKey, Key: KeyRune, Rune: []rune{'\\', ']', '^', '-'}[c-0x1c], Ctrl: true}, 1
	case c < 0x20:
		return Event{Type: EvKey, Key: KeyRune, Rune: rune(c) + 'a' - 1, Ctrl: true}, 1
	}
	if !utf8.FullRune(b) && !flush {
		return Event{}, 0
	}
	r, n := utf8.DecodeRune(b)
	if r == utf8.RuneError {
		return key(KeyNone), 1
	}
	return Event{Type: EvKey, Key: KeyRune, Rune: r}, n
}

func decodeCSI(b []byte, flush bool) (Event, int) {
	end := -1
	for i := 2; i < len(b); i++ {
		if b[i] >= 0x40 && b[i] <= 0x7e {
			end = i
			break
		}
	}
	if end < 0 {
		if flush || len(b) > 64 {
			return key(KeyNone), len(b)
		}
		return Event{}, 0
	}
	n := end + 1
	final := b[end]
	body := string(b[2:end])
	if strings.HasPrefix(body, "<") && (final == 'M' || final == 'm') {
		return decodeMouse(body[1:], final == 'm'), n
	}
	var params []int
	for _, f := range strings.Split(body, ";") {
		f, _, _ = strings.Cut(f, ":") // drop kitty sub-parameters
		v, _ := strconv.Atoi(strings.TrimLeft(f, "?>="))
		params = append(params, v)
	}
	ev := Event{Type: EvKey}
	if len(params) >= 2 && params[1] > 1 {
		m := params[1] - 1
		ev.Shift, ev.Alt, ev.Ctrl = m&1 != 0, m&2 != 0, m&4 != 0
	}
	if final == 'u' {
		// Kitty keyboard protocol: CSI code ; modifiers u
		return keyFromCode(ev, params[0]), n
	}
	if final == '~' && params[0] == 27 && len(params) >= 3 {
		// xterm modifyOtherKeys: CSI 27 ; modifiers ; code ~
		return keyFromCode(ev, params[2]), n
	}
	switch final {
	case 'A':
		ev.Key = KeyUp
	case 'B':
		ev.Key = KeyDown
	case 'C':
		ev.Key = KeyRight
	case 'D':
		ev.Key = KeyLeft
	case 'H':
		ev.Key = KeyHome
	case 'F':
		ev.Key = KeyEnd
	case 'Z':
		ev.Key = KeyBackTab
	case 'P':
		ev.Key = KeyF1
	case 'Q':
		ev.Key = KeyF2
	case 'R':
		ev.Key = KeyF3
	case 'S':
		ev.Key = KeyF4
	case '~':
		switch params[0] {
		case 1, 7:
			ev.Key = KeyHome
		case 3:
			ev.Key = KeyDelete
		case 4, 8:
			ev.Key = KeyEnd
		case 5:
			ev.Key = KeyPgUp
		case 6:
			ev.Key = KeyPgDn
		case 11:
			ev.Key = KeyF1
		case 12:
			ev.Key = KeyF2
		case 13:
			ev.Key = KeyF3
		case 14:
			ev.Key = KeyF4
		case 15:
			ev.Key = KeyF5
		case 17:
			ev.Key = KeyF6
		case 18:
			ev.Key = KeyF7
		case 19:
			ev.Key = KeyF8
		case 20:
			ev.Key = KeyF9
		case 21:
			ev.Key = KeyF10
		case 23:
			ev.Key = KeyF11
		case 24:
			ev.Key = KeyF12
		}
	}
	return ev, n
}

// keyFromCode turns a Unicode key code reported by the kitty protocol (or
// modifyOtherKeys) into an event; ev already carries the modifiers.
func keyFromCode(ev Event, code int) Event {
	switch code {
	case 27:
		ev.Key = KeyEsc
	case 13, 57414:
		ev.Key = KeyEnter
	case 9:
		ev.Key = KeyTab
		if ev.Shift {
			ev.Key = KeyBackTab
		}
	case 127, 8:
		ev.Key = KeyBackspace
	case 57413: // keypad +
		ev.Key, ev.Rune = KeyRune, '+'
	case 57412: // keypad -
		ev.Key, ev.Rune = KeyRune, '-'
	case 57399: // keypad 0
		ev.Key, ev.Rune = KeyRune, '0'
	default:
		if code < 0x20 || (code >= 57344 && code <= 63743) {
			return key(KeyNone) // modifier keys and other private-use codes
		}
		ev.Key, ev.Rune = KeyRune, rune(code)
		if ev.Ctrl && !ev.Alt && !ev.Shift {
			switch code {
			case 'c':
				return key(KeyCtrlC)
			case 's':
				return key(KeyCtrlS)
			case 'l':
				return key(KeyCtrlL)
			}
		}
	}
	return ev
}

func decodeMouse(body string, release bool) Event {
	f := strings.Split(body, ";")
	if len(f) != 3 {
		return key(KeyNone)
	}
	code, _ := strconv.Atoi(f[0])
	x, _ := strconv.Atoi(f[1])
	y, _ := strconv.Atoi(f[2])
	ev := Event{Type: EvMouse, X: x - 1, Y: y - 1, Button: code & 3,
		Shift: code&4 != 0, Alt: code&8 != 0, Ctrl: code&16 != 0}
	switch {
	case code&64 != 0:
		ev.Action = MouseWheelUp
		if code&1 != 0 {
			ev.Action = MouseWheelDown
		}
		ev.Button = ButtonNone
	case code&32 != 0:
		ev.Action = MouseDrag
		if ev.Button == ButtonNone {
			ev.Action = MouseMove
		}
	case release:
		ev.Action = MouseRelease
	default:
		ev.Action = MousePress
	}
	return ev
}
