import AppKit

protocol TerminalAreaViewControllerDelegate: AnyObject {
    func terminalArea(_ area: TerminalAreaViewController, didSelect tab: TerminalTab, in workspace: Workspace)
    func terminalArea(_ area: TerminalAreaViewController, didClose tab: TerminalTab, in workspace: Workspace)
    func terminalArea(_ area: TerminalAreaViewController, didMove surface: TerminalSurfaceView, to target: TerminalSurfaceView, action: PaneDropAction)
    func terminalAreaDidRequestNewTab(_ area: TerminalAreaViewController)
}

/// The terminal surface for the selected tab, filling the area below the
/// titlebar. The tabs themselves live in the window's titlebar.
final class TerminalAreaViewController: NSViewController, TabBarViewDelegate {
    weak var delegate: TerminalAreaViewControllerDelegate?
    var agentIndicator: @MainActor (TerminalTab) -> AgentIndicator? = { _ in nil }
    /// Whether an agent runs in the tab, for its sparkle icon.
    var agentRunning: @MainActor (TerminalTab) -> Bool = { _ in false }

    /// Hosted in the titlebar by the window controller.
    let tabBar = TabBarView()
    let paneTreeView = PaneTreeView()
    private var workspace: Workspace?

    override func loadView() {
        tabBar.delegate = self
        paneTreeView.onMovePane = { [weak self] surface, target, action in
            guard let self else { return }
            delegate?.terminalArea(self, didMove: surface, to: target, action: action)
        }
        view = paneTreeView
    }

    func show(_ workspace: Workspace?) {
        self.workspace = workspace
        reloadTabs()
        paneTreeView.show(workspace?.selectedTab?.panes)
        paneTreeView.updateDimming(focused: workspace?.selectedTab?.focusedSurface)
    }

    func reloadTabs() {
        let tabs = workspace?.tabs ?? []
        let selectedIndex = workspace?.selectedTab.flatMap { selected in tabs.firstIndex { $0 === selected } }
        tabBar.reload(titles: tabs.map(\.title), indicators: tabs.map(agentIndicator),
                      agents: tabs.map(agentRunning), selectedIndex: selectedIndex)
    }

    /// Re-reads the whole strip, so a tab's title, state and icon all follow
    /// changes to its agent.
    func updateAgentIndicator(for tab: TerminalTab) {
        reloadTabs()
    }

    func focusSelectedSurface() {
        guard let surface = workspace?.selectedTab?.focusedSurface else { return }
        view.window?.makeFirstResponder(surface)
    }

    // MARK: TabBarViewDelegate

    func tabBar(_ tabBar: TabBarView, didSelectTabAt index: Int) {
        guard let workspace, index < workspace.tabs.count else { return }
        delegate?.terminalArea(self, didSelect: workspace.tabs[index], in: workspace)
    }

    func tabBar(_ tabBar: TabBarView, didCloseTabAt index: Int) {
        guard let workspace, index < workspace.tabs.count else { return }
        delegate?.terminalArea(self, didClose: workspace.tabs[index], in: workspace)
    }

    func tabBarDidRequestNewTab(_ tabBar: TabBarView) {
        delegate?.terminalAreaDidRequestNewTab(self)
    }
}
