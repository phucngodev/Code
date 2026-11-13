package agent

import (
	"bytes"
	"net"
	"strings"
	"testing"
	"time"
)

func TestEventFromArgsRejectsTrailingFlag(t *testing.T) {
	if _, err := eventFromArgs([]string{"working", "--agent"}, map[string]string{}, time.Now()); err == nil {
		t.Fatal("a flag without a value should be rejected")
	}
	if _, err := eventFromArgs([]string{"working", "--agent", "claude"}, map[string]string{}, time.Now()); err != nil {
		t.Fatalf("a complete flag should parse: %v", err)
	}
}

// TestSendEventTruncatesLongDetail checks an oversized event is reduced to a
// line the socket listener can read, instead of being silently dropped.
func TestSendEventTruncatesLongDetail(t *testing.T) {
	path := shortSocketPath(t)
	delivered := make(chan Event, 1)
	listener := NewListener(path, func(e Event) { delivered <- e })
	if err := listener.Start(); err != nil {
		t.Fatal(err)
	}
	defer listener.Stop()

	environment := map[string]string{EnvSocket: path}
	sendEvent(Event{
		Version: 1, Kind: KindWorking, Agent: "claude", SurfaceID: "term-1",
		At: time.Now(), Detail: strings.Repeat("x", maximumLineLength),
	}, path, environment)

	select {
	case event := <-delivered:
		if len([]rune(event.Detail)) > detailLimit {
			t.Fatalf("detail length = %d, want at most %d", len([]rune(event.Detail)), detailLimit)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the event was not delivered")
	}
}

func TestRunCdeHookBoundsStdin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	input := strings.NewReader(strings.Repeat("x", maximumHookPayload+4096))
	code := RunCde([]string{"hook", "claude"}, []string{EnvSocket + "="}, input, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("cde hook returned %d, want 0", code)
	}
}

// TestListenerRejectsOversizedLine documents the reader's bound: a longer line
// ends the connection instead of growing the buffer without limit.
func TestListenerRejectsOversizedLine(t *testing.T) {
	path := shortSocketPath(t)
	delivered := make(chan Event, 1)
	listener := NewListener(path, func(e Event) { delivered <- e })
	if err := listener.Start(); err != nil {
		t.Fatal(err)
	}
	defer listener.Stop()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	// The listener may close the connection as soon as the line is too long.
	_, _ = conn.Write([]byte(strings.Repeat("x", maximumLineLength+1) + "\n"))
	conn.Close()
	select {
	case <-delivered:
		t.Fatal("an oversized line should not become an event")
	case <-time.After(100 * time.Millisecond):
	}
}
