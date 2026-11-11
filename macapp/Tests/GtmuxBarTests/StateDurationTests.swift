import Foundation
import XCTest
@testable import GtmuxBar

final class StateDurationTests: XCTestCase {
    func testCSR004DecodedUncertainDurationDoesNotUseEitherClock() throws {
        let now = Int(Date().timeIntervalSince1970)
        func decode(_ fields: String) throws -> Agent {
            try JSONDecoder().decode(Agent.self, from: Data("""
            {"pane_id":"%17","agent":"Codex","status":"working","activity_at":\(now),\(fields)}
            """.utf8))
        }
        XCTAssertEqual(try decode("\"since_uncertain\":true,\"since\":\(now - 17 * 3600)").relativeTimeLabel, "")
        XCTAssertEqual(try decode("\"since_uncertain\":true,\"since\":0").relativeTimeLabel, "")
        XCTAssertEqual(try decode("\"since\":\(now - 60)").relativeTimeLabel, "1m")
        XCTAssertFalse(try decode("\"since\":0").relativeTimeLabel.isEmpty)
    }
}
