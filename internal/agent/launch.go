package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Launcher is what `cde launch` runs in place of the agent the user typed: the
// real binary, found on PATH past the shim that called it, with the adapter's
// changes to its arguments or environment.
type Launcher struct {
	Executable  string
	Arguments   []string
	Environment map[string]string
}

// NewLauncher builds the launcher for an agent, or reports false when its
// binary is nowhere on PATH but in the shim directory.
func NewLauncher(adapter Adapter, arguments []string, environment map[string]string, cde string) (Launcher, bool) {
	env := make(map[string]string, len(environment))
	for key, value := range environment {
		env[key] = value
	}
	shims := env[EnvShimDir]
	delete(env, EnvShimDir)
	executable, ok := ResolveBinary(adapter.Binary, env["PATH"], shims)
	if !ok {
		return Launcher{}, false
	}
	args := append([]string{}, arguments...)
	if _, disabled := environment[EnvAgentsDisabled]; !disabled {
		switch adapter.Kind {
		case LaunchFlags:
			if adapter.Arguments != nil {
				args = adapter.Arguments(args, cde)
			}
		case LaunchEnvironment:
			if adapter.Environment != nil {
				for key, value := range adapter.Environment(env, cde) {
					env[key] = value
				}
			}
		}
	}
	return Launcher{
		Executable:  executable,
		Arguments:   append([]string{adapter.Binary}, args...),
		Environment: env,
	}, true
}

// ResolveBinary finds the first executable named binary in path, skipping the
// shim directory and the empty entries a shell reads as the current directory.
// A Code shim is skipped even when a shell profile rewrote the directory's
// path, so `cde launch` can never resolve the shim that called it.
func ResolveBinary(binary, path, excluding string) (string, bool) {
	excluded := standardize(excluding)
	for _, directory := range strings.Split(path, string(os.PathListSeparator)) {
		if directory == "" || standardize(directory) == excluded {
			continue
		}
		candidate := filepath.Join(directory, binary)
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}
		if isCodeShim(candidate) {
			continue
		}
		return candidate, true
	}
	return "", false
}

// isCodeShim reports whether a file is one of the launcher scripts Code
// writes, rather than the agent's real binary. Only the first line is read:
// the real binary can be many megabytes.
func isCodeShim(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	var head [256]byte
	n, err := file.Read(head[:])
	if err != nil && n == 0 {
		return false
	}
	return strings.HasPrefix(string(head[:n]), "#!/bin/sh\n"+EnvShimDir+"=")
}

func standardize(path string) string {
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			return resolved
		}
		return abs
	}
	return filepath.Clean(path)
}

// ShimsRoot is the directory holding every running Code's terminal shims. It
// lives in the user's own cache, not in a predictable shared directory under
// /tmp: another user must not be able to create the shims that end up first
// on this user's PATH.
func ShimsRoot(pid int) string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "code", "shims", strconv.Itoa(pid))
}

// ShimsDir is the shim directory of one terminal.
func ShimsDir(root, surfaceID string) string {
	return filepath.Join(root, surfaceID)
}

// InstallShims writes a shim for every adapter that needs one. The binary is
// not looked up here: the app's PATH differs from the login shell's, so the
// lookup happens in `cde launch`, after the shell's own profile has run.
func InstallShims(directory, cde string, adapters []Adapter) error {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	// MkdirAll leaves an existing directory as it is; refuse one another user
	// could write into, since its contents run as this user.
	info, err := os.Stat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("shim directory %s is writable by others", directory)
	}
	for _, adapter := range adapters {
		shim := filepath.Join(directory, adapter.Binary)
		if !adapter.NeedsShim() {
			os.Remove(shim)
			continue
		}
		script := "#!/bin/sh\n" + EnvShimDir + "=" + shellQuote(directory) +
			" exec " + shellQuote(cde) + " launch " + adapter.Name + ` "$@"` + "\n"
		if existing, err := os.ReadFile(shim); err == nil && string(existing) == script {
			continue
		}
		// A symlink left in a shim directory would make WriteFile overwrite
		// its target instead of the shim.
		if info, err := os.Lstat(shim); err == nil && info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(shim); err != nil {
				return err
			}
		}
		if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// RemoveStaleShims removes the shims of Codes that have quit and of terminals
// that no longer exist.
func RemoveStaleShims(root string, pid int, isLive func(string) bool) {
	base := filepath.Dir(root)
	if entries, err := os.ReadDir(base); err == nil {
		for _, entry := range entries {
			owner, err := strconv.Atoi(entry.Name())
			if err != nil || owner <= 0 || owner == pid {
				continue
			}
			if !processRunning(owner) {
				os.RemoveAll(filepath.Join(base, entry.Name()))
			}
		}
	}
	if entries, err := os.ReadDir(root); err == nil {
		for _, entry := range entries {
			if !isLive(entry.Name()) {
				os.RemoveAll(filepath.Join(root, entry.Name()))
			}
		}
	}
}
