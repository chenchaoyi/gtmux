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
/// unattended — and nothing else.
struct ServerModeConfirmView: View {
    @ObservedObject var l10n: L10n
    /// Non-nil when this machine is a configuration the project has not verified.
    var unverifiedOS: String?
    var onConfirm: () -> Void
    var onCancel: () -> Void

    private var points: [(String, String)] {
        [
            (l10n.tr("Stays on until you turn it off", "一直有效，直到你自己关闭"),
             l10n.tr("The menu-bar icon shows a red dot the whole time.",
                     "菜单栏图标会一直显示一个红点。")),
            (l10n.tr("Works on battery", "用电池也能跑"),
             l10n.tr("Carry it between rooms with the lid shut. Sleep comes back on its own at 20%.",
                     "可以合着盖子带着走。电量到 20% 会自动恢复睡眠。")),
            (l10n.tr("Runs hotter with the lid shut", "合盖散热更差"),
             l10n.tr("Keep it on a hard surface, not in a bag. A fanless Air suffers most.",
                     "请放在硬质平面，别塞进包里；无风扇的 Air 影响最大。")),
            (l10n.tr("Stays reachable while unattended", "无人值守时保持可远程访问"),
             l10n.tr("Your screen lock is unaffected.", "屏幕锁不受影响，照常按你的设置锁。")),
        ]
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 10) {
                Image(systemName: "powersleep")
                    .font(.system(size: 20, weight: .regular))
                    .foregroundStyle(Theme.Status.waiting)
                Text(l10n.tr("Turn on server mode?", "开启服务器模式？"))
                    .font(.system(size: 15, weight: .semibold))
            }
            .padding(.bottom, 8)

            Text(l10n.tr("Your Mac keeps running with the lid closed, so your agents, the tunnel and your phone keep working.",
                         "合上盖子 Mac 也继续运行 —— agent、隧道和手机端都不中断。"))
                .font(.system(size: 12))
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.bottom, 12)

            // Hanging indent: the wrapped line of a bullet lines up with its text,
            // not with the bullet. The NSAlert version wrapped back to the margin.
            VStack(alignment: .leading, spacing: 9) {
                ForEach(points, id: \.0) { title, detail in
                    HStack(alignment: .firstTextBaseline, spacing: 7) {
                        Circle().frame(width: 3, height: 3).foregroundStyle(.tertiary)
                            .padding(.top, 5)
                        VStack(alignment: .leading, spacing: 1) {
                            Text(title).font(.system(size: 12, weight: .medium))
                            Text(detail).font(.system(size: 11)).foregroundStyle(.secondary)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                    }
                }
            }

            if let os = unverifiedOS {
                Divider().padding(.vertical, 12)
                Text(l10n.tr("Not verified on macOS \(os). It relies on an undocumented system setting, so please check it once: turn it on, close the lid for two minutes, then confirm it kept serving.",
                             "macOS \(os) 未经验证。它依赖一项未公开文档的系统设置，请验证一次：开启后合盖两分钟，再确认它一直在服务。"))
                    .font(.system(size: 11))
                    .foregroundStyle(Color(Theme.Status.errored))
                    .fixedSize(horizontal: false, vertical: true)
            }

            Divider().padding(.vertical, 12)

            HStack(spacing: 10) {
                Text(l10n.tr("macOS will ask for your administrator password.",
                             "macOS 会要求输入一次管理员密码。"))
                    .font(.system(size: 11))
                    .foregroundStyle(.tertiary)
                Spacer()
                Button(l10n.tr("Cancel", "取消"), action: onCancel)
                    .keyboardShortcut(.cancelAction)
                Button(unverifiedOS == nil
                       ? l10n.tr("Turn on", "开启")
                       : l10n.tr("Turn on anyway", "仍然开启"), action: onConfirm)
                    .keyboardShortcut(.defaultAction)
                    .buttonStyle(.borderedProminent)
            }
        }
        .padding(18)
        .frame(width: 420)
    }
}
