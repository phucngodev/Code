package model

import (
	"os"
	"path/filepath"
	"strings"
)

// PaneLeaf is a terminal pane: comparable, so a pane can be identified in a
// split tree, and able to say its id, title and directory.
type PaneLeaf interface {
	comparable
	Leaf
}

// Leaf is what the workspace model needs from a terminal pane.
type Leaf interface {
	// LeafID is stable across a relaunch, since agents are told it.
	LeafID() string
	// LeafTitle is the terminal's own title, or empty.
	LeafTitle() string
	// LeafDir is the terminal's working directory, or empty.
	LeafDir() string
	// LeafKind is the pane's kind: "" for a terminal, "canvas" for a
	// drawing canvas, "notes" for a Markdown notes view, so each comes
	// back as itself on restore.
	LeafKind() string
}

// Tab is one tab's split layout and the pane that has its focus.
type Tab[L PaneLeaf] struct {
	ID      string
	Panes   *Tree[L]
	focused L
}

// NewTab makes a one-pane tab.
func NewTab[L PaneLeaf](id string, leaf L) *Tab[L] {
	return &Tab[L]{ID: id, Panes: NewTree(leaf), focused: leaf}
}

// NewTabFrom makes a tab around an existing tree.
func NewTabFrom[L PaneLeaf](id string, panes *Tree[L], focused L) *Tab[L] {
	return &Tab[L]{ID: id, Panes: panes, focused: focused}
}

// Focused returns the pane with focus.
func (t *Tab[L]) Focused() L { return t.focused }

// Title is the focused pane's title.
func (t *Tab[L]) Title() string { return DisplayTitle(t.focused) }

// Focus moves focus to a pane in the tab.
func (t *Tab[L]) Focus(leaf L) {
	if t.Panes.Contains(leaf) {
		t.focused = leaf
	}
}

// RemovePane drops a pane that is not the tab's last, moving focus only when
// the removed pane had it.
func (t *Tab[L]) RemovePane(leaf L) {
	next, ok := t.Panes.Remove(leaf)
	if !ok || t.focused != leaf {
		return
	}
	t.focused = next
}

// DisplayTitle is the pane's own title, or else where it is.
func DisplayTitle[L PaneLeaf](leaf L) string {
	if title := leaf.LeafTitle(); title != "" {
		return title
	}
	if dir := leaf.LeafDir(); dir != "" {
		return AbbreviateHome(dir)
	}
	return "Terminal"
}

// Workspace is one project's tabs.
type Workspace[L PaneLeaf] struct {
	ID      string
	Accent  int
	Name    string
	HasName bool
	Tabs    []*Tab[L]
	// selected is the tab in view.
	selected *Tab[L]
}

// NewWorkspace makes an empty workspace.
func NewWorkspace[L PaneLeaf](id string, customName *string, accent int) *Workspace[L] {
	w := &Workspace[L]{ID: id, Accent: accent}
	if customName != nil {
		w.Name = *customName
		w.HasName = true
	}
	return w
}

// SelectedTab returns the tab in view.
func (w *Workspace[L]) SelectedTab() *Tab[L] { return w.selected }

// DisplayName is the chosen name, or the focused pane's folder.
func (w *Workspace[L]) DisplayName() string {
	if w.HasName {
		return w.Name
	}
	if w.selected != nil {
		if dir := w.selected.Focused().LeafDir(); dir != "" {
			return FolderName(dir)
		}
	}
	return "~"
}

// Add appends a tab and selects it.
func (w *Workspace[L]) Add(tab *Tab[L]) {
	w.Tabs = append(w.Tabs, tab)
	w.selected = tab
}

// Remove drops a tab, selecting a neighbour.
func (w *Workspace[L]) Remove(tab *Tab[L]) {
	index := -1
	for i, t := range w.Tabs {
		if t == tab {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	w.Tabs = append(w.Tabs[:index], w.Tabs[index+1:]...)
	if w.selected == tab {
		if len(w.Tabs) == 0 {
			w.selected = nil
		} else {
			w.selected = w.Tabs[min(index, len(w.Tabs)-1)]
		}
	}
}

// Select chooses a tab already in the workspace.
func (w *Workspace[L]) Select(tab *Tab[L]) {
	for _, t := range w.Tabs {
		if t == tab {
			w.selected = tab
			return
		}
	}
}

// TabContaining returns the tab holding a pane, or nil.
func (w *Workspace[L]) TabContaining(leaf L) *Tab[L] {
	for _, t := range w.Tabs {
		if t.Panes.Contains(leaf) {
			return t
		}
	}
	return nil
}

// TabCloseMode names which tabs a close request closes.
type TabCloseMode int

const (
	// CloseThis closes only the tab the request came from.
	CloseThis TabCloseMode = iota
	// CloseOthers closes every other tab in the workspace.
	CloseOthers
	// CloseRight closes the tabs to the right.
	CloseRight
)

// ClosingTabs returns the tabs a close request would close.
func (w *Workspace[L]) ClosingTabs(mode TabCloseMode, from *Tab[L]) []*Tab[L] {
	index := -1
	for i, t := range w.Tabs {
		if t == from {
			index = i
			break
		}
	}
	if index < 0 {
		return nil
	}
	switch mode {
	case CloseThis:
		return []*Tab[L]{from}
	case CloseOthers:
		var out []*Tab[L]
		for _, t := range w.Tabs {
			if t != from {
				out = append(out, t)
			}
		}
		return out
	case CloseRight:
		return append([]*Tab[L]{}, w.Tabs[index+1:]...)
	}
	return nil
}

// Store is every workspace in a window plus which one is selected.
type Store[L PaneLeaf] struct {
	Workspaces []*Workspace[L]
	selected   *Workspace[L]
	// RecentlyFocused lists panes by when they last had focus, most recent
	// first.
	RecentlyFocused []string
	// OnChange runs after each mutation, or once at the end of a batch.
	OnChange func()

	batchDepth   int
	pendingBatch bool
}

// NewStore makes an empty store.
func NewStore[L PaneLeaf]() *Store[L] { return &Store[L]{} }

// Selected returns the workspace in view.
func (s *Store[L]) Selected() *Workspace[L] { return s.selected }

// AddWorkspace inserts a workspace after the selected one and selects it.
func (s *Store[L]) AddWorkspace(id string, customName *string, accent int) *Workspace[L] {
	if accent < 0 {
		accent = len(s.Workspaces)
	}
	w := NewWorkspace[L](id, customName, accent%8)
	index := len(s.Workspaces)
	for i, existing := range s.Workspaces {
		if existing == s.selected {
			index = i + 1
			break
		}
	}
	s.Workspaces = append(s.Workspaces, nil)
	copy(s.Workspaces[index+1:], s.Workspaces[index:])
	s.Workspaces[index] = w
	s.selected = w
	s.notify()
	return w
}

// Remove drops a workspace.
func (s *Store[L]) Remove(w *Workspace[L]) {
	index := -1
	for i, existing := range s.Workspaces {
		if existing == w {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	s.Workspaces = append(s.Workspaces[:index], s.Workspaces[index+1:]...)
	if s.selected == w {
		if len(s.Workspaces) == 0 {
			s.selected = nil
		} else {
			s.selected = s.Workspaces[min(index, len(s.Workspaces)-1)]
		}
	}
	s.notify()
}

// Select chooses a workspace.
func (s *Store[L]) Select(w *Workspace[L]) {
	s.selected = w
	s.notify()
}

// Move reorders a workspace to an insertion index.
func (s *Store[L]) Move(w *Workspace[L], index int) {
	source := -1
	for i, existing := range s.Workspaces {
		if existing == w {
			source = i
			break
		}
	}
	if source < 0 {
		return
	}
	if index > source {
		index--
	}
	if index < 0 {
		index = 0
	}
	if index > len(s.Workspaces)-1 {
		index = len(s.Workspaces) - 1
	}
	if source == index {
		return
	}
	s.Workspaces = append(s.Workspaces[:source], s.Workspaces[source+1:]...)
	s.Workspaces = append(s.Workspaces, nil)
	copy(s.Workspaces[index+1:], s.Workspaces[index:])
	s.Workspaces[index] = w
	s.notify()
}

// NoteFocus records a pane as most recently focused, dropping the ids of
// terminals that have closed.
func (s *Store[L]) NoteFocus(leaf L) {
	open := map[string]bool{}
	for _, w := range s.Workspaces {
		for _, tab := range w.Tabs {
			for _, l := range tab.Panes.Leaves() {
				open[l.LeafID()] = true
			}
		}
	}
	id := leaf.LeafID()
	kept := make([]string, 0, len(s.RecentlyFocused)+1)
	kept = append(kept, id)
	for _, existing := range s.RecentlyFocused {
		if existing != id && open[existing] {
			kept = append(kept, existing)
		}
	}
	s.RecentlyFocused = kept
}

// WorkspaceContaining returns the workspace and tab holding a pane.
func (s *Store[L]) WorkspaceContaining(leaf L) (*Workspace[L], *Tab[L], bool) {
	for _, w := range s.Workspaces {
		if tab := w.TabContaining(leaf); tab != nil {
			return w, tab, true
		}
	}
	return nil, nil, false
}

// Panes returns every pane in the store.
func (s *Store[L]) Panes() []L {
	var out []L
	for _, w := range s.Workspaces {
		for _, tab := range w.Tabs {
			out = append(out, tab.Panes.Leaves()...)
		}
	}
	return out
}

// Notify runs the change callback.
func (s *Store[L]) Notify() { s.notify() }

func (s *Store[L]) notify() {
	if s.batchDepth > 0 {
		s.pendingBatch = true
		return
	}
	if s.OnChange != nil {
		s.OnChange()
	}
}

// Batch coalesces changes into one notification.
func (s *Store[L]) Batch(fn func()) {
	s.batchDepth++
	fn()
	s.batchDepth--
	if s.batchDepth == 0 && s.pendingBatch {
		s.pendingBatch = false
		s.notify()
	}
}

// AbbreviateHome turns "/Users/me/Code" into "~/Code".
func AbbreviateHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):]
	}
	return path
}

// FolderName is the last path component, with "~" and "/" kept as they are.
func FolderName(path string) string {
	abbreviated := AbbreviateHome(path)
	if abbreviated == "~" || abbreviated == "/" {
		return abbreviated
	}
	base := filepath.Base(abbreviated)
	if base == "." || base == string(filepath.Separator) {
		return abbreviated
	}
	return base
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
