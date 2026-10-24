import Foundation

/// How many recent entries lead the knowledge index.
///
/// Small on purpose: this answers "what did it just learn", which is a glance, not a
/// browse. Everything else is reachable under its topic, so the number is a display
/// choice and not a limit on what can be acted on.
let KBRecentCount = 12

/// The other language's half of an entry (kb-bilingual).
struct KBAlt: Decodable, Equatable {
    let lang: String
    let title: String
    let body: String?
}

/// One knowledge entry, as `gtmux knowledge list --json` prints it.
struct KBEntry: Decodable, Identifiable {
    let id: String
    let topic: String
    let title: String
    let at: Int64?
    let promotedAt: Int64?
    let landedAt: Int64?
    let promoteWhy: String?
    let promoteTarget: String?
    let landedRef: String?
    /// The entry's prose. `knowledge list --json` already carries it, so opening one
    /// entry costs no second process.
    let body: String?
    /// The three axes (hq-knowledge-engine): what it is, where it came from and how
    /// often, who must know it. `kindAssumed` marks a kind the migration guessed;
    /// `issueUrl` is the everyone audience's exit on a pending promotion.
    let kind: String?
    let kindAssumed: Bool?
    let provenance: String?
    let hits: Int?
    let audience: String?
    let audienceRepo: String?
    let status: String?
    let issueUrl: String?
    /// Language (kb-bilingual): the entry's own, whether it was inferred, and the other
    /// language's half when HQ wrote one. Absent from an older CLI: the source, untagged.
    let lang: String?
    let langAssumed: Bool?
    let alt: KBAlt?
    /// The commander's own detail (kb-sensitive-entries): stays on this machine, shown
    /// with a lock. Absent from an older CLI.
    let sensitive: Bool?

    enum CodingKeys: String, CodingKey {
        case id, topic, title, at, body
        case promotedAt, landedAt, promoteWhy, promoteTarget, landedRef
        case kind, kindAssumed, provenance, hits, audience, audienceRepo, status, issueUrl
        case lang, langAssumed, alt, sensitive
    }

    /// resolved picks the half a reader gets — one rule on every surface: the source
    /// when it is the reader's language, else the alternate when that is, else the source
    /// with a tag naming its language.
    func resolved(_ readerLang: String) -> (title: String, body: String?, tag: String) {
        guard let l = lang, !l.isEmpty, l != readerLang else { return (title, body, "") }
        if let a = alt, a.lang == readerLang, !a.title.isEmpty { return (a.title, a.body ?? body, "") }
        return (title, body, l)
    }

    /// The title as shown: resolved, tagged when the reader did not get their language.
    func displayTitle(_ readerLang: String) -> String {
        let r = resolved(readerLang)
        return r.tag.isEmpty ? r.title : r.title + " [" + r.tag + "]"
    }

    /// The shown title split into the identifier at its head and the sentence after it.
    func titleParts(_ readerLang: String) -> (key: String?, rest: String) {
        splitTitleKey(displayTitle(readerLang), id: id)
    }

    /// The memberwise shape older callers and tests use, with the axes optional: a row
    /// that predates them is a row without them.
    init(
        id: String, topic: String, title: String, at: Int64?, promotedAt: Int64?, landedAt: Int64?,
        promoteWhy: String?, promoteTarget: String?, landedRef: String?, body: String?,
        kind: String? = nil, kindAssumed: Bool? = nil, provenance: String? = nil, hits: Int? = nil,
        audience: String? = nil, audienceRepo: String? = nil, status: String? = nil,
        issueUrl: String? = nil,
        lang: String? = nil, langAssumed: Bool? = nil, alt: KBAlt? = nil, sensitive: Bool? = nil
    ) {
        self.id = id
        self.topic = topic
        self.title = title
        self.at = at
        self.promotedAt = promotedAt
        self.landedAt = landedAt
        self.promoteWhy = promoteWhy
        self.promoteTarget = promoteTarget
        self.landedRef = landedRef
        self.body = body
        self.kind = kind
        self.kindAssumed = kindAssumed
        self.provenance = provenance
        self.hits = hits
        self.audience = audience
        self.audienceRepo = audienceRepo
        self.status = status
        self.issueUrl = issueUrl
        self.lang = lang
        self.langAssumed = langAssumed
        self.alt = alt
        self.sensitive = sensitive
    }

    /// The axes as one metadata line: `pitfalls? · from mined ×6 · for this machine`.
    func axesLine(_ l10n: L10n) -> String {
        var parts: [String] = []
        if let k = kind, !k.isEmpty { parts.append(knowledgeKind(k, l10n) + ((kindAssumed ?? false) ? l10n.tr(" (unconfirmed category)", "（类别待确认）") : "")) }
        if let pv = provenance, !pv.isEmpty {
            var s = l10n.tr("Source: ", "来源：") + knowledgeProvenance(pv, l10n)
            if let h = hits, h > 1 { s += " ×\(h)" }
            parts.append(s)
        }
        if status == "hypothesis" { parts.append(l10n.tr("hypothesis", "待验证")) }
        if sensitive ?? false { parts.append(l10n.tr("Personal details", "个人资料")) }
        return parts.joined(separator: " · ")
    }

    /// Promoted and not yet carried: the one part of the knowledge lifecycle that waits on
    /// a person.
    var pending: Bool { (promotedAt ?? 0) > 0 && (landedAt ?? 0) == 0 }
}

/// splitTitleKey pulls the KEY off the head of an entry title.
///
/// HQ writes titles as "kb-entry-date-is-utc KB 条目落款用 UTC,…": a stable identifier
/// followed by the prose. Both matter, but not equally and not in the same voice. Printed
/// as one run, four kebab words take the most prominent line of a row and say the least,
/// which is what the reader sees first in a rail this narrow. The phone and iPad have set
/// the identifier on its own line in a monospace face since #1099; this surface went on
/// running the two together (reported 2026-09-18).
///
/// The split is decided by EVIDENCE, not by shape. A pass keyed on "looks kebab-case"
/// turns "well-known trap in the office network" into a key plus a fragment, and a title
/// losing its first phrase to a guess is worse than a title with a plain head. So the head
/// must also match the entry's own id, which the ledger derives from the title.
///
/// The id keeps only the first six words (knowledge.Slug caps it there), so a longer key
/// is compared by its first six; the key SHOWN is still the whole head. Same rule, same
/// number, as `splitTitleKey` in mobileapp/src/screens/knowledgeModel.ts.
let kbIDSlugWords = 6

func splitTitleKey(_ title: String, id: String) -> (key: String?, rest: String) {
    let trimmed = title.trimmingCharacters(in: .whitespacesAndNewlines)
    guard let sep = trimmed.firstIndex(where: { $0 == ":" || $0.isWhitespace }) else {
        return (nil, title)
    }
    let head = String(trimmed[trimmed.startIndex..<sep])
    let words = head.split(separator: "-", omittingEmptySubsequences: false)
    guard words.count >= 2,
        words.allSatisfy({ w in
            !w.isEmpty && w.allSatisfy { $0.isASCII && (($0.isLetter && $0.isLowercase) || $0.isNumber) }
        })
    else { return (nil, title) }
    let rest = trimmed[trimmed.index(after: sep)...].drop(while: { $0.isWhitespace })
    guard !rest.isEmpty else { return (nil, title) }
    let slug = id.split(separator: "/").last.map(String.init) ?? ""
    guard slug.hasPrefix(words.prefix(kbIDSlugWords).joined(separator: "-")) else {
        return (nil, title)
    }
    return (head, String(rest))
}

/// One pending-distill candidate, as `gtmux capture --list --json` prints it. A candidate
/// is NOT a knowledge entry: it is a cheap one-line notice any worker can drop, waiting on
/// the supervisor's quality gate.
struct KBCandidate: Decodable {
    let at: Int64
    let topic: String
    let key: String
    let lesson: String
    /// Family number from `capture --list --json`: candidates that read as one lesson
    /// share it; a singleton has none.
    let group: Int?
}

/// Candidates SHARING a dedup key, as one thing to act on.
///
/// The grouping is not presentation. `gtmux knowledge dismiss --capture <key>` consumes
/// EVERY pending line with that key (the merge the distill discipline promises), so a row
/// per line would offer three buttons that each do the same thing to all three. The row is
/// the key, and it says how many lines it stands for.
struct KBCandidateGroup: Identifiable {
    let key: String
    let topic: String
    let lesson: String
    let at: Int64
    let count: Int
    /// The family this key belongs to (0 = none): rows of one family sit together, and
    /// the family's keys are what one `knowledge add --capture k1,k2,…` takes.
    let family: Int
    var id: String { key }
}

/// groupCandidates folds the spool by key, oldest group first (the oldest is the one
/// rotting), keeping the newest phrasing as the line to show.
func groupCandidates(_ cands: [KBCandidate]) -> [KBCandidateGroup] {
    var order: [String] = []
    var byKey: [String: [KBCandidate]] = [:]
    for c in cands {
        if byKey[c.key] == nil { order.append(c.key) }
        byKey[c.key, default: []].append(c)
    }
    return order.compactMap { key -> KBCandidateGroup? in
        guard let rows = byKey[key], let newest = rows.last else { return nil }
        let oldest = rows.map { $0.at }.min() ?? newest.at
        return KBCandidateGroup(
            key: key, topic: newest.topic, lesson: newest.lesson,
            at: oldest, count: rows.count, family: newest.group ?? 0)
    }
    // Oldest first, but a family stays together (by its earliest member), so the reader
    // sees "these three are one thing" without reading all three.
    .sorted { $0.at < $1.at }
    .familiesTogether()
}

extension Array where Element == KBCandidateGroup {
    func familiesTogether() -> [KBCandidateGroup] {
        var firstAt: [Int: Int64] = [:]
        for g in self where g.family > 0 {
            firstAt[g.family] = Swift.min(firstAt[g.family] ?? g.at, g.at)
        }
        return sorted { a, b in
            let ka = a.family > 0 ? (firstAt[a.family] ?? a.at) : a.at
            let kb = b.family > 0 ? (firstAt[b.family] ?? b.at) : b.at
            if ka != kb { return ka < kb }
            if a.family != b.family { return a.family < b.family }
            return a.at < b.at
        }
    }
}

/// What the board read returns. `exists:false` is ORDINARY — a fresh HQ has written none.
struct BoardDoc: Decodable {
    let exists: Bool
    let updatedAt: Int64?
    let text: String?

    enum CodingKeys: String, CodingKey {
        case exists
        case updatedAt = "updated_at"
        case text
    }
}
