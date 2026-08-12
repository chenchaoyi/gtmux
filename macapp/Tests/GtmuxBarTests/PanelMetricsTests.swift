import AppKit
import XCTest
@testable import GtmuxBar

/// Tests for the popover panel's height arithmetic.
///
/// These exist because the v0.50.0 off-screen panel passed every test in this suite. The
/// numbers below are not invented: they were read off the real machine with
/// `GTMUXBAR_DEBUG=1 GTMUXBAR_MEASURE=1`, twelve agents, a 1728×1117 display.
final class PanelMetricsTests: XCTestCase {

    // The measured reality this file is written against.
    private let visible: CGFloat = 1007 // NSScreen.main.visibleFrame.height
    private let chrome: CGFloat = 160   // header + HQ card + divider + footer
    private let list: CGFloat = 572     // what twelve agent rows want

    /// The panel must fit the screen whatever the fleet does — and it must fit BY
    /// CONSTRUCTION, before any clamp. Asserting on the clamped number instead would be
    /// tautological: `popoverHeight` clamps, so that assertion passes even on a cap that
    /// ignores the chrome entirely (verified — the old formula passes it).
    func testPanelFitsBeforeItIsEverClamped() {
        for chrome in stride(from: CGFloat(80), through: 400, by: 20) {
            for screen in stride(from: CGFloat(700), through: 1600, by: 100) {
                let cap = PanelMetrics.listMax(visible: screen, chrome: chrome)
                let panel = chrome + cap // the tallest this configuration can ever be
                let budget = PanelMetrics.budget(visible: screen)
                // The one exception is the floor: on a display too small to hold the
                // chrome, the list keeps a couple of rows and the clamp takes over.
                guard cap > PanelMetrics.listFloor else { continue }
                XCTAssertLessThanOrEqual(panel, budget, "chrome=\(chrome) screen=\(screen)")
            }
        }
    }

    /// The real fleet must fit WITHOUT being clamped — the panel that broke was 732pt on a
    /// 983pt budget, so clamping was never what it needed.
    func testTheMeasuredFleetFitsUnclamped() {
        let cap = PanelMetrics.listMax(visible: visible, chrome: chrome)
        XCTAssertGreaterThanOrEqual(cap, list, "twelve agents must not be forced to scroll")
        let panel = chrome + min(list, cap)
        XCTAssertEqual(PanelMetrics.popoverHeight(panel: panel, visible: visible), 732,
                       "the panel is passed through, not clamped")
    }

    /// Chrome is measured because it CHANGES: the HQ card is there only with a supervisor
    /// running, the update banner only when there is an update. A fixed reserve cannot
    /// track that, and the list must give up exactly the room the chrome took.
    func testMoreChromeLeavesLessList() {
        let bare = PanelMetrics.listMax(visible: 700, chrome: 120)
        let loaded = PanelMetrics.listMax(visible: 700, chrome: 220)
        XCTAssertEqual(bare - loaded, 100, "the list yields point for point")
        XCTAssertLessThanOrEqual(220 + loaded, PanelMetrics.budget(visible: 700))
    }

    /// A small display still shows rows rather than collapsing to a sliver, even when the
    /// chrome alone would eat the whole budget.
    func testTinyDisplayKeepsAFloor() {
        XCTAssertEqual(PanelMetrics.listMax(visible: 300, chrome: 400), PanelMetrics.listFloor)
    }

    /// A very tall display does not turn the popover into a full-height panel.
    func testHugeDisplayKeepsItAPopover() {
        XCTAssertEqual(PanelMetrics.listMax(visible: 2400, chrome: 160), PanelMetrics.listCeiling)
    }

    /// The backstop: even if some future chrome outgrows what was measured, what the
    /// popover is TOLD stays inside the screen.
    func testPopoverHeightIsClampedToTheBudget() {
        XCTAssertEqual(PanelMetrics.popoverHeight(panel: 5000, visible: visible),
                       PanelMetrics.budget(visible: visible))
        XCTAssertEqual(PanelMetrics.popoverHeight(panel: 0, visible: visible), 1,
                       "never zero — a zero-height popover is an invisible one")
    }

    // MARK: the all-panes window

    /// The browser opened at a fixed 620pt whatever was running. These numbers were read
    /// off the machine with GTMUXBAR_DEBUG=1: title bar 32pt on a 1007pt usable height.
    func testTheBrowserOpensAtTheHeightItsSessionsNeed() {
        // 81 sessions / 240 panes want 10725pt — capped at what the display can hold,
        // deliberately short of filling it (1007 − 32 title bar − 120 margin).
        XCTAssertEqual(PanelMetrics.windowContentHeight(desired: 10725, visible: 1007, titleBar: 32), 855)
        // 9 sessions / 22 panes (the real machine) want more than the display too.
        XCTAssertEqual(PanelMetrics.windowContentHeight(desired: 1115, visible: 1007, titleBar: 32), 855)
        // A handful of panes gets a window that is not a sliver.
        XCTAssertEqual(PanelMetrics.windowContentHeight(desired: 368, visible: 1007, titleBar: 32),
                       PanelMetrics.windowFloor)
    }

    /// A window in between is passed through — the point of the change is that the size
    /// tracks the content instead of being one constant for every machine.
    func testTheBrowserTracksItsContentInBetween() {
        for wanted in stride(from: CGFloat(420), through: 840, by: 60) {
            XCTAssertEqual(PanelMetrics.windowContentHeight(desired: wanted, visible: 1007, titleBar: 32),
                           wanted, "content of \(wanted) should be honoured")
        }
    }

    /// The window opens at the design width, never at the bare minimum it can be dragged
    /// to. 480 vs 420 is the difference a hosting controller silently made.
    func testTheBrowserOpensAtItsDesignWidth() {
        XCTAssertEqual(PanelMetrics.browserWidth, 480)
    }

    /// It also leaves real room around itself: filling the display would read as taking
    /// the screen over, and the list scrolls anyway.
    func testTheBrowserLeavesTheDisplaySomeRoom() {
        let h = PanelMetrics.windowContentHeight(desired: 9999, visible: 1007, titleBar: 32)
        XCTAssertLessThanOrEqual(h + 32, 1007 - 100, "a window, not a takeover")
    }

    /// On a display too small to hold even the floor, the floor wins: the window is
    /// resizable and a usable-but-too-tall window beats an unusable sliver.
    func testTheBrowserFloorWinsOnATinyDisplay() {
        XCTAssertEqual(PanelMetrics.windowContentHeight(desired: 900, visible: 380, titleBar: 32),
                       PanelMetrics.windowFloor)
    }

    // MARK: the off-screen repair

    // The measured display: 1728×1117, Dock 77pt, menu bar 33pt.
    private let screenFrame = CGRect(x: 0, y: 0, width: 1728, height: 1117)
    private let visibleFrame = CGRect(x: 0, y: 77, width: 1728, height: 1007)

    /// A correctly placed panel touches the top of the DISPLAY — its arrow tucks under the
    /// menu bar. Calling that misplaced would re-show forever, because the repair is
    /// itself a layout. These are the real frames, read off the machine.
    func testAPanelUnderTheMenuBarIsNotOffScreen() {
        for h in [326.0, 758.0, 1009.0] { // empty state, twelve agents, three hundred
            let win = CGRect(x: 640, y: 1089 - h, width: 446, height: h)
            XCTAssertFalse(PanelMetrics.hasLeftTheScreen(window: win,
                                                         screenFrame: screenFrame,
                                                         visibleFrame: visibleFrame),
                           "panel of \(h)pt sits under the menu bar")
        }
    }

    /// What 300 agents actually did on the first opening, before the repair existed.
    func testTheMeasuredOffScreenPanelIsCaught() {
        let win = CGRect(x: 640, y: -3504, width: 446, height: 1006)
        XCTAssertTrue(PanelMetrics.hasLeftTheScreen(window: win,
                                                    screenFrame: screenFrame,
                                                    visibleFrame: visibleFrame))
    }

    /// And the original report: pushed up past the top of the display.
    func testAPanelAboveTheDisplayIsCaught() {
        let win = CGRect(x: 640, y: 743, width: 446, height: 758) // top = 1501
        XCTAssertTrue(PanelMetrics.hasLeftTheScreen(window: win,
                                                    screenFrame: screenFrame,
                                                    visibleFrame: visibleFrame))
    }

    /// The height has to REACH AppKit. The bug was not a wrong number; it was a number
    /// NSPopover was never given — it went on positioning the window for the size it was
    /// first handed and let the surplus run off the top of the screen.
    func testPanelSizePublishesItsHeight() {
        let panel = PanelSize()
        var seen: [CGFloat] = []
        let sub = panel.$height.sink { seen.append($0) }
        panel.height = 732
        sub.cancel()
        XCTAssertEqual(seen.last, 732, "the host has to be able to observe the height")
    }
}
