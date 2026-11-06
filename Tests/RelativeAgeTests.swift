import XCTest

final class RelativeAgeTests: XCTestCase {
    let now = Date(timeIntervalSince1970: 1_000_000)

    private func age(_ seconds: TimeInterval) -> String {
        RelativeAge.string(since: now.addingTimeInterval(-seconds), now: now)
    }

    func testAgeRoundsDownThroughSecondsMinutesHoursAndDays() {
        XCTAssertEqual(age(0), "now")
        XCTAssertEqual(age(59), "now")
        XCTAssertEqual(age(60), "1m")
        XCTAssertEqual(age(3599), "59m")
        XCTAssertEqual(age(3600), "1h")
        XCTAssertEqual(age(86_399), "23h")
        XCTAssertEqual(age(86_400), "1d")
        XCTAssertEqual(age(864_000), "10d")
    }

    func testAFutureDateReadsAsNow() {
        XCTAssertEqual(RelativeAge.string(since: now.addingTimeInterval(60), now: now), "now")
    }
}
