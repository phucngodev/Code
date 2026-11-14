package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"github.com/phucngodev/code/internal/board/doc"
)

// Open asks for a drawing and shows it.
func (a *App) Open() {
	win := a.host.Window()
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: win, Title: "Open Drawing",
			Filters: []mygo.FileFilter{{Name: "tldraw Drawing", Extensions: []string{"tldr"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		data, err := os.ReadFile(paths[0])
		if err != nil {
			return
		}
		d, err := doc.Read(bytes.NewReader(data))
		if err != nil {
			return
		}
		a.host.Update(func() {
			a.ed.Replace(d)
			a.host.SetPath(paths[0])
			a.host.Changed(d)
		})
	}()
}

// Save writes the drawing to the file it came from, or asks for one.
func (a *App) Save() {
	a.saveDrawing(a.host.Path())
}

// SaveAs asks where to save the drawing and writes it there.
func (a *App) SaveAs() {
	a.saveDrawing("")
}

func (a *App) saveDrawing(path string) {
	d := a.ed.Doc
	if path == "" {
		win := a.host.Window()
		name := d.Name
		if name == "" {
			name = "drawing"
		}
		chosen, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent: win, Title: "Save Drawing", DefaultPath: name + ".tldr",
			Filters: []mygo.FileFilter{{Name: "tldraw Drawing", Extensions: []string{"tldr"}}},
		})
		if err != nil || chosen == "" {
			return
		}
		path = chosen
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf, d); err != nil {
		return
	}
	if !strings.EqualFold(filepath.Ext(path), ".tldr") {
		path += ".tldr"
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return
	}
	a.host.SetPath(path)
	a.ed.SetDocName(strings.TrimSuffix(filepath.Base(path), ".tldr"))
}
