import AppKit
import UniformTypeIdentifiers

/// Code's config file. It uses Ghostty's syntax, so every Ghostty option works
/// in it, but Code reads only this file and not Ghostty's own config.
enum ConfigFile {
    /// `~/.config/code/config` (`code-debug` for a debug build), or the file
    /// named by the CODE_CONFIG environment variable.
    static var url: URL {
        if let override = ProcessInfo.processInfo.environment["CODE_CONFIG"] {
            return URL(fileURLWithPath: override)
        }
        return configHome.appendingPathComponent(AppFolder.name, isDirectory: true).appendingPathComponent("config")
    }

    /// The installed Code's config, which a debug build starts from so it
    /// looks the same.
    private static var installedURL: URL {
        configHome.appendingPathComponent("code", isDirectory: true).appendingPathComponent("config")
    }

    /// The themes Code finds by name.
    static var themesDirectory: URL {
        configHome.appendingPathComponent(AppFolder.name, isDirectory: true).appendingPathComponent("themes", isDirectory: true)
    }

    /// Ghostty's config files, in the order Ghostty loads them.
    static var ghosttyFiles: [URL] {
        let appSupport = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("com.mitchellh.ghostty", isDirectory: true)
        let xdg = configHome.appendingPathComponent("ghostty", isDirectory: true)
        return [
            xdg.appendingPathComponent("config"),
            xdg.appendingPathComponent("config.ghostty"),
            appSupport.appendingPathComponent("config"),
            appSupport.appendingPathComponent("config.ghostty"),
        ]
    }

    private static var configHome: URL {
        ProcessInfo.processInfo.environment["XDG_CONFIG_HOME"].map { URL(fileURLWithPath: $0, isDirectory: true) }
            ?? FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".config", isDirectory: true)
    }

    static let header = """
        # Code settings. The Settings window rewrites the lines it manages; all
        # other lines are preserved. All Ghostty options are supported.

        """

    /// Earlier versions of Code read Ghostty's config, so the first time Code
    /// makes its own it copies those settings over and nothing changes for
    /// people who set Code up through Ghostty. A debug build copies the
    /// installed Code's instead.
    static func createIfMissing() {
        guard ProcessInfo.processInfo.environment["CODE_CONFIG"] == nil else { return }
        do {
            if AppFolder.isDebugBuild {
                try copy(installedURL, to: url)
            } else {
                try create(at: url, importing: ghosttyFiles)
            }
        } catch {
            logger.error("could not create \(url.path, privacy: .public): \(error, privacy: .public)")
        }
    }

    static func create(at url: URL, importing ghosttyFiles: [URL]) throws {
        guard !FileManager.default.fileExists(atPath: url.path) else { return }
        try write(header + imported(from: ghosttyFiles), to: url)
    }

    /// The copy's relative paths still lead to the files the original's did.
    /// Without an original, the copy starts as the header alone.
    static func copy(_ original: URL, to url: URL) throws {
        guard !FileManager.default.fileExists(atPath: url.path) else { return }
        guard let text = try? String(contentsOf: original, encoding: .utf8) else { return try write(header, to: url) }
        var document = ConfigDocument(text)
        document.anchorPaths(at: original.deletingLastPathComponent())
        try write(document.text, to: url)
    }

    /// Writes through symlinks, including one whose target does not exist
    /// yet, so a config kept with someone's dotfiles stays a link to them.
    static func write(_ text: String, to url: URL) throws {
        let target = writeTarget(for: url)
        // Writing to a link that still points on, as in a loop, would replace
        // it with a plain file.
        if (try? FileManager.default.destinationOfSymbolicLink(atPath: target.path)) != nil {
            throw CocoaError(.fileWriteUnknown, userInfo: [NSFilePathErrorKey: url.path, NSLocalizedDescriptionKey: "Too many symbolic links lead to \(url.path)"])
        }
        try FileManager.default.createDirectory(at: target.deletingLastPathComponent(), withIntermediateDirectories: true)
        try text.write(to: target, atomically: true, encoding: .utf8)
    }

    static func writeTarget(for url: URL) -> URL {
        var url = url
        for _ in 0..<16 {
            guard let destination = try? FileManager.default.destinationOfSymbolicLink(atPath: url.path) else { break }
            url = URL(fileURLWithPath: destination, relativeTo: url.deletingLastPathComponent()).standardizedFileURL
        }
        return url
    }

    /// The settings in the given files, each under a note of where it came
    /// from. Files holding only comments, such as the template Ghostty writes
    /// on its first launch, are left out.
    ///
    /// libghostty reads bytes rather than UTF-8, so a file with other text in
    /// it, usually in a comment, is still imported, with that text replaced.
    static func imported(from files: [URL]) -> String {
        files.compactMap { file -> String? in
            guard let data = try? Data(contentsOf: file) else { return nil }
            // Ghostty skips a byte order mark only at the very start of a
            // file, and these files are combined below a header.
            var text = String(decoding: data, as: UTF8.self)
            if text.hasPrefix("\u{FEFF}") { text.removeFirst() }
            var document = ConfigDocument(text)
            guard !document.isEmpty else { return nil }
            document.anchorPaths(at: file.deletingLastPathComponent())
            return "\n# Imported from \(file.path.abbreviatingHome)\n" + document.text
        }.joined()
    }
}

extension ConfigFile {
    /// The files a config includes through `config-file`, and theirs in turn.
    /// Ghostty loads them after the file that names them, so their settings win.
    static func includes(of document: ConfigDocument, at url: URL) -> [(url: URL, document: ConfigDocument)] {
        var found: [(url: URL, document: ConfigDocument)] = []
        var seen: Set<String> = [url.standardizedFileURL.path]
        func visit(_ document: ConfigDocument, in directory: URL) {
            // An empty value clears the includes named before it.
            let values = document.values(for: "config-file")
            let current = values.lastIndex(of: "").map { values[($0 + 1)...] } ?? values[...]
            for value in current {
                guard let include = ConfigPath(value)?.resolved(from: directory),
                      seen.insert(include.standardizedFileURL.path).inserted,
                      let text = try? String(contentsOf: include, encoding: .utf8) else { continue }
                let included = ConfigDocument(text)
                found.append((include, included))
                visit(included, in: include.deletingLastPathComponent())
            }
        }
        visit(document, in: url.deletingLastPathComponent())
        return found
    }

    /// A file setting `theme` to the themes Code finds for the names in the
    /// given config, so a theme kept in Code's themes directory resolves even
    /// though libghostty looks only in Ghostty's own. Nil when the config
    /// names no theme Code can find, leaving the option to libghostty.
    static func themeOverride(for url: URL, in directories: [URL] = Themes.directories) -> URL? {
        guard let text = try? String(contentsOf: url, encoding: .utf8) else { return nil }
        let document = ConfigDocument(text)
        var value = document["theme"]
        for include in includes(of: document, at: url) {
            if let theme = include.document["theme"] { value = theme }
        }
        guard let value, let resolved = Themes.resolved(value, in: directories) else { return nil }
        let file = FileManager.default.temporaryDirectory.appendingPathComponent("code-theme-\(UUID().uuidString)")
        guard (try? "theme = \(resolved)\n".write(to: file, atomically: true, encoding: .utf8)) != nil else { return nil }
        return file
    }

    /// Opens the config file in the user's text editor, creating it first if
    /// it was removed.
    static func openInEditor() {
        let url = url
        if !FileManager.default.fileExists(atPath: url.path) {
            try? write(header, to: url)
        }
        // The file has no extension, so it is opened as plain text rather
        // than with whatever claims extensionless files.
        guard let editor = NSWorkspace.shared.urlForApplication(toOpen: .plainText) else {
            NSWorkspace.shared.open(url)
            return
        }
        NSWorkspace.shared.open([url], withApplicationAt: editor, configuration: NSWorkspace.OpenConfiguration())
    }
}
