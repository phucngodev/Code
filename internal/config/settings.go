package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Header is written at the top of a new config file.
const Header = `# Code settings. The Settings window rewrites the lines it manages; all
# other lines are preserved. Options Code does not support are listed in
# Settings > Advanced.

`

// Paths holds where Code keeps its files.
type Paths struct {
	// Config is the config file the Settings window reads and writes.
	Config string
	// Themes is the directory themes are found in by name.
	Themes string
	// Session is where the window and workspace layout is saved.
	Session string
	// SocketDir holds the per-process agent sockets.
	SocketDir string
}

// DefaultPaths derives Code's file locations, honouring CODE_CONFIG and
// XDG_CONFIG_HOME as the macOS app does.
func DefaultPaths() Paths {
	if override := os.Getenv("CODE_CONFIG"); override != "" {
		dir := filepath.Dir(override)
		return Paths{
			Config:    override,
			Themes:    filepath.Join(dir, "themes"),
			Session:   filepath.Join(dir, "session.json"),
			SocketDir: dir,
		}
	}
	dir := configHome()
	code := filepath.Join(dir, "code")
	return Paths{
		Config:    filepath.Join(code, "config"),
		Themes:    filepath.Join(code, "themes"),
		Session:   filepath.Join(code, "session.json"),
		SocketDir: code,
	}
}

// configHome is the XDG config home, which is ~/.config when unset.
func configHome() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".config")
}

// GhosttyFiles lists the config files Ghostty loads, in load order.
func GhosttyFiles() []string {
	dir := configHome()
	home, _ := os.UserHomeDir()
	xdg := filepath.Join(dir, "ghostty")
	appSupport := ""
	if home != "" {
		appSupport = filepath.Join(home, "Library", "Application Support", "com.mitchellh.ghostty")
	}
	files := []string{
		filepath.Join(xdg, "config"),
		filepath.Join(xdg, "config.ghostty"),
	}
	if appSupport != "" {
		files = append(files,
			filepath.Join(appSupport, "config"),
			filepath.Join(appSupport, "config.ghostty"))
	}
	return files
}

// Settings is the config file as the app sees it. It is safe for concurrent
// use; changes are written to disk as they are made.
type Settings struct {
	mu   sync.Mutex
	path string
	doc  *Document
	read error
	// OnSave runs after a successful write, to load the new file into the
	// terminals.
	OnSave func()

	// saveMu guards saveTimer, so a burst of changes writes once.
	saveMu    sync.Mutex
	saveTimer *time.Timer
}

// LoadSettings reads the config file. A missing file loads as an empty
// document; one that exists but cannot be read is remembered as a read error
// so the Settings window leaves it alone.
func LoadSettings(path string) *Settings {
	s := &Settings{path: path}
	s.reload()
	return s
}

// Path returns the config file's path.
func (s *Settings) Path() string { return s.path }

// Document returns a copy of the parsed file.
func (s *Settings) Document() *Document {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ParseDocument(s.doc.Text())
}

// Reload re-reads the config file, so edits made outside Code are picked up
// by Reload Configuration. A read error is remembered the same way a load
// error is, leaving the previous document in place.
func (s *Settings) Reload() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reload()
}

// ReadError returns why the file could not be read, if it could not.
func (s *Settings) ReadError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read
}

// UnsupportedKeys lists the config keys Code does not read, in the order the
// file first sets them, so the Settings window can say what it ignores.
func (s *Settings) UnsupportedKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc == nil {
		return nil
	}
	var out []string
	for _, key := range s.doc.Keys() {
		if !supportedKeys[key] {
			out = append(out, key)
		}
	}
	return out
}

func (s *Settings) reload() {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.doc = ParseDocument(Header)
		s.read = nil
		return
	}
	if err != nil {
		s.read = err
		return
	}
	s.doc = ParseDocument(string(data))
	s.read = nil
}

// String returns the last value of key, or def when unset or unparseable.
func (s *Settings) String(key, def string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.doc.Get(key); ok {
		return v
	}
	return def
}

// Float returns a numeric option, or def when unset or unparseable.
func (s *Settings) Float(key string, def float64) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.doc.Get(key); ok {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f
		}
	}
	return def
}

// Int returns an integer option, or def when unset or unparseable.
func (s *Settings) Int(key string, def int) int {
	return int(s.Float(key, float64(def)))
}

// Bool returns a boolean option, accepting Ghostty's spellings.
func (s *Settings) Bool(key string, def bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.doc.Get(key); ok {
		if b, ok := parseBool(v); ok {
			return b
		}
	}
	return def
}

func parseBool(text string) (bool, bool) {
	switch strings.TrimSpace(text) {
	case "1", "t", "T", "true", "yes", "on":
		return true, true
	case "0", "f", "F", "false", "no", "off":
		return false, true
	}
	return false, false
}

// Strings returns every value of a key, in order, for list options such as
// font-family.
func (s *Settings) Strings(key string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doc.Values(key)
}

// WorkingDirectory is the directory a new terminal starts in, or the user's
// home when the config sets none.
func (s *Settings) WorkingDirectory() string {
	dir := s.String(KeyWorkingDirectory, "")
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	return dir
}

// Value returns a key's value and whether the file sets it.
func (s *Settings) Value(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doc.Get(key)
}

// Set writes a value, removing the key when value is empty so Ghostty's
// default applies, then saves.
func (s *Settings) Set(key, value string) {
	s.mu.Lock()
	if value == "" {
		s.doc.Remove(key)
	} else {
		s.doc.Set(key, value)
	}
	s.mu.Unlock()
	s.scheduleSave()
}

// SetValues writes several values for a list option such as font-family.
func (s *Settings) SetValues(key string, values []string) {
	s.mu.Lock()
	s.doc.SetValues(values, key)
	s.mu.Unlock()
	s.scheduleSave()
}

// scheduleSave writes the file shortly after changes stop, so dragging a
// slider writes once rather than at every step.
func (s *Settings) scheduleSave() {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	if s.saveTimer != nil {
		return
	}
	s.saveTimer = time.AfterFunc(250*time.Millisecond, func() {
		s.saveMu.Lock()
		s.saveTimer = nil
		s.saveMu.Unlock()
		_ = s.save()
	})
}

// Save writes the file now, cancelling a scheduled write. It is a no-op while
// the file could not be read, so Settings never replaces whatever is in it.
func (s *Settings) Save() error {
	s.saveMu.Lock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
		s.saveTimer = nil
	}
	s.saveMu.Unlock()
	return s.save()
}

func (s *Settings) save() error {
	s.mu.Lock()
	if s.read != nil {
		err := s.read
		s.mu.Unlock()
		return err
	}
	text := s.doc.Text()
	onSave := s.OnSave
	s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(text), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	if onSave != nil {
		onSave()
	}
	return nil
}

// CreateIfMissing makes the config file with the header and the settings
// imported from Ghostty, so someone who set Code up through Ghostty keeps
// their fonts, theme and keybindings.
func CreateIfMissing(paths Paths) error {
	if _, err := os.Stat(paths.Config); err == nil {
		return nil
	}
	text := Header + importedFromGhostty()
	if err := os.MkdirAll(filepath.Dir(paths.Config), 0o755); err != nil {
		return err
	}
	return os.WriteFile(paths.Config, []byte(text), 0o644)
}

// importedFromGhostty reads Ghostty's config files and returns their settings,
// each under a note of where it came from. Files that only hold comments are
// left out.
func importedFromGhostty() string {
	var b strings.Builder
	for _, file := range GhosttyFiles() {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		doc := ParseDocument(strings.TrimPrefix(string(data), "\uFEFF"))
		if doc.IsEmpty() {
			continue
		}
		fmt.Fprintf(&b, "\n# Imported from %s\n", file)
		b.WriteString(doc.Text())
	}
	return b.String()
}
