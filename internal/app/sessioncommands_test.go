package app

import (
	"strings"
	"testing"

	"github.com/phucngodev/code/internal/model"
)

func TestSafeResumeCommand(t *testing.T) {
	cases := map[string]bool{
		"claude --resume 0377ff3f-4509-451b-ae60-d08293b7fb9d": true,
		"pi --session abc_123":                                 true,
		"pig --session abc_123":                                true,
		"bash -lic echo hi":                                    false,
		"claude --resume id; rm -rf /":                         false,
		"claude --resume $(whoami)":                            false,
		"claude --resume `whoami`":                             false,
		"claude":                                               false,
		"":                                                     false,
		"claude --resume " + strings.Repeat("a", 600): false,
	}
	for command, want := range cases {
		got := safeResumeCommand(command)
		if (got != "") != want {
			t.Errorf("safeResumeCommand(%q) = %q, accepted = %v, want %v", command, got, got != "", want)
		}
	}
}

// TestLayoutCommandsRoundTrip checks a resumed agent's command survives a
// session save and reads back into the restored pane.
func TestLayoutCommandsRoundTrip(t *testing.T) {
	w := &WindowState{app: testApp(t), store: model.NewStore[*Surface]()}
	w.newWorkspace("")
	s := w.focusedSurface()
	s.command = "claude --resume abc-123"

	saved := w.store.Snapshot(nil)
	fillLayoutCommands(&saved, w.allSurfaces())
	commands := layoutCommands(&saved)
	if commands[s.id] != s.command {
		t.Fatalf("saved command = %q, want %q", commands[s.id], s.command)
	}

	restored := &WindowState{app: testApp(t), store: model.NewStore[*Surface]()}
	restored.store.Restore(saved, newID, func(id, dir, workspace string) *Surface {
		return restored.newSurface(id, dir, workspace)
	})
	for id, command := range layoutCommands(&saved) {
		if surface := restored.surfaceByID(id); surface != nil {
			surface.command = safeResumeCommand(command)
		}
	}
	if got := restored.focusedSurface().command; got != s.command {
		t.Fatalf("restored command = %q, want %q", got, s.command)
	}
}

func TestLayoutCommandsRejectsInjectedCommand(t *testing.T) {
	frame := &model.WindowSession{Workspaces: []model.WorkspaceSession{{
		ID: "ws", Tabs: []model.TabSession{{Layout: &model.Layout{
			Kind: "terminal", ID: "pane", Command: "claude --resume id; rm -rf /",
		}}},
	}}}
	if got := layoutCommands(frame)["pane"]; got != "" {
		t.Fatalf("an injected command was accepted: %q", got)
	}
}
