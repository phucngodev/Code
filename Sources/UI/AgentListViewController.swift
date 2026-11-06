import AppKit

protocol AgentListViewControllerDelegate: AnyObject {
    func agentList(_ list: AgentListViewController, didSelectTerminal terminal: UUID)
}

/// The terminals of the selected workspace, at the bottom of the sidebar.
/// A row shows what its agent is doing, or just the terminal when none runs.
final class AgentListViewController: NSViewController, NSTableViewDataSource, NSTableViewDelegate {
    weak var delegate: AgentListViewControllerDelegate?
    var agentName: @MainActor (UUID) -> String? = { _ in nil }
    var agentIndicator: @MainActor (UUID) -> AgentIndicator? = { _ in nil }

    private let store: WorkspaceStore
    private let tableView = NSTableView()
    private let scrollView = NSScrollView()
    private let headerLabel = NSTextField(labelWithString: "Tabs & Panes & Agents")
    private let placeholderLabel = NSTextField(labelWithString: "No running agents")
    private var rows: [SidebarAgentRow] = []

    init(store: WorkspaceStore) {
        self.store = store
        super.init(nibName: nil, bundle: nil)
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) is not supported")
    }

    override func loadView() {
        let column = NSTableColumn(identifier: .init("terminal"))
        column.resizingMask = .autoresizingMask
        tableView.addTableColumn(column)
        tableView.headerView = nil
        tableView.style = .plain
        tableView.backgroundColor = .clear
        tableView.intercellSpacing = .zero
        tableView.rowHeight = 44
        tableView.dataSource = self
        tableView.delegate = self
        tableView.allowsEmptySelection = true
        // Clicking a row moves focus to the terminal, so the list itself never
        // takes the keyboard from it.
        tableView.refusesFirstResponder = true
        tableView.target = self
        tableView.action = #selector(selectClickedTerminal)

        scrollView.documentView = tableView
        scrollView.hasVerticalScroller = true
        scrollView.autohidesScrollers = true
        scrollView.drawsBackground = false
        scrollView.scrollerStyle = .overlay
        scrollView.automaticallyAdjustsContentInsets = false
        scrollView.translatesAutoresizingMaskIntoConstraints = false

        headerLabel.font = .systemFont(ofSize: 11, weight: .semibold)
        headerLabel.textColor = .secondaryLabelColor
        headerLabel.translatesAutoresizingMaskIntoConstraints = false

        placeholderLabel.font = .systemFont(ofSize: 11)
        placeholderLabel.textColor = .tertiaryLabelColor
        placeholderLabel.translatesAutoresizingMaskIntoConstraints = false

        let container = NSView()
        container.addSubview(headerLabel)
        container.addSubview(scrollView)
        container.addSubview(placeholderLabel)
        NSLayoutConstraint.activate([
            headerLabel.leadingAnchor.constraint(equalTo: container.leadingAnchor, constant: 12),
            headerLabel.trailingAnchor.constraint(lessThanOrEqualTo: container.trailingAnchor, constant: -12),
            headerLabel.topAnchor.constraint(equalTo: container.topAnchor, constant: 10),
            scrollView.leadingAnchor.constraint(equalTo: container.leadingAnchor),
            scrollView.trailingAnchor.constraint(equalTo: container.trailingAnchor),
            scrollView.topAnchor.constraint(equalTo: headerLabel.bottomAnchor, constant: 4),
            scrollView.bottomAnchor.constraint(equalTo: container.bottomAnchor),
            placeholderLabel.leadingAnchor.constraint(equalTo: container.leadingAnchor, constant: 20),
            placeholderLabel.trailingAnchor.constraint(lessThanOrEqualTo: container.trailingAnchor, constant: -12),
            placeholderLabel.topAnchor.constraint(equalTo: headerLabel.bottomAnchor, constant: 8),
        ])
        view = container
    }

    /// Rebuilds the list for the selected workspace and highlights the terminal
    /// the user is in. When the same terminals are listed, only the cells whose
    /// row changed are touched, so a reload arriving with every shell prompt
    /// does not rebuild the table.
    func reload() {
        let updated = store.selected.map {
            SidebarAgentRows.rows(in: $0, agent: agentName, indicator: agentIndicator)
        } ?? []
        let sameTerminals = updated.count == rows.count && zip(updated, rows).allSatisfy { $0.terminal == $1.terminal }
        rows = updated
        if sameTerminals {
            for index in rows.indices {
                guard let cell = tableView.view(atColumn: 0, row: index, makeIfNecessary: false) as? AgentRowCellView,
                      cell.row != rows[index] else { continue }
                cell.row = rows[index]
            }
        } else {
            tableView.reloadData()
        }
        let empty = rows.isEmpty
        placeholderLabel.isHidden = !empty
        scrollView.isHidden = empty
        selectFocusedTerminal()
    }

    /// Agents change state far more often than rows need rebuilding, so only
    /// the changed row is updated.
    func updateIndicator(for terminal: UUID) {
        guard let index = rows.firstIndex(where: { $0.terminal == terminal }) else { return }
        rows[index].indicator = agentIndicator(terminal)
        rows[index].agent = agentName(terminal)
        guard let cell = tableView.view(atColumn: 0, row: index, makeIfNecessary: false) as? AgentRowCellView else { return }
        cell.row = rows[index]
    }

    private func selectFocusedTerminal() {
        guard let focused = store.selected?.selectedTab?.focusedLeaf.id,
              let index = rows.firstIndex(where: { $0.terminal == focused }) else {
            tableView.deselectAll(nil)
            return
        }
        guard tableView.selectedRow != index else { return }
        tableView.selectRowIndexes([index], byExtendingSelection: false)
    }

    /// Reveals the terminal on a click, which the table sends once the press
    /// ends without becoming a drag.
    @objc private func selectClickedTerminal() {
        let row = tableView.clickedRow
        guard rows.indices.contains(row) else { return }
        delegate?.agentList(self, didSelectTerminal: rows[row].terminal)
    }

    // MARK: NSTableViewDataSource

    func numberOfRows(in tableView: NSTableView) -> Int {
        rows.count
    }

    // MARK: NSTableViewDelegate

    func tableView(_ tableView: NSTableView, rowViewForRow row: Int) -> NSTableRowView? {
        SidebarRowView()
    }

    func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
        guard rows.indices.contains(row) else { return nil }
        let identifier = NSUserInterfaceItemIdentifier("agentCell")
        let cell = tableView.makeView(withIdentifier: identifier, owner: nil) as? AgentRowCellView ?? {
            let cell = AgentRowCellView()
            cell.identifier = identifier
            return cell
        }()
        cell.row = rows[row]
        return cell
    }
}

private final class AgentRowCellView: NSTableCellView {
    private let glyph = AgentGlyphView()
    private let titleLabel = NSTextField(labelWithString: "")
    private let subtitleLabel = NSTextField(labelWithString: "")

    var row: SidebarAgentRow? {
        didSet { update() }
    }

    init() {
        super.init(frame: .zero)
        titleLabel.font = .systemFont(ofSize: 13)
        titleLabel.lineBreakMode = .byTruncatingTail
        subtitleLabel.font = .monospacedSystemFont(ofSize: 11, weight: .regular)
        subtitleLabel.textColor = .secondaryLabelColor
        subtitleLabel.lineBreakMode = .byTruncatingTail

        let labels = NSStackView(views: [titleLabel, subtitleLabel])
        labels.orientation = .vertical
        labels.alignment = .leading
        labels.spacing = 1
        labels.translatesAutoresizingMaskIntoConstraints = false
        addSubview(glyph)
        addSubview(labels)
        NSLayoutConstraint.activate([
            glyph.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 14),
            glyph.centerYAnchor.constraint(equalTo: centerYAnchor),
            labels.leadingAnchor.constraint(equalTo: glyph.trailingAnchor, constant: 6),
            labels.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -12),
            labels.centerYAnchor.constraint(equalTo: centerYAnchor),
            titleLabel.widthAnchor.constraint(equalTo: labels.widthAnchor),
            subtitleLabel.widthAnchor.constraint(equalTo: labels.widthAnchor),
        ])
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) is not supported")
    }

    private func update() {
        guard let row else { return }
        glyph.indicator = row.indicator
        glyph.isHidden = row.indicator == nil
        titleLabel.stringValue = row.title
        let subtitle = [row.agent, row.location].compactMap { $0?.isEmpty == false ? $0 : nil }.joined(separator: " · ")
        subtitleLabel.stringValue = subtitle
        subtitleLabel.isHidden = subtitle.isEmpty
    }
}
