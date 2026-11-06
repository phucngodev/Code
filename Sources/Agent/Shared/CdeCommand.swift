import Foundation

/// What an invocation of `cde` asks for, worked out from its arguments and
/// environment alone so it can be tested without running the tool.
enum CdeCommand: Equatable {
    /// A nil socket is for `cde` run outside Code, which looks for one itself.
    case event(AgentEvent, socket: String?)
    case hook(adapter: String, socket: String?)
    case launch(adapter: String, arguments: [String])
    case shims(directory: String)
    case refreshAgents(script: String)
    case ping(socket: String?)
    case usage

    struct UsageError: Error, Equatable, CustomStringConvertible {
        let description: String
    }

    static let usageText = """
        usage: cde event <kind> [--agent NAME] [--session ID] [--detail TEXT]
                                [--cwd PATH] [--surface UUID] [--workspace UUID]
               cde hook <agent>              (reads the agent's hook JSON on stdin)
               cde launch <agent> [args...]
               cde shims <dir>
               cde agents refresh
               cde ping

        kinds: sessionStarted turnStarted working needsInput turnEnded sessionEnded attention
        """

    static func parse(_ arguments: [String], environment: [String: String], currentDirectory: String, now: Date) throws -> CdeCommand {
        let socket = environment[AgentEnvironment.socketKey].flatMap { $0.isEmpty ? nil : $0 }
        switch (arguments.first, arguments.dropFirst().first) {
        case ("event", _):
            return .event(try event(Array(arguments.dropFirst()), environment: environment, currentDirectory: currentDirectory, now: now), socket: socket)
        case let ("hook", adapter?):
            return .hook(adapter: adapter, socket: socket)
        case let ("launch", adapter?):
            return .launch(adapter: adapter, arguments: Array(arguments.dropFirst(2)))
        case let ("shims", directory?):
            return .shims(directory: directory)
        case ("agents", "refresh"):
            return .refreshAgents(script: refreshScript(environment: environment))
        case ("ping", _):
            return .ping(socket: socket)
        default:
            return .usage
        }
    }

    /// What the shell integration runs at the first prompt, for the user to
    /// run again after installing an agent, written for their shell.
    static func refreshScript(environment: [String: String]) -> String {
        let posix = #""$CODE_CDE" shims "$dir" && PATH="$dir:$PATH""#
        guard environment["SHELL"].map({ ($0 as NSString).lastPathComponent }) == "fish" else {
            return #"dir="$CODE_SHIMS/$CODE_SURFACE_ID""# + "\n" + posix
        }
        return """
            set dir $CODE_SHIMS/$CODE_SURFACE_ID
            "$CODE_CDE" shims $dir; and set -gx --prepend PATH $dir
            """
    }

    private static func event(_ arguments: [String], environment: [String: String], currentDirectory: String, now: Date) throws -> AgentEvent {
        guard let name = arguments.first else { throw UsageError(description: "missing event kind") }
        guard let kind = AgentEvent.Kind(rawValue: name), kind != .unknown else {
            throw UsageError(description: "unknown event kind \(name)")
        }
        var options: [String: String] = [:]
        var rest = arguments.dropFirst()
        while let flag = rest.popFirst() {
            guard ["--agent", "--session", "--detail", "--cwd", "--surface", "--workspace"].contains(flag) else {
                throw UsageError(description: "unknown option \(flag)")
            }
            guard let value = rest.popFirst() else { throw UsageError(description: "\(flag) needs a value") }
            options[flag] = value
        }
        return AgentEvent(
            agent: options["--agent"] ?? "",
            kind: kind,
            sessionID: options["--session"] ?? "",
            surfaceID: try uuid(options["--surface"], flag: "--surface") ?? environment[AgentEnvironment.surfaceKey].flatMap(UUID.init),
            workspaceID: try uuid(options["--workspace"], flag: "--workspace") ?? environment[AgentEnvironment.workspaceKey].flatMap(UUID.init),
            cwd: options["--cwd"] ?? currentDirectory,
            at: now,
            detail: options["--detail"]
        )
    }

    private static func uuid(_ value: String?, flag: String) throws -> UUID? {
        guard let value else { return nil }
        guard let uuid = UUID(uuidString: value) else { throw UsageError(description: "\(flag) is not a UUID: \(value)") }
        return uuid
    }
}
