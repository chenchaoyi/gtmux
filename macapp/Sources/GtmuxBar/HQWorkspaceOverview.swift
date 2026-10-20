import SwiftUI

/// A brief over explicit records. Full navigation remains in the sidebar.
struct HQWorkspaceOverview: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQWorkspaceStore
    @State private var allMatters: HQMatterGroup?

    var body: some View {
        GeometryReader { bounds in
            ScrollView {
                VStack(alignment: .leading, spacing: 24) {
                    HStack(alignment: .top, spacing: 12) {
                        VStack(alignment: .leading, spacing: 6) {
                            Text(l10n.tr("Overview", "总览")).font(HQTypography.title)
                            Text(l10n.tr("Decisions, progress and changes in the last 24 hours.", "需要你决定的事、最新进展，以及最近 24 小时的变化。"))
                                .font(HQTypography.body).foregroundStyle(.secondary)
                        }
                        Spacer(minLength: 4)
                        Button(l10n.tr("Talk to HQ", "与 HQ 对话")) { store.select(.conversation) }
                    }
                    if store.snapshot?.matters.isEmpty == true {
                        HQWorkspaceEmptyMatters(l10n: l10n, store: store)
                    } else {
                        if bounds.size.width >= 780 {
                            HStack(alignment: .top, spacing: 20) {
                                matterSection(.needsInput)
                                matterSection(.following)
                            }
                        } else {
                            matterSection(.needsInput)
                            matterSection(.following)
                        }
                        recentChanges
                    }
                    Divider()
                    HStack(spacing: 10) {
                        Button { store.select(.resource(.board)) } label: {
                            Label(l10n.tr("Handoff snapshot", "交接快照"), systemImage: HQResourcePage.board.symbol)
                        }.buttonStyle(.borderless)
                        Text(l10n.tr("Saved context and handoff notes", "保存的背景与交接记录"))
                            .font(HQTypography.caption).foregroundStyle(.secondary)
                    }
                }.padding(22).frame(maxWidth: 1000, alignment: .leading).frame(maxWidth: .infinity)
            }
        }
        .sheet(item: $allMatters) { group in
            HQOverviewMatterPicker(l10n: l10n, group: group, matters: group.matters(in: store.snapshot)) {
                allMatters = nil
                store.select(.matter($0))
            } close: { allMatters = nil }
        }
    }

    private func matterSection(_ group: HQMatterGroup) -> some View {
        let matters = group.matters(in: store.snapshot)
        let items = HQOverviewModel.items(group, in: store.snapshot)
        return VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 7) {
                StatusBadge(status: group.status, size: 17).accessibilityHidden(true)
                Text(group.title(l10n)).font(HQTypography.section)
                Text("\(matters.count)").font(HQTypography.body.monospacedDigit()).foregroundStyle(.secondary)
                Spacer(minLength: 4)
                if matters.count > items.count {
                    Button(l10n.tr("View all", "查看全部")) { allMatters = group }
                        .font(HQTypography.caption).accessibilityIdentifier("hq-overview-all-\(group.rawValue)")
                }
            }
            if matters.isEmpty {
                Text(group == .needsInput
                     ? l10n.tr("Nothing needs your input right now.", "目前没有待你处理的事项。")
                     : l10n.tr("No matters are being followed.", "目前没有跟进中的事项。"))
                    .font(HQTypography.body).foregroundStyle(.secondary).padding(.vertical, 10)
            } else {
                VStack(spacing: 0) {
                    ForEach(items) { item in
                        if item.id != items.first?.id { Divider().padding(.horizontal, 12) }
                        if let matter = matters.first(where: { $0.id == item.matterId }) {
                            HQOverviewMatterRow(l10n: l10n, item: item, matter: matter,
                                                workers: HQOverviewModel.workers(matter, in: store.snapshot)) {
                                store.select(.matter(matter.id))
                            }
                        }
                    }
                }.background(Color.primary.opacity(0.025), in: RoundedRectangle(cornerRadius: 10))
                    .overlay(RoundedRectangle(cornerRadius: 10).strokeBorder(Color.primary.opacity(0.08)))
            }
        }.frame(maxWidth: .infinity, alignment: .leading)
    }

    private var recentChanges: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Text(l10n.tr("Recent changes", "最近变化")).font(HQTypography.section)
                Text(l10n.tr("24 hours", "24 小时")).font(HQTypography.caption).foregroundStyle(.secondary)
                Spacer()
                Button(l10n.tr("All activity", "全部运行记录")) { store.select(.activity) }
                    .font(HQTypography.caption)
            }
            if let overview = store.snapshot?.overview {
                if overview.recent.isEmpty {
                    Text(l10n.tr("No recorded matter changes in this period.", "这段时间没有可展示的事项变化。"))
                        .font(HQTypography.body).foregroundStyle(.secondary).padding(.vertical, 8)
                } else {
                    ForEach(overview.recent) { change in
                        if let matter = store.snapshot?.matters.first(where: { $0.id == change.matterId }) {
                            Button { store.select(.matter(matter.id)) } label: {
                                HStack(alignment: .firstTextBaseline, spacing: 12) {
                                    Text(HQChatTimeline.label(Date(timeIntervalSince1970: TimeInterval(change.at)), zh: l10n.lang == "zh"))
                                        .font(HQTypography.caption).foregroundStyle(.secondary)
                                        .frame(width: 110, alignment: .leading).lineLimit(1)
                                    Text(change.title(l10n)).font(HQTypography.caption).foregroundStyle(.secondary)
                                    Text(matter.title).font(HQTypography.body).foregroundStyle(.primary).lineLimit(1)
                                    Spacer(minLength: 0)
                                    Image(systemName: "chevron.right").font(HQTypography.caption).foregroundStyle(.tertiary)
                                }.frame(maxWidth: .infinity, minHeight: 32, alignment: .leading).contentShape(Rectangle())
                            }.buttonStyle(.plain)
                        }
                    }
                    if overview.recentTotal > overview.recent.count {
                        Text(l10n.tr("Latest \(overview.recent.count) of \(overview.recentTotal) changed matters",
                                     "展示 \(overview.recentTotal) 个有变化事项中的最近 \(overview.recent.count) 个"))
                            .font(HQTypography.caption).foregroundStyle(.secondary)
                    }
                }
            } else {
                Text(l10n.tr("Matter change details are unavailable.", "暂时无法读取事项变化明细。"))
                    .font(HQTypography.body).foregroundStyle(.secondary)
            }
        }
    }
}
