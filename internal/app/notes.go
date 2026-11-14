package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/model"
	notesapp "github.com/phucngodev/code/internal/notes/app"
	notestheme "github.com/phucngodev/code/internal/notes/theme"
)

// notesCanvas is a surface's notes view: the Markdown editor of internal/notes
// over a folder of notes, shown where a terminal would be.
type notesCanvas struct {
	app  *notesapp.App
	win  *mygo.Window
	root string
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
	// The folder is chosen off the main thread, since the dialog blocks.
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: w.win, Title: "Open Notes Folder", ButtonLabel: "Open",
			Directory: true, CreateDirectories: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		root := paths[0]
		mygo.RunOnMain(func() { w.openNotes(root, ws) })
	}()
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
		n := &notesCanvas{win: w.win, root: s.dir}
		n.app = notesapp.New(&notesHost{c: n}, n.root)
		s.notes = n
	}
	term := w.app.terminalCfg.Light
	if c.Theme().Dark {
		term = w.app.terminalCfg.Dark
	}
	s.notes.app.View(c, notestheme.Terminal(c.Theme().Dark, term))
}
