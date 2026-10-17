import SwiftUI

struct HQWorkspaceActivityView: View {
    let l10n: L10n
    let detail: HQWorkspaceDetail?
    var openMatter: ((String) -> Void)? = nil
    @State private var category: HQActivityCategory?
    @State private var selected: HQActivityItem?

    var body: some View {
        let all = HQActivityTimeline.items(detail)
        let filtered = all.filter { $0.presentation.includes(in: category) }
        VStack(alignment: .leading, spacing: 0) {
            ViewThatFits(in: .horizontal) {
                HStack(spacing: 12) { title(count: filtered.count); Spacer(minLength: 8); controls(all) }
                VStack(alignment: .leading, spacing: 10) {
                    title(count: filtered.count)
                    HStack { controls(all); Spacer(minLength: 0) }
                }
            }.padding(18)
            Divider()
            ForEach(Array(Set(detail?.warnings ?? [])).sorted(), id: \.self) { code in
                Text(hqWorkspaceWarning(code, l10n)).font(HQTypography.caption).foregroundStyle(.secondary)
                    .padding(.horizontal, 18).padding(.vertical, 6)
            }
            if detail == nil {
                HQBrandLoading(label: l10n.tr("Loading activity…", "正在载入运行记录…"))
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else if filtered.isEmpty {
                ContentUnavailableView(
                    l10n.tr("No matching activity", "暂无此类记录"), systemImage: category?.symbol ?? "clock",
                    description: Text(l10n.tr("Choose another type to see more records.", "可切换类型查看其他记录。")))
            } else {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 20) {
                        ForEach(HQActivityTimeline.days(all, category: category)) { day in
                            VStack(alignment: .leading, spacing: 10) {
                                Text(HQActivityTimeline.dateLabel(day.at, zh: l10n.lang == "zh"))
                                    .font(HQTypography.section).accessibilityAddTraits(.isHeader)
                                VStack(spacing: 0) {
                                    ForEach(day.episodes) { episode in
                                        HQActivityEpisodeView(l10n: l10n, episode: episode, openMatter: openMatter) { selected = $0 }
                                    }
                                }
                            }
                        }
                    }.padding(18).frame(maxWidth: 1100, alignment: .leading)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
        }.sheet(item: $selected) { item in
            HQActivityRecordView(l10n: l10n, item: item) { selected = nil }
        }
    }
    private func title(count: Int) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 10) {
            Text(l10n.tr("HQ activity", "HQ 运行记录")).font(HQTypography.title)
            Text(l10n.tr("\(count) loaded", "已载入 \(count) 条"))
                .font(HQTypography.caption.monospacedDigit()).foregroundStyle(.secondary)
        }.fixedSize(horizontal: true, vertical: false)
    }
    private func controls(_ all: [HQActivityItem]) -> some View {
        HStack(spacing: 8) {
            HQWorkspaceInfo(
                title: l10n.tr("Recording scope", "记录范围"),
                text: l10n.tr(
                    "Workspace actions and HQ journal events, ordered by recorded time. Up to 200 per source; journal events cover the last seven days. Consecutive actions on the same matter within five minutes are grouped without removing records. This is loaded history, not a total count or proof of task completion.",
                    "按记录时间排列工作台操作和 HQ 事件。每个来源最多载入 200 条，事件覆盖近七天。同一事项五分钟内的连续操作归为一组，保留每条原记录。这里显示已载入历史，不是全部操作次数，也不代表任务已完成。"),
                iconOnly: true, done: l10n.tr("Done", "完成"))
            Picker(l10n.tr("Activity type", "动作类型"), selection: $category) {
                Text(l10n.tr("All actions", "全部动作")).tag(Optional<HQActivityCategory>.none)
                ForEach(HQActivityCategory.allCases) { c in
                    Label("\(c.title(l10n)) · \(all.filter { $0.presentation.category == c }.count)", systemImage: c.symbol)
                        .tag(Optional(c))
                }
            }.pickerStyle(.menu).labelsHidden().font(HQTypography.navigation)
                .accessibilityLabel(l10n.tr("Filter activity by type", "按动作类型筛选"))
        }.fixedSize(horizontal: true, vertical: false)
    }

}

struct HQActivityEpisodeView: View {
    let l10n: L10n
    let episode: HQActivityEpisode
    var openMatter: ((String) -> Void)? = nil
    let inspect: (HQActivityItem) -> Void
    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            VStack(alignment: .trailing, spacing: 3) {
                Text(HQActivityTimeline.clock(episode.at))
                if let last = episode.items.last, last.at != episode.at {
                    Text("–" + HQActivityTimeline.clock(last.at))
                }
            }.font(HQTypography.code).foregroundStyle(.secondary)
                .frame(width: 52, alignment: .trailing).padding(.top, 10)
            Color.clear.frame(width: 14).accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 0) {
                if let context = episode.context {
                    if let id = episode.items[0].contextID, let openMatter {
                        Button { openMatter(id) } label: {
                            HStack(spacing: 5) {
                                Text(context).font(HQTypography.rowTitle).multilineTextAlignment(.leading)
                                Image(systemName: "chevron.right").font(HQTypography.caption).foregroundStyle(.secondary)
                            }.padding(.top, 7).padding(.bottom, 3)
                        }.buttonStyle(.plain).accessibilityLabel(l10n.tr("Open matter: ", "打开事项：") + context)
                    } else {
                        Text(context).font(HQTypography.rowTitle).textSelection(.enabled).padding(.top, 7).padding(.bottom, 3)
                    }
                }
                ForEach(episode.items) { item in
                    HStack(alignment: .top, spacing: 8) {
                        Image(systemName: item.presentation.symbol).font(HQTypography.navigation)
                            .frame(width: 20, height: 22)
                            .foregroundStyle(item.presentation.needsAttention ? Theme.Status.errored : Color.secondary)
                            .accessibilityLabel(item.presentation.category.title(l10n))
                        VStack(alignment: .leading, spacing: 3) {
                            HStack(alignment: .firstTextBaseline, spacing: 8) {
                                Text(hqActivityTitle(item.kind, l10n)).font(HQTypography.body)
                                if let pane = item.pane { Text(pane).font(HQTypography.code).foregroundStyle(.secondary) }
                                if let outcome = item.outcome {
                                    Text(hqActivityOutcome(outcome, l10n)).font(HQTypography.caption)
                                        .foregroundStyle(item.presentation.needsAttention ? Theme.Status.errored : Color.secondary)
                                }
                            }
                            if let text = item.detail(l10n), !text.isEmpty, text != episode.context, text != item.target {
                                Text(text).font(HQTypography.caption).foregroundStyle(.secondary).lineLimit(2)
                            }
                            if let target = item.target {
                                Text(l10n.tr("Target: \(target)", "目标：\(target)"))
                                    .font(HQTypography.caption).foregroundStyle(.secondary).lineLimit(1)
                            }
                        }.frame(maxWidth: .infinity, alignment: .leading)
                        Button { inspect(item) } label: {
                            Image(systemName: "doc.text.magnifyingglass").font(HQTypography.navigation)
                                .frame(width: 28, height: 28)
                        }.buttonStyle(.borderless).foregroundStyle(.secondary)
                            .accessibilityLabel(l10n.tr("View full record: ", "查看完整记录：") + hqActivityTitle(item.kind, l10n))
                    }.padding(.vertical, 5)
                }
                Divider().padding(.top, 6)
            }.padding(.bottom, 10)
        }.fixedSize(horizontal: false, vertical: true)
            .background(alignment: .leading) {
                Rectangle().fill(Color.primary.opacity(0.12)).frame(width: 1).offset(x: 71)
            }
            .overlay(alignment: .topLeading) {
                Circle().fill(Color.secondary.opacity(0.5)).frame(width: 7, height: 7)
                    .frame(width: 14, height: 26).offset(x: 64).accessibilityHidden(true)
            }
    }
}

func hqActivityOutcome(_ raw: String, _ l: L10n) -> String {
    switch raw {
    case "ok": return l.tr("Recorded", "已记录")
    case "failed", "partial-or-failed", "committed-view-failed": return l.tr("Failed", "失败")
    case "refused": return l.tr("Refused", "已拒绝")
    case "resource-not-closed": return l.tr("Cleanup incomplete", "清理未完成")
    default: return raw
    }
}
