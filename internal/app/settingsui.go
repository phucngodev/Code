package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// settingsSection is a titled group of cards, as macOS System Settings shows.
func settingsSection(c *ui.Context, title string, fn func()) {
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, title).FontSize(12).Bold().TextColor(c.Theme().TextMuted).Padding(0, 2)
		fn()
	})
}

// settingsCard is a rounded panel holding rows, which the caller divides.
func settingsCard(c *ui.Context, fn func()) {
	card := ui.Column(c).FillWidth().Background(c.Theme().Surface).Radius(10).Clip()
	card.Children(fn)
}

// settingsRow places a label and its control on one line.
func settingsRow(c *ui.Context, label string, control func(caption ui.Element)) {
	ui.Row(c).FillWidth().MinHeight(42).Padding(9, 12).Gap(12).AlignItems(ui.Center).
		Role(ui.RoleGroup).Label(label).Children(func() {
		caption := ui.Text(c, label).Grow(1)
		control(caption)
	})
}

// settingsNote is a muted line of explanation inside a card.
func settingsNote(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(11).TextColor(c.Theme().TextMuted).Padding(6, 12, 9, 12)
}

// settingsFootnote is a muted line under a card.
func settingsFootnote(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(11).TextColor(c.Theme().TextMuted).Padding(0, 2)
}

// settingsChoice is one name a control offers, and the value it stands for.
type settingsChoice struct {
	name  string
	value string
}

// settingsChoiceSelect shows a select whose names map back to values, and
// reports a new choice by calling pick.
func settingsChoiceSelect(c *ui.Context, caption ui.Element, current string, choices []settingsChoice, pick func(string)) {
	names := make([]string, len(choices))
	selected := choices[0].name
	for i, choice := range choices {
		names[i] = choice.name
		if choice.value == current {
			selected = choice.name
		}
	}
	if ui.Select(c, &selected, names).Width(240).LabelledBy(caption).Changed() {
		for _, choice := range choices {
			if choice.name == selected {
				pick(choice.value)
				return
			}
		}
	}
}

// parseThemeParts splits a `light:…,dark:…` theme option into its names.
func parseThemeParts(value string) (light, dark string) {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "light:"):
			light = strings.TrimSpace(strings.TrimPrefix(part, "light:"))
		case strings.HasPrefix(part, "dark:"):
			dark = strings.TrimSpace(strings.TrimPrefix(part, "dark:"))
		case part != "":
			light, dark = part, part
		}
	}
	return light, dark
}

// joinThemeParts renders a light and a dark name as a theme option.
func joinThemeParts(light, dark string) string {
	if light == "" && dark == "" {
		return ""
	}
	if light == dark {
		return light
	}
	switch {
	case light == "":
		return "dark:" + dark
	case dark == "":
		return "light:" + light
	default:
		return "light:" + light + ",dark:" + dark
	}
}
