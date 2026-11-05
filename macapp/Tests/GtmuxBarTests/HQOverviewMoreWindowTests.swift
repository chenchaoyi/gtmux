import AppKit
import SwiftUI
import XCTest

@testable import GtmuxBar

private struct ClarityClient: HQWorkspaceServing {
    let value: HQWorkspaceSnapshot
    func snapshot() throws -> HQWorkspaceSnapshot { value }
    func detail(_ scope: String, before: UInt64?, earlier: Int) throws -> HQWorkspaceDetail {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        return try decoder.decode(
            HQWorkspaceDetail.self,
            from: Data(
                #"{"schema":1,"messages":[],"turns":[],"earlier_available":false,"changes":[],"events":[],"warnings":[]}"#
                    .utf8))
    }
    func participant(_ matter: String, association: String) throws -> HQWorkspaceDetail {
        try detail(matter, before: nil, earlier: 0)
    }
    func act(_ action: HQWorkspaceAction) throws { XCTFail("Overview selection must not write") }
    func drain() throws { XCTFail("Overview selection must not deliver") }
}

@MainActor final class HQOverviewMoreWindowTests: XCTestCase {
    func testBottomRemainingControlOpensAllAndSelectsHiddenGoalWithoutSending() async throws {
        NativeUIProbe.prepareAccessibility()
        let previous = L10n.shared.mode
        defer { L10n.shared.mode = previous }
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            for scheme in [ColorScheme.light, .dark] {
                for width in [CGFloat(900), 1440] {
                    let snapshot = try overviewClaritySnapshot()
                    let store = HQWorkspaceStore(
                        client: ClarityClient(value: snapshot), resources: HQResourceStore(read: { _ in nil })
                    )
                    store.refresh()
                    try await HQChoiceTestSupport.waitUntil { !store.loading }
                    let controller = NSHostingController(
                        rootView: HQWorkspaceOverview(l10n: .shared, store: store)
                            .environment(\.colorScheme, scheme).font(HQTypography.body)
                            .background(Color(nsColor: .windowBackgroundColor)))
                    controller.sizingOptions = []
                    let window = NSWindow(
                        contentRect: NSRect(x: 0, y: 0, width: width, height: 900),
                        styleMask: [.titled, .resizable], backing: .buffered, defer: false)
                    window.isReleasedWhenClosed = false
                    window.appearance = NSAppearance(named: scheme == .dark ? .darkAqua : .aqua)
                    window.contentViewController = controller
                    window.setContentSize(NSSize(width: width, height: 900))
                    window.orderFront(nil)
                    defer {
                        window.close()
                        store.stop()
                    }
                    try await Task.sleep(nanoseconds: 150_000_000)
                    let host = controller.view
                    host.layoutSubtreeIfNeeded()
                    let more = try XCTUnwrap(NativeUIProbe.find("hq-overview-all-following", in: host))
                    let last = try XCTUnwrap(NativeUIProbe.find("hq-overview-matter-open-2", in: host))
                    XCTAssertTrue(more.accessibilityLabel()?.contains("23") == true)
                    XCTAssertLessThanOrEqual(
                        more.accessibilityFrame().maxY, last.accessibilityFrame().minY + 1,
                        "The remaining action must follow the last card, not sit in the header")
                    XCTAssertNil(NativeUIProbe.find("hq-overview-matter-completed", in: host))
                    XCTAssertNil(NativeUIProbe.find("hq-overview-matter-merged", in: host))
                    if let directory = ProcessInfo.processInfo.environment["GTMUX_HQ_TEST_OUTPUT"] {
                        let bitmap = try XCTUnwrap(host.bitmapImageRepForCachingDisplay(in: host.bounds))
                        host.cacheDisplay(in: host.bounds, to: bitmap)
                        try XCTUnwrap(bitmap.representation(using: .png, properties: [:])).write(
                            to: URL(fileURLWithPath: directory)
                                .appendingPathComponent(
                                    "hq-overview-\(mode.rawValue)-\(scheme == .dark ? "dark" : "light")-\(Int(width)).png"
                                ))
                    }
                    XCTAssertTrue(more.accessibilityPerformPress())
                    try await HQChoiceTestSupport.waitUntil { !window.sheets.isEmpty }
                    let sheet = try XCTUnwrap(window.sheets.first)
                    let sheetContent = try XCTUnwrap(sheet.contentView)
                    for scroll in NativeUIProbe.views(NSScrollView.self, in: sheetContent) {
                        if let doc = scroll.documentView {
                            scroll.contentView.scroll(
                                to: NSPoint(x: 0, y: max(0, doc.bounds.height - scroll.contentSize.height)))
                            scroll.reflectScrolledClipView(scroll.contentView)
                        }
                    }
                    try await Task.sleep(nanoseconds: 100_000_000)
                    let hidden = try XCTUnwrap(NativeUIProbe.find("hq-overview-picker-open-25", in: sheet))
                    XCTAssertTrue(hidden.accessibilityPerformPress())
                    XCTAssertEqual(store.selection, .matter("open-25"))
                    try await Task.sleep(nanoseconds: 100_000_000)
                }
            }
        }
    }
    func testFullyShownListHasNoRemainingControlAndUnknownRosterDoesNotSayIdle() async throws {
        NativeUIProbe.prepareAccessibility()
        var snapshot = try overviewClaritySnapshot(count: 2)
        snapshot.sourcesIncluded = false
        let conversation = HQConversation(
            agent: "codex", homeId: "synthetic", sessionId: "same-session", name: "Linked worker")
        snapshot.matters[0].participants = [
            HQParticipant(id: "synthetic-association", conversation: conversation, role: "execution", from: 1)
        ]
        snapshot.conversations = [
            HQCandidate(
                agent: "codex", homeId: "synthetic", sessionId: "same-session", name: "Linked worker",
                status: "idle", since: 1)
        ]
        let store = HQWorkspaceStore(
            client: ClarityClient(value: snapshot), resources: HQResourceStore(read: { _ in nil }))
        store.refresh()
        try await HQChoiceTestSupport.waitUntil { !store.loading }
        let host = NSHostingView(rootView: HQWorkspaceOverview(l10n: .shared, store: store))
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1000, height: 900), styleMask: [.titled],
            backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false
        window.contentView = host
        window.orderFront(nil)
        defer {
            window.close()
            store.stop()
        }
        try await HQChoiceTestSupport.waitUntil {
            NativeUIProbe.find("hq-overview-matter-open-0", in: host) != nil
        }
        XCTAssertNil(NativeUIProbe.find("hq-overview-all-following", in: host))
        XCTAssertFalse(store.sourcesKnown)
        XCTAssertTrue(HQMatterGroup.needsInput.matters(in: store.snapshot).isEmpty)
        let row = try XCTUnwrap(NativeUIProbe.find("hq-overview-matter-open-0", in: host))
        let value = try XCTUnwrap(row.accessibilityValue() as? String)
        XCTAssertTrue(value.contains(L10n.shared.tr("Session status unavailable", "会话状态待刷新")))
        XCTAssertFalse(value.contains("Linked worker"))
    }
}
