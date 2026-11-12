package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("library"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestFindLocalLibghosttyIgnoresMygoLibrary checks that the resolver cannot
// pick up the library MyGo bundles, even when it sits beside Code's.
func TestFindLocalLibghosttyIgnoresMygoLibrary(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "libghostty-vt.so"))
	writeFile(t, filepath.Join(dir, "libghostty-vt.dylib"))
	want := writeFile(t, filepath.Join(dir, localLibghosttyName()))
	got, err := findLocalLibghostty([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("findLocalLibghostty = %q, want %q", got, want)
	}
}

func TestFindLocalLibghosttyInPlatformDir(t *testing.T) {
	dir := t.TempDir()
	want := writeFile(t, filepath.Join(dir, runtime.GOOS+"-"+runtime.GOARCH, localLibghosttyName()))
	got, err := findLocalLibghostty([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("findLocalLibghostty = %q, want %q", got, want)
	}
}

func TestFindLocalLibghosttyRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, filepath.Join(dir, "libghostty-vt.so"))
	link := filepath.Join(dir, localLibghosttyName())
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := findLocalLibghostty([]string{dir}); err == nil {
		t.Fatal("a symlink to MyGo's library must not be accepted as Code's")
	}
}

func TestSelectFromDirsFailsWithoutLocalLibrary(t *testing.T) {
	t.Setenv("MYGO_GHOSTTY_VT", "")
	t.Setenv(allowMygoEnv, "")
	if _, err := selectFromDirs([]string{t.TempDir()}); err == nil {
		t.Fatal("starting without Code's library must fail")
	} else if !strings.Contains(err.Error(), "build-libghostty.sh") {
		t.Fatalf("error should tell the user how to build the library: %v", err)
	}
}

func TestSelectFromDirsAllowsOptInFallback(t *testing.T) {
	t.Setenv("MYGO_GHOSTTY_VT", "")
	t.Setenv(allowMygoEnv, "1")
	got, err := selectFromDirs([]string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("opt-in fallback should return no path, got %q", got)
	}
}

func TestSelectFromDirsRejectsMygoOverride(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "libghostty-vt.so"))
	t.Setenv("MYGO_GHOSTTY_VT", filepath.Join(dir, "libghostty-vt.so"))
	t.Setenv(allowMygoEnv, "")
	if _, err := selectFromDirs([]string{dir}); err == nil {
		t.Fatal("an override naming MyGo's library must be rejected")
	}
}

func TestSelectFromDirsUsesLocalLibrary(t *testing.T) {
	dir := t.TempDir()
	want := writeFile(t, filepath.Join(dir, localLibghosttyName()))
	t.Setenv("MYGO_GHOSTTY_VT", "")
	t.Setenv(allowMygoEnv, "")
	got, err := selectFromDirs([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("selectFromDirs = %q, want %q", got, want)
	}
	if os.Getenv("MYGO_GHOSTTY_VT") != want {
		t.Fatal("the terminal plugin must be pointed at Code's library")
	}
}

func TestPinnedGhosttyVersion(t *testing.T) {
	onDisk, err := os.ReadFile("ghostty.version")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(pinnedGhosttyVersion) != strings.TrimSpace(string(onDisk)) {
		t.Fatalf("embedded ghostty.version = %q, file = %q", pinnedGhosttyVersion, onDisk)
	}
	if strings.TrimSpace(pinnedGhosttyVersion) == "" {
		t.Fatal("ghostty.version is empty")
	}
}
