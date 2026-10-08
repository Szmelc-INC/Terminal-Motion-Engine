package tty

import (
	"bytes"
	"strings"
	"testing"
)

func decodeAll(t *testing.T, in string) []Event {
	t.Helper()
	out := make(chan Event, 64)
	rest := parse([]byte(in), out, true)
	if len(rest) != 0 {
		t.Fatalf("%q: %d bytes left over", in, len(rest))
	}
	close(out)
	var evs []Event
	for e := range out {
		evs = append(evs, e)
	}
	return evs
}

func TestKeys(t *testing.T) {
	cases := map[string]Key{
		"\x1b[A": KeyUp, "\x1b[B": KeyDown, "\x1b[C": KeyRight, "\x1b[D": KeyLeft,
		"\x1bOA": KeyUp, "\x1b[H": KeyHome, "\x1b[F": KeyEnd, "\x1b[1~": KeyHome,
		"\x1b[3~": KeyDelete, "\x1b[5~": KeyPgUp, "\x1b[6~": KeyPgDn, "\x1b[Z": KeyBackTab,
		"\x1bOP": KeyF1, "\x1b[12~": KeyF2, "\x1bOQ": KeyF2, "\r": KeyEnter, "\t": KeyTab,
		"\x7f": KeyBackspace, "\x03": KeyCtrlC, "\x13": KeyCtrlS, "\x1b": KeyEsc,
	}
	for in, want := range cases {
		evs := decodeAll(t, in)
		if len(evs) != 1 || evs[0].Key != want {
			t.Errorf("%q -> %+v, want key %d", in, evs, want)
		}
	}
	evs := decodeAll(t, "\x1b[1;2C")
	if len(evs) != 1 || evs[0].Key != KeyRight || !evs[0].Shift {
		t.Errorf("shift-right -> %+v", evs)
	}
	evs = decodeAll(t, "aż\x1bx")
	if len(evs) != 3 || evs[0].Rune != 'a' || evs[1].Rune != 'ż' || evs[2].Rune != 'x' || !evs[2].Alt {
		t.Errorf("runes -> %+v", evs)
	}
}

func TestIncompleteSequenceWaits(t *testing.T) {
	out := make(chan Event, 8)
	rest := parse([]byte("\x1b[<0;10"), out, false)
	if len(rest) == 0 || len(out) != 0 {
		t.Fatalf("partial mouse sequence was consumed early (rest=%q, events=%d)", rest, len(out))
	}
	rest = parse(append(rest, []byte(";5M")...), out, false)
	if len(rest) != 0 || len(out) != 1 {
		t.Fatalf("completed sequence not decoded (rest=%q, events=%d)", rest, len(out))
	}
	// A lone ESC is held back until the flush timeout.
	rest = parse([]byte("\x1b"), out, false)
	if len(rest) != 1 {
		t.Fatal("lone ESC should wait for more input")
	}
}

func TestMouse(t *testing.T) {
	type want struct {
		action MouseAction
		button int
		x, y   int
	}
	cases := map[string]want{
		"\x1b[<0;10;5M":  {MousePress, ButtonLeft, 9, 4},
		"\x1b[<0;10;5m":  {MouseRelease, ButtonLeft, 9, 4},
		"\x1b[<2;1;1M":   {MousePress, ButtonRight, 0, 0},
		"\x1b[<1;3;3M":   {MousePress, ButtonMiddle, 2, 2},
		"\x1b[<32;11;5M": {MouseDrag, ButtonLeft, 10, 4},
		"\x1b[<35;20;7M": {MouseMove, ButtonNone, 19, 6},
		"\x1b[<64;4;4M":  {MouseWheelUp, ButtonNone, 3, 3},
		"\x1b[<65;4;4M":  {MouseWheelDown, ButtonNone, 3, 3},
	}
	for in, w := range cases {
		evs := decodeAll(t, in)
		if len(evs) != 1 {
			t.Fatalf("%q -> %d events", in, len(evs))
		}
		e := evs[0]
		if e.Type != EvMouse || e.Action != w.action || e.Button != w.button || e.X != w.x || e.Y != w.y {
			t.Errorf("%q -> %+v, want %+v", in, e, w)
		}
	}
}

func TestScreenDiff(t *testing.T) {
	var buf bytes.Buffer
	s := NewScreen(&buf, 24)
	s.Resize(10, 3)
	s.Text(0, 0, "hello", 0xff0000, 0x000000, 0, -1)
	s.Flush()
	first := buf.String()
	if !strings.Contains(first, "hello") || !strings.Contains(first, "38;2;255;0;0") {
		t.Fatalf("first frame missing content: %q", first)
	}
	if !strings.HasPrefix(first, "\x1b[?2026h") || !strings.HasSuffix(first, "\x1b[?2026l") {
		t.Error("frame is not wrapped in a synchronized update")
	}
	if strings.Contains(first, "\x1b[2J") {
		t.Error("screen must never be cleared with ED — that is what flickers")
	}

	// An identical frame writes no cells at all.
	buf.Reset()
	s.Flush()
	if buf.Len() != 0 {
		t.Errorf("unchanged frame produced output: %q", buf.String())
	}

	// Changing one cell writes exactly that cell.
	buf.Reset()
	s.Set(4, 0, Cell{Ch: 'X', Fg: 0xff0000, Bg: 0})
	s.Flush()
	got := buf.String()
	if !strings.Contains(got, "\x1b[1;5H") || !strings.Contains(got, "X") || strings.Contains(got, "hell") {
		t.Errorf("one-cell change produced: %q", got)
	}
}

func TestScreenDepths(t *testing.T) {
	for depth, want := range map[int]string{24: "48;2;255;0;0", 8: "48;5;", 4: "101"} {
		var buf bytes.Buffer
		s := NewScreen(&buf, depth)
		s.Resize(2, 1)
		s.Set(0, 0, Cell{Ch: ' ', Bg: 0xff0000})
		s.Flush()
		if !strings.Contains(buf.String(), want) {
			t.Errorf("depth %d: %q lacks %q", depth, buf.String(), want)
		}
	}
}

func TestDump(t *testing.T) {
	cells := []Cell{{Ch: 'a', Fg: 0x00ff00, Bg: ColDefault}, {Ch: 'b', Fg: 0x00ff00, Bg: ColDefault}}
	out := string(Dump(cells, 2, 1, 24))
	if strings.Count(out, "38;2;0;255;0") != 1 || !strings.Contains(out, "ab") || !strings.HasSuffix(out, "\x1b[0m\n") {
		t.Errorf("Dump = %q", out)
	}
}

func TestClean(t *testing.T) {
	if got := Clean("a\x1b[2Jb\tc日本"); got != "a?[2Jb?c??" {
		t.Errorf("Clean = %q", got)
	}
}
