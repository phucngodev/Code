package app

import (
	"github.com/egoist/mygo"

	"github.com/phucngodev/code/internal/agent"
	"github.com/phucngodev/code/internal/config"
)

// handleAgentUpdate tells the user about an agent that needs them or finished,
// with a desktop notification, unless they are already looking at it.
func (a *App) handleAgentUpdate(update agent.Update) {
	if !update.Alerts() || update.RepeatsWait() {
		return
	}
	w := a.ownerOf(update.Surface)
	if w == nil || w.isInView(update.Surface) {
		return
	}
	needsInput := update.Kind == agent.KindNeedsInput || update.Kind == agent.KindAttention
	if needsInput {
		if !a.settings.Bool(config.KeyAgentNeedsInputNotify, true) {
			return
		}
	} else if !a.settings.Bool(config.KeyAgentFinishedNotify, true) {
		return
	}
	title := "Code"
	if name := w.workspaceName(update.Surface); name != "" {
		title = name
	}
	body := update.Session.Detail
	if body == "" {
		if needsInput {
			body = "The agent needs your input."
		} else {
			body = "The agent finished."
		}
	}
	notification := mygo.NewNotification(mygo.NotificationOptions{
		ID:     update.Surface,
		Title:  title,
		Body:   body,
		Group:  "agent",
		Silent: true,
	})
	if previous := a.notifications[update.Surface]; previous != nil {
		previous.Close()
	}
	a.notifications[update.Surface] = notification
	notification.OnClick(func() {
		w.update(func() {
			if s := w.surfaceByID(update.Surface); s != nil {
				w.reveal(s)
			}
		})
	})
	_ = notification.Show()
}

// postTerminalNotification shows an OSC notification a program asked for when
// no agent adapter claimed it, so a plain shell's notifications are not lost.
func (a *App) postTerminalNotification(w *WindowState, s *Surface, title, body string) {
	if body == "" {
		body = title
	}
	if body == "" {
		return
	}
	if title == "" || title == body {
		title = "Code"
		if name := w.workspaceName(s.id); name != "" {
			title = name
		}
	}
	id := "terminal:" + s.id
	notification := mygo.NewNotification(mygo.NotificationOptions{
		ID: id, Title: title, Body: body, Group: "terminal", Silent: true,
	})
	if previous := a.notifications[id]; previous != nil {
		previous.Close()
	}
	a.notifications[id] = notification
	surface := s
	notification.OnClick(func() {
		w.update(func() {
			if s := w.surfaceByID(surface.id); s != nil {
				w.reveal(s)
			}
		})
	})
	_ = notification.Show()
}

// closeNotification drops the alert shown for a terminal that closed.
func (a *App) closeNotification(surface string) {
	if notification := a.notifications[surface]; notification != nil {
		notification.Close()
		delete(a.notifications, surface)
	}
}

// updateBadge shows the number of terminals waiting on the user on the app
// icon.
func (a *App) updateBadge() {
	total := 0
	for _, w := range a.windows {
		total += a.agents.PendingCount(surfaceIDs(w.allSurfaces()))
	}
	mygo.App.SetBadgeCount(total)
}

func surfaceIDs(surfaces []*Surface) []string {
	ids := make([]string, 0, len(surfaces))
	for _, s := range surfaces {
		ids = append(ids, s.id)
	}
	return ids
}

// isInView reports whether the user is looking at a terminal right now.
func (w *WindowState) isInView(id string) bool {
	return w.focusedID == id && w.win != nil && w.win.IsFocused()
}

// surfaceByID finds one of the window's terminals.
func (w *WindowState) surfaceByID(id string) *Surface {
	for _, s := range w.allSurfaces() {
		if s.id == id {
			return s
		}
	}
	return nil
}

// workspaceName is the workspace a terminal belongs to.
func (w *WindowState) workspaceName(id string) string {
	if s := w.surfaceByID(id); s != nil {
		if ws, _, ok := w.store.WorkspaceContaining(s); ok {
			return ws.DisplayName()
		}
	}
	return ""
}
