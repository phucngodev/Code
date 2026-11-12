package app

import (
	"strings"

	"github.com/phucngodev/code/internal/model"
)

// layoutCommands reads the program each saved pane was opened to run. A
// command is only kept when it is a plain agent invocation, since a session
// file can be edited by hand and the command is handed to a login shell.
func layoutCommands(frame *model.WindowSession) map[string]string {
	out := map[string]string{}
	if frame == nil {
		return out
	}
	var walk func(*model.Layout)
	walk = func(l *model.Layout) {
		if l == nil {
			return
		}
		if l.Kind == "terminal" && l.ID != "" && l.Command != "" {
			out[l.ID] = safeResumeCommand(l.Command)
		}
		walk(l.First)
		walk(l.Second)
	}
	for i := range frame.Workspaces {
		for j := range frame.Workspaces[i].Tabs {
			walk(frame.Workspaces[i].Tabs[j].Layout)
		}
	}
	return out
}

// fillLayoutCommands writes the program each live pane runs into the layouts
// about to be saved, keyed by the pane's stable id.
func fillLayoutCommands(session *model.WindowSession, surfaces []*Surface) {
	commands := make(map[string]string, len(surfaces))
	for _, s := range surfaces {
		if s.command != "" {
			commands[s.id] = s.command
		}
	}
	var walk func(*model.Layout)
	walk = func(l *model.Layout) {
		if l == nil {
			return
		}
		if l.Kind == "terminal" {
			l.Command = commands[l.ID]
		}
		walk(l.First)
		walk(l.Second)
	}
	for i := range session.Workspaces {
		for j := range session.Workspaces[i].Tabs {
			walk(session.Workspaces[i].Tabs[j].Layout)
		}
	}
}

// safeResumeCommand returns command when it is a plain agent invocation with
// no shell syntax, or empty when it is not. Session files live in the user's
// config directory and are plain JSON, so they must not be able to inject a
// command into the login shell that restores a pane.
func safeResumeCommand(command string) string {
	if len(command) > 512 {
		return ""
	}
	parts := splitCommand(command)
	if len(parts) < 2 {
		return ""
	}
	switch parts[0] {
	case "claude", "pi", "pig":
	default:
		return ""
	}
	for _, part := range parts {
		if part == "" {
			return ""
		}
		for _, r := range part {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			case strings.ContainsRune("-_./=:@+", r):
			default:
				return ""
			}
		}
	}
	return command
}
