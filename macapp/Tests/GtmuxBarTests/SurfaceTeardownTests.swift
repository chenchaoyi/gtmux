import AppKit
import XCTest
@testable import GtmuxBar

/// The working ring must not animate against a surface nobody can see — and the surface
/// must not be rebuilt to achieve that.
///
/// Three approaches were measured on a fresh build with one working agent:
///
///   as shipped (ring animating in a closed panel) ... 4.6 – 11 % CPU, climbing
///   telling the animation to stop ................... unchanged; a running
///                                                     repeatForever survives its state
///                                                     flipping and its animation being
///                                                     replaced with nil
///   tearing the view tree down ...................... 0.0 – 0.2 %, but every re-open laid
///                                                     out from scratch: the first pass
///                                                     measured 168pt inside a window still
///                                                     sized 983 for the previous content,
///                                                     so the panel opened with a band of
///                                                     empty space above it and its last
///                                                     rows past the bottom edge
///   swapping the RING view (this) ................... 1.0 – 1.3 %, flat, and the tree is
///                                                     built once
///
/// Removing a view is what ends its animation. Removing the smallest view that carries one
/// is what leaves the rest of the panel measured and ready.
final class MenuVisibilityTests: XCTestCase {
    func testVisibilityIsOffUntilASurfaceOpens() {
        let v = MenuVisibility.shared
        v.setVisible(false)
        XCTAssertFalse(v.visible, "nothing is on screen at launch, so nothing should animate")
    }

    func testItTracksOpenAndClose() {
        let v = MenuVisibility.shared
        v.setVisible(true)
        XCTAssertTrue(v.visible)
        v.setVisible(false)
        XCTAssertFalse(v.visible, "a closed surface must stop animating")
    }
}

/// The all-panes window keeps BOTH its frame and its content across a close: the frame so
/// it reopens where the user left it, the content so it reopens already laid out.
final class PaneBrowserWindowTests: XCTestCase {
    func testTheWindowAndItsContentSurviveAClose() {
        let c = PaneBrowserController()
        c.show(l10n: L10n.shared, radar: AgentStore())
        guard let w = c.window else { return XCTFail("show() made no window") }
        XCTAssertNotNil(w.contentViewController)

        NotificationCenter.default.post(name: NSWindow.willCloseNotification, object: w)
        XCTAssertNotNil(w.contentViewController,
                        "the content stays — rebuilding it is what made the panel open mis-sized")

        c.show(l10n: L10n.shared, radar: AgentStore())
        XCTAssertTrue(c.window === w, "same window, so its size and position survive")
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
