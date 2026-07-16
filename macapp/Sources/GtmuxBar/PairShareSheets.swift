import AppKit
import SwiftUI

// The two pair-share-model sheets (S3): pair a new OWN device (one code, three
// media) and create a share link with its scope in one step. Both are deliberately
// plain — neutral chrome, no marketing voice (design 铁律).

/// PairDeviceSheet — one short-lived code, three doors: phone QR / browser link /
/// terminal one-liner. Minted once when the sheet opens; every medium redeems the
/// SAME code exactly once.
struct PairDeviceSheet: View {
    @ObservedObject var l10n: L10n
    let onClose: () -> Void

    @State private var info: PairingInfo?
    @State private var code: String?
    @State private var failed = false

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(l10n.tr("Pair one of your own devices", "配对你自己的设备"))
                .font(.system(size: 14, weight: .semibold))
            Text(l10n.tr("Full control — this is you. The code is one-time and expires in 5 minutes; use exactly one of the three.",
                         "全权 —— 这是你自己。配对码一次性、5 分钟内有效；三种方式选一种用。"))
                .font(.system(size: 11)).foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)

            if let info = info, let code = code {
                HStack(alignment: .top, spacing: 16) {
                    VStack(spacing: 6) {
                        if let img = Pairing.qrImage(Pairing.payload(info, enrollCode: code), size: 168) {
                            Image(nsImage: img).interpolation(.none)
                                .resizable().frame(width: 168, height: 168)
                        }
                        Label(l10n.tr("Phone — scan in the app", "手机 —— App 里扫码"), systemImage: "iphone")
                            .font(.system(size: 11)).foregroundStyle(.secondary)
                    }
                    VStack(alignment: .leading, spacing: 14) {
                        mediumRow(icon: "globe",
                                  title: l10n.tr("Browser", "浏览器"),
                                  value: "\(info.url)/#c=\(code)")
                        mediumRow(icon: "terminal",
                                  title: l10n.tr("Another computer's terminal", "另一台电脑的终端"),
                                  value: "gtmux attach '\(info.url)/#c=\(code)'")
                        if !info.anywhere {
                            Text(l10n.tr("(LAN address — run `gtmux tunnel` first to pair from outside)",
                                         "（局域网地址 —— 想在外网配对，先跑 `gtmux tunnel`）"))
                                .font(.system(size: 10)).foregroundStyle(.tertiary)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                    }
                }
            } else if failed {
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
        .onAppear(perform: mint)
    }

    private func mint() {
        guard let p = Pairing.current() else {
            failed = true
            return
        }
        info = p
        Pairing.mintEnrollCode(token: p.token) { c in
            DispatchQueue.main.async {
                if let c = c, !c.isEmpty { code = c } else { failed = true }
            }
        }
    }

    @ViewBuilder private func mediumRow(icon: String, title: String, value: String) -> some View {
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

/// NewShareSheet — name the link AND choose its scope in one step (per-link,
/// pair-share-model): each session row carries the See/Type pair; Type implies See.
struct NewShareSheet: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var share: ShareStore
    @ObservedObject var store: AgentStore
    let onClose: () -> Void

    @State private var label = ""
    @State private var view: Set<String> = []
    @State private var input: Set<String> = []

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
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
                Button(l10n.tr("Create & copy link", "创建并复制链接")) {
                    share.newLink(label: label, view: view.sorted(), input: input.sorted())
                    onClose()
                }
                .keyboardShortcut(.defaultAction)
                .disabled(share.busy || view.isEmpty)
            }
        }
        .padding(18)
        .frame(width: 460)
    }

    // The See/Type pair for one pane row (Type ⊆ See enforced live in the sheet).
    @ViewBuilder private func scopeCell(pane: String) -> some View {
        HStack(spacing: 4) {
            Image(systemName: "eye").font(.system(size: 11)).foregroundStyle(.secondary)
            Toggle("", isOn: Binding(
                get: { view.contains(pane) },
                set: { on in
                    if on { view.insert(pane) } else {
                        view.remove(pane)
                        input.remove(pane) // removing See drops Type
                    }
                }
            )).labelsHidden().toggleStyle(.checkbox)
        }
        .frame(width: 44)
        Divider().frame(height: 16)
        HStack(spacing: 4) {
            Image(systemName: "keyboard").font(.system(size: 11)).foregroundStyle(.secondary)
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
            )).labelsHidden().toggleStyle(.checkbox)
        }
        .frame(width: 44)
    }
}
