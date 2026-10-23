import SwiftUI

/// Availability comes from the core's pending plan, never from radar row counts.
/// Kept as a view so plan and in-flight updates remain observable during recovery.
struct RestoreSection: View {
    @ObservedObject var store: AgentStore
    @ObservedObject var l10n: L10n
    var onRestore: () -> Void
    @State private var restoreExpanded = false
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(spacing: 0) {
            if store.showsRestore {
                restoreRow(p)
                Divider().overlay(p.divider)
            }
        }
    }

    @ViewBuilder private func restoreRow(_ p: Theme.Palette) -> some View {
        let plan = store.restorePlan
        let count = plan?.sessions.count ?? 0
        if store.restoring {
            restoreBusyRow(p)
        } else if count > 0 {
            restoreBanner(count, p)
        } else if plan == nil {
            restoreQuietRow(p)
        }
    }

    /// What the row says while the work is happening.
    ///
    /// It replaces the button rather than merely dimming it: a restore takes many seconds
    /// (a tab per session, each waiting on the terminal), and a row that looks the same
    /// before and after the click reads as "nothing happened" — which is how a person ends
    /// up asking for the whole working set twice.
    @ViewBuilder private func restoreBusyRow(_ p: Theme.Palette) -> some View {
        HStack(spacing: 7) {
            ProgressView().controlSize(.small).scaleEffect(0.7).frame(width: 13, height: 13)
            Text(l10n.tr("Restoring your working set…", "正在恢复工作现场…"))
                .font(.system(size: 11.5, weight: .semibold))
                .foregroundStyle(p.fg)
            Spacer(minLength: 0)
            Text(l10n.tr("this can take a moment", "可能需要一会儿"))
                .font(.system(size: 10)).foregroundStyle(p.fg3)
        }
        .padding(.horizontal, 12).padding(.vertical, 8)
        .frame(maxWidth: .infinity)
        .background(p.rowSelected)
        // Not tappable at all while it runs — the honest form of "clicking again does
        // nothing", rather than a button that silently discards the click.
        .allowsHitTesting(false)
        .accessibilityElement(children: .combine)
        .accessibilityIdentifier("restore-busy")
    }

    // The prominent form: something IS waiting to come back.
    @ViewBuilder private func restoreBanner(_ count: Int, _ p: Theme.Palette) -> some View {
        VStack(spacing: 0) {
            HStack(spacing: 6) {
                Button { onRestore() } label: {
                    HStack(spacing: 7) {
                        Image(systemName: "arrow.uturn.backward.circle.fill").font(.system(size: 13))
                        Text(l10n.tr("Restore \(count) session\(count == 1 ? "" : "s")", "恢复 \(count) 个会话"))
                            .font(.system(size: 11.5, weight: .semibold))
                        Spacer(minLength: 4)
                    }
                    .foregroundStyle(p.fg)
                    .contentShape(Rectangle())
                }.buttonStyle(.plain)
                .accessibilityIdentifier("restore-action")
                Button { withAnimation(.easeInOut(duration: 0.12)) { restoreExpanded.toggle() } } label: {
                    Image(systemName: restoreExpanded ? "chevron.down" : "chevron.right")
                        .font(.system(size: 12, weight: .semibold))
                        .foregroundStyle(p.fg2)
                        .frame(width: 22, height: 22)
                        .contentShape(Rectangle())
                }.buttonStyle(.plain)
                .help(l10n.tr("Show which sessions would be restored", "看看会恢复哪些会话"))
                .accessibilityIdentifier("restore-expand")
            }
            .padding(.horizontal, 12).padding(.vertical, 8)
            .frame(maxWidth: .infinity)
            .background(p.rowSelected)

            if restoreExpanded, let plan = store.restorePlan {
                restorePlanList(plan, p)
            }
        }
    }

    // The quiet form: we do not KNOW whether there is anything to restore.
    @ViewBuilder private func restoreQuietRow(_ p: Theme.Palette) -> some View {
        VStack(spacing: 0) {
            HStack(spacing: 6) {
                // Same weight as the form that KNOWS there is something to restore. It
                // used to be 11pt medium at fg2 with no background — the faintest thing on
                // a panel whose emptiness is the reason you are looking at it. "We have
                // not counted the sessions yet" is a reason to word it carefully, not a
                // reason to whisper the one action the screen offers.
                Button { onRestore() } label: {
                    HStack(spacing: 7) {
                        Image(systemName: "arrow.uturn.backward.circle.fill").font(.system(size: 13))
                        Text(l10n.tr("Restore sessions", "恢复会话"))
                            .font(.system(size: 11.5, weight: .semibold))
                        Spacer(minLength: 0)
                    }
                    .foregroundStyle(p.fg)
                    .contentShape(Rectangle())
                }.buttonStyle(.plain)
            }
            .padding(.horizontal, 12).padding(.vertical, 8)
            .frame(maxWidth: .infinity)
            .background(p.rowSelected)
            // No chevron here on purpose: with no plan there is nothing to expand into.
        }
    }

    @ViewBuilder private func restorePlanList(_ plan: RestorePlan, _ p: Theme.Palette) -> some View {
        VStack(alignment: .leading, spacing: 5) {
            ForEach(plan.sessions) { s in
                VStack(alignment: .leading, spacing: 2) {
                    HStack(spacing: 6) {
                        Text(s.name).font(.system(size: 11, weight: .medium)).foregroundStyle(p.fg2)
                            .lineLimit(1).truncationMode(.tail)
                        Text(l10n.tr("\(s.windows)w · \(s.panes)p", "\(s.windows) 窗口 · \(s.panes) 窗格"))
                            .font(.system(size: 9)).foregroundStyle(p.fg3)
                        Spacer(minLength: 0)
                        if let state = s.state {
                            Text(state == "detached" ? l10n.tr("Detached", "未连接") : l10n.tr("From save", "来自存档"))
                                .font(.system(size: 9)).foregroundStyle(p.fg3)
                        }
                    }
                    ForEach(s.agents) { a in
                        HStack(spacing: 5) {
                            // ↻ resumable · × transcript gone (won't resume). Color, not
                            // just glyph, matches the status language (green=ok, dim=gone).
                            Image(systemName: a.alive ? "arrow.clockwise" : "xmark")
                                .font(.system(size: 12, weight: .semibold))
                                .foregroundStyle(a.alive ? Theme.Status.idle : p.fg3)
                            Text(a.label).font(.system(size: 10)).foregroundStyle(p.fg3)
                                .lineLimit(1).truncationMode(.tail)
                            Spacer(minLength: 0)
                        }.padding(.leading, 10)
                    }
                }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("restore-plan")
        .padding(.horizontal, 14).padding(.top, 1).padding(.bottom, 7)
        .frame(maxWidth: .infinity, alignment: .leading)
    }

}
