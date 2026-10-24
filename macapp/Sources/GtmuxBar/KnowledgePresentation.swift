import Foundation

// Presentation only: these labels never change ledger keys or instruction files.
func knowledgeTopic(_ value: String, _ l10n: L10n) -> String {
    let words: [String: (String, String)] = [
        "accounts": ("Accounts and services", "账号与服务"),
        "workflows": ("Workflows", "操作流程"),
        "best-practices": ("Working principles", "做事原则"),
        "pitfalls": ("Common pitfalls", "避坑提醒"),
        "corrections": ("Corrections", "纠正记录"),
        "environment": ("Computer environment", "本机环境"),
    ]
    guard let w = words[value] else { return value }
    return l10n.tr(w.0, w.1)
}

func knowledgeKind(_ value: String, _ l10n: L10n) -> String {
    let words: [String: (String, String)] = [
        "facts": ("Facts", "事实信息"), "howto": ("How-to", "操作方法"),
        "pitfalls": ("Pitfalls", "避坑提醒"), "judgment": ("Judgment criteria", "判断依据"),
        "decisions": ("Decision record", "决策记录"),
    ]
    guard let w = words[value] else { return value }
    return l10n.tr(w.0, w.1)
}

func knowledgeProvenance(_ value: String, _ l10n: L10n) -> String {
    let words: [String: (String, String)] = [
        "correction": ("User correction", "用户纠正"), "recurrence": ("Repeated observation", "重复出现"),
        "mined": ("Conversation review", "会话整理"), "capture": ("Agent note", "Agent 记录"),
        "self": ("HQ observation", "HQ 观察"),
    ]
    guard let w = words[value] else { return value }
    return l10n.tr(w.0, w.1)
}

struct KnowledgeUsage {
    let title: String
    let state: String
    let explanation: String
    var line: String { state.isEmpty ? title : title + " · " + state }
}

extension KBEntry {
    // Recorded use is not proof that a running agent loaded an instruction file.
    func usage(_ l10n: L10n) -> KnowledgeUsage {
        guard let scope = KnowledgeAudience(rawValue: audience ?? "") else {
            if pending || (landedAt ?? 0) > 0 || !(audience ?? "").isEmpty {
                return KnowledgeUsage(title: l10n.tr("Use not recorded", "用途待确认"),
                    state: "",
                    explanation: l10n.tr("This older record has no recognized use. Review its destination before choosing a new plan.",
                        "这条旧记录没有注明可识别的用途。请先核对应用记录，再选择新的使用方案。"))
            }
            return KnowledgeUsage(title: l10n.tr("HQ reference", "HQ 按需查阅"), state: "",
                explanation: l10n.tr("Available for HQ lookup and delegation. No standing-instruction use is recorded for this entry.",
                    "供 HQ 查找、派活时参考；当前记录未注明固定指令用途。"))
        }
        let state = pending
            ? (scope == .everyone ? l10n.tr("Feedback to complete", "待完成反馈") : l10n.tr("Write pending", "待完成写入"))
            : ((landedAt ?? 0) > 0 ? l10n.tr("Application recorded", "已记录应用") : l10n.tr("Use selected", "已指定用途"))
        return KnowledgeUsage(title: scope.word(l10n), state: state,
            explanation: pending ? scope.explanation(l10n) : l10n.tr(
                "The ledger records the result below. It does not confirm that running agents loaded it or that product feedback has shipped.",
                "应用结果见记录详情；这不代表运行中的 Agent 已加载，也不代表产品建议已发布。"))
    }
}

extension KnowledgeAudience {
    func explanation(_ l10n: L10n) -> String {
        switch self {
        case .hq: return l10n.tr(
            "Add to HQ's standing instructions. Use for delegation, review and when HQ should ask you to decide.",
            "加入 HQ 每次加载的固定指令。适合派活、审阅、何时请你确认等协作约定。")
        case .machine: return l10n.tr(
            "Make available through local agents' instruction files. Use for facts and precautions shared on this computer. Running sessions may need to reload.",
            "通过本机 Agent 的指令文件提供。适合共同的环境事实与操作注意事项；已有会话可能需要重新加载。")
        case .repo: return l10n.tr(
            "Write into the selected project's instructions. Use for that project's build, test and coding conventions. Files are not committed automatically.",
            "写入指定项目的指令文件。适合该项目的构建、测试和代码约定；不会自动提交文件。")
        case .everyone: return l10n.tr(
            "Prepare feedback for the gtmux project for you to review and submit. It does not publish your knowledge base or change every user's rules.",
            "准备给 gtmux 项目的改进反馈，由你审阅后提交。不会公开整个知识库，也不会立即改变其他用户的规则。")
        }
    }
}
