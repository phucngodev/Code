package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/phucngodev/code/internal/board/doc"
)

// TestSaveWritesToHostPath checks Save writes the drawing to the file the host
// names, as Save As and Open set it.
func TestSaveWritesToHostPath(t *testing.T) {
	host := &testHost{}
	a := New(doc.New(), host)
	path := filepath.Join(t.TempDir(), "my-drawing.tldr")
	host.SetPath(path)

	a.Save()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Save did not write the file: %v", err)
	}
	if _, err := doc.Read(bytes.NewReader(data)); err != nil {
		t.Fatalf("the saved file does not read back: %v", err)
	}
}
