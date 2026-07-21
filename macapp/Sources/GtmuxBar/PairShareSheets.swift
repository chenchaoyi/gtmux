import AppKit
import SwiftUI

// The two pair-share-model sheets (S3): pair a new OWN device (one code, three
// media) and create a share link with its scope in one step. Both are deliberately
// plain — neutral chrome, no marketing voice (design 铁律). Both hand off through the
// SAME "one code, three doors" delivery block (CodeDeliveryBlock) so pairing and
// sharing are isomorphic (DESIGN §13 「与配对同构的一码三媒介」).

/// CodeMediaRow — one delivery door: a labelled, selectable mono value + a copy button.
struct CodeMediaRow: View {
    @ObservedObject var l10n: L10n
    let icon: String
    let title: String
    let value: String

    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            Label(title, systemImage: icon).font(.system(size: 11)).foregroundStyle(.secondary)
            HStack(spacing: 6) {
                Text(value).font(.system(size: 11, design: .monospaced))
                    .textSelection(.enabled)
                    .lineLimit(2).truncationMode(.middle)
                Button {
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(value, forType: .string)
                } label: {
                    Image(systemName: "doc.on.doc")
                }
                .buttonStyle(.plain).help(l10n.tr("Copy", "复制"))
            }
        }
    }
}

/// CodeDeliveryBlock — the shared "one code, three doors" hand-off: a QR plus the
/// browser + terminal one-liners. Used by PairDeviceSheet (own device, `#c=`) and
/// NewShareSheet's delivery page (guest, `#g=`), so the two flows look the same. The
/// QR-encoded string is passed in (a structured pairing payload for `#c=`, the plain
/// guest URL for `#g=`) so each caller encodes the right thing.
struct CodeDeliveryBlock: View {
    @ObservedObject var l10n: L10n
    let qrText: String
    let phoneHint: String
    let browserTitle: String
    let browserValue: String
    let terminalValue: String
    var note: String? = nil

    // Memoized QR: the pairing/share code is stable while the sheet is open, but the
    // sheet re-renders every poll (it observes RemoteAccess for the status bar), and
    // re-encoding the QR each render produced a NEW NSImage → the code visibly
    // flickered/redrew ~once a second. Compute it ONCE per distinct qrText instead.
    @State private var qr: NSImage?

    var body: some View {
        HStack(alignment: .top, spacing: 16) {
            VStack(spacing: 6) {
                if let qr {
                    Image(nsImage: qr).interpolation(.none)
                        .resizable().frame(width: 168, height: 168)
                } else {
                    Color.clear.frame(width: 168, height: 168) // reserve space during the one-time encode
                }
                Label(phoneHint, systemImage: "iphone")
                    .font(.system(size: 11)).foregroundStyle(.secondary)
            }
            VStack(alignment: .leading, spacing: 14) {
                CodeMediaRow(l10n: l10n, icon: "globe", title: browserTitle, value: browserValue)
                CodeMediaRow(l10n: l10n, icon: "terminal",
                             title: l10n.tr("Terminal", "终端"), value: terminalValue)
                if let note = note {
                    Text(note).font(.system(size: 10)).foregroundStyle(.tertiary)
                        .fixedSize(horizontal: false, vertical: true)
                }
            }
        }
        .onAppear { if qr == nil { qr = Pairing.qrImage(qrText, size: 168) } }
        .onChange(of: qrText) { _, t in qr = Pairing.qrImage(t, size: 168) }
    }
}

/// PairDeviceSheet — one short-lived code, three doors: phone QR / browser link /
/// terminal one-liner. Minted once when the sheet opens; every medium redeems the
/// SAME code exactly once.
struct PairDeviceSheet: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var pairStore = PairStore.shared
    @ObservedObject var remote = RemoteAccess.shared
    let onClose: () -> Void

    // Pre-step choices (only used when the door is shut): LAN vs anywhere, and — for
    // anywhere — standard (Cloudflare) vs direct (self-hosted, redeem-unlocked).
    @State private var preLan = true
    @State private var preDirect = false
    @State private var confirmAnywhere = false

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(l10n.tr("Pair one of your own devices", "配对你自己的设备"))
                .font(.system(size: 14, weight: .semibold))
            Text(l10n.tr("Full control — this is you. The code is one-time and expires in 5 minutes; use exactly one of the three.",
                         "全权 —— 这是你自己。配对码一次性、5 分钟内有效；三种方式选一种用。"))
                .font(.system(size: 11)).foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)

            accessStatusBar

            if remote.mode == .off {
                preStep // door shut → open it first; then this view self-heals to the code
            } else {
                codeStep
            }

            HStack {
                Spacer()
                Button(l10n.tr("Done", "完成")) { onClose() }.keyboardShortcut(.defaultAction)
            }
        }
        .padding(18)
        .frame(width: 480)
        // Mint only once the door is open; on close clear so a reopen mints fresh.
        .onAppear { remote.refresh(); if remote.mode != .off { pairStore.mintPairCodeIfNeeded() } }
        .onChange(of: remote.mode) { _, m in
            if m != .off && pairStore.pairCode == nil { pairStore.mintPairCodeIfNeeded() }
        }
        .onDisappear { pairStore.clearPairCode() }
    }

    // Always-visible access status bar: the current door (mode · backend) so the sheet
    // is self-explanatory — Preferences is the management panel, this is the task flow.
    @ViewBuilder private var accessStatusBar: some View {
        HStack(spacing: 6) {
            Image(systemName: "antenna.radiowaves.left.and.right").font(.system(size: 11))
            Text(accessBarText).font(.system(size: 11, weight: .medium))
            Spacer(minLength: 0)
        }
        .foregroundStyle(remote.mode == .off ? Theme.Status.none : Theme.Status.idle)
        .padding(.horizontal, 10).padding(.vertical, 6)
        .background(RoundedRectangle(cornerRadius: 6).fill(Color.secondary.opacity(0.12)))
    }

    private var accessBarText: String {
        switch remote.mode {
        case .off: return l10n.tr("Remote access is off", "远程访问未开启")
        case .lan: return l10n.tr("Wi-Fi (LAN)", "局域网")
        case .anywhere:
            let b = remote.backend == .selfHosted
                ? l10n.tr("Direct", "直连") : l10n.tr("Standard", "标准")
            return l10n.tr("Anywhere · ", "任意网络 · ") + b
        }
    }

    // The pre-step: choose the door, open it. "开启" only opens access — the code is
    // minted by the code step once the door is up (no round-trip back here).
    @ViewBuilder private var preStep: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(l10n.tr("Turn on remote access first, then a pairing code is generated.",
                         "先开启远程访问，再生成配对码。"))
                .font(.system(size: 11)).foregroundStyle(.secondary)
            Picker("", selection: $preLan) {
                Text(l10n.tr("Wi-Fi", "局域网")).tag(true)
                Text(l10n.tr("Anywhere", "任意网络")).tag(false)
            }.pickerStyle(.segmented).labelsHidden()

            if !preLan {
                HStack(spacing: 8) {
                    backendChip(l10n.tr("Standard", "标准"), on: !preDirect) { preDirect = false }
                    backendChip(l10n.tr("Direct", "直连"), on: preDirect,
                                disabled: !remote.selfTunnelConfigured) {
                        if remote.selfTunnelConfigured { preDirect = true }
                    }
                }
                if !remote.selfTunnelConfigured {
                    Text(l10n.tr("Direct needs a redeem code — unlock it in Preferences › Remote access.",
                                 "直连需兑换码解锁 —— 在 偏好设置 › 远程访问 里解锁。"))
                        .font(.system(size: 10)).foregroundStyle(.tertiary)
                        .fixedSize(horizontal: false, vertical: true)
                }
            }

            Button(l10n.tr("Turn on", "开启")) {
                if preLan { remote.enableLan() } else { confirmAnywhere = true }
            }
            .buttonStyle(.borderedProminent)
            .disabled(remote.busy)

            if let e = remote.lastError, !e.isEmpty {
                Text(e).font(.system(size: 10)).foregroundStyle(Theme.Status.waiting)
                    .lineLimit(2).fixedSize(horizontal: false, vertical: true)
            }
        }
        // Anywhere is a standing exposure — confirm before opening it.
        .confirmationDialog(l10n.tr("Expose this Mac to the whole internet?",
                                    "把这台 Mac 暴露到整个互联网？"),
                            isPresented: $confirmAnywhere, titleVisibility: .visible) {
            Button(l10n.tr("Turn on Anywhere", "开启任意网络"), role: .destructive) {
                remote.enableAnywhere(selfHosted: preDirect)
            }
            Button(l10n.tr("Cancel", "取消"), role: .cancel) {}
        } message: {
            Text(l10n.tr("A tunnel stays up so paired devices reach this Mac from anywhere until you turn it off.",
                         "隧道会一直开着，配对设备可从任意网络访问这台 Mac，直到你关闭。"))
        }
    }

    // The code step: one code, three doors (shown once the door is open).
    @ViewBuilder private var codeStep: some View {
        if let info = pairStore.pairInfo, let code = pairStore.pairCode {
            CodeDeliveryBlock(
                l10n: l10n,
                qrText: Pairing.payload(info, enrollCode: code),
                phoneHint: l10n.tr("Phone — scan in the app", "手机 —— App 里扫码"),
                browserTitle: l10n.tr("Browser", "浏览器"),
                browserValue: "\(info.url)/#c=\(code)",
                terminalValue: "gtmux attach '\(info.url)/#c=\(code)'",
                note: info.anywhere ? nil : l10n.tr(
                    "(LAN address — switch to Anywhere to pair from outside)",
                    "（局域网地址 —— 想在外网配对，切到「任意网络」）"))
        } else if pairStore.pairFailed {
            Text(l10n.tr("Couldn't mint a pairing code — try reopening.",
                         "生成配对码失败 —— 重新打开试试。"))
                .font(.system(size: 11)).foregroundStyle(.secondary)
        } else {
            ProgressView().controlSize(.small)
        }
    }

    // A selectable backend chip (bordered capsule); greyed + inert when disabled.
    @ViewBuilder private func backendChip(_ title: String, on: Bool,
                                          disabled: Bool = false,
                                          _ action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(title).font(.system(size: 11, weight: .medium))
                .padding(.horizontal, 12).padding(.vertical, 5)
                .background(RoundedRectangle(cornerRadius: 6)
                    .fill(on ? Theme.Status.working.opacity(0.18) : Color.secondary.opacity(0.10)))
                .overlay(RoundedRectangle(cornerRadius: 6)
                    .strokeBorder(on ? Theme.Status.working.opacity(0.55) : Color.clear, lineWidth: 1))
        }
        .buttonStyle(.plain)
        .disabled(disabled)
        .opacity(disabled ? 0.4 : 1)
    }
}

/// NewShareSheet — name the link AND choose its scope in one step (per-link,
/// pair-share-model): each session row carries the See/Type pair; Type implies See.
/// On create it flips to a DELIVERY page (the guest one-code-three-media, `#g=` shown
/// once) — isomorphic with PairDeviceSheet.
struct NewShareSheet: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var share: ShareStore
    @ObservedObject var store: AgentStore
    let onClose: () -> Void

    @State private var label = ""
    @State private var view: Set<String> = []
    @State private var input: Set<String> = []
    /// The minted guest URL (`…/#g=<token>`). Non-nil ⇒ show the delivery page. The
    /// token is in this URL and is shown ONCE — reopening the sheet mints a new link.
    @State private var delivered: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            if let url = delivered {
                deliveryPage(url)
            } else {
                scopePage
            }
        }
        .padding(18)
        .frame(width: 460)
    }

    // Phase 1 — name + per-session scope.
    @ViewBuilder private var scopePage: some View {
        Text(l10n.tr("New share link", "新建分享"))
            .font(.system(size: 14, weight: .semibold))
        Text(l10n.tr("Least privilege — a collaborator sees and types ONLY what you tick here. Revoke any time.",
                     "最小授权 —— 协作者只能看/输入你在这里勾选的；随时可吊销。"))
            .font(.system(size: 11)).foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)

        TextField(l10n.tr("Who is it for? e.g. Alex", "给谁的？例如 张三"), text: $label)
            .textFieldStyle(.roundedBorder)

        if store.shareablePanes.isEmpty {
            Text(l10n.tr("No tmux panes to share right now.", "当前没有可分享的 tmux pane。"))
                .font(.system(size: 11)).foregroundStyle(.secondary)
        } else {
            // Column headers — the unified 可见/输入 wording, over the scope cells.
            HStack(spacing: 8) {
                Spacer(minLength: 0)
                Text(l10n.tr("See", "可见")).frame(width: 44)
                Text(l10n.tr("Type", "输入")).frame(width: 44)
            }
            .font(.system(size: 10, weight: .medium)).foregroundStyle(.tertiary)
            ScrollView {
                VStack(alignment: .leading, spacing: 6) {
                    ForEach(store.shareablePanes) { a in
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
                            scopeCell(pane: a.paneID)
                        }
                    }
                }
            }
            .frame(maxHeight: 220)
        }

        HStack {
            Spacer()
            Button(l10n.tr("Cancel", "取消")) { onClose() }
            Button(l10n.tr("Create link", "创建链接")) {
                share.newLink(label: label, view: view.sorted(), input: input.sorted()) { url in
                    if let url = url { delivered = url } // → delivery page (token shown once)
                }
            }
            .keyboardShortcut(.defaultAction)
            .disabled(share.busy || view.isEmpty) // at least one See to create
        }
    }

    // Phase 2 — the delivery page: guest one-code-three-media, isomorphic with pairing.
    @ViewBuilder private func deliveryPage(_ url: String) -> some View {
        Text(l10n.tr("Share link ready", "分享链接已就绪"))
            .font(.system(size: 14, weight: .semibold))
        Text(l10n.tr("Hand this to the collaborator — one link, three ways. The full link is shown ONCE; reopen New share to mint another.",
                     "把它交给协作者 —— 一条链接、三种方式。完整链接只显示这一次；要再要一条请重新「新建分享」。"))
            .font(.system(size: 11)).foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)

        CodeDeliveryBlock(
            l10n: l10n,
            qrText: url,
            phoneHint: l10n.tr("Collaborator — scan in the app", "协作者 —— App 里扫码"),
            browserTitle: l10n.tr("Browser", "浏览器"),
            browserValue: url,
            terminalValue: "gtmux attach '\(url)'")

        HStack {
            Spacer()
            Button(l10n.tr("Done", "完成")) { onClose() }.keyboardShortcut(.defaultAction)
        }
    }

    // The See/Type pair for one pane row (Type ⊆ See enforced live in the sheet).
    @ViewBuilder private func scopeCell(pane: String) -> some View {
        Toggle("", isOn: Binding(
            get: { view.contains(pane) },
            set: { on in
                if on { view.insert(pane) } else {
                    view.remove(pane)
                    input.remove(pane) // removing See drops Type
                }
            }
        )).labelsHidden().toggleStyle(.checkbox).frame(width: 44)
        Toggle("", isOn: Binding(
            get: { input.contains(pane) },
            set: { on in
                if on {
                    input.insert(pane)
                    view.insert(pane) // Type implies See
                } else {
                    input.remove(pane)
                }
            }
        )).labelsHidden().toggleStyle(.checkbox).frame(width: 44)
    }
}
