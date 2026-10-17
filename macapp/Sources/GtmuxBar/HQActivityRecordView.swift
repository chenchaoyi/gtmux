import SwiftUI

struct HQActivityRecordView: View {
    let l10n: L10n
    let item: HQActivityItem
    let close: () -> Void
    @Environment(\.hqWorkspaceSheetSize) private var size
    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack {
                Text(hqActivityTitle(item.kind, l10n)).font(HQTypography.section)
                Spacer()
                Button(l10n.tr("Done", "完成"), action: close).keyboardShortcut(.cancelAction)
            }
            Divider()
            ScrollView {
                VStack(alignment: .leading, spacing: 10) {
                    Text(HQActivityTimeline.dateLabel(item.at, zh: l10n.lang == "zh") + " " + HQActivityTimeline.clock(item.at, seconds: true))
                    if let context = item.context { Text(context).font(HQTypography.rowTitle) }
                    if let target = item.target { Text(l10n.tr("Target: \(target)", "目标：\(target)")) }
                    if let text = item.detail(l10n) { Text(text) }
                    if let outcome = item.outcome { Text(hqActivityOutcome(outcome, l10n)) }
                    Divider()
                    Text(l10n.tr("Source", "记录来源")).font(HQTypography.navigation)
                    Text(item.source == .workspace ? l10n.tr("Workspace actions", "工作台操作") : l10n.tr("HQ journal", "HQ 事件日志"))
                    metadata(l10n.tr("Action", "动作"), item.kind)
                    if let id = item.contextID { metadata(l10n.tr("Matter ID", "事项 ID"), id) }
                    if let pane = item.pane { metadata(l10n.tr("Pane", "窗格"), pane) }
                    if let session = item.sessionID { metadata(l10n.tr("Session ID", "会话 ID"), session) }
                    metadata(l10n.tr("Record ID", "记录 ID"), item.id)
                }.font(HQTypography.body).textSelection(.enabled).frame(maxWidth: .infinity, alignment: .leading)
            }
        }.padding(18).frame(width: size.width, height: size.height)
    }
    private func metadata(_ title: String, _ value: String) -> some View {
        VStack(alignment: .leading, spacing: 3) {
            Text(title).font(HQTypography.caption).foregroundStyle(.secondary)
            Text(value).font(HQTypography.code)
        }
    }
}
