import SwiftUI

/// The same reachability choices in Preferences and the pairing window. Keep the
/// controls and their labels together so these two entry points cannot drift.
struct RemoteAccessControls: View {
    @ObservedObject var l10n: L10n
    let mode: RemoteMode
    let busy: Bool
    let modeSelection: Binding<RemoteMode>
    let backendSelection: Binding<TunnelBackend>
    let backendRevert: Int
    let controlWidth: CGFloat

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            choiceLabel("Access", "访问", symbol: "antenna.radiowaves.left.and.right")
            Picker(l10n.tr("Access", "访问"), selection: modeSelection) {
                Text(l10n.tr("Off", "关闭")).tag(RemoteMode.off)
                Text(l10n.tr("Local network", "局域网")).tag(RemoteMode.lan)
                Text(l10n.tr("Anywhere", "任意网络")).tag(RemoteMode.anywhere)
            }
            .pickerStyle(.segmented)
            .labelsHidden()
            .frame(width: controlWidth)
            .disabled(busy)
            .help(l10n.tr("Off: this Mac cannot be reached remotely. Local network: same network only. Anywhere: also works on cellular.",
                          "关闭：外部设备无法连接本机。局域网：只在同一网络可连。任意网络：蜂窝网络也能连。"))

            if mode == .anywhere {
                Divider()
                choiceLabel("Connection method", "连接方式", symbol: "network")
                Picker(l10n.tr("Connection method", "连接方式"), selection: backendSelection) {
                    Text(l10n.tr("Standard", "标准")).tag(TunnelBackend.cloudflare)
                    Text(l10n.tr("Direct", "直连")).tag(TunnelBackend.selfHosted)
                }
                .id(backendRevert)
                .pickerStyle(.segmented)
                .labelsHidden()
                .frame(width: controlWidth)
                .disabled(busy)
                .help(l10n.tr("Standard needs no setup. Direct works on networks that block Standard and requires an access code.",
                              "标准无需设置。直连可用于屏蔽标准线路的网络，需要访问码。"))
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func choiceLabel(_ en: String, _ zh: String, symbol: String) -> some View {
        Label(l10n.tr(en, zh), systemImage: symbol)
            .font(.system(size: 11, weight: .semibold))
            .foregroundStyle(.secondary)
    }
}
