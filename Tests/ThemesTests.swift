import XCTest

final class ThemesTests: XCTestCase {
    private var directory: URL!

    override func setUpWithError() throws {
        directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try FileManager.default.removeItem(at: directory)
    }

    @discardableResult
    private func theme(_ name: String, in directory: URL) throws -> URL {
        let file = directory.appendingPathComponent(name)
        try "background = 000000\n".write(to: file, atomically: true, encoding: .utf8)
        return file
    }

    func testFindsAThemeInTheFirstDirectory() throws {
        let other = directory.appendingPathComponent("other")
        try FileManager.default.createDirectory(at: other, withIntermediateDirectories: true)
        let mine = try theme("Mine", in: directory)
        try theme("Mine", in: other)
        XCTAssertEqual(Themes.file(named: "Mine", in: [directory, other])?.path, mine.path)
    }

    func testFindsAPathAsGiven() {
        XCTAssertEqual(Themes.file(named: "/tmp/Mine", in: [])?.path, "/tmp/Mine")
    }

    func testResolvesAThemeNameToItsFile() throws {
        let file = try theme("Mine", in: directory)
        XCTAssertEqual(Themes.resolved("Mine", in: [directory]), file.path)
    }

    func testKeepsTheLightAndDarkLabels() throws {
        let light = try theme("Day", in: directory)
        let dark = try theme("Night", in: directory)
        XCTAssertEqual(Themes.resolved("light:Day,dark:Night", in: [directory]), "light:\(light.path),dark:\(dark.path)")
    }

    func testLeavesAThemeCodeCannotFindAlone() {
        XCTAssertNil(Themes.resolved("Missing", in: [directory]))
    }

    func testLeavesAnAbsolutePathAlone() {
        XCTAssertNil(Themes.resolved("/tmp/Mine", in: [directory]))
    }

    func testOverrideResolvesTheConfigsTheme() throws {
        try theme("Mine", in: directory)
        let config = directory.appendingPathComponent("config")
        try "theme = Mine\n".write(to: config, atomically: true, encoding: .utf8)

        let override = try XCTUnwrap(ConfigFile.themeOverride(for: config, in: [directory]))
        defer { try? FileManager.default.removeItem(at: override) }
        let text = try String(contentsOf: override, encoding: .utf8)
        XCTAssertEqual(ConfigDocument(text)["theme"], directory.appendingPathComponent("Mine").path)
    }

    func testOverrideTakesAThemeFromAnInclude() throws {
        try theme("Mine", in: directory)
        let included = directory.appendingPathComponent("extra")
        try "theme = Mine\n".write(to: included, atomically: true, encoding: .utf8)
        let config = directory.appendingPathComponent("config")
        try "config-file = extra\n".write(to: config, atomically: true, encoding: .utf8)

        let override = try XCTUnwrap(ConfigFile.themeOverride(for: config, in: [directory]))
        defer { try? FileManager.default.removeItem(at: override) }
        let text = try String(contentsOf: override, encoding: .utf8)
        XCTAssertEqual(ConfigDocument(text)["theme"], directory.appendingPathComponent("Mine").path)
    }

    func testNoOverrideWithoutAThemeCodeCanFind() throws {
        let config = directory.appendingPathComponent("config")
        try "font-size = 14\n".write(to: config, atomically: true, encoding: .utf8)
        XCTAssertNil(ConfigFile.themeOverride(for: config, in: [directory]))
    }

    func testUsesCodeDefaultThemeWhenTheConfigNamesNone() throws {
        let file = try theme(Themes.defaultTheme, in: directory)
        let config = directory.appendingPathComponent("config")
        try "font-size = 14\n".write(to: config, atomically: true, encoding: .utf8)

        let override = try XCTUnwrap(ConfigFile.themeOverride(for: config, in: [directory]))
        defer { try? FileManager.default.removeItem(at: override) }
        XCTAssertEqual(ConfigDocument(try String(contentsOf: override, encoding: .utf8))["theme"], file.path)
    }

    func testAnEmptyThemeAsksForGhosttyColors() throws {
        try theme(Themes.defaultTheme, in: directory)
        let config = directory.appendingPathComponent("config")
        try "theme = \"\"\n".write(to: config, atomically: true, encoding: .utf8)
        XCTAssertNil(ConfigFile.themeOverride(for: config, in: [directory]),
                     "an empty theme leaves the colors to Ghostty, even with a default")
    }

    func testNamesIgnoreCaseSoTheFirstDirectoryWins() throws {
        let other = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: other, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: other) }
        try theme("nord", in: directory)
        try theme("Nord", in: other)
        XCTAssertEqual(Themes.names(in: [directory, other]), ["nord"])
    }
}
