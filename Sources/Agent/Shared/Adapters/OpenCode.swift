import Foundation

extension AgentAdapter {
    static let openCode = AgentAdapter(
        name: "opencode",
        binary: "opencode",
        launch: .environment(OpenCode.environment),
        translate: { _ in nil }
    )
}

/// OpenCode runs no hooks but loads plugins, and adds the ones named in
/// `OPENCODE_CONFIG_CONTENT` to those in the user's own configuration, never
/// touching `~/.config/opencode`. The plugin sends its events with `cde event`,
/// so there is nothing to translate.
///
/// OpenCode 2 replaced the plugin API and takes a directory of plugins, while
/// OpenCode 1 took a single file, so both are named: each version loads the one
/// it understands and ignores the other.
enum OpenCode {
    static let contentKey = "OPENCODE_CONFIG_CONTENT"

    /// The OpenCode 1 plugin, found from the `cde` inside it.
    static func plugin(cde: String) -> String {
        agentsDirectory(cde: cde).appendingPathComponent("code.js").path
    }

    /// The directory OpenCode 2 loads, whose `index.js` is its plugin.
    static func pluginDirectory(cde: String) -> String {
        agentsDirectory(cde: cde).appendingPathComponent("v2").path
    }

    private static func agentsDirectory(cde: String) -> URL {
        URL(fileURLWithPath: cde)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Resources/agents/opencode", isDirectory: true)
    }

    /// Configuration the user set in `OPENCODE_CONFIG_CONTENT` is kept, with
    /// Code's plugins added to theirs. Content Code cannot read is left for
    /// OpenCode to report, without Code's plugins.
    static func environment(_ environment: [String: String], cde: String) -> [String: String] {
        var config: [String: Any] = [:]
        if let content = environment[contentKey], !content.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            guard let user = (try? JSONSerialization.jsonObject(with: Data(content.utf8))) as? [String: Any] else { return [:] }
            config = user
        }
        config["plugin"] = (config["plugin"] as? [Any] ?? []) + [plugin(cde: cde)]
        config["plugins"] = (config["plugins"] as? [Any] ?? []) + [pluginDirectory(cde: cde)]
        guard let json = try? JSONSerialization.data(withJSONObject: config, options: [.sortedKeys, .withoutEscapingSlashes]) else {
            return [:]
        }
        return [contentKey: String(decoding: json, as: UTF8.self)]
    }
}
