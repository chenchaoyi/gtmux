import XCTest
@testable import GtmuxBar

final class SessionFollowTests: XCTestCase {
    func testDesktopDetectionDoesNotGrantFollowAndOtherClientsRetainCounts() throws {
        let rows = try JSONDecoder().decode([Agent].self, from: Data(#"[{"source":"native","client":"chatgpt_desktop","agent":"Codex","session_id":"desk","status":"working","follow":{"hq":false,"notify":false,"knowledge":false,"revision":0}},{"source":"native","client":"terminal","agent":"Codex","session_id":"term","status":"waiting"}]"#.utf8))
        let store = AgentStore()
        store.setAgentsForTesting(rows)
        XCTAssertEqual(store.total, 1)
        XCTAssertEqual(store.waiting, 1)
        XCTAssertEqual(store.working, 0)
        XCTAssertEqual(store.nativeSessions(query: "", desktop: true).map(\.sessionID), ["desk"])
        XCTAssertEqual(store.nativeSessions(query: "").map(\.sessionID), ["term"])
        XCTAssertTrue(rows[0].statusOnlyDesktop)
        var followed = rows[0]
        followed.follow?.hq = true
        store.setAgentsForTesting([followed, rows[1]])
        XCTAssertEqual(store.total, 2)
        XCTAssertEqual(store.working, 1)
        XCTAssertFalse(followed.follow!.notify)
        XCTAssertFalse(followed.follow!.knowledge)
    }

    func testSettingsTargetTheVerifiedIDAndPreserveTheReadRevision() {
        var s = SessionFollowSettings()
        s.hq = true; s.revision = 7
        XCTAssertEqual(SessionFollowSettings.arguments(id: "conversation-a", settings: s),
                       ["follow", "conversation-a", "--hq", "on", "--notify", "off", "--knowledge", "off", "--revision", "7", "--json"])
        let a = Agent(source: "native", client: "chatgpt_desktop")
        XCTAssertTrue(a.statusOnlyDesktop, "an old core with no permissions must fail closed")
    }

    func testDesktopOnlyListRemainsReachableAndDoesNotInflateSummary() throws {
        let original = L10n.shared.mode
        defer { L10n.shared.mode = original }
        L10n.shared.mode = .en
        let rows = try JSONDecoder().decode([Agent].self, from: Data(#"[{"source":"native","client":"chatgpt_desktop","agent":"Codex","session_id":"desk","status":"working","follow":{"hq":false,"notify":false,"knowledge":false,"revision":0}},{"source":"native","client":"terminal","agent":"Codex","session_id":"term","status":"waiting"}]"#.utf8))
        let store = AgentStore()
        let view = MenuView(store: store, l10n: .shared, onJump: { _ in }, onAction: { _ in })
        XCTAssertEqual(view.contentState, .empty)
        store.setAgentsForTesting([rows[0]])
        XCTAssertEqual(store.total, 0)
        XCTAssertEqual(view.contentState, .list, "a desktop-only launch must offer follow settings")
        XCTAssertEqual(view.summaryText, "Desktop conversations · status only")
        store.setAgentsForTesting(rows)
        XCTAssertEqual(view.contentState, .list, "native-only sessions must not be mistaken for no search matches")
        XCTAssertEqual(view.summaryText, "1 agent · 1 awaiting input · 0 working · 0 idle")
    }

}
