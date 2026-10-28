import Combine
import SwiftUI

struct PairingNotice: View {
    @ObservedObject var l10n: L10n
    var body: some View {
        Text(l10n.tr("A paired device has full access to this Mac. Pair only your own devices. Each code works once and renews automatically while this window is open.",
                     "配对设备可完整访问这台 Mac，请只配对自己的设备。配对码仅可使用一次，窗口打开时会自动更新。"))
            .font(.system(size: 12)).foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
    }
}

/// Both pairing entry points display and copy these same one-time credentials.
struct PairingDeliveryValues {
    let qr: String
    let browser: String
    let terminal: String

    init(info: PairingInfo, code: String) {
        let base = info.url.hasSuffix("/") ? String(info.url.dropLast()) : info.url
        browser = "\(base)/#c=\(code)"
        terminal = "gtmux attach '\(browser)'"
        qr = Pairing.payload(info, enrollCode: code)
    }
}

struct PairingCodeContent<Connection: View>: View {
    @ObservedObject var l10n: L10n
    let info: PairingInfo
    let onRefresh: () -> Void
    @ViewBuilder var connection: Connection
    @ObservedObject private var pairStore = PairStore.shared
    @State private var now = Date()
    @State private var timer = Timer.publish(every: 1, on: .main, in: .common).autoconnect()

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            connection
            if let code = pairStore.pairCode {
                let values = PairingDeliveryValues(info: info, code: code)
                CodeDeliveryBlock(l10n: l10n, qrText: values.qr,
                                  linkValue: values.browser, terminalValue: values.terminal,
                                  appHint: l10n.tr("Pair → Scan", "配对 → 扫一扫"))
            } else if pairStore.pairFailed {
                Text(l10n.tr("Could not create a pairing code yet. Retrying…", "暂时无法生成配对码，正在重试…"))
                    .font(.system(size: 12)).foregroundStyle(.secondary)
            } else {
                ProgressView().controlSize(.small)
            }
            HStack(spacing: 12) {
                if let minted = pairStore.pairMinted {
                    Text(countdown(minted.expiresAt))
                        .font(.system(size: 11).monospacedDigit()).foregroundStyle(.secondary)
                }
                Spacer(minLength: 0)
                Button(action: onRefresh) {
                    Label(l10n.tr("Refresh code", "刷新配对码"), systemImage: "arrow.clockwise")
                }.buttonStyle(.borderless).controlSize(.small)
            }
        }
        .onReceive(timer) { now = $0 }
    }

    private func countdown(_ expiration: Date) -> String {
        guard let left = codeLeft(expiration, now: now) else {
            return l10n.tr("Renewing the code…", "正在换新的配对码…")
        }
        return l10n.tr("This code renews itself in \(left)", "配对码 \(left) 后自动换新")
    }
}
