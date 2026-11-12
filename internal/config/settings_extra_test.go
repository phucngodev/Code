package config

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

// TestLoadSettingsUnreadableFile covers a config path that exists but is not a
// readable file. Code used to leave its document nil and crash on the first
// getter during startup.
func TestLoadSettingsUnreadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	s := LoadSettings(path)
	if s.ReadError() == nil {
		t.Fatal("an unreadable config should be remembered")
	}
	if got := s.String(KeyFontSize, "13"); got != "13" {
		t.Fatalf("String after an unreadable file = %q, want the default", got)
	}
	if _, ok := s.Value(KeyFontSize); ok {
		t.Fatal("an unreadable config should hold no values")
	}
	if _, cleared := s.Keybinds(); cleared {
		t.Fatal("an unreadable config should hold no keybinds")
	}
	s.Set(KeyFontSize, "20")
	if err := s.Save(); err == nil {
		t.Fatal("Save should refuse to replace a file that could not be read")
	}
	if len(s.UnsupportedKeys()) != 0 {
		t.Fatal("an unreadable config should report no keys")
	}
}

// TestReloadIfChangedSkipsOwnWrites covers the watcher reading Code's own
// debounced save back as an external edit and discarding a newer UI change.
func TestReloadIfChangedSkipsOwnWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	s := LoadSettings(path)
	s.Set(KeyFontSize, "15")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if s.Dirty() {
		t.Fatal("a saved change is no longer pending")
	}
	if s.ReloadIfChanged() {
		t.Fatal("Code's own write must not reload the document")
	}

	// A UI change that has not been written yet must survive an external edit
	// until its own save wins.
	s.Set(KeyFontSize, "16")
	if err := os.WriteFile(path, []byte(KeyFontSize+" = 18\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if s.ReloadIfChanged() {
		t.Fatal("a pending change must not be discarded by the watcher")
	}
	if got := s.String(KeyFontSize, ""); got != "16" {
		t.Fatalf("font-size = %q, want the pending edit 16", got)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if s.ReloadIfChanged() {
		t.Fatal("the save just written must not reload the document")
	}
}

func TestReloadIfChangedAppliesExternalEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	s := LoadSettings(path)
	if err := os.WriteFile(path, []byte(KeyFontSize+" = 18\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !s.ReloadIfChanged() {
		t.Fatal("an edit made outside Code should reload the document")
	}
	if got := s.String(KeyFontSize, ""); got != "18" {
		t.Fatalf("font-size = %q, want 18", got)
	}
	if s.ReloadIfChanged() {
		t.Fatal("an unchanged file should not reload")
	}
}

func TestReloadIfChangedRecoversAfterUnreadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	s := LoadSettings(path)
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	if s.ReadError() == nil {
		t.Fatal("the unreadable path should be remembered")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(KeyFontSize+" = 21\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !s.ReloadIfChanged() {
		t.Fatal("a readable file should recover the watcher")
	}
	if got := s.String(KeyFontSize, ""); got != "21" {
		t.Fatalf("font-size = %q, want 21", got)
	}
}

func TestConfigWritesArePrivate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	s := LoadSettings(path)
	s.Set(KeyFontSize, "14")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("config mode = %o, want 0600", info.Mode().Perm())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("a save should leave no temporary file, got %d entries", len(entries))
	}
}

// TestSettingsConcurrentSaveAndReload runs the watcher's read, a save and the
// UI's writes at once; run with -race to catch a data race on the file or the
// document. ioMu must serialize the file so a reload never reads a
// half-written config.
func TestSettingsConcurrentSaveAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	s := LoadSettings(path)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		i := i
		wg.Add(3)
		go func() {
			defer wg.Done()
			s.Set(KeyFontSize, strconv.Itoa(i))
		}()
		go func() {
			defer wg.Done()
			_ = s.Save()
		}()
		go func() {
			defer wg.Done()
			s.ReloadIfChanged()
		}()
	}
	wg.Wait()
	if got := s.String(KeyFontSize, ""); got == "" {
		t.Fatal("the document should still hold a font-size")
	}
}

func TestUnsupportedKeysReportsUnimplementedOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	text := KeyNotifyOnFinish + " = always\n" + KeyAutoSecureInput + " = true\n" + KeyFontSize + " = 13\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	unsupported := LoadSettings(path).UnsupportedKeys()
	want := map[string]bool{KeyNotifyOnFinish: true, KeyAutoSecureInput: true}
	for _, key := range unsupported {
		if !want[key] {
			t.Errorf("unexpected unsupported key %q", key)
		}
		delete(want, key)
	}
	for key := range want {
		t.Errorf("missing unsupported key %q", key)
	}
}
