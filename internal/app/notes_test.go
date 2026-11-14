package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/model"
)

// TestNotesViewRenders checks a notes pane is its own layout kind and builds
// the Markdown view, showing the notes in the folder.
func TestNotesViewRenders(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	ws := w.store.Selected()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.md"), []byte("# Hello\n\nWorld\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	w.openNotes(dir, ws)
	s := w.focusedSurface()
	if s == nil || s.kind != "notes" {
		t.Fatal("opening notes should make a notes pane")
	}

	// The notes view draws a full frame without panicking.
	ui.NewTester(w.view, 900, 600).Frame()
	if s.notes == nil {
		t.Fatal("the notes view should have built")
	}
	if s.LeafKind() != "notes" || s.LeafTitle() != "Notes" {
		t.Fatalf("kind = %q, title = %q, want notes", s.LeafKind(), s.LeafTitle())
	}
}

// TestNotesTabRoundTrips checks a notes pane is saved and restored as a notes
// pane, keeping its folder.
func TestNotesTabRoundTrips(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	ws := w.store.Selected()
	dir := t.TempDir()

	w.openNotes(dir, ws)
	snapshot := w.store.Snapshot(nil)
	var notes *model.Layout
	for _, tab := range snapshot.Workspaces[0].Tabs {
		if tab.Layout.Kind == "notes" {
			notes = tab.Layout
		}
	}
	if notes == nil {
		t.Fatal("no notes tab was saved")
	}
	if notes.Dir != dir {
		t.Fatalf("saved dir = %q, want %q", notes.Dir, dir)
	}

	restored := &WindowState{app: testApp(t), store: model.NewStore[*Surface]()}
	restored.store.Restore(snapshot, newID, func(id, dir, workspace, kind string) *Surface {
		s := restored.newSurface(id, dir, workspace)
		s.kind = paneKind(kind)
		return s
	})
	surfaces := restored.store.Panes()
	var found *Surface
	for _, s := range surfaces {
		if s.kind == "notes" {
			found = s
		}
	}
	if found == nil || found.dir != dir {
		t.Fatalf("restored = %+v, want one notes pane in %q", surfaces, dir)
	}
}

// TestRestoreKeepsPaneKinds checks a restored session gives a terminal the
// empty kind, so it launches a shell, and a canvas or notes view its own.
func TestRestoreKeepsPaneKinds(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	ws := w.store.Selected()
	s := w.newSurface(newID(), "", ws.ID)
	s.kind = "canvas"
	ws.Add(model.NewTab(newID(), s))
	w.openNotes(t.TempDir(), ws)

	snapshot := w.store.Snapshot(nil)
	restored := &WindowState{app: testApp(t), store: model.NewStore[*Surface]()}
	restored.store.Restore(snapshot, newID, func(id, dir, workspace, kind string) *Surface {
		s := restored.newSurface(id, dir, workspace)
		s.kind = paneKind(kind)
		return s
	})
	var canvas, notes, terminal int
	for _, s := range restored.store.Panes() {
		switch s.kind {
		case "canvas":
			canvas++
		case "notes":
			notes++
		case "":
			terminal++
		}
	}
	if canvas != 1 || notes != 1 || terminal < 1 {
		t.Fatalf("restored kinds: canvas=%d notes=%d terminal=%d", canvas, notes, terminal)
	}
}

// TestClosingNotesReleasesIt checks closing a notes tab closes its app, so the
// workspace's watch and scanning goroutines stop and its memory is freed.
func TestClosingNotesReleasesIt(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	ws := w.store.Selected()
	w.openNotes(t.TempDir(), ws)
	s := w.focusedSurface()
	if s == nil || s.kind != "notes" {
		t.Fatal("no notes view")
	}
	ui.NewTester(w.view, 900, 600).Frame()
	if s.notes == nil {
		t.Fatal("the notes app should have built")
	}
	app := s.notes.app
	if app == nil {
		t.Fatal("the notes app should have built")
	}
	w.closeTabNow(w.selectedTab())
	if s.notes != nil {
		t.Fatal("closing should drop the notes view")
	}
	if !app.Closed() {
		t.Fatal("closing should close the notes app")
	}
}
