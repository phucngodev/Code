package model

import (
	"os"
	"strings"
	"testing"
)

type testLeaf struct {
	id, title, dir string
	kind           string
}

func (l *testLeaf) LeafID() string    { return l.id }
func (l *testLeaf) LeafTitle() string { return l.title }
func (l *testLeaf) LeafDir() string   { return l.dir }
func (l *testLeaf) LeafKind() string  { return l.kind }

// TestCanvasTabRoundTrips checks a pane's kind survives a session save and
// restore, so a drawing tab comes back as a drawing tab.
func TestCanvasTabRoundTrips(t *testing.T) {
	s := NewStore[*testLeaf]()
	s.AddWorkspace("ws", nil, 0).Add(NewTab("tab", &testLeaf{id: "c", kind: "canvas"}))

	snapshot := s.Snapshot(nil)
	if snapshot.Workspaces[0].Tabs[0].Layout.Kind != "canvas" {
		t.Fatal("the canvas kind should be saved")
	}

	restored := NewStore[*testLeaf]()
	restored.Restore(snapshot, func() string { return "new" }, func(id, dir, workspace, kind string) *testLeaf {
		return &testLeaf{id: id, kind: kind}
	})
	leaves := restored.Panes()
	if len(leaves) != 1 || leaves[0].LeafKind() != "canvas" {
		t.Fatalf("restored leaves = %+v, want one canvas", leaves)
	}
}

func TestTreeSplitAndRemove(t *testing.T) {
	a := &testLeaf{id: "a"}
	b := &testLeaf{id: "b"}
	c := &testLeaf{id: "c"}
	tree := NewTree(a)
	tree.Split(a, DirRight, b)
	if got := len(tree.Leaves()); got != 2 {
		t.Fatalf("leaves = %d, want 2", got)
	}
	tree.Split(a, DirDown, c)
	if got := len(tree.Leaves()); got != 3 {
		t.Fatalf("leaves = %d, want 3", got)
	}
	if _, ok := tree.Remove(b); !ok {
		t.Fatal("remove b failed")
	}
	if got := len(tree.Leaves()); got != 2 {
		t.Fatalf("leaves = %d, want 2", got)
	}
	if tree.Contains(b) {
		t.Fatal("b should be gone")
	}
}

func TestTreeRemoveLastPaneFails(t *testing.T) {
	a := &testLeaf{id: "a"}
	tree := NewTree(a)
	if _, ok := tree.Remove(a); ok {
		t.Fatal("removing the only pane should fail")
	}
}

func TestTreeSwap(t *testing.T) {
	a := &testLeaf{id: "a"}
	b := &testLeaf{id: "b"}
	tree := NewTree(a)
	tree.Split(a, DirRight, b)
	tree.Swap(a, b)
	leaves := tree.Leaves()
	if leaves[0] != b || leaves[1] != a {
		t.Fatalf("swap produced %v %v, want b a", leaves[0].id, leaves[1].id)
	}
}

func TestTreeZoom(t *testing.T) {
	a := &testLeaf{id: "a"}
	b := &testLeaf{id: "b"}
	tree := NewTree(a)
	tree.Split(a, DirRight, b)
	tree.ToggleZoom(a)
	if zoomed, ok := tree.Zoomed(); !ok || zoomed != a {
		t.Fatal("a should be zoomed")
	}
	if tree.ZoomedIndex() != 0 {
		t.Fatalf("zoomed index = %d, want 0", tree.ZoomedIndex())
	}
	tree.ToggleZoom(a)
	if _, ok := tree.Zoomed(); ok {
		t.Fatal("zoom should be cleared")
	}
}

func TestTreeNeighbor(t *testing.T) {
	a := &testLeaf{id: "a"}
	b := &testLeaf{id: "b"}
	tree := NewTree(a)
	tree.Split(a, DirRight, b)
	if got, ok := tree.Neighbor(a, NavRight); !ok || got != b {
		t.Fatal("neighbor right of a should be b")
	}
	if got, ok := tree.Neighbor(b, NavLeft); !ok || got != a {
		t.Fatal("neighbor left of b should be a")
	}
	if got, ok := tree.Neighbor(a, NavNext); !ok || got != b {
		t.Fatal("next after a should be b")
	}
}

func TestTreeResize(t *testing.T) {
	a := &testLeaf{id: "a"}
	b := &testLeaf{id: "b"}
	tree := NewTree(a)
	tree.Split(a, DirRight, b)
	if got := tree.DividerExtent(b, DirLeft); got != 1 {
		t.Fatalf("divider extent = %v, want 1", got)
	}
	// Resizing left moves the divider left, growing the pane on the right.
	tree.Resize(b, DirLeft, 10)
	if tree.Root.Pct != 40 {
		t.Fatalf("pct = %v, want 40", tree.Root.Pct)
	}
	// A resize larger than the split keeps both panes visible.
	tree.Resize(b, DirRight, 100)
	if tree.Root.Pct != 98 {
		t.Fatalf("pct = %v, want 98", tree.Root.Pct)
	}
}

func TestTreeResizeIgnoresMissingAxis(t *testing.T) {
	a := &testLeaf{id: "a"}
	b := &testLeaf{id: "b"}
	tree := NewTree(a)
	tree.Split(a, DirRight, b)
	// There is no vertical split, so a vertical resize does nothing.
	tree.Resize(b, DirUp, 10)
	if tree.Root.Pct != 50 {
		t.Fatalf("pct = %v, want 50", tree.Root.Pct)
	}
}

func TestStoreAddSelectMove(t *testing.T) {
	store := NewStore[*testLeaf]()
	w1 := store.AddWorkspace("w1", nil, -1)
	w2 := store.AddWorkspace("w2", nil, -1)
	if store.Selected() != w2 {
		t.Fatal("a new workspace should be selected")
	}
	store.Select(w1)
	if store.Selected() != w1 {
		t.Fatal("select failed")
	}
	store.Move(w1, 2)
	if store.Workspaces[1] != w1 {
		t.Fatalf("move failed: %v", store.Workspaces)
	}
	store.Remove(w1)
	if len(store.Workspaces) != 1 || store.Workspaces[0] != w2 {
		t.Fatalf("remove failed: %v", store.Workspaces)
	}
}

func TestWorkspaceNameFollowsFocus(t *testing.T) {
	store := NewStore[*testLeaf]()
	ws := store.AddWorkspace("w", nil, -1)
	leaf := &testLeaf{id: "a", dir: "/tmp/project"}
	ws.Add(NewTab("t", leaf))
	if got := ws.DisplayName(); got != "project" {
		t.Fatalf("name = %q, want project", got)
	}
	name := "Renamed"
	ws.Name = name
	ws.HasName = true
	if got := ws.DisplayName(); got != name {
		t.Fatalf("name = %q, want %q", got, name)
	}
}

func TestSessionRoundTrip(t *testing.T) {
	store := NewStore[*testLeaf]()
	ws := store.AddWorkspace("w", nil, -1)
	a := &testLeaf{id: "a", dir: "/tmp/a"}
	b := &testLeaf{id: "b", dir: "/tmp/b"}
	tab := NewTab("t", a)
	tab.Panes.Split(a, DirRight, b)
	ws.Add(tab)

	snapshot := store.Snapshot(nil)
	if len(snapshot.Workspaces) != 1 || len(snapshot.Workspaces[0].Tabs) != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}

	restored := NewStore[*testLeaf]()
	restored.Restore(snapshot, func() string { return "new-id" }, func(id, dir, workspace, kind string) *testLeaf {
		return &testLeaf{id: id, dir: dir}
	})
	if got := len(restored.Panes()); got != 2 {
		t.Fatalf("restored panes = %d, want 2", got)
	}
	if restored.Panes()[0].LeafID() != "a" || restored.Panes()[1].LeafID() != "b" {
		t.Fatalf("restored ids = %s, %s", restored.Panes()[0].LeafID(), restored.Panes()[1].LeafID())
	}
}

func TestAbbreviateHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := AbbreviateHome(home + "/Code"); got != "~/Code" {
		t.Fatalf("AbbreviateHome = %q, want ~/Code", got)
	}
	if got := FolderName(home + "/Code"); got != "Code" {
		t.Fatalf("FolderName = %q, want Code", got)
	}
}

func TestSessionSidebarFieldsPersist(t *testing.T) {
	hidden := true
	s := Session{Windows: []WindowSession{{Selected: 0}}, SidebarHidden: &hidden, SidebarWidth: 240}
	path := t.TempDir() + "/session.json"
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\"sidebarHidden\": true") || !strings.Contains(string(data), "\"sidebarWidth\": 240") {
		t.Fatalf("session = %s", data)
	}
	loaded, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SidebarHidden == nil || !*loaded.SidebarHidden || loaded.SidebarWidth != 240 {
		t.Fatalf("loaded = %+v", loaded)
	}
}
