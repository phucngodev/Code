package code

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSummaryFoldsToOneLine(t *testing.T) {
	if got := summary("line one\n\nline two"); got != "line one line two" {
		t.Fatalf("summary = %q", got)
	}
}

func TestSummaryCapsToOneDetailLimit(t *testing.T) {
	got := summary(strings.Repeat("a", detailLimit+50))
	if utf8.RuneCountInString(got) != detailLimit {
		t.Fatalf("summary holds %d runes, want %d", utf8.RuneCountInString(got), detailLimit)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("capped summary = %q", got)
	}
}

func TestSummaryCapsByRunes(t *testing.T) {
	got := summary(strings.Repeat("你", detailLimit+10))
	if utf8.RuneCountInString(got) != detailLimit {
		t.Fatalf("summary holds %d runes, want %d", utf8.RuneCountInString(got), detailLimit)
	}
}

func TestAssistantTextJoinsTextBlocksOnly(t *testing.T) {
	message := map[string]any{"content": []any{
		map[string]any{"type": "text", "text": " hello "},
		map[string]any{"type": "tool_use", "name": "bash"},
		map[string]any{"type": "text", "text": "world "},
	}}
	if got := assistantText(message); got != "hello world" {
		t.Fatalf("assistantText = %q", got)
	}
}

func TestStringOrFallsBack(t *testing.T) {
	if got := stringOr("", "error"); got != "error" {
		t.Fatalf("empty string = %q, want error", got)
	}
	if got := stringOr("boom", "error"); got != "boom" {
		t.Fatalf("value = %q, want boom", got)
	}
}

func TestSettledDetailPrefersTheError(t *testing.T) {
	if got := settledDetail(false, "boom", "answer"); got != "boom" {
		t.Fatalf("settledDetail = %q, want boom", got)
	}
	if got := settledDetail(false, "", "answer"); got != "answer" {
		t.Fatalf("settledDetail = %q, want answer", got)
	}
}

func TestSettledDetailIsEmptyWhenAborted(t *testing.T) {
	if got := settledDetail(true, "boom", "answer"); got != "" {
		t.Fatalf("settledDetail = %q, want empty", got)
	}
}
