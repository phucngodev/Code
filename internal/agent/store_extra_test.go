package agent

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestStoreConcurrentEventsAndReads exercises the store's lock from a socket
// goroutine and the main thread at once. Run with -race.
func TestStoreConcurrentEventsAndReads(t *testing.T) {
	store := NewStore()
	store.IsLive = func(string) bool { return true }
	store.IsVisible = func(string) bool { return false }

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			store.Apply(Event{
				Version: 1, Agent: "claude", SurfaceID: "term-1",
				Kind: KindWorking, At: time.Now(),
			})
		}
	}()
	for i := 0; i < 300; i++ {
		store.PendingCount([]string{"term-1"})
		store.UnreadCount([]string{"term-1"})
		store.Sessions()
		store.RunningAgent("term-1")
		store.MostPending([]string{"term-1"})
		store.IndicatorForSurfaces([]string{"term-1"})
		store.StatusForSurfaces([]string{"term-1"})
		store.Route("term-1", "hello", time.Now())
		store.MarkSeen("term-1")
	}
	wg.Wait()
	store.Remove("term-1")
	if len(store.Sessions()) != 0 {
		t.Fatal("Remove should drop the session")
	}
}

func TestSessionsReturnsACopy(t *testing.T) {
	store := NewStore()
	store.Apply(Event{Version: 1, Agent: "claude", SurfaceID: "a", Kind: KindSessionStarted, At: time.Now()})
	sessions := store.Sessions()
	sessions["a"] = Session{Agent: "changed"}
	if store.Sessions()["a"].Agent != "claude" {
		t.Fatal("Sessions must not expose the store's map")
	}
}

func TestShimsRootIsUserScoped(t *testing.T) {
	root := ShimsRoot(os.Getpid())
	pid := strconv.Itoa(os.Getpid())
	if !strings.HasSuffix(root, filepath.Join("code", "shims", pid)) {
		t.Fatalf("ShimsRoot = %q, want a per-user code/shims/<pid> directory", root)
	}
	if root == filepath.Join(os.TempDir(), "code-shims", pid) {
		t.Fatal("ShimsRoot must not be a predictable shared /tmp path")
	}
}

func TestResolveBinarySkipsCodeShim(t *testing.T) {
	shimDir := t.TempDir()
	cde := "/usr/bin/true"
	if err := InstallShims(shimDir, cde, []Adapter{{Name: "claude", Binary: "claude", Kind: LaunchFlags}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := ResolveBinary("claude", shimDir, ""); ok {
		t.Fatal("ResolveBinary must not resolve Code's own shim")
	}

	realDir := t.TempDir()
	real := filepath.Join(realDir, "claude")
	if err := os.WriteFile(real, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := ResolveBinary("claude", realDir, ""); !ok || got != real {
		t.Fatalf("ResolveBinary = %q, %v; want the real binary", got, ok)
	}
}

func TestInstallShimsRejectsWorldWritableDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shims")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := InstallShims(dir, "/usr/bin/true", Adapters()); err == nil {
		t.Fatal("a directory others can write into must be refused")
	}
}
