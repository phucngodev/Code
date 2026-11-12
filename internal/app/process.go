package app

import (
	"os"
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

// foregroundGroupRunning reports whether a terminal with process group pgrp
// has a different group in the foreground: a group of its own means the shell
// sits at a prompt, and another means a command is running.
func foregroundGroupRunning(pgrp, tpgid int) bool {
	return tpgid > 0 && tpgid != pgrp
}
