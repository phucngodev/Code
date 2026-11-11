package code

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"unicode/utf8"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

// detailLimit matches the length Code keeps for a sidebar detail.
const detailLimit = 200

// PiG runs one process per session, and session_start resets it, so one set of
// turn state is enough. Handlers run on their own goroutines, so the state and
// the cde calls need a lock.
var (
	stateMu   sync.Mutex
	sessionID string
	lastText  string
	lastErr   string

	sendMu sync.Mutex
)

func Extension() *sdk.Extension {
	ext := sdk.New("code")

	ext.OnSessionStart(func(ctx sdk.Context, _ map[string]any) (any, error) {
		// Cached here because a ui_prompt handler is not awaited and its
		// context can be cancelled before send runs.
		id, err := ctx.GetSessionID()
		if err != nil {
			id = ""
		}
		stateMu.Lock()
		sessionID, lastText, lastErr = id, "", ""
		stateMu.Unlock()
		send("sessionStarted", "")
		return nil, nil
	})

	ext.OnEvent(sdk.EventBeforeAgentStart, func(_ sdk.Context, event map[string]any) (any, error) {
		// A run started by a custom message is not a turn the user asked for.
		if messageType, _ := event["messageType"].(string); messageType == "user" {
			send("turnStarted", "")
		}
		return nil, nil
	})

	ext.OnEvent(sdk.EventToolExecutionStart, func(_ sdk.Context, event map[string]any) (any, error) {
		tool, _ := event["toolName"].(string)
		send("working", tool)
		return nil, nil
	})

	// PiG reports its own UI prompts, so Code can tell waiting on the user
	// from working.
	ext.OnEvent(sdk.EventUIPromptStart, func(_ sdk.Context, event map[string]any) (any, error) {
		detail, _ := event["title"].(string)
		if detail == "" {
			detail, _ = event["kind"].(string)
		}
		send("needsInput", detail)
		return nil, nil
	})

	ext.OnEvent(sdk.EventUIPromptEnd, func(_ sdk.Context, _ map[string]any) (any, error) {
		send("working", "")
		return nil, nil
	})

	// A run may retry, compact or continue, so only agent_settled means the
	// turn is over. The last assistant message is the turn's detail, or the
	// error that ended it.
	ext.OnEvent(sdk.EventMessageEnd, func(_ sdk.Context, event map[string]any) (any, error) {
		message, _ := event["message"].(map[string]any)
		if message == nil || message["role"] != "assistant" {
			return nil, nil
		}
		if message["stopReason"] == "error" {
			stateMu.Lock()
			lastErr = stringOr(message["errorMessage"], "error")
			stateMu.Unlock()
			return nil, nil
		}
		if text := assistantText(message); text != "" {
			stateMu.Lock()
			lastText, lastErr = text, ""
			stateMu.Unlock()
		}
		return nil, nil
	})

	ext.OnEvent(sdk.EventAgentSettled, func(_ sdk.Context, event map[string]any) (any, error) {
		aborted, _ := event["aborted"].(bool)
		stateMu.Lock()
		detail := settledDetail(aborted, lastErr, lastText)
		lastText, lastErr = "", ""
		stateMu.Unlock()
		send("turnEnded", detail)
		return nil, nil
	})

	ext.OnSessionShutdown(func(_ sdk.Context, event map[string]any) (any, error) {
		if reason, _ := event["reason"].(string); reason == "quit" {
			send("sessionEnded", "")
		}
		return nil, nil
	})

	return ext
}

// settledDetail is what Code shows for a finished turn: the error that ended
// it, the last assistant message, or nothing when the user aborted the run,
// whose partial message would read as the turn's answer.
func settledDetail(aborted bool, err, text string) string {
	if aborted {
		return ""
	}
	if err != "" {
		return err
	}
	return text
}

// send reports one event to Code. Handlers run concurrently, so the mutex
// keeps two cde processes from starting together: Code orders events by when
// each starts.
func send(kind, detail string) {
	cde := os.Getenv("CODE_CDE")
	if cde == "" {
		return
	}
	stateMu.Lock()
	session := sessionID
	stateMu.Unlock()
	args := []string{"event", kind, "--agent", "pig"}
	if session != "" {
		args = append(args, "--session", session)
	}
	if detail = summary(detail); detail != "" {
		args = append(args, "--detail", detail)
	}
	sendMu.Lock()
	defer sendMu.Unlock()
	// cde event never fails the agent, so a failed call is dropped.
	_ = exec.Command(cde, args...).Run()
}

// summary folds a message onto one line short enough to show beside a
// workspace.
func summary(text string) string {
	line := strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(line) > detailLimit {
		return string([]rune(line)[:detailLimit-1]) + "…"
	}
	return line
}

// assistantText joins the text blocks of an assistant message.
func assistantText(message map[string]any) string {
	blocks, _ := message["content"].([]any)
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		entry, _ := block.(map[string]any)
		if entry["type"] != "text" {
			continue
		}
		if text, _ := entry["text"].(string); strings.TrimSpace(text) != "" {
			parts = append(parts, strings.TrimSpace(text))
		}
	}
	return strings.Join(parts, " ")
}

func stringOr(value any, fallback string) string {
	if text, _ := value.(string); text != "" {
		return text
	}
	return fallback
}
