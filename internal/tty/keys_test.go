package tty

import "testing"

func TestFunctionKeys(t *testing.T) {
	cases := map[string]Key{
		"\x1b[15~": KeyF5, "\x1b[17~": KeyF6, "\x1b[18~": KeyF7, "\x1b[19~": KeyF8, "\x1b[20~": KeyF9,
		"\x1b[21~": KeyF10, "\x1b[23~": KeyF11, "\x1b[24~": KeyF12,
	}
	for in, want := range cases {
		evs := decodeAll(t, in)
		if len(evs) != 1 || evs[0].Key != want {
			t.Errorf("%q -> %+v, want key %d", in, evs, want)
		}
	}
}

// The kitty keyboard protocol reports keys that the legacy encoding cannot
// express (Ctrl+=) and re-encodes ones it can (Esc, Ctrl+C).
func TestKittyProtocol(t *testing.T) {
	type want struct {
		key              Key
		r                rune
		ctrl, alt, shift bool
	}
	cases := map[string]want{
		"\x1b[61;5u":    {KeyRune, '=', true, false, false},
		"\x1b[61;6u":    {KeyRune, '=', true, false, true},
		"\x1b[45;5u":    {KeyRune, '-', true, false, false},
		"\x1b[48;5u":    {KeyRune, '0', true, false, false},
		"\x1b[61;3u":    {KeyRune, '=', false, true, false},
		"\x1b[27u":      {KeyEsc, 0, false, false, false},
		"\x1b[99;5u":    {KeyCtrlC, 0, false, false, false},
		"\x1b[115;5u":   {KeyCtrlS, 0, false, false, false},
		"\x1b[13u":      {KeyEnter, 0, false, false, false},
		"\x1b[9;2u":     {KeyBackTab, 0, false, false, true},
		"\x1b[61:43;6u": {KeyRune, '=', true, false, true},
		"\x1b[27;5;61~": {KeyRune, '=', true, false, false}, // modifyOtherKeys
	}
	for in, w := range cases {
		evs := decodeAll(t, in)
		if len(evs) != 1 {
			t.Errorf("%q -> %d events: %+v", in, len(evs), evs)
			continue
		}
		e := evs[0]
		if e.Key != w.key || (w.key == KeyRune && e.Rune != w.r) || e.Ctrl != w.ctrl || e.Alt != w.alt ||
			(w.key != KeyBackTab && e.Shift != w.shift) {
			t.Errorf("%q -> %+v, want %+v", in, e, w)
		}
	}
	// A bare modifier press must not turn into a key.
	if evs := decodeAll(t, "\x1b[57441;5u"); len(evs) != 0 {
		t.Errorf("modifier key -> %+v", evs)
	}
}

func TestLegacyCtrlMinus(t *testing.T) {
	evs := decodeAll(t, "\x1f")
	if len(evs) != 1 || evs[0].Rune != '-' || !evs[0].Ctrl {
		t.Errorf("ctrl+- -> %+v", evs)
	}
	evs = decodeAll(t, "\x1b=")
	if len(evs) != 1 || evs[0].Rune != '=' || !evs[0].Alt {
		t.Errorf("alt+= -> %+v", evs)
	}
}
