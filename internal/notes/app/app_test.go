package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/notes/theme"
)

type testHost struct{}

func (testHost) Window() *mygo.Window { return nil }
func (testHost) Update(fn func())     { fn() }
func (testHost) OpenFolder()          {}
func (testHost) OpenFile()            {}

// newNotes makes a notes view over a folder of two notes.
func newNotes(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []struct{ name, text string }{
		{"a.md", "# Alpha\n\nFirst note body.\n"},
		{"b.md", "# Beta\n\nSecond note body.\n"},
	} {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := New(testHost{}, dir)
	t.Cleanup(a.Close)
	// The workspace reads the notes on goroutines; wait for its index.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(a.ws.FindFiles("", 10, false)) < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	return a
}

func frame(t *testing.T, a *App) {
	t.Helper()
	th := theme.Terminal(false, nil)
	ui.NewTester(func(c *ui.Context) { a.View(c, th) }, 1000, 700).Frame()
}

// TestViewRenders checks the sidebar and the editor draw a full frame without
// panicking, and that opening a folder does not open a note on its own.
func TestViewRenders(t *testing.T) {
	a := newNotes(t)
	frame(t, a)
	if a.editor() != nil {
		t.Fatal("opening a notes folder should not open a note")
	}
}

// TestPaletteModesRender checks the quick open, search and command palettes
// all render.
func TestPaletteModesRender(t *testing.T) {
	a := newNotes(t)
	for _, mode := range []paletteMode{modeFiles, modeSearch, modeCommands} {
		a.palette.show(mode, "")
		if !a.palette.open {
			t.Fatalf("mode %d did not open", mode)
		}
		frame(t, a)
		a.palette.close()
	}
}

// TestFindFilesListsNotes checks quick open lists the notes by name.
func TestFindFilesListsNotes(t *testing.T) {
	a := newNotes(t)
	a.palette.show(modeFiles, "Alpha")
	a.palette.update()
	if len(a.palette.items) == 0 {
		t.Fatal("quick open should match a note by title")
	}
}

// TestPaletteEnterOpensTheSelectedNote checks Enter in the palette opens the
// item highlighted.
func TestPaletteEnterOpensTheSelectedNote(t *testing.T) {
	a := newNotes(t)
	th := theme.Terminal(false, nil)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c, th) }, 1000, 700)
	tt.Frame()

	a.palette.show(modeFiles, "Beta")
	tt.Frame()
	if len(a.palette.items) == 0 {
		t.Fatal("quick open listed nothing")
	}

	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if got := a.tab(); got == nil || filepath.Base(got.path) != "b.md" {
		t.Fatalf("Enter opened %+v, want b.md", got)
	}
}

// TestPaletteArrowsMoveTheSelection checks Up and Down move the highlighted
// item.
func TestPaletteArrowsMoveTheSelection(t *testing.T) {
	a := newNotes(t)
	th := theme.Terminal(false, nil)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c, th) }, 1000, 700)
	tt.Frame()

	a.palette.show(modeFiles, "")
	tt.Frame()
	if len(a.palette.items) < 2 {
		t.Fatalf("expected at least two notes, got %d", len(a.palette.items))
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if a.palette.selected != 1 {
		t.Fatalf("Down left the selection at %d, want 1", a.palette.selected)
	}
	tt.Key(0, ui.KeyUp)
	tt.Frame()
	if a.palette.selected != 0 {
		t.Fatalf("Up left the selection at %d, want 0", a.palette.selected)
	}
}

// TestOpenAtPlacesTheCaret checks a search result opens its note with the
// caret on the match.
func TestOpenAtPlacesTheCaret(t *testing.T) {
	a := newNotes(t)
	a.openAt(filepath.Join(a.root, "b.md"), [2]int{6, 10})
	tb := a.tab()
	if tb == nil || tb.ed == nil || filepath.Base(tb.path) != "b.md" {
		t.Fatalf("openAt opened %+v", tb)
	}
	if got, _ := tb.ed.Selection(); got != 6 {
		t.Fatalf("caret at %d, want 6", got)
	}
}

// TestNotesShortcutModifiers checks the palettes' modifiers per platform: on
// Linux Code binds Control+Shift for its own find and go to, so the notes view
// must not use them.
func TestNotesShortcutModifiers(t *testing.T) {
	if runtime.GOOS == "darwin" {
		if searchMods != ui.Cmd|ui.Shift || commandMods != ui.Cmd|ui.Shift {
			t.Fatalf("macOS modifiers = %v/%v, want Command+Shift", searchMods, commandMods)
		}
		return
	}
	if searchMods != ui.Cmd|ui.Alt || commandMods != ui.Cmd|ui.Alt {
		t.Fatalf("non-macOS modifiers = %v/%v, want Control+Alt", searchMods, commandMods)
	}
}

// TestOpenNoteOutsideTheFolder checks a Markdown file from another folder can
// be opened on its own.
func TestOpenNoteOutsideTheFolder(t *testing.T) {
	a := newNotes(t)
	other := t.TempDir()
	path := filepath.Join(other, "outside.md")
	if err := os.WriteFile(path, []byte("# Outside\n\nBody.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.OpenNote(path)
	tb := a.tab()
	if tb == nil || tb.ed == nil || filepath.Base(tb.path) != "outside.md" {
		t.Fatalf("open outside = %+v", tb)
	}
}
