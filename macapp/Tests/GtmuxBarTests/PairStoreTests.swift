import XCTest
@testable import GtmuxBar

final class PairStoreTests: XCTestCase {
    // parseDevices keeps OWNER entries only (guests live in ShareStore) and maps
    // the /api/devices wire shape.
    func testParseDevicesFiltersGuests() throws {
        let json = """
        {"devices":[
          {"id":"d1","name":"ccy iPhone","enrolledAt":100,"lastSeen":200},
          {"id":"g1","name":"Alice","enrolledAt":150,"scope":"guest","viewPanes":["%1"]},
          {"id":"d2","name":"work-laptop","enrolledAt":300}
        ]}
        """
        let devices = try XCTUnwrap(PairStore.parseDevices(Data(json.utf8)))
        XCTAssertEqual(devices.map(\.id), ["d1", "d2"])
        XCTAssertEqual(devices[0].lastSeen, 200)
    }

    // kind guesses the row icon from the device name — chrome only, best-effort.
    func testDeviceKind() {
        XCTAssertEqual(PairedDevice(id: "1", name: "ccy iPhone 15", enrolledAt: 0, lastSeen: 0).kind, "iphone")
        XCTAssertEqual(PairedDevice(id: "2", name: "Safari · macOS", enrolledAt: 0, lastSeen: 0).kind, "globe")
        XCTAssertEqual(PairedDevice(id: "3", name: "work-mbp", enrolledAt: 0, lastSeen: 0).kind, "laptopcomputer")
    }
}

// The roster's job is letting you tell YOUR devices apart well enough to revoke the right
// one. Every entry used to read `gtmux • iPhone`: a "gtmux" prefix inside gtmux's own
// roster (nothing in that list is not a gtmux device) over a word true of every iPhone.
final class PairedDeviceNameTests: XCTestCase {
    private func dev(_ name: String) -> PairedDevice {
        PairedDevice(id: "d1", name: name, enrolledAt: 0, lastSeen: 0)
    }

    func testLegacyPrefixIsStrippedForDisplay() {
        // Entries paired before the rename tidy themselves up — no re-pairing asked.
        XCTAssertEqual(dev("gtmux • iPhone").displayName, "iPhone")
        XCTAssertEqual(dev("gtmux · iPad").displayName, "iPad")
        XCTAssertEqual(dev("gtmux iPhone").displayName, "iPhone")
        XCTAssertEqual(dev("GTMUX • iPhone · iOS 18.5").displayName, "iPhone · iOS 18.5")
    }

    func testANameWithoutThePrefixIsUntouched() {
        XCTAssertEqual(dev("iPhone · iOS 18.5").displayName, "iPhone · iOS 18.5")
        XCTAssertEqual(dev("ccy-mbp.local").displayName, "ccy-mbp.local")
        // A device legitimately NAMED after the tool keeps something to show, rather
        // than being stripped to an empty row.
        XCTAssertEqual(dev("gtmux").displayName, "gtmux")
    }

    func testTheIconStillResolvesFromTheNewNaming() {
        XCTAssertEqual(dev("iPhone · iOS 18.5").kind, "iphone")
        XCTAssertEqual(dev("iPad · iOS 18.5").kind, "iphone")
        XCTAssertEqual(dev("Safari").kind, "globe")
        XCTAssertEqual(dev("ccy-mbp.local").kind, "laptopcomputer")
    }
}
