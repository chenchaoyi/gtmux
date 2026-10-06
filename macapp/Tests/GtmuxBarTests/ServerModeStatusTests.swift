import XCTest
@testable import GtmuxBar

// `gtmux awake --json` reports `unknown` when the Mac cannot read its own sleep setting.
// Its system_disablesleep is then a placeholder false: the menu bar must not read that as
// "off", and while something of gtmux's is in place it may still be keeping the Mac awake.
final class ServerModeStatusTests: XCTestCase {
    private func status(_ state: String, owned: Bool, guardInstalled: Bool, disable: Bool = false) throws -> ServerModeStatus {
        let json = """
        {"state":"\(state)","power":"ac","guard":{"installed":\(guardInstalled),"healthy":\(guardInstalled)},
         "system_disablesleep":\(disable),"persisted_disablesleep":false,"owned_by_gtmux":\(owned),
         "platform":{"ok":false,"verified":false,"reason":"no-readback"}}
        """
        return try JSONDecoder().decode(ServerModeStatus.self, from: Data(json.utf8))
    }

    func testUnreadableWhileGtmuxMayHaveItOnAsksForALook() throws {
        let st = try status("unknown", owned: true, guardInstalled: true)
        XCTAssertFalse(st.isOn, "the placeholder false is not a reading")
        XCTAssertTrue(st.isUnknown)
        XCTAssertTrue(st.mayBeOn)
        XCTAssertTrue(st.needsAttention)
        XCTAssertNotNil(st.attentionReason)
    }

    func testUnreadableWithNothingOfGtmuxsStaysQuiet() throws {
        let st = try status("unknown", owned: false, guardInstalled: false)
        XCTAssertFalse(st.mayBeOn)
        XCTAssertFalse(st.needsAttention)
    }

    func testAReadingIsUnchanged() throws {
        XCTAssertTrue(try status("on", owned: true, guardInstalled: true, disable: true).mayBeOn)
        let off = try status("off", owned: false, guardInstalled: false)
        XCTAssertFalse(off.mayBeOn)
        XCTAssertFalse(off.needsAttention)
    }
}
