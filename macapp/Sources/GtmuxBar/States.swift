import SwiftUI

/// EmptyStateView (DESIGN §5): no error, no awkward blank — a copyable start
/// command and copy that names ANY coding agent (not just Claude).
struct EmptyStateView: View {
    @ObservedObject var l10n: L10n
    var onNew: () -> Void = {}
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(spacing: 9) {
            GtmuxLogo(size: 26).opacity(0.85)
            Text(l10n.tr("No agents running", "没有运行中的 agent"))
                .font(.system(size: 13, weight: .medium)).foregroundStyle(p.fg)
            Text(l10n.tr("Start any coding agent in a tmux pane\n(Claude Code · Codex · Gemini · aider…)",
                         "在 tmux pane 里启动任意 coding agent\n(Claude Code · Codex · Gemini · aider…)"))
                .font(.system(size: 11)).foregroundStyle(p.fg2).multilineTextAlignment(.center)
            Text(verbatim: "tmux new -s work \\; claude")
                .font(Theme.Font.mono).foregroundStyle(p.fg)
                .padding(.horizontal, 10).padding(.vertical, 6)
                .background(RoundedRectangle(cornerRadius: 6, style: .continuous)
                    .fill(scheme == .dark ? Color.white.opacity(0.06) : Color.black.opacity(0.05)))
                .textSelection(.enabled)
        }
        .padding(.horizontal, 16).padding(.vertical, 22).frame(maxWidth: .infinity)
    }
}

/// FirstRunView (DESIGN §5): the Automation-permission explainer. Plain,
/// matter-of-fact copy — NO marketing tone.
struct FirstRunView: View {
    @ObservedObject var l10n: L10n
    var terminalName: String = "Ghostty"
    var onAllow: () -> Void
    var onLater: () -> Void
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(spacing: 14) {
            GtmuxLogo(size: 40)
            Text(l10n.tr("Jumping needs “Automation” permission", "跳转需要「自动化」权限"))
                .font(.system(size: 16, weight: .semibold)).foregroundStyle(p.fg)
                .multilineTextAlignment(.center)
            Text(l10n.tr(
                "When you click an agent, gtmux uses AppleScript to bring its terminal tab and tmux pane to the front. That needs a one-time “Automation” grant — it only switches windows, it does not read terminal contents.",
                "点击某个 agent 时，gtmux 用 AppleScript 把它所在的终端标签页和 tmux pane 切到最前。这需要一次「自动化」授权，只切换窗口、不读取终端内容。"))
                .font(.system(size: 12)).foregroundStyle(p.fg2).multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)

            VStack(alignment: .leading, spacing: 8) {
                step(1, l10n.tr("Click “Allow & continue” — macOS shows a system dialog.",
                                "点下方「允许并继续」，会弹出 macOS 系统对话框。"), p)
                step(2, l10n.tr("In “‘gtmux’ wants to control ‘\(terminalName)’”, click OK.",
                                "在「“gtmux” 想要控制 “\(terminalName)”」中点「好」。"), p)
                step(3, l10n.tr("Revoke anytime in System Settings › Privacy & Security › Automation.",
                                "随时可在 系统设置 › 隐私与安全性 › 自动化 撤销。"), p)
            }
            .padding(12)
            .background(RoundedRectangle(cornerRadius: 8, style: .continuous)
                .fill(scheme == .dark ? Color.white.opacity(0.05) : Color.black.opacity(0.04)))

            HStack(spacing: 10) {
                Button(action: onAllow) {
                    Text(l10n.tr("Allow & continue", "允许并继续"))
                        .font(.system(size: 13, weight: .semibold)).foregroundStyle(.white)
                        .frame(maxWidth: .infinity).padding(.vertical, 9)
                        .background(RoundedRectangle(cornerRadius: 8, style: .continuous).fill(Theme.Status.working))
                }.buttonStyle(.plain)
                Button(action: onLater) {
                    Text(l10n.tr("Later", "稍后")).font(.system(size: 13)).foregroundStyle(p.fg2)
                        .padding(.horizontal, 16).padding(.vertical, 9)
                        .background(RoundedRectangle(cornerRadius: 8, style: .continuous)
                            .fill(scheme == .dark ? Color.white.opacity(0.07) : Color.black.opacity(0.05)))
                }.buttonStyle(.plain)
            }

            Text(l10n.tr("Works without it: agents and overview keep working — you just can't click to jump.",
                         "不授权也能用：agents、overview 照常工作，只是不能点击跳转。"))
                .font(.system(size: 10)).foregroundStyle(p.fg3).multilineTextAlignment(.center)
        }
        .padding(20).frame(width: 360)
        .background(VisualEffectWindow())
    }

    private func step(_ n: Int, _ text: String, _ p: Theme.Palette) -> some View {
        HStack(alignment: .top, spacing: 8) {
            Text("\(n)").font(.system(size: 10, weight: .bold)).foregroundStyle(p.fg2)
                .frame(width: 18, height: 18)
                .background(Circle().fill(scheme == .dark ? Color.white.opacity(0.08) : Color.black.opacity(0.06)))
            Text(text).font(.system(size: 11.5)).foregroundStyle(p.fg2)
                .fixedSize(horizontal: false, vertical: true)
            Spacer(minLength: 0)
        }
    }
}

struct VisualEffectWindow: NSViewRepresentable {
    func makeNSView(context: Context) -> NSVisualEffectView {
        let v = NSVisualEffectView(); v.material = .popover; v.state = .active; return v
    }
    func updateNSView(_ nsView: NSVisualEffectView, context: Context) {}
}
