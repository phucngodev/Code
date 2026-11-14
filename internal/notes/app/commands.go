package app

import (
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/notes/theme"
)

// command is a command of the palette: what it runs, by id.
type command struct {
	id, title, keys string
}

// The modifiers of the notes view's palettes. macOS uses Command, as its apps
// do; elsewhere Code binds Control+Shift for its own commands (find, go to),
// so the notes view takes Control+Alt instead.
var (
	openMods    = ui.Cmd
	searchMods  = ui.Cmd | ui.Shift
	commandMods = ui.Cmd | ui.Shift
)

func init() {
	if runtime.GOOS != "darwin" {
		searchMods = ui.Cmd | ui.Alt
		commandMods = ui.Cmd | ui.Alt
	}
}

// accel is the accelerator for a shortcut that differs on macOS and elsewhere.
func accel(mac, other string) string {
	if runtime.GOOS == "darwin" {
		return mac
	}
	return other
}

// commands are the notes view's commands, in the order the palette lists them.
var commands = []command{
	{id: "newNote", title: "New Note", keys: keysLabel("CmdOrCtrl+N")},
	{id: "quickOpen", title: "Open a Note", keys: keysLabel("CmdOrCtrl+O")},
	{id: "searchNotes", title: "Search Notes", keys: keysLabel(accel("CmdOrCtrl+Shift+F", "Alt+CmdOrCtrl+F"))},
	{id: "commands", title: "Commands", keys: keysLabel(accel("CmdOrCtrl+Shift+P", "Alt+CmdOrCtrl+P"))},
	{id: "closeTab", title: "Close Note", keys: keysLabel("CmdOrCtrl+W")},
	{id: "nextTab", title: "Next Note", keys: keysLabel("Ctrl+Tab")},
	{id: "previousTab", title: "Previous Note", keys: keysLabel("Ctrl+Shift+Tab")},
	{id: "toggleSidebar", title: "Toggle Sidebar", keys: ""},
	{id: "toggleBold", title: "Bold", keys: keysLabel("CmdOrCtrl+B")},
	{id: "toggleItalic", title: "Italic", keys: keysLabel("CmdOrCtrl+I")},
	{id: "toggleStrikethrough", title: "Strikethrough", keys: keysLabel("CmdOrCtrl+Shift+X")},
	{id: "toggleHighlight", title: "Highlight", keys: keysLabel("CmdOrCtrl+Shift+H")},
	{id: "toggleInlineCode", title: "Code", keys: keysLabel("CmdOrCtrl+Shift+E")},
	{id: "insertLink", title: "Link", keys: keysLabel("CmdOrCtrl+K")},
	{id: "toggleQuote", title: "Quote", keys: keysLabel("CmdOrCtrl+'")},
	{id: "toggleBulletList", title: "Bulleted List", keys: keysLabel("CmdOrCtrl+Shift+8")},
	{id: "toggleNumberedList", title: "Numbered List", keys: keysLabel("CmdOrCtrl+Shift+7")},
	{id: "toggleTaskList", title: "Task List", keys: keysLabel("CmdOrCtrl+Shift+9")},
	{id: "toggleCodeBlock", title: "Code Block", keys: keysLabel("CmdOrCtrl+Alt+C")},
}

// commandList is the commands the palette offers.
func (a *App) commandList() []command { return commands }

func itoa(n int) string { return strconv.Itoa(n) }

// run runs a command by id, handing anything else to the editor.
func (a *App) run(id string) {
	switch id {
	case "newNote":
		a.NewNote()
	case "quickOpen":
		a.palette.show(modeFiles, "")
	case "searchNotes":
		a.palette.show(modeSearch, "")
	case "commands":
		a.palette.show(modeCommands, "")
	case "closeTab":
		a.CloseTab()
	case "nextTab":
		a.NextTab()
	case "previousTab":
		a.PrevTab()
	case "toggleSidebar":
		a.ToggleSidebar()
	default:
		if ed := a.editor(); ed != nil {
			ed.Run(id)
		}
	}
}

// keysLabel is an accelerator as the palette shows it.
func keysLabel(accel string) string {
	if accel == "" {
		return ""
	}
	if runtime.GOOS != "darwin" {
		return strings.NewReplacer("CmdOrCtrl+", "Ctrl+").Replace(accel)
	}
	r := strings.NewReplacer("CmdOrCtrl+", "⌘", "Ctrl+", "⌃", "Shift+", "⇧", "Alt+", "⌥", "Backspace", "⌫", "Enter", "↩", "Plus", "+", "Tab", "⇥")
	// The modifiers in the order macOS's menus show them: ⌃⌥⇧⌘.
	parts := strings.Split(accel, "+")
	key := parts[len(parts)-1]
	var mods string
	for _, m := range []string{"Ctrl", "Alt", "Shift", "CmdOrCtrl"} {
		if slices.Contains(parts[:len(parts)-1], m) {
			mods += m + "+"
		}
	}
	return r.Replace(mods + key)
}

// keycaps shows a shortcut's keys, each on a cap.
func keycaps(c *ui.Context, th theme.Theme, keys string) {
	ui.Row(c).Gap(3).Children(func() {
		for _, k := range splitKeys(keys) {
			ui.Box(c).MinWidth(20).Height(20).Padding(0, 5).Radius(5).Center().
				Background(th.Text.Alpha(0.05)).Border(1, th.Border).Children(func() {
				ui.Text(c, k).FontSize(11).TextColor(th.Secondary)
			})
		}
	})
}

// splitKeys splits a shortcut's label into its keys.
func splitKeys(keys string) []string {
	if !strings.ContainsAny(keys, "⌘⌃⇧⌥") && strings.Contains(keys, "+") && len(keys) > 1 {
		parts := strings.Split(keys, "+")
		if parts[len(parts)-1] == "" {
			parts = append(parts[:len(parts)-2], "+")
		}
		return parts
	}
	var out []string
	rs := []rune(keys)
	for i, r := range rs {
		if strings.ContainsRune("⌘⌃⇧⌥", r) || i == len(rs)-1 {
			out = append(out, string(r))
			continue
		}
		out = append(out, string(rs[i:]))
		break
	}
	return out
}
