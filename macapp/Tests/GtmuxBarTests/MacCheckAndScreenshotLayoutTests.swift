import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

final class MacCheckAndScreenshotLayoutTests: XCTestCase {
    func testCheckDetailsKeepGuidanceWithoutRepeatingVisibleText() {
        let purpose = "Checks the installed session plugin."
        let row = EnvironmentReport.Row(status: "ok", label: "Plugin manager", value: "Installed",
                                        note: "Plugin manager", description: purpose)
        XCTAssertEqual(row.purpose, purpose)
        XCTAssertEqual(row.detailLines, [])
        let technical = EnvironmentReport.Row(status: "ok", label: "Plugin manager", value: "Installed",
                                              note: "~/.tmux/plugins/tpm", description: purpose)
        XCTAssertEqual(technical.detailLines, ["~/.tmux/plugins/tpm"])
        let samePurpose = EnvironmentReport.Row(status: "recommended", label: "Check", value: "Needs review",
                                                note: "Checks the installed session plugin.\n", description: purpose)
        XCTAssertEqual(samePurpose.detailLines, [])
        let legacy = EnvironmentReport.Row(status: "ok", label: "Plugin manager", value: "Installed", note: " Installed\n")
        XCTAssertEqual(legacy.purpose, "")
        XCTAssertEqual(legacy.detailLines, [])
        let oldGuidance = EnvironmentReport.Row(status: "recommended", label: "Check", value: "Needs review", note: "Run a separate manual action")
        XCTAssertEqual(oldGuidance.purpose, "Run a separate manual action")
        XCTAssertEqual(oldGuidance.detailLines, [])
        let value = String(repeating: "long result ", count: 20)
        let long = EnvironmentReport.Row(status: "recommended", label: "Check", value: value, note: "Read the full result", description: purpose)
        XCTAssertTrue(long.hasLongResult)
        XCTAssertEqual(long.detailLines, [value, "Read the full result"])
        let multiline = EnvironmentReport.Row(status: "ok", label: "Check", value: "first\nsecond", note: "", description: purpose)
        XCTAssertEqual(multiline.detailLines, ["first\nsecond"])
    }

    private func key(_ modifiers: NSEvent.ModifierFlags = [], code: UInt16 = 36, repeatKey: Bool = false) -> NSEvent {
        NSEvent.keyEvent(with: .keyDown, location: .zero, modifierFlags: modifiers, timestamp: 0,
                         windowNumber: 0, context: nil, characters: "\r", charactersIgnoringModifiers: "\r",
                         isARepeat: repeatKey, keyCode: code)!
    }

    @MainActor func testNoteReturnInsertsNewlineAndExplicitSendIsGuarded() {
        let note = ScreenshotNoteTextView(frame: NSRect(x: 0, y: 0, width: 400, height: 64))
        note.isRichText = false
        note.string = "first line"
        note.setSelectedRange(NSRange(location: note.string.utf16.count, length: 0))
        var sends = 0
        note.canSend = { true }; note.onSend = { sends += 1 }
        note.keyDown(with: key())
        XCTAssertEqual(note.string, "first line\n")
        XCTAssertEqual(sends, 0)
        XCTAssertTrue(note.performKeyEquivalent(with: key(.command)))
        XCTAssertEqual(sends, 1)
        XCTAssertEqual(note.string, "first line\n")
        note.canSend = { false }
        note.keyDown(with: key(.command))
        XCTAssertEqual(sends, 1)
        note.canSend = { true }
        note.keyDown(with: key(.command, repeatKey: true))
        XCTAssertEqual(sends, 1)
        note.setMarkedText("中", selectedRange: NSRange(location: 1, length: 0), replacementRange: NSRange(location: NSNotFound, length: 0))
        XCTAssertTrue(note.hasMarkedText())
        XCTAssertTrue(note.performKeyEquivalent(with: key(.command)))
        XCTAssertEqual(sends, 1, "An input-method composition must not be sent")
        note.unmarkText()
        XCTAssertTrue(note.performKeyEquivalent(with: key(.command, code: 76)))
        XCTAssertEqual(sends, 2)
    }

    @MainActor func testNativeNoteBindingWrapsAndRetainsMultilineText() throws {
        var text = ""
        let field = ScreenshotMessageField(text: Binding(get: { text }, set: { text = $0 }),
                                           label: "Message", help: "Return for a new line", canSend: { false }, onSend: {})
        let host = NSHostingView(rootView: field)
        host.sizingOptions = []
        let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 400, height: 64), styleMask: [.borderless], backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false
        window.contentView = host
        defer { window.close() }
        host.layoutSubtreeIfNeeded()
        RunLoop.main.run(until: Date().addingTimeInterval(0.1))
        func find(_ view: NSView) -> ScreenshotNoteTextView? {
            if let input = view as? ScreenshotNoteTextView { return input }
            return view.subviews.compactMap(find).first
        }
        let note = try XCTUnwrap(find(host))
        XCTAssertGreaterThan(note.frame.width, 300)
        XCTAssertTrue(note.allowsUndo, "Multiline editing retains native undo")
        XCTAssertTrue(try XCTUnwrap(note.textContainer).widthTracksTextView)
        note.insertText("first\nsecond", replacementRange: NSRange(location: 0, length: 0))
        XCTAssertEqual(text, "first\nsecond", "The payload must retain the note's newline")
        XCTAssertEqual(host.frame.height, 64)
    }

    @MainActor func testLiveWindowResizeUpdatesImageMarksAndPointerMappingWithoutChangingExport() throws {
        let context = try XCTUnwrap(CGContext(data: nil, width: 600, height: 400, bitsPerComponent: 8, bytesPerRow: 0,
                                            space: CGColorSpace(name: CGColorSpace.sRGB)!, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
        let doc = ScreenshotDocument(image: try XCTUnwrap(context.makeImage()), pointSize: CGSize(width: 300, height: 200))
        doc.add(Annotation(.rect(CGRect(x: 10, y: 20, width: 30, height: 40)), color: .red))
        let model = ScreenshotEditorModel(doc: doc, captureFile: URL(fileURLWithPath: "/tmp/unused-synthetic-shot.png"), target: nil)
        let view = ScreenshotEditorView(model: model, store: AgentStore(), l10n: .shared, onSend: {}, onCancel: {}, onShowPane: { _ in })
        let host = NSHostingView(rootView: view)
        host.sizingOptions = []
        let window = NSWindow(contentRect: CGRect(origin: .zero, size: ScreenshotLayout.windowSize(display: doc.pointSize)),
                              styleMask: [.borderless], backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false; window.contentView = host
        defer { window.close() }
        let before = try XCTUnwrap(model.flattened())
        let png = try XCTUnwrap(AnnotationRenderer.pngData(before, pointSize: doc.pointSize))
        func resize(_ size: CGSize) throws -> CGFloat {
            window.setContentSize(size)
            host.layoutSubtreeIfNeeded()
            RunLoop.main.run(until: Date().addingTimeInterval(0.15))
            host.layoutSubtreeIfNeeded()
            let canvas = try XCTUnwrap(ScreenshotEditorController.canvas(in: host))
            XCTAssertEqual(canvas.displaySize.width / canvas.displaySize.height, 1.5, accuracy: 0.001)
            XCTAssertEqual(canvas.marks.factor, canvas.factor)
            let point = canvas.convert(NSPoint(x: 25 * canvas.factor, y: 15 * canvas.factor), to: nil)
            let event = try XCTUnwrap(NSEvent.mouseEvent(with: .leftMouseDown, location: point, modifierFlags: [], timestamp: 0,
                                                       windowNumber: window.windowNumber, context: nil, eventNumber: 1, clickCount: 1, pressure: 1))
            XCTAssertEqual(canvas.imagePoint(event).x, 25, accuracy: 0.001)
            XCTAssertEqual(canvas.imagePoint(event).y, 15, accuracy: 0.001)
            return canvas.factor
        }
        let original = try resize(ScreenshotLayout.windowSize(display: doc.pointSize))
        XCTAssertEqual(original, 1, accuracy: 0.001)
        XCTAssertGreaterThan(try resize(CGSize(width: 1000, height: 700)), original)
        XCTAssertLessThan(try resize(CGSize(width: 680, height: 380)), original)
        let after = try XCTUnwrap(model.flattened())
        XCTAssertEqual([after.width, after.height], [600, 400])
        XCTAssertEqual(AnnotationRenderer.pngData(after, pointSize: doc.pointSize), png)
    }
}
