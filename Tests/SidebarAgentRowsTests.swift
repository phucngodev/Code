import XCTest

final class SidebarAgentRowsTests: XCTestCase {
    func testRowsListEveryPaneWithItsAgentAndIndicator() {
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

        XCTAssertEqual(rows.map(\.terminal), [a.id, b.id, c.id])
        XCTAssertEqual(rows[0].agent, "pi")
        XCTAssertEqual(rows[0].indicator, .waiting)
        XCTAssertEqual(rows[0].title, "a")
        XCTAssertEqual(rows[1].agent, "claude")
        XCTAssertEqual(rows[1].indicator, .working)
        // A split names its panes; a later single-pane tab is named by number.
        XCTAssertEqual(rows[0].location, "Tab 1 › Left")
        XCTAssertEqual(rows[1].location, "Tab 1 › Right")
        XCTAssertEqual(rows[2].location, "Tab 2")
    }

    func testATerminalWithoutAnAgentIsStillListed() {
        let leaf = FakeLeaf("shell")
        let workspace = WorkspaceModel<FakeLeaf>()
        workspace.add(TestTab(leaf: leaf))

        let rows = SidebarAgentRows.rows(in: workspace, agent: { _ in nil }, indicator: { _ in nil })

        XCTAssertEqual(rows.count, 1)
        XCTAssertNil(rows[0].agent)
        XCTAssertNil(rows[0].indicator)
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
