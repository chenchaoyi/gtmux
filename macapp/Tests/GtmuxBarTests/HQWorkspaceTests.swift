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
    var failRead = false
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
        if failRead { throw HQWorkspaceError.command("fixture read failed") }
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

    func testEmptyWorkspacePreparesReviewWithoutSendingAndShowsRegisteredWork() async throws {
        let fake = WorkspaceFake()
        fake.snapshotValue = decodeWorkspace(#"{"schema":1,"revision":0,"matters":[],"conversations":[]}"#)
        let store = HQWorkspaceStore(client: fake)
        store.refresh()
        try await settled(store)
        XCTAssertEqual(store.snapshot?.matters.count, 0)
        store.select(.newTask)
        store.draft = "Keep my new-task draft"
        XCTAssertTrue(store.prepareExistingWorkReview(body: "Review existing work"))
        try await settled(store)
        XCTAssertEqual(store.selection, .conversation)
        XCTAssertEqual(store.draft, "Review existing work")
        XCTAssertTrue(fake.actions.isEmpty, "preparing a review silently sent an instruction")
        store.select(.newTask)
        XCTAssertEqual(store.draft, "Keep my new-task draft")
        try await settled(store)

        fake.snapshotValue = decodeWorkspace(#"{"schema":1,"revision":1,"matters":[{"id":"existing-goal","title":"Already assigned","state":"needs-input","created_at":100,"updated_at":100,"revision":1,"participants":[],"origin":{"kind":"task","ref":"original-task-id"}}],"conversations":[]}"#)
        store.refresh()
        try await settled(store)
        XCTAssertEqual(store.snapshot?.matters.first?.id, "existing-goal")
        XCTAssertEqual(store.snapshot?.matters.first?.origin?.ref, "original-task-id")
        XCTAssertEqual(store.snapshot?.matters.first?.state, "needs-input")
        XCTAssertTrue(fake.actions.isEmpty)
    }

    func testReviewRequestCannotReplaceGlobalOrScopedDrafts() async throws {
        let fake = WorkspaceFake()
        let store = HQWorkspaceStore(client: fake)
        store.select(.conversation)
        try await settled(store)
        store.draft = "Existing global draft"
        store.select(.newTask)
        store.draft = "Scoped draft"
        XCTAssertFalse(store.prepareExistingWorkReview(body: "Do not replace"))
        try await settled(store)
        XCTAssertEqual(store.draft, "Existing global draft")
        XCTAssertTrue(fake.actions.isEmpty)
        store.select(.newTask)
        XCTAssertEqual(store.draft, "Scoped draft")
        try await settled(store)
        store.select(.conversation)
        try await settled(store)
        XCTAssertFalse(store.prepareExistingWorkReview(body: "Still do not replace"))
        XCTAssertEqual(store.draft, "Existing global draft")
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
        store.select(.conversation)
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
        store.select(.conversation)
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
        let controller = HQWorkspaceController(store: store, autosaveName: nil)
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
    func testParticipantReaderKeepsItsEnvelopeAfterLoadingLongHistory() async throws {
        let fake = WorkspaceFake()
        fake.page = { _, _, _ in
            var detail: HQWorkspaceDetail = decodeWorkspace(detailFixture)
            detail.turns = (0..<40).map { _ in HQChatTurn(prompt: String(repeating: "A long user instruction. ", count: 20), response: String(repeating: "A long response. ", count: 200)) }
            return detail
        }
        let store = HQWorkspaceStore(client: fake)
        let participant = HQParticipant(id: "linked", conversation: HQConversation(agent: "codex", sessionId: "session", name: "Linked conversation"), role: "execution", from: 100)
        for parent in [CGSize(width: 900, height: 480), CGSize(width: 1100, height: 760)] {
            let size = HQWorkspaceSheet.size(in: parent)
            XCTAssertLessThan(size.height, parent.height)
            let view = HQParticipantReader(l10n: .shared, store: store, participant: participant, matterID: "matter-a")
                .environment(\.hqWorkspaceSheetSize, size)
            let host = NSHostingView(rootView: view)
            let window = NSWindow(contentRect: NSRect(origin: .zero, size: CGSize(width: size.width, height: min(600, size.height))), styleMask: [.titled, .closable], backing: .buffered, defer: false)
            window.isReleasedWhenClosed = false
            host.sizingOptions = []
            window.contentView = host
            window.orderFront(nil)
            let before = window.frame
            try await Task.sleep(nanoseconds: 200_000_000)
            host.layoutSubtreeIfNeeded()
            XCTAssertEqual(window.frame.size, before.size, "Loading history must not expand the sheet beyond its close controls")
            XCTAssertLessThanOrEqual(host.fittingSize.height, min(600, size.height) + 1)
            window.close()
        }
        XCTAssertTrue(fake.actions.isEmpty, "Viewing a source must not send an instruction")
    }
    func testParticipantLabelsUseExactIdentityAndMarkSavedPanesHistorical() {
        let linked = HQParticipant(
            id: "association",
            conversation: HQConversation(
                agent: "codex", homeId: "home-a", sessionId: "original", name: "Saved title", pane: "%43"),
            role: "execution", from: 100)
        var live = HQCandidate(
            agent: "codex", homeId: "home-a", sessionId: "original", name: "Current title", pane: "%57",
            loc: "work:2.0", status: "working", since: 100)
        let current = HQParticipantPresentation(participant: linked, current: live)
        XCTAssertEqual(current.title, "Current title")
        XCTAssertEqual(current.metadata(.shared), "Codex · %57 · work:2.0")
        XCTAssertEqual(current.state(.shared), hqConversationStatus("working", .shared))
        live.sessionId = "different-occupant"
        live.pane = "%43"
        let reused = HQParticipantPresentation(participant: linked, current: live)
        XCTAssertNil(reused.current)
        XCTAssertEqual(reused.title, "Saved title")
        XCTAssertEqual(
            reused.metadata(.shared), "Codex · " + L10n.shared.tr("Last pane %43", "最近窗格 %43"))
        XCTAssertEqual(
            reused.state(.shared),
            L10n.shared.tr("Original record · no current session", "原始记录 · 无当前会话"))
        live.sessionId = "original"
        live.homeId = "other-home"
        XCTAssertNil(HQParticipantPresentation(participant: linked, current: live).current)
    }
    func testAllResourceEntrancesReuseWindowAndPreserveAllDraftScopes() async throws {
        let fake = WorkspaceFake()
        let reads = ResourceReadFake()
        let resources = HQResourceStore(read: reads.read)
        let store = HQWorkspaceStore(client: fake, resources: resources)
        let controller = HQWorkspaceController(store: store, autosaveName: nil)
        controller.show(l10n: .shared)
        try await settled(store)
        let window = try XCTUnwrap(controller.window)
        for scope in [HQWorkspaceSelection.conversation, .matter("matter-a"), .newTask] {
            store.select(scope)
            try await settled(store)
            store.draft = "Draft for \(scope)"
            for page in HQResourcePage.allCases {
                controller.show(l10n: .shared, selection: .resource(page))
                try await settled(store)
                XCTAssertTrue(window === controller.window)
                XCTAssertEqual(store.selection, .resource(page))
                XCTAssertFalse(store.selection.showsComposer)
                store.draft = "Even programmatic text cannot send from resources"
                XCTAssertFalse(store.canSend)
                store.send()
                controller.show(l10n: .shared)
                XCTAssertEqual(store.selection, .resource(page), "generic open preserves destination")
            }
            store.select(scope)
            try await settled(store)
            XCTAssertEqual(store.draft, "Draft for \(scope)")
            XCTAssertFalse(resources.isPolling)
        }
        XCTAssertTrue(fake.actions.isEmpty)
        controller.show(l10n: .shared, selection: .resource(.knowledge))
        try await settled(store)
        resources.knowledgeNavigation.query = "lesson"
        resources.knowledgeNavigation.pane = .entry(id: "fixture/lesson")
        store.select(.conversation)
        store.select(.resource(.knowledge))
        XCTAssertEqual(resources.knowledgeNavigation.query, "lesson")
        XCTAssertEqual(resources.knowledgeNavigation.pane, .entry(id: "fixture/lesson"))
        window.close()
        XCTAssertFalse(resources.isPolling)
        XCTAssertNil(resources.activePage)
        controller.show(l10n: .shared)
        XCTAssertEqual(resources.activePage, .knowledge)
        window.close()
    }
    func testResourceDestinationsFitCompactAndWideWorkspaceInBothAppearances() async throws {
        let reads = ResourceReadFake()
        let resources = HQResourceStore(read: reads.read)
        let store = HQWorkspaceStore(client: WorkspaceFake(), resources: resources)
        store.start()
        try await settled(store)
        for page in HQResourcePage.allCases {
            store.select(.resource(page))
            try await settled(store)
            for _ in 0..<200 {
                if resources.refreshingPages.isEmpty { break }
                try await Task.sleep(nanoseconds: 5_000_000)
            }
            XCTAssertTrue(resources.refreshingPages.isEmpty)
            resources.knowledgeNavigation.pane = .entry(id: "fixture/lesson")
            for scheme in [ColorScheme.light, .dark] {
                let host = NSHostingView(
                    rootView: HQWorkspaceView(l10n: .shared, store: store)
                        .environment(\.colorScheme, scheme))
                for width in [CGFloat(900), 1100] {
                    host.frame = NSRect(x: 0, y: 0, width: width, height: 720)
                    host.layoutSubtreeIfNeeded()
                    XCTAssertLessThanOrEqual(
                        host.fittingSize.width, width + 1, "Overflow: \(page) at \(width)")
                }
            }
        }
        store.stop()
    }

    func testQuietPollSkipsUnchangedSourcesButReadsChangedPublicTranscript() async throws {
        let fake = WorkspaceFake()
        fake.snapshotValue.sourceRevision = "source-one"
        let store = HQWorkspaceStore(client: fake)
        store.start()
        try await settled(store)
        store.select(.conversation)
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
    func testOverviewIsReadOnlyAndDoesNotParseConversationUntilSelected() async throws {
        let fake = WorkspaceFake()
        let store = HQWorkspaceStore(client: fake)
        XCTAssertEqual(store.selection, .overview)
        XCTAssertEqual(store.readPhase, .loading)
        store.start()
        try await settled(store)
        XCTAssertEqual(store.readPhase, .content)
        XCTAssertEqual(fake.detailReads, 0)
        store.draft = "Cannot send here"
        store.send()
        XCTAssertFalse(store.canSend)
        XCTAssertTrue(fake.actions.isEmpty)
        XCTAssertEqual(HQResourcePage.board.workspaceShortcut, .overview)
        XCTAssertFalse(HQResourcePage.sidebarPages.contains(.board))
        for page in HQResourcePage.sidebarPages { XCTAssertEqual(page.workspaceShortcut, .resource(page)) }
        store.select(.conversation)
        try await settled(store)
        store.draft = "Keep conversation draft"
        store.select(.overview)
        try await settled(store)
        store.select(.conversation)
        try await settled(store)
        XCTAssertEqual(store.draft, "Keep conversation draft")
        store.stop()
    }
    func testReadFailureAndRetainedRefreshAreNotEmptyOrOffline() async throws {
        let fake = WorkspaceFake()
        fake.delay = 0.03
        fake.failRead = true
        let store = HQWorkspaceStore(client: fake)
        store.start()
        XCTAssertEqual(store.readPhase, .loading)
        XCTAssertNil(store.snapshot, "No offline claim before reading supervisor")
        try await settled(store)
        XCTAssertEqual(store.readPhase, .failed)
        fake.failRead = false
        store.refresh()
        try await settled(store)
        XCTAssertEqual(store.readPhase, .content)
        let ids = store.snapshot?.matters.map(\.id)
        fake.failRead = true
        store.refresh()
        XCTAssertEqual(store.readPhase, .content)
        try await settled(store)
        XCTAssertEqual(store.readPhase, .content)
        XCTAssertEqual(store.snapshot?.matters.map(\.id), ids)
        XCTAssertNotNil(store.error)
        store.stop()
    }
    func testDisclosuresAreIndependentAndSurviveNavigationAndReopen() async throws {
        let store = HQWorkspaceStore(client: WorkspaceFake())
        store.start()
        try await settled(store)
        store.toggleSection(.matters(.following))
        store.toggleSection(.resources)
        store.select(.matter("matter-a"))
        try await settled(store)
        store.stop()
        store.start()
        try await settled(store)
        XCTAssertEqual(store.collapsedSections, [.matters(.following), .resources])
        XCTAssertEqual(store.selection, .matter("matter-a"))
        store.toggleSection(.resources)
        XCTAssertEqual(store.collapsedSections, [.matters(.following)])
        store.stop()
    }
    func testOverviewAndSidebarGroupEachExplicitMatterExactlyOnce() {
        var snapshot: HQWorkspaceSnapshot = decodeWorkspace(workspaceFixture)
        var ended = snapshot.matters[0]
        ended.id = "ended"
        ended.state = "completed"
        var closed = ended
        closed.id = "closed"
        closed.state = "closed"
        var future = ended
        future.id = "future"
        future.state = "future-state"
        snapshot.matters += [ended, closed, future]
        let grouped = HQMatterGroup.allCases.flatMap { $0.matters(in: snapshot) }
        XCTAssertEqual(Set(grouped.map(\.id)), Set(snapshot.matters.map(\.id)))
        XCTAssertEqual(grouped.count, snapshot.matters.count)
        XCTAssertEqual(HQMatterGroup.needsInput.matters(in: snapshot).map(\.id), ["matter-b"])
        XCTAssertEqual(Set(HQMatterGroup.ended.matters(in: snapshot).map(\.id)), ["ended", "closed"])
        XCTAssertEqual(HQMatterGroup.needsInput.status, .waiting)
        XCTAssertEqual(HQMatterGroup.following.status, .working)
        XCTAssertEqual(HQMatterGroup.ended.status, .idle)
        XCTAssertEqual(HQMatterGroup.other.matters(in: snapshot).map(\.id), ["future"])
    }
    func testOverviewConversationActivityAndLoadingFitCompactAndWideLayouts() async throws {
        let fake = WorkspaceFake()
        let store = HQWorkspaceStore(client: fake)
        // Unread state uses a loading view, then the same host can render valid data.
        for selection in [HQWorkspaceSelection.overview, .conversation, .activity] {
            if selection != .overview { store.select(selection); try await settled(store) }
            for scheme in [ColorScheme.light, .dark] {
                let host = NSHostingView(rootView: HQWorkspaceView(l10n: .shared, store: store)
                    .environment(\.colorScheme, scheme))
                for width in [CGFloat(900), 1100] {
                    host.frame = NSRect(x: 0, y: 0, width: width, height: 720)
                    host.layoutSubtreeIfNeeded()
                    XCTAssertLessThanOrEqual(host.fittingSize.width, width + 1, "Overflow: \(selection) at \(width)")
                }
            }
        }
        store.select(.overview)
        try await settled(store)
        // These offscreen hosts do not promise window visibility. Explicitly finish
        // a read after a discarded selection rather than relying on a polling timer.
        store.refresh()
        try await settled(store)
        XCTAssertNotNil(store.snapshot)
        let host = NSHostingView(rootView: HQWorkspaceView(l10n: .shared, store: store))
        host.frame = NSRect(x: 0, y: 0, width: 900, height: 720)
        host.layoutSubtreeIfNeeded()
        XCTAssertLessThanOrEqual(host.fittingSize.width, 901)
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
    func testActivityTaxonomyKeepsUnknownAndFailedReceiptsVisible() throws {
        let groups: [(HQActivityCategory, [String])] = [
            (.instructions, ["instruction", "cancel", "retry", "delivery-held", "delivery-attempt", "delivery-delivered", "delivery-accepted-queue", "delivery-uncertain", "UserPromptSubmit", "gtmux:audit:send"]),
            (.updates, ["hq-report", "hq-progress", "hq-result", "hq-decision", "hq-error"]),
            (.organization, ["create", "register", "update", "associate", "move", "unlink", "interval", "merge", "split"]),
            (.supervision, ["Stop", "StopFailure", "gtmux:audit:wake-delivered", "gtmux:audit:wake-dropped", "gtmux:wake-degraded", "gtmux:self-check"]),
            (.knowledge, ["gtmux:audit:knowledge", "gtmux:distill"]),
            (.maintenance, ["gtmux:audit:rotate", "gtmux:audit:rotate-requested", "gtmux:audit:rotate-failed", "gtmux:audit:hq-session", "gtmux:audit:reap", "SessionStart", "SessionEnd"]),
            (.other, ["future-action"]),
        ]
        for (category, kinds) in groups {
            for kind in kinds {
                let item = HQActivityPresentation(kind: kind)
                XCTAssertEqual(item.category, category, kind)
                XCTAssertTrue(item.includes(in: nil))
                XCTAssertTrue(item.includes(in: category))
                XCTAssertEqual(HQActivityCategory.allCases.filter { item.includes(in: $0) }, [category])
                XCTAssertNotNil(NSImage(systemSymbolName: item.symbol, accessibilityDescription: nil), item.symbol)
            }
        }
        let records = ["future-action", "hq-progress", "associate", "hq-result", "hq-error"]
        XCTAssertEqual(records.filter { HQActivityPresentation(kind: $0).includes(in: .updates) }, ["hq-progress", "hq-result", "hq-error"])
        XCTAssertFalse(HQActivityPresentation(kind: "gtmux:audit:send", outcome: "ok").needsAttention)
        XCTAssertTrue(HQActivityPresentation(kind: "gtmux:audit:send", outcome: "refused").needsAttention)
        XCTAssertTrue(HQActivityPresentation(kind: "gtmux:audit:knowledge", outcome: "committed-view-failed").needsAttention)
        XCTAssertTrue(HQActivityPresentation(kind: "delivery-uncertain").needsAttention)
        XCTAssertFalse(HQActivityPresentation(kind: "future-action", outcome: "future-outcome").needsAttention)
        XCTAssertEqual(HQActivityPresentation(kind: "instruction").detail("execution", l10n: .shared), "execution", "free prose must not be rewritten as an enum")
    }

    func testReadingScaleKeepsNavigationHierarchyAndUnrelatedMarkdownDefaults() {
        XCTAssertEqual(HQTypography.Role.body.nativeFont.pointSize, 15)
        XCTAssertGreaterThanOrEqual(HQTypography.Role.sidebarGroup.size, HQTypography.Role.sidebarItem.size)
        XCTAssertGreaterThan(HQTypography.Role.sidebarGroup.weight.rawValue, HQTypography.Role.sidebarItem.weight.rawValue)
        XCTAssertGreaterThan(HQTypography.Role.section.size, HQTypography.Role.body.size)
        XCTAssertGreaterThan(HQTypography.Role.title.size, HQTypography.Role.section.size)
        XCTAssertEqual(MarkdownTypography.workspace.body, HQTypography.Role.body.size)
        XCTAssertEqual(MarkdownTypography.workspace.code, HQTypography.Role.code.size)
        XCTAssertEqual(EnvironmentValues().markdownTypography, .standard)
        XCTAssertEqual(MarkdownTypography.standard.body, 12)
    }

    func testReadableActivityFitsAndRendersWithoutRealHQ() async throws {
        let fake = WorkspaceFake()
        fake.page = { _, _, _ in
            var detail: HQWorkspaceDetail = decodeWorkspace(detailFixture)
            detail.matterTitles = ["matter-a": "Investigate storage and preserve active work"]
            detail.changes = ["instruction", "associate", "interval", "hq-progress", "hq-result", "gtmux:audit:knowledge", "future-action"].enumerated().map { i, kind in
                HQWorkspaceChange(seq: UInt64(i + 1), at: Int64(1792112400 + i * 60), kind: kind, matter: "matter-a", detail: kind == "associate" ? "execution" : nil)
            }
            detail.events = [HQWorkspaceEvent(ts: 1792113000, seq: 50, event: "gtmux:audit:rotate-requested", summary: "Rotation will wait for a safe idle input box.", pane: "%22")]
            return detail
        }
        let store = HQWorkspaceStore(client: fake)
        store.select(.activity)
        try await settled(store)
        for scheme in [ColorScheme.light, .dark] {
            let host = NSHostingView(rootView: HQWorkspaceView(l10n: .shared, store: store)
                .environment(\.colorScheme, scheme).background(Color(nsColor: .windowBackgroundColor)))
            let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 1100, height: 760), styleMask: [.borderless], backing: .buffered, defer: false)
            window.isReleasedWhenClosed = false
            window.appearance = NSAppearance(named: scheme == .light ? .aqua : .darkAqua)
            window.contentView = host
            for width in [CGFloat(900), 1100] {
                host.frame = NSRect(x: 0, y: 0, width: width, height: 760)
                host.layoutSubtreeIfNeeded()
                XCTAssertLessThanOrEqual(host.fittingSize.width, width + 1)
            }
            if let path = ProcessInfo.processInfo.environment["GTMUX_WORKSPACE_PREVIEW_DIR"] {
                window.orderFront(nil)
                try await Task.sleep(nanoseconds: 250_000_000)
                host.layoutSubtreeIfNeeded()
                let bitmap = try XCTUnwrap(host.bitmapImageRepForCachingDisplay(in: host.bounds))
                window.appearance?.performAsCurrentDrawingAppearance { host.cacheDisplay(in: host.bounds, to: bitmap) }
                try XCTUnwrap(bitmap.representation(using: .png, properties: [:])).write(to: URL(fileURLWithPath: path).appendingPathComponent("hq-activity-\(scheme == .light ? "light" : "dark").png"))
            }
            window.close()
        }
        XCTAssertTrue(fake.actions.isEmpty)
        store.stop()
    }

}

extension HQWorkspaceTests {
    func testReselectingConversationRequestsLatestWithoutDiscardingDraftOrHistory() async throws {
        let fake = WorkspaceFake()
        let store = HQWorkspaceStore(client: fake)
        store.select(.conversation); try await settled(store)
        let serial = store.conversationRevealSerial
        let ids = store.detail?.messages.map(\.id)
        store.draft = "An unfinished request"
        store.select(.conversation); try await settled(store)
        XCTAssertEqual(store.conversationRevealSerial, serial + 1)
        XCTAssertEqual(store.draft, "An unfinished request")
        XCTAssertEqual(store.detail?.messages.map(\.id), ids)
        store.select(.activity); try await settled(store)
        XCTAssertEqual(store.conversationRevealSerial, serial + 1)
        store.select(.matter("matter-a")); try await settled(store)
        XCTAssertEqual(store.conversationRevealSerial, serial + 2)
        XCTAssertTrue(fake.actions.isEmpty)
    }

    func testCachedConversationOpensAtLatestAndReselectionReturnsFromHistory() async throws {
        let fake = WorkspaceFake()
        var count = 50
        fake.page = { _, _, _ in
            var detail: HQWorkspaceDetail = decodeWorkspace(detailFixture)
            detail.messages = (0..<count).map { i in
                HQWorkspaceMessage(id: "message-\(i)", speaker: "hq", kind: "reply",
                    body: "Reply \(i): a synthetic record for checking retained reading position.", at: Int64(1792190000 + i), seq: UInt64(i + 1))
            }
            return detail
        }
        let store = HQWorkspaceStore(client: fake)
        store.select(.conversation); try await settled(store)
        let host = NSHostingView(rootView: HQWorkspaceConversationView(l10n: .shared, store: store)
            .font(HQTypography.body).environment(\.markdownTypography, .workspace))
        let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 800, height: 480), styleMask: [.borderless], backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false; window.contentView = host; window.orderFront(nil)
        defer { window.close(); store.stop() }
        func scrollView(_ view: NSView) -> NSScrollView? {
            if let s = view as? NSScrollView, (s.documentView?.bounds.height ?? 0) > s.contentView.bounds.height + 100 { return s }
            for child in view.subviews { if let found = scrollView(child) { return found } }
            return nil
        }
        func isAtTail(_ scroll: NSScrollView) -> Bool {
            guard let document = scroll.documentView else { return false }
            let visible = scroll.documentVisibleRect
            return document.isFlipped ? abs(document.bounds.maxY - visible.maxY) < 45 : abs(visible.minY - document.bounds.minY) < 45
        }
        var scroll: NSScrollView?
        for _ in 0..<100 {
            host.layoutSubtreeIfNeeded(); scroll = scrollView(host)
            if let scroll, isAtTail(scroll) { break }
            try await Task.sleep(nanoseconds: 20_000_000)
        }
        let scroller = try XCTUnwrap(scroll)
        XCTAssertTrue(isAtTail(scroller), "Already-loaded content must not first open at the oldest record")
        let top = scroller.documentView!.isFlipped ? CGFloat(0) : scroller.documentView!.bounds.maxY - scroller.contentView.bounds.height
        scroller.contentView.scroll(to: NSPoint(x: 0, y: top)); scroller.reflectScrolledClipView(scroller.contentView)
        try await Task.sleep(nanoseconds: 60_000_000)
        let before = scroller.documentVisibleRect
        count += 1; fake.snapshotValue.revision += 1
        store.refresh(); try await settled(store)
        try await Task.sleep(nanoseconds: 60_000_000)
        XCTAssertLessThan(abs(scroller.documentVisibleRect.minY - before.minY), 45, "A refresh cannot pull a historical reader back to the tail")
        store.select(.conversation); try await settled(store)
        for _ in 0..<100 {
            host.layoutSubtreeIfNeeded()
            if isAtTail(scroller) { break }
            try await Task.sleep(nanoseconds: 20_000_000)
        }
        XCTAssertTrue(isAtTail(scroller), "Clicking the selected conversation again must reveal the latest record")
        XCTAssertTrue(fake.actions.isEmpty)
    }
}

extension HQWorkspaceTests {
    /// Real native controls with synthetic content, independent of installed HQ/keychain data.
    func testReadingComponentsFitAndRenderInBothLanguagesAndAppearances() async throws {
        let language = L10n.shared.mode
        defer { L10n.shared.mode = language }
        let resources = HQResourceStore(read: { args in
            if args.contains("--records") { return Data(#"{"exists":true,"bytes":12582912,"snapshots":14}"#.utf8) }
            return Data("[]".utf8)
        })
        resources.start(page: .knowledge)
        for _ in 0..<200 {
            if resources.refreshingPages.isEmpty { break }
            try await Task.sleep(nanoseconds: 5_000_000)
        }
        resources.stop()
        let entries: [KBEntry] = decodeWorkspace(###"[{"id":"safe-cleanup","topic":"preferences","title":"Review storage before removing build files","body":"## Safe cleanup\n\nCheck available storage and identify task-owned build artifacts.\n\nPreserve active workspaces and shared caches.","lang":"en","alt":{"lang":"zh","title":"清理构建文件前先检查磁盘","body":"## 安全清理\n\n检查可用空间，确认构建产物属于当前任务。\n\n保留正在使用的工作区和共享缓存。"}}]"###)
        resources.setEntriesForTest(entries)
        resources.knowledgeNavigation.pane = .entry(id: "safe-cleanup")
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            let flow = HQExportFlow(readPassphrase: { nil })
            let fake = WorkspaceFake()
            let store = HQWorkspaceStore(client: fake)
            store.select(.conversation); try await settled(store)
            store.draft = mode == .en ? "Check storage and report before deleting anything." : "请检查磁盘占用，先汇报再清理。"
            let views: [(String, AnyView, CGSize)] = [
                ("knowledge", AnyView(HQKnowledgeView(l10n: .shared, store: resources, navigation: resources.knowledgeNavigation)), CGSize(width: 920, height: 680)),
                ("export", AnyView(HQExportSheet(l10n: .shared, flow: flow, onClose: {})), CGSize(width: 480, height: 620)),
                ("import", AnyView(HQImportSheet(l10n: .shared, flow: HQImportFlow(purpose: .restore, runner: { _, _ in XCTFail("Preview cannot invoke import"); return (1, "", "unexpected") }), onClose: {})), CGSize(width: 560, height: 620)),
                ("composer", AnyView(HQWorkspaceComposer(l10n: .shared, store: store)), CGSize(width: 560, height: 180)),
            ]
            for scheme in [ColorScheme.light, .dark] {
                for (name, view, size) in views {
                    let host = NSHostingView(rootView: view.environment(\.colorScheme, scheme)
                        .font(HQTypography.body).environment(\.markdownTypography, .workspace)
                        .background(Color(nsColor: .windowBackgroundColor)))
                    let window = NSWindow(contentRect: NSRect(origin: .zero, size: size), styleMask: [.borderless], backing: .buffered, defer: false)
                    window.isReleasedWhenClosed = false
                    window.appearance = NSAppearance(named: scheme == .light ? .aqua : .darkAqua)
                    window.contentView = host
                    host.layoutSubtreeIfNeeded()
                    XCTAssertLessThanOrEqual(host.fittingSize.width, size.width + 1, "\(name) overflows in \(mode)")
                    XCTAssertLessThanOrEqual(host.fittingSize.height, size.height + 1, "\(name) clips vertically in \(mode)")
                    if let path = ProcessInfo.processInfo.environment["GTMUX_WORKSPACE_PREVIEW_DIR"] {
                        window.orderFront(nil)
                        try await Task.sleep(nanoseconds: 100_000_000)
                        host.layoutSubtreeIfNeeded()
                        let bitmap = try XCTUnwrap(host.bitmapImageRepForCachingDisplay(in: host.bounds))
                        window.appearance?.performAsCurrentDrawingAppearance { host.cacheDisplay(in: host.bounds, to: bitmap) }
                        try XCTUnwrap(bitmap.representation(using: .png, properties: [:])).write(to: URL(fileURLWithPath: path)
                            .appendingPathComponent("hq-\(name)-\(mode.rawValue)-\(scheme == .light ? "light" : "dark").png"))
                    }
                    window.close()
                }
            }
            XCTAssertTrue(fake.actions.isEmpty)
            store.stop()
        }
    }
}
