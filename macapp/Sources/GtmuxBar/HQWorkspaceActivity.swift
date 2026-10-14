import SwiftUI

struct HQWorkspaceActivityView: View {
    let l10n: L10n
    let detail: HQWorkspaceDetail?
    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(l10n.tr("HQ activity", "HQ 运行记录")).font(Theme.Font.title).padding([.horizontal, .top], 16)
            Text(
                l10n.tr(
                    "Instructions, supervision, dispatch and maintenance. Up to 200 recent records per source; event journal covers seven days.",
                    "查看指令、观察、派发与维护记录。每个来源显示最近 200 条，事件范围为近七天。")
            ).font(Theme.Font.window).foregroundStyle(.secondary).padding(.horizontal, 16)
            List {
                Section(l10n.tr("Workspace actions", "工作台操作")) {
                    ForEach((detail?.changes ?? []).reversed()) { c in
                        HStack(alignment: .top, spacing: 10) {
                            Image(systemName: "clock").frame(width: 20).foregroundStyle(.secondary)
                            VStack(alignment: .leading, spacing: 4) {
                                Text(hqActivityTitle(c.kind, l10n)).font(Theme.Font.session)
                                if let id = c.matter, let name = detail?.matterTitles?[id] {
                                    Text(name).font(Theme.Font.task).foregroundStyle(.secondary)
                                }
                                if let id = c.target, let name = detail?.matterTitles?[id] {
                                    Text(l10n.tr("Target: \(name)", "目标：\(name)")).font(Theme.Font.window)
                                        .foregroundStyle(.secondary)
                                }
                                if let detail = c.detail, !detail.isEmpty {
                                    Text(detail).font(Theme.Font.window).foregroundStyle(.secondary)
                                        .textSelection(.enabled)
                                }
                            }
                            Spacer()
                            Text(Date(timeIntervalSince1970: TimeInterval(c.at)), style: .time).font(
                                Theme.Font.window
                            ).foregroundStyle(.secondary)
                        }.padding(.vertical, 5)
                    }
                }
                Section(l10n.tr("HQ event journal", "HQ 事件记录")) {
                    ForEach((detail?.events ?? []).reversed()) { e in
                        DisclosureGroup {
                            VStack(alignment: .leading, spacing: 6) {
                                if let summary = e.summary { Text(summary).textSelection(.enabled) }
                                Text(
                                    [e.event, e.pane, e.agentSession, e.outcome].compactMap { $0 }.joined(
                                        separator: " · ")
                                ).font(.system(size: 11, design: .monospaced)).textSelection(.enabled)
                            }.font(Theme.Font.task).padding(.vertical, 5)
                        } label: {
                            HStack {
                                Text(hqActivityTitle(e.event, l10n)).font(Theme.Font.session)
                                Spacer()
                                Text(Date(timeIntervalSince1970: TimeInterval(e.ts)), style: .time).font(
                                    Theme.Font.window
                                ).foregroundStyle(.secondary)
                            }
                        }.padding(.vertical, 4)
                    }
                }
            }.listStyle(.inset)
        }
    }
}
func hqActivityTitle(_ kind: String, _ l: L10n) -> String {
    switch kind {
    case "create": return l.tr("Matter created", "创建事项")
    case "instruction": return l.tr("Instruction saved", "指令已保存")
    case "update": return l.tr("Matter updated", "更新事项")
    case "associate": return l.tr("Conversation linked", "关联会话")
    case "move", "unlink": return l.tr("Conversation membership changed", "调整会话归属")
    case "interval": return l.tr("Included interval changed", "调整关联时段")
    case "merge": return l.tr("Matters merged", "合并事项")
    case "split": return l.tr("Matter split", "拆分事项")
    case "cancel": return l.tr("Instruction withdrawn", "撤回指令")
    case "retry": return l.tr("Explicit delivery retry", "确认再次发送")
    case "delivery-held": return l.tr("Delivery held", "暂缓送达")
    case "delivery-attempt": return l.tr("Confirming delivery", "正在确认送达")
    case "delivery-delivered": return l.tr("Instruction delivered", "指令已送达")
    case "delivery-accepted-queue": return l.tr("Instruction accepted in queue", "指令进入队列")
    case "delivery-uncertain": return l.tr("Delivery unconfirmed", "送达未确认")
    case "hq-report", "hq-progress", "hq-result", "hq-decision", "hq-error": return l.tr("HQ update", "HQ 汇报")
    case "UserPromptSubmit": return l.tr("HQ received input", "HQ 收到输入")
    case "Stop": return l.tr("HQ turn ended", "HQ 本轮结束")
    case "StopFailure": return l.tr("HQ turn failed", "HQ 本轮失败")
    case "gtmux:audit:wake-delivered": return l.tr("Supervision signal delivered", "观察信号已送达")
    case "gtmux:audit:wake-dropped": return l.tr("Supervision signal not delivered", "观察信号未送达")
    case "gtmux:audit:send": return l.tr("Agent instruction receipt", "执行指令回执")
    case "gtmux:audit:rotate", "gtmux:audit:rotate-requested": return l.tr("HQ rotation requested", "HQ 请求轮换")
    case "gtmux:audit:rotate-failed": return l.tr("HQ rotation failed", "HQ 轮换失败")
    case "gtmux:audit:hq-session": return l.tr("HQ conversation changed", "HQ 会话切换")
    case "gtmux:audit:knowledge": return l.tr("Knowledge ledger changed", "知识台账更新")
    case "gtmux:audit:reap": return l.tr("Dispatched session reclaimed", "回收执行会话")
    default: return kind
    }
}
