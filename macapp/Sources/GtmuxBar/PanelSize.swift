import AppKit
import Combine
import SwiftUI

/// ListContentHeight carries a scrolling list's measured content height up to the view
/// that sizes it. A `ScrollView` cannot report an intrinsic height — it is happy at any
/// size — so the content inside it has to say how tall it wants to be.
struct ListContentHeight: PreferenceKey {
    static var defaultValue: CGFloat = 0
    static func reduce(value: inout CGFloat, nextValue: () -> CGFloat) {
        value = max(value, nextValue())
    }
}

/// ChromeHeight totals a surface's non-list parts — header above, footer below — so the
/// list can be given exactly the room that is left.
///
/// It SUMS rather than maxes, unlike the list's own measurement: separate groups report
/// into this key and all of them occupy height. The reserve was a CONSTANT before, which
/// could not track chrome that comes and goes with what is running — the popover's HQ card
/// only with a supervisor, its update banner only with an update.
struct ChromeHeight: PreferenceKey {
    static var defaultValue: CGFloat = 0
    static func reduce(value: inout CGFloat, nextValue: () -> CGFloat) {
        value += nextValue()
    }
}

/// PanelSize carries the popover panel's settled height from SwiftUI out to AppKit.
///
/// WHY THIS EXISTS — the measurement, not the theory. `NSPopover` positions its window
/// from `contentSize`. A SwiftUI view inside an `NSHostingController` that resizes ITSELF
/// never updates that property, and the popover then keeps the window's BOTTOM edge and
/// lets the extra height run off the top of the screen. Read live on v0.50.0, twelve
/// agents, a 1728×1117 display:
///
///     panel measured 732pt · popover.contentSize 320×320 · window 758pt tall at y=743
///     → the window's top edge sat at 1501, and the screen ends at 1084
///
/// Four hundred points of header, HQ card and the first rows were above the display edge.
/// Note what the numbers rule out: 732 fits the 983pt budget with room to spare, so the
/// panel was never too TALL — it was never DECLARED. Two fixes that reasoned about height
/// alone (a bigger reserve, then a measured one) both failed for that reason.
///
/// So the panel measures itself, publishes it here, and the host sets `contentSize`.
final class PanelSize: ObservableObject {
    static let shared = PanelSize()
    /// The panel's own measured height. The host clamps it to the screen budget before
    /// handing it to the popover; PanelMetrics owns that arithmetic.
    @Published var height: CGFloat = 0
}

/// PanelMetrics is the popover's height arithmetic, kept pure so it can be tested without
/// a screen — the bug above was invisible to "it builds and the tests pass".
enum PanelMetrics {
    /// The list never grows past this, so the panel stays a popover and not a full-height
    /// panel even on a very tall display.
    static let listCeiling: CGFloat = 820
    /// ...and never collapses below this, so a small display still shows a few rows.
    static let listFloor: CGFloat = 140
    /// The popover's own arrow and corner padding, plus a hair so the panel never sits
    /// flush against the screen edge.
    static let edgeMargin: CGFloat = 24

    /// visibleHeight is the usable height of the display holding the menu bar.
    static var visibleHeight: CGFloat { NSScreen.main?.visibleFrame.height ?? 900 }

    /// budget is how tall the WHOLE panel may be.
    static func budget(visible: CGFloat) -> CGFloat { max(240, visible - edgeMargin) }

    /// listMax is how tall the scrolling list may be, given the MEASURED chrome (header +
    /// HQ card + footer + update banner — all of which come and go with what is running,
    /// which is why the reserve cannot be a constant).
    static func listMax(visible: CGFloat, chrome: CGFloat) -> CGFloat {
        min(listCeiling, max(listFloor, budget(visible: visible) - chrome))
    }

    /// popoverHeight is what the popover must be TOLD it is. Clamped to the budget: the
    /// list cap already keeps the panel inside it, and this is the backstop that holds
    /// even if some future chrome grows past what was measured.
    static func popoverHeight(panel: CGFloat, visible: CGFloat) -> CGFloat {
        min(max(panel, 1), budget(visible: visible))
    }

    /// A window's own floor and ceiling. Unlike the popover, a window carries a title bar
    /// OUTSIDE its content, and it must not cover the Dock — so the room left for content
    /// is the usable height less the title bar and a margin.
    static let windowFloor: CGFloat = 400

    /// The design width for the browser (DESIGN §11). It has to be stated: an
    /// `NSHostingController` sizes its window to the SwiftUI IDEAL, so a view that only
    /// declares a minimum gets a window at that minimum — the browser was created at 480
    /// and measured 420 wide for exactly that reason.
    static let browserWidth: CGFloat = 480

    /// How much of the display the browser deliberately leaves alone. Generous on
    /// purpose: a window that opens at very nearly full height reads as taking the screen
    /// over rather than as a panel you opened, and the list scrolls anyway.
    static let windowMargin: CGFloat = 120

    /// windowContentHeight is how tall to make the pane browser's CONTENT so the window
    /// shows its sessions without scrolling, while still fitting the display. Same idea as
    /// the popover, different container: a window is resizable and the user is in charge
    /// of it, so this is what to open AT, not a cap enforced while they use it.
    static func windowContentHeight(desired: CGFloat, visible: CGFloat, titleBar: CGFloat) -> CGFloat {
        let room = max(windowFloor, visible - titleBar - windowMargin)
        return min(max(desired, windowFloor), room)
    }

    /// hasLeftTheScreen decides whether an open panel needs to be re-attached to the
    /// status item. A resize does not re-anchor the popover, and with a big fleet the
    /// first opening lands far off-screen (measured at 300 agents: y=-3504).
    ///
    /// The asymmetry is deliberate and load-bearing. The panel's top edge legitimately
    /// reaches the top of the DISPLAY — its arrow tucks under the menu bar — so the
    /// ceiling is the screen's full frame, while the floor is the usable area above the
    /// Dock. Judging the top against `visibleFrame` instead would call a correctly placed
    /// panel misplaced, and since the repair is a re-show, which is itself a layout, that
    /// would re-show forever.
    static func hasLeftTheScreen(window: CGRect, screenFrame: CGRect, visibleFrame: CGRect) -> Bool {
        window.maxY > screenFrame.maxY || window.minY < visibleFrame.minY
    }
}
