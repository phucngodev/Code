package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func payload(t *testing.T, agent, jsonText string) HookPayload {
	t.Helper()
	env := map[string]string{EnvSurfaceID: "term-1", EnvWorkspaceID: "ws-1"}
	parsed, ok := ParseHookPayload([]byte(jsonText), agent, env, time.Unix(100, 0))
	if !ok {
		t.Fatalf("could not parse %s", jsonText)
	}
	return parsed
}

func TestSummarizeFoldsAndLimits(t *testing.T) {
	if got := Summarize("  hello\n  world "); got != "hello world" {
		t.Fatalf("Summarize = %q", got)
	}
	long := strings.Repeat("a", detailLimit+50)
	got := Summarize(long)
	if len([]rune(got)) != detailLimit || !strings.HasSuffix(got, "…") {
		t.Fatalf("Summarize length = %d, want %d with ellipsis", len([]rune(got)), detailLimit)
	}
}

func TestClaudeTranslate(t *testing.T) {
	session := `"session_id":"s1","cwd":"/tmp","hook_event_name":"%s"`

	started := claudeTranslate(payload(t, "claude", `{`+sprintf(session, "SessionStart")+`}`))
	if started == nil || started.Kind != KindSessionStarted || started.Agent != "claude" {
		t.Fatalf("SessionStart = %+v", started)
	}
	working := claudeTranslate(payload(t, "claude", `{`+sprintf(session, "PreToolUse")+`,"tool_name":"Bash"}`))
	if working == nil || working.Kind != KindWorking || working.Detail != "Bash" {
		t.Fatalf("PreToolUse = %+v", working)
	}
	needs := claudeTranslate(payload(t, "claude", `{`+sprintf(session, "Notification")+`,"notification_type":"permission_prompt"}`))
	if needs == nil || needs.Kind != KindNeedsInput {
		t.Fatalf("Notification = %+v", needs)
	}
	ignored := claudeTranslate(payload(t, "claude", `{`+sprintf(session, "Notification")+`,"notification_type":"idle_prompt"}`))
	if ignored != nil {
		t.Fatalf("idle prompt should be ignored, got %+v", ignored)
	}
	sub := claudeTranslate(payload(t, "claude", `{"agent_id":"sub","hook_event_name":"Stop"}`))
	if sub != nil {
		t.Fatalf("a subagent hook should be ignored, got %+v", sub)
	}
}

func TestClaudeArgumentsAddsSettings(t *testing.T) {
	arguments := claudeArguments([]string{"--resume"}, "/app/Code")
	if len(arguments) < 2 || arguments[0] != "--settings" {
		t.Fatalf("arguments = %v", arguments)
	}
	if !strings.Contains(arguments[1], "hook claude") || !strings.Contains(arguments[1], "SessionStart") {
		t.Fatalf("settings missing Code's hooks: %s", arguments[1])
	}
	if arguments[len(arguments)-1] != "--resume" {
		t.Fatalf("the user's argument should follow: %v", arguments)
	}
}

func TestCodexArgumentsAddHooksAndNotifications(t *testing.T) {
	arguments := codexArguments([]string{"exec"}, "/app/Code")
	joined := strings.Join(arguments, " ")
	if !strings.Contains(joined, "hook codex") {
		t.Fatalf("Codex arguments missing hooks: %v", arguments)
	}
	if !strings.Contains(joined, "tui.notifications") {
		t.Fatalf("Codex arguments missing notification settings: %v", arguments)
	}
	if arguments[len(arguments)-1] != "exec" {
		t.Fatalf("the user's argument should follow: %v", arguments)
	}
}

func TestCodexTranslateNotification(t *testing.T) {
	kind, detail, ok := codexTranslateNotification("Approval requested: rm -rf")
	if !ok || kind != KindNeedsInput || !strings.Contains(detail, "Approval requested") {
		t.Fatalf("notification = %q, %q, %v", kind, detail, ok)
	}
	if _, _, ok := codexTranslateNotification("Turn complete"); ok {
		t.Fatal("a turn-complete notification should be ignored")
	}
}

func TestOpenCodeEnvironmentAddsPlugins(t *testing.T) {
	environment := map[string]string{EnvAgentsDir: "/assets"}
	got := openCodeEnvironment(environment, "/app/Code")
	content := got[openCodeContentKey]
	if !strings.Contains(content, "/assets/agents/opencode/code.js") || !strings.Contains(content, "/assets/agents/opencode/v2") {
		t.Fatalf("OPENCODE_CONFIG_CONTENT = %s", content)
	}
}

func TestIsCdeCommand(t *testing.T) {
	for _, args := range [][]string{{"event", "working"}, {"hook", "claude"}, {"launch", "codex"}, {"ping"}} {
		if !IsCdeCommand(args) {
			t.Fatalf("IsCdeCommand(%v) = false", args)
		}
	}
	if IsCdeCommand(nil) || IsCdeCommand([]string{"-psn_0_1"}) {
		t.Fatal("a normal launch should not be a cde command")
	}
}

// sprintf avoids importing fmt in the test for one call.
func sprintf(format, value string) string {
	return strings.Replace(format, "%s", value, 1)
}

func TestNewLauncherAppliesFlags(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "claude")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	launcher, ok := NewLauncher(claudeAdapter, []string{"--resume"}, map[string]string{"PATH": dir}, "/app/Code")
	if !ok {
		t.Fatal("no launcher built")
	}
	if launcher.Executable != binary {
		t.Fatalf("executable = %q, want %q", launcher.Executable, binary)
	}
	if launcher.Arguments[0] != "claude" || launcher.Arguments[1] != "--settings" {
		t.Fatalf("arguments = %v", launcher.Arguments)
	}
}

func TestResolveBinarySkipsShimDirectory(t *testing.T) {
	shim := t.TempDir()
	real := t.TempDir()
	if err := os.WriteFile(filepath.Join(shim, "claude"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(real, "claude")
	if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := shim + string(os.PathListSeparator) + real
	got, ok := ResolveBinary("claude", path, shim)
	if !ok || got != target {
		t.Fatalf("ResolveBinary = %q, %v; want %q", got, ok, target)
	}
}
