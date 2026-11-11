package agent

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

// assets are the files Code installs or points agents at: OpenCode's plugins,
// Pi's and PiG's extensions and the shell integration scripts.
//
//go:embed all:assets
var assets embed.FS

// ExtractAssets writes the embedded agent resources under dir, creating it if
// needed, and returns dir. Files are rewritten only when their contents differ.
func ExtractAssets(dir string) error {
	return fs.WalkDir(assets, "assets", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel("assets", path)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := assets.ReadFile(path)
		if err != nil {
			return err
		}
		if existing, err := os.ReadFile(target); err == nil && string(existing) == string(data) {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// ExtractAssetsString is a convenience over ExtractAssets that also reports the
// assets directory.
func ExtractAssetsString(dir string) (string, error) {
	if err := ExtractAssets(dir); err != nil {
		return "", err
	}
	return dir, nil
}
