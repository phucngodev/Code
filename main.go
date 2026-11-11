// Command code is Code, a terminal built on libghostty, in Go with MyGo's
// native UI, on macOS, Linux and Windows.
package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"

	"github.com/phucngodev/code/internal/agent"
	"github.com/phucngodev/code/internal/app"
)

func main() {
	// The app also acts as cde, the helper agents call back through, so a
	// single binary serves both.
	if agent.IsCdeCommand(os.Args[1:]) {
		os.Exit(agent.RunCde(os.Args[1:], os.Environ(), os.Stdin, os.Stdout, os.Stderr))
	}
	disableFontSmoothing()
	useLocalLibghostty()
	_ = app.New()
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// keepSystemFontSmoothing reports whether the user asked Code to keep the
// system's font smoothing, with CODE_FONT_SMOOTHING.
func keepSystemFontSmoothing() bool {
	return os.Getenv("CODE_FONT_SMOOTHING") != ""
}

// useLocalLibghostty points MyGo's terminal plugin at the libghostty-vt that
// scripts/build-libghostty.sh built into the app's resources, so Code uses
// that revision instead of the one MyGo pins. An explicit MYGO_GHOSTTY_VT wins.
func useLocalLibghostty() {
	if os.Getenv("MYGO_GHOSTTY_VT") != "" {
		return
	}
	name := localLibghosttyName()
	platform := runtime.GOOS + "-" + runtime.GOARCH
	var candidates []string
	if dir, err := mygo.App.Path(mygo.PathResources); err == nil {
		candidates = append(candidates, filepath.Join(dir, name), filepath.Join(dir, platform, name))
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), name))
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			os.Setenv("MYGO_GHOSTTY_VT", path)
			return
		}
	}
}

// localLibghosttyName is the name the build script gives the library for this
// platform.
func localLibghosttyName() string {
	switch runtime.GOOS {
	case "darwin":
		return "libghostty-vt-local-darwin-" + runtime.GOARCH + ".dylib"
	case "windows":
		return "libghostty-vt-local-windows-" + runtime.GOARCH + ".dll"
	default:
		return "libghostty-vt-local-linux-" + runtime.GOARCH + ".so"
	}
}
