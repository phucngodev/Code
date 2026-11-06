import AppKit

/// The state mark a sidebar row draws for a group of terminals: a turning arc
/// while they work, a dot when they need the user, a check when a turn
/// finished, and nothing when idle.
///
/// Everything is drawn in `draw(_:)`, with no sublayer or Core Animation: the
/// sidebar's table cells are drawn with their subviews flattened, so a layer
/// added to this view is not rendered. The turn is driven by a timer, which
/// runs only while the view is working and in a window, and respects Reduce
/// Motion.
final class AgentGlyphView: NSView {
    static let size: CGFloat = 12
    /// A full turn every two seconds.
    private static let tick: TimeInterval = 1.0 / 30.0
    private static let degreesPerTick: CGFloat = 6

    var indicator: AgentIndicator? {
        didSet {
            guard indicator != oldValue else { return }
            needsDisplay = true
            updateAnimation()
            setAccessibilityLabel(label)
            toolTip = label
        }
    }

    private var spinTimer: Timer?
    private var angle: CGFloat = 0
    private var motionObserver: Any?

    init() {
        super.init(frame: NSRect(x: 0, y: 0, width: Self.size, height: Self.size))
        wantsLayer = true
        translatesAutoresizingMaskIntoConstraints = false
        setAccessibilityElement(true)
        setAccessibilityRole(.image)
        NSLayoutConstraint.activate([
            widthAnchor.constraint(equalToConstant: Self.size),
            heightAnchor.constraint(equalToConstant: Self.size),
        ])
        motionObserver = NSWorkspace.shared.notificationCenter.addObserver(
            forName: NSWorkspace.accessibilityDisplayOptionsDidChangeNotification, object: nil, queue: .main
        ) { [weak self] _ in self?.updateAnimation() }
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) is not supported")
    }

    deinit {
        spinTimer?.invalidate()
        motionObserver.map(NSWorkspace.shared.notificationCenter.removeObserver)
    }

    private var color: NSColor? {
        switch indicator {
        case .working: .systemBlue
        case .waiting: .systemYellow
        case .done: .systemGreen
        case nil: nil
        }
    }

    private var label: String? {
        switch indicator {
        case .working: "Working"
        case .waiting: "Waiting for input"
        case .done: "Finished"
        case nil: nil
        }
    }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        needsDisplay = true
    }

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        updateAnimation()
    }

    override func draw(_ dirtyRect: NSRect) {
        guard let color else { return }
        color.setStroke()
        color.setFill()
        switch indicator {
        case .working:
            let arc = NSBezierPath()
            arc.appendArc(withCenter: NSPoint(x: bounds.midX, y: bounds.midY), radius: 4.25,
                          startAngle: angle, endAngle: angle + 270, clockwise: false)
            arc.lineWidth = 1.5
            arc.lineCapStyle = .round
            arc.stroke()
        case .waiting:
            let diameter: CGFloat = 7
            NSBezierPath(ovalIn: NSRect(x: (bounds.width - diameter) / 2, y: (bounds.height - diameter) / 2,
                                        width: diameter, height: diameter)).fill()
        case .done:
            let check = NSBezierPath()
            check.lineWidth = 1.8
            check.lineCapStyle = .round
            check.lineJoinStyle = .round
            check.move(to: NSPoint(x: bounds.midX - 4, y: bounds.midY))
            check.line(to: NSPoint(x: bounds.midX - 1, y: bounds.midY - 3))
            check.line(to: NSPoint(x: bounds.midX + 4, y: bounds.midY + 3.5))
            check.stroke()
        case nil:
            break
        }
    }

    private func updateAnimation() {
        let spins = indicator == .working && window != nil && !NSWorkspace.shared.accessibilityDisplayShouldReduceMotion
        guard spins else {
            spinTimer?.invalidate()
            spinTimer = nil
            return
        }
        guard spinTimer == nil else { return }
        let timer = Timer(timeInterval: Self.tick, repeats: true) { [weak self] _ in
            guard let self else { return }
            self.angle = (self.angle + Self.degreesPerTick).truncatingRemainder(dividingBy: 360)
            self.needsDisplay = true
        }
        RunLoop.main.add(timer, forMode: .common)
        spinTimer = timer
    }
}
