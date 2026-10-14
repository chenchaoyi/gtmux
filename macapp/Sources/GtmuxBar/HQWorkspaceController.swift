import AppKit
import SwiftUI

@MainActor
final class HQWorkspaceController: NSObject, NSWindowDelegate {
    static let shared = HQWorkspaceController()
    private(set) var window: NSWindow?
    private let store: HQWorkspaceStore
    init(store: HQWorkspaceStore? = nil) {
        self.store = store ?? HQWorkspaceStore()
        super.init()
    }
    func show(l10n: L10n) {
        if window == nil {
            let w = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: 980, height: 740),
                styleMask: [.titled, .closable, .miniaturizable, .resizable], backing: .buffered, defer: false
            )
            w.title = "gtmux HQ"
            w.isReleasedWhenClosed = false
            w.minSize = NSSize(width: 720, height: 480)
            w.delegate = self
            let host = NSHostingController(rootView: HQWorkspaceView(l10n: l10n, store: store))
            host.sizingOptions = []
            w.contentViewController = host
            w.setContentSize(NSSize(width: 980, height: 740))
            w.contentMinSize = NSSize(width: 700, height: 440)
            w.setFrameAutosaveName("gtmux-hq-workspace")
            if let screen = NSScreen.main, !screen.visibleFrame.contains(w.frame) {
                var frame = w.frame
                frame.size.width = min(frame.width, screen.visibleFrame.width)
                frame.size.height = min(frame.height, screen.visibleFrame.height)
                frame.origin = CGPoint(
                    x: screen.visibleFrame.midX - frame.width / 2,
                    y: screen.visibleFrame.midY - frame.height / 2)
                w.setFrame(frame, display: false)
            }
            window = w
        }
        store.start()
        NSApp.activate(ignoringOtherApps: true)
        window?.makeKeyAndOrderFront(nil)
    }
    func windowWillClose(_ notification: Notification) { store.stop() }
    func windowDidChangeOcclusionState(_ notification: Notification) {
        if window?.occlusionState.contains(.visible) == true { store.start() } else { store.stop() }
    }
}
