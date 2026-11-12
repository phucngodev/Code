// Package history finds the agent sessions kept on disk, so the sidebar can
// offer to reopen one. Code only ever reads the agents' transcripts.
package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Session is one agent session on disk that Code can offer to reopen.
type Session struct {
	// ID is the agent's own id for the session, unique within its directory.
	ID string
	// Agent that wrote it, such as "claude".
	Agent string
	// Cwd is where the session ran, so a new terminal can start there.
	Cwd string
	// Title is the first thing the user asked, or the agent's own summary.
	Title    string
	Modified time.Time
	// Resume is what resumes it, or nil when Code has none for the agent.
	Resume *Resume
	// search is the lowercased title and directory, computed once so a
	// sidebar filter does not lowercase every session every frame.
	search string
}

// Resume is the invocation that resumes a session in the agent's own CLI.
type Resume struct {
	Binary    string
	Arguments []string
}

// DisplayTitle is the title, or the directory when the transcript held no
// prompt.
func (s Session) DisplayTitle() string {
	if s.Title == "" {
		return AbbreviateHome(s.Cwd)
	}
	return s.Title
}

// Root is one place transcripts live, and the agent that wrote them.
type Root struct {
	Dir   string
	Agent string
}

const (
	// headBytes is enough of a transcript to hold the first prompt and the
	// working directory, without reading a long conversation to the end.
	headBytes = 262_144
	// DefaultLimit caps how many sessions a scan keeps.
	DefaultLimit = 200
)

// DefaultRoots returns the directories agents keep transcripts in.
func DefaultRoots() []Root {
	home, _ := os.UserHomeDir()
	return []Root{
		{filepath.Join(home, ".claude", "projects"), "claude"},
		{filepath.Join(home, ".codex", "sessions"), "codex"},
		{filepath.Join(home, ".pi", "agent", "sessions"), "pi"},
		{filepath.Join(home, ".config", "pig", "agent", "sessions"), "pig"},
	}
}

// transcript is a JSONL file found under an agent's root.
type transcript struct {
	path  string
	agent string
	mod   time.Time
}

// Scan finds the newest transcripts under the roots, capped at limit. It stats
// every file but reads only the ones it can keep: an agent may have thousands
// of transcripts, and each used to cost a 256 KB read.
func Scan(roots []Root, limit int) []Session {
	if limit <= 0 {
		limit = DefaultLimit
	}
	var files []transcript
	for _, root := range roots {
		filepath.WalkDir(root.Dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				if strings.HasPrefix(entry.Name(), ".") && path != root.Dir {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".jsonl" {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return nil
			}
			files = append(files, transcript{path: path, agent: root.Agent, mod: info.ModTime()})
			return nil
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.After(files[j].mod) })
	sessions := make([]Session, 0, limit)
	for _, file := range files {
		if len(sessions) >= limit {
			break
		}
		if session, ok := Read(file.path, file.agent); ok {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// Read parses one transcript's session, or reports false when it holds no
// working directory.
func Read(path, agent string) (Session, bool) {
	file, err := os.Open(path)
	if err != nil {
		return Session{}, false
	}
	defer file.Close()
	// Read lines only until the head holds what the sidebar needs, instead of
	// pulling a fixed block from every transcript.
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 4096), headBytes)
	var parsed parsedTranscript
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var object map[string]any
		if err := json.Unmarshal([]byte(line), &object); err != nil {
			continue
		}
		parseFields(object, &parsed)
		if parsed.cwd != "" && parsed.title != "" {
			break
		}
	}
	if parsed.cwd == "" {
		return Session{}, false
	}
	id := parsed.id
	if id == "" {
		id = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	var modified time.Time
	if info, err := file.Stat(); err == nil {
		modified = info.ModTime()
	}
	return Session{
		ID:       id,
		Agent:    agent,
		Cwd:      parsed.cwd,
		Title:    parsed.title,
		Modified: modified,
		Resume:   resume(agent, id),
		search:   strings.ToLower(parsed.title + " " + parsed.cwd),
	}, true
}

type parsedTranscript struct {
	cwd   string
	title string
	id    string
}

// parse reads a transcript's head. A transcript is one JSON object per line; a
// partial last line, which only the head cut off, is ignored.
func parse(data []byte) parsedTranscript {
	var parsed parsedTranscript
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var object map[string]any
		if err := json.Unmarshal([]byte(line), &object); err != nil {
			continue
		}
		parseFields(object, &parsed)
	}
	return parsed
}

// parseFields folds one transcript line into what the sidebar needs.
func parseFields(object map[string]any, parsed *parsedTranscript) {
	if parsed.id == "" {
		if kind, _ := object["type"].(string); kind == "session" {
			if value, _ := object["id"].(string); value != "" {
				parsed.id = value
			}
		}
	}
	if parsed.cwd == "" {
		if value, _ := object["cwd"].(string); value != "" {
			parsed.cwd = value
		}
	}
	if parsed.title == "" {
		parsed.title = prompt(object)
	}
}

// resume is the invocation that resumes a session in the agent's own CLI.
func resume(agent, id string) *Resume {
	if id == "" || !safeID(id) {
		return nil
	}
	switch agent {
	case "claude":
		return &Resume{Binary: "claude", Arguments: []string{"--resume", id}}
	case "pi":
		return &Resume{Binary: "pi", Arguments: []string{"--session", id}}
	case "pig":
		return &Resume{Binary: "pig", Arguments: []string{"--session", id}}
	default:
		return nil
	}
}

func safeID(id string) bool {
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// Filter returns the sessions whose title or directory contains query.
func Filter(sessions []Session, query string) []Session {
	query = strings.TrimSpace(query)
	if query == "" {
		return sessions
	}
	lower := strings.ToLower(query)
	var out []Session
	for _, session := range sessions {
		hay := session.search
		if hay == "" {
			hay = strings.ToLower(session.Title + " " + session.Cwd)
		}
		if strings.Contains(hay, lower) {
			out = append(out, session)
		}
	}
	return out
}

// prompt is the first user prompt in one transcript line, across the shapes the
// agents use.
func prompt(object map[string]any) string {
	if summary, ok := object["summary"].(string); ok {
		if text, ok := oneLine(summary); ok {
			return text
		}
	}
	if payload, ok := object["payload"].(map[string]any); ok {
		if text, ok := messageText(payload["content"]); ok {
			return text
		}
		if message, ok := payload["message"].(map[string]any); ok {
			if text, ok := messageText(message["content"]); ok {
				return text
			}
		}
	}
	if message, ok := object["message"].(map[string]any); ok {
		switch object["type"] {
		case "user":
			if text, ok := messageText(message["content"]); ok {
				return text
			}
		case "message":
			if role, _ := message["role"].(string); role == "user" {
				if text, ok := messageText(message["content"]); ok {
					return text
				}
			}
		}
	}
	if role, _ := object["role"].(string); role == "user" {
		if text, ok := messageText(object["content"]); ok {
			return text
		}
	}
	return ""
}

func messageText(content any) (string, bool) {
	if text, ok := content.(string); ok {
		return oneLine(text)
	}
	if parts, ok := content.([]any); ok {
		for _, part := range parts {
			entry, _ := part.(map[string]any)
			if text, ok := entry["text"].(string); ok {
				if line, ok := oneLine(text); ok {
					return line, true
				}
			}
		}
	}
	return "", false
}

// oneLine folds text onto one line, capped so a pasted prompt stays a row's
// worth.
func oneLine(text string) (string, bool) {
	collapsed := strings.Join(strings.Fields(text), " ")
	if collapsed == "" {
		return "", false
	}
	runes := []rune(collapsed)
	if len(runes) > 120 {
		return string(runes[:119]) + "…", true
	}
	return collapsed, true
}

// AbbreviateHome turns the user's home prefix into "~".
func AbbreviateHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):]
	}
	return path
}
