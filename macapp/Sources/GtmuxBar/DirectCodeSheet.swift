import SwiftUI

/// DirectCodeSheet — the shared "Unlock Direct" access-code sheet.
///
/// Paste a paid Direct access code, redeem it (the CLI validates it server-side and
/// writes `~/.config/gtmux/selftunnel.conf`), and on success switch Anywhere to Direct.
///
/// Extracted so the pairing window AND the Preferences popover present the SAME unlock
/// UI. Preferences used to only point at the CLI (`gtmux tunnel --redeem <code>`) while
/// the pairing flow had this sheet — one feature, two different unlock paths. Now both
/// open this control when you pick Direct on a Mac that hasn't unlocked it yet.
struct DirectCodeSheet: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var remote: RemoteAccess
    @Binding var isPresented: Bool

    @State private var code = ""
    @State private var error: String?
    @State private var redeeming = false

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(l10n.tr("Unlock Direct", "解锁 Direct")).font(.headline)
            Text(l10n.tr("Direct routes through gtmux's own server — useful when a network blocks the standard tunnel. Enter your access code.",
                         "Direct 走 gtmux 自己的服务器 —— 当网络屏蔽标准隧道时有用。请输入你的访问码。"))
                .font(.system(size: 12)).foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            TextField(l10n.tr("access code", "访问码"), text: $code)
                .textFieldStyle(.roundedBorder)
                .disableAutocorrection(true)
                .onSubmit { submit() }
            Link(l10n.tr("Don't have a code? Get one →", "还没有访问码？获取一个 →"),
                 destination: URL(string: "https://ccy.dev/projects/gtmux/direct")!)
                .font(.system(size: 11))
            if let e = error {
                Text(e).font(.system(size: 11)).foregroundStyle(.red)
                    .fixedSize(horizontal: false, vertical: true)
            }
            HStack {
                Spacer()
                Button(l10n.tr("Cancel", "取消")) { isPresented = false }
                Button(l10n.tr("Unlock", "解锁")) { submit() }
                    .keyboardShortcut(.defaultAction)
                    .disabled(redeeming || code.trimmingCharacters(in: .whitespaces).isEmpty)
            }
        }
        .padding(20).frame(width: 320)
    }

    private func submit() {
        guard !redeeming else { return }
        redeeming = true
        error = nil
        remote.redeemDirect(code) { err in
            redeeming = false
            if let err = err {
                error = err
                return
            }
            isPresented = false
            code = ""
            remote.enableAnywhere(selfHosted: true) // now unlocked → switch to Direct
        }
    }
}
