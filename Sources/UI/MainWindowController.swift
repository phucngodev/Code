import AppKit
import GhosttyKit

/// One window: workspace sidebar on the left, terminal area on the right.
final class MainWindowController: NSWindowController {
    let sidebar: SidebarViewController
    let terminalArea = TerminalAreaViewController()
    private let store: WorkspaceStore
    private let content: WindowContentViewController
    private(set) var commandPalette: CommandPaletteView?
    var agentIndicator: @MainActor (UUID) -> AgentIndicator? = { _ in nil }
    /// How many terminals in this window have unread alerts, for the bell.
    var unreadCount: @MainActor () -> Int = { 0 }
    /// Brings the agent that needs the user most recently to the front.
    var onJumpToLatestUnread: @MainActor () -> Void = {}
    var onResetZoom: (() -> Void)?
    var onNewWorkspace: (() -> Void)?
    /// Splits the focused pane of the current tab, from the titlebar buttons.
    var onSplit: ((SplitDirection) -> Void)?
    private let sidebarAccessory = NSTitlebarAccessoryViewController()
    private let tabAccessory = NSTitlebarAccessoryViewController()
    private let zoomAccessory = NSTitlebarAccessoryViewController()
    private var resizeObserver: NSObjectProtocol?
    private let resetZoomButton = NSButton(
        image: NSImage(systemSymbolName: "arrow.down.right.and.arrow.up.left", accessibilityDescription: "Reset zoom")!,
        target: nil,
        action: nil)
    private let bellBadge = UnreadBadgeView()
    /// Split the current tab from the titlebar, beside the zoom button.
    private let splitRightButton = NSButton(
        image: NSImage(systemSymbolName: "square.split.2x1", accessibilityDescription: "Split right")!,
        target: nil,
        action: nil)
    private let splitDownButton = NSButton(
        image: NSImage(systemSymbolName: "square.split.1x2", accessibilityDescription: "Split down")!,
        target: nil,
        action: nil)
    /// The hairline under the titlebar, split around the selected tab so its
    /// card still meets the content. The tab strip draws its own only where it
    /// is, so without these the line would stop at the sidebar and the zoom
    /// button and the sides would read a different colour.
    private let titlebarSeparatorLeft = TitlebarSeparatorView()
    private let titlebarSeparatorRight = TitlebarSeparatorView()

    /// Without a saved frame the window opens offset from the frontmost one.
    init(store: WorkspaceStore, frame: CGRect?) {
        self.store = store
        self.sidebar = SidebarViewController(store: store)

        self.content = WindowContentViewController(sidebar: sidebar, content: terminalArea)

        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1100, height: 700),
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered,
            defer: false)
        window.toolbarStyle = .unified
        window.contentViewController = content
        window.setContentSize(NSSize(width: 1100, height: 700))
        window.minSize = NSSize(width: 600, height: 400)
        window.isReleasedWhenClosed = false
        if let frame {
            window.setFrame(Session.Window.placing(frame, on: NSScreen.screens.map(\.visibleFrame)), display: false)
        } else if let front = NSApp.keyWindow ?? NSApp.mainWindow {
            window.setFrame(front.frame.offsetBy(dx: 24, dy: -24), display: false)
        } else {
            window.center()
        }

        super.init(window: window)

        resetZoomButton.target = self
        resetZoomButton.action = #selector(resetZoomTapped)
        resetZoomButton.isBordered = false
        resetZoomButton.toolTip = "Show all panes"
        resetZoomButton.isHidden = true
        for (button, tooltip, action) in [
            (splitRightButton, "Split right (⌘D)", #selector(splitRightTapped)),
            (splitDownButton, "Split down (⇧⌘D)", #selector(splitDownTapped)),
        ] as [(NSButton, String, Selector)] {
            button.target = self
            button.action = action
            button.isBordered = false
            button.toolTip = tooltip
        }
        // The zoom button is hidden unless a pane is zoomed, so it goes first:
        // the split buttons then sit at the trailing edge, where they are easy
        // to reach, with no reserved gap to their right.
        zoomAccessory.view = Self.accessoryRow([resetZoomButton, splitRightButton, splitDownButton])
        zoomAccessory.layoutAttribute = .trailing
        window.addTitlebarAccessoryViewController(zoomAccessory)

        // In the title bar rather than the sidebar, so the toggle is still
        // there to bring the sidebar back once it is collapsed.
        sidebarAccessory.view = Self.accessoryRow([
            titlebarButton(symbol: "sidebar.left", description: "Toggle sidebar", action: #selector(toggleWorkspaceSidebar)),
            titlebarButton(symbol: "folder.badge.plus", description: "New workspace", action: #selector(newWorkspaceTapped)),
            bellItem(),
        ])
        sidebarAccessory.layoutAttribute = .leading
        window.addTitlebarAccessoryViewController(sidebarAccessory)

        // The tabs live in the titlebar, after the sidebar buttons, so the
        // window title steps aside and the titlebar's separator goes with it,
        // letting the selected tab merge into the content below.
        window.titleVisibility = .hidden
        window.titlebarSeparatorStyle = .none
        tabAccessory.view = terminalArea.tabBar
        tabAccessory.layoutAttribute = .leading
        window.addTitlebarAccessoryViewController(tabAccessory)

        installTitlebarSeparators()
        updateTabBarLayout()
        resizeObserver = NotificationCenter.default.addObserver(
            forName: NSWindow.didResizeNotification, object: window, queue: .main
        ) { [weak self] _ in self?.updateTabBarLayout() }
    }

    deinit {
        resizeObserver.map(NotificationCenter.default.removeObserver)
    }

    /// What the traffic lights take at the left of the titlebar, used until
    /// the leading accessory's own position is known.
    private static let trafficLightsWidth: CGFloat = 78
    /// Space kept between the strip and the zoom button at the right.
    private static let titlebarMargin: CGFloat = 24

    /// The strip spans the titlebar so its bottom border runs across, and fills
    /// its height so the selected tab meets the content below it.
    private func updateTabBarLayout() {
        guard let window else { return }
        let content = NSWindow.contentRect(forFrameRect: window.frame, styleMask: window.styleMask)
        terminalArea.tabBar.titlebarHeight = window.frame.height - content.height
        // The titlebar puts the strip in a clip view that starts after the
        // traffic lights and the sidebar buttons, so its origin is the leading
        // inset, and the zoom button takes the trailing one.
        let leading = tabAccessory.view.superview?.frame.minX
            ?? (Self.trafficLightsWidth + sidebarAccessory.view.frame.width)
        let trailing = zoomAccessory.view.frame.width + Self.titlebarMargin
        terminalArea.tabBar.titlebarWidth = max(window.frame.width - leading - trailing, 0)
        updateTitlebarSeparators()
    }

    /// Puts the two separator halves into the window's frame view, where they
    /// can span the titlebar's full width rather than only the tab strip.
    private func installTitlebarSeparators() {
        guard let host = window?.contentView?.superview else { return }
        for separator in [titlebarSeparatorLeft, titlebarSeparatorRight] {
            separator.frame.size.height = 1
            host.addSubview(separator)
        }
    }

    /// Lays the two halves along the seam, leaving the selected tab's card
    /// uncovered so it still joins the content below.
    private func updateTitlebarSeparators() {
        guard let window, !window.styleMask.contains(.fullScreen),
              let host = titlebarSeparatorLeft.superview, let content = window.contentView else {
            titlebarSeparatorLeft.isHidden = true
            titlebarSeparatorRight.isHidden = true
            return
        }
        let y = content.frame.maxY
        let width = host.bounds.width
        let gap = terminalArea.tabBar.selectedCardFrame.map { host.convert($0, from: nil) }
        let leftWidth = gap.map { min(max($0.minX, 0), width) } ?? width
        let rightStart = gap.map { min(max($0.maxX, 0), width) } ?? width
        titlebarSeparatorLeft.frame = NSRect(x: 0, y: y, width: leftWidth, height: 1)
        titlebarSeparatorRight.frame = NSRect(x: rightStart, y: y, width: max(0, width - rightStart), height: 1)
        titlebarSeparatorLeft.isHidden = leftWidth <= 0
        titlebarSeparatorRight.isHidden = rightStart >= width
    }

    private func titlebarButton(symbol: String, description: String, action: Selector) -> NSButton {
        let button = NSButton(image: NSImage(systemSymbolName: symbol, accessibilityDescription: description)!, target: self, action: action)
        button.isBordered = false
        button.toolTip = description
        button.contentTintColor = .secondaryLabelColor
        return button
    }

    /// A titlebar accessory view must keep a zero origin: AppKit raises when
    /// a screen change re-lays out the titlebar and Auto Layout re-centres the
    /// view from its fitting size. A plain frame-based row has no fitting size
    /// of its own, so only the items inside it move.
    private static func accessoryRow(_ items: [NSView]) -> NSView {
        let buttonSize = NSSize(width: 32, height: 28)
        let row = NSView(frame: NSRect(x: 0, y: 0, width: buttonSize.width * CGFloat(items.count), height: buttonSize.height))
        for (index, item) in items.enumerated() {
            item.frame = NSRect(x: buttonSize.width * CGFloat(index), y: 0, width: buttonSize.width, height: buttonSize.height)
            item.autoresizingMask = [.minYMargin, .maxYMargin]
            row.addSubview(item)
        }
        return row
    }

    /// The bell and its unread badge as one titlebar item.
    private func bellItem() -> NSView {
        let container = NSView(frame: NSRect(x: 0, y: 0, width: 32, height: 28))
        let button = titlebarButton(symbol: "bell", description: "Jump to the agent that needs you", action: #selector(jumpToLatestUnread))
        button.frame = container.bounds
        button.autoresizingMask = [.width, .height]
        container.addSubview(button)
        bellBadge.translatesAutoresizingMaskIntoConstraints = false
        container.addSubview(bellBadge)
        NSLayoutConstraint.activate([
            bellBadge.trailingAnchor.constraint(equalTo: container.trailingAnchor, constant: -2),
            bellBadge.topAnchor.constraint(equalTo: container.topAnchor, constant: 1),
        ])
        return container
    }

    func refreshUnreadBadge() {
        bellBadge.count = unreadCount()
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) is not supported")
    }

    /// Ghostty renders the terminal background with the configured opacity, but
    /// only a non-opaque window lets the desktop show through it.
    func applyAppearance(config: GhosttyConfig, app: ghostty_app_t) {
        guard let window else { return }
        window.appearance = config.windowAppearance
        let chrome = Self.chromeColor(for: config.backgroundColor)
        window.titlebarAppearsTransparent = true
        terminalArea.tabBar.contentColor = config.backgroundColor
        terminalArea.tabBar.backgroundColor = chrome
        let dim = config.unfocusedSplitDimming
        terminalArea.paneTreeView.unfocusedFill = dim > 0.001 ? config.unfocusedSplitFill.withAlphaComponent(dim) : nil
        terminalArea.paneTreeView.dividerColor = config.splitDividerColor
        terminalArea.show(store.selected)
        if config.backgroundOpacity < 1 {
            window.isOpaque = false
            window.backgroundColor = .white.withAlphaComponent(0.001)
            // Applies the configured radius, so a reload with blur off clears it.
            ghostty_set_window_background_blur(app, Unmanaged.passUnretained(window).toOpaque())
        } else {
            window.isOpaque = true
            window.backgroundColor = chrome
        }
    }

    /// The titlebar's chrome: the terminal background shifted a little away
    /// from the content, darker on a light theme and lighter on a dark one, so
    /// the content-colored tab reads against it without the chrome matching the
    /// sidebar's selection.
    private static func chromeColor(for background: NSColor) -> NSColor {
        background.isLightColor ? background.darken(by: 0.05) : background.lighten(by: 0.06)
    }

    /// Re-reads the store and updates the sidebar, tab strip, and window title.
    func refresh() {
        sidebar.reload()
        terminalArea.show(store.selected)
        updateWindowTitle()
        refreshUnreadBadge()
        updateTitlebarSeparators()
        // The accessory controller's own isHidden is not honored in the
        // unified titlebar, so the button itself is hidden.
        resetZoomButton.isHidden = store.selected?.selectedTab?.panes.zoomed == nil
    }

    func agentsDidChange(in workspace: Workspace, tab: TerminalTab, surface: UUID) {
        sidebar.updateAgentStatus(for: workspace)
        sidebar.updateTerminalIndicator(for: surface)
        terminalArea.updateAgentIndicator(for: tab)
        // The bell counts an agent that is waiting, which changes with the
        // indicator rather than with unread alone.
        refreshUnreadBadge()
    }

    func titleDidChange(of surface: TerminalSurfaceView) {
        guard let workspace = store.selected,
              workspace.tabs.contains(where: { $0.panes.surfaces.contains { $0 === surface } }) else { return }
        terminalArea.reloadTabs()
        sidebar.reloadAgents()
        updateWindowTitle()
    }

    private func updateWindowTitle() {
        let workspace = store.selected?.name ?? "Code"
        let tab = store.selected?.selectedTab?.title
        window?.title = tab.map { "\(workspace) — \($0)" } ?? workspace
    }

    /// A rename is typed into the workspace's row, so a hidden sidebar is
    /// brought back for it.
    func beginRename(of workspace: Workspace) {
        if content.isSidebarHidden {
            content.isSidebarHidden = false
            window?.layoutIfNeeded()
        }
        sidebar.beginRename(of: workspace)
    }

    /// Covers the whole window, so a click anywhere outside the palette cancels it.
    func showCommandPalette(_ palette: CommandPaletteView) {
        closeCommandPalette()
        commandPalette = palette
        palette.translatesAutoresizingMaskIntoConstraints = false
        content.view.addSubview(palette)
        NSLayoutConstraint.activate([
            palette.leadingAnchor.constraint(equalTo: content.view.leadingAnchor),
            palette.trailingAnchor.constraint(equalTo: content.view.trailingAnchor),
            palette.topAnchor.constraint(equalTo: content.view.topAnchor),
            palette.bottomAnchor.constraint(equalTo: content.view.bottomAnchor),
        ])
        palette.focusField()
    }

    func closeCommandPalette() {
        commandPalette?.removeFromSuperview()
        commandPalette = nil
    }

    @objc func toggleWorkspaceSidebar(_ sender: Any?) {
        content.toggleSidebar()
    }

    /// Opens just below the button.
    @objc private func newWorkspaceTapped() {
        onNewWorkspace?()
    }

    @objc func resetZoomTapped() {
        onResetZoom?()
    }

    @objc private func splitRightTapped() {
        onSplit?(.right)
    }

    @objc private func splitDownTapped() {
        onSplit?(.down)
    }

    @objc private func jumpToLatestUnread() {
        onJumpToLatestUnread()
    }
}

/// A hairline under the titlebar, repainted in each layer update so it follows
/// the appearance between light and dark.
private final class TitlebarSeparatorView: NSView {
    override var wantsUpdateLayer: Bool { true }

    override func updateLayer() {
        layer?.backgroundColor = NSColor.separatorColor.cgColor
    }
}
