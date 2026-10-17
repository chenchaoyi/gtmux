import Foundation

// The JUDGMENT calls the knowledge reader can make (DESIGN §12, menubar-kb-actions, and
// the audience exits of hq-knowledge-engine).
//
// §12 drew its line between READING HQ's memory and DRIVING the fleet, and put the whole
// knowledge base's action side on the phone and in the CLI. That was one line doing two
// jobs. Driving is still remote-only: nothing here dispatches, spawns or sends. But the
// verbs below are JUDGMENT on something already written, made by a reader looking at the
// entry, and they are cheapest exactly where the reading happens.
//
// What stays off the Mac is AUTHORING: `add` and `supersede` carry prose, and prose typed
// into a popover fills a knowledge base with entries nobody wants to read. Same reason
// they are off the phone's door.
//
// Every one that takes something away or changes a judgement requires a REASON, because
// the CLI requires one. `carry` does not: it is gtmux doing what the promotion already
// decided, and the only question is "now?".

/// One judgment call on one subject. The subject is an entry id, except for `dismiss`,
/// whose subject is a capture KEY (candidates are spool lines, not ledger entries), and
/// `feedback`, whose subject is the prefilled issue URL the CLI computed.
enum KnowledgeAct: Hashable {
    /// audience is who must know it — "hq" | "machine" | "repo:<path>" | "everyone" — or
    /// "" when the reader has not chosen yet (the sheet asks).
    case promote(id: String, audience: String)
    case land(id: String)
    /// gtmux writes it where the audience reads (LOCAL.md / every agent's block / the
    /// repository's file) and lands it — `land` without a ref.
    case carry(id: String)
    case withdraw(id: String)
    case retire(id: String)
    case dismiss(key: String)
    /// The everyone audience's exit: open the prefilled issue. No CLI process.
    case feedback(url: String)

    /// The exact `gtmux` invocation. There is no second implementation of what any of
    /// these verbs MEANS: the app spends a process and lets the CLI decide, so a refusal
    /// the ledger would give at the terminal is the refusal it gives here.
    func argv(reason: String) -> [String] {
        switch self {
        case let .promote(id, audience):
            var a = ["knowledge", "promote", id, "--why", reason]
            if !audience.isEmpty { a += ["--for", audience] }
            return a
        case let .land(id): return ["knowledge", "land", id, "--ref", reason]
        case let .carry(id): return ["knowledge", "land", id]
        case let .withdraw(id): return ["knowledge", "withdraw", id, "--why", reason]
        case let .retire(id): return ["knowledge", "retire", id, "--why", reason]
        case let .dismiss(key): return ["knowledge", "dismiss", "--capture", key, "--why", reason]
        case .feedback: return []
        }
    }

    /// True for the two that take something away. Only the wording differs; the CLI
    /// gates them identically.
    var removes: Bool {
        switch self {
        case .retire, .dismiss: return true
        default: return false
        }
    }

    /// Whether the sheet needs a line of text before it can confirm.
    var needsReason: Bool {
        switch self {
        case .carry, .feedback: return false
        default: return true
        }
    }

    /// Whether this act runs the CLI at all (`feedback` opens a browser instead).
    var runsCLI: Bool {
        if case .feedback = self { return false }
        return true
    }
}

/// The four audiences, in the order the picker shows them, with the one word each wears
/// on every screen.
enum KnowledgeAudience: String, CaseIterable {
    case hq, machine, repo, everyone

    func word(_ l10n: L10n) -> String {
        switch self {
        case .hq: return l10n.tr("HQ only", "仅 HQ")
        case .machine: return l10n.tr("This computer · all local agents", "本机 · 所有本地 agent")
        case .repo: return l10n.tr("A repository · its agents", "项目仓库 · 仓库内的 agent")
        case .everyone: return l10n.tr("Everyone · the product", "全体 · gtmux 产品")
        }
    }
}

/// The short word an entry's audience wears in a line of metadata.
func audienceShort(_ audience: String?, _ l10n: L10n) -> String {
    switch audience ?? "" {
    case "hq": return "HQ"
    case "machine": return l10n.tr("this machine", "本机")
    case "repo": return l10n.tr("a repository", "仓库")
    case "everyone": return l10n.tr("everyone", "全体")
    default: return ""
    }
}

/// The words one act wears: the row's button, and the confirm sheet's title, hint and
/// example.
///
/// UI labels describe the user's decision; argv retains the established ledger verbs.
/// Choosing an audience prepares a brief and never silently applies it.
struct KnowledgeActCopy {
    let button: String
    let title: String
    let hint: String
    let placeholder: String
    /// What the reason is called by the CLI flag it becomes ("" when there is none).
    let field: String
}

extension KnowledgeAct {
    func copy(_ l10n: L10n) -> KnowledgeActCopy {
        switch self {
        case .land:
            return KnowledgeActCopy(
                button: l10n.tr("Record destination…", "记录分发位置…"),
                title: l10n.tr("Record where this knowledge was shared", "记录知识分发位置"),
                hint: l10n.tr(
                    "Enter where you recorded it, such as a PR, spec, runbook or issue. The ledger keeps this reference.",
                    "填写实际记录位置，例如 PR、规范、操作手册或 issue。台账会保存这项信息。"),
                placeholder: l10n.tr("e.g. AGENTS.md / PR #888", "例如 AGENTS.md / PR #888"),
                field: "--ref")
        case .carry:
            return KnowledgeActCopy(
                button: l10n.tr("Apply knowledge", "写入所选范围"),
                title: l10n.tr("Apply knowledge to the selected audience", "将知识写入所选范围"),
                hint: l10n.tr(
                    "gtmux writes it to LOCAL.md, the machine knowledge block or the repository instructions, then marks it landed. Repository files are not committed automatically.",
                    "gtmux 会按所选范围写入 LOCAL.md、本机知识块或仓库指令文件，并标记为已落地。仓库文件不会自动提交。"),
                placeholder: "",
                field: "")
        case .withdraw:
            return KnowledgeActCopy(
                button: l10n.tr("Cancel sharing…", "取消分发…"),
                title: l10n.tr("Cancel knowledge sharing", "取消知识分发"),
                hint: l10n.tr(
                    "The knowledge entry stays. Cancel its pending sharing request and record the reason.",
                    "保留知识条目，取消尚未完成的分发，并记录原因。"),
                placeholder: l10n.tr("e.g. only true on this machine", "例如 只在这台机器上成立"),
                field: "--why")
        case .retire:
            return KnowledgeActCopy(
                button: l10n.tr("Mark as no longer applicable…", "标记为不再适用…"),
                title: l10n.tr("mark as no longer applicable", "标记为不再适用"),
                hint: l10n.tr(
                    "Explain why this entry no longer applies. The ledger keeps your reason.",
                    "说明这条知识为何不再适用。原因会保留在台账中。"),
                placeholder: l10n.tr("e.g. the office network was fixed",
                                     "例如 办公网已修好，这条不再成立"),
                field: "--why")
        case .promote:
            return KnowledgeActCopy(
                button: l10n.tr("Choose audience…", "设置分发范围…"),
                title: l10n.tr("Choose who needs this knowledge", "设置知识分发范围"),
                hint: l10n.tr(
                    "Choose an audience and explain why. This prepares a sharing brief; it does not apply the knowledge yet.",
                    "选择需要使用这条知识的范围，并说明原因。此步生成分发简报，尚不写入目标位置。"),
                placeholder: l10n.tr("e.g. every dispatch repeats this mistake",
                                     "例如 多个 agent 反复遇到同一问题"),
                field: "--why")
        case .dismiss:
            return KnowledgeActCopy(
                button: l10n.tr("Dismiss lead…", "驳回线索…"),
                title: l10n.tr("Dismiss this lead", "驳回这条线索"),
                hint: l10n.tr(
                    "Explain why this lead should be dismissed. It leaves the review queue, and the activity log keeps your reason.",
                    "说明驳回原因。线索将从待整理列表移除，原因会保留在活动记录中。"),
                placeholder: l10n.tr("e.g. already covered by pitfalls/…",
                                     "例如 已被 pitfalls/… 覆盖"),
                field: "--why")
        case .feedback:
            return KnowledgeActCopy(
                button: l10n.tr("Feedback to gtmux ↗", "反馈给 gtmux ↗"),
                title: l10n.tr("feedback to gtmux", "反馈给 gtmux"),
                hint: l10n.tr(
                    "Opens an issue filled from the brief. After submitting it, mark this entry landed with the issue URL.",
                    "会打开预填简报内容的 issue。提交后，用 issue 链接将这条知识标记为已落地。"),
                placeholder: "",
                field: "")
        }
    }
}

/// Which acts an entry offers, in the order they are shown.
///
/// The promotion lifecycle decides the first ones, and the CLI is the authority on that:
/// promoting an entry that is already promoted and unlanded is refused, so a pending entry
/// offers the exit its AUDIENCE has instead — gtmux carries it for hq / machine / repo, a
/// person opens the issue for everyone, and a promotion with no audience can only be
/// landed by hand or withdrawn.
func knowledgeActs(for entry: KBEntry) -> [KnowledgeAct] {
    let id = entry.id
    guard entry.pending else { return [.promote(id: id, audience: ""), .retire(id: id)] }
    switch entry.audience ?? "" {
    case "hq", "machine", "repo":
        return [.carry(id: id), .withdraw(id: id), .retire(id: id)]
    case "everyone":
        var acts: [KnowledgeAct] = []
        if let url = entry.issueUrl, !url.isEmpty { acts.append(.feedback(url: url)) }
        return acts + [.land(id: id), .withdraw(id: id), .retire(id: id)]
    default:
        return [.land(id: id), .withdraw(id: id), .retire(id: id)]
    }
}
