import AppKit

/// Restore, automatic release notes and screenshots opt the current presentation out of
/// AppKit's activation-driven dismissal. User dismissal still wins; no callback
/// shows or activates the menu again. The next opening is transient by default.
final class MenuPopoverPresentation {
    private let popover: NSPopover
    private weak var companion: NSWindow?
    private var closeObserver: NSObjectProtocol?
    private var localMonitor: Any?
    private var globalMonitor: Any?
    private var selectingScreenshot = false

    init(popover: NSPopover) {
        self.popover = popover
        popover.behavior = .transient
        closeObserver = NotificationCenter.default.addObserver(
            forName: NSPopover.didCloseNotification, object: popover, queue: .main
        ) { [weak self] _ in self?.reset() }
    }

    deinit {
        if let closeObserver { NotificationCenter.default.removeObserver(closeObserver) }
        removeMonitors()
    }

    func prepareToShow(restoring: Bool) {
        guard !popover.isShown else { return }
        reset()
        if restoring { protectFromActivation() }
    }

    func restoreStarted() {
        guard popover.isShown else { return }
        protectFromActivation()
    }

    /// Keep the live menu in the frozen display, including when the selector or
    /// system fallback takes focus. Selection clicks belong to the capture.
    func screenshotStarted() {
        guard popover.isShown else { return }
        selectingScreenshot = true
        protectFromActivation()
    }

    func screenshotFinished(_ outcome: CaptureOutcome) {
        guard selectingScreenshot else { return }
        selectingScreenshot = false
        if case .image = outcome { popover.performClose(nil) }
        // Cancellation/failure leaves the menu open with normal explicit dismissal.
        // Keep activation protection until close: the system selector may still
        // restore the previously active application after its completion callback.
    }

    /// The caller retains the pending receipt when the menu is no longer visible.
    /// Protect BEFORE showing the reader: ordering its window can change activation.
    @discardableResult
    func presentAutomaticNotes(_ show: () -> NSWindow?) -> Bool {
        guard popover.isShown else { return false }
        protectFromActivation()
        companion = show()
        if let companion, let menu = popover.contentViewController?.view.window {
            position(companion, beside: menu)
        }
        return true
    }

    private func protectFromActivation() {
        popover.behavior = .applicationDefined
        guard localMonitor == nil else { return }
        let clicks: NSEvent.EventTypeMask = [.leftMouseDown, .rightMouseDown, .otherMouseDown]
        localMonitor = NSEvent.addLocalMonitorForEvents(matching: [clicks, .keyDown]) { [weak self] event in
            guard let self, self.popover.isShown else { return event }
            guard !self.selectingScreenshot else { return event }
            if event.type == .keyDown {
                // Do not consume Escape destined for a terminal or the notes reader.
                if event.keyCode == 53, self.isMenuWindow(event.window) {
                    self.popover.performClose(nil)
                    return nil
                }
            } else if self.dismissesForClick(in: event.window) {
                self.popover.performClose(nil)
            }
            return event
        }
        // Mouse observation needs no Accessibility/key-monitor permission. Never
        // monitor global keys or consume events intended for another application.
        globalMonitor = NSEvent.addGlobalMonitorForEvents(matching: clicks) { [weak self] _ in
            guard let self, !self.selectingScreenshot else { return }
            self.popover.performClose(nil)
        }
    }

    func dismissesForClick(in window: NSWindow?) -> Bool {
        !isMenuWindow(window) && !belongs(window, to: companion)
    }

    private func isMenuWindow(_ window: NSWindow?) -> Bool {
        belongs(window, to: popover.contentViewController?.view.window)
    }

    private func belongs(_ window: NSWindow?, to root: NSWindow?) -> Bool {
        guard let root else { return false }
        var candidate = window
        while let current = candidate {
            if current === root { return true }
            candidate = current.parent
        }
        return false
    }

    private func reset() {
        selectingScreenshot = false
        removeMonitors()
        companion = nil
        popover.behavior = .transient
    }

    private func removeMonitors() {
        if let localMonitor { NSEvent.removeMonitor(localMonitor) }
        if let globalMonitor { NSEvent.removeMonitor(globalMonitor) }
        localMonitor = nil; globalMonitor = nil
    }

    private func position(_ reader: NSWindow, beside menu: NSWindow) {
        let visible = menu.screen?.visibleFrame ?? NSScreen.main?.visibleFrame ?? menu.frame
        let gap: CGFloat = 12
        let left = menu.frame.minX - reader.frame.width - gap
        let right = menu.frame.maxX + gap
        let x = left >= visible.minX ? left : (right + reader.frame.width <= visible.maxX ? right : visible.minX)
        let y = min(visible.maxY - reader.frame.height, max(visible.minY, menu.frame.maxY - reader.frame.height))
        reader.setFrameOrigin(NSPoint(x: x, y: y))
    }
}
