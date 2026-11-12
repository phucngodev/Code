package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanRespectsLimit(t *testing.T) {
	dir := t.TempDir()
	for i, name := range []string{"a.jsonl", "b.jsonl", "c.jsonl"} {
		path := filepath.Join(dir, name)
		line := `{"type":"session","id":"` + name + `","cwd":"` + dir + `","summary":"prompt ` + name + `"}` + "\n"
		if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
			t.Fatal(err)
		}
		stamp := time.Now().Add(time.Duration(i) * time.Hour)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	sessions := Scan([]Root{{Dir: dir, Agent: "claude"}}, 2)
	if len(sessions) != 2 {
		t.Fatalf("Scan returned %d sessions, want 2", len(sessions))
	}
	if sessions[0].ID != "c.jsonl" {
		t.Fatalf("newest session = %q, want c.jsonl", sessions[0].ID)
	}
}

func TestReadStopsAfterItHasWhatItNeeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	head := `{"type":"session","id":"abc","cwd":"/tmp","summary":"first prompt"}` + "\n"
	// A truncated tail must not stop Read from finding the head.
	data := head + strings.Repeat("x", 512)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	session, ok := Read(path, "claude")
	if !ok || session.Title != "first prompt" || session.Cwd != "/tmp" {
		t.Fatalf("Read = %+v, %v", session, ok)
	}
	if session.Resume == nil || session.Resume.Binary != "claude" {
		t.Fatalf("resume = %+v", session.Resume)
	}
}
