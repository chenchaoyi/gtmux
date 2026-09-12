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
        case .hq: return l10n.tr("HQ · only this supervisor", "HQ · 只给中控")
        case .machine: return l10n.tr("This machine · every agent here", "本机 · 这台机器上所有 agent")
        case .repo: return l10n.tr("A repository · the agents working there", "仓库 · 在那个仓库干活的 agent")
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
/// `land` and `retire` are the phone's copy, carried over verbatim (KnowledgeSheet.tsx /
/// knowledgeModel.ts). The commander meets the same verbs on two screens and should not
/// have to notice they are the same; a third phrasing invented here would be a third
/// thing to keep true.
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
                button: l10n.tr("Mark it landed…", "标记为已落地…"),
                title: l10n.tr("Mark it landed", "标记为已落地"),
                hint: l10n.tr(
                    "Where did it land? A PR, a spec, a runbook name, the issue you opened — this survives in the ledger.",
                    "落到哪儿了？PR、spec、runbook 名、你开的 issue 都行 —— 这条会留在账本里。"),
                placeholder: l10n.tr("e.g. AGENTS.md / PR #888", "例如 AGENTS.md / PR #888"),
                field: "--ref")
        case .carry:
            return KnowledgeActCopy(
                button: l10n.tr("Write it in", "写进去"),
                title: l10n.tr("let gtmux carry it", "让 gtmux 搬进去"),
                hint: l10n.tr(
                    "gtmux writes it where this audience reads — your LOCAL.md, every agent's knowledge block on this machine, or the repository's instruction file (not committed) — and marks it landed.",
                    "gtmux 把它写到这个读者看的地方 —— 你的 LOCAL.md、本机每个 agent 的知识块、或那个仓库的指令文件（不提交）—— 然后标记为已落地。"),
                placeholder: "",
                field: "")
        case .withdraw:
            return KnowledgeActCopy(
                button: l10n.tr("Withdraw the promotion…", "撤回晋升…"),
                title: l10n.tr("withdraw this promotion", "撤回这次晋升"),
                hint: l10n.tr(
                    "The entry stays. Only the promotion goes — why is it not worth carrying? The reason survives in the journal.",
                    "条目留着，只撤掉晋升 —— 为什么不值得搬？理由会留在事件流里。"),
                placeholder: l10n.tr("e.g. only true on this machine", "例如 只在这台机器上成立"),
                field: "--why")
        case .retire:
            return KnowledgeActCopy(
                button: l10n.tr("Retire it…", "退休这一条…"),
                title: l10n.tr("retire this entry", "退休这一条"),
                hint: l10n.tr(
                    "Why? The reason survives in the ledger — it is the only place a later reader can learn what was wrong with it.",
                    "为什么？理由会留在账本里 —— 「这条后来错在哪」将来只能从它读到。"),
                placeholder: l10n.tr("e.g. the office network was fixed",
                                     "例如 办公网已修好，这条不再成立"),
                field: "--why")
        case .promote:
            return KnowledgeActCopy(
                button: l10n.tr("Promote it…", "晋升这一条…"),
                title: l10n.tr("promote this entry", "晋升这一条"),
                hint: l10n.tr(
                    "Who must know it, and why? The audience decides the exit; the case is what whoever carries it will read in the brief.",
                    "这条给谁看、为什么？读者决定出口；理由会写进简报，带走它的人读的就是它。"),
                placeholder: l10n.tr("e.g. every dispatch repeats this mistake",
                                     "例如 每次派活都在重犯这个错"),
                field: "--why")
        case .dismiss:
            return KnowledgeActCopy(
                button: l10n.tr("Dismiss it…", "驳回这条候选…"),
                title: l10n.tr("dismiss this candidate", "驳回这条候选"),
                hint: l10n.tr(
                    "Why? The candidate goes away and the reason stays in the journal, so a rejection does not vanish the way an acceptance does.",
                    "为什么？候选会消失，理由留在事件流里，驳回不该像采纳那样悄无声息。"),
                placeholder: l10n.tr("e.g. already covered by pitfalls/…",
                                     "例如 已被 pitfalls/… 覆盖"),
                field: "--why")
        case .feedback:
            return KnowledgeActCopy(
                button: l10n.tr("Feedback to gtmux ↗", "反馈给 gtmux ↗"),
                title: l10n.tr("feedback to gtmux", "反馈给 gtmux"),
                hint: l10n.tr(
                    "Opens a new issue prefilled from the brief. When it is filed, mark this entry landed with the issue's URL.",
                    "打开一条用简报预填好的新 issue。提交后，用 issue 链接把这条标记为已落地。"),
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
