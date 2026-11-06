import AppKit

/// What dropping a dragged pane at a point means: the middle of the target
/// swaps the two panes, an outer quarter moves the dragged one to that side.
enum PaneDropAction: Equatable {
    case swap
    case split(SplitDirection)

    /// The action for a drop at `point`, in the coordinate space of the target
    /// pane's `frame`. The nearest edge within a quarter wins; otherwise the
    /// middle. AppKit gives a plain view a bottom-left origin, so a low `y` is
    /// the bottom edge.
    static func at(_ point: NSPoint, in frame: NSRect) -> PaneDropAction {
        guard frame.width > 0, frame.height > 0 else { return .swap }
        let toLeft = point.x - frame.minX
        let toRight = frame.maxX - point.x
        let toBottom = point.y - frame.minY
        let toTop = frame.maxY - point.y
        let horizontal = min(toLeft, toRight)
        let vertical = min(toBottom, toTop)
        if horizontal < frame.width * 0.25, horizontal <= vertical {
            return .split(toLeft < toRight ? .left : .right)
        }
        if vertical < frame.height * 0.25 {
            return .split(toBottom < toTop ? .down : .up)
        }
        return .swap
    }
}

/// Renders a tab's pane tree as nested split views. Rebuilt whenever the tree's
/// structure changes; divider drags write their ratio back into the tree.
final class PaneTreeView: NSView {
    /// Called when a pane is dropped onto another in this tab. Only ever fires
    /// for panes in the tab being shown, since the tree holds just that tab's.
    var onMovePane: ((TerminalSurfaceView, TerminalSurfaceView, PaneDropAction) -> Void)?

    /// The themed line between panes, painted opaque so nothing behind the
    /// window shows through it.
    var dividerColor: NSColor = .separatorColor {
        didSet { splitViews.values.forEach { $0.dividerLine = dividerColor } }
    }
    /// Color laid over unfocused panes, nil when dimming is disabled.
    var unfocusedFill: NSColor?
    private var tree: PaneTree?
    private var renderedVersion = -1
    private var splitViews: [ObjectIdentifier: PaneSplitView] = [:]
    private var trackingArea: NSTrackingArea?
    /// The pane being dragged, and the pane and action under the pointer.
    private var dragged: TerminalSurfaceView?
    private var drop: (surface: TerminalSurfaceView, action: PaneDropAction)?

    /// The grip appears over the pane under the pointer. Its drag is tracked by
    /// its own mouse events, so the terminal never sees them, and it only shows
    /// when there is another pane to move it beside.
    private lazy var grip: PaneGripView = {
        let grip = PaneGripView()
        grip.isHidden = true
        grip.onDrag = { [weak self] phase, location in self?.handleGrip(phase, at: location) }
        return grip
    }()

    private lazy var dropIndicator: PaneDropIndicatorView = {
        let indicator = PaneDropIndicatorView()
        indicator.isHidden = true
        return indicator
    }()

    func show(_ tree: PaneTree?) {
        if tree === self.tree, tree?.version == renderedVersion { return }
        self.tree = tree
        renderedVersion = tree?.version ?? -1
        splitViews.removeAll()
        subviews.forEach { $0.removeFromSuperview() }
        cancelDrag()
        guard let tree else { return }
        let view = tree.zoomed.map(zoomedView) ?? build(tree.root)
        view.frame = bounds
        view.autoresizingMask = [.width, .height]
        addSubview(view)
        dropIndicator.frame = bounds
        dropIndicator.autoresizingMask = [.width, .height]
        addSubview(dropIndicator)
        addSubview(grip)
    }

    func updateDimming(focused: TerminalSurfaceView?) {
        let surfaces = tree?.surfaces ?? []
        let dims = surfaces.count > 1 && tree?.zoomed == nil
        for surface in surfaces {
            surface.setDimColor(dims && surface !== focused ? unfocusedFill : nil)
        }
    }

    private func zoomedView(_ surface: TerminalSurfaceView) -> NSView {
        surface.removeFromSuperview()
        return surface
    }

    private func build(_ pane: Pane) -> NSView {
        if let surface = pane.surface {
            surface.removeFromSuperview()
            return surface
        }
        let split = PaneSplitView(pane: pane)
        split.dividerLine = dividerColor
        splitViews[ObjectIdentifier(pane)] = split
        split.addArrangedSubview(build(pane.first!))
        split.addArrangedSubview(build(pane.second!))
        return split
    }

    /// Moves the divider that borders the pane in the given direction.
    func resize(_ surface: TerminalSurfaceView, direction: ResizeDirection, amount: CGFloat) {
        guard let tree, var child = tree.node(for: surface) else { return }
        let horizontal = direction == .left || direction == .right
        let towardSecond = direction == .right || direction == .down
        while let parent = child.parent {
            if (parent.axis == .horizontal) == horizontal, (child === parent.first) == towardSecond,
               let split = splitViews[ObjectIdentifier(parent)] {
                split.setPosition(split.dividerPosition + (towardSecond ? amount : -amount), ofDividerAt: 0)
                return
            }
            child = parent
        }
    }

    // MARK: Moving panes

    /// The pane at a point, with its frame in this view, or nil over a divider
    /// or the gap the split views leave.
    private func pane(at point: NSPoint) -> (surface: TerminalSurfaceView, frame: NSRect)? {
        for surface in tree?.surfaces ?? [] {
            let frame = surface.convert(surface.bounds, to: self)
            if frame.contains(point) { return (surface, frame) }
        }
        return nil
    }

    private func handleGrip(_ phase: PaneGripView.Phase, at location: NSPoint) {
        let point = convert(location, from: nil)
        switch phase {
        case .began:
            guard let hit = pane(at: point), (tree?.surfaces.count ?? 0) > 1, tree?.zoomed == nil else { return }
            dragged = hit.surface
            updateDrop(at: point)
        case .changed:
            updateDrop(at: point)
        case .ended:
            updateDrop(at: point)
            if let dragged, let drop { onMovePane?(dragged, drop.surface, drop.action) }
            cancelDrag()
        }
    }

    private func updateDrop(at point: NSPoint) {
        guard let dragged, let hit = pane(at: point), hit.surface !== dragged else {
            drop = nil
            dropIndicator.isHidden = true
            return
        }
        let action = PaneDropAction.at(point, in: hit.frame)
        drop = (hit.surface, action)
        dropIndicator.highlight = highlight(for: action, in: hit.frame)
        dropIndicator.isHidden = false
    }

    private func cancelDrag() {
        dragged = nil
        drop = nil
        dropIndicator.isHidden = true
        dropIndicator.highlight = nil
        grip.isHidden = true
    }

    /// The rectangle to light up for a drop: the whole target for a swap, or
    /// the half the dragged pane would take.
    private func highlight(for action: PaneDropAction, in frame: NSRect) -> NSRect {
        switch action {
        case .swap:
            return frame.insetBy(dx: 3, dy: 3)
        case .split(.left):
            return NSRect(x: frame.minX, y: frame.minY, width: frame.width / 2, height: frame.height)
        case .split(.right):
            return NSRect(x: frame.midX, y: frame.minY, width: frame.width / 2, height: frame.height)
        case .split(.down):
            return NSRect(x: frame.minX, y: frame.minY, width: frame.width, height: frame.height / 2)
        case .split(.up):
            return NSRect(x: frame.minX, y: frame.midY, width: frame.width, height: frame.height / 2)
        }
    }

    // MARK: Grip tracking

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let trackingArea { removeTrackingArea(trackingArea) }
        let area = NSTrackingArea(rect: bounds, options: [.mouseEnteredAndExited, .mouseMoved, .activeInKeyWindow, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        trackingArea = area
    }

    override func mouseMoved(with event: NSEvent) {
        guard dragged == nil, (tree?.surfaces.count ?? 0) > 1, tree?.zoomed == nil else { return }
        let point = convert(event.locationInWindow, from: nil)
        guard let hit = pane(at: point) else {
            grip.isHidden = true
            return
        }
        grip.frame = Self.gripFrame(in: hit.frame)
        grip.isHidden = false
    }

    override func mouseExited(with event: NSEvent) {
        if dragged == nil { grip.isHidden = true }
    }

    /// The grip's home: centered along the top of the pane.
    private static func gripFrame(in frame: NSRect) -> NSRect {
        NSRect(x: frame.midX - PaneGripView.size.width / 2,
               y: frame.maxY - PaneGripView.size.height - 6,
               width: PaneGripView.size.width,
               height: PaneGripView.size.height)
    }
}

/// The dots dragged to move a pane. It keeps the mouse through the drag, so the
/// terminal below never sees the events.
private final class PaneGripView: NSView {
    enum Phase { case began, changed, ended }

    static let size = NSSize(width: 26, height: 16)
    var onDrag: ((Phase, NSPoint) -> Void)?

    override init(frame frameRect: NSRect) {
        super.init(frame: NSRect(origin: .zero, size: Self.size))
        setAccessibilityElement(true)
        setAccessibilityRole(.button)
        setAccessibilityLabel("Move pane")
        toolTip = "Drag to move this pane"
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) is not supported")
    }

    override func resetCursorRects() {
        addCursorRect(bounds, cursor: .openHand)
    }

    override func mouseDown(with event: NSEvent) {
        onDrag?(.began, event.locationInWindow)
    }

    override func mouseDragged(with event: NSEvent) {
        onDrag?(.changed, event.locationInWindow)
    }

    override func mouseUp(with event: NSEvent) {
        onDrag?(.ended, event.locationInWindow)
    }

    override func draw(_ dirtyRect: NSRect) {
        let background = NSBezierPath(roundedRect: bounds.insetBy(dx: 0.5, dy: 0.5), xRadius: 4, yRadius: 4)
        NSColor.windowBackgroundColor.withAlphaComponent(0.8).setFill()
        background.fill()
        NSColor.separatorColor.setStroke()
        background.lineWidth = 1
        background.stroke()

        NSColor.secondaryLabelColor.setFill()
        let dot: CGFloat = 2.5, gap: CGFloat = 3
        let width = dot * 3 + gap * 2
        var x = bounds.midX - width / 2
        for _ in 0..<3 {
            NSBezierPath(ovalIn: NSRect(x: x, y: bounds.midY - dot / 2, width: dot, height: dot)).fill()
            x += dot + gap
        }
    }
}

/// Lights up where a dragged pane would land.
private final class PaneDropIndicatorView: NSView {
    var highlight: NSRect? {
        didSet { needsDisplay = true }
    }

    override func draw(_ dirtyRect: NSRect) {
        guard let highlight else { return }
        let path = NSBezierPath(roundedRect: highlight, xRadius: 6, yRadius: 6)
        NSColor.controlAccentColor.withAlphaComponent(0.22).setFill()
        path.fill()
        NSColor.controlAccentColor.withAlphaComponent(0.8).setStroke()
        path.lineWidth = 2
        path.stroke()
    }
}

private final class PaneSplitView: NSSplitView {
    private let pane: Pane
    private var appliedRatio = false
    /// A split view that is its own delegate sends AppKit into endless
    /// recursion when it checks the responder chain for toggleSidebar.
    private lazy var events = Events(owner: self)
    /// Named apart from `NSSplitView.dividerColor`, which AppKit exposes read-only.
    var dividerLine: NSColor = .separatorColor {
        didSet { needsDisplay = true }
    }

    init(pane: Pane) {
        self.pane = pane
        super.init(frame: .zero)
        isVertical = pane.axis == .horizontal
        dividerStyle = .thin
        delegate = events
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) is not supported")
    }

    private var length: CGFloat {
        isVertical ? bounds.width : bounds.height
    }

    var dividerPosition: CGFloat {
        guard let first = arrangedSubviews.first else { return 0 }
        return isVertical ? first.frame.width : first.frame.height
    }

    override func drawDivider(in rect: NSRect) {
        dividerLine.setFill()
        rect.fill()
    }

    /// A dynamic divider colour, such as the system separator, has to be
    /// resolved again when the window turns light or dark.
    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        needsDisplay = true
    }

    override func layout() {
        super.layout()
        guard !appliedRatio, length > 0 else { return }
        setPosition(pane.ratio * length, ofDividerAt: 0)
        appliedRatio = true
    }

    private final class Events: NSObject, NSSplitViewDelegate {
        unowned let owner: PaneSplitView

        init(owner: PaneSplitView) {
            self.owner = owner
        }

        func splitViewDidResizeSubviews(_ notification: Notification) {
            guard owner.appliedRatio, owner.length > 0 else { return }
            owner.pane.ratio = owner.dividerPosition / owner.length
        }

        func splitView(_ splitView: NSSplitView, constrainMinCoordinate proposedMinimumPosition: CGFloat, ofSubviewAt dividerIndex: Int) -> CGFloat {
            80
        }

        func splitView(_ splitView: NSSplitView, constrainMaxCoordinate proposedMaximumPosition: CGFloat, ofSubviewAt dividerIndex: Int) -> CGFloat {
            owner.length - 80
        }
    }
}
