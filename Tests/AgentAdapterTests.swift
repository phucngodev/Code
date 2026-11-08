import XCTest

/// Hook input recorded from Claude Code 2.1.283.
enum ClaudeHookFixtures {
    static let sessionStart = #"{"session_id":"62dfedaf-72b8-418a-a11c-30c406412c34","transcript_path":"/Users/me/.claude/projects/-Users-me-project/62dfedaf-72b8-418a-a11c-30c406412c34.jsonl","cwd":"/Users/me/project","hook_event_name":"SessionStart","source":"startup"}"#
    static let userPromptSubmit = #"{"session_id":"62dfedaf-72b8-418a-a11c-30c406412c34","transcript_path":"/Users/me/.claude/projects/-Users-me-project/62dfedaf-72b8-418a-a11c-30c406412c34.jsonl","cwd":"/Users/me/project","prompt_id":"f7775cc5-c1db-4737-8ed2-2d010c75f921","permission_mode":"default","hook_event_name":"UserPromptSubmit","prompt":"First run the shell command 'echo hi' with the Bash tool. Then use the Agent/Task tool to launch a general-purpose subagent that runs 'echo sub' with Bash. Then reply with the single word ok."}"#
    static let preToolUse = #"{"session_id":"62dfedaf-72b8-418a-a11c-30c406412c34","transcript_path":"/Users/me/.claude/projects/-Users-me-project/62dfedaf-72b8-418a-a11c-30c406412c34.jsonl","cwd":"/Users/me/project","prompt_id":"f7775cc5-c1db-4737-8ed2-2d010c75f921","permission_mode":"default","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo hi","description":"Echo hi to test bash tool"},"tool_use_id":"toolu_01Qh7rZi5PRxMyHKtJ554mnq"}"#
    static let postToolUse = #"{"session_id":"62dfedaf-72b8-418a-a11c-30c406412c34","transcript_path":"/Users/me/.claude/projects/-Users-me-project/62dfedaf-72b8-418a-a11c-30c406412c34.jsonl","cwd":"/Users/me/project","prompt_id":"f7775cc5-c1db-4737-8ed2-2d010c75f921","permission_mode":"default","hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"echo hi","description":"Echo hi to test bash tool"},"tool_response":{"stdout":"hi","stderr":"","interrupted":false,"isImage":false,"noOutputExpected":false},"tool_use_id":"toolu_01Qh7rZi5PRxMyHKtJ554mnq","duration_ms":31}"#
    static let subagentPreToolUse = #"{"session_id":"62dfedaf-72b8-418a-a11c-30c406412c34","transcript_path":"/Users/me/.claude/projects/-Users-me-project/62dfedaf-72b8-418a-a11c-30c406412c34.jsonl","cwd":"/Users/me/project","prompt_id":"f7775cc5-c1db-4737-8ed2-2d010c75f921","permission_mode":"default","agent_id":"af40843b4cd16fa52","agent_type":"general-purpose","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo sub","description":"Run echo sub command"},"tool_use_id":"toolu_01BKzNHaTX38cP1a2Szfxivm"}"#
    static let subagentStop = #"{"session_id":"62dfedaf-72b8-418a-a11c-30c406412c34","transcript_path":"/Users/me/.claude/projects/-Users-me-project/62dfedaf-72b8-418a-a11c-30c406412c34.jsonl","cwd":"/Users/me/project","prompt_id":"f7775cc5-c1db-4737-8ed2-2d010c75f921","permission_mode":"default","agent_id":"af40843b4cd16fa52","agent_type":"general-purpose","hook_event_name":"SubagentStop","stop_hook_active":false,"agent_transcript_path":"/Users/me/.claude/projects/-Users-me-project/62dfedaf-72b8-418a-a11c-30c406412c34/subagents/agent-af40843b4cd16fa52.jsonl","last_assistant_message":"The command `echo sub` executed successfully. The output is:\n\n```\nsub\n```","background_tasks":[{"id":"af40843b4cd16fa52","type":"subagent","status":"running","description":"Run echo sub command","agent_type":"general-purpose"}],"session_crons":[]}"#
    static let stop = #"{"session_id":"62dfedaf-72b8-418a-a11c-30c406412c34","transcript_path":"/Users/me/.claude/projects/-Users-me-project/62dfedaf-72b8-418a-a11c-30c406412c34.jsonl","cwd":"/Users/me/project","prompt_id":"f7775cc5-c1db-4737-8ed2-2d010c75f921","permission_mode":"default","hook_event_name":"Stop","stop_hook_active":false,"last_assistant_message":"ok","background_tasks":[{"id":"af40843b4cd16fa52","type":"subagent","status":"running","description":"Run echo sub command","agent_type":"general-purpose"}],"session_crons":[]}"#
    static let stopWithoutMessage = #"{"session_id":"62dfedaf-72b8-418a-a11c-30c406412c34","transcript_path":"/Users/me/.claude/projects/-Users-me-project/62dfedaf-72b8-418a-a11c-30c406412c34.jsonl","cwd":"/Users/me/project","prompt_id":"1f4a6a5c-83ab-4093-a210-4c6eb238314e","permission_mode":"default","hook_event_name":"Stop","stop_hook_active":false,"background_tasks":[],"session_crons":[]}"#
    static let sessionEnd = #"{"session_id":"62dfedaf-72b8-418a-a11c-30c406412c34","transcript_path":"/Users/me/.claude/projects/-Users-me-project/62dfedaf-72b8-418a-a11c-30c406412c34.jsonl","cwd":"/Users/me/project","prompt_id":"1f4a6a5c-83ab-4093-a210-4c6eb238314e","hook_event_name":"SessionEnd","reason":"other"}"#

    // Print mode never notifies and the PushNotification tool is behind a
    // feature flag, so these follow the documented shape instead.
    static let permissionPrompt = #"{"session_id":"62dfedaf-72b8-418a-a11c-30c406412c34","transcript_path":"/Users/me/.claude/projects/-Users-me-project/62dfedaf-72b8-418a-a11c-30c406412c34.jsonl","cwd":"/Users/me/project","hook_event_name":"Notification","message":"Claude needs your permission to use Bash","notification_type":"permission_prompt"}"#
    static let authSuccess = #"{"session_id":"62dfedaf-72b8-418a-a11c-30c406412c34","cwd":"/Users/me/project","hook_event_name":"Notification","message":"Authenticated","notification_type":"auth_success"}"#
    static let pushNotification = #"{"session_id":"62dfedaf-72b8-418a-a11c-30c406412c34","cwd":"/Users/me/project","hook_event_name":"PostToolUse","tool_name":"PushNotification","tool_input":{"message":"Build finished","status":"proactive"},"tool_response":{},"tool_use_id":"toolu_01"}"#

    // A failed API call cannot be forced on demand, so this follows the
    // documented shape too.
    static let stopFailure = #"{"session_id":"62dfedaf-72b8-418a-a11c-30c406412c34","transcript_path":"/Users/me/.claude/projects/-Users-me-project/62dfedaf-72b8-418a-a11c-30c406412c34.jsonl","cwd":"/Users/me/project","hook_event_name":"StopFailure","error":"rate_limit","error_details":"429 Too Many Requests","last_assistant_message":"API Error: Rate limit reached"}"#
}

final class AgentAdapterTests: XCTestCase {
    func testNamesAreUniqueAndEveryAdapterHasABinary() {
        XCTAssertEqual(AgentAdapter.all.map(\.name), ["claude", "opencode", "codex", "pi", "pig"])
        XCTAssertEqual(Set(AgentAdapter.all.map(\.name)).count, AgentAdapter.all.count)
        XCTAssertEqual(Set(AgentAdapter.all.map(\.binary)).count, AgentAdapter.all.count)
        for adapter in AgentAdapter.all {
            XCTAssertFalse(adapter.binary.isEmpty, adapter.name)
            XCTAssertFalse(adapter.binary.contains("/"), adapter.name)
            XCTAssertEqual(AgentAdapter.named(adapter.name)?.binary, adapter.binary)
        }
        XCTAssertNil(AgentAdapter.named("nonexistent"))
    }

    func testOnlyCodexReadsItsNotifications() {
        XCTAssertNil(AgentAdapter.claudeCode.translateNotification)
        XCTAssertNil(AgentAdapter.openCode.translateNotification)
        XCTAssertNil(AgentAdapter.pi.translateNotification)
        XCTAssertNil(AgentAdapter.pig.translateNotification)
        XCTAssertNotNil(AgentAdapter.codex.translateNotification)
    }
}

final class ClaudeCodeTests: XCTestCase {
    let surface = UUID(), workspace = UUID()
    let now = Date(timeIntervalSince1970: 1_790_000_000)
    let session = "62dfedaf-72b8-418a-a11c-30c406412c34"

    func translate(_ json: String) throws -> AgentEvent? {
        let payload = try XCTUnwrap(HookPayload(
            json: Data(json.utf8), agent: "claude",
            environment: ["CODE_SURFACE_ID": surface.uuidString, "CODE_WORKSPACE_ID": workspace.uuidString], now: now
        ))
        return ClaudeCode.translate(payload)
    }

    func expected(_ kind: AgentEvent.Kind, _ detail: String? = nil) -> AgentEvent {
        AgentEvent(agent: "claude", kind: kind, sessionID: session, surfaceID: surface, workspaceID: workspace,
                   cwd: "/Users/me/project", at: now, detail: detail)
    }

    func testTranslatesTheSessionLifecycle() throws {
        XCTAssertEqual(try translate(ClaudeHookFixtures.sessionStart), expected(.sessionStarted))
        XCTAssertEqual(try translate(ClaudeHookFixtures.userPromptSubmit), expected(.turnStarted))
        XCTAssertEqual(try translate(ClaudeHookFixtures.preToolUse), expected(.working, "Bash"))
        XCTAssertEqual(try translate(ClaudeHookFixtures.postToolUse), expected(.working, "Bash"))
        XCTAssertEqual(try translate(ClaudeHookFixtures.stop), expected(.turnEnded, "ok"))
        XCTAssertEqual(try translate(ClaudeHookFixtures.stopWithoutMessage), expected(.turnEnded))
        XCTAssertEqual(try translate(ClaudeHookFixtures.sessionEnd), expected(.sessionEnded))
    }

    func testNotificationsThatWaitOnTheUserNeedInput() throws {
        XCTAssertEqual(try translate(ClaudeHookFixtures.permissionPrompt), expected(.needsInput, "permission_prompt"))
        for type in ["agent_needs_input", "elicitation_dialog", "elicitation_url_dialog"] {
            let json = ClaudeHookFixtures.permissionPrompt.replacingOccurrences(of: "permission_prompt", with: type)
            XCTAssertEqual(try translate(json), expected(.needsInput, type))
        }
        XCTAssertNil(try translate(ClaudeHookFixtures.authSuccess))
        XCTAssertNil(try translate(ClaudeHookFixtures.permissionPrompt.replacingOccurrences(of: "permission_prompt", with: "idle_prompt")))
    }

    func testAFailedTurnEndsWithTheError() throws {
        XCTAssertEqual(try translate(ClaudeHookFixtures.stopFailure), expected(.turnEnded, "API Error: Rate limit reached"))
        let withoutMessage = ClaudeHookFixtures.stopFailure.replacingOccurrences(of: #","last_assistant_message":"API Error: Rate limit reached""#, with: "")
        XCTAssertEqual(try translate(withoutMessage), expected(.turnEnded, "429 Too Many Requests"))
        let typeOnly = withoutMessage.replacingOccurrences(of: #","error_details":"429 Too Many Requests""#, with: "")
        XCTAssertEqual(try translate(typeOnly), expected(.turnEnded, "rate_limit"))
        let long = String(repeating: "overloaded ", count: 50)
        let detail = try XCTUnwrap(try translate(ClaudeHookFixtures.stopFailure.replacingOccurrences(of: "API Error: Rate limit reached", with: #"API Error:\n\#(long)"#))?.detail)
        XCTAssertEqual(detail.count, HookPayload.detailLimit)
        XCTAssertFalse(detail.contains("\n"))
    }

    func testPushNotificationAsksForAttention() throws {
        XCTAssertEqual(try translate(ClaudeHookFixtures.pushNotification), expected(.attention, "Build finished"))
    }

    func testDropsSubagentsAndEventsCodeDoesNotUse() throws {
        XCTAssertNil(try translate(ClaudeHookFixtures.subagentPreToolUse))
        XCTAssertNil(try translate(ClaudeHookFixtures.subagentStop))
        XCTAssertNil(try translate(#"{"session_id":"s","hook_event_name":"PreCompact"}"#))
        XCTAssertNil(try translate(#"{"session_id":"s"}"#))
    }

    func testTurnEndedDetailIsOneShortLine() throws {
        let long = String(repeating: "word ", count: 100)
        let json = ClaudeHookFixtures.stop.replacingOccurrences(of: #""last_assistant_message":"ok""#, with: #""last_assistant_message":"Done.\n\n\#(long)""#)
        let detail = try XCTUnwrap(try translate(json)?.detail)
        XCTAssertEqual(detail.count, HookPayload.detailLimit)
        XCTAssertTrue(detail.hasPrefix("Done. word word"))
        XCTAssertTrue(detail.hasSuffix("…"))
        XCTAssertFalse(detail.contains("\n"))
    }

    // MARK: Settings

    let cde = "/Applications/Code Beta.app/Contents/Helpers/cde"

    func settings(in arguments: [String]) throws -> [String: Any] {
        XCTAssertEqual(arguments.filter { $0 == "--settings" }.count, 1)
        let index = try XCTUnwrap(arguments.firstIndex(of: "--settings"))
        return try XCTUnwrap(JSONSerialization.jsonObject(with: Data(arguments[index + 1].utf8)) as? [String: Any])
    }

    func commands(_ settings: [String: Any], _ event: String) -> [String] {
        let entries = (settings["hooks"] as? [String: Any])?[event] as? [[String: Any]] ?? []
        return entries.flatMap { ($0["hooks"] as? [[String: Any]] ?? []).compactMap { $0["command"] as? String } }
    }

    func testAddsCodesHooksAndSilencesClaudesNotifications() throws {
        let arguments = ClaudeCode.arguments(["-p", "hi"], cde: cde)
        XCTAssertEqual(Array(arguments.suffix(2)), ["-p", "hi"])
        let settings = try settings(in: arguments)

        XCTAssertEqual(settings["preferredNotifChannel"] as? String, "notifications_disabled")
        XCTAssertEqual(Set((settings["hooks"] as? [String: Any] ?? [:]).keys), Set(ClaudeCode.hookEvents))
        for event in ClaudeCode.hookEvents {
            XCTAssertEqual(commands(settings, event), ["'/Applications/Code Beta.app/Contents/Helpers/cde' hook claude"])
        }
        let hook = try XCTUnwrap(((settings["hooks"] as? [String: Any])?["Stop"] as? [[String: Any]])?.first?["hooks"] as? [[String: Any]]).first
        XCTAssertEqual(hook?["type"] as? String, "command")
        XCTAssertNil(hook?["async"])
    }

    func testMergesIntoTheUsersInlineSettings() throws {
        let user = #"{"model":"opus","hooks":{"Stop":[{"hooks":[{"type":"command","command":"say done"}]}],"PreCompact":[{"hooks":[{"type":"command","command":"true"}]}]}}"#
        let arguments = ClaudeCode.arguments(["--model", "haiku", "--settings", user, "-p", "hi"], cde: cde)
        XCTAssertEqual(Array(arguments.suffix(4)), ["--model", "haiku", "-p", "hi"])
        let settings = try settings(in: arguments)

        XCTAssertEqual(settings["model"] as? String, "opus")
        XCTAssertEqual(commands(settings, "Stop"), ["say done", "'\(cde)' hook claude"])
        XCTAssertEqual(commands(settings, "PreCompact"), ["true"])
        XCTAssertEqual(commands(settings, "SessionStart"), ["'\(cde)' hook claude"])
    }

    func testMergesTheUsersSettingsFile() throws {
        let file = FileManager.default.temporaryDirectory.appendingPathComponent("code-settings-\(UUID().uuidString).json")
        try Data(#"{"hooks":{"Notification":[{"matcher":"idle_prompt","hooks":[{"type":"command","command":"say idle"}]}]}}"#.utf8).write(to: file)
        defer { try? FileManager.default.removeItem(at: file) }

        let settings = try settings(in: ClaudeCode.arguments(["--settings=\(file.path)"], cde: cde))

        XCTAssertEqual(commands(settings, "Notification"), ["say idle", "'\(cde)' hook claude"])
    }

    func testLeavesSettingsItCannotReadForClaudeToReport() {
        XCTAssertEqual(ClaudeCode.arguments(["--settings", "/nonexistent.json"], cde: cde), ["--settings", "/nonexistent.json"])
        XCTAssertEqual(ClaudeCode.arguments(["--settings", "{not json"], cde: cde), ["--settings", "{not json"])
    }

    func testArgumentsAfterDoubleDashAreNotOptions() throws {
        let arguments = ClaudeCode.arguments(["--", "--settings", "x"], cde: cde)
        XCTAssertEqual(Array(arguments.suffix(3)), ["--", "--settings", "x"])
        XCTAssertEqual(arguments.count, 5)
    }
}

/// Hook input recorded from Codex 0.156.1: an interactive session, and a
/// `codex exec` run that started a subagent, given the one session id.
enum CodexHookFixtures {
    static let sessionStart = #"{"session_id":"01a0e974-84c5-74f1-bbf6-676b95c1bea9","transcript_path":"/Users/me/.codex/sessions/2026/09/29/rollout-2026-09-29T00-48-42-01a0e974-84c5-74f1-bbf6-676b95c1bea9.jsonl","cwd":"/Users/me/project","hook_event_name":"SessionStart","model":"gpt-6-sol","permission_mode":"default","source":"startup"}"#
    static let userPromptSubmit = #"{"session_id":"01a0e974-84c5-74f1-bbf6-676b95c1bea9","turn_id":"01a0e974-a74b-7461-b482-a7cc53047f22","transcript_path":"/Users/me/.codex/sessions/2026/09/29/rollout-2026-09-29T00-48-42-01a0e974-84c5-74f1-bbf6-676b95c1bea9.jsonl","cwd":"/Users/me/project","hook_event_name":"UserPromptSubmit","model":"gpt-6-sol","permission_mode":"default","prompt":"Create a file named made.txt containing hi, using the shell command: echo hi > made.txt"}"#
    static let preToolUse = #"{"session_id":"01a0e974-84c5-74f1-bbf6-676b95c1bea9","turn_id":"01a0e974-a74b-7461-b482-a7cc53047f22","transcript_path":"/Users/me/.codex/sessions/2026/09/29/rollout-2026-09-29T00-48-42-01a0e974-84c5-74f1-bbf6-676b95c1bea9.jsonl","cwd":"/Users/me/project","hook_event_name":"PreToolUse","model":"gpt-6-sol","permission_mode":"default","tool_name":"Bash","tool_input":{"command":"echo hi > made.txt"},"tool_use_id":"exec-61b6fa80-9fbb-4ff0-9bcc-380bbc1f7f39"}"#
    static let postToolUse = #"{"session_id":"01a0e974-84c5-74f1-bbf6-676b95c1bea9","turn_id":"01a0e974-a74b-7461-b482-a7cc53047f22","transcript_path":"/Users/me/.codex/sessions/2026/09/29/rollout-2026-09-29T00-48-42-01a0e974-84c5-74f1-bbf6-676b95c1bea9.jsonl","cwd":"/Users/me/project","hook_event_name":"PostToolUse","model":"gpt-6-sol","permission_mode":"default","tool_name":"Bash","tool_input":{"command":"echo hi > made.txt"},"tool_response":"zsh:1: operation not permitted: made.txt\n","tool_use_id":"exec-61b6fa80-9fbb-4ff0-9bcc-380bbc1f7f39"}"#
    static let permissionRequest = #"{"session_id":"01a0e974-84c5-74f1-bbf6-676b95c1bea9","turn_id":"01a0e974-a74b-7461-b482-a7cc53047f22","transcript_path":"/Users/me/.codex/sessions/2026/09/29/rollout-2026-09-29T00-48-42-01a0e974-84c5-74f1-bbf6-676b95c1bea9.jsonl","cwd":"/Users/me/project","hook_event_name":"PermissionRequest","model":"gpt-6-sol","permission_mode":"default","tool_name":"Bash","tool_input":{"command":"echo hi > made.txt","description":"May I run the requested shell command to create made.txt in this read-only workspace?"}}"#
    static let interrupt = #"{"session_id":"01a0e974-84c5-74f1-bbf6-676b95c1bea9","turn_id":"01a0e974-a74b-7461-b482-a7cc53047f22","transcript_path":"/Users/me/.codex/sessions/2026/09/29/rollout-2026-09-29T00-48-42-01a0e974-84c5-74f1-bbf6-676b95c1bea9.jsonl","cwd":"/Users/me/project","hook_event_name":"Interrupt","model":"gpt-6-sol","permission_mode":"default"}"#
    static let sessionEnd = #"{"session_id":"01a0e974-84c5-74f1-bbf6-676b95c1bea9","transcript_path":"/Users/me/.codex/sessions/2026/09/29/rollout-2026-09-29T00-48-42-01a0e974-84c5-74f1-bbf6-676b95c1bea9.jsonl","cwd":"/Users/me/project","hook_event_name":"SessionEnd","reason":"other"}"#
    static let stop = #"{"session_id":"01a0e974-84c5-74f1-bbf6-676b95c1bea9","turn_id":"01a0e973-c23e-7501-9e2d-856004adec92","transcript_path":"/Users/me/.codex/sessions/2026/09/29/rollout-2026-09-29T00-47-52-01a0e974-84c5-74f1-bbf6-676b95c1bea9.jsonl","cwd":"/Users/me/project","hook_event_name":"Stop","model":"gpt-6-sol","permission_mode":"bypassPermissions","stop_hook_active":false,"last_assistant_message":"ok"}"#
    static let subagentStart = #"{"session_id":"01a0e974-84c5-74f1-bbf6-676b95c1bea9","turn_id":"01a0e973-ebdd-7b32-8014-35e396e79b91","transcript_path":"/Users/me/.codex/sessions/2026/09/29/rollout-2026-09-29T00-48-03-01a0e973-ebd2-7993-b9a3-1b1ffd4d0634.jsonl","cwd":"/Users/me/project","hook_event_name":"SubagentStart","model":"gpt-6-sol","permission_mode":"bypassPermissions","agent_id":"01a0e973-ebd2-7993-b9a3-1b1ffd4d0634","agent_type":"default"}"#
    static let subagentPreToolUse = #"{"session_id":"01a0e974-84c5-74f1-bbf6-676b95c1bea9","turn_id":"01a0e973-ebdd-7b32-8014-35e396e79b91","agent_id":"01a0e973-ebd2-7993-b9a3-1b1ffd4d0634","agent_type":"default","transcript_path":"/Users/me/.codex/sessions/2026/09/29/rollout-2026-09-29T00-48-03-01a0e973-ebd2-7993-b9a3-1b1ffd4d0634.jsonl","cwd":"/Users/me/project","hook_event_name":"PreToolUse","model":"gpt-6-sol","permission_mode":"bypassPermissions","tool_name":"Bash","tool_input":{"command":"echo sub"},"tool_use_id":"exec-19b92c86-9faf-405c-b14d-dafe66874e66"}"#
    static let subagentStop = #"{"session_id":"01a0e974-84c5-74f1-bbf6-676b95c1bea9","turn_id":"01a0e973-ebdd-7b32-8014-35e396e79b91","transcript_path":"/Users/me/.codex/sessions/2026/09/29/rollout-2026-09-29T00-47-52-01a0e974-84c5-74f1-bbf6-676b95c1bea9.jsonl","agent_transcript_path":"/Users/me/.codex/sessions/2026/09/29/rollout-2026-09-29T00-48-03-01a0e973-ebd2-7993-b9a3-1b1ffd4d0634.jsonl","cwd":"/Users/me/project","hook_event_name":"SubagentStop","model":"gpt-6-sol","permission_mode":"bypassPermissions","stop_hook_active":false,"agent_id":"01a0e973-ebd2-7993-b9a3-1b1ffd4d0634","agent_type":"default","last_assistant_message":"sub"}"#
}

final class CodexTests: XCTestCase {
    let surface = UUID(), workspace = UUID()
    let now = Date(timeIntervalSince1970: 1_790_000_000)
    let session = "01a0e974-84c5-74f1-bbf6-676b95c1bea9"
    let cde = "/Applications/Code Beta.app/Contents/Helpers/cde"

    func translate(_ json: String) throws -> AgentEvent? {
        let payload = try XCTUnwrap(HookPayload(
            json: Data(json.utf8), agent: "codex",
            environment: ["CODE_SURFACE_ID": surface.uuidString, "CODE_WORKSPACE_ID": workspace.uuidString], now: now
        ))
        return Codex.translate(payload)
    }

    func expected(_ kind: AgentEvent.Kind, _ detail: String? = nil) -> AgentEvent {
        AgentEvent(agent: "codex", kind: kind, sessionID: session, surfaceID: surface, workspaceID: workspace,
                   cwd: "/Users/me/project", at: now, detail: detail)
    }

    func testTranslatesTheSessionLifecycle() throws {
        XCTAssertEqual(try translate(CodexHookFixtures.sessionStart), expected(.sessionStarted))
        XCTAssertEqual(try translate(CodexHookFixtures.userPromptSubmit), expected(.turnStarted))
        XCTAssertEqual(try translate(CodexHookFixtures.preToolUse), expected(.working, "Bash"))
        XCTAssertEqual(try translate(CodexHookFixtures.postToolUse), expected(.working, "Bash"))
        XCTAssertEqual(try translate(CodexHookFixtures.stop), expected(.turnEnded, "ok"))
        XCTAssertEqual(try translate(CodexHookFixtures.interrupt), expected(.turnEnded))
        XCTAssertEqual(try translate(CodexHookFixtures.sessionEnd), expected(.sessionEnded))
    }

    func testDropsSubagentsAndEventsCodeDoesNotUse() throws {
        XCTAssertNil(try translate(CodexHookFixtures.subagentStart))
        XCTAssertNil(try translate(CodexHookFixtures.subagentPreToolUse))
        XCTAssertNil(try translate(CodexHookFixtures.subagentStop))
        XCTAssertNil(try translate(#"{"session_id":"s","hook_event_name":"PreCompact","trigger":"auto"}"#))
        XCTAssertNil(try translate(#"{"session_id":"s"}"#))
    }

    func testPermissionRequestIsLeftToTheNotification() throws {
        XCTAssertNil(try translate(CodexHookFixtures.permissionRequest))
    }

    func testTurnEndedDetailIsOneShortLine() throws {
        let long = String(repeating: "word ", count: 100)
        let json = CodexHookFixtures.stop.replacingOccurrences(of: #""last_assistant_message":"ok""#, with: #""last_assistant_message":"Done.\n\n\#(long)""#)
        let detail = try XCTUnwrap(try translate(json)?.detail)
        XCTAssertEqual(detail.count, HookPayload.detailLimit)
        XCTAssertTrue(detail.hasPrefix("Done. word word"))
        XCTAssertTrue(detail.hasSuffix("…"))
    }

    // MARK: Notifications

    func testNotificationsThatPutSomethingToTheUserNeedInput() throws {
        for message in ["Approval requested: echo hi > made.txt", "Codex wants to edit made.txt", "Codex wants to edit 3 files",
                        "Approval requested by github", "Plan mode prompt: Implement this plan?", "Question: Which branch?"] {
            let translated = try XCTUnwrap(Codex.translateNotification(message), message)
            XCTAssertEqual(translated.kind, .needsInput)
            XCTAssertEqual(translated.detail, message)
        }
    }

    func testNotificationDetailIsOneShortLine() throws {
        let command = String(repeating: "echo hi && ", count: 50)
        let detail = try XCTUnwrap(Codex.translateNotification("Approval requested: \(command)\nexit")?.detail)
        XCTAssertEqual(detail.count, HookPayload.detailLimit)
        XCTAssertTrue(detail.hasPrefix("Approval requested: echo hi && echo"))
        XCTAssertTrue(detail.hasSuffix("…"))
    }

    func testDropsNotificationsTheHooksCover() {
        XCTAssertNil(Codex.translateNotification("Agent turn complete"))
        XCTAssertNil(Codex.translateNotification("I created made.txt."))
        XCTAssertNil(Codex.translateNotification(""))
    }

    // MARK: Arguments

    let settings = ["tui.notifications": #"["approval-requested","plan-mode-prompt","async-question"]"#, "tui.notification_method": #""osc9""#, "tui.notification_condition": #""always""#]

    let hook = #"command="'/Applications/Code Beta.app/Contents/Helpers/cde' hook codex""#

    func overrides(_ arguments: [String]) -> [String: String] {
        var overrides: [String: String] = [:]
        for (index, argument) in arguments.enumerated() where argument == "-c" && index + 1 < arguments.count {
            let parts = arguments[index + 1].split(separator: "=", maxSplits: 1).map(String.init)
            overrides[parts[0], default: ""] += parts.count > 1 ? parts[1] : ""
        }
        return overrides
    }

    func testAddsCodesHooksAheadOfTheUsersArguments() {
        let arguments = Codex.arguments(["exec", "hi"], cde: cde)
        XCTAssertEqual(Array(arguments.suffix(2)), ["exec", "hi"])
        XCTAssertEqual(arguments.count, (settings.count + Codex.hookEvents.count) * 2 + 2)
        XCTAssertEqual(Set(overrides(arguments).keys), Set(Codex.hookEvents.map { "hooks.\($0)" }).union(settings.keys))
        XCTAssertEqual(overrides(arguments)["hooks.Stop"], #"[{hooks=[{type="command",\#(hook)}]}]"#)
        XCTAssertEqual(overrides(arguments)["hooks.SessionEnd"], #"[{hooks=[{type="command",\#(hook)}]}]"#)
    }

    func testForcesCodexsNotificationsFirst() {
        let arguments = Codex.arguments(["exec", "hi"], cde: cde)
        XCTAssertEqual(Array(arguments.prefix(6)), ["-c", #"tui.notifications=["approval-requested","plan-mode-prompt","async-question"]"#, "-c", #"tui.notification_method="osc9""#,
                                                    "-c", #"tui.notification_condition="always""#])
    }

    func testLeavesNotificationSettingsTheUserOverridesToThem() {
        let user = ["-c", #"tui.notification_condition="unfocused""#, #"--config="tui.notification_method"=bel"#]
        let arguments = Codex.arguments(user, cde: cde)
        XCTAssertEqual(Array(arguments.suffix(user.count)), user)
        XCTAssertEqual(overrides(Array(arguments.dropLast(user.count))).filter { $0.key.hasPrefix("tui.") }, ["tui.notifications": settings["tui.notifications"]!])

        let table = ["-c", "tui={notifications=false}"]
        XCTAssertEqual(overrides(Codex.arguments(table, cde: cde)).filter { settings[$0.key] != nil }, [:])
        XCTAssertEqual(Array(Codex.arguments(table, cde: cde).suffix(2)), table)
    }

    func testHooksAreTheSameEveryLaunch() {
        XCTAssertEqual(Codex.arguments([], cde: cde), Array(Codex.arguments(["-m", "o3"], cde: cde).dropLast(2)))
    }

    func testQuotesTheCommandForToml() {
        XCTAssertEqual(Codex.override(for: "Stop", cde: #"/a"b\c/cde"#),
                       #"hooks.Stop=[{hooks=[{type="command",command="'/a\"b\\c/cde' hook codex"}]}]"#)
    }

    func testLeavesEventsTheUserSetsHooksForToThem() {
        let user = ["-c", "hooks.Stop=[]", "--config=hooks.PreToolUse=[]", #"-chooks."Interrupt"=[]"#, "--config", "hooks.SessionStart.0.matcher='x'", "-c", "model=\"o3\""]
        let arguments = Codex.arguments(user, cde: cde)
        XCTAssertEqual(Array(arguments.suffix(user.count)), user)
        XCTAssertEqual(Set(overrides(Array(arguments.dropLast(user.count))).keys),
                       Set(["hooks.UserPromptSubmit", "hooks.PostToolUse", "hooks.PermissionRequest", "hooks.SessionEnd"]).union(settings.keys))
        XCTAssertEqual(Set(overrides(Codex.arguments(["-c", "hooks={}"], cde: cde)).keys), Set(settings.keys).union(["hooks"]))
    }

    func testArgumentsAfterDoubleDashAreNotOptions() {
        let user = ["exec", "--", "-c", "hooks.Stop=[]"]
        let arguments = Codex.arguments(user, cde: cde)
        XCTAssertEqual(Array(arguments.suffix(4)), user)
        XCTAssertEqual(arguments.count, (settings.count + Codex.hookEvents.count) * 2 + 4)
    }
}

final class OpenCodeTests: XCTestCase {
    let cde = "/Applications/Code Beta.app/Contents/Helpers/cde"
    let plugin = "/Applications/Code Beta.app/Contents/Resources/agents/opencode/code.js"
    let pluginDirectory = "/Applications/Code Beta.app/Contents/Resources/agents/opencode/v2"

    func config(_ environment: [String: String]) throws -> [String: Any]? {
        let variables = OpenCode.environment(environment, cde: cde)
        guard let content = variables["OPENCODE_CONFIG_CONTENT"] else { return nil }
        XCTAssertEqual(variables.count, 1)
        return try XCTUnwrap(JSONSerialization.jsonObject(with: Data(content.utf8)) as? [String: Any])
    }

    func testPluginsAreBundledNextToCde() {
        XCTAssertEqual(OpenCode.plugin(cde: cde), plugin)
        XCTAssertEqual(OpenCode.pluginDirectory(cde: cde), pluginDirectory)
    }

    func testNamesOnlyTheCodePlugins() throws {
        XCTAssertEqual(OpenCode.environment(["HOME": "/Users/me"], cde: cde), [
            "OPENCODE_CONFIG_CONTENT": #"{"plugin":["\#(plugin)"],"plugins":["\#(pluginDirectory)"]}"#,
        ])
        let config = try XCTUnwrap(try config(["OPENCODE_CONFIG_CONTENT": " "]))
        XCTAssertEqual(config["plugin"] as? [String], [plugin])
        XCTAssertEqual(config["plugins"] as? [String], [pluginDirectory])
    }

    func testKeepsTheUsersInlineConfig() throws {
        let user = #"{"model":"anthropic/claude","plugin":["opencode-wakatime",["my-plugin",{"a":1}]],"plugins":["opencode-other"]}"#
        let config = try XCTUnwrap(try config(["OPENCODE_CONFIG_CONTENT": user]))
        XCTAssertEqual(config["model"] as? String, "anthropic/claude")
        let plugins = try XCTUnwrap(config["plugin"] as? [Any])
        XCTAssertEqual(plugins.count, 3)
        XCTAssertEqual(plugins.first as? String, "opencode-wakatime")
        XCTAssertEqual(plugins.last as? String, plugin)
        XCTAssertEqual(config["plugins"] as? [String], ["opencode-other", pluginDirectory])
    }

    func testLeavesUnreadableInlineConfigAlone() throws {
        XCTAssertNil(try config(["OPENCODE_CONFIG_CONTENT": "{ // mine\n }"]))
    }

    func testLaunchSetsTheConfig() throws {
        let bin = FileManager.default.temporaryDirectory.appendingPathComponent("code-opencode-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: bin, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: bin) }
        FileManager.default.createFile(atPath: bin.appendingPathComponent("opencode").path, contents: Data("#!/bin/sh\n".utf8), attributes: [.posixPermissions: 0o755])

        let launcher = try XCTUnwrap(AgentLauncher(.openCode, arguments: ["run", "hi"], environment: ["PATH": bin.path], cde: cde))

        XCTAssertEqual(launcher.arguments, ["opencode", "run", "hi"])
        XCTAssertEqual(launcher.environment["OPENCODE_CONFIG_CONTENT"], #"{"plugin":["\#(plugin)"],"plugins":["\#(pluginDirectory)"]}"#)
    }
}

final class PiTests: XCTestCase {
    func testExtensionIsBundledInTheAppResources() {
        XCTAssertEqual(Pi.extensionResource, "agents/pi/code.ts")
    }

    func testPiIsNotShadowedAndLaunchesAsTyped() throws {
        let bin = FileManager.default.temporaryDirectory.appendingPathComponent("code-pi-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: bin, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: bin) }
        FileManager.default.createFile(atPath: bin.appendingPathComponent("pi").path, contents: Data("#!/bin/sh\n".utf8), attributes: [.posixPermissions: 0o755])

        XCTAssertFalse(AgentAdapter.pi.needsShim, "Pi loads its extension from its config folder, so its binary is not shadowed")
        let launcher = try XCTUnwrap(AgentLauncher(.pi, arguments: ["hello"], environment: ["PATH": bin.path], cde: "/cde"))
        XCTAssertEqual(launcher.arguments, ["pi", "hello"], "Pi launches with the arguments as typed")
    }
}

final class PigTests: XCTestCase {
    func testExtensionIsBundledInTheAppResources() {
        XCTAssertEqual(Pig.extensionResource, "agents/pig/code")
    }

    func testPigIsNotShadowedAndLaunchesAsTyped() throws {
        let bin = FileManager.default.temporaryDirectory.appendingPathComponent("code-pig-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: bin, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: bin) }
        FileManager.default.createFile(atPath: bin.appendingPathComponent("pig").path, contents: Data("#!/bin/sh\n".utf8), attributes: [.posixPermissions: 0o755])

        XCTAssertFalse(AgentAdapter.pig.needsShim, "PiG loads its extension from its config folder, so its binary is not shadowed")
        let launcher = try XCTUnwrap(AgentLauncher(.pig, arguments: ["hello"], environment: ["PATH": bin.path], cde: "/cde"))
        XCTAssertEqual(launcher.arguments, ["pig", "hello"], "PiG launches with the arguments as typed")
    }
}

final class AgentLauncherTests: XCTestCase {
    var root: URL!

    override func setUpWithError() throws {
        root = FileManager.default.temporaryDirectory.appendingPathComponent("code-launch-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try FileManager.default.removeItem(at: root)
    }

    @discardableResult
    func makeExecutable(_ directory: String, _ name: String, mode: Int = 0o755) throws -> String {
        let dir = root.appendingPathComponent(directory)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let file = dir.appendingPathComponent(name).path
        FileManager.default.createFile(atPath: file, contents: Data("#!/bin/sh\n".utf8), attributes: [.posixPermissions: mode])
        return file
    }

    func path(_ directories: String...) -> String {
        directories.map { root.appendingPathComponent($0).path }.joined(separator: ":")
    }

    func testResolvesPastTheShimDirectory() throws {
        try makeExecutable("shims", "claude")
        try makeExecutable("notexec", "claude", mode: 0o644)
        try FileManager.default.createDirectory(at: root.appendingPathComponent("dir/claude"), withIntermediateDirectories: true)
        let real = try makeExecutable("bin", "claude")
        let shims = root.appendingPathComponent("shims").path

        XCTAssertEqual(AgentLauncher.resolve("claude", path: path("shims", "notexec", "dir", "bin"), excluding: shims + "/"), real)
        XCTAssertEqual(AgentLauncher.resolve("claude", path: path("shims", "bin")), shims + "/claude")
        XCTAssertNil(AgentLauncher.resolve("claude", path: path("shims"), excluding: shims))
        XCTAssertNil(AgentLauncher.resolve("claude", path: nil))
    }

    func testRewritesArgumentsAndDropsTheShimVariable() throws {
        let real = try makeExecutable("bin", "claude")
        let shims = root.appendingPathComponent("shims").path
        let environment = ["PATH": shims + ":" + path("bin"), "CODE_SHIM_DIR": shims, "HOME": "/Users/me"]

        let launcher = try XCTUnwrap(AgentLauncher(.claudeCode, arguments: ["-p", "hi"], environment: environment, cde: "/cde"))

        XCTAssertEqual(launcher.executable, real)
        XCTAssertEqual(launcher.arguments.first, "claude")
        XCTAssertEqual(launcher.arguments[1], "--settings")
        XCTAssertEqual(Array(launcher.arguments.suffix(2)), ["-p", "hi"])
        XCTAssertEqual(launcher.environment, ["PATH": environment["PATH"]!, "HOME": "/Users/me"])
    }

    func testOtherLaunchKinds() throws {
        let real = try makeExecutable("bin", "agent")
        let translate: (HookPayload) -> AgentEvent? = { _ in nil }
        let byEnvironment = AgentAdapter(name: "agent", binary: "agent", launch: .environment { environment, cde in
            ["AGENT_HOOKS": "\(environment["AGENT_HOOKS"] ?? "on") \(cde)"]
        }, translate: translate)
        let untouched = AgentAdapter(name: "agent", binary: "agent", launch: .none, translate: translate)

        XCTAssertEqual(AgentLauncher(byEnvironment, arguments: ["x"], environment: ["PATH": path("bin")], cde: "/cde"),
                       AgentLauncher(untouched, arguments: ["x"], environment: ["PATH": path("bin"), "AGENT_HOOKS": "on /cde"], cde: "/cde"))
        XCTAssertEqual(AgentLauncher(byEnvironment, arguments: [], environment: ["PATH": path("bin"), "AGENT_HOOKS": "off"], cde: "/cde")?.environment["AGENT_HOOKS"],
                       "off /cde")
        XCTAssertEqual(AgentLauncher(untouched, arguments: ["x"], environment: ["PATH": path("bin")], cde: "/cde")?.arguments, ["agent", "x"])
        XCTAssertEqual(AgentLauncher(untouched, arguments: [], environment: ["PATH": path("bin")], cde: "/cde")?.executable, real)
    }

    func testDisabledLaunchesAsTyped() throws {
        try makeExecutable("bin", "claude")
        let launcher = AgentLauncher(.claudeCode, arguments: ["-p", "hi"], environment: ["PATH": path("bin"), "CODE_AGENTS_DISABLED": "1"], cde: "/cde")
        XCTAssertEqual(launcher?.arguments, ["claude", "-p", "hi"])
    }

    func testMissingBinaryIsNil() {
        XCTAssertNil(AgentLauncher(.claudeCode, arguments: [], environment: ["PATH": path("bin")], cde: "/cde"))
    }

    // MARK: Shims

    func testShimsFollowTheBinariesOnPath() throws {
        let shims = root.appendingPathComponent("shims").path
        let unrelated = try makeExecutable("shims", "notes")
        try makeExecutable("bin", "claude")
        let cde = "/Applications/Code.app/Contents/Helpers/cde"

        try AgentShims.install(in: shims, path: shims + ":" + path("bin"), cde: cde)

        let shim = shims + "/claude"
        XCTAssertEqual(String(decoding: try Data(contentsOf: URL(fileURLWithPath: shim)), as: UTF8.self), """
            #!/bin/sh
            CODE_SHIM_DIR='\(shims)' exec '\(cde)' launch claude "$@"

            """)
        XCTAssertEqual(try FileManager.default.attributesOfItem(atPath: shim)[.posixPermissions] as? Int, 0o755)

        try AgentShims.install(in: shims, path: shims + ":" + path("bin"), cde: cde)
        XCTAssertTrue(FileManager.default.fileExists(atPath: shim))

        try FileManager.default.removeItem(at: root.appendingPathComponent("bin"))
        try AgentShims.install(in: shims, path: shims + ":" + path("bin"), cde: cde)
        XCTAssertFalse(FileManager.default.fileExists(atPath: shim))
        XCTAssertTrue(FileManager.default.fileExists(atPath: unrelated))
    }

    func testCreatesTheDirectoryPrivately() throws {
        let shims = root.appendingPathComponent("new/shims").path
        try AgentShims.install(in: shims, path: path("bin"), cde: "/cde")
        XCTAssertEqual(try FileManager.default.attributesOfItem(atPath: shims)[.posixPermissions] as? Int, 0o700)
    }

    func testShimsSkipAgentsThatReadTheirExtensionsFromAFolder() throws {
        let shims = root.appendingPathComponent("shims").path
        try makeExecutable("bin", "pi")
        try makeExecutable("bin", "pig")
        // Shims left by an older Code, from when Pi and PiG were shadowed, go.
        try makeExecutable("shims", "pi")
        try makeExecutable("shims", "pig")

        try AgentShims.install(in: shims, path: shims + ":" + path("bin"), cde: "/cde")

        XCTAssertFalse(FileManager.default.fileExists(atPath: shims + "/pi"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: shims + "/pig"))
    }

    func testRemovesTheShimsOfQuitCodesAndClosedTerminals() throws {
        let files = FileManager.default
        let own: pid_t = 4242, dead = deadPID(), running = getpid()
        let live = UUID(), closed = UUID()
        func make(_ path: String) throws {
            try files.createDirectory(at: root.appendingPathComponent("code-shims/" + path), withIntermediateDirectories: true)
        }
        for name in [live.uuidString, closed.uuidString, "not-a-terminal"] {
            try make("\(own)/\(name)")
        }
        try make("\(dead)/\(UUID().uuidString)")
        try make("\(running)/\(closed.uuidString)")
        try make(closed.uuidString)

        AgentShims.removeStale(temporaryDirectory: root.path, pid: own) { $0 == live }

        XCTAssertEqual(Set(try files.contentsOfDirectory(atPath: root.appendingPathComponent("code-shims").path)),
                       ["\(own)", "\(running)", closed.uuidString])
        XCTAssertEqual(try files.contentsOfDirectory(atPath: AgentShims.root(pid: own, temporaryDirectory: root.path)), [live.uuidString])
        XCTAssertEqual(try files.contentsOfDirectory(atPath: AgentShims.root(pid: running, temporaryDirectory: root.path)), [closed.uuidString])
    }
}

final class HookSummaryTests: XCTestCase {
    func testSummaryMatchesWhitespaceAndUnicodeBoundaries() throws {
        for message in ["  one\n\t two  ", String(repeating: "é", count: 200),
                        String(repeating: "👩🏽‍💻 ", count: 10000), String(repeating: "a", count: 199) + " b"] {
            let input = try JSONSerialization.data(withJSONObject: ["message": message])
            let payload = try XCTUnwrap(HookPayload(json: input, agent: "test", environment: [:], now: Date()))
            let normalized = message.split(whereSeparator: \.isWhitespace).joined(separator: " ")
            let expected = normalized.count > HookPayload.detailLimit
                ? String(normalized.prefix(HookPayload.detailLimit - 1)) + "…" : normalized
            XCTAssertEqual(payload.summary("message"), expected)
        }
    }
}
