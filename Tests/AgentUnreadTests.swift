import XCTest

@MainActor
final class AgentUnreadTests: XCTestCase {
    private var store: AgentSessionStore!
    private var unreadChanges = 0

    override func setUp() async throws {
        store = AgentSessionStore(isLive: { _ in true }, isVisible: { _ in false })
        store.onUnreadChange = { [unowned self] in self.unreadChanges += 1 }
    }

    private func send(_ kind: AgentEvent.Kind, to surface: UUID, at: Date = Date(), detail: String? = nil) {
        store.apply(AgentEvent(agent: "test", kind: kind, surfaceID: surface, workspaceID: nil, cwd: "/", at: at, detail: detail))
    }

    func testAFinishedTurnIsUnreadUntilSeen() {
        let surface = UUID()
        XCTAssertEqual(store.unreadCount(for: [surface]), 0)
        send(.turnStarted, to: surface)
        XCTAssertEqual(store.unreadCount(for: [surface]), 0)
        send(.turnEnded, to: surface)
        XCTAssertEqual(store.unreadCount(for: [surface]), 1)
        XCTAssertEqual(store.totalUnread, 1)
        store.markSeen(surface)
        XCTAssertEqual(store.unreadCount(for: [surface]), 0)
        XCTAssertEqual(store.totalUnread, 0)
    }

    func testATurnEndingInViewIsAlreadySeen() {
        let surface = UUID()
        let seen = AgentSessionStore(isLive: { _ in true }, isVisible: { _ in true })
        seen.apply(AgentEvent(agent: "test", kind: .turnStarted, surfaceID: surface, workspaceID: nil, cwd: "/", at: Date()))
        seen.apply(AgentEvent(agent: "test", kind: .turnEnded, surfaceID: surface, workspaceID: nil, cwd: "/", at: Date()))
        XCTAssertEqual(seen.unreadCount(for: [surface]), 0)
    }

    func testAttentionMarksATerminalUnreadWithoutChangingItsIndicator() {
        let surface = UUID()
        send(.turnStarted, to: surface)
        send(.attention, to: surface, detail: "ping")
        XCTAssertEqual(store.unreadCount(for: [surface]), 1)
        XCTAssertEqual(store.indicator(for: [surface]), .working)
        store.markSeen(surface)
        XCTAssertEqual(store.unreadCount(for: [surface]), 0)
    }

    func testUnreadChangesAreReportedButWorkingIsNot() {
        let surface = UUID()
        send(.turnStarted, to: surface)
        send(.working, to: surface)
        XCTAssertEqual(unreadChanges, 0)
        send(.turnEnded, to: surface)
        XCTAssertEqual(unreadChanges, 1)
        store.markSeen(surface)
        XCTAssertEqual(unreadChanges, 2)
    }

    func testRemovingAnUnreadTerminalReportsIt() {
        let surface = UUID()
        send(.turnStarted, to: surface)
        send(.turnEnded, to: surface)
        store.remove(surface)
        XCTAssertEqual(store.unreadCount(for: [surface]), 0)
        XCTAssertEqual(unreadChanges, 2)
    }

    func testPendingCountsAWaittoo() {
        let surface = UUID()
        send(.turnStarted, to: surface)
        XCTAssertEqual(store.pendingCount(for: [surface]), 0)
        send(.needsInput, to: surface, detail: "permission")
        XCTAssertEqual(store.pendingCount(for: [surface]), 1)
        XCTAssertEqual(store.unreadCount(for: [surface]), 0, "a wait is not unread")
        send(.working, to: surface)
        XCTAssertEqual(store.pendingCount(for: [surface]), 0, "resuming clears the wait")
        send(.turnEnded, to: surface)
        XCTAssertEqual(store.pendingCount(for: [surface]), 1, "a finished turn counts until seen")
        store.markSeen(surface)
        XCTAssertEqual(store.pendingCount(for: [surface]), 0)
    }

    func testWorkspaceStatusPicksTheMostUrgentAndItsStart() {
        let done = UUID(), waiting = UUID()
        let start = Date(timeIntervalSince1970: 100)
        send(.turnStarted, to: done, at: start)
        send(.turnEnded, to: done, at: start.addingTimeInterval(5))
        send(.turnStarted, to: waiting, at: start.addingTimeInterval(1))
        send(.needsInput, to: waiting, at: start.addingTimeInterval(2))

        XCTAssertEqual(store.status(for: [done, waiting]),
                       AgentStatus(indicator: .waiting, since: start.addingTimeInterval(2)))
    }

    func testWorkspaceStatusTakesTheEarliestOfTheSameIndicator() {
        let first = UUID(), second = UUID()
        let start = Date(timeIntervalSince1970: 100)
        send(.turnStarted, to: first, at: start)
        send(.needsInput, to: first, at: start.addingTimeInterval(1))
        send(.turnStarted, to: second, at: start.addingTimeInterval(2))
        send(.needsInput, to: second, at: start.addingTimeInterval(3))

        XCTAssertEqual(store.status(for: [second, first])?.since, start.addingTimeInterval(1))
    }
}
