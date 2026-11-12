package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
)

// Code builds its own libghostty-vt with scripts/build-libghostty.sh. MyGo
// pins a different Ghostty commit and ships that library in every app it
// packages; Code must never load or ship it.
//
//go:embed ghostty.version
var pinnedGhosttyVersion string

const (
	localLibPrefix   = "libghostty-vt-local-"
	localVersionFile = "libghostty-vt-local.version"
	// CODE_ALLOW_MYGO_GHOSTTY lets a developer run Code without building the
	// library. Packaged apps never set it.
	allowMygoEnv = "CODE_ALLOW_MYGO_GHOSTTY"
)

// mygoLibNames are the file names of the library MyGo pins and bundles.
var mygoLibNames = map[string]bool{
	"libghostty-vt.dylib": true,
	"libghostty-vt.so":    true,
	"ghostty-vt.dll":      true,
}

// localLibghosttyName is the name scripts/build-libghostty.sh gives the
// library for this platform.
func localLibghosttyName() string {
	switch runtime.GOOS {
	case "darwin":
		return localLibPrefix + "darwin-" + runtime.GOARCH + ".dylib"
	case "windows":
		return localLibPrefix + "windows-" + runtime.GOARCH + ".dll"
	default:
		return localLibPrefix + "linux-" + runtime.GOARCH + ".so"
	}
}

// libghosttyDirs lists where Code's library may be. Packaged apps install
// resources at the top of the resource directory; under `go run` the build
// script leaves them in resources/<goos>-<goarch>/.
func libghosttyDirs() []string {
	var dirs []string
	if dir, err := mygo.App.Path(mygo.PathResources); err == nil {
		dirs = append(dirs, dir)
	}
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	return dirs
}

// findLocalLibghostty looks only for Code's own library. It never considers
// the libghostty-vt.* files MyGo puts beside the executable.
func findLocalLibghostty(dirs []string) (string, error) {
	name := localLibghosttyName()
	platform := runtime.GOOS + "-" + runtime.GOARCH
	for _, dir := range dirs {
		for _, path := range []string{filepath.Join(dir, name), filepath.Join(dir, platform, name)} {
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			return path, nil
		}
	}
	return "", fmt.Errorf("Code's %s is missing; build it with ./scripts/build-libghostty.sh (or set MYGO_GHOSTTY_VT to that file)", name)
}

// selectLibghostty points the terminal plugin at Code's library and returns
// it. An empty path means the developer explicitly allowed MyGo's library.
func selectLibghostty() (string, error) {
	return selectFromDirs(libghosttyDirs())
}

// selectFromDirs is selectLibghostty with the resource directories supplied,
// so the missing-library paths can be tested without an app.
func selectFromDirs(dirs []string) (string, error) {
	if override := os.Getenv("MYGO_GHOSTTY_VT"); override != "" {
		name := filepath.Base(override)
		if mygoLibNames[name] && !allowMygo() {
			return "", fmt.Errorf("MYGO_GHOSTTY_VT names %q, which is MyGo's libghostty-vt; point it at the library built by scripts/build-libghostty.sh (or set %s=1)", name, allowMygoEnv)
		}
		info, err := os.Stat(override)
		if err != nil || !info.Mode().IsRegular() {
			return "", fmt.Errorf("MYGO_GHOSTTY_VT: %q is not a regular file", override)
		}
		log.Printf("codeterm: MYGO_GHOSTTY_VT overrides Code's libghostty-vt: %s", override)
		return override, nil
	}

	path, err := findLocalLibghostty(dirs)
	if err != nil {
		if allowMygo() {
			log.Printf("codeterm: %v; continuing with MyGo's library because %s is set", err, allowMygoEnv)
			return "", nil
		}
		return "", err
	}
	if err := os.Setenv("MYGO_GHOSTTY_VT", path); err != nil {
		return "", fmt.Errorf("selecting Code's libghostty-vt: %w", err)
	}
	warnIfVersionDiffers(dirs, path)
	return path, nil
}

func allowMygo() bool { return os.Getenv(allowMygoEnv) != "" }

// warnIfVersionDiffers reports when the library on disk was built from a
// Ghostty ref other than the one ghostty.version pins. A developer may build
// another ref on purpose, so this warns instead of refusing to start.
func warnIfVersionDiffers(dirs []string, lib string) {
	want := strings.TrimSpace(pinnedGhosttyVersion)
	places := append([]string{filepath.Dir(lib), filepath.Dir(filepath.Dir(lib))}, dirs...)
	for _, dir := range places {
		data, err := os.ReadFile(filepath.Join(dir, localVersionFile))
		if err != nil {
			continue
		}
		if got := strings.TrimSpace(string(data)); got != want {
			log.Printf("codeterm: warning: %s says %q but ghostty.version pins %q; rebuild with ./scripts/build-libghostty.sh %s", localVersionFile, got, want, want)
		}
		return
	}
	log.Printf("codeterm: warning: %s is missing; cannot confirm the Ghostty revision of %s", localVersionFile, lib)
}

// verifyLibghostty loads the library and asks the plugin which one it
// resolved, so a future MyGo that ignores MYGO_GHOSTTY_VT cannot quietly put
// its older library back.
func verifyLibghostty(chosen string) (string, error) {
	if err := terminal.Load(); err != nil {
		return "", fmt.Errorf("could not load Code's libghostty-vt %s: %w", chosen, err)
	}
	loaded, err := terminal.LibraryPath()
	if err != nil {
		return "", fmt.Errorf("could not resolve libghostty-vt: %w", err)
	}
	if filepath.Clean(loaded) != filepath.Clean(chosen) {
		return "", fmt.Errorf("the terminal resolved %q instead of Code's %q", loaded, chosen)
	}
	if mygoLibNames[filepath.Base(loaded)] {
		return "", fmt.Errorf("the terminal resolved MyGo's %q", loaded)
	}
	return loaded, nil
}
