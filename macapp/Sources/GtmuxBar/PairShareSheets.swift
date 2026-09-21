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

/// CodeDeliveryBlock — one link, and the three ways to move it.
///
/// It used to be a 168pt QR with everything else crammed into the 240pt column beside it:
/// the browser URL, the terminal one-liner, and the link said as two lines to read out.
/// At that width the last of them could not fit, and what got cut was the code itself
/// (「这个UI都展示不全」, 2026-09-21). The rows also read as a list rather than as
/// alternatives 「看不出来是三种并列的不同的方式」.
///
/// So the LINK is the headline, full width and never truncated, and the three media sit
/// under it as three equal cards: scan it, copy it, run it. They are the same link.
///
/// Nothing here tells the owner how to deliver it. A row that said "read out these two
/// lines" was instructing them in their own hand-off 「用户自己选择如何传递信息即可，read
/// it out这种指令很蠢」; the link carries its own short code, and what to do with it is
/// not gtmux's call.
struct CodeDeliveryBlock: View {
    @ObservedObject var l10n: L10n
    /// What the QR encodes — a structured pairing payload for `#c=`, the plain URL for a
    /// share link — so each caller encodes the right thing.
    let qrText: String
    /// The link itself, shown whole.
    let linkValue: String
    let terminalValue: String
    var note: String? = nil

    // Memoized QR: the code is stable while the sheet is open, but the sheet re-renders
    // every poll (it observes RemoteAccess for the status bar), and re-encoding each
    // render produced a NEW NSImage → the code visibly flickered ~once a second.
    @State private var qr: NSImage?

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            LinkLine(l10n: l10n, value: linkValue)
            HStack(alignment: .top, spacing: 12) {
                // The three captions are the three MEDIA, so they read as peers. The
                // first used to be an instruction ("scan in the app") beside two nouns,
                // which is the shape that made them look like a list rather than a choice.
                DeliveryDoor(title: l10n.tr("Phone", "手机")) {
                    if let qr {
                        Image(nsImage: qr).interpolation(.none)
                            .resizable().frame(width: 132, height: 132)
                    } else {
                        Color.clear.frame(width: 132, height: 132) // during the one-time encode
                    }
                }
                CopyDoor(l10n: l10n, icon: "globe", title: l10n.tr("Browser", "浏览器"),
                         action: l10n.tr("Copy link", "复制链接"), value: linkValue)
                CopyDoor(l10n: l10n, icon: "terminal", title: l10n.tr("Terminal", "终端"),
                         action: l10n.tr("Copy command", "复制命令"), value: terminalValue)
            }
            if let note = note {
                Text(note).font(.system(size: 10)).foregroundStyle(.tertiary)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .onAppear { if qr == nil { qr = Pairing.qrImage(qrText, size: 132) } }
        .onChange(of: qrText) { _, t in qr = Pairing.qrImage(t, size: 132) }
    }
}

/// LinkLine — the link, whole, selectable, with its own copy button. Full width, because
/// the thing being handed over is the one thing that must never be shown in part.
private struct LinkLine: View {
    @ObservedObject var l10n: L10n
    let value: String
    @State private var copied = false

    var body: some View {
        HStack(spacing: 8) {
            Text(value)
                .font(.system(size: 12, design: .monospaced))
                .textSelection(.enabled)
                .lineLimit(2)
                .frame(maxWidth: .infinity, alignment: .leading)
            Button {
                copy(value, $copied)
            } label: {
                Image(systemName: copied ? "checkmark" : "doc.on.doc")
            }
            .buttonStyle(.plain).help(l10n.tr("Copy", "复制"))
        }
        .padding(.horizontal, 10).padding(.vertical, 8)
        .background(RoundedRectangle(cornerRadius: 7).fill(Color.secondary.opacity(0.10)))
    }
}

/// DeliveryDoor — one of the three equal cards. Its content is whatever that medium is.
private struct DeliveryDoor<Content: View>: View {
    let title: String
    @ViewBuilder let content: Content

    var body: some View {
        VStack(spacing: 8) {
            content.frame(maxWidth: .infinity, maxHeight: .infinity)
            Text(title).font(.system(size: 11)).foregroundStyle(.secondary)
                .lineLimit(1).minimumScaleFactor(0.8)
        }
        .padding(10)
        .frame(maxWidth: .infinity, minHeight: 176)
        .background(RoundedRectangle(cornerRadius: 9).fill(Color.secondary.opacity(0.08)))
        .overlay(RoundedRectangle(cornerRadius: 9).strokeBorder(Color.secondary.opacity(0.18), lineWidth: 1))
    }
}

/// CopyDoor — a door whose whole card is the button: the medium's mark, then the one
/// thing it does. The label says what happened for a moment, since a clipboard write is
/// otherwise silent.
private struct CopyDoor: View {
    @ObservedObject var l10n: L10n
    let icon: String
    let title: String
    let action: String
    let value: String
    @State private var copied = false

    var body: some View {
        Button {
            copy(value, $copied)
        } label: {
            DeliveryDoor(title: title) {
                VStack(spacing: 10) {
                    Image(systemName: icon).font(.system(size: 30, weight: .light))
                        .foregroundStyle(.secondary)
                    Text(copied ? l10n.tr("Copied", "已复制") : action)
                        .font(.system(size: 12, weight: .medium))
                        .lineLimit(1).minimumScaleFactor(0.8)
                }
            }
        }
        .buttonStyle(.plain)
    }
}

/// copy writes to the pasteboard and flips a "done" flag back after a moment, so the
/// card can say something happened: a clipboard write is otherwise entirely silent.
private func copy(_ value: String, _ flag: Binding<Bool>) {
    NSPasteboard.general.clearContents()
    NSPasteboard.general.setString(value, forType: .string)
    flag.wrappedValue = true
    DispatchQueue.main.asyncAfter(deadline: .now() + 1.6) { flag.wrappedValue = false }
}

/// PairDeviceSheet — one short-lived code, three doors: phone QR / browser link /
/// terminal one-liner. All three show the SAME code, which PairStore replaces while the
/// sheet is open whenever it can no longer be redeemed (expired, used, or dropped by a
/// serve restart).
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
    // Whether this sheet holds PairStore's code. start/stop are reference-counted and
    // the "Pair your phone" window shares them, so every start needs exactly one stop.
    @State private var holdingCode = false

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(l10n.tr("Pair one of your own devices", "配对你自己的设备"))
                .font(.system(size: 14, weight: .semibold))
            // Say what pairing DOES before naming the mechanics: the old copy opened on
            // "Full control — this is you", a fragment that reads as a riddle, and then
            // referred to "the three" before the reader had seen any of them.
            Text(l10n.tr("A paired device gets full control of this Mac, so pair only your own. Each code works once, and while this window is open it is replaced before it runs out. Pick whichever of the three ways fits the device.",
                         "配对后的设备对这台 Mac 有完全控制权，只配对你自己的设备。每个配对码只能用一次，窗口开着时快过期前会自动换新的。三种方式挑一种适合的用。"))
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
        .frame(width: 560)
        // Hold a code only once the door is open; let go on close so a reopen starts fresh.
        .onAppear { remote.refresh(); if remote.mode != .off { holdCode() } }
        .onChange(of: remote.mode) { _, m in if m != .off { holdCode() } }
        .onDisappear {
            if holdingCode { pairStore.stopPairCode(); holdingCode = false }
        }
    }

    private func holdCode() {
        guard !holdingCode else { return }
        holdingCode = true
        pairStore.startPairCode()
    }

    // Always-visible access status bar: the current door (mode · backend) so the sheet
    // is self-explanatory — Preferences is the management panel, this is the task flow.
    @ViewBuilder private var accessStatusBar: some View {
        HStack(spacing: 6) {
            Image(systemName: "antenna.radiowaves.left.and.right").font(.system(size: 12))
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
        case .lan: return l10n.tr("Local network", "局域网")
        case .anywhere:
            let b = remote.backend == .selfHosted
                ? l10n.tr("Direct", "直连") : l10n.tr("Standard", "标准")
            // What the tunnel reports about itself, end to end; nothing when it has not
            // reported recently, since a stale status is not a fact.
            var state = ""
            if let st = TunnelStatus.read(), st.fresh {
                switch st.state {
                case "connected": state = l10n.tr(" · connected", " · 已连上")
                case "down": state = l10n.tr(" · down", " · 断开")
                default: state = l10n.tr(" · connecting", " · 连接中")
                }
            }
            return l10n.tr("Anywhere · ", "任意网络 · ") + b + state
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
                Text(l10n.tr("Local network", "局域网")).tag(true)
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
                    Text(l10n.tr("Direct needs an access code; unlock it in Preferences › Remote access.",
                                 "直连需要访问码，在 偏好设置 › 远程访问 里解锁。"))
                        .font(.system(size: 10)).foregroundStyle(.tertiary)
                        .fixedSize(horizontal: false, vertical: true)
                    Link(l10n.tr("Get an access code →", "获取访问码 →"),
                         destination: URL(string: "https://ccy.dev/projects/gtmux/direct")!)
                        .font(.system(size: 10))
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
                linkValue: "\(info.url)/#c=\(code)",
                terminalValue: "gtmux attach '\(info.url)/#c=\(code)'",
                note: info.anywhere ? nil : l10n.tr(
                    "(a local-network address; switch to Anywhere to pair from outside)",
                    "（局域网地址，想在外网配对请切到「任意网络」）"))
        } else if pairStore.pairFailed {
            Text(l10n.tr("gtmux on this Mac hasn't handed out a code yet. It tries again every few seconds.",
                         "这台 Mac 上的 gtmux 还没发出配对码，每隔几秒会自动再试。"))
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
    @State private var delivered: SharedLink?

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            if let link = delivered {
                deliveryPage(link)
            } else {
                scopePage
            }
        }
        .padding(18)
        .frame(width: 560)
    }

    // Phase 1 — name + per-session scope.
    @ViewBuilder private var scopePage: some View {
        Text(l10n.tr("New share link", "新建分享"))
            .font(.system(size: 14, weight: .semibold))
        Text(l10n.tr("A collaborator sees and types only what you tick here, and you can revoke it any time.",
                     "协作者只能看、只能输入你在这里勾选的，随时可以吊销。"))
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
                share.newLink(label: label, view: view.sorted(), input: input.sorted()) { link in
                    if let link = link { delivered = link } // → delivery page
                }
            }
            .keyboardShortcut(.defaultAction)
            .disabled(share.busy || view.isEmpty) // at least one See to create
        }
    }

    // Phase 2 — the delivery page: guest one-code-three-media, isomorphic with pairing.
    @ViewBuilder private func deliveryPage(_ link: SharedLink) -> some View {
        Text(l10n.tr("Share link ready", "分享链接已就绪"))
            .font(.system(size: 14, weight: .semibold))
        Text(l10n.tr("Hand it to the collaborator, whichever way suits them. Everything here opens the same access, and you can reopen this panel from the link's row.",
                     "把它交给协作者，哪种方便用哪种。这里每一样打开的都是同一份访问权，之后也能从这条链接那一行再打开这个面板。"))
            .font(.system(size: 11)).foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)

        CodeDeliveryBlock(
            l10n: l10n,
            qrText: link.url,
            linkValue: link.url,
            terminalValue: "gtmux attach '\(link.url)'")

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

/// ShareLinkDeliverySheet — re-open the delivery panel for an EXISTING guest link. The
/// SAME panel NewShareSheet shows on mint, reachable later from the link's row so the
/// host can re-hand it without minting a new one. The link is re-fetched by the CLI on
/// demand, so this stays token-safe (the app never reads the roster). Each door carries
/// its own copy button, so this supersedes the old bare "copy the URL" action.
struct ShareLinkDeliverySheet: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var share = ShareStore.shared
    let id: String
    let label: String
    let link: SharedLink
    let onClose: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(label.isEmpty
                    ? l10n.tr("Share link", "分享链接")
                    : l10n.tr("Share link · \(label)", "分享链接 · \(label)"))
                .font(.system(size: 14, weight: .semibold))
            Text(l10n.tr("One link, however it reaches them. Everything here opens the same access.",
                         "同一条链接，怎么到对方手上都行。这里每一样打开的都是同一份访问权。"))
                .font(.system(size: 11)).foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)

            CodeDeliveryBlock(
                l10n: l10n,
                qrText: link.url,
                linkValue: link.url,
                terminalValue: "gtmux attach '\(link.url)'")

            HStack {
                Spacer()
                Button(l10n.tr("Done", "完成")) { onClose() }.keyboardShortcut(.defaultAction)
            }
        }
        .padding(18)
        .frame(width: 560)
    }
}
