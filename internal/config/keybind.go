package config

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Keybind is one `keybind` line: a key with its modifiers bound to a Ghostty
// action. Action is the name before the first colon and Arg what follows, so
// `goto_split:left` is Action "goto_split" and Arg "left".
type Keybind struct {
	Mods        ui.Modifiers
	Key         ui.Key
	Action      string
	Arg         string
	Performable bool
}

// Keybinds returns the keybind lines the config sets, in order, and whether a
// `keybind = clear` reset the defaults before them. A line whose trigger or
// action the parser does not know is left out.
func (s *Settings) Keybinds() ([]Keybind, bool) {
	var out []Keybind
	cleared := false
	for _, value := range s.Strings("keybind") {
		value = strings.TrimSpace(value)
		switch {
		case value == "":
			continue
		case value == "clear":
			// Ghostty's clear drops every binding made before it, defaults
			// included, so the bindings after it stand alone.
			cleared = true
			out = out[:0]
			continue
		}
		if binding, ok := ParseKeybind(value); ok {
			out = append(out, binding)
		}
	}
	return out, cleared
}

// ParseKeybind parses a Ghostty keybind value: `trigger=action`, with an
// optional `performable:` prefix and an action argument after a colon.
func ParseKeybind(value string) (Keybind, bool) {
	value = strings.TrimSpace(value)
	performable := false
	if rest, ok := strings.CutPrefix(value, "performable:"); ok {
		performable = true
		value = rest
	}
	trigger, action, ok := strings.Cut(value, "=")
	if !ok {
		return Keybind{}, false
	}
	mods, key, ok := parseTrigger(trigger)
	if !ok {
		return Keybind{}, false
	}
	name, arg, _ := strings.Cut(action, ":")
	name = strings.TrimSpace(name)
	if name == "" {
		return Keybind{}, false
	}
	return Keybind{Mods: mods, Key: key, Action: name, Arg: strings.TrimSpace(arg), Performable: performable}, true
}

// parseTrigger reads the key and its modifiers from a trigger: the modifiers
// before the last `+`, and the key after it.
func parseTrigger(trigger string) (ui.Modifiers, ui.Key, bool) {
	parts := strings.Split(trigger, "+")
	if len(parts) == 0 {
		return 0, ui.KeyUnknown, false
	}
	var mods ui.Modifiers
	for _, part := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "cmd", "command", "super", "meta":
			mods |= ui.Super
		case "ctrl", "control":
			mods |= ui.Ctrl
		case "alt", "option", "opt":
			mods |= ui.Alt
		case "shift":
			mods |= ui.Shift
		default:
			return 0, ui.KeyUnknown, false
		}
	}
	key, ok := keyByName(parts[len(parts)-1])
	return mods, key, ok
}

// keyByName finds the key a Ghostty trigger names, without regard to case.
func keyByName(name string) (ui.Key, bool) {
	key, ok := keyNames[strings.ToLower(strings.TrimSpace(name))]
	return key, ok
}

var keyNames = func() map[string]ui.Key {
	names := map[string]ui.Key{
		"enter": ui.KeyEnter, "return": ui.KeyEnter,
		"escape": ui.KeyEscape, "esc": ui.KeyEscape,
		"backspace": ui.KeyBackspace,
		"tab":       ui.KeyTab,
		"space":     ui.KeySpace,
		"delete":    ui.KeyDelete, "del": ui.KeyDelete,
		"insert": ui.KeyInsert, "ins": ui.KeyInsert,
		"home":    ui.KeyHome,
		"end":     ui.KeyEnd,
		"page_up": ui.KeyPageUp, "pageup": ui.KeyPageUp, "pgup": ui.KeyPageUp,
		"page_down": ui.KeyPageDown, "pagedown": ui.KeyPageDown, "pgdn": ui.KeyPageDown,
		"up": ui.KeyUp, "down": ui.KeyDown, "left": ui.KeyLeft, "right": ui.KeyRight,
		"minus": ui.KeyMinus, "equal": ui.KeyEqual, "plus": ui.KeyEqual,
		"comma": ui.KeyComma, "period": ui.KeyPeriod, "slash": ui.KeySlash,
		"semicolon": ui.KeySemicolon,
		"quote":     ui.KeyQuote, "apostrophe": ui.KeyQuote,
		"backslash": ui.KeyBackslash,
		"backquote": ui.KeyBackquote, "grave": ui.KeyBackquote,
		"bracket_left": ui.KeyBracketLeft, "bracket_right": ui.KeyBracketRight,
	}
	letters := []ui.Key{
		ui.KeyA, ui.KeyB, ui.KeyC, ui.KeyD, ui.KeyE, ui.KeyF, ui.KeyG, ui.KeyH,
		ui.KeyI, ui.KeyJ, ui.KeyK, ui.KeyL, ui.KeyM, ui.KeyN, ui.KeyO, ui.KeyP,
		ui.KeyQ, ui.KeyR, ui.KeyS, ui.KeyT, ui.KeyU, ui.KeyV, ui.KeyW, ui.KeyX,
		ui.KeyY, ui.KeyZ,
	}
	for i, key := range letters {
		names[string(rune('a'+i))] = key
	}
	digits := []ui.Key{
		ui.Key0, ui.Key1, ui.Key2, ui.Key3, ui.Key4,
		ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9,
	}
	for i, key := range digits {
		names[string(rune('0'+i))] = key
	}
	fkeys := []ui.Key{
		ui.KeyF1, ui.KeyF2, ui.KeyF3, ui.KeyF4, ui.KeyF5, ui.KeyF6,
		ui.KeyF7, ui.KeyF8, ui.KeyF9, ui.KeyF10, ui.KeyF11, ui.KeyF12,
	}
	for i, key := range fkeys {
		names["f"+strconv.Itoa(i+1)] = key
	}
	return names
}()
