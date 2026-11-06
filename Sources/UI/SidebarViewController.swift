import AppKit

protocol SidebarViewControllerDelegate: AnyObject {
    func sidebar(_ sidebar: SidebarViewController, didSelect workspace: Workspace)
    /// The name is nil when the rename was abandoned.
    func sidebar(_ sidebar: SidebarViewController, didFinishRenaming workspace: Workspace, to name: String?)
    func sidebar(_ sidebar: SidebarViewController, wantsClose workspaces: [Workspace])
    /// A terminal in the agents section was chosen.
    func sidebar(_ sidebar: SidebarViewController, didSelectTerminal terminal: UUID)
    /// A saved session in the history section was chosen.
    func sidebar(_ sidebar: SidebarViewController, didSelectHistory session: HistorySession)
}

/// Told when a sidebar is shown or collapsed, so it can free the panes a
/// hidden sidebar has no use for. The window drives this from the view's
/// hidden state, which is the only place that knows.
protocol SidebarVisibilityDelegate: AnyObject {
    func sidebarVisibilityDidChange(hidden: Bool)
}

/// Where a divider between the sidebar's panes may sit, in the split view's own
/// coordinates, so no pane is ever squeezed below the minimum. `NSSplitView` is
/// flipped, so it measures from the top and the panes before the divider are
/// the ones nearer the origin.
enum SidebarDividers {
    /// Panes before the divider, on the origin side.
    private static func above(_ dividerIndex: Int) -> Int { dividerIndex + 1 }
    /// Panes after it.
    private static func below(_ dividerIndex: Int, panes: Int) -> Int { panes - 1 - dividerIndex }

    static func minimum(dividerIndex: Int, panes: Int, flipped: Bool, minPaneHeight: CGFloat) -> CGFloat {
        let side = flipped ? above(dividerIndex) : below(dividerIndex, panes: panes)
        return CGFloat(side) * minPaneHeight
    }

    static func maximum(dividerIndex: Int, panes: Int, flipped: Bool, minPaneHeight: CGFloat, height: CGFloat) -> CGFloat {
        let side = flipped ? below(dividerIndex, panes: panes) : above(dividerIndex)
        return max(height - CGFloat(side) * minPaneHeight, minPaneHeight)
    }
}

/// The left of the window: workspaces on top, the selected workspace's
/// terminals in the middle, and the agents' saved sessions at the bottom, with
/// draggable dividers between them. The panes keep their own scroll position
/// and the dividers' are remembered. Nothing is built until the sidebar is
/// shown, and everything is released once it is collapsed.
final class SidebarViewController: NSViewController, NSSplitViewDelegate, SidebarVisibilityDelegate {
    weak var delegate: SidebarViewControllerDelegate?

    /// The most urgent status of a workspace's terminals.
    var agentStatus: @MainActor (Workspace) -> AgentStatus? = { _ in nil } {
        didSet { content?.workspaceList.agentStatus = agentStatus }
    }

    /// How many of a workspace's terminals have unread alerts.
    var agentUnread: @MainActor (Workspace) -> Int = { _ in 0 } {
        didSet { content?.workspaceList.agentUnread = agentUnread }
    }

    var terminalAgent: @MainActor (UUID) -> String? = { _ in nil } {
        didSet { content?.agentList.agentName = terminalAgent }
    }

    var terminalIndicator: @MainActor (UUID) -> AgentIndicator? = { _ in nil } {
        didSet { content?.agentList.agentIndicator = terminalIndicator }
    }

    private static let workspacesHeightKey = "SidebarWorkspacesHeight"
    private static let historyHeightKey = "SidebarHistoryHeight"
    /// The agents pane's height the first time, so it is neither half the
    /// sidebar nor a single compressed row.
    private static let defaultAgentsHeight: CGFloat = 168
    /// The history pane starts short: it is a convenience, not the main work.
    private static let defaultHistoryHeight: CGFloat = 150
    /// Neither pane may be squeezed away; the divider stops here.
    private static let minPaneHeight: CGFloat = 88

    private let store: WorkspaceStore
    private let defaults: UserDefaults
    private var content: Content?
    /// Set once the saved dividers have been applied, so restoring them is not
    /// mistaken for the user moving them.
    private var didRestoreDivider = false

    /// The panes, built only while the sidebar is visible. Releasing this frees
    /// the tables, cells and file watchers a collapsed sidebar never draws.
    private struct Content {
        let splitView: SidebarSplitView
        let workspaceList: WorkspaceListViewController
        let agentList: AgentListViewController
        let historyList: HistoryListViewController
    }

    init(store: WorkspaceStore, defaults: UserDefaults = .standard) {
        self.store = store
        self.defaults = defaults
        super.init(nibName: nil, bundle: nil)
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) is not supported")
    }

    /// Whether the panes are currently built; used by tests to hold the
    /// visibility behaviour in place.
    var isContentLoaded: Bool { content != nil }

    override func loadView() {
        let container = NSVisualEffectView()
        container.material = .sidebar
        container.blendingMode = .behindWindow
        view = container
    }

    override func viewDidLayout() {
        super.viewDidLayout()
        restoreDividerIfNeeded()
    }

    // MARK: Visibility

    func sidebarVisibilityDidChange(hidden: Bool) {
        if hidden {
            removeContent()
        } else {
            buildContent()
        }
    }

    private func buildContent() {
        guard content == nil else { return }
        let workspaceList = WorkspaceListViewController(store: store)
        workspaceList.delegate = self
        workspaceList.agentStatus = agentStatus
        workspaceList.agentUnread = agentUnread
        let agentList = AgentListViewController(store: store)
        agentList.delegate = self
        agentList.agentName = terminalAgent
        agentList.agentIndicator = terminalIndicator
        let historyList = HistoryListViewController()
        historyList.delegate = self

        let splitView = SidebarSplitView()
        splitView.isVertical = false
        splitView.dividerStyle = .thin
        splitView.delegate = self
        splitView.translatesAutoresizingMaskIntoConstraints = false
        addChild(workspaceList)
        addChild(agentList)
        addChild(historyList)
        splitView.addSubview(workspaceList.view)
        splitView.addSubview(agentList.view)
        splitView.addSubview(historyList.view)

        // The sidebar material lets a bright desktop colour the whole sidebar,
        // so a wash of the window colour keeps only a hint of it.
        let tint = SidebarTint()
        tint.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(tint)
        view.addSubview(splitView)
        NSLayoutConstraint.activate([
            tint.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            tint.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            tint.topAnchor.constraint(equalTo: view.topAnchor),
            tint.bottomAnchor.constraint(equalTo: view.bottomAnchor),
            splitView.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            splitView.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            splitView.topAnchor.constraint(equalTo: view.topAnchor),
            splitView.bottomAnchor.constraint(equalTo: view.bottomAnchor),
        ])

        content = Content(splitView: splitView, workspaceList: workspaceList, agentList: agentList, historyList: historyList)
        didRestoreDivider = false
        workspaceList.reload()
        agentList.reload()
        historyList.load()
        view.layoutSubtreeIfNeeded()
        restoreDividerIfNeeded()
    }

    private func removeContent() {
        guard let content else { return }
        content.splitView.removeFromSuperview()
        content.workspaceList.removeFromParent()
        content.agentList.removeFromParent()
        content.historyList.removeFromParent()
        // The tint is the only other subview; removing it drops its constraints.
        view.subviews.forEach { $0.removeFromSuperview() }
        self.content = nil
        didRestoreDivider = false
    }

    // MARK: Refreshing

    func reload() {
        guard let content else { return }
        content.workspaceList.reload()
        content.agentList.reload()
    }

    /// Refreshes only the agents pane, for a change that leaves the workspaces
    /// alone, such as a terminal's own title.
    func reloadAgents() {
        content?.agentList.reload()
    }

    func beginRename(of workspace: Workspace) {
        content?.workspaceList.beginRename(of: workspace)
    }

    /// Agents change state far more often than rows need rebuilding, so the
    /// workspace mark and the terminal row are updated in place.
    func updateAgentStatus(for workspace: Workspace) {
        content?.workspaceList.updateAgentStatus(for: workspace)
    }

    /// Repaints unread badges after a terminal's alerts change.
    func refreshAgentBadges() {
        content?.workspaceList.refreshAgentState()
    }

    // MARK: Divider

    /// The saved dividers are pane heights from the top of the split view. They
    /// are applied once the split view has the height to clamp against.
    private func restoreDividerIfNeeded() {
        guard let splitView = content?.splitView,
              !didRestoreDivider, splitView.bounds.height >= Self.minPaneHeight * 3 else { return }
        didRestoreDivider = true
        let total = splitView.bounds.height
        let savedTop = (defaults.object(forKey: Self.workspacesHeightKey) as? NSNumber)?.doubleValue
        let top = min(max(savedTop.map { CGFloat($0) } ?? (total - Self.defaultAgentsHeight - Self.defaultHistoryHeight),
                         Self.minPaneHeight), total - Self.minPaneHeight * 2)
        let savedHistory = (defaults.object(forKey: Self.historyHeightKey) as? NSNumber)?.doubleValue
        let history = min(max(savedHistory.map { CGFloat($0) } ?? Self.defaultHistoryHeight, Self.minPaneHeight),
                          total - top - Self.minPaneHeight)
        // NSSplitView measures a divider from the top when it is flipped and
        // from the bottom otherwise, so a pane height is turned into a position.
        // The history divider goes first: fixing the top one shrinks the agents
        // pane between them and leaves it where it was put.
        splitView.setPosition(splitView.isFlipped ? total - history : history, ofDividerAt: 1)
        splitView.setPosition(splitView.isFlipped ? top : total - top, ofDividerAt: 0)
    }

    func splitView(_ splitView: NSSplitView, constrainMinCoordinate proposedMinimumPosition: CGFloat, ofSubviewAt dividerIndex: Int) -> CGFloat {
        SidebarDividers.minimum(dividerIndex: dividerIndex, panes: splitView.subviews.count,
                                flipped: splitView.isFlipped, minPaneHeight: Self.minPaneHeight)
    }

    func splitView(_ splitView: NSSplitView, constrainMaxCoordinate proposedMaximumPosition: CGFloat, ofSubviewAt dividerIndex: Int) -> CGFloat {
        SidebarDividers.maximum(dividerIndex: dividerIndex, panes: splitView.subviews.count, flipped: splitView.isFlipped,
                                minPaneHeight: Self.minPaneHeight, height: splitView.bounds.height)
    }

    func splitView(_ splitView: NSSplitView, canCollapseSubview subview: NSView) -> Bool {
        false
    }

    func splitViewDidResizeSubviews(_ notification: Notification) {
        guard didRestoreDivider, let subviews = content?.splitView.subviews, subviews.count == 3 else { return }
        defaults.set(Double(subviews[0].frame.height), forKey: Self.workspacesHeightKey)
        defaults.set(Double(subviews[2].frame.height), forKey: Self.historyHeightKey)
    }
}

// MARK: - Pane delegates

extension SidebarViewController: WorkspaceListViewControllerDelegate {
    func workspaceList(_ list: WorkspaceListViewController, didSelect workspace: Workspace) {
        delegate?.sidebar(self, didSelect: workspace)
    }

    func workspaceList(_ list: WorkspaceListViewController, didFinishRenaming workspace: Workspace, to name: String?) {
        delegate?.sidebar(self, didFinishRenaming: workspace, to: name)
    }

    func workspaceList(_ list: WorkspaceListViewController, wantsClose workspaces: [Workspace]) {
        delegate?.sidebar(self, wantsClose: workspaces)
    }
}

extension SidebarViewController: AgentListViewControllerDelegate {
    func agentList(_ list: AgentListViewController, didSelectTerminal terminal: UUID) {
        delegate?.sidebar(self, didSelectTerminal: terminal)
    }
}

extension SidebarViewController: HistoryListViewControllerDelegate {
    func historyList(_ list: HistoryListViewController, didSelect session: HistorySession) {
        delegate?.sidebar(self, didSelectHistory: session)
    }
}

/// The divider between the workspaces and agents panes. It is drawn in the
/// same separator colour as the sidebar's outer edge, since AppKit's own is
/// nearly invisible on a dark theme.
private final class SidebarSplitView: NSSplitView {
    /// The system separator. The theme's `split-divider-color` comes from the
    /// terminal background, which on a light theme is nearly the sidebar's own
    /// colour, so it would not show here. This is the same colour as the
    /// sidebar's outer edge, and reads on the vibrant sidebar either way.
    override func drawDivider(in rect: NSRect) {
        NSColor.separatorColor.setFill()
        rect.fill()
    }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        needsDisplay = true
    }
}

/// A wash of the window colour over the sidebar. It sets the colour again
/// in each layer update, since an NSBox keeps the one it first drew with
/// when the window turns light or dark.
private final class SidebarTint: NSView {
    override var wantsUpdateLayer: Bool { true }

    override func updateLayer() {
        layer?.backgroundColor = NSColor.windowBackgroundColor.withAlphaComponent(0.8).cgColor
    }
}

/// The translucent selection both sidebar lists draw, so a selected workspace
/// and a selected terminal read the same. The workspace being viewed draws a
/// stronger pill than the rest of a multi-selection.
final class SidebarRowView: NSTableRowView {
    var isActive = false

    override func drawSelection(in dirtyRect: NSRect) {
        NSColor.labelColor.withAlphaComponent(isActive ? 0.14 : 0.07).setFill()
        bounds.fill()
    }
}
