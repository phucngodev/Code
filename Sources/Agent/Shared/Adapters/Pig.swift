import Foundation

extension AgentAdapter {
    /// PiG loads extensions from its own config folder, so Code does not stand
    /// in for its binary on PATH: the extension is installed there.
    static let pig = AgentAdapter(
        name: "pig",
        binary: "pig",
        launch: .none,
        translate: { _ in nil }
    )
}

/// PiG runs no hooks but loads extensions, discovering a Go extension in its
/// config folder. Code's is installed there rather than passed at each launch,
/// so `pig` on PATH stays the user's own binary and PiG never reads the user's
/// Pi config.
enum Pig {
    /// Code's extension inside the app's resources, for Code to install itself.
    static let extensionResource = "agents/pig/code"
}
