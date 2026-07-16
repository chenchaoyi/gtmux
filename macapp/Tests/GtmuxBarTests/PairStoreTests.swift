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
