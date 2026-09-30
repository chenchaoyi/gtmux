import AppKit
import SwiftUI

struct HQImportReview: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var flow: HQImportFlow
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(alignment: .leading, spacing: 12) {
            if let manifest = flow.manifest, let preview = flow.preview {
                HStack {
                    Picker(l10n.tr("Content", "内容"), selection: $flow.reviewTab) {
                        if manifest.files["knowledge/.ledger.jsonl"] != nil { Text(l10n.tr("Knowledge", "知识库")).tag(0) }
                        if manifest.files["LOCAL.md"] != nil { Text(l10n.tr("Personal requirements", "个人要求")).tag(1) }
                        if manifest.files.keys.contains(where: { $0.hasPrefix("knowledge/tools/") }) { Text(l10n.tr("Tool attachments", "工具附件")).tag(2) }
                    }.pickerStyle(.segmented).labelsHidden()
                    Spacer()
                    Button(l10n.tr("Refresh review", "刷新核对内容")) { flow.loadReview(l10n: l10n) }
                }
                if flow.reviewTab == 0 {
                    knowledgeReview(preview, p)
                } else if flow.reviewTab == 1 {
                    HStack(alignment: .top, spacing: 12) {
                        textColumn(l10n.tr("Current personal requirements", "当前个人要求"), preview.current_local ?? "", p)
                        Divider()
                        textColumn(l10n.tr("From the archive", "档案中的个人要求"), preview.local ?? "", p)
                    }.frame(maxHeight: .infinity)
                    Toggle(l10n.tr("I reviewed both texts. Replace the current requirements with the archive copy.", "我已核对两份内容，使用档案中的个人要求替换当前内容。"), isOn: $flow.replacePersonal).toggleStyle(.checkbox)
                } else {
                    Text(l10n.tr("Attachments are staged without executable permissions. Check paths and dependencies before using them on this Mac.", "附件已暂存并移除执行权限。请先核对路径和依赖，再决定如何在这台 Mac 上使用。"))
                        .foregroundStyle(p.fg2).fixedSize(horizontal: false, vertical: true)
                    if let path = manifest.path {
                        Button(l10n.tr("Show staged attachments", "查看暂存附件")) { NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: path).appendingPathComponent("knowledge/tools")]) }
                    }
                }
            }
        }
    }

    @ViewBuilder private func knowledgeReview(_ preview: HQMigrationPreview, _ p: Theme.Palette) -> some View {
        HStack(alignment: .top, spacing: 12) {
            ScrollView {
                VStack(alignment: .leading, spacing: 8) {
                    Button(l10n.tr("Select new entries", "选择可导入项")) {
                        flow.selected = Set(preview.entries.filter { $0.status == "new" }.map { $0.id })
                        flow.reviewedKnowledge = false
                    }.buttonStyle(.link)
                    ForEach(preview.entries) { entry in
                        HStack(alignment: .top, spacing: 6) {
                            Toggle(entry.resolved(l10n.lang).title, isOn: Binding(get: { flow.selected.contains(entry.id) }, set: { checked in
                                if checked { flow.selected.insert(entry.id) } else { flow.selected.remove(entry.id) }
                                flow.reviewedKnowledge = false
                            })).labelsHidden().toggleStyle(.checkbox).disabled(entry.status != "new" || flow.busy)
                            Button { flow.inspected = entry.id } label: {
                                VStack(alignment: .leading, spacing: 3) {
                                    Text(entry.resolved(l10n.lang).title).font(.system(size: 12, weight: .medium)).lineLimit(2).fixedSize(horizontal: false, vertical: true)
                                    Text(entry.status == "conflict" ? l10n.tr("Conflicting history · not imported", "历史冲突 · 不导入") : entry.status == "identical" ? l10n.tr("History already present", "此历史已存在") : entry.sensitive == true ? l10n.tr("Sensitive · review carefully", "敏感知识 · 请核对") : entry.topic)
                                        .font(.system(size: 10.5)).foregroundStyle(p.fg3)
                                }.frame(maxWidth: .infinity, alignment: .leading)
                            }.buttonStyle(.plain)
                        }.padding(7).background(flow.inspected == entry.id ? p.rowSelected : Color.clear)
                    }
                }
            }.frame(width: 250)
            Divider()
            ScrollView {
                if let entry = flow.entry {
                    VStack(alignment: .leading, spacing: 10) {
                        let resolved = entry.resolved(l10n.lang)
                        Text(resolved.title).font(.system(size: 14, weight: .semibold))
                        Text(entry.id + " · " + l10n.tr("\(entry.records) history records", "\(entry.records) 条历史记录")).font(.system(size: 11)).foregroundStyle(p.fg3)
                        if !resolved.tag.isEmpty { Text(resolved.tag).font(.system(size: 11)).foregroundStyle(p.fg3) }
                        MarkdownBody(markdown: resolved.body, p: p)
                    }.textSelection(.enabled).frame(maxWidth: .infinity, alignment: .leading).padding(8)
                }
            }.frame(maxWidth: .infinity)
        }.frame(maxHeight: .infinity)
        Toggle(l10n.tr("I reviewed the selected knowledge for this Mac. Keep it within HQ until I choose a new audience.", "我已核对所选知识适用于这台 Mac。导入后仅供 HQ 使用，分发范围另行决定。"), isOn: $flow.reviewedKnowledge)
            .toggleStyle(.checkbox).font(.system(size: 12)).fixedSize(horizontal: false, vertical: true)
    }

    @ViewBuilder private func textColumn(_ title: String, _ text: String, _ p: Theme.Palette) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title).font(.system(size: 12, weight: .semibold))
            ScrollView { Text(text.isEmpty ? l10n.tr("No content", "暂无内容") : text).font(.system(size: 12)).textSelection(.enabled).frame(maxWidth: .infinity, alignment: .leading) }
        }.padding(10).frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
    }

}
