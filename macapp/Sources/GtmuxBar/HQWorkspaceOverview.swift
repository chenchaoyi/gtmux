import SwiftUI

/// Current decisions are explicit matters. Continuity prose stays a secondary source.
struct HQWorkspaceOverview: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQWorkspaceStore

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 24) {
                HStack(alignment: .top) {
                    VStack(alignment: .leading, spacing: 6) {
                        Text(l10n.tr("Overview", "总览")).font(HQTypography.title)
                        Text(l10n.tr("Decisions and work followed by HQ.", "查看待处理事项和 HQ 跟进的工作。"))
                            .font(HQTypography.body).foregroundStyle(.secondary)
                    }
                    Spacer(minLength: 12)
                    Button(l10n.tr("Talk to HQ", "与 HQ 对话")) { store.select(.conversation) }
                }
                if store.snapshot?.matters.isEmpty == true {
                    HQWorkspaceEmptyMatters(l10n: l10n, store: store)
                } else {
                    ForEach(HQMatterGroup.allCases) { group in
                        let matters = group.matters(in: store.snapshot)
                        if !matters.isEmpty {
                            VStack(alignment: .leading, spacing: 8) {
                                HStack(spacing: 8) {
                                    StatusBadge(status: group.status, size: 17).accessibilityHidden(true)
                                    Text(group.title(l10n)).font(HQTypography.rowTitle)
                                    Text("\(matters.count)").font(HQTypography.body.monospacedDigit()).foregroundStyle(
                                        .secondary)
                                }
                                VStack(spacing: 0) {
                                    ForEach(matters) { matter in
                                        if matter.id != matters.first?.id { Divider().padding(.horizontal, 12) }
                                        Button {
                                            store.select(.matter(matter.id))
                                        } label: {
                                            HStack(spacing: 12) {
                                                VStack(alignment: .leading, spacing: 6) {
                                                    Text(matter.title).font(HQTypography.rowTitle).foregroundStyle(
                                                        .primary
                                                    )
                                                    .multilineTextAlignment(.leading).fixedSize(
                                                        horizontal: false, vertical: true)
                                                    HStack(spacing: 8) {
                                                        Text(matter.stateLabel(l10n))
                                                        if !matter.activeParticipants.isEmpty {
                                                            Text(
                                                                l10n.tr(
                                                                    "\(matter.activeParticipants.count) conversations",
                                                                    "\(matter.activeParticipants.count) 个关联会话"))
                                                        }
                                                        if matter.updatedAt > 0 {
                                                            Text(
                                                                HQChatTimeline.label(
                                                                    Date(
                                                                        timeIntervalSince1970: TimeInterval(
                                                                            matter.updatedAt)),
                                                                    zh: l10n.lang == "zh")
                                                            )
                                                            .help(l10n.tr("Last matter update", "事项更新时间"))
                                                        }
                                                    }.font(HQTypography.caption).foregroundStyle(.secondary)
                                                }
                                                Spacer(minLength: 0)
                                                Image(systemName: "chevron.right").font(HQTypography.body)
                                                    .foregroundStyle(.secondary)
                                            }.padding(12).frame(maxWidth: .infinity, minHeight: 58, alignment: .leading)
                                                .contentShape(Rectangle())
                                        }.buttonStyle(.plain)
                                    }
                                }.background(Color.primary.opacity(0.025), in: RoundedRectangle(cornerRadius: 10))
                                    .overlay(
                                        RoundedRectangle(cornerRadius: 10).strokeBorder(Color.primary.opacity(0.08)))
                            }
                        }
                    }
                }
                Divider()
                HStack(alignment: .center, spacing: 12) {
                    Image(systemName: HQResourcePage.board.symbol).font(.system(size: 20)).foregroundStyle(.secondary)
                    VStack(alignment: .leading, spacing: 4) {
                        Text(l10n.tr("Handoff snapshot", "交接快照")).font(HQTypography.rowTitle)
                        Text(l10n.tr("HQ’s saved context and handoff notes.", "HQ 保存的工作背景与交接记录。"))
                            .font(HQTypography.body).foregroundStyle(.secondary)
                    }
                    Spacer(minLength: 8)
                    Button(l10n.tr("View snapshot", "查看快照")) { store.select(.resource(.board)) }
                }
            }.padding(22).frame(maxWidth: 900, alignment: .leading).frame(maxWidth: .infinity)
        }
    }
}
