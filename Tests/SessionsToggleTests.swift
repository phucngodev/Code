import AppKit
import XCTest

@MainActor
final class SessionsToggleTests: XCTestCase {
    private func makeList() -> HistoryListViewController {
        let list = HistoryListViewController()
        _ = list.view
        list.setSessions([
            HistorySession(id: "a", agent: "claude", cwd: "/tmp", title: "Fix login", modified: .distantPast, resume: nil),
        ])
        return list
    }

    /// The list's scroll view, which the toggle shows and hides.
    private func listView(of controller: HistoryListViewController) -> NSScrollView? {
        controller.view.subviews.compactMap { $0 as? NSScrollView }.first
    }

    func testTheListStartsCollapsed() {
        let list = makeList()
        XCTAssertFalse(list.isExpanded)
        XCTAssertEqual(listView(of: list)?.isHidden, true, "the list is hidden until asked for")
    }

    func testExpandingShowsTheList() {
        let list = makeList()
        list.isExpanded = true
        XCTAssertEqual(listView(of: list)?.isHidden, false)
    }

    func testAnEmptyListStaysHiddenEvenWhenExpanded() {
        let list = HistoryListViewController()
        _ = list.view
        list.setSessions([])
        list.isExpanded = true
        XCTAssertEqual(listView(of: list)?.isHidden, true)
    }

    func testTheHeadingIsOnlyShownWhenExpanded() {
        let list = makeList()
        func heading() -> NSTextField? {
            list.view.subviews.compactMap { $0 as? NSTextField }.first { $0.stringValue == "Sessions" }
        }
        XCTAssertEqual(heading()?.isHidden, true, "only the search bar shows while collapsed")
        list.isExpanded = true
        XCTAssertEqual(heading()?.isHidden, false)
    }

    func testTheToggleButtonReportsTheChange() {
        let list = makeList()
        var changes: [Bool] = []
        list.onExpandedChange = { changes.append($0) }
        let button = list.view.subviews.compactMap { $0 as? NSButton }.first
        XCTAssertNotNil(button, "the toggle sits beside the search field")

        button?.performClick(nil)

        XCTAssertEqual(changes, [true])
        XCTAssertTrue(list.isExpanded)
        XCTAssertEqual(listView(of: list)?.isHidden, false, "pressing the toggle shows the list")
    }

    func testTypingAQueryOpensTheList() throws {
        let list = makeList()
        var changes: [Bool] = []
        list.onExpandedChange = { changes.append($0) }
        let search = try XCTUnwrap(list.view.subviews.compactMap { $0 as? NSSearchField }.first)

        search.stringValue = "fix"
        search.sendAction(search.action, to: search.target)

        XCTAssertTrue(list.isExpanded, "a query opens the list so its matches are visible")
        XCTAssertEqual(changes, [true])
        XCTAssertEqual(listView(of: list)?.isHidden, false)
    }

    func testAnEmptyQueryLeavesTheListAsItIs() throws {
        let list = makeList()
        let search = try XCTUnwrap(list.view.subviews.compactMap { $0 as? NSSearchField }.first)

        search.stringValue = ""
        search.sendAction(search.action, to: search.target)

        XCTAssertFalse(list.isExpanded, "an empty search does not open the list")
    }
}
