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

// liveWindow builds a window with two panes and a real terminal, so keyboard
// routing is tested against the terminal's own key handling.
func liveWindow(t *testing.T, binds []string) *WindowState {
	t.Helper()
	if err := terminal.Load(); err != nil {
		t.Skipf("libghostty not available: %v", err)
	}
	a := testApp(t)
	text := "command = /usr/bin/true\nmacos-option-as-alt = true\n"
	for _, b := range binds {
		text += "keybind = " + b + "\n"
	}
	if err := os.WriteFile(a.settings.Path(), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	a.settings.Reload()
	a.reloadKeybinds()
	a.terminalCfg = a.settings.Terminal("")
	w := &WindowState{app: a, store: model.NewStore[*Surface](), sidebarHidden: true}
	w.newWorkspace("")
	w.splitFocused(model.DirRight)
	return w
}

func TestLiveNavKeybind(t *testing.T) {
	w := liveWindow(t, []string{"command+shift+h=goto_split:left"})
	before := w.focusedSurface().id
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	tester.Key(ui.Super|ui.Shift, ui.KeyH)
	tester.Frame()
	if got := w.focusedSurface().id; got == before {
		t.Fatal("command+shift+h did not move the focus")
	}
}

// TestLiveNavWithScrollbarNever checks a pane keeps its focus and keys when its
// view is wrapped in the scoped theme that hides the scroll bar.
func TestLiveNavWithScrollbarNever(t *testing.T) {
	w := liveWindow(t, nil)
	w.app.termLight = terminalChrome(ui.LightTheme(), true)
	w.app.termDark = terminalChrome(ui.DarkTheme(), true)
	w.app.settings.Set(config.KeyScrollbar, "never")
	before := w.focusedSurface().id
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	tester.Key(ui.Cmd|ui.Alt, ui.KeyLeft)
	tester.Frame()
	if got := w.focusedSurface().id; got == before {
		t.Fatal("navigation did not move the focus with the scoped terminal theme")
	}
}

func TestLivePaneSwitchKeybind(t *testing.T) {
	w := liveWindow(t, nil)
	before := w.focusedSurface().id
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	tester.Key(ui.Cmd|ui.Super, ui.KeyBracketRight)
	tester.Frame()
	next := w.focusedSurface().id
	if next == before {
		t.Fatal("the next-pane shortcut did not move the focus")
	}
	tester.Key(ui.Cmd|ui.Super, ui.KeyBracketLeft)
	tester.Frame()
	if got := w.focusedSurface().id; got != before {
		t.Fatalf("the previous-pane shortcut moved to %v, want %v", got, before)
	}
}

// TestLiveNavMovesKeyboardFocus checks that navigating panes hands the target
// pane the keyboard, not only the model's focus. A pane that already exists
// must take the focus explicitly, since AutoFocus only fires on the frame an
// element is born.
func TestLiveNavMovesKeyboardFocus(t *testing.T) {
	w := liveWindow(t, nil)
	tab := w.store.Selected().SelectedTab()
	left, right := tab.Panes.Leaves()[0], tab.Panes.Leaves()[1]
	var ctx *ui.Context
	view := func(c *ui.Context) { ctx = c; w.view(c) }
	tester := ui.NewTester(view, 900, 600)
	tester.Frame()
	if !right.handle.Focused(ctx) {
		t.Fatal("the new split should start with the keyboard")
	}
	tester.Key(ui.Cmd|ui.Alt, ui.KeyLeft)
	tester.Frame()
	if !left.handle.Focused(ctx) {
		t.Fatal("navigating left should give the left pane the keyboard")
	}
}

func TestLiveResizeKeybind(t *testing.T) {
	w := liveWindow(t, []string{"alt+shift+l=resize_split:right,20"})
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	s := w.focusedSurface()
	if s == nil || s.term == nil {
		t.Fatalf("terminal did not start: %+v", s)
	}
	tab := w.store.Selected().SelectedTab()
	before := tab.Panes.Root.Pct
	// macOS sends the key, then the character Option composes for it. The
	// app keybind must take the key first, so the character never reaches the
	// program; TypeKey models that order.
	tester.TypeKey(ui.Alt|ui.Shift, ui.KeyL, "Ò")
	tester.Frame()
	if after := tab.Panes.Root.Pct; after == before {
		t.Fatalf("alt+shift+l did not resize the split (pct %v)", before)
	}
	if text := s.term.Text(); strings.Contains(text, "Ò") {
		t.Fatal("the character Option composed reached the terminal")
	}
}

func TestNoteFocusedMovesTabFocus(t *testing.T) {
	a := testApp(t)
	w := &WindowState{app: a, store: model.NewStore[*Surface](), sidebarHidden: true}
	w.newWorkspace("")
	w.splitFocused(model.DirRight)
	panes := w.store.Panes()
	if len(panes) != 2 {
		t.Fatalf("panes = %d, want 2", len(panes))
	}
	w.noteFocused(panes[0])
	if got := w.focusedSurface(); got != panes[0] {
		t.Fatalf("tab focus = %v, want the pane the user focused", got.id)
	}
}
