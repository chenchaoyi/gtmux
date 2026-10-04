import AppKit

/// The screenshot flow: permission → close our own surfaces → capture → editor. Nothing
/// here touches a pane; only the editor's Send does, when the user presses it.
final class ScreenshotController {
    static let shared = ScreenshotController()

    var access = ScreenCaptureAccess()
    private var capturing = false
    private static let requestedKey = "screenshot.permissionRequested"

    /// `closeSurfaces` closes the popover and the palette so neither is in the picture.
    func start(store: AgentStore, l10n: L10n, closeSurfaces: @escaping () -> Void) {
        let editor = ScreenshotEditorController.shared
        if editor.isOpen {
            // A second press keeps the work in progress instead of replacing it.
            editor.bringToFront()
            return
        }
        guard !capturing else { return }
        guard access.preflight() else {
            askForPermission(l10n: l10n)
            return
        }
        closeSurfaces()
        capturing = true
        AgentStoreSnapshot.store = store

        // Which pane each terminal shows, read while the user is still selecting.
        var viewed: [String: Int64] = [:]
        let lookup = DispatchGroup()
        lookup.enter()
        DispatchQueue.global(qos: .userInitiated).async {
            viewed = ScreenshotTargets.viewedAt(fromPanesJSON: GtmuxCLI.capture(["panes", "--json"]) ?? Data())
            lookup.leave()
        }

        // Let the popover's close animation finish before the screen is captured.
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.25) { [weak self] in
            let file = ScreenshotCapture.temporaryURL()
            ScreenshotSelector.run(output: file) { outcome in
                self?.capturing = false
                switch outcome {
                case .cancelled:
                    DiagLog.act("act.screenshot.capture", target: "", outcome: "refused", "screenshot cancelled")
                case .failed(let why):
                    DiagLog.act("act.screenshot.capture", target: "", outcome: "failed", "screenshot failed", ["error": why])
                    Self.alert(l10n.tr("The screenshot failed", "截图失败"), why)
                case .image(let url):
                    guard let doc = ScreenshotDocument(contentsOf: url) else {
                        try? FileManager.default.removeItem(at: url)
                        Self.alert(l10n.tr("The screenshot could not be read", "读不了这张截图"), url.lastPathComponent)
                        return
                    }
                    lookup.notify(queue: .main) {
                        let candidates = store.shareablePanes
                        let target = ScreenshotTargets.defaultTarget(
                            candidates: candidates, viewedAt: viewed, last: ScreenshotTarget.load())
                        ScreenshotEditorController.shared.show(
                            doc: doc, captureFile: url, target: target?.paneID,
                            recentPane: ScreenshotTargets.recentPane(candidates: candidates, viewedAt: viewed),
                            store: store, l10n: l10n)
                    }
                }
            }
        }
    }

    /// The first time, macOS shows its own prompt (and lists Gtmux under Screen Recording);
    /// after that, only the app can say where the switch is.
    private func askForPermission(l10n: L10n) {
        let defaults = UserDefaults.standard
        if !defaults.bool(forKey: Self.requestedKey) {
            defaults.set(true, forKey: Self.requestedKey)
            _ = access.request()
            return
        }
        NSApp.activate(ignoringOtherApps: true)
        let alert = NSAlert()
        alert.messageText = l10n.tr("Turn on Screen Recording for Gtmux", "为 Gtmux 打开「屏幕录制」权限")
        alert.informativeText = l10n.tr(
            "To capture other apps' windows, Gtmux needs Screen Recording. Turn it on in System Settings → Privacy & Security → Screen & System Audio Recording, then press ⌥⌘4 again. macOS may ask you to reopen Gtmux first.",
            "要截到其他 app 的窗口，Gtmux 需要「屏幕录制」权限。在 系统设置 → 隐私与安全性 → 录屏与系统录音 里打开它，再按一次 ⌥⌘4。macOS 可能会先让你重新打开 Gtmux。")
        alert.addButton(withTitle: l10n.tr("Open System Settings", "打开系统设置"))
        alert.addButton(withTitle: l10n.tr("Cancel", "取消"))
        if alert.runModal() == .alertFirstButtonReturn {
            NSWorkspace.shared.open(ScreenCaptureAccess.settingsURL)
        }
    }

    private static func alert(_ title: String, _ detail: String) {
        NSApp.activate(ignoringOtherApps: true)
        let a = NSAlert()
        a.messageText = title
        a.informativeText = detail
        a.runModal()
    }
}
