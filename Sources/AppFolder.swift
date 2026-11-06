import Foundation

/// The folder name Code keeps its config and session under. A debug build
/// keeps its own, so running one beside the installed Code never changes the
/// installed Code's settings or windows.
enum AppFolder {
    static let isDebugBuild = Bundle.main.bundleIdentifier?.hasSuffix(".debug") == true
    static let name = isDebugBuild ? "code-debug" : "code"
}
