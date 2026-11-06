import AppKit
import XCTest

final class SidebarDividerTests: XCTestCase {
    private let minPane: CGFloat = 88

    func testFlippedThreePaneBounds() {
        // NSSplitView is flipped, measuring positions from the top.
        XCTAssertEqual(SidebarDividers.minimum(dividerIndex: 0, panes: 3, flipped: true, minPaneHeight: minPane), 88)
        XCTAssertEqual(SidebarDividers.maximum(dividerIndex: 0, panes: 3, flipped: true, minPaneHeight: minPane, height: 500), 324)
        XCTAssertEqual(SidebarDividers.minimum(dividerIndex: 1, panes: 3, flipped: true, minPaneHeight: minPane), 176)
        XCTAssertEqual(SidebarDividers.maximum(dividerIndex: 1, panes: 3, flipped: true, minPaneHeight: minPane, height: 500), 412)
    }

    func testTwoPanesMatchTheSymmetricCase() {
        XCTAssertEqual(SidebarDividers.minimum(dividerIndex: 0, panes: 2, flipped: true, minPaneHeight: minPane), 88)
        XCTAssertEqual(SidebarDividers.maximum(dividerIndex: 0, panes: 2, flipped: true, minPaneHeight: minPane, height: 400), 312)
    }

    func testUnflippedMirrorsTheFlippedBounds() {
        XCTAssertEqual(SidebarDividers.minimum(dividerIndex: 0, panes: 3, flipped: false, minPaneHeight: minPane), 176)
        XCTAssertEqual(SidebarDividers.maximum(dividerIndex: 0, panes: 3, flipped: false, minPaneHeight: minPane, height: 500), 412)
    }

    /// The placement the sidebar restores — top and bottom from saved heights,
    /// the middle what is left — must leave every pane at least the minimum.
    func testRestoredPlacementKeepsEveryPaneAtLeastTheMinimum() {
        let height: CGFloat = 500
        let divider0 = SidebarDividers.minimum(dividerIndex: 0, panes: 3, flipped: true, minPaneHeight: minPane)
        let divider1 = SidebarDividers.maximum(dividerIndex: 1, panes: 3, flipped: true, minPaneHeight: minPane, height: height)

        XCTAssertGreaterThanOrEqual(divider0, minPane, "the workspaces pane")
        XCTAssertGreaterThanOrEqual(height - divider1, minPane, "the history pane")
        XCTAssertGreaterThanOrEqual(divider1 - divider0, minPane, "the agents pane")
    }
}
