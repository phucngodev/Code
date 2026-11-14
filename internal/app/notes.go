package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/model"
	notesapp "github.com/phucngodev/code/internal/notes/app"
	notestheme "github.com/phucngodev/code/internal/notes/theme"
)

// notesCanvas is a surface's notes view: the Markdown editor of internal/notes
// over a folder of notes, shown where a terminal would be.
type notesCanvas struct {
	app     *notesapp.App
	win     *mygo.Window
	root    string
	owner   *WindowState
	surface *Surface
}

// notesHost connects the notes view to the window that shows it.
type notesHost struct{ c *notesCanvas }

func (h *notesHost) Window() *mygo.Window { return h.c.win }

func (h *notesHost) Update(fn func()) {
	if h.c.win != nil {
		h.c.win.Update(fn)
		return
	}
	fn()
}

// OpenFolder asks for a folder and shows it in place, so the view switches to
// another folder without opening a second tab.
func (h *notesHost) OpenFolder() {
	w, s := h.c.owner, h.c.surface
	if w == nil || s == nil {
		return
	}
	w.pickNotesFolder(func(root string) {
		if s.notes != nil {
			s.notes.close()
		}
		s.notes = nil
		s.dir = root
	})
}

// OpenFile asks for a Markdown file, from any folder, and shows it in this
// view, so a note outside the workspace can be opened on its own.
func (h *notesHost) OpenFile() {
	w := h.c.owner
	if w == nil {
		return
	}
	w.pickNoteFile(func(path string) {
		if h.c.app != nil {
			h.c.app.OpenNote(path)
		}
	})
}

// save writes every note with unsaved changes, quietly: a notes view that
// cannot be saved must not stop the app from closing.
func (n *notesCanvas) save() {
	if n.app != nil {
		n.app.SaveAll()
	}
}

// close stops the notes workspace and lets go of the view, so a closed tab
// releases its memory: the workspace's watch and scanning goroutines would
// otherwise keep it all alive.
func (n *notesCanvas) close() {
	if n.app != nil {
		n.app.Close()
		n.app = nil
	}
	n.win = nil
}

// toggleNotes opens the notes view in a tab of its own, or closes it when it
// is the tab in view, so the terminals in the other tabs stay.
func (w *WindowState) toggleNotes() {
	if s := w.focusedSurface(); s != nil && s.kind == "notes" {
		if tab := w.selectedTab(); tab != nil {
			w.closeTabNow(tab)
		}
		return
	}
	ws := w.store.Selected()
	if ws == nil {
		w.newWorkspace("")
		ws = w.store.Selected()
	}
	if ws == nil {
		return
	}
	// A notes view already open in this workspace is shown again, so it keeps
	// one folder.
	for _, tab := range ws.Tabs {
		if s := tab.Focused(); s != nil && s.kind == "notes" {
			ws.Select(tab)
			w.store.Notify()
			w.focusSelected()
			return
		}
	}
	// The folder opened last is shown again; with none, one is chosen.
	if dir := w.app.loadNotesFolder(); dir != "" {
		w.openNotes(dir, ws)
		return
	}
	w.pickNotesFolder(func(root string) { w.openNotes(root, ws) })
}

// pickNotesFolder asks for a notes folder off the main thread, since the
// dialog blocks, records it, and runs done with it on the main thread.
func (w *WindowState) pickNotesFolder(done func(root string)) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: w.win, Title: "Open Notes Folder", ButtonLabel: "Open",
			Directory: true, CreateDirectories: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		root := paths[0]
		mygo.RunOnMain(func() {
			w.app.saveNotesFolder(root)
			done(root)
		})
	}()
}

// pickNoteFile asks for a Markdown file off the main thread and runs done with
// it on the main thread.
func (w *WindowState) pickNoteFile(done func(path string)) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: w.win, Title: "Open Markdown File", ButtonLabel: "Open",
			Filters: []mygo.FileFilter{{Name: "Markdown", Extensions: []string{"md", "markdown", "mdown", "mkd"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		path := paths[0]
		mygo.RunOnMain(func() { done(path) })
	}()
}

// notesFolderPath is where Code remembers the last notes folder between
// launches.
func (a *App) notesFolderPath() string {
	return filepath.Join(filepath.Dir(a.paths.Config), "notes-folder")
}

// loadNotesFolder is the last notes folder, or "" when there is none or it is
// gone.
func (a *App) loadNotesFolder() string {
	data, err := os.ReadFile(a.notesFolderPath())
	if err != nil {
		return ""
	}
	dir := strings.TrimSpace(string(data))
	if dir == "" {
		return ""
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return ""
	}
	return dir
}

// saveNotesFolder records the folder Code last showed notes from.
func (a *App) saveNotesFolder(dir string) {
	_ = os.WriteFile(a.notesFolderPath(), []byte(dir), 0o600)
}

// openNotes opens the notes view on root in the workspace ws, on the main
// thread.
func (w *WindowState) openNotes(root string, ws *model.Workspace[*Surface]) {
	s := w.newSurface(newID(), root, ws.ID)
	s.kind = "notes"
	ws.Add(model.NewTab(newID(), s))
	w.store.Notify()
	w.focusSelected()
}

// buildNotes draws a notes view in the terminal area.
func (w *WindowState) buildNotes(c *ui.Context, s *Surface) {
	if s.notes == nil {
		n := &notesCanvas{win: w.win, root: s.dir, owner: w, surface: s}
		n.app = notesapp.New(&notesHost{c: n}, n.root)
		s.notes = n
	}
	term := w.app.terminalCfg.Light
	if c.Theme().Dark {
		term = w.app.terminalCfg.Dark
	}
	s.notes.app.View(c, notestheme.Terminal(c.Theme().Dark, term))
}
