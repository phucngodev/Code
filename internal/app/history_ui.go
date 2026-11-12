package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/history"
	"github.com/phucngodev/code/internal/model"
)

// loadHistory scans the agents' transcripts once, off the main thread, when the
// sessions panel first shows.
func (w *WindowState) loadHistory() {
	if w.history.loaded || w.history.loading {
		return
	}
	w.history.loading = true
	go func() {
		sessions := history.Scan(history.DefaultRoots(), history.DefaultLimit)
		w.update(func() {
			w.history.sessions = sessions
			w.history.loading = false
			w.history.loaded = true
		})
	}()
}

// buildSessions draws the sessions panel at the bottom of the sidebar: a
// search bar alone when collapsed, and the heading with the list once opened.
func (w *WindowState) buildSessions(c *ui.Context) {
	t := c.Theme()
	w.loadHistory()
	if !w.history.expanded {
		w.sessionSearchRow(c)
		return
	}
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Text(c, "Sessions").FontSize(11).Bold().TextColor(t.TextMuted)
		shown := history.Filter(w.history.sessions, w.history.query)
		if len(shown) == 0 {
			message := "No saved sessions"
			if len(w.history.sessions) > 0 {
				message = "No matches"
			}
			ui.Text(c, message).FontSize(11).TextColor(t.TextMuted)
		} else {
			// A List builds only the rows in view, where a Scroll would build
			// every saved session each frame.
			ui.List(c, &w.history.list, len(shown), func(i int) {
				w.buildSessionRow(c, shown[i])
			}).FillWidth().MaxHeight(200).ItemKey(func(i int) any {
				return shown[i].Agent + "\x00" + shown[i].ID
			})
		}
		w.sessionSearchRow(c)
	})
}

// sessionSearchRow is the search bar with the chevron that opens the list.
func (w *WindowState) sessionSearchRow(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
		in := ui.SearchField(c, &w.history.query).Placeholder("Search sessions").Label("Search sessions").Grow(1)
		if in.Changed() && !w.history.expanded && strings.TrimSpace(w.history.query) != "" {
			w.history.expanded = true
		}
		label := "Show sessions"
		if w.history.expanded {
			label = "Hide sessions"
		}
		el := ui.ButtonBase(c).Label(label).Padding(2, 4).Radius(4).Cursor(ui.CursorPointer)
		if el.Hovered() {
			el = el.Background(t.SurfaceHover)
		}
		el.Children(func() {
			icon := ui.Icon(c, chevronIcon).FontSize(12).TextColor(t.TextMuted)
			if w.history.expanded {
				icon.Rotate(90)
			}
		})
		el.OnClick(func() {
			w.history.expanded = !w.history.expanded
			if !w.history.expanded {
				w.history.query = ""
			}
			w.markChanged()
		})
	})
}

func (w *WindowState) buildSessionRow(c *ui.Context, session history.Session) {
	t := c.Theme()
	row := ui.ButtonBase(c).FillWidth().Padding(4, 4).Gap(8).AlignItems(ui.Center).Label(session.DisplayTitle())
	hovered := row.Hovered()
	if hovered {
		row.Background(t.Background.Mix(t.Text, 0.10))
	}
	row.Children(func() {
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, session.DisplayTitle()).FontSize(12).SingleLine()
			parts := []string{session.Agent, history.AbbreviateHome(session.Cwd), formatAge(session.Modified)}
			ui.Text(c, strings.Join(nonEmpty(parts), " · ")).FontSize(10).TextColor(t.TextMuted).SingleLine()
		})
	})
	row.OnClick(func() { w.openSession(session) })
}

// openSession opens a terminal where a saved session ran, resuming it when
// Code knows the agent's command.
func (w *WindowState) openSession(session history.Session) {
	name := model.FolderName(session.Cwd)
	w.store.Batch(func() {
		ws := w.store.AddWorkspace(newID(), &name, -1)
		surface := w.newSurface(newID(), session.Cwd, ws.ID)
		if session.Resume != nil {
			surface.command = strings.Join(append([]string{session.Resume.Binary}, session.Resume.Arguments...), " ")
		}
		ws.Add(model.NewTab(newID(), surface))
	})
	w.focusSelected()
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
