import AppKit
import XCTest

@MainActor
final class UnreadBadgeViewTests: XCTestCase {
    private func hasVisiblePixel(_ rep: NSBitmapImageRep) -> Bool {
        for x in 0..<rep.pixelsWide {
            for y in 0..<rep.pixelsHigh {
                if let color = rep.colorAt(x: x, y: y), color.alphaComponent > 0.1 { return true }
            }
        }
        return false
    }

    private func rendered(count: Int) -> NSBitmapImageRep {
        let badge = UnreadBadgeView()
        badge.count = count
        badge.frame = NSRect(x: 0, y: 0, width: max(14, badge.intrinsicContentSize.width), height: UnreadBadgeView.height)
        let rep = badge.bitmapImageRepForCachingDisplay(in: badge.bounds)!
        badge.cacheDisplay(in: badge.bounds, to: rep)
        return rep
    }

    func testACountDrawsAndZeroDoesNot() {
        XCTAssertTrue(hasVisiblePixel(rendered(count: 3)), "a count draws the capsule")
        XCTAssertFalse(hasVisiblePixel(rendered(count: 0)), "no count draws nothing")
    }

    func testTheCapsuleGrowsWithTheCount() {
        let one = UnreadBadgeView()
        one.count = 1
        let many = UnreadBadgeView()
        many.count = 99
        XCTAssertGreaterThan(many.intrinsicContentSize.width, one.intrinsicContentSize.width)
    }

    func testTheBadgeLetsClicksThroughToTheControlItAnnotates() {
        let container = NSView(frame: NSRect(x: 0, y: 0, width: 32, height: 28))
        let badge = UnreadBadgeView()
        badge.count = 1
        badge.translatesAutoresizingMaskIntoConstraints = false
        container.addSubview(badge)
        NSLayoutConstraint.activate([
            badge.trailingAnchor.constraint(equalTo: container.trailingAnchor, constant: -2),
            badge.topAnchor.constraint(equalTo: container.topAnchor, constant: 1),
        ])
        container.layoutSubtreeIfNeeded()

        // A point on the badge, in the coordinate system hitTest expects.
        let onBadge = NSPoint(x: badge.frame.midX, y: badge.frame.midY)
        XCTAssertTrue(badge.bounds.contains(badge.convert(onBadge, from: container)), "the sampled point is on the badge")
        XCTAssertNil(badge.hitTest(onBadge), "a click on the badge falls through to the control beneath it")
    }
}
