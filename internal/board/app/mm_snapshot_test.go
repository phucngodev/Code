package app

import (
	"testing"

	"github.com/phucngodev/code/internal/board/doc"
	"github.com/phucngodev/code/internal/board/geo"
)

// TestMindmapRenders builds a mindmap and draws it, so the two-sided layout,
// the coloured branches, the text leaves and a boundary are exercised. With
// GODRAW_SNAPSHOTS set it writes the frame out to look at.
func TestMindmapRenders(t *testing.T) {
	a, tt, _ := newTest(t)
	e := a.Editor()
	root := e.CreateMindmapRoot(geo.V(500, 400))
	e.EditText = "Project"
	e.EditChanged()
	e.StopEditing()

	add := func(parent doc.ID, text string) doc.ID {
		id := e.AddMindmapChild(parent)
		e.EditText = text
		e.EditChanged()
		e.StopEditing()
		return id
	}
	add(root, "Plan")
	add(root, "Solution")
	ideas := add(root, "Ideas")

	deep := add(ideas, "34efdsfw")
	var group []doc.ID
	for _, t := range []string{"fdsafds", "fdsafaf", "fdsaft refwf", "fadfsasdfa"} {
		group = append(group, add(deep, t))
	}
	add(ideas, "fdsafasrewf")
	add(ideas, "fdsafsadfs")
	add(ideas, "fdsafsf")

	e.Select(group...)
	e.AddBoundary()
	e.Select()
	e.ZoomToFit()

	tt.Frame()
	snapshot(t, tt, "mindmap")
}
