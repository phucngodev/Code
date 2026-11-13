package app

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// shortcut is one row of the Keyboard Shortcuts window.
type shortcut struct {
	title string
	// accelerator is the MyGo accelerator the menus bind. Menus and this list
	// share acc, so a shortcut cannot be added to one without the other.
	accelerator string
	// mac and other override the rendered keys for a range of numbers or a
	// combination an accelerator string cannot express.
	mac   []string
	other []string
	// covers is every accelerator a range row stands for, so a test can check
	// that no menu shortcut is missing from the list.
	covers []string
}

// shortcutSection groups shortcuts by task.
type shortcutSection struct {
	title     string
	shortcuts []shortcut
}

// accelerators are the keys Code binds in its menus. The Keyboard Shortcuts
// window renders the same values, so the list cannot drift from the menus.
var acc = struct {
	settings, reloadConfig                        string
	newWindow, newTab, closePane, closeTab        string
	closeWindow, splitRight, splitDown            string
	copy, paste, selectAll, find                  string
	findNext, findPrevious                        string
	toggleSidebar, goTo, zoomPane                 string
	prevWorkspace, nextWorkspace, renameWorkspace string
	minimize                                      string
	fill, center, previousSize                    string
	tileLeft, tileRight, tileTop, tileBottom      string
}{
	settings:        "CmdOrCtrl+,",
	reloadConfig:    "Shift+CmdOrCtrl+,",
	newWindow:       "Shift+CmdOrCtrl+N",
	newTab:          "CmdOrCtrl+T",
	closePane:       paneCloseKey(runtime.GOOS),
	closeTab:        "Alt+CmdOrCtrl+W",
	closeWindow:     "Shift+CmdOrCtrl+W",
	splitRight:      "CmdOrCtrl+D",
	splitDown:       "Shift+CmdOrCtrl+D",
	copy:            appKey("C"),
	paste:           appKey("V"),
	selectAll:       appKey("A"),
	find:            appKey("F"),
	findNext:        "CmdOrCtrl+G",
	findPrevious:    "Shift+CmdOrCtrl+G",
	toggleSidebar:   "CmdOrCtrl+B",
	goTo:            appKey("P"),
	zoomPane:        "Shift+CmdOrCtrl+Enter",
	prevWorkspace:   "Shift+CmdOrCtrl+[",
	nextWorkspace:   "Shift+CmdOrCtrl+]",
	renameWorkspace: "Shift+CmdOrCtrl+R",
	minimize:        "CmdOrCtrl+M",
	fill:            "Ctrl+Shift+F",
	center:          "Ctrl+Shift+X",
	previousSize:    "Ctrl+Shift+R",
	tileLeft:        "Ctrl+Shift+Left",
	tileRight:       "Ctrl+Shift+Right",
	tileTop:         "Ctrl+Shift+Up",
	tileBottom:      "Ctrl+Shift+Down",
}

// paneCloseKey closes the focused pane. macOS keeps Command+W, the system's
// close gesture; elsewhere Alt+W, because vim and other programs read
// Control+W for their own window commands.
func paneCloseKey(goos string) string {
	if goos == "darwin" {
		return "Cmd+W"
	}
	return "Alt+W"
}

// workspaceAccelerator and tabAccelerator are shared by the Workspace menu and
// the shortcuts list, which shows their 1-8 as a range.
func workspaceAccelerator(i int) string { return fmt.Sprintf("Alt+%d", i) }
func tabAccelerator(i int) string       { return fmt.Sprintf("CmdOrCtrl+%d", i) }

func workspaceAccelerators() []string {
	var out []string
	for i := 1; i <= 8; i++ {
		out = append(out, workspaceAccelerator(i))
	}
	return out
}

func tabAccelerators() []string {
	var out []string
	for i := 1; i <= 8; i++ {
		out = append(out, tabAccelerator(i))
	}
	return out
}

// shortcutList is every shortcut Code adds, grouped by what it does.
var shortcutList = []shortcutSection{
	{"Workspaces", []shortcut{
		{"Switch workspace", "", []string{"⌥", "1–8"}, []string{"Alt", "1–8"}, workspaceAccelerators()},
		{"Last workspace", workspaceAccelerator(9), nil, nil, nil},
		{"Previous workspace", acc.prevWorkspace, nil, nil, nil},
		{"Next workspace", acc.nextWorkspace, nil, nil, nil},
		{"Rename workspace", acc.renameWorkspace, nil, nil, nil},
		{"Show or hide the sidebar", acc.toggleSidebar, nil, nil, nil},
	}},
	{"Go to", []shortcut{
		{"Go to a tab, pane, workspace or agent", acc.goTo, nil, nil, nil},
		{"Dismiss the go-to palette", "", []string{"Esc"}, []string{"Esc"}, nil},
	}},
	{"Tabs and panes", []shortcut{
		{"New tab", acc.newTab, nil, nil, nil},
		{"Switch tab", "", []string{"⌘", "1–8"}, []string{"Ctrl", "1–8"}, tabAccelerators()},
		{"Last tab", tabAccelerator(9), nil, nil, nil},
		{"Split right", acc.splitRight, nil, nil, nil},
		{"Split down", acc.splitDown, nil, nil, nil},
		{"Zoom pane", acc.zoomPane, nil, nil, nil},
		{"Close pane", acc.closePane, nil, nil, nil},
		{"Close tab", acc.closeTab, nil, nil, nil},
	}},
	{"Pane focus", []shortcut{
		{"Focus the next pane", "", []string{"⌘", "]"}, []string{"Ctrl", "Super", "]"}, nil},
		{"Focus the previous pane", "", []string{"⌘", "["}, []string{"Ctrl", "Super", "["}, nil},
		{"Focus the pane on the left", "Alt+CmdOrCtrl+Left", nil, nil, nil},
		{"Focus the pane on the right", "Alt+CmdOrCtrl+Right", nil, nil, nil},
		{"Focus the pane above", "Alt+CmdOrCtrl+Up", nil, nil, nil},
		{"Focus the pane below", "Alt+CmdOrCtrl+Down", nil, nil, nil},
	}},
	{"Windows", []shortcut{
		{"New window", acc.newWindow, nil, nil, nil},
		{"Close window", acc.closeWindow, nil, nil, nil},
		{"Minimize", acc.minimize, nil, nil, nil},
		{"Fill the screen", acc.fill, nil, nil, nil},
		{"Center the window", acc.center, nil, nil, nil},
		{"Return to previous size", acc.previousSize, nil, nil, nil},
		{"Tile left half", acc.tileLeft, nil, nil, nil},
		{"Tile right half", acc.tileRight, nil, nil, nil},
		{"Tile top half", acc.tileTop, nil, nil, nil},
		{"Tile bottom half", acc.tileBottom, nil, nil, nil},
	}},
	{"Terminal", []shortcut{
		{"Copy", acc.copy, nil, nil, nil},
		{"Paste", acc.paste, nil, nil, nil},
		{"Select all", acc.selectAll, nil, nil, nil},
		{"Find", acc.find, nil, nil, nil},
		{"Close the find bar", "", []string{"Esc"}, []string{"Esc"}, nil},
		{"Find next", acc.findNext, nil, nil, nil},
		{"Find previous", acc.findPrevious, nil, nil, nil},
		{"Bigger text", "CmdOrCtrl+Equal", nil, nil, nil},
		{"Smaller text", "CmdOrCtrl+Minus", nil, nil, nil},
		{"Reset text size", "CmdOrCtrl+0", nil, nil, nil},
	}},
	{"Settings", []shortcut{
		{"Open Settings", acc.settings, nil, nil, nil},
		{"Reload configuration", acc.reloadConfig, nil, nil, nil},
	}},
}

// shortcutKeys renders an accelerator as the keys the current platform shows.
func shortcutKeys(s shortcut, goos string) []string {
	if goos == "darwin" {
		if len(s.mac) > 0 {
			return s.mac
		}
		return formatAccelerator(s.accelerator, true)
	}
	if len(s.other) > 0 {
		return s.other
	}
	return formatAccelerator(s.accelerator, false)
}

func formatAccelerator(accelerator string, mac bool) []string {
	if accelerator == "" {
		return nil
	}
	parts := strings.Split(accelerator, "+")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if glyph := keyGlyph(part, mac); glyph != "" {
			out = append(out, glyph)
		}
	}
	return out
}

// keyGlyph names one part of an accelerator as the key a user presses.
func keyGlyph(part string, mac bool) string {
	switch part {
	case "CmdOrCtrl":
		if mac {
			return "⌘"
		}
		return "Ctrl"
	case "Cmd":
		if mac {
			return "⌘"
		}
		return "Super"
	case "Super":
		if mac {
			return "⌘"
		}
		return "Super"
	case "Alt":
		if mac {
			return "⌥"
		}
		return "Alt"
	case "Shift":
		if mac {
			return "⇧"
		}
		return "Shift"
	case "Ctrl":
		if mac {
			return "⌃"
		}
		return "Ctrl"
	case "Enter":
		if mac {
			return "↩"
		}
		return "Enter"
	case "Left":
		return "←"
	case "Right":
		return "→"
	case "Up":
		return "↑"
	case "Down":
		return "↓"
	case "Equal":
		return "="
	case "Minus":
		return "-"
	case "BracketLeft":
		return "["
	case "BracketRight":
		return "]"
	case "Escape":
		return "Esc"
	}
	return part
}

// ShowShortcuts opens the Keyboard Shortcuts window.
func (a *App) ShowShortcuts() {
	if a.shortcutsWin == nil {
		a.shortcutsWin = mygo.NewWindow(mygo.WindowOptions{
			Title:     "Keyboard Shortcuts",
			Width:     520,
			Height:    620,
			MinWidth:  360,
			MinHeight: 320,
			Content:   ui.View(a.shortcutsView),
		})
		a.shortcutsWin.OnClosed(func() { a.shortcutsWin = nil })
		a.applyChromeToWindow(a.shortcutsWin)
	}
	a.shortcutsWin.Show()
	a.shortcutsWin.Focus()
}

func (a *App) shortcutsView(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Fill().Padding(16).Children(func() {
		ui.Column(c).Gap(16).Children(func() {
			for _, section := range shortcutList {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, section.title).Bold()
					for _, s := range section.shortcuts {
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Text(c, s.title).Grow(1)
							for _, key := range shortcutKeys(s, runtime.GOOS) {
								ui.Text(c, key).FontSize(12).Padding(2, 6).
									Background(t.Surface).Radius(4).TextColor(t.TextMuted)
							}
						})
					}
				})
			}
		})
	})
}
