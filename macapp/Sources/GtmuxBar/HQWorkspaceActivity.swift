import SwiftUI

struct HQWorkspaceActivityView: View {
    let l10n: L10n
    let detail: HQWorkspaceDetail?
    @State private var category: HQActivityCategory?

    private var kinds: [String] { (detail?.changes ?? []).map(\.kind) + (detail?.events ?? []).map(\.event) }
    private func count(_ category: HQActivityCategory?) -> Int {
        kinds.filter { HQActivityPresentation(kind: $0).includes(in: category) }.count
    }
    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            VStack(alignment: .leading, spacing: 8) {
                Text(l10n.tr("HQ activity", "HQ 运行记录")).font(HQTypography.title)
                HStack(alignment: .firstTextBaseline, spacing: 12) {
                    Text(l10n.tr("Browse HQ actions by type.", "按类型查看 HQ 的操作与处理记录。"))
                        .font(HQTypography.caption).foregroundStyle(.secondary)
                    Spacer(minLength: 8)
                    Picker(l10n.tr("Activity type", "动作类型"), selection: $category) {
                        Text(l10n.tr("All actions", "全部动作")).tag(Optional<HQActivityCategory>.none)
                        ForEach(HQActivityCategory.allCases) { c in
                            Label("\(c.title(l10n)) · \(count(c))", systemImage: c.symbol).tag(Optional(c))
                        }
                    }.pickerStyle(.menu).labelsHidden().font(HQTypography.navigation)
                        .accessibilityLabel(l10n.tr("Filter activity by type", "按动作类型筛选"))
                    if detail != nil {
                        Text(l10n.tr("\(count(category)) records", "\(count(category)) 条记录"))
                            .font(HQTypography.caption.monospacedDigit()).foregroundStyle(.secondary)
                    }
                }
                Text(l10n.tr("Up to 200 records per source; journal events cover seven days.", "每个来源最多显示 200 条，事件记录覆盖近七天。"))
                    .font(HQTypography.caption).foregroundStyle(.secondary)
            }.padding([.horizontal, .top], 18)
            if detail == nil {
                HQBrandLoading(label: l10n.tr("Loading activity…", "正在载入运行记录…"))
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else if count(category) == 0 {
                ContentUnavailableView(
                    l10n.tr("No matching activity", "暂无此类记录"), systemImage: category?.symbol ?? "clock",
                    description: Text(l10n.tr("Choose another type to see more records.", "可切换类型查看其他记录。")))
            } else {
                List {
                    let changes = (detail?.changes ?? []).reversed().filter { HQActivityPresentation(kind: $0.kind).includes(in: category) }
                    if !changes.isEmpty {
                        Section { ForEach(changes) { change in changeRow(change) } }
                        header: { Text(l10n.tr("Workspace actions", "工作台操作")).font(HQTypography.section) }
                    }
                    let events = (detail?.events ?? []).reversed().filter { HQActivityPresentation(kind: $0.event).includes(in: category) }
                    if !events.isEmpty {
                        Section {
                            ForEach(events) { e in
                                DisclosureGroup {
                                    VStack(alignment: .leading, spacing: 8) {
                                        if let summary = e.summary { Text(summary).textSelection(.enabled) }
                                        Text([e.event, e.pane, e.agentSession, e.outcome].compactMap { $0 }.joined(separator: " · "))
                                            .font(HQTypography.code).textSelection(.enabled)
                                    }.font(HQTypography.body).padding(.vertical, 6)
                                } label: {
                                    HStack(alignment: .top, spacing: 12) {
                                        icon(HQActivityPresentation(kind: e.event, outcome: e.outcome))
                                        Text(hqActivityTitle(e.event, l10n)).font(HQTypography.rowTitle)
                                        Spacer(minLength: 8)
                                        timestamp(e.ts)
                                    }
                                }.padding(.vertical, 7)
                            }
                        } header: { Text(l10n.tr("HQ event journal", "HQ 事件记录")).font(HQTypography.section) }
                    }
                }.listStyle(.inset)
            }
        }
    }
    private func changeRow(_ c: HQWorkspaceChange) -> some View {
        let presentation = HQActivityPresentation(kind: c.kind)
        return HStack(alignment: .top, spacing: 12) {
            icon(presentation)
            VStack(alignment: .leading, spacing: 5) {
                Text(hqActivityTitle(c.kind, l10n)).font(HQTypography.rowTitle)
                if let id = c.matter, let name = detail?.matterTitles?[id] {
                    Text(name).font(HQTypography.body).foregroundStyle(.secondary)
                }
                if let id = c.target, let name = detail?.matterTitles?[id] {
                    Text(l10n.tr("Target: \(name)", "目标：\(name)")).font(HQTypography.caption).foregroundStyle(.secondary)
                }
                if let text = presentation.detail(c.detail, l10n: l10n) {
                    Text(text).font(HQTypography.caption).foregroundStyle(.secondary).textSelection(.enabled)
                }
            }.frame(maxWidth: .infinity, alignment: .leading)
            timestamp(c.at)
        }.padding(.vertical, 7)
    }
    private func icon(_ presentation: HQActivityPresentation) -> some View {
        Image(systemName: presentation.symbol).font(.system(size: 17, weight: .regular))
            .frame(width: 24, height: 23)
            .foregroundStyle(presentation.needsAttention ? Theme.Status.errored : Color.secondary)
            .help(presentation.category.title(l10n))
            .accessibilityLabel(presentation.category.title(l10n))
    }
    private func timestamp(_ at: Int64) -> some View {
        Text(Date(timeIntervalSince1970: TimeInterval(at)), style: .time)
            .font(HQTypography.caption.monospacedDigit()).foregroundStyle(.secondary).fixedSize()
    }
}
