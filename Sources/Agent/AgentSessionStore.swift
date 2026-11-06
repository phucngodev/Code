import Foundation

/// What each terminal's coding agent is doing, built from the events `cde`
/// sends. Indicator observers are notified only when the visible state changes.
@MainActor
final class AgentSessionStore {
    enum State: Equatable {
        case idle, working, waiting, ended
    }

    struct Session: Equatable {
        var agent: String
        var sessionID: String
        var state: State = .idle
        var detail: String?
        var attention: String?
        /// A turn ended and the user has not looked at the terminal since.
        var finishedUnseen = false
        var startedAt: Date
        var updatedAt: Date
        /// When the session took the indicator it shows, so agents can be
        /// taken in the order they started waiting or finished. Repeated
        /// events keep it; nil while it shows none.
        var indicatorSince: Date?

        /// A finished turn or a message the user has not looked at since.
        var isUnread: Bool { finishedUnseen || attention != nil }
    }

    /// One applied event, with the terminal's session before and after it.
    struct Update {
        var alerts: Bool {
            switch kind {
            case .needsInput, .turnEnded, .attention: true
            case .sessionStarted, .turnStarted, .working, .sessionEnded, .unknown: false
            }
        }

        var surface: UUID
        var kind: AgentEvent.Kind
        var previous: Session?
        var session: Session

        /// A wait for the same reason the terminal was already waiting for,
        /// which the user has already been told about.
        var repeatsWait: Bool {
            kind == .needsInput && previous?.state == .waiting && previous?.detail == session.detail
        }
    }

    /// What to do with a desktop notification a terminal asked for.
    enum NotificationRoute: Equatable {
        /// The terminal's agent is telling Code what it is doing.
        case event(AgentEvent)
        /// The agent's hooks already reported it.
        case ignore
        /// Not the agent's; post it as the terminal's.
        case post
    }

    private(set) var sessions: [UUID: Session] = [:]
    var onChange: ((UUID) -> Void)?
    var onUpdate: ((Update) -> Void)?
    /// Fired when a terminal gains or loses an unread alert. Separate from
    /// `onChange`, which only reports the visible indicator changing.
    var onUnreadChange: (() -> Void)?
    private let isLive: (UUID) -> Bool
    private let isVisible: (UUID) -> Bool

    /// `isLive` tells whether a terminal with that id is open; events for any
    /// other id are dropped. `isVisible` tells whether the user is looking at
    /// it, in which case a finished turn is already seen.
    init(isLive: @escaping (UUID) -> Bool, isVisible: @escaping (UUID) -> Bool = { _ in false }) {
        self.isLive = isLive
        self.isVisible = isVisible
    }

    func apply(_ event: AgentEvent) {
        guard event.kind != .unknown else {
            logger.notice("agent event of an unknown kind dropped")
            return
        }
        guard let surface = event.surfaceID, isLive(surface) else {
            logger.notice("agent event \(event.kind.rawValue, privacy: .public) dropped: no terminal \(event.surfaceID?.uuidString ?? "nil", privacy: .public)")
            return
        }
        let previous = sessions[surface]
        // Resuming or compacting announces the running session again, and
        // startup hooks can land after its turn has begun.
        let restatesLiveSession = event.kind == .sessionStarted && previous.map { $0.sessionID == event.sessionID && $0.state != .ended } == true
        // Each hook reaches the socket in its own process, so a later hook can
        // overtake an earlier one.
        if let previous, event.kind != .sessionStarted, event.at < previous.updatedAt {
            logger.notice("agent event \(event.kind.rawValue, privacy: .public) ignored: older than the last one for terminal \(surface.uuidString, privacy: .public)")
            return
        }
        logger.debug("agent event \(event.kind.rawValue, privacy: .public) from \(event.agent, privacy: .public) for terminal \(surface.uuidString, privacy: .public)")

        // A session that ended, or was never announced, is started afresh by
        // whatever the agent says next, in case its sessionStarted was missed.
        var session = previous.flatMap { (event.kind == .sessionStarted && !restatesLiveSession) || $0.state == .ended ? nil : $0 }
            ?? Session(agent: event.agent, sessionID: event.sessionID, startedAt: event.at, updatedAt: event.at)
        session.agent = event.agent
        session.sessionID = event.sessionID
        session.updatedAt = max(session.updatedAt, event.at)
        if event.kind != .attention, !restatesLiveSession {
            session.detail = event.detail
        }

        switch event.kind {
        case .sessionStarted, .unknown:
            break
        case .turnStarted, .working:
            session.state = .working
            session.finishedUnseen = false
        case .needsInput:
            session.state = .waiting
        case .turnEnded:
            session.state = .idle
            session.finishedUnseen = !isVisible(surface)
        case .sessionEnded:
            session.state = .ended
        case .attention:
            session.attention = event.detail
        }
        if previous.flatMap(AgentIndicator.init) != AgentIndicator(session) {
            session.indicatorSince = AgentIndicator(session) == nil ? nil : event.at
        }
        sessions[surface] = session
        onUpdate?(Update(surface: surface, kind: event.kind, previous: previous, session: session))
        if (previous?.isUnread ?? false) != session.isUnread {
            onUnreadChange?()
        }
        if previous.flatMap(AgentIndicator.init) != AgentIndicator(session) {
            onChange?(surface)
        }
    }

    func route(notification message: String, from surface: UUID, workspaceID: UUID?, cwd: String, at: Date) -> NotificationRoute {
        guard let session = sessions[surface], session.state != .ended,
              let translate = AgentAdapter.named(session.agent)?.translateNotification else { return .post }
        guard let (kind, detail) = translate(message) else { return .ignore }
        return .event(AgentEvent(agent: session.agent, kind: kind, sessionID: session.sessionID, surfaceID: surface,
                                 workspaceID: workspaceID, cwd: cwd, at: at, detail: detail))
    }

    func markSeen(_ surface: UUID) {
        guard var session = sessions[surface], session.isUnread else { return }
        let previous = AgentIndicator(session)
        session.finishedUnseen = false
        session.attention = nil
        if AgentIndicator(session) == nil { session.indicatorSince = nil }
        sessions[surface] = session
        if previous != AgentIndicator(session) { onChange?(surface) }
        onUnreadChange?()
    }

    /// Terminals in these that finished or asked for attention and have not
    /// been looked at since.
    func unreadCount(for surfaces: some Sequence<UUID>) -> Int {
        surfaces.reduce(into: 0) { count, surface in
            if sessions[surface]?.isUnread == true { count += 1 }
        }
    }

    /// Terminals in these whose agent is waiting for the user or finished
    /// unseen. Unlike `unreadCount`, a wait still in progress counts, so this
    /// is what the titlebar bell shows.
    func pendingCount(for surfaces: some Sequence<UUID>) -> Int {
        surfaces.reduce(into: 0) { count, surface in
            guard let session = sessions[surface], session.state != .ended else { return }
            if session.isUnread || session.state == .waiting { count += 1 }
        }
    }

    var totalUnread: Int { unreadCount(for: sessions.keys) }

    /// The most urgent status among these terminals, dated from when it was
    /// taken so its age can be shown.
    func status(for surfaces: some Sequence<UUID>) -> AgentStatus? {
        surfaces.compactMap { status(of: $0) }.max { a, b in
            a.indicator == b.indicator ? a.since > b.since : a.indicator < b.indicator
        }
    }

    /// The name the terminal's agent reports, until its session ends. An
    /// event sent by hand without a name has no agent to show.
    func runningAgent(in surface: UUID) -> String? {
        guard let session = sessions[surface], session.state != .ended, !session.agent.isEmpty else { return nil }
        return session.agent
    }

    func remove(_ surface: UUID) {
        let wasUnread = sessions[surface]?.isUnread ?? false
        sessions.removeValue(forKey: surface)
        if wasUnread { onUnreadChange?() }
    }
}
