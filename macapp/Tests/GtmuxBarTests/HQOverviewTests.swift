import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

private func overviewFixture(zh: Bool = false) throws -> HQWorkspaceSnapshot {
    let now = Int64(Date().timeIntervalSince1970)
    let titles = zh ? ["核对发版候选", "确认恢复目标", "选择通知策略", "核对待处理权限", "桌面命令展示", "后台请求取消", "DeepSeek 用量来源", "知识库文档"] : ["Review the release candidate", "Confirm a recovery destination", "Choose a notification policy", "Review pending access",
                  "Desktop command presentation", "Background delivery reconciliation", "DeepSeek usage sources", "Knowledge documentation"]
    let reports = zh ? ["候选已通过必要检查。安装前，还需要确认剩余的设备验收范围。", "恢复包已校验，保存位置和保留周期还需要你决定。", "当前通知行为保持原样，调整方案已准备好供审阅。", "", "相关命令已归组，展开后可检查完整命令。", "已覆盖后台取消场景；前一笔请求返回前，后续写入仍按顺序等待。", "钱包余额与额度窗口分开展示，并保留原始十进制精度。", ""] : ["The candidate passed the required checks. Please review the remaining device acceptance boundary before installation.",
                   "The recovery package has been verified. The destination and retention period still need your decision.",
                   "The current notification behavior is preserved. The proposed change is ready for review.", "",
                   "The command list now groups related steps and keeps full command text available for inspection.",
                   "Lifecycle cancellation is covered. Writes remain serialized until the previous transport settles.",
                   "Usage keeps wallet balances separate from quota windows and preserves decimal precision.", ""]
    var matters: [[String: Any]] = []
    var conversations: [[String: Any]] = []
    for i in 0..<8 {
        matters.append(["id": "goal-\(i)", "title": titles[i], "state": i < 4 ? "needs-input" : "open",
                        "created_at": now - 3600, "updated_at": now - Int64(i * 60), "revision": i + 1, "participants": []])
    }
    for i in 4...5 {
        let identity: [String: Any] = ["agent": i == 4 ? "codex" : "claude", "home_id": "fixture-home",
            "session_id": "session-\(i)", "name": zh ? "执行会话 \(i)" : "Worker \(i)", "pane": "%\(i)"]
        matters[i]["participants"] = [["id": "association-\(i)", "conversation": identity,
                                       "role": "execution", "from": now - 3600]]
        var candidate = identity
        candidate["status"] = i == 4 ? "working" : "idle"
        candidate["since"] = now - 60
        conversations.append(candidate)
    }
    for i in 8..<40 {
        matters.append(["id": "goal-\(i)", "title": zh ? "归档事项 \(i)" : "Archived matter \(i)", "state": "completed",
                        "created_at": now - 7200, "updated_at": now - 600, "revision": i + 1, "participants": []])
    }
    func item(_ i: Int) -> [String: Any] {
        ["matter_id": "goal-\(i)", "preview": ["message_id": "report-\(i)", "kind": "progress", "body": reports[i], "at": now - Int64(i * 60)]]
    }
    let value: [String: Any] = ["schema": 1, "revision": 40, "matters": matters, "conversations": conversations,
        "overview": ["generated_at": now, "since": now - 86400, "needs_input": (0..<3).map(item), "following": (4..<7).map(item),
                     "recent": [["seq": 41, "at": now - 600, "matter_id": "goal-8", "kind": "update", "state": "completed"],
                                ["seq": 42, "at": now - 900, "matter_id": "goal-1", "kind": "hq-decision"]], "recent_total": 2]]
    let decoder = JSONDecoder()
    decoder.keyDecodingStrategy = .convertFromSnakeCase
    return try decoder.decode(HQWorkspaceSnapshot.self, from: JSONSerialization.data(withJSONObject: value))
}
private struct OverviewFixtureClient: HQWorkspaceServing {
    var value: HQWorkspaceSnapshot
    func snapshot() throws -> HQWorkspaceSnapshot { value }
    func detail(_ scope: String, before: UInt64?, earlier: Int) throws -> HQWorkspaceDetail {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        return try decoder.decode(HQWorkspaceDetail.self, from: Data(#"{"schema":1,"messages":[],"turns":[],"earlier_available":false,"changes":[],"events":[],"warnings":[]}"#.utf8))
    }
    func participant(_ matter: String, association: String) throws -> HQWorkspaceDetail {
        try detail(matter, before: nil, earlier: 0)
    }
    func act(_ action: HQWorkspaceAction) throws { XCTFail("Overview must never write") }
    func drain() throws { XCTFail("Overview must never deliver") }
}

@MainActor
final class HQOverviewTests: XCTestCase {
    func testExplicitProjectionAndIdentityDoNotInferTaskCompletionOrPaneOwnership() throws {
        var snapshot = try overviewFixture()
        XCTAssertEqual(HQOverviewModel.items(.needsInput, in: snapshot).count, 3)
        XCTAssertEqual(HQOverviewModel.items(.following, in: snapshot).count, 3)
        let c = HQConversation(agent: "codex", homeId: "home-one", sessionId: "session-a", name: "worker", pane: "%4")
        snapshot.matters[0].participants = [HQParticipant(id: "association", conversation: c, role: "execution", from: 1)]
        var other = c
        other.homeId = "home-two"
        snapshot.conversations = [HQCandidate(agent: other.agent, homeId: other.homeId, sessionId: other.sessionId,
                                             name: "different conversation", pane: "%4", status: "working", since: 1)]
        XCTAssertTrue(HQOverviewModel.workers(snapshot.matters[0], in: snapshot).isEmpty)
        snapshot.conversations[0].homeId = "home-one"
        snapshot.conversations[0].status = "idle"
        XCTAssertEqual(HQOverviewModel.workers(snapshot.matters[0], in: snapshot).first?.status, "idle")
        XCTAssertEqual(snapshot.matters[0].state, "needs-input")
        snapshot.matters[0].participants[0].until = 2
        XCTAssertTrue(HQOverviewModel.workers(snapshot.matters[0], in: snapshot).isEmpty)
        let text = HQOverviewModel.snippet("**Full report** [source](https://example.invalid)")
        XCTAssertEqual(String(text.characters), "Full report source")
        XCTAssertTrue(text.runs.allSatisfy { $0.link == nil })
    }

    func testNativeBriefRendersBoundedRowsAndOpensSourceWithoutSending() async throws {
        NativeUIProbe.prepareAccessibility()
        let previous = L10n.shared.mode
        defer { L10n.shared.mode = previous }
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            let snapshot = try overviewFixture(zh: mode == .zh)
            for scheme in [ColorScheme.light, .dark] {
                for width in [CGFloat(1000), 620] {
                    let store = HQWorkspaceStore(client: OverviewFixtureClient(value: snapshot), resources: HQResourceStore(read: { _ in nil }))
                    store.refresh()
                    for _ in 0..<200 {
                        if !store.loading { break }
                        try await Task.sleep(nanoseconds: 5_000_000)
                    }
                    XCTAssertNotNil(store.snapshot?.overview)
                    let host = NSHostingView(rootView: HQWorkspaceOverview(l10n: .shared, store: store)
                        .environment(\.colorScheme, scheme).font(HQTypography.body)
                        .background(Color(nsColor: .windowBackgroundColor)))
                    let window = NSWindow(contentRect: NSRect(x: 100, y: 100, width: width, height: width >= 780 ? 700 : 900),
                                          styleMask: [.borderless], backing: .buffered, defer: false)
                    window.isReleasedWhenClosed = false
                    window.appearance = NSAppearance(named: scheme == .light ? .aqua : .darkAqua)
                    window.contentView = host
                    window.orderFront(nil)
                    defer { window.close(); store.stop() }
                    try await Task.sleep(nanoseconds: 100_000_000)
                    host.layoutSubtreeIfNeeded()
                    let first = try XCTUnwrap(NativeUIProbe.find("hq-overview-matter-goal-0", in: host))
                    XCTAssertNotNil(NativeUIProbe.find("hq-overview-matter-goal-6", in: host))
                    XCTAssertNil(NativeUIProbe.find("hq-overview-matter-goal-3", in: host), "More than three decision cards were rendered")
                    XCTAssertNil(NativeUIProbe.find("hq-overview-matter-goal-39", in: host), "Full history was repeated in Overview")
                    XCTAssertNotNil(NativeUIProbe.find("hq-overview-all-needsInput", in: host))
                    XCTAssertLessThanOrEqual(first.accessibilityFrame().width, width - 40)
                    if let path = ProcessInfo.processInfo.environment["GTMUX_WORKSPACE_PREVIEW_DIR"] {
                        let bitmap = try XCTUnwrap(host.bitmapImageRepForCachingDisplay(in: host.bounds))
                        window.appearance?.performAsCurrentDrawingAppearance { host.cacheDisplay(in: host.bounds, to: bitmap) }
                        let name = "hq-overview-\(mode.rawValue)-\(scheme == .light ? "light" : "dark")-\(Int(width)).png"
                        try XCTUnwrap(bitmap.representation(using: .png, properties: [:])).write(to: URL(fileURLWithPath: path).appendingPathComponent(name))
                    }
                    if mode == .en && scheme == .light && width == 1000 {
                        let all = try XCTUnwrap(NativeUIProbe.find("hq-overview-all-needsInput", in: host))
                        XCTAssertTrue(all.accessibilityPerformPress())
                        for _ in 0..<50 {
                            if !window.sheets.isEmpty { break }
                            try await Task.sleep(nanoseconds: 10_000_000)
                        }
                        let sheet = try XCTUnwrap(window.sheets.first)
                        let fourth = try XCTUnwrap(NativeUIProbe.find("hq-overview-picker-goal-3", in: sheet))
                        XCTAssertTrue(fourth.accessibilityPerformPress())
                        XCTAssertEqual(store.selection, .matter("goal-3"))
                        try await Task.sleep(nanoseconds: 100_000_000)
                    }
                    XCTAssertTrue(first.accessibilityPerformPress())
                    XCTAssertEqual(store.selection, .matter("goal-0"))
                }
            }
        }
    }

    func testClientRequestsTheBoundedCoreProjection() throws {
        var args: [String] = []
        let client = HQWorkspaceClient(execute: { a, _ in
            args = a
            return (0, #"{"schema":1,"revision":0,"matters":[],"conversations":[]}"#, "")
        })
        XCTAssertNil(try client.snapshot().overview)
        XCTAssertEqual(args, ["hq", "workspace", "list", "--overview", "--json"])
    }
}
