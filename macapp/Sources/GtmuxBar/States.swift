import AppKit
import SwiftUI

/// EmptyStateView (DESIGN §5) — two doors, not a poster.
///
/// It used to be a centred card: the app mark again (the header already draws it), three
/// stacked lines of near-equal weight, and a command you could only copy by dragging the
/// mouse across it inside a popover that closes when you click away. The one action it
/// implied lived in the footer, as far from "there is nothing here" as the panel allows.
///
/// So: what this screen is, in two lines; the command, with a button that copies it; then
/// the doors, in the same row language the agent list uses — "New session" here, and the
/// restore row MenuView draws directly below it when there is a working set to come back
/// to. Nothing is centred, nothing is repeated, and every line says one thing.
struct EmptyStateView: View {
    @ObservedObject var l10n: L10n
    var onNew: () -> Void = {}
    @Environment(\.colorScheme) private var scheme
    /// The copy button's confirmation. It reverts on its own: a button that says "Copied"
    /// forever cannot tell you whether the NEXT click worked.
    @State private var copied = false

    /// The command the copy button puts on the pasteboard, and the line on screen.
    static let startCommand = "tmux new -s work \\; claude"

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(alignment: .leading, spacing: 0) {
            VStack(alignment: .leading, spacing: 3) {
                Text(l10n.tr("No agents running", "没有运行中的 agent"))
                    .font(.system(size: 13.5, weight: .semibold)).foregroundStyle(p.fg)
                Text(l10n.tr("Start one and it shows up here, with what it is waiting for.",
                             "启动一个，它就会出现在这里，并告诉你它在等什么。"))
                    .font(.system(size: 11.5)).foregroundStyle(p.fg2)
                    .fixedSize(horizontal: false, vertical: true)
            }
            .padding(.horizontal, 14).padding(.top, 13).padding(.bottom, 11)

            command(p)

            Divider().overlay(p.divider)
            newSessionRow(p)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    /// The command, and who gtmux recognises when it runs. The named agents are the
    /// HOOK-EQUIPPED ones (internal/agents: claude, codex, cursor, gemini, opencode) — the
    /// tier gtmux genuinely senses turn by turn. aider was listed here once and is
    /// detect-only, which advertised the shallowest support we have as an example of what
    /// gtmux is for.
    @ViewBuilder private func command(_ p: Theme.Palette) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(l10n.tr("Or start one yourself, in any terminal", "也可以自己来，在任意终端里"))
                .font(.system(size: 10.5)).foregroundStyle(p.fg3)
            HStack(spacing: 8) {
                Text(verbatim: Self.startCommand)
                    .font(Theme.Font.mono).foregroundStyle(p.fg)
                    .textSelection(.enabled)
                    .lineLimit(1).truncationMode(.tail)
                Spacer(minLength: 0)
                Button(action: copyCommand) {
                    HStack(spacing: 4) {
                        Image(systemName: copied ? "checkmark" : "doc.on.doc")
                            .font(.system(size: 12, weight: .semibold))
                        Text(copied ? l10n.tr("Copied", "已复制") : l10n.tr("Copy", "复制"))
                            .font(.system(size: 10.5, weight: .medium))
                    }
                    .foregroundStyle(copied ? Theme.Status.idle : p.fg2)
                    .padding(.horizontal, 8).padding(.vertical, 4)
                    .background(RoundedRectangle(cornerRadius: 6, style: .continuous).fill(p.rowSelected))
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .help(l10n.tr("Copy the command", "复制这行命令"))
            }
            Text(l10n.tr("Claude Code · Codex · Cursor · Gemini · opencode are all recognised",
                         "Claude Code · Codex · Cursor · Gemini · opencode 都认得出来"))
                .font(.system(size: 10.5)).foregroundStyle(p.fg3)
                .lineLimit(1).truncationMode(.tail)
        }
        .padding(.horizontal, 14).padding(.bottom, 12)
    }

    /// The first door. The restore row MenuView draws below is the second; both are rows
    /// of the same shape, so the panel offers two comparable things rather than a button
    /// and a banner.
    @ViewBuilder private func newSessionRow(_ p: Theme.Palette) -> some View {
        Button(action: onNew) {
            HStack(spacing: 10) {
                Image(systemName: "plus")
                    .font(.system(size: 13, weight: .semibold)).foregroundStyle(p.fg)
                    .frame(width: 26, height: 26)
                    .background(RoundedRectangle(cornerRadius: 7, style: .continuous).fill(p.rowSelected))
                VStack(alignment: .leading, spacing: 1) {
                    Text(l10n.tr("New session", "新建会话"))
                        .font(.system(size: 12, weight: .semibold)).foregroundStyle(p.fg)
                    Text(l10n.tr("opens a terminal tab with tmux running", "开一个终端标签页，里面跑着 tmux"))
                        .font(.system(size: 10.5)).foregroundStyle(p.fg2)
                        .lineLimit(1).truncationMode(.tail)
                }
                Spacer(minLength: 0)
                Image(systemName: "chevron.right")
                    .font(.system(size: 12, weight: .semibold)).foregroundStyle(p.fg3)
            }
            .padding(.horizontal, 12).padding(.vertical, 8)
            .frame(maxWidth: .infinity)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }

    private func copyCommand() {
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(Self.startCommand, forType: .string)
        copied = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 2) { copied = false }
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
                "When you click an agent, gtmux uses AppleScript to bring its terminal tab and tmux pane to the front. That needs a one-time “Automation” permission. It only switches windows, and does not read what is in the terminal.",
                "点击某个 agent 时，gtmux 用 AppleScript 把它所在的终端标签页和 tmux pane 切到最前。这需要一次「自动化」授权，只切换窗口、不读取终端内容。"))
                .font(.system(size: 12)).foregroundStyle(p.fg2).multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)

            VStack(alignment: .leading, spacing: 8) {
                step(1, l10n.tr("Click “Allow & continue”, then macOS shows a system dialog.",
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

            Text(l10n.tr("Works without it: agents and overview keep working, you just can't click to jump.",
                         "不授权也能用：agents、overview 照常工作，只是不能点击跳转。"))
                .font(.system(size: 10)).foregroundStyle(p.fg3).multilineTextAlignment(.center)
        }
        .padding(20).frame(width: 360)
        .background(VisualEffectWindow())
    }

    private func step(_ n: Int, _ text: String, _ p: Theme.Palette) -> some View {
        HStack(alignment: .top, spacing: 8) {
            Text("\(n)").font(.system(size: 10, weight: .bold)).foregroundStyle(p.fg2)
                .frame(width: 22, height: 22)
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
