package theme

import (
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// Terminal derives an editor theme from a terminal theme, so the notes in a
// Code window wear the same colors as the terminal beside them: the page and
// its text from the theme's background and foreground, the syntax from its
// 16-color palette.
func Terminal(dark bool, t *terminal.Theme) Theme {
	if t == nil {
		return System(dark, ui.RGB(0x0A, 0x84, 0xFF))
	}
	bg, fg := t.Background, t.Foreground
	p := t.Palette
	// A palette entry a theme leaves out falls back to a mix of the page.
	or := func(i int, fallback uint32) ui.Color {
		if p[i].A != 0 {
			return p[i]
		}
		return hex(fallback)
	}
	accent := or(12, 0x4C9AFF)
	if p[4].A != 0 {
		accent = p[4]
	}
	selection := t.Selection
	if selection.A == 0 {
		selection = accent.Alpha(0.3)
	}
	th := Theme{
		ID: "terminal", Name: "Terminal", Dark: dark,
		Background: bg, Text: fg,
		Secondary: mix(bg, fg, 0.6), Syntax: mix(bg, fg, 0.35),
		Accent: accent, Heading: fg, Link: accent,
		CodeText: or(1, 0xE06C75), CodeBackground: mix(bg, fg, 0.06),
		QuoteBar: mix(bg, fg, 0.2), Rule: mix(bg, fg, 0.16),
		Highlight: or(3, 0xE5C07B).Alpha(0.4),
		Keyword:   or(5, 0xC678DD), String: or(2, 0x98C379), Comment: or(8, 0x5C6370),
		Number: or(3, 0xE5C07B), Type: or(6, 0x56B6C2), Function: accent,
		Constant: or(3, 0xE5C07B), Property: or(1, 0xE06C75),
		Inserted: or(2, 0x98C379), Deleted: or(1, 0xE06C75),
		Selection: selection,
		Panel:     mix(bg, fg, 0.045),
		Border:    mix(bg, fg, 0.1),
	}
	th.Callouts = [5]ui.Color{accent, or(2, 0x98C379), or(5, 0xC678DD), or(3, 0xE5C07B), or(1, 0xE06C75)}
	return th
}
