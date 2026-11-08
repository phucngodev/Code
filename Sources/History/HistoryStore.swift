import Foundation

/// One agent session on disk that Code can offer to reopen. Code only ever
/// reads the agents' transcripts, never writes them.
struct HistorySession: Equatable, Identifiable {
    /// How to resume a session in the agent's own CLI.
    struct Resume: Equatable {
        let binary: String
        let arguments: [String]
    }

    /// The agent's own id for the session, unique within its directory.
    let id: String
    /// The agent that wrote it, such as "claude".
    let agent: String
    /// Where the session ran, so a new terminal can start there.
    let cwd: String
    /// The first thing the user asked, or the agent's own summary.
    let title: String
    let modified: Date
    /// What resumes it, or nil when Code has none for the agent.
    let resume: Resume?

    /// The title, or the directory when the transcript held no prompt.
    var displayTitle: String {
        title.isEmpty ? cwd.abbreviatingHome : title
    }
}

/// Finds the sessions agents kept on disk, newest first. A scan reads only the
/// head of each transcript, so it never walks a whole conversation, and it
/// keeps nothing between scans.
enum HistoryStore {
    /// Enough of a transcript to hold the first prompt and the working
    /// directory, without reading a long conversation to the end.
    static let headBytes = 262_144
    static let defaultLimit = 200

    /// One place transcripts live, and the agent that wrote them.
    struct Root {
        var url: URL
        var agent: String
    }

    static func defaultRoots(home: URL = FileManager.default.homeDirectoryForCurrentUser) -> [Root] {
        [
            Root(url: home.appendingPathComponent(".claude/projects"), agent: "claude"),
            Root(url: home.appendingPathComponent(".codex/sessions"), agent: "codex"),
            Root(url: home.appendingPathComponent(".pi/agent/sessions"), agent: "pi"),
            Root(url: home.appendingPathComponent(".config/pig/agent/sessions"), agent: "pig"),
        ]
    }

    /// Every transcript under the roots, newest first, capped at `limit`.
    /// Unreadable files are skipped. The result is kept briefly, since the
    /// sidebar rescans whenever it is shown and history is a convenience.
    static func scan(roots: [Root], limit: Int = defaultLimit) -> [HistorySession] {
        let key = roots.map { "\($0.agent):\($0.url.path)" }.joined(separator: "|") + "|\(limit)"
        cacheLock.lock()
        let cached = cache.flatMap { $0.key == key && Date().timeIntervalSince($0.date) < cacheLifetime ? $0.sessions : nil }
        cacheLock.unlock()
        if let cached { return cached }

        let sessions = scanUncached(roots: roots, limit: limit)
        cacheLock.lock()
        cache = (key, Date(), sessions)
        cacheLock.unlock()
        return sessions
    }

    private static let cacheLock = NSLock()
    private static var cache: (key: String, date: Date, sessions: [HistorySession])?
    private static let cacheLifetime: TimeInterval = 30

    private static func scanUncached(roots: [Root], limit: Int) -> [HistorySession] {
        let fileManager = FileManager.default
        var sessions: [HistorySession] = []
        for root in roots {
            guard let enumerator = fileManager.enumerator(at: root.url, includingPropertiesForKeys: nil,
                                                          options: [.skipsHiddenFiles]) else { continue }
            for case let url as URL in enumerator where url.pathExtension == "jsonl" {
                guard let session = read(url, agent: root.agent) else { continue }
                sessions.append(session)
            }
        }
        return Array(sessions.sorted { $0.modified > $1.modified }.prefix(limit))
    }

    /// One transcript's session, or nil when it holds no working directory.
    static func read(_ url: URL, agent: String) -> HistorySession? {
        guard let handle = try? FileHandle(forReadingFrom: url) else { return nil }
        defer { try? handle.close() }
        let data = (try? handle.read(upToCount: headBytes)) ?? Data()
        let modified = (try? url.resourceValues(forKeys: [.contentModificationDateKey]).contentModificationDate) ?? .distantPast
        let parsed = parse(data)
        guard let cwd = parsed.cwd, !cwd.isEmpty else { return nil }
        // Agents that name the session inside the file (pi) use that; the rest
        // are identified by the file name.
        let id = parsed.id ?? url.deletingPathExtension().lastPathComponent
        return HistorySession(id: id, agent: agent, cwd: cwd, title: parsed.title,
                              modified: modified, resume: resume(agent: agent, id: id))
    }

    /// The working directory, first prompt and session id from a transcript's
    /// head. A transcript is one JSON object per line; a partial last line,
    /// which only the head cut off, is ignored.
    static func parse(_ data: Data) -> (cwd: String?, title: String, id: String?) {
        var cwd: String?
        var title = ""
        var id: String?
        for line in data.split(separator: 0x0A) {
            guard let object = (try? JSONSerialization.jsonObject(with: Data(line))) as? [String: Any] else { continue }
            if id == nil, object["type"] as? String == "session", let value = object["id"] as? String, !value.isEmpty { id = value }
            if cwd == nil, let value = object["cwd"] as? String, !value.isEmpty { cwd = value }
            if title.isEmpty { title = prompt(in: object) ?? "" }
            if cwd != nil, !title.isEmpty { break }
        }
        return (cwd, title, id)
    }

    /// The invocation that resumes a session, in the agent's own CLI. Only
    /// agents whose syntax Code knows return one.
    static func resume(agent: String, id: String) -> HistorySession.Resume? {
        // Only ids shaped like the ones the agents use, so a stray file name
        // can never reach a shell.
        guard !id.isEmpty, id.allSatisfy({ $0.isLetter || $0.isNumber || $0 == "-" || $0 == "_" }) else { return nil }
        switch agent {
        case "claude": return .init(binary: "claude", arguments: ["--resume", id])
        case "pi": return .init(binary: "pi", arguments: ["--session", id])
        case "pig": return .init(binary: "pig", arguments: ["--session", id])
        default: return nil
        }
    }

    /// The sessions whose title or directory contains `query`, or all of them
    /// for an empty query.
    static func filter(_ sessions: [HistorySession], query: String) -> [HistorySession] {
        let trimmed = query.trimmingCharacters(in: .whitespaces)
        guard !trimmed.isEmpty else { return sessions }
        return sessions.filter {
            $0.title.localizedCaseInsensitiveContains(trimmed) || $0.cwd.localizedCaseInsensitiveContains(trimmed)
        }
    }

    /// The first user prompt in one transcript line, across the shapes the
    /// agents use, or nil when the line holds none.
    private static func prompt(in object: [String: Any]) -> String? {
        if let summary = object["summary"] as? String, let text = oneLine(summary) { return text }
        if let payload = object["payload"] as? [String: Any] {
            if let text = messageText(payload["content"]) { return text }
            if let message = payload["message"] as? [String: Any] { return messageText(message["content"]) }
        }
        if let message = object["message"] as? [String: Any] {
            switch object["type"] as? String {
            case "user":
                return messageText(message["content"])
            case "message" where message["role"] as? String == "user":
                return messageText(message["content"])
            default:
                break
            }
        }
        if object["role"] as? String == "user" {
            return messageText(object["content"])
        }
        return nil
    }

    /// A message's text whether it is a plain string or the parts array the
    /// agents also use.
    private static func messageText(_ content: Any?) -> String? {
        if let text = content as? String { return oneLine(text) }
        if let parts = content as? [[String: Any]] {
            for part in parts {
                if let text = part["text"] as? String, let line = oneLine(text) { return line }
            }
        }
        return nil
    }

    /// `text` as one line, capped so a pasted prompt stays a row's worth.
    private static func oneLine(_ text: String) -> String? {
        let collapsed = text.split(whereSeparator: \.isWhitespace).joined(separator: " ")
        guard !collapsed.isEmpty else { return nil }
        return collapsed.count > 120 ? String(collapsed.prefix(119)) + "…" : collapsed
    }
}
