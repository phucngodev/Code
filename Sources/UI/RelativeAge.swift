import Foundation

/// A short age for a status line: "now", "5m", "2h", "3d".
enum RelativeAge {
    static func string(since: Date, now: Date = Date()) -> String {
        let seconds = max(0, now.timeIntervalSince(since))
        if seconds < 60 { return "now" }
        let minutes = Int(seconds / 60)
        if minutes < 60 { return "\(minutes)m" }
        let hours = minutes / 60
        if hours < 24 { return "\(hours)h" }
        return "\(hours / 24)d"
    }
}
