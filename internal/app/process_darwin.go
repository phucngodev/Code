//go:build darwin

package app

import (
	"bytes"
	"encoding/binary"
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
	sysctlFn       = symbol("sysctl")
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

// proc_vnodepathinfo, read with proc_pidinfo's PROC_PIDVNODEPATHINFO flavor,
// holds the process's working directory in pvi_cdir.vip_path. The size and the
// path's offset are from <sys/proc_info.h>.
const (
	procPIDVnodePathInfo = 9
	vnodePathInfoSize    = 2352
	vnodePathOffset      = 152
)

// processDir returns the working directory of a process, or false when it
// cannot be read.
func processDir(pid int) (string, bool) {
	if procPIDInfoFn == 0 {
		return "", false
	}
	buf := make([]byte, vnodePathInfoSize)
	n, _, _ := purego.SyscallN(procPIDInfoFn, uintptr(pid), procPIDVnodePathInfo, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if int32(n) != vnodePathInfoSize {
		return "", false
	}
	field := buf[vnodePathOffset:]
	end := bytes.IndexByte(field, 0)
	if end <= 0 {
		return "", false
	}
	return validProcessDir(string(field[:end]))
}

// foregroundPID returns the PID of the process group the terminal whose shell
// has this PID holds in the foreground when it is a command rather than the
// shell's own group, or 0.
func foregroundPID(shellPID int) int {
	info, ok := pidInfo(shellPID)
	if !ok || info.TPGID == 0 || info.TPGID == info.PGID {
		return 0
	}
	return int(info.TPGID)
}

// processArgs returns the argument list of a process, or nil. It reads
// KERN_PROCARGS2, whose environment part an app may not see.
func processArgs(pid int) []string {
	if sysctlFn == 0 {
		return nil
	}
	mib := [3]int32{1, 49, int32(pid)} // CTL_KERN, KERN_PROCARGS2, pid
	size := uintptr(0)
	if r, _, _ := purego.SyscallN(sysctlFn, uintptr(unsafe.Pointer(&mib[0])), 3, 0, uintptr(unsafe.Pointer(&size)), 0, 0); int64(r) == -1 || size == 0 || size > 1<<20 {
		return nil
	}
	buf := make([]byte, size)
	if r, _, _ := purego.SyscallN(sysctlFn, uintptr(unsafe.Pointer(&mib[0])), 3, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0, 0); int64(r) == -1 {
		return nil
	}
	return parseProcArgs(buf[:size])
}

// parseProcArgs reads the argument list from a KERN_PROCARGS2 buffer: an int
// count of arguments, the executable path, then that many NUL-terminated
// strings.
func parseProcArgs(buf []byte) []string {
	if len(buf) < 4 {
		return nil
	}
	argc := int(int32(binary.LittleEndian.Uint32(buf[:4])))
	if argc <= 0 || argc > len(buf) {
		return nil
	}
	p := 4
	for p < len(buf) && buf[p] != 0 {
		p++
	}
	for p < len(buf) && buf[p] == 0 {
		p++
	}
	var args []string
	for i := 0; i < argc && p < len(buf); i++ {
		start := p
		for p < len(buf) && buf[p] != 0 {
			p++
		}
		args = append(args, string(buf[start:p]))
		p++
	}
	return args
}
