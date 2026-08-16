import AppKit
import Carbon.HIToolbox
import Combine
import SwiftUI

final class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusItem: NSStatusItem!
    /// Server mode shows as a thin ring on the EXISTING brand mark — not as a second
    /// status item. Two icons read as two apps; one icon in a different state reads as
    /// "gtmux is doing something". The ring is neutral, so the mark's colour still
    /// means only what it has always meant: the fleet's most urgent agent state.
    private let serverMode = ServerModeStore.shared
    /// Drives the awake dot's breathing. It exists ONLY while server mode is on —
    /// gtmux is otherwise a zero-animation app (DESIGN §10) and must not burn a timer
    /// redrawing a static icon. 8fps is plenty for a slow fade and costs nothing.
    private var awakeTimer: Timer?
    private var awakePhase: CGFloat = 0
    private let popover = NSPopover()
    private let store = AgentStore()
    private let l10n = L10n.shared
    private let settings = AppSettings.shared
    private var timer: Timer?
    private var tabOrderTimer: Timer?
    private var resourceTimer: Timer?
    private var hotkey: GlobalHotkey?
    private var cancellables = Set<AnyCancellable>()

    // A notification click BOTH activates this accessory app (→ reopen) and delivers
    // to the notification delegate (→ didReceive, with the specific pane). These two
    // coordinate so the generic `focus --last` never races (or overrides) the jump to
    // the exact pane you clicked: didReceive stamps the jump + cancels a pending
    // reopen focus; reopen skips when a jump just happened, else defers as a fallback.
    private var pendingReopenFocus: DispatchWorkItem?
    private var lastNotificationJumpAt: Date?

    private var displayMode: DisplayMode { settings.displayMode }

    func applicationDidFinishLaunching(_ notification: Notification) {
        dbg("launched")
        // An LSUIElement app has NO main menu, so AppKit has no standard Edit menu to
        // route ⌘C/⌘V/⌘X/⌘A to the first responder — text fields (e.g. the Direct
        // access-code input) then can't paste. Installing a hidden Edit menu with the
        // standard editing selectors fixes copy/paste in EVERY text field, app-wide.
        // (The menu never shows — an agent app has no visible menu bar.)
        installEditMenu()
        // Single-instance, newest-wins: terminate any OTHER running GtmuxBar (same
        // bundle id) so a reinstall/update/manual reopen — or a stray copy at
        // /Applications alongside ~/Applications — can never leave two status items in
        // the menu bar. This one is the freshly-launched (newest) bundle, so it stays
        // and the older process goes. The updater's pkill is belt; this is suspenders.
        terminateOtherInstances()
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        if let button = statusItem.button {
            button.target = self
            button.action = #selector(statusItemClicked)
            // Left-click → popover; right-click (or ctrl-click) → context menu with
            // Quit. Without this an LSUIElement app is unquittable except via pkill.
            button.sendAction(on: [.leftMouseUp, .rightMouseUp])
        }

        // Server mode: poll slowly and mirror it into a dedicated indicator.
        serverMode.start()
        // A server-mode change only needs the mark redrawn — the ring is part of it.
        serverMode.$status
            .receive(on: RunLoop.main)
            .sink { [weak self] _ in self?.renderIcon(self?.store.agents ?? []) }
            .store(in: &cancellables)

        popover.behavior = .transient
        // NO frame animation: the list resizes while the popover is OPEN (agents come
        // and go every poll), and NSPopover animating a contentSize change under an
        // NSHostingController mis-draws the SwiftUI content mid-transition — the whole
        // popover rendered shifted/clipped (seen live 2026-08-08). Instant resize also
        // matches the design language (DESIGN.md: 动效最小).
        popover.animates = false
        // The panel's hosting controller is created ONCE and retained by NSPopover, so
        // closing the panel does not tear its SwiftUI graph down — a `.repeatForever`
        // animation inside it goes on running against a window nobody can see. Tell the
        // gate when the panel actually closes, whatever closed it.
        NotificationCenter.default.addObserver(
            forName: NSPopover.didCloseNotification, object: popover, queue: .main) { [weak self] _ in
            self?.lastPopoverClose = Date()
            // The panel's view tree STAYS. Tearing it down on close did stop the ring
            // animating against a hidden window, but it made every re-open lay out from
            // scratch: the first frame measured 168pt inside a window still sized 983 for
            // the previous content, so the panel opened with a band of empty space above
            // it and its last rows past the bottom edge. Measured on a re-open:
            //   menu: root=168 chrome=0 listContent=0    <- first pass, window still 983
            //   menu: root=983 chrome=167 listContent=844 <- settled
            // The animation is stopped by removing the RING instead (PanelVisibility).
            MenuVisibility.shared.setVisible(false)
        }
        popover.contentViewController = makeMenuHost()

        // TELL the popover how tall it is. Without this the panel's own growth never
        // reaches NSPopover, which goes on positioning the window for the size it was
        // first given and lets the surplus run off the top of the screen (PanelSize
        // carries the measurement that proved it). Guarded on a whole-point change: the
        // resize relays out the panel, which re-measures, and an unguarded assignment
        // would ping-pong on sub-pixel differences.
        PanelSize.shared.$height
            .receive(on: RunLoop.main)
            .sink { [weak self] h in
                guard let self, h > 1 else { return }
                let want = PanelMetrics.popoverHeight(panel: h, visible: PanelMetrics.visibleHeight)
                guard abs(self.popover.contentSize.height - want) >= 1 else { return }
                self.popover.contentSize = NSSize(width: Theme.Size.popoverWidth, height: want)
                self.reanchorPopover()
                // Again on the next turn: the window does not always finish moving inside
                // this one, and the check makes a second call free when it already did.
                DispatchQueue.main.async { self.reanchorPopover() }
            }
            .store(in: &cancellables)

        // Repaint the status item whenever agents change.
        store.$agents.receive(on: RunLoop.main)
            .sink { [weak self] in self?.renderIcon($0) }
            .store(in: &cancellables)
        renderIcon([])

        store.refresh()
        store.refreshResource()
        resetTimer()

        // The HQ medallion's resource state (menubar-hq-state-parity) polls the machine
        // tier on a SLOW cadence — it changes on a hardware/human cadence, not per fast
        // tick, and it's a second shell-out — so it rides its own 25s timer, not the
        // agents refresh.
        resourceTimer = Timer.scheduledTimer(withTimeInterval: 25, repeats: true) { [weak self] _ in
            self?.store.refreshResource()
        }

        // Quietly check for a newer release a few seconds after launch (throttled to
        // once/day inside Updater). If one exists, the popover shows a "new version"
        // banner the user can click to install — same effect as `gtmux update`.
        DispatchQueue.main.asyncAfter(deadline: .now() + 4) { Updater.shared.autoCheck() }

        // GTMUXBAR_MEASURE opens and closes the panel by itself, so its geometry can be
        // read without a click — the only way to check WHERE the panel landed on a
        // machine where screen capture is permission-blocked. It repeats because a panel
        // that measures itself can creep a few points per opening, and one opening
        // cannot show that. Pair it with GTMUXBAR_DEBUG=1 and read stderr.
        if ProcessInfo.processInfo.environment["GTMUXBAR_MEASURE"] != nil {
            // The all-panes window sizes itself the same way and needs the same proof.
            DispatchQueue.main.asyncAfter(deadline: .now() + 2) { [weak self] in
                self?.perform(.browsePanes)
            }
            for i in 0..<6 {
                DispatchQueue.main.asyncAfter(deadline: .now() + 3 + Double(i) * 2) { [weak self] in
                    self?.togglePopover()
                }
            }
        }

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
            onJump: { [weak self] pane in self?.jumpFromNotification(pane) },
            onSend: { [weak self] pane, n in self?.sendText(pane, "\(n)") },
            onSendText: { [weak self] pane, text in self?.sendText(pane, text) })

        // Test seam: GTMUXBAR_SHOW_PALETTE auto-opens the palette so it can be
        // exercised without a (flaky) synthetic global keystroke. No-op normally.
        if ProcessInfo.processInfo.environment["GTMUXBAR_SHOW_PALETTE"] != nil {
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) { [weak self] in
                self?.toggleCommandPalette()
            }
        }
        // Test seam: GTMUXBAR_SHOW_PREFS auto-opens the Preferences window — the only
        // interactive route is a status-item click, which a menu-bar manager (e.g. Ice)
        // can hide entirely, making Preferences unreachable for headless verification
        // and documentation screenshots. No-op normally.
        if ProcessInfo.processInfo.environment["GTMUXBAR_SHOW_PREFS"] != nil {
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) { [weak self] in
                guard let self else { return }
                PreferencesController.shared.show(l10n: self.l10n, store: self.store)
            }
        }
        // Test seam: GTMUXBAR_SHOW_BROWSER auto-opens the pane browser window, so it
        // can be verified headlessly (the only interactive route is the ⚙ menu). No-op
        // normally.
        if ProcessInfo.processInfo.environment["GTMUXBAR_SHOW_BROWSER"] != nil {
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) { [weak self] in
                guard let self else { return }
                PaneBrowserController.shared.show(l10n: self.l10n, radar: self.store)
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
            ShareStore.shared.refresh()          // keep the shared-input exposure indicator live
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
        let awake = serverMode.status?.isOn == true
        syncAwakeTimer(awake)
        button.image = StatusItemGlyph.image(mostUrgent: urgent, empty: empty, dark: dark,
                                            awake: awake, phase: awakePhase)
        let badge = displayMode == .dot ? "" : store.badge
        button.title = badge.isEmpty ? "" : " \(badge)"
        button.imagePosition = badge.isEmpty ? .imageOnly : .imageLeft
    }

    /// Install a minimal main menu with a standard Edit submenu so ⌘X/⌘C/⌘V/⌘A route
    /// to the first responder's cut:/copy:/paste:/selectAll: (an LSUIElement app has no
    /// main menu otherwise, so text fields can't paste). The menu never renders — an
    /// agent app shows no menu bar — it exists purely for keyboard-shortcut routing.
    private func installEditMenu() {
        let mainMenu = NSMenu()
        let editItem = NSMenuItem()
        mainMenu.addItem(editItem)
        let edit = NSMenu(title: "Edit")
        editItem.submenu = edit
        let items: [(String, String, String)] = [
            ("Undo", "undo:", "z"),
            ("Redo", "redo:", "Z"),
            ("", "", ""), // separator
            ("Cut", "cut:", "x"),
            ("Copy", "copy:", "c"),
            ("Paste", "paste:", "v"),
            ("Select All", "selectAll:", "a"),
        ]
        for (title, sel, key) in items {
            if sel.isEmpty {
                edit.addItem(.separator())
            } else {
                edit.addItem(NSMenuItem(title: title, action: NSSelectorFromString(sel), keyEquivalent: key))
            }
        }
        NSApp.mainMenu = mainMenu
    }

    /// Route the status-item click by mouse button: right-click (or ctrl-click, the
    /// macOS convention) opens a small context menu — Quit lives there, since an
    /// LSUIElement app has no Dock icon or app menu to quit from — and a plain
    /// left-click toggles the popover as before.
    @objc private func statusItemClicked() {
        let ev = NSApp.currentEvent
        let ctrlClick = ev?.type == .leftMouseUp && ev?.modifierFlags.contains(.control) == true
        if ev?.type == .rightMouseUp || ctrlClick {
            showContextMenu()
        } else {
            togglePopover()
        }
    }

    /// Show the right-click menu. NSStatusItem shows an attached `menu` on ANY
    /// click, which would eat the left-click popover — so attach it only for this
    /// click, pop it via performClick, then detach.
    private func showContextMenu() {
        if popover.isShown { popover.performClose(nil) }
        let menu = NSMenu()
        let quit = NSMenuItem(title: l10n.tr("Quit gtmux", "退出 gtmux"),
                              action: #selector(quitApp), keyEquivalent: "q")
        quit.target = self
        menu.addItem(quit)
        statusItem.menu = menu
        statusItem.button?.performClick(nil)
        statusItem.menu = nil // left-click must keep opening the popover
    }

    /// Quitting hides the server-mode indicator while the STATE persists — the guard
    /// and the heartbeat own it, not this app. So warn, and offer to switch it off,
    /// rather than letting a user quit into a Mac that silently never sleeps with no
    /// visible sign of why. Quitting anyway is fine: `gtmux serve` keeps the heartbeat
    /// and the CLI/phone can still see and end it.
    /// Start the breathing timer when server mode turns on, and — just as important —
    /// tear it down the moment it turns off, so an idle gtmux is idle.
    private func syncAwakeTimer(_ awake: Bool) {
        if awake, awakeTimer == nil {
            awakeTimer = Timer.scheduledTimer(withTimeInterval: 0.125, repeats: true) { [weak self] _ in
                guard let self, let button = self.statusItem.button else { return }
                // 2.4s cycle: slow enough to read as breathing rather than blinking.
                self.awakePhase += 0.125 / 2.4
                if self.awakePhase > 1 { self.awakePhase -= 1 }
                let dark = button.effectiveAppearance.bestMatch(from: [.aqua, .darkAqua]) == .darkAqua
                button.image = StatusItemGlyph.image(
                    mostUrgent: self.store.mostUrgent, empty: self.store.total == 0,
                    dark: dark, awake: true, phase: self.awakePhase)
            }
        } else if !awake, let t = awakeTimer {
            t.invalidate()
            awakeTimer = nil
            awakePhase = 0
        }
    }

    @objc private func quitApp() {
        if serverMode.status?.isOn == true {
            let a = NSAlert()
            a.messageText = l10n.tr("Server mode is still on",
                                    "服务器模式仍然开着")
            a.informativeText = l10n.tr(
                "Quitting hides the indicator, but this Mac will keep running with the lid closed. You can still turn it off with `gtmux awake off` or from your phone.",
                "退出只是隐藏了标记，这台 Mac 仍会合盖继续运行。你之后可以用 `gtmux awake off` 或在手机上关闭它。")
            a.addButton(withTitle: l10n.tr("Turn off and quit", "关闭并退出"))
            a.addButton(withTitle: l10n.tr("Quit anyway", "仍然退出"))
            a.addButton(withTitle: l10n.tr("Cancel", "取消"))
            switch a.runModal() {
            case .alertFirstButtonReturn:
                serverMode.turnOff { NSApp.terminate(nil) }
                return
            case .alertThirdButtonReturn:
                return
            default:
                break
            }
        }
        NSApp.terminate(nil)
    }

    /// A FRESH hosting controller for the panel, built at each open.
    ///
    /// The panel's SwiftUI graph used to be built once and kept by NSPopover for the life
    /// of the app. That is what let the working ring's `.repeatForever` go on animating
    /// against a panel nobody could see: measured on a fresh build, one working agent,
    /// 0.1–0.3 % CPU before the panel was ever opened and 4.6–11 % after opening and
    /// CLOSING it, climbing with every re-open. The commander's 3.5-hour-old menu bar sat
    /// at 20 %, and 87.9 % with surfaces open.
    ///
    /// Telling the ring to stop was tried first and did NOT work: a running repeatForever
    /// keeps going even after its driving state flips and the animation is replaced with
    /// nil (verified — the gate reached `false`, the CPU stayed). What reliably ends an
    /// animation is not having the view. So the graph is built on open and dropped on
    /// close. Nothing visible is lost: what the panel remembers (folded sections, the
    /// language, the watched set) lives in stores that outlive it, not in view state.
    private func makeMenuHost() -> NSHostingController<MenuView> {
        NSHostingController(
            rootView: MenuView(
                store: store, l10n: l10n,
                onJump: { [weak self] in self?.jump($0) },
                onAction: { [weak self] in self?.perform($0) },
                onAdopt: { [weak self] in self?.adopt($0) },
                onSend: { [weak self] in self?.sendReply($0, $1) },
                onUnwatch: { [weak self] in self?.unwatch($0) },
                onClose: { [weak self] in self?.popover.performClose(nil) }))
    }

    /// When the panel last closed — the mouse-DOWN half of a click on the status item
    /// lands here before the action does. See PopoverClick.
    private var lastPopoverClose = Date.distantPast

    @objc private func togglePopover() {
        guard let button = statusItem.button else { return }
        if popover.isShown { popover.performClose(nil); return }
        // A click on the status item while the panel is OPEN never reaches here as a
        // close: the panel is `.transient`, so AppKit dismisses it on mouse-DOWN and our
        // action fires on mouse-UP, by which time `isShown` is already false — we would
        // reopen the panel the user just asked to close. To them that is a click that did
        // nothing, and clicking again (the natural response) closes-and-reopens again.
        if PopoverClick.reopenIsAnEcho(ofCloseAt: lastPopoverClose, now: Date()) { return }
        MenuVisibility.shared.setVisible(true)
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
        logPopoverGeometry()
    }

    /// reanchorPopover puts an already-open panel back under the status item — but ONLY
    /// when it has actually left the screen.
    ///
    /// A resize while the popover is OPEN does not re-attach it. With a big fleet the
    /// first opening lands thousands of points off-screen and only snaps back half a
    /// second later, when some later resize happens to fix it — measured at 300 agents,
    /// the window sat at y=-3504 for 0.5s. Re-showing re-runs AppKit's own positioning,
    /// which is the thing that knows where a popover belongs.
    ///
    /// It is a REPAIR, not a routine: the panel resizes whenever the fleet changes, which
    /// is every poll, and re-showing each time would restart the panel's appearance — and
    /// with it the keyboard selection — under a user who is arrowing through rows. The
    /// check is also what stops the loop, since a re-show is itself a layout.
    private func reanchorPopover() {
        guard popover.isShown, let button = statusItem.button,
              let win = popover.contentViewController?.view.window,
              let screen = button.window?.screen ?? NSScreen.main else { return }
        guard PanelMetrics.hasLeftTheScreen(window: win.frame,
                                            screenFrame: screen.frame,
                                            visibleFrame: screen.visibleFrame) else { return }
        popover.show(relativeTo: button.bounds, of: button, preferredEdge: .minY)
    }

    /// logPopoverGeometry reports where the panel ACTUALLY landed, once layout has
    /// settled. The popover-off-the-top-edge bug (v0.50.0) survived two fixes that were
    /// reasoned about on paper; every hypothesis died on contact with a real number, so
    /// the numbers are readable now: run the app with GTMUXBAR_DEBUG=1.
    private func logPopoverGeometry() {
        guard ProcessInfo.processInfo.environment["GTMUXBAR_DEBUG"] != nil else { return }
        for t in [0.05, 0.15, 0.35, 0.75, 1.5] {
        DispatchQueue.main.asyncAfter(deadline: .now() + t) { [weak self] in
            guard let self else { return }
            let scr = NSScreen.main
            let vf = scr?.visibleFrame ?? .zero
            let f = scr?.frame ?? .zero
            let size = self.popover.contentSize
            let win = self.popover.contentViewController?.view.window?.frame ?? .zero
            // The panel legitimately reaches the top of the display — its arrow tucks
            // under the menu bar — so the ceiling is the screen's own frame, not the
            // menu-bar-excluding visibleFrame. The floor is the Dock.
            let fits = win.minY >= vf.minY && win.maxY <= f.maxY
            dbg(String(format:
                "geom+%.2fs: screen.frame=%.0fx%.0f visible=%.0fx%.0f@y%.0f | content=%.0fx%.0f | window=%.0fx%.0f@y%.0f (top=%.0f) | onScreen=%@",
                t, f.width, f.height, vf.width, vf.height, vf.minY,
                size.width, size.height,
                win.width, win.height, win.minY, win.maxY,
                fits ? "yes" : "NO — clipped"))
        }
        }
    }

    private func jump(_ agent: Agent) {
        popover.performClose(nil)
        GtmuxCLI.spawn(agent.jumpArgs())
    }

    /// Adopt a sensed non-tmux session into tmux: confirm the duplicate-instance
    /// caveat, then `gtmux adopt <session_id>` (resumes the conversation in a fresh
    /// tmux session + terminal tab). The original process is left running.
    private func adopt(_ agent: Agent) {
        guard agent.adoptable, !agent.sessionID.isEmpty else { return }
        let a = NSAlert()
        a.messageText = l10n.tr("Move into tmux?", "转入 tmux？")
        a.informativeText = l10n.tr(
            "Resumes the conversation in a new tmux session and exits the original process. The original terminal tab stays open (empty).",
            "会在新的 tmux session 里恢复该对话，并退出原来的进程。原终端标签页会留着（空的）。")
        a.addButton(withTitle: l10n.tr("Move to tmux", "转入 tmux"))
        a.addButton(withTitle: l10n.tr("Cancel", "取消"))
        if a.runModal() == .alertFirstButtonReturn {
            GtmuxCLI.spawn(["adopt", agent.sessionID])
            popover.performClose(nil)
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.6) { [weak self] in self?.store.refresh() }
        }
    }

    /// In-place reply (A1/A3): send the chosen option to the pane and re-poll soon
    /// so the row flips to working. Stays in the popover (no close).
    private func sendReply(_ agent: Agent, _ n: Int) { sendText(agent.paneID, "\(n)") }

    /// Stop watching a promoted plain pane (tiered-pane-control): drop its watched
    /// marker and re-poll so it leaves the radar's watched section.
    private func unwatch(_ agent: Agent) {
        guard !agent.paneID.isEmpty else { return }
        GtmuxCLI.spawn(["panes", "unwatch", agent.paneID])
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { [weak self] in self?.store.refresh() }
    }

    /// Send literal text to a pane (notification 1/2/3 + free-text reply, A2).
    private func sendText(_ pane: String, _ text: String) {
        guard !pane.isEmpty else { return }
        GtmuxCLI.spawn(["send", pane, text])
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.4) { [weak self] in self?.store.refresh() }
    }

    private func perform(_ action: MenuAction) {
        // Preferences / Pair open a REAL window. Close the (transient, high-window-level)
        // popover FIRST — otherwise the popover stays in front and the window opens hidden
        // behind it, and dismissing the popover later leaves the window orphaned. One
        // surface at a time: the popover gives way to the window (like newSession does).
        switch action {
        case .restore:    GtmuxCLI.spawn(["restore"])
        case .newSession: newSession() // manages its own popover close + prompt
        case .preferences:
            popover.performClose(nil)
            PreferencesController.shared.show(l10n: l10n, store: store)
        case .pairPhone:
            popover.performClose(nil)
            PairingController.shared.show(l10n: l10n)
        case .browsePanes:
            popover.performClose(nil)
            PaneBrowserController.shared.show(l10n: l10n, radar: store)
        case .quit:       quitApp()
        case .startHQ:    GtmuxCLI.spawn(["hq"]) // spawns/focuses the supervisor session + tab
        }
        if action != .quit && action != .preferences && action != .pairPhone && action != .newSession && action != .browsePanes {
            popover.performClose(nil)
        }
    }

    /// New session with a name prompt: close the (transient) popover first, then ask
    /// for an optional session name and run `gtmux new [name]` (blank → tmux
    /// auto-names). The name is sanitized server-side to tmux's rules.
    private func newSession() {
        popover.performClose(nil)
        DispatchQueue.main.async { [weak self] in
            guard let self = self else { return }
            let a = NSAlert()
            a.messageText = self.l10n.tr("New tmux session", "新建 tmux session")
            a.informativeText = self.l10n.tr("Name it, or leave blank to auto-name.",
                                             "取个名字，留空则自动命名。")
            let field = NSTextField(frame: NSRect(x: 0, y: 0, width: 240, height: 24))
            field.placeholderString = self.l10n.tr("session name (optional)", "session 名（可选）")
            a.accessoryView = field
            a.addButton(withTitle: self.l10n.tr("Create", "创建"))
            a.addButton(withTitle: self.l10n.tr("Cancel", "取消"))
            NSApp.activate(ignoringOtherApps: true)
            a.window.initialFirstResponder = field
            if a.runModal() == .alertFirstButtonReturn {
                let name = field.stringValue.trimmingCharacters(in: .whitespacesAndNewlines)
                GtmuxCLI.spawn(name.isEmpty ? ["new"] : ["new", name])
            }
        }
    }

    /// The notification's didReceive does the real jump (to the SPECIFIC pane you
    /// clicked). It stamps the time and cancels any pending reopen `focus --last` so
    /// the generic fallback can't fire on top of it and bounce you to the wrong,
    /// last-finished pane — the "sometimes jumps nowhere / feels slow" (double focus).
    private func jumpFromNotification(_ pane: String) {
        lastNotificationJumpAt = Date()
        pendingReopenFocus?.cancel()
        pendingReopenFocus = nil
        GtmuxCLI.spawn(pane.isEmpty ? ["focus", "--last"] : ["focus", pane])
    }

    /// Terminate every OTHER running instance of this app (matched by bundle id),
    /// leaving only this newest one — so the menu bar never shows duplicate status
    /// items after an update/reinstall or a second copy on disk. `.terminate()` asks
    /// politely; a still-alive straggler after a short grace is force-killed so we
    /// never linger with two icons.
    private func terminateOtherInstances() {
        let me = NSRunningApplication.current
        guard let bundleID = me.bundleIdentifier else { return }
        let others = NSRunningApplication.runningApplications(withBundleIdentifier: bundleID)
            .filter { $0.processIdentifier != me.processIdentifier }
        guard !others.isEmpty else { return }
        dbg("single-instance: terminating \(others.count) older instance(s)")
        for other in others { other.terminate() }
        DispatchQueue.global(qos: .utility).asyncAfter(deadline: .now() + 1.5) {
            for other in others where !other.isTerminated { other.forceTerminate() }
        }
    }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        // Accessory app (no Dock icon) → reopen only fires from a notification click,
        // which also calls didReceive with the specific pane. If that jump just ran,
        // do nothing. Otherwise (reopen arrived first) defer `focus --last` briefly as
        // a fallback so an incoming didReceive can cancel it — the two never race.
        if let t = lastNotificationJumpAt, Date().timeIntervalSince(t) < 1.0 {
            dbg("reopen: notification jump just handled it → skip focus --last")
            return true
        }
        dbg("reopen → deferred focus --last (fallback)")
        let work = DispatchWorkItem { GtmuxCLI.spawn(["focus", "--last"]) }
        pendingReopenFocus = work
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.4, execute: work)
        return true
    }
}
