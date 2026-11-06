import XCTest

final class HistoryStoreTests: XCTestCase {
    private var directory: URL!

    override func setUpWithError() throws {
        directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: directory)
    }

    private func write(_ lines: [String], to name: String, agent: String = "claude", modified: Date? = nil) throws -> URL {
        let url = directory.appendingPathComponent(name)
        try lines.joined(separator: "\n").write(to: url, atomically: true, encoding: .utf8)
        if let modified {
            try FileManager.default.setAttributes([.modificationDate: modified], ofItemAtPath: url.path)
        }
        return url
    }

    func testParseReadsTheWorkingDirectoryAndFirstPrompt() {
        let data = Data("""
        {"type":"user","cwd":"/Users/me/project","message":{"role":"user","content":"Fix the failing test"}}
        {"type":"assistant","cwd":"/Users/me/project","message":{"role":"assistant","content":"On it"}}
        """.utf8)

        let parsed = HistoryStore.parse(data)

        XCTAssertEqual(parsed.cwd, "/Users/me/project")
        XCTAssertEqual(parsed.title, "Fix the failing test")
    }

    func testParseCollapsesWhitespaceAndCapsTheTitle() {
        let long = String(repeating: "a", count: 400)
        let data = Data("{\"type\":\"user\",\"cwd\":\"/tmp\",\"message\":{\"role\":\"user\",\"content\":\"line one\\n\\nline two \(long)\"}}".utf8)

        let parsed = HistoryStore.parse(data)

        XCTAssertEqual(parsed.title.prefix(11), "line one li")
        XCTAssertLessThanOrEqual(parsed.title.count, 120)
        XCTAssertTrue(parsed.title.hasSuffix("…"))
    }

    func testParseReadsContentParts() {
        let data = Data("{\"type\":\"user\",\"cwd\":\"/tmp\",\"message\":{\"role\":\"user\",\"content\":[{\"type\":\"text\",\"text\":\"Hello there\"}]}}".utf8)
        XCTAssertEqual(HistoryStore.parse(data).title, "Hello there")
    }

    func testParseReadsASummaryLine() {
        let data = Data("{\"type\":\"summary\",\"cwd\":\"/tmp\",\"summary\":\"Refactored the parser\"}".utf8)
        XCTAssertEqual(HistoryStore.parse(data).title, "Refactored the parser")
    }

    func testParseToleratesMalformedLinesAndNoDirectory() {
        let data = Data("not json\n{\"type\":\"user\",\"message\":{\"role\":\"user\",\"content\":\"hi\"}}".utf8)
        let parsed = HistoryStore.parse(data)
        XCTAssertNil(parsed.cwd)
        XCTAssertEqual(parsed.title, "hi")
    }

    func testParseReadsAPiSessionHeaderAndPrompt() {
        let data = Data("""
        {"type":"session","version":3,"id":"01a1aead-e504-7477-8866-791346960bd4","timestamp":"2026-11-06T02:26:34.373Z","cwd":"/Users/phuc/Documents/Codes/Code"}
        {"type":"model_change","id":"391bceef","provider":"magpie"}
        {"type":"message","id":"e95ff3c1","message":{"role":"user","content":[{"type":"text","text":"hello there"}]}}
        """.utf8)

        let parsed = HistoryStore.parse(data)

        XCTAssertEqual(parsed.cwd, "/Users/phuc/Documents/Codes/Code")
        XCTAssertEqual(parsed.title, "hello there")
        XCTAssertEqual(parsed.id, "01a1aead-e504-7477-8866-791346960bd4")
    }

    func testReadBuildsASessionFromTheFile() throws {
        let url = try write([
            #"{"type":"user","cwd":"/Users/me/project","message":{"role":"user","content":"Add the sidebar"}}"#,
        ], to: "abc-123.jsonl")

        let session = try XCTUnwrap(HistoryStore.read(url, agent: "claude"))

        XCTAssertEqual(session.id, "abc-123")
        XCTAssertEqual(session.agent, "claude")
        XCTAssertEqual(session.cwd, "/Users/me/project")
        XCTAssertEqual(session.title, "Add the sidebar")
        XCTAssertEqual(session.resume, .init(binary: "claude", arguments: ["--resume", "abc-123"]))
    }

    func testReadSkipsATranscriptWithoutADirectory() throws {
        let url = try write([#"{"type":"user","message":{"role":"user","content":"hi"}}"#], to: "no-cwd.jsonl")
        XCTAssertNil(HistoryStore.read(url, agent: "claude"))
    }

    func testScanFindsTranscriptsNewestFirst() throws {
        let old = Date(timeIntervalSince1970: 1_000)
        let new = Date(timeIntervalSince1970: 2_000)
        try write([#"{"cwd":"/old","type":"user","message":{"content":"old"}}"#], to: "old.jsonl", modified: old)
        try write([#"{"cwd":"/new","type":"user","message":{"content":"new"}}"#], to: "new.jsonl", modified: new)
        try write(["ignored"], to: "notes.txt", modified: new)

        let sessions = HistoryStore.scan(roots: [.init(url: directory, agent: "claude")])

        XCTAssertEqual(sessions.map(\.id), ["new", "old"])
    }

    func testScanSkipsMissingRoots() {
        XCTAssertTrue(HistoryStore.scan(roots: [.init(url: directory.appendingPathComponent("nope"), agent: "claude")]).isEmpty)
    }

    func testFilterMatchesTitleOrDirectoryCaseInsensitively() {
        let sessions = [
            HistorySession(id: "a", agent: "claude", cwd: "/Users/me/api", title: "Fix login", modified: .distantPast, resume: nil),
            HistorySession(id: "b", agent: "claude", cwd: "/Users/me/web", title: "Style the header", modified: .distantPast, resume: nil),
        ]

        XCTAssertEqual(HistoryStore.filter(sessions, query: "LOGIN").map(\.id), ["a"])
        XCTAssertEqual(HistoryStore.filter(sessions, query: "me/web").map(\.id), ["b"])
        XCTAssertEqual(HistoryStore.filter(sessions, query: "   ").count, 2)
        XCTAssertTrue(HistoryStore.filter(sessions, query: "nothing").isEmpty)
    }

    func testResumeOnlyForAgentsCodeKnows() {
        XCTAssertEqual(HistoryStore.resume(agent: "claude", id: "abc-123"), .init(binary: "claude", arguments: ["--resume", "abc-123"]))
        XCTAssertEqual(HistoryStore.resume(agent: "pi", id: "01a1aead-e504"), .init(binary: "pi", arguments: ["--session", "01a1aead-e504"]))
        XCTAssertNil(HistoryStore.resume(agent: "codex", id: "abc"))
        XCTAssertNil(HistoryStore.resume(agent: "opencode", id: "abc"))
        XCTAssertNil(HistoryStore.resume(agent: "claude", id: "bad id; rm"))
    }

    func testReadNamesAPiSessionByItsOwnId() throws {
        let url = try write([
            #"{"type":"session","id":"01a1aead-e504-7477-8866-791346960bd4","cwd":"/Users/phuc/project"}"#,
            #"{"type":"message","message":{"role":"user","content":[{"type":"text","text":"hello"}]}}"#,
        ], to: "2026-11-06T02-26-34-373Z_01a1aead-e504-7477-8866-791346960bd4.jsonl", agent: "pi")

        let session = try XCTUnwrap(HistoryStore.read(url, agent: "pi"))

        XCTAssertEqual(session.id, "01a1aead-e504-7477-8866-791346960bd4")
        XCTAssertEqual(session.resume, .init(binary: "pi", arguments: ["--session", "01a1aead-e504-7477-8866-791346960bd4"]))
        XCTAssertEqual(session.title, "hello")
    }
}
