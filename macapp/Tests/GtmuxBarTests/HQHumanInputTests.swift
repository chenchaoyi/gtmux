import Foundation
import XCTest
@testable import GtmuxBar

@MainActor
final class HQHumanInputTests: XCTestCase {
    func testQuestionAndSavedReplyDecodeWithoutClaimingDeliveryOrExecution() throws {
        let json = #"{"id":"goal","title":"Synthetic","state":"needs-input","created_at":1,"updated_at":2,"revision":2,"participants":[],"user_input":{"id":"question-id","kind":"decision","prompt":"Keep sample?","why":"Two choices remain","impact":"1 MiB","choices":["Keep","Remove"],"at":2,"answer_id":"answer-id"}}"#
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        let matter = try decoder.decode(HQMatter.self, from: Data(json.utf8))
        XCTAssertEqual(matter.userInput?.prompt, "Keep sample?")
        XCTAssertEqual(matter.userInput?.answerId, "answer-id")
        XCTAssertEqual(matter.state, "needs-input")
    }
}
