package app

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/phucngodev/code/internal/agent"
)

// shellPID returns the PID of the shell running in this surface. The app
// records it when the terminal's program starts, as the child of this process
// that appears then.
func (s *Surface) shellPID() (int, bool) {
	if s.owner == nil || s.owner.app == nil {
		return 0, false
	}
	return s.owner.app.shellPIDFor(s)
}

// shellPIDFor reads the shell PID recorded for a surface.
func (a *App) shellPIDFor(s *Surface) (int, bool) {
	a.shellMu.Lock()
	defer a.shellMu.Unlock()
	if s.shellPid > 0 {
		return s.shellPid, true
	}
	return 0, false
}

// watchShell records the shell of a surface waiting for one. The terminal's
// program starts after this, as a new child of this process, so the watcher
// matches each new child to the surface that started next. Reading another
// process's environment to identify the shell is not done: macOS and sandboxes
// hide it.
func (a *App) watchShell(s *Surface) {
	a.shellMu.Lock()
	// A fresh watcher learns the children there are now, before this
	// surface's shell starts; one already running keeps its set, which it
	// updates each pass, so a shell it has not assigned yet is not taken for
	// an existing child.
	if !a.shellWatching {
		a.shellKnown = make(map[int]bool)
		for _, pid := range childPIDs(os.Getpid()) {
			a.shellKnown[pid] = true
		}
	}
	a.shellQueue = append(a.shellQueue, s)
	start := !a.shellWatching
	a.shellWatching = true
	a.shellMu.Unlock()
	if start {
		go a.assignShells()
	}
}

// assignShells matches new child processes to queued surfaces, in the order
// they appeared and started, until every surface has a shell or they time out.
func (a *App) assignShells() {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		children := childPIDs(os.Getpid())
		a.shellMu.Lock()
		var fresh []int
		for _, pid := range children {
			if a.shellKnown[pid] || isHelperProcess(pid) {
				continue
			}
			fresh = append(fresh, pid)
		}
		if len(fresh) > 0 && len(a.shellQueue) > 0 {
			// PIDs rise with creation order, which follows the order the
			// surfaces started, so the queue is matched in that order.
			sort.Ints(fresh)
			a.shellQueue = matchShells(a.shellQueue, fresh)
		}
		known := make(map[int]bool, len(children))
		for _, pid := range children {
			known[pid] = true
		}
		a.shellKnown = known
		done := len(a.shellQueue) == 0
		a.shellWatching = !done
		a.shellMu.Unlock()
		if done {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	a.shellMu.Lock()
	a.shellWatching = false
	// A surface whose shell never appeared must not stay queued, or a later
	// pane would be given the wrong shell.
	a.shellQueue = nil
	a.shellMu.Unlock()
}

// matchShells pairs new children with the surfaces waiting for them. A distinct
// working directory identifies the pane; otherwise the order they started is
// the best Code can do, since the terminal plugin does not expose its child PID.
func matchShells(queue []*Surface, pids []int) []*Surface {
	dirs := map[int]string{}
	for _, pid := range pids {
		if dir, ok := processDir(pid); ok {
			dirs[pid] = dir
		}
	}
	used := map[int]bool{}
	var waiting []*Surface
	for _, s := range queue {
		if s.closed {
			continue
		}
		pick := -1
		if s.dir != "" {
			var matches []int
			for _, pid := range pids {
				if !used[pid] && dirs[pid] == s.dir {
					matches = append(matches, pid)
				}
			}
			if len(matches) == 1 {
				pick = matches[0]
			}
		}
		if pick < 0 {
			for _, pid := range pids {
				if !used[pid] {
					pick = pid
					break
				}
			}
		}
		if pick < 0 {
			waiting = append(waiting, s)
			continue
		}
		used[pick] = true
		s.shellPid = pick
	}
	return waiting
}

// isHelperProcess reports a direct child that is Code acting as the cde helper
// rather than a terminal's shell.
func isHelperProcess(pid int) bool {
	args := processArgs(pid)
	if len(args) < 2 {
		return false
	}
	switch filepath.Base(args[0]) {
	case "cde", "Code", "code":
		return agent.IsCdeCommand(args[1:])
	}
	return false
}

// sshRemoteFor returns the host an ssh running as pid connects to, or empty
// when it is not an ssh. The foreground process group is read by the caller,
// which caches it so the arguments are not read every half second.
func sshRemoteFor(pid int) string {
	if pid <= 0 {
		return ""
	}
	args := processArgs(pid)
	if len(args) < 2 || filepath.Base(args[0]) != "ssh" {
		return ""
	}
	return sshDestination(args[1:])
}

// sshValueOptions are ssh options that take the value after them, so the
// destination is not mistaken for one.
var sshValueOptions = map[string]bool{
	"-b": true, "-c": true, "-D": true, "-E": true, "-e": true, "-F": true,
	"-I": true, "-i": true, "-J": true, "-L": true, "-l": true, "-m": true,
	"-O": true, "-o": true, "-p": true, "-Q": true, "-R": true, "-S": true,
	"-W": true, "-w": true,
}

// sshDestination reads the host ssh connects to, [user@]host, from its
// arguments, skipping options and their values.
func sshDestination(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				return sshHost(args[i+1])
			}
			return ""
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			if sshValueOptions[arg] {
				i++
			}
			continue
		}
		return sshHost(arg)
	}
	return ""
}

// sshHost reads the host from a destination, dropping a user and a port.
func sshHost(arg string) string {
	if at := strings.LastIndexByte(arg, '@'); at >= 0 {
		arg = arg[at+1:]
	}
	if colon := strings.LastIndexByte(arg, ':'); colon >= 0 {
		arg = arg[:colon]
	}
	return arg
}

// validProcessDir keeps a process's reported cwd only when it is a real
// absolute directory. A hard-coded ABI offset on macOS can otherwise hand a
// pane garbage that ends up in its tab title and saved session.
func validProcessDir(path string) (string, bool) {
	path = strings.TrimSuffix(path, " (deleted)")
	if path == "" || !filepath.IsAbs(path) {
		return "", false
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return "", false
	}
	return path, true
}

// shellNames are the login shells a pane at a prompt runs. A tracked process
// whose name is not one of these is a program the pane runs itself, so it is
// busy while it runs rather than sitting at a prompt.
var shellNames = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "fish": true, "dash": true,
	"ash": true, "ksh": true, "mksh": true, "csh": true, "tcsh": true,
	"nu": true, "pwsh": true, "elvish": true, "xonsh": true, "oil": true,
	"osh": true, "yash": true,
}

// runsShell reports whether the process is a shell, whose foreground group
// tells whether it sits at a prompt. A name that cannot be read is not a
// shell, so the pane is confirmed rather than closed without asking.
func runsShell(pid int) bool {
	args := processArgs(pid)
	if len(args) == 0 {
		return false
	}
	name := strings.TrimPrefix(filepath.Base(args[0]), "-")
	return shellNames[name] || name == strings.TrimPrefix(filepath.Base(loginShell()), "-")
}
