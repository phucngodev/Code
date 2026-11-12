package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/config"
)

// settingsWindow is the Settings window's state: which tab is open.
type settingsWindow struct {
	tab int
}

// settingsTabs names and icons the tabs.
var settingsTabs = []struct {
	name string
	icon *ui.SVG
}{
	{"General", gearIcon},
	{"Appearance", paletteIcon},
	{"Notifications", bellIcon},
	{"Advanced", gearsIcon},
}

// ShowSettings opens the Settings window, building it the first time.
func (a *App) ShowSettings() {
	if a.settingsState == nil {
		a.settingsState = &settingsWindow{}
	}
	if a.settingsWin == nil {
		a.settingsWin = mygo.NewWindow(mygo.WindowOptions{
			Title:     "Settings",
			Width:     620,
			Height:    640,
			MinWidth:  460,
			MinHeight: 360,
			Content:   ui.View(a.settingsView),
		})
		a.settingsWin.OnClosed(func() { a.settingsWin = nil })
		a.applyChromeToWindow(a.settingsWin)
	}
	a.settingsWin.Show()
	a.settingsWin.Focus()
}

func (a *App) settingsView(c *ui.Context) {
	c.SetTheme(a.currentChrome(c.Theme().Dark))
	t := c.Theme()
	if a.settingsState.tab < 0 || a.settingsState.tab >= len(settingsTabs) {
		a.settingsState.tab = 0
	}
	if a.settingsWin != nil {
		a.settingsWin.SetTitle(settingsTabs[a.settingsState.tab].name)
	}
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		a.settingsTabBar(c)
		ui.Divider(c)
		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(16, 20).Gap(18).Children(func() {
				switch a.settingsState.tab {
				case 0:
					a.settingsGeneral(c, a.settings)
				case 1:
					a.settingsAppearance(c, a.settings)
				case 2:
					a.settingsNotifications(c, a.settings)
				default:
					a.settingsAdvanced(c, a.settings)
				}
			})
		})
	})
}

// settingsTabBar is the row of icon tabs, centered under the title.
func (a *App) settingsTabBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Padding(8).Justify(ui.Center).Gap(6).Children(func() {
		for i, tab := range settingsTabs {
			i, tab := i, tab
			selected := a.settingsState.tab == i
			item := ui.Column(c).Padding(6, 16).Gap(3).Radius(8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			if selected {
				item.Background(t.SurfaceHover)
			} else if item.Hovered() {
				item.Background(t.Surface)
			}
			item.Children(func() {
				color := t.TextMuted
				if selected {
					color = t.Accent
				}
				ui.Icon(c, tab.icon).FontSize(22).TextColor(color)
				ui.Text(c, tab.name).FontSize(11).TextColor(color)
			})
			item.OnClick(func() { a.settingsState.tab = i })
		}
	})
}

// General -------------------------------------------------------------

func (a *App) settingsGeneral(c *ui.Context, s *config.Settings) {
	ui.Column(c).FillWidth().Gap(18).Children(func() {
		settingsCard(c, func() {
			settingsRow(c, "Shell", func(caption ui.Element) {
				command := s.String(config.KeyCommand, "")
				value := command
				if value == "" {
					value = "Login shell"
				}
				options := []string{"Login shell", "/bin/zsh", "/bin/bash", "/bin/sh", "/usr/bin/fish"}
				if !containsString(options, value) {
					options = append([]string{value}, options...)
				}
				if ui.Select(c, &value, options).Width(240).LabelledBy(caption).Changed() {
					if value == "Login shell" {
						s.Set(config.KeyCommand, "")
					} else {
						s.Set(config.KeyCommand, value)
					}
				}
			})
		})
		settingsSection(c, "Terminals", func() {
			settingsCard(c, func() {
				settingsRow(c, "Confirm closing terminals", func(caption ui.Element) {
					settingsChoiceSelect(c, caption, s.String(config.KeyConfirmClose, "true"), []settingsChoice{
						{"Never", "false"},
						{"With a running process", "true"},
						{"Always", "always"},
					}, func(value string) { s.Set(config.KeyConfirmClose, value) })
				})
				ui.Divider(c)
				settingsRow(c, "Copy on select", func(caption ui.Element) {
					on := copyOnSelectEnabled(s)
					if ui.Switch(c, &on).LabelledBy(caption).Changed() {
						s.Set(config.KeyCopyOnSelect, boolString(on))
					}
				})
				ui.Divider(c)
				settingsRow(c, "Hide mouse pointer while typing", func(caption ui.Element) {
					on := s.Bool(config.KeyMouseHideWhileTyping, false)
					if ui.Switch(c, &on).LabelledBy(caption).Changed() {
						s.Set(config.KeyMouseHideWhileTyping, boolString(on))
					}
				})
				settingsNote(c, "Not active yet: the value is saved but Code does not hide the pointer.")
			})
		})
		settingsCard(c, func() {
			settingsRow(c, "Secure keyboard entry", func(caption ui.Element) {
				settingsChoiceSelect(c, caption, s.String(config.KeyAutoSecureInput, "true"), []settingsChoice{
					{"Off", "false"},
					{"At password prompts", "true"},
					{"On", "always"},
				}, func(value string) { s.Set(config.KeyAutoSecureInput, value) })
			})
			settingsNote(c, "Not active yet: the value is saved but secure keyboard entry is not enforced.")
		})
		settingsSection(c, "Scrollback", func() {
			settingsCard(c, func() {
				settingsRow(c, "Scrollback limit (bytes)", func(caption ui.Element) {
					value := float64(s.Int(config.KeyScrollback, 10_000_000))
					if ui.NumberInput(c, &value, 0, 1_000_000_000, 1_000_000).Width(180).LabelledBy(caption).Changed() {
						s.Set(config.KeyScrollback, fmt.Sprintf("%d", int(value)))
					}
				})
				ui.Divider(c)
				settingsRow(c, "Shell integration", func(caption ui.Element) {
					on := s.Bool(config.KeyShellIntegration, true)
					if ui.Switch(c, &on).LabelledBy(caption).Changed() {
						s.Set(config.KeyShellIntegration, boolString(on))
					}
				})
				settingsNote(c, "Marks zsh prompts so Code can clear and redraw them on resize.")
			})
		})
	})
}

// Appearance ----------------------------------------------------------

func (a *App) settingsAppearance(c *ui.Context, s *config.Settings) {
	ui.Column(c).FillWidth().Gap(18).Children(func() {
		settingsSection(c, "Colors", func() {
			settingsCard(c, func() {
				settingsRow(c, "Theme", func(caption ui.Element) {
					names := config.ThemeNames()
					value := config.ThemeName(s.String(config.KeyTheme, ""))
					if !containsString(names, value) {
						names = append([]string{value}, names...)
					}
					if ui.Select(c, &value, names).Width(240).LabelledBy(caption).Changed() {
						s.Set(config.KeyTheme, value)
					}
				})
				ui.Divider(c)
				a.settingsThemePreview(c)
			})
		})
		settingsSection(c, "Text", func() {
			settingsCard(c, func() {
				settingsRow(c, "Font", func(caption ui.Element) {
					families := monospacedFamilies()
					current := primaryFamily(s)
					choices := []settingsChoice{{name: "Default", value: ""}}
					// Keep a family Code does not offer, so a hand-set font still
					// shows and is not silently replaced by the default.
					if current != "" && !containsString(families, current) {
						choices = append(choices, settingsChoice{name: current, value: current})
					}
					for _, family := range families {
						choices = append(choices, settingsChoice{name: family, value: family})
					}
					settingsChoiceSelect(c, caption, current, choices, func(value string) {
						s.SetPrimaryFontFamily(value)
					})
				})
				ui.Divider(c)
				settingsRow(c, "Size", func(caption ui.Element) {
					value := s.Float(config.KeyFontSize, 13)
					if ui.NumberInput(c, &value, 6, 40, 1).Width(120).LabelledBy(caption).Changed() {
						s.Set(config.KeyFontSize, fmt.Sprintf("%g", value))
					}
				})
				ui.Divider(c)
				settingsRow(c, "Thicken text", func(caption ui.Element) {
					on := s.Bool(config.KeyFontThicken, false)
					if ui.Switch(c, &on).LabelledBy(caption).Changed() {
						s.Set(config.KeyFontThicken, boolString(on))
					}
				})
			})
		})
		settingsSection(c, "Cursor", func() {
			settingsCard(c, func() {
				settingsRow(c, "Shape", func(caption ui.Element) {
					value := s.String(config.KeyCursorStyle, "block")
					options := []string{"block", "bar", "underline", "block_hollow"}
					if ui.Select(c, &value, options).Width(200).LabelledBy(caption).Changed() {
						s.Set(config.KeyCursorStyle, value)
					}
				})
				ui.Divider(c)
				settingsRow(c, "Blink", func(caption ui.Element) {
					on := s.Bool(config.KeyCursorBlink, true)
					if ui.Switch(c, &on).LabelledBy(caption).Changed() {
						s.Set(config.KeyCursorBlink, boolString(on))
					}
				})
			})
		})
		settingsSection(c, "Window", func() {
			settingsCard(c, func() {
				a.settingsSliderRow(c, s, "Background opacity", config.KeyBackgroundOpacity, 1, 0.5, 1)
				ui.Divider(c)
				a.settingsSliderRow(c, s, "Unfocused pane opacity", config.KeyUnfocusedSplitOpacity, 0.7, 0.3, 1)
			})
		})
	})
}

// settingsThemePreview shows a sample prompt in the chosen theme's colors.
// It reads the app's cached terminal theme, not the files, since the view
// builds every frame.
func (a *App) settingsThemePreview(c *ui.Context) {
	theme := a.terminalCfg.Light
	if theme == nil {
		theme = terminal.LightTheme()
	}
	ui.Column(c).FillWidth().Padding(8, 12, 12, 12).Children(func() {
		box := ui.Column(c).FillWidth().Padding(12).Gap(5).Radius(8).Background(theme.Background)
		box.Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				ui.Text(c, "~/code").TextColor(theme.Palette[4])
				ui.Text(c, "main").TextColor(theme.Palette[5])
				ui.Text(c, "$ ls").TextColor(theme.Foreground)
			})
			ui.Row(c).Gap(12).Children(func() {
				ui.Text(c, "Sources").TextColor(theme.Palette[4])
				ui.Text(c, "README.md").TextColor(theme.Foreground)
				ui.Text(c, "build.sh").TextColor(theme.Palette[2])
			})
			ui.Row(c).Gap(12).Children(func() {
				ui.Text(c, "✓ 12 passed").TextColor(theme.Palette[2])
				ui.Text(c, "✗ 1 failed").TextColor(theme.Palette[1])
			})
			ui.Row(c).Gap(4).Padding(2, 0, 0, 0).Children(func() {
				for _, color := range theme.Palette {
					ui.Box(c).Size(16, 10).Radius(2).Background(color)
				}
			})
		})
	})
}

// Notifications -------------------------------------------------------

func (a *App) settingsNotifications(c *ui.Context, s *config.Settings) {
	sounds := soundNames()
	ui.Column(c).FillWidth().Gap(18).Children(func() {
		settingsSection(c, "Agent needs input", func() {
			settingsCard(c, func() {
				settingsRow(c, "Notification", func(caption ui.Element) {
					on := s.Bool(config.KeyAgentNeedsInputNotify, true)
					if ui.Switch(c, &on).LabelledBy(caption).Changed() {
						s.Set(config.KeyAgentNeedsInputNotify, boolString(on))
					}
				})
				ui.Divider(c)
				settingsRow(c, "Sound", func(caption ui.Element) {
					settingsSoundSelect(c, caption, s, config.KeyAgentNeedsInputSound, "Ping", sounds)
				})
			})
		})
		settingsSection(c, "Agent finished", func() {
			settingsCard(c, func() {
				settingsRow(c, "Notification", func(caption ui.Element) {
					on := s.Bool(config.KeyAgentFinishedNotify, true)
					if ui.Switch(c, &on).LabelledBy(caption).Changed() {
						s.Set(config.KeyAgentFinishedNotify, boolString(on))
					}
				})
				ui.Divider(c)
				settingsRow(c, "Sound", func(caption ui.Element) {
					settingsSoundSelect(c, caption, s, config.KeyAgentFinishedSound, "Glass", sounds)
				})
			})
			settingsFootnote(c, "Agent alerts are only for terminals that are not visible.")
			settingsFootnote(c, "Sound names are saved but no sound is played yet.")
		})
		settingsSection(c, "Command finished", func() {
			settingsCard(c, func() {
				settingsRow(c, "Notify", func(caption ui.Element) {
					settingsChoiceSelect(c, caption, s.String(config.KeyNotifyOnFinish, "never"), []settingsChoice{
						{"Never", "never"},
						{"When the terminal is not visible", "unfocused"},
						{"Always", "always"},
					}, func(value string) { s.Set(config.KeyNotifyOnFinish, value) })
				})
				ui.Divider(c)
				settingsRow(c, "Minimum duration (seconds)", func(caption ui.Element) {
					value := float64(s.Int(config.KeyNotifyOnFinishAfter, 5))
					if ui.NumberInput(c, &value, 0, 600, 1).Width(140).LabelledBy(caption).Changed() {
						s.Set(config.KeyNotifyOnFinishAfter, fmt.Sprintf("%ds", int(value)))
					}
				})
				ui.Divider(c)
				settingsRow(c, "Notification", func(caption ui.Element) {
					on := notifyActionOn(s, "notify")
					if ui.Switch(c, &on).LabelledBy(caption).Changed() {
						setNotifyAction(s, "notify", on)
					}
				})
				ui.Divider(c)
				settingsRow(c, "Bell", func(caption ui.Element) {
					on := notifyActionOn(s, "bell")
					if ui.Switch(c, &on).LabelledBy(caption).Changed() {
						setNotifyAction(s, "bell", on)
					}
				})
			})
			settingsNote(c, "Not active yet: Code cannot tell when a command finishes, so these values are saved but no alert is raised.")
		})
	})
}

// Advanced ------------------------------------------------------------

func (a *App) settingsAdvanced(c *ui.Context, s *config.Settings) {
	t := c.Theme()
	problems := a.settingsProblems()
	ui.Column(c).FillWidth().Gap(18).Children(func() {
		settingsCard(c, func() {
			ui.Row(c).FillWidth().Padding(10, 12).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "Config file")
					ui.Text(c, s.Path()).FontSize(11).TextColor(t.TextMuted)
					ui.Text(c, fmt.Sprintf("%d keybindings loaded", len(a.keybinds))).FontSize(11).TextColor(t.TextMuted)
				})
				if ui.Button(c, "Show in Finder").Clicked() {
					mygo.Shell.ShowItemInFolder(s.Path())
				}
				if ui.Button(c, "Open in Editor").Clicked() {
					_ = mygo.Shell.OpenPath(s.Path())
				}
			})
		})
		settingsFootnote(c, "Settings in this window are saved to this file. Lines added by hand and comments are preserved; options Code does not read are listed under Problems.")
		settingsSection(c, "Problems", func() {
			settingsCard(c, func() {
				ui.Row(c).FillWidth().Padding(10, 12).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						label := "No problems"
						if len(problems) == 1 {
							label = "1 problem"
						} else if len(problems) > 1 {
							label = fmt.Sprintf("%d problems", len(problems))
						}
						ui.Text(c, label)
						ui.Text(c, "Reload after editing the file (⇧⌘,).").FontSize(11).TextColor(t.TextMuted)
					})
					if ui.Button(c, "Reload Configuration").Clicked() {
						a.ReloadConfig()
					}
				})
				for _, problem := range problems {
					ui.Divider(c)
					ui.Row(c).FillWidth().Padding(8, 12).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "⚠").TextColor(t.Warning)
						ui.Text(c, problem).FontSize(12).TextColor(t.TextMuted)
					})
				}
			})
		})
	})
}

// helpers -------------------------------------------------------------

// settingsProblems lists what could not be read from the config file and
// what Code does not honour, so an option Code ignores is not silent.
func (a *App) settingsProblems() []string {
	if err := a.settings.ReadError(); err != nil {
		return []string{err.Error()}
	}
	var problems []string
	for _, key := range a.settings.UnsupportedKeys() {
		problems = append(problems, fmt.Sprintf("Option %q is not supported and is ignored.", key))
	}
	return problems
}

// settingsSliderRow is a labeled row holding a slider and its percentage.
func (a *App) settingsSliderRow(c *ui.Context, s *config.Settings, label, key string, def, lo, hi float64) {
	settingsRow(c, label, func(caption ui.Element) {
		value := s.Float(key, def)
		ui.Row(c).Width(240).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Slider(c, &value, lo, hi).LabelledBy(caption).Grow(1)
			ui.Text(c, fmt.Sprintf("%.0f%%", value*100)).FontSize(12).TextColor(c.Theme().TextMuted).Width(44)
		})
		if value != s.Float(key, def) {
			s.Set(key, fmt.Sprintf("%g", value))
		}
	})
}

// settingsSoundSelect chooses an alert sound by name.
func settingsSoundSelect(c *ui.Context, caption ui.Element, s *config.Settings, key, def string, sounds []string) {
	value := s.String(key, def)
	if !containsString(sounds, value) {
		sounds = append([]string{value}, sounds...)
	}
	if ui.Select(c, &value, sounds).Width(200).LabelledBy(caption).Changed() {
		s.Set(key, value)
	}
}

func notifyActionOn(s *config.Settings, action string) bool {
	for _, part := range strings.Split(s.String(config.KeyNotifyOnFinishAction, "bell"), ",") {
		if strings.TrimSpace(part) == action {
			return true
		}
	}
	return false
}

func setNotifyAction(s *config.Settings, action string, on bool) {
	var actions []string
	for _, part := range strings.Split(s.String(config.KeyNotifyOnFinishAction, "bell"), ",") {
		part = strings.TrimSpace(part)
		if part != "" && part != action {
			actions = append(actions, part)
		}
	}
	if on {
		actions = append(actions, action)
	}
	if len(actions) == 0 {
		s.Set(config.KeyNotifyOnFinishAction, "none")
		return
	}
	s.Set(config.KeyNotifyOnFinishAction, strings.Join(actions, ","))
}

func copyOnSelectEnabled(s *config.Settings) bool {
	switch s.String(config.KeyCopyOnSelect, "none") {
	case "true", "clipboard", "both":
		return true
	}
	return false
}

func primaryFamily(s *config.Settings) string {
	values := s.Strings(config.KeyFontFamily)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func boolString(on bool) string {
	if on {
		return "true"
	}
	return "false"
}

func containsString(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

// soundNames lists the alert sounds the system offers, or a default set. It is
// read once, since the view builds every frame.
var (
	soundNamesOnce sync.Once
	soundNamesList []string
)

func soundNames() []string {
	soundNamesOnce.Do(func() {
		fallback := []string{"Basso", "Blow", "Bottle", "Frog", "Funk", "Glass", "Hero", "Morse", "Ping", "Pop", "Purr", "Sosumi", "Submarine", "Tink"}
		entries, err := os.ReadDir("/System/Library/Sounds")
		if err != nil {
			soundNamesList = fallback
			return
		}
		var names []string
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			names = append(names, strings.TrimSuffix(name, filepath.Ext(name)))
		}
		if len(names) == 0 {
			names = fallback
		}
		soundNamesList = names
	})
	return soundNamesList
}

// monospacedFamilies lists the font families a terminal can use. It is read
// once, since asking the system for its fonts is slow and the view builds
// every frame.
var (
	monospacedFamiliesOnce sync.Once
	monospacedFamiliesList []string
)

func monospacedFamilies() []string {
	monospacedFamiliesOnce.Do(func() {
		for _, family := range ui.FontFamilies() {
			if family.Monospaced {
				monospacedFamiliesList = append(monospacedFamiliesList, family.Name)
			}
		}
	})
	return monospacedFamiliesList
}
