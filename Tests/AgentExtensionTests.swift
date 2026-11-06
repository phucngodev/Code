import XCTest

final class AgentExtensionTests: XCTestCase {
    private var root: URL!

    override func setUpWithError() throws {
        root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: root)
    }

    private func makeSource(_ contents: String) throws -> URL {
        let url = root.appendingPathComponent("bundled.ts")
        try contents.write(to: url, atomically: true, encoding: .utf8)
        return url
    }

    private func extensionInstalling(_ source: URL) -> AgentExtension {
        AgentExtension(agent: "Pi", directory: root.appendingPathComponent("extensions"), name: "code.ts", source: source)
    }

    func testInstallCopiesTheExtensionAndReportsItsState() throws {
        let ext = extensionInstalling(try makeSource("code"))
        XCTAssertEqual(ext.status(), .missing)

        XCTAssertTrue(try ext.install())
        XCTAssertEqual(ext.status(), .installed)
        XCTAssertEqual(try String(contentsOf: ext.destination, encoding: .utf8), "code")

        XCTAssertFalse(try ext.install(), "the extension is already up to date")

        try "newer".write(to: ext.source, atomically: true, encoding: .utf8)
        XCTAssertEqual(ext.status(), .outdated)
        XCTAssertTrue(try ext.install())
        XCTAssertEqual(ext.status(), .installed)
    }

    func testMissingBundledExtensionThrows() {
        let ext = extensionInstalling(root.appendingPathComponent("absent.ts"))
        XCTAssertThrowsError(try ext.install())
        XCTAssertEqual(ext.status(), .missing)
    }

    func testPiUsesTheAgentDirectoryOverride() {
        let resources = root.appendingPathComponent("Resources")

        let custom = AgentExtensions.pi(home: root, resources: resources, environment: ["PI_CODING_AGENT_DIR": "/custom/agent"])
        XCTAssertEqual(custom?.directory.path, "/custom/agent/extensions")
        XCTAssertEqual(custom?.name, "code.ts")
        XCTAssertEqual(custom?.source.path, resources.appendingPathComponent("agents/pi/code.ts").path)

        let byDefault = AgentExtensions.pi(home: root, resources: resources, environment: [:])
        XCTAssertEqual(byDefault?.directory.path, root.appendingPathComponent(".pi/agent/extensions").path)
    }
}
