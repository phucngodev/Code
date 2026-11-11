package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/agent"
	"github.com/phucngodev/code/internal/config"
	"github.com/phucngodev/code/internal/history"
	"github.com/phucngodev/code/internal/model"
)

// WindowState is one window: the workspaces it holds, the pane styles, and
// the transient UI state of the workspace sidebar, the command palette and
// the find bar.
type WindowState struct {
	app   *App
	win   *mygo.Window
	store *model.Store[*Surface]

	sidebarWidth  float32
	sidebarHidden bool
	// sidebarTop is the height of the workspaces pane above the agents and
	// sessions, which the divider between them adjusts.
	sidebarTop float32

	workspaceList     ui.ListState
	workspaceSelected int
	focusedID         string
	history           historyState
	// paneDrop holds a pane drag that ended this frame, applied after the tree
	// is built so the layout is not mutated mid-build.
	paneDrop *paneDropAction
	// closed is set when the window is gone, so callbacks from goroutines stop
	// touching a destroyed window.
	closed bool
	// bounds is the window's last known position and size, tracked as it moves
	// and resizes so the session can save it even though Bounds reads zero at
	// quit time.
	bounds    model.Frame
	hasBounds bool

	// renaming is the id of the workspace being renamed.
	renaming  string
	renameBuf string

	palette paletteState
	find    findState
}

type paletteState struct {
	open     bool
	query    string
	list     ui.ListState
	selected int
	// scrollTo is the row the list should show next frame, set when the keys
	// moved the choice, or -1 for none.
	scrollTo int
}

// historyState is the sidebar's sessions panel.
type historyState struct {
	expanded bool
	query    string
	sessions []history.Session
	loading  bool
	loaded   bool
}

type findState struct {
	open   bool
	query  string
	status terminal.SearchStatus
}

// view is the window's content.
func view(w *WindowState) *ui.Content {
	return ui.View(w.view)
}

func (w *WindowState) view(c *ui.Context) {
	c.SetTheme(w.app.currentChrome(c.Theme().Dark))
	w.applyWindowKeybinds(c)
	t := c.Theme()
	if !w.hasBounds {
		w.captureBounds()
	}
	root := ui.Column(c).Fill().Background(t.Background)
	root.Children(func() {
		w.buildTopBar(c)
		if w.sidebarHidden {
			w.buildTerminalArea(c).Grow(1)
		} else {
			ui.Split(c, &w.sidebarWidth, func() {
				w.buildSidebar(c)
			}, func() {
				w.buildTerminalArea(c)
			}).SplitReset(240).Fill().Grow(1)
		}
	})
	// Keep the app's copy current, so a window opened later and the saved
	// session both start with the width the user chose.
	w.app.sidebarWidth = w.sidebarWidth
	if w.palette.open {
		w.buildPalette(c)
	}
	if w.find.open {
		w.buildFindBar(c)
	}
}

// titleBarIconSize is the one size every button icon on the title bar uses.
const titleBarIconSize float32 = 16

// buildTopBar draws the sidebar toggle, the tab strip and the window actions
// in the room the window controls leave, so the tabs sit in the title bar as
// the Swift app's do.
func (w *WindowState) buildTopBar(c *ui.Context) {
	t := c.Theme()
	bar := c.TitleBar()
	height := bar.Height
	if height < 30 {
		height = 30
	}
	ui.Row(c).FillWidth().Height(height).Gap(2).
		Padding(0, bar.Right+6, 0, bar.Left+6).
		AlignItems(ui.Center).Background(t.Surface).DragWindow().Children(func() {
		w.iconSVGButton(c, sidebarIcon, "Toggle sidebar", titleBarIconSize, func() { w.toggleSidebar() })
		w.iconSVGButton(c, folderPlusIcon, "New workspace", titleBarIconSize, func() { w.newWorkspace("") })
		w.buildBell(c)
		w.iconSVGButton(c, gearIcon, "Settings", titleBarIconSize, func() { w.app.ShowSettings() })
		ui.Box(c).Width(6)
		ws := w.store.Selected()
		if ws != nil {
			for i, tab := range ws.Tabs {
				w.buildTab(c, ws, tab, i, tab == ws.SelectedTab())
				if i < len(ws.Tabs)-1 && tab != ws.SelectedTab() && ws.Tabs[i+1] != ws.SelectedTab() {
					ui.Divider(c).Height(18)
				}
			}
		}
		w.iconSVGButton(c, plusIcon, "New tab", titleBarIconSize, func() { w.newTab("") })
		ui.Spacer(c)
		w.iconSVGButton(c, searchIcon, "Go to…", titleBarIconSize, func() { w.openPalette() })
		w.iconSVGButton(c, splitRightIcon, "Split right", titleBarIconSize, func() { w.splitFocused(model.DirRight) })
		w.iconSVGButton(c, splitDownIcon, "Split down", titleBarIconSize, func() { w.splitFocused(model.DirDown) })
	})
}

// iconSVGButton draws a chrome-less icon button for the title bar, tinted when
// the pointer is over it.
func (w *WindowState) iconSVGButton(c *ui.Context, icon *ui.SVG, description string, size float32, onClick func()) {
	t := c.Theme()
	el := ui.Row(c).Padding(2, 6).Radius(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	hovered := el.Hovered()
	color := t.TextMuted
	if hovered {
		el.Background(t.SurfaceHover)
		color = t.Text
	}
	el.Description(description)
	el.Label(description)
	el.OnClick(onClick)
	el.Children(func() {
		ui.Icon(c, icon).FontSize(size).TextColor(color)
	})
}

func (w *WindowState) buildTab(c *ui.Context, ws *model.Workspace[*Surface], tab *model.Tab[*Surface], index int, selected bool) {
	t := c.Theme()
	label := w.tabLabel(tab)
	if label == "" {
		label = fmt.Sprintf("Tab %d", index+1)
	}
	leaves := tab.Panes.Leaves()
	bar := c.TitleBar()
	height := bar.Height
	if height < 30 {
		height = 30
	}
	row := ui.Row(c).Padding(0, 10).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	hovered := row.Hovered()
	switch {
	case selected:
		// The selected tab is a card rounded at the top and flush with the
		// bottom of the title bar, so it meets the terminal below it.
		row = row.Height(height).AlignSelf(ui.End).Radius(7, 7, 0, 0).Background(t.Background)
	case hovered:
		row = row.Height(30).Radius(6).Background(t.SurfaceHover)
	default:
		row = row.Height(30).Radius(6)
	}
	textColor := t.TextMuted
	if selected {
		textColor = t.Text
	}
	row.Children(func() {
		if w.agentRunning(leaves) {
			ui.Icon(c, sparkleIcon).FontSize(13).TextColor(textColor)
		} else {
			ui.Icon(c, terminalIcon).FontSize(13).TextColor(textColor)
		}
		if indicator, ok := w.indicatorFor(leaves); ok {
			agentGlyph(c, indicator)
		} else {
			ui.Box(c).Size(12, 12)
		}
		ui.Text(c, label).FontSize(12).TextColor(textColor)
		if selected || hovered {
			close := ui.Text(c, "×").FontSize(11).TextColor(t.TextMuted).
				Padding(0, 3).Radius(4).Cursor(ui.CursorPointer)
			if close.Hovered() {
				close = close.Background(t.SurfaceHover).TextColor(t.Text)
			}
			close.OnClick(func() { w.closeTab(tab) })
		}
	})
	row.OnClick(func() {
		ws.Select(tab)
		w.store.Notify()
		w.focusSelected()
	})
}

func (w *WindowState) buildBell(c *ui.Context) {
	t := c.Theme()
	count := w.unreadCount()
	el := ui.Row(c).Padding(2, 6).Gap(4).Radius(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	hovered := el.Hovered()
	color := t.TextMuted
	if hovered {
		el.Background(t.SurfaceHover)
		color = t.Text
	}
	el.Description("Jump to the agent that needs you")
	el.OnClick(func() { w.jumpToLatestUnread() })
	el.Children(func() {
		ui.Icon(c, bellIcon).FontSize(titleBarIconSize).TextColor(color)
		if count > 0 {
			ui.Text(c, fmt.Sprint(count)).FontSize(11).TextColor(color)
		}
	})
}

// buildTerminalArea shows the selected tab's pane tree.
func (w *WindowState) buildTerminalArea(c *ui.Context) ui.Element {
	col := ui.Column(c).Fill()
	col.Children(func() {
		ws := w.store.Selected()
		if ws == nil {
			w.buildEmpty(c)
			return
		}
		tab := ws.SelectedTab()
		if tab == nil {
			w.buildEmpty(c)
			return
		}
		if zoomed, ok := tab.Panes.Zoomed(); ok {
			w.buildSurface(c, tab, zoomed, false)
			return
		}
		split := len(tab.Panes.Leaves()) > 1
		w.buildPaneTree(c, tab, tab.Panes.Root, split)
	})
	if drop := w.paneDrop; drop != nil {
		w.paneDrop = nil
		w.applyPaneDrop(*drop)
	}
	return col
}

// agentRunning reports whether any of the panes has an agent running in it.
func (w *WindowState) agentRunning(surfaces []*Surface) bool {
	for _, s := range surfaces {
		if w.app.agents.RunningAgent(s.id) != "" {
			return true
		}
	}
	return false
}

func (w *WindowState) buildPaneTree(c *ui.Context, tab *model.Tab[*Surface], n *model.Node[*Surface], split bool) {
	if n.HasLeaf {
		w.buildSurface(c, tab, n.Leaf, split)
		return
	}
	if n.Pct <= 0 || n.Pct >= 100 {
		n.Pct = 50
	}
	if n.Axis == model.Horizontal {
		ui.SplitPercent(c, &n.Pct, func() {
			w.buildPaneTree(c, tab, n.First, split)
		}, func() {
			w.buildPaneTree(c, tab, n.Second, split)
		}).Fill()
	} else {
		ui.SplitVerticalPercent(c, &n.Pct, func() {
			w.buildPaneTree(c, tab, n.First, split)
		}, func() {
			w.buildPaneTree(c, tab, n.Second, split)
		}).Fill()
	}
}

// paneDimmed reports whether a pane draws dimmed: only an unfocused pane in a
// tab that shows more than one, as Ghostty's unfocused-split-opacity.
func paneDimmed(tab *model.Tab[*Surface], s *Surface, split bool) bool {
	return split && s != tab.Focused()
}

func (w *WindowState) buildSurface(c *ui.Context, tab *model.Tab[*Surface], s *Surface, split bool) {
	s.start()
	if s.term == nil {
		msg := "Could not start the terminal."
		if s.startErr != nil {
			msg = s.startErr.Error()
		}
		ui.Column(c).Fill().Center().Padding(16).Children(func() {
			ui.Text(c, msg).TextColor(c.Theme().TextMuted).TextAlign(ui.Center)
		})
		return
	}
	t := c.Theme()
	pane := ui.Box(c).Fill()
	pane.Children(func() {
		// Key the terminal before it builds, so the terminal's own view reads
		// the focus while it builds: Element.Key runs after View has built,
		// leaving the element's built identity different from the focused
		// one, and its cursor would draw unfocused and never blink.
		c.Key(s.id)
		el := terminal.View(c, s.term).Bind(&s.handle).Fill()
		if s.wantFocus {
			// Focus, not AutoFocus: a pane that already exists is not born
			// this frame, so AutoFocus would leave the keyboard where it was
			// when navigation moves focus to it. wantFocus is cleared now, so
			// this only claims the focus for the frame it was asked for.
			el = el.Focus()
			s.wantFocus = false
		}
		if paneDimmed(tab, s, split) {
			el = el.Opacity(float32(w.app.settings.Float(config.KeyUnfocusedSplitOpacity, 0.7)))
		}
		if s.handle.Focused(c) && c.WindowActive() {
			w.noteFocused(s)
		}
		if len(tab.Panes.Leaves()) > 1 {
			w.buildPaneGrip(c, s)
		}
	})
	if dragged, ok := ui.Drop[*Surface](pane); ok && dragged != s {
		split, dir := paneDropAt(pane)
		w.paneDrop = &paneDropAction{dragged: dragged, target: s, tab: tab, split: split, dir: dir}
	}
	if _, over := ui.DragOver[*Surface](pane); over {
		pane.Border(2, t.Accent)
	}
}

// buildPaneGrip draws the dots a pane is dragged by, at its top center.
func (w *WindowState) buildPaneGrip(c *ui.Context, s *Surface) {
	t := c.Theme()
	grip := ui.Row(c).Attach(ui.AnchorTop, ui.AnchorTop).Top(6).Gap(3).
		Padding(3, 7).Radius(5).Background(t.Surface).Border(1, t.Border).
		Cursor(ui.CursorPointer).Drag(s)
	grip.Children(func() {
		for i := 0; i < 3; i++ {
			ui.Box(c).Size(3, 3).Radius(2).Background(t.TextMuted)
		}
	})
}

// paneDropAction is a pane drag that ended on another pane.
type paneDropAction struct {
	dragged *Surface
	target  *Surface
	tab     *model.Tab[*Surface]
	split   bool
	dir     model.Direction
}

// paneDropAt reads where in the pane the pointer let go: an outer quarter
// splits, the middle swaps.
func paneDropAt(pane ui.Element) (bool, model.Direction) {
	x, y, ok := pane.PointerPosition()
	if !ok {
		return false, model.DirRight
	}
	b := pane.Bounds()
	if b.W <= 0 || b.H <= 0 {
		return false, model.DirRight
	}
	toLeft, toRight := x, b.W-x
	toTop, toBottom := y, b.H-y
	horizontal := minFloat(toLeft, toRight)
	vertical := minFloat(toTop, toBottom)
	if horizontal < b.W*0.25 && horizontal <= vertical {
		if toLeft < toRight {
			return true, model.DirLeft
		}
		return true, model.DirRight
	}
	if vertical < b.H*0.25 {
		if toTop < toBottom {
			return true, model.DirUp
		}
		return true, model.DirDown
	}
	return false, model.DirRight
}

// applyPaneDrop moves or swaps a dragged pane once the frame is built.
func (w *WindowState) applyPaneDrop(drop paneDropAction) {
	if drop.dragged == drop.target || !drop.tab.Panes.Contains(drop.dragged) {
		return
	}
	if drop.split {
		drop.tab.Panes.Move(drop.dragged, drop.target, drop.dir)
	} else {
		drop.tab.Panes.Swap(drop.dragged, drop.target)
	}
	drop.tab.Focus(drop.dragged)
	w.store.Notify()
	w.focusSelected()
}

func minFloat(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func (w *WindowState) buildEmpty(c *ui.Context) {
	ui.Column(c).Fill().Center().Gap(12).Children(func() {
		ui.Text(c, "No terminals").TextColor(c.Theme().TextMuted)
		if ui.PrimaryButton(c, "New Workspace").Clicked() {
			w.newWorkspace("")
		}
	})
}

// buildFindBar is the terminal search overlay.
func (w *WindowState) buildFindBar(c *ui.Context) {
	ui.Overlay(c, func() {
		panel := ui.Row(c).Attach(ui.AnchorTop, ui.AnchorTop).Top(48).Padding(6, 10).Gap(8).
			Radius(8).Background(c.Theme().Surface).Shadow(0, 4, 16, 0, ui.RGBA(0, 0, 0, 60))
		panel.Children(func() {
			in := ui.TextInput(c, &w.find.query).Placeholder("Find").Width(220).AutoFocus()
			if in.Changed() {
				w.searchCurrent()
			}
			if in.Submitted() {
				w.searchNext()
			}
			status := w.find.status
			label := "0 of 0"
			if status.Total > 0 {
				label = fmt.Sprintf("%d of %d", status.Selected, status.Total)
			}
			ui.Text(c, label).TextColor(c.Theme().TextMuted)
			if ui.Button(c, "Previous").Clicked() {
				w.searchPrevious()
			}
			if ui.Button(c, "Next").Clicked() {
				w.searchNext()
			}
			if ui.Button(c, "Done").Clicked() {
				w.closeFind()
			}
		})
	})
}

// helpers ---------------------------------------------------------------

func (w *WindowState) onStoreChange() {
	w.markChanged()
	if w.win != nil {
		w.win.SetTitle(w.title())
	}
	w.app.ScheduleSave()
}

func (w *WindowState) markChanged() {
	if w.win != nil && !w.closed {
		w.win.Invalidate()
	}
}

func (w *WindowState) update(fn func()) {
	if w.win != nil && !w.closed {
		w.win.Update(fn)
	}
}

func (w *WindowState) frame() *model.Frame {
	if w.hasBounds {
		return &w.bounds
	}
	if w.win == nil {
		return nil
	}
	b := w.win.Bounds()
	return &model.Frame{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height}
}

// captureBounds records the window's position and size while it is valid.
func (w *WindowState) captureBounds() {
	if w.win == nil || w.closed {
		return
	}
	b := w.win.Bounds()
	if b.Width <= 0 || b.Height <= 0 {
		return
	}
	w.bounds = model.Frame{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height}
	w.hasBounds = true
}

func (w *WindowState) title() string {
	ws := w.store.Selected()
	if ws == nil {
		return "Code"
	}
	if tab := ws.SelectedTab(); tab != nil {
		return ws.DisplayName() + " — " + tab.Title()
	}
	return ws.DisplayName()
}

func (w *WindowState) newSurface(id, dir, workspace string) *Surface {
	// A terminal with no directory starts where the config's working-directory
	// says, so its tab shows a path before the shell reports one; a path that
	// does not exist here, as one from another machine, falls back to home.
	if dir == "" {
		dir = w.app.settings.WorkingDirectory()
	}
	s := &Surface{id: id, workspaceID: workspace, dir: validDir(dir), owner: w}
	w.app.mu.Lock()
	if w.app.surfaceOwners == nil {
		w.app.surfaceOwners = map[string]*WindowState{}
	}
	w.app.surfaceOwners[id] = w
	w.app.mu.Unlock()
	return s
}

func (w *WindowState) newWorkspace(dir string) {
	if dir == "" {
		dir = w.app.settings.WorkingDirectory()
	}
	w.store.Batch(func() {
		ws := w.store.AddWorkspace(newID(), nil, -1)
		s := w.newSurface(newID(), dir, ws.ID)
		ws.Add(model.NewTab(newID(), s))
	})
	w.focusSelected()
}

func (w *WindowState) newTab(dir string) {
	ws := w.store.Selected()
	if ws == nil {
		w.newWorkspace(dir)
		return
	}
	if dir == "" {
		if tab := ws.SelectedTab(); tab != nil {
			dir = tab.Focused().LeafDir()
		}
	}
	if dir == "" {
		dir = w.app.settings.WorkingDirectory()
	}
	s := w.newSurface(newID(), dir, ws.ID)
	ws.Add(model.NewTab(newID(), s))
	w.store.Notify()
	w.focusSelected()
}

func (w *WindowState) splitFocused(dir model.Direction) {
	ws, tab, ok := w.store.WorkspaceContaining(w.focusedSurface())
	if !ok {
		return
	}
	focused := tab.Focused()
	s := w.newSurface(newID(), focused.LeafDir(), ws.ID)
	tab.Panes.Split(focused, dir, s)
	tab.Focus(s)
	w.store.Notify()
	s.wantFocus = true
}

func (w *WindowState) focusedSurface() *Surface {
	ws := w.store.Selected()
	if ws == nil {
		return nil
	}
	tab := ws.SelectedTab()
	if tab == nil {
		return nil
	}
	return tab.Focused()
}

func (w *WindowState) focusSelected() {
	if s := w.focusedSurface(); s != nil {
		s.wantFocus = true
	}
}

// noteFocused records a terminal as most recently focused, moves the tab's
// focus to it so navigation starts where the user last was, and clears its
// unread agent alerts.
func (w *WindowState) noteFocused(s *Surface) {
	if w.focusedID != s.id {
		w.focusedID = s.id
		if _, tab, ok := w.store.WorkspaceContaining(s); ok {
			tab.Focus(s)
		}
		w.store.NoteFocus(s)
		w.app.ScheduleSave()
	}
	w.app.agents.MarkSeen(s.id)
}

// routeNotification folds an agent's terminal notification into an event when
// its adapter recognizes it, as Codex's approvals are.
func (w *WindowState) routeNotification(s *Surface, body string) {
	if event, ok := w.app.agents.Route(s.id, body, time.Now()); ok {
		w.app.agents.Apply(event)
	}
}

// reveal switches to a terminal and focuses it.
func (w *WindowState) reveal(s *Surface) {
	ws, tab, ok := w.store.WorkspaceContaining(s)
	if !ok {
		return
	}
	if zoomed, has := tab.Panes.Zoomed(); has && zoomed != s {
		tab.Panes.Unzoom()
	}
	ws.Select(tab)
	tab.Focus(s)
	w.store.Select(ws)
	s.wantFocus = true
	if w.win != nil {
		w.win.Focus()
	}
	w.markChanged()
}

// surfaceExited closes a pane whose program ended, as Ghostty does when a
// shell logs out. It is a no-op once the pane is already closing, so the
// hang-up a close sends does not remove the pane twice.
func (w *WindowState) surfaceExited(s *Surface) {
	if s.closed {
		return
	}
	w.closeSurface(s)
}

func (w *WindowState) closeSurface(s *Surface) {
	_, tab, ok := w.store.WorkspaceContaining(s)
	if !ok {
		return
	}
	if len(tab.Panes.Leaves()) > 1 {
		tab.RemovePane(s)
		s.close()
		w.store.Notify()
		w.focusSelected()
	} else {
		w.closeTab(tab)
	}
}

func (w *WindowState) closeTab(tab *model.Tab[*Surface]) {
	ws, tab2, ok := w.store.WorkspaceContaining(tab.Focused())
	if !ok || tab2 != tab {
		return
	}
	surfaces := tab.Panes.Leaves()
	ws.Remove(tab)
	for _, s := range surfaces {
		s.close()
	}
	if len(ws.Tabs) == 0 {
		w.store.Remove(ws)
	} else {
		w.store.Notify()
	}
	w.focusSelected()
}

func (w *WindowState) closeWorkspace(ws *model.Workspace[*Surface]) {
	// Close the terminals before dropping the workspace, so their shells are
	// hung up and their sessions and shims released.
	for _, s := range workspaceSurfaces(ws) {
		s.close()
	}
	w.store.Remove(ws)
	if len(w.store.Workspaces) == 0 {
		if w.win != nil {
			w.win.Close()
		}
		return
	}
	w.focusSelected()
}

func (w *WindowState) cycleWorkspace(offset int) {
	workspaces := w.store.Workspaces
	if len(workspaces) == 0 {
		return
	}
	index := 0
	for i, ws := range workspaces {
		if ws == w.store.Selected() {
			index = i
			break
		}
	}
	index = ((index+offset)%len(workspaces) + len(workspaces)) % len(workspaces)
	w.store.Select(workspaces[index])
	w.focusSelected()
}

func (w *WindowState) selectWorkspaceIndex(index int) {
	if index >= 0 && index < len(w.store.Workspaces) {
		w.store.Select(w.store.Workspaces[index])
		w.focusSelected()
	}
}

func (w *WindowState) selectTabIndex(index int) {
	ws := w.store.Selected()
	if ws == nil || index < 0 || index >= len(ws.Tabs) {
		return
	}
	ws.Select(ws.Tabs[index])
	w.store.Notify()
	w.focusSelected()
}

func (w *WindowState) toggleZoom() {
	s := w.focusedSurface()
	if s == nil {
		return
	}
	_, tab, ok := w.store.WorkspaceContaining(s)
	if !ok {
		return
	}
	tab.Panes.ToggleZoom(s)
	tab.Focus(s)
	w.store.Notify()
	w.focusSelected()
}

// goToPane moves the focus to the pane in the given direction.
func (w *WindowState) goToPane(nav model.Navigation) {
	s := w.focusedSurface()
	if s == nil {
		return
	}
	_, tab, ok := w.store.WorkspaceContaining(s)
	if !ok {
		return
	}
	target, found := tab.Panes.Neighbor(s, nav)
	if !found {
		return
	}
	if zoomed, has := tab.Panes.Zoomed(); has && zoomed == s {
		tab.Panes.ToggleZoom(target)
	}
	tab.Focus(target)
	target.wantFocus = true
	w.store.Notify()
}

func (w *WindowState) toggleSidebar() {
	w.sidebarHidden = !w.sidebarHidden
	w.app.sidebarHidden = w.sidebarHidden
	if w.sidebarHidden {
		// A collapsed sidebar builds nothing and frees what only its panes
		// used: the sessions are rescanned and the lists rebuilt when it is
		// shown again.
		w.history.sessions = nil
		w.history.loaded = false
		w.workspaceList = ui.ListState{}
	}
	w.focusSelected()
	w.markChanged()
}

func (w *WindowState) allSurfaces() []*Surface {
	return w.store.Panes()
}

func (w *WindowState) unreadCount() int {
	var ids []string
	for _, s := range w.allSurfaces() {
		ids = append(ids, s.id)
	}
	return w.app.agents.PendingCount(ids)
}

func (w *WindowState) jumpToLatestUnread() {
	for _, s := range w.allSurfaces() {
		if w.app.agents.PendingCount([]string{s.id}) > 0 {
			w.reveal(s)
			return
		}
	}
}

func (w *WindowState) indicatorFor(surfaces []*Surface) (agent.Indicator, bool) {
	ids := make([]string, 0, len(surfaces))
	for _, s := range surfaces {
		ids = append(ids, s.id)
	}
	return w.app.agents.IndicatorForSurfaces(ids)
}

// searchCurrent restarts the search in the focused terminal.
func (w *WindowState) searchCurrent() {
	s := w.focusedSurface()
	if s == nil || s.term == nil {
		return
	}
	s.term.Search(w.find.query)
	w.find.status = s.term.SearchStatus()
}

func (w *WindowState) searchNext() {
	s := w.focusedSurface()
	if s == nil || s.term == nil {
		return
	}
	s.term.SearchNext()
	w.find.status = s.term.SearchStatus()
}

func (w *WindowState) searchPrevious() {
	s := w.focusedSurface()
	if s == nil || s.term == nil {
		return
	}
	s.term.SearchPrevious()
	w.find.status = s.term.SearchStatus()
}

func (w *WindowState) openFind() {
	w.find.open = true
	w.searchCurrent()
	w.markChanged()
}

func (w *WindowState) closeFind() {
	w.find.open = false
	if s := w.focusedSurface(); s != nil && s.term != nil {
		s.term.EndSearch()
	}
	w.focusSelected()
	w.markChanged()
}

// tabLabel is a tab's title: the terminal's own title when it sets one,
// otherwise its working directory, truncated from whichever end keeps the
// useful part.
func (w *WindowState) tabLabel(tab *model.Tab[*Surface]) string {
	s := tab.Focused()
	if title := s.LeafTitle(); title != "" {
		return truncate(title, 24)
	}
	if dir := s.LeafDir(); dir != "" {
		return truncateStart(strings.TrimRight(model.AbbreviateHome(dir), "/"), 28)
	}
	return "Terminal"
}

// truncate keeps the start of a string and marks what it drops.
func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

// truncateStart keeps the end of a path, where the folder name is, marking
// what it drops.
func truncateStart(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return "…" + string(r[len(r)-(n-1):])
}
