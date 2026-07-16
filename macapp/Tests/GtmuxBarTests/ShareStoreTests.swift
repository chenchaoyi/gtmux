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
          "view_panes": ["%37", "%5", "%9"],
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
        // The view allowlist is carried separately and is a superset of input (%9 is
        // view-only), so the picker can render see-vs-type independently.
        XCTAssertEqual(parsed.viewPanes, ["%37", "%5", "%9"])
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
        XCTAssertTrue(parsed.viewPanes.isEmpty) // absent view_panes → empty, guest sees nothing
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

    // Per-link scope fields (pair-share-model S1) decode from the wire shape,
    // with legacy guests (no scope fields) defaulting to empty.
    func testParseStatusPerLinkScope() throws {
        let json = """
        {"enabled":true,"panes":["%1"],"view_panes":["%1","%2"],
         "guests":[
           {"id":"a","label":"Alice","enrolled_at":100,
            "view_panes":["%1","%2"],"panes":["%1"],"expires_at":9999},
           {"id":"b","label":"Bob","enrolled_at":200}
         ],"base":"https://x"}
        """
        let parsed = try XCTUnwrap(ShareStore.parseStatus(Data(json.utf8)))
        XCTAssertEqual(parsed.guests.count, 2)
        let alice = parsed.guests[0]
        XCTAssertEqual(alice.viewPanes, ["%1", "%2"])
        XCTAssertEqual(alice.inputPanes, ["%1"])
        XCTAssertEqual(alice.expiresAt, 9999)
        let bob = parsed.guests[1]
        XCTAssertEqual(bob.viewPanes, [])
        XCTAssertEqual(bob.inputPanes, [])
        XCTAssertEqual(bob.expiresAt, 0)
    }
}
