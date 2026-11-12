//go:build linux

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// childPIDs returns the PIDs of this process's direct children. The kernel
// lists a child under the task that forked it, and Go runs exec on whichever
// thread its goroutine is on, so every task's children are read and joined.
func childPIDs(pid int) []int {
	paths, _ := filepath.Glob(fmt.Sprintf("/proc/%d/task/*/children", pid))
	seen := map[int]bool{}
	var out []int
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, field := range strings.Fields(string(data)) {
			n, err := strconv.Atoi(field)
			if err != nil || seen[n] {
				continue
			}
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// commandRunning reports whether a command is running in the terminal whose
// shell has this PID, from the process group and the terminal's foreground
// process group in /proc/<pid>/stat. It reports true when the process cannot
// be read, so a shell Code cannot inspect is confirmed like one running a
// command, as Ghostty confirms when shell integration cannot tell.
func commandRunning(pid int) bool {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return true
	}
	pgrp, tpgid, ok := statProcessGroups(string(data))
	if !ok {
		return true
	}
	return foregroundGroupRunning(pgrp, tpgid)
}

// statProcessGroups reads the process group and the foreground process group
// from the contents of /proc/<pid>/stat. The command name may hold spaces and
// parentheses, so only the fields after its last closing one are read.
func statProcessGroups(stat string) (pgrp, tpgid int, ok bool) {
	end := strings.LastIndexByte(stat, ')')
	if end < 0 {
		return 0, 0, false
	}
	fields := strings.Fields(stat[end+1:])
	if len(fields) < 6 {
		return 0, 0, false
	}
	pgrp, err := strconv.Atoi(fields[2])
	if err != nil {
		return 0, 0, false
	}
	tpgid, err = strconv.Atoi(fields[5])
	if err != nil {
		return 0, 0, false
	}
	return pgrp, tpgid, true
}
