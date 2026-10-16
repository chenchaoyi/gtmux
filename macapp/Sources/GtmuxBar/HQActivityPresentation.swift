import Foundation

/// Presentation only: the canonical action kind and both source journals stay intact.
enum HQActivityCategory: String, CaseIterable, Identifiable {
    case instructions, updates, organization, supervision, knowledge, maintenance, other
    var id: Self { self }
    func title(_ l: L10n) -> String {
        switch self {
        case .instructions: return l.tr("Instructions & delivery", "指令送达")
        case .updates: return l.tr("Progress & decisions", "进展汇报")
        case .organization: return l.tr("Matters & conversations", "事项与会话")
        case .supervision: return l.tr("Supervision", "观察处理")
        case .knowledge: return l.tr("Knowledge", "知识整理")
        case .maintenance: return l.tr("Maintenance & rotation", "维护轮换")
        case .other: return l.tr("Other activity", "其他动作")
        }
    }
    var symbol: String {
        switch self {
        case .instructions: return "paperplane"
        case .updates: return "text.bubble"
        case .organization: return "square.stack.3d.up"
        case .supervision: return "eye"
        case .knowledge: return "book"
        case .maintenance: return "arrow.triangle.2.circlepath"
        case .other: return "info.circle"
        }
    }
}

struct HQActivityPresentation {
    let kind: String
    var outcome: String? = nil
    var category: HQActivityCategory {
        switch kind {
        case "instruction", "cancel", "retry", "delivery-held", "delivery-attempt", "delivery-delivered",
            "delivery-accepted-queue", "delivery-uncertain", "UserPromptSubmit", "gtmux:audit:send": return .instructions
        case "hq-report", "hq-progress", "hq-result", "hq-decision", "hq-error": return .updates
        case "create", "register", "update", "associate", "move", "unlink", "interval", "merge", "split": return .organization
        case "Stop", "StopFailure", "gtmux:audit:wake-delivered", "gtmux:audit:wake-dropped",
            "gtmux:wake-degraded", "gtmux:self-check": return .supervision
        case "gtmux:audit:knowledge", "gtmux:distill": return .knowledge
        case "gtmux:audit:rotate", "gtmux:audit:rotate-requested", "gtmux:audit:rotate-failed",
            "gtmux:audit:hq-session", "gtmux:audit:reap", "SessionStart", "SessionEnd": return .maintenance
        default: return .other
        }
    }
    var symbol: String {
        switch kind {
        case "associate": return "link"
        case "unlink": return "link"
        case "interval": return "clock.arrow.circlepath"
        case "create", "register": return "plus.rectangle.on.rectangle"
        case "merge": return "arrow.triangle.merge"
        case "split": return "arrow.triangle.branch"
        case "hq-decision": return "checkmark.bubble"
        case "hq-result": return "doc.text"
        case "hq-error", "StopFailure", "gtmux:audit:rotate-failed", "gtmux:wake-degraded": return "exclamationmark.triangle"
        case "delivery-uncertain": return "questionmark.circle"
        default: return category.symbol
        }
    }
    var needsAttention: Bool {
        ["hq-error", "StopFailure", "delivery-uncertain", "gtmux:audit:rotate-failed", "gtmux:wake-degraded"].contains(kind)
            || ["failed", "refused", "partial-or-failed", "committed-view-failed", "resource-not-closed"].contains(outcome ?? "")
    }
    func includes(in selected: HQActivityCategory?) -> Bool { selected == nil || selected == category }

    /// These fields are enum values only for the listed action kinds, not arbitrary prose.
    func detail(_ detail: String?, l10n l: L10n) -> String? {
        guard let detail, !detail.isEmpty else { return nil }
        if kind == "register" {
            switch detail {
            case "conversation": return l.tr("From an existing conversation", "来源：已有对话")
            case "task": return l.tr("From the task ledger", "来源：任务台账")
            case "board": return l.tr("From the situation board", "来源：态势板")
            default: break
            }
        }
        if kind == "associate" {
            switch detail {
            case "execution": return l.tr("Execution conversation", "执行会话")
            case "reference": return l.tr("Reference conversation", "参考会话")
            default: break
            }
        }
        return detail
    }
}

func hqActivityTitle(_ kind: String, _ l: L10n) -> String {
    switch kind {
    case "create": return l.tr("Matter created", "创建事项")
    case "register": return l.tr("Existing work registered", "登记已有事项")
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
    case "hq-report": return l.tr("HQ report", "HQ 汇报")
    case "hq-progress": return l.tr("Progress update", "进展更新")
    case "hq-result": return l.tr("Result recorded", "结果已记录")
    case "hq-decision": return l.tr("Decision recorded", "决定已记录")
    case "hq-error": return l.tr("Problem reported", "问题已报告")
    case "UserPromptSubmit": return l.tr("HQ received input", "HQ 收到输入")
    case "Stop": return l.tr("HQ turn ended", "HQ 本轮结束")
    case "StopFailure": return l.tr("HQ turn failed", "HQ 本轮失败")
    case "gtmux:audit:wake-delivered": return l.tr("Supervision signal delivered", "观察信号已送达")
    case "gtmux:audit:wake-dropped": return l.tr("Supervision signal not delivered", "观察信号未送达")
    case "gtmux:audit:send": return l.tr("Agent instruction receipt", "执行指令回执")
    case "gtmux:audit:rotate", "gtmux:audit:rotate-requested": return l.tr("HQ rotation requested", "HQ 请求轮换")
    case "gtmux:audit:rotate-failed": return l.tr("HQ rotation failed", "HQ 轮换失败")
    case "gtmux:audit:hq-session": return l.tr("HQ conversation changed", "HQ 会话切换")
    case "gtmux:audit:knowledge": return l.tr("Knowledge operation receipt", "知识台账操作回执")
    case "gtmux:audit:reap": return l.tr("Session reclamation receipt", "会话回收回执")
    case "gtmux:self-check": return l.tr("HQ self-check", "HQ 自查")
    case "gtmux:distill": return l.tr("Knowledge review requested", "知识整理请求")
    case "gtmux:wake-degraded": return l.tr("Supervision needs attention", "观察通道异常")
    case "SessionStart": return l.tr("HQ conversation started", "HQ 会话开始")
    case "SessionEnd": return l.tr("HQ conversation ended", "HQ 会话结束")
    default: return kind
    }
}
