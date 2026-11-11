package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractAssetsIncludesDotFiles(t *testing.T) {
	dir := t.TempDir()
	if err := ExtractAssets(dir); err != nil {
		t.Fatal(err)
	}
	// The zsh integration is a dot file, which a plain //go:embed would drop.
	for _, name := range []string{
		"shell-integration/zsh/.zshenv",
		"agents/opencode/code.js",
		"agents/pi/code.ts",
		"agents/pig/code/extension.go.txt",
		"agents/pig/code/go.mod.txt",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s not extracted: %v", name, err)
		}
	}
}

func TestInstallExtensionRenamesEmbeddedTxt(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "extension.go.txt"), []byte("package code"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "go.mod.txt"), []byte("module code"), 0o644); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	installExtension(directory, "code", source)
	for _, name := range []string{"extension.go", "go.mod"} {
		if _, err := os.Stat(filepath.Join(directory, "code", name)); err != nil {
			t.Errorf("%s not installed: %v", name, err)
		}
	}
}
