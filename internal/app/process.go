package app

import (
	"os"
	"path/filepath"
	"strings"
	"time"
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
		for _, pid := range children {
			if a.shellKnown[pid] {
				continue
			}
			if len(a.shellQueue) == 0 {
				break
			}
			s := a.shellQueue[0]
			a.shellQueue = a.shellQueue[1:]
			s.shellPid = pid
		}
		known := make(map[int]bool, len(children))
		for _, pid := range children {
			known[pid] = true
		}
		a.shellKnown = known
		done := len(a.shellQueue) == 0
		a.shellMu.Unlock()
		if done {
			a.shellMu.Lock()
			a.shellWatching = false
			a.shellMu.Unlock()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	a.shellMu.Lock()
	a.shellWatching = false
	a.shellMu.Unlock()
}

// sshRemote returns the host an ssh running in the terminal's shell connects
// to, or empty when none runs.
func sshRemote(shellPID int) string {
	pid := foregroundPID(shellPID)
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

// foregroundGroupRunning reports whether a terminal with process group pgrp
// has a different group in the foreground: a group of its own means the shell
// sits at a prompt, and another means a command is running.
func foregroundGroupRunning(pgrp, tpgid int) bool {
	return tpgid > 0 && tpgid != pgrp
}
