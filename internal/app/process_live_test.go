package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/agent"
	"github.com/phucngodev/code/internal/config"
	"github.com/phucngodev/code/internal/model"
)

// waitUntil polls until cond holds or the timeout passes.
func waitUntil(cond func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}

// liveShellWindow starts a window with one terminal running /bin/sh and
// returns the window and its surface.
func liveShellWindow(t *testing.T) (*WindowState, *Surface) {
	t.Helper()
	if err := terminal.Load(); err != nil {
		t.Skipf("libghostty not available: %v", err)
	}
	path := t.TempDir() + "/config"
	if err := os.WriteFile(path, []byte("command = /bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	settings := config.LoadSettings(path)
	a := &App{
		settings:  settings,
		agents:    agent.NewStore(),
		writer:    newSessionWriter(t.TempDir() + "/session.json"),
		shimsRoot: t.TempDir(),
		cdePath:   "/usr/bin/true",
	}
	a.reloadKeybinds()
	a.terminalCfg = settings.Terminal("")
	w := &WindowState{app: a, store: model.NewStore[*Surface](), sidebarHidden: true}
	w.newWorkspace("")
	s := w.focusedSurface()
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	if s.term == nil {
		t.Skipf("terminal did not start: %v", s.startErr)
	}
	return w, s
}

// shellOf waits for the shell PID the app recorded for a surface.
func shellOf(t *testing.T, s *Surface) int {
	t.Helper()
	var pid int
	if !waitUntil(func() bool {
		p, ok := s.shellPID()
		if ok {
			pid = p
		}
		return ok
	}, 5*time.Second) {
		t.Fatal("the shell process was not recorded for the surface")
	}
	return pid
}

// TestForegroundCommandDetection checks that a shell sitting at a prompt does
// not read as running a command, and one running a command does, from the
// process groups of the shell the app recorded.
func TestForegroundCommandDetection(t *testing.T) {
	_, s := liveShellWindow(t)
	defer s.close()
	shell := shellOf(t, s)

	if !waitUntil(func() bool { return !commandRunning(shell) }, 5*time.Second) {
		t.Fatal("an idle shell should not read as running a command")
	}
	// Wait for the shell's first prompt before typing into it.
	if !waitUntil(func() bool { return strings.TrimSpace(s.term.Text()) != "" }, 5*time.Second) {
		t.Fatal("the shell produced no prompt")
	}
	s.term.Send([]byte("sleep 30\n"))
	if !waitUntil(func() bool { return commandRunning(shell) }, 5*time.Second) {
		t.Fatal("a command should read as running")
	}
}

// TestRunningFollowsForegroundProcess checks Surface.running follows the
// foreground process of the shell the app recorded.
func TestRunningFollowsForegroundProcess(t *testing.T) {
	_, s := liveShellWindow(t)
	defer s.close()
	shellOf(t, s)

	if !waitUntil(func() bool { return !s.running() }, 5*time.Second) {
		t.Fatal("an idle shell should not count as running a command")
	}
	s.term.Send([]byte("sleep 30\n"))
	if !waitUntil(func() bool { return s.running() }, 5*time.Second) {
		t.Fatal("a command should count as running")
	}
}

// TestProcessDirFollowsCd checks that the shell's working directory is read
// from the OS, since a shell may report it only once.
func TestProcessDirFollowsCd(t *testing.T) {
	_, s := liveShellWindow(t)
	defer s.close()
	shell := shellOf(t, s)

	if !waitUntil(func() bool { dir, ok := processDir(shell); return ok && dir != "" }, 5*time.Second) {
		t.Fatal("the shell's working directory was not read")
	}
	if !waitUntil(func() bool { return strings.TrimSpace(s.term.Text()) != "" }, 5*time.Second) {
		t.Fatal("the shell produced no prompt")
	}
	// The kernel reports the resolved path, so compare with the resolved one.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.term.Send([]byte("cd '" + dir + "'\n"))
	if !waitUntil(func() bool { got, _ := processDir(shell); return got == dir }, 5*time.Second) {
		t.Fatal("the shell's working directory did not follow cd")
	}
}

// TestForegroundCommandArgs checks that the running command's arguments are
// read from the OS, which is how an ssh's host is found.
func TestForegroundCommandArgs(t *testing.T) {
	_, s := liveShellWindow(t)
	defer s.close()
	shell := shellOf(t, s)

	if !waitUntil(func() bool { return strings.TrimSpace(s.term.Text()) != "" }, 5*time.Second) {
		t.Fatal("the shell produced no prompt")
	}
	s.term.Send([]byte("sleep 30\n"))
	if !waitUntil(func() bool {
		pid := foregroundPID(shell)
		if pid <= 0 {
			return false
		}
		for _, arg := range processArgs(pid) {
			if arg == "sleep" {
				return true
			}
		}
		return false
	}, 5*time.Second) {
		t.Fatal("the foreground command's arguments were not read")
	}
}

// TestConfirmCloseAsksForARunningCommand checks the confirm predicate itself:
// an idle shell does not ask, a command does.
func TestConfirmCloseAsksForARunningCommand(t *testing.T) {
	w, s := liveShellWindow(t)
	defer s.close()
	shellOf(t, s)

	if !waitUntil(func() bool { return !w.confirmClose([]*Surface{s}) }, 5*time.Second) {
		t.Fatal("an idle shell should not ask to confirm")
	}
	s.term.Send([]byte("sleep 30\n"))
	if !waitUntil(func() bool { return w.confirmClose([]*Surface{s}) }, 5*time.Second) {
		t.Fatal("a running command should ask to confirm")
	}
}

// TestClosingTabWithRunningCommandConfirms checks that closing the tab holding
// a running command asks to confirm rather than dropping it.
func TestClosingTabWithRunningCommandConfirms(t *testing.T) {
	w, s := liveShellWindow(t)
	defer s.close()
	shellOf(t, s)
	s.term.Send([]byte("sleep 30\n"))
	if !waitUntil(func() bool { return s.running() }, 5*time.Second) {
		t.Fatal("a command should count as running")
	}

	w.closeTab(w.store.Selected().SelectedTab())
	if w.pending == nil {
		t.Fatal("closing a tab with a running command should ask to confirm")
	}
}
