import AppKit
import SwiftUI

/// One layout for the same reachability choices in Preferences and Pair your phone.
/// A Picker with only an outer frame kept its intrinsic segment width and floated
/// in the middle of the settings card; these choices fill their rows instead.
struct RemoteAccessControls: View {
    @ObservedObject var l10n: L10n
    let mode: RemoteMode
    let busy: Bool
    let modeSelection: Binding<RemoteMode>
    let backendSelection: Binding<TunnelBackend>
    let backendRevert: Int
    let controlWidth: CGFloat
    let currentAddress: String?

    @State private var showAccessHelp = false
    @State private var showMethodHelp = false

    var body: some View {
        VStack(alignment: .leading, spacing: 9) {
            header("Access", "访问范围", symbol: "antenna.radiowaves.left.and.right",
                   explanation: accessHelp, showing: $showAccessHelp,
                   address: currentAddress)
            segments([
                (.off, "Off", "关闭"),
                (.lan, "Local network", "局域网"),
                (.anywhere, "Anywhere", "任意网络"),
            ], selection: modeSelection)

            if mode == .anywhere {
                Divider()
                header("Connection method", "连接方式", symbol: "network",
                       explanation: methodHelp, showing: $showMethodHelp)
                segments([
                    (.cloudflare, "Standard", "标准"),
                    (.selfHosted, "Direct", "直连"),
                ], selection: backendSelection)
                .id(backendRevert)
            }
        }
        .frame(width: controlWidth, alignment: .leading)
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var accessHelp: String {
        l10n.tr("Off blocks remote connections. Local network works on the same network; Anywhere also works over cellular.",
                "关闭后无法远程连接；局域网仅限同一网络，任意网络也可通过蜂窝网络连接。")
    }

    private var methodHelp: String {
        l10n.tr("Standard works without setup. Direct needs an access code and can work where Standard is blocked.",
                "标准无需设置；直连需要访问码，适合标准连接受阻的网络。")
    }

    private func header(_ en: String, _ zh: String, symbol: String,
                        explanation: String, showing: Binding<Bool>,
                        address: String? = nil) -> some View {
        HStack(spacing: 6) {
            Label(l10n.tr(en, zh), systemImage: symbol)
                .font(.system(size: 11, weight: .semibold))
                .foregroundStyle(.secondary)
            Button { showing.wrappedValue.toggle() } label: {
                Image(systemName: "questionmark.circle")
                    .font(.system(size: 12))
                    .foregroundStyle(.secondary)
            }
            .buttonStyle(.plain)
            .accessibilityLabel(l10n.tr("About \(en)", "关于\(zh)"))
            .help(explanation)
            .popover(isPresented: showing, arrowEdge: .bottom) {
                VStack(alignment: .leading, spacing: 10) {
                    Text(explanation)
                    if let address, !address.isEmpty {
                        Divider()
                        Text(l10n.tr("Current address", "当前地址"))
                            .fontWeight(.semibold)
                        Text(address)
                            .font(.system(.caption, design: .monospaced))
                            .textSelection(.enabled)
                        Button(l10n.tr("Copy address", "复制地址")) {
                            NSPasteboard.general.clearContents()
                            NSPasteboard.general.setString(address, forType: .string)
                        }
                    }
                }
                .font(.system(size: 11))
                .fixedSize(horizontal: false, vertical: true)
                .frame(width: 260, alignment: .leading)
                .padding(12)
            }
            Spacer(minLength: 0)
        }
    }

    private func segments<Value: Hashable>(_ options: [(Value, String, String)],
                                           selection: Binding<Value>) -> some View {
        HStack(spacing: 2) {
            ForEach(options.indices, id: \.self) { index in
                let option = options[index]
                let selected = selection.wrappedValue == option.0
                Button { selection.wrappedValue = option.0 } label: {
                    Text(l10n.tr(option.1, option.2))
                        .font(.system(size: 12, weight: selected ? .semibold : .regular))
                        .lineLimit(1)
                        .minimumScaleFactor(0.85)
                        .frame(maxWidth: .infinity)
                        .frame(height: 27)
                        .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .foregroundStyle(selected ? Color.white : Color.primary)
                .background(selected ? Color.accentColor : Color.clear,
                            in: RoundedRectangle(cornerRadius: 6))
                .accessibilityAddTraits(selected ? .isSelected : [])
            }
        }
        .padding(3)
        .background(Color.primary.opacity(0.06), in: RoundedRectangle(cornerRadius: 8))
        .frame(width: controlWidth)
        .disabled(busy)
    }
}

/// Preferences and pairing ask for the same lasting change. Keep the decision
/// and its wording together so the two entry points cannot give different advice.
func confirmAnywhereAccess(l10n: L10n) -> Bool {
    let alert = NSAlert()
    alert.messageText = l10n.tr("Turn on Anywhere access?", "开启任意网络访问？")
    alert.informativeText = l10n.tr(
        "Paired devices and people you share a link with can connect to this Mac from any network. Access stays on after a restart until you turn it off.",
        "开启后，已配对的设备和收到分享链接的人可以从任何网络连接这台 Mac。Mac 重启后仍会保持开启，直到你关闭。")
    alert.addButton(withTitle: l10n.tr("Enable", "开启"))
    alert.addButton(withTitle: l10n.tr("Cancel", "取消"))
    return alert.runModal() == .alertFirstButtonReturn
}
