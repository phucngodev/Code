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
	owner       *WindowState
	// command runs in place of the shell, for a session Code reopens.
	command string

	term     *terminal.Terminal
	startErr error
	// started is set once start has attempted to launch the program, so a
	// failure is not retried every frame. closed keeps close idempotent.
	started  bool
	closed   bool
	exited   bool
	exitCode int

	handle    ui.Handle
	wantFocus bool
}

// LeafID is stable across a relaunch, since agents are told it.
func (s *Surface) LeafID() string { return s.id }

// LeafTitle is the terminal's own title, or empty.
func (s *Surface) LeafTitle() string { return s.title }

// LeafDir is the terminal's working directory, or empty.
func (s *Surface) LeafDir() string { return s.dir }

// start launches the surface's program the first time a view shows it, so a
// restored session does not run a shell per pane before it is looked at.
func (s *Surface) start() {
	if s.term != nil || s.started || s.closed || s.owner == nil {
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
				if set != nil {
					s.dir = set.Dir()
				}
				s.owner.markChanged()
			})
		},
		OnExit: func(code int) {
			s.owner.update(func() {
				s.exited = true
				s.exitCode = code
				s.owner.markChanged()
			})
		},
		OnNotify: func(title, body string) {
			s.owner.update(func() { s.owner.routeNotification(s, body) })
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
	if s.term != nil {
		s.term.Close()
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
