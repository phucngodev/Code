// Command code is Code, a terminal built on libghostty, in Go with MyGo's
// native UI, on macOS and Linux.
package main

import (
	"log"
	"os"
	"strings"

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

	// A second launch hands its command line to the running instance and
	// exits, so one instance owns the config and session files.
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	lib, err := selectLibghostty()
	if err != nil {
		log.Fatal(err)
	}
	if lib == "" {
		log.Printf("codeterm: using MyGo's libghostty-vt (%s=1)", allowMygoEnv)
	} else {
		loaded, err := verifyLibghostty(lib)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("codeterm: using %s (Ghostty %s)", loaded, strings.TrimSpace(pinnedGhosttyVersion))
	}

	a := app.New()
	mygo.App.OnSecondInstance(func([]string, string) { a.BringToFront() })
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// keepSystemFontSmoothing reports whether the user asked Code to keep the
// system's font smoothing, with CODE_FONT_SMOOTHING.
func keepSystemFontSmoothing() bool {
	return os.Getenv("CODE_FONT_SMOOTHING") != ""
}
