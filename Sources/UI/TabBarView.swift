import AppKit

protocol TabBarViewDelegate: AnyObject {
    func tabBar(_ tabBar: TabBarView, didSelectTabAt index: Int)
    func tabBar(_ tabBar: TabBarView, didCloseTabAt index: Int)
    func tabBarDidRequestNewTab(_ tabBar: TabBarView)
}

/// A row of closable tabs, hosted in the window's titlebar. The selected tab is
/// a card in the terminal's background color that meets the content below it;
/// the others sit flat on the titlebar.
final class TabBarView: NSView {
    weak var delegate: TabBarViewDelegate?

    /// The terminal's background, which the selected tab's card is filled with
    /// so it reads as joined to the content below.
    var contentColor: NSColor = .windowBackgroundColor {
        didSet {
            items.forEach { $0.cardColor = contentColor }
            needsDisplay = true
        }
    }

    /// The chrome behind the tabs, a shade off the terminal background so the
    /// selected card stands out on both light and dark themes.
    var backgroundColor: NSColor = .clear {
        didSet {
            guard backgroundColor != oldValue else { return }
            needsDisplay = true
        }
    }

    /// The strip's width, spanning the titlebar between the sidebar buttons and
    /// the zoom button, so the bottom border runs across. Set by the controller.
    var titlebarWidth: CGFloat = 0 {
        didSet {
            guard titlebarWidth != oldValue else { return }
            fitToTitlebar()
        }
    }

    /// The titlebar's height, so the selected card can meet the content below.
    var titlebarHeight: CGFloat = 0 {
        didSet {
            guard titlebarHeight != oldValue else { return }
            fitToTitlebar()
        }
    }

    private let stack = NSStackView()
    private var items: [TabItemView] = []
    private let newTabButton = NSButton()

    override init(frame: NSRect) {
        super.init(frame: frame)
        stack.orientation = .horizontal
        stack.spacing = 0
        stack.edgeInsets = NSEdgeInsets(top: 0, left: 6, bottom: 0, right: 6)
        stack.translatesAutoresizingMaskIntoConstraints = false
        addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: leadingAnchor),
            stack.topAnchor.constraint(equalTo: topAnchor),
            stack.bottomAnchor.constraint(equalTo: bottomAnchor),
            stack.trailingAnchor.constraint(lessThanOrEqualTo: trailingAnchor),
        ])

        newTabButton.image = NSImage(systemSymbolName: "plus", accessibilityDescription: "New tab")
        newTabButton.isBordered = false
        newTabButton.imagePosition = .imageOnly
        newTabButton.symbolConfiguration = .init(pointSize: 12, weight: .medium)
        newTabButton.contentTintColor = .secondaryLabelColor
        newTabButton.toolTip = "New tab"
        newTabButton.target = self
        newTabButton.action = #selector(newTabTapped)
        newTabButton.translatesAutoresizingMaskIntoConstraints = false
        stack.addArrangedSubview(newTabButton)
        NSLayoutConstraint.activate([
            newTabButton.widthAnchor.constraint(equalToConstant: 30),
            newTabButton.heightAnchor.constraint(equalTo: heightAnchor),
        ])
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) is not supported")
    }

    /// A hairline along the bottom joins the titlebar to the content. It is
    /// skipped under the selected card, which meets the content instead.
    override func draw(_ dirtyRect: NSRect) {
        guard bounds.width > 0 else { return }
        backgroundColor.setFill()
        bounds.fill()
        NSColor.separatorColor.setFill()
        let y = bounds.minY
        let line = { (x: CGFloat, width: CGFloat) in
            guard width > 0 else { return }
            NSRect(x: x, y: y, width: width, height: 1).fill()
        }
        if let selected = items.first(where: \.isSelected).map({ cardRect(in: convert($0.bounds, from: $0)) }) {
            line(bounds.minX, selected.minX - bounds.minX)
            line(selected.maxX, bounds.maxX - selected.maxX)
        } else {
            line(bounds.minX, bounds.width)
        }
    }

    /// The number of tabs shown, not counting the new-tab button.
    var tabCount: Int { items.count }

    /// The selected tab's card in window coordinates, so the window can leave a
    /// gap in the titlebar's separator under it, where the card meets content.
    var selectedCardFrame: NSRect? {
        guard let selected = items.first(where: \.isSelected) else { return nil }
        return convert(cardRect(in: convert(selected.bounds, from: selected)), to: nil)
    }

    /// Titles change with every shell prompt, so items are kept and updated;
    /// views are only added or removed when the number of tabs changes.
    func reload(titles: [String], indicators: [AgentIndicator?], agents: [Bool] = [], selectedIndex: Int?) {
        while items.count > titles.count {
            let item = items.removeLast()
            stack.removeArrangedSubview(item)
            item.removeFromSuperview()
        }
        while items.count < titles.count {
            let item = TabItemView()
            item.cardColor = contentColor
            item.onSelect = { [weak self, weak item] in
                guard let self, let item, let index = self.items.firstIndex(of: item) else { return }
                self.delegate?.tabBar(self, didSelectTabAt: index)
            }
            item.onClose = { [weak self, weak item] in
                guard let self, let item, let index = self.items.firstIndex(of: item) else { return }
                self.delegate?.tabBar(self, didCloseTabAt: index)
            }
            // Before the new-tab button, which always trails the tabs.
            stack.insertArrangedSubview(item, at: items.count)
            items.append(item)
            item.heightAnchor.constraint(equalTo: heightAnchor).isActive = true
        }
        for (index, item) in items.enumerated() {
            item.title = titles[index]
            item.agentIndicator = indicators[index]
            item.isAgentTab = agents.indices.contains(index) ? agents[index] : false
            item.isSelected = index == selectedIndex
            // A divider only between tabs that are both inactive, so none
            // touches the selected card.
            item.showsTrailingSeparator = index < items.count - 1 && index != selectedIndex && index + 1 != selectedIndex
        }
        needsDisplay = true
        fitToTitlebar()
    }

    /// The titlebar accessory is frame-based. Under Auto Layout, as in tests,
    /// the surrounding stack sizes it instead.
    private func fitToTitlebar() {
        guard translatesAutoresizingMaskIntoConstraints else { return }
        layoutSubtreeIfNeeded()
        let width = titlebarWidth > 0 ? titlebarWidth : fittingSize.width
        frame.size = NSSize(width: width, height: max(resolvedTitlebarHeight, fittingSize.height))
    }

    /// The titlebar's height, read from the window so it is never stale, and
    /// falling back to what the controller set.
    private var resolvedTitlebarHeight: CGFloat {
        guard let window, window.frame.height > 0 else { return titlebarHeight }
        let content = NSWindow.contentRect(forFrameRect: window.frame, styleMask: window.styleMask)
        return max(window.frame.height - content.height, titlebarHeight)
    }

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        fitToTitlebar()
    }

    @objc private func newTabTapped() {
        delegate?.tabBarDidRequestNewTab(self)
    }
}

/// The selected card is inset from the top and flush with the bottom, so it
/// joins the terminal content below.
private let cardTopInset: CGFloat = 4
private let cardCornerRadius: CGFloat = 7

func cardRect(in tab: NSRect) -> NSRect {
    NSRect(x: tab.minX, y: tab.minY, width: tab.width, height: max(tab.height - cardTopInset, 0))
}

/// A rectangle rounded only at the top, so the card meets the content below.
func cardPath(in rect: NSRect) -> NSBezierPath {
    let path = cardTopPath(in: rect)
    path.line(to: NSPoint(x: rect.minX, y: rect.minY))
    path.close()
    return path
}

/// The card's top edge and sides as an open path, for stroking a border that
/// runs down to the content without drawing across the bottom.
func cardTopPath(in rect: NSRect) -> NSBezierPath {
    let radius = min(cardCornerRadius, min(rect.width / 2, rect.height))
    let path = NSBezierPath()
    path.move(to: NSPoint(x: rect.minX, y: rect.minY))
    path.line(to: NSPoint(x: rect.minX, y: rect.maxY - radius))
    path.appendArc(withCenter: NSPoint(x: rect.minX + radius, y: rect.maxY - radius),
                   radius: radius, startAngle: 180, endAngle: 90, clockwise: true)
    path.line(to: NSPoint(x: rect.maxX - radius, y: rect.maxY))
    path.appendArc(withCenter: NSPoint(x: rect.maxX - radius, y: rect.maxY - radius),
                   radius: radius, startAngle: 90, endAngle: 0, clockwise: true)
    path.line(to: NSPoint(x: rect.maxX, y: rect.minY))
    return path
}

private final class TabItemView: NSView {
    var onSelect: (() -> Void)?
    var onClose: (() -> Void)?
    private let label = NSTextField(labelWithString: "")
    private let close: NSButton
    private let tabIcon = NSImageView()
    private let agentDot = AgentDotView()
    private var trackingArea: NSTrackingArea?
    private var isHovered = false {
        didSet {
            updateClose()
            needsDisplay = true
        }
    }

    /// The terminal background, painted as the card when this tab is selected.
    var cardColor: NSColor = .windowBackgroundColor {
        didSet { needsDisplay = true }
    }

    /// A divider on the trailing edge, set by the tab bar so none touches the card.
    var showsTrailingSeparator = false {
        didSet { if showsTrailingSeparator != oldValue { needsDisplay = true } }
    }

    var title: String {
        get { label.stringValue }
        set { if newValue != label.stringValue { label.stringValue = newValue } }
    }

    var agentIndicator: AgentIndicator? {
        get { agentDot.indicator }
        set {
            agentDot.indicator = newValue
            agentDot.isHidden = newValue == nil
        }
    }

    /// Whether an agent runs in this tab: the icon is a sparkle then, and the
    /// plain terminal glyph otherwise.
    var isAgentTab = false {
        didSet {
            guard isAgentTab != oldValue else { return }
            updateIcon()
        }
    }

    var isSelected = false {
        didSet {
            guard isSelected != oldValue else { return }
            updateStyle()
            needsDisplay = true
        }
    }

    init() {
        self.close = NSButton(image: NSImage(systemSymbolName: "xmark", accessibilityDescription: "Close tab")!,
                              target: nil, action: nil)
        super.init(frame: .zero)

        updateStyle()
        label.font = .systemFont(ofSize: 13)
        label.lineBreakMode = .byTruncatingTail
        label.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
        label.setContentHuggingPriority(.defaultLow, for: .horizontal)

        close.target = self
        close.action = #selector(closeTapped)
        close.isBordered = false
        close.imagePosition = .imageOnly
        close.symbolConfiguration = .init(pointSize: 10, weight: .semibold)
        close.contentTintColor = .secondaryLabelColor
        close.setContentHuggingPriority(.required, for: .horizontal)

        agentDot.isHidden = true
        updateIcon()
        tabIcon.symbolConfiguration = .init(pointSize: 11, weight: .medium)
        tabIcon.contentTintColor = .secondaryLabelColor
        tabIcon.setContentHuggingPriority(.required, for: .horizontal)
        let stack = NSStackView(views: [tabIcon, agentDot, label, close])
        stack.orientation = .horizontal
        stack.distribution = .fill
        stack.spacing = 6
        stack.edgeInsets = NSEdgeInsets(top: 0, left: 8, bottom: 0, right: 8)
        stack.translatesAutoresizingMaskIntoConstraints = false
        addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: leadingAnchor),
            stack.trailingAnchor.constraint(equalTo: trailingAnchor),
            stack.topAnchor.constraint(equalTo: topAnchor),
            stack.bottomAnchor.constraint(equalTo: bottomAnchor),
            widthAnchor.constraint(lessThanOrEqualToConstant: 220),
            widthAnchor.constraint(greaterThanOrEqualToConstant: 110),
        ])
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) is not supported")
    }

    override func draw(_ dirtyRect: NSRect) {
        if isSelected {
            let card = cardRect(in: bounds)
            cardColor.setFill()
            cardPath(in: card).fill()
            // A hairline along the top and sides lifts the card off the
            // titlebar, where the two colors are close on a light theme.
            NSColor.separatorColor.setStroke()
            let border = cardTopPath(in: card)
            border.lineWidth = 1
            border.stroke()
        } else if isHovered {
            NSColor.labelColor.withAlphaComponent(0.06).setFill()
            cardPath(in: cardRect(in: bounds)).fill()
        }
        if showsTrailingSeparator, !isHovered {
            NSColor.separatorColor.setFill()
            NSRect(x: bounds.maxX - 1, y: 7, width: 1, height: bounds.height - 14).fill()
        }
    }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let trackingArea { removeTrackingArea(trackingArea) }
        let area = NSTrackingArea(rect: bounds, options: [.mouseEnteredAndExited, .activeInKeyWindow, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        trackingArea = area
    }

    override func mouseEntered(with event: NSEvent) {
        isHovered = true
    }

    override func mouseExited(with event: NSEvent) {
        isHovered = false
    }

    private func updateStyle() {
        label.textColor = isSelected ? .labelColor : .secondaryLabelColor
        tabIcon.contentTintColor = isSelected ? .labelColor : .secondaryLabelColor
        updateClose()
    }

    private func updateIcon() {
        tabIcon.image = NSImage(systemSymbolName: isAgentTab ? "sparkle" : "terminal",
                                accessibilityDescription: isAgentTab ? "Agent" : "Terminal")
    }

    /// Hiding the button would drop it from the stack and reflow the title,
    /// so it stays in place and only fades out.
    private func updateClose() {
        let visible = isSelected || isHovered
        close.alphaValue = visible ? 1 : 0
        close.isEnabled = visible
    }

    override func mouseDown(with event: NSEvent) {
        onSelect?()
    }

    @objc private func closeTapped() {
        onClose?()
    }
}
