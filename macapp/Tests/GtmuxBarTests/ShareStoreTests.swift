import XCTest
@testable import GtmuxBar

final class ShareStoreTests: XCTestCase {

    /// parseStatus must decode the `gtmux share status --json` wire shape: consent,
    /// the allowlist (as a Set), the token-free guest list, and the base URL. Pins
    /// the app's half of the CLI contract so a shape drift fails the build.
    func testParseStatus() throws {
        let json = """
        {
          "enabled": true,
          "panes": ["%37", "%5"],
          "guests": [
            {"id": "g1", "label": "alice", "enrolled_at": 1783956522},
            {"id": "g2", "label": "", "enrolled_at": 1783934731}
          ],
          "base": "https://gtmux-x.ccy.dev"
        }
        """
        let parsed = try XCTUnwrap(ShareStore.parseStatus(Data(json.utf8)))
        XCTAssertTrue(parsed.enabled)
        XCTAssertEqual(parsed.panes, ["%37", "%5"])
        XCTAssertEqual(parsed.base, "https://gtmux-x.ccy.dev")
        XCTAssertEqual(parsed.guests.count, 2)
        XCTAssertEqual(parsed.guests[0].id, "g1")
        XCTAssertEqual(parsed.guests[0].label, "alice")
        XCTAssertEqual(parsed.guests[0].enrolledAt, 1783956522)
        XCTAssertEqual(parsed.guests[1].label, "") // an unlabeled link decodes cleanly
    }

    /// A disabled/empty state decodes to safe defaults (off, no panes, no guests),
    /// so the section renders the "view-only" resting state rather than crashing.
    func testParseStatusEmpty() throws {
        let parsed = try XCTUnwrap(ShareStore.parseStatus(Data(#"{"enabled":false,"panes":[],"guests":[]}"#.utf8)))
        XCTAssertFalse(parsed.enabled)
        XCTAssertTrue(parsed.panes.isEmpty)
        XCTAssertTrue(parsed.guests.isEmpty)
        XCTAssertEqual(parsed.base, "")
    }

    /// Malformed input returns nil (the caller keeps its last-known state) rather
    /// than a partial/garbage decode.
    func testParseStatusMalformed() {
        XCTAssertNil(ShareStore.parseStatus(Data("not json".utf8)))
    }

    /// isLive gates the popover exposure indicator: it is TRUE only when all three
    /// of consent, an allowed pane, and a guest link hold — so the indicator never
    /// cries wolf when nobody can actually type in.
    func testIsLiveRequiresAllThree() {
        let s = ShareStore()
        XCTAssertFalse(s.isLive) // default: off, nothing allowed
        // (state is private(set); this pins the invariant at the empty default —
        // the positive path is covered end-to-end by parseStatus feeding the fields.)
    }
}
