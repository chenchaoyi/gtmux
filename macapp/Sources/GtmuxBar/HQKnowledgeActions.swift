import Foundation

// The four JUDGMENT calls the knowledge reader can make (DESIGN §12, menubar-kb-actions).
//
// §12 drew its line between READING HQ's memory and DRIVING the fleet, and put the whole
// knowledge base's action side on the phone and in the CLI. That was one line doing two
// jobs. Driving is still remote-only: nothing here dispatches, spawns or sends. But the
// four verbs below are JUDGMENT on something already written, made by a reader looking at
// the entry, and they are cheapest exactly where the reading happens.
//
// What stays off the Mac is AUTHORING: `add` and `supersede` carry prose, and prose typed
// into a popover fills a knowledge base with entries nobody wants to read. Same reason
// they are off the phone's door.
//
// Every one of them requires a REASON, because the CLI requires one. That is not a UI
// nicety copied inward: "why did this go away" is unanswerable from an absence, and the
// ledger is the only place a later reader can learn it.

/// One judgment call on one subject. The subject is an entry id, except for `dismiss`,
/// whose subject is a capture KEY (candidates are spool lines, not ledger entries).
enum KnowledgeAct: Hashable {
    case promote(id: String)
    case land(id: String)
    case retire(id: String)
    case dismiss(key: String)

    /// The exact `gtmux` invocation. There is no second implementation of what any of
    /// these verbs MEANS: the app spends a process and lets the CLI decide, so a refusal
    /// the ledger would give at the terminal is the refusal it gives here.
    func argv(reason: String) -> [String] {
        switch self {
        case let .promote(id): return ["knowledge", "promote", id, "--why", reason]
        case let .land(id): return ["knowledge", "land", id, "--ref", reason]
        case let .retire(id): return ["knowledge", "retire", id, "--why", reason]
        case let .dismiss(key): return ["knowledge", "dismiss", "--capture", key, "--why", reason]
        }
    }

    /// True for the two that take something away. Only the wording differs; the CLI
    /// gates all four identically.
    var removes: Bool {
        switch self {
        case .retire, .dismiss: return true
        case .promote, .land: return false
        }
    }
}

/// The words one act wears: the row's button, and the confirm sheet's title, hint and
/// example.
///
/// `land` and `retire` are the phone's copy, carried over verbatim (KnowledgeSheet.tsx /
/// knowledgeModel.ts). The commander meets the same two verbs on two screens and should
/// not have to notice they are the same; a third phrasing invented here would be a third
/// thing to keep true. `promote` and `dismiss` are new to any GUI and are written in that
/// same voice.
struct KnowledgeActCopy {
    let button: String
    let title: String
    let hint: String
    let placeholder: String
    /// What the reason is called by the CLI flag it becomes.
    let field: String
}

extension KnowledgeAct {
    func copy(_ l10n: L10n) -> KnowledgeActCopy {
        switch self {
        case .land:
            return KnowledgeActCopy(
                button: l10n.tr("mark it landed…", "标记为已落地…"),
                title: l10n.tr("mark it landed", "标记为已落地"),
                hint: l10n.tr(
                    "Where did it land? A PR, a spec, a runbook name — this survives in the ledger.",
                    "落到哪儿了？PR、spec、runbook 名都行 —— 这条会留在账本里。"),
                placeholder: l10n.tr("e.g. AGENTS.md / PR #888", "例如 AGENTS.md / PR #888"),
                field: "--ref")
        case .retire:
            return KnowledgeActCopy(
                button: l10n.tr("retire it…", "退休这一条…"),
                title: l10n.tr("retire this entry", "退休这一条"),
                hint: l10n.tr(
                    "Why? The reason survives in the ledger — it is the only place a later reader can learn what was wrong with it.",
                    "为什么？理由会留在账本里 —— 「这条后来错在哪」将来只能从它读到。"),
                placeholder: l10n.tr("e.g. the office network was fixed",
                                     "例如 办公网已修好，这条不再成立"),
                field: "--why")
        case .promote:
            return KnowledgeActCopy(
                button: l10n.tr("promote it…", "晋升这一条…"),
                title: l10n.tr("promote this entry", "晋升这一条"),
                hint: l10n.tr(
                    "Why is this charter-level? The case is what whoever carries it will read in the export brief.",
                    "为什么它够得上守则级？这段理由会写进外送简报，带走它的人读的就是它。"),
                placeholder: l10n.tr("e.g. every dispatch repeats this mistake",
                                     "例如 每次派活都在重犯这个错"),
                field: "--why")
        case .dismiss:
            return KnowledgeActCopy(
                button: l10n.tr("dismiss it…", "驳回这条候选…"),
                title: l10n.tr("dismiss this candidate", "驳回这条候选"),
                hint: l10n.tr(
                    "Why? The candidate goes away and the reason stays in the journal, so a rejection does not vanish the way an acceptance does.",
                    "为什么？候选会消失，理由留在事件流里，驳回不该像采纳那样悄无声息。"),
                placeholder: l10n.tr("e.g. already covered by pitfalls/…",
                                     "例如 已被 pitfalls/… 覆盖"),
                field: "--why")
        }
    }
}

/// Which acts an entry offers, in the order they are shown.
///
/// The promotion lifecycle decides the first one, and the CLI is the authority on that:
/// promoting an entry that is already promoted and unlanded is refused ("land it before
/// promoting again"), so offering `promote` there would be offering a button whose only
/// outcome is an error message. A promoted-and-unlanded entry offers `land` instead, which
/// is the step that is actually waiting.
func knowledgeActs(for entry: KBEntry) -> [KnowledgeAct] {
    (entry.pending ? [.land(id: entry.id)] : [.promote(id: entry.id)]) + [.retire(id: entry.id)]
}
