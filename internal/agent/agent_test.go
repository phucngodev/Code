package agent

import (
	"net"
	"testing"
	"time"
)

// apply is a helper for a store that considers every terminal live and none
// visible.
func apply(t *testing.T, store *Store, kind Kind, at time.Time, detail string) {
	t.Helper()
	store.Apply(Event{
		Version:   1,
		Agent:     "claude",
		Kind:      kind,
		SessionID: "s1",
		SurfaceID: "term-1",
		At:        at,
		Detail:    detail,
	})
}

func TestSessionLifecycle(t *testing.T) {
	store := NewStore()
	base := time.Now()
	apply(t, store, KindSessionStarted, base, "")
	apply(t, store, KindWorking, base.Add(time.Second), "Read")
	if got := store.RunningAgent("term-1"); got != "claude" {
		t.Fatalf("running agent = %q, want claude", got)
	}
	if ind, ok := store.IndicatorForSurfaces([]string{"term-1"}); !ok || ind != IndicatorWorking {
		t.Fatalf("indicator = %v, %v; want working", ind, ok)
	}

	apply(t, store, KindNeedsInput, base.Add(2*time.Second), "Approve")
	if ind, _ := store.IndicatorForSurfaces([]string{"term-1"}); ind != IndicatorWaiting {
		t.Fatalf("indicator = %v, want waiting", ind)
	}
	if got := store.PendingCount([]string{"term-1"}); got != 1 {
		t.Fatalf("pending = %d, want 1", got)
	}

	apply(t, store, KindTurnEnded, base.Add(3*time.Second), "Done")
	session := store.Sessions()["term-1"]
	if !session.FinishedUnseen {
		t.Fatal("a finished turn on an unseen terminal should be unread")
	}
	if got := store.UnreadCount([]string{"term-1"}); got != 1 {
		t.Fatalf("unread = %d, want 1", got)
	}

	store.MarkSeen("term-1")
	if got := store.UnreadCount([]string{"term-1"}); got != 0 {
		t.Fatalf("unread after markSeen = %d, want 0", got)
	}
	if _, ok := store.IndicatorForSurfaces([]string{"term-1"}); ok {
		t.Fatal("a seen idle session should show no indicator")
	}
}

func TestStaleEventIsDropped(t *testing.T) {
	store := NewStore()
	base := time.Now()
	apply(t, store, KindWorking, base.Add(5*time.Second), "")
	apply(t, store, KindNeedsInput, base, "old") // older than the last
	if store.Sessions()["term-1"].State != StateWorking {
		t.Fatal("an older event should not change the state")
	}
}

func TestEventLineRoundTrip(t *testing.T) {
	event := Event{Version: 1, Agent: "codex", Kind: KindWorking, SurfaceID: "x", At: time.Now(), Detail: "Bash"}
	line, err := event.Line()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Kind != KindWorking || parsed.Agent != "codex" || parsed.Detail != "Bash" {
		t.Fatalf("round trip lost data: %+v", parsed)
	}
}

func TestUnknownKind(t *testing.T) {
	parsed, err := ParseLine([]byte(`{"kind":"somethingNew"}`))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Kind != KindUnknown {
		t.Fatalf("kind = %q, want unknown", parsed.Kind)
	}
}

func TestSocketRoundTrip(t *testing.T) {
	path := shortSocketPath(t)
	received := make(chan Event, 1)
	listener := NewListener(path, func(e Event) { received <- e })
	if err := listener.Start(); err != nil {
		t.Fatal(err)
	}
	defer listener.Stop()

	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	line, err := Event{Agent: "pi", Kind: KindWorking, SurfaceID: "term-9", At: time.Now()}.Line()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(line); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-received:
		if got.SurfaceID != "term-9" || got.Kind != KindWorking {
			t.Fatalf("received %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the event")
	}
}
