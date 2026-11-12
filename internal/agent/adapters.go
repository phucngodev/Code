package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Claude ---------------------------------------------------------------

var claudeHookEvents = []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Notification", "Stop", "StopFailure", "SessionEnd"}

var claudeNeedsInput = map[string]bool{
	"permission_prompt":      true,
	"agent_needs_input":      true,
	"elicitation_dialog":     true,
	"elicitation_url_dialog": true,
}

var claudeAdapter = Adapter{
	Name:      "claude",
	Binary:    "claude",
	Kind:      LaunchFlags,
	Arguments: claudeArguments,
	Translate: claudeTranslate,
}

func claudeSettings(cde string) map[string]any {
	hook := map[string]any{"type": "command", "command": shellQuote(cde) + " hook claude"}
	hooks := map[string]any{}
	for _, event := range claudeHookEvents {
		hooks[event] = []any{map[string]any{"hooks": []any{hook}}}
	}
	return map[string]any{
		"hooks":                 hooks,
		"preferredNotifChannel": "notifications_disabled",
	}
}

// claudeArguments adds Code's settings to the user's arguments, merging a
// --settings the user passed rather than shadowing it.
func claudeArguments(arguments []string, cde string) []string {
	settings := claudeSettings(cde)
	optionsEnd := len(arguments)
	for i, argument := range arguments {
		if argument == "--" {
			optionsEnd = i
			break
		}
	}
	index, joined := -1, false
	for i := 0; i < optionsEnd; i++ {
		if arguments[i] == "--settings" {
			index = i
			break
		}
		if strings.HasPrefix(arguments[i], "--settings=") {
			index, joined = i, true
			break
		}
	}
	if index >= 0 {
		var value string
		removeEnd := index
		if joined {
			value = strings.TrimPrefix(arguments[index], "--settings=")
		} else if index+1 < len(arguments) {
			value = arguments[index+1]
			removeEnd = index + 1
		} else {
			return arguments
		}
		user := claudeUserSettings(value)
		if user == nil {
			return arguments
		}
		settings = claudeMerge(settings, user)
		arguments = append(append([]string{}, arguments[:index]...), arguments[removeEnd+1:]...)
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return arguments
	}
	return append([]string{"--settings", string(data)}, arguments...)
}

func claudeUserSettings(value string) map[string]any {
	trimmed := strings.TrimSpace(value)
	var data []byte
	if strings.HasPrefix(trimmed, "{") {
		data = []byte(trimmed)
	} else if file, err := os.ReadFile(value); err == nil {
		data = file
	}
	if data == nil {
		return nil
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil
	}
	return settings
}

// claudeMerge keeps the user's hooks and lets Code's other keys win.
func claudeMerge(code, user map[string]any) map[string]any {
	merged := map[string]any{}
	for key, value := range user {
		merged[key] = value
	}
	for key, value := range code {
		if key != "hooks" {
			merged[key] = value
		}
	}
	hooks, _ := user["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	codeHooks, _ := code["hooks"].(map[string]any)
	for event, entries := range codeHooks {
		existing, _ := hooks[event].([]any)
		hooks[event] = append(existing, entries.([]any)...)
	}
	merged["hooks"] = hooks
	return merged
}

func claudeTranslate(payload HookPayload) *Event {
	if payload.Has("agent_id") {
		return nil
	}
	name := payload.String("hook_event_name")
	if name == "" {
		return nil
	}
	tool := payload.String("tool_name")
	session := payload.String("session_id")
	cwd := payload.String("cwd")
	switch name {
	case "SessionStart":
		return payload.Event(KindSessionStarted, session, cwd, "")
	case "UserPromptSubmit":
		return payload.Event(KindTurnStarted, session, cwd, "")
	case "PostToolUse":
		if tool == "PushNotification" {
			message, _ := payload.Object("tool_input")["message"].(string)
			return payload.Event(KindAttention, session, cwd, message)
		}
		return payload.Event(KindWorking, session, cwd, tool)
	case "PreToolUse":
		return payload.Event(KindWorking, session, cwd, tool)
	case "Notification":
		if !claudeNeedsInput[payload.String("notification_type")] {
			return nil
		}
		return payload.Event(KindNeedsInput, session, cwd, payload.String("notification_type"))
	case "Stop":
		return payload.Event(KindTurnEnded, session, cwd, payload.Summary("last_assistant_message"))
	case "StopFailure":
		detail := payload.Summary("last_assistant_message")
		if detail == "" {
			detail = payload.Summary("error_details")
		}
		if detail == "" {
			detail = payload.Summary("error")
		}
		return payload.Event(KindTurnEnded, session, cwd, detail)
	case "SessionEnd":
		return payload.Event(KindSessionEnded, session, cwd, "")
	}
	return nil
}

// Codex ----------------------------------------------------------------

var codexHookEvents = []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "PermissionRequest", "Stop", "Interrupt", "SessionEnd"}

var codexNotificationSettings = [][2]string{
	{"tui.notifications", `["approval-requested","plan-mode-prompt","async-question"]`},
	{"tui.notification_method", `"osc9"`},
	{"tui.notification_condition", `"always"`},
}

var codexWaitingNotifications = []string{"Approval requested", "Codex wants to edit", "Plan mode prompt:", "Question:"}

var codexAdapter = Adapter{
	Name:                  "codex",
	Binary:                "codex",
	Kind:                  LaunchFlags,
	Arguments:             codexArguments,
	Translate:             codexTranslate,
	TranslateNotification: codexTranslateNotification,
}

func codexOverride(event, cde string) string {
	command := shellQuote(cde) + " hook codex"
	return `hooks.` + event + `=[{hooks=[{type="command",command=` + tomlQuote(command) + `}]}]`
}

func codexArguments(arguments []string, cde string) []string {
	optionsEnd := len(arguments)
	for i, argument := range arguments {
		if argument == "--" {
			optionsEnd = i
			break
		}
	}
	overridden := codexUserOverrides(arguments[:optionsEnd])
	has := func(key string) bool {
		for _, overriddenKey := range overridden {
			if overriddenKey == key {
				return true
			}
		}
		return false
	}
	var out []string
	if !has("tui") {
		for _, setting := range codexNotificationSettings {
			if has(setting[0]) {
				continue
			}
			out = append(out, "-c", setting[0]+"="+setting[1])
		}
	}
	if !has("hooks") {
		for _, event := range codexHookEvents {
			skip := false
			for _, key := range overridden {
				if key == "hooks."+event || strings.HasPrefix(key, "hooks."+event+".") {
					skip = true
					break
				}
			}
			if !skip {
				out = append(out, "-c", codexOverride(event, cde))
			}
		}
	}
	return append(out, arguments...)
}

func codexUserOverrides(options []string) []string {
	var keys []string
	for i := 0; i < len(options); i++ {
		option := options[i]
		value, has := "", false
		switch {
		case option == "-c" || option == "--config":
			if i+1 < len(options) {
				value, has = options[i+1], true
				i++
			}
		case strings.HasPrefix(option, "--config="):
			value, has = strings.TrimPrefix(option, "--config="), true
		case strings.HasPrefix(option, "-c"):
			value, has = option[2:], true
		}
		if has {
			key := value
			if index := strings.IndexByte(value, '='); index >= 0 {
				key = value[:index]
			}
			keys = append(keys, strings.TrimSpace(strings.ReplaceAll(key, `"`, "")))
		}
	}
	return keys
}

func codexTranslate(payload HookPayload) *Event {
	if payload.Has("agent_id") {
		return nil
	}
	name := payload.String("hook_event_name")
	session := payload.String("session_id")
	cwd := payload.String("cwd")
	switch name {
	case "SessionStart":
		return payload.Event(KindSessionStarted, session, cwd, "")
	case "UserPromptSubmit":
		return payload.Event(KindTurnStarted, session, cwd, "")
	case "PreToolUse", "PostToolUse":
		return payload.Event(KindWorking, session, cwd, payload.String("tool_name"))
	case "PermissionRequest":
		return nil
	case "Stop":
		return payload.Event(KindTurnEnded, session, cwd, payload.Summary("last_assistant_message"))
	case "Interrupt":
		return payload.Event(KindTurnEnded, session, cwd, "")
	case "SessionEnd":
		return payload.Event(KindSessionEnded, session, cwd, "")
	}
	return nil
}

func codexTranslateNotification(message string) (Kind, string, bool) {
	for _, prefix := range codexWaitingNotifications {
		if strings.HasPrefix(message, prefix) {
			return KindNeedsInput, Summarize(message), true
		}
	}
	return "", "", false
}

func tomlQuote(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}

// OpenCode -------------------------------------------------------------

const openCodeContentKey = "OPENCODE_CONFIG_CONTENT"

var openCodeAdapter = Adapter{
	Name:        "opencode",
	Binary:      "opencode",
	Kind:        LaunchEnvironment,
	Environment: openCodeEnvironment,
	Translate:   func(HookPayload) *Event { return nil },
}

func openCodeEnvironment(environment map[string]string, cde string) map[string]string {
	dir := environment[EnvAgentsDir]
	if dir == "" {
		dir = filepath.Join(filepath.Dir(cde), "..", "Resources", "agents")
	}
	plugin := filepath.Join(dir, "agents", "opencode", "code.js")
	pluginDir := filepath.Join(dir, "agents", "opencode", "v2")
	config := map[string]any{}
	if content := environment[openCodeContentKey]; strings.TrimSpace(content) != "" {
		if err := json.Unmarshal([]byte(content), &config); err != nil {
			return map[string]string{}
		}
	}
	config["plugin"] = appendUnique(anySlice(config["plugin"]), plugin)
	config["plugins"] = appendUnique(anySlice(config["plugins"]), pluginDir)
	data, err := json.Marshal(config)
	if err != nil {
		return map[string]string{}
	}
	return map[string]string{openCodeContentKey: string(data)}
}

func anySlice(value any) []any {
	if slice, ok := value.([]any); ok {
		return slice
	}
	return nil
}

// appendUnique keeps a plugin path from being added twice when an agent
// launches another agent with the same environment.
func appendUnique(values []any, value string) []any {
	for _, existing := range values {
		if text, ok := existing.(string); ok && text == value {
			return values
		}
	}
	return append(values, value)
}

// Pi and PiG load extensions from their own config folders, so Code installs
// the extension instead of shadowing the binary.

var piAdapter = Adapter{
	Name:      "pi",
	Binary:    "pi",
	Kind:      LaunchNone,
	Translate: func(HookPayload) *Event { return nil },
}

var pigAdapter = Adapter{
	Name:      "pig",
	Binary:    "pig",
	Kind:      LaunchNone,
	Translate: func(HookPayload) *Event { return nil },
}
