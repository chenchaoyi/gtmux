import SwiftUI

struct AccessHistoryFilters: View {
    let query: AccessHistoryFilter
    let groups: [AccessHistoryGroup]
    let operations: [AccessHistoryOperation]
    let zh: Bool
    let change: (AccessHistoryFilter) -> Void
    @State private var enteringID = false
    @State private var deviceID = ""
    private func update(_ edit: (inout AccessHistoryFilter) -> Void) { var q = query; edit(&q); change(q) }
    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Picker(zh ? "视图" : "View", selection: Binding(get: { query.groupBy ?? "" }, set: { mode in update { $0.groupBy = mode.isEmpty ? nil : mode } })) {
                Text(zh ? "日志" : "Logs").tag("")
                Text(zh ? "来源汇总" : "Sources").tag("device")
            }.pickerStyle(.segmented).frame(width: 190)
            HStack(spacing: 8) {
                Menu {
                    Button(zh ? "全部来源" : "All sources") { update { $0.deviceId = nil; $0.actor = nil } }
                    ForEach(groups) { g in
                        Button(AccessHistoryPresentation.name(scope: g.scope, value: g.name, zh: zh) + " · " + (g.deviceId ?? g.sourceIp ?? (zh ? "未标识设备" : "Unidentified client"))) {
                            update { $0.deviceId = g.deviceId; $0.actor = g.deviceId == nil ? g.actor : nil }
                        }
                    }
                    Divider()
                    Button(zh ? "按设备 ID 筛选…" : "Filter by device ID…") { deviceID = query.deviceId ?? ""; enteringID = true }
                } label: {
                    Text(groups.first(where: { $0.key == query.identity }).map { AccessHistoryPresentation.name(scope: $0.scope, value: $0.name, zh: zh) + " · " + ($0.deviceId.map { String($0.prefix(8)) } ?? $0.sourceIp ?? "") } ?? query.deviceId ?? query.actor ?? (zh ? "全部来源" : "All sources"))
                        .lineLimit(1)
                }.popover(isPresented: $enteringID) {
                    VStack(alignment: .leading, spacing: 10) {
                        Text(zh ? "完整设备 ID" : "Full device ID").font(Theme.Font.session)
                        TextField(zh ? "设备 ID" : "Device ID", text: $deviceID).textFieldStyle(.roundedBorder).onSubmit { applyID() }
                        Button(zh ? "筛选" : "Filter") { applyID() }.disabled(deviceID.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                    }.padding(16).frame(width: 300)
                }
                Picker(zh ? "操作" : "Operation", selection: Binding(get: { query.operation ?? "" }, set: { v in update { $0.operation = v.isEmpty ? nil : v } })) {
                    Text(zh ? "全部操作" : "All operations").tag("")
                    ForEach(operations) { op in Text(zh ? op.zh : op.en).tag(op.value) }
                }.labelsHidden()
                Picker(zh ? "结果" : "Outcome", selection: Binding(get: { query.outcome ?? "" }, set: { v in update { $0.outcome = v.isEmpty ? nil : v } })) {
                    Text(zh ? "全部结果" : "All outcomes").tag("")
                    Text(zh ? "已接受" : "Accepted").tag("ok")
                    Text(zh ? "已拒绝" : "Refused").tag("refused")
                    Text(zh ? "失败" : "Failed").tag("failed")
                }.labelsHidden()
                if query.deviceId != nil || query.actor != nil || query.operation != nil || query.outcome != nil {
                    Button(zh ? "清除" : "Clear") { change(.init(groupBy: query.groupBy)) }
                }
            }.font(Theme.Font.task)
        }
    }
    private func applyID() {
        let id = deviceID.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !id.isEmpty, id.utf8.count <= 128 else { return }
        update { $0.deviceId = id; $0.actor = nil }; enteringID = false
    }
}
struct AccessHistoryGroups: View {
    let groups: [AccessHistoryGroup]
    let zh: Bool
    let select: (AccessHistoryGroup) -> Void
    var body: some View {
        LazyVStack(alignment: .leading, spacing: 0) {
            ForEach(groups) { group in
                Button { select(group) } label: {
                    VStack(alignment: .leading, spacing: 5) {
                        HStack {
                            Text(AccessHistoryPresentation.name(scope: group.scope, value: group.name, zh: zh)).font(Theme.Font.session)
                            Spacer()
                            Text(zh ? "查看日志 ›" : "View log ›").font(Theme.Font.task).foregroundStyle(.secondary)
                        }
                        Text([group.deviceId ?? (zh ? "未记录设备身份" : "No paired device identity"), group.platform ?? "", group.sourceIp ?? ""].filter { !$0.isEmpty }.joined(separator: " · "))
                            .font(Theme.Font.window).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                        let at = AccessHistoryEntry(id: group.key, at: group.lastAt, actor: group.actor, name: group.name, scope: group.scope, labelEn: "", labelZh: "", outcome: "")
                        let clock = AccessHistoryPresentation.lines([at], zh: zh)[0]
                        Text((clock.day ?? "") + " " + clock.clock).font(Theme.Font.window).foregroundStyle(.secondary)
                        Text(zh ? "\(group.count) 条已记录 · \(group.refused) 次拒绝 · \(group.failed) 次失败" : "\(group.count) recorded · \(group.refused) refused · \(group.failed) failed")
                            .font(Theme.Font.task)
                    }.padding(.vertical, 10).contentShape(Rectangle())
                }.buttonStyle(.plain)
                Divider()
            }
        }
    }
}
