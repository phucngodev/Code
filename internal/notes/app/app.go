// Package app draws a workspace of Markdown notes as a view inside a Code
// tab: a file tree, the note tabs, the editor and the status bar. It is a
// port of Deckle's window, reusing its editor and Markdown engine.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/notes/editor"
	"github.com/phucngodev/code/internal/notes/markdown"
	"github.com/phucngodev/code/internal/notes/theme"
	"github.com/phucngodev/code/internal/notes/workspace"
)

// Host is what the view asks of the window that shows it.
type Host interface {
	// Window is the window the view is drawn in, for dialogs.
	Window() *mygo.Window
	// Update runs fn on the main thread and draws a frame after it.
	Update(fn func())
}

// App is the notes view: a workspace of Markdown files and its editor.
type App struct {
	host    Host
	root    string
	name    string
	ws      *workspace.Workspace
	tabs    []*tab
	sel     int
	tree    *tree
	palette *palette
	// words is the shown note's word count, worked out off the frame.
	words int
	// sidebarHidden hides the file tree; sideWidth is its width in points.
	sidebarHidden bool
	sideWidth     float32
	focusEditor   bool
	mu            sync.Mutex
	queue         []func()
	// closed stops the workspace and makes later posts no-ops, so a closed
	// view releases its memory and its timers cannot run on it.
	closed bool
}

// tab is a note shown: its file and its editor, read when first shown.
type tab struct {
	path    string
	ed      *editor.Editor
	loadErr string
	modTime time.Time
	saveAt  time.Time
}

// New opens the workspace at root and shows the first note found.
func New(host Host, root string) *App {
	a := &App{host: host, root: root, name: filepath.Base(root), tabs: []*tab{{}}, sideWidth: 220}
	a.ws = workspace.Open(root, func(e workspace.Event) { a.post(func() { a.workspaceEvent(e) }) })
	a.tree = newTree(a)
	a.palette = &palette{a: a}
	a.focusEditor = true
	return a
}

// Root is the folder the view shows.
func (a *App) Root() string { return a.root }

// Close stops the workspace's watch and drops everything the view held, so a
// closed Notes tab releases its memory. Safe to call more than once.
func (a *App) Close() {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	a.closed = true
	ws := a.ws
	a.ws, a.tabs, a.tree, a.palette, a.queue = nil, nil, nil, nil, nil
	a.mu.Unlock()
	if ws != nil {
		ws.Close()
	}
	editor.ClearImageCache()
}

// Closed reports whether the view was closed, so a caller can check it
// released its memory.
func (a *App) Closed() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.closed
}

// SaveAll writes every note with unsaved changes.
func (a *App) SaveAll() {
	for _, t := range a.tabs {
		a.save(t)
	}
}

// ToggleSidebar shows or hides the file tree.
func (a *App) ToggleSidebar() { a.sidebarHidden = !a.sidebarHidden }

// NewNote makes an empty note in the workspace and opens it.
func (a *App) NewNote() {
	p := freeName(a.root, "Untitled", "md")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		return
	}
	a.ws.Touch(p)
	a.tree.folderChanged(a.root)
	a.open(p, true)
}

// OpenNote shows the note at path, reading it if it is not open yet.
func (a *App) OpenNote(path string) { a.open(path, true) }

// NextTab and PrevTab move through the notes open.
func (a *App) NextTab() { a.selectTab((a.sel + 1) % len(a.tabs)) }
func (a *App) PrevTab() { a.selectTab((a.sel + len(a.tabs) - 1) % len(a.tabs)) }

// CloseTab closes the note shown.
func (a *App) CloseTab() { a.closeTab(a.sel) }

// post runs fn on the main thread before the next frame.
func (a *App) post(fn func()) {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	a.queue = append(a.queue, fn)
	a.mu.Unlock()
	if a.host != nil {
		a.host.Update(func() {})
	}
}

func (a *App) runQueue() {
	for {
		a.mu.Lock()
		q := a.queue
		a.queue = nil
		a.mu.Unlock()
		if len(q) == 0 {
			return
		}
		for _, f := range q {
			f()
		}
	}
}

func (a *App) tab() *tab {
	if a.sel >= 0 && a.sel < len(a.tabs) {
		return a.tabs[a.sel]
	}
	return nil
}

func (a *App) editor() *editor.Editor {
	if t := a.tab(); t != nil {
		return t.ed
	}
	return nil
}

// View draws the view: the file tree, the note tabs, the editor and the
// status bar, in the terminal's theme.
func (a *App) View(c *ui.Context, th theme.Theme) {
	if a.closed {
		return
	}
	a.runQueue()
	useTheme(c, th)
	look := a.look(th)
	a.shortcuts(c)
	root := ui.Box(c).Fill().Background(th.Background).TextColor(th.Text)
	root.Children(func() {
		ui.Row(c.Key("notes")).Fill().AlignItems(ui.Stretch).Children(func() {
			if a.sidebarHidden {
				a.pane(c, th, look)
				return
			}
			ui.Split(c.Key("split-side"), &a.sideWidth, func() { a.tree.view(c, th) }, func() { a.pane(c, th, look) }).Fill()
		})
		a.palette.view(c, th)
	})
}

// shortcuts are the keys the notes view takes while it is in front.
func (a *App) shortcuts(c *ui.Context) {
	if a.palette.open {
		if c.Shortcut(0, ui.KeyEscape) {
			a.palette.close()
		}
		return
	}
	if c.Shortcut(openMods, ui.KeyO) {
		a.post(func() { a.palette.show(modeFiles, "") })
	}
	if c.Shortcut(commandMods, ui.KeyP) {
		a.post(func() { a.palette.show(modeCommands, "") })
	}
	if c.Shortcut(searchMods, ui.KeyF) {
		a.post(func() { a.palette.show(modeSearch, "") })
	}
}

// look is how the editor shows text.
func (a *App) look(th theme.Theme) editor.Look {
	return editor.Look{
		Theme:        th,
		Fonts:        theme.Fonts{Size: 15},
		LineHeight:   1.6,
		LineWidth:    0.75,
		HidesMarkers: true,
	}
}

// useTheme gives the controls of a view the colors of th.
func useTheme(c *ui.Context, th theme.Theme) {
	t := *c.Theme()
	t.Accent = th.Accent
	t.AccentHover = th.Accent.Mix(th.Text, 0.12)
	t.AccentPressed = th.Accent.Mix(th.Text, 0.24)
	t.AccentText = th.OnAccent()
	t.Focus = th.Accent.Alpha(0.5)
	t.Selection = th.Selection
	t.Background = th.Background
	t.Text = th.Text
	t.TextMuted = th.Secondary
	c.SetTheme(&t)
}

// pane is the tabs, the toolbar, the editor shown and the status bar.
func (a *App) pane(c *ui.Context, th theme.Theme, look editor.Look) {
	ui.Column(c.Key("pane")).Fill().Background(th.Background).Children(func() {
		a.tabBar(c, th)
		a.toolbar(c, th)
		ui.Box(c.Key("content")).Grow(1).Basis(0).MinHeight(0).Clip().Children(func() {
			a.content(c, th, look)
		})
		a.statusBar(c, th)
	})
}

// tabBar is the row of note tabs over the panel, a new-note button and the
// search and sidebar toggles. The note shown wears the page's own color, so
// it reads as connected to the editor under it.
func (a *App) tabBar(c *ui.Context, th theme.Theme) {
	ui.Row(c.Key("notetabs")).Height(40).Padding(0, 8).Gap(4).AlignItems(ui.Center).
		Background(th.Background).BorderWidth(0, 0, 1, 0).BorderColor(th.Border).Children(func() {
		if a.sidebarHidden {
			if iconButton(c, "sidebar", iconSidebar, "Show Sidebar", th).Clicked() {
				a.sidebarHidden = false
			}
		}
		ui.Row(c.Key("tabs")).Grow(1).Gap(3).AlignItems(ui.Center).MinWidth(0).Children(func() {
			closeAt := -1
			for i, t := range a.tabs {
				title := "New Tab"
				if t.path != "" {
					title = tabTitle(t)
				}
				el := ui.Row(c.Key(fmt.Sprintf("tab-%p", t))).Grow(1).MaxWidth(240).MinWidth(60).Height(30).Radius(8).Padding(0, 9).Gap(6).AlignItems(ui.Center).
					Cursor(ui.CursorDefault).Tooltip(t.path)
				hovered := el.Hovered()
				switch {
				case i == a.sel:
					el.Background(th.Panel).Border(1, th.Border)
				case hovered:
					el.Background(th.Text.Alpha(0.05))
				}
				if el.Clicked() {
					a.selectTab(i)
				}
				el.Children(func() {
					if i == a.sel {
						ui.Box(c).Absolute().Left(7).Right(7).Bottom(0).Height(2).Radius(1).Background(th.Accent)
					}
					color := th.Secondary
					if i == a.sel {
						color = th.Text
					}
					ui.Icon(c, iconNote).FontSize(13).TextColor(color)
					label := ui.Text(c, title).SingleLine().Grow(1).FontSize(12.5).TextColor(color)
					if i == a.sel {
						label.FontWeight(600)
					}
					if hovered || i == a.sel {
						x := ui.Box(c.Key("x")).Size(16, 16).Radius(4).Center()
						if x.Hovered() {
							x.Background(th.Text.Alpha(0.1))
						}
						if x.Clicked() {
							closeAt = i
						}
						x.Children(func() { ui.Icon(c, iconClose).FontSize(10).TextColor(th.Secondary) })
					}
				})
			}
			if closeAt >= 0 {
				i := closeAt
				a.post(func() { a.closeTab(i) })
			}
		})
		if iconButton(c, "newnote", iconNewNote, "New Note", th).Clicked() {
			a.post(a.NewNote)
		}
		if iconButton(c, "commands", iconCommand, "Commands", th).Clicked() {
			a.post(func() { a.palette.show(modeCommands, "") })
		}
		if iconButton(c, "searchnotes", iconSearch, "Search Notes", th).Clicked() {
			a.post(func() { a.palette.show(modeSearch, "") })
		}
	})
}

// toolbar is the row of editing buttons above the note.
func (a *App) toolbar(c *ui.Context, th theme.Theme) {
	ed := a.editor()
	if ed == nil || !ed.IsMarkdown() {
		return
	}
	ui.Row(c.Key("toolbar")).Height(36).Padding(0, 10).Gap(2).AlignItems(ui.Center).Children(func() {
		a.toolButton(c, "bold", "B", "toggleBold", ed, th)
		a.toolButton(c, "italic", "I", "toggleItalic", ed, th)
		a.toolButton(c, "strike", "S", "toggleStrikethrough", ed, th)
		a.toolButton(c, "mark", "H", "toggleHighlight", ed, th)
		a.toolButton(c, "code", "</>", "toggleInlineCode", ed, th)
		a.toolButton(c, "link", "Link", "insertLink", ed, th)
		ui.Box(c).Width(8)
		a.toolButton(c, "quote", "Quote", "toggleQuote", ed, th)
		a.toolButton(c, "bullet", "• List", "toggleBulletList", ed, th)
		a.toolButton(c, "number", "1. List", "toggleNumberedList", ed, th)
		a.toolButton(c, "task", "Task", "toggleTaskList", ed, th)
		a.toolButton(c, "block", "Code Block", "toggleCodeBlock", ed, th)
	})
}

func (a *App) toolButton(c *ui.Context, key, label, id string, ed *editor.Editor, th theme.Theme) {
	on := ed.IsOn(id)
	b := ui.Box(c.Key("tool-"+key)).Height(26).Padding(0, 8).Radius(6).Center().FocusRing(false).Cursor(ui.CursorDefault)
	switch {
	case on:
		b.Background(th.Accent.Alpha(0.18))
	case b.Hovered():
		b.Background(th.Text.Alpha(0.06))
	}
	color := th.Text
	if on {
		color = th.Accent
	}
	if !ed.CanRun(id) {
		color = th.Secondary.Alpha(0.5)
	}
	if b.Clicked() {
		ed.Run(id)
	}
	b.Children(func() { ui.Text(c, label).FontSize(12.5).TextColor(color) })
}

// content is the editor of the note shown, or what to do with none.
func (a *App) content(c *ui.Context, th theme.Theme, look editor.Look) {
	t := a.tab()
	switch {
	case t == nil || t.path == "":
		a.emptyTab(c, th)
	case t.loadErr != "":
		ui.Column(c).Fill().Center().Gap(6).Children(func() {
			ui.Text(c, "This file can't be opened").Bold()
			ui.Text(c, t.loadErr).TextColor(th.Secondary).FontSize(12)
		})
	case t.ed != nil:
		el := t.ed.View(c.Key(fmt.Sprintf("ed-%p", t)), look).Fill()
		if a.focusEditor {
			el.Focus()
			a.focusEditor = false
		}
	}
}

// emptyTab is what a view with no note shows.
func (a *App) emptyTab(c *ui.Context, th theme.Theme) {
	ui.Column(c).Fill().Center().Padding(24).Gap(10).Children(func() {
		ui.Box(c).Size(52, 52).Radius(14).Center().Background(th.Accent.Alpha(0.12)).Children(func() {
			ui.Icon(c, iconNote).FontSize(24).TextColor(th.Accent)
		})
		ui.Text(c, "No Note Open").FontSize(16).Bold().TextColor(th.Text)
		ui.Text(c, a.name).FontSize(12.5).TextColor(th.Secondary)
		b := ui.ButtonBase(c.Key("newnote")).Label("New Note").Height(32).Padding(0, 14).Radius(8)
		if b.Clicked() {
			a.post(a.NewNote)
		}
	})
}

// statusBar shows where the caret is and how many words the note has.
func (a *App) statusBar(c *ui.Context, th theme.Theme) {
	t := a.tab()
	ui.Row(c.Key("status")).Height(26).Padding(0, 16).Gap(16).AlignItems(ui.Center).Children(func() {
		if t != nil && t.ed != nil {
			line, col := t.ed.CaretLine()
			ui.Text(c, fmt.Sprintf("Ln %d, Col %d", line, col)).FontSize(11.5).TextColor(th.Secondary)
		}
		ui.Box(c).Grow(1)
		if t != nil && t.ed != nil && t.ed.IsMarkdown() {
			label := fmt.Sprintf("%d words", a.words)
			if a.words == 1 {
				label = "1 word"
			}
			ui.Text(c, label).FontSize(11.5).TextColor(th.Secondary)
		}
	})
}

// iconButton is a small square button with an icon, for the bars.
func iconButton(c *ui.Context, key string, svg *ui.SVG, tip string, th theme.Theme) ui.Element {
	b := ui.Box(c.Key(key)).Size(28, 28).Radius(7).Center().Focusable().FocusRing(false).Cursor(ui.CursorDefault).Tooltip(tip)
	if b.Hovered() {
		b.Background(th.Text.Alpha(0.07))
	}
	b.Children(func() { ui.Icon(c, svg).FontSize(16).TextColor(th.Secondary) })
	return b
}

func tabTitle(t *tab) string {
	if t.path == "" {
		return "New Tab"
	}
	base := filepath.Base(t.path)
	if workspace.IsMarkdown(t.path) {
		return strings.TrimSuffix(base, filepath.Ext(base))
	}
	return base
}

// open shows the note at path, in the tab shown or a new one, reading it
// first if it is not open.
func (a *App) open(path string, focus bool) {
	path = filepath.Clean(path)
	for i, t := range a.tabs {
		if t.path == path {
			a.selectTab(i)
			return
		}
	}
	cur := a.tab()
	t := &tab{path: path}
	if cur != nil && cur.path == "" {
		a.tabs[a.sel] = t
	} else {
		a.tabs = append(a.tabs, t)
		a.sel = len(a.tabs) - 1
	}
	a.load(t)
	if focus {
		a.focusEditor = true
	}
	a.tree.reveal(path)
	a.countLater()
}

// openAt shows a note and puts the caret on a place in it, as a search
// result does.
func (a *App) openAt(path string, sel [2]int) {
	a.open(path, true)
	if t := a.tab(); t != nil && t.ed != nil {
		t.ed.SetSelection(sel[0], sel[1])
		t.ed.RevealCentered()
	}
}

func (a *App) selectTab(i int) {
	if i < 0 || i >= len(a.tabs) || i == a.sel {
		return
	}
	if cur := a.tab(); cur != nil {
		a.save(cur)
	}
	a.sel = i
	a.load(a.tab())
	a.focusEditor = true
	a.countLater()
}

func (a *App) closeTab(i int) {
	if i < 0 || i >= len(a.tabs) {
		return
	}
	t := a.tabs[i]
	a.save(t)
	if len(a.tabs) == 1 {
		a.tabs[0] = &tab{}
		a.countLater()
		return
	}
	a.tabs = append(a.tabs[:i], a.tabs[i+1:]...)
	if a.sel >= i && a.sel > 0 {
		a.sel--
	}
	a.load(a.tab())
	a.focusEditor = true
	a.countLater()
}

// load reads a tab's file into a new editor.
func (a *App) load(t *tab) {
	if t == nil || t.path == "" || t.ed != nil {
		return
	}
	data, err := os.ReadFile(t.path)
	if err != nil {
		t.loadErr = err.Error()
		return
	}
	t.loadErr = ""
	if st, err := os.Stat(t.path); err == nil {
		t.modTime = st.ModTime()
	}
	lang := markdown.LanguageForExtension(strings.TrimPrefix(filepath.Ext(t.path), "."))
	ed := editor.New(strings.TrimPrefix(string(data), "\xef\xbb\xbf"), lang, t.path)
	ed.Post = func(f func()) { a.post(f) }
	ed.Hooks = editor.Hooks{Changed: func() { a.edited(t) }}
	t.ed = ed
}

// edited saves the note a moment after typing stops.
func (a *App) edited(t *tab) {
	t.saveAt = time.Now().Add(600 * time.Millisecond)
	time.AfterFunc(650*time.Millisecond, func() {
		a.post(func() {
			if !t.saveAt.IsZero() && time.Now().After(t.saveAt) {
				a.save(t)
			}
		})
	})
	a.countLater()
}

// save writes a tab's note if it changed.
func (a *App) save(t *tab) {
	t.saveAt = time.Time{}
	if t == nil || t.ed == nil || !t.ed.Modified() {
		return
	}
	if err := writeFile(t.path, t.ed.Text()); err != nil {
		return
	}
	t.ed.MarkSaved()
	if st, err := os.Stat(t.path); err == nil {
		t.modTime = st.ModTime()
	}
	if a.ws != nil {
		a.ws.Touch(t.path)
	}
}

// countLater works out the word count off the frame.
func (a *App) countLater() {
	time.AfterFunc(300*time.Millisecond, func() {
		a.post(func() {
			if ed := a.editor(); ed != nil {
				a.words = ed.WordCount()
			} else {
				a.words = 0
			}
		})
	})
}

// workspaceEvent takes in a change to the files on disk.
func (a *App) workspaceEvent(e workspace.Event) {
	a.palette.stale()
	switch e.Kind {
	case workspace.EventFolder:
		a.tree.folderChanged(e.Path)
	case workspace.EventFile:
		for _, t := range a.tabs {
			if t.ed != nil && t.path == e.Path {
				if st, err := os.Stat(t.path); err == nil && !st.ModTime().Equal(t.modTime) && !t.ed.Modified() {
					if data, err := os.ReadFile(t.path); err == nil {
						t.modTime = st.ModTime()
						t.ed.SetText(strings.TrimPrefix(string(data), "\xef\xbb\xbf"))
						a.countLater()
					}
				}
			}
		}
	}
}

// writeFile writes text to path through a temporary file beside it, so a
// failure leaves the note whole.
func writeFile(path, text string) error {
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".code")
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	if err := os.WriteFile(tmp, []byte(text), mode); err != nil {
		os.Remove(tmp)
		return os.WriteFile(path, []byte(text), mode)
	}
	return os.Rename(tmp, path)
}

// freeName is a path in dir for a file named name.ext that is not taken.
func freeName(dir, name, ext string) string {
	if ext != "" {
		ext = "." + ext
	}
	p := filepath.Join(dir, name+ext)
	for i := 2; ; i++ {
		if _, err := os.Lstat(p); os.IsNotExist(err) {
			return p
		}
		p = filepath.Join(dir, fmt.Sprintf("%s %d%s", name, i, ext))
	}
}

// initial is the first letter of a name, as a capital.
func initial(name string) string {
	for _, r := range name {
		return strings.ToUpper(string(r))
	}
	return "?"
}
