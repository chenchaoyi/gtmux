import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

@MainActor final class ScreenshotDeliveryTests: XCTestCase {
    private func document() throws -> ScreenshotDocument {
        let context = try XCTUnwrap(CGContext(data: nil, width: 800, height: 500, bitsPerComponent: 8,
            bytesPerRow: 0, space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
        context.setFillColor(NSColor.lightGray.cgColor)
        context.fill(CGRect(x: 0, y: 0, width: 800, height: 500))
        return ScreenshotDocument(image: try XCTUnwrap(context.makeImage()), pointSize: CGSize(width: 800, height: 500))
    }
    private func flush(_ window: NSWindow) async throws {
        try await Task.sleep(nanoseconds: 80_000_000)
        window.contentView?.layoutSubtreeIfNeeded()
    }
    func testDeliveryLocksNativeEditorAndFailureRestoresWork() async throws {
        NativeUIProbe.prepareAccessibility()
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        let controller = ScreenshotEditorController()
        let pasteboard = NSPasteboard(name: .init("gtmux-delivery-\(UUID().uuidString)"))
        controller.pasteboard = pasteboard
        defer { pasteboard.releaseGlobally() }
        var sends: [(String, String, String)] = []
        var finish: ((ScreenshotSendResult) -> Void)?
        controller.deliver = { pane, session, note, _, reply in
            sends.append((pane, session, note)); finish = reply
        }
        controller.rememberTarget = { _ in XCTFail("A failed delivery cannot become the default target") }
        controller.show(doc: try document(), captureFile: folder.appendingPathComponent("capture.png"), target: "%fixture",
                        store: AgentStore(), l10n: .shared)
        let window = try XCTUnwrap(controller.window)
        defer { window.delegate = nil; window.close() }
        let model = try XCTUnwrap(controller.model)
        model.note = "**Keep my draft**"
        let mark = Annotation(.rect(CGRect(x: 10, y: 10, width: 100, height: 60)), color: .red)
        model.doc.add(mark)
        try await flush(window)
        let canvas = try XCTUnwrap(ScreenshotEditorController.canvas(in: window.contentView))
        let note = try XCTUnwrap(NativeUIProbe.views(MessageTextView.self, in: try XCTUnwrap(window.contentView)).first)
        let canvasFrame = canvas.convert(canvas.bounds, to: nil)
        window.makeFirstResponder(note)
        controller.send()
        XCTAssertEqual(sends.count, 1)
        XCTAssertEqual(sends[0].0, "%fixture")
        XCTAssertEqual(sends[0].2, "**Keep my draft**")
        try await flush(window)
        XCTAssertTrue(model.editingBlocked)
        XCTAssertFalse(note.isEditable)
        XCTAssertFalse(note.isSelectable)
        XCTAssertFalse(canvas.acceptsFirstResponder)
        XCTAssertNil(window.undoManager, "The Edit menu must not consume annotation undo while sending")
        XCTAssertFalse(try XCTUnwrap(window.standardWindowButton(.closeButton)).isEnabled)
        let overlay = try XCTUnwrap(NativeUIProbe.find("screenshot-delivery-overlay", in: window))
        XCTAssertGreaterThan(overlay.accessibilityFrame().width, canvasFrame.width)
        XCTAssertGreaterThan(overlay.accessibilityFrame().height, canvasFrame.height)
        XCTAssertTrue(overlay.accessibilityLabel()?.contains("%fixture") == true)
        for id in [ScreenshotEditorController.copyItem, ScreenshotEditorController.saveItem] {
            XCTAssertFalse(try XCTUnwrap(window.toolbar?.items.first { $0.itemIdentifier == id }?.view as? NSButton).isEnabled)
        }
        var saves = 0
        controller.saveForTesting = { saves += 1 }
        controller.copyImage(); controller.saveImage(); controller.send()
        XCTAssertFalse(model.copy(to: pasteboard))
        XCTAssertFalse(model.write(to: folder.appendingPathComponent("must-not-exist.png")))
        model.doc.remove(mark)
        canvas.drag(from: CGPoint(x: 0, y: 0), to: CGPoint(x: 80, y: 80), tool: .rect, color: .blue)
        canvas.finish(from: CGPoint(x: 0, y: 0), to: CGPoint(x: 80, y: 80), tool: .rect, color: .blue)
        let key = try XCTUnwrap(NSEvent.keyEvent(with: .keyDown, location: .zero, modifierFlags: [.command], timestamp: 0,
            windowNumber: window.windowNumber, context: nil, characters: "z", charactersIgnoringModifiers: "z", isARepeat: false, keyCode: 6))
        XCTAssertTrue(window.performKeyEquivalent(with: key))
        XCTAssertEqual(model.doc.items, [mark])
        XCTAssertEqual(sends.count, 1)
        XCTAssertEqual(saves, 0)
        XCTAssertNil(pasteboard.data(forType: .png))
        XCTAssertEqual(model.status, .sending, "Copy must not overwrite delivery status")
        // A broken state guard could open the real discard sheet. The state
        // assertion above fails that mutation without leaving an interactive test.
        if model.sending { XCTAssertFalse(controller.windowShouldClose(window)) }
        XCTAssertEqual(canvas.convert(canvas.bounds, to: nil), canvasFrame)
        finish?(.failed("fixture transport error"))
        try await flush(window)
        XCTAssertFalse(model.editingBlocked)
        XCTAssertTrue(note.isEditable)
        XCTAssertEqual(note.string, "**Keep my draft**")
        XCTAssertEqual(model.doc.items, [mark])
        XCTAssertNil(NativeUIProbe.find("screenshot-delivery-overlay", in: window))
        XCTAssertTrue(canvas.acceptsFirstResponder)
        XCTAssertTrue(try XCTUnwrap(window.standardWindowButton(.closeButton)).isEnabled)
        XCTAssertTrue(try XCTUnwrap(window.undoManager).canUndo)
        window.undoManager?.undo()
        XCTAssertTrue(model.doc.items.isEmpty, "The blocked shortcut must preserve undo for failure recovery")
        XCTAssertEqual(canvas.convert(canvas.bounds, to: nil), canvasFrame)
    }

    func testSuccessKeepsTheCapturedDestinationAndCannotCloseANewerEditor() async throws {
        NativeUIProbe.prepareAccessibility()
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        let controller = ScreenshotEditorController()
        var finish: ((ScreenshotSendResult) -> Void)?
        var close: (() -> Void)?
        controller.deliver = { _, _, _, _, reply in finish = reply }
        controller.rememberTarget = { _ in }
        controller.scheduleDeliveryClose = { delay, action in XCTAssertEqual(delay, 1.2); close = action }
        controller.show(doc: try document(), captureFile: folder.appendingPathComponent("capture.png"), target: "%original",
                        store: AgentStore(), l10n: .shared)
        let first = try XCTUnwrap(controller.window)
        let model = try XCTUnwrap(controller.model)
        controller.send()
        model.targetID = "%changed"
        finish?(.delivered(queued: true))
        try await flush(first)
        XCTAssertTrue(model.editingBlocked)
        XCTAssertFalse(model.canSend)
        XCTAssertEqual(model.sendingTarget?.paneID, "%original")
        let overlay = try XCTUnwrap(NativeUIProbe.find("screenshot-delivery-overlay", in: first))
        XCTAssertTrue(overlay.accessibilityLabel()?.contains("%original") == true)
        XCTAssertFalse(overlay.accessibilityLabel()?.contains("%changed") == true)
        XCTAssertNotNil(close)
        controller.show(doc: try document(), captureFile: folder.appendingPathComponent("next.png"), target: nil,
                        store: AgentStore(), l10n: .shared)
        let next = try XCTUnwrap(controller.window)
        defer { next.delegate = nil; next.close() }
        close?()
        XCTAssertTrue(controller.isCurrent(next), "The old receipt must not dismiss a newer screenshot")
    }

    func testOnlyConfirmedSuccessBlocksEditingAfterTheReceipt() throws {
        let model = ScreenshotEditorModel(doc: try document(), captureFile: URL(fileURLWithPath: "/unused"), target: "%test")
        let results: [ScreenshotSendResult] = [.refusedWaiting, .heldAfterPaste, .paneGone, .agentsUnreadable,
            .refusedDraft("draft"), .duplicate, .notConfirmed("unknown"), .failed("failure")]
        for result in results {
            model.status = .sending
            model.status = .result(result)
            XCTAssertFalse(model.editingBlocked, "\(result) must preserve a recoverable editor")
            XCTAssertTrue(model.doc.editingEnabled)
        }
        for queued in [false, true] {
            model.status = .result(.delivered(queued: queued))
            XCTAssertTrue(model.editingBlocked)
            XCTAssertFalse(model.doc.editingEnabled)
        }
    }

    func testDeliveryOverlayFitsBothLanguagesAndAppearances() throws {
        let language = L10n.shared.mode
        defer { L10n.shared.mode = language }
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            for scheme in [ColorScheme.light, .dark] {
                let root = ScreenshotDeliveryOverlay(status: .sending,
                    target: .init(paneID: "%123", session: "gtmux dev", name: "Codex"), l10n: .shared)
                    .environment(\.colorScheme, scheme)
                let host = NSHostingView(rootView: root)
                let window = NSWindow(contentRect: NSRect(x: 100, y: 100, width: 640, height: 420),
                                      styleMask: [.borderless], backing: .buffered, defer: false)
                window.isReleasedWhenClosed = false
                window.contentView = host
                window.orderFront(nil)
                host.layoutSubtreeIfNeeded()
                XCTAssertLessThanOrEqual(host.fittingSize.width, 640)
                XCTAssertLessThanOrEqual(host.fittingSize.height, 420)
                if let path = ProcessInfo.processInfo.environment["GTMUX_WORKSPACE_PREVIEW_DIR"] {
                    let bitmap = try XCTUnwrap(host.bitmapImageRepForCachingDisplay(in: host.bounds))
                    host.cacheDisplay(in: host.bounds, to: bitmap)
                    try XCTUnwrap(bitmap.representation(using: .png, properties: [:])).write(to: URL(fileURLWithPath: path)
                        .appendingPathComponent("screenshot-delivery-\(mode.rawValue)-\(scheme == .light ? "light" : "dark").png"))
                }
                window.close()
            }
        }
    }
}
