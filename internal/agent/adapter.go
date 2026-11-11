package agent

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
)

// detailLimit is the longest a detail Code keeps for a sidebar.
const detailLimit = 200

// Environment variable names Code sets for the agents it starts.
const (
	EnvSurfaceID      = "CODE_SURFACE_ID"
	EnvWorkspaceID    = "CODE_WORKSPACE_ID"
	EnvSocket         = "CODE_SOCKET"
	EnvCde            = "CODE_CDE"
	EnvShims          = "CODE_SHIMS"
	EnvShimDir        = "CODE_SHIM_DIR"
	EnvAgentsDisabled = "CODE_AGENTS_DISABLED"
	EnvAgentsDir      = "CODE_AGENTS_DIR"
)

// LaunchKind says how an adapter changes an agent's launch.
type LaunchKind int

const (
	// LaunchNone leaves the agent's arguments and environment alone.
	LaunchNone LaunchKind = iota
	// LaunchFlags rewrites the command-line arguments.
	LaunchFlags
	// LaunchEnvironment sets variables over the environment.
	LaunchEnvironment
)

// Adapter is everything Code knows about one coding agent: how to start it so
// that it reports to Code, and how to read what its hooks report.
type Adapter struct {
	// Name is also the label Code shows for the agent.
	Name string
	// Binary is the executable looked up on PATH.
	Binary string
	Kind   LaunchKind
	// Arguments rewrites the arguments the user typed (LaunchFlags).
	Arguments func(arguments []string, cde string) []string
	// Environment returns variables to set over the user's (LaunchEnvironment).
	Environment func(environment map[string]string, cde string) map[string]string
	// Translate returns nil for a hook call Code has no use for.
	Translate func(HookPayload) *Event
	// TranslateNotification reads a desktop notification the agent asked its
	// terminal for. An agent without it has its notifications posted as the
	// terminal's.
	TranslateNotification func(message string) (kind Kind, detail string, ok bool)
}

// NeedsShim reports whether Code must stand in for the binary on PATH.
func (a Adapter) NeedsShim() bool { return a.Kind != LaunchNone }

var adapters = []Adapter{claudeAdapter, openCodeAdapter, codexAdapter, piAdapter, pigAdapter}

// Adapters returns the supported agents. It is a copy, so a caller cannot
// change the set the app uses.
func Adapters() []Adapter {
	out := make([]Adapter, len(adapters))
	copy(out, adapters)
	return out
}

// AdapterNamed finds an adapter by name.
func AdapterNamed(name string) (Adapter, bool) {
	for _, a := range adapters {
		if a.Name == name {
			return a, true
		}
	}
	return Adapter{}, false
}

// HookPayload is the JSON object an agent hands its hook on stdin, together
// with the Code terminal the hook runs in.
type HookPayload struct {
	fields      map[string]any
	agent       string
	surfaceID   string
	workspaceID string
	at          time.Time
}

// ParseHookPayload reads a hook's JSON, or reports false when it is not an
// object.
func ParseHookPayload(data []byte, agent string, environment map[string]string, now time.Time) (HookPayload, bool) {
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return HookPayload{}, false
	}
	return HookPayload{
		fields:      fields,
		agent:       agent,
		surfaceID:   environment[EnvSurfaceID],
		workspaceID: environment[EnvWorkspaceID],
		at:          now,
	}, true
}

// String returns a string field.
func (p HookPayload) String(key string) string {
	value, _ := p.fields[key].(string)
	return value
}

// Object returns an object field.
func (p HookPayload) Object(key string) map[string]any {
	value, _ := p.fields[key].(map[string]any)
	return value
}

// Has reports whether a field is present and non-nil.
func (p HookPayload) Has(key string) bool {
	value, ok := p.fields[key]
	return ok && value != nil
}

// Summary returns a message field folded onto one line.
func (p HookPayload) Summary(key string) string {
	value, _ := p.fields[key].(string)
	return Summarize(value)
}

// Event builds an event for this payload's terminal.
func (p HookPayload) Event(kind Kind, sessionID, cwd, detail string) *Event {
	return &Event{
		Version:     1,
		Agent:       p.agent,
		Kind:        kind,
		SessionID:   sessionID,
		SurfaceID:   p.surfaceID,
		WorkspaceID: p.workspaceID,
		Cwd:         cwd,
		At:          p.at,
		Detail:      detail,
	}
}

// Summarize folds a message onto one line no longer than the detail limit.
func Summarize(message string) string {
	fields := strings.Fields(message)
	line := strings.Join(fields, " ")
	if utf8.RuneCountInString(line) > detailLimit {
		return string([]rune(line)[:detailLimit-1]) + "…"
	}
	return line
}

// shellQuote makes a string one word for sh, whatever it contains.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
