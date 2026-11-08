import Foundation

/// Code's extension for an agent that loads extensions from a config folder.
/// Installing it lets the agent report to Code without Code shadowing the
/// user's binary on PATH.
struct AgentExtension: Equatable {
    /// The agent's name, for the log.
    let agent: String
    /// The folder the agent discovers extensions in.
    let directory: URL
    /// The name the extension takes there, a file or folder name.
    let name: String
    /// The bundled extension Code ships.
    let source: URL

    /// Where the extension is installed.
    var destination: URL { directory.appendingPathComponent(name) }

    /// The agent's own config folder. Its presence shows the agent is set up,
    /// so Code does not make an extensions folder for a user who does not use it.
    var agentDirectory: URL { directory.deletingLastPathComponent() }

    enum InstallError: Error, CustomStringConvertible {
        case sourceMissing(String)

        var description: String {
            switch self {
            case let .sourceMissing(path): "Code's extension is missing at \(path)"
            }
        }
    }

    /// Copies the bundled extension over the installed one, returning false
    /// when it is already up to date. An extension is a file or a folder of
    /// source files, depending on the agent's runtime.
    @discardableResult
    func install(fileManager: FileManager = .default) throws -> Bool {
        var isDirectory: ObjCBool = false
        guard fileManager.fileExists(atPath: source.path, isDirectory: &isDirectory) else {
            throw InstallError.sourceMissing(source.path)
        }
        if isDirectory.boolValue {
            return try installDirectory(fileManager: fileManager)
        }
        guard let bundled = fileManager.contents(atPath: source.path) else {
            throw InstallError.sourceMissing(source.path)
        }
        if fileManager.contents(atPath: destination.path) == bundled { return false }
        try fileManager.createDirectory(at: directory, withIntermediateDirectories: true)
        try bundled.write(to: destination, options: .atomic)
        return true
    }

    /// Copies each top-level file of a folder extension, returning false when
    /// every file at the destination already matches.
    private func installDirectory(fileManager: FileManager) throws -> Bool {
        let files = try fileManager.contentsOfDirectory(at: source, includingPropertiesForKeys: [.isRegularFileKey])
            .filter { url in
                guard !url.lastPathComponent.hasPrefix(".") else { return false }
                return (try? url.resourceValues(forKeys: [.isRegularFileKey]).isRegularFile) == true
            }
        var changed = false
        for file in files {
            let contents = try Data(contentsOf: file)
            let installed = destination.appendingPathComponent(file.lastPathComponent)
            if fileManager.contents(atPath: installed.path) == contents { continue }
            try fileManager.createDirectory(at: destination, withIntermediateDirectories: true)
            try contents.write(to: installed, options: .atomic)
            changed = true
        }
        return changed
    }
}

/// The extensions Code can install, one per agent that reads them from a
/// config folder rather than having Code stand in for its binary.
enum AgentExtensions {
    /// Installs or updates Code's extension for every agent that is set up,
    /// leaving alone one whose config folder is not there, so nothing is made
    /// for a user who does not use it. Called at each launch, which also brings
    /// the extension up to date after Code is updated.
    static func installAll(extensions: [AgentExtension] = all(), fileManager: FileManager = .default) {
        for ext in extensions where fileManager.fileExists(atPath: ext.agentDirectory.path) {
            do {
                if try ext.install(fileManager: fileManager) {
                    logger.notice("installed Code's extension for \(ext.agent, privacy: .public)")
                }
            } catch {
                logger.error("could not install Code's extension for \(ext.agent, privacy: .public): \(error, privacy: .public)")
            }
        }
    }

    static func all(home: URL = FileManager.default.homeDirectoryForCurrentUser,
                    resources: URL? = Bundle.main.resourceURL,
                    environment: [String: String] = ProcessInfo.processInfo.environment) -> [AgentExtension] {
        guard let resources else { return [] }
        return [pi(home: home, resources: resources, environment: environment),
                pig(home: home, resources: resources)].compactMap { $0 }
    }

    /// Pi's extension, in `~/.pi/agent/extensions` unless the user set
    /// `PI_CODING_AGENT_DIR`.
    static func pi(home: URL, resources: URL, environment: [String: String]) -> AgentExtension? {
        let agentDirectory = environment["PI_CODING_AGENT_DIR"]
            .map { URL(fileURLWithPath: ($0 as NSString).expandingTildeInPath) }
            ?? home.appendingPathComponent(".pi/agent")
        return AgentExtension(agent: "Pi",
                              directory: agentDirectory.appendingPathComponent("extensions"),
                              name: "code.ts",
                              source: resources.appendingPathComponent(Pi.extensionResource))
    }

    /// PiG's extension, in `~/.config/pig/agent/extensions`.
    static func pig(home: URL, resources: URL) -> AgentExtension? {
        let agentDirectory = home.appendingPathComponent(".config/pig/agent")
        return AgentExtension(agent: "PiG",
                              directory: agentDirectory.appendingPathComponent("extensions"),
                              name: "code",
                              source: resources.appendingPathComponent(Pig.extensionResource))
    }
}
