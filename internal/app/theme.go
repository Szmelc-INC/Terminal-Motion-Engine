package app

import (
	"math/rand"
	"strings"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
)

// Interface colors. They are variables so a theme can replace them.
var (
	cBar    uint32 = 0x16161e
	cPanel  uint32 = 0x1a1b26
	cFg     uint32 = 0xc0caf5
	cDim    uint32 = 0x565f89
	cAccent uint32 = 0x7aa2f7
	cSel    uint32 = 0x283457
	cHot    uint32 = 0x3b4261
	cGreen  uint32 = 0x9ece6a
	cYellow uint32 = 0xe0af68
	cRed    uint32 = 0xf7768e
	cTrack  uint32 = 0x414868
	cBtn    uint32 = 0x24283b
)

// ThemeSlots names the colors of a theme, in the order they are stored.
var ThemeSlots = []string{"bar", "panel", "text", "dim", "accent", "selection", "hover",
	"ok", "warn", "error", "track", "button"}

var themeVars = []*uint32{&cBar, &cPanel, &cFg, &cDim, &cAccent, &cSel, &cHot, &cGreen, &cYellow, &cRed, &cTrack, &cBtn}

// Theme is a named set of interface colors (hex strings, see ThemeSlots).
type Theme struct {
	Name    string   `json:"name"`
	Colors  []string `json:"colors"`
	Builtin bool     `json:"-"`
}

func theme(name, hex string) Theme {
	f := strings.Fields(hex)
	for i := range f {
		f[i] = "#" + f[i]
	}
	return Theme{Name: name, Colors: f, Builtin: true}
}

// BuiltinThemes ship with termo.
var BuiltinThemes = []Theme{
	theme("tokyo-night", "16161e 1a1b26 c0caf5 565f89 7aa2f7 283457 3b4261 9ece6a e0af68 f7768e 414868 24283b"),
	theme("gruvbox", "1d2021 282828 ebdbb2 928374 83a598 504945 665c54 b8bb26 fabd2f fb4934 3c3836 32302f"),
	theme("dracula", "21222c 282a36 f8f8f2 6272a4 bd93f9 44475a 565a70 50fa7b f1fa8c ff5555 44475a 343746"),
	theme("nord", "2e3440 3b4252 eceff4 7b88a1 88c0d0 434c5e 4c566a a3be8c ebcb8b bf616a 4c566a 434c5e"),
	theme("catppuccin", "181825 1e1e2e cdd6f4 6c7086 89b4fa 313244 45475a a6e3a1 f9e2af f38ba8 45475a 313244"),
	theme("solarized", "002b36 073642 eee8d5 657b83 268bd2 0b4a5a 14586a 859900 b58900 dc322f 586e75 0a4050"),
	theme("monokai", "1e1f1c 272822 f8f8f2 75715e 66d9ef 3e3d32 49483e a6e22e e6db74 f92672 49483e 34352f"),
	theme("one-dark", "21252b 282c34 abb2bf 5c6370 61afef 3e4451 4b5263 98c379 e5c07b e06c75 4b5263 2c313a"),
	theme("rose-pine", "191724 1f1d2e e0def4 6e6a86 c4a7e7 26233a 403d52 9ccfd8 f6c177 eb6f92 403d52 2a273f"),
	theme("everforest", "232a2e 2d353b d3c6aa 7a8478 7fbbb3 3d484d 475258 a7c080 dbbc7f e67e80 475258 343f44"),
	theme("kanagawa", "16161d 1f1f28 dcd7ba 727169 7e9cd8 2d4f67 363646 98bb6c e6c384 e46876 54546d 2a2a37"),
	theme("synthwave", "1a1028 241b2f f4eeff 7f6a9f ff7edb 3b2a5a 4a3470 72f1b8 fede5d fe4450 495495 2f2240"),
	theme("cyberpunk", "0a0a12 0d0221 e0f7ff 5a6a8a 00f0ff 261447 3a1f6b 05ffa1 fcee0c ff2a6d 3a1f6b 1a0f3a"),
	theme("ocean", "06141f 0b1e2d cfe8f5 5a7a8c 4fc3f7 123247 1a425c 7ee0b0 ffd479 ff6b81 244b63 0f2a3d"),
	theme("matrix", "000000 001100 33ff66 0a7a2a 00ff41 003b00 005500 00ff41 afffaf ff4040 004400 002200"),
	theme("amber", "0a0500 140a00 ffb000 8a5a00 ffd27f 3a2200 4f2f00 ffd27f ffb000 ff5a2a 5a3800 241400"),
	theme("mono", "000000 0a0a0a e6e6e6 777777 ffffff 2a2a2a 3a3a3a ffffff cccccc ffffff 444444 1c1c1c"),
	theme("paper", "e8e3d5 f5f0e1 1c1c1c 7a7466 2a5db0 d6cfbd c9c1ac 3a7d2c 9a6a00 b3261e b0a890 e0d9c6"),
}

// RGBs returns the theme's colors, falling back to the default theme for
// slots that are missing or malformed.
func (t Theme) RGBs() []engine.RGB {
	out := make([]engine.RGB, len(ThemeSlots))
	for i := range out {
		out[i], _ = engine.ParseHex(BuiltinThemes[0].Colors[i])
		if i < len(t.Colors) {
			if c, err := engine.ParseHex(t.Colors[i]); err == nil {
				out[i] = c
			}
		}
	}
	return out
}

// applyTheme switches the interface colors.
func applyTheme(t Theme) {
	for i, c := range t.RGBs() {
		*themeVars[i] = c.U32()
	}
}

// GenTheme derives a whole theme from one hue. Neutrals carry a trace of
// the hue; the status colors stay green / yellow / red so they keep their
// meaning.
func GenTheme(name string, hue, chroma float64, light bool) Theme {
	L := func(l float64) float64 {
		if light {
			return 1.08 - l
		}
		return l
	}
	n := chroma * 0.18 // tint of the neutrals
	status := 0.74
	if light {
		status = 0.5
	}
	cols := []engine.RGB{
		engine.FromOklch(L(0.17), n, hue),           // bar
		engine.FromOklch(L(0.21), n*1.1, hue),       // panel
		engine.FromOklch(L(0.90), n, hue),           // text
		engine.FromOklch(L(0.52), n*1.6, hue),       // dim
		engine.FromOklch(L(0.74), chroma, hue),      // accent
		engine.FromOklch(L(0.33), chroma*0.45, hue), // selection
		engine.FromOklch(L(0.39), chroma*0.35, hue), // hover
		engine.FromOklch(status+0.04, 0.16, 140),    // ok
		engine.FromOklch(status+0.08, 0.14, 85),     // warn
		engine.FromOklch(status-0.06, 0.2, 22),      // error
		engine.FromOklch(L(0.42), n*1.8, hue),       // track
		engine.FromOklch(L(0.27), n*1.3, hue),       // button
	}
	t := Theme{Name: name}
	for _, c := range cols {
		t.Colors = append(t.Colors, c.Hex())
	}
	return t
}

// RandomTheme rolls a theme.
func RandomTheme(name string, rng *rand.Rand) Theme {
	return GenTheme(name, rng.Float64()*360, 0.08+rng.Float64()*0.14, rng.Intn(6) == 0)
}
