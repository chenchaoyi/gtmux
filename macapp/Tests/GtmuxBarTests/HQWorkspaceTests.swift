import AppKit
import SwiftUI
import XCTest

@testable import GtmuxBar

private func decodeWorkspace<T: Decodable>(_ json: String) -> T {
    let d = JSONDecoder()
    d.keyDecodingStrategy = .convertFromSnakeCase
    return try! d.decode(T.self, from: Data(json.utf8))
}
private let workspaceFixture =
    #"{"schema":1,"revision":1,"matters":[{"id":"matter-a","title":"Storage investigation","state":"open","created_at":100,"updated_at":100,"revision":1,"participants":[]},{"id":"matter-b","title":"Release preparation","state":"needs-input","created_at":100,"updated_at":100,"revision":1,"participants":[]}],"conversations":[]}"#
private let detailFixture =
    #"{"schema":1,"messages":[{"id":"request-a","speaker":"user","kind":"instruction","body":"**Check storage**","at":100,"seq":2,"delivery":"pending"}],"turns":[],"earlier_available":false,"message_earlier_available":false,"transcript_earlier_available":false,"changes":[],"events":[],"warnings":[]}"#

private final class WorkspaceFake: HQWorkspaceServing, @unchecked Sendable {
    private let lock = NSLock()
    private var _actions: [HQWorkspaceAction] = []
    private var failures = 0
    private var reads = 0
    var detailReads: Int {
        lock.lock()
        defer { lock.unlock() }
        return reads
    }
    var snapshotValue: HQWorkspaceSnapshot = decodeWorkspace(workspaceFixture)
    var delay: TimeInterval = 0
    var page: ((String, UInt64?, Int) -> HQWorkspaceDetail)?
    var actions: [HQWorkspaceAction] {
        lock.lock()
        defer { lock.unlock() }
        return _actions
    }
    func failOnce() {
        lock.lock()
        failures = 1
        lock.unlock()
    }
    func snapshot() throws -> HQWorkspaceSnapshot {
        if delay > 0 { Thread.sleep(forTimeInterval: delay) }
        return snapshotValue
    }
    func detail(_ scope: String, before: UInt64?, earlier: Int) throws -> HQWorkspaceDetail {
        lock.lock()
        reads += 1
        lock.unlock()
        if let page { return page(scope, before, earlier) }
        var d: HQWorkspaceDetail = decodeWorkspace(detailFixture)
        d.messages[0].body = scope
        if let matter = snapshotValue.matters.first(where: { $0.id == scope }) { d.matter = matter }
        return d
    }
    func participant(_ matter: String, association: String) throws -> HQWorkspaceDetail {
        try detail(matter, before: nil, earlier: 0)
    }
    func act(_ action: HQWorkspaceAction) throws {
        lock.lock()
        defer { lock.unlock() }
        _actions.append(action)
        if failures > 0 {
            failures -= 1
            throw HQWorkspaceError.command("lost receipt")
        }
    }
    func drain() throws {}
}

@MainActor final class HQWorkspaceTests: XCTestCase {
    private func settled(_ store: HQWorkspaceStore) async throws {
        for _ in 0..<200 {
            if !store.loading && !store.acting { return }
            try await Task.sleep(nanoseconds: 5_000_000)
        }
        XCTFail("workspace did not settle")
    }
    func testCLIUsesLiteralStdinAndImmutableIdentity() throws {
        var args: [String] = []
        var body: Data?
        let client = HQWorkspaceClient(execute: { a, b in
            args = a
            body = b
            return (0, "{\"ok\":true}", "")
        })
        let text = "**Markdown**\n`$(touch /tmp/must-not-run)`"
        try client.act(HQWorkspaceAction(op: "send", id: "request-a", matter: "matter-a", body: text))
        XCTAssertEqual(args, ["hq", "workspace", "act", "--stdin", "--json"])
        let wire = try JSONSerialization.jsonObject(with: XCTUnwrap(body)) as! [String: Any]
        XCTAssertEqual(wire["body"] as? String, text)
        XCTAssertEqual(wire["matter"] as? String, "matter-a")
        let a = HQConversation(agent: "codex", homeId: "home-a", sessionId: "sid", name: "same", pane: "%1")
        var b = a
        b.name = "renamed"
        b.pane = "%9"
        XCTAssertEqual(a.id, b.id)
        b.homeId = "home-b"
        XCTAssertNotEqual(a.id, b.id)
    }
    func testOldCoreAndUnknownSchemaAreExplicitErrors() {
        let old = HQWorkspaceClient(execute: { _, _ in (2, "", "unknown HQ workspace command") })
        XCTAssertThrowsError(try old.snapshot())
        let future = HQWorkspaceClient(execute: { _, _ in
            (0, workspaceFixture.replacingOccurrences(of: "\"schema\":1", with: "\"schema\":99"), "")
        })
        XCTAssertThrowsError(try future.snapshot())
    }
    func testLiteralInputOutputBudgetAndTimeout() {
        let cat = Process()
        cat.executableURL = URL(fileURLWithPath: "/bin/cat")
        let text = "$(touch /tmp/never)\n**body**"
        let result = GtmuxCLI.runBounded(cat, timeout: 2, stdin: Data(text.utf8))
        XCTAssertEqual(result.status, 0)
        XCTAssertEqual(result.stdout, text)
        let bounded = Process()
        bounded.executableURL = URL(fileURLWithPath: "/bin/cat")
        XCTAssertEqual(
            GtmuxCLI.runBounded(bounded, timeout: 2, stdin: Data(repeating: 65, count: 101), outputLimit: 100)
                .status, -3)
        let sleep = Process()
        sleep.executableURL = URL(fileURLWithPath: "/bin/sleep")
        sleep.arguments = ["3"]
        XCTAssertEqual(GtmuxCLI.runBounded(sleep, timeout: 0.03).status, -2)
    }
    func testDraftsAreRetainedAcrossScopesAndClosing() async throws {
        let store = HQWorkspaceStore(client: WorkspaceFake())
        store.start()
        try await settled(store)
        store.draft = "global draft"
        store.select(.matter("matter-a"))
        try await settled(store)
        store.draft = "matter draft"
        store.select(.conversation)
        try await settled(store)
        XCTAssertEqual(store.draft, "global draft")
        store.select(.matter("matter-a"))
        try await settled(store)
        XCTAssertEqual(store.draft, "matter draft")
        store.stop()
        store.start()
        try await settled(store)
        XCTAssertEqual(store.draft, "matter draft")
        store.stop()
    }
    func testDelayedReadDoesNotReplaceNewSelection() async throws {
        let fake = WorkspaceFake()
        fake.delay = 0.04
        let store = HQWorkspaceStore(client: fake)
        store.start()
        store.select(.matter("matter-b"))
        try await settled(store)
        XCTAssertEqual(store.selection, .matter("matter-b"))
        XCTAssertEqual(store.detail?.messages.first?.body, "matter-b")
        store.stop()
    }
    func testFailedSubmissionRetainsDraftAndRetryIDAndScope() async throws {
        let fake = WorkspaceFake()
        fake.failOnce()
        let store = HQWorkspaceStore(client: fake)
        store.start()
        try await settled(store)
        store.select(.matter("matter-a"))
        try await settled(store)
        store.draft = "investigate **space**"
        store.send()
        try await settled(store)
        XCTAssertEqual(store.draft, "investigate **space**")
        XCTAssertNotNil(store.error)
        store.send()
        try await settled(store)
        XCTAssertEqual(fake.actions.count, 2)
        XCTAssertEqual(fake.actions[0].id, fake.actions[1].id)
        XCTAssertEqual(fake.actions[1].matter, "matter-a")
        XCTAssertEqual(store.draft, "")
        store.stop()
    }
    func testMissingMatterCannotAccidentallySendGlobal() async throws {
        let fake = WorkspaceFake()
        let store = HQWorkspaceStore(client: fake)
        store.start()
        try await settled(store)
        store.select(.matter("missing"))
        try await settled(store)
        store.draft = "must stay scoped"
        store.send()
        XCTAssertFalse(store.canSend)
        XCTAssertEqual(fake.actions.count, 0)
        store.stop()
    }
    func testNewTaskCommitsGoalAndInstructionTogether() async throws {
        let fake = WorkspaceFake()
        let store = HQWorkspaceStore(client: fake)
        store.start()
        try await settled(store)
        store.select(.newTask)
        try await settled(store)
        store.draft = "Storage investigation\nOnly inspect safe caches"
        store.send()
        try await settled(store)
        let a = try XCTUnwrap(fake.actions.first)
        XCTAssertEqual(a.op, "create")
        XCTAssertEqual(a.title, "Storage investigation")
        XCTAssertEqual(a.body, "Storage investigation\nOnly inspect safe caches")
        XCTAssertEqual(store.selection, .matter(try XCTUnwrap(a.id)))
        store.stop()
    }
    func testReceiptIsNotCompletionAndSequenceOrdersSameSecond() {
        var detail: HQWorkspaceDetail = decodeWorkspace(detailFixture)
        var m = detail.messages[0]
        m.id = "z-first"
        m.seq = 1
        m.delivery = "delivered"
        var n = m
        n.id = "a-second"
        n.seq = 2
        n.delivery = "uncertain"
        detail.messages = [n, m]
        XCTAssertEqual(HQConversationRow.rows(detail).map(\.id), ["z-first", "a-second"])
        XCTAssertEqual(hqDeliveryLabel(m, L10n.shared), L10n.shared.tr("Delivered to HQ", "已送达 HQ"))
        XCTAssertNotEqual(hqDeliveryLabel(n, L10n.shared), "Completed")
    }
    func testEarlierMessagesRemainLoadedAfterPollAndDoNotReopenFinishedPagination() async throws {
        let fake = WorkspaceFake()
        fake.page = { scope, before, _ in
            var d: HQWorkspaceDetail = decodeWorkspace(detailFixture)
            d.matter = fake.snapshotValue.matters.first { $0.id == scope }
            var m = d.messages[0]
            m.id = before == nil ? "new" : "old"
            m.seq = before == nil ? 100 : 1
            d.messages = [m]
            d.earlierBefore = before == nil ? 100 : nil
            d.messageEarlierAvailable = before == nil
            d.earlierAvailable = before == nil
            return d
        }
        let store = HQWorkspaceStore(client: fake)
        store.start()
        try await settled(store)
        store.select(.matter("matter-a"))
        try await settled(store)
        store.loadEarlier()
        try await settled(store)
        XCTAssertEqual(store.detail?.messages.map(\.id), ["old", "new"])
        XCTAssertFalse(store.detail!.earlierAvailable)
        store.refresh()
        try await settled(store)
        XCTAssertEqual(store.detail?.messages.map(\.id), ["old", "new"])
        XCTAssertFalse(store.detail!.earlierAvailable)
        store.stop()
    }
    func testGlobalHistoryPagesMessagesAndSessionSourcesTogether() async throws {
        let fake = WorkspaceFake()
        fake.page = { _, before, hops in
            var d: HQWorkspaceDetail = decodeWorkspace(detailFixture)
            var m = d.messages[0]
            m.id = before == nil ? "new" : "old"
            m.seq = before == nil ? 100 : 1
            d.messages = [m]
            d.earlierBefore = before == nil ? 100 : nil
            d.messageEarlierAvailable = before == nil
            d.transcriptEarlierAvailable = hops == 0
            d.earlierAvailable = before == nil || hops == 0
            d.turns = decodeWorkspace(
                hops == 0
                    ? #"[{"prompt":"current","response":"now"}]"#
                    : #"[{"prompt":"earlier","response":"before"},{"prompt":"current","response":"now"}]"#)
            return d
        }
        let store = HQWorkspaceStore(client: fake)
        store.start()
        try await settled(store)
        store.loadEarlier()
        try await settled(store)
        XCTAssertEqual(store.detail?.messages.map(\.id), ["old", "new"])
        XCTAssertEqual(store.detail?.turns.first?.prompt, "earlier")
        XCTAssertFalse(store.detail!.earlierAvailable)
        store.stop()
    }
    func testWorkspaceControllerKeepsAUsableInitialWindowAndOneStore() async throws {
        let store = HQWorkspaceStore(client: WorkspaceFake())
        let controller = HQWorkspaceController(store: store)
        controller.show(l10n: .shared)
        try await settled(store)
        let first = try XCTUnwrap(controller.window)
        XCTAssertGreaterThanOrEqual(first.contentView!.bounds.width, 700)
        XCTAssertGreaterThanOrEqual(first.contentView!.bounds.height, 440)
        store.draft = "retained"
        first.close()
        controller.show(l10n: .shared)
        try await settled(store)
        XCTAssertTrue(first === controller.window)
        XCTAssertEqual(store.draft, "retained")
        first.close()
    }
    func testQuietPollSkipsUnchangedSourcesButReadsChangedPublicTranscript() async throws {
        let fake = WorkspaceFake()
        fake.snapshotValue.sourceRevision = "source-one"
        let store = HQWorkspaceStore(client: fake)
        store.start()
        try await settled(store)
        let initial = fake.detailReads
        store.refresh(quiet: true)
        try await settled(store)
        XCTAssertEqual(fake.detailReads, initial)
        fake.snapshotValue.sourceRevision = "source-two"
        store.refresh(quiet: true)
        try await settled(store)
        XCTAssertEqual(fake.detailReads, initial + 1)
        store.stop()
    }
    func testNativeLayoutAndPreview() async throws {
        let store = HQWorkspaceStore(client: WorkspaceFake())
        store.start()
        try await settled(store)
        store.select(.matter("matter-a"))
        try await settled(store)
        store.draft = "Keep the investigation within safe cache cleanup."
        for scheme in [ColorScheme.light, .dark] {
            let host = NSHostingView(
                rootView: HQWorkspaceView(l10n: .shared, store: store).environment(\.colorScheme, scheme)
                    .background(Color(nsColor: .windowBackgroundColor)))
            let window = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: 980, height: 740), styleMask: [.borderless],
                backing: .buffered, defer: false)
            window.isReleasedWhenClosed = false
            window.appearance = NSAppearance(named: scheme == .light ? .aqua : .darkAqua)
            window.contentView = host
            if ProcessInfo.processInfo.environment["GTMUX_WORKSPACE_PREVIEW_DIR"] != nil {
                window.orderFront(nil)
                window.displayIfNeeded()
                try await Task.sleep(nanoseconds: 250_000_000)
            }
            for width in [CGFloat(720), 980] {
                host.frame = NSRect(x: 0, y: 0, width: width, height: 740)
                host.layoutSubtreeIfNeeded()
                XCTAssertLessThanOrEqual(host.fittingSize.width, width + 1)
            }
            if let path = ProcessInfo.processInfo.environment["GTMUX_WORKSPACE_PREVIEW_DIR"] {
                host.frame = NSRect(x: 0, y: 0, width: 980, height: 740)
                host.layoutSubtreeIfNeeded()
                try await Task.sleep(nanoseconds: 250_000_000)
                let bitmap = try XCTUnwrap(host.bitmapImageRepForCachingDisplay(in: host.bounds))
                window.appearance?.performAsCurrentDrawingAppearance {
                    host.cacheDisplay(in: host.bounds, to: bitmap)
                }
                try XCTUnwrap(bitmap.representation(using: .png, properties: [:])).write(
                    to: URL(fileURLWithPath: path).appendingPathComponent(
                        "hq-workspace-\(scheme == .light ? "light" : "dark").png"))
            }
            window.close()
        }
        store.stop()
    }
}
