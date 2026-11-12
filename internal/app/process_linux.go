//go:build linux

package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// childPIDs returns the PIDs of this process's direct children, as the
// kernel lists them for each task.
func childPIDs(pid int) []int {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%d/children", pid, pid))
	if err != nil {
		return nil
	}
	fields := strings.Fields(string(data))
	out := make([]int, 0, len(fields))
	for _, field := range fields {
		if n, err := strconv.Atoi(field); err == nil {
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
