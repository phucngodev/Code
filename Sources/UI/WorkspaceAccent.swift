import AppKit

/// The colors a workspace's chip is drawn from. A workspace keeps its index
/// across relaunches, so the same project keeps its color.
enum WorkspaceAccent {
    /// Chosen to read both on the vibrant sidebar and on the selected pill.
    static let colors: [NSColor] = [
        .systemBlue, .systemPurple, .systemPink, .systemRed,
        .systemOrange, .systemYellow, .systemGreen, .systemTeal,
    ]

    /// The color at an index, wrapped so a stale saved index still draws.
    static func color(for index: Int) -> NSColor {
        let count = colors.count
        return colors[((index % count) + count) % count]
    }
}
