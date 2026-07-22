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
    @ObservedObject var ent = Entitlements.shared
    @ObservedObject var updater = Updater.shared
    @ObservedObject var share = ShareStore.shared
    @ObservedObject var store: AgentStore
    @ObservedObject var pairStore = PairStore.shared
    @State private var showPaywall = false
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

            Section(l10n.tr("Status bar", "状态栏")) {
                Picker(selection: $settings.displayMode) {
                    Text(l10n.tr("Dot + count", "点 + 数字")).tag(DisplayMode.dotCount)
                    Text(l10n.tr("Dot only", "仅圆点")).tag(DisplayMode.dot)
                    Text(l10n.tr("Hide when idle", "空闲时隐藏")).tag(DisplayMode.hideWhenIdle)
                } label: {
                    prefLabel("Display", "显示", symbol: "menubar.rectangle")
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

            Section(l10n.tr("Notifications", "通知")) {
                Toggle(isOn: $settings.notifications) {
                    prefLabel("Notify when an agent waits / finishes", "agent 开始等你 / 完成时提醒", symbol: "bell")
                }
            }

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
                connectedDevices
            }

            // YOUR DEVICES (PAIR) — your own phone / browser / terminal, full control.
            // They reach the Mac through the door above.
            Section(l10n.tr("Your devices · Pair", "我的设备 · 配对")) {
                pairSection
            }

            // SHARE — collaborators, least privilege, per-link scope.
            Section(l10n.tr("Sharing", "分享")) {
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

            Section(l10n.tr("Software update", "软件更新")) {
                updateRow
            }
        }
        .formStyle(.grouped)
        .frame(width: 460, height: 640)
        .onAppear { remote.refresh(); share.refresh(); share.loadDetail(); pairStore.refresh(); updater.autoCheck() }
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
        case .failed: return l10n.tr("Check failed — try again", "检查失败，请重试")
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
    // Standard (zero-config hosted Cloudflare) and Direct (your own VPS + domain). The
    // picker was hiding this choice. When Direct is configured on this Mac, offer a
    // Standard | Direct switch; otherwise show which backend is active (read-only) plus
    // how to set Direct up — so it's never a mystery which tunnel you're on.
    @ViewBuilder private var tunnelBackendRow: some View {
        if remote.mode == .anywhere {
            if remote.selfTunnelConfigured {
                LabeledContent {
                    Picker("", selection: backendBinding) {
                        Text(l10n.tr("Standard", "标准")).tag(TunnelBackend.cloudflare)
                        Text(l10n.tr("Direct", "直连")).tag(TunnelBackend.selfHosted)
                    }
                    .pickerStyle(.segmented).labelsHidden().disabled(remote.busy)
                } label: {
                    prefLabel("Tunnel", "隧道", symbol: "network")
                }
                Text(backendSubtitle)
                    .font(.system(size: 11)).foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
            } else {
                Text(l10n.tr("Tunnel: Standard (zero-config, hosted). Direct — your own VPS + domain — sets up with `gtmux tunnel --backend self`.",
                             "隧道：Standard（零配置托管）。Direct —— 你自己的 VPS + 域名 —— 用 `gtmux tunnel --backend self` 配置。"))
                    .font(.system(size: 11)).foregroundStyle(.tertiary)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
    }

    // Switching backend re-runs the tunnel service on the chosen backend (both are the
    // already-consented "Anywhere" exposure, so no extra confirm — the user picked it).
    private var backendBinding: Binding<TunnelBackend> {
        Binding(
            get: { remote.backend == .selfHosted ? .selfHosted : .cloudflare },
            set: { b in remote.enableAnywhere(selfHosted: b == .selfHosted) })
    }

    private var backendSubtitle: String {
        switch remote.backend {
        case .selfHosted:
            return l10n.tr("Direct — reached over your own VPS + domain.", "直连 —— 经你自己的 VPS + 域名可达。")
        case .cloudflare:
            return l10n.tr("Standard — a zero-config hosted tunnel.", "标准 —— 零配置托管隧道。")
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
            return l10n.tr("Off — anyone with a share link is view-only.",
                           "已关闭 —— 持分享链接的访客只读。")
        }
        if share.allowedPanes.isEmpty {
            return l10n.tr("On, but no panes are allowed yet — tick a pane below.",
                           "已开启，但还没允许任何 pane —— 在下方勾选。")
        }
        return l10n.tr("On — a guest with a share link can type into the ticked panes.",
                       "已开启 —— 持分享链接的访客可向勾选的 pane 输入。")
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
            Image(systemName: icon).font(.system(size: 11)).foregroundStyle(.secondary)
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
                    .font(.system(size: 9, weight: .semibold)).foregroundStyle(.secondary)
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
                                    .font(.system(size: 9, weight: .semibold)).foregroundStyle(.secondary)
                                    .frame(width: 10)
                            }.buttonStyle(.plain)
                            Image(systemName: "link").font(.system(size: 11))
                                .foregroundStyle(.secondary).frame(width: 14)
                            VStack(alignment: .leading, spacing: 1) {
                                Text(g.label.isEmpty ? l10n.tr("Share link", "分享链接") : g.label)
                                    .font(.system(size: 12))
                                Text(shareLinkAge(g.enrolledAt) + "  ·  " + linkScopeSummary(g))
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
                                .help(l10n.tr("Show link — QR, browser, terminal", "展示链接 —— 二维码 / 浏览器 / 终端"))
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
                    Text(l10n.tr("New link — copied to clipboard:", "新链接 —— 已复制到剪贴板："))
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
            Text(l10n.tr("No paired devices yet — pair your phone, a browser, or another computer's terminal.",
                         "还没有配对设备 —— 配对你的手机、浏览器或另一台电脑的终端。"))
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

    private func pairLastSeen(_ d: PairedDevice) -> String {
        if d.lastSeen > 0 {
            return l10n.tr("last seen ", "上次连接 ") + relativeTime(d.lastSeen, now: Int(Date().timeIntervalSince1970)) + l10n.tr(" ago", "前")
        }
        return l10n.tr("never connected", "从未连接")
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
            "Your Mac stays reachable at a public URL (token-gated) across reboots until you turn this off. It's a standing exposure — enable it consciously.",
            "开启后，你的 Mac 会一直在一个公网地址可达（有 token 把关），重启也不会停，直到你手动关闭。这是个长期敞口，请想清楚再开。")
        a.addButton(withTitle: l10n.tr("Enable", "开启"))
        a.addButton(withTitle: l10n.tr("Cancel", "取消"))
        if a.runModal() == .alertFirstButtonReturn {
            remote.enableAnywhere()
        } else {
            remote.objectWillChange.send() // snap the picker back
        }
    }
}
