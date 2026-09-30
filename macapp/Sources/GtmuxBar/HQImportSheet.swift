import AppKit
import SwiftUI

struct HQImportSheet: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var flow: HQImportFlow
    var onClose: () -> Void
    @Environment(\.colorScheme) private var scheme
    @FocusState private var passFocused: Bool

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(alignment: .leading, spacing: 14) {
            Text(flow.purpose == .restore ? l10n.tr("Restore HQ backup", "恢复 HQ 备份") : l10n.tr("Move HQ to this Mac", "从另一台 Mac 迁移"))
                .font(.system(size: 17, weight: .semibold))
            Divider()
            switch flow.step {
            case .choose: choose(p)
            case .preview: preview(p)
            case .staged: review(p)
            case .stages: stages(p)
            case .done:
                ScrollView { Text(flow.result).textSelection(.enabled).frame(maxWidth: .infinity, alignment: .leading) }
            }
            Spacer(minLength: 0)
            if flow.busy {
                HStack { ProgressView().controlSize(.small); Text(flow.progress) }.font(.system(size: 12))
            }
            if let error = flow.error, !error.isEmpty {
                ScrollView { Text(error).font(.system(size: 12)).foregroundStyle(Theme.Status.waiting).textSelection(.enabled).frame(maxWidth: .infinity, alignment: .leading) }
                    .frame(maxHeight: 70)
            }
            if !flow.result.isEmpty, flow.step != .done {
                Text(flow.result).font(.system(size: 11)).foregroundStyle(p.fg2).textSelection(.enabled).lineLimit(3)
            }
            Divider()
            HStack {
                Text(l10n.tr("Preview and staging do not change HQ. Stop HQ before applying.", "预览和暂存不会修改 HQ。应用前请先退出 HQ。"))
                    .font(.system(size: 11)).foregroundStyle(p.fg3)
                Spacer()
                Button(l10n.tr("Close", "关闭"), action: onClose).keyboardShortcut(.cancelAction).disabled(flow.busy)
            }
        }
        .padding(20)
        .frame(width: 800, height: 640)
        .background(p.bg)
        .disabled(flow.busy)
        .interactiveDismissDisabled(flow.busy)
        .onAppear { if flow.purpose == .resume { flow.loadStages(l10n: l10n) } }
    }

    @ViewBuilder private func choose(_ p: Theme.Palette) -> some View {
        Text(flow.purpose == .restore
             ? l10n.tr("Restore the complete archive, including its situation board. Existing records will be kept as a separate backup.", "恢复整份档案，包括态势板。当前档案会保留为独立备份。")
             : l10n.tr("Bring knowledge and personal requirements to this Mac. The old situation board, sessions, built-in rules and connection credentials stay behind.", "迁入知识库和个人要求。旧态势板、会话、内置规则及连接授权不迁移。"))
            .foregroundStyle(p.fg2).fixedSize(horizontal: false, vertical: true)
        HStack {
            Text(flow.path.isEmpty ? l10n.tr("No archive selected", "尚未选择档案") : flow.path)
                .font(.system(size: 12)).lineLimit(2).textSelection(.enabled)
            Spacer()
            Button(l10n.tr("Choose archive…", "选择档案…")) { flow.chooseFile(l10n: l10n); passFocused = true }
        }.padding(.vertical, 8)
        VStack(alignment: .leading, spacing: 5) {
            Text(l10n.tr("Passphrase (for encrypted archives)", "口令（加密档案需要）")).font(.system(size: 12, weight: .medium))
            SecureField(l10n.tr("Archive passphrase", "档案口令"), text: $flow.passphrase).focused($passFocused)
                .textFieldStyle(.roundedBorder).frame(maxWidth: 380)
            Text(l10n.tr("Unencrypted .tar.gz copies do not need a passphrase.", "未加密的 .tar.gz 副本无需口令。"))
                .font(.system(size: 11)).foregroundStyle(p.fg3)
        }
        HStack { Spacer(); Button(l10n.tr("Preview", "预览")) { flow.inspect(l10n: l10n) }.keyboardShortcut(.defaultAction).disabled(flow.path.isEmpty || flow.busy) }
    }

    @ViewBuilder private func preview(_ p: Theme.Palette) -> some View {
        if let preview = flow.preview {
            HStack { Text(l10n.tr("Archive contents", "档案内容")).font(.system(size: 13, weight: .semibold)); Spacer(); Button(l10n.tr("Change archive…", "更换档案…")) { flow.changeArchive() } }
            Text(l10n.tr("\(preview.entries.count) regular knowledge entries · \(preview.sensitive_count) sensitive · \(preview.tools) tool attachments", "\(preview.entries.count) 条非敏感知识 · \(preview.sensitive_count) 条敏感知识 · \(preview.tools) 份工具附件"))
                .font(.system(size: 12)).foregroundStyle(p.fg2)
            if !preview.encrypted {
                Toggle(l10n.tr("I understand this archive is not encrypted.", "我已了解此档案未加密。"), isOn: $flow.allowPlain).toggleStyle(.checkbox)
            }
            if flow.purpose == .restore {
                Text(l10n.tr("This restores everything in the archive. To choose long-term content for a new Mac, use Move from another Mac instead.", "此操作会恢复档案中的全部内容。若要为新 Mac 选择长期内容，请使用“从另一台 Mac 迁移”。"))
                    .foregroundStyle(p.fg2).fixedSize(horizontal: false, vertical: true)
                Toggle(l10n.tr("Restore this complete backup and keep the current records separately.", "恢复这份完整备份，并另存当前档案。"), isOn: $flow.confirmRestore).toggleStyle(.checkbox)
                HStack { Spacer(); Button(l10n.tr("Restore backup", "恢复备份")) { flow.restore(l10n: l10n) }.disabled(flow.busy || !flow.confirmRestore || !preview.encrypted && !flow.allowPlain) }
            } else {
                Text(l10n.tr("Choose what to stage for review", "选择要暂存并核对的内容")).font(.system(size: 13, weight: .semibold))
                Toggle(l10n.tr("Knowledge — current entries and their revision history", "知识库：当前条目及其修订历史"), isOn: $flow.knowledge)
                    .disabled(preview.entries.isEmpty && preview.sensitive_count == 0)
                if preview.sensitive_count > 0 {
                    Toggle(l10n.tr("Include sensitive knowledge for separate review", "包含敏感知识，另行核对"), isOn: $flow.sensitive).disabled(!flow.knowledge).padding(.leading, 20)
                }
                Toggle(l10n.tr("Personal requirements — review LOCAL.md before replacing", "个人要求：先核对 LOCAL.md，再决定是否替换"), isOn: $flow.personal).disabled(!preview.has_local)
                Toggle(l10n.tr("Tool attachments — stage only, never install or execute", "工具附件：仅暂存，不安装或执行"), isOn: $flow.tools).disabled(preview.tools == 0)
                Text(l10n.tr("The destination stays unchanged until you review and apply the selected content.", "核对并应用之前，当前 HQ 内容保持不变。"))
                    .font(.system(size: 12)).foregroundStyle(p.fg2)
                HStack { Spacer(); Button(l10n.tr("Stage and review", "暂存并核对")) { flow.stage(l10n: l10n) }.disabled(!flow.stageReady) }
            }
        }
    }

    @ViewBuilder private func review(_ p: Theme.Palette) -> some View {
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
                    textColumn(l10n.tr("From the archive", "档案中的个人要求"), preview.local ?? "", p)
                }.frame(maxHeight: .infinity)
                Toggle(l10n.tr("I reviewed both texts. Replace the current requirements with the archive copy.", "我已核对两份内容，使用档案中的个人要求替换当前内容。"), isOn: $flow.replacePersonal).toggleStyle(.checkbox)
                HStack { Text(l10n.tr("Current requirements are kept unless you choose replacement.", "默认保留当前个人要求。" )).font(.system(size: 11)).foregroundStyle(p.fg3); Spacer(); Button(l10n.tr("Replace requirements", "替换个人要求")) { flow.applyPersonal(l10n: l10n) }.disabled(!flow.personalReady) }
            } else {
                Text(l10n.tr("Attachments are staged without executable permissions. Check paths and dependencies before using them on this Mac.", "附件已暂存并移除执行权限。请先核对路径和依赖，再决定如何在这台 Mac 上使用。"))
                    .foregroundStyle(p.fg2).fixedSize(horizontal: false, vertical: true)
                if let path = manifest.path {
                    Button(l10n.tr("Show staged attachments", "查看暂存附件")) { NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: path).appendingPathComponent("knowledge/tools")]) }
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
                            Toggle("", isOn: Binding(get: { flow.selected.contains(entry.id) }, set: { checked in
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
        HStack { Text(l10n.tr("\(flow.selected.count) selected", "已选 \(flow.selected.count) 条")).font(.system(size: 11)).foregroundStyle(p.fg3); Spacer(); Button(l10n.tr("Import selected knowledge", "导入所选知识")) { flow.applyKnowledge(l10n: l10n) }.disabled(!flow.knowledgeReady) }
    }

    @ViewBuilder private func textColumn(_ title: String, _ text: String, _ p: Theme.Palette) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title).font(.system(size: 12, weight: .semibold))
            ScrollView { Text(text.isEmpty ? l10n.tr("No content", "暂无内容") : text).font(.system(size: 12)).textSelection(.enabled).frame(maxWidth: .infinity, alignment: .leading) }
        }.padding(10).frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
    }

    @ViewBuilder private func stages(_ p: Theme.Palette) -> some View {
        Text(l10n.tr("Continue reviewing staged content", "继续核对暂存内容")).font(.system(size: 13, weight: .semibold))
        if flow.stages.isEmpty && !flow.busy { Text(l10n.tr("No staged migrations", "暂无暂存的迁移内容")).foregroundStyle(p.fg3) }
        ScrollView {
            VStack(alignment: .leading, spacing: 10) {
                ForEach(flow.stages) { stage in
                    HStack {
                        VStack(alignment: .leading, spacing: 3) {
                            Text(Date(timeIntervalSince1970: TimeInterval(stage.created_at)), style: .date)
                            Text(stage.id).font(.system(size: 10, design: .monospaced)).foregroundStyle(p.fg3)
                        }
                        Spacer()
                        Button(l10n.tr("Review", "核对")) { flow.manifest = stage; flow.loadReview(l10n: l10n) }.disabled(flow.busy)
                    }
                }
            }
        }
    }
}
