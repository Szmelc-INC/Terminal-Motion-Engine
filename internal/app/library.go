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
// glyph ramps, interface themes, sound presets and effect presets. It lives
// in library.json next to the preset file.
type Library struct {
	Path     string         `json:"-"`
	Palettes []PaletteEntry `json:"palettes"`
	Charsets []CharsetEntry `json:"charsets"`
	Themes   []Theme        `json:"themes"`
	Sounds   []SoundEntry   `json:"sounds"`
	FX       []FXEntry      `json:"effects"`
}

// FXEntry is a user effect preset.
type FXEntry struct {
	Name string    `json:"name"`
	FX   engine.FX `json:"fx"`
}

// UnmarshalJSON decodes over the defaults, like SoundEntry.
func (e *FXEntry) UnmarshalJSON(b []byte) error {
	type plain FXEntry
	p := plain{FX: engine.DefaultFX()}
	err := json.Unmarshal(b, &p)
	*e = FXEntry(p)
	return err
}

// AllFX lists user effect presets first, then the built-in ones they do
// not shadow.
func (l *Library) AllFX() []engine.FXPreset {
	var out []engine.FXPreset
	for _, f := range l.FX {
		out = append(out, engine.FXPreset{Name: f.Name, FX: f.FX})
	}
	for _, b := range engine.BuiltinFX {
		dup := false
		for _, u := range l.FX {
			dup = dup || strings.EqualFold(u.Name, b.Name)
		}
		if !dup {
			out = append(out, b)
		}
	}
	return out
}

// FindFX looks an effect preset up by name.
func (l *Library) FindFX(name string) (engine.FXPreset, bool) {
	for _, f := range l.AllFX() {
		if strings.EqualFold(f.Name, name) {
			return f, true
		}
	}
	return engine.FXPreset{}, false
}

// SaveFX creates or replaces a user effect preset.
func (l *Library) SaveFX(name string, f engine.FX) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	for i := range l.FX {
		if strings.EqualFold(l.FX[i].Name, name) {
			l.FX[i].FX = f
			return l.save()
		}
	}
	l.FX = append(l.FX, FXEntry{Name: name, FX: f})
	return l.save()
}

// SoundEntry is a user sound preset.
type SoundEntry struct {
	Name  string       `json:"name"`
	Sound engine.Sound `json:"sound"`
}

// UnmarshalJSON decodes over the defaults, so a preset written by an older
// version gets sane values for the settings added since.
func (e *SoundEntry) UnmarshalJSON(b []byte) error {
	type plain SoundEntry
	p := plain{Sound: engine.DefaultSound()}
	err := json.Unmarshal(b, &p)
	*e = SoundEntry(p)
	return err
}

// AllSounds lists user sound presets first, then the built-in ones they do
// not shadow.
func (l *Library) AllSounds() []engine.SoundPreset {
	var out []engine.SoundPreset
	for _, s := range l.Sounds {
		out = append(out, engine.SoundPreset{Name: s.Name, Sound: s.Sound})
	}
	for _, b := range engine.BuiltinSounds {
		dup := false
		for _, u := range l.Sounds {
			dup = dup || strings.EqualFold(u.Name, b.Name)
		}
		if !dup {
			out = append(out, b)
		}
	}
	return out
}

// FindSound looks a sound preset up by name.
func (l *Library) FindSound(name string) (engine.SoundPreset, bool) {
	for _, s := range l.AllSounds() {
		if strings.EqualFold(s.Name, name) {
			return s, true
		}
	}
	return engine.SoundPreset{}, false
}

// SaveSound creates or replaces a user sound preset.
func (l *Library) SaveSound(name string, s engine.Sound) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	for i := range l.Sounds {
		if strings.EqualFold(l.Sounds[i].Name, name) {
			l.Sounds[i].Sound = s
			return l.save()
		}
	}
	l.Sounds = append(l.Sounds, SoundEntry{Name: name, Sound: s})
	return l.save()
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
// "theme", "sound", "fx").
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
	case "sound":
		for i, p := range l.Sounds {
			if strings.EqualFold(p.Name, name) {
				l.Sounds, found = append(l.Sounds[:i], l.Sounds[i+1:]...), true
				break
			}
		}
	case "fx":
		for i, p := range l.FX {
			if strings.EqualFold(p.Name, name) {
				l.FX, found = append(l.FX[:i], l.FX[i+1:]...), true
				break
			}
		}
	}
	what := kind
	if kind == "fx" {
		what = "effect preset"
	}
	if !found {
		return fmt.Errorf("no user %s named %q (built-in ones cannot be deleted)", what, name)
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
	case "sound":
		for i := range l.Sounds {
			check(&l.Sounds[i].Name)
		}
	case "fx":
		for i := range l.FX {
			check(&l.FX[i].Name)
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

	// Hints shows the keys of the active bind mode above the seek bar.
	Hints bool `json:"key_hints"`
	// Keys is the bind mode to start in: a mode name, or "last" for the one
	// that was active when termo was closed, which Mode remembers.
	Keys string `json:"keys"`
	Mode string `json:"last_keys,omitempty"`
}

// startMode is the name of the bind mode to start in.
func (p *Prefs) startMode() string {
	if p.Keys == "last" {
		return p.Mode
	}
	return p.Keys
}

// LoadPrefs reads config.json from dir; a missing file gives the defaults.
func LoadPrefs(dir string) (*Prefs, error) {
	p := &Prefs{Path: filepath.Join(dir, "config.json"), UI: "normal", Theme: BuiltinThemes[0].Name, HUD: "auto",
		Hints: true, Keys: "play"}
	err := readJSON(p.Path, p)
	if p.Keys != "last" && findMode(p.Keys) < 0 {
		p.Keys = "play"
	}
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
