import AppKit
import XCTest

final class WorkspaceAccentTests: XCTestCase {
    func testThePaletteMatchesTheModelCount() {
        XCTAssertEqual(WorkspaceAccent.colors.count, WorkspaceAccentPalette.count)
    }

    func testColorWrapsForStaleIndices() {
        XCTAssertTrue(WorkspaceAccent.color(for: 0) === WorkspaceAccent.colors[0])
        XCTAssertTrue(WorkspaceAccent.color(for: WorkspaceAccent.colors.count) === WorkspaceAccent.colors[0])
        XCTAssertTrue(WorkspaceAccent.color(for: -1) === WorkspaceAccent.colors[WorkspaceAccent.colors.count - 1])
    }
}
