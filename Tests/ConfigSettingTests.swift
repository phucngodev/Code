import AppKit
import GhosttyKit
import XCTest

final class ConfigSettingTests: XCTestCase {
    private static let initialized = ghostty_init(UInt(CommandLine.argc), CommandLine.unsafeArgv)

    /// An option the file leaves out should show what Ghostty actually uses.
    func testDefaultsMatchGhostty() throws {
        XCTAssertEqual(Self.initialized, GHOSTTY_SUCCESS)
        let config = try GhosttyConfig(file: nil)
        func get<T>(_ key: String, _ value: T) -> T {
            var result = value
            XCTAssertTrue(key.withCString { ghostty_config_get(config.cValue, &result, $0, UInt(key.utf8.count)) }, key)
            return result
        }
        func string(_ key: String) -> String {
            String(cString: get(key, UnsafePointer<CChar>?.none)!)
        }
        XCTAssertEqual(Double(get(Option.fontSize.key, Float(0))), Option.fontSize.defaultValue)
        XCTAssertEqual(get(Option.fontThicken.key, true), Option.fontThicken.defaultValue)
        XCTAssertEqual(string(Option.cursorStyle.key), Option.cursorStyle.defaultValue)
        XCTAssertEqual(get(Option.backgroundOpacity.key, Double(0)), Option.backgroundOpacity.defaultValue)
        XCTAssertEqual(get(Option.unfocusedSplitOpacity.key, Double(0)), Option.unfocusedSplitOpacity.defaultValue)
        XCTAssertEqual(string(Option.scrollbar.key), Option.scrollbar.defaultValue)
        XCTAssertEqual(string(Option.confirmClose.key), Option.confirmClose.defaultValue)
        XCTAssertEqual(string(Option.copyOnSelect.key), Option.copyOnSelect.defaultValue)
        XCTAssertEqual(get(Option.mouseHideWhileTyping.key, true), Option.mouseHideWhileTyping.defaultValue)
        XCTAssertEqual(get(Option.shellIntegrationFeatures.key, CUnsignedInt(0)), 0b100101, "cursor, title and path")
        XCTAssertEqual(get(Option.autoSecureInput.key, false), Option.autoSecureInput.defaultValue)
        XCTAssertEqual(config.notifyOnCommandFinish, Option.notifyOnCommandFinish.defaultValue)
        XCTAssertEqual(config.notifyOnCommandFinishAction, Option.notifyOnCommandFinishAction.defaultValue)
        XCTAssertEqual(config.notifyOnCommandFinishAfter, Option.notifyOnCommandFinishAfter.defaultValue)
    }

    func testNumbersUseADecimalPointInEveryLocale() {
        XCTAssertEqual(Option.backgroundOpacity.format(0.85), "0.85")
        XCTAssertEqual(Option.fontSize.format(13.5), "13.5")
        XCTAssertEqual(Option.fontSize.format(14), "14")
    }

    func testDurationsReadAndWriteLikeGhostty() {
        XCTAssertEqual(ConfigDuration.parse("5s"), .seconds(5))
        XCTAssertEqual(ConfigDuration.parse("1m 30s"), .seconds(90))
        XCTAssertEqual(ConfigDuration.parse("1h"), .seconds(3600))
        XCTAssertEqual(ConfigDuration.parse("500ms"), .milliseconds(500))
        XCTAssertEqual(ConfigDuration.parse("1d 2h"), .seconds(93600))
        XCTAssertEqual(ConfigDuration.parse("250us"), .microseconds(250))
        XCTAssertNil(ConfigDuration.parse("soon"))
        XCTAssertEqual(ConfigDuration.parse("0"), .zero)
        XCTAssertEqual(ConfigDuration.format(.seconds(90)), "90s")
        XCTAssertEqual(ConfigDuration.format(.seconds(120)), "2m")
        XCTAssertEqual(ConfigDuration.format(.milliseconds(500)), "500ms")
        XCTAssertEqual(ConfigDuration.format(.milliseconds(1500)), "1500ms")
    }

    func testBlurIsOnForAnyRadius() {
        XCTAssertEqual(Option.backgroundBlur.parse("true"), true)
        XCTAssertEqual(Option.backgroundBlur.parse("f"), false)
        XCTAssertEqual(Option.backgroundBlur.parse("30"), true)
        XCTAssertEqual(Option.backgroundBlur.parse("0"), false)
        XCTAssertNil(Option.backgroundBlur.parse("macos-glass-regular"))
        XCTAssertEqual(Option.backgroundBlur.format(true), "true")
    }

    func testChoicesReadGhosttysOtherSpellings() {
        XCTAssertEqual(Option.confirmClose.parse("1"), "true")
        XCTAssertEqual(Option.confirmClose.parse("F"), "false")
        XCTAssertEqual(Option.confirmClose.parse("always"), "always")
        XCTAssertEqual(Option.copyOnSelect.parse("clipboard"), "clipboard")
        XCTAssertNil(Option.cursorStyle.parse("beam"))
    }

    func testPathsReadLikeGhostty() {
        XCTAssertEqual(ConfigPath("?\"x y\"").map { [$0.path, String($0.optional)] }, ["x y", "true"])
        XCTAssertEqual(ConfigPath("?x").map { [$0.path, String($0.optional)] }, ["x", "true"])
        XCTAssertNil(ConfigPath("?"))
        XCTAssertNil(ConfigPath(""))
        XCTAssertEqual(ConfigPath("~/a")?.resolved(from: URL(fileURLWithPath: "/tmp")).path, NSHomeDirectory() + "/a")
        XCTAssertEqual(ConfigPath("a")?.resolved(from: URL(fileURLWithPath: "/tmp")).path, "/tmp/a")
    }

    func testTheFontIsTheFirstActiveFamilyThePickerOffers() {
        let choices: Set = ["Menlo", "Monaco"]
        XCTAssertEqual(FontFamilies.font(in: ["Menlo", "Apple Color Emoji"], choices: choices), "Menlo")
        XCTAssertNil(FontFamilies.font(in: ["Apple Color Emoji"], choices: choices))
        XCTAssertEqual(FontFamilies.font(in: ["Monaco", "", "Menlo"], choices: choices), "Menlo")

        let toDefault = FontFamilies.setting(nil, in: ["Menlo", "Apple Color Emoji"], choices: choices)
        XCTAssertEqual(toDefault, ["Apple Color Emoji"])
        XCTAssertEqual(FontFamilies.setting("Menlo", in: toDefault, choices: choices), ["Menlo", "Apple Color Emoji"])
        XCTAssertEqual(FontFamilies.setting("Monaco", in: ["Monaco", "", "Menlo"], choices: choices), ["Monaco", "", "Monaco"])
        XCTAssertEqual(FontFamilies.setting("Menlo", in: [], choices: choices), ["Menlo"])
    }

    func testBooleansTakeGhosttysSpellings() {
        XCTAssertEqual(Option.fontThicken.parse("t"), true)
        XCTAssertEqual(Option.fontThicken.parse("1"), true)
        XCTAssertEqual(Option.fontThicken.parse("F"), false)
        XCTAssertNil(Option.fontThicken.parse("yes"))
    }

    func testCommandFinishActionsStartFromTheBell() {
        XCTAssertEqual(CommandFinish.Actions(config: "notify"), [.bell, .notify])
        XCTAssertEqual(CommandFinish.Actions(config: "no-bell,notify"), [.notify])
        XCTAssertEqual(CommandFinish.Actions(config: "false"), [])
        XCTAssertNil(CommandFinish.Actions(config: "shout"))
        XCTAssertEqual(CommandFinish.Actions([.notify]).config, "no-bell,notify")
    }

    func testShellIntegrationFeaturesKeepOtherSwitches() {
        let setting = Option.shellIntegrationFeatures
        var features = try! XCTUnwrap(setting.parse("sudo,no-title"))
        XCTAssertEqual(features.on, ["cursor", "sudo", "path"])
        features.on.remove("cursor")
        XCTAssertEqual(setting.format(features), "no-cursor,sudo,no-title")
        XCTAssertEqual(setting.parse("false")?.on, [])
        XCTAssertNil(setting.parse("teleport"))
    }

    func testThemeIsOneThemeOrALightAndDarkPair() {
        XCTAssertEqual(ThemeChoice("Nord"), ThemeChoice(light: "Nord", dark: "Nord"))
        XCTAssertEqual(ThemeChoice("light:Builtin Light, dark:Nord"), ThemeChoice(light: "Builtin Light", dark: "Nord"))
        XCTAssertEqual(ThemeChoice(nil), ThemeChoice(light: nil, dark: nil))
    }

    /// A theme that names no `split-divider-color` gets the system separator
    /// colour, which reads on both a light and a dark theme. A named color wins.
    func testSplitDividerUsesTheDeclaredColorOrTheSeparator() throws {
        XCTAssertEqual(Self.initialized, GHOSTTY_SUCCESS)

        func divider(background: String, declared: String? = nil) throws -> NSColor {
            let url = FileManager.default.temporaryDirectory.appendingPathComponent("code-divider-\(UUID().uuidString)")
            defer { try? FileManager.default.removeItem(at: url) }
            let lines = ["background = \(background)"] + (declared.map { ["split-divider-color = \($0)"] } ?? [])
            try (lines + [""]).joined(separator: "\n").write(to: url, atomically: true, encoding: .utf8)
            let config = try GhosttyConfig(file: url)
            return config.splitDividerColor
        }

        XCTAssertEqual(try divider(background: "#ffffff"), .separatorColor)
        XCTAssertEqual(try divider(background: "#101010"), .separatorColor)

        let declared = try divider(background: "#ffffff", declared: "#ff0000")
        let rgb = try XCTUnwrap(declared.usingColorSpace(.sRGB))
        XCTAssertEqual(rgb.redComponent, 1, accuracy: 0.001)
        XCTAssertEqual(rgb.greenComponent, 0, accuracy: 0.001)
    }
}
