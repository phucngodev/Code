package agent

import (
	"os"
	"path/filepath"
	"strings"
)

// InstallExtensions installs or updates Code's extension for every agent that
// reads one from a config folder and is set up on this machine, leaving alone
// one whose config folder is not there.
func InstallExtensions(assetsDir string, environment map[string]string) {
	home, _ := os.UserHomeDir()
	piAgentDir := environment["PI_CODING_AGENT_DIR"]
	if piAgentDir == "" {
		piAgentDir = filepath.Join(home, ".pi", "agent")
	}
	pigAgentDir := filepath.Join(home, ".config", "pig", "agent")
	extensions := []struct {
		directory string
		name      string
		source    string
	}{
		{filepath.Join(piAgentDir, "extensions"), "code.ts", filepath.Join(assetsDir, "agents", "pi", "code.ts")},
		{filepath.Join(pigAgentDir, "extensions"), "code", filepath.Join(assetsDir, "agents", "pig", "code")},
	}
	for _, extension := range extensions {
		if _, err := os.Stat(filepath.Dir(extension.directory)); err != nil {
			continue
		}
		installExtension(extension.directory, extension.name, extension.source)
	}
}

// installExtension copies a file or a folder of extension files into place,
// writing only what changed.
func installExtension(directory, name, source string) {
	info, err := os.Stat(source)
	if err != nil {
		return
	}
	if !info.IsDir() {
		data, err := os.ReadFile(source)
		if err != nil {
			return
		}
		writeIfChanged(filepath.Join(directory, name), data, 0o644)
		return
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			continue
		}
		// Extension sources are embedded with a .txt suffix, since a nested
		// go.mod would put the subtree outside the module that embeds it.
		target := filepath.Join(directory, name, strings.TrimSuffix(entry.Name(), ".txt"))
		writeIfChanged(target, data, 0o644)
	}
}

func writeIfChanged(path string, data []byte, mode os.FileMode) {
	if existing, err := os.ReadFile(path); err == nil && string(existing) == string(data) {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	os.WriteFile(path, data, mode)
}
