package app

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/notes/theme"
	"github.com/phucngodev/code/internal/notes/workspace"
)

// tree is the sidebar: the workspace's folders and notes, read as they are
// opened.
type tree struct {
	a        *App
	expanded map[string]bool
	children map[string][]treeEntry
	rows     []treeRow
	list     ui.ListState
	// revealing is a note to scroll to once the tree shows it.
	revealing string
}

type treeEntry struct {
	path string
	dir  bool
}

type treeRow struct {
	kind  int
	path  string
	dir   bool
	depth int
}

const (
	rowRoot = iota
	rowEntry
)

func newTree(a *App) *tree {
	t := &tree{a: a, expanded: map[string]bool{}, children: map[string][]treeEntry{}}
	t.list.Key = func(i int) any {
		if i >= len(t.rows) {
			return i
		}
		return t.rows[i].path
	}
	return t
}

// entries are the folders and Markdown notes of dir, folders first, each
// sorted as a person reads names.
func (t *tree) entries(dir string) []treeEntry {
	if e, ok := t.children[dir]; ok {
		return e
	}
	folders, files := workspace.Folders(dir)
	cmp := func(a, b string) int { return workspace.NaturalCompare(filepath.Base(a), filepath.Base(b)) }
	slices.SortFunc(folders, cmp)
	slices.SortFunc(files, cmp)
	var out []treeEntry
	for _, f := range folders {
		out = append(out, treeEntry{f, true})
	}
	for _, f := range files {
		if workspace.IsMarkdown(f) {
			out = append(out, treeEntry{f, false})
		}
	}
	t.children[dir] = out
	return out
}

// folderChanged forgets what it read of a folder that changed.
func (t *tree) folderChanged(dir string) {
	delete(t.children, dir)
	for k := range t.children {
		if realPath(k) == realPath(dir) {
			delete(t.children, k)
		}
	}
}

// reveal opens the folders that hold path, so the tree shows the note the
// editor does.
func (t *tree) reveal(path string) {
	t.revealing = path
	for dir := filepath.Dir(path); len(dir) > len(t.a.root) && strings.HasPrefix(dir, t.a.root+string(filepath.Separator)); dir = filepath.Dir(dir) {
		t.expanded[dir] = true
	}
}

func (t *tree) view(c *ui.Context, th theme.Theme) {
	ui.Column(c.Key("sidebar")).Fill().Background(th.Panel).BorderWidth(0, 1, 0, 0).BorderColor(th.Border).Children(func() {
		ui.Row(c.Key("sidebar-top")).Height(38).Padding(0, 10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, t.a.name).FontSize(12.5).Bold().SingleLine().TextColor(th.Text).Grow(1)
			if iconButton(c, "hide-sidebar", iconSidebar, "Hide Sidebar", th).Clicked() {
				t.a.sidebarHidden = true
			}
		})
		t.flatten()
		ui.List(c.Key("tree"), &t.list, len(t.rows), func(i int) {
			r := t.rows[i]
			if r.kind == rowRoot {
				t.row(c, th, t.a.root, true, 0, true)
				return
			}
			t.row(c, th, r.path, r.dir, r.depth, false)
		}).Grow(1).Gap(1).Padding(4, 6)
	})
}

// flatten lists the rows the tree shows: the workspace, then the folders and
// notes, those of each open folder under it.
func (t *tree) flatten() {
	t.rows = append(t.rows[:0], treeRow{kind: rowRoot})
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		for _, e := range t.entries(dir) {
			t.rows = append(t.rows, treeRow{kind: rowEntry, path: e.path, dir: e.dir, depth: depth})
			if e.dir && t.expanded[e.path] {
				walk(e.path, depth+1)
			}
		}
	}
	walk(t.a.root, 0)
	if t.revealing != "" {
		for i, r := range t.rows {
			if r.kind == rowEntry && r.path == t.revealing {
				t.list.ScrollIntoView(i)
				break
			}
		}
		t.revealing = ""
	}
}

// row is a row of the tree: a folder that opens when chosen, or a note that
// shows in the editor.
func (t *tree) row(c *ui.Context, th theme.Theme, path string, dir bool, depth int, isRoot bool) {
	selected := false
	if dir {
		selected = isRoot
	} else if cur := t.a.tab(); cur != nil {
		selected = cur.path == path
	}
	r := ui.Row(c.Key(path)).Height(28).Padding(0, 8, 0, 6+float32(depth)*16).Gap(6).Radius(7).AlignItems(ui.Center).Cursor(ui.CursorDefault)
	switch {
	case selected && !dir:
		r.Background(th.Accent.Alpha(0.16))
	case r.Hovered():
		r.Background(th.Text.Alpha(0.045))
	}
	name := filepath.Base(path)
	if isRoot {
		name = "All Notes"
	} else if !dir {
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	if r.Clicked() {
		if dir {
			if !isRoot {
				t.expanded[path] = !t.expanded[path]
			}
		} else {
			t.a.open(path, false)
		}
	}
	if r.DoubleClicked() && !dir {
		t.a.open(path, true)
	}
	r.Children(func() {
		switch {
		case dir && !isRoot:
			open := t.expanded[path]
			ch := ui.Box(c.Key("chevron")).Size(14, 14).Center()
			if ch.Clicked() {
				t.expanded[path] = !open
			}
			ch.Children(func() {
				ic := ui.Icon(c, iconChevron).FontSize(11).TextColor(th.Secondary)
				if !open {
					ic.Rotate(-90)
				}
			})
		case !isRoot:
			ui.Box(c).Width(14)
		}
		ic := iconNote
		if isRoot {
			ic = iconLibrary
		} else if dir {
			ic = iconFolder
		}
		color := th.Secondary
		if selected || isRoot {
			color = th.Accent
		}
		ui.Icon(c, ic).FontSize(15).TextColor(color)
		label := ui.Text(c, name).SingleLine().FontSize(13).Grow(1).TextColor(th.Text)
		if selected {
			label.FontWeight(600)
		}
	})
}

// realPath is a path with its symlinks resolved, or itself.
func realPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}
