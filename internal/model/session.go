package model

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Session is what survives a relaunch: each window's workspaces, their tabs'
// layouts and working directories, the ids of workspaces and terminals, and
// which of each is selected. Shell processes and screen contents do not.
type Session struct {
	Windows []WindowSession `json:"windows"`
	// SidebarHidden is whether the workspace sidebar was collapsed, and
	// SidebarWidth its width, shared by every window.
	SidebarHidden *bool   `json:"sidebarHidden,omitempty"`
	SidebarWidth  float32 `json:"sidebarWidth,omitempty"`
}

// WindowSession is one window's saved workspaces.
type WindowSession struct {
	Workspaces []WorkspaceSession `json:"workspaces"`
	Selected   int                `json:"selectedWorkspace"`
	Frame      *Frame             `json:"frame,omitempty"`
}

// WorkspaceSession is one workspace's saved tabs.
type WorkspaceSession struct {
	ID         string       `json:"id"`
	CustomName *string      `json:"customName,omitempty"`
	Accent     *int         `json:"accent,omitempty"`
	Tabs       []TabSession `json:"tabs"`
	Selected   int          `json:"selectedTab"`
}

// TabSession is one tab's saved layout.
type TabSession struct {
	Layout     *Layout `json:"layout"`
	ZoomedPane *int    `json:"zoomedPane,omitempty"`
}

// Layout is a pane tree as saved: one terminal, or a split of two layouts.
type Layout struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
	Dir  string `json:"workingDirectory,omitempty"`
	// Command is the program a terminal was opened to resume, so a session
	// reopened after a relaunch runs the same agent.
	Command string  `json:"command,omitempty"`
	Axis    Axis    `json:"axis,omitempty"`
	Ratio   float64 `json:"ratio,omitempty"`
	First   *Layout `json:"first,omitempty"`
	Second  *Layout `json:"second,omitempty"`
}

// Frame is a window's saved position and size.
type Frame struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// LayoutFrom snapshots a pane node.
func LayoutFrom[L PaneLeaf](n *Node[L]) *Layout {
	if n.HasLeaf {
		return &Layout{Kind: "terminal", ID: n.Leaf.LeafID(), Dir: n.Leaf.LeafDir()}
	}
	return &Layout{
		Kind:   "split",
		Axis:   n.Axis,
		Ratio:  float64(n.Pct) / 100,
		First:  LayoutFrom(n.First),
		Second: LayoutFrom(n.Second),
	}
}

// Build rebuilds a pane node, asking makeLeaf for each terminal.
func (l *Layout) Build[L PaneLeaf](makeLeaf func(id, dir string) L) *Node[L] {
	if l == nil {
		return nil
	}
	if l.Kind == "terminal" || l.First == nil || l.Second == nil {
		return NewLeaf(makeLeaf(l.ID, l.Dir))
	}
	pct := float32(l.Ratio * 100)
	return NewSplit(l.Axis, l.First.Build(makeLeaf), l.Second.Build(makeLeaf), pct)
}

// Snapshot records a window's workspaces.
func (s *Store[L]) Snapshot(frame *Frame) WindowSession {
	ws := make([]WorkspaceSession, 0, len(s.Workspaces))
	selected := 0
	for i, w := range s.Workspaces {
		tabs := make([]TabSession, 0, len(w.Tabs))
		selectedTab := 0
		for j, tab := range w.Tabs {
			zoomed := tab.Panes.ZoomedIndex()
			var zoomedPtr *int
			if zoomed >= 0 {
				zoomedPtr = &zoomed
			}
			tabs = append(tabs, TabSession{
				Layout:     LayoutFrom(tab.Panes.Root),
				ZoomedPane: zoomedPtr,
			})
			if tab == w.selected {
				selectedTab = j
			}
		}
		var customName *string
		if w.HasName {
			name := w.Name
			customName = &name
		}
		accent := w.Accent
		ws = append(ws, WorkspaceSession{
			ID:         w.ID,
			CustomName: customName,
			Accent:     &accent,
			Tabs:       tabs,
			Selected:   selectedTab,
		})
		if w == s.selected {
			selected = i
		}
	}
	return WindowSession{Workspaces: ws, Selected: selected, Frame: frame}
}

// Restore rebuilds a window's workspaces. makeLeaf recreates a terminal from
// its saved id and working directory, given its workspace id. A workspace
// whose tabs were all lost is skipped.
func (s *Store[L]) Restore(session WindowSession, newID func() string, makeLeaf func(id, dir, workspace string) L) {
	s.Batch(func() {
		var selected *Workspace[L]
		for wi, saved := range session.Workspaces {
			w := s.AddWorkspace(saved.ID, saved.CustomName, accentOr(saved.Accent, wi))
			var selectedTab *Tab[L]
			for ti, tab := range saved.Tabs {
				root := tab.Layout.Build(func(id, dir string) L {
					if id == "" {
						id = newID()
					}
					return makeLeaf(id, dir, w.ID)
				})
				if root == nil {
					continue
				}
				leaves := root.Leaves()
				if len(leaves) == 0 {
					continue
				}
				panes := NewTreeFrom(root)
				focused := leaves[0]
				if tab.ZoomedPane != nil && *tab.ZoomedPane >= 0 && *tab.ZoomedPane < len(leaves) {
					focused = leaves[*tab.ZoomedPane]
					panes.ToggleZoom(focused)
				}
				restored := NewTabFrom(newID(), panes, focused)
				w.Add(restored)
				if ti == saved.Selected {
					selectedTab = restored
				}
			}
			if selectedTab != nil {
				w.Select(selectedTab)
			}
			if len(w.Tabs) == 0 {
				s.Remove(w)
			} else if wi == session.Selected {
				selected = w
			}
		}
		if selected != nil {
			s.selected = selected
		}
	})
}

func accentOr(accent *int, fallback int) int {
	if accent != nil {
		return *accent
	}
	return fallback
}

// LoadSession reads a session file. A missing or malformed file loads as none.
func LoadSession(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// Save writes the session as pretty JSON. A unique temporary name keeps two
// Code instances from writing the same file, and 0600 keeps the saved paths
// private.
func (s *Session) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".code-session-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
