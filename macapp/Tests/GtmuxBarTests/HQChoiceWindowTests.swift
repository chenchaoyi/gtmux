import AppKit
import SwiftUI
import XCTest

@testable import GtmuxBar

@MainActor final class HQChoiceWindowTests: XCTestCase {
    func testCompleteWindowPinsComposerAndNativeChoiceUpdatesAndFocusesEditor() async throws {
        NativeUIProbe.prepareAccessibility()
        for scheme in [ColorScheme.light, ColorScheme.dark] {
            for size in [NSSize(width: 900, height: 620), NSSize(width: 1440, height: 900)] {
                let fake = ChoiceFixture()
                fake.replace {
                    $0.matters[0].userInput?.why = String(
                        repeating: "Synthetic explanation. ", count: 90)
                }
                let store = try await HQChoiceTestSupport.load(fake)
                let hosting = NSHostingController(
                    rootView: HQWorkspaceView(l10n: .shared, store: store).environment(
                        \.colorScheme, scheme
                    )
                    .background(Color(nsColor: .windowBackgroundColor)))
                hosting.sizingOptions = []
                let host = hosting.view
                let window = NSWindow(
                    contentRect: NSRect(origin: .zero, size: size),
                    styleMask: [.titled, .resizable],
                    backing: .buffered, defer: false)
                window.appearance = NSAppearance(named: scheme == .dark ? .darkAqua : .aqua)
                window.isReleasedWhenClosed = false
                window.minSize = NSSize(width: 900, height: 480)
                window.contentMinSize = NSSize(width: 900, height: 440)
                window.contentViewController = hosting
                window.setContentSize(size)
                window.orderFront(nil)
                window.displayIfNeeded()
                defer {
                    window.close()
                    store.stop()
                }
                try await Task.sleep(nanoseconds: 200_000_000)
                host.layoutSubtreeIfNeeded()
                let note = try XCTUnwrap(NativeUIProbe.views(MessageTextView.self, in: host).first)
                let viewport = try XCTUnwrap(note.enclosingScrollView)
                let rect = viewport.convert(viewport.bounds, to: host)
                XCTAssertTrue(
                    host.bounds.insetBy(dx: -1, dy: -1).contains(rect),
                    "Composer outside full window: \(rect) / \(host.bounds)")
                window.makeFirstResponder(nil)
                if NativeUIProbe.find("hq-human-choice-0", in: host) == nil {
                    for scroll in NativeUIProbe.views(NSScrollView.self, in: host)
                    where scroll !== viewport {
                        if let document = scroll.documentView,
                            document.bounds.height > scroll.contentSize.height
                        {
                            scroll.contentView.scroll(
                                to: NSPoint(
                                    x: 0,
                                    y: max(0, document.bounds.height - scroll.contentSize.height)))
                            scroll.reflectScrolledClipView(scroll.contentView)
                        }
                    }
                    try await Task.sleep(nanoseconds: 100_000_000)
                }
                let option = try XCTUnwrap(NativeUIProbe.find("hq-human-choice-0", in: host))
                XCTAssertTrue(option.accessibilityPerformPress())
                try await HQChoiceTestSupport.waitUntil {
                    note.string == "Confirm the synthetic sample\nDone"
                        && window.firstResponder === note
                }
                XCTAssertEqual(note.string, "Confirm the synthetic sample\nDone")
                XCTAssertEqual(
                    NativeUIProbe.find("hq-human-choice-0", in: host)?.accessibilityValue()
                        as? String,
                    L10n.shared.tr("Prepared", "已填入输入框"))
                XCTAssertTrue(
                    window.firstResponder === note, "Prepared reply did not focus its native editor"
                )
                XCTAssertTrue(fake.actions.isEmpty)
                if let directory = ProcessInfo.processInfo.environment["GTMUX_HQ_TEST_OUTPUT"] {
                    let bitmap = try XCTUnwrap(
                        host.bitmapImageRepForCachingDisplay(in: host.bounds))
                    host.cacheDisplay(in: host.bounds, to: bitmap)
                    try XCTUnwrap(bitmap.representation(using: .png, properties: [:])).write(
                        to: URL(fileURLWithPath: directory).appendingPathComponent(
                            "hq-choice-\(Int(size.width))-\(scheme == .dark ? "dark":"light").png"))
                }
            }
        }
    }

    func testCurrentQuestionRemainsUsableWhileConversationReadWaits() async throws {
        NativeUIProbe.prepareAccessibility()
        let fake = ChoiceFixture()
        let gate = DispatchSemaphore(value: 0)
        fake.storedOnly = true
        let store = HQWorkspaceStore(client: fake, resources: HQResourceStore(read: { _ in nil }))
        store.refresh()
        try await HQChoiceTestSupport.waitUntil { !store.loading }
        fake.detailGate = gate
        store.select(.matter("choice-matter"))
        let hosting = NSHostingController(rootView: HQWorkspaceView(l10n: .shared, store: store))
        hosting.sizingOptions = []
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1100, height: 740), styleMask: [.titled],
            backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false
        window.contentViewController = hosting
        window.setContentSize(NSSize(width: 1100, height: 740))
        window.orderFront(nil)
        defer {
            gate.signal()
            window.close()
            store.stop()
        }
        let host = hosting.view
        try await HQChoiceTestSupport.waitUntil {
            NativeUIProbe.find("hq-human-choice-0", in: host) != nil
        }
        XCTAssertTrue(store.loading)
        XCTAssertEqual(store.readPhase, .content)
        let labels = NativeUIProbe.elements(host).compactMap { $0.accessibilityLabel() }
        XCTAssertFalse(labels.contains(L10n.shared.tr("No conversation yet.", "还没有对话。")))
        let note = try XCTUnwrap(NativeUIProbe.views(MessageTextView.self, in: host).first)
        window.makeFirstResponder(nil)
        XCTAssertTrue(
            try XCTUnwrap(NativeUIProbe.find("hq-human-choice-0", in: host))
                .accessibilityPerformPress())
        try await HQChoiceTestSupport.waitUntil {
            note.string == "Confirm the synthetic sample\nDone" && window.firstResponder === note
        }
        XCTAssertTrue(store.canSend)
        XCTAssertTrue(fake.actions.isEmpty)
        gate.signal()
        try await HQChoiceTestSupport.waitUntil { !store.loading }
    }

}
