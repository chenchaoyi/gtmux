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
    private var hotkey: GlobalHotkey?
    private var cancellables = Set<AnyCancellable>()

    private var displayMode: DisplayMode { settings.displayMode }

    func applicationDidFinishLaunching(_ notification: Notification) {
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
                onClose: { [weak self] in self?.popover.performClose(nil) }))

        // Repaint the status item whenever agents change.
        store.$agents.receive(on: RunLoop.main)
            .sink { [weak self] in self?.renderIcon($0) }
            .store(in: &cancellables)
        renderIcon([])

        store.refresh()
        resetTimer()

        // Live-apply preference changes (refresh interval, status-bar display mode).
        settings.objectWillChange.receive(on: RunLoop.main).sink { [weak self] in
            DispatchQueue.main.async {
                self?.resetTimer()
                self?.renderIcon(self?.store.agents ?? [])
            }
        }.store(in: &cancellables)

        // Global hotkey ⌥⇧G opens the popover (DESIGN §4).
        hotkey = GlobalHotkey(keyCode: UInt32(kVK_ANSI_G), modifiers: UInt32(optionKey | shiftKey)) { [weak self] in
            DispatchQueue.main.async { self?.togglePopover() }
        }
    }

    private func resetTimer() {
        timer?.invalidate()
        timer = Timer.scheduledTimer(withTimeInterval: settings.refreshInterval, repeats: true) { [weak self] _ in
            self?.store.refresh()
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

        button.image = StatusItemGlyph.image(mostUrgent: urgent, empty: empty)
        let badge = displayMode == .dot ? "" : store.badge
        button.title = badge.isEmpty ? "" : " \(badge)"
        button.imagePosition = badge.isEmpty ? .imageOnly : .imageLeft
    }

    @objc private func togglePopover() {
        guard let button = statusItem.button else { return }
        if popover.isShown { popover.performClose(nil); return }
        store.refresh()
        popover.show(relativeTo: button.bounds, of: button, preferredEdge: .minY)
        NSApp.activate(ignoringOtherApps: true)
        popover.contentViewController?.view.window?.makeKey()
    }

    private func jump(_ agent: Agent) {
        popover.performClose(nil)
        GtmuxCLI.spawn(agent.jumpArgs())
    }

    private func perform(_ action: MenuAction) {
        switch action {
        case .overview:   openGhosttyWindow(running: GtmuxCLI.shellInvocation(["overview", "--hold"]))
        case .watch:      openGhosttyWindow(running: GtmuxCLI.shellInvocation(["agents", "--watch"]))
        case .restore:    GtmuxCLI.spawn(["restore"])
        case .newSession: GtmuxCLI.spawn(["new"])
        case .preferences: PreferencesController.shared.show(l10n: l10n)
        case .quit:       NSApp.terminate(nil)
        }
        if action != .quit && action != .preferences { popover.performClose(nil) }
    }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        dbg("reopen (notification click) → focus --last")
        GtmuxCLI.spawn(["focus", "--last"])
        return true
    }
}
