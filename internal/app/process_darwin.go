//go:build darwin

package app

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// proc_bsdinfo, from <sys/proc_info.h>, read with proc_pidinfo's
// PROC_PIDTBSDINFO flavor.
const procPIDTBSDInfo = 3

type procBSDInfo struct {
	Flags     uint32
	Status    uint32
	XStatus   uint32
	PID       uint32
	PPID      uint32
	UID       uint32
	GID       uint32
	RUID      uint32
	RGID      uint32
	SVUID     uint32
	SVGID     uint32
	Rfu1      uint32
	Comm      [16]byte
	Name      [32]byte
	NFiles    uint32
	PGID      uint32
	PJobC     uint32
	TDev      uint32
	TPGID     uint32
	Nice      int32
	StartSec  uint64
	StartUsec uint64
}

var (
	procListPIDsFn = symbol("proc_listpids")
	procPIDInfoFn  = symbol("proc_pidinfo")
)

// symbol resolves a function the process already has loaded, as libproc's
// are, and returns 0 when it is not there.
func symbol(name string) uintptr {
	fn, err := purego.Dlsym(purego.RTLD_DEFAULT, name)
	if err != nil {
		return 0
	}
	return fn
}

// childPIDs returns the PIDs of this process's direct children, by listing
// every process and keeping those whose parent is this one.
func childPIDs(pid int) []int {
	if procListPIDsFn == 0 || procPIDInfoFn == 0 {
		return nil
	}
	// PROC_ALL_PIDS = 1.
	buf := make([]int32, 8192)
	n, _, _ := purego.SyscallN(procListPIDsFn, 1, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*4))
	count := min(int(n)/4, len(buf))
	var out []int
	for _, child := range buf[:count] {
		if child <= 0 {
			continue
		}
		if info, ok := pidInfo(int(child)); ok && int(info.PPID) == pid {
			out = append(out, int(child))
		}
	}
	return out
}

// pidInfo returns the proc_bsdinfo of a process, or false when it is gone.
func pidInfo(pid int) (procBSDInfo, bool) {
	if procPIDInfoFn == 0 {
		return procBSDInfo{}, false
	}
	var info procBSDInfo
	n, _, _ := purego.SyscallN(procPIDInfoFn, uintptr(pid), procPIDTBSDInfo, 0, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
	return info, int32(n) != 0
}

// commandRunning reports whether a command is running in the terminal whose
// shell has this PID, from the process group and the terminal's foreground
// process group of its proc_bsdinfo. It reports true when the process cannot
// be read, so a shell Code cannot inspect is confirmed like one running a
// command, as Ghostty confirms when shell integration cannot tell.
func commandRunning(pid int) bool {
	info, ok := pidInfo(pid)
	if !ok {
		return true
	}
	return foregroundGroupRunning(int(info.PGID), int(info.TPGID))
}
