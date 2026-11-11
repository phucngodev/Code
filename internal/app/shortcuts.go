package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// shortcut is one row of the Keyboard Shortcuts window.
type shortcut struct {
	title string
	keys  []string
}

// shortcutSection groups shortcuts by task.
type shortcutSection struct {
	title     string
	shortcuts []shortcut
}

// shortcutList is every shortcut Code adds, grouped by what it does.
var shortcutList = []shortcutSection{
	{"Workspaces", []shortcut{
		{"New workspace", []string{"⌘", "N"}},
		{"Switch workspace", []string{"⌥", "1–8"}},
		{"Last workspace", []string{"⌥", "9"}},
		{"Previous workspace", []string{"⇧", "⌘", "["}},
		{"Next workspace", []string{"⇧", "⌘", "]"}},
		{"Rename workspace", []string{"⇧", "⌘", "R"}},
		{"Show or hide the sidebar", []string{"⌘", "B"}},
	}},
	{"Go to", []shortcut{
		{"Go to a tab, pane, workspace or agent", []string{"⌘", "P"}},
	}},
	{"Tabs and panes", []shortcut{
		{"New tab", []string{"⌘", "T"}},
		{"Switch tab", []string{"⌘", "1–8"}},
		{"Last tab", []string{"⌘", "9"}},
		{"Split right", []string{"⌘", "D"}},
		{"Split down", []string{"⇧", "⌘", "D"}},
		{"Zoom pane", []string{"⇧", "⌘", "↩"}},
		{"Close pane", []string{"⌘", "W"}},
		{"Close tab", []string{"⌥", "⌘", "W"}},
	}},
	{"Pane focus", []shortcut{
		{"Focus the pane on the left", []string{"⌥", "⌘", "←"}},
		{"Focus the pane on the right", []string{"⌥", "⌘", "→"}},
		{"Focus the pane above", []string{"⌥", "⌘", "↑"}},
		{"Focus the pane below", []string{"⌥", "⌘", "↓"}},
	}},
	{"Windows", []shortcut{
		{"New window", []string{"⇧", "⌘", "N"}},
		{"Close window", []string{"⇧", "⌘", "W"}},
		{"Minimize", []string{"⌘", "M"}},
		{"Fill the screen", []string{"⌃", "⌘", "F"}},
		{"Center the window", []string{"⌃", "⌘", "X"}},
		{"Tile left half", []string{"⌃", "⌘", "←"}},
		{"Tile right half", []string{"⌃", "⌘", "→"}},
		{"Tile top half", []string{"⌃", "⌘", "↑"}},
		{"Tile bottom half", []string{"⌃", "⌘", "↓"}},
	}},
	{"Terminal", []shortcut{
		{"Find", []string{"⌘", "F"}},
		{"Find next", []string{"⌘", "G"}},
		{"Find previous", []string{"⇧", "⌘", "G"}},
		{"Bigger text", []string{"⌘", "="}},
		{"Smaller text", []string{"⌘", "-"}},
		{"Reset text size", []string{"⌘", "0"}},
	}},
	{"Settings", []shortcut{
		{"Open Settings", []string{"⌘", ","}},
		{"Reload configuration", []string{"⇧", "⌘", ","}},
	}},
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
							for _, key := range s.keys {
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
