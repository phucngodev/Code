package config

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func TestDocumentKeepsCommentsAndOrder(t *testing.T) {
	doc := ParseDocument("# a comment\nfont-size = 13\n\n# another\nbackground-opacity = 0.9\n")
	if got := doc.Values("font-size"); len(got) != 1 || got[0] != "13" {
		t.Fatalf("font-size = %v, want [13]", got)
	}
	doc.Set("font-size", "14")
	text := doc.Text()
	if want := "# a comment\nfont-size = 14\n\n# another\nbackground-opacity = 0.9\n"; text != want {
		t.Fatalf("text =\n%q\nwant\n%q", text, want)
	}
}

func TestDocumentRemovesWhenEmpty(t *testing.T) {
	doc := ParseDocument("theme = Foo\nfont-size = 13\n")
	doc.Set("theme", "")
	if _, ok := doc.Get("theme"); ok {
		t.Fatal("theme should have been removed")
	}
	if got, _ := doc.Get("font-size"); got != "13" {
		t.Fatalf("font-size = %q, want 13", got)
	}
}

func TestDocumentBoolSpellings(t *testing.T) {
	for _, text := range []string{"1", "t", "true", "yes", "on"} {
		if b, ok := parseBool(text); !ok || !b {
			t.Fatalf("%q should parse as true", text)
		}
	}
	for _, text := range []string{"0", "f", "false", "no", "off"} {
		if b, ok := parseBool(text); !ok || b {
			t.Fatalf("%q should parse as false", text)
		}
	}
	if _, ok := parseBool("maybe"); ok {
		t.Fatal("maybe should not parse")
	}
}

func TestParseThemeChoice(t *testing.T) {
	cases := []struct {
		in          string
		light, dark string
	}{
		{"", "", ""},
		{"Catppuccin Mocha", "Catppuccin Mocha", "Catppuccin Mocha"},
		{"light:Catppuccin Latte,dark:Catppuccin Mocha", "Catppuccin Latte", "Catppuccin Mocha"},
		{"dark:Catppuccin Mocha", "", "Catppuccin Mocha"},
	}
	for _, tc := range cases {
		light, dark := parseThemeChoice(tc.in)
		if light != tc.light || dark != tc.dark {
			t.Fatalf("parseThemeChoice(%q) = %q, %q; want %q, %q", tc.in, light, dark, tc.light, tc.dark)
		}
	}
}

func TestAccentWraps(t *testing.T) {
	if Accent(0) != Accent(AccentCount) {
		t.Fatal("accent index should wrap")
	}
	if Accent(-1) != Accent(AccentCount-1) {
		t.Fatal("negative accent index should wrap")
	}
}

func TestDefaultThemeIsBundledModus(t *testing.T) {
	theme := resolveTheme("", t.TempDir(), false)
	// modus's background is #fafafa, which the built-in light theme is not.
	if theme.Background.R != 0xfa || theme.Background.G != 0xfa || theme.Background.B != 0xfa {
		t.Fatalf("default theme background = %+v, want #fafafa", theme.Background)
	}
}

func TestActiveFamilies(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{nil, nil},
		{[]string{"Mono"}, []string{"Mono"}},
		{[]string{"Mono", "Fallback"}, []string{"Mono", "Fallback"}},
		{[]string{"Old", "", "New"}, []string{"New"}},
	}
	for _, tc := range cases {
		got := activeFamilies(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("activeFamilies(%v) = %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("activeFamilies(%v)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestAdjustRatio(t *testing.T) {
	if got := adjustRatio("20%"); got != 1.2 {
		t.Fatalf("adjustRatio(20%%) = %v, want 1.2", got)
	}
	if got := adjustRatio("1.24"); got != 1.24 {
		t.Fatalf("adjustRatio(1.24) = %v, want 1.24", got)
	}
	if got := adjustRatio(""); got != 0 {
		t.Fatalf("adjustRatio(\"\") = %v, want 0", got)
	}
}

func TestDocumentSetNewKeyDoesNotDuplicate(t *testing.T) {
	doc := ParseDocument(Header)
	doc.Set("font-size", "14")
	doc.Set("theme", "modus")
	doc.Set("font-size", "16")
	text := doc.Text()
	if got := strings.Count(text, "# Code settings"); got != 1 {
		t.Fatalf("header appears %d times in:\n%s", got, text)
	}
	if got := doc.Values("font-size"); len(got) != 1 || got[0] != "16" {
		t.Fatalf("font-size = %v, want [16]", got)
	}
	if want := Header + "font-size = 16\ntheme = modus\n"; text != want {
		t.Fatalf("text =\n%q\nwant\n%q", text, want)
	}
}

func TestSettingsReloadPicksUpDiskChanges(t *testing.T) {
	path := t.TempDir() + "/config"
	s := LoadSettings(path)
	s.Set("font-size", "13")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("font-size = 18\ntheme = modus\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	if got := s.String("font-size", ""); got != "18" {
		t.Fatalf("font-size after reload = %q, want 18", got)
	}
}

func TestSettingsReloadRemembersReadError(t *testing.T) {
	path := t.TempDir() + "/config"
	s := LoadSettings(path)
	s.Set("font-size", "13")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	if s.ReadError() == nil {
		t.Fatal("reload should remember why the file could not be read")
	}
	if got := s.String("font-size", ""); got != "13" {
		t.Fatalf("document should be kept on a read error, font-size = %q", got)
	}
}

func TestSetPrimaryFontFamilyKeepsFallbacks(t *testing.T) {
	s := LoadSettings(t.TempDir() + "/config")
	s.SetValues(KeyFontFamily, []string{"Mono", "Symbols"})
	s.SetPrimaryFontFamily("Menlo")
	if got := s.Strings(KeyFontFamily); len(got) != 2 || got[0] != "Menlo" || got[1] != "Symbols" {
		t.Fatalf("font-family = %v, want [Menlo Symbols]", got)
	}
}

func TestSetPrimaryFontFamilyDefaultKeepsFallbacks(t *testing.T) {
	s := LoadSettings(t.TempDir() + "/config")
	s.SetValues(KeyFontFamily, []string{"Mono", "Symbols"})
	s.SetPrimaryFontFamily("")
	got := s.Strings(KeyFontFamily)
	if len(got) != 2 || got[0] != "" || got[1] != "Symbols" {
		t.Fatalf("font-family = %v, want [\"\" Symbols]", got)
	}
}

func TestParseKeybind(t *testing.T) {
	kb, ok := ParseKeybind("command+shift+h=goto_split:left")
	if !ok {
		t.Fatal("should parse")
	}
	if kb.Mods != ui.Super|ui.Shift || kb.Key != ui.KeyH || kb.Action != "goto_split" || kb.Arg != "left" {
		t.Fatalf("parsed %+v", kb)
	}
	resize, ok := ParseKeybind("alt+shift+k=resize_split:up,20")
	if !ok || resize.Action != "resize_split" || resize.Arg != "up,20" {
		t.Fatalf("parsed %+v ok=%v", resize, ok)
	}
	performable, ok := ParseKeybind("performable:cmd+c=copy_to_clipboard")
	if !ok || !performable.Performable {
		t.Fatalf("performable = %+v ok=%v", performable, ok)
	}
	for _, bad := range []string{"", "no-equals", "cmd+unknown=x", "cmd+a="} {
		if _, ok := ParseKeybind(bad); ok {
			t.Fatalf("%q should not parse", bad)
		}
	}
}

func TestKeybindsClear(t *testing.T) {
	s := LoadSettings(t.TempDir() + "/config")
	s.SetValues("keybind", []string{"cmd+a=new_tab", "clear", "cmd+b=new_tab"})
	bindings, cleared := s.Keybinds()
	if !cleared {
		t.Fatal("clear should be reported")
	}
	if len(bindings) != 1 || bindings[0].Key != ui.KeyB {
		t.Fatalf("bindings = %+v, want only cmd+b", bindings)
	}
}

func TestParseColor(t *testing.T) {
	if got, ok := parseColor("#112233"); !ok || got != ui.RGB(0x11, 0x22, 0x33) {
		t.Fatalf("#112233 = %v ok=%v", got, ok)
	}
	if got, ok := parseColor("36c"); !ok || got != ui.RGB(0x33, 0x66, 0xcc) {
		t.Fatalf("36c = %v ok=%v", got, ok)
	}
	if _, ok := parseColor("not-a-color"); ok {
		t.Fatal("not-a-color should not parse")
	}
}

func TestTerminalColorOverrides(t *testing.T) {
	s := LoadSettings(t.TempDir() + "/config")
	s.Set("background", "#112233")
	s.Set("foreground", "#f0f0f0")
	s.SetValues("palette", []string{"1=#abcdef"})
	cfg := s.Terminal(t.TempDir())
	if cfg.Light.Background != ui.RGB(0x11, 0x22, 0x33) {
		t.Fatalf("light background = %v", cfg.Light.Background)
	}
	if cfg.Dark.Background != ui.RGB(0x11, 0x22, 0x33) {
		t.Fatalf("dark background = %v", cfg.Dark.Background)
	}
	if cfg.Light.Palette[1] != ui.RGB(0xab, 0xcd, 0xef) {
		t.Fatalf("palette[1] = %v", cfg.Light.Palette[1])
	}
}

func TestTerminalOptions(t *testing.T) {
	s := LoadSettings(t.TempDir() + "/config")
	s.Set("mouse-scroll-multiplier", "precision:0.5,discrete:4")
	s.Set("right-click-action", "paste")
	s.Set("clipboard-read", "allow")
	s.Set("clipboard-write", "deny")
	s.Set("minimum-contrast", "4.5")
	s.Set("image-storage-limit", "1000")
	s.Set("background-opacity", "0.8")
	cfg := s.Terminal(t.TempDir())
	if cfg.ScrollMultiplier.Precision != 0.5 || cfg.ScrollMultiplier.Discrete != 4 {
		t.Fatalf("scroll multiplier = %+v", cfg.ScrollMultiplier)
	}
	if cfg.RightClick != terminal.RightClickPaste {
		t.Fatalf("right click = %v", cfg.RightClick)
	}
	if cfg.ClipboardRead != terminal.ClipboardAllow || !cfg.NoClipboardWrite {
		t.Fatalf("clipboard read=%v write-denied=%v", cfg.ClipboardRead, cfg.NoClipboardWrite)
	}
	if cfg.MinimumContrast != 4.5 || cfg.ImageLimit != 1000 {
		t.Fatalf("contrast=%v limit=%v", cfg.MinimumContrast, cfg.ImageLimit)
	}
	if !cfg.Transparent {
		t.Fatal("background-opacity below 1 should make the terminal transparent")
	}
}

func TestUnsupportedKeys(t *testing.T) {
	s := LoadSettings(t.TempDir() + "/config")
	s.Set("font-size", "14")
	s.Set("split-inherit-working-directory", "true")
	s.Set("font-style", "false")
	got := s.UnsupportedKeys()
	if len(got) != 2 || got[0] != "split-inherit-working-directory" || got[1] != "font-style" {
		t.Fatalf("unsupported = %v", got)
	}
}

func TestSettingsWritesOnSave(t *testing.T) {
	path := t.TempDir() + "/config"
	s := LoadSettings(path)
	s.Set("font-size", "14")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "font-size = 14") {
		t.Fatalf("file = %s", data)
	}
}
