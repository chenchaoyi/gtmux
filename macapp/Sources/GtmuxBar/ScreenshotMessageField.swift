import AppKit
import SwiftUI

/// A native multiline note: normal Return and input-method handling stay with
/// NSTextView; only the explicit send chord is intercepted.
struct ScreenshotMessageField: NSViewRepresentable {
    @Binding var text: String
    let label: String
    let help: String
    let canSend: () -> Bool
    let onSend: () -> Void

    func makeCoordinator() -> Coordinator { Coordinator(self) }

    func makeNSView(context: Context) -> NSScrollView {
        let scroll = NSScrollView()
        scroll.drawsBackground = false
        scroll.hasVerticalScroller = true
        scroll.autohidesScrollers = true
        let note = ScreenshotNoteTextView()
        note.isRichText = false
        note.isEditable = true
        note.allowsUndo = true
        note.drawsBackground = false
        note.font = .systemFont(ofSize: 13)
        note.textColor = .textColor
        note.insertionPointColor = .textColor
        note.textContainerInset = CGSize(width: 8, height: 4)
        note.textContainer?.lineFragmentPadding = 4
        note.isVerticallyResizable = true
        note.isHorizontallyResizable = false
        note.autoresizingMask = [.width]
        note.textContainer?.widthTracksTextView = true
        note.maxSize = CGSize(width: CGFloat.greatestFiniteMagnitude, height: CGFloat.greatestFiniteMagnitude)
        note.delegate = context.coordinator
        scroll.documentView = note
        updateNSView(scroll, context: context)
        return scroll
    }

    func updateNSView(_ scroll: NSScrollView, context: Context) {
        context.coordinator.parent = self
        guard let note = scroll.documentView as? ScreenshotNoteTextView else { return }
        // Do not replace text during composition or ordinary cursor movement.
        if note.string != text && !note.hasMarkedText() {
            let selection = note.selectedRange()
            note.string = text
            note.setSelectedRange(NSRange(location: min(selection.location, (text as NSString).length), length: 0))
        }
        note.canSend = canSend
        note.onSend = onSend
        note.setAccessibilityLabel(label)
        note.setAccessibilityHelp(help)
        scroll.toolTip = help
    }

    final class Coordinator: NSObject, NSTextViewDelegate {
        var parent: ScreenshotMessageField
        init(_ parent: ScreenshotMessageField) { self.parent = parent }
        func textDidChange(_ notification: Notification) {
            guard let note = notification.object as? NSTextView else { return }
            parent.text = note.string
        }
    }
}

class ScreenshotNoteTextView: NSTextView {
    var canSend: () -> Bool = { false }
    var onSend: () -> Void = {}

    private func handleSend(_ event: NSEvent) -> Bool {
        let modifiers = event.modifierFlags.intersection([.command, .shift, .option, .control])
        guard [UInt16(36), UInt16(76)].contains(event.keyCode), modifiers == .command else { return false }
        if !event.isARepeat && !hasMarkedText() && canSend() { onSend() }
        return true
    }
    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        if handleSend(event) { return true }
        return super.performKeyEquivalent(with: event)
    }
    override func keyDown(with event: NSEvent) {
        if !handleSend(event) { super.keyDown(with: event) }
    }
}
