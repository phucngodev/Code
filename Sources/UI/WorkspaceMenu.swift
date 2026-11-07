import AppKit

/// The Workspace menu lists every workspace in the key window and every tab
/// in its selected workspace, under the names the sidebar and tab strip show.
/// It is filled in only when it opens or is searched for a key equivalent, so
/// it is never out of date and costs nothing in between.
final class WorkspaceMenu: NSObject, NSMenuDelegate {
    struct Entries: Equatable {
        var names: [String]
        var selected: Int?
    }

    struct Contents: Equatable {
        var workspaces: Entries
        var tabs: Entries
    }

    let menu: NSMenu
    /// Nil when no terminal window is key.
    var contents: () -> Contents? = { nil }
    private let fixedItemCount: Int
    private weak var target: AnyObject?
    private let selectWorkspace: Selector
    private let selectTab: Selector
    private var displayedContents: Contents?

    /// Workspace and tab items carry their index as the tag.
    init(title: String, fixedItems: [NSMenuItem], target: AnyObject, selectWorkspace: Selector, selectTab: Selector) {
        menu = NSMenu(title: title)
        fixedItems.forEach(menu.addItem)
        fixedItemCount = fixedItems.count
        self.target = target
        self.selectWorkspace = selectWorkspace
        self.selectTab = selectTab
        super.init()
        menu.delegate = self
    }

    func menuNeedsUpdate(_ menu: NSMenu) {
        let current = contents()
        guard current != displayedContents else { return }
        displayedContents = current
        while menu.numberOfItems > fixedItemCount {
            menu.removeItem(at: fixedItemCount)
        }
        guard let contents = current else { return }
        add(contents.workspaces, action: selectWorkspace, shortcuts: .option)
        add(contents.tabs, action: selectTab, shortcuts: .command)
    }

    /// AppKit does not refill the menu before matching key equivalents,
    /// but it asks this first and then searches the items, so they are
    /// brought up to date here.
    func menuHasKeyEquivalent(_ menu: NSMenu, for event: NSEvent, target: AutoreleasingUnsafeMutablePointer<AnyObject?>, action: UnsafeMutablePointer<Selector?>) -> Bool {
        menuNeedsUpdate(menu)
        return false
    }

    /// `shortcuts` is the modifier for the key equivalents to add, or nil for
    /// entries listed without any. Workspaces take ⌥ and tabs ⌘, so the same
    /// number keys choose a workspace or a tab.
    private func add(_ entries: Entries, action: Selector, shortcuts: NSEvent.ModifierFlags?) {
        guard !entries.names.isEmpty else { return }
        menu.addItem(.separator())
        for (index, name) in entries.names.enumerated() {
            let key = shortcuts.map { _ in Self.keyEquivalent(at: index, of: entries.names.count) } ?? ""
            let item = menu.addItem(withTitle: name, action: action, keyEquivalent: key)
            if let shortcuts { item.keyEquivalentModifierMask = shortcuts }
            item.target = target
            item.tag = index
            item.state = index == entries.selected ? .on : .off
        }
    }

    /// ⌘9 is Ghostty's last-tab binding, so the menu puts it on the last tab
    /// rather than the ninth.
    static func keyEquivalent(at index: Int, of count: Int) -> String {
        if index < 8 { return "\(index + 1)" }
        return index == count - 1 ? "9" : ""
    }

    /// The entry a number key picks, or nil when that number has nothing to
    /// select. `9` is the last, the inverse of `keyEquivalent(at:of:)`.
    static func index(forKey key: String, of count: Int) -> Int? {
        guard let number = Int(key), (1...9).contains(number), count > 0 else { return nil }
        let index = number == 9 ? count - 1 : number - 1
        return (0..<count).contains(index) ? index : nil
    }
}
