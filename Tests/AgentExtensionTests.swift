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

    func testInstallCopiesTheExtension() throws {
        let ext = extensionInstalling(try makeSource("code"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: ext.destination.path))

        XCTAssertTrue(try ext.install(), "the first install writes the file")
        XCTAssertEqual(try String(contentsOf: ext.destination, encoding: .utf8), "code")

        XCTAssertFalse(try ext.install(), "the extension is already up to date")

        try "newer".write(to: ext.source, atomically: true, encoding: .utf8)
        XCTAssertTrue(try ext.install(), "a changed extension is written again")
        XCTAssertEqual(try String(contentsOf: ext.destination, encoding: .utf8), "newer")
    }

    func testMissingBundledExtensionThrows() {
        let ext = extensionInstalling(root.appendingPathComponent("absent.ts"))
        XCTAssertThrowsError(try ext.install())
        XCTAssertFalse(FileManager.default.fileExists(atPath: ext.destination.path))
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

    func testPigUsesTheConfigFolder() {
        let resources = root.appendingPathComponent("Resources")

        let ext = AgentExtensions.pig(home: root, resources: resources)

        XCTAssertEqual(ext?.directory.path, root.appendingPathComponent(".config/pig/agent/extensions").path)
        XCTAssertEqual(ext?.name, "code")
        XCTAssertEqual(ext?.source.path, resources.appendingPathComponent("agents/pig/code").path)
    }

    func testInstallCopiesAFolderExtension() throws {
        let source = root.appendingPathComponent("bundled")
        try FileManager.default.createDirectory(at: source, withIntermediateDirectories: true)
        try "module code".write(to: source.appendingPathComponent("go.mod"), atomically: true, encoding: .utf8)
        try "package code".write(to: source.appendingPathComponent("extension.go"), atomically: true, encoding: .utf8)
        let ext = AgentExtension(agent: "PiG", directory: root.appendingPathComponent("extensions"),
                                 name: "code", source: source)

        XCTAssertTrue(try ext.install(), "the first install writes the folder")
        XCTAssertEqual(try String(contentsOf: ext.destination.appendingPathComponent("extension.go"), encoding: .utf8), "package code")
        XCTAssertFalse(try ext.install(), "the folder is already up to date")

        try "package code // newer".write(to: source.appendingPathComponent("extension.go"), atomically: true, encoding: .utf8)
        XCTAssertTrue(try ext.install(), "a changed file is written again")
        XCTAssertEqual(try String(contentsOf: ext.destination.appendingPathComponent("extension.go"), encoding: .utf8), "package code // newer")
    }

    func testInstallAllCopiesEveryExtension() throws {
        let ext = extensionInstalling(try makeSource("code"))

        AgentExtensions.installAll(extensions: [ext])

        XCTAssertEqual(try String(contentsOf: ext.destination, encoding: .utf8), "code")
    }

    func testInstallAllSkipsAnAgentWhoseConfigFolderIsMissing() throws {
        let missing = root.appendingPathComponent("absent")
        let ext = AgentExtension(agent: "Pi", directory: missing.appendingPathComponent("extensions"),
                                 name: "code.ts", source: try makeSource("code"))

        AgentExtensions.installAll(extensions: [ext])

        XCTAssertFalse(FileManager.default.fileExists(atPath: missing.path), "nothing is made for an agent that is not set up")
    }
}
