package app

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/config"
	"github.com/phucngodev/code/internal/model"
)

// windowBinding is one key bound to something the window does.
type windowBinding struct {
	mods ui.Modifiers
	key  ui.Key
	run  func()
}

// applyWindowKeybinds registers the window shortcuts Code always offers and
// every app action the config binds, so a Ghostty keybind line like
// `goto_split` or `new_tab` works. A config binding replaces a default of the
// same key, and an unbind drops it.
func (w *WindowState) applyWindowKeybinds(c *ui.Context) {
	for _, b := range w.windowBindings() {
		c.OnWindowShortcut(b.mods, b.key, b.run)
	}
}

func (w *WindowState) windowBindings() []windowBinding {
	type keyID struct {
		mods ui.Modifiers
		key  ui.Key
	}
	// One binding per key, so a config binding replaces a default of the same
	// key and an unbind or ignore drops it.
	runs := map[keyID]func(){}
	add := func(mods ui.Modifiers, key ui.Key, run func()) {
		runs[keyID{mods, key}] = run
	}
	add(ui.Cmd|ui.Alt, ui.KeyLeft, func() { w.goToPane(model.NavLeft) })
	add(ui.Cmd|ui.Alt, ui.KeyRight, func() { w.goToPane(model.NavRight) })
	add(ui.Cmd|ui.Alt, ui.KeyUp, func() { w.goToPane(model.NavUp) })
	add(ui.Cmd|ui.Alt, ui.KeyDown, func() { w.goToPane(model.NavDown) })
	// Ghostty's pane switching: the next and previous pane in layout order,
	// which is Command+bracket on macOS and Control+Super+bracket elsewhere.
	add(ui.Cmd|ui.Super, ui.KeyBracketLeft, func() { w.goToPane(model.NavPrevious) })
	add(ui.Cmd|ui.Super, ui.KeyBracketRight, func() { w.goToPane(model.NavNext) })
	for _, kb := range w.app.keybinds {
		// A binding that uses Alt is left to the terminal's OnKey
		// (runAppKeybind): on macOS Option reaches the input method and
		// composes a character unless the focused terminal takes the key
		// first, which it cannot while a window shortcut claims it. OnKey
		// runs first and suppresses the character. An unbind or ignore only
		// drops a default, and the terminal answers the key itself.
		if kb.Mods&ui.Alt != 0 || kb.Action == "unbind" || kb.Action == "ignore" {
			delete(runs, keyID{kb.Mods, kb.Key})
			continue
		}
		if run := w.appAction(kb); run != nil {
			add(kb.Mods, kb.Key, run)
		}
	}
	out := make([]windowBinding, 0, len(runs))
	for id, run := range runs {
		if run != nil {
			out = append(out, windowBinding{id.mods, id.key, run})
		}
	}
	return out
}

// appAction turns a Ghostty action into what the window does for it, or nil
// when the action is not one the window runs (a terminal action, say).
func (w *WindowState) appAction(kb config.Keybind) func() {
	switch kb.Action {
	case "goto_split":
		nav := splitNavigation(kb.Arg)
		return func() { w.goToPane(nav) }
	case "resize_split":
		dir, amount := splitResize(kb.Arg)
		return func() { w.resizePane(dir, amount) }
	case "new_split":
		dir := splitDirection(kb.Arg)
		return func() { w.splitFocused(dir) }
	case "close_surface":
		return func() {
			if s := w.focusedSurface(); s != nil {
				w.closeSurface(s)
			}
		}
	case "toggle_split_zoom":
		return w.toggleZoom
	case "equalize_splits":
		return func() {
			if tab := w.selectedTab(); tab != nil {
				tab.Panes.Equalize()
				w.store.Notify()
			}
		}
	case "new_tab":
		return func() { w.newTab("") }
	case "close_tab":
		return func() {
			if tab := w.selectedTab(); tab != nil {
				w.closeTab(tab)
			}
		}
	case "next_tab":
		return func() { w.cycleTab(1) }
	case "previous_tab":
		return func() { w.cycleTab(-1) }
	case "goto_tab":
		return w.gotoTabAction(kb.Arg)
	case "new_window":
		return func() { w.app.newWindow(nil) }
	case "close_window":
		return func() {
			if w.win != nil {
				w.win.Close()
			}
		}
	case "toggle_fullscreen":
		return func() {
			if w.win != nil {
				w.win.SetFullScreen(!w.win.IsFullScreen())
			}
		}
	case "toggle_sidebar":
		return w.toggleSidebar
	case "reload_config":
		return w.app.ReloadConfig
	default:
		return nil
	}
}

// gotoTabAction binds `goto_tab:N`, `goto_tab:next`, `goto_tab:previous` and
// `goto_tab:last`, or nil when the argument names no tab.
func (w *WindowState) gotoTabAction(arg string) func() {
	switch strings.TrimSpace(arg) {
	case "next":
		return func() { w.cycleTab(1) }
	case "previous":
		return func() { w.cycleTab(-1) }
	case "last":
		return func() {
			if ws := w.store.Selected(); ws != nil {
				w.selectTabIndex(len(ws.Tabs) - 1)
			}
		}
	}
	n, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil || n < 1 {
		return nil
	}
	return func() { w.selectTabIndex(n - 1) }
}

// resizePane moves the divider nearest the focused pane by amount pixels, as
// Ghostty's resize_split does. The split's own extent, not the whole window,
// is what the amount is measured against.
func (w *WindowState) resizePane(dir model.Direction, amount float32) {
	s := w.focusedSurface()
	if s == nil {
		return
	}
	_, tab, ok := w.store.WorkspaceContaining(s)
	if !ok {
		return
	}
	extent := tab.Panes.DividerExtent(s, dir)
	if extent <= 0 {
		return
	}
	area := w.terminalAreaSize(dir)
	if area <= 0 {
		return
	}
	tab.Panes.Resize(s, dir, amount/(extent*area)*100)
	w.store.Notify()
}

// terminalAreaSize is the pixel size of the terminal area along the axis dir
// moves, approximating the room the window bars and sidebar leave.
func (w *WindowState) terminalAreaSize(dir model.Direction) float32 {
	var size float64
	if dir == model.DirLeft || dir == model.DirRight {
		size = float64(w.bounds.Width)
		if !w.sidebarHidden {
			size -= float64(w.sidebarWidth)
		}
	} else {
		size = float64(w.bounds.Height) - 60
	}
	if size < 100 {
		// The window has not been measured yet; a typical width keeps a
		// press useful rather than a no-op.
		size = 1000
	}
	return float32(size)
}

// cycleTab selects the tab offset steps from the selected one, wrapping.
func (w *WindowState) cycleTab(offset int) {
	ws := w.store.Selected()
	if ws == nil || len(ws.Tabs) == 0 {
		return
	}
	index := 0
	for i, tab := range ws.Tabs {
		if tab == ws.SelectedTab() {
			index = i
			break
		}
	}
	index = ((index+offset)%len(ws.Tabs) + len(ws.Tabs)) % len(ws.Tabs)
	w.selectTabIndex(index)
}

// selectedTab is the tab the selected workspace shows.
func (w *WindowState) selectedTab() *model.Tab[*Surface] {
	ws := w.store.Selected()
	if ws == nil {
		return nil
	}
	return ws.SelectedTab()
}

// splitNavigation reads a goto_split argument.
func splitNavigation(arg string) model.Navigation {
	switch strings.TrimSpace(arg) {
	case "left":
		return model.NavLeft
	case "right":
		return model.NavRight
	case "up":
		return model.NavUp
	case "down":
		return model.NavDown
	case "previous":
		return model.NavPrevious
	default:
		return model.NavNext
	}
}

// splitDirection reads the direction of a new_split or resize_split argument,
// which may carry an amount after a comma.
func splitDirection(arg string) model.Direction {
	name, _, _ := strings.Cut(strings.TrimSpace(arg), ",")
	switch strings.TrimSpace(name) {
	case "left":
		return model.DirLeft
	case "up":
		return model.DirUp
	case "down":
		return model.DirDown
	default:
		return model.DirRight
	}
}

// splitResize reads a resize_split argument, `direction[,amount]`, with
// Ghostty's default amount of 10 pixels.
func splitResize(arg string) (model.Direction, float32) {
	dir := splitDirection(arg)
	_, amount, ok := strings.Cut(arg, ",")
	if !ok {
		return dir, 10
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(amount), 32)
	if err != nil || f <= 0 {
		return dir, 10
	}
	return dir, float32(f)
}

// runAppKeybind runs the app action a config keybind gives this key, before
// the terminal encodes it, so an Option binding is not sent to the program as
// text when the window shortcut did not claim the key. It reports whether a
// binding ran.
func (a *App) runAppKeybind(w *WindowState, mods ui.Modifiers, key ui.Key) bool {
	if w == nil {
		return false
	}
	for _, kb := range a.keybinds {
		if kb.Mods != mods || kb.Key != key {
			continue
		}
		switch kb.Action {
		case "unbind":
			return false
		case "ignore":
			// Ghostty's ignore takes the key and does nothing, so the input
			// method's composed character must not reach the program.
			return true
		}
		if run := w.appAction(kb); run != nil {
			run()
			return true
		}
	}
	return false
}

// terminalKeybinds maps the config's bindings to the terminal plugin's, so
// copy, paste, zoom and scroll keys reach the terminal instead of the window.
func terminalKeybinds(bindings []config.Keybind) []terminal.Keybind {
	var out []terminal.Keybind
	for _, kb := range bindings {
		action, ok := terminalActions[kb.Action]
		if !ok {
			continue
		}
		out = append(out, terminal.Keybind{Mods: kb.Mods, Key: kb.Key, Action: action, Performable: kb.Performable})
	}
	return out
}

// terminalActions maps the Ghostty actions the terminal plugin runs itself.
var terminalActions = map[string]terminal.Action{
	"unbind":               terminal.ActionNone,
	"ignore":               terminal.ActionIgnore,
	"copy_to_clipboard":    terminal.ActionCopy,
	"paste_from_clipboard": terminal.ActionPaste,
	"select_all":           terminal.ActionSelectAll,
	"clear_screen":         terminal.ActionClearScreen,
	"increase_font_size":   terminal.ActionIncreaseFontSize,
	"decrease_font_size":   terminal.ActionDecreaseFontSize,
	"reset_font_size":      terminal.ActionResetFontSize,
	"scroll_page_up":       terminal.ActionScrollPageUp,
	"scroll_page_down":     terminal.ActionScrollPageDown,
	"scroll_to_top":        terminal.ActionScrollToTop,
	"scroll_to_bottom":     terminal.ActionScrollToBottom,
}
