import XCTest
@testable import GtmuxBar

final class ObservedIdentityTests: XCTestCase {
    func testSID008HistoryNeverPopulatesCurrentIdentity() throws {
        let data = Data(#"{"pane_id":"%fixture","session":"synthetic","agent":"Codex","observed_conversation":{"session_id":"old-thread","home_id":"home","state":"last-known"}}"#.utf8)
        let agent = try JSONDecoder().decode(Agent.self, from: data)
        XCTAssertEqual(agent.sessionID, "")
        XCTAssertFalse(agent.isNative)
        XCTAssertEqual(agent.observedConversation?.sessionID, "old-thread")
        XCTAssertTrue(agent.secondary.contains("Association pending") || agent.secondary.contains("关联待确认"))
        XCTAssertEqual(agent.jumpArgs(), ["focus", "%fixture"])
        let normal = try JSONDecoder().decode(Agent.self, from: Data(#"{"pane_id":"%fixture","session":"synthetic","agent":"Codex"}"#.utf8))
        XCTAssertFalse(normal.secondary.contains("Association pending") || normal.secondary.contains("关联待确认"))
    }
}
