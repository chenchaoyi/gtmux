import AppKit
import SwiftUI

/// What the editor's status line says.
enum ScreenshotStatus: Equatable {
    case idle
    case copied
    case saved(String)
    case sending
    case result(ScreenshotSendResult)
    case error(String)
}

/// Where a send went, fixed when Send is pressed: the picker may change while the send is in
/// flight, and the result must name the pane it was actually for.
struct ScreenshotSendTarget: Equatable {
    let paneID: String
    let session: String
    let name: String
}

/// The editor's state: the capture, the tool, the note, the target and the send.
final class ScreenshotEditorModel: ObservableObject {
    let doc: ScreenshotDocument
    let captureFile: URL
    @Published var tool: AnnotationTool = .rect
    @Published var color: AnnotationColor = .red
    @Published var width: AnnotationWidth = .medium
    /// A text mark being typed, at its point in image space.
    @Published var textAt: CGPoint?
    @Published var textValue = ""
    @Published var note = ""
    @Published var targetID: String?
    @Published var status: ScreenshotStatus = .idle {
        didSet { doc.editingEnabled = !editingBlocked }
    }
    @Published var sendingTarget: ScreenshotSendTarget?
    /// The agent pane a terminal showed most recently, when there is one: the picker says so.
    let recentPane: String?

    init(doc: ScreenshotDocument, captureFile: URL, target: String?, recentPane: String? = nil) {
        self.doc = doc
        self.captureFile = captureFile
        self.targetID = target
        self.recentPane = recentPane
    }

    var sending: Bool { status == .sending }
    var editingBlocked: Bool { Self.blocksEditing(status) }
    static func blocksEditing(_ status: ScreenshotStatus) -> Bool {
        if status == .sending { return true }
        if case .result(let result) = status { return result.isSuccess }
        return false
    }
    var canSend: Bool {
        !editingBlocked && targetID != nil
    }
    /// Work that Esc or the close button would throw away.
    var hasWork: Bool { doc.hasEdits || !note.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty }
    /// A delivery that may or may not have happened: the next Send is a retry, said as one.
    var isRetry: Bool {
        if case .result(.notConfirmed) = status { return true }
        if case .result(.failed) = status { return true }
        return false
    }

    func commitText() {
        guard !editingBlocked else { return }
        let s = textValue.trimmingCharacters(in: .whitespacesAndNewlines)
        if let at = textAt, !s.isEmpty {
            doc.add(Annotation(.text(s, at: at), color: color, width: width))
        }
        textAt = nil
        textValue = ""
    }

    /// A text mark still being typed is part of the picture: Copy, Save and Send all take it.
    func commitPendingText() {
        if textAt != nil { commitText() }
    }

    func cancelText() {
        guard !editingBlocked else { return }
        textAt = nil
        textValue = ""
    }

    /// The one export: what Copy, Save and Send all use.
    func flattened() -> CGImage? { AnnotationRenderer.flatten(doc) }

    /// True only when the image is on the pasteboard: the editor confirms a copy on that, and
    /// never on a failure.
    @discardableResult
    func copy(to pasteboard: NSPasteboard = .general) -> Bool {
        guard !editingBlocked else { return false }
        commitPendingText()
        guard let cg = flattened(), AnnotationRenderer.copy(cg, pointSize: doc.pointSize, to: pasteboard) else {
            status = .error("could not render the image")
            return false
        }
        status = .copied
        return true
    }

    func write(to url: URL) -> Bool {
        guard !editingBlocked else { return false }
        commitPendingText()
        guard let cg = flattened(), let png = AnnotationRenderer.pngData(cg, pointSize: doc.pointSize) else { return false }
        do {
            try png.write(to: url, options: .atomic)
            return true
        } catch {
            return false
        }
    }
}
