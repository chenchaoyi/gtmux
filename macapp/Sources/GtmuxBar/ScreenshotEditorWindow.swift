import AppKit

/// Native menus and key equivalents must obey the same lock as the hosted editor.
final class ScreenshotEditorWindow: NSWindow {
    weak var editorModel: ScreenshotEditorModel?

    override var undoManager: UndoManager? {
        editorModel?.editingBlocked == true ? nil : super.undoManager
    }

    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        if editorModel?.editingBlocked == true, event.modifierFlags.contains(.command),
           let key = event.charactersIgnoringModifiers?.lowercased(),
           ["a", "c", "s", "v", "x", "z"].contains(key) { return true }
        return super.performKeyEquivalent(with: event)
    }
}
