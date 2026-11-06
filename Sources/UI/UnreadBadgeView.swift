import AppKit

/// A small capsule with a count, for a row that needs the user's attention.
final class UnreadBadgeView: NSView {
    static let height: CGFloat = 14
    private static let minWidth: CGFloat = 14

    var count: Int = 0 {
        didSet {
            guard count != oldValue else { return }
            invalidateIntrinsicContentSize()
            needsDisplay = true
            isHidden = count == 0
            setAccessibilityLabel(count == 0 ? nil : "\(count) unread")
        }
    }

    init() {
        super.init(frame: .zero)
        translatesAutoresizingMaskIntoConstraints = false
        wantsLayer = true
        isHidden = true
        setAccessibilityElement(true)
        setAccessibilityRole(.staticText)
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) is not supported")
    }

    override var intrinsicContentSize: NSSize {
        let width = count > 0 ? (Self.string(count) as NSString).size(withAttributes: Self.attributes).width + 9 : Self.minWidth
        return NSSize(width: max(width, Self.minWidth), height: Self.height)
    }

    /// The badge annotates a control and often sits on top of it, so it must
    /// not swallow the clicks meant for that control.
    override func hitTest(_ point: NSPoint) -> NSView? {
        nil
    }

    override func draw(_ dirtyRect: NSRect) {
        guard count > 0 else { return }
        NSColor.systemRed.setFill()
        NSBezierPath(roundedRect: bounds, xRadius: bounds.height / 2, yRadius: bounds.height / 2).fill()
        let text = Self.string(count) as NSString
        let size = text.size(withAttributes: Self.attributes)
        text.draw(at: NSPoint(x: bounds.midX - size.width / 2, y: bounds.midY - size.height / 2),
                  withAttributes: Self.attributes)
    }

    private static let attributes: [NSAttributedString.Key: Any] = [
        .font: NSFont.monospacedDigitSystemFont(ofSize: 9, weight: .bold),
        .foregroundColor: NSColor.white,
    ]

    /// Counts above 99 read as "99+", so the capsule stays narrow.
    private static func string(_ count: Int) -> String {
        count > 99 ? "99+" : "\(count)"
    }
}
