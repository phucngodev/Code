package app

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	boardapp "github.com/phucngodev/code/internal/board/app"
	boarddoc "github.com/phucngodev/code/internal/board/doc"
	"github.com/phucngodev/code/internal/model"
)

// boardCanvas is a surface's drawing canvas: the whiteboard UI of
// internal/board over the drawing at path, shown where a terminal would be.
type boardCanvas struct {
	state *boardapp.App
	doc   *boarddoc.Drawing
	win   *mygo.Window
	path  string
}

// boardHost connects the whiteboard to the window that shows it, as it asks of
// what holds it.
type boardHost struct{ c *boardCanvas }

// Changed keeps the latest snapshot of the drawing, so closing saves it.
func (h *boardHost) Changed(d *boarddoc.Drawing) { h.c.doc = d }

// Update runs fn on the main thread and draws a frame after it.
func (h *boardHost) Update(fn func()) {
	if h.c.win != nil {
		h.c.win.Update(fn)
	}
}

// Window is the window the whiteboard's dialogs and drag-drop use.
func (h *boardHost) Window() *mygo.Window { return h.c.win }

// Path is the file the drawing is saved to, and SetPath records a new one
// after Open or Save As.
func (h *boardHost) Path() string        { return h.c.path }
func (h *boardHost) SetPath(path string) { h.c.path = path }

// save writes the drawing to its file, quietly: a canvas that cannot be saved
// must not stop the app from closing.
func (b *boardCanvas) save() {
	if b.path == "" || b.doc == nil {
		return
	}
	var buf bytes.Buffer
	if err := boarddoc.Write(&buf, b.doc); err != nil {
		return
	}
	_ = os.WriteFile(b.path, buf.Bytes(), 0o600)
}

// drawingPath is where Code keeps the drawing canvas between launches.
func (a *App) drawingPath() string {
	return filepath.Join(filepath.Dir(a.paths.Config), "drawing.tldr")
}

// close lets go of the whiteboard's view and drawing, so a closed tab releases
// its memory.
func (b *boardCanvas) close() {
	b.state = nil
	b.doc = nil
	b.win = nil
}

// loadDrawing reads the saved drawing, or a new one when there is none.
func (a *App) loadDrawing() *boarddoc.Drawing {
	if data, err := os.ReadFile(a.drawingPath()); err == nil {
		if d, err := boarddoc.Read(bytes.NewReader(data)); err == nil {
			return d
		}
	}
	return boarddoc.New()
}

// toggleBoard opens the drawing canvas in a tab of its own, or closes it when
// it is the tab in view, so the terminals in the other tabs stay.
func (w *WindowState) toggleBoard() {
	if s := w.focusedSurface(); s != nil && s.kind == "canvas" {
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
	// A canvas already open in this workspace is shown again, so its drawing
	// keeps one file.
	for _, tab := range ws.Tabs {
		if s := tab.Focused(); s != nil && s.kind == "canvas" {
			ws.Select(tab)
			w.store.Notify()
			w.focusSelected()
			return
		}
	}
	s := w.newSurface(newID(), "", ws.ID)
	s.kind = "canvas"
	ws.Add(model.NewTab(newID(), s))
	w.store.Notify()
	w.focusSelected()
}

// buildBoard draws a canvas surface in the terminal area.
func (w *WindowState) buildBoard(c *ui.Context, s *Surface) {
	if s.board == nil {
		path := w.app.drawingPath()
		b := &boardCanvas{doc: w.app.loadDrawing(), win: w.win, path: path}
		b.state = boardapp.New(b.doc, &boardHost{c: b})
		// The canvas opens with its grid shown.
		b.state.Editor().Grid = true
		s.board = b
	}
	s.board.state.View(c)
	if s.wantFocus {
		s.board.state.FocusCanvas(c)
		s.wantFocus = false
	}
}
