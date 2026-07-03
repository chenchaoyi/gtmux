import AppKit
import Carbon.HIToolbox
import Combine
import SwiftUI

final class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusItem: NSStatusItem!
    private let popover = NSPopover()
    private let store = AgentStore()
    private let l10n = L10n.shared
    private let settings = AppSettings.shared
    private var timer: Timer?
    private var tabOrderTimer: Timer?
    private var hotkey: GlobalHotkey?
    private var cancellables = Set<AnyCancellable>()

    private var displayMode: DisplayMode { settings.displayMode }

    func applicationDidFinishLaunching(_ notification: Notification) {
        dbg("launched")
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        if let button = statusItem.button {
            button.target = self
            button.action = #selector(togglePopover)
        }

        popover.behavior = .transient
        popover.animates = true
        popover.contentViewController = NSHostingController(
            rootView: MenuView(
                store: store, l10n: l10n,
                onJump: { [weak self] in self?.jump($0) },
                onAction: { [weak self] in self?.perform($0) },
                onSend: { [weak self] in self?.sendReply($0, $1) },
                onClose: { [weak self] in self?.popover.performClose(nil) }))

        // Repaint the status item whenever agents change.
        store.$agents.receive(on: RunLoop.main)
            .sink { [weak self] in self?.renderIcon($0) }
            .store(in: &cancellables)
        renderIcon([])

        store.refresh()
        resetTimer()

        // Quietly check for a newer release a few seconds after launch (throttled to
        // once/day inside Updater). If one exists, the popover shows a "new version"
        // banner the user can click to install — same effect as `gtmux update`.
        DispatchQueue.main.asyncAfter(deadline: .now() + 4) { Updater.shared.autoCheck() }

        // Record the live terminal tab→session order on a SLOW timer (reads the
        // terminal via AppleScript, so not on the 1.5s poll) so `gtmux restore`
        // can replay your tab arrangement instead of tmux's alphabetical order.
        GtmuxCLI.spawn(["save-tab-order"])
        tabOrderTimer = Timer.scheduledTimer(withTimeInterval: 12, repeats: true) { _ in
            GtmuxCLI.spawn(["save-tab-order"])
        }

        // Live-apply preference changes (refresh interval, status-bar display mode).
        settings.objectWillChange.receive(on: RunLoop.main).sink { [weak self] in
            DispatchQueue.main.async {
                self?.resetTimer()
                self?.renderIcon(self?.store.agents ?? [])
            }
        }.store(in: &cancellables)

        // Global hotkey ⌘⌥G opens the command palette (DESIGN §4 B); the menu-bar
        // click opens the popover.
        hotkey = GlobalHotkey(keyCode: UInt32(kVK_ANSI_G), modifiers: UInt32(cmdKey | optionKey)) { [weak self] in
            DispatchQueue.main.async { self?.toggleCommandPalette() }
        }

        // Deliver desktop notifications natively (replaces terminal-notifier): the
        // hook queues requests, we post them and jump on click.
        NotificationManager.shared.start(
            onJump: { pane in GtmuxCLI.spawn(pane.isEmpty ? ["focus", "--last"] : ["focus", pane]) },
            onSend: { [weak self] pane, n in self?.sendText(pane, "\(n)") },
            onSendText: { [weak self] pane, text in self?.sendText(pane, text) })

        // Test seam: GTMUXBAR_SHOW_PALETTE auto-opens the palette so it can be
        // exercised without a (flaky) synthetic global keystroke. No-op normally.
        if ProcessInfo.processInfo.environment["GTMUXBAR_SHOW_PALETTE"] != nil {
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) { [weak self] in
                self?.toggleCommandPalette()
            }
        }
    }

    private func toggleCommandPalette() {
        dbg("hotkey fired → toggle command palette")
        CommandPaletteController.shared.toggle(
            store: store, l10n: l10n,
            onJump: { agent in GtmuxCLI.spawn(agent.jumpArgs()) },
            onSend: { [weak self] agent, n in self?.sendReply(agent, n) })
    }

    private func resetTimer() {
        timer?.invalidate()
        timer = Timer.scheduledTimer(withTimeInterval: settings.refreshInterval, repeats: true) { [weak self] _ in
            self?.store.refresh()
            RemoteAccess.shared.refreshClients() // keep the remote-viewer indicator live
        }
    }

    private func renderIcon(_ agents: [Agent]) {
        guard let button = statusItem.button else { return }
        let empty = store.total == 0
        let urgent = store.mostUrgent

        // hide-when-idle: only show when something is waiting on you.
        if displayMode == .hideWhenIdle {
            statusItem.isVisible = store.waiting > 0
            if store.waiting == 0 { return }
        } else {
            statusItem.isVisible = true
        }

        // Auto-withdraw waiting notifications whose pane is no longer waiting (A2).
        NotificationManager.shared.reconcile(
            waitingPanes: Set(agents.filter { $0.state == .waiting }.map { $0.paneID }))

        let dark = button.effectiveAppearance.bestMatch(from: [.aqua, .darkAqua]) == .darkAqua
        button.image = StatusItemGlyph.image(mostUrgent: urgent, empty: empty, dark: dark)
        let badge = displayMode == .dot ? "" : store.badge
        button.title = badge.isEmpty ? "" : " \(badge)"
        button.imagePosition = badge.isEmpty ? .imageOnly : .imageLeft
    }

    @objc private func togglePopover() {
        guard let button = statusItem.button else { return }
        if popover.isShown { popover.performClose(nil); return }
        // The popover and the center-screen command palette must never coexist.
        CommandPaletteController.shared.dismiss()
        store.refresh()
        // Check for a new release each time the menu is opened (throttled inside
        // Updater) so the "new version" banner is fresh when you actually look —
        // not just once a day.
        Updater.shared.autoCheck()
        // Activate FIRST: a hotkey press from another app must make gtmux active,
        // else the transient popover dismisses immediately.
        NSApp.activate(ignoringOtherApps: true)
        popover.show(relativeTo: button.bounds, of: button, preferredEdge: .minY)
        popover.contentViewController?.view.window?.makeKey()
    }

    private func jump(_ agent: Agent) {
        popover.performClose(nil)
        GtmuxCLI.spawn(agent.jumpArgs())
    }

    /// In-place reply (A1/A3): send the chosen option to the pane and re-poll soon
    /// so the row flips to working. Stays in the popover (no close).
    private func sendReply(_ agent: Agent, _ n: Int) { sendText(agent.paneID, "\(n)") }

    /// Send literal text to a pane (notification 1/2/3 + free-text reply, A2).
    private func sendText(_ pane: String, _ text: String) {
        guard !pane.isEmpty else { return }
        GtmuxCLI.spawn(["send", pane, text])
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.4) { [weak self] in self?.store.refresh() }
    }

    private func perform(_ action: MenuAction) {
        switch action {
        case .restore:    GtmuxCLI.spawn(["restore"])
        case .newSession: GtmuxCLI.spawn(["new"])
        case .preferences: PreferencesController.shared.show(l10n: l10n)
        case .pairPhone:  PairingController.shared.show(l10n: l10n)
        case .quit:       NSApp.terminate(nil)
        }
        if action != .quit && action != .preferences && action != .pairPhone {
            popover.performClose(nil)
        }
    }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        dbg("reopen (notification click) → focus --last")
        GtmuxCLI.spawn(["focus", "--last"])
        return true
    }
}
