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
