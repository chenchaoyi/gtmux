import AppKit
import SwiftUI

/// A compact assistant until the user needs the full list/detail review workspace.
struct HQImportSheet: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var flow: HQImportFlow
    var onClose: () -> Void

    private var reviewing: Bool { flow.step == .staged }
    private var title: String {
        flow.purpose == .restore ? l10n.tr("Restore HQ backup", "恢复 HQ 备份") : l10n.tr("Move HQ to this Mac", "迁移到这台 Mac")
    }
    private var stepTitle: String {
        switch flow.step {
        case .choose: return l10n.tr("1 · Choose a backup", "1 · 选择备份")
        case .preview: return flow.purpose == .restore ? l10n.tr("2 · Confirm restore", "2 · 确认恢复") : l10n.tr("2 · Choose content", "2 · 选择内容")
        case .staged: return l10n.tr("3 · Review and import", "3 · 核对并导入")
        case .stages: return l10n.tr("Continue a migration", "继续迁移")
        case .done: return l10n.tr("Restore complete", "恢复完成")
        }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 12) {
                Image(systemName: flow.purpose == .restore ? "clock.arrow.circlepath" : "arrow.down.doc")
                    .font(.system(size: 24)).foregroundStyle(Color.accentColor)
                    .frame(width: 36)
                VStack(alignment: .leading, spacing: 4) {
                    Text(title).font(.title3.weight(.semibold))
                    Text(stepTitle).font(.callout).foregroundStyle(.secondary)
                }
                Spacer()
            }.padding(24)
            Divider()
            content.padding(24)
            if flow.busy {
                HStack(spacing: 8) { ProgressView().controlSize(.small); Text(flow.progress).font(.callout) }
                    .padding(.horizontal, 24).padding(.bottom, 16)
            }
            if let error = flow.error, !error.isEmpty {
                Label { ScrollView { Text(error).textSelection(.enabled).frame(maxWidth: .infinity, alignment: .leading) }.frame(maxHeight: 70) }
                    icon: { Image(systemName: "exclamationmark.triangle") }
                    .font(.callout).foregroundStyle(.red).padding(.horizontal, 24).padding(.bottom, 16)
            }
            if !flow.result.isEmpty, flow.step != .done {
                Text(flow.result).font(.callout).foregroundStyle(.secondary).textSelection(.enabled)
                    .lineLimit(3).padding(.horizontal, 24).padding(.bottom, 16)
            }
            Divider()
            footer.padding(.horizontal, 24).padding(.vertical, 16)
        }
        .frame(width: reviewing ? 820 : 560)
        .fixedSize(horizontal: false, vertical: true)
        .disabled(flow.busy)
        .interactiveDismissDisabled(flow.busy)
        .onAppear { if flow.purpose == .resume { flow.loadStages(l10n: l10n) } }
    }

    @ViewBuilder private var content: some View {
        switch flow.step {
        case .choose:
            HQImportArchiveChoice(l10n: l10n, flow: flow)
        case .preview:
            HQImportPreviewContent(l10n: l10n, flow: flow)
        case .staged:
            VStack(alignment: .leading, spacing: 12) {
                HQImportReview(l10n: l10n, flow: flow).frame(height: 390)
                Text(l10n.tr("Stop HQ before importing. You can close this window and resume the review later.", "导入前请先退出 HQ。所选内容已暂存，可稍后继续核对。"))
                    .font(.callout).foregroundStyle(.secondary)
            }
        case .stages:
            stages
        case .done:
            VStack(alignment: .leading, spacing: 16) {
                Label(l10n.tr("HQ backup restored", "HQ 备份已恢复"), systemImage: "checkmark.circle.fill")
                    .font(.headline).foregroundStyle(.green)
                ScrollView { Text(flow.result).font(.callout).textSelection(.enabled).frame(maxWidth: .infinity, alignment: .leading) }
                    .frame(height: 140)
            }
        }
    }

    private var footer: some View {
        HStack(spacing: 10) {
            if flow.step == .preview {
                Button(l10n.tr("Back", "上一步")) { flow.changeArchive() }
            } else if reviewing && flow.reviewTab == 0 {
                Text(l10n.tr("\(flow.selected.count) selected", "已选 \(flow.selected.count) 条"))
                    .font(.callout).foregroundStyle(.secondary)
            }
            Spacer()
            Button(flow.step == .choose || flow.step == .preview ? l10n.tr("Cancel", "取消") : l10n.tr("Close", "关闭"), action: onClose)
                .keyboardShortcut(.cancelAction)
            primaryAction.buttonStyle(.borderedProminent)
        }.controlSize(.large)
    }

    @ViewBuilder private var primaryAction: some View {
        switch flow.step {
        case .choose:
            if flow.path.isEmpty {
                Button(l10n.tr("Choose backup…", "选择备份…")) { flow.chooseFile(l10n: l10n) }
                    .keyboardShortcut(.defaultAction)
            } else {
                Button(l10n.tr("Preview backup", "预览备份")) { flow.inspect(l10n: l10n) }
                    .keyboardShortcut(.defaultAction).disabled(!flow.previewReady)
            }
        case .preview:
            if flow.purpose == .restore {
                Button(l10n.tr("Restore backup", "恢复备份")) { flow.restore(l10n: l10n) }
                    .keyboardShortcut(.defaultAction)
                    .disabled(!flow.confirmRestore || !(flow.preview?.encrypted == true || flow.allowPlain))
            } else {
                Button(l10n.tr("Review selected content", "核对所选内容")) { flow.stage(l10n: l10n) }
                    .keyboardShortcut(.defaultAction).disabled(!flow.stageReady)
            }
        case .staged:
            if flow.reviewTab == 0 {
                Button(l10n.tr("Import knowledge", "导入知识")) { flow.applyKnowledge(l10n: l10n) }
                    .keyboardShortcut(.defaultAction).disabled(!flow.knowledgeReady)
            } else if flow.reviewTab == 1 {
                Button(l10n.tr("Replace requirements", "替换个人要求")) { flow.applyPersonal(l10n: l10n) }
                    .keyboardShortcut(.defaultAction).disabled(!flow.personalReady)
            }
        case .stages, .done: EmptyView()
        }
    }

    private var stages: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(l10n.tr("Your previous selections are saved. Choose a migration to continue reviewing.", "之前选择的内容已暂存，可继续核对后导入。"))
                .foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
            if flow.stages.isEmpty && !flow.busy {
                Label(l10n.tr("No migrations to continue", "暂无进行中的迁移"), systemImage: "tray")
                    .foregroundStyle(.secondary).padding(.vertical, 20)
            } else {
                ScrollView {
                    VStack(spacing: 12) {
                        ForEach(flow.stages) { stage in
                            HStack {
                                VStack(alignment: .leading, spacing: 4) {
                                    Text(Date(timeIntervalSince1970: TimeInterval(stage.created_at)), style: .date)
                                    Text(stage.id).font(.caption.monospaced()).foregroundStyle(.secondary)
                                }
                                Spacer()
                                Button(l10n.tr("Review", "继续核对")) { flow.manifest = stage; flow.loadReview(l10n: l10n) }
                            }
                            Divider()
                        }
                    }
                }.frame(height: 220)
            }
        }
    }
}
