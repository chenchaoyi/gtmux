import SwiftUI

/// First-run/upgrade guidance. All writes still require the normal composer Send.
struct HQWorkspaceEmptyMatters: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQWorkspaceStore
    @State private var occupiedDraft = false

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(l10n.tr("No matters yet", "尚无事项")).font(HQTypography.rowTitle)
            Text(l10n.tr(
                "Existing work needs HQ review before it appears as matters.",
                "已有工作需经 HQ 核对后整理为事项。"))
                .font(HQTypography.body).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
            Button(l10n.tr("Review existing work…", "整理已有工作…")) {
                occupiedDraft = !store.prepareExistingWorkReview(
                    body: l10n.tr(
                        "Review current work and organize goals that still need follow-up as matters. Include their sources, progress and unresolved questions. Do not execute existing tasks again; flag anything whose ownership is unclear.",
                        "请核对当前工作，将仍需跟进的目标整理为事项，注明来源、进展和待解决的问题。已有任务不要重新执行；归属不明确的先说明。"))
            }.disabled(store.acting)
            Button(l10n.tr("View handoff snapshot", "查看交接快照")) {
                store.select(.resource(.board))
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
