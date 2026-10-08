package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
)

// PaletteEntry is a user color palette.
type PaletteEntry struct {
	Name   string   `json:"name"`
	Colors []string `json:"colors"`
}

// CharsetEntry is a user glyph ramp, emptiest glyph first.
type CharsetEntry struct {
	Name  string `json:"name"`
	Chars string `json:"chars"`
}

// Library holds everything the user made besides presets: color palettes,
// glyph ramps and interface themes. It lives in library.json next to the
// preset file.
type Library struct {
	Path     string         `json:"-"`
	Palettes []PaletteEntry `json:"palettes"`
	Charsets []CharsetEntry `json:"charsets"`
	Themes   []Theme        `json:"themes"`
}

func writeJSON(path string, v any, mode os.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %v", path, err)
	}
	return nil
}

// LoadLibrary reads library.json from dir and registers its palettes and
// charsets with the engine. A missing file is an empty library.
func LoadLibrary(dir string) (*Library, error) {
	l := &Library{Path: filepath.Join(dir, "library.json")}
	err := readJSON(l.Path, l)
	l.sync()
	return l, err
}

// sync hands the palettes and charsets to the engine.
func (l *Library) sync() {
	var pals []engine.NamedPalette
	for _, p := range l.Palettes {
		var cols []engine.RGB
		for _, h := range p.Colors {
			if c, err := engine.ParseHex(h); err == nil {
				cols = append(cols, c)
			}
		}
		if len(cols) > 0 {
			pals = append(pals, engine.NamedPalette{Name: p.Name, Colors: cols})
		}
	}
	engine.SetUserPalettes(pals)
	var sets []engine.Charset
	for _, c := range l.Charsets {
		if r := []rune(c.Chars); len(r) >= 2 {
			sets = append(sets, engine.Charset{Name: c.Name, Runes: r})
		}
	}
	engine.SetUserCharsets(sets)
}

func (l *Library) save() error {
	l.sync()
	return writeJSON(l.Path, l, 0o644)
}

// reserved names cannot be given to a palette: they select generators.
var reservedPalettes = []string{engine.PalOff, engine.PalGray, engine.PalCube, engine.PalAnsi16,
	engine.PalXterm, engine.PalHarmony, engine.PalAdaptive, engine.PalCustom}

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if err := ValidName(name); err != nil {
		return "", errors.New(strings.Replace(err.Error(), "preset name", "name", 1))
	}
	if strings.ContainsAny(name, ",/\\") {
		return "", errors.New("names cannot contain , / or \\")
	}
	return name, nil
}

// SavePalette creates or replaces a user palette.
func (l *Library) SavePalette(name string, colors []engine.RGB) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	for _, r := range reservedPalettes {
		if strings.EqualFold(r, name) {
			return fmt.Errorf("%q is the name of a built-in generator, pick another name", name)
		}
	}
	if len(colors) == 0 {
		return errors.New("a palette needs at least one color")
	}
	if len(colors) > 256 {
		colors = colors[:256]
	}
	e := PaletteEntry{Name: name}
	for _, c := range colors {
		e.Colors = append(e.Colors, c.Hex())
	}
	for i := range l.Palettes {
		if strings.EqualFold(l.Palettes[i].Name, name) {
			l.Palettes[i] = e
			return l.save()
		}
	}
	l.Palettes = append(l.Palettes, e)
	return l.save()
}

// SaveCharset creates or replaces a user glyph ramp.
func (l *Library) SaveCharset(name, chars string) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	if strings.EqualFold(name, "custom") {
		return errors.New(`"custom" is reserved, pick another name`)
	}
	if len([]rune(chars)) < 2 {
		return errors.New("a charset needs at least two characters")
	}
	e := CharsetEntry{Name: name, Chars: chars}
	for i := range l.Charsets {
		if strings.EqualFold(l.Charsets[i].Name, name) {
			l.Charsets[i] = e
			return l.save()
		}
	}
	l.Charsets = append(l.Charsets, e)
	return l.save()
}

// SaveTheme creates or replaces a user theme.
func (l *Library) SaveTheme(t Theme) error {
	name, err := cleanName(t.Name)
	if err != nil {
		return err
	}
	t.Name, t.Builtin = name, false
	for i := range l.Themes {
		if strings.EqualFold(l.Themes[i].Name, name) {
			l.Themes[i] = t
			return l.save()
		}
	}
	l.Themes = append(l.Themes, t)
	return l.save()
}

// Delete removes a user item of the given kind ("palette", "charset",
// "theme").
func (l *Library) Delete(kind, name string) error {
	found := false
	switch kind {
	case "palette":
		for i, p := range l.Palettes {
			if strings.EqualFold(p.Name, name) {
				l.Palettes, found = append(l.Palettes[:i], l.Palettes[i+1:]...), true
				break
			}
		}
	case "charset":
		for i, p := range l.Charsets {
			if strings.EqualFold(p.Name, name) {
				l.Charsets, found = append(l.Charsets[:i], l.Charsets[i+1:]...), true
				break
			}
		}
	case "theme":
		for i, p := range l.Themes {
			if strings.EqualFold(p.Name, name) {
				l.Themes, found = append(l.Themes[:i], l.Themes[i+1:]...), true
				break
			}
		}
	}
	if !found {
		return fmt.Errorf("no user %s named %q (built-in ones cannot be deleted)", kind, name)
	}
	return l.save()
}

// Rename renames a user item of the given kind.
func (l *Library) Rename(kind, old, name string) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	var target *string
	clash := false
	check := func(n *string) {
		if strings.EqualFold(*n, old) {
			target = n
		} else if strings.EqualFold(*n, name) {
			clash = true
		}
	}
	switch kind {
	case "palette":
		for i := range l.Palettes {
			check(&l.Palettes[i].Name)
		}
	case "charset":
		for i := range l.Charsets {
			check(&l.Charsets[i].Name)
		}
	case "theme":
		for i := range l.Themes {
			check(&l.Themes[i].Name)
		}
	}
	if target == nil {
		return fmt.Errorf("no user %s named %q (built-in ones cannot be renamed)", kind, old)
	}
	if clash {
		return fmt.Errorf("a %s named %q already exists", kind, name)
	}
	*target = name
	return l.save()
}

// AllThemes lists user themes first, then the built-in ones.
func (l *Library) AllThemes() []Theme {
	out := append([]Theme(nil), l.Themes...)
	for _, b := range BuiltinThemes {
		dup := false
		for _, u := range l.Themes {
			dup = dup || strings.EqualFold(u.Name, b.Name)
		}
		if !dup {
			out = append(out, b)
		}
	}
	return out
}

// FindTheme looks a theme up by name.
func (l *Library) FindTheme(name string) (Theme, bool) {
	for _, t := range l.AllThemes() {
		if strings.EqualFold(t.Name, name) {
			return t, true
		}
	}
	return Theme{}, false
}

// Prefs are the settings that describe the program rather than a look:
// interface size, theme, folders, API keys. They live in config.json, which
// is written with mode 0600 because it can hold keys.
type Prefs struct {
	Path        string `json:"-"`
	UI          string `json:"ui"`
	Theme       string `json:"theme"`
	HUD         string `json:"hud"`
	Stats       bool   `json:"stats"`
	DownloadDir string `json:"download_dir"`
	YouTubeKey  string `json:"youtube_api_key"`
	GiphyKey    string `json:"giphy_api_key"`
}

// LoadPrefs reads config.json from dir; a missing file gives the defaults.
func LoadPrefs(dir string) (*Prefs, error) {
	p := &Prefs{Path: filepath.Join(dir, "config.json"), UI: "normal", Theme: BuiltinThemes[0].Name, HUD: "auto"}
	err := readJSON(p.Path, p)
	if UIScale(p.UI) < 0 {
		p.UI = "normal"
	}
	return p, err
}

// Save writes config.json.
func (p *Prefs) Save() error { return writeJSON(p.Path, p, 0o600) }

// Downloads is the folder downloaded media goes to.
func (p *Prefs) Downloads() string {
	if d := strings.TrimSpace(p.DownloadDir); d != "" {
		if strings.HasPrefix(d, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				d = filepath.Join(home, d[2:])
			}
		}
		return d
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Videos", "termo")
	}
	return "termo-downloads"
}
