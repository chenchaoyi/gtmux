import XCTest
import SwiftUI
import AppKit
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
        XCTAssertEqual(view.summaryText, "Desktop conversations · HQ off")
        store.setAgentsForTesting(rows)
        XCTAssertEqual(view.contentState, .list, "native-only sessions must not be mistaken for no search matches")
        XCTAssertEqual(view.summaryText, "1 agent · 1 awaiting input · 0 working · 0 idle")
    }

    @MainActor func testInitialReadFailureCannotSaveDefaults() async throws {
        let model = SessionFollowModel(agent: Agent(source: "native", client: "chatgpt_desktop", sessionID: "desk"),
            l10n: .shared, onSaved: { XCTFail("A failed read must not save") },
            capture: { _ in (1, "", "offline") })
        model.load()
        for _ in 0..<100 {
            if !model.loading { break }
            try await Task.sleep(nanoseconds: 10_000_000)
        }
        XCTAssertFalse(model.loading)
        XCTAssertFalse(model.loaded)
        XCTAssertFalse(model.error.isEmpty)
        model.choose(true)
        model.save()
        XCTAssertFalse(model.saving)
    }

    @MainActor func testDraftStopAndReenableDoesNotRestoreOptionalPermissions() {
        let model = SessionFollowModel(agent: Agent(), l10n: .shared, onSaved: {})
        model.loaded = true; model.loading = false
        model.current = SessionFollowSettings(hq: true, notify: true, knowledge: true, revision: 9)
        model.draft = model.current
        model.choose(false)
        XCTAssertFalse(model.draft.notify)
        XCTAssertFalse(model.draft.knowledge)
        model.choose(true)
        XCTAssertFalse(model.draft.notify)
        XCTAssertFalse(model.draft.knowledge)
        XCTAssertEqual(model.draft.revision, 9)
        XCTAssertTrue(model.dirty)
        XCTAssertTrue(model.current.notify, "Draft controls must not change the saved policy")
    }

    @MainActor func testCompactFormRendersBothLanguagesAndThemes() throws {
        let oldMode = L10n.shared.mode
        defer { L10n.shared.mode = oldMode }
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            for dark in [false, true] {
                for scenario in ["off", "on", "stop", "error"] {
                    let model = SessionFollowModel(agent: Agent(agent: "Codex", task: "Improve desktop conversation settings", source: "native", client: "chatgpt_desktop"), l10n: .shared, onSaved: {})
                    model.loading = false
                    model.loaded = scenario != "error"
                    model.current.hq = scenario == "on" || scenario == "stop"
                    model.draft = model.current
                    if scenario == "stop" { model.choose(false) }
                    if scenario == "error" { model.error = L10n.shared.tr("Could not load settings. Reload to try again.", "无法读取设置，请重新加载。") }
                    let view = SessionFollowView(model: model, l10n: .shared)
                    let host = NSHostingView(rootView: view.background(Color(nsColor: .windowBackgroundColor)).environment(\.colorScheme, dark ? .dark : .light))
                    let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 420, height: view.contentHeight), styleMask: [.titled], backing: .buffered, defer: false)
                    window.isReleasedWhenClosed = false
                    window.appearance = NSAppearance(named: dark ? .darkAqua : .aqua)
                    window.contentView = host
                    defer { window.close() }
                    host.layoutSubtreeIfNeeded()
                    XCTAssertEqual(host.fittingSize.width, 420, accuracy: 1)
                    XCTAssertEqual(host.fittingSize.height, scenario == "on" ? 380 : 230, accuracy: 1)
                    if let dir = ProcessInfo.processInfo.environment["GTMUX_FOLLOW_SNAPSHOT_DIR"] {
                        RunLoop.main.run(until: Date().addingTimeInterval(0.03))
                        host.layoutSubtreeIfNeeded()
                        let rep = try XCTUnwrap(host.bitmapImageRepForCachingDisplay(in: host.bounds))
                        host.cacheDisplay(in: host.bounds, to: rep)
                        let png = try XCTUnwrap(rep.representation(using: .png, properties: [:]))
                        try png.write(to: URL(fileURLWithPath: dir).appendingPathComponent("follow-\(mode == .zh ? "zh" : "en")-\(dark ? "dark" : "light")-\(scenario).png"))
                    }
                }
            }
        }
    }

}
