import Foundation

/// One terminal of the selected workspace, as the sidebar's agents section
/// lists it. A terminal without an agent is still listed, so the section shows
/// everything the workspace is running.
struct SidebarAgentRow: Equatable {
    var terminal: UUID
    var title: String
    /// The running agent's name, or nil for a plain shell.
    var agent: String?
    /// Where the terminal sits, such as "Tab 2 › Left". Empty when the
    /// terminal is the only pane of the only tab.
    var location: String
    var indicator: AgentIndicator?
}

enum SidebarAgentRows {
    /// Every pane of the workspace's tabs, in sidebar order. The agent name
    /// and indicator are asked per terminal id, so a terminal without one is
    /// still listed.
    static func rows<Leaf: PaneLeaf>(
        in workspace: WorkspaceModel<Leaf>,
        agent: (UUID) -> String?,
        indicator: (UUID) -> AgentIndicator?
    ) -> [SidebarAgentRow] {
        let singleTab = workspace.tabs.count == 1
        return workspace.tabs.enumerated().flatMap { tabIndex, tab in
            let panes = tab.panes.leaves
            let positions = panes.count > 1 ? CommandPalette.positions(in: tab.panes.root) : []
            let tabName = "Tab \(tabIndex + 1)"
            return panes.enumerated().map { paneIndex, leaf in
                let location: String
                if panes.count > 1 {
                    location = "\(tabName) › \(positions[paneIndex])"
                } else {
                    location = singleTab ? "" : tabName
                }
                return SidebarAgentRow(
                    terminal: leaf.id,
                    title: leaf.displayTitle,
                    agent: agent(leaf.id),
                    location: location,
                    indicator: indicator(leaf.id))
            }
        }
    }
}
