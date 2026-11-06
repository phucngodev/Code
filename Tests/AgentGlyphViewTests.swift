import AppKit
import XCTest

@MainActor
final class AgentGlyphViewTests: XCTestCase {
    private func rendered(_ indicator: AgentIndicator?) -> NSBitmapImageRep {
        let view = AgentGlyphView()
        view.frame = NSRect(x: 0, y: 0, width: AgentGlyphView.size, height: AgentGlyphView.size)
        view.indicator = indicator
        let rep = view.bitmapImageRepForCachingDisplay(in: view.bounds)!
        view.cacheDisplay(in: view.bounds, to: rep)
        return rep
    }

    private func hasVisiblePixel(_ rep: NSBitmapImageRep) -> Bool {
        for x in 0..<rep.pixelsWide {
            for y in 0..<rep.pixelsHigh {
                guard let color = rep.colorAt(x: x, y: y) else { continue }
                if color.alphaComponent > 0.1 { return true }
            }
        }
        return false
    }

    func testEachStateDrawsSomething() {
        XCTAssertTrue(hasVisiblePixel(rendered(.working)), "working draws the arc")
        XCTAssertTrue(hasVisiblePixel(rendered(.waiting)), "waiting draws the dot")
        XCTAssertTrue(hasVisiblePixel(rendered(.done)), "done draws the check")
        XCTAssertFalse(hasVisiblePixel(rendered(nil)), "idle draws nothing")
    }
}
