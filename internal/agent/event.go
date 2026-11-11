// Package agent tracks what a coding agent running in a Code terminal is
// doing, from the events the cde tool reports over a Unix socket.
package agent

import (
	"encoding/json"
	"time"
)

// Event is what a coding agent running in a Code terminal reports about
// itself. It travels as one JSON object per line over the agent socket.
type Event struct {
	// Version is the protocol version; newer senders may add fields.
	Version int `json:"v"`
	// Agent is shown to the user only; Code never branches on it.
	Agent string `json:"agent"`
	// Kind is the only required field.
	Kind Kind `json:"kind"`
	// SessionID identifies the agent's session.
	SessionID string `json:"sessionID,omitempty"`
	// SurfaceID is the terminal the event is about.
	SurfaceID string `json:"surfaceID,omitempty"`
	// WorkspaceID is the workspace the terminal belongs to.
	WorkspaceID string `json:"workspaceID,omitempty"`
	// Cwd is where the agent runs.
	Cwd string `json:"cwd,omitempty"`
	// At orders events, since hooks race each other to the socket.
	At time.Time `json:"at"`
	// Detail is the tool for working, the reason for needsInput, the last
	// message for turnEnded, the message for attention.
	Detail string `json:"detail,omitempty"`
}

// Kind is the kind of an agent event.
type Kind string

// Event kinds.
const (
	KindSessionStarted Kind = "sessionStarted"
	KindTurnStarted    Kind = "turnStarted"
	KindWorking        Kind = "working"
	KindNeedsInput     Kind = "needsInput"
	KindTurnEnded      Kind = "turnEnded"
	KindSessionEnded   Kind = "sessionEnded"
	KindAttention      Kind = "attention"
	// KindUnknown is a kind from a newer sender, which is skipped.
	KindUnknown Kind = "unknown"
)

// UnmarshalJSON reads only the fields this build knows.
func (e *Event) UnmarshalJSON(data []byte) error {
	type raw struct {
		Version     int             `json:"v"`
		Agent       string          `json:"agent"`
		Kind        string          `json:"kind"`
		SessionID   string          `json:"sessionID"`
		SurfaceID   string          `json:"surfaceID"`
		WorkspaceID string          `json:"workspaceID"`
		Cwd         string          `json:"cwd"`
		At          json.RawMessage `json:"at"`
		Detail      string          `json:"detail"`
	}
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	e.Version = r.Version
	if e.Version == 0 {
		e.Version = 1
	}
	e.Agent = r.Agent
	e.Kind = Kind(r.Kind)
	switch e.Kind {
	case KindSessionStarted, KindTurnStarted, KindWorking, KindNeedsInput, KindTurnEnded, KindSessionEnded, KindAttention:
	default:
		e.Kind = KindUnknown
	}
	e.SessionID = r.SessionID
	e.SurfaceID = r.SurfaceID
	e.WorkspaceID = r.WorkspaceID
	e.Cwd = r.Cwd
	e.Detail = r.Detail
	e.At = parseTime(r.At)
	return nil
}

func parseTime(raw json.RawMessage) time.Time {
	if len(raw) == 0 {
		return time.Now()
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return time.Now()
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.999999999Z0700"} {
		if t, err := time.Parse(layout, text); err == nil {
			return t
		}
	}
	return time.Now()
}

// Line encodes the event as one JSON line, newline included.
func (e Event) Line() ([]byte, error) {
	if e.Version == 0 {
		e.Version = 1
	}
	data, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// ParseLine reads one JSON line into an event.
func ParseLine(line []byte) (Event, error) {
	var e Event
	if err := json.Unmarshal(line, &e); err != nil {
		return Event{}, err
	}
	return e, nil
}
