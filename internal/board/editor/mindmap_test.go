package editor

import (
	"slices"
	"testing"

	"github.com/phucngodev/code/internal/board/doc"
	"github.com/phucngodev/code/internal/board/geo"
)

// labelNode gives a node a label and ends its editing, so a text leaf survives
// (an unlabelled text shape goes away).
func labelNode(e *Editor, id doc.ID, text string) {
	e.update(id, func(s *doc.Shape) { s.Text = text })
	e.StopEditing()
}

// TestMindmapGrow checks the interactive tree: a root, its child and a
// sibling, joined by bound arrows, laid out to the right.
func TestMindmapGrow(t *testing.T) {
	h := newHarness(t)
	e := h.Editor

	root := e.CreateMindmapRoot(geo.V(0, 0))
	if root == "" {
		t.Fatal("CreateMindmapRoot made no node")
	}
	e.StopEditing()
	child := e.AddMindmapChild(root)
	if got := e.MindmapParent(child); got != root {
		t.Fatalf("child parent = %q, want %q", got, root)
	}
	if kids := e.MindmapChildren(root); len(kids) != 1 || kids[0] != child {
		t.Fatalf("children = %v, want [%s]", kids, child)
	}
	if r, c := e.nodeCenter(root), e.nodeCenter(child); c.X <= r.X {
		t.Fatalf("child x %v should be right of root x %v", c.X, r.X)
	}

	e.StopEditing()
	sib := e.AddMindmapSibling(child)
	if got := e.MindmapParent(sib); got != root {
		t.Fatalf("sibling parent = %q, want %q", got, root)
	}
	if kids := e.MindmapChildren(root); len(kids) != 2 {
		t.Fatalf("children after sibling = %v", kids)
	}
	// Two branches balance: one on each side of the title.
	r := e.nodeCenter(root)
	cl, sl := e.nodeCenter(child).X-r.X, e.nodeCenter(sib).X-r.X
	if (cl >= 0) == (sl >= 0) {
		t.Fatal("with two branches, one should sit on each side of the root")
	}
}

// TestMindmapChildOrderStable checks the tree lays out the same every frame:
// the children come back in a fixed order, not the order of a map.
func TestMindmapChildOrderStable(t *testing.T) {
	h := newHarness(t)
	e := h.Editor

	root := e.CreateMindmapRoot(geo.V(0, 0))
	e.StopEditing()
	for i := 0; i < 5; i++ {
		e.AddMindmapChild(root)
		e.StopEditing()
	}
	first := e.MindmapChildren(root)
	if len(first) != 5 {
		t.Fatalf("children = %d, want 5", len(first))
	}
	for i := 0; i < 10; i++ {
		if got := e.MindmapChildren(root); !slices.Equal(got, first) {
			t.Fatalf("child order changed: %v vs %v", got, first)
		}
	}
}

// TestMindmapPromote checks a node climbs to its grandparent.
func TestMindmapPromote(t *testing.T) {
	h := newHarness(t)
	e := h.Editor

	root := e.CreateMindmapRoot(geo.V(0, 0))
	e.StopEditing()
	child := e.AddMindmapChild(root)
	e.StopEditing()
	grand := e.AddMindmapChild(child)
	labelNode(e, grand, "grand")

	e.PromoteMindmapNode(grand)
	if got := e.MindmapParent(grand); got != root {
		t.Fatalf("promoted parent = %q, want %q", got, root)
	}
}

// TestArrangeAsMindmap checks the action turns a loose selection into a tree.
func TestArrangeAsMindmap(t *testing.T) {
	h := newHarness(t)
	e := h.Editor

	a := e.createMindNode(geo.V(0, 0))
	b := e.createMindNode(geo.V(400, 250))
	c := e.createMindNode(geo.V(800, 500))
	e.Select(a, b, c)
	e.ArrangeAsMindmap()

	if got := e.MindmapParent(b); got != a {
		t.Fatalf("b parent = %q, want the root %q", got, a)
	}
	if got := e.MindmapParent(c); got != a {
		t.Fatalf("c parent = %q, want the root %q", got, a)
	}
	if kids := e.MindmapChildren(a); len(kids) != 2 {
		t.Fatalf("root children = %v, want 2", kids)
	}
	// Two branches balance: one on each side of the root.
	r := e.nodeCenter(a)
	bl, cl := e.nodeCenter(b).X-r.X, e.nodeCenter(c).X-r.X
	if (bl >= 0) == (cl >= 0) {
		t.Fatal("the two branches should sit on opposite sides of the root")
	}
	if bl == 0 || cl == 0 {
		t.Fatal("children should sit away from the root")
	}
}

// TestMindmapTextLeaf checks a node deeper than the branches is plain text,
// that the title's branches stay boxes, and that a branch has no arrowhead.
func TestMindmapTextLeaf(t *testing.T) {
	h := newHarness(t)
	e := h.Editor

	root := e.CreateMindmapRoot(geo.V(0, 0))
	e.StopEditing()
	child := e.AddMindmapChild(root)
	e.StopEditing()
	grand := e.AddMindmapChild(child)
	labelNode(e, grand, "leaf")

	if s := e.Doc.Shapes[grand]; s == nil || s.Type != doc.TypeText {
		t.Fatalf("grandchild = %+v, want a text leaf", e.Doc.Shapes[grand])
	}
	if s := e.Doc.Shapes[child]; s == nil || s.Type != doc.TypeGeo {
		t.Fatalf("top-level branch = %+v, want a box", e.Doc.Shapes[child])
	}
	if a := e.edgeArrow(root, child); a != "" && e.Doc.Shapes[a].HeadEnd != "none" {
		t.Fatalf("branch head = %q, want none", e.Doc.Shapes[a].HeadEnd)
	}
}

// TestMindmapBranchColoursDiffer checks neighbouring branches take different
// colours, so the map reads as one colour per branch.
func TestMindmapBranchColoursDiffer(t *testing.T) {
	seen := map[string]bool{}
	for k := 0; k < len(mindBranchColors); k++ {
		c := mindBranchColor(k)
		if seen[c] {
			t.Fatalf("colour %q repeats within a cycle", c)
		}
		seen[c] = true
	}
}

// TestMindmapBranchesTakeColours checks the root keeps the plain ink and a
// branch takes its own colour.
func TestMindmapBranchesTakeColours(t *testing.T) {
	h := newHarness(t)
	e := h.Editor

	root := e.CreateMindmapRoot(geo.V(0, 0))
	e.StopEditing()
	child := e.AddMindmapChild(root)
	e.StopEditing()

	if got := e.Doc.Shapes[root].Color; got != "black" {
		t.Fatalf("root colour = %q, want black", got)
	}
	want := mindBranchColor(0)
	if e.Doc.Shapes[child].Color != want {
		t.Fatalf("child colour = %q, want %q", e.Doc.Shapes[child].Color, want)
	}
	if a := e.edgeArrow(root, child); a == "" || e.Doc.Shapes[a].Color != want {
		t.Fatalf("branch colour = %q, want %q", e.Doc.Shapes[a].Color, want)
	}
}

// TestAddBoundaryWrapsSiblings checks a boundary makes a bracket and a label
// for the selected siblings, follows them when they move, and takes its label
// with it when removed.
func TestAddBoundaryWrapsSiblings(t *testing.T) {
	h := newHarness(t)
	e := h.Editor
	root := e.CreateMindmapRoot(geo.V(0, 0))
	e.StopEditing()
	var kids []doc.ID
	for _, name := range []string{"a", "b", "c"} {
		k := e.AddMindmapChild(root)
		labelNode(e, k, name)
		kids = append(kids, k)
	}

	e.Select(kids[0], kids[1])
	brk := e.AddBoundary()
	if brk == "" {
		t.Fatal("AddBoundary made nothing")
	}
	m, ok := e.boundaryOf(brk)
	if !ok || len(m.Nodes) != 2 || m.Label == "" || e.Doc.Shapes[m.Label] == nil {
		t.Fatalf("boundary = %+v ok=%v", m, ok)
	}

	// One wrapped node moves: the bracket follows it.
	before := e.PageBounds(brk)
	c := e.nodeCenter(kids[1])
	e.moveNodeCenter(kids[1], c.X, c.Y+120)
	e.RefreshBoundaries()
	if after := e.PageBounds(brk); after == before {
		t.Fatal("the bracket should follow the wrapped nodes")
	}

	// A colour the user sets on the label sticks through a refresh.
	e.update(m.Label, func(s *doc.Shape) { s.Color = "green" })
	e.RefreshBoundaries()
	if got := e.Doc.Shapes[m.Label].Color; got != "green" {
		t.Fatalf("label colour = %q, want green after a refresh", got)
	}

	// Removing the boundary takes its label with it.
	label := m.Label
	e.RemoveBoundary(brk)
	if e.Doc.Shapes[brk] != nil || e.Doc.Shapes[label] != nil {
		t.Fatal("RemoveBoundary should remove the bracket and its label")
	}
}

// TestMindmapKeepsAChosenColour checks a colour the user sets on a node or a
// branch survives a re-layout instead of being reset to the branch colour.
func TestMindmapKeepsAChosenColour(t *testing.T) {
	h := newHarness(t)
	e := h.Editor
	root := e.CreateMindmapRoot(geo.V(0, 0))
	e.StopEditing()
	child := e.AddMindmapChild(root)
	labelNode(e, child, "a")

	if c := e.Doc.Shapes[child].Color; c == "" || c == "black" {
		t.Fatalf("the branch should take a colour, got %q", c)
	}
	e.update(child, func(s *doc.Shape) { s.Color = "red" })
	e.layoutTree(root)
	if got := e.Doc.Shapes[child].Color; got != "red" {
		t.Fatalf("node colour = %q, want red after a re-layout", got)
	}

	a := e.edgeArrow(root, child)
	e.update(a, func(s *doc.Shape) { s.Color = "red" })
	e.layoutTree(root)
	if got := e.Doc.Shapes[a].Color; got != "red" {
		t.Fatalf("branch colour = %q, want red after a re-layout", got)
	}
}
