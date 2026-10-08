package engine

import (
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// User palettes and charsets live next to the built-in ones. They are set by
// the application when its library is loaded or edited.
var (
	userPalettes []NamedPalette
	userCharsets []Charset
	libGen       int // bumped on every change so cached palettes are rebuilt
)

// SetUserPalettes replaces the user-defined palettes.
func SetUserPalettes(p []NamedPalette) {
	userPalettes = append([]NamedPalette(nil), p...)
	libGen++
	refreshChoices()
}

// SetUserCharsets replaces the user-defined character ramps.
func SetUserCharsets(c []Charset) {
	userCharsets = append([]Charset(nil), c...)
	libGen++
	refreshChoices()
}

func refreshChoices() {
	if o := FindOption("palette"); o != nil {
		o.Choices = PaletteChoices()
	}
	if o := FindOption("charset"); o != nil {
		o.Choices = charsetNames()
	}
}

// AllPalettes lists user palettes first, then the built-in ones they do not
// shadow.
func AllPalettes() []NamedPalette {
	out := append([]NamedPalette(nil), userPalettes...)
	for _, b := range NamedPalettes {
		if !hasPalette(userPalettes, b.Name) {
			out = append(out, b)
		}
	}
	return out
}

func hasPalette(l []NamedPalette, name string) bool {
	for _, p := range l {
		if strings.EqualFold(p.Name, name) {
			return true
		}
	}
	return false
}

// FindPalette looks a named palette up, user palettes first.
func FindPalette(name string) ([]RGB, bool) {
	for _, l := range [][]NamedPalette{userPalettes, NamedPalettes} {
		for _, p := range l {
			if strings.EqualFold(p.Name, name) {
				return p.Colors, true
			}
		}
	}
	return nil, false
}

// IsBuiltinPalette reports whether name is one of the palettes that ship
// with termo (and is not shadowed by a user palette).
func IsBuiltinPalette(name string) bool {
	return hasPalette(NamedPalettes, name) && !hasPalette(userPalettes, name)
}

// AllCharsets lists user ramps first, then the built-in ones, then "custom".
func AllCharsets() []Charset {
	out := append([]Charset(nil), userCharsets...)
	for _, c := range Charsets {
		dup := false
		for _, u := range userCharsets {
			dup = dup || strings.EqualFold(u.Name, c.Name)
		}
		if !dup {
			out = append(out, c)
		}
	}
	return out
}

// --- palette generators -------------------------------------------------------

// Gradient returns n colors blended from a to b through OKLab, so the steps
// look evenly spaced.
func Gradient(stops []RGB, n int) []RGB {
	if n < 1 || len(stops) == 0 {
		return nil
	}
	if len(stops) == 1 {
		stops = append(stops, stops[0])
	}
	out := make([]RGB, n)
	for i := range out {
		t := 0.0
		if n > 1 {
			t = float64(i) / float64(n-1) * float64(len(stops)-1)
		}
		k := min(int(t), len(stops)-2)
		f := t - float64(k)
		l1, a1, b1 := ToOklab(stops[k])
		l2, a2, b2 := ToOklab(stops[k+1])
		r, g, b := oklabToLinear(l1+(l2-l1)*f, a1+(a2-a1)*f, b1+(b2-b1)*f)
		out[i] = RGB{to8(r), to8(g), to8(b)}
	}
	return out
}

// SortByLuma orders colors from dark to light.
func SortByLuma(c []RGB) []RGB {
	out := append([]RGB(nil), c...)
	sort.SliceStable(out, func(i, j int) bool {
		return Luma(out[i].R, out[i].G, out[i].B) < Luma(out[j].R, out[j].G, out[j].B)
	})
	return out
}

// RandomColors returns a random but usable palette: a random color-theory
// scheme, or a random multi-stop gradient.
func RandomColors(n int, rng *rand.Rand) []RGB {
	if rng.Intn(3) == 0 {
		stops := make([]RGB, 2+rng.Intn(3))
		for i := range stops {
			t := float64(i) / float64(len(stops)-1)
			stops[i] = FromOklch(0.12+0.8*t, 0.05+rng.Float64()*0.2, rng.Float64()*360)
		}
		return Gradient(stops, n)
	}
	return Harmony(Schemes[rng.Intn(len(Schemes))], rng.Float64()*360, 0.06+rng.Float64()*0.22,
		0.05+rng.Float64()*0.2, 0.8+rng.Float64()*0.2, n)
}

var hexRe = regexp.MustCompile(`(?i)(?:#|0x|\b)([0-9a-f]{6})\b`)

// ParsePaletteText reads colors from the common palette file formats: one
// hex per line (.hex), GIMP .gpl, JSON arrays, comma-separated lists.
func ParsePaletteText(text string) ([]RGB, error) {
	var out []RGB
	add := func(c RGB) {
		if len(out) < 256 {
			out = append(out, c)
		}
	}
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "GIMP Palette") {
		for _, line := range strings.Split(trimmed, "\n")[1:] {
			f := strings.Fields(line)
			if len(f) < 3 || strings.HasPrefix(line, "#") || strings.Contains(f[0], ":") {
				continue
			}
			var v [3]int
			ok := true
			for i := range v {
				n, err := strconv.Atoi(f[i])
				ok = ok && err == nil && n >= 0 && n <= 255
				v[i] = n
			}
			if ok {
				add(RGB{uint8(v[0]), uint8(v[1]), uint8(v[2])})
			}
		}
	} else {
		if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
			// JSON: pull every string out, wherever it sits.
			var any interface{}
			if json.Unmarshal([]byte(trimmed), &any) == nil {
				var sb strings.Builder
				var walk func(v interface{})
				walk = func(v interface{}) {
					switch t := v.(type) {
					case string:
						sb.WriteString(t + "\n")
					case []interface{}:
						for _, e := range t {
							walk(e)
						}
					case map[string]interface{}:
						for _, e := range t {
							walk(e)
						}
					}
				}
				walk(any)
				trimmed = sb.String()
			}
		}
		for _, m := range hexRe.FindAllStringSubmatch(trimmed, -1) {
			if c, err := ParseHex(m[1]); err == nil {
				add(c)
			}
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no colors found (expected hex colors such as #1a1c2c)")
	}
	return out, nil
}

// --- charset generators -------------------------------------------------------

// asciiByDensity is printable ASCII ordered from the emptiest glyph to the
// fullest, as measured on typical monospace fonts.
const asciiByDensity = " `.-':_,^=;><+!rc*/z?sLTv)J7(|Fi{C}fI31tlu[neoZ5Yxjya]2ESwqkP6h9d4VpOGbUAKXHm8RD#$Bg0MNWQ%&@"

// Density estimates how much of its cell a glyph fills, 0..1. It is exact
// for braille and block elements and a good guess for everything else.
func Density(r rune) float64 {
	switch {
	case r == ' ' || r == 0x2800 || r == 0x3000:
		return 0
	case r >= 0x2800 && r <= 0x28ff: // braille: count the dots
		n := 0
		for v := r - 0x2800; v != 0; v &= v - 1 {
			n++
		}
		return float64(n) / 8 * 0.6
	case r == '█':
		return 1
	case r == '▓':
		return 0.75
	case r == '▒':
		return 0.5
	case r == '░':
		return 0.25
	case r >= 0x2581 && r <= 0x2588: // lower blocks ▁..█
		return float64(r-0x2580) / 8
	case r >= 0x2589 && r <= 0x258f: // left blocks ▉..▏
		return float64(0x2590-r) / 8
	case r == '▀' || r == '▄' || r == '▌' || r == '▐':
		return 0.5
	case r >= 0x2596 && r <= 0x259f: // quadrants
		return [...]float64{0.25, 0.25, 0.25, 0.75, 0.5, 0.75, 0.75, 0.25, 0.5, 0.75}[r-0x2596]
	case r >= 0x2500 && r <= 0x257f: // box drawing
		if (r >= 0x2550 && r <= 0x256c) || r == 0x2501 || r == 0x2503 || (r >= 0x250f && r <= 0x254b && r%4 == 3) {
			return 0.32
		}
		return 0.2
	case r >= 0x25a0 && r <= 0x25ff: // geometric shapes
		switch r {
		case '■', '◼', '●', '◆', '▲', '▼', '◀', '▶':
			return 0.62
		case '▪', '•', '◾':
			return 0.3
		}
		return 0.34
	case r >= 0xff61 && r <= 0xff9f: // half-width katakana
		return 0.18 + float64((r*7)%23)/100
	}
	if i := strings.IndexRune(asciiByDensity, r); i >= 0 {
		return float64(i) / float64(len(asciiByDensity)-1) * 0.55
	}
	switch {
	case unicode.IsPunct(r):
		return 0.12
	case unicode.IsSymbol(r):
		return 0.3
	case unicode.IsUpper(r), unicode.IsDigit(r):
		return 0.42
	}
	return 0.34
}

// SortByDensity orders a set of glyphs from the emptiest to the fullest and
// drops duplicates, which is what an ASCII ramp needs.
func SortByDensity(chars string) string {
	seen := map[rune]bool{}
	var rs []rune
	for _, r := range chars {
		if !seen[r] && !unicode.IsControl(r) {
			seen[r] = true
			rs = append(rs, r)
		}
	}
	sort.SliceStable(rs, func(i, j int) bool { return Density(rs[i]) < Density(rs[j]) })
	return string(rs)
}

// GlyphSets are the pools the charset generator draws from.
var GlyphSets = []struct{ Name, Chars string }{
	{"ascii", asciiByDensity},
	{"letters", " abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"},
	{"digits", " 0123456789"},
	{"punctuation", " .,:;'`\"!?-_~^*+=/\\|()[]{}<>#%&@$"},
	{"blocks", " ░▒▓█▁▂▃▄▅▆▇▏▎▍▌▋▊▉▖▗▘▝▚▞▙▛▜▟▀▐"},
	{"shades", " ░▒▓█"},
	{"bars", " ▁▂▃▄▅▆▇█"},
	{"box", " ─│┌┐└┘├┤┬┴┼━┃┏┓┗┛┣┫┳┻╋═║╔╗╚╝╠╣╦╩╬"},
	{"braille", " ⠁⠂⠄⠈⠐⠠⡀⢀⠃⠅⠉⠑⠡⡁⢁⠇⠋⠓⠣⡃⢃⠏⠗⠧⡇⢇⠟⠯⡏⢏⠿⡟⢟⡿⢿⣿"},
	{"shapes", " ·•○◌◍◎●◐◑◒◓◔◕◖◗□▢▣▤▥▦▧▨▩■▪▫▬▭▮▯▰▱△▲▽▼◇◆◈"},
	{"math", " ·∙∘∶∷≈≋≡≣∑∏∫√∞±×÷≠≤≥⊕⊗⊙⊞⊠"},
	{"arrows", " ←↑→↓↔↕↖↗↘↙⇐⇑⇒⇓⇔"},
	{"katakana", " ･ｰｧｨｩｱｲｳｴｵｶｷｸｹｻｼｽｾﾀﾁﾂﾃﾄﾅﾆﾇﾈﾊﾋﾌﾍﾎﾏﾐﾑﾒﾓﾔﾕﾖﾗﾘﾙﾚﾛﾜﾝ"},
	{"runes", " ᚠᚢᚦᚨᚱᚲᚷᚹᚺᚾᛁᛃᛇᛈᛉᛊᛏᛒᛖᛗᛚᛜᛞᛟ"},
	{"cards", " ♠♡♢♣♤♥♦♧♩♪♫♬"},
	{"games", " ⚀⚁⚂⚃⚄⚅♙♘♗♖♕♔♟♞♝♜♛♚"},
	{"greek", " ·ιτγνλσπαεδβθψωΞΣΦΩΨ"},
	{"cyrillic", " ·гтсукеаодлбяфюжщш"},
}

// GlyphSetNames lists the generator's pools.
func GlyphSetNames() []string {
	out := make([]string, len(GlyphSets))
	for i, g := range GlyphSets {
		out[i] = g.Name
	}
	return out
}

// GenCharset builds a ramp of about n glyphs from a pool. The glyphs are
// picked evenly across the pool's density range, so the ramp has a usable
// spread of brightness; with shuffle the picks within each band are random.
func GenCharset(set string, n int, shuffle bool, rng *rand.Rand) string {
	pool := GlyphSets[0].Chars
	for _, g := range GlyphSets {
		if g.Name == set {
			pool = g.Chars
		}
	}
	rs := []rune(SortByDensity(pool))
	if len(rs) == 0 || rs[0] != ' ' {
		rs = append([]rune{' '}, rs...)
	}
	n = max(2, min(n, len(rs)))
	out := make([]rune, 0, n)
	for i := 0; i < n; i++ {
		lo := i * len(rs) / n
		hi := max(lo+1, (i+1)*len(rs)/n)
		k := lo
		if i == n-1 {
			k = len(rs) - 1
		} else if shuffle && i > 0 {
			k = lo + rng.Intn(hi-lo)
		}
		out = append(out, rs[k])
	}
	return string(out)
}

// RampContrast reports how evenly a ramp's glyphs spread from empty to full
// (1 = perfectly even), as a hint for the charset editor.
func RampContrast(chars string) float64 {
	rs := []rune(chars)
	if len(rs) < 2 {
		return 0
	}
	lo, hi := math.MaxFloat64, 0.0
	for _, r := range rs {
		d := Density(r)
		lo, hi = math.Min(lo, d), math.Max(hi, d)
	}
	return hi - lo
}
