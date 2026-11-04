import SwiftUI

struct AccessHistoryFilters: View {
    let query: AccessHistoryFilter
    let groups: [AccessHistoryGroup]
    let operations: [AccessHistoryOperation]
    let zh: Bool
    let change: (AccessHistoryFilter) -> Void

    private var viewSelection: Binding<String> {
        Binding(get: { query.groupBy ?? "" }, set: { mode in
            var next = query
            next.groupBy = mode.isEmpty ? nil : mode
            change(next)
        })
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Picker(zh ? "视图" : "View", selection: viewSelection) {
                Text(zh ? "日志" : "Logs").tag("")
                Text(zh ? "来源汇总" : "Sources").tag("device")
            }.pickerStyle(.segmented).frame(width: 190)
            AccessHistoryFilterControls(query: query, groups: groups, operations: operations, zh: zh, change: change)
        }
    }
}

private struct AccessHistoryFilterControls: View {
    let query: AccessHistoryFilter
    let groups: [AccessHistoryGroup]
    let operations: [AccessHistoryOperation]
    let zh: Bool
    let change: (AccessHistoryFilter) -> Void

    private var operationSelection: Binding<String> {
        Binding(get: { query.operation ?? "" }, set: { value in
            var next = query
            next.operation = value.isEmpty ? nil : value
            change(next)
        })
    }
    private var outcomeSelection: Binding<String> {
        Binding(get: { query.outcome ?? "" }, set: { value in
            var next = query
            next.outcome = value.isEmpty ? nil : value
            change(next)
        })
    }
    private var hasFilters: Bool {
        query.deviceId != nil || query.actor != nil || query.operation != nil || query.outcome != nil
    }

    var body: some View {
        HStack(spacing: 8) {
            AccessHistorySourceMenu(query: query, groups: groups, zh: zh, change: change)
            Picker(zh ? "操作" : "Operation", selection: operationSelection) {
                Text(zh ? "全部操作" : "All operations").tag("")
                ForEach(operations) { operation in
                    Text(zh ? operation.zh : operation.en).tag(operation.value)
                }
            }.labelsHidden()
            Picker(zh ? "结果" : "Outcome", selection: outcomeSelection) {
                Text(zh ? "全部结果" : "All outcomes").tag("")
                Text(zh ? "已接受" : "Accepted").tag("ok")
                Text(zh ? "已拒绝" : "Refused").tag("refused")
                Text(zh ? "失败" : "Failed").tag("failed")
            }.labelsHidden()
            if hasFilters {
                Button(zh ? "清除" : "Clear") { change(.init(groupBy: query.groupBy)) }
            }
        }.font(Theme.Font.task)
    }
}

private struct AccessHistorySourceMenu: View {
    let query: AccessHistoryFilter
    let groups: [AccessHistoryGroup]
    let zh: Bool
    let change: (AccessHistoryFilter) -> Void
    @State private var enteringID = false
    @State private var deviceID = ""

    private var sourceLabel: String {
        guard let group = groups.first(where: { $0.key == query.identity }) else {
            return query.deviceId ?? query.actor ?? (zh ? "全部来源" : "All sources")
        }
        let name = AccessHistoryPresentation.name(scope: group.scope, value: group.name, zh: zh)
        let identity = group.deviceId.map { String($0.prefix(8)) } ?? group.sourceIp ?? ""
        return name + " · " + identity
    }
    private func optionLabel(_ group: AccessHistoryGroup) -> String {
        let name = AccessHistoryPresentation.name(scope: group.scope, value: group.name, zh: zh)
        let identity = group.deviceId ?? group.sourceIp ?? (zh ? "未标识设备" : "Unidentified client")
        return name + " · " + identity
    }
    private func select(_ group: AccessHistoryGroup?) {
        var next = query
        next.deviceId = group?.deviceId
        next.actor = group?.deviceId == nil ? group?.actor : nil
        change(next)
    }

    var body: some View {
        Menu {
            Button(zh ? "全部来源" : "All sources") { select(nil) }
            ForEach(groups) { group in
                Button(optionLabel(group)) { select(group) }
            }
            Divider()
            Button(zh ? "按设备 ID 筛选…" : "Filter by device ID…") {
                deviceID = query.deviceId ?? ""
                enteringID = true
            }
        } label: {
            Text(sourceLabel).lineLimit(1)
        }.popover(isPresented: $enteringID) {
            VStack(alignment: .leading, spacing: 10) {
                Text(zh ? "完整设备 ID" : "Full device ID").font(Theme.Font.session)
                TextField(zh ? "设备 ID" : "Device ID", text: $deviceID)
                    .textFieldStyle(.roundedBorder).onSubmit { applyID() }
                Button(zh ? "筛选" : "Filter") { applyID() }
                    .disabled(deviceID.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
            }.padding(16).frame(width: 300)
        }
    }

    private func applyID() {
        let id = deviceID.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !id.isEmpty, id.utf8.count <= 128 else { return }
        var next = query
        next.deviceId = id
        next.actor = nil
        change(next)
        enteringID = false
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
