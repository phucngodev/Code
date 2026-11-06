import AppKit
import XCTest

@MainActor
final class SidebarVisibilityTests: XCTestCase {
    private var defaults: UserDefaults!
    private var suite: String!

    override func setUp() async throws {
        _ = NSApplication.shared
        suite = "code-sidebar-visibility-test-" + UUID().uuidString
        defaults = UserDefaults(suiteName: suite)
    }

    override func tearDown() async throws {
        defaults.removePersistentDomain(forName: suite)
        defaults = nil
    }

    func testPanesAreBuiltOnlyWhileVisible() {
        let sidebar = SidebarViewController(store: WorkspaceStore(), defaults: defaults)
        _ = sidebar.view
        XCTAssertFalse(sidebar.isContentLoaded, "a collapsed sidebar builds nothing")

        sidebar.sidebarVisibilityDidChange(hidden: false)
        XCTAssertTrue(sidebar.isContentLoaded, "showing the sidebar builds its panes")

        sidebar.sidebarVisibilityDidChange(hidden: true)
        XCTAssertFalse(sidebar.isContentLoaded, "collapsing the sidebar releases its panes")
    }

    func testHidingAnUnbuiltSidebarKeepsItEmpty() {
        let sidebar = SidebarViewController(store: WorkspaceStore(), defaults: defaults)
        _ = sidebar.view

        sidebar.sidebarVisibilityDidChange(hidden: true)

        XCTAssertFalse(sidebar.isContentLoaded)
    }

    func testShowingTwiceBuildsThePanesOnce() {
        let sidebar = SidebarViewController(store: WorkspaceStore(), defaults: defaults)
        _ = sidebar.view

        sidebar.sidebarVisibilityDidChange(hidden: false)
        sidebar.sidebarVisibilityDidChange(hidden: false)

        XCTAssertTrue(sidebar.isContentLoaded)
    }

    func testWindowShowingTheSidebarBuildsThePanes() {
        let sidebar = SidebarViewController(store: WorkspaceStore(), defaults: defaults)
        let content = NSViewController()
        content.view = NSView()
        let window = WindowContentViewController(sidebar: sidebar, content: content, defaults: defaults)
        _ = window.view

        window.isSidebarHidden = false
        XCTAssertTrue(sidebar.isContentLoaded, "the window reaches the sidebar through SidebarVisibilityDelegate")

        window.isSidebarHidden = true
        XCTAssertFalse(sidebar.isContentLoaded)
    }
}
