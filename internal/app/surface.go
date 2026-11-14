// Package app wires the settings, the agent socket, the windows and their
// views together, and builds the menus.
package app

import (
	"log"
	"os"
	"strings"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// Surface is one terminal pane: the emulator, its title, its working
// directory, and the handle that keeps its keyboard focus.
type Surface struct {
	id          string
	workspaceID string
	title       string
	dir         string
	// remote is the host an ssh in the terminal connects to, shown as the tab
	// title in its place.
	remote string
	owner  *WindowState
	// command runs in place of the shell, for a session Code reopens.
	command string
	// kind is "" for a terminal, "canvas" for a drawing canvas or "notes"
	// for a Markdown notes view.
	kind  string
	board *boardCanvas
	notes *notesCanvas

	term     *terminal.Terminal
	startErr error
	// shellPid is the shell process found for this surface, once looked up.
	shellPid int
	// foreground is the last foreground process group seen for the shell, so
	// its arguments are only read when a different command takes the terminal.
	foreground int
	// started is set once start has attempted to launch the program, so a
	// failure is not retried every frame. closed keeps close idempotent.
	started bool
	closed  bool

	handle    ui.Handle
	wantFocus bool
}

// LeafID is stable across a relaunch, since agents are told it.
func (s *Surface) LeafID() string { return s.id }

// LeafTitle is the terminal's own title, a canvas says Drawing, a notes
// view says Notes.
func (s *Surface) LeafTitle() string {
	switch s.kind {
	case "canvas":
		return "Drawing"
	case "notes":
		return "Notes"
	}
	return s.title
}

// LeafDir is the terminal's working directory, or empty.
func (s *Surface) LeafDir() string { return s.dir }

// LeafKind is the pane's kind: "" for a terminal, "canvas" or "notes".
func (s *Surface) LeafKind() string { return s.kind }

// paneKind maps a saved layout kind to a surface's kind: a terminal, the
// default, is the empty kind.
func paneKind(kind string) string {
	if kind == "terminal" {
		return ""
	}
	return kind
}

// busy reports whether a command is running in this surface. A pane that runs
// a shell sits at a prompt when the shell is its own terminal's foreground
// group and busy when another group is: a command is running. A pane that runs
// a program of its own (an agent, say) has that program as its foreground
// group, so the group alone cannot tell it from a prompt; it is busy while the
// program runs. A surface whose shell cannot be found is busy, as Ghostty
// confirms when shell integration cannot tell.
func (s *Surface) busy() bool {
	if s.kind != "" || s.term == nil || s.closed {
		return false
	}
	select {
	case <-s.term.Done():
		return false
	default:
	}
	pid, ok := s.shellPID()
	if !ok {
		return true
	}
	// The tracked process is the shell only when the pane runs one; a program
	// the pane runs itself is busy until it ends.
	if !runsShell(pid) {
		return true
	}
	return foregroundPID(pid) != 0
}

// setDir records the directory the shell reported, keeping the last one when a
// report is empty, as a shell that has not reported one yet must not clear
// where Code started the pane.
func (s *Surface) setDir(dir string) {
	if dir != "" {
		s.dir = dir
	}
}

// start launches the surface's program the first time a view shows it, so a
// restored session does not run a shell per pane before it is looked at.
func (s *Surface) start() {
	if s.term != nil || s.started || s.closed || s.owner == nil {
		return
	}
	if s.kind != "" {
		// A canvas or notes view has nothing to launch; it draws instead.
		s.started = true
		return
	}
	s.started = true
	app := s.owner.app
	cfg := app.terminalCfg

	opts := terminal.Options{
		Dir:               validDir(s.dir),
		Env:               app.agentEnv(s),
		Font:              cfg.Font,
		Theme:             cfg.Light,
		DarkTheme:         cfg.Dark,
		Cursor:            cfg.Cursor,
		NoBlink:           cfg.NoBlink,
		Scrollback:        cfg.Scrollback,
		ScrollMultiplier:  cfg.ScrollMultiplier,
		CopyOnSelect:      cfg.CopyOnSelect,
		Padding:           cfg.Padding,
		PaddingColor:      cfg.PaddingColor,
		OptionAsAlt:       cfg.OptionAsAlt,
		Keybinds:          terminalKeybinds(app.keybinds),
		NoDefaultKeybinds: app.keybindsCleared,
		RightClick:        cfg.RightClick,
		NoClipboardWrite:  cfg.NoClipboardWrite,
		ClipboardRead:     cfg.ClipboardRead,
		Transparent:       cfg.Transparent,
		MinimumContrast:   cfg.MinimumContrast,
		ImageLimit:        cfg.ImageLimit,
		ShellIntegration:  app.settings.Bool("shell-integration", true),
		OnKey: func(mods ui.Modifiers, key ui.Key) bool {
			if s.owner == nil {
				return false
			}
			return s.owner.app.runAppKeybind(s.owner, mods, key)
		},
		OnTitle: func(title string) {
			set := s.term
			s.owner.update(func() {
				s.title = title
				// A shell that sets its title before reporting a directory, or
				// one whose shell integration is off, must not clear where Code
				// started the pane: keep the last directory until a new one comes.
				if set != nil {
					s.setDir(set.Dir())
				}
				s.owner.markChanged()
			})
		},
		OnExit: func(_ int) {
			// A surface whose program ended is closed, as Ghostty does when a
			// shell logs out, so its pane does not linger on the exit message.
			s.owner.update(func() { s.owner.surfaceExited(s) })
		},
		OnNotify: func(title, body string) {
			s.owner.update(func() { s.owner.routeNotification(s, title, body) })
		},
		// Search runs across frames of scrollback; OnSearch is the only way a
		// find bar's "1 of 3" follows it.
		OnSearch: func(status terminal.SearchStatus) {
			s.owner.update(func() {
				if s.owner.focusedSurface() == s {
					s.owner.find.status = status
					s.owner.markChanged()
				}
			})
		},
	}
	if cfg.Command != "" {
		opts.Command = splitCommand(cfg.Command)
	} else if s.command != "" {
		opts.Command = []string{loginShell(), "-lic", s.command}
	}
	term, err := terminal.New(opts)
	if err != nil {
		s.startErr = err
		log.Printf("codeterm: could not start terminal %s: %v", s.id, err)
		return
	}
	s.term = term
	s.owner.app.addLive(s)
	s.owner.app.watchShell(s)
}

// applyConfig updates a running terminal after the settings changed.
func (s *Surface) applyConfig() {
	if s.term == nil {
		return
	}
	cfg := s.owner.app.terminalCfg
	s.term.SetFont(cfg.Font)
	s.term.SetTheme(cfg.Light, cfg.Dark)
	s.term.SetCopyOnSelect(cfg.CopyOnSelect)
	s.term.SetScrollMultiplier(cfg.ScrollMultiplier)
	s.term.SetMinimumContrast(cfg.MinimumContrast)
}

// close hangs up the program and frees the terminal. It is safe to call more
// than once.
func (s *Surface) close() {
	if s.closed {
		return
	}
	s.closed = true
	if s.board != nil {
		s.board.save()
		s.board.close()
		s.board = nil
	}
	if s.notes != nil {
		s.notes.save()
		s.notes.close()
		s.notes = nil
	}
	if s.term != nil {
		if err := s.term.Close(); err != nil {
			log.Printf("codeterm: could not close terminal %s: %v", s.id, err)
		}
	}
	if s.owner != nil {
		s.owner.app.removeShims(s.id)
		s.owner.app.removeLive(s.id)
	}
}

// validDir returns dir when it is a directory, else the user's home, so a
// config or session carried from another machine does not stop a terminal from
// starting where the path is missing.
func validDir(dir string) string {
	if dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
}

// loginShell is the user's login shell, so a resumed session finds the agent
// on the PATH its profile builds.
func loginShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	if info, err := os.Stat(shell); err == nil && !info.IsDir() {
		return shell
	}
	return "/bin/sh"
}

// splitCommand splits a config command line into a program and its arguments,
// honouring single and double quotes.
func splitCommand(command string) []string {
	var out []string
	var current strings.Builder
	quote := rune(0)
	started := false
	for _, r := range command {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			started = true
		case r == ' ' || r == '\t':
			if started {
				out = append(out, current.String())
				current.Reset()
				started = false
			}
		default:
			current.WriteRune(r)
			started = true
		}
	}
	if started {
		out = append(out, current.String())
	}
	return out
}
