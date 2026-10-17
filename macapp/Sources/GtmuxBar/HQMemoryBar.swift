import SwiftUI

struct HQMemoryBar: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQResourceStore
    @State private var exportFlow: HQExportFlow?
    @State private var importFlow: HQImportFlow?
    @Environment(\.colorScheme) private var scheme
    private var mem: HQMemoryState { store.memory }
    var body: some View {
        memoryBar(Theme.Palette.of(scheme))
            .sheet(isPresented: Binding(get: { exportFlow != nil }, set: { if !$0 { exportFlow = nil } })) {
                if let flow = exportFlow {
                    HQExportSheet(l10n: l10n, flow: flow) {
                        exportFlow = nil
                        store.refresh()
                    }
                }
            }
            .sheet(isPresented: Binding(get: { importFlow != nil }, set: { if !$0 { importFlow = nil } })) {
                if let flow = importFlow {
                    HQImportSheet(l10n: l10n, flow: flow) {
                        importFlow = nil
                        store.refresh()
                    }
                }
            }
    }

    private var archiveHelp: String {
        if !mem.exists, store.failedPages.contains(.knowledge) {
            return l10n.tr("Archive information could not be read. Refresh to try again.", "暂时无法读取档案信息，请刷新重试。")
        }
        guard mem.exists else { return l10n.tr("No HQ records on this Mac", "此 Mac 尚无 HQ 档案") }
        var lines = [
            "\(mem.sizeText) · \(mem.snapshots) " + l10n.tr("local snapshots", "份本地快照"), l10n.tr("Stored on this computer. Export a copy to keep it elsewhere.", "档案保存在此电脑，可导出副本另行保存。"),
        ]
        if let backup = hqArchiveBackupHint(mem.offMachine, l10n) { lines.append(backup) }
        if mem.lastExportAt > 0 {
            let age = relativeTime(Int(mem.lastExportAt), now: Int(Date().timeIntervalSince1970))
            lines.append(
                l10n.tr("Last export: \(age) ago", "最近导出：\(age)前") + " · "
                    + (mem.lastExportEncrypted ? l10n.tr("Encrypted", "已加密") : l10n.tr("Unencrypted", "未加密")))
        }
        return lines.filter { !$0.isEmpty }.joined(separator: "\n")
    }

    @ViewBuilder private func memoryBar(_ p: Theme.Palette) -> some View {
        HStack(spacing: 8) {
            HQWorkspaceInfo(
                title: l10n.tr("HQ archives", "HQ 档案"), text: archiveHelp,
                symbol: "archivebox", done: l10n.tr("Done", "完成"))
                .foregroundStyle(p.fg2)
            Spacer(minLength: 8)
            Button {
                exportFlow = HQExportFlow()
            } label: {
                Text(l10n.tr("Export…", "导出…"))
            }
            .buttonStyle(.bordered)
            .disabled(!mem.exists)
            Menu(l10n.tr("Import…", "导入…")) {
                Button(l10n.tr("Restore HQ backup…", "恢复 HQ 备份…")) {
                    importFlow = HQImportFlow(purpose: .restore)
                }
                Button(l10n.tr("Move from another computer…", "从另一台电脑迁移…")) {
                    importFlow = HQImportFlow(purpose: .migrate)
                }
                Divider()
                Button(l10n.tr("Continue staged migration…", "继续核对暂存内容…")) {
                    importFlow = HQImportFlow(purpose: .resume)
                }
            }
        }
        .font(HQTypography.navigation).controlSize(.regular)
        .padding(.horizontal, 12)
        .padding(.vertical, 7)
    }

}

/// Older CLI versions expose this observation as localized prose. Normalize only
/// known wire phrases; preserve future observations instead of guessing backup state.
func hqArchiveBackupHint(_ raw: String, _ l10n: L10n) -> String? {
    switch raw {
    case "": return nil
    case "Time Machine is configured", "已配置 Time Machine":
        return l10n.tr("Time Machine is configured", "已配置 Time Machine")
    case "a synced folder exists; put an export there", "有同步盘，导一份过去":
        return l10n.tr("A sync folder is available for an exported copy", "可将导出副本保存到同步文件夹")
    case "nothing carries it off this disk", "没有任何东西把它带离这块盘":
        return l10n.tr("No backup destination detected", "未检测到备份位置")
    default: return raw
    }
}
