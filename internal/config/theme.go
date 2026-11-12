package config

import (
	"embed"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// DefaultTheme is the theme Code uses when the config names none. It is one of
// the themes Code ships, so the picker lists it too.
const DefaultTheme = "modus"

// bundledThemes are the themes Code ships: light and dark pairs for a handful
// of popular color schemes.
//
//go:embed themes/*
var bundledThemes embed.FS

// Option keys the Settings window manages.
const (
	KeyFontFamily            = "font-family"
	KeyFontSize              = "font-size"
	KeyFontThicken           = "font-thicken"
	KeyTheme                 = "theme"
	KeyCursorStyle           = "cursor-style"
	KeyCursorBlink           = "cursor-style-blink"
	KeyBackgroundOpacity     = "background-opacity"
	KeyUnfocusedSplitOpacity = "unfocused-split-opacity"
	KeyScrollback            = "scrollback-limit"
	KeyCommand               = "command"
	KeyCopyOnSelect          = "copy-on-select"
	KeyShellIntegration      = "shell-integration"
	KeyConfirmClose          = "confirm-close-surface"
	KeyMouseHideWhileTyping  = "mouse-hide-while-typing"
	KeyAutoSecureInput       = "macos-auto-secure-input"
	KeyNotifyOnFinish        = "notify-on-command-finish"
	KeyNotifyOnFinishAction  = "notify-on-command-finish-action"
	KeyNotifyOnFinishAfter   = "notify-on-command-finish-after"
	KeyWorkingDirectory      = "working-directory"
)

// More Ghostty options Code reads for the terminals, beside those the Settings
// window manages.
const (
	KeyFontFeature         = "font-feature"
	KeyAdjustCellWidth     = "adjust-cell-width"
	KeyAdjustCellHeight    = "adjust-cell-height"
	KeyBackground          = "background"
	KeyForeground          = "foreground"
	KeyCursorColor         = "cursor-color"
	KeyCursorText          = "cursor-text"
	KeySelectionBackground = "selection-background"
	KeySelectionForeground = "selection-foreground"
	KeyPalette             = "palette"
	KeySearchBackground    = "search-background"
	KeySearchForeground    = "search-foreground"
	KeySearchSelBackground = "search-selected-background"
	KeySearchSelForeground = "search-selected-foreground"
	KeyScrollMultiplier    = "mouse-scroll-multiplier"
	KeyRightClickAction    = "right-click-action"
	KeyClipboardWrite      = "clipboard-write"
	KeyClipboardRead       = "clipboard-read"
	KeyMinimumContrast     = "minimum-contrast"
	KeyImageLimit          = "image-storage-limit"
	KeyWindowPaddingX      = "window-padding-x"
	KeyWindowPaddingY      = "window-padding-y"
	KeyWindowPaddingColor  = "window-padding-color"
	KeyOptionAsAlt         = "macos-option-as-alt"
)

// Code-only preference keys, which Ghostty has no option for. They live in the
// same file; Ghostty never reads it.
const (
	KeyAgentNeedsInputNotify = "code-agent-needs-input-notification"
	KeyAgentNeedsInputSound  = "code-agent-needs-input-sound"
	KeyAgentFinishedNotify   = "code-agent-finished-notification"
	KeyAgentFinishedSound    = "code-agent-finished-sound"
)

// supportedKeys is every config key Code acts on. A key outside it is ignored
// and shown in the Settings window's problems, so a Ghostty option Code does
// not honour is not silently dropped.
var supportedKeys = map[string]bool{
	KeyFontFamily: true, KeyFontSize: true, KeyFontThicken: true, KeyFontFeature: true,
	KeyAdjustCellWidth: true, KeyAdjustCellHeight: true,
	KeyTheme:      true,
	KeyBackground: true, KeyForeground: true,
	KeyCursorColor: true, KeyCursorText: true,
	KeySelectionBackground: true, KeySelectionForeground: true,
	KeyPalette:          true,
	KeySearchBackground: true, KeySearchForeground: true,
	KeySearchSelBackground: true, KeySearchSelForeground: true,
	KeyCursorStyle: true, KeyCursorBlink: true,
	KeyBackgroundOpacity: true, KeyUnfocusedSplitOpacity: true,
	KeyScrollback: true, KeyScrollMultiplier: true,
	KeyCommand: true, "keybind": true,
	KeyCopyOnSelect: true, KeyRightClickAction: true,
	KeyClipboardWrite: true, KeyClipboardRead: true,
	KeyMinimumContrast: true, KeyImageLimit: true,
	KeyShellIntegration: true, KeyConfirmClose: true,
	KeyWorkingDirectory: true, KeyOptionAsAlt: true,
	KeyWindowPaddingX: true, KeyWindowPaddingY: true, KeyWindowPaddingColor: true,
	KeyAgentNeedsInputNotify: true, KeyAgentFinishedNotify: true,
}

// Themes are the terminal colors for light and dark windows.
type Themes struct {
	Light *terminal.Theme
	Dark  *terminal.Theme
}

// ResolveThemes reads the theme option, which takes Ghostty's
// `light:Name,dark:Name` form, a bare name for both, or a theme file in Code's
// themes directory.
func ResolveThemes(set *Settings, themesDir string) Themes {
	value := set.String(KeyTheme, "")
	lightName, darkName := parseThemeChoice(value)
	return Themes{
		Light: resolveTheme(lightName, themesDir, false),
		Dark:  resolveTheme(darkName, themesDir, true),
	}
}

// parseThemeChoice splits `light:…,dark:…` into a light and a dark name. A
// bare name applies to both.
func parseThemeChoice(value string) (light, dark string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}
	parts := strings.Split(value, ",")
	bare := ""
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "light:"):
			light = strings.TrimSpace(strings.TrimPrefix(part, "light:"))
		case strings.HasPrefix(part, "dark:"):
			dark = strings.TrimSpace(strings.TrimPrefix(part, "dark:"))
		default:
			bare = part
		}
	}
	if bare != "" {
		if light == "" {
			light = bare
		}
		if dark == "" {
			dark = bare
		}
	}
	return light, dark
}

// ThemeNames returns the names of the themes Code ships, sorted. The embedded
// set never changes, so it is listed once.
func ThemeNames() []string {
	themeNamesOnce.Do(func() {
		entries, err := bundledThemes.ReadDir("themes")
		if err != nil {
			return
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			if !entry.IsDir() {
				names = append(names, entry.Name())
			}
		}
		sort.Strings(names)
		themeNames = names
	})
	return themeNames
}

var (
	themeNamesOnce sync.Once
	themeNames     []string
)

// SetPrimaryFontFamily replaces the first font family while keeping the
// fallbacks after it.
func (s *Settings) SetPrimaryFontFamily(name string) {
	values := s.Strings(KeyFontFamily)
	if len(values) == 0 {
		if name != "" {
			s.SetValues(KeyFontFamily, []string{name})
		}
		return
	}
	values[0] = name
	s.SetValues(KeyFontFamily, values)
}

// ThemeName returns the light theme's name, or the default when none is set.
func ThemeName(value string) string {
	light, _ := parseThemeChoice(value)
	if light == "" {
		return DefaultTheme
	}
	return light
}

// resolveTheme finds a theme by name: a file in Code's themes directory, an
// absolute path, one of the themes Code ships, or one of Ghostty's bundled
// themes. The fallback is the window appearance's built-in theme.
func resolveTheme(name, themesDir string, dark bool) *terminal.Theme {
	fallback := func() *terminal.Theme {
		if dark {
			return terminal.DarkTheme()
		}
		return terminal.LightTheme()
	}
	if name == "" {
		name = DefaultTheme
	}
	for _, path := range themeCandidates(name, themesDir) {
		if data, err := os.ReadFile(path); err == nil {
			if theme, err := terminal.ParseGhosttyTheme(data); err == nil {
				return theme
			}
		}
	}
	if data := bundledTheme(name); data != nil {
		if theme, err := terminal.ParseGhosttyTheme(data); err == nil {
			return theme
		}
	}
	if theme, err := terminal.GhosttyTheme(name); err == nil {
		return theme
	}
	return fallback()
}

// themeCandidates lists the files a theme name may name, in lookup order.
func themeCandidates(name, themesDir string) []string {
	var out []string
	if filepath.IsAbs(name) {
		out = append(out, name)
	}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(name, "~/") {
		out = append(out, filepath.Join(home, name[2:]))
	}
	if themesDir != "" {
		out = append(out, filepath.Join(themesDir, name))
	}
	return out
}

// bundledTheme returns the bytes of a theme Code ships, matched by base name
// without regard to case, or nil.
func bundledTheme(name string) []byte {
	entries, err := bundledThemes.ReadDir("themes")
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), name) {
			data, err := bundledThemes.ReadFile("themes/" + entry.Name())
			if err == nil {
				return data
			}
		}
	}
	return nil
}

// Accent is a workspace's color in the sidebar. The index is kept across
// relaunches so the same project keeps its color.
func Accent(index int) ui.Color {
	count := len(accents)
	return accents[((index%count)+count)%count]
}

// AccentCount is how many colors the palette holds.
const AccentCount = 8

var accents = [AccentCount]ui.Color{
	ui.RGB(0x0A, 0x84, 0xFF), // blue
	ui.RGB(0xBF, 0x5A, 0xF2), // purple
	ui.RGB(0xFF, 0x2D, 0x55), // pink
	ui.RGB(0xFF, 0x3B, 0x30), // red
	ui.RGB(0xFF, 0x95, 0x00), // orange
	ui.RGB(0xFF, 0xCC, 0x00), // yellow
	ui.RGB(0x28, 0xCD, 0x41), // green
	ui.RGB(0x40, 0xC8, 0xBE), // teal
}

// TerminalConfig is everything a terminal takes from the config file.
type TerminalConfig struct {
	Font         terminal.Font
	Light        *terminal.Theme
	Dark         *terminal.Theme
	Cursor       terminal.CursorStyle
	NoBlink      bool
	Command      string
	Opacity      float64
	Unfocused    float64
	Scrollback   int
	CopyOnSelect terminal.CopyOnSelect
	Padding      terminal.Padding
	PaddingColor terminal.PaddingColor
	OptionAsAlt  bool
	// ScrollMultiplier scales wheel and touchpad scrolling.
	ScrollMultiplier terminal.ScrollMultiplier
	// RightClick is what a secondary click does.
	RightClick terminal.RightClickAction
	// NoClipboardWrite keeps programs from writing the clipboard.
	NoClipboardWrite bool
	// ClipboardRead is what programs get when they read the clipboard.
	ClipboardRead terminal.ClipboardAccess
	// Transparent leaves the terminal's background undrawn, so a background
	// opacity below 1 shows what is behind the window.
	Transparent bool
	// MinimumContrast is the least WCAG contrast text has with its
	// background, 1 for none.
	MinimumContrast float32
	// ImageLimit is the most bytes images programs show may take.
	ImageLimit int64
}

// Terminal reads the terminal options out of the settings.
func (s *Settings) Terminal(themesDir string) TerminalConfig {
	themes := ResolveThemes(s, themesDir)
	applyColorOverrides(themes.Light, s)
	applyColorOverrides(themes.Dark, s)
	font := terminal.Font{
		Family:     strings.Join(activeFamilies(s.Strings(KeyFontFamily)), ", "),
		Size:       float32(s.Float(KeyFontSize, 13)),
		Thicken:    s.Bool(KeyFontThicken, false),
		Features:   s.Strings(KeyFontFeature),
		CellWidth:  adjustRatio(s.String(KeyAdjustCellWidth, "")),
		LineHeight: adjustRatio(s.String(KeyAdjustCellHeight, "")),
	}
	opacity := clamp(s.Float(KeyBackgroundOpacity, 1), 0.1, 1)
	return TerminalConfig{
		Font:             font,
		Light:            themes.Light,
		Dark:             themes.Dark,
		Cursor:           cursorStyle(s.String(KeyCursorStyle, "block")),
		NoBlink:          !s.Bool(KeyCursorBlink, true),
		Command:          s.String(KeyCommand, ""),
		Opacity:          opacity,
		Unfocused:        clamp(s.Float(KeyUnfocusedSplitOpacity, 0.7), 0.1, 1),
		Scrollback:       s.Int(KeyScrollback, 10_000_000),
		CopyOnSelect:     copyOnSelect(s.String(KeyCopyOnSelect, "none")),
		ScrollMultiplier: scrollMultiplier(s.String(KeyScrollMultiplier, "")),
		RightClick:       rightClickAction(s.String(KeyRightClickAction, "")),
		NoClipboardWrite: strings.TrimSpace(s.String(KeyClipboardWrite, "allow")) == "deny",
		ClipboardRead:    clipboardRead(s.String(KeyClipboardRead, "ask")),
		Transparent:      opacity < 1,
		MinimumContrast:  float32(s.Float(KeyMinimumContrast, 1)),
		ImageLimit:       int64(s.Int(KeyImageLimit, 0)),
		Padding: terminal.Padding{
			X: float32(s.Float(KeyWindowPaddingX, 6)),
			Y: float32(s.Float(KeyWindowPaddingY, 4)),
		},
		PaddingColor: paddingColor(s.String(KeyWindowPaddingColor, "")),
		OptionAsAlt:  s.Bool(KeyOptionAsAlt, false),
	}
}

// applyColorOverrides applies Ghostty's single-color options over a resolved
// theme, so a config can change the background, the cursor or a palette entry
// without naming a theme of its own.
func applyColorOverrides(theme *terminal.Theme, s *Settings) {
	if theme == nil {
		return
	}
	set := func(key string, dst *ui.Color) {
		if c, ok := parseColor(s.String(key, "")); ok {
			*dst = c
		}
	}
	set(KeyForeground, &theme.Foreground)
	set(KeyBackground, &theme.Background)
	set(KeyCursorColor, &theme.Cursor)
	set(KeyCursorText, &theme.CursorText)
	set(KeySelectionBackground, &theme.Selection)
	set(KeySelectionForeground, &theme.SelectionText)
	set(KeySearchBackground, &theme.Search)
	set(KeySearchForeground, &theme.SearchText)
	set(KeySearchSelBackground, &theme.SearchSelected)
	set(KeySearchSelForeground, &theme.SearchSelectedText)
	for _, value := range s.Strings(KeyPalette) {
		if index, color, ok := parsePalette(value); ok {
			theme.Palette[index] = color
		}
	}
}

// activeFamilies returns the font families in effect, honouring an empty value
// that clears the families before it, as Ghostty does.
func activeFamilies(values []string) []string {
	reset := 0
	for i, value := range values {
		if value == "" {
			reset = i + 1
		}
	}
	return values[reset:]
}

// adjustRatio reads Ghostty's adjust-cell-width and adjust-cell-height, which
// are a percentage like "20%" or a ratio like "1.2".
func adjustRatio(value string) float32 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if strings.HasSuffix(value, "%") {
		if f, err := parseFloat(strings.TrimSuffix(value, "%")); err == nil {
			return float32(1 + f/100)
		}
		return 0
	}
	f, err := parseFloat(value)
	if err != nil || f <= 0 {
		return 0
	}
	if f < 3 {
		return float32(f)
	}
	return float32(1 + f/100)
}

func paddingColor(value string) terminal.PaddingColor {
	switch value {
	case "extend":
		return terminal.PaddingExtend
	case "extend-always":
		return terminal.PaddingExtendAlways
	default:
		return terminal.PaddingBackground
	}
}

func cursorStyle(value string) terminal.CursorStyle {
	switch value {
	case "bar":
		return terminal.CursorBar
	case "underline":
		return terminal.CursorUnderline
	default:
		return terminal.CursorBlock
	}
}

func copyOnSelect(value string) terminal.CopyOnSelect {
	switch value {
	case "true", "clipboard", "both":
		return terminal.CopyOnSelectClipboard
	default:
		return terminal.CopyOnSelectNone
	}
}

// scrollMultiplier reads mouse-scroll-multiplier, either one number for both
// the touchpad and the wheel or `precision:N,discrete:M`.
func scrollMultiplier(value string) terminal.ScrollMultiplier {
	value = strings.TrimSpace(value)
	if value == "" {
		return terminal.ScrollMultiplier{}
	}
	if f, err := parseFloat(value); err == nil {
		return terminal.ScrollMultiplier{Precision: float32(f), Discrete: float32(f)}
	}
	var m terminal.ScrollMultiplier
	for _, part := range strings.Split(value, ",") {
		name, raw, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok {
			continue
		}
		f, err := parseFloat(raw)
		if err != nil {
			continue
		}
		switch strings.TrimSpace(name) {
		case "precision":
			m.Precision = float32(f)
		case "discrete":
			m.Discrete = float32(f)
		}
	}
	return m
}

func rightClickAction(value string) terminal.RightClickAction {
	switch strings.TrimSpace(value) {
	case "paste":
		return terminal.RightClickPaste
	case "copy":
		return terminal.RightClickCopy
	case "copy-or-paste":
		return terminal.RightClickCopyOrPaste
	case "ignore":
		return terminal.RightClickIgnore
	default:
		return terminal.RightClickContextMenu
	}
}

func clipboardRead(value string) terminal.ClipboardAccess {
	switch strings.TrimSpace(value) {
	case "allow":
		return terminal.ClipboardAllow
	case "deny":
		return terminal.ClipboardDeny
	case "forward":
		return terminal.ClipboardForward
	default:
		return terminal.ClipboardAsk
	}
}

// parseColor reads a color as Ghostty writes one: `#rgb`, `#rgba`, `#rrggbb`
// or `#rrggbbaa`, with the `#` optional.
func parseColor(value string) (ui.Color, bool) {
	h := strings.TrimPrefix(strings.TrimSpace(value), "#")
	switch len(h) {
	case 3:
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	case 4:
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2], h[3], h[3]})
	case 6, 8:
	default:
		return ui.Color{}, false
	}
	n, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return ui.Color{}, false
	}
	if len(h) == 8 {
		return ui.RGBA(uint8(n>>24), uint8(n>>16), uint8(n>>8), float32(uint8(n))/255), true
	}
	return ui.RGB(uint8(n>>16), uint8(n>>8), uint8(n)), true
}

// parsePalette reads one `palette` line, `INDEX=COLOR` for the 16 program
// colors.
func parsePalette(value string) (int, ui.Color, bool) {
	index, raw, ok := strings.Cut(strings.TrimSpace(value), "=")
	if !ok {
		return 0, ui.Color{}, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(index))
	if err != nil || n < 0 || n > 15 {
		return 0, ui.Color{}, false
	}
	color, ok := parseColor(raw)
	return n, color, ok
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func parseFloat(value string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(value), 64)
}
