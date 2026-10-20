import SwiftUI

struct HQOverviewMatterRow: View {
    let l10n: L10n
    let item: HQOverviewItem
    let matter: HQMatter
    let workers: [HQCandidate]
    let open: () -> Void
    var body: some View {
        Button(action: open) {
            VStack(alignment: .leading, spacing: 7) {
                HStack(alignment: .top, spacing: 8) {
                    Text(matter.title).font(HQTypography.rowTitle).foregroundStyle(.primary)
                        .lineLimit(2).multilineTextAlignment(.leading)
                    Spacer(minLength: 4)
                    Image(systemName: "chevron.right").font(HQTypography.caption).foregroundStyle(.tertiary)
                }
                if let preview = item.preview {
                    Text(HQOverviewModel.snippet(preview.body))
                        .font(HQTypography.body).foregroundStyle(.secondary).lineLimit(3)
                        .multilineTextAlignment(.leading)
                    HStack(spacing: 5) {
                        Text(preview.kind == "assessment"
                             ? l10n.tr("Saved assessment", "已存评估")
                             : hqActivityTitle("hq-" + preview.kind, l10n))
                        if let at = preview.at, at > 0 {
                            Text("· " + HQChatTimeline.label(Date(timeIntervalSince1970: TimeInterval(at)), zh: l10n.lang == "zh"))
                        }
                        if preview.truncated == true { Text("· " + l10n.tr("Excerpt", "节选")) }
                    }.font(HQTypography.caption).foregroundStyle(.secondary).lineLimit(1)
                } else {
                    Text(l10n.tr("Open the conversation for context.", "打开对话查看具体背景。"))
                        .font(HQTypography.body).foregroundStyle(.secondary)
                }
                if let worker = workers.first {
                    HStack(spacing: 6) {
                        StatusBadge(status: Status(rawValue: worker.status) ?? .running, size: 12).accessibilityHidden(true)
                        Text(worker.name.isEmpty ? worker.agent : worker.name).lineLimit(1)
                        Text("· " + hqConversationStatus(worker.status, l10n)).fixedSize()
                        if workers.count > 1 { Text(l10n.tr("+\(workers.count - 1) more", "另 \(workers.count - 1) 个")) }
                    }.font(HQTypography.caption).foregroundStyle(.secondary).lineLimit(1)
                }
            }.frame(maxWidth: .infinity, alignment: .leading).padding(12).contentShape(Rectangle())
        }.buttonStyle(.plain)
            .accessibilityIdentifier("hq-overview-matter-\(matter.id)")
            .accessibilityLabel(matter.title + ", " + matter.stateLabel(l10n))
    }
}

struct HQOverviewMatterPicker: View {
    let l10n: L10n
    let group: HQMatterGroup
    let matters: [HQMatter]
    let pick: (String) -> Void
    let close: () -> Void
    @Environment(\.hqWorkspaceSheetSize) private var size
    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Text(group.title(l10n)).font(HQTypography.title)
                Text("\(matters.count)").font(HQTypography.body).foregroundStyle(.secondary)
                Spacer()
                Button(l10n.tr("Done", "完成"), action: close).keyboardShortcut(.cancelAction)
            }.padding(18)
            Divider()
            ScrollView {
                LazyVStack(spacing: 0) {
                    ForEach(matters) { matter in
                        Button { pick(matter.id) } label: {
                            HStack(spacing: 10) {
                                StatusBadge(status: group.status, size: 16).accessibilityHidden(true)
                                Text(matter.title).font(HQTypography.body).multilineTextAlignment(.leading)
                                Spacer(minLength: 8)
                                Image(systemName: "chevron.right").foregroundStyle(.secondary)
                            }.padding(14).frame(maxWidth: .infinity, alignment: .leading).contentShape(Rectangle())
                        }.buttonStyle(.plain).accessibilityIdentifier("hq-overview-picker-\(matter.id)")
                        Divider().padding(.leading, 40)
                    }
                }
            }
        }.frame(width: size.width, height: size.height)
    }
}
