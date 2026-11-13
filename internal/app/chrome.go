package app

import (
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/config"
)

// rebuildChromeThemes reads the terminal settings once and derives the window
// chrome's colors from the terminal theme, so the sidebar, tab strip and menus
// match the terminal instead of the system appearance. Call it after the
// settings change; the views read the cached values, never the files.
func (a *App) rebuildChromeThemes() {
	a.terminalCfg = a.settings.Terminal(a.paths.Themes)
	a.chromeLight = chromeTheme(a.terminalCfg.Light)
	a.chromeDark = chromeTheme(a.terminalCfg.Dark)
	hide := a.hideScrollbar()
	a.termLight = terminalChrome(a.chromeLight, hide)
	a.termDark = terminalChrome(a.chromeDark, hide)
	a.syncAppAppearance()
}

// hideScrollbar reports whether the terminals hide their scroll bar, under
// Ghostty's scrollbar = never.
func (a *App) hideScrollbar() bool {
	return a.settings.String(config.KeyScrollbar, "system") == "never"
}

// terminalChrome is the theme the terminal panes draw with. Ghostty's
// scrollbar = never hides the bar by drawing it in a transparent color, since
// the plugin colors its bar from the theme; TrackWidth shrinks with it so the
// plugin does not take the pointer over an invisible strip.
func terminalChrome(base *ui.Theme, hide bool) *ui.Theme {
	if !hide || base == nil {
		return base
	}
	t := *base
	t.Scrollbar = ui.Transparent
	t.ScrollbarStyle.TrackWidth = 1
	return &t
}

// currentTerminalChrome is the terminal theme for the window's appearance.
func (a *App) currentTerminalChrome(dark bool) *ui.Theme {
	if dark && a.termDark != nil {
		return a.termDark
	}
	if a.termLight != nil {
		return a.termLight
	}
	return a.currentChrome(dark)
}

// syncAppAppearance makes the app's native chrome (the Settings and Shortcuts
// title bars, the menus) match the terminal theme, so a dark theme does not
// sit under a light title bar. A light/dark theme pair follows the system
// appearance; one theme for both draws the whole app in its colors.
func (a *App) syncAppAppearance() {
	lightDark := isDark(a.terminalCfg.Light.Background)
	darkDark := isDark(a.terminalCfg.Dark.Background)
	switch {
	case !lightDark && darkDark:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	case lightDark:
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeLight)
	}
}

// chromeTheme builds the UI theme widgets use from the terminal's colors.
func chromeTheme(term *terminal.Theme) *ui.Theme {
	if term == nil {
		return ui.LightTheme()
	}
	var base *ui.Theme
	if isDark(term.Background) {
		base = ui.DarkTheme()
	} else {
		base = ui.LightTheme()
	}
	t := *base
	bg, fg := term.Background, term.Foreground
	t.Background = bg
	t.Text = fg
	t.Surface = bg.Mix(fg, 0.06)
	t.SurfaceHover = bg.Mix(fg, 0.10)
	t.SurfacePressed = bg.Mix(fg, 0.15)
	t.Border = bg.Mix(fg, 0.20)
	t.TextMuted = fg.Mix(bg, 0.45)
	t.Selection = bg.Mix(fg, 0.25)
	return &t
}

// applyWindowChrome sets a window's background to the terminal's, so the area
// behind the view matches while frames are drawn.
func (a *App) applyWindowChrome(w *WindowState) {
	if w == nil || w.win == nil {
		return
	}
	a.applyChromeToWindow(w.win)
	// background-opacity makes the whole window translucent, the closest a
	// single window layer comes to Ghostty's terminal background opacity.
	w.win.SetOpacity(a.terminalCfg.Opacity)
}

// applyChromeToWindow sets a window's background color, for windows that are
// not a WindowState, as Settings and Keyboard Shortcuts.
func (a *App) applyChromeToWindow(win *mygo.Window) {
	if win == nil {
		return
	}
	_ = win.SetBackgroundColor(a.windowBackgroundColor())
}

// windowBackgroundColor is the color behind the views for the appearance in
// effect.
func (a *App) windowBackgroundColor() string {
	theme := a.terminalCfg.Light
	if mygo.Theme.IsDark() {
		theme = a.terminalCfg.Dark
	}
	return hexColor(theme.Background)
}

// currentChrome is the UI theme for the window's appearance.
func (a *App) currentChrome(dark bool) *ui.Theme {
	if dark && a.chromeDark != nil {
		return a.chromeDark
	}
	if a.chromeLight != nil {
		return a.chromeLight
	}
	return ui.LightTheme()
}

// isDark reports whether a color is nearer black than white.
func isDark(c ui.Color) bool {
	lum := (0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)) / 255
	return lum < 0.5
}

// hexColor renders a color as "#rrggbb" for the window background.
func hexColor(c ui.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}
