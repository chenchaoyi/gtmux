import XCTest
@testable import GtmuxBar

final class LaunchHostPresentationTests: XCTestCase {
    func testHostLabelPreservesAgentAndUnconfirmedInput() throws {
        let agent = try JSONDecoder().decode(Agent.self, from: Data(#"{"agent":"Codex","source":"native","current_client":"unconfirmed","can_read_conversation":true,"launch_host":{"name":"sample-client","kind":"process","observed_at":1700000000}}"#.utf8))
        let l10n = L10n.shared; let previous = l10n.mode; defer { l10n.mode = previous }
        for mode in [LangMode.en, .zh] {
            l10n.mode = mode
            XCTAssertEqual(nativeOriginLabel(agent, l10n: l10n), "Codex · sample-client")
            XCTAssertTrue(nativeEvidenceLabel(agent, l10n: l10n).contains(mode == .en ? "observed" : "记录于"))
            XCTAssertTrue(nativeEvidenceLabel(agent, l10n: l10n).contains(mode == .en ? "Current input client unconfirmed" : "当前输入客户端未确认"))
        }
        XCTAssertTrue(agent.readableConversation); XCTAssertFalse(agent.adoptable)
        var missing = agent; missing.launchHost = nil; l10n.mode = .en
        XCTAssertEqual(nativeOriginLabel(missing, l10n: l10n), "Codex · External session")
        missing.launchHost = LaunchHost(name: "sample-client", kind: "invalid", observedAt: 1)
        XCTAssertEqual(nativeOriginLabel(missing, l10n: l10n), "Codex · External session")
    }
}
