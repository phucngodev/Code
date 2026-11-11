package app

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/config"
	"github.com/phucngodev/code/internal/model"
)

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

func TestWindowBindingOverridesDefault(t *testing.T) {
	a := &App{keybinds: []config.Keybind{{Mods: ui.Cmd | ui.Alt, Key: ui.KeyLeft, Action: "new_tab"}}}
	w := &WindowState{app: a, store: model.NewStore[*Surface]()}
	count := 0
	for _, b := range w.windowBindings() {
		if b.mods == ui.Cmd|ui.Alt && b.key == ui.KeyLeft {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("the same key should be bound once, got %d", count)
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

func TestThemeParts(t *testing.T) {
	light, dark := parseThemeParts("light:Latte,dark:Mocha")
	if light != "Latte" || dark != "Mocha" {
		t.Fatalf("parts = %q, %q", light, dark)
	}
	if got := joinThemeParts("Latte", "Latte"); got != "Latte" {
		t.Fatalf("join same = %q", got)
	}
	if got := joinThemeParts("Latte", "Mocha"); got != "light:Latte,dark:Mocha" {
		t.Fatalf("join different = %q", got)
	}
}

func TestShortcutsViewRenders(t *testing.T) {
	a := &App{}
	tester := ui.NewTester(a.shortcutsView, 520, 620)
	tester.Frame()
	if !tester.HasText("New workspace") {
		t.Fatal("the shortcuts window should list New workspace")
	}
	if !tester.HasText("Keyboard Shortcuts") && !tester.HasText("Tabs and panes") {
		t.Fatal("the shortcuts window should list its sections")
	}
}

func TestSettingsViewRenders(t *testing.T) {
	settings := config.LoadSettings(t.TempDir() + "/config")
	a := &App{settings: settings, settingsState: &settingsWindow{}}
	tester := ui.NewTester(a.settingsView, 560, 560)
	tester.Frame()
	if !tester.HasText("General") {
		t.Fatal("the settings window should show its tabs")
	}
}

func TestEverySettingsTabRenders(t *testing.T) {
	settings := config.LoadSettings(t.TempDir() + "/config")
	a := &App{settings: settings, settingsState: &settingsWindow{}}
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
	settings := config.LoadSettings(t.TempDir() + "/config")
	a := &App{settings: settings}
	w := &WindowState{app: a, store: model.NewStore[*Surface](), sidebarHidden: true}
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	if !tester.HasText("No terminals") {
		t.Fatal("an empty window should invite a new workspace")
	}
}

func TestTitleBarSettingsButton(t *testing.T) {
	settings := config.LoadSettings(t.TempDir() + "/config")
	a := &App{settings: settings}
	w := &WindowState{app: a, store: model.NewStore[*Surface](), sidebarHidden: true}
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	if _, ok := tester.Access("Settings"); !ok {
		t.Fatal("the title bar should offer a Settings button")
	}
}

func TestCommandPaletteRenders(t *testing.T) {
	settings := config.LoadSettings(t.TempDir() + "/config")
	a := &App{settings: settings}
	w := &WindowState{app: a, store: model.NewStore[*Surface](), sidebarHidden: true}
	w.openPalette()
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	if !tester.HasText("Nothing to go to") {
		t.Fatal("the command palette should show its empty message")
	}
}

func TestTruncatePathKeepsTheTail(t *testing.T) {
	got := truncateStart("~/Documents/Codes/Code/codego", 20)
	if len([]rune(got)) != 20 {
		t.Fatalf("truncateStart length = %d, want 20 (%q)", len([]rune(got)), got)
	}
	if !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "codego") {
		t.Fatalf("truncateStart = %q, want a leading ellipsis and the folder name", got)
	}
}

func TestSurfaceDefaultsToWorkingDirectory(t *testing.T) {
	project := t.TempDir()
	path := t.TempDir() + "/config"
	if err := os.WriteFile(path, []byte("working-directory = "+project+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	settings := config.LoadSettings(path)
	a := &App{settings: settings}
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

func TestSessionWriterFlush(t *testing.T) {
	path := t.TempDir() + "/session.json"
	w := newSessionWriter(path)
	w.flush(&model.Session{})
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session not written: %v", err)
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
