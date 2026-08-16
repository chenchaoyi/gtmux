import AppKit
import XCTest
@testable import GtmuxBar

/// A closed surface must not keep its SwiftUI graph.
///
/// The all-panes window is kept across closes (`isReleasedWhenClosed = false`) so it
/// reopens where the user left it. Keeping the WINDOW is fine; keeping its CONTENT is not.
/// The working ring animates with `.repeatForever`, which goes on running against a window
/// nobody can see — measured on a fresh build with one working agent: 0.1–0.3 % CPU before
/// anything was opened, 4.6–11 % after opening and closing, climbing with every re-open.
///
/// Stopping the animation was tried first and does NOT work: a running repeatForever
/// survives its driving state flipping and its animation being replaced with nil. What
/// ends an animation is not having the view.
final class SurfaceTeardownTests: XCTestCase {
    func testTheBrowserDropsItsContentOnCloseAndRebuildsOnOpen() {
        let c = PaneBrowserController()
        let l10n = L10n.shared
        let radar = AgentStore()

        c.show(l10n: l10n, radar: radar)
        guard let w = c.window else { return XCTFail("show() made no window") }
        XCTAssertNotNil(w.contentViewController, "an open browser must have content")

        NotificationCenter.default.post(name: NSWindow.willCloseNotification, object: w)
        XCTAssertNil(w.contentViewController,
                     "a closed window must drop its view graph — a kept graph keeps animating")

        // Reopening must rebuild it, or the second open shows an empty frame.
        c.show(l10n: l10n, radar: radar)
        XCTAssertNotNil(c.window?.contentViewController, "reopening must rebuild the content")
        XCTAssertTrue(c.window === w, "the window itself is kept, so its size and position survive")
        c.window?.close()
    }
}

/// Clicking the status item to CLOSE the panel must close it — not close and reopen it.
///
/// A transient popover is dismissed by AppKit on mouse-DOWN; the status item's action
/// arrives on mouse-UP, when `isShown` is already false. Without a guard the toggle reads
/// that as "open it", and the panel the user dismissed comes back. That is the reported
/// "clicking fast sometimes does nothing" — a close and an open cancelling out.
final class PopoverClickTests: XCTestCase {
    func testTheOpenThatFollowsItsOwnCloseIsIgnored() {
        let close = Date()
        XCTAssertTrue(PopoverClick.reopenIsAnEcho(ofCloseAt: close, now: close.addingTimeInterval(0.01)),
                      "the mouse-up half of the same click must not reopen the panel")
        XCTAssertTrue(PopoverClick.reopenIsAnEcho(ofCloseAt: close, now: close.addingTimeInterval(0.15)))
    }

    /// The other half matters as much: a user who closes the panel and then decides to
    /// open it again must be able to. Swallowing that is the same bug wearing a hat.
    func testADeliberateReopenIsNotSwallowed() {
        let close = Date()
        XCTAssertFalse(PopoverClick.reopenIsAnEcho(ofCloseAt: close, now: close.addingTimeInterval(0.35)))
        XCTAssertFalse(PopoverClick.reopenIsAnEcho(ofCloseAt: close, now: close.addingTimeInterval(2)))
    }

    /// Never opened yet: `distantPast` must not read as "just closed", or the very first
    /// click of a session would be eaten.
    func testTheFirstEverClickOpens() {
        XCTAssertFalse(PopoverClick.reopenIsAnEcho(ofCloseAt: .distantPast, now: Date()))
    }
}
