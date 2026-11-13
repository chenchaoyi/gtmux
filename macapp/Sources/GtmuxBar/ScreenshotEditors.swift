import AppKit

/// Each captured document owns its window, responder, export paths and delivery callbacks.
/// The registry retains controllers only while their documents are open.
final class ScreenshotEditors {
    static let shared = ScreenshotEditors()
    static let maximumCount = 8
    private(set) var documents: [UUID: ScreenshotEditorController] = [:]
    private var sequence = 0

    var hasCapacity: Bool { documents.count < Self.maximumCount }

    @discardableResult
    func open(doc: ScreenshotDocument, captureFile: URL, target: String?, recentPane: String? = nil,
              store: AgentStore, l10n: L10n) -> Bool {
        guard hasCapacity else { return false }
        let id = UUID(), controller = ScreenshotEditorController()
        controller.onClose = { [weak self] in self?.documents.removeValue(forKey: id) }
        documents[id] = controller
        controller.show(doc: doc, captureFile: captureFile, target: target, recentPane: recentPane,
                        store: store, l10n: l10n)
        sequence += 1
        controller.window?.title = ScreenshotLayout.title(l10n) + " · \(sequence)"
        // Cascade within the actual display; never move a window beyond its usable frame.
        if let window = controller.window, let screen = window.screen {
            let visible = screen.visibleFrame
            let offset = CGFloat((documents.count - 1) % 5) * 22
            window.setFrameOrigin(NSPoint(x: min(visible.maxX - window.frame.width, window.frame.minX + offset),
                                          y: max(visible.minY, window.frame.minY - offset)))
        }
        return true
    }

    static func capacityMessage(_ l10n: L10n) -> String {
        l10n.tr("Up to \(maximumCount) screenshots can stay open. Save or close one before taking another.",
                 "最多同时编辑 \(maximumCount) 张截图。请先保存或关闭一张，再截取新图。")
    }
}
