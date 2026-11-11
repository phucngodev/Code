package agent

import (
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
		return candidate, true
	}
	return "", false
}

func standardize(path string) string {
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

// ShimsRoot is the directory holding every running Code's terminal shims.
func ShimsRoot(pid int) string {
	return filepath.Join(os.TempDir(), "code-shims", strconv.Itoa(pid))
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
