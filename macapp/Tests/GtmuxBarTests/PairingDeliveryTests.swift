import XCTest
@testable import GtmuxBar

final class PairingDeliveryTests: XCTestCase {
    func testAllPairingMediaCarryTheSameCodeAndNeverTheMasterToken() throws {
        let info = PairingInfo(url: "https://tunnel.example/p12345/", token: "master-secret", name: "Example Mac", anywhere: true)
        let values = PairingDeliveryValues(info: info, code: "abcd1234")
        XCTAssertEqual(values.browser, "https://tunnel.example/p12345/#c=abcd1234")
        XCTAssertEqual(values.terminal, "gtmux attach '\(values.browser)'")
        let qr = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(values.qr.utf8)) as? [String: Any])
        XCTAssertEqual(qr["enrollCode"] as? String, "abcd1234")
        XCTAssertNil(qr["token"])
        for medium in [values.qr, values.browser, values.terminal] {
            XCTAssertFalse(medium.contains("master-secret"))
        }
    }
}
