import Foundation

extension AgentAdapter {
    /// Pi loads extensions from its own config folder, so Code does not stand
    /// in for its binary on PATH: the extension is installed from Settings.
    static let pi = AgentAdapter(
        name: "pi",
        binary: "pi",
        launch: .none,
        translate: { _ in nil }
    )
}

/// Pi runs no hooks but loads extensions, discovering the ones in its config
/// folder. Code's is installed there rather than passed with `--extension` at
/// each launch, so `pi` on PATH stays the user's own binary.
enum Pi {
    /// Code's extension inside the app's resources, for Code to install itself.
    static let extensionResource = "agents/pi/code.ts"
}
