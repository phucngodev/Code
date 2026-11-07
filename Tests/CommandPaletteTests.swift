import XCTest

final class CommandPaletteTests: XCTestCase {
    private typealias Item = CommandPalette.Item

    // MARK: Items

    func testListsEachTerminalOnceInSidebarOrder() {
        let store = TestStore()
        let a = FakeLeaf("vim", workingDirectory: "/src/api"), b = FakeLeaf(workingDirectory: "/src/api"), c = FakeLeaf("logs")
        let api = store.addWorkspace(customName: "api")
        api.add(TestTab(leaf: a))
        let split = TestTab(leaf: b)
        split.panes.split(b, direction: .right, with: c)
        api.add(split)
        let d = FakeLeaf("zsh")
        store.addWorkspace(customName: "web").add(TestTab(leaf: d))

        let items = CommandPalette.items(in: store)
        let terminals = items.filter { $0.workspaceID == nil }
        XCTAssertEqual(terminals.map { "\($0.title) | \($0.detail) | \($0.workspace) | \($0.location)" }, [
            "vim | /src/api | api | api › Tab 1",
            "/src/api |  | api | api › Tab 2 › Left",
            "logs |  | api | api › Tab 2 › Right",
            "zsh |  | web | web › Tab 1",
        ])
        XCTAssertEqual(terminals.map(\.terminal), [a, b, c, d].map(\.id))
        XCTAssertEqual(items.filter { $0.workspaceID != nil }.map(\.title), ["api", "web"], "each workspace leads its tabs")
    }

    func testListsEachWorkspaceBeforeItsTabsAndRevealsItsTerminalInView() {
        let store = TestStore()
        let a = FakeLeaf("a"), b = FakeLeaf("b")
        let api = store.addWorkspace(customName: "api")
        api.add(TestTab(leaf: a))
        let web = store.addWorkspace(customName: "web")
        web.add(TestTab(leaf: b))

        let items = CommandPalette.items(in: store)
        XCTAssertEqual(items.map(\.title), ["api", "a", "web", "b"], "a workspace leads its own tabs")
        XCTAssertEqual(items.map(\.workspaceID), [api.id, nil, web.id, nil])
        // A workspace row reveals the terminal it last had in view.
        XCTAssertEqual(items.map(\.terminal), [a.id, a.id, b.id, b.id])
        XCTAssertEqual(items.filter { $0.workspaceID != nil }.map(\.layout), [nil, nil], "a workspace has no pane of its own")

        // Its name finds it, and the row outranks the tab that only names it in its location.
        XCTAssertEqual(CommandPalette.filter(items, query: "web").map(\.title), ["web", "b"])
    }

    func testFindsAWorkspacesOnlyTabByNumber() {
        let store = TestStore()
        let a = FakeLeaf("a"), b = FakeLeaf("b"), c = FakeLeaf("c"), d = FakeLeaf("d")
        let api = store.addWorkspace(customName: "api")
        api.add(TestTab(leaf: a))
        let split = TestTab(leaf: b)
        split.panes.split(b, direction: .down, with: c)
        api.add(split)
        store.addWorkspace(customName: "web").add(TestTab(leaf: d))

        let items = CommandPalette.items(in: store)
        XCTAssertEqual(items.filter { $0.workspaceID == nil }.map(\.location),
                       ["api › Tab 1", "api › Tab 2 › Top", "api › Tab 2 › Bottom", "web › Tab 1"])
        XCTAssertEqual(CommandPalette.filter(items, query: "web tab 1").map(\.title), ["d"])
    }

    func testDetailIsTheDirectoryUnlessItIsTheTitle() {
        let store = TestStore()
        let home = NSHomeDirectory()
        store.addWorkspace(customName: "w").add(TestTab(leaf: FakeLeaf("vim", workingDirectory: home + "/src")))
        store.addWorkspace(customName: "x").add(TestTab(leaf: FakeLeaf(workingDirectory: home + "/src")))
        store.addWorkspace(customName: "y").add(TestTab(leaf: FakeLeaf()))
        XCTAssertEqual(CommandPalette.items(in: store).filter { $0.workspaceID == nil }.map { "\($0.title) | \($0.detail)" }, ["vim | ~/src", "~/src | ", "Terminal | "])
    }

    func testDrawsTheTabSplitWithTheItemsPaneHighlighted() {
        let store = TestStore()
        let a = FakeLeaf("a"), b = FakeLeaf("b"), c = FakeLeaf("c")
        let tab = TestTab(leaf: a)
        tab.panes.split(a, direction: .right, with: b)
        tab.panes.split(b, direction: .down, with: c)
        store.addWorkspace().add(tab)

        let panes = CommandPalette.items(in: store).filter { $0.workspaceID == nil }
        XCTAssertEqual(panes.map(\.location), ["~ › Tab 1 › Left", "~ › Tab 1 › Top right", "~ › Tab 1 › Bottom right"])
        XCTAssertEqual(panes.compactMap(\.layout).map(\.highlighted), [0, 1, 2])
        XCTAssertEqual(panes[0].layout?.frames, [
            CGRect(x: 0, y: 0, width: 0.5, height: 1),
            CGRect(x: 0.5, y: 0, width: 0.5, height: 0.5),
            CGRect(x: 0.5, y: 0.5, width: 0.5, height: 0.5),
        ])
    }

    func testTagsPanesOfAZoomedTab() {
        let store = TestStore()
        let a = FakeLeaf("a"), b = FakeLeaf("b"), c = FakeLeaf("c")
        let tab = TestTab(leaf: a)
        tab.panes.split(a, direction: .right, with: b)
        store.addWorkspace().add(tab)
        store.addWorkspace().add(TestTab(leaf: c))
        XCTAssertEqual(CommandPalette.items(in: store).filter { $0.workspaceID == nil }.map { $0.layout?.zoom }, [nil, nil, nil])

        tab.panes.toggleZoom(b)
        XCTAssertEqual(CommandPalette.items(in: store).filter { $0.workspaceID == nil }.map { $0.layout?.zoom }, [.hidden, .zoomed, nil])
    }

    // MARK: Pane names

    /// Builds a tab's split from its first pane; each step splits an existing
    /// pane, named by letter, and adds the next letter.
    private func tree(_ splits: [(String, SplitDirection)]) -> (tree: TestTree, panes: [String: FakeLeaf]) {
        var panes = ["a": FakeLeaf("a")]
        let tree = TestTree(leaf: panes["a"]!)
        for (pane, direction) in splits {
            let added = FakeLeaf(String(UnicodeScalar(UInt8(97 + panes.count))))
            tree.split(panes[pane]!, direction: direction, with: added)
            panes[added.title] = added
        }
        return (tree, panes)
    }

    /// Each pane's name, keyed by the pane, so tests read independent of tree order.
    private func names(_ tree: TestTree) -> [String: String] {
        Dictionary(uniqueKeysWithValues: zip(tree.leaves.map(\.title), CommandPalette.positions(in: tree.root)))
    }

    func testNamesTwoPanesSideBySide() {
        XCTAssertEqual(names(tree([("a", .right)]).tree), ["a": "Left", "b": "Right"])
    }

    func testNamesTwoPanesStacked() {
        XCTAssertEqual(names(tree([("a", .down)]).tree), ["a": "Top", "b": "Bottom"])
    }

    func testNamesFollowWhereAPaneIsNotWhenItWasMade() {
        XCTAssertEqual(names(tree([("a", .left)]).tree), ["a": "Right", "b": "Left"])
        XCTAssertEqual(names(tree([("a", .up)]).tree), ["a": "Bottom", "b": "Top"])
    }

    func testNamesAPaneBesideASplitColumn() {
        XCTAssertEqual(names(tree([("a", .right), ("b", .down)]).tree), ["a": "Left", "b": "Top right", "c": "Bottom right"])
        XCTAssertEqual(names(tree([("a", .right), ("a", .down)]).tree), ["a": "Top left", "c": "Bottom left", "b": "Right"])
    }

    func testNamesAPaneBelowASplitRow() {
        XCTAssertEqual(names(tree([("a", .down), ("a", .right)]).tree), ["a": "Top left", "c": "Top right", "b": "Bottom"])
        XCTAssertEqual(names(tree([("a", .down), ("b", .right)]).tree), ["a": "Top", "b": "Bottom left", "c": "Bottom right"])
    }

    func testNamesEveryCornerOfAGridWhicheverWayItWasSplit() {
        let corners = ["Top left", "Top right", "Bottom left", "Bottom right"]
        let columnsFirst = names(tree([("a", .right), ("a", .down), ("b", .down)]).tree)
        XCTAssertEqual(columnsFirst, ["a": "Top left", "c": "Bottom left", "b": "Top right", "d": "Bottom right"])
        let rowsFirst = names(tree([("a", .down), ("a", .right), ("b", .right)]).tree)
        XCTAssertEqual(rowsFirst, ["a": "Top left", "c": "Top right", "b": "Bottom left", "d": "Bottom right"])
        XCTAssertEqual(Set(columnsFirst.values), Set(corners))
    }

    func testNamesIgnoreHowFarADividerIsDragged() {
        let (tree, _) = tree([("a", .right), ("b", .down)])
        tree.root.ratio = 0.9
        tree.root.second?.ratio = 0.1
        XCTAssertEqual(names(tree), ["a": "Left", "b": "Top right", "c": "Bottom right"])
    }

    func testNamesIgnoreZoom() {
        let (tree, panes) = tree([("a", .right)])
        tree.toggleZoom(panes["b"]!)
        XCTAssertEqual(names(tree), ["a": "Left", "b": "Right"])
    }

    func testNumbersPanesInTreeOrderWhenTheirSidesRepeat() {
        XCTAssertEqual(CommandPalette.positions(in: tree([("a", .right), ("b", .right)]).tree.root), ["Pane 1", "Pane 2", "Pane 3"])
        XCTAssertEqual(CommandPalette.positions(in: tree([("a", .down), ("b", .down)]).tree.root), ["Pane 1", "Pane 2", "Pane 3"])
    }

    func testOneRepeatNumbersEveryPaneInTheTab() {
        let (tree, _) = tree([("a", .right), ("b", .down), ("c", .right)])
        XCTAssertEqual(names(tree), ["a": "Pane 1", "b": "Pane 2", "c": "Pane 3", "d": "Pane 4"])
    }

    func testNamesComeBackWhenTheRepeatCloses() {
        let (tree, panes) = tree([("a", .right), ("b", .right)])
        XCTAssertEqual(names(tree), ["a": "Pane 1", "b": "Pane 2", "c": "Pane 3"])
        _ = tree.remove(panes["b"]!)
        XCTAssertEqual(names(tree), ["a": "Left", "c": "Right"])
    }

    func testEachItemCarriesItsOwnPanesName() {
        let store = TestStore()
        let (tree, panes) = tree([("a", .left), ("a", .down)])
        store.addWorkspace(customName: "api").add(TestTab(panes: tree, focused: panes["a"]!))
        let locations = Dictionary(uniqueKeysWithValues: CommandPalette.items(in: store).filter { $0.workspaceID == nil }.map { ($0.title, $0.location) })
        XCTAssertEqual(locations, ["b": "api › Tab 1 › Left", "a": "api › Tab 1 › Top right", "c": "api › Tab 1 › Bottom right"])
    }

    // MARK: Recency

    func testMostRecentlyFocusedComeFirst() {
        let items = ["a", "b", "c", "d"].map { item($0) }
        XCTAssertEqual(CommandPalette.recentFirst(items, recentlyFocused: [items[3], items[1]].map(\.terminal)).map(\.title), ["d", "b", "a", "c"])
    }

    func testRecencyIgnoresClosedTerminals() {
        let items = ["a", "b"].map { item($0) }
        XCTAssertEqual(CommandPalette.recentFirst(items, recentlyFocused: [UUID(), items[1].terminal]).map(\.title), ["b", "a"])
    }

    func testStoreKeepsEachOpenTerminalOnceMostRecentFirst() {
        let store = TestStore()
        let a = FakeLeaf(), b = FakeLeaf(), c = FakeLeaf()
        let workspace = store.addWorkspace()
        let tab = TestTab(leaf: a)
        tab.panes.split(a, direction: .right, with: b)
        tab.panes.split(b, direction: .down, with: c)
        workspace.add(tab)
        [a, b, c, a].forEach(store.noteFocus)
        XCTAssertEqual(store.recentlyFocused, [a.id, c.id, b.id])

        tab.removePane(c)
        store.noteFocus(b)
        XCTAssertEqual(store.recentlyFocused, [b.id, a.id])
    }

    // MARK: Search

    func testEmptyQueryKeepsEveryItemInOrder() {
        let items = ["a", "b", "c"].map { item($0) }
        XCTAssertEqual(CommandPalette.filter(items, query: "  "), items)
    }

    func testMatchesCharactersInOrderIgnoringCase() {
        let items = ["Code", "wolf", "shell"].map { item($0) }
        XCTAssertEqual(CommandPalette.filter(items, query: "CE").map(\.title), ["Code"])
        XCTAssertEqual(CommandPalette.filter(items, query: "zz"), [])
    }

    func testWordStartsAndRunsRankFirst() {
        let items = ["selfless", "my elf", "el"].map { item($0) }
        XCTAssertEqual(CommandPalette.filter(items, query: "el").map(\.title), ["el", "my elf", "selfless"])
    }

    func testTitleMatchesOutrankLocationMatches() {
        let items = [item("zsh", location: "api"), item("api docs", location: "web")]
        XCTAssertEqual(CommandPalette.filter(items, query: "api").map(\.title), ["api docs", "zsh"])
    }

    func testQueryCanNameTheLocationAndTitle() {
        let items = [item("zsh", location: "api"), item("zsh", location: "web")]
        XCTAssertEqual(CommandPalette.filter(items, query: "web zsh").map(\.location), ["web"])
    }

    func testMatchesTheDirectory() {
        var vim = item("vim")
        vim.detail = "~/src/api"
        XCTAssertEqual(CommandPalette.filter([vim, item("zsh")], query: "src api").map(\.title), ["vim"])
    }

    func testEveryWordHasToMatchInAnyOrder() {
        let items = [
            item("~/C/any2any", location: "any2any › Tab 1"),
            item("~/C/code", location: "code › Tab 2"),
            item("~/C/getcode", location: "getcode › Tab 12"),
        ]
        XCTAssertEqual(CommandPalette.filter(items, query: "tab code").map(\.location), ["code › Tab 2", "getcode › Tab 12"])
        XCTAssertEqual(CommandPalette.filter(items, query: "get").map(\.location), ["getcode › Tab 12"])
    }

    func testLettersInOrderStayWithinOneField() {
        let items = [
            item("~/Code/any2any", location: "any2any › Tab 1"),
            item("~/Code/segment", location: "segment › Tab 1"),
        ]
        XCTAssertEqual(CommandPalette.filter(items, query: "an").map(\.title), ["~/Code/any2any"])
        XCTAssertEqual(CommandPalette.filter(items, query: "sgmt").map(\.title), ["~/Code/segment"])
    }

    func testFindsTabsAndPanesByTheirNames() {
        let items = [
            item("zsh", location: "code › Tab 1 › Left"),
            item("vim", location: "code › Tab 1 › Right"),
            item("zsh", location: "api › Tab 2 › Left"),
        ]
        XCTAssertEqual(CommandPalette.filter(items, query: "tab 1").map(\.title), ["zsh", "vim"])
        XCTAssertEqual(CommandPalette.filter(items, query: "right").map(\.title), ["vim"])
        XCTAssertEqual(CommandPalette.filter(items, query: "code lef").map(\.location), ["code › Tab 1 › Left"])
    }

    func testFindsTerminalsByTheirAgent() {
        var claude = item("~/Code/code"), codex = item("~/Code/api")
        claude.agent = "claude"
        codex.agent = "codex"
        let items = [claude, codex, item("zsh")]
        XCTAssertEqual(CommandPalette.filter(items, query: "claude").map(\.title), ["~/Code/code"])
        XCTAssertEqual(CommandPalette.filter(items, query: "cod api").map(\.title), ["~/Code/api"])
    }

    func testItemsCarryTheirTerminalsAgent() {
        let store = TestStore()
        let a = FakeLeaf("a"), b = FakeLeaf("b")
        store.addWorkspace().add(TestTab(leaf: a))
        store.addWorkspace().add(TestTab(leaf: b))
        let items = CommandPalette.items(in: store) { $0 == b.id ? "claude" : nil }.filter { $0.workspaceID == nil }
        XCTAssertEqual(items.map(\.agent), [nil, "claude"])
    }

    func testNumbersMatchWholeNumbersOnly() {
        let items = [
            item("~/C/any2any", location: "any2any › Tab 1"),
            item("~/C/code", location: "code › Tab 2"),
            item("~/C/getcode", location: "getcode › Tab 12"),
        ]
        XCTAssertEqual(CommandPalette.filter(items, query: "any tab 2"), [])
        XCTAssertEqual(CommandPalette.filter(items, query: "tab 2").map(\.location), ["code › Tab 2"])
    }

    private func item(_ title: String, location: String = "") -> Item {
        Item(terminal: UUID(), title: title, detail: "", workspace: "", location: location,
             layout: .init(frames: [CGRect(x: 0, y: 0, width: 1, height: 1)], highlighted: 0))
    }
}

@MainActor
final class CommandPaletteViewTests: XCTestCase {
    private func items(_ count: Int) -> [CommandPalette.Item] {
        (0..<count).map { index in
            CommandPalette.Item(terminal: UUID(), title: "\(index)", detail: "", workspace: "", location: "",
                                layout: .init(frames: [CGRect(x: 0, y: 0, width: 1, height: 1)], highlighted: 0))
        }
    }

    private func type(_ text: String, in palette: CommandPaletteView) throws {
        func fields(in view: NSView) -> [NSTextField] {
            view.subviews.flatMap { ([$0 as? NSTextField].compactMap { $0 }.filter(\.isEditable)) + fields(in: $0) }
        }
        let field = try XCTUnwrap(fields(in: palette).first)
        field.stringValue = text
        palette.controlTextDidChange(Notification(name: NSControl.textDidChangeNotification, object: field))
    }

    func testASearchSelectsItsBestMatchOverTheTerminalInView() throws {
        var items = items(2)
        items[0].title = "apiary"
        items[1].title = "api"
        let (current, api) = (items[0], items[1])
        let palette = CommandPaletteView(items: items, current: current.terminal, agentIndicator: { _ in nil })
        var selected: [UUID?] = []
        palette.onSelect = { selected.append($0?.terminal) }
        try type("api", in: palette)
        XCTAssertEqual(selected, [api.terminal], "the terminal in view matches too, but less well")
        try type("", in: palette)
        XCTAssertEqual(selected, [api.terminal, current.terminal])
    }

    func testStartsOnTheTerminalInViewSoSteppingOnceGoesToTheNext() {
        let items = items(3)
        let palette = CommandPaletteView(items: items, current: items[0].terminal, agentIndicator: { _ in nil })
        var selected: [UUID?] = []
        palette.onSelect = { selected.append($0?.terminal) }
        palette.selectNext()
        XCTAssertEqual(selected, [items[1].terminal])
    }

    func testStartsOnNothingWhenTheTerminalInViewIsNotListed() {
        let items = items(3)
        let palette = CommandPaletteView(items: items, current: UUID(), agentIndicator: { _ in nil })
        var selected: [UUID?] = []
        palette.onSelect = { selected.append($0?.terminal) }
        palette.selectNext()
        XCTAssertEqual(selected, [items[0].terminal])
    }

    func testItemsFollowOtherWindowsWithoutMovingTheSelection() {
        let items = items(3)
        var current = Dictionary(uniqueKeysWithValues: items.map { ($0.terminal, $0) })
        let palette = CommandPaletteView(items: items, current: items[0].terminal, agentIndicator: { _ in nil })
        var selected: [UUID?] = []
        palette.onSelect = { selected.append($0?.terminal) }
        palette.selectNext()
        XCTAssertEqual(selected, [items[1].terminal])

        current[items[2].terminal] = nil
        current[items[1].terminal]?.location = "Tab 1"
        palette.updateItems { current[$0.terminal] }
        XCTAssertEqual(selected, [items[1].terminal], "still on its item, so nothing new to preview")

        current[items[1].terminal] = nil
        palette.updateItems { current[$0.terminal] }
        XCTAssertEqual(selected, [items[1].terminal, items[0].terminal], "back on the terminal in view")
    }
}
