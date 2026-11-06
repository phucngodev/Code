import Foundation

/// Code's extension for an agent that loads extensions from a config folder.
/// Installing it lets the agent report to Code without Code shadowing the
/// user's binary on PATH.
struct AgentExtension: Equatable {
    /// The agent's name, for the Settings row.
    let agent: String
    /// The folder the agent discovers extensions in.
    let directory: URL
    /// The file name the extension takes there.
    let name: String
    /// The bundled extension Code ships.
    let source: URL

    /// Where the extension is installed.
    var destination: URL { directory.appendingPathComponent(name) }

    /// Whether Code's extension is installed, and up to date.
    enum Status: Equatable {
        case installed
        case outdated
        case missing
    }

    enum InstallError: Error, CustomStringConvertible {
        case sourceMissing(String)

        var description: String {
            switch self {
            case let .sourceMissing(path): "Code's extension is missing at \(path)"
            }
        }
    }

    func status(fileManager: FileManager = .default) -> Status {
        guard let bundled = fileManager.contents(atPath: source.path),
              let installed = fileManager.contents(atPath: destination.path) else { return .missing }
        return bundled == installed ? .installed : .outdated
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
