import Foundation

/// Code's extension for an agent that loads extensions from a config folder.
/// Installing it lets the agent report to Code without Code shadowing the
/// user's binary on PATH.
struct AgentExtension: Equatable {
    /// The agent's name, for the log.
    let agent: String
    /// The folder the agent discovers extensions in.
    let directory: URL
    /// The file name the extension takes there.
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
    /// when it is already up to date.
    @discardableResult
    func install(fileManager: FileManager = .default) throws -> Bool {
        guard let bundled = fileManager.contents(atPath: source.path) else {
            throw InstallError.sourceMissing(source.path)
        }
        if fileManager.contents(atPath: destination.path) == bundled { return false }
        try fileManager.createDirectory(at: directory, withIntermediateDirectories: true)
        try bundled.write(to: destination, options: .atomic)
        return true
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
        return [pi(home: home, resources: resources, environment: environment)].compactMap { $0 }
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
}
