package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"

	"github.com/phucngodev/code/internal/agent"
	"github.com/phucngodev/code/internal/config"
	"github.com/phucngodev/code/internal/model"
)

func TestShellIntegrationEnvForZsh(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("ZDOTDIR", "/home/me/zsh")
	env := shellIntegrationEnv("/assets", config.TerminalConfig{})
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "CODE_ZSH_ZDOTDIR=/home/me/zsh") {
		t.Fatalf("the user's ZDOTDIR must be preserved: %q", joined)
	}
	want := "ZDOTDIR=" + filepath.Join("/assets", "shell-integration", "zsh")
	if !strings.Contains(joined, want) {
		t.Fatalf("zsh must read Code's .zshenv: %q", joined)
	}
}

func TestShellIntegrationEnvForFish(t *testing.T) {
	t.Setenv("SHELL", "/usr/bin/fish")
	t.Setenv("XDG_DATA_DIRS", "/usr/share")
	env := shellIntegrationEnv("/assets", config.TerminalConfig{})
	want := "XDG_DATA_DIRS=" + filepath.Join("/assets", "shell-integration") + string(os.PathListSeparator) + "/usr/share"
	found := false
	for _, entry := range env {
		if entry == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("fish must find Code's vendor_conf.d, got %v", env)
	}
}

func TestShellIntegrationEnvSkipsFixedCommand(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	if env := shellIntegrationEnv("/assets", config.TerminalConfig{Command: "/usr/bin/true"}); len(env) != 0 {
		t.Fatalf("a fixed program needs no shell integration, got %v", env)
	}
}

func TestFrameOnScreen(t *testing.T) {
	areas := []mygo.Rectangle{{X: 0, Y: 0, Width: 1000, Height: 800}}
	cases := []struct {
		frame *model.Frame
		want  bool
	}{
		{&model.Frame{X: 0, Y: 0, Width: 400, Height: 300}, true},
		{&model.Frame{X: 900, Y: 700, Width: 400, Height: 300}, true},
		{&model.Frame{X: 2000, Y: 0, Width: 400, Height: 300}, false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := frameOnScreen(tc.frame, areas); got != tc.want {
			t.Errorf("frameOnScreen(%+v) = %v, want %v", tc.frame, got, tc.want)
		}
	}
}

// TestAgentEnvKeepsShimsInTheAppRoot guards against a test or a launch with a
// missing shim root writing shim scripts into the working directory.
func TestAgentEnvKeepsShimsInTheAppRoot(t *testing.T) {
	a := testApp(t)
	w := &WindowState{app: a, store: model.NewStore[*Surface]()}
	s := w.newSurface("pane-1", "", "ws")
	env := a.agentEnv(s)
	shimDir := agent.ShimsDir(a.shimsRoot, s.id)
	if info, err := os.Stat(shimDir); err != nil || !info.IsDir() {
		t.Fatalf("shims should be installed in %s: %v", shimDir, err)
	}
	if _, err := os.Stat(s.id); err == nil {
		t.Fatalf("shims must not be written into the working directory as %q", s.id)
	}
	found := false
	for _, entry := range env {
		if strings.HasPrefix(entry, "PATH="+shimDir) {
			found = true
		}
	}
	if !found {
		t.Fatalf("PATH should start with the shim directory, got %v", env)
	}
}

func TestAgentEnvWithoutHelperLeavesPathAlone(t *testing.T) {
	a := testApp(t)
	a.cdePath = ""
	w := &WindowState{app: a, store: model.NewStore[*Surface]()}
	s := w.newSurface("pane-2", "", "ws")
	for _, entry := range a.agentEnv(s) {
		if strings.HasPrefix(entry, "PATH=") {
			t.Fatalf("without a helper binary PATH must not be overridden: %v", entry)
		}
	}
}

func TestAgentEnvAdvertisesTheTerminal(t *testing.T) {
	a := testApp(t)
	w := &WindowState{app: a, store: model.NewStore[*Surface]()}
	s := w.newSurface("pane-1", "", "ws")
	GhosttyVersion = "1.4.0"
	defer func() { GhosttyVersion = "" }()
	got := map[string]bool{}
	for _, entry := range a.agentEnv(s) {
		got[entry] = true
	}
	// Programs like Pi and PiG key image support on TERM_PROGRAM, not a query.
	if !got["TERM_PROGRAM=ghostty"] {
		t.Fatal("the terminal should advertise TERM_PROGRAM=ghostty")
	}
	if !got["TERM_PROGRAM_VERSION=1.4.0"] {
		t.Fatal("the terminal should advertise its Ghostty revision")
	}
}

func TestMatchShellsDropsClosedSurfaces(t *testing.T) {
	live := &Surface{id: "live", dir: "/somewhere"}
	closed := &Surface{id: "closed", dir: "/somewhere", closed: true}
	waiting := matchShells([]*Surface{closed, live}, nil)
	if len(waiting) != 1 || waiting[0] != live {
		t.Fatalf("waiting = %v, want only the live surface", waiting)
	}
	if closed.shellPid != 0 {
		t.Fatal("a closed surface must not be given a shell")
	}
}

// TestMatchShellsFallsBackToOrder pairs surfaces with no working directory to
// the new children in the order they came, when the directory cannot tell them
// apart.
func TestMatchShellsFallsBackToOrder(t *testing.T) {
	a := &Surface{id: "a"}
	b := &Surface{id: "b"}
	waiting := matchShells([]*Surface{a, b}, []int{101, 102})
	if a.shellPid != 101 || b.shellPid != 102 {
		t.Fatalf("a=%d b=%d, want 101 and 102", a.shellPid, b.shellPid)
	}
	if len(waiting) != 0 {
		t.Fatalf("waiting = %v, want none", waiting)
	}
}
