package history

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseClaudeTranscript(t *testing.T) {
	data := `{"type":"user","cwd":"/tmp/project","message":{"content":"add a feature"}}` + "\n" +
		`{"type":"assistant","message":{"content":"sure"}}` + "\n"
	parsed := parse([]byte(data))
	if parsed.cwd != "/tmp/project" {
		t.Fatalf("cwd = %q", parsed.cwd)
	}
	if parsed.title != "add a feature" {
		t.Fatalf("title = %q", parsed.title)
	}
}

func TestParsePiSessionIDAndSummary(t *testing.T) {
	data := `{"type":"session","id":"abc-123"}` + "\n" +
		`{"summary":"Fix the build"}` + "\n" +
		`{"cwd":"/work"}` + "\n"
	parsed := parse([]byte(data))
	if parsed.id != "abc-123" || parsed.cwd != "/work" || parsed.title != "Fix the build" {
		t.Fatalf("parsed = %+v", parsed)
	}
}

func TestResume(t *testing.T) {
	if got := resume("claude", "abc"); got == nil || got.Binary != "claude" || got.Arguments[0] != "--resume" {
		t.Fatalf("claude resume = %+v", got)
	}
	if got := resume("pi", "abc"); got == nil || got.Arguments[0] != "--session" {
		t.Fatalf("pi resume = %+v", got)
	}
	if resume("claude", "bad id; rm -rf") != nil {
		t.Fatal("an unsafe id should not produce a resume command")
	}
	if resume("codex", "abc") != nil {
		t.Fatal("codex has no resume command")
	}
}

func TestFilter(t *testing.T) {
	sessions := []Session{
		{Title: "Fix the build", Cwd: "/work/app"},
		{Title: "Write docs", Cwd: "/work/docs"},
	}
	if got := Filter(sessions, "build"); len(got) != 1 || got[0].Title != "Fix the build" {
		t.Fatalf("Filter = %+v", got)
	}
	if got := Filter(sessions, "docs"); len(got) != 1 {
		t.Fatalf("Filter by cwd = %+v", got)
	}
	if got := Filter(sessions, ""); len(got) != 2 {
		t.Fatalf("empty query = %+v", got)
	}
}

func TestScanFindsTranscripts(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "proj")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"type":"user","cwd":"/tmp/p","message":{"content":"hello"}}` + "\n"
	if err := os.WriteFile(filepath.Join(nested, "one.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	sessions := Scan([]Root{{Dir: dir, Agent: "claude"}}, 10)
	if len(sessions) != 1 || sessions[0].Cwd != "/tmp/p" || sessions[0].Agent != "claude" {
		t.Fatalf("Scan = %+v", sessions)
	}
}
