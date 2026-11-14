package app

import (
	"bytes"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	boarddoc "github.com/phucngodev/code/internal/board/doc"
)

// TestBoardViewRenders checks the drawing canvas the title bar toggles opens as
// a tab of its own, builds, and closes again.
func TestBoardViewRenders(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	before := len(w.store.Selected().Tabs)

	w.toggleBoard()
	s := w.focusedSurface()
	if s == nil || s.kind != "canvas" {
		t.Fatal("toggling should open a drawing canvas")
	}
	if got := len(w.store.Selected().Tabs); got != before+1 {
		t.Fatalf("tabs = %d, want %d", got, before+1)
	}

	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	if s.board == nil {
		t.Fatal("the canvas should have built")
	}
	if !s.board.state.Editor().Grid {
		t.Fatal("the canvas should open with its grid shown")
	}

	w.toggleBoard()
	if got := w.focusedSurface(); got == nil || got.kind == "canvas" {
		t.Fatal("toggling again should close the canvas tab")
	}
	if got := len(w.store.Selected().Tabs); got != before {
		t.Fatalf("tabs after close = %d, want %d", got, before)
	}
}

// TestBoardSavesOnClose checks the drawing is written to its file when the
// canvas closes, so it survives closing the tab or the app.
func TestBoardSavesOnClose(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	w.toggleBoard()
	s := w.focusedSurface()
	if s == nil || s.kind != "canvas" {
		t.Fatal("toggling should open a drawing canvas")
	}
	ui.NewTester(w.view, 900, 600).Frame()
	if s.board == nil {
		t.Fatal("the canvas did not build")
	}
	s.board.doc.GridSize = 42

	s.close()
	data, err := os.ReadFile(w.app.drawingPath())
	if err != nil {
		t.Fatalf("the drawing was not saved: %v", err)
	}
	loaded, err := boarddoc.Read(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("read drawing: %v", err)
	}
	if loaded.GridSize != 42 {
		t.Fatalf("saved grid = %v, want 42", loaded.GridSize)
	}
}

// TestClosingBoardReleasesIt checks closing the drawing tab drops its view, so
// the drawing is freed.
func TestClosingBoardReleasesIt(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	w.toggleBoard()
	s := w.focusedSurface()
	if s == nil || s.kind != "canvas" {
		t.Fatal("no drawing canvas")
	}
	ui.NewTester(w.view, 900, 600).Frame()
	if s.board == nil {
		t.Fatal("the canvas did not build")
	}
	w.toggleBoard()
	if s.board != nil {
		t.Fatal("closing should drop the drawing view")
	}
}
