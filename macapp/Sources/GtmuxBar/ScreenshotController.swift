import AppKit

/// The screenshot flow: permission → preserve the open menu → capture → editor. Nothing
/// here touches a pane; only the editor's Send does, when the user presses it.
final class ScreenshotController {
    static let shared = ScreenshotController()

    var access = ScreenCaptureAccess()
    var capture: (URL, @escaping (CaptureOutcome) -> Void) -> Void = ScreenshotSelector.run
    var readPanes: () -> Data = { GtmuxCLI.capture(["panes", "--json"]) ?? Data() }
    var presentAlert: (String, String) -> Void = { ScreenshotController.alert($0, $1) }
    /// The permission flow can be observed without requesting real TCC permission in tests.
    var requestPermission: ((L10n) -> Void)?
    private var capturing = false
    private static let requestedKey = "screenshot.permissionRequested"

    /// Prepare only after permission and reentry checks. Finish on every capture
    /// outcome, before alerts or the editor can change focus.
    func start(store: AgentStore, l10n: L10n, prepareSurfaces: @escaping () -> Void,
               finishSurfaces: @escaping (CaptureOutcome) -> Void) {
        if !ScreenshotEditors.shared.hasCapacity {
            presentAlert(l10n.tr("Screenshot limit reached", "截图窗口已达上限"), ScreenshotEditors.capacityMessage(l10n))
            return
        }
        guard !capturing else { return }
        guard access.preflight() else {
            if let requestPermission { requestPermission(l10n) }
            else { askForPermission(l10n: l10n) }
            return
        }
        prepareSurfaces()
        capturing = true

        // Which pane each terminal shows, read while the user is still selecting.
        var viewed: [String: Int64] = [:]
        let readPanes = self.readPanes
        let lookup = DispatchGroup()
        lookup.enter()
        DispatchQueue.global(qos: .userInitiated).async {
            viewed = ScreenshotTargets.viewedAt(fromPanesJSON: readPanes())
            lookup.leave()
        }

        // Let the palette's close animation finish; the menu stays in the picture.
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.25) { [weak self] in
            let file = ScreenshotCapture.temporaryURL()
            self?.capture(file) { outcome in
                finishSurfaces(outcome)
                switch outcome {
                case .cancelled:
                    self?.capturing = false
                    DiagLog.act("act.screenshot.capture", target: "", outcome: "refused", "screenshot cancelled")
                case .failed(let why):
                    self?.capturing = false
                    DiagLog.act("act.screenshot.capture", target: "", outcome: "failed", "screenshot failed", ["error": why])
                    self?.presentAlert(l10n.tr("The screenshot failed", "截图失败"), why)
                case .image(let url):
                    guard let doc = ScreenshotDocument(contentsOf: url) else {
                        self?.capturing = false
                        try? FileManager.default.removeItem(at: url)
                        self?.presentAlert(l10n.tr("The screenshot could not be read", "无法读取截图"), url.lastPathComponent)
                        return
                    }
                    lookup.notify(queue: .main) {
                        self?.capturing = false
                        let candidates = store.shareablePanes
                        let target = ScreenshotTargets.defaultTarget(
                            candidates: candidates, viewedAt: viewed, last: ScreenshotTarget.load())
                        let opened = ScreenshotEditors.shared.open(
                            doc: doc, captureFile: url, target: target?.paneID,
                            recentPane: ScreenshotTargets.recentPane(candidates: candidates, viewedAt: viewed),
                            store: store, l10n: l10n)
                        if !opened {
                            try? FileManager.default.removeItem(at: url)
                            self?.presentAlert(l10n.tr("Screenshot limit reached", "截图窗口已达上限"), ScreenshotEditors.capacityMessage(l10n))
                        }
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
