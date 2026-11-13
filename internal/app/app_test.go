package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/agent"
	"github.com/phucngodev/code/internal/config"
	"github.com/phucngodev/code/internal/history"
	"github.com/phucngodev/code/internal/model"
)

func TestConfirmCloseFollowsSetting(t *testing.T) {
	w := testWindow(t)
	idle := w.newSurface("idle", "", "ws")
	cases := []struct {
		setting string
		want    bool
	}{
		{"false", false},
		{"never", false},
		{"always", true},
		{"true", false}, // no program running
	}
	for _, tc := range cases {
		w.app.settings.Set(config.KeyConfirmClose, tc.setting)
		if got := w.confirmClose([]*Surface{idle}); got != tc.want {
			t.Fatalf("confirmClose with %q = %v, want %v", tc.setting, got, tc.want)
		}
	}
}

// TestBeforeQuitFollowsSetting checks the quit gate follows the same
// confirm-close-surface setting as closing a window, tab or workspace.
func TestBeforeQuitFollowsSetting(t *testing.T) {
	a := testApp(t)
	w := &WindowState{app: a, store: model.NewStore[*Surface]()}
	a.windows = append(a.windows, w)
	w.newWorkspace("")

	a.settings.Set(config.KeyConfirmClose, "always")
	e := &mygo.QuitEvent{}
	a.beforeQuit(e)
	if !e.DefaultPrevented() {
		t.Fatal("confirm-close-surface = always should stop a quit")
	}
	w.pending = nil

	a.settings.Set(config.KeyConfirmClose, "false")
	e = &mygo.QuitEvent{}
	a.beforeQuit(e)
	if e.DefaultPrevented() {
		t.Fatal("confirm-close-surface = false should not stop a quit")
	}
}

func TestSurfaceBusy(t *testing.T) {
	if err := terminal.Load(); err != nil {
		t.Skipf("libghostty not available: %v", err)
	}
	held, err := terminal.New(terminal.Options{Command: []string{"/bin/sleep", "10"}})
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if s := (&Surface{id: "held", term: held}); !s.busy() {
		t.Fatal("a program still running should read as busy")
	}

	exited, err := terminal.New(terminal.Options{Command: []string{"/usr/bin/true"}})
	if err != nil {
		t.Fatal(err)
	}
	defer exited.Close()
	s := &Surface{id: "exited", term: exited}
	select {
	case <-exited.Done():
		if s.busy() {
			t.Fatal("a program that ended should not read as busy")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the program did not exit")
	}
}

func TestConfirmDialogShowsAndCancels(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	ran := false
	w.askClose("Close this terminal?", "A process is still running in it.", func() { ran = true })

	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	if !tester.HasText("Close this terminal?") || !tester.HasText("Close") || !tester.HasText("Cancel") {
		t.Fatal("the close confirmation should show its title and buttons")
	}

	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if w.pending != nil {
		t.Fatal("Escape should cancel the close")
	}
	if ran {
		t.Fatal("canceling should not run the close")
	}
}

func TestQuitDialogShowsQuitButton(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	w.askChoice("Quit Code?", "A process is still running in a terminal.", "Quit", func() {})

	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	if !tester.HasText("Quit Code?") || !tester.HasText("Quit") || !tester.HasText("Cancel") {
		t.Fatal("the quit confirmation should show its title and buttons")
	}
}

func TestConfirmDialogCloseRuns(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	ran := false
	w.askClose("Close this terminal?", "A process is still running in it.", func() { ran = true })

	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	if err := tester.Click("Close"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !ran {
		t.Fatal("confirming should run the close")
	}
	if w.pending != nil {
		t.Fatal("the alert should close after confirming")
	}
}

func TestTerminalChromeHidesScrollbar(t *testing.T) {
	base := ui.DarkTheme()
	if terminalChrome(base, false) != base {
		t.Fatal("scrollbar shown should use the chrome theme itself")
	}
	hidden := terminalChrome(base, true)
	if hidden == base {
		t.Fatal("scrollbar never should clone the theme")
	}
	if hidden.Scrollbar != ui.Transparent || hidden.ScrollbarStyle.TrackWidth != 1 {
		t.Fatalf("hidden theme = %+v", hidden.ScrollbarStyle)
	}
	if style := (ui.ScrollbarStyle{}).Resolve(hidden); style.Thumb.A != 0 {
		t.Fatalf("resolved thumb alpha = %d, want 0", style.Thumb.A)
	}
	if base.Scrollbar == ui.Transparent {
		t.Fatal("the base theme must keep its scroll bar")
	}
}

func TestHideScrollbarFollowsSetting(t *testing.T) {
	a := testApp(t)
	if a.hideScrollbar() {
		t.Fatal("the default should show the bar")
	}
	a.settings.Set(config.KeyScrollbar, "never")
	if !a.hideScrollbar() {
		t.Fatal("scrollbar = never should hide the bar")
	}
}

func TestAgentStoreFollowsTerminalOwners(t *testing.T) {
	a := testApp(t)
	a.wireAgentStore()
	if a.agents.IsLive("missing") {
		t.Fatal("a terminal with no window should not be live")
	}
	a.surfaceOwners["s"] = &WindowState{}
	if !a.agents.IsLive("s") {
		t.Fatal("a terminal with a window should be live")
	}
}

func TestTitleBarButtonsAreAccessible(t *testing.T) {
	w := testWindow(t)
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	for _, name := range []string{"Toggle sidebar", "New workspace", "Agent alerts", "Settings"} {
		if _, ok := tester.AccessByRole(ui.RoleButton, name); !ok {
			t.Errorf("%q should be an accessible button", name)
		}
	}
}

func TestTabCloseIsAccessible(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	if _, ok := tester.AccessByRole(ui.RoleTab, ""); !ok {
		t.Error("a tab should be exposed as a tab")
	}
	if _, ok := tester.AccessByRole(ui.RoleButton, "Close tab"); !ok {
		t.Error("the selected tab's close should be an accessible button")
	}
}

func TestSettingsControlsAreLabelled(t *testing.T) {
	a := testApp(t)
	a.settingsState = &settingsWindow{}
	tester := ui.NewTester(a.settingsView, 560, 560)
	tester.Frame()
	if _, ok := tester.AccessByRole(ui.RoleSwitch, "Copy on select"); !ok {
		t.Error("the copy-on-select switch should be named by its label")
	}
	if _, ok := tester.AccessByRole(ui.RolePopUpButton, "Confirm closing terminals"); !ok {
		t.Error("the confirm-closing select should be named by its label")
	}
}

func TestSessionsListRenders(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	w.history.expanded = true
	w.history.loaded = true
	w.history.sessions = []history.Session{
		{ID: "a", Agent: "claude", Cwd: "/tmp/a", Title: "First session"},
		{ID: "b", Agent: "codex", Cwd: "/tmp/b", Title: "Second session"},
	}
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	if !tester.HasText("First session") || !tester.HasText("Second session") {
		t.Fatal("the sessions list should show the saved sessions")
	}
}

func TestAppKeyFreesControlKeysOnLinux(t *testing.T) {
	if runtime.GOOS == "darwin" {
		if got := appKey("F"); got != "Cmd+F" {
			t.Fatalf("appKey(F) on macOS = %q, want Cmd+F", got)
		}
		return
	}
	if got := appKey("F"); got != "Shift+CmdOrCtrl+F" {
		t.Fatalf("appKey(F) = %q, want Shift+CmdOrCtrl+F", got)
	}
	if got := appKey("P"); got != "Shift+CmdOrCtrl+P" {
		t.Fatalf("appKey(P) = %q, want Shift+CmdOrCtrl+P", got)
	}
}

func TestCleanTitleLocalAndRemote(t *testing.T) {
	local, err := os.Hostname()
	if err != nil {
		t.Skipf("no host name: %v", err)
	}
	short := local
	if i := strings.IndexByte(short, '.'); i >= 0 {
		short = short[:i]
	}
	remote := short + "-remote"
	cases := []struct{ in, want string }{
		// The machine Code runs on: show the path.
		{"user@" + local + ":~/Projects", "~/Projects"},
		{"user@" + short + ":~", "~"},
		{"user@localhost:/srv/app", "/srv/app"},
		// Another machine, reached by ssh: show its host.
		{"user@" + remote + ":~/Projects", remote},
		{"user@" + remote + ".example.com:/srv", remote + ".example.com"},
		// A title a program sets is kept.
		{"vim main.go", "vim main.go"},
		{"~/Projects", "~/Projects"},
		{"user@host", "user@host"},
		{"user@host:", "user@host:"},
		{"", ""},
	}
	for _, c := range cases {
		if got := cleanTitle(c.in); got != c.want {
			t.Errorf("cleanTitle(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSplitArgumentParsing(t *testing.T) {
	if got := splitNavigation("left"); got != model.NavLeft {
		t.Fatalf("splitNavigation(left) = %v", got)
	}
	if got := splitNavigation("next"); got != model.NavNext {
		t.Fatalf("splitNavigation(next) = %v", got)
	}
	dir, amount := splitResize("up,20")
	if dir != model.DirUp || amount != 20 {
		t.Fatalf("splitResize(up,20) = %v, %v", dir, amount)
	}
	dir, amount = splitResize("right")
	if dir != model.DirRight || amount != 10 {
		t.Fatalf("splitResize(right) = %v, %v", dir, amount)
	}
}

func TestTerminalKeybindMapping(t *testing.T) {
	got := terminalKeybinds([]config.Keybind{
		{Mods: ui.Cmd, Key: ui.KeyC, Action: "copy_to_clipboard", Performable: true},
		{Mods: ui.Cmd, Key: ui.KeyD, Action: "goto_split", Arg: "left"},
	})
	if len(got) != 1 || got[0].Action != terminal.ActionCopy || !got[0].Performable {
		t.Fatalf("terminalKeybinds = %+v", got)
	}
}

func TestWindowBindingLeavesOptionBindingsToTheTerminal(t *testing.T) {
	a := testApp(t)
	a.keybinds = []config.Keybind{{Mods: ui.Cmd | ui.Alt, Key: ui.KeyLeft, Action: "new_tab"}}
	w := &WindowState{app: a, store: model.NewStore[*Surface]()}
	for _, b := range w.windowBindings() {
		if b.mods == ui.Cmd|ui.Alt && b.key == ui.KeyLeft {
			t.Fatal("an Option binding must not be a window shortcut, or the terminal never sees it")
		}
	}
	if !a.runAppKeybind(w, ui.Cmd|ui.Alt, ui.KeyLeft) {
		t.Fatal("the terminal's OnKey should run the Option binding")
	}
}

func TestRunAppKeybindRunsResize(t *testing.T) {
	a := testApp(t)
	a.keybinds = []config.Keybind{{Mods: ui.Alt | ui.Shift, Key: ui.KeyH, Action: "resize_split", Arg: "right,20"}}
	w := &WindowState{app: a, store: model.NewStore[*Surface]()}
	if !a.runAppKeybind(w, ui.Alt|ui.Shift, ui.KeyH) {
		t.Fatal("the Option resize binding should run through OnKey")
	}
}

func TestWindowBindingHasPaneSwitching(t *testing.T) {
	a := testApp(t)
	w := &WindowState{app: a, store: model.NewStore[*Surface]()}
	found := map[ui.Key]bool{}
	for _, b := range w.windowBindings() {
		if b.mods == ui.Cmd|ui.Super && (b.key == ui.KeyBracketLeft || b.key == ui.KeyBracketRight) {
			found[b.key] = true
		}
	}
	if !found[ui.KeyBracketLeft] || !found[ui.KeyBracketRight] {
		t.Fatalf("windowBindings should bind both pane switch keys, got %v", found)
	}
}

func TestSplitCommand(t *testing.T) {
	got := splitCommand(`/bin/zsh -lic "echo hi"`)
	want := []string{"/bin/zsh", "-lic", "echo hi"}
	if len(got) != len(want) {
		t.Fatalf("splitCommand = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitCommand[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestShortcutsViewRenders(t *testing.T) {
	a := testApp(t)
	tester := ui.NewTester(a.shortcutsView, 520, 620)
	tester.Frame()
	if !tester.HasText("Switch workspace") {
		t.Fatal("the shortcuts window should list its workspace shortcuts")
	}
	if !tester.HasText("Keyboard Shortcuts") && !tester.HasText("Tabs and panes") {
		t.Fatal("the shortcuts window should list its sections")
	}
}

func TestSettingsViewRenders(t *testing.T) {
	a := testApp(t)
	a.settingsState = &settingsWindow{}
	tester := ui.NewTester(a.settingsView, 560, 560)
	tester.Frame()
	if !tester.HasText("General") {
		t.Fatal("the settings window should show its tabs")
	}
}

func TestEverySettingsTabRenders(t *testing.T) {
	a := testApp(t)
	a.settingsState = &settingsWindow{}
	tester := ui.NewTester(a.settingsView, 560, 560)
	for tab := 0; tab < len(settingsTabs); tab++ {
		a.settingsState.tab = tab
		tester.Frame()
		if !tester.HasText(settingsTabs[tab].name) {
			t.Fatalf("tab %d should render its title", tab)
		}
	}
}

func TestEmptyWindowRenders(t *testing.T) {
	a := testApp(t)
	w := &WindowState{app: a, store: model.NewStore[*Surface](), sidebarHidden: true}
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	if !tester.HasText("No terminals") {
		t.Fatal("an empty window should invite a new workspace")
	}
}

func TestTitleBarSettingsButton(t *testing.T) {
	a := testApp(t)
	w := &WindowState{app: a, store: model.NewStore[*Surface](), sidebarHidden: true}
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	if _, ok := tester.Access("Settings"); !ok {
		t.Fatal("the title bar should offer a Settings button")
	}
}

func TestCommandPaletteRenders(t *testing.T) {
	a := testApp(t)
	w := &WindowState{app: a, store: model.NewStore[*Surface](), sidebarHidden: true}
	w.openPalette()
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	if !tester.HasText("Nothing to go to") {
		t.Fatal("the command palette should show its empty message")
	}
}

func TestTabLabelPrefersTheReportedDirectory(t *testing.T) {
	local, err := os.Hostname()
	if err != nil {
		t.Skipf("no host name: %v", err)
	}
	remote := "remote-host"
	cases := []struct {
		title, dir, remote, want string
	}{
		// A shell's path-like title is its directory: the tab follows the
		// directory Code reports, which is fresh after a cd.
		{"~/Projects", "/Users/me/Projects", "", "Projects"},
		{"user@" + local + ":~/Projects", "/Users/me/Projects", "", "Projects"},
		{"~/Old", "/Users/me/New", "", "New"},
		// A remote prompt keeps the host, with or without the user.
		{"user@" + remote + ":~/Projects", "/Users/me/Projects", "", remote},
		{remote + ":~/Projects", "/Users/me/Projects", "", remote},
		// An ssh in the terminal names the host, whatever the title says.
		{"~/Projects", "/Users/me/Projects", remote, remote},
		// A program's own title is kept.
		{"vim main.go", "/Users/me/Projects", "", "vim main.go"},
		{"main.go:12", "/Users/me/Projects", "", "main.go:12"},
		// Without a title, the directory's last segment shows.
		{"", "/home/me/code", "", "code"},
		{"", "", "", "Terminal"},
	}
	for _, c := range cases {
		s := &Surface{id: "a", title: c.title, dir: c.dir, remote: c.remote}
		tab := model.NewTab("t", s)
		if got := (&WindowState{}).tabLabel(tab); got != c.want {
			t.Errorf("tabLabel(title=%q dir=%q remote=%q) = %q, want %q", c.title, c.dir, c.remote, got, c.want)
		}
	}
}

func TestSurfaceDefaultsToWorkingDirectory(t *testing.T) {
	project := t.TempDir()
	a := testApp(t)
	if err := os.WriteFile(a.settings.Path(), []byte("working-directory = "+project+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.settings.Reload()
	w := &WindowState{app: a, store: model.NewStore[*Surface]()}
	s := w.newSurface("id", "", "ws")
	if s.LeafDir() != project {
		t.Fatalf("dir = %q, want %q", s.LeafDir(), project)
	}
}

func TestPanePositions(t *testing.T) {
	a := &Surface{id: "a"}
	b := &Surface{id: "b"}
	c := &Surface{id: "c"}

	root := model.NewSplit(model.Horizontal, model.NewLeaf(a), model.NewLeaf(b), 50)
	if got := panePositions(root); len(got) != 2 || got[0] != "Left" || got[1] != "Right" {
		t.Fatalf("panePositions = %v, want [Left Right]", got)
	}

	nested := model.NewSplit(model.Horizontal,
		model.NewLeaf(a),
		model.NewSplit(model.Vertical, model.NewLeaf(b), model.NewLeaf(c), 50), 50)
	got := panePositions(nested)
	if len(got) != 3 || got[0] != "Left" || got[1] != "Top right" || got[2] != "Bottom right" {
		t.Fatalf("panePositions = %v, want [Left Top right Bottom right]", got)
	}
}

func TestSidebarRowThemeTextOnGray(t *testing.T) {
	base := ui.Theme{Background: ui.RGBA(255, 255, 255, 255), Text: ui.RGBA(0, 0, 0, 255)}
	got := sidebarRowTheme(&base)
	if got.Accent != got.InactiveAccent {
		t.Fatal("a chosen workspace should look the same in and out of focus")
	}
	if got.AccentText != base.Text {
		t.Fatalf("text on the chosen row should be the theme's text, got %v", got.AccentText)
	}
	if got.Radius != 0 {
		t.Fatalf("workspace rows should stay square, got radius %v", got.Radius)
	}
}

func TestCloseSurfaceRefocusesSurvivor(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	tab := w.store.Selected().SelectedTab()
	w.splitFocused(model.DirRight)
	survivor := tab.Panes.Leaves()[0]
	survivor.wantFocus = false

	w.closeSurface(tab.Focused())

	if tab.Focused() != survivor {
		t.Fatal("closing the focused pane should focus the survivor")
	}
	if !survivor.wantFocus {
		t.Fatal("the survivor should take the keyboard")
	}
}

func TestClosingViewsRefocusTheTerminal(t *testing.T) {
	cases := []struct {
		name  string
		close func(w *WindowState)
	}{
		{"palette", func(w *WindowState) { w.palette.open = true; w.closePalette() }},
		{"find", func(w *WindowState) { w.find.open = true; w.closeFind() }},
		{"rename", func(w *WindowState) {
			ws := w.store.Selected()
			w.renaming, w.renameBuf = ws.ID, "Renamed"
			w.commitRename(ws)
		}},
		{"sidebar", func(w *WindowState) { w.toggleSidebar() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := testWindow(t)
			w.newWorkspace("")
			s := w.focusedSurface()
			s.wantFocus = false

			tc.close(w)

			if !s.wantFocus {
				t.Fatalf("closing %s should return the keyboard to the terminal", tc.name)
			}
		})
	}
}

func TestPaneDropRefocusesDragged(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	tab := w.store.Selected().SelectedTab()
	w.splitFocused(model.DirRight)
	left, right := tab.Panes.Leaves()[0], tab.Panes.Leaves()[1]
	left.wantFocus = false

	w.applyPaneDrop(paneDropAction{dragged: left, target: right, tab: tab, split: true, dir: model.DirLeft})

	if tab.Focused() != left {
		t.Fatal("a dropped pane should take the focus")
	}
	if !left.wantFocus {
		t.Fatal("a dropped pane should take the keyboard")
	}
}

func TestPaneDimmedOnlyUnfocused(t *testing.T) {
	a := &Surface{id: "a"}
	b := &Surface{id: "b"}
	tab := model.NewTab("t", a)
	if paneDimmed(tab, a, false) {
		t.Fatal("a lone pane should never dim")
	}
	tab.Panes.Split(a, model.DirRight, b)
	tab.Focus(b)
	if paneDimmed(tab, b, true) {
		t.Fatal("the focused pane should not dim")
	}
	if !paneDimmed(tab, a, true) {
		t.Fatal("the unfocused pane should dim")
	}
	if paneDimmed(tab, a, false) {
		t.Fatal("a pane should not dim when the tab is not split")
	}
}

func TestSessionWriterFlush(t *testing.T) {
	path := t.TempDir() + "/session.json"
	w := newSessionWriter(path)
	w.flush(&model.Session{})
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session not written: %v", err)
	}
}

func TestSurfaceExitedClosesItsPane(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	tab := w.store.Selected().SelectedTab()
	w.splitFocused(model.DirRight)
	exited := tab.Focused()

	w.surfaceExited(exited)

	if leaves := tab.Panes.Leaves(); len(leaves) != 1 || leaves[0] == exited {
		t.Fatalf("exited pane should be gone, leaves = %v", leaves)
	}
	if !exited.closed {
		t.Fatal("the exited surface should be closed")
	}
}

func TestSurfaceExitedOnLastPaneClosesTab(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	ws := w.store.Selected()
	exited := ws.SelectedTab().Focused()

	w.surfaceExited(exited)

	if len(ws.Tabs) != 0 {
		t.Fatalf("the tab of a lone exited pane should close, tabs = %d", len(ws.Tabs))
	}
}

func TestSurfaceExitedIgnoresAnAlreadyClosedPane(t *testing.T) {
	w := testWindow(t)
	w.newWorkspace("")
	tab := w.store.Selected().SelectedTab()
	w.splitFocused(model.DirRight)
	exited := tab.Focused()
	exited.closed = true

	w.surfaceExited(exited)

	if leaves := tab.Panes.Leaves(); len(leaves) != 2 {
		t.Fatalf("a closed pane should not be removed again, leaves = %v", leaves)
	}
}

// testApp is an app whose files and shims live in a temporary directory, so
// a view that starts a terminal cannot write into the package directory.
func testApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	return &App{
		paths: config.Paths{
			Config:    filepath.Join(dir, "config"),
			Themes:    filepath.Join(dir, "themes"),
			Session:   filepath.Join(dir, "session.json"),
			SocketDir: dir,
		},
		settings:      config.LoadSettings(filepath.Join(dir, "config")),
		agents:        agent.NewStore(),
		writer:        newSessionWriter(filepath.Join(dir, "session.json")),
		byWindow:      map[*mygo.Window]*WindowState{},
		surfaceOwners: map[string]*WindowState{},
		notifications: map[string]*mygo.Notification{},
		sidebarHidden: true,
		sidebarWidth:  240,
		shimsRoot:     filepath.Join(dir, "shims"),
		cdePath:       "/usr/bin/true",
	}
}

// testWindow is a window with just enough wiring for the store and the close
// paths, so a pane can exit without a real terminal or window.
func testWindow(t *testing.T) *WindowState {
	t.Helper()
	return &WindowState{app: testApp(t), store: model.NewStore[*Surface]()}
}

func TestSurfaceKeepsDirWhenReportIsEmpty(t *testing.T) {
	s := &Surface{dir: "/home/phuc/Projects"}
	s.setDir("")
	if s.dir != "/home/phuc/Projects" {
		t.Fatalf("an empty report should keep the directory, got %q", s.dir)
	}
	s.setDir("/home/phuc/Code")
	if s.dir != "/home/phuc/Code" {
		t.Fatalf("a real report should replace the directory, got %q", s.dir)
	}
}

func TestValidDirFallsBackWhenMissing(t *testing.T) {
	dir := t.TempDir()
	if got := validDir(dir); got != dir {
		t.Fatalf("validDir(%q) = %q, want it unchanged", dir, got)
	}
	missing := dir + "/does-not-exist"
	if got := validDir(missing); got == "" || got == missing {
		t.Fatalf("validDir(%q) = %q, want a fallback", missing, got)
	}
}
