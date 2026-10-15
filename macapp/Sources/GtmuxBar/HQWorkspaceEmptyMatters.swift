import SwiftUI

/// First-run/upgrade guidance. All writes still require the normal composer Send.
struct HQWorkspaceEmptyMatters: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQWorkspaceStore
    @State private var occupiedDraft = false

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(l10n.tr("No matters yet", "尚无事项")).font(Theme.Font.session)
            Text(l10n.tr(
                "Existing work needs HQ review before it appears as matters.",
                "已有工作需经 HQ 核对后整理为事项。"))
                .font(Theme.Font.task).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
            Button(l10n.tr("Review existing work…", "整理已有工作…")) {
                occupiedDraft = !store.prepareExistingWorkReview(
                    body: l10n.tr(
                        "Review the current situation board, task ledger and verified conversations. Register only goals that still need follow-up in the workspace, reusing existing matter IDs. Use register to record the source and current assessment without redispatching or executing the original task. Keep uncertain membership explicit; do not guess from titles or pane numbers. Report what was registered and what remains uncertain.",
                        "请核对当前态势板、任务台账和已确认的对话，将仍需跟进的目标整理为工作台事项，复用已有事项 ID。用 register 记录来源和当前进展，不重新派发或执行原任务。不确定的归属明确说明，不按标题或窗格编号猜测。完成后说明已登记和仍待核实的内容。"))
            }.disabled(store.acting)
            Button(l10n.tr("Open situation board", "查看态势板")) {
                HQReaderController.shared.show(l10n: l10n, tab: .board)
            }
        }.padding(8)
            .alert(l10n.tr("Unsent message", "有未发送的内容"), isPresented: $occupiedDraft) {
                Button(l10n.tr("OK", "好"), role: .cancel) {}
            } message: {
                Text(l10n.tr(
                    "Your draft is preserved. Send or clear it before preparing the review request.",
                    "已保留原有内容。请先发送或清空，再整理已有工作。"))
            }
    }
}
