import AppKit
import XCTest

final class WorkspaceMenuTests: XCTestCase {
    private final class Receiver: NSObject {
        var selectedWorkspace: Int?
        var selectedTab: Int?

        @objc func selectWorkspace(_ sender: NSMenuItem) { selectedWorkspace = sender.tag }
        @objc func selectTab(_ sender: NSMenuItem) { selectedTab = sender.tag }
    }

    private let receiver = Receiver()

    private func makeMenu(_ contents: WorkspaceMenu.Contents?) -> WorkspaceMenu {
        let menu = WorkspaceMenu(
            title: "Workspace",
            fixedItems: [NSMenuItem(title: "Rename Workspace", action: nil, keyEquivalent: "")],
            target: receiver,
            selectWorkspace: #selector(Receiver.selectWorkspace(_:)),
            selectTab: #selector(Receiver.selectTab(_:)))
        menu.contents = { contents }
        return menu
    }

    func testListsEveryWorkspaceAndTabWithNineOnTheLastTab() {
        let workspaceNames = (1...11).map { "w\($0)" }
        let tabNames = (1...11).map { "t\($0)" }
        let menu = makeMenu(.init(workspaces: .init(names: workspaceNames, selected: 10), tabs: .init(names: tabNames, selected: 0)))
        menu.menuNeedsUpdate(menu.menu)

        let workspaces = menu.menu.items.filter { $0.action == #selector(Receiver.selectWorkspace(_:)) }
        XCTAssertEqual(workspaces.map(\.title), workspaceNames)
        XCTAssertEqual(workspaces.map(\.keyEquivalent), (1...8).map { "\($0)" } + ["", "", "9"])
        XCTAssertEqual(workspaces.map(\.keyEquivalentModifierMask), Array(repeating: .option, count: 11))
        XCTAssertEqual(workspaces.map(\.state), Array(repeating: .off, count: 10) + [.on])

        let tabs = menu.menu.items.filter { $0.action == #selector(Receiver.selectTab(_:)) }
        XCTAssertEqual(tabs.map(\.title), tabNames)
        XCTAssertEqual(tabs.map(\.keyEquivalent), (1...8).map { "\($0)" } + ["", "", "9"])
        XCTAssertEqual(tabs.map(\.keyEquivalentModifierMask), Array(repeating: .command, count: 11))
        XCTAssertEqual(tabs.map(\.state), [.on] + Array(repeating: .off, count: 10))
    }

    func testKeyEquivalentsPutNineOnTheLastTab() {
        XCTAssertEqual((0..<11).map { WorkspaceMenu.keyEquivalent(at: $0, of: 11) }, (1...8).map { "\($0)" } + ["", "", "9"])
        XCTAssertEqual((0..<3).map { WorkspaceMenu.keyEquivalent(at: $0, of: 3) }, ["1", "2", "3"], "a short list keeps its numbers")
    }

    func testNumberKeysMapToEntriesWithNineOnTheLast() {
        XCTAssertEqual(WorkspaceMenu.index(forKey: "1", of: 3), 0)
        XCTAssertEqual(WorkspaceMenu.index(forKey: "2", of: 3), 1)
        XCTAssertEqual(WorkspaceMenu.index(forKey: "3", of: 3), 2)
        XCTAssertEqual(WorkspaceMenu.index(forKey: "9", of: 3), 2, "9 is the last")
        XCTAssertNil(WorkspaceMenu.index(forKey: "4", of: 3), "there is no fourth")
        XCTAssertNil(WorkspaceMenu.index(forKey: "9", of: 0))
        XCTAssertNil(WorkspaceMenu.index(forKey: "x", of: 3))

        // Both directions agree for every entry the menu gives a key.
        for count in 1...12 {
            for index in 0..<count {
                let key = WorkspaceMenu.keyEquivalent(at: index, of: count)
                guard !key.isEmpty else { continue }
                XCTAssertEqual(WorkspaceMenu.index(forKey: key, of: count), index, "count \(count) index \(index)")
            }
        }
    }

    private func keyEvent(_ key: String, characters: String, flags: NSEvent.ModifierFlags, keyCode: UInt16) -> NSEvent {
        NSEvent.keyEvent(with: .keyDown, location: .zero, modifierFlags: flags, timestamp: 1, windowNumber: 0,
                         context: nil, characters: characters, charactersIgnoringModifiers: key, isARepeat: false, keyCode: keyCode)!
    }

    func testOptionNumbersReachTheWorkspaceItems() {
        let menu = makeMenu(.init(workspaces: .init(names: ["w1", "w2"], selected: 0), tabs: .init(names: ["t1"], selected: 0)))
        menu.menuNeedsUpdate(menu.menu)
        // Option+1 arrives with a special `characters` but ignores to "1",
        // which is what AppKit matches a key equivalent against.
        XCTAssertTrue(menu.menu.performKeyEquivalent(with: keyEvent("1", characters: "\u{00A1}", flags: .option, keyCode: 18)))
        XCTAssertTrue(menu.menu.performKeyEquivalent(with: keyEvent("2", characters: "\u{2122}", flags: .option, keyCode: 19)))
    }

    func testRebuildingKeepsTheFixedItemsOnce() {
        let menu = makeMenu(.init(workspaces: .init(names: ["w"], selected: 0), tabs: .init(names: ["a"], selected: 0)))
        menu.menuNeedsUpdate(menu.menu)
        menu.menuNeedsUpdate(menu.menu)
        XCTAssertEqual(menu.menu.items.map(\.title), ["Rename Workspace", "", "w", "", "a"])
    }

    func testWithoutAKeyWindowOnlyTheFixedItemsRemain() {
        let menu = makeMenu(nil)
        menu.menuNeedsUpdate(menu.menu)
        XCTAssertEqual(menu.menu.items.map(\.title), ["Rename Workspace"])
    }
}

@MainActor
final class MenuReuseTests: XCTestCase {
    func testUnchangedMenuKeepsItemsAndSelectionChangesStillApply() {
        let menu = WorkspaceMenu(title: "Workspace", fixedItems: [], target: NSObject(),
                                 selectWorkspace: Selector(("workspace:")), selectTab: Selector(("tab:")))
        var selection = 0
        menu.contents = {
            .init(workspaces: .init(names: ["one", "two"], selected: selection),
                  tabs: .init(names: [], selected: nil))
        }
        menu.menuNeedsUpdate(menu.menu)
        let original = menu.menu.items
        menu.menuNeedsUpdate(menu.menu)
        XCTAssertTrue(zip(original, menu.menu.items).allSatisfy { $0 === $1 })
        selection = 1
        menu.menuNeedsUpdate(menu.menu)
        XCTAssertEqual(menu.menu.items.filter { !$0.isSeparatorItem }.map(\.state), [.off, .on])
        menu.contents = { nil }
        menu.menuNeedsUpdate(menu.menu)
        XCTAssertTrue(menu.menu.items.isEmpty)
    }
}
