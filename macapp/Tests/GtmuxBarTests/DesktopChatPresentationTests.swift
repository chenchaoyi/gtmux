import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

@MainActor final class DesktopChatPresentationTests: XCTestCase {
    private func fixture() throws -> DesktopChatTurn {
        let step: [String: Any] = ["title": "functions.exec_command", "operation": "command", "input": "printf 'first line\\n'\n" + String(repeating: "echo inspect-long-command && ", count: 35), "input_kind": "command", "workdir": "/example/project", "details_truncated": true]
        let object: [String: Any] = ["prompt": "Review the public source and report the result.", "response": "Legacy text", "time": "2026-10-22T01:00:00.120Z", "from": ["kind": "hq", "label": "HQ", "agent": "codex"], "segments": [["text": "### Checking the source\n\nRead the configuration and run the focused checks.", "steps": [step]], ["text": "The recorded changes are ready to review.\n\n- Titles retain their session identity.\n- Input permissions are unchanged."]]]
        return try JSONDecoder().decode(DesktopChatTurn.self, from: JSONSerialization.data(withJSONObject: object))
    }
    func testCoreAttributionAndStructuredEvidenceSurviveDecoding() throws {
        let turn = try fixture()
        XCTAssertFalse(turn.isHuman)
        XCTAssertEqual(turn.senderLabel(.shared), "gtmux HQ")
        XCTAssertTrue(turn.fullReply.contains("Checking the source"))
        XCTAssertTrue(turn.fullReply.contains("Input permissions are unchanged"))
        XCTAssertFalse(turn.fullReply.contains("Legacy text"))
        XCTAssertTrue(turn.preview.hasPrefix("Checking the source"))
        XCTAssertFalse(turn.preview.contains("###"))
        let step = try XCTUnwrap(turn.segments?.first?.steps?.first)
        XCTAssertEqual(step.action, "command")
        XCTAssertEqual(step.workdir, "/example/project")
        XCTAssertEqual(step.detailsTruncated, true)
        XCTAssertTrue(step.recordedInput.contains("\n"))
        XCTAssertTrue(step.hasDetails)
        XCTAssertLessThanOrEqual(step.preview(.shared).count, 160)
        var human = turn; human.from = nil
        XCTAssertTrue(human.isHuman)
        XCTAssertEqual(turn.identity(index: 0), turn.identity(index: 12), "source time preserves fold identity across positional changes")
    }
    func testParametersDescribeRequestedWorkWithoutInventingSuccessOrDuration() throws {
        let json = #"[{"title":"unknown","operation":"agent","input_kind":"parameters","input":"{\"description\":\"Review the adapter\",\"message\":\"Full instructions\"}"},{"title":"wait_agent","input_kind":"parameters","input":"{\"timeout_ms\":2500}"},{"title":"wait_agent","input_kind":"parameters","input":"{\"timeout_ms\":true}"},{"title":"vendor_tool"}]"#
        let steps = try JSONDecoder().decode([DesktopChatStep].self, from: Data(json.utf8))
        XCTAssertEqual(steps[0].action, "agent")
        XCTAssertEqual(steps[0].preview(.shared), "Review the adapter")
        XCTAssertTrue(steps[1].preview(.shared).contains("2.5"))
        XCTAssertFalse(steps[2].preview(.shared).contains("0.001"))
        XCTAssertEqual(steps[3].actionLabel(.shared), "vendor_tool")
        XCTAssertFalse(steps[3].hasDetails)
    }
    private func settle(_ host: NSView) async throws {
        try await Task.sleep(nanoseconds: 180_000_000)
        host.layoutSubtreeIfNeeded()
    }
    private func snapshot(_ host: NSView, _ name: String) throws {
        guard let dir = ProcessInfo.processInfo.environment["GTMUX_DESKTOP_SNAPSHOT_DIR"]
            ?? ProcessInfo.processInfo.environment["GTMUX_WORKSPACE_PREVIEW_DIR"] else { return }
        let bitmap = try XCTUnwrap(host.bitmapImageRepForCachingDisplay(in: host.bounds))
        host.effectiveAppearance.performAsCurrentDrawingAppearance { host.cacheDisplay(in: host.bounds, to: bitmap) }
        try XCTUnwrap(bitmap.representation(using: .png, properties: [:])).write(to: URL(fileURLWithPath: dir).appendingPathComponent(name + ".png"))
    }
    func testNativeReaderFoldingAttributionAndToolInspectorAcrossLayouts() async throws {
        NativeUIProbe.prepareAccessibility()
        let old = L10n.shared.mode; defer { L10n.shared.mode = old }
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            for dark in [false, true] {
                for width in [CGFloat(520), 840] {
                    var fixtureAgent = Agent(agent: "DeepSeek Harness", task: "Synthetic review", source: "native", client: "unconfirmed", sessionID: "synthetic")
                    let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
                    fixtureAgent.icon = root.appendingPathComponent("assets/agent-icons/dsh.png").path
                    let model = DesktopConversationModel(agent: fixtureAgent, capture: { _ in XCTFail("fixture must not read live data"); return (1,"","") })
                    model.loading = false; model.turns = [try fixture()]
                    let host = NSHostingView(rootView: DesktopConversationView(model: model, l10n: .shared).environment(\.colorScheme, dark ? .dark : .light))
                    let window = NSWindow(contentRect: NSRect(x: 80, y: 80, width: width, height: 720), styleMask: [.titled, .closable], backing: .buffered, defer: false)
                    window.isReleasedWhenClosed = false; window.title = "gtmux synthetic conversation preview"
                    window.appearance = NSAppearance(named: dark ? .darkAqua : .aqua); window.contentView = host; window.orderBack(nil)
                    defer { window.close(); model.stop() }
                    try await settle(host)
                    XCTAssertEqual(NativeUIProbe.find("desktop-prompt-sender", in: host)?.accessibilityValue() as? String, "gtmux HQ")
                    let fold = try XCTUnwrap(NativeUIProbe.find("desktop-fold-all", in: host))
                    XCTAssertTrue(fold.accessibilityPerformPress()); try await settle(host)
                    XCTAssertNil(NativeUIProbe.find("desktop-tool-group", in: host))
                    XCTAssertNotNil(NativeUIProbe.find("desktop-copy-reply", in: host), "copy remains available for the full folded reply")
                    XCTAssertTrue(try XCTUnwrap(NativeUIProbe.find("desktop-fold-all", in: host)).accessibilityPerformPress()); try await settle(host)
                    XCTAssertTrue(try XCTUnwrap(NativeUIProbe.find("desktop-tool-group", in: host)).accessibilityPerformPress()); try await settle(host)
                    XCTAssertTrue(try XCTUnwrap(NativeUIProbe.find("desktop-tool-0", in: host)).accessibilityPerformPress()); try await settle(host)
                    let inspector = try XCTUnwrap(NativeUIProbe.find("desktop-tool-input", in: host))
                    XCTAssertGreaterThan(inspector.accessibilityFrame().height, 20)
                    XCTAssertLessThanOrEqual(inspector.accessibilityFrame().width, width - 40)
                    let name = "reader-\(mode.rawValue)-\(dark ? "dark" : "light")-\(Int(width))"
                    try snapshot(host, name)
                    XCTAssertTrue(try XCTUnwrap(NativeUIProbe.find("desktop-reply-fold", in: host)).accessibilityPerformPress()); try await settle(host)
                    XCTAssertNil(NativeUIProbe.find("desktop-tool-input", in: host))
                    try snapshot(host, name + "-folded")
                }
            }
        }
    }
    func testRefreshPreservesHistoricalReadingAndExplicitLatestReturnsToTail() async throws {
        NativeUIProbe.prepareAccessibility()
        let model = DesktopConversationModel(agent: Agent(agent: "Codex", source: "native", sessionID: "history"))
        model.loading = false
        model.turns = (0..<12).map { index in
            DesktopChatTurn(prompt: "Synthetic request \(index)", response: String(repeating: "Recorded public reply stays in this synthetic fixture. ", count: 45), segments: nil, time: "2026-10-22T01:\(String(format: "%02d", index)):00Z")
        }
        var navigation: ScrollViewProxy?
        var tailIsVisible = false
        var entrySettled = false
        let host = NSHostingView(rootView: DesktopConversationView(model: model, l10n: .shared, scrollObserver: { proxy, visible, settled in
            navigation = proxy; tailIsVisible = visible; entrySettled = settled
        }))
        let window = NSWindow(contentRect: NSRect(x: 80, y: 80, width: 760, height: 640), styleMask: [.titled], backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false; window.contentView = host; window.orderFront(nil)
        defer { window.close(); model.stop() }
        for _ in 0..<100 {
            host.layoutSubtreeIfNeeded()
            if tailIsVisible && entrySettled { break }
            try await Task.sleep(nanoseconds: 20_000_000)
        }
        XCTAssertTrue(tailIsVisible, "Initial positioning must reveal the actual tail marker")
        XCTAssertTrue(entrySettled, "Initial navigation must finish before deliberate history reading")
        try snapshot(host, "reader-scroll-initial")
        let scroll = try XCTUnwrap(NativeUIProbe.views(NSScrollView.self, in: host).first)
        func viewport() throws -> NSBitmapImageRep {
            host.layoutSubtreeIfNeeded()
            let clip = scroll.contentView
            let rect = NSRect(x: clip.bounds.minX + 20, y: clip.bounds.minY + 20,
                width: clip.bounds.width - 60, height: min(300, clip.bounds.height - 40))
            let image = try XCTUnwrap(clip.bitmapImageRepForCachingDisplay(in: rect))
            clip.cacheDisplay(in: rect, to: image)
            return image
        }
        func png(_ image: NSBitmapImageRep) throws -> Data {
            try XCTUnwrap(image.representation(using: .png, properties: [:]))
        }
        func viewportHasContent() throws -> Bool {
            let bitmap = try viewport()
            let pixels = try XCTUnwrap(bitmap.bitmapData)
            return Set(Data(bytes: pixels, count: bitmap.bytesPerRow * bitmap.pixelsHigh)).count > 8
        }
        XCTAssertTrue(try viewportHasContent(), "Cold entry must show recorded content, not a blank viewport")
        NotificationCenter.default.post(name: NSScrollView.willStartLiveScrollNotification, object: scroll)
        // SwiftUI owns a virtual document. Navigate by record identity, as the HQ
        // harness does, and verify the actual viewport rather than estimated offsets.
        try XCTUnwrap(navigation).scrollTo(model.turns[0].identity(index: 0), anchor: .top)
        var before: Data?
        var stable = 0
        for _ in 0..<100 {
            let current = try png(viewport())
            stable = current == before ? stable + 1 : 0
            before = current
            if stable >= 5 { break }
            try await Task.sleep(nanoseconds: 20_000_000)
        }
        XCTAssertGreaterThanOrEqual(stable, 5, "Historical content must settle before refresh")
        XCTAssertFalse(tailIsVisible, "History navigation must actually leave the tail")
        try snapshot(host, "reader-scroll-history")
        XCTAssertTrue(try viewportHasContent(), "A blank viewport cannot prove reading-position preservation")
        model.turns.append(DesktopChatTurn(prompt: "A new synthetic request", response: "New public answer", segments: nil, time: "2026-10-22T02:00:00Z"))
        try await settle(host)
        var after: Data?
        for _ in 0..<100 {
            after = try png(viewport())
            if after == before { break }
            try await Task.sleep(nanoseconds: 20_000_000)
        }
        XCTAssertEqual(after, before, "Refresh must preserve the actual historical viewport")
        XCTAssertFalse(tailIsVisible, "An appended turn must not pull history to the tail")
        try snapshot(host, "reader-scroll-refreshed")
        let latest = try XCTUnwrap(NativeUIProbe.find("desktop-latest", in: host))
        XCTAssertTrue(latest.accessibilityPerformPress())
        for _ in 0..<100 {
            try await Task.sleep(nanoseconds: 20_000_000)
            host.layoutSubtreeIfNeeded()
            if tailIsVisible && entrySettled { break }
        }
        XCTAssertTrue(tailIsVisible, "Explicit latest must reveal the actual tail marker")
        XCTAssertTrue(entrySettled, "Explicit navigation must finish")
        XCTAssertTrue(try viewportHasContent(), "Explicit latest must show recorded content")
        try snapshot(host, "reader-scroll-latest")
    }

}
