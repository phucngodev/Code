package app

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/model"
)

// TestEveryMenuAcceleratorIsListed keeps the Keyboard Shortcuts window and the
// menus in step, as the project's shortcut rule requires: every accelerator
// Code binds has a row, and every row's accelerator is a menu or Code's own.
func TestEveryMenuAcceleratorIsListed(t *testing.T) {
	listed := map[string]bool{}
	for _, section := range shortcutList {
		for _, s := range section.shortcuts {
			if s.accelerator != "" {
				listed[s.accelerator] = true
			}
			for _, covered := range s.covers {
				listed[covered] = true
			}
		}
	}
	// acc is the menu's own shortcuts: every field must have a row.
	v := reflect.ValueOf(acc)
	for i := 0; i < v.NumField(); i++ {
		if a := v.Field(i).String(); a != "" && !listed[a] {
			t.Errorf("Code binds %q but no shortcut row lists it", a)
		}
	}
	// Every plain menu item uses one of them, including the 1-9 workspace and
	// tab ranges. Role items take their keys from the OS, so they are skipped;
	// their Code-set accelerators are already covered by acc above.
	menu := (&App{}).buildMenu()
	var walk func([]*mygo.MenuItem)
	walk = func(items []*mygo.MenuItem) {
		for _, item := range items {
			if item.Role == "" && item.Accelerator != "" && !listed[item.Accelerator] {
				t.Errorf("menu %q binds %q, which no shortcut row lists", item.Label, item.Accelerator)
			}
			walk(item.Submenu)
		}
	}
	walk(menu.Items())
}

func TestShortcutKeysArePlatformAware(t *testing.T) {
	cases := []struct {
		shortcut shortcut
		goos     string
		want     []string
	}{
		{shortcut{accelerator: "Shift+CmdOrCtrl+,"}, "darwin", []string{"⇧", "⌘", ","}},
		{shortcut{accelerator: "Shift+CmdOrCtrl+,"}, "linux", []string{"Shift", "Ctrl", ","}},
		{shortcut{accelerator: "Ctrl+Shift+F"}, "linux", []string{"Ctrl", "Shift", "F"}},
		{shortcut{accelerator: "CmdOrCtrl+Equal"}, "darwin", []string{"⌘", "="}},
		{shortcut{accelerator: "Alt+CmdOrCtrl+Left"}, "darwin", []string{"⌥", "⌘", "←"}},
		{shortcut{accelerator: "Alt+CmdOrCtrl+Left"}, "linux", []string{"Alt", "Ctrl", "←"}},
		{shortcut{mac: []string{"⌥", "1–8"}, other: []string{"Alt", "1–8"}}, "linux", []string{"Alt", "1–8"}},
	}
	for _, tc := range cases {
		got := shortcutKeys(tc.shortcut, tc.goos)
		if len(got) != len(tc.want) {
			t.Fatalf("shortcutKeys(%v, %q) = %v, want %v", tc.shortcut, tc.goos, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("shortcutKeys(%v, %q) = %v, want %v", tc.shortcut, tc.goos, got, tc.want)
			}
		}
	}
}

// TestPaneCloseKeyKeepsControlWFreeOnLinux checks the close-pane default does
// not collide with the Control+W a program such as vim reads itself.
func TestPaneCloseKeyKeepsControlWFreeOnLinux(t *testing.T) {
	cases := map[string]string{
		"darwin":  "Cmd+W",
		"linux":   "Alt+W",
		"windows": "Alt+W",
	}
	for goos, want := range cases {
		if got := paneCloseKey(goos); got != want {
			t.Errorf("paneCloseKey(%q) = %q, want %q", goos, got, want)
		}
	}
}

func TestShortcutListNamesEveryAppShortcut(t *testing.T) {
	titles := map[string]bool{}
	for _, section := range shortcutList {
		for _, s := range section.shortcuts {
			titles[s.title] = true
		}
	}
	for _, title := range []string{
		"Focus the next pane", "Focus the previous pane",
		"Close the find bar", "Dismiss the go-to palette",
		"Open Settings", "Reload configuration",
	} {
		if !titles[title] {
			t.Errorf("missing shortcut row %q", title)
		}
	}
}

// TestFindBarClosesOnEscape checks the overlay shortcut the find bar shares
// with the command palette.
func TestFindBarClosesOnEscape(t *testing.T) {
	w := &WindowState{app: testApp(t), store: model.NewStore[*Surface](), sidebarHidden: true}
	w.newWorkspace("")
	w.openFind()
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()
	if !w.find.open {
		t.Fatal("the find bar should be open")
	}
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if w.find.open {
		t.Fatal("Escape should close the find bar")
	}
}

// TestAskCloseKeepsTheFirstAlert checks a second close request does not drop
// the confirmation already waiting for the user.
func TestAskCloseKeepsTheFirstAlert(t *testing.T) {
	w := testWindow(t)
	first, second := false, false
	w.askClose("First", "one", func() { first = true })
	w.askClose("Second", "two", func() { second = true })
	if w.pending == nil || w.pending.title != "First" {
		t.Fatalf("pending = %+v, want the first alert", w.pending)
	}
	w.pending.close()
	if !first || second {
		t.Fatal("only the first close should run")
	}
}
