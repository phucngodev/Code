import AppKit

protocol WorkspaceListViewControllerDelegate: AnyObject {
    func workspaceList(_ list: WorkspaceListViewController, didSelect workspace: Workspace)
    /// The name is nil when the rename was abandoned.
    func workspaceList(_ list: WorkspaceListViewController, didFinishRenaming workspace: Workspace, to name: String?)
    func workspaceList(_ list: WorkspaceListViewController, wantsClose workspaces: [Workspace])
}

/// The workspaces pane at the top of the sidebar. Each row shows the workspace
/// name, its focused terminal's branch or directory, and a mark for what the
/// agents in its terminals are doing.
final class WorkspaceListViewController: NSViewController, NSTableViewDataSource, NSTableViewDelegate, NSMenuDelegate {
    weak var delegate: WorkspaceListViewControllerDelegate?
    /// The most urgent status of a workspace's terminals, for its mark and how
    /// long it has held.
    var agentStatus: @MainActor (Workspace) -> AgentStatus? = { _ in nil }
    /// How many of a workspace's terminals have unread alerts.
    var agentUnread: @MainActor (Workspace) -> Int = { _ in 0 }

    private let store: WorkspaceStore
    private let tableView = WorkspaceTableView()
    private let scrollView = NSScrollView()
    private let headerLabel = NSTextField(labelWithString: "Workspaces")
    private static let workspacePasteboardType = NSPasteboard.PasteboardType("dev.phucngo.code.workspace")
    private var draggingWorkspaces: [Workspace] = []
    private var isReloading = false
    private var hoveredRow = -1
    /// Its row is left alone by reloads, which would otherwise replace the
    /// cell and discard the name being typed.
    private var renaming: Workspace?
    /// Ages roll over while the sidebar is open, so the rows are refreshed on
    /// a slow beat rather than only when something changes.
    private var ageTimer: Timer?

    init(store: WorkspaceStore) {
        self.store = store
        super.init(nibName: nil, bundle: nil)
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) is not supported")
    }

    deinit {
        ageTimer?.invalidate()
        NotificationCenter.default.removeObserver(self)
    }

    override func loadView() {
        let column = NSTableColumn(identifier: .init("workspace"))
        column.resizingMask = .autoresizingMask
        tableView.addTableColumn(column)
        tableView.headerView = nil
        tableView.style = .plain
        tableView.backgroundColor = .clear
        tableView.intercellSpacing = .zero
        tableView.rowHeight = 52
        tableView.dataSource = self
        tableView.delegate = self
        tableView.allowsEmptySelection = false
        tableView.allowsMultipleSelection = true
        tableView.target = self
        tableView.action = #selector(selectClickedWorkspace)
        tableView.registerForDraggedTypes([Self.workspacePasteboardType])
        tableView.setDraggingSourceOperationMask(.move, forLocal: true)
        tableView.setDraggingSourceOperationMask([], forLocal: false)

        let menu = NSMenu()
        menu.addItem(withTitle: "Rename Workspace", action: #selector(renameClickedWorkspace), keyEquivalent: "")
        menu.addItem(withTitle: "Close Workspace", action: #selector(closeClickedWorkspace), keyEquivalent: "")
        menu.delegate = self
        tableView.menu = menu

        scrollView.documentView = tableView
        scrollView.hasVerticalScroller = true
        scrollView.autohidesScrollers = true
        scrollView.drawsBackground = false
        // A connected mouse switches the system to always-visible scrollers,
        // which would take width from the rows.
        scrollView.scrollerStyle = .overlay
        NotificationCenter.default.addObserver(self, selector: #selector(keepOverlayScroller), name: NSScroller.preferredScrollerStyleDidChangeNotification, object: nil)
        scrollView.automaticallyAdjustsContentInsets = false
        scrollView.translatesAutoresizingMaskIntoConstraints = false
        // Scrolling moves rows under a still pointer, so the hovered row is
        // worked out again from where the pointer is.
        scrollView.contentView.postsBoundsChangedNotifications = true
        NotificationCenter.default.addObserver(self, selector: #selector(updateHoveredRow), name: NSView.boundsDidChangeNotification, object: scrollView.contentView)
        tableView.onPointerMove = { [weak self] in self?.updateHoveredRow() }

        headerLabel.font = .systemFont(ofSize: 11, weight: .semibold)
        headerLabel.textColor = .secondaryLabelColor
        headerLabel.translatesAutoresizingMaskIntoConstraints = false

        let container = NSView()
        container.addSubview(headerLabel)
        container.addSubview(scrollView)
        NSLayoutConstraint.activate([
            headerLabel.leadingAnchor.constraint(equalTo: container.leadingAnchor, constant: 12),
            headerLabel.trailingAnchor.constraint(lessThanOrEqualTo: container.trailingAnchor, constant: -12),
            headerLabel.topAnchor.constraint(equalTo: container.topAnchor, constant: 10),
            scrollView.leadingAnchor.constraint(equalTo: container.leadingAnchor),
            scrollView.trailingAnchor.constraint(equalTo: container.trailingAnchor),
            scrollView.topAnchor.constraint(equalTo: headerLabel.bottomAnchor, constant: 4),
            scrollView.bottomAnchor.constraint(equalTo: container.bottomAnchor),
        ])
        view = container
        ageTimer = Timer.scheduledTimer(withTimeInterval: 60, repeats: true) { [weak self] _ in
            self?.refreshAgentState()
        }
    }

    func reload() {
        reloadRows()
    }

    private func reloadRows() {
        let wasReloading = isReloading
        isReloading = true
        defer { isReloading = wasReloading }
        if tableView.numberOfRows == store.workspaces.count {
            var rows = IndexSet(0..<store.workspaces.count)
            if let renaming, let row = row(of: renaming) {
                rows.remove(row)
            }
            tableView.reloadData(forRowIndexes: rows, columnIndexes: [0])
            // Reloads arrive with every shell prompt. Animated height changes
            // overlap and leave cells taller than their rows, which then snap
            // back on the next full layout.
            NSAnimationContext.runAnimationGroup { context in
                context.duration = 0
                tableView.noteHeightOfRows(withIndexesChanged: rows)
            }
        } else {
            // A full reload replaces every cell, so a rename in progress is
            // abandoned rather than saved half typed.
            for row in 0..<tableView.numberOfRows {
                (tableView.view(atColumn: 0, row: row, makeIfNecessary: false) as? WorkspaceCellView)?.cancelRenaming()
            }
            renaming = nil
            tableView.reloadData()
        }
        if let selected = store.selected,
           let row = row(of: selected) {
            tableView.selectRowIndexes([row], byExtendingSelection: false)
        }
        // Selecting during the reload is swallowed by `isReloading`, so the
        // rows' badges and active pill are brought in line now that it is done.
        refreshAgentState()
        updateRowActiveStates()
    }

    /// Agents change state far more often than rows need rebuilding, so only
    /// the mark, badge and age are updated.
    func updateAgentStatus(for workspace: Workspace) {
        guard let row = row(of: workspace),
              row < tableView.numberOfRows,
              let cell = tableView.view(atColumn: 0, row: row, makeIfNecessary: false) as? WorkspaceCellView else { return }
        apply(agentStateOf: workspace, to: cell, row: row)
    }

    /// Repaints every visible row's mark, badge and age, for a change that
    /// does not rebuild the list.
    func refreshAgentState() {
        for row in 0..<tableView.numberOfRows {
            guard let workspace = workspace(at: row),
                  let cell = tableView.view(atColumn: 0, row: row, makeIfNecessary: false) as? WorkspaceCellView else { continue }
            apply(agentStateOf: workspace, to: cell, row: row)
        }
    }

    private func apply(agentStateOf workspace: Workspace, to cell: WorkspaceCellView, row: Int) {
        cell.status = agentStatus(workspace)
        cell.unread = agentUnread(workspace)
        cell.isSelected = workspace === store.selected
        cell.isHovered = row == hoveredRow
    }

    /// The workspaces a close acts on: the whole multi-selection when the
    /// target is part of one, otherwise just the target.
    private func closeTargets(for workspace: Workspace?) -> [Workspace] {
        guard let workspace else { return [] }
        let rows = tableView.selectedRowIndexes
        guard rows.count > 1, let row = row(of: workspace), rows.contains(row) else { return [workspace] }
        return rows.compactMap { self.workspace(at: $0) }
    }

    /// The active workspace is shown apart from the rest of a multi-selection,
    /// so its row gets the stronger pill.
    private func updateRowActiveStates() {
        for row in 0..<tableView.numberOfRows {
            (tableView.rowView(atRow: row, makeIfNecessary: false) as? SidebarRowView)?.isActive = workspace(at: row) === store.selected
        }
    }

    private func row(of workspace: Workspace) -> Int? {
        store.workspaces.firstIndex { $0 === workspace }
    }

    private func workspace(at row: Int) -> Workspace? {
        store.workspaces.indices.contains(row) ? store.workspaces[row] : nil
    }

    private var clickedWorkspace: Workspace? {
        workspace(at: tableView.clickedRow)
    }

    /// The table sends its action when a press ends without becoming a drag
    /// or a context menu. A plain click switches workspace; ⌘-click toggles a
    /// row in a multi-selection and ⇧-click extends one, without switching.
    @objc private func selectClickedWorkspace() {
        guard let workspace = clickedWorkspace else { return }
        let row = tableView.clickedRow
        let flags = NSApp.currentEvent?.modifierFlags.intersection(.deviceIndependentFlagsMask) ?? []
        if flags.contains(.command) {
            var rows = tableView.selectedRowIndexes
            if rows.contains(row) { rows.remove(row) } else { rows.insert(row) }
            if rows.isEmpty { rows.insert(row) }
            applySelection(rows)
        } else if flags.contains(.shift) {
            let anchor = tableView.selectedRow >= 0 ? tableView.selectedRow : row
            applySelection(IndexSet(integersIn: min(anchor, row)...max(anchor, row)))
        } else {
            applySelection([row])
            guard workspace !== store.selected else { return }
            delegate?.workspaceList(self, didSelect: workspace)
        }
    }

    /// Changes the table's selection without reporting the active workspace
    /// changing, since a multi-selection only decides what a close or drag
    /// covers.
    private func applySelection(_ rows: IndexSet) {
        let wasReloading = isReloading
        isReloading = true
        tableView.selectRowIndexes(rows, byExtendingSelection: false)
        isReloading = wasReloading
        refreshAgentState()
        updateRowActiveStates()
    }

    @objc private func renameClickedWorkspace() {
        guard let workspace = clickedWorkspace else { return }
        beginRename(of: workspace)
    }

    func beginRename(of workspace: Workspace) {
        guard renaming == nil, let row = row(of: workspace) else { return }
        tableView.scrollRowToVisible(row)
        guard let cell = tableView.view(atColumn: 0, row: row, makeIfNecessary: true) as? WorkspaceCellView else { return }
        renaming = workspace
        cell.beginRenaming { [weak self] name in
            guard let self else { return }
            self.renaming = nil
            let exists = self.store.workspaces.contains { $0 === workspace }
            self.delegate?.workspaceList(self, didFinishRenaming: workspace, to: name.isEmpty || !exists ? nil : name)
        }
    }

    @objc private func closeClickedWorkspace() {
        delegate?.workspaceList(self, wantsClose: closeTargets(for: clickedWorkspace))
    }

    /// Renames the close item for how many rows it would close.
    func menuNeedsUpdate(_ menu: NSMenu) {
        let count = closeTargets(for: clickedWorkspace).count
        menu.item(at: 1)?.title = count > 1 ? "Close \(count) Workspaces" : "Close Workspace"
    }

    /// One row at most shows its close button: the row under the pointer.
    @objc private func keepOverlayScroller() {
        scrollView.scrollerStyle = .overlay
    }

    @objc private func updateHoveredRow() {
        let row = tableView.hoveredRow
        guard row != hoveredRow else { return }
        hoveredRow = row
        for visible in 0..<tableView.numberOfRows {
            (tableView.view(atColumn: 0, row: visible, makeIfNecessary: false) as? WorkspaceCellView)?.isHovered = visible == row
        }
    }

    // MARK: NSTableViewDataSource

    func numberOfRows(in tableView: NSTableView) -> Int {
        store.workspaces.count
    }

    func tableView(_ tableView: NSTableView, pasteboardWriterForRow row: Int) -> NSPasteboardWriting? {
        guard workspace(at: row) != nil else { return nil }
        let item = NSPasteboardItem()
        item.setData(Data(), forType: Self.workspacePasteboardType)
        return item
    }

    func tableView(_ tableView: NSTableView, draggingSession session: NSDraggingSession,
                   willBeginAt screenPoint: NSPoint, forRowIndexes rowIndexes: IndexSet) {
        draggingWorkspaces = rowIndexes.compactMap { workspace(at: $0) }
    }

    private func draggedWorkspaces(_ info: NSDraggingInfo) -> [Workspace]? {
        guard let source = info.draggingSource as? NSTableView, source === tableView,
              !draggingWorkspaces.isEmpty,
              draggingWorkspaces.allSatisfy({ dragged in store.workspaces.contains { $0 === dragged } }) else { return nil }
        return draggingWorkspaces
    }

    func tableView(_ tableView: NSTableView, validateDrop info: NSDraggingInfo,
                   proposedRow row: Int, proposedDropOperation dropOperation: NSTableView.DropOperation) -> NSDragOperation {
        guard draggedWorkspaces(info) != nil, (0...store.workspaces.count).contains(row) else { return [] }
        var index = row
        if dropOperation == .on {
            guard store.workspaces.indices.contains(row) else { return [] }
            let pointer = tableView.convert(info.draggingLocation, from: nil)
            index = insertionIndex(forDropOn: row, pointerY: pointer.y, rowRect: tableView.rect(ofRow: row))
        }
        // Dropping a row back where it is stays allowed, as in Finder, and
        // leaves the order alone.
        tableView.setDropRow(index, dropOperation: .above)
        return .move
    }

    func tableView(_ tableView: NSTableView, acceptDrop info: NSDraggingInfo,
                   row: Int, dropOperation: NSTableView.DropOperation) -> Bool {
        guard dropOperation == .above, let workspaces = draggedWorkspaces(info) else { return false }
        return store.move(workspaces, toInsertionIndex: row)
    }

    func tableView(_ tableView: NSTableView, draggingSession session: NSDraggingSession,
                   endedAt screenPoint: NSPoint, operation: NSDragOperation) {
        draggingWorkspaces = []
        self.tableView.refreshPointer(at: screenPoint)
    }

    // MARK: NSTableViewDelegate

    func tableView(_ tableView: NSTableView, rowViewForRow row: Int) -> NSTableRowView? {
        let view = SidebarRowView()
        view.isActive = workspace(at: row) === store.selected
        return view
    }

    func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
        guard let workspace = workspace(at: row) else { return nil }
        let identifier = NSUserInterfaceItemIdentifier("workspaceCell")
        let cell = tableView.makeView(withIdentifier: identifier, owner: nil) as? WorkspaceCellView ?? {
            let cell = WorkspaceCellView()
            cell.identifier = identifier
            return cell
        }()
        cell.configure(title: workspace.name,
                       accent: workspace.accent,
                       path: workspace.selectedTab?.focusedSurface.workingDirectory?.abbreviatingHome ?? "",
                       status: agentStatus(workspace),
                       unread: agentUnread(workspace),
                       selected: workspace === store.selected,
                       hovered: row == hoveredRow)
        cell.onClose = { [weak self, weak workspace] in
            guard let self, let workspace, self.store.workspaces.contains(where: { $0 === workspace }) else { return }
            self.delegate?.workspaceList(self, wantsClose: self.closeTargets(for: workspace))
        }
        return cell
    }

    /// Mouse presses may become drags. Accessibility selection can proceed
    /// without waiting for a click action.
    func tableView(_ tableView: NSTableView, selectionIndexesForProposedSelection proposedSelectionIndexes: IndexSet) -> IndexSet {
        self.tableView.isHandlingMouseDown ? tableView.selectedRowIndexes : proposedSelectionIndexes
    }

    func tableViewSelectionDidChange(_ notification: Notification) {
        guard !isReloading, !tableView.isHandlingMouseDown,
              let workspace = workspace(at: tableView.selectedRow) else { return }
        refreshAgentState()
        guard workspace !== store.selected else { return }
        delegate?.workspaceList(self, didSelect: workspace)
    }
}

/// Tracks the pointer over the whole list rather than row by row, so the
/// hovered row stays right while rows scroll or are reused.
private final class WorkspaceTableView: NSTableView {
    var onPointerMove: (() -> Void)?
    private(set) var isHandlingMouseDown = false

    override func mouseDown(with event: NSEvent) {
        isHandlingMouseDown = true
        defer { isHandlingMouseDown = false }
        super.mouseDown(with: event)
    }

    func refreshPointer(at screenPoint: NSPoint) {
        pointer = window?.convertPoint(fromScreen: screenPoint)
        onPointerMove?()
    }

    private var pointer: NSPoint?
    private var pointerArea: NSTrackingArea?

    /// Clicking a row leaves the keyboard with the terminal, so typing and
    /// arrow keys go on reaching the shell instead of switching workspaces.
    override var acceptsFirstResponder: Bool { false }

    var hoveredRow: Int {
        guard let pointer else { return -1 }
        let point = convert(pointer, from: nil)
        return visibleRect.contains(point) ? row(at: point) : -1
    }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let pointerArea { removeTrackingArea(pointerArea) }
        let area = NSTrackingArea(rect: bounds, options: [.mouseEnteredAndExited, .mouseMoved, .activeInKeyWindow, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        pointerArea = area
    }

    override func mouseEntered(with event: NSEvent) {
        pointer = event.locationInWindow
        onPointerMove?()
    }

    override func mouseMoved(with event: NSEvent) {
        pointer = event.locationInWindow
        onPointerMove?()
    }

    override func mouseExited(with event: NSEvent) {
        pointer = nil
        onPointerMove?()
    }
}

private final class WorkspaceCellView: NSTableCellView, NSTextFieldDelegate {
    private let accentChip = NSView()
    private let titleLabel = NSTextField(labelWithString: "")
    private let detailLabel = NSTextField(labelWithString: "")
    private let ageLabel = NSTextField(labelWithString: "")
    private let glyph = AgentGlyphView()
    private let badge = UnreadBadgeView()
    private let closeButton = NSButton(image: NSImage(systemSymbolName: "xmark", accessibilityDescription: "Close workspace")!, target: nil, action: nil)
    private let trailingSlot = NSView()
    private var accent = 0
    private var path = ""
    private var onRename: ((String) -> Void)?
    private var nameBeforeRenaming = ""
    private var clickMonitor: Any?
    var onClose: (() -> Void)?

    /// The dragged row shows what it is rather than what the pointer happens
    /// to reveal on it, so the agent dot stays visible.
    override var draggingImageComponents: [NSDraggingImageComponent] {
        let wasHovered = isHovered
        isHovered = false
        layoutSubtreeIfNeeded()
        // Vector rather than bitmap, so it stays sharp on any display the
        // drag crosses.
        let snapshot = NSImage(data: dataWithPDF(inside: bounds))
        isHovered = wasHovered

        let appearance = effectiveAppearance
        let image = NSImage(size: bounds.size, flipped: false) { rect in
            appearance.performAsCurrentDrawingAppearance {
                let card = NSBezierPath(rect: rect.insetBy(dx: 2, dy: 2))
                NSColor.windowBackgroundColor.setFill()
                card.fill()
                NSColor.separatorColor.setStroke()
                card.lineWidth = 1
                card.stroke()
            }
            snapshot?.draw(in: rect)
            return true
        }
        let component = NSDraggingImageComponent(key: .icon)
        component.contents = image
        component.frame = bounds
        return [component]
    }

    init() {
        super.init(frame: .zero)
        titleLabel.font = .systemFont(ofSize: 13, weight: .semibold)
        titleLabel.lineBreakMode = .byTruncatingTail
        titleLabel.delegate = self

        accentChip.wantsLayer = true
        accentChip.translatesAutoresizingMaskIntoConstraints = false
        accentChip.layer?.cornerRadius = 2.5

        detailLabel.font = .monospacedSystemFont(ofSize: 11, weight: .regular)
        detailLabel.textColor = .secondaryLabelColor
        detailLabel.lineBreakMode = .byTruncatingMiddle
        detailLabel.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)

        ageLabel.font = .monospacedDigitSystemFont(ofSize: 11, weight: .regular)
        ageLabel.textColor = .tertiaryLabelColor
        ageLabel.setContentHuggingPriority(.required, for: .horizontal)

        let detailRow = NSStackView(views: [detailLabel, ageLabel])
        detailRow.orientation = .horizontal
        detailRow.alignment = .centerY
        detailRow.spacing = 4
        detailRow.translatesAutoresizingMaskIntoConstraints = false

        let labels = NSStackView(views: [titleLabel, detailRow])
        labels.orientation = .vertical
        labels.alignment = .leading
        labels.spacing = 2
        labels.translatesAutoresizingMaskIntoConstraints = false

        trailingSlot.translatesAutoresizingMaskIntoConstraints = false
        closeButton.isBordered = false
        closeButton.imagePosition = .imageOnly
        closeButton.symbolConfiguration = .init(pointSize: 9, weight: .semibold)
        closeButton.contentTintColor = .secondaryLabelColor
        closeButton.target = self
        closeButton.action = #selector(closeTapped)

        addSubview(accentChip)
        addSubview(labels)
        addSubview(trailingSlot)
        for view in [glyph, badge, closeButton] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            trailingSlot.addSubview(view)
            NSLayoutConstraint.activate([
                view.centerXAnchor.constraint(equalTo: trailingSlot.centerXAnchor),
                view.centerYAnchor.constraint(equalTo: trailingSlot.centerYAnchor),
            ])
        }
        NSLayoutConstraint.activate([
            accentChip.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 12),
            accentChip.widthAnchor.constraint(equalToConstant: 10),
            accentChip.heightAnchor.constraint(equalToConstant: 10),
            accentChip.centerYAnchor.constraint(equalTo: titleLabel.centerYAnchor),
            labels.leadingAnchor.constraint(equalTo: accentChip.trailingAnchor, constant: 8),
            labels.trailingAnchor.constraint(equalTo: trailingSlot.leadingAnchor, constant: -6),
            labels.centerYAnchor.constraint(equalTo: centerYAnchor),
            titleLabel.widthAnchor.constraint(equalTo: labels.widthAnchor),
            detailRow.widthAnchor.constraint(equalTo: labels.widthAnchor),
            trailingSlot.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -10),
            trailingSlot.widthAnchor.constraint(equalToConstant: 20),
            trailingSlot.heightAnchor.constraint(equalToConstant: 20),
            trailingSlot.centerYAnchor.constraint(equalTo: titleLabel.centerYAnchor),
        ])
        updateContent()
    }

    var status: AgentStatus? {
        didSet { updateContent() }
    }

    var unread = 0 {
        didSet { updateContent() }
    }

    var isSelected = false {
        didSet { updateContent() }
    }

    var isHovered = false {
        didSet { updateContent() }
    }

    /// Sets everything a row shows at once, so a reused cell cannot show the
    /// previous workspace's color or branch for a frame.
    func configure(title: String, accent: Int, path: String, status: AgentStatus?, unread: Int, selected: Bool, hovered: Bool) {
        titleLabel.stringValue = title
        self.accent = accent
        self.path = path
        self.status = status
        self.unread = unread
        self.isSelected = selected
        self.isHovered = hovered
        updateContent()
    }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        updateContent()
    }

    private func updateContent() {
        accentChip.layer?.backgroundColor = WorkspaceAccent.color(for: accent).cgColor
        detailLabel.stringValue = path
        // The age is for a status the user may need to act on, and only while
        // the pointer is elsewhere.
        let age = isHovered ? nil : status.map { RelativeAge.string(since: $0.since) }
        ageLabel.stringValue = age ?? ""
        ageLabel.isHidden = age == nil
        updateTrailing()
    }

    /// The mark, the unread badge and the close button share one spot at the
    /// end of the name; the close button wins under the pointer, then a badge.
    private func updateTrailing() {
        closeButton.isHidden = !isHovered
        let showsBadge = !isHovered && !isSelected && unread > 0
        badge.count = showsBadge ? unread : 0
        glyph.indicator = status?.indicator
        glyph.isHidden = isHovered || showsBadge || status?.indicator == nil
    }

    @objc private func closeTapped() {
        onClose?()
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) is not supported")
    }

    /// Return or a click anywhere outside the field keeps the new name and
    /// Escape restores the old one, as renaming works in Finder. The completion
    /// gets the new name, or an empty one when the rename was abandoned.
    func beginRenaming(_ completion: @escaping (String) -> Void) {
        onRename = completion
        nameBeforeRenaming = titleLabel.stringValue
        titleLabel.isEditable = true
        titleLabel.isBezeled = true
        titleLabel.bezelStyle = .roundedBezel
        titleLabel.drawsBackground = true
        titleLabel.textColor = .labelColor
        window?.makeFirstResponder(titleLabel)
        titleLabel.currentEditor()?.selectAll(nil)
        // Clicks on the title bar, the tab strip or empty sidebar space do not
        // take keyboard focus, so on their own they would leave the field open.
        clickMonitor = NSEvent.addLocalMonitorForEvents(matching: [.leftMouseDown, .rightMouseDown]) { [weak self] event in
            guard let self else { return event }
            let inField = event.window === window
                && titleLabel.bounds.contains(titleLabel.convert(event.locationInWindow, from: nil))
            if !inField { window?.makeFirstResponder(nil) }
            return event
        }
    }

    /// Abandons a rename without telling the sidebar, which is reloading.
    func cancelRenaming() {
        onRename = nil
        endRenaming(keepingName: false)
    }

    func control(_ control: NSControl, textView: NSTextView, doCommandBy commandSelector: Selector) -> Bool {
        guard commandSelector == #selector(NSResponder.cancelOperation(_:)) else { return false }
        let completion = onRename
        onRename = nil
        endRenaming(keepingName: false)
        completion?("")
        return true
    }

    func controlTextDidEndEditing(_ notification: Notification) {
        let completion = onRename
        onRename = nil
        let name = titleLabel.stringValue.trimmingCharacters(in: .whitespaces)
        endRenaming(keepingName: true)
        completion?(name)
    }

    private func endRenaming(keepingName: Bool) {
        if let clickMonitor {
            NSEvent.removeMonitor(clickMonitor)
            self.clickMonitor = nil
        }
        if titleLabel.currentEditor() != nil {
            titleLabel.abortEditing()
        }
        if !keepingName {
            titleLabel.stringValue = nameBeforeRenaming
        }
        titleLabel.isEditable = false
        titleLabel.isBezeled = false
        titleLabel.drawsBackground = false
    }
}
