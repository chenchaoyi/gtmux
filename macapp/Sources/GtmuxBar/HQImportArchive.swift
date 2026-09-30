import SwiftUI

struct HQImportArchiveChoice: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var flow: HQImportFlow
    @FocusState private var passwordFocused: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            Text(flow.purpose == .restore
                 ? l10n.tr("Choose an exported HQ backup. You can review its contents before restoring.", "选择已导出的 HQ 备份，预览内容后再确认恢复。")
                 : l10n.tr("Choose a backup from your other Mac, then select the knowledge and personal requirements to bring over.", "选择另一台 Mac 导出的备份，再挑选要迁入的知识和个人要求。"))
                .foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
            if flow.path.isEmpty {
                HStack(spacing: 12) {
                    Image(systemName: "doc.zipper").font(.system(size: 30)).foregroundStyle(.secondary)
                    VStack(alignment: .leading, spacing: 4) {
                        Text(l10n.tr("HQ backup file", "HQ 备份文件")).font(.headline)
                        Text(l10n.tr("Encrypted backup or .tar.gz archive", "支持加密备份和 .tar.gz 文件"))
                            .font(.callout).foregroundStyle(.secondary)
                    }
                }.padding(.vertical, 12)
            } else {
                HStack(spacing: 12) {
                    Image(systemName: flow.archiveEncrypted ? "lock.doc" : "doc.zipper")
                        .font(.system(size: 24)).foregroundStyle(Color.accentColor)
                    VStack(alignment: .leading, spacing: 4) {
                        Text(URL(fileURLWithPath: flow.path).lastPathComponent).font(.headline).lineLimit(1).truncationMode(.middle)
                        Text(URL(fileURLWithPath: flow.path).deletingLastPathComponent().path)
                            .font(.caption).foregroundStyle(.secondary).lineLimit(1).truncationMode(.middle)
                    }.help(flow.path)
                    Spacer(minLength: 8)
                    Button(l10n.tr("Change…", "更换…")) { flow.chooseFile(l10n: l10n) }
                }.padding(12).background(.quaternary.opacity(0.5), in: RoundedRectangle(cornerRadius: 8))
                if flow.archiveEncrypted {
                    VStack(alignment: .leading, spacing: 8) {
                        Text(l10n.tr("Backup password", "备份密码")).font(.headline)
                        SecureField(l10n.tr("Enter the password set when exporting", "输入导出时设置的密码"), text: $flow.passphrase)
                            .textFieldStyle(.roundedBorder).focused($passwordFocused)
                            .accessibilityIdentifier("hq-import-password")
                        Text(l10n.tr("Used only to open this backup. It is not saved.", "仅用于打开这份备份，不会保存。"))
                            .font(.caption).foregroundStyle(.secondary)
                    }
                }
            }
        }
        .onChange(of: flow.path) { _, _ in passwordFocused = flow.archiveEncrypted }
        .onAppear { passwordFocused = flow.archiveEncrypted }
    }
}

struct HQImportPreviewContent: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var flow: HQImportFlow

    var body: some View {
        if let preview = flow.preview {
            VStack(alignment: .leading, spacing: 16) {
                Label(URL(fileURLWithPath: flow.path).lastPathComponent, systemImage: "doc.zipper")
                    .font(.headline).lineLimit(1).truncationMode(.middle).help(flow.path)
                Text(l10n.tr("\(preview.entries.count) non-sensitive entries · \(preview.sensitive_count) sensitive entries · \(preview.tools) attachments", "\(preview.entries.count) 条普通知识 · \(preview.sensitive_count) 条敏感知识 · \(preview.tools) 份附件"))
                    .font(.callout).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                Divider()
                if flow.purpose == .restore {
                    Label(l10n.tr("Restore all HQ records", "恢复全部 HQ 内容"), systemImage: "arrow.counterclockwise")
                        .font(.headline)
                    Text(l10n.tr("Knowledge, personal requirements and the situation board will be replaced by this backup. Your current records will be saved separately.", "知识库、个人要求和态势板将恢复到备份时的状态。当前内容会另存为备份。"))
                        .foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                    Text(l10n.tr("To bring only selected content to a new Mac, cancel and choose Move from another Mac.", "如果只想迁入部分内容，请取消并选择“从另一台 Mac 迁移”。"))
                        .font(.callout).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                    Toggle(l10n.tr("I understand and want to restore this backup.", "我已了解，恢复这份备份。"), isOn: $flow.confirmRestore)
                } else {
                    Text(l10n.tr("Choose what to bring over", "选择迁移内容")).font(.headline)
                    option(l10n.tr("Knowledge", "知识库"), detail: l10n.tr("Current entries with their revision history and sources", "保留条目、修订历史及来源"), value: $flow.knowledge)
                        .disabled(preview.entries.isEmpty && preview.sensitive_count == 0)
                    if preview.sensitive_count > 0 {
                        Toggle(l10n.tr("Include sensitive knowledge for review", "同时核对敏感知识"), isOn: $flow.sensitive)
                            .disabled(!flow.knowledge).padding(.leading, 20)
                    }
                    option(l10n.tr("Personal requirements", "个人要求"), detail: l10n.tr("Compare both versions before choosing a replacement", "对照新旧内容，核对后再决定是否替换"), value: $flow.personal).disabled(!preview.has_local)
                    option(l10n.tr("Tool attachments", "工具附件"), detail: l10n.tr("Save for review; no installation or execution", "仅保存附件，不会安装或运行"), value: $flow.tools).disabled(preview.tools == 0)
                    Text(l10n.tr("The old situation board, sessions and connection credentials are excluded.", "不迁移旧态势板、会话和连接授权。"))
                        .font(.callout).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                }
                if !preview.encrypted {
                    Toggle(l10n.tr("I understand this backup is not encrypted.", "我已了解此备份未加密。"), isOn: $flow.allowPlain)
                }
                Label(flow.purpose == .restore
                      ? l10n.tr("Stop HQ before restoring. Previewing does not change your records.", "恢复前请先退出 HQ。预览不会修改当前内容。")
                      : l10n.tr("Next, review the content. Stop HQ only when you are ready to apply it.", "下一步核对具体内容；确认导入前需先退出 HQ。"), systemImage: "info.circle")
                    .font(.callout).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
            }.toggleStyle(.checkbox)
        }
    }

    private func option(_ title: String, detail: String, value: Binding<Bool>) -> some View {
        Toggle(isOn: value) {
            VStack(alignment: .leading, spacing: 3) {
                Text(title).font(.body.weight(.medium))
                Text(detail).font(.callout).foregroundStyle(.secondary)
            }.fixedSize(horizontal: false, vertical: true)
        }
    }
}
