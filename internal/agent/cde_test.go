package agent

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// shortSocketPath names a socket under /tmp, whose path is short enough for
// the 104-byte limit a Unix socket address has.
func shortSocketPath(t *testing.T) string {
	t.Helper()
	path := fmt.Sprintf("/tmp/code-cde-%d-%d.sock", os.Getpid(), time.Now().UnixNano())
	t.Cleanup(func() { os.Remove(path) })
	return path
}

func TestRunCdeEventReachesListener(t *testing.T) {
	path := shortSocketPath(t)
	received := make(chan Event, 1)
	listener := NewListener(path, func(e Event) { received <- e })
	if err := listener.Start(); err != nil {
		t.Fatal(err)
	}
	defer listener.Stop()

	env := []string{EnvSocket + "=" + path, EnvSurfaceID + "=term-1"}
	code := RunCde([]string{"event", "working", "--agent", "claude", "--detail", "Read"}, env, strings.NewReader(""), io.Discard, io.Discard)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	select {
	case event := <-received:
		if event.Kind != KindWorking || event.Agent != "claude" || event.Detail != "Read" || event.SurfaceID != "term-1" {
			t.Fatalf("received %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the event")
	}
}

func TestRunCdeHookTranslatesClaude(t *testing.T) {
	path := shortSocketPath(t)
	received := make(chan Event, 1)
	listener := NewListener(path, func(e Event) { received <- e })
	if err := listener.Start(); err != nil {
		t.Fatal(err)
	}
	defer listener.Stop()

	env := []string{EnvSocket + "=" + path, EnvSurfaceID + "=term-1"}
	hook := `{"hook_event_name":"PreToolUse","session_id":"s1","cwd":"/tmp","tool_name":"Bash"}`
	code := RunCde([]string{"hook", "claude"}, env, strings.NewReader(hook), io.Discard, io.Discard)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	select {
	case event := <-received:
		if event.Kind != KindWorking || event.Detail != "Bash" {
			t.Fatalf("received %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the event")
	}
}
