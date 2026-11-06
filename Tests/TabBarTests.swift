import AppKit
import XCTest

@MainActor
final class TabBarTests: XCTestCase {
    func testReloadKeepsOneItemPerTab() {
        let tabs = TabBarView()
        tabs.reload(titles: ["one", "two"], indicators: [nil, .working], selectedIndex: 1)
        XCTAssertEqual(tabs.tabCount, 2)

        tabs.reload(titles: ["one"], indicators: [nil], selectedIndex: 0)
        XCTAssertEqual(tabs.tabCount, 1)
    }

    /// The titlebar accessory sizes to its content, so the strip has to hug its
    /// tabs rather than stretch.
    func testTheStripHugsItsTabs() {
        let tabs = TabBarView()
        tabs.reload(titles: ["one"], indicators: [nil], selectedIndex: 0)
        let single = tabs.fittingSize.width

        tabs.reload(titles: ["one", "two", "three"], indicators: [nil, nil, nil], selectedIndex: 0)
        XCTAssertGreaterThan(tabs.fittingSize.width, single)
    }
}
