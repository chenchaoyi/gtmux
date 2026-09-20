import AppKit
import SwiftUI

/// PreferencesController owns the Preferences window (DESIGN §8).
final class PreferencesController {
    static let shared = PreferencesController()
    private var window: NSWindow?

    func show(l10n: L10n, store: AgentStore) {
        if window == nil {
            let w = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: 460, height: 560),
                styleMask: [.titled, .closable], backing: .buffered, defer: false)
            w.contentViewController = NSHostingController(
                rootView: PreferencesView(l10n: l10n, settings: AppSettings.shared, store: store))
            w.isReleasedWhenClosed = false
            w.center()
            window = w
        }
        window?.title = l10n.tr("gtmux Preferences", "gtmux 偏好设置")
        window?.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
}

/// PreferencesView — a grouped macOS settings Form (sectioned cards, the native
/// System-Settings idiom), so preferences read at a glance like Moshi's settings.
struct PreferencesView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var settings: AppSettings
    @ObservedObject var remote = RemoteAccess.shared
    @ObservedObject var serverMode = ServerModeStore.shared
    @State private var showServerModeConfirm = false
    // tab-alert lives in tmux, not in defaults — read once when the pane appears and
    // after every write, so the switch reflects what tmux actually has.
    @State private var tabAlertOn = false
    @State private var tabAlertBusy = false
    @ObservedObject var ent = Entitlements.shared
    @ObservedObject var updater = Updater.shared
    @ObservedObject var share = ShareStore.shared
    @ObservedObject var store: AgentStore
    @ObservedObject var pairStore = PairStore.shared
    @ObservedObject var diag = DiagnosticsStore.shared
    @State private var showPaywall = false
    // Presents the shared DirectCodeSheet (same "Unlock Direct" flow as the pairing
    // window). backendRevert snaps the Standard/Direct picker back when an unlock is
    // canceled (the segmented control renders its tap optimistically).
    @State private var showDirectCode = false
    @State private var backendRevert = 0
    @State private var showPairSheet = false
    @State private var showNewShareSheet = false
    // The share link whose per-link scope editor is expanded ("" = none).
    @State private var expandedLink = ""
    // Collapse state for the two long share lists (default expanded; the header
    // shows a count so a collapsed list still tells you how much is inside).
    @State private var panesExpanded = true
    @State private var linksExpanded = true
    // Revoke is destructive and immediate (a device/link stops working at once), so it
    // goes through a confirmation alert. One target covers both the share-link and the
    // paired-device buttons; nil = no alert.
    @State private var revokeTarget: RevokeTarget?
    // A pending "re-hand this link" — set once the CLI has re-fetched the full URL,
    // then presented as the one-link-three-doors delivery sheet. nil = no sheet.
    @State private var deliverLink: DeliverLink?

    // The re-fetched full URL of an existing guest link + its display name, carried
    // into ShareLinkDeliverySheet.
    private struct DeliverLink: Identifiable {
        let id = UUID()
        let url: String
        let label: String
    }

    // What a pending revoke points at — carries the display name so the alert message
    // can say exactly what's about to be cut off.
    private enum RevokeTarget: Identifiable {
        case share(id: String, label: String)
        case pair(id: String, name: String)
        var id: String {
            switch self {
            case .share(let id, _): return "share:" + id
            case .pair(let id, _): return "pair:" + id
            }
        }
    }

    private var appVersion: String {
        Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "dev"
    }

    // prefLabel mirrors the mobile app's settings rows (SettingsRow's leading outline
    // icon in the secondary color, fixed-width so titles align) — one settings visual
    // language across the two surfaces. Neutral color only: per the design 铁律,
    // color is reserved for agent STATE, so chrome icons stay monochrome secondary.
    private func prefLabel(_ en: String, _ zh: String, symbol: String) -> some View {
        Label {
            Text(l10n.tr(en, zh))
        } icon: {
            Image(systemName: symbol)
                .font(.system(size: 13))
                .foregroundStyle(.secondary)
                .frame(width: 20)
        }
    }

    // A grouped Form (macOS System-Settings idiom) — sectioned cards instead of a
    // flat grid, so the preferences read at a glance like Moshi's settings.
    var body: some View {
        Form {
            // THE DOOR — is this Mac reachable, and how (mode + tunnel backend). It's a
            // SHARED reachability layer: BOTH your own paired devices AND shared
            // collaborators come through it, so it's its own section above Pair and
            // Sharing — not nested under "your devices" (the tunnel Standard/Direct
            // choice governs share links' URLs too, not just pair).
            Section(l10n.tr("Remote access", "远程访问")) {
                // The door: Off / Wi-Fi (free LAN) / Anywhere (Pro tunnel).
                LabeledContent {
                    Picker("", selection: remoteModeBinding) {
                        Text(l10n.tr("Off", "关闭")).tag(RemoteMode.off)
                        Text(l10n.tr("Wi-Fi", "局域网")).tag(RemoteMode.lan)
                        Text(l10n.tr("Anywhere", "任意网络")).tag(RemoteMode.anywhere)
                    }
                    .pickerStyle(.segmented).labelsHidden().disabled(remote.busy)
                } label: {
                    prefLabel("Access", "访问", symbol: "antenna.radiowaves.left.and.right")
                }
                tunnelBackendRow
                // The reachable ADDRESS belongs BELOW the whole door config (access +
                // tunnel), as a summary of "here's where you're reachable" — not wedged
                // between the Access and Tunnel rows.
                Text(remoteSubtitle)
                    .font(.system(size: 11)).foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                // WHY a mode change didn't take. RemoteAccess has always published this,
                // and the pair sheet has always shown it — this pane never did, so a
                // failed switch to Anywhere looked like the confirmation dialog simply
                // vanishing: the picker snapped back and nothing said a word. A control
                // that can fail has to be able to say so where it is.
                if let e = remote.lastError, !e.isEmpty {
                    Text(e)
                        .font(.system(size: 11))
                        .foregroundStyle(Color(Theme.Status.errored))
                        .fixedSize(horizontal: false, vertical: true)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .textSelection(.enabled)
                }
                connectedDevices
            }

            // YOUR DEVICES (PAIR) — your own phone / browser / terminal, full control.
            // They reach the Mac through the door above.

            Section(l10n.tr("Your devices · Pair", "我的设备 · 配对")) {
                pairSection
            }

            // SHARE — collaborators, least privilege, per-link scope.
            Section(l10n.tr("Sharing", "分享")) {
                // A refusal nobody can see reads as a broken app. The Mac has always
                // turned away a grant whose pane ids were made against a tmux that has
                // since restarted — this is the part that says so, instead of showing a
                // scope that looks like it still works.
                if share.grantsStale {
                    Text(l10n.tr("tmux restarted, so these grants no longer apply and links are being refused. Re-pick the panes to restore them.",
                                 "tmux 重启过，这些授权已失效，链接目前一律被拒。重新勾选一次即可恢复。"))
                        .font(.system(size: 11)).foregroundStyle(Theme.Status.waiting)
                        .fixedSize(horizontal: false, vertical: true)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                Toggle(isOn: shareEnabledBinding) {
                    prefLabel("Let a collaborator type into the terminal",
                              "允许协作者向终端输入", symbol: "keyboard")
                }.disabled(share.busy)
                Text(shareSubtitle)
                    .font(.system(size: 11)).foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                shareGuestLinks
            }

            Section(l10n.tr("Notifications", "通知")) {
                Toggle(isOn: $settings.notifications) {
                    prefLabel("Notify when an agent waits or finishes", "agent 开始等你、或者干完了就提醒", symbol: "bell")
                }
                // Terminal-tab marker. NOT backed by UserDefaults like its neighbours:
                // the truth lives in tmux's own `set-titles-string`, so the CLI is both
                // the reader and the writer. A defaults-backed mirror would be a second
                // opinion about the same fact, and the two would drift the moment the
                // user ran `gtmux config tab-alert` in a shell.
                Toggle(isOn: Binding(
                    get: { tabAlertOn },
                    set: { setTabAlert($0) })) {
                    prefLabel("Mark the terminal tab when an agent waits",
                              "有 agent 在等你时，标记其终端标签", symbol: "macwindow.badge.plus")
                }
                .disabled(tabAlertBusy)
                Text(l10n.tr("Adds a ● in front of that session's tab title. Your own title format is kept, and turning this off restores it.",
                             "在该 session 的标签标题前加一个 ●。你原来的标题格式会保留，关闭即还原。"))
                    .font(.system(size: 10)).foregroundStyle(.secondary)
            }

            // THIS MAC — what the machine itself does: does it stay awake, and what it
            // puts in the menu bar. Server mode used to be a titled section of its own
            // and the status-bar settings another, which made two one-row sections about
            // the same machine sit on opposite sides of the window.
            Section(l10n.tr("This Mac", "这台 Mac")) {
                serverModeRow
                Divider()
                Picker(selection: $settings.displayMode) {
                    Text(l10n.tr("Dot + count", "点 + 数字")).tag(DisplayMode.dotCount)
                    Text(l10n.tr("Dot only", "仅圆点")).tag(DisplayMode.dot)
                    Text(l10n.tr("Hide when idle", "空闲时隐藏")).tag(DisplayMode.hideWhenIdle)
                } label: {
                    prefLabel("Menu bar shows", "菜单栏显示", symbol: "menubar.rectangle")
                }
                LabeledContent {
                    HStack {
                        Slider(value: $settings.refreshInterval, in: 0.5...5.0, step: 0.5).frame(width: 170)
                        Text(String(format: "%.1fs", settings.refreshInterval))
                            .font(.system(size: 11, design: .monospaced)).foregroundStyle(.secondary)
                    }
                } label: {
                    prefLabel("Refresh", "刷新间隔", symbol: "arrow.clockwise")
                }
            }

            Section(l10n.tr("General", "通用")) {
                Picker(selection: $l10n.mode) {
                    Text(l10n.tr("System", "跟随系统")).tag(LangMode.system)
                    Text("English").tag(LangMode.en)
                    Text("中文").tag(LangMode.zh)
                } label: {
                    prefLabel("Language", "语言", symbol: "globe")
                }
                Toggle(isOn: $settings.launchAtLogin) {
                    prefLabel("Launch at login", "开机自启", symbol: "power")
                }
                LabeledContent {
                    HStack(spacing: 8) {
                        Text("⌘⌥G").font(.system(size: 12, weight: .medium))
                            .padding(.horizontal, 8).padding(.vertical, 3)
                            .background(RoundedRectangle(cornerRadius: 6).stroke(.secondary.opacity(0.4)))
                        Text(l10n.tr("opens the popover", "打开 popover"))
                            .font(.system(size: 11)).foregroundStyle(.secondary)
                    }
                } label: {
                    prefLabel("Global hotkey", "全局热键", symbol: "command")
                }
            }


            // DIAGNOSTICS — the app is where a person is when something breaks, and
            // until now the only way to see what gtmux had recorded was to know the
            // command and have a terminal open. Three rows: look at it, pack it, or
            // record more of it while chasing something.
            Section(l10n.tr("Diagnostics", "诊断")) {
                diagnosticsRows
            }

            Section(l10n.tr("Software update", "软件更新")) {
                updateRow
            }
        }
        .sheet(isPresented: $showServerModeConfirm) {
            ServerModeConfirmView(
                l10n: l10n,
                unverifiedOS: serverMode.status?.platform.verified == false
                    ? (serverMode.status?.platform.osVersion ?? "?") : nil,
                onConfirm: {
                    showServerModeConfirm = false
                    serverMode.turnOn { ok, err in
                        if !ok { reportServerModeFailure(err) }
                    }
                },
                onCancel: { showServerModeConfirm = false })
        }
        .formStyle(.grouped)
        .frame(width: 460, height: 640)
        .onAppear { remote.refresh(); share.refresh(); share.loadDetail(); pairStore.refresh(); updater.autoCheck(); serverMode.refresh(); refreshTabAlert(); diag.refreshStats() }
        .sheet(isPresented: $showPairSheet) {
            PairDeviceSheet(l10n: l10n) { showPairSheet = false; pairStore.refresh() }
        }
        .sheet(isPresented: $showNewShareSheet) {
            NewShareSheet(l10n: l10n, share: share, store: store) { showNewShareSheet = false }
        }
        .sheet(item: $deliverLink) { d in
            ShareLinkDeliverySheet(l10n: l10n, label: d.label, url: d.url) { deliverLink = nil }
        }
        .sheet(isPresented: $showPaywall) {
            PaywallView(l10n: l10n,
                        onUnlock: { ent.unlockFree(); showPaywall = false; confirmAnywhere() },
                        onClose: { showPaywall = false })
        }
        .sheet(isPresented: $showDirectCode) {
            DirectCodeSheet(l10n: l10n, remote: remote, isPresented: $showDirectCode)
        }
        .alert(
            l10n.tr("Revoke access?", "吊销访问？"),
            isPresented: Binding(get: { revokeTarget != nil },
                                 set: { if !$0 { revokeTarget = nil } }),
            presenting: revokeTarget
        ) { target in
            Button(l10n.tr("Revoke", "吊销"), role: .destructive) {
                switch target {
                case .share(let id, _): share.revoke(id)
                case .pair(let id, _): pairStore.revoke(id)
                }
            }
            Button(l10n.tr("Cancel", "取消"), role: .cancel) {}
        } message: { target in
            switch target {
            case .share(_, let label):
                Text(l10n.tr("“\(label)” stops working immediately. Anyone holding this link loses access.",
                             "“\(label)”将立即失效。持有该链接的人会失去访问权限。"))
            case .pair(_, let name):
                Text(l10n.tr("“\(name)” stops working immediately and must be paired again to reconnect.",
                             "“\(name)”将立即失效，需重新配对才能再次连接。"))
            }
        }
    }

    // WHO is connected right now — paired phones by name, browsers as anonymous
    // "Safari · macOS" rows. Hidden entirely when nobody's viewing, so the section
    // stays quiet at rest (matches the "idle 静" ethos). A phone icon vs a globe
    // reuses the phone/browser distinction from the popover indicator.
    @ViewBuilder private var connectedDevices: some View {
        let list = remote.remoteClientList
        if !list.isEmpty {
            Divider()
            VStack(alignment: .leading, spacing: 6) {
                Text(l10n.tr("Connected now", "当前已连接"))
                    .font(.system(size: 11, weight: .medium)).foregroundStyle(.secondary)
                ForEach(list) { c in
                    HStack(spacing: 8) {
                        Image(systemName: c.isPhone ? "iphone" : "globe")
                            .font(.system(size: 12)).foregroundStyle(Theme.Status.idle)
                            .frame(width: 16)
                        Text(c.title(l10n.tr)).font(.system(size: 12))
                        let sub = c.subtitle(l10n.tr)
                        if !sub.isEmpty {
                            Text(sub).font(.system(size: 10)).foregroundStyle(.tertiary)
                        }
                        Spacer(minLength: 0)
                    }
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    // ONE row does the whole job (mirrors macOS System Settings › Software Update):
    // the version + a status word live on the same line, and re-checking is a
    // borderless ⟳ — no separate "Check for updates" button on its own line. Only a
    // genuinely actionable state (an update is ready to install, or it failed) earns
    // its own emphasized row with a prominent button.
    @ViewBuilder private var serverModeRow: some View {
        let st = serverMode.status
        let on = st?.isOn ?? false
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 8) {
                Image(systemName: "laptopcomputer")
                    .foregroundStyle(on ? (st?.needsAttention == true
                                           ? Theme.Status.waiting : Theme.Status.idle)
                                        : Color.secondary)
                VStack(alignment: .leading, spacing: 1) {
                    // The row names the setting and then says which way it is, so the
                    // section heading does not have to carry the name for it. What it
                    // IS stays a tooltip: one sentence, and only for whoever needs it.
                    HStack(spacing: 4) {
                        Text(l10n.tr("Server mode", "服务器模式")).font(.system(size: 12))
                        Image(systemName: "questionmark.circle")
                            .font(.system(size: 12))
                            .foregroundStyle(.tertiary)
                            .help(l10n.tr(
                                "Keeps this Mac running with the lid closed, so an agent can finish what it is doing and your phone can still reach it. It stays on until you turn it off, and below 20% battery it starts sleeping again on its own.",
                                "让这台 Mac 合上盖子也继续跑，正在干活的 agent 能干完，手机也还连得上。开了就一直开着，直到你自己关掉；电量掉到 20% 以下，它自己恢复睡眠。"))
                    }
                    Text(on ? l10n.tr("On: the lid can close and the agents keep running",
                                      "开着：合上盖子，agent 继续跑")
                            : l10n.tr("Off: closing the lid sleeps this Mac",
                                      "关着：合上盖子这台 Mac 就睡了"))
                        .font(.system(size: 11)).foregroundStyle(.secondary)
                    if let sub = serverModeDetail(st) {
                        Text(sub).font(.system(size: 10)).foregroundStyle(.tertiary)
                    }
                }
                Spacer()
                if on {
                    Button(l10n.tr("Turn off", "关闭")) {
                        serverMode.turnOff {}
                    }
                } else {
                    // Enabling is the only action that needs a password, so it is the
                    // only one that gets an explainer first (DESIGN §17): the limits
                    // are stated BEFORE the system dialog, never after.
                    Button(l10n.tr("Turn on…", "开启…")) { confirmServerModeOn() }
                        .disabled(st?.platform.ok == false)
                }
            }
            if let p = st?.platform, !p.ok || !p.verified {
                Text(serverModePlatformNote(p))
                    .font(.system(size: 10))
                    .foregroundStyle(p.ok ? .secondary : Color(Theme.Status.waitingNS))
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
    }

    // DIAGNOSTICS. Three rows, in the order a problem is actually worked: see what
    // gtmux recorded, hand it to someone, and (only while chasing something) record
    // more of it. Every one of them is a CLI call — `gtmux logs`, `gtmux doctor
    // --bundle`, `gtmux config debug` — so what the window shows and what a terminal
    // shows are the same store read the same way.
    @ViewBuilder private var diagnosticsRows: some View {
        let st = diag.stats
        HStack(spacing: 8) {
            Image(systemName: "doc.text.magnifyingglass")
                .font(.system(size: 13))
                .foregroundStyle(st.problems > 0 ? Theme.Status.errored : Color.secondary)
                .frame(width: 20)
            VStack(alignment: .leading, spacing: 1) {
                Text(l10n.tr("What gtmux recorded", "gtmux 记下了什么")).font(.system(size: 12))
                Text(recordedSubtitle(st)).font(.system(size: 10)).foregroundStyle(.tertiary)
            }
            Spacer()
            Button(l10n.tr("Open", "打开")) { DiagnosticsController.shared.show(l10n: l10n) }
        }

        HStack(spacing: 8) {
            Image(systemName: "shippingbox")
                .font(.system(size: 13)).foregroundStyle(.secondary).frame(width: 20)
            VStack(alignment: .leading, spacing: 1) {
                Text(l10n.tr("Report a problem", "报告一个问题")).font(.system(size: 12))
                Text(l10n.tr("Packs that record, the status files and this Mac's versions into one file. Tokens are replaced; nothing is sent anywhere.",
                             "把这份记录、状态文件和这台 Mac 的各个版本打成一个文件。token 会被替换掉，不会发去任何地方。"))
                    .font(.system(size: 10)).foregroundStyle(.tertiary)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Spacer()
            if diag.packing {
                ProgressView().controlSize(.small)
            } else {
                Button(l10n.tr("Pack…", "打包…")) { diag.pack() }
            }
        }
        if let path = diag.packed {
            // Where it went, and one click to it: a file on the Desktop nobody names is
            // a file nobody finds.
            HStack(spacing: 8) {
                Image(systemName: "checkmark.circle").foregroundStyle(Theme.Status.idle)
                Text(l10n.tr("Saved to the Desktop as \((path as NSString).lastPathComponent)",
                             "已存到桌面：\((path as NSString).lastPathComponent)"))
                    .font(.system(size: 11)).foregroundStyle(.secondary)
                Spacer()
                Button(l10n.tr("Show", "显示")) {
                    NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: path)])
                }
                .buttonStyle(.link).font(.system(size: 11))
            }
        }
        if let e = diag.packError {
            Text(e).font(.system(size: 11)).foregroundStyle(Color(Theme.Status.errored))
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, alignment: .leading)
                .textSelection(.enabled)
        }

        Toggle(isOn: Binding(get: { diag.debugOn }, set: { diag.setDebug($0) })) {
            prefLabel("Record extra detail", "多记一些细节", symbol: "waveform")
        }
        Text(l10n.tr("For chasing something specific. Each gtmux process picks it up when it next starts, so turn it off again when you are done.",
                     "为追某个具体问题用。各个 gtmux 进程下次启动时才会生效；追完了记得关掉。"))
            .font(.system(size: 10)).foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
    }

    // What the record row says about itself. Problems lead when there are any: the
    // reason someone is reading this row is that they suspect one.
    private func recordedSubtitle(_ st: LogStats) -> String {
        if st.files == 0 {
            return l10n.tr("Nothing recorded yet", "还没有记录")
        }
        let size = ByteCountFormatter.string(fromByteCount: st.bytes, countStyle: .file)
        let kept = l10n.tr("\(size), kept \(st.retainDays) days", "\(size)，保留 \(st.retainDays) 天")
        if st.problems == 0 { return kept }
        let trouble = st.errors > 0
            ? l10n.tr("\(st.problems) problems today", "今天 \(st.problems) 个问题")
            : l10n.tr("\(st.warnings) warnings today", "今天 \(st.warnings) 条警告")
        return kept + " · " + trouble
    }

    private func serverModeDetail(_ st: ServerModeStatus?) -> String? {
        guard let st, st.isOn else { return nil }
        var parts: [String] = []
        if let since = st.since {
            let m = max(0, Int(Date().timeIntervalSince1970) - since) / 60
            parts.append(l10n.tr("on for \(m < 60 ? "\(m)m" : "\(m/60)h\(m%60)m")",
                                 "已开启 \(m < 60 ? "\(m)分钟" : "\(m/60)小时\(m%60)分")"))
        }
        if st.power == "battery", let pct = st.batteryPct {
            parts.append(l10n.tr("battery \(pct)% · sleep returns at 20%",
                                 "电池 \(pct)% · 到 20% 自动恢复睡眠"))
        }
        if let r = st.attentionReason { parts.append("⚠ " + r) }
        return parts.isEmpty ? nil : parts.joined(separator: " · ")
    }

    private func serverModePlatformNote(_ p: ServerModeStatus.Platform) -> String {
        if !p.ok {
            return l10n.tr("Not supported on this system. gtmux will not manage a sleep setting it cannot verify.",
                           "此系统不支持。gtmux 不会去管理一个它无法验证的睡眠设置。")
        }
        return l10n.tr("Unverified on macOS \(p.osVersion ?? "?"). It relies on an undocumented setting. Verify once: turn it on, shut the lid for two minutes, check it stayed reachable.",
                       "macOS \(p.osVersion ?? "?") 未经验证。它依赖一项未公开文档的系统设置。请验证一次：开启后合盖两分钟，再看是否仍然连得上。")
    }

    /// Presenting the explainer. The dialog itself is ServerModeConfirmView — a sheet
    /// rather than a widened NSAlert, which centres its title once it gets wide and
    /// ends up mixing alignments.
    private func confirmServerModeOn() { showServerModeConfirm = true }

    /// Failures stay an NSAlert: a short message in a narrow alert is exactly what
    /// that control is for, and it is the one shape macOS lays out well by default.
    private func reportServerModeFailure(_ err: String) {
        let f = NSAlert()
        f.alertStyle = .warning
        f.messageText = l10n.tr("Server mode could not be turned on", "服务器模式未能开启")
        f.informativeText = err.isEmpty ? l10n.tr("Nothing was changed.", "什么都没有改动。") : err
        f.runModal()
    }

    @ViewBuilder private var updateRow: some View {
        switch updater.state {
        case .available(let v):
            HStack(spacing: 8) {
                Image(systemName: "arrow.down.circle.fill").foregroundStyle(Theme.Status.working)
                VStack(alignment: .leading, spacing: 1) {
                    Text(l10n.tr("Update available", "有可用更新")).font(.system(size: 12))
                    Text("\(appVersion) → \(v)")
                        .font(.system(size: 10, design: .monospaced)).foregroundStyle(.tertiary)
                }
                Spacer()
                Button(l10n.tr("Update now", "立即更新")) { updater.install() }
                    .buttonStyle(.borderedProminent)
            }
        case .updating:
            HStack(spacing: 8) {
                ProgressView().controlSize(.small)
                Text(l10n.tr("Updating… the app will relaunch when done", "正在更新…完成后会自动重启"))
                    .font(.system(size: 12)).foregroundStyle(.secondary)
            }
        case .updateFailed:
            HStack(spacing: 8) {
                Image(systemName: "exclamationmark.triangle.fill").foregroundStyle(Theme.Status.waiting)
                VStack(alignment: .leading, spacing: 1) {
                    Text(l10n.tr("Update failed", "更新失败")).font(.system(size: 12))
                    if let e = updater.lastError, !e.isEmpty {
                        Text(e).font(.system(size: 10)).foregroundStyle(.tertiary)
                            .lineLimit(1).truncationMode(.tail)
                    }
                }
                Spacer()
                Button(l10n.tr("Retry", "重试")) { updater.install() }
                    .buttonStyle(.borderedProminent)
            }
        default:
            // Idle / checking / up-to-date → collapse into the version row.
            LabeledContent {
                HStack(spacing: 8) {
                    Text(appVersion).font(.system(size: 12, design: .monospaced)).foregroundStyle(.secondary)
                    if let s = checkStatusText {
                        Text("·").foregroundStyle(.tertiary)
                        Text(s).font(.system(size: 11)).foregroundStyle(.tertiary)
                    }
                    if isChecking {
                        ProgressView().controlSize(.small)
                    } else {
                        Button { updater.check() } label: {
                            Image(systemName: "arrow.clockwise")
                        }
                        .buttonStyle(.borderless)
                        .help(l10n.tr("Check for updates", "检查更新"))
                    }
                }
            } label: {
                prefLabel("Current version", "当前版本", symbol: "info.circle")
            }
        }
    }

    private var isChecking: Bool { if case .checking = updater.state { return true }; return false }
    private var checkStatusText: String? {
        switch updater.state {
        case .upToDate: return l10n.tr("Up to date", "已是最新")
        case .failed: return l10n.tr("Check failed, try again", "检查失败，请重试")
        default: return nil
        }
    }

    private var remoteModeBinding: Binding<RemoteMode> {
        Binding(
            get: { remote.mode },
            set: { m in
                switch m {
                case .off: remote.turnOff()
                case .lan: remote.enableLan()
                case .anywhere: ent.isPro ? confirmAnywhere() : (showPaywall = true)
                }
            })
    }

    private var remoteSubtitle: String {
        switch remote.mode {
        case .off:
            return ent.isPro
                ? l10n.tr("Phone access is off.", "手机访问已关闭。")
                : l10n.tr("Off. “Anywhere” is a Pro feature.", "已关闭。“任意网络”为 Pro 功能。")
        case .lan:
            return l10n.tr("Reachable on the same Wi-Fi.", "同一 Wi-Fi 下可达。")
        case .anywhere:
            return remote.url ?? l10n.tr("Reachable from anywhere (always-on).", "任意网络可达（常驻）。")
        }
    }

    // TUNNEL BACKEND — "Anywhere" reaches the Mac over a tunnel, and there are two:
    // Standard (zero-config hosted Cloudflare) and Direct (a chisel tunnel straight over
    // 443 — access-code unlock, or self-hosted). ALWAYS offer the Standard | Direct switch
    // (matches the pairing window): picking Direct on a Mac that hasn't unlocked it opens
    // the shared DirectCodeSheet, so Settings and pairing no longer diverge (Settings used
    // to be read-only here, pointing you at the CLI).
    @ViewBuilder private var tunnelBackendRow: some View {
        if remote.mode == .anywhere {
            LabeledContent {
                Picker("", selection: backendBinding) {
                    Text(l10n.tr("Standard", "标准")).tag(TunnelBackend.cloudflare)
                    Text(l10n.tr("Direct", "直连")).tag(TunnelBackend.selfHosted)
                }
                .id(backendRevert)
                .pickerStyle(.segmented).labelsHidden().disabled(remote.busy)
            } label: {
                prefLabel("Tunnel", "隧道", symbol: "network")
            }
            Text(backendSubtitle)
                .font(.system(size: 11)).foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    // Switching backend re-runs the tunnel service on the chosen backend (both are the
    // already-consented "Anywhere" exposure, so no extra confirm — the user picked it).
    // Direct is the paid tunnel: if it isn't unlocked on this Mac yet, picking it opens the
    // shared "Unlock Direct" sheet instead of switching (backendRevert snaps the control
    // back so it doesn't rest on Direct while Standard is what's actually running).
    private var backendBinding: Binding<TunnelBackend> {
        Binding(
            get: { remote.backend == .selfHosted ? .selfHosted : .cloudflare },
            set: { b in
                if b == .selfHosted && !remote.selfTunnelConfigured {
                    showDirectCode = true
                    backendRevert += 1
                    return
                }
                remote.enableAnywhere(selfHosted: b == .selfHosted)
            })
    }

    private var backendSubtitle: String {
        // Direct not yet unlocked on this Mac → say so + how (picking Direct opens the
        // unlock sheet). Otherwise describe the active backend.
        if !remote.selfTunnelConfigured {
            return l10n.tr("Standard is a hosted tunnel that needs no setup. Direct goes straight over port 443, for networks that block the hosted one, and needs an access code; pick Direct to unlock (or self-host).",
                           "标准是免配置的托管隧道。直连走 443 端口直达，用在屏蔽了托管隧道的网络，需要访问码；选「直连」即可解锁（或自托管）。")
        }
        switch remote.backend {
        case .selfHosted:
            return l10n.tr("Direct: straight over port 443 (works where the hosted tunnel is blocked).", "直连：走 443 端口（屏蔽了托管隧道的网络也可达）。")
        case .cloudflare:
            return l10n.tr("Standard: a hosted tunnel that needs no setup.", "标准：免配置的托管隧道。")
        case .none:
            return l10n.tr("Bringing the tunnel up…", "隧道启动中…")
        }
    }

    // MARK: shared input (web-shared input host controls — mirrors `gtmux share`)

    private var shareEnabledBinding: Binding<Bool> {
        Binding(get: { share.enabled }, set: { share.setEnabled($0) })
    }

    private var shareSubtitle: String {
        if !share.enabled {
            return l10n.tr("Off: anyone with a share link can look but not type.",
                           "已关闭：持分享链接的人只能看，不能输入。")
        }
        if share.allowedPanes.isEmpty {
            return l10n.tr("On, but no panes are allowed yet. Tick a pane below.",
                           "已开启，但还没允许任何 pane，请在下方勾选。")
        }
        return l10n.tr("On: a guest with a share link can type into the ticked panes.",
                       "已开启：持分享链接的访客可向勾选的 pane 输入。")
    }

    // The allowlist, rendered from the LIVE agent list (tmux panes only — a guest
    // types via tmux send-keys, so native/hook-less rows can't be targets). Each row
    // mirrors the session-list identity — AgentAvatar (icon + state) + the agent's own
    // session title (`primary`) + dim `session · %pane` — and carries TWO independent
    // controls: 👁 See (the guest may VIEW the pane) and ⌨️ Type (the guest may type
    // into it). Type is disabled unless See is on, since input ⊆ view — a guest can
    // never type into a pane it can't see.
    // One permission column: icon + word on the left, checkbox pinned right, in a
    // fixed width so See/Type line up as columns across every row.
    @ViewBuilder
    private func permissionCell(icon: String, label: String, isOn: Binding<Bool>,
                                disabled: Bool, help: String) -> some View {
        HStack(spacing: 4) {
            Image(systemName: icon).font(.system(size: 12)).foregroundStyle(.secondary)
            Text(label).font(.system(size: 11)).foregroundStyle(.secondary)
            Spacer(minLength: 6)
            Toggle("", isOn: isOn).labelsHidden().toggleStyle(.checkbox)
        }
        .frame(width: 78)
        .disabled(disabled)
        .help(help)
    }

    // A tappable collapse header: chevron + title + a dim count, so a collapsed
    // list still says how much it holds. Used by both long share lists.
    @ViewBuilder
    private func collapseHeader(_ title: String, count: Int, expanded: Binding<Bool>) -> some View {
        Button { expanded.wrappedValue.toggle() } label: {
            HStack(spacing: 4) {
                Image(systemName: expanded.wrappedValue ? "chevron.down" : "chevron.right")
                    .font(.system(size: 12, weight: .semibold)).foregroundStyle(.secondary)
                    .frame(width: 10)
                Text(title).font(.system(size: 11, weight: .medium)).foregroundStyle(.secondary)
                Text("\(count)").font(.system(size: 10)).foregroundStyle(.tertiary)
                    .padding(.horizontal, 5).padding(.vertical, 1)
                    .background(Capsule().fill(Color.secondary.opacity(0.12)))
                Spacer(minLength: 0)
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }

    // Existing guest links (revocable), a "New link" button (mints + copies the URL),
    // and — right after minting — the fresh link, shown + selectable so the host can
    // re-copy it to send to a collaborator.
    @ViewBuilder private var shareGuestLinks: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 6) {
                if share.guests.isEmpty {
                    Text(l10n.tr("Share links", "分享链接"))
                        .font(.system(size: 11, weight: .medium)).foregroundStyle(.secondary)
                } else {
                    collapseHeader(l10n.tr("Share links", "分享链接"),
                                   count: share.guests.count, expanded: $linksExpanded)
                }
                Spacer(minLength: 8)
                // One-step creation: name + per-link scope in the same sheet.
                Button(l10n.tr("New share…", "新建分享…")) { showNewShareSheet = true }
                    .disabled(share.busy)
            }
            if share.guests.isEmpty {
                Text(l10n.tr("No links yet. Create one to invite a collaborator.",
                             "还没有链接。新建一个邀请协作者。"))
                    .font(.system(size: 11)).foregroundStyle(.tertiary)
            } else if linksExpanded {
                ForEach(share.guests) { g in
                    VStack(alignment: .leading, spacing: 4) {
                        HStack(spacing: 8) {
                            Button {
                                expandedLink = (expandedLink == g.id) ? "" : g.id
                            } label: {
                                Image(systemName: expandedLink == g.id ? "chevron.down" : "chevron.right")
                                    .font(.system(size: 12, weight: .semibold)).foregroundStyle(.secondary)
                                    .frame(width: 10)
                            }.buttonStyle(.plain)
                            Image(systemName: "link").font(.system(size: 12))
                                .foregroundStyle(.secondary).frame(width: 14)
                            VStack(alignment: .leading, spacing: 1) {
                                Text(g.label.isEmpty ? l10n.tr("Share link", "分享链接") : g.label)
                                    .font(.system(size: 12))
                                Text(shareLinkAge(g.enrolledAt) + "  ·  " + linkScopeSummary(g))
                                    .font(.system(size: 10)).foregroundStyle(.tertiary)
                                // WHO used it. The rows above say what the link PERMITS;
                                // a link handed to someone else is only really answered by
                                // whether anyone walked through it, and from where.
                                Text(g.usage(now: Int(Date().timeIntervalSince1970), tr: l10n.tr))
                                    .font(.system(size: 10)).foregroundStyle(.tertiary)
                            }
                            Spacer(minLength: 8)
                            // Show + Revoke as ONE tidy trailing group with matching
                            // chrome (both bordered). "Show" re-opens the full delivery
                            // panel (QR + browser + terminal, each with its own copy) for
                            // this link — a far stronger hand-off than a bare URL copy.
                            HStack(spacing: 6) {
                                Button {
                                    let lbl = g.label.isEmpty ? l10n.tr("Share link", "分享链接") : g.label
                                    share.fetchLinkURL(g.id) { url in
                                        if let url = url { deliverLink = DeliverLink(url: url, label: lbl) }
                                    }
                                } label: {
                                    Image(systemName: "qrcode")
                                }
                                .buttonStyle(.bordered)
                                .disabled(share.busy)
                                .help(l10n.tr("Show the link: QR, browser, terminal", "展示链接：二维码 / 浏览器 / 终端"))
                                Button(l10n.tr("Revoke", "吊销")) {
                                    revokeTarget = .share(id: g.id,
                                                          label: g.label.isEmpty ? l10n.tr("Share link", "分享链接") : g.label)
                                }
                                    .buttonStyle(.bordered)
                                    .disabled(share.busy)
                            }
                        }
                        if expandedLink == g.id {
                            linkScopeEditor(g)
                                .padding(.leading, 24)
                        }
                    }
                }
            }
            if let link = share.lastMintedLink, !link.isEmpty {
                Divider()
                VStack(alignment: .leading, spacing: 3) {
                    Text(l10n.tr("New link, copied to clipboard:", "新链接，已复制到剪贴板："))
                        .font(.system(size: 10)).foregroundStyle(.secondary)
                    Text(link).font(.system(size: 11, design: .monospaced))
                        .foregroundStyle(.secondary).textSelection(.enabled)
                        .lineLimit(2).truncationMode(.middle)
                }
            }
        }.frame(maxWidth: .infinity, alignment: .leading)
    }

    // ── PAIR section (your own devices, full control) ────────────────────────

    @ViewBuilder private var pairSection: some View {
        if pairStore.devices.isEmpty {
            Text(l10n.tr("No paired devices yet. Pair your phone, a browser, or another computer's terminal.",
                         "还没有配对设备。可以配对你的手机、浏览器，或另一台电脑的终端。"))
                .font(.system(size: 11)).foregroundStyle(.secondary)
        } else {
            ForEach(pairStore.devices) { d in
                HStack(spacing: 8) {
                    Image(systemName: d.kind).font(.system(size: 12))
                        .foregroundStyle(.secondary).frame(width: 16)
                    VStack(alignment: .leading, spacing: 1) {
                        Text(d.displayName).font(.system(size: 12))
                        Text(pairLastSeen(d))
                            .font(.system(size: 10)).foregroundStyle(.tertiary)
                    }
                    Spacer(minLength: 0)
                    Button(l10n.tr("Revoke", "吊销")) {
                        revokeTarget = .pair(id: d.id, name: d.displayName)
                    }
                        .disabled(pairStore.busy)
                }
            }
        }
        HStack {
            Spacer()
            Button(l10n.tr("Pair a device…", "配对新设备…")) { showPairSheet = true }
        }
    }

    /// The row's second line. It used to be the last-seen time ALONE, while the phone app
    /// had been showing the platform for months off the same payload the menu bar simply
    /// did not decode — two surfaces, one dataset, different answers. One builder now,
    /// living next to the model so neither surface can drift from it again.
    private func pairLastSeen(_ d: PairedDevice) -> String {
        d.subtitle(now: Int(Date().timeIntervalSince1970), tr: l10n.tr)
    }

    // ── per-link scope editor (SHARE section) ─────────────────────────────────

    // linkScopeSummary renders a link's grant: "2 See · 1 Type" (+ expiry).
    private func linkScopeSummary(_ g: GuestLink) -> String {
        var s = "\(g.viewPanes.count) See · \(g.inputPanes.count) Type"
        if g.expiresAt > 0 {
            let left = g.expiresAt - Int(Date().timeIntervalSince1970)
            s += left <= 0 ? l10n.tr(" · expired", " · 已过期")
                           : l10n.tr(" · expires in ", " · 剩 ") + relativeTime(Int(Date().timeIntervalSince1970) - left, now: Int(Date().timeIntervalSince1970))
        }
        return s
    }

    // linkScopeEditor edits ONE link's See/Type per session — `share set`, never
    // the legacy global (broadcast) forms.
    @ViewBuilder private func linkScopeEditor(_ g: GuestLink) -> some View {
        let panes = store.shareablePanes
        if panes.isEmpty {
            Text(l10n.tr("No tmux panes to share right now.", "当前没有可分享的 tmux pane。"))
                .font(.system(size: 11)).foregroundStyle(.secondary)
        } else {
            VStack(alignment: .leading, spacing: 4) {
                ForEach(panes) { a in
                    HStack(spacing: 8) {
                        AgentAvatar(agent: a)
                        VStack(alignment: .leading, spacing: 1) {
                            Text(a.primary.isEmpty ? (a.agent.isEmpty ? a.paneID : a.agent) : a.primary)
                                .font(Theme.Font.session).lineLimit(1).truncationMode(.tail)
                            Text(a.secondary)
                                .font(Theme.Font.window).foregroundStyle(.secondary)
                                .lineLimit(1).truncationMode(.tail)
                        }
                        Spacer(minLength: 10)
                        permissionCell(icon: "eye", label: l10n.tr("See", "可见"),
                                       isOn: linkViewBinding(g, a.paneID), disabled: false,
                                       help: l10n.tr("Let this link see the pane", "让这条链接看到此 pane"))
                        Divider().frame(height: 16)
                        permissionCell(icon: "keyboard", label: l10n.tr("Type", "输入"),
                                       isOn: linkTypeBinding(g, a.paneID),
                                       disabled: !g.viewPanes.contains(a.paneID),
                                       help: l10n.tr("Let this link type into the pane", "让这条链接向此 pane 输入"))
                    }
                    .disabled(share.busy)
                }
            }
        }
    }

    private func linkViewBinding(_ g: GuestLink, _ pane: String) -> Binding<Bool> {
        Binding(get: { g.viewPanes.contains(pane) }, set: { on in
            var view = Set(g.viewPanes)
            var input = Set(g.inputPanes)
            if on { view.insert(pane) } else {
                view.remove(pane)
                input.remove(pane) // removing See drops Type
            }
            share.setLinkScope(g.id, view: view.sorted(), input: input.sorted())
        })
    }

    private func linkTypeBinding(_ g: GuestLink, _ pane: String) -> Binding<Bool> {
        Binding(get: { g.inputPanes.contains(pane) }, set: { on in
            var view = Set(g.viewPanes)
            var input = Set(g.inputPanes)
            if on {
                input.insert(pane)
                view.insert(pane) // Type implies See
            } else {
                input.remove(pane)
            }
            share.setLinkScope(g.id, view: view.sorted(), input: input.sorted())
        })
    }

    // "created 5m ago" from the link's enroll time (relativeTime is the shared
    // formatter used across the popover).
    private func shareLinkAge(_ enrolledAt: Int) -> String {
        guard enrolledAt > 0 else { return "" }
        let ago = relativeTime(enrolledAt, now: Int(Date().timeIntervalSince1970))
        return l10n.tr("created \(ago) ago", "\(ago)前创建")
    }

    // confirmAnywhere shows the standing-exposure warning before enabling the
    // always-on tunnel (the CLI's own prompt is skipped via --yes since we confirm
    // here). Reached only when Pro is unlocked.
    private func confirmAnywhere() {
        let a = NSAlert()
        a.messageText = l10n.tr("Keep Anywhere access on?", "保持任意网络访问开启？")
        a.informativeText = l10n.tr(
            "Your Mac stays reachable at a public address across reboots, until you turn this off. Only a device holding your token gets in, but the address is exposed the whole time.",
            "开启后，你的 Mac 会一直在一个公网地址可达，重启也不会停，直到你手动关闭。只有持你 token 的设备进得来，但这个地址会一直敞着。")
        a.addButton(withTitle: l10n.tr("Enable", "开启"))
        a.addButton(withTitle: l10n.tr("Cancel", "取消"))
        if a.runModal() == .alertFirstButtonReturn {
            remote.enableAnywhere()
        } else {
            remote.objectWillChange.send() // snap the picker back
        }
    }
}

// MARK: tab-alert (terminal-tab attention marker)
//
// The state lives in tmux's own `set-titles-string`, so the CLI is the single reader and
// writer. `gtmux config tab-alert` prints a line that BEGINS with a stable token in both
// languages — `tab-alert = on` / `tab-alert = off` — which is what this parses; the prose
// after it is localized and deliberately not depended on.
extension PreferencesView {
    var tabAlertQueryOn: Bool {
        guard let out = GtmuxCLI.capture(["config", "tab-alert"]),
              let s = String(data: out, encoding: .utf8) else { return false }
        return s.hasPrefix("tab-alert = on")
    }

    func refreshTabAlert() { tabAlertOn = tabAlertQueryOn }

    /// Writes through the CLI, then RE-READS rather than trusting the write. Enabling can
    /// legitimately fail (no tmux server) and disabling deliberately refuses when the user
    /// has edited the title format since — in both cases the switch must show what tmux
    /// has, not what was asked for.
    func setTabAlert(_ on: Bool) {
        tabAlertBusy = true
        DispatchQueue.global(qos: .userInitiated).async {
            _ = GtmuxCLI.capture(["config", "tab-alert", on ? "on" : "off"])
            let actual = tabAlertQueryOn
            DispatchQueue.main.async {
                tabAlertOn = actual
                tabAlertBusy = false
            }
        }
    }
}
