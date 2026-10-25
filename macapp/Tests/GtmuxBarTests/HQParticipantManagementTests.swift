import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

private final class ParticipationFixture: HQWorkspaceServing, @unchecked Sendable {
    private let lock = NSLock()
    private var sent: [HQWorkspaceAction] = []
    var reject = false
    var value: HQWorkspaceSnapshot
    let now = Int64(Date().timeIntervalSince1970)
    init() {
        let current = HQCandidate(agent: "codex", homeId: "fixture-home", sessionId: "fixture-disk", name: "盘点可释放磁盘空间 | tmp", pane: "%901", loc: "inspect-disk-space:0.0", status: "idle", since: now - 240)
        let extra = HQCandidate(agent: "claude", sessionId: "fixture-tests", name: "核对构建缓存的保留规则", pane: "%902", loc: "cache-policy:1.0", status: "working", since: now - 120)
        let saved = HQCandidate(agent: "codex", sessionId: "fixture-archive", name: "上周磁盘占用排查记录", loc: "old-investigation:0.0", status: "historical", since: now - 86400)
        let p = HQParticipant(id: "disk", conversation: current.conversation, role: "execution", from: now - 3600)
        let r = HQParticipant(id: "notes", conversation: saved.conversation, role: "reference", from: now - 86400 * 2, until: now - 86400)
        let matter = HQMatter(id: "fixture-cleanup", title: "清理磁盘，保留正在使用的工作区", state: "open", createdAt: now - 3600, updatedAt: now, revision: 7, participants: [p, r])
        let target = HQMatter(id: "fixture-build", title: "整理构建缓存与保留规则", state: "open", createdAt: now - 3600, updatedAt: now, revision: 3, participants: [])
        value = HQWorkspaceSnapshot(schema: 1, revision: 7, matters: [matter, target], conversations: [current, extra], references: [saved])
    }
    var actions: [HQWorkspaceAction] { lock.lock(); defer { lock.unlock() }; return sent }
    func snapshot() throws -> HQWorkspaceSnapshot { lock.lock(); defer { lock.unlock() }; return value }
    func detail(_ scope: String, before: UInt64?, earlier: Int) throws -> HQWorkspaceDetail {
        HQWorkspaceDetail(schema: 1, messages: [], turns: [], earlierAvailable: false, changes: [], events: [], warnings: [])
    }
    func participant(_ matter: String, association: String) throws -> HQWorkspaceDetail {
        HQWorkspaceDetail(schema: 1, messages: [], turns: [HQChatTurn(prompt: "检查磁盘占用，先汇报再清理。", response: "构建缓存占用 18 GB；正在使用的工作区已排除。", time: ISO8601DateFormatter().string(from: Date()))], earlierAvailable: false, changes: [], events: [], warnings: [])
    }
    func act(_ action: HQWorkspaceAction) throws {
        lock.lock(); defer { lock.unlock() }
        sent.append(action)
        if reject { throw HQWorkspaceError.command("This matter changed. Return to the list and try again.") }
        guard action.op != "send", action.op != "create", action.op != "focus" else { throw HQWorkspaceError.command("Fixture forbids dispatch and focus") }
        if action.op == "associate", let a = action.association { value.matters[0].participants.append(a) }
        if action.op == "interval", let i = value.matters[0].participants.firstIndex(where: { $0.id == action.associationId }) {
            value.matters[0].participants[i].from = action.from!
            value.matters[0].participants[i].until = action.until
        }
        value.matters[0].revision += 1
        value.revision += 1
    }
    func drain() throws { XCTFail("Management must never dispatch instructions") }
}

@MainActor final class HQParticipantManagementTests: XCTestCase {
    private func settle(_ store: HQWorkspaceStore) async throws {
        for _ in 0..<200 {
            if !store.loading && !store.acting { break }
            try await Task.sleep(nanoseconds: 5_000_000)
        }
        try await Task.sleep(nanoseconds: 100_000_000)
        XCTAssertFalse(store.loading || store.acting)
    }
    private func host(_ store: HQWorkspaceStore, _ flow: HQParticipantManagement, width: CGFloat = 660, dark: Bool = false) -> NSWindow {
        NativeUIProbe.prepareAccessibility()
        let size = CGSize(width: width, height: 640)
        let root = HQWorkspaceParticipants(l10n: .shared, store: store, matter: store.snapshot!.matters[0], flow: flow)
            .environment(\.hqWorkspaceSheetSize, size).environment(\.colorScheme, dark ? .dark : .light)
            .background(Color(nsColor: .windowBackgroundColor))
        let view = NSHostingView(rootView: root)
        let window = NSWindow(contentRect: NSRect(origin: .zero, size: size), styleMask: [.titled, .closable], backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false
        window.title = "gtmux · synthetic conversation management"
        window.appearance = NSAppearance(named: dark ? .darkAqua : .aqua)
        window.contentView = view
        window.orderFront(nil)
        view.layoutSubtreeIfNeeded()
        return window
    }
    private func press(_ id: String, _ window: NSWindow) throws {
        let element = try XCTUnwrap(NativeUIProbe.find(id, in: window), "Missing control \(id)")
        XCTAssertTrue(element.accessibilityPerformPress(), id)
    }
    private func capture(_ name: String, _ window: NSWindow) async throws {
        guard let path = ProcessInfo.processInfo.environment["GTMUX_WORKSPACE_PREVIEW_DIR"] else { return }
        try await Task.sleep(nanoseconds: 200_000_000)
        let view = try XCTUnwrap(window.contentView)
        view.layoutSubtreeIfNeeded()
        // Native child-sheet compositing is unavailable in this test host.
        // Keep their AX/geometry checks, but never export invalid visual evidence.
        if window.sheetParent != nil { return }
        let bitmap = try XCTUnwrap(view.bitmapImageRepForCachingDisplay(in: view.bounds))
        window.effectiveAppearance.performAsCurrentDrawingAppearance { view.cacheDisplay(in: view.bounds, to: bitmap) }
        try XCTUnwrap(bitmap.representation(using: .png, properties: [:])).write(to: URL(fileURLWithPath: path).appendingPathComponent(name + ".png"))
    }
    func testNativeAddAndRangeEditsDispatchOnlyScopedMetadata() async throws {
        let mode = L10n.shared.mode; L10n.shared.mode = .zh; defer { L10n.shared.mode = mode }
        let fake = ParticipationFixture(), flow = HQParticipantManagement()
        let store = HQWorkspaceStore(client: fake); store.refresh(); try await settle(store)
        let window = host(store, flow); defer { window.close(); store.stop() }
        try await capture("02-after-list-zh", window)
        XCTAssertNil(NativeUIProbe.find("participants-from", in: window))
        XCTAssertNil(NativeUIProbe.find("participants-destination", in: window))
        try press("participants-add", window); try await settle(store)
        XCTAssertEqual(flow.page, .add)
        try await capture("03-choose-conversation-zh", window)
        try press("participants-choose-fixture-tests", window); try await settle(store)
        XCTAssertEqual(flow.picked, fake.value.conversations[1].id)
        XCTAssertFalse(flow.reference)
        try await capture("04-add-range-zh", window)
        try press("participants-add-save", window); try await settle(store)
        let add = try XCTUnwrap(fake.actions.last)
        XCTAssertEqual(add.op, "associate"); XCTAssertEqual(add.matter, "fixture-cleanup"); XCTAssertEqual(add.revision, 7)
        XCTAssertEqual(add.association?.conversation.sessionId, "fixture-tests")
        XCTAssertEqual(add.association?.role, "execution"); XCTAssertNil(add.association?.until)
        XCTAssertEqual(flow.page, .list)
        let matter = try XCTUnwrap(store.snapshot?.matters[0]), p = matter.participants[0]
        flow.begin(.interval(p.id), matter: matter, participant: p)
        flow.from = Date(timeIntervalSince1970: TimeInterval(fake.now - 1800))
        flow.ended = true; flow.until = Date(timeIntervalSince1970: TimeInterval(fake.now - 60))
        try await settle(store)
        try await capture("05-adjust-range-zh", window)
        try press("participants-save-range", window); try await settle(store)
        let edit = try XCTUnwrap(fake.actions.last)
        XCTAssertEqual(edit.op, "interval"); XCTAssertEqual(edit.associationId, "disk"); XCTAssertEqual(edit.until, fake.now - 60)
        XCTAssertEqual(fake.actions.map(\.op), ["associate", "interval"])
        XCTAssertNotNil(store.snapshot?.matters[0].participants[0].until)
    }
    func testHistoryForcesBoundedReferenceAndFailureKeepsDraft() async throws {
        let fake = ParticipationFixture(), flow = HQParticipantManagement()
        // Leave the archived conversation available to add.
        fake.value.matters[0].participants.removeLast()
        let store = HQWorkspaceStore(client: fake); store.refresh(); try await settle(store)
        let window = host(store, flow); defer { window.close() }
        try press("participants-add", window); try await settle(store)
        try press("participants-choose-fixture-archive", window); try await settle(store)
        XCTAssertTrue(flow.reference && flow.ended)
        XCTAssertNil(NativeUIProbe.find("participants-reference", in: window))
        try press("participants-rechoose", window); try await settle(store)
        try press("participants-choose-fixture-tests", window); try await settle(store)
        XCTAssertFalse(flow.reference || flow.ended)
        XCTAssertGreaterThan(flow.from.timeIntervalSince1970, Date().timeIntervalSince1970 - 5,
            "A different live source must not inherit the previous historical start")
        try press("participants-rechoose", window); try await settle(store)
        try press("participants-choose-fixture-archive", window); try await settle(store)
        let picked = flow.picked
        fake.reject = true
        try press("participants-add-save", window); try await settle(store)
        XCTAssertNotNil(store.error); XCTAssertEqual(flow.page, .add); XCTAssertEqual(flow.picked, picked)
        XCTAssertEqual(fake.actions.last?.association?.role, "reference")
        XCTAssertNotNil(fake.actions.last?.association?.until)
    }
    func testNarrowBilingualAndDarkLayouts() async throws {
        let mode = L10n.shared.mode; defer { L10n.shared.mode = mode }
        let fake = ParticipationFixture(), store = HQWorkspaceStore(client: ParticipationFixture())
        store.refresh(); try await settle(store)
        for language in [LangMode.en, .zh] {
            L10n.shared.mode = language
            for width: CGFloat in [350, 660] {
                for dark in [false, true] {
                    let flow = HQParticipantManagement(), window = host(store, flow, width: width, dark: dark)
                    defer { window.close() }
                    for (page, name) in [(HQParticipantManagement.Page.list, "list"), (.add, "add"), (.interval("disk"), "interval"), (.move("disk"), "move"), (.organize, "organize")] {
                        flow.begin(page, matter: fake.value.matters[0], participant: fake.value.matters[0].participants[0])
                        if page == .move("disk") || page == .organize { flow.destination = "fixture-build" }
                        try await settle(store)
                        let view = try XCTUnwrap(window.contentView)
                        XCTAssertLessThanOrEqual(view.fittingSize.width, width + 1, "\(page) / \(language)")
                        XCTAssertLessThanOrEqual(view.fittingSize.height, 641)
                        for id in ["participants-add", "participants-save-range", "participants-back", "participants-move", "participants-split-name", "participants-merge"] {
                            if let node = NativeUIProbe.find(id, in: window) {
                                let frame = node.accessibilityFrame()
                                XCTAssertGreaterThan(frame.width, 0, id)
                                XCTAssertGreaterThanOrEqual(frame.minX, window.frame.minX - 1, id)
                                XCTAssertLessThanOrEqual(frame.maxX, window.frame.maxX + 1, "\(id) overflows at \(width)")
                            }
                        }
                        // Keep review evidence concise; every combination above is laid out.
                        if width == 350 || language == .en && !dark && page == .list {
                            try await capture("layout-\(name)-\(language.rawValue)-\(Int(width))-\(dark ? "dark" : "light")", window)
                        }
                    }
                }
            }
        }
    }
    private func alertButton(_ title: String, in window: NSWindow) async throws {
        for _ in 0..<50 {
            if !window.sheets.isEmpty { break }
            try await Task.sleep(nanoseconds: 10_000_000)
        }
        let sheet = try XCTUnwrap(window.sheets.first, "Expected a native confirmation sheet")
        let button = try XCTUnwrap(NativeUIProbe.views(NSButton.self, in: try XCTUnwrap(sheet.contentView)).first { $0.title == title }, "Missing alert button \(title)")
        button.performClick(nil)
        try await Task.sleep(nanoseconds: 150_000_000)
    }
    func testNativeRemovalMoveAndWholeMatterConfirmations() async throws {
        let mode = L10n.shared.mode; L10n.shared.mode = .zh; defer { L10n.shared.mode = mode }
        let fake = ParticipationFixture(), flow = HQParticipantManagement()
        let store = HQWorkspaceStore(client: fake); store.refresh(); try await settle(store)
        let window = host(store, flow); defer { window.close() }
        try press("participants-adjust-disk", window); try await settle(store)
        try await capture("06-adjust-actions-zh", window)
        try press("participants-remove-disk", window); try await settle(store)
        let removal = try XCTUnwrap(flow.confirmation)
        XCTAssertEqual(removal.action.op, "unlink"); XCTAssertEqual(removal.action.associationId, "disk")
        XCTAssertTrue(removal.detail.contains(fake.value.matters[0].title))
        XCTAssertTrue(fake.actions.isEmpty)
        if let sheet = window.sheets.first { try await capture("07-remove-confirmation-zh", sheet) }
        try await alertButton("取消", in: window)
        XCTAssertTrue(fake.actions.isEmpty, "Cancel cannot mutate metadata")
        try press("participants-move-open-disk", window); try await settle(store)
        XCTAssertEqual(flow.page, .move("disk"))
        // Seed the destination; the native review and confirmation buttons are pressed below.
        flow.destination = "fixture-build"; try await settle(store)
        try await capture("08-move-zh", window)
        try press("participants-move", window); try await settle(store)
        let move = try XCTUnwrap(flow.confirmation)
        XCTAssertEqual(move.action.op, "move"); XCTAssertEqual(move.action.into, "fixture-build")
        XCTAssertEqual(move.action.associationId, "disk"); XCTAssertEqual(move.action.revision, 7)
        XCTAssertTrue(fake.actions.isEmpty)
        if let sheet = window.sheets.first { try await capture("09-move-confirmation-zh", sheet) }
        try await alertButton("移动对话", in: window); try await settle(store)
        XCTAssertEqual(fake.actions.map(\.op), ["move"])
        try press("participants-organize", window); try await settle(store)
        XCTAssertEqual(flow.destination, "")
        try press("participants-split-disk", window); try await settle(store)
        XCTAssertEqual(flow.splitIDs, ["disk"])
        flow.splitName = "核对缓存保留规则"; try await settle(store)
        try press("participants-split", window); try await settle(store)
        let split = try XCTUnwrap(flow.confirmation)
        XCTAssertEqual(split.action.op, "split"); XCTAssertEqual(split.action.associations, ["disk"])
        XCTAssertTrue(split.detail.contains(fake.value.matters[0].title))
        try await alertButton("取消", in: window)
        flow.destination = "fixture-build"; try await settle(store)
        try press("participants-merge", window); try await settle(store)
        let merge = try XCTUnwrap(flow.confirmation)
        XCTAssertEqual(merge.action.op, "merge"); XCTAssertNil(merge.action.associationId)
        XCTAssertTrue(merge.detail.contains(fake.value.matters[0].title))
        XCTAssertTrue(merge.detail.contains(fake.value.matters[1].title))
        if let sheet = window.sheets.first { try await capture("10-merge-confirmation-zh", sheet) }
        try await alertButton("取消", in: window)
        XCTAssertEqual(fake.actions.map(\.op), ["move"])
    }
    func testNativeReadAndConfirmedRemoveSplitMergeNeverDispatch() async throws {
        let mode = L10n.shared.mode; L10n.shared.mode = .zh; defer { L10n.shared.mode = mode }
        for operation in ["unlink", "split", "merge"] {
            let fake = ParticipationFixture(), flow = HQParticipantManagement()
            let store = HQWorkspaceStore(client: fake); store.refresh(); try await settle(store)
            let window = host(store, flow); defer { window.close() }
            if operation == "unlink" {
                try press("participants-read-disk", window); try await settle(store)
                let reader = try XCTUnwrap(window.sheets.first)
                try await capture("11-read-source-zh", reader)
                XCTAssertTrue(fake.actions.isEmpty)
                try press("participant-reader-done", reader); try await settle(store)
                try press("participants-adjust-disk", window); try await settle(store)
                try press("participants-remove-disk", window); try await settle(store)
                try await alertButton("移除此对话", in: window)
            } else {
                try press("participants-organize", window); try await settle(store)
                if operation == "split" {
                    try press("participants-split-disk", window); try await settle(store)
                    flow.splitName = "缓存规则检查"; try await settle(store)
                    try press("participants-split", window); try await settle(store)
                    try await alertButton("创建并移入", in: window)
                } else {
                    flow.destination = "fixture-build"; try await settle(store)
                    try press("participants-merge", window); try await settle(store)
                    try await alertButton("合并并结束", in: window)
                }
            }
            try await settle(store)
            XCTAssertEqual(fake.actions.map(\.op), [operation])
            XCTAssertEqual(fake.actions[0].matter, "fixture-cleanup")
            XCTAssertEqual(fake.actions[0].revision, 7)
            if operation == "unlink" { XCTAssertEqual(fake.actions[0].associationId, "disk") }
            if operation == "split" { XCTAssertEqual(fake.actions[0].associations, ["disk"]) }
            if operation == "merge" { XCTAssertEqual(fake.actions[0].into, "fixture-build") }
        }
    }
    func testSourceReaderAtPhoneWidthKeepsItsControlsInBounds() async throws {
        let mode = L10n.shared.mode; L10n.shared.mode = .zh; defer { L10n.shared.mode = mode }
        let fake = ParticipationFixture(), flow = HQParticipantManagement()
        let store = HQWorkspaceStore(client: fake); store.refresh(); try await settle(store)
        let window = host(store, flow, width: 350); defer { window.close() }
        try press("participants-read-disk", window); try await settle(store)
        let reader = try XCTUnwrap(window.sheets.first)
        try await capture("12-read-source-narrow-zh", reader)
        let done = try XCTUnwrap(NativeUIProbe.find("participant-reader-done", in: reader))
        XCTAssertGreaterThanOrEqual(done.accessibilityFrame().minX, reader.frame.minX)
        XCTAssertLessThanOrEqual(done.accessibilityFrame().maxX, reader.frame.maxX)
        XCTAssertLessThanOrEqual(try XCTUnwrap(reader.contentView).fittingSize.width, 351)
        try press("participant-reader-done", reader)
        XCTAssertTrue(fake.actions.isEmpty)

    }
    func testIdentityAndCollectionEndAreIndependentOfProcessState() {
        let fake = ParticipationFixture()
        var participant = fake.value.matters[0].participants[0]
        participant.until = fake.now - 60
        let record = HQParticipantRecord(participant: participant, current: fake.value.conversations[0])
        XCTAssertNotNil(record.presentation.current, "Ended collection does not mean the agent ended")
        XCTAssertTrue(record.scope(.shared).contains(HQParticipantRecord.date(fake.now - 60, .shared)))
        var reused = fake.value.conversations[0]; reused.sessionId = "different-session"
        XCTAssertNil(HQParticipantRecord(participant: participant, current: reused).presentation.current)
        let flow = HQParticipantManagement(); flow.begin(.move(participant.id), matter: fake.value.matters[0], participant: participant)
        flow.destination = "fixture-build"
        flow.begin(.organize, matter: fake.value.matters[0])
        XCTAssertEqual(flow.destination, "", "Moving one record cannot preselect a whole-matter merge")
        XCTAssertEqual(flow.revision, 7)
        flow.reference = true; flow.until = flow.from.addingTimeInterval(-1)
        XCTAssertFalse(flow.validInterval())
        flow.until = Date().addingTimeInterval(100)
        XCTAssertFalse(flow.validInterval())
    }
}
