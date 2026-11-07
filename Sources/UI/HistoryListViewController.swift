import AppKit

protocol HistoryListViewControllerDelegate: AnyObject {
    func historyList(_ list: HistoryListViewController, didSelect session: HistorySession)
}

/// The agent sessions kept on disk, at the bottom of the sidebar. A row
/// reopens the session where it ran. The scan runs off the main thread and
/// only reads the head of each transcript.
final class HistoryListViewController: NSViewController, NSTableViewDataSource, NSTableViewDelegate {
    weak var delegate: HistoryListViewControllerDelegate?
    /// Where to look for transcripts. Read on the scan queue.
    var roots: [HistoryStore.Root] = HistoryStore.defaultRoots()

    private let tableView = NSTableView()
    private let scrollView = NSScrollView()
    private let searchField = NSSearchField()
    private let headerLabel = NSTextField(labelWithString: "Sessions")
    private let placeholderLabel = NSTextField(labelWithString: "No saved sessions")
    private let toggleButton = NSButton(
        image: NSImage(systemSymbolName: "chevron.right", accessibilityDescription: "Show sessions")!,
        target: nil, action: nil)
    private var sessions: [HistorySession] = []
    private var shown: [HistorySession] = []
    private var isLoading = false
    private let queue = DispatchQueue(label: "dev.phucngo.code.history", qos: .utility)

    /// Whether the list is showing. Collapsed by default, so the sessions a
    /// user rarely opens do not take the sidebar's space until asked for.
    var isExpanded = false {
        didSet { applyQuery() }
    }

    /// Called when the toggle is pressed, so the sidebar can resize the pane.
    var onExpandedChange: ((Bool) -> Void)?
    /// Which constraints lay the list out below the heading, and which lay the
    /// bar out on its own. Only one set is active at a time.
    private var expandedConstraints: [NSLayoutConstraint] = []
    private var collapsedConstraints: [NSLayoutConstraint] = []

    override func loadView() {
        let column = NSTableColumn(identifier: .init("history"))
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
        tableView.refusesFirstResponder = true
        tableView.target = self
        tableView.action = #selector(openClickedSession)

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

        searchField.placeholderString = "Search sessions"
        searchField.controlSize = .small
        searchField.font = .systemFont(ofSize: 11)
        searchField.target = self
        searchField.action = #selector(searchChanged)
        searchField.translatesAutoresizingMaskIntoConstraints = false

        toggleButton.target = self
        toggleButton.action = #selector(toggleExpanded)
        toggleButton.isBordered = false
        toggleButton.contentTintColor = .secondaryLabelColor
        toggleButton.translatesAutoresizingMaskIntoConstraints = false

        placeholderLabel.font = .systemFont(ofSize: 11)
        placeholderLabel.textColor = .tertiaryLabelColor
        placeholderLabel.translatesAutoresizingMaskIntoConstraints = false

        let container = NSView()
        container.addSubview(headerLabel)
        container.addSubview(searchField)
        container.addSubview(toggleButton)
        container.addSubview(scrollView)
        container.addSubview(placeholderLabel)
        NSLayoutConstraint.activate([
            headerLabel.leadingAnchor.constraint(equalTo: container.leadingAnchor, constant: 12),
            headerLabel.trailingAnchor.constraint(lessThanOrEqualTo: container.trailingAnchor, constant: -12),
            headerLabel.topAnchor.constraint(equalTo: container.topAnchor, constant: 10),
            scrollView.leadingAnchor.constraint(equalTo: container.leadingAnchor),
            scrollView.trailingAnchor.constraint(equalTo: container.trailingAnchor),
            searchField.leadingAnchor.constraint(equalTo: container.leadingAnchor, constant: 12),
            searchField.trailingAnchor.constraint(equalTo: toggleButton.leadingAnchor, constant: -6),
            toggleButton.trailingAnchor.constraint(equalTo: container.trailingAnchor, constant: -12),
            toggleButton.centerYAnchor.constraint(equalTo: searchField.centerYAnchor),
            placeholderLabel.leadingAnchor.constraint(equalTo: container.leadingAnchor, constant: 12),
            placeholderLabel.trailingAnchor.constraint(lessThanOrEqualTo: container.trailingAnchor, constant: -12),
        ])
        // Open: the list fills between the heading and the search row. Closed:
        // the bar is the whole pane, so the search sits centred on its own.
        expandedConstraints = [
            scrollView.topAnchor.constraint(equalTo: headerLabel.bottomAnchor, constant: 4),
            scrollView.bottomAnchor.constraint(equalTo: searchField.topAnchor, constant: -4),
            searchField.bottomAnchor.constraint(equalTo: container.bottomAnchor, constant: -8),
            placeholderLabel.topAnchor.constraint(equalTo: scrollView.topAnchor, constant: 4),
        ]
        collapsedConstraints = [
            scrollView.topAnchor.constraint(equalTo: container.topAnchor),
            scrollView.bottomAnchor.constraint(equalTo: container.bottomAnchor),
            searchField.centerYAnchor.constraint(equalTo: container.centerYAnchor),
            placeholderLabel.topAnchor.constraint(equalTo: container.topAnchor, constant: 4),
        ]
        view = container
        updateVisibility()
    }

    /// Scans the roots in the background and shows what it finds. Repeated
    /// calls while a scan runs are ignored; a scan is quick and rare.
    func load() {
        guard !isLoading else { return }
        isLoading = true
        let roots = roots
        queue.async { [weak self] in
            let found = HistoryStore.scan(roots: roots)
            DispatchQueue.main.async {
                guard let self else { return }
                self.isLoading = false
                self.sessions = found
                self.applyQuery()
            }
        }
    }

    /// Replaces what is listed, for tests.
    func setSessions(_ sessions: [HistorySession]) {
        self.sessions = sessions
        applyQuery()
    }

    private func applyQuery() {
        shown = HistoryStore.filter(sessions, query: searchField.stringValue)
        tableView.reloadData()
        placeholderLabel.stringValue = sessions.isEmpty ? "No saved sessions" : "No matches"
        updateVisibility()
    }

    /// Shows only what the state calls for: the search bar alone while
    /// collapsed, and the heading with the list once it is open.
    private func updateVisibility() {
        guard isViewLoaded else { return }
        NSLayoutConstraint.deactivate(isExpanded ? collapsedConstraints : expandedConstraints)
        NSLayoutConstraint.activate(isExpanded ? expandedConstraints : collapsedConstraints)
        headerLabel.isHidden = !isExpanded
        let empty = shown.isEmpty
        placeholderLabel.isHidden = !isExpanded || !empty
        scrollView.isHidden = !isExpanded || empty
        // The chevron points the way the list will move when pressed.
        let expanded = isExpanded
        toggleButton.image = NSImage(systemSymbolName: expanded ? "chevron.down" : "chevron.right",
                                     accessibilityDescription: expanded ? "Hide sessions" : "Show sessions")
        toggleButton.toolTip = expanded ? "Hide sessions" : "Show sessions"
    }

    @objc private func toggleExpanded() {
        isExpanded.toggle()
        onExpandedChange?(isExpanded)
    }

    @objc private func searchChanged() {
        // A query opens the list, so its matches are visible rather than
        // filtering a list the user cannot see.
        if !searchField.stringValue.trimmingCharacters(in: .whitespaces).isEmpty, !isExpanded {
            toggleExpanded()
        } else {
            applyQuery()
        }
    }

    @objc private func openClickedSession() {
        let row = tableView.clickedRow
        guard shown.indices.contains(row) else { return }
        delegate?.historyList(self, didSelect: shown[row])
    }

    // MARK: NSTableViewDataSource

    func numberOfRows(in tableView: NSTableView) -> Int {
        shown.count
    }

    // MARK: NSTableViewDelegate

    func tableView(_ tableView: NSTableView, rowViewForRow row: Int) -> NSTableRowView? {
        SidebarRowView()
    }

    func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
        guard shown.indices.contains(row) else { return nil }
        let identifier = NSUserInterfaceItemIdentifier("historyCell")
        let cell = tableView.makeView(withIdentifier: identifier, owner: nil) as? HistoryCellView ?? {
            let cell = HistoryCellView()
            cell.identifier = identifier
            return cell
        }()
        cell.session = shown[row]
        return cell
    }
}

private final class HistoryCellView: NSTableCellView {
    private let titleLabel = NSTextField(labelWithString: "")
    private let subtitleLabel = NSTextField(labelWithString: "")

    var session: HistorySession? {
        didSet { update() }
    }

    init() {
        super.init(frame: .zero)
        titleLabel.font = .systemFont(ofSize: 12)
        titleLabel.lineBreakMode = .byTruncatingTail
        subtitleLabel.font = .monospacedSystemFont(ofSize: 10, weight: .regular)
        subtitleLabel.textColor = .secondaryLabelColor
        subtitleLabel.lineBreakMode = .byTruncatingMiddle

        let labels = NSStackView(views: [titleLabel, subtitleLabel])
        labels.orientation = .vertical
        labels.alignment = .leading
        labels.spacing = 1
        labels.translatesAutoresizingMaskIntoConstraints = false
        addSubview(labels)
        NSLayoutConstraint.activate([
            labels.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 12),
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
        guard let session else { return }
        titleLabel.stringValue = session.displayTitle
        let age = RelativeAge.string(since: session.modified)
        subtitleLabel.stringValue = [session.agent, session.cwd.abbreviatingHome, age]
            .filter { !$0.isEmpty }
            .joined(separator: " · ")
    }
}
