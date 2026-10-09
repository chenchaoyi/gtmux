import XCTest
import AppKit
import SwiftUI
@testable import GtmuxBar

final class DesktopConversationTests: XCTestCase {
    private let agent = Agent(agent: "Codex", task: "Desktop work", source: "native", client: "chatgpt_desktop", sessionID: "desk")
    private let snapshot = #"{"turns":[{"prompt":"User request","response":"Intermediate reply","segments":[{"text":"Intermediate reply","steps":[{"title":"exec","detail":"ls"}]}]}],"dropped":2,"etag":"one"}"#
    @MainActor private func wait(_ predicate: () -> Bool) async throws {
        for _ in 0..<100 {
            if predicate() { return }
            try await Task.sleep(nanoseconds: 10_000_000)
        }
        XCTFail("Timed out waiting for read completion")
    }
    func testCLIOnlyReadsTheExactIdentity() {
        XCTAssertEqual(DesktopConversationModel.arguments(id: "desk", etag: "one"), ["transcript", "desk", "--json", "--etag", "one"])
    }
    @MainActor func testUpdatesUnchangedAndErrorsPreserveHistory() async throws {
        var calls = 0
        let content = snapshot
        let model = DesktopConversationModel(agent: agent, capture: { args in
            calls += 1
            if calls == 1 { return (0, content, "") }
            XCTAssertEqual(Array(args.suffix(2)), ["--etag", "one"])
            if calls == 2 { return (0, #"{"turns":null,"dropped":0,"unchanged":true,"etag":"one"}"#, "") }
            return (1, "", "offline")
        })
        defer { model.stop() }
        model.start(); try await wait { !model.loading }
        XCTAssertEqual(model.turns.first?.response, "Intermediate reply")
        XCTAssertEqual(model.dropped, 2)
        model.refresh(); try await Task.sleep(nanoseconds: 50_000_000)
        XCTAssertFalse(model.failed)
        XCTAssertEqual(model.dropped, 2)
        model.refresh(); try await wait { model.failed }
        XCTAssertEqual(model.turns.first?.response, "Intermediate reply")
    }
    @MainActor func testSerialReadsAndStopIgnoreLateResult() async throws {
        let began = expectation(description: "read began")
        let release = DispatchSemaphore(value: 0)
        let lock = NSLock()
        var calls = 0
        let content = snapshot
        let model = DesktopConversationModel(agent: agent, capture: { _ in
            lock.lock(); calls += 1; let n = calls; lock.unlock()
            if n == 1 { began.fulfill(); _ = release.wait(timeout: .now() + 3) }
            return (0, content, "")
        })
        defer { model.stop() }
        model.start(); await fulfillment(of: [began], timeout: 1)
        model.refresh(); model.refresh()
        XCTAssertEqual(lock.withLock { calls }, 1)
        model.stop(); release.signal()
        try await Task.sleep(nanoseconds: 50_000_000)
        XCTAssertTrue(model.turns.isEmpty)
        model.start(); try await wait { !model.turns.isEmpty }
        XCTAssertEqual(lock.withLock { calls }, 2)
    }
    @MainActor func testInactiveReaderAndBilingualLayout() throws {
        let model = DesktopConversationModel(agent: agent, active: { false }, capture: { _ in XCTFail("Inactive read"); return (1, "", "") })
        model.start(); model.refresh(); model.stop()
        let old = L10n.shared.mode; defer { L10n.shared.mode = old }
        model.loading = false
        model.turns = try JSONDecoder().decode(DesktopChatSnapshot.self, from: Data(snapshot.utf8)).turns ?? []
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            for dark in [false, true] {
                let host = NSHostingView(rootView: DesktopConversationView(model: model, l10n: .shared).environment(\.colorScheme, dark ? .dark : .light))
                let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 760, height: 640), styleMask: [.titled], backing: .buffered, defer: false)
                window.isReleasedWhenClosed = false; window.contentView = host
                host.layoutSubtreeIfNeeded()
                XCTAssertEqual(host.bounds.width, 760, accuracy: 1)
                XCTAssertEqual(host.bounds.height, 640, accuracy: 1)
                if let dir = ProcessInfo.processInfo.environment["GTMUX_DESKTOP_SNAPSHOT_DIR"], let rep = host.bitmapImageRepForCachingDisplay(in: host.bounds) {
                    host.cacheDisplay(in: host.bounds, to: rep)
                    try rep.representation(using: .png, properties: [:])?.write(to: URL(fileURLWithPath: dir).appendingPathComponent("desktop-\(mode == .zh ? "zh" : "en")-\(dark ? "dark" : "light").png"))
                }
                window.close()
            }
        }
    }
}
