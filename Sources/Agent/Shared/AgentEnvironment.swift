import Foundation

/// Variables every shell Code starts inherits, so that coding agents and the
/// hooks they run can tell Code which terminal they are in and reach it.
enum AgentEnvironment {
    static let surfaceKey = "CODE_SURFACE_ID"
    static let workspaceKey = "CODE_WORKSPACE_ID"
    static let socketKey = "CODE_SOCKET"
    /// The `cde` inside the app, for the shell integration to call.
    static let cdeKey = "CODE_CDE"
    /// Where this Code's shells keep their shims, one directory per terminal.
    static let shimsKey = "CODE_SHIMS"
    /// Set by the user to start agents exactly as typed.
    static let disabledKey = "CODE_AGENTS_DISABLED"
    /// Set by a shim, so `cde launch` can look for the real binary past it.
    static let shimDirectoryKey = "CODE_SHIM_DIR"

    static let socketDirectory = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
        .appendingPathComponent("code", isDirectory: true)

    /// Each running Code listens on its own socket, so two copies of the app
    /// never take each other's.
    static func socketURL(pid: pid_t = getpid(), in directory: URL = socketDirectory) -> URL {
        directory.appendingPathComponent("agent.\(pid).sock")
    }

    /// The agent sockets in `directory`, with the pid of the Code that made each.
    static func sockets(in directory: URL = socketDirectory) -> [(url: URL, pid: pid_t)] {
        let names = (try? FileManager.default.contentsOfDirectory(atPath: directory.path)) ?? []
        return names.compactMap { name in
            guard name.hasPrefix("agent."), name.hasSuffix(".sock"),
                  let pid = pid_t(name.dropFirst("agent.".count).dropLast(".sock".count)), pid > 0 else { return nil }
            return (directory.appendingPathComponent(name), pid)
        }
    }

    /// A process owned by someone else still exists, it just cannot be signalled.
    static func isRunning(_ pid: pid_t) -> Bool {
        kill(pid, 0) == 0 || errno != ESRCH
    }

    /// For `cde` run outside Code: the socket of the Code started most recently.
    static func newestRunningSocket(in directory: URL = socketDirectory) -> URL? {
        sockets(in: directory)
            .filter { isRunning($0.pid) }
            .max { modified($0.url) < modified($1.url) }?
            .url
    }

    private static func modified(_ url: URL) -> Date {
        (try? FileManager.default.attributesOfItem(atPath: url.path)[.modificationDate] as? Date) ?? .distantPast
    }

    static func variables(surface: UUID, workspace: UUID, socket: String, cde: String, shims: String) -> [String: String] {
        [
            surfaceKey: surface.uuidString,
            workspaceKey: workspace.uuidString,
            socketKey: socket,
            cdeKey: cde,
            shimsKey: shims,
        ]
    }
}
