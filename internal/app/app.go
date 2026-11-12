package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/agent"
	"github.com/phucngodev/code/internal/config"
	"github.com/phucngodev/code/internal/model"
)

// App is the whole application: its settings, its agent socket, and its
// windows.
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

	mu            sync.Mutex
	surfaceOwners map[string]*WindowState
	saveTimer     *time.Timer
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
	// configMod is the config file's last seen modification time, so the
	// watcher reloads it when it changes on disk.
	configMod time.Time

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
	a.startDirWatcher()
	a.restoreWindows()
	mygo.App.OnActivate(func(visible bool) {
		if len(a.windows) == 0 {
			a.newWindow(nil)
		}
	})
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
		if f.X != 0 || f.Y != 0 {
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
	// already answered by closing its last workspace.
	win.OnClose(func(e *mygo.CloseEvent) {
		if w.closing || !w.confirmClose(w.allSurfaces()) {
			return
		}
		e.PreventDefault()
		w.askClose("Close this window?", "A process is still running in it.", func() {
			w.closing = true
			w.win.Close()
		})
	})

	if frame != nil {
		store.Restore(*frame, newID, func(id, dir, workspace string) *Surface {
			return w.newSurface(id, dir, workspace)
		})
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

// agentEnv is the variables a shell inherits so the agent hooks can find Code.
func (a *App) agentEnv(s *Surface) []string {
	shimDir := agent.ShimsDir(a.shimsRoot, s.id)
	_ = agent.InstallShims(shimDir, a.cdePath, agent.Adapters())
	path := shimDir + string(os.PathListSeparator) + os.Getenv("PATH")
	return []string{
		agent.EnvSurfaceID + "=" + s.id,
		agent.EnvWorkspaceID + "=" + s.workspaceID,
		agent.EnvSocket + "=" + a.socketPath,
		agent.EnvCde + "=" + a.cdePath,
		agent.EnvShims + "=" + a.shimsRoot,
		agent.EnvAgentsDir + "=" + a.assetsDir,
		"PATH=" + path,
	}
}

// setUpAgents prepares cde, the shim root and the agent resources, and installs
// the extensions of the agents that read them from a config folder.
func (a *App) setUpAgents() {
	a.cdePath, _ = os.Executable()
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
	owner.win.Update(func() {
		a.agents.Apply(event)
	})
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
}

// ScheduleSave coalesces session writes, which arrive in bursts as shells
// report new directories. The delay runs from the first unsaved change, so a
// steady stream cannot put the save off indefinitely.
func (a *App) ScheduleSave() {
	if a.saveTimer != nil {
		return
	}
	a.saveTimer = time.AfterFunc(time.Second, func() {
		mygo.RunOnMain(func() {
			a.saveTimer = nil
			a.saveSession()
		})
	})
}

// flushSession writes any unsaved session now, for quitting.
func (a *App) flushSession() {
	if a.saveTimer != nil {
		a.saveTimer.Stop()
		a.saveTimer = nil
	}
	a.writer.flush(a.snapshot())
}

// shutdown stops background work and flushes what is unsaved.
func (a *App) shutdown() {
	if a.dirStop != nil {
		close(a.dirStop)
		a.dirStop = nil
	}
	a.flushSession()
	_ = a.settings.Save()
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
		session.Windows = append(session.Windows, w.store.Snapshot(w.frame()))
	}
	return session
}

// startDirWatcher follows the working directory shells report with OSC 7 and
// the config file, once a second, instead of reading them every frame.
func (a *App) startDirWatcher() {
	a.dirStop = make(chan struct{})
	stop := a.dirStop
	go func() {
		ticker := time.NewTicker(time.Second)
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
func (a *App) reloadConfigIfChanged() {
	info, err := os.Stat(a.paths.Config)
	if err != nil {
		return
	}
	mod := info.ModTime()
	if a.configMod.IsZero() {
		a.configMod = mod
		return
	}
	if mod.After(a.configMod) {
		a.configMod = mod
		a.ReloadConfig()
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
			if dir := s.term.Dir(); dir != "" && dir != s.dir {
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
