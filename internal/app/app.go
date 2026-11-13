package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/agent"
	"github.com/phucngodev/code/internal/config"
	"github.com/phucngodev/code/internal/history"
	"github.com/phucngodev/code/internal/model"
)

// App is the whole application: its settings, its agent socket, and its
// windows. Except for the fields guarded by mu, shellMu and saveMu, every
// field is read and written on the main thread only.
type App struct {
	paths      config.Paths
	settings   *config.Settings
	agents     *agent.Store
	listener   *agent.Listener
	socketPath string
	menu       *mygo.Menu

	windows    []*WindowState
	byWindow   map[*mygo.Window]*WindowState
	closedLast *model.WindowSession
	// quitConfirmed is set once the user answered the quit confirmation, so
	// the quit that runs then closes the windows without asking again.
	quitConfirmed bool

	mu            sync.Mutex
	surfaceOwners map[string]*WindowState
	saveMu        sync.Mutex
	saveTimer     *time.Timer
	// workAreas are the displays' usable areas, read once when the toolkit is
	// ready, so a restored window frame can be checked before it is used.
	workAreas []mygo.Rectangle
	// historyMu guards the agent session scan shared by every window, so
	// opening a second window does not walk the transcripts again.
	historyMu       sync.Mutex
	historySessions []history.Session
	historyLoading  bool
	historyLoaded   bool
	// writer saves the session off the main thread. dirStop ends the directory
	// watcher.
	writer  *sessionWriter
	dirStop chan struct{}
	// notifications keeps the alert shown for each terminal, so it can be
	// replaced or closed rather than left registered.
	notifications map[string]*mygo.Notification
	// sidebarHidden and sidebarWidth are the sidebar's state, shared by every
	// window and saved in the session.
	sidebarHidden bool
	sidebarWidth  float32

	// cdePath is the app binary, which also acts as the cde helper agents
	// call back through. shimsRoot holds the per-terminal PATH shims, and
	// assetsDir the agent resources (OpenCode plugins, Pi and PiG extensions).
	cdePath   string
	shimsRoot string
	assetsDir string

	settingsWin   *mygo.Window
	settingsState *settingsWindow
	shortcutsWin  *mygo.Window

	// keybinds are the config's keybind lines, parsed when the settings load
	// or change, so the view does not parse them every frame. keybindsCleared
	// is whether `keybind = clear` dropped the terminal's default bindings.
	keybinds        []config.Keybind
	keybindsCleared bool
	// shellMu guards the shell lookup for surfaces: shellQueue holds the
	// surfaces still waiting for their shell process, shellKnown the child
	// processes already seen, and shellWatching whether a watcher runs.
	shellMu       sync.Mutex
	shellQueue    []*Surface
	shellKnown    map[int]bool
	shellWatching bool
	// chromeLight and chromeDark are the widget themes derived from the
	// terminals' light and dark themes, so the chrome matches the terminal.
	// terminalCfg is the parsed settings the views read instead of re-reading
	// the theme files every frame.
	chromeLight *ui.Theme
	chromeDark  *ui.Theme
	terminalCfg config.TerminalConfig
}

// New builds the application, starts the agent socket and schedules the
// windows to open once the toolkit is ready.
func New() *App {
	paths := config.DefaultPaths()
	if err := config.CreateIfMissing(paths); err != nil {
		log.Printf("codeterm: could not create the config file: %v", err)
	}
	if err := terminal.Load(); err != nil {
		log.Printf("codeterm: could not load libghostty-vt: %v", err)
	}
	a := &App{
		paths:         paths,
		settings:      config.LoadSettings(paths.Config),
		writer:        newSessionWriter(paths.Session),
		agents:        agent.NewStore(),
		byWindow:      map[*mygo.Window]*WindowState{},
		surfaceOwners: map[string]*WindowState{},
		notifications: map[string]*mygo.Notification{},
		sidebarHidden: true,
		sidebarWidth:  240,
	}
	a.settings.OnSave = func() { mygo.RunOnMain(a.applyConfig) }
	a.reloadKeybinds()
	a.rebuildChromeThemes()
	a.setUpAgents()
	a.agents.OnChange = func(id string) {
		mygo.RunOnMain(func() {
			if w := a.ownerOf(id); w != nil {
				w.win.Invalidate()
			}
			a.invalidateAll()
		})
	}
	a.agents.OnUnreadChange = func() {
		a.invalidateAll()
		a.updateBadge()
	}
	a.agents.OnUpdate = a.handleAgentUpdate

	agent.RemoveStaleSockets(paths.SocketDir)
	a.socketPath = agent.SocketPath(paths.SocketDir, os.Getpid())
	a.listener = agent.NewListener(a.socketPath, a.deliverAgentEvent)
	if err := a.listener.Start(); err != nil {
		log.Printf("codeterm: agent socket unavailable at %s: %v", a.socketPath, err)
	}

	mygo.App.WhenReady(a.ready)
	return a
}

func (a *App) ready() {
	mygo.App.SetName("Code")
	a.menu = a.buildMenu()
	mygo.App.SetMenu(a.menu)
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { a.shutdown() })
	mygo.App.OnBeforeQuit(a.beforeQuit)
	mygo.App.OnQuitCanceled(func() { a.quitConfirmed = false })
	for _, display := range mygo.Screen.Displays() {
		a.workAreas = append(a.workAreas, display.WorkArea)
	}
	a.startDirWatcher()
	a.restoreWindows()
	mygo.App.OnActivate(func(visible bool) {
		if len(a.windows) == 0 {
			a.newWindow(nil)
		}
	})
}

// BringToFront shows a window of this instance, opening one when there is
// none. A second launch of Code forwards to the running instance, which then
// raises a window rather than starting another process that shares the config
// and session files.
func (a *App) BringToFront() {
	if len(a.windows) == 0 {
		a.newWindow(nil)
		return
	}
	if w := a.windows[len(a.windows)-1]; w != nil && w.win != nil {
		w.win.Show()
		w.win.Focus()
	}
}

// restoreWindows rebuilds the saved windows, or opens one when none is saved.
func (a *App) restoreWindows() {
	session, err := model.LoadSession(a.paths.Session)
	if err == nil && session != nil {
		if session.SidebarHidden != nil {
			a.sidebarHidden = *session.SidebarHidden
		}
		if session.SidebarWidth > 0 {
			a.sidebarWidth = session.SidebarWidth
		}
		for i := range session.Windows {
			a.newWindow(&session.Windows[i])
		}
	}
	if len(a.windows) == 0 {
		a.newWindow(nil)
	}
	for _, w := range a.windows {
		w.win.Show()
	}
}

// newWindow makes a window, restoring its workspaces when a session is given.
func (a *App) newWindow(frame *model.WindowSession) *WindowState {
	store := model.NewStore[*Surface]()
	w := &WindowState{
		app:           a,
		store:         store,
		sidebarWidth:  a.sidebarWidth,
		sidebarHidden: a.sidebarHidden,
	}
	store.OnChange = w.onStoreChange

	opts := mygo.WindowOptions{
		Title:         "Code",
		Width:         1100,
		Height:        700,
		MinWidth:      600,
		MinHeight:     400,
		TitleBarStyle: mygo.TitleBarHidden,
	}
	if frame != nil && frame.Frame != nil {
		f := frame.Frame
		// A frame saved on a display that is gone must not reopen off screen.
		if frameOnScreen(f, a.workAreas) || len(a.workAreas) == 0 {
			opts.X, opts.Y = f.X, f.Y
		}
		if f.Width > 0 {
			opts.Width = f.Width
		}
		if f.Height > 0 {
			opts.Height = f.Height
		}
	}
	opts.Content = view(w)
	win := mygo.NewWindow(opts)
	w.win = win
	win.SetMenu(a.menu)
	// Save the window's size and position as the user changes them, so the
	// session has them even though Bounds reads zero at quit time.
	win.OnResize(func() {
		w.captureBounds()
		a.ScheduleSave()
	})
	win.OnMove(func() {
		w.captureBounds()
		a.ScheduleSave()
	})
	a.applyWindowChrome(w)
	a.windows = append(a.windows, w)
	a.byWindow[win] = w
	win.OnClosed(func() { a.windowClosed(w) })
	// Closing a window whose terminals still run asks first, unless the user
	// already answered by closing its last workspace, or by confirming a quit.
	win.OnClose(func(e *mygo.CloseEvent) {
		if w.closing || a.quitConfirmed || !w.confirmClose(w.allSurfaces()) {
			return
		}
		e.PreventDefault()
		w.askClose("Close this window?", runningMessage, func() {
			w.closing = true
			w.win.Close()
		})
	})

	if frame != nil {
		store.Restore(*frame, newID, func(id, dir, workspace string) *Surface {
			return w.newSurface(id, dir, workspace)
		})
		// A terminal opened to resume an agent keeps that command after a
		// relaunch, so its pane runs the agent again rather than a plain shell.
		for id, command := range layoutCommands(frame) {
			if s := w.surfaceByID(id); s != nil {
				s.command = safeResumeCommand(command)
			}
		}
	}
	if len(store.Workspaces) == 0 {
		w.newWorkspace("")
	}
	// A restored window's focused pane takes the keyboard too, as one made
	// fresh does, so its terminal starts focused and its cursor blinks.
	w.focusSelected()
	return w
}

func (a *App) windowClosed(w *WindowState) {
	w.closed = true
	a.sidebarHidden = w.sidebarHidden
	a.sidebarWidth = w.sidebarWidth
	for i, existing := range a.windows {
		if existing == w {
			a.windows = append(a.windows[:i], a.windows[i+1:]...)
			break
		}
	}
	delete(a.byWindow, w.win)
	if len(a.windows) == 0 {
		snapshot := w.store.Snapshot(w.frame())
		if len(snapshot.Workspaces) > 0 {
			a.closedLast = &snapshot
		}
	}
	for _, s := range w.allSurfaces() {
		s.close()
	}
	a.ScheduleSave()
}

// windowFor maps a menu event's window to app state, falling back to the
// focused window.
func (a *App) windowFor(win *mygo.Window) *WindowState {
	if win != nil {
		if w, ok := a.byWindow[win]; ok {
			return w
		}
	}
	return a.focusedWindow()
}

// focusedWindow returns the frontmost window.
func (a *App) focusedWindow() *WindowState {
	if focused := mygo.FocusedWindow(); focused != nil {
		if w, ok := a.byWindow[focused]; ok {
			return w
		}
	}
	if len(a.windows) > 0 {
		return a.windows[len(a.windows)-1]
	}
	return nil
}

func (a *App) invalidateAll() {
	for _, w := range a.windows {
		w.win.Invalidate()
	}
}

// reloadKeybinds parses the config's keybind lines into the app's cache.
func (a *App) reloadKeybinds() {
	a.keybinds, a.keybindsCleared = a.settings.Keybinds()
}

// ReloadConfig re-reads the config file and applies it, so edits made by hand
// take effect without a relaunch.
func (a *App) ReloadConfig() {
	a.settings.Reload()
	a.applyConfig()
}

// applyConfig pushes the settings Code already holds into every running
// terminal and the window chrome. It is called after a save and after a
// reload, and never reads the file itself.
func (a *App) applyConfig() {
	a.reloadKeybinds()
	a.rebuildChromeThemes()
	for _, w := range a.windows {
		a.applyWindowChrome(w)
		for _, s := range w.allSurfaces() {
			s.applyConfig()
		}
		w.win.Invalidate()
	}
	// Settings and Keyboard Shortcuts are not window states, but their title
	// bars and content follow the same theme.
	for _, win := range []*mygo.Window{a.settingsWin, a.shortcutsWin} {
		if win != nil {
			a.applyChromeToWindow(win)
			win.Invalidate()
		}
	}
}

// GhosttyVersion is the Ghostty revision Code's libghostty-vt comes from,
// set by main from ghostty.version. It is advertised as TERM_PROGRAM_VERSION
// beside TERM_PROGRAM, so a program that recognizes Ghostty turns on images,
// true color and hyperlinks. The terminal emulator is Ghostty's, so
// TERM_PROGRAM names it.
var GhosttyVersion string

// agentEnv is the variables a shell inherits so the agent hooks can find Code,
// and so programs can tell which terminal emulator they run in. Pi and PiG,
// for instance, show an image inline only when they recognize the terminal
// from TERM_PROGRAM, KITTY_WINDOW_ID or GHOSTTY_* rather than by querying it.
func (a *App) agentEnv(s *Surface) []string {
	env := []string{
		"TERM_PROGRAM=ghostty",
		agent.EnvSurfaceID + "=" + s.id,
		agent.EnvWorkspaceID + "=" + s.workspaceID,
		agent.EnvSocket + "=" + a.socketPath,
		agent.EnvCde + "=" + a.cdePath,
		agent.EnvShims + "=" + a.shimsRoot,
		agent.EnvAgentsDir + "=" + a.assetsDir,
	}
	if v := strings.TrimSpace(GhosttyVersion); v != "" {
		env = append(env, "TERM_PROGRAM_VERSION="+v)
	}
	// Without a helper binary and a root, installing shims would land in the
	// working directory. Leaving PATH alone is safer than pointing at a shim
	// directory that does not exist.
	if a.shimsRoot != "" && a.cdePath != "" {
		shimDir := agent.ShimsDir(a.shimsRoot, s.id)
		if err := agent.InstallShims(shimDir, a.cdePath, agent.Adapters()); err != nil {
			log.Printf("codeterm: could not install agent shims: %v", err)
		} else {
			env = append(env, "PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
	}
	return append(env, shellIntegrationEnv(a.assetsDir, a.terminalCfg)...)
}

// shellIntegrationEnv points a shell at Code's startup files, which put the
// agent shims back in front of PATH after the user's profile has set it.
// bash needs a line in the user's own startup file; see the bash script.
func shellIntegrationEnv(assetsDir string, cfg config.TerminalConfig) []string {
	if assetsDir == "" || cfg.Command != "" {
		return nil
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		if runtime.GOOS == "darwin" {
			shell = "/bin/zsh"
		} else {
			shell = "/bin/sh"
		}
	}
	root := filepath.Join(assetsDir, "shell-integration")
	switch filepath.Base(shell) {
	case "zsh":
		// MyGo's zsh integration restores ZDOTDIR and sources the .zshenv in
		// it, so Code's script chains to the user's own startup files.
		var out []string
		if original := os.Getenv("ZDOTDIR"); original != "" {
			out = append(out, "CODE_ZSH_ZDOTDIR="+original)
		}
		return append(out, "ZDOTDIR="+filepath.Join(root, "zsh"))
	case "fish":
		dirs := os.Getenv("XDG_DATA_DIRS")
		if dirs == "" {
			dirs = "/usr/local/share:/usr/share"
		}
		return []string{"XDG_DATA_DIRS=" + root + string(os.PathListSeparator) + dirs}
	}
	return nil
}

// setUpAgents prepares cde, the shim root and the agent resources, and installs
// the extensions of the agents that read them from a config folder.
func (a *App) setUpAgents() {
	// The store tells whether a terminal is open and whether the user is
	// looking at it through the window that owns it.
	a.wireAgentStore()
	a.cdePath, _ = os.Executable()
	if a.cdePath == "" {
		log.Printf("codeterm: could not find the app binary; agent hooks are disabled")
	}
	a.shimsRoot = agent.ShimsRoot(os.Getpid())
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	a.assetsDir = filepath.Join(cacheDir, "code", "assets")
	if err := agent.ExtractAssets(a.assetsDir); err != nil {
		log.Printf("codeterm: could not write the agent resources: %v", err)
	}
	agent.InstallExtensions(a.assetsDir, environMap())
	agent.RemoveStaleShims(a.shimsRoot, os.Getpid(), func(id string) bool { return a.ownerOf(id) != nil })
}

// wireAgentStore lets the agent store tell a terminal's window whether the
// user is looking at it: an event for a closed terminal is dropped, and a turn
// that ends in view is already seen, so neither raises an unread alert.
func (a *App) wireAgentStore() {
	a.agents.IsLive = func(id string) bool { return a.ownerOf(id) != nil }
	a.agents.IsVisible = func(id string) bool {
		w := a.ownerOf(id)
		return w != nil && w.isInView(id)
	}
}

// removeShims drops a closed terminal's shim directory.
func (a *App) removeShims(surface string) {
	os.RemoveAll(agent.ShimsDir(a.shimsRoot, surface))
}

func environMap() map[string]string {
	out := map[string]string{}
	for _, entry := range os.Environ() {
		if key, value, ok := strings.Cut(entry, "="); ok {
			out[key] = value
		}
	}
	return out
}

// deliverAgentEvent runs on the socket goroutine; it marshals to the window
// owning the terminal, which applies the event on the main thread.
func (a *App) deliverAgentEvent(event agent.Event) {
	owner := a.ownerOf(event.SurfaceID)
	if owner == nil {
		return
	}
	owner.update(func() {
		// Hooks carry the workspace they ran in; an event claiming a different
		// one is misrouted and must not change this terminal's status.
		if event.WorkspaceID != "" {
			if id, ok := a.workspaceIDFor(event.SurfaceID); ok && id != event.WorkspaceID {
				return
			}
		}
		a.agents.Apply(event)
		// The hook's directory is the terminal's own, a fallback for one whose
		// shell has not reported a directory yet.
		if event.Cwd != "" {
			if s := owner.surfaceByID(event.SurfaceID); s != nil && s.dir == "" {
				s.dir = event.Cwd
			}
		}
	})
}

// workspaceIDFor reads a terminal's workspace on the main thread.
func (a *App) workspaceIDFor(surface string) (string, bool) {
	for _, w := range a.windows {
		if s := w.surfaceByID(surface); s != nil {
			return s.workspaceID, true
		}
	}
	return "", false
}

func (a *App) ownerOf(surface string) *WindowState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.surfaceOwners[surface]
}

func (a *App) addLive(s *Surface) {
	a.mu.Lock()
	a.surfaceOwners[s.id] = s.owner
	a.mu.Unlock()
}

func (a *App) removeLive(surface string) {
	a.mu.Lock()
	delete(a.surfaceOwners, surface)
	a.mu.Unlock()
	a.agents.Remove(surface)
	a.closeNotification(surface)
	a.closeNotification("terminal:" + surface)
}

// ScheduleSave coalesces session writes, which arrive in bursts as shells
// report new directories. The delay runs from the first unsaved change, so a
// steady stream cannot put the save off indefinitely.
func (a *App) ScheduleSave() {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	if a.saveTimer != nil {
		return
	}
	a.saveTimer = time.AfterFunc(time.Second, func() {
		mygo.RunOnMain(func() {
			a.saveMu.Lock()
			a.saveTimer = nil
			a.saveMu.Unlock()
			a.saveSession()
		})
	})
}

// flushSession writes any unsaved session now, for quitting.
func (a *App) flushSession() {
	a.saveMu.Lock()
	if a.saveTimer != nil {
		a.saveTimer.Stop()
		a.saveTimer = nil
	}
	a.saveMu.Unlock()
	a.writer.flush(a.snapshot())
}

// beforeQuit gates a quit (Cmd+Q, the Quit menu item, a signal) on the
// terminals still running a command, showing one alert for the whole app. A
// session that is ending cannot be canceled, so it is let through to save.
// Confirming sets quitConfirmed and asks to quit again, so the whole quit
// sequence runs and every window closes together.
func (a *App) beforeQuit(e *mygo.QuitEvent) {
	if e.SessionEnding || a.quitConfirmed || !a.confirmQuit() {
		return
	}
	e.PreventDefault()
	w := a.confirmWindow()
	if w == nil {
		return
	}
	w.askChoice("Quit Code?", "A process is still running in a terminal.", "Quit", func() {
		a.quitConfirmed = true
		mygo.App.Quit()
	})
}

// confirmQuit reports whether quitting needs the user's answer, by the same
// confirm-close-surface rule as closing a window: never, with a running
// command, or always.
func (a *App) confirmQuit() bool {
	for _, w := range a.windows {
		if w.confirmClose(w.allSurfaces()) {
			return true
		}
	}
	return false
}

// confirmWindow is the window the quit alert shows in: the focused one, else
// the first.
func (a *App) confirmWindow() *WindowState {
	if w := a.byWindow[mygo.FocusedWindow()]; w != nil {
		return w
	}
	if len(a.windows) > 0 {
		return a.windows[0]
	}
	return nil
}

// shutdown stops background work and flushes what is unsaved.
func (a *App) shutdown() {
	if a.dirStop != nil {
		close(a.dirStop)
		a.dirStop = nil
	}
	if a.listener != nil {
		a.listener.Stop()
	}
	for id, notification := range a.notifications {
		notification.Close()
		delete(a.notifications, id)
	}
	if a.shimsRoot != "" {
		if err := os.RemoveAll(a.shimsRoot); err != nil {
			log.Printf("codeterm: could not remove the shim directory: %v", err)
		}
	}
	a.flushSession()
	// An unchanged config must not be rewritten on quit: doing so can clobber
	// an edit made outside Code just before it quit.
	if a.settings.Dirty() {
		_ = a.settings.Save()
	}
}

// snapshot reads the windows into a session, on the main thread.
func (a *App) snapshot() *model.Session {
	hidden := a.sidebarHidden
	session := &model.Session{
		SidebarHidden: &hidden,
		SidebarWidth:  a.sidebarWidth,
	}
	if len(a.windows) == 0 {
		if a.closedLast != nil {
			session.Windows = []model.WindowSession{*a.closedLast}
		}
		return session
	}
	for _, w := range a.windows {
		saved := w.store.Snapshot(w.frame())
		fillLayoutCommands(&saved, w.allSurfaces())
		session.Windows = append(session.Windows, saved)
	}
	return session
}

// frameOnScreen reports whether a saved frame still overlaps a display.
func frameOnScreen(f *model.Frame, areas []mygo.Rectangle) bool {
	if f == nil {
		return false
	}
	for _, area := range areas {
		if area.Width <= 0 || area.Height <= 0 {
			continue
		}
		if f.X < int(area.X+area.Width) && f.X+max(f.Width, 1) > int(area.X) &&
			f.Y < int(area.Y+area.Height) && f.Y+max(f.Height, 1) > int(area.Y) {
			return true
		}
	}
	return false
}

// startDirWatcher follows the working directory shells report with OSC 7 and
// the config file, every half second, instead of reading them every frame.
func (a *App) startDirWatcher() {
	a.dirStop = make(chan struct{})
	stop := a.dirStop
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				mygo.RunOnMain(func() {
					a.refreshDirs()
					a.reloadConfigIfChanged()
				})
			}
		}
	}()
}

// reloadConfigIfChanged reloads the config when its file changed on disk, so
// an edit made by hand applies without the Reload Configuration command.
// Settings knows which writes were its own and which edits are still pending,
// so a debounced save is never read back over an unsaved change.
func (a *App) reloadConfigIfChanged() {
	if a.settings.ReloadIfChanged() {
		a.applyConfig()
	}
}

// refreshDirs reads the running terminals' directories and redraws the windows
// whose tabs changed.
func (a *App) refreshDirs() {
	for _, w := range a.windows {
		if w.closed {
			continue
		}
		changed := false
		for _, s := range w.allSurfaces() {
			if s.closed || s.term == nil {
				continue
			}
			// The shell's own working directory is the truth, since its
			// title and OSC 7 reports may come only once.
			dir := s.term.Dir()
			if pid, ok := s.shellPID(); ok {
				if d, ok := processDir(pid); ok {
					dir = d
				}
				// Reading a process's arguments is costly; only do it when
				// the foreground process actually changed.
				if fg := foregroundPID(pid); fg != s.foreground {
					s.foreground = fg
					if host := sshRemoteFor(fg); host != s.remote {
						s.remote = host
						changed = true
					}
				}
			}
			if dir != "" && dir != s.dir {
				s.dir = dir
				changed = true
			}
		}
		if changed {
			w.markChanged()
		}
	}
}

// saveSession queues the current session to be written off the main thread.
func (a *App) saveSession() {
	a.writer.save(a.snapshot())
}

// newID returns a random UUIDv4 string.
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("id-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
