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

    var body: some View {
        HStack(alignment: .top, spacing: 16) {
            VStack(spacing: 6) {
                if let img = Pairing.qrImage(qrText, size: 168) {
                    Image(nsImage: img).interpolation(.none)
                        .resizable().frame(width: 168, height: 168)
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
    }
}

/// PairDeviceSheet — one short-lived code, three doors: phone QR / browser link /
/// terminal one-liner. Minted once when the sheet opens; every medium redeems the
/// SAME code exactly once.
struct PairDeviceSheet: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var pairStore = PairStore.shared
    let onClose: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(l10n.tr("Pair one of your own devices", "配对你自己的设备"))
                .font(.system(size: 14, weight: .semibold))
            Text(l10n.tr("Full control — this is you. The code is one-time and expires in 5 minutes; use exactly one of the three.",
                         "全权 —— 这是你自己。配对码一次性、5 分钟内有效；三种方式选一种用。"))
                .font(.system(size: 11)).foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)

            if let info = pairStore.pairInfo, let code = pairStore.pairCode {
                CodeDeliveryBlock(
                    l10n: l10n,
                    qrText: Pairing.payload(info, enrollCode: code),
                    phoneHint: l10n.tr("Phone — scan in the app", "手机 —— App 里扫码"),
                    browserTitle: l10n.tr("Browser", "浏览器"),
                    browserValue: "\(info.url)/#c=\(code)",
                    terminalValue: "gtmux attach '\(info.url)/#c=\(code)'",
                    note: info.anywhere ? nil : l10n.tr(
                        "(LAN address — run `gtmux tunnel` first to pair from outside)",
                        "（局域网地址 —— 想在外网配对，先跑 `gtmux tunnel`）"))
            } else if pairStore.pairFailed {
                Text(l10n.tr("Couldn't mint a pairing code — is remote access on? (gtmux serve / gtmux tunnel)",
                             "生成配对码失败 —— 远程访问开了吗？（gtmux serve / gtmux tunnel）"))
                    .font(.system(size: 11)).foregroundStyle(.secondary)
            } else {
                ProgressView().controlSize(.small)
            }

            HStack {
                Spacer()
                Button(l10n.tr("Done", "完成")) { onClose() }.keyboardShortcut(.defaultAction)
            }
        }
        .padding(18)
        .frame(width: 480)
        // Mint once (idempotent in the store); clear on close so a reopen mints fresh.
        .onAppear { pairStore.mintPairCodeIfNeeded() }
        .onDisappear { pairStore.clearPairCode() }
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
