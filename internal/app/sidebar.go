package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/agent"
	"github.com/phucngodev/code/internal/config"
	"github.com/phucngodev/code/internal/model"
)

// buildSidebar draws the workspaces list above the running agents and the
// sessions, on the window's own background, with a divider between the two
// the user can drag, as the Swift app's sidebar does.
func (w *WindowState) buildSidebar(c *ui.Context) {
	t := c.Theme()
	if w.sidebarTop == 0 {
		w.sidebarTop = 320
	}
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.SplitVertical(c, &w.sidebarTop, func() {
			ui.Column(c).Fill().Background(t.Background).Padding(10).Gap(8).Children(func() {
				ui.Text(c, "Workspaces").FontSize(11).Bold().TextColor(t.TextMuted).Padding(2, 2, 0, 2)
				w.buildWorkspaceList(c)
			})
		}, func() {
			ui.Column(c).Fill().Background(t.Background).Padding(10).Gap(8).Children(func() {
				ui.Text(c, "Agents").FontSize(11).Bold().TextColor(t.TextMuted).Padding(2, 2, 0, 2)
				w.buildAgentList(c)
				// The divider runs the full width, as the one above the agents,
				// not inset by the column's padding.
				ui.Divider(c).MarginX(-10)
				w.buildSessions(c)
			})
		}).Fill().SplitReset(320)
	})
}

func (w *WindowState) buildWorkspaceList(c *ui.Context) {
	ws := w.store.Workspaces
	if len(ws) == 0 {
		ui.Text(c, "No workspaces").TextColor(c.Theme().TextMuted)
		return
	}
	selected := -1
	for i, workspace := range ws {
		if workspace == w.store.Selected() {
			selected = i
		}
	}
	w.workspaceList.Selected = &w.workspaceSelected
	w.workspaceSelected = selected
	// The list paints the chosen row in the theme's accent, so it gets the
	// sidebar's gray instead of the app accent, with the focus ring off. The
	// accent's text goes with the gray, since the accent's own light text does
	// not read on it.
	rowTheme := sidebarRowTheme(c.Theme())
	themed := ui.Themed(c, &rowTheme).Grow(1).MarginX(-10)
	themed.Children(func() {
		list := ui.List(c, &w.workspaceList, len(ws), func(i int) {
			w.buildWorkspaceRow(c, ws[i])
		}).Fill().FocusRing(false)
		if list.Changed() && w.workspaceSelected >= 0 && w.workspaceSelected < len(ws) {
			w.store.Select(ws[w.workspaceSelected])
			w.focusSelected()
		}
	})
}

// sidebarRowTheme is the theme for the workspace list: the chosen row takes the
// sidebar's gray instead of the app accent, with the theme's own text on it,
// since the accent's light text does not read on a gray.
func sidebarRowTheme(t *ui.Theme) ui.Theme {
	rowTheme := *t
	rowTheme.Accent = t.Background.Mix(t.Text, 0.20)
	rowTheme.InactiveAccent = rowTheme.Accent
	rowTheme.AccentText = t.Text
	rowTheme.Radius = 0
	return rowTheme
}

func (w *WindowState) buildWorkspaceRow(c *ui.Context, ws *model.Workspace[*Surface]) {
	t := c.Theme()
	row := ui.Row(c).Padding(8, 8).Gap(10).AlignItems(ui.Center)
	hovered := row.Hovered()
	row.Children(func() {
		ui.Box(c).Size(16, 16).Radius(4).Background(config.Accent(ws.Accent))
		if w.renaming == ws.ID {
			in := ui.TextInput(c, &w.renameBuf).AutoFocus().Grow(1)
			if in.Submitted() {
				w.commitRename(ws)
			}
			return
		}
		ui.Column(c).Grow(1).Gap(0).Children(func() {
			ui.Text(c, ws.DisplayName()).Bold()
			if dir := displayDir(workspaceDir(ws)); dir != "" {
				ui.Text(c, dir).FontSize(11).TextColor(t.TextMuted)
			}
		})
		if hovered {
			close := ui.Text(c, "×").FontSize(11).TextColor(t.TextMuted).
				Padding(0, 3).Radius(4).Cursor(ui.CursorPointer)
			if close.Hovered() {
				close = close.Background(t.SurfaceHover).TextColor(t.Text)
			}
			close.OnClick(func() { w.closeWorkspace(ws) })
		} else if status, ok := w.workspaceStatus(ws); ok {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				agentGlyph(c, status.Indicator)
				ui.Text(c, formatAge(status.Since)).FontSize(11).TextColor(t.TextMuted)
			})
		}
	})
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("Rename").Chosen() {
			w.beginRename(ws)
		}
		m.Separator()
		if m.Item("Close Workspace").Chosen() {
			w.closeWorkspace(ws)
		}
	})
}

// agentRow is one terminal of the selected workspace that runs an agent.
type agentRow struct {
	surface   *Surface
	title     string
	subtitle  string
	indicator *agent.Indicator
}

// buildAgentList lists the selected workspace's agent terminals, as the Swift
// app's Agents section does: a glyph, the terminal's title, and an
// "agent · Tab n › Position" subtitle.
func (w *WindowState) buildAgentList(c *ui.Context) {
	t := c.Theme()
	rows := w.selectedWorkspaceAgentRows()
	column := ui.Column(c).FillWidth().Grow(1).Gap(2)
	column.Children(func() {
		if len(rows) == 0 {
			ui.Text(c, "No running agents").FontSize(11).TextColor(t.TextMuted).Padding(2, 2)
			return
		}
		for _, row := range rows {
			row := row
			item := ui.Row(c).Padding(6, 6).Gap(8).Radius(7).AlignItems(ui.Center)
			hovered := item.Hovered()
			if hovered {
				item.Background(t.Background.Mix(t.Text, 0.10))
			}
			item.Children(func() {
				if row.indicator != nil {
					agentGlyph(c, *row.indicator)
				} else {
					ui.Box(c).Size(12, 12)
				}
				ui.Column(c).Grow(1).Gap(1).Children(func() {
					ui.Text(c, row.title)
					if row.subtitle != "" {
						ui.Text(c, row.subtitle).FontSize(11).TextColor(t.TextMuted)
					}
				})
			})
			item.OnClick(func() { w.reveal(row.surface) })
		}
	})
}

// selectedWorkspaceAgentRows builds the agent rows for the selected workspace.
func (w *WindowState) selectedWorkspaceAgentRows() []agentRow {
	ws := w.store.Selected()
	if ws == nil {
		return nil
	}
	var out []agentRow
	for ti, tab := range ws.Tabs {
		leaves := tab.Panes.Leaves()
		var positions []string
		if len(leaves) > 1 {
			positions = panePositions(tab.Panes.Root)
		}
		for pi, s := range leaves {
			name := w.app.agents.RunningAgent(s.id)
			if name == "" {
				continue
			}
			location := ""
			if len(leaves) > 1 {
				pos := ""
				if pi < len(positions) {
					pos = positions[pi]
				}
				location = fmt.Sprintf("Tab %d › %s", ti+1, pos)
			} else if len(ws.Tabs) > 1 {
				location = fmt.Sprintf("Tab %d", ti+1)
			}
			subtitle := name
			if location != "" {
				subtitle = name + " · " + location
			}
			var indicator *agent.Indicator
			if value, ok := w.indicatorFor([]*Surface{s}); ok {
				indicator = &value
			}
			out = append(out, agentRow{
				surface:   s,
				title:     model.DisplayTitle[*Surface](s),
				subtitle:  subtitle,
				indicator: indicator,
			})
		}
	}
	return out
}

// panePositions names each pane for its side of the outermost splits, such as
// "Left" or "Top right", numbering them when two names would match.
func panePositions[L comparable](root *model.Node[L]) []string {
	var names []string
	var visit func(n *model.Node[L], vertical, horizontal string)
	visit = func(n *model.Node[L], vertical, horizontal string) {
		if n.HasLeaf {
			parts := make([]string, 0, 2)
			if vertical != "" {
				parts = append(parts, vertical)
			}
			if horizontal != "" {
				parts = append(parts, horizontal)
			}
			name := strings.Join(parts, " ")
			if name != "" {
				name = strings.ToUpper(name[:1]) + name[1:]
			}
			names = append(names, name)
			return
		}
		if n.Axis == model.Horizontal {
			left, right := horizontal, horizontal
			if left == "" {
				left = "left"
			}
			if right == "" {
				right = "right"
			}
			visit(n.First, vertical, left)
			visit(n.Second, vertical, right)
		} else {
			top, bottom := vertical, vertical
			if top == "" {
				top = "top"
			}
			if bottom == "" {
				bottom = "bottom"
			}
			visit(n.First, top, horizontal)
			visit(n.Second, bottom, horizontal)
		}
	}
	visit(root, "", "")
	counts := map[string]int{}
	for _, name := range names {
		if name != "" {
			counts[name]++
		}
	}
	for _, count := range counts {
		if count > 1 {
			for i := range names {
				names[i] = fmt.Sprintf("Pane %d", i+1)
			}
			break
		}
	}
	return names
}

func workspaceSurfaces(ws *model.Workspace[*Surface]) []*Surface {
	var out []*Surface
	for _, tab := range ws.Tabs {
		out = append(out, tab.Panes.Leaves()...)
	}
	return out
}

func workspaceDir(ws *model.Workspace[*Surface]) string {
	if tab := ws.SelectedTab(); tab != nil {
		return tab.Focused().LeafDir()
	}
	return ""
}

// workspaceStatus is the most urgent agent status among a workspace's panes.
func (w *WindowState) workspaceStatus(ws *model.Workspace[*Surface]) (agent.Status, bool) {
	surfaces := workspaceSurfaces(ws)
	ids := make([]string, 0, len(surfaces))
	for _, s := range surfaces {
		ids = append(ids, s.id)
	}
	return w.app.agents.StatusForSurfaces(ids)
}

// displayDir abbreviates a path and drops its trailing separator.
func displayDir(dir string) string {
	if dir == "" {
		return ""
	}
	return strings.TrimRight(model.AbbreviateHome(dir), "/")
}

// formatAge renders how long ago a status was taken, short.
func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func (w *WindowState) beginRename(ws *model.Workspace[*Surface]) {
	w.renaming = ws.ID
	w.renameBuf = ws.DisplayName()
	w.markChanged()
}

func (w *WindowState) commitRename(ws *model.Workspace[*Surface]) {
	name := strings.TrimSpace(w.renameBuf)
	if name == "" {
		ws.HasName = false
	} else {
		ws.Name = name
		ws.HasName = true
	}
	w.renaming = ""
	w.store.Notify()
	w.focusSelected()
}

// openPalette opens the command palette.
func (w *WindowState) openPalette() {
	w.palette.open = true
	w.palette.query = ""
	w.palette.selected = 0
	w.palette.scrollTo = -1
	w.palette.list = ui.ListState{}
	w.markChanged()
}

func (w *WindowState) closePalette() {
	w.palette.open = false
	w.focusSelected()
	w.markChanged()
}

type paletteItem struct {
	surface   *Surface
	title     string
	detail    string
	workspace string
	agent     string
	// isWorkspace marks the row standing for a whole workspace, which shows
	// the workspace icon and reveals its focused terminal.
	isWorkspace bool
}

// paletteItems lists every workspace and terminal: a workspace head row, then
// its terminals with the most recently focused first.
func (w *WindowState) paletteItems() []paletteItem {
	rank := map[string]int{}
	for i, id := range w.store.RecentlyFocused {
		rank[id] = i
	}
	rankOf := func(id string) int {
		if r, ok := rank[id]; ok {
			return r
		}
		return len(rank) + 1
	}
	var items []paletteItem
	for _, ws := range w.store.Workspaces {
		if tab := ws.SelectedTab(); tab != nil {
			s := tab.Focused()
			items = append(items, paletteItem{
				surface:     s,
				title:       ws.DisplayName(),
				detail:      paletteDetail(s, ws.DisplayName()),
				workspace:   ws.DisplayName(),
				isWorkspace: true,
			})
		}
		var panes []*Surface
		for _, tab := range ws.Tabs {
			panes = append(panes, tab.Panes.Leaves()...)
		}
		sort.SliceStable(panes, func(i, j int) bool { return rankOf(panes[i].id) < rankOf(panes[j].id) })
		for _, s := range panes {
			title := model.DisplayTitle[*Surface](s)
			items = append(items, paletteItem{
				surface:   s,
				title:     title,
				detail:    paletteDetail(s, title),
				workspace: ws.DisplayName(),
				agent:     w.app.agents.RunningAgent(s.id),
			})
		}
	}
	return items
}

// paletteDetail is a terminal's path, unless it already reads as the title.
func paletteDetail(s *Surface, title string) string {
	if s.LeafDir() == "" {
		return ""
	}
	abbreviated := model.AbbreviateHome(s.LeafDir())
	if abbreviated == title {
		return ""
	}
	return abbreviated
}

func filterPalette(items []paletteItem, query string) []paletteItem {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return items
	}
	var out []paletteItem
	for _, item := range items {
		haystack := strings.ToLower(strings.Join([]string{item.title, item.detail, item.workspace, item.agent}, " "))
		match := true
		for _, word := range words {
			if !strings.Contains(haystack, word) {
				match = false
				break
			}
		}
		if match {
			out = append(out, item)
		}
	}
	return out
}

// buildPalette draws the command palette over the window.
func (w *WindowState) buildPalette(c *ui.Context) {
	t := c.Theme()
	ui.Overlay(c, func() {
		items := filterPalette(w.paletteItems(), w.palette.query)
		if w.palette.selected >= len(items) {
			w.palette.selected = len(items) - 1
		}
		if w.palette.selected < 0 {
			w.palette.selected = 0
		}
		// The list paints the chosen row in the theme's accent, so the palette
		// gets a soft one instead of the solid accent the rest of the app uses,
		// and square corners rather than a rounded highlight.
		paletteTheme := *t
		paletteTheme.Accent = t.Accent.Mix(t.Background, 0.82)
		paletteTheme.InactiveAccent = paletteTheme.Accent
		paletteTheme.Radius = 0
		panel := ui.Themed(c, &paletteTheme).Width(620).MaxHeight(430).Radius(12).
			Background(t.Background).Border(1, t.Border).Clip().
			Shadow(0, 24, 60, 0, ui.RGBA(0, 0, 0, 80)).
			Attach(ui.AnchorTop, ui.AnchorTop).Top(64)
		panel.Children(func() {
			ui.Row(c).FillWidth().Padding(10, 14).Gap(8).AlignItems(ui.Center).Background(t.Surface).Children(func() {
				ui.Icon(c, searchIcon).FontSize(15).TextColor(t.TextMuted)
				in := ui.TextInput(c, &w.palette.query).Placeholder("Go to tab, pane, workspace or agent").
					Grow(1).FocusRing(false).AutoFocus().
					OnTextKey(func(k ui.TextKey) bool {
						switch k.Key {
						case ui.KeyDown:
							if w.palette.selected < len(items)-1 {
								w.palette.selected++
								w.palette.scrollTo = w.palette.selected
							}
							return true
						case ui.KeyUp:
							if w.palette.selected > 0 {
								w.palette.selected--
								w.palette.scrollTo = w.palette.selected
							}
							return true
						case ui.KeyEnter:
							if w.palette.selected >= 0 && w.palette.selected < len(items) {
								w.reveal(items[w.palette.selected].surface)
								w.closePalette()
							}
							return true
						}
						return false
					})
				if in.Changed() {
					w.palette.selected = 0
				}
			})
			ui.Divider(c)
			if len(items) == 0 {
				ui.Text(c, "Nothing to go to").Padding(14).TextColor(t.TextMuted)
			} else {
				w.palette.list.Selected = &w.palette.selected
				list := ui.List(c, &w.palette.list, len(items)).MaxHeight(370).Grow(1).Rows(func(row ui.ListRow) {
					w.buildPaletteRow(row, items[row.Index], row.Index == len(items)-1)
				})
				if w.palette.scrollTo >= 0 {
					w.palette.list.ScrollIntoView(w.palette.scrollTo)
					w.palette.scrollTo = -1
				}
				if list.Changed() && w.palette.selected >= 0 && w.palette.selected < len(items) {
					w.reveal(items[w.palette.selected].surface)
					w.closePalette()
				}
			}
		})
		if panel.PressedOutside() || panel.OverlayShortcut(0, ui.KeyEscape) {
			w.closePalette()
		}
	})
}

// buildPaletteRow draws one search result: its icon, title and path, and the
// workspace it belongs to.
func (w *WindowState) buildPaletteRow(row ui.ListRow, item paletteItem, last bool) {
	c := row.Context
	t := c.Theme()
	line := ui.Row(c).FillWidth().Padding(8, 14).Gap(10).AlignItems(ui.Center)
	if !last {
		line = line.BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
	}
	line.Children(func() {
		icon := terminalIcon
		switch {
		case item.isWorkspace:
			icon = gridIcon
		case item.agent != "":
			icon = sparkleIcon
		}
		ui.Icon(c, icon).FontSize(15).TextColor(t.TextMuted)
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, item.title)
			if item.detail != "" {
				ui.Text(c, item.detail).FontSize(11).TextColor(t.TextMuted)
			}
		})
		if item.workspace != "" {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				if indicator, ok := w.indicatorFor([]*Surface{item.surface}); ok {
					agentGlyph(c, indicator)
				}
				ui.Text(c, item.workspace).FontSize(12).TextColor(t.TextMuted)
			})
		}
	})
}
