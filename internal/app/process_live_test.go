package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

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
	return liveCommandWindow(t, "/bin/sh")
}

// liveCommandWindow starts a window whose terminal runs command directly,
// rather than a shell, and returns the window and its surface.
func liveCommandWindow(t *testing.T, command string) (*WindowState, *Surface) {
	t.Helper()
	if err := terminal.Load(); err != nil {
		t.Skipf("libghostty not available: %v", err)
	}
	a := testApp(t)
	if err := os.WriteFile(a.settings.Path(), []byte("command = "+command+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.settings.Reload()
	a.reloadKeybinds()
	a.terminalCfg = a.settings.Terminal("")
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

// waitPrompt waits for the shell to print its first prompt.
func waitPrompt(t *testing.T, s *Surface) {
	t.Helper()
	if !waitUntil(func() bool { return strings.TrimSpace(s.term.Text()) != "" }, 5*time.Second) {
		t.Fatal("the shell produced no prompt")
	}
}

// waitBusy waits until the surface reads busy, or not, as want.
func waitBusy(t *testing.T, s *Surface, want bool) {
	t.Helper()
	if !waitUntil(func() bool { return s.busy() == want }, 5*time.Second) {
		t.Fatalf("surface busy = %v, want %v", !want, want)
	}
}

// startCommand waits for the shell's first prompt, then runs a command that
// keeps running, so the surface reads busy.
func startCommand(t *testing.T, s *Surface) {
	t.Helper()
	waitPrompt(t, s)
	s.term.Send([]byte("sleep 30\n"))
	waitBusy(t, s, true)
}

// TestBusyFollowsForegroundProcess checks Surface.busy follows the shell's
// foreground process: idle at a prompt, busy while a command runs.
func TestBusyFollowsForegroundProcess(t *testing.T) {
	_, s := liveShellWindow(t)
	defer s.close()
	shellOf(t, s)

	waitBusy(t, s, false)
	startCommand(t, s)
}

// TestBusyForADirectProgram checks a pane running its own program, not a
// shell, reads as busy: the program is its own foreground process group, so
// the foreground-group check alone would call it idle and close it without
// asking.
func TestBusyForADirectProgram(t *testing.T) {
	_, s := liveCommandWindow(t, "/bin/sleep 30")
	defer s.close()
	shellOf(t, s)
	waitBusy(t, s, true)
}

// TestProcessDirFollowsCd checks that the shell's working directory is read
// from the OS, since a shell may report it only once.
func TestProcessDirFollowsCd(t *testing.T) {
	_, s := liveShellWindow(t)
	defer s.close()
	shell := shellOf(t, s)
	waitPrompt(t, s)

	if !waitUntil(func() bool { dir, ok := processDir(shell); return ok && dir != "" }, 5*time.Second) {
		t.Fatal("the shell's working directory was not read")
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
	startCommand(t, s)

	if !waitUntil(func() bool {
		for _, arg := range processArgs(foregroundPID(shell)) {
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

	if w.confirmClose([]*Surface{s}) {
		t.Fatal("an idle shell should not ask to confirm")
	}
	startCommand(t, s)
	if !w.confirmClose([]*Surface{s}) {
		t.Fatal("a running command should ask to confirm")
	}
}

// TestClosingTabWithRunningCommandConfirms checks that closing the tab holding
// a running command asks to confirm rather than dropping it.
func TestClosingTabWithRunningCommandConfirms(t *testing.T) {
	w, s := liveShellWindow(t)
	defer s.close()
	shellOf(t, s)
	startCommand(t, s)

	w.closeTab(w.store.Selected().SelectedTab())
	if w.pending == nil {
		t.Fatal("closing a tab with a running command should ask to confirm")
	}
}

// TestBeforeQuitStopsForARunningCommand checks the app-level quit gate: an
// idle shell does not stop a quit, one running a command does and shows the
// alert.
func TestBeforeQuitStopsForARunningCommand(t *testing.T) {
	w, s := liveShellWindow(t)
	defer s.close()
	w.app.windows = append(w.app.windows, w)
	shellOf(t, s)

	waitBusy(t, s, false)
	e := &mygo.QuitEvent{}
	w.app.beforeQuit(e)
	if e.DefaultPrevented() {
		t.Fatal("an idle shell should not stop a quit")
	}

	startCommand(t, s)
	e = &mygo.QuitEvent{}
	w.app.beforeQuit(e)
	if !e.DefaultPrevented() {
		t.Fatal("a running command should stop a quit")
	}
	if w.pending == nil {
		t.Fatal("the quit should wait for the user's answer")
	}
	if w.pending.confirm != "Quit" {
		t.Fatalf("the quit alert's confirm button = %q, want Quit", w.pending.confirm)
	}
}
