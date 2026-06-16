import AppKit
import Carbon.HIToolbox
import Combine
import SwiftUI

final class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusItem: NSStatusItem!
    private let popover = NSPopover()
    private let store = AgentStore()
    private var timer: Timer?
    private var hotkey: GlobalHotkey?
    private var cancellables = Set<AnyCancellable>()

    func applicationDidFinishLaunching(_ notification: Notification) {
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        if let button = statusItem.button {
            button.image = dotImage(AgentState.none.color)
            button.imagePosition = .imageLeft
            button.target = self
            button.action = #selector(togglePopover)
        }

        popover.behavior = .transient
        popover.animates = true
        popover.contentViewController = NSHostingController(
            rootView: MenuView(
                store: store,
                onJump: { [weak self] in self?.jump($0) },
                onAction: { [weak self] in self?.perform($0) }))

        // Repaint the icon whenever the agent set changes (SwiftUI updates itself).
        store.$agents
            .receive(on: RunLoop.main)
            .sink { [weak self] in self?.renderIcon($0) }
            .store(in: &cancellables)

        store.refresh()
        timer = Timer.scheduledTimer(withTimeInterval: 1.5, repeats: true) { [weak self] _ in
            self?.store.refresh()
        }

        // Global hotkey ⌘⌥G toggles the popover — the keyboard path we couldn't
        // do on the old systray stack.
        hotkey = GlobalHotkey(keyCode: UInt32(kVK_ANSI_G), modifiers: UInt32(cmdKey | optionKey)) { [weak self] in
            DispatchQueue.main.async { self?.togglePopover() }
        }
    }

    private func renderIcon(_ agents: [Agent]) {
        guard let button = statusItem.button else { return }
        button.image = dotImage(AgentState.of(agents).color)
        let badge = store.badge
        button.title = badge.isEmpty ? "" : " \(badge)"
        button.imagePosition = badge.isEmpty ? .imageOnly : .imageLeft
    }

    @objc private func togglePopover() {
        dbg("togglePopover: isShown=\(popover.isShown)")
        guard let button = statusItem.button else { return }
        if popover.isShown {
            popover.performClose(nil)
            return
        }
        store.refresh()
        popover.show(relativeTo: button.bounds, of: button, preferredEdge: .minY)
        dbg("popover.show called; isShown=\(popover.isShown)")
        // LSUIElement apps must activate for the popover to take keyboard focus,
        // which matters when it's summoned by the hotkey rather than a click.
        NSApp.activate(ignoringOtherApps: true)
    }

    private func jump(_ agent: Agent) {
        popover.performClose(nil)
        GtmuxCLI.spawn(["focus", agent.paneID])
    }

    private func perform(_ action: MenuAction) {
        switch action {
        case .overview:   openGhosttyWindow(running: GtmuxCLI.shellInvocation(["overview", "--hold"]))
        case .watch:      openGhosttyWindow(running: GtmuxCLI.shellInvocation(["agents", "--watch"]))
        case .restore:    GtmuxCLI.spawn(["restore"])
        case .newSession: GtmuxCLI.spawn(["new"])
        case .quit:       NSApp.terminate(nil)
        }
        if action != .quit { popover.performClose(nil) }
    }

    // A notification click activates this app (bundle id com.gtmux.menubar);
    // jump to the agent that just finished — same contract as GtmuxFocus.app.
    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        dbg("reopen (notification click) → focus --last")
        GtmuxCLI.spawn(["focus", "--last"])
        return true
    }
}
