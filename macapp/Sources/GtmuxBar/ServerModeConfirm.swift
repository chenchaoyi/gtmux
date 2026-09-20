import SwiftUI

/// The explainer shown before macOS asks for the administrator password.
///
/// This used to be an NSAlert widened with an accessory view, which backfired: a wide
/// NSAlert switches to a CENTRED layout, so the title centred while the body stayed
/// left-aligned — a mixed-alignment dialog that looks improvised. A sheet of our own
/// keeps one alignment, one type scale, and the same visual language as Preferences,
/// which is where it is presented from.
///
/// Content rules (DESIGN §5): plain, factual, no marketing. It states the four things
/// a user cannot consent without knowing — that it never expires, what happens on
/// battery, that a closed lid runs hotter, and that the machine stays reachable while
/// unattended — and nothing else. Each is ONE sentence: the four used to be a bold
/// label over a grey gloss, and a dialog where every item is bold has no emphasis left
/// to spend.
struct ServerModeConfirmView: View {
    @ObservedObject var l10n: L10n
    /// Non-nil when this machine is a configuration the project has not verified.
    var unverifiedOS: String?
    var onConfirm: () -> Void
    var onCancel: () -> Void
    @Environment(\.colorScheme) private var scheme

    private var points: [String] {
        [
            l10n.tr("It stays on until you turn it off in this window. While it is on, a red dot pulses on the menu bar icon:",
                    "开了就一直开着，直到你在这个窗口里关掉。开着的时候，菜单栏图标上有一个红点在呼吸："),
            l10n.tr("It works on battery, so you can carry the Mac between rooms with the lid shut. Below 20% it starts sleeping again on its own.",
                    "用电池也能跑，可以合着盖子在屋里拎着走。电量掉到 20% 以下，它自己恢复睡眠。"),
            l10n.tr("A closed lid runs hotter. Keep the Mac on a hard surface and out of a bag; a fanless Air feels it most.",
                    "合盖更热。放在硬桌面上，别塞进包里；无风扇的 Air 最明显。"),
            l10n.tr("The Mac stays reachable while nobody is sitting at it. Your screen still locks the way you set it.",
                    "你不在跟前的时候，这台 Mac 也一直连得上。屏幕锁还是按你设的锁。"),
        ]
    }

    /// The first point ends with the menu-bar mark as it will actually look once this
    /// is on, drawn by the same code that draws the real one. Saying "a red dot
    /// appears" and then showing it is the difference between a claim and an
    /// instruction. Inline in the Text so it sits on the sentence's own baseline.
    private var firstPoint: Text {
        Text(points[0]) + Text("  ")
            + Text(Image(nsImage: StatusItemGlyph.image(mostUrgent: .running, empty: true,
                                                        dark: scheme == .dark,
                                                        awake: true, phase: 0.5)))
    }

    private var cautionInk: Color {
        scheme == .dark ? Theme.Status.errored : Theme.Quota.amberTextLight
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 10) {
                // The laptop, not a red power symbol: red is "waiting for you" in the
                // status language, and a 20pt red mark at the top of a dialog reads as
                // an alarm. It also matches the row in Preferences this came from.
                Image(systemName: "laptopcomputer")
                    .font(.system(size: 20, weight: .regular))
                    .foregroundStyle(.secondary)
                Text(l10n.tr("Turn on server mode?", "开启服务器模式？"))
                    .font(.system(size: 15, weight: .semibold))
            }
            .padding(.bottom, 8)

            Text(l10n.tr("Your Mac keeps running with the lid closed, so an agent can finish what it is doing and your phone can still reach it.",
                         "合上盖子 Mac 也继续跑，正在干活的 agent 能干完，手机也还连得上。"))
                .font(.system(size: 12))
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.bottom, 13)

            // Hanging indent: the wrapped line of a bullet lines up with its text,
            // not with the bullet. The NSAlert version wrapped back to the margin.
            VStack(alignment: .leading, spacing: 10) {
                ForEach(Array(points.enumerated()), id: \.offset) { i, line in
                    HStack(alignment: .firstTextBaseline, spacing: 7) {
                        Circle().frame(width: 3, height: 3).foregroundStyle(.tertiary)
                            .padding(.top, 5)
                        // The mark rides at the end of the first sentence, so the claim
                        // and the thing it describes are read in one pass.
                        (i == 0 ? firstPoint : Text(line))
                            .font(.system(size: 12))
                            .fixedSize(horizontal: false, vertical: true)
                    }
                }
            }

            if let os = unverifiedOS {
                // A caution the reader must be able to READ: amber as ink on white is
                // the one thing #F59E0B is bad at, so the tint carries the colour and
                // the text takes the darker amber the quota bars already use.
                HStack(alignment: .top, spacing: 9) {
                    Image(systemName: "exclamationmark.triangle")
                        .font(.system(size: 12))
                        .foregroundStyle(cautionInk)
                    Text(l10n.tr("gtmux has not verified this on macOS \(os), and the setting it uses is undocumented. Check it once: turn it on, close the lid for two minutes, then see whether your phone still reaches this Mac.",
                                 "这台机器的 macOS \(os)，gtmux 还没验证过，它用的那个系统设置苹果也没写进文档。请你自己验一次：开启后合盖两分钟，再看手机是不是还连得上这台 Mac。"))
                        .font(.system(size: 11))
                        .foregroundStyle(cautionInk)
                        .fixedSize(horizontal: false, vertical: true)
                }
                .padding(11)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(RoundedRectangle(cornerRadius: 8)
                    .fill(Theme.Status.errored.opacity(scheme == .dark ? 0.16 : 0.10)))
                .overlay(RoundedRectangle(cornerRadius: 8)
                    .stroke(Theme.Status.errored.opacity(0.34)))
                .padding(.top, 14)
            }

            Divider().padding(.vertical, 12)

            // The password note gets its own line. Sharing one with the buttons, it
            // pushed them off-centre as soon as the sentence wrapped, which in Chinese
            // it always did.
            Text(l10n.tr("macOS asks for your administrator password once.",
                         "macOS 会要你输一次管理员密码。"))
                .font(.system(size: 11))
                .foregroundStyle(.tertiary)
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, alignment: .leading)

            HStack(spacing: 10) {
                Spacer()
                Button(l10n.tr("Cancel", "取消"), action: onCancel)
                    .keyboardShortcut(.cancelAction)
                Button(unverifiedOS == nil
                       ? l10n.tr("Turn on", "开启")
                       : l10n.tr("Turn on anyway", "仍然开启"), action: onConfirm)
                    .keyboardShortcut(.defaultAction)
                    .buttonStyle(.borderedProminent)
            }
            .padding(.top, 11)
        }
        .padding(18)
        .frame(width: 420)
    }
}
