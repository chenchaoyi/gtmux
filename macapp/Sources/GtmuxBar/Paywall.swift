import SwiftUI

/// PaywallView — the "Unlock Anywhere access (Pro)" sheet, shown when a user
/// reaches for the always-on tunnel without Pro. The tunnel is a hosted service
/// (real cost), so anywhere-reachable is the paid tier; LAN (same Wi-Fi) is free.
///
/// STEP 1: the unlock is free (public beta) — the real purchase flow lands in
/// step 2. Copy stays plain per DESIGN (no marketing tone).
struct PaywallView: View {
    @ObservedObject var l10n: L10n
    var onUnlock: () -> Void
    var onClose: () -> Void

    var body: some View {
        VStack(spacing: 15) {
            Image(systemName: "globe")
                .font(.system(size: 38, weight: .regular))
                .foregroundStyle(Color.accentColor)

            Text(l10n.tr("Reach your Mac from anywhere", "随时随地连到你的 Mac"))
                .font(.system(size: 16, weight: .semibold))
                .multilineTextAlignment(.center)

            Text(l10n.tr(
                "Pair and drive your agents over any network — not just the same Wi-Fi — at a stable address that survives reboots.",
                "在任意网络下配对并操控你的 agent —— 不限同一 Wi-Fi —— 用一个重启也不变的固定地址。"))
                .font(.system(size: 12)).foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)

            VStack(alignment: .leading, spacing: 9) {
                feature("globe", l10n.tr("Reachable from any network", "任意网络可达"))
                feature("link", l10n.tr("Stable address, unchanged across reboots", "固定地址，重启也不变"))
                feature("lock.shield", l10n.tr("Token-gated, end to end", "Token 把关，端到端"))
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.vertical, 2)

            // STEP 1: free during beta; pricing lands with the purchase flow.
            Text(l10n.tr("Free during the beta", "公测期间免费"))
                .font(.system(size: 11)).foregroundStyle(.tertiary)

            VStack(spacing: 8) {
                Button(action: onUnlock) {
                    Text(l10n.tr("Unlock Anywhere access", "解锁任意网络访问"))
                        .frame(maxWidth: .infinity)
                }
                .controlSize(.large)
                .keyboardShortcut(.defaultAction)

                Button(l10n.tr("Maybe later", "以后再说"), action: onClose)
                    .buttonStyle(.plain)
                    .font(.system(size: 12)).foregroundStyle(.secondary)
            }
        }
        .padding(26)
        .frame(width: 360)
    }

    private func feature(_ symbol: String, _ text: String) -> some View {
        HStack(spacing: 9) {
            Image(systemName: symbol)
                .font(.system(size: 12)).foregroundStyle(Color.accentColor)
                .frame(width: 18)
            Text(text).font(.system(size: 12.5))
            Spacer(minLength: 0)
        }
    }
}
