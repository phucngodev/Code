package editor

import (
	"fmt"
	"testing"

	"github.com/phucngodev/code/internal/board/doc"
	"github.com/phucngodev/code/internal/board/geo"
)

// TestMindmapStress grows a big tree, wraps groups, moves, deletes, promotes
// and undoes, to surface a panic in the layout or a boundary.
func TestMindmapStress(t *testing.T) {
	h := newHarness(t)
	e := h.Editor
	root := e.CreateMindmapRoot(geo.V(0, 0))
	e.StopEditing()
	nodes := []doc.ID{root}
	for i := 0; i < 40; i++ {
		parent := nodes[i/2]
		id := e.AddMindmapChild(parent)
		labelNode(e, id, fmt.Sprintf("n%d", i))
		nodes = append(nodes, id)
	}
	// Wrap groups in boundaries.
	for i := 0; i+1 < len(nodes) && i < 20; i += 2 {
		e.Select(nodes[i], nodes[i+1])
		e.AddBoundary()
	}
	// Move nodes and refresh the brackets.
	for i, id := range nodes {
		c := e.nodeCenter(id)
		e.moveNodeCenter(id, c.X+float64(i), c.Y+float64(i))
		e.RefreshBoundaries()
	}
	// Promote, delete and arrange.
	e.Select(nodes[3])
	e.PromoteMindmapNode(nodes[3])
	e.Select(nodes[5], nodes[7])
	e.Delete()
	e.Select(nodes...)
	e.ArrangeAsMindmap()
	// Undo and redo everything.
	for i := 0; i < 60; i++ {
		e.Undo()
	}
	for i := 0; i < 60; i++ {
		e.Redo()
	}
}
