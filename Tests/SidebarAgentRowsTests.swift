import XCTest

final class SidebarAgentRowsTests: XCTestCase {
    func testRowsListOnlyTerminalsRunningAnAgent() {
        let a = FakeLeaf("a", workingDirectory: "/tmp/a")
        let b = FakeLeaf("b", workingDirectory: "/tmp/b")
        let workspace = WorkspaceModel<FakeLeaf>()
        let tab = TestTab(leaf: a)
        tab.panes.split(a, direction: .right, with: b)
        workspace.add(tab)
        let c = FakeLeaf("c")
        workspace.add(TestTab(leaf: c))

        let agents: [UUID: String] = [a.id: "pi", b.id: "claude"]
        let indicators: [UUID: AgentIndicator] = [a.id: .waiting, b.id: .working]
        let rows = SidebarAgentRows.rows(in: workspace,
                                         agent: { agents[$0] },
                                         indicator: { indicators[$0] })

        // The third terminal runs no agent, so it is left out.
        XCTAssertEqual(rows.map(\.terminal), [a.id, b.id])
        XCTAssertEqual(rows[0].agent, "pi")
        XCTAssertEqual(rows[0].indicator, .waiting)
        XCTAssertEqual(rows[0].title, "a")
        XCTAssertEqual(rows[1].agent, "claude")
        XCTAssertEqual(rows[1].indicator, .working)
        XCTAssertEqual(rows[0].location, "Tab 1 › Left")
        XCTAssertEqual(rows[1].location, "Tab 1 › Right")
    }

    func testATerminalWithoutAnAgentIsNotListed() {
        let leaf = FakeLeaf("shell")
        let workspace = WorkspaceModel<FakeLeaf>()
        workspace.add(TestTab(leaf: leaf))

        let rows = SidebarAgentRows.rows(in: workspace, agent: { _ in nil }, indicator: { _ in nil })

        XCTAssertTrue(rows.isEmpty)
    }

    func testAnAgentKeepsTheLocationOfItsPane() {
        let a = FakeLeaf("a"), b = FakeLeaf("b")
        let workspace = WorkspaceModel<FakeLeaf>()
        let tab = TestTab(leaf: a)
        tab.panes.split(a, direction: .right, with: b)
        workspace.add(tab)

        let rows = SidebarAgentRows.rows(in: workspace,
                                         agent: { $0 == b.id ? "pi" : nil },
                                         indicator: { _ in .working })

        // The pane beside it is not listed, but the location still names it.
        XCTAssertEqual(rows.map(\.terminal), [b.id])
        XCTAssertEqual(rows[0].location, "Tab 1 › Right")
    }

    func testSingleTerminalWorkspaceHasNoLocation() {
        let leaf = FakeLeaf("solo")
        let workspace = WorkspaceModel<FakeLeaf>()
        workspace.add(TestTab(leaf: leaf))

        let rows = SidebarAgentRows.rows(in: workspace, agent: { _ in "pi" }, indicator: { _ in .done })

        XCTAssertEqual(rows[0].location, "")
        XCTAssertEqual(rows[0].indicator, .done)
    }
}
