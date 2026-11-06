import AppKit

/// The `theme` option: one theme, or one for light mode and one for dark.
/// Nil is Ghostty's own colors.
struct ThemeChoice: Equatable {
    var light: String?
    var dark: String?

    init(light: String?, dark: String?) {
        self.light = light
        self.dark = dark
    }

    init(_ value: String?) {
        guard let value, !value.isEmpty else { self.init(light: nil, dark: nil); return }
        guard value.contains(where: { ",:=".contains($0) }) else { self.init(light: value, dark: value); return }
        var light: String?, dark: String?
        for part in value.split(separator: ",") {
            let pair = part.split(separator: ":", maxSplits: 1).map { $0.trimmingCharacters(in: .whitespaces) }
            guard pair.count == 2 else { continue }
            if pair[0] == "light" { light = pair[1] }
            if pair[0] == "dark" { dark = pair[1] }
        }
        self.init(light: light, dark: dark)
    }

}

enum Themes {
    /// The directories themes are found in, in lookup order: the user's own,
    /// then the ones Code ships.
    static var directories: [URL] {
        [ConfigFile.themesDirectory,
         Bundle.main.resourceURL?.appendingPathComponent("code-themes")].compactMap { $0 }
    }

    /// The theme Code uses when the config does not name one. It is one of
    /// the themes in the bundled `code-themes`, so the picker lists it too.
    static let defaultTheme = "modus"

    /// The themes found by name, in the order a name is looked up. Names are
    /// matched case-insensitively, so the first directory holding one decides
    /// how it is spelled and shown: on a case-insensitive disk a user theme and
    /// a bundled one of the same name cannot both be offered, and the user's
    /// wins.
    static func names(in directories: [URL]) -> [String] {
        var seen = Set<String>()
        var names: [String] = []
        for directory in directories {
            let entries = (try? FileManager.default.contentsOfDirectory(atPath: directory.path)) ?? []
            for name in entries.filter({ !$0.hasPrefix(".") }) where seen.insert(name.lowercased()).inserted {
                names.append(name)
            }
        }
        return names.sorted { $0.localizedStandardCompare($1) == .orderedAscending }
    }

    static let names: [String] = names(in: directories)

    /// The file Ghostty loads for a theme: a path as given, otherwise the
    /// user's theme of that name before the bundled one.
    static func file(named theme: String) -> URL? {
        file(named: theme, in: directories)
    }

    static func file(named theme: String, in directories: [URL]) -> URL? {
        if theme.hasPrefix("/") { return URL(fileURLWithPath: theme) }
        return directories
            .map { $0.appendingPathComponent(theme) }
            .first { FileManager.default.fileExists(atPath: $0.path) }
    }

    /// A theme option with each name Code can find replaced by the file it
    /// names, so libghostty loads Code's themes. Nil when no name needs it,
    /// which leaves the option to libghostty as it is.
    static func resolved(_ value: String, in directories: [URL] = directories) -> String? {
        var changed = false
        let parts = value.split(separator: ",").map { part -> String in
            let pair = part.split(separator: ":", maxSplits: 1).map { $0.trimmingCharacters(in: .whitespaces) }
            func resolve(_ name: String) -> String {
                guard let file = file(named: name, in: directories) else { return name }
                changed = changed || file.path != name
                return file.path
            }
            guard pair.count == 2 else { return resolve(part.trimmingCharacters(in: .whitespaces)) }
            return "\(pair[0]):\(resolve(pair[1]))"
        }
        guard changed else { return nil }
        return parts.joined(separator: ",")
    }

    private static var previews: [String: Colors] = [:]

    struct Colors {
        var background: NSColor
        var foreground: NSColor
        var palette: [NSColor]
    }

    /// The colors libghostty gives a theme, or its own colors for nil.
    static func colors(of theme: String?) -> Colors? {
        let key = theme ?? ""
        if let cached = previews[key] { return cached }
        let file = FileManager.default.temporaryDirectory.appendingPathComponent("code-theme-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: file) }
        let contents = theme.map { "theme = \($0)\n" } ?? ""
        guard (try? contents.write(to: file, atomically: true, encoding: .utf8)) != nil,
              let config = try? GhosttyConfig(file: file), config.palette.count == 16 else { return nil }
        let colors = Colors(background: config.backgroundColor, foreground: config.foregroundColor, palette: config.palette)
        previews[key] = colors
        return colors
    }
}

/// The `font-family` list as Ghostty uses it. An empty value clears the
/// families before it. Of the rest, the first is the font when it is one the
/// picker offers; the others, such as symbol or emoji fonts, are fallbacks
/// tried after it.
enum FontFamilies {
    static func font(in values: [String], choices: Set<String>) -> String? {
        active(values).first.flatMap { choices.contains($0) ? $0 : nil }
    }

    /// Nil is Ghostty's own font, which follows any fallbacks.
    static func setting(_ font: String?, in values: [String], choices: Set<String>) -> [String] {
        let reset = values.lastIndex(of: "").map { $0 + 1 } ?? 0
        var families = Array(values[reset...])
        if let first = families.first, choices.contains(first) {
            families.removeFirst()
        }
        if let font {
            families.insert(font, at: 0)
        }
        return values[..<reset] + families
    }

    private static func active(_ values: [String]) -> ArraySlice<String> {
        values.lastIndex(of: "").map { values[($0 + 1)...] } ?? values[...]
    }
}

enum Fonts {
    /// Font families whose fonts are all one width, which is what a terminal
    /// can use.
    static let monospaced: [String] = {
        let manager = NSFontManager.shared
        return manager.availableFontFamilies.filter { family in
            manager.availableMembers(ofFontFamily: family)?.contains { member in
                guard member.count > 3, let traits = member[3] as? UInt else { return false }
                return NSFontTraitMask(rawValue: traits).contains(.fixedPitchFontMask)
            } ?? false
        }.filter { !$0.hasPrefix(".") }
    }()
}
