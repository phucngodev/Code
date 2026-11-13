package agent

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// maximumHookPayload bounds the JSON an agent's hook may send on stdin.
const maximumHookPayload = 1 << 20

const usageText = `usage: cde event <kind> [--agent NAME] [--session ID] [--detail TEXT]
                        [--cwd PATH] [--surface UUID] [--workspace UUID]
       cde hook <agent>              (reads the agent's hook JSON on stdin)
       cde launch <agent> [args...]
       cde shims <dir>
       cde agents refresh
       cde ping

kinds: sessionStarted turnStarted working needsInput turnEnded sessionEnded attention`

// IsCdeCommand reports whether args begin with a cde subcommand, so the app
// binary can act as the helper when an agent calls it.
func IsCdeCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "event", "hook", "launch", "shims", "agents", "ping":
		return true
	}
	return false
}

// DefaultSocketDir is where a running Code keeps its agent socket, matching
// the app's own path.
func DefaultSocketDir() string {
	home, _ := os.UserHomeDir()
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "code")
}

// RunCde runs the cde helper and returns its exit code. Apart from ping,
// launch and shims it always returns 0: a Code that is closed or confused must
// never fail or slow down the agent.
func RunCde(args []string, environ []string, stdin io.Reader, stdout, stderr io.Writer) int {
	environment := environmentMap(environ)
	now := time.Now()
	socket := environment[EnvSocket]
	cde := environment[EnvCde]
	if cde == "" {
		cde, _ = os.Executable()
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, usageText)
		return 2
	}
	switch args[0] {
	case "ping":
		path := resolveSocket(socket, environment)
		if path == "" {
			fmt.Fprintln(stderr, "cde: no Code is running")
			return 1
		}
		fmt.Fprintln(stdout, path)
		conn, err := net.DialTimeout("unix", path, 250*time.Millisecond)
		if err != nil {
			return 1
		}
		conn.Close()
		return 0
	case "event":
		event, err := eventFromArgs(args[1:], environment, now)
		if err != nil {
			return 0
		}
		sendEvent(event, socket, environment)
		return 0
	case "hook":
		if len(args) < 2 {
			return 0
		}
		adapter, ok := AdapterNamed(args[1])
		if !ok {
			return 0
		}
		data, _ := io.ReadAll(io.LimitReader(stdin, maximumHookPayload))
		payload, ok := ParseHookPayload(data, adapter.Name, environment, now)
		if !ok {
			return 0
		}
		if adapter.Translate != nil {
			if event := adapter.Translate(payload); event != nil {
				sendEvent(*event, socket, environment)
			}
		}
		return 0
	case "launch":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "cde: usage: cde launch <agent> [args...]")
			return 127
		}
		adapter, ok := AdapterNamed(args[1])
		if !ok {
			fmt.Fprintf(stderr, "cde: no agent named %s\n", args[1])
			return 127
		}
		launcher, ok := NewLauncher(adapter, args[2:], environment, cde)
		if !ok {
			fmt.Fprintf(stderr, "%s: command not found\n", adapter.Binary)
			return 127
		}
		if err := launcher.Exec(); err != nil {
			fmt.Fprintf(stderr, "cde: %v\n", err)
			return 126
		}
		return 0
	case "shims":
		if len(args) < 2 {
			return 1
		}
		if err := InstallShims(args[1], cde, Adapters()); err != nil {
			fmt.Fprintf(stderr, "cde: %v\n", err)
			return 1
		}
		return 0
	case "agents":
		if len(args) >= 2 && args[1] == "refresh" {
			fmt.Fprintln(stdout, refreshScript(environment))
			return 0
		}
	}
	fmt.Fprintln(stderr, usageText)
	return 2
}

func sendEvent(event Event, socket string, environment map[string]string) {
	path := resolveSocket(socket, environment)
	if path == "" {
		return
	}
	line, err := event.Line()
	if err != nil {
		return
	}
	if len(line) > maximumLineLength {
		event.Detail = Summarize(event.Detail)
		if line, err = event.Line(); err != nil || len(line) > maximumLineLength {
			return
		}
	}
	conn, err := net.DialTimeout("unix", path, 250*time.Millisecond)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetWriteDeadline(time.Now().Add(250 * time.Millisecond))
	_, _ = conn.Write(line)
}

// resolveSocket is CODE_SOCKET, or the socket of the Code started last.
func resolveSocket(socket string, environment map[string]string) string {
	if socket != "" {
		return socket
	}
	dir := environment["CODE_SOCKET_DIR"]
	if dir == "" {
		dir = DefaultSocketDir()
	}
	return NewestSocket(dir)
}

func eventFromArgs(args []string, environment map[string]string, now time.Time) (Event, error) {
	if len(args) == 0 {
		return Event{}, fmt.Errorf("missing event kind")
	}
	kind := Kind(args[0])
	switch kind {
	case KindSessionStarted, KindTurnStarted, KindWorking, KindNeedsInput, KindTurnEnded, KindSessionEnded, KindAttention:
	default:
		return Event{}, fmt.Errorf("unknown event kind %s", args[0])
	}
	event := Event{Version: 1, Kind: kind, At: now, Cwd: environment["PWD"]}
	cwd, _ := os.Getwd()
	if cwd != "" {
		event.Cwd = cwd
	}
	rest := args[1:]
	for len(rest) >= 2 {
		flag, value := rest[0], rest[1]
		rest = rest[2:]
		switch flag {
		case "--agent":
			event.Agent = value
		case "--session":
			event.SessionID = value
		case "--detail":
			event.Detail = value
		case "--cwd":
			event.Cwd = value
		case "--surface":
			event.SurfaceID = value
		case "--workspace":
			event.WorkspaceID = value
		default:
			return Event{}, fmt.Errorf("unknown option %s", flag)
		}
	}
	if len(rest) != 0 {
		return Event{}, fmt.Errorf("missing value for %s", rest[0])
	}
	if event.SurfaceID == "" {
		event.SurfaceID = environment[EnvSurfaceID]
	}
	if event.WorkspaceID == "" {
		event.WorkspaceID = environment[EnvWorkspaceID]
	}
	return event, nil
}

func refreshScript(environment map[string]string) string {
	posix := `dir="$CODE_SHIMS/$CODE_SURFACE_ID"` + "\n" +
		`"$CODE_CDE" shims "$dir" && PATH="$dir:$PATH"`
	if filepath.Base(environment["SHELL"]) == "fish" {
		return "set dir $CODE_SHIMS/$CODE_SURFACE_ID\n" +
			`"$CODE_CDE" shims $dir; and set -gx --prepend PATH $dir`
	}
	return posix
}

func environmentMap(environ []string) map[string]string {
	out := make(map[string]string, len(environ))
	for _, entry := range environ {
		if index := strings.IndexByte(entry, '='); index >= 0 {
			out[entry[:index]] = entry[index+1:]
		}
	}
	return out
}
