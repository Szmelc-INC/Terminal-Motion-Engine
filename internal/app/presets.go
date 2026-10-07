// Package app is the interactive player: playback loop, HUD, settings
// menu, dialogs and preset storage.
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

// Store keeps user presets in a JSON file. Built-in presets are always
// listed alongside them; a user preset with the same name shadows the
// built-in one.
type Store struct {
	Path string
	User []engine.Preset
}

type storeFile struct {
	Presets []storedPreset `json:"presets"`
}

type storedPreset struct {
	Name string          `json:"name"`
	Look json.RawMessage `json:"look"`
}

// StorePath returns the preset file location, honouring XDG_CONFIG_HOME.
func StorePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "termo", "presets.json")
}

// LoadStore reads the preset file. A missing file is an empty store.
func LoadStore(path string) (*Store, error) {
	s := &Store{Path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	var f storeFile
	if err := json.Unmarshal(data, &f); err != nil {
		return s, fmt.Errorf("%s: %v", path, err)
	}
	for _, sp := range f.Presets {
		// Decode over the defaults so presets written by an older
		// version pick up sane values for settings added since.
		l := engine.DefaultLook()
		if err := json.Unmarshal(sp.Look, &l); err != nil {
			return s, fmt.Errorf("%s: preset %q: %v", path, sp.Name, err)
		}
		s.User = append(s.User, engine.Preset{Name: sp.Name, Look: l})
	}
	return s, nil
}

func (s *Store) write() error {
	f := storeFile{Presets: []storedPreset{}}
	for _, p := range s.User {
		raw, err := json.Marshal(p.Look)
		if err != nil {
			return err
		}
		f.Presets = append(f.Presets, storedPreset{Name: p.Name, Look: raw})
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}

func (s *Store) userIndex(name string) int {
	for i, p := range s.User {
		if strings.EqualFold(p.Name, name) {
			return i
		}
	}
	return -1
}

// All lists user presets first, then the built-ins they do not shadow.
func (s *Store) All() []engine.Preset {
	out := append([]engine.Preset(nil), s.User...)
	for _, b := range engine.BuiltinPresets {
		if s.userIndex(b.Name) < 0 {
			out = append(out, b)
		}
	}
	return out
}

// Find looks a preset up by name (case-insensitive).
func (s *Store) Find(name string) (engine.Preset, bool) {
	for _, p := range s.All() {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return engine.Preset{}, false
}

// ValidName reports whether a preset name is acceptable.
func ValidName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("preset name cannot be empty")
	}
	if len([]rune(name)) > 40 {
		return errors.New("preset name is too long (max 40 characters)")
	}
	return nil
}

// Save creates or overwrites a user preset.
func (s *Store) Save(name string, l engine.Look) error {
	name = strings.TrimSpace(name)
	if err := ValidName(name); err != nil {
		return err
	}
	l.Resample = 0
	if i := s.userIndex(name); i >= 0 {
		s.User[i].Look = l
	} else {
		s.User = append(s.User, engine.Preset{Name: name, Look: l})
	}
	return s.write()
}

// Delete removes a user preset.
func (s *Store) Delete(name string) error {
	i := s.userIndex(name)
	if i < 0 {
		if _, ok := s.Find(name); ok {
			return fmt.Errorf("%q is a built-in preset and cannot be deleted", name)
		}
		return fmt.Errorf("no preset named %q", name)
	}
	s.User = append(s.User[:i], s.User[i+1:]...)
	return s.write()
}

// Rename changes a user preset's name.
func (s *Store) Rename(old, name string) error {
	name = strings.TrimSpace(name)
	if err := ValidName(name); err != nil {
		return err
	}
	i := s.userIndex(old)
	if i < 0 {
		return fmt.Errorf("%q is a built-in preset; save a copy under a new name instead", old)
	}
	if j := s.userIndex(name); j >= 0 && j != i {
		return fmt.Errorf("a preset named %q already exists", name)
	}
	s.User[i].Name = name
	return s.write()
}

// Summary describes a look in a few words.
func Summary(l engine.Look) string {
	pal := l.Palette
	switch l.Palette {
	case engine.PalHarmony:
		pal = fmt.Sprintf("%s×%d", l.Scheme, l.Colors)
	case engine.PalGray, engine.PalCube, engine.PalAdaptive:
		pal = fmt.Sprintf("%s×%d", l.Palette, l.Colors)
	case engine.PalCustom:
		pal = fmt.Sprintf("custom×%d", len(l.Custom))
	}
	if !l.Color {
		pal = "mono"
	}
	mode := l.Mode
	if l.Mode == engine.ModeASCII {
		mode += "/" + l.Charset
	}
	return mode + " · " + pal + " · " + l.Dither
}
