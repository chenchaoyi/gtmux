import Foundation

// BoardOutline — the situation board as an outline, the same one the phone reads.
//
// The Mac was rendering the whole board as one scroll while the phone had moved on
// three times (mobile #945, #947, #951). On this machine that is 74 KB: reaching any
// particular entry meant dragging through the rest, which is precisely the complaint
// the phone fixed. Two surfaces of one product must not disagree about what reading the
// board is like, so this is a deliberate port of `mobileapp/src/screens/boardSections.ts`
// rather than a second design.
//
// The rules it carries over, each for a reason that was paid for once already:
//
//   - TWO LEVELS. The board's `##` level is two rows; its entries live at `###`. An
//     outline that stops at `##` gives a reader a screen of void or a wall of text.
//   - ORDER IS THE AUTHOR'S, never re-sorted. HQ pins a "read this first" handoff at the
//     TOP and appends the newest progress at the BOTTOM, so position is not chronology
//     and sorting by it would call the pinned summary the oldest entry.
//   - A heading inside a fenced code block is NOT a heading. The board quotes shell and
//     JSON constantly, and splitting on those cuts a section in half at a comment.
//   - The document's own `# ` title is dropped: the reader already has a title bar, and
//     the file's title rendered under it as a second, larger one.

struct BoardSection: Identifiable, Equatable {
    /// The heading text, without its `#` marks. Empty for content before the first one.
    let title: String
    /// This section's OWN markdown. A child's text is not repeated here.
    let body: String
    /// Stable across re-parses of the same board, so expansion survives a poll.
    let id: String
    /// The `###` entries under a `##`. Empty for a leaf.
    let children: [BoardSection]
}

enum BoardOutline {
    /// Splits markdown into `##` sections, each holding its `###` entries.
    static func parse(_ md: String) -> [BoardSection] {
        var out: [BoardSection] = []
        var secTitle: String?
        var secBody = ""
        var secKids: [BoardSection] = []
        var subTitle: String?
        var buf: [String] = []
        var fenced = false
        var n = 0
        var started = false

        func text() -> String { buf.joined(separator: "\n").trimmingCharacters(in: .whitespacesAndNewlines) }
        func key(_ t: String) -> String {
            defer { n += 1 }
            return "\(n):\(String(t.prefix(40)))"
        }
        func closeSub() {
            if let t = subTitle {
                secKids.append(BoardSection(title: t, body: text(), id: key(t), children: []))
                buf = []
                subTitle = nil
            }
        }
        func closeSec() {
            closeSub()
            if let t = secTitle {
                if secBody.isEmpty { secBody = text() }
                buf = []
                // A `##` with neither text nor entries is a heading over nothing.
                if !secBody.isEmpty || !secKids.isEmpty {
                    out.append(BoardSection(title: t, body: secBody, id: key(t), children: secKids))
                }
                secTitle = nil
                secBody = ""
                secKids = []
            }
        }
        func preamble() {
            var body = text()
            if body.hasPrefix("# ") {
                if let nl = body.firstIndex(of: "\n") {
                    body = String(body[body.index(after: nl)...]).trimmingCharacters(in: .whitespacesAndNewlines)
                } else {
                    body = ""
                }
            }
            buf = []
            if !body.isEmpty {
                out.append(BoardSection(title: "", body: body, id: key(""), children: []))
            }
        }

        for line in md.components(separatedBy: "\n") {
            let t = line.trimmingCharacters(in: .whitespaces)
            if t.hasPrefix("```") || t.hasPrefix("~~~") { fenced.toggle() }
            let h2 = fenced ? nil : heading(line, level: 2)
            let h3 = fenced ? nil : heading(line, level: 3)
            if let h = h2 {
                if !started {
                    preamble()
                    started = true
                } else {
                    closeSec()
                }
                secTitle = h
                continue
            }
            if let h = h3, secTitle != nil {
                // The `##`'s own text ends where its first entry begins.
                // Keyed on "no entry open and none closed yet", not on the body being
                // empty: a section whose own text is genuinely empty would otherwise
                // take this branch again on its second entry and swallow the first
                // entry's text. (The TS twin gets this for free by pushing the child
                // eagerly, so its children.length guards the branch.)
                if subTitle == nil && secKids.isEmpty {
                    secBody = text()
                } else {
                    closeSub()
                }
                buf = []
                subTitle = h
                continue
            }
            buf.append(line)
        }
        if !started { preamble() } else { closeSec() }
        return out
    }

    /// The heading text at exactly `level`, or nil. `### x` is not an `##`.
    private static func heading(_ line: String, level: Int) -> String? {
        let marks = String(repeating: "#", count: level)
        guard line.hasPrefix(marks + " ") else { return nil }
        guard !line.hasPrefix(marks + "# ") else { return nil }
        let t = String(line.dropFirst(level + 1)).trimmingCharacters(in: .whitespaces)
        return t.isEmpty ? nil : t
    }

    /// What a section HOLDS, for the bubble beside its heading: its own table rows or
    /// bullets when it has any, else the entries nested under it. nil when neither.
    ///
    /// Own content wins because the heading is a promise. 「① 现状 — 在跑的 pane」 leads
    /// with a 13-row table of panes AND carries four sub-headings; counting the
    /// sub-headings turned that 13 into a 4 and hid the number the title just asked
    /// about. It is never the LINE count, which read "154" beside a twelve-row table.
    static func count(_ s: BoardSection) -> Int? {
        if let own = countOwn(s.body) { return own }
        return s.children.isEmpty ? nil : s.children.count
    }

    static func countOwn(_ body: String) -> Int? {
        let lines = body.components(separatedBy: "\n")
        if let sep = lines.firstIndex(where: { isTableSeparatorLine($0) }), sep > 0,
           lines[sep - 1].contains("|") {
            var rows = 0
            var i = sep + 1
            while i < lines.count {
                let l = lines[i].trimmingCharacters(in: .whitespaces)
                if !l.hasPrefix("|") { break }
                rows += 1
                i += 1
            }
            if rows > 0 { return rows }
        }
        let bullets = lines.filter { l in
            let t = l.drop(while: { $0 == " " })
            guard l.count - t.count <= 3, let f = t.first, "-*•".contains(f) else { return false }
            let rest = t.dropFirst()
            return rest.first == " " && rest.dropFirst().first != nil
        }.count
        return bullets > 0 ? bullets : nil
    }

    private static func isTableSeparatorLine(_ l: String) -> Bool {
        let t = l.trimmingCharacters(in: .whitespaces)
        guard t.contains("-"), t.contains("|") else { return false }
        return t.allSatisfy { "|-: \t".contains($0) }
    }
}

/// One topic and the entries filed under it.
struct KBTopic: Identifiable {
    let name: String
    let entries: [KBEntry]
    var id: String { name }
}

/// knowledgeTopics groups entries by topic, BIGGEST FIRST, matching the phone.
///
/// The base is 386 entries across seven topics, two of which hold 177 and 162. A flat
/// list of 386 is not a list anyone reads; it is a thing you scroll past looking for the
/// end. Grouped and folded, the whole base is seven rows.
///
/// Biggest first because that is what the phone shows and a reader who learns the order
/// on one screen should not have to learn it again on the other. Ties break on the name,
/// so two topics of equal size do not swap places between polls.
func knowledgeTopics(_ entries: [KBEntry]) -> [KBTopic] {
    var order: [String] = []
    var byTopic: [String: [KBEntry]] = [:]
    for e in entries {
        if byTopic[e.topic] == nil { order.append(e.topic) }
        byTopic[e.topic, default: []].append(e)
    }
    return order
        .map { KBTopic(name: $0, entries: byTopic[$0] ?? []) }
        .sorted { a, b in
            a.entries.count == b.entries.count ? a.name < b.name : a.entries.count > b.entries.count
        }
}

/// matchKB filters the base by what someone typed.
///
/// Matching runs over the title, the id and the topic, case-insensitively. The id is how
/// HQ names an entry in a dispatch, so it is a thing people actually paste. Whitespace
/// splits the query into terms that must ALL match, so two words narrow instead of widen
/// — the same rule as the phone's `matchEntries`, so one query behaves the same on both.
func matchKB(_ entries: [KBEntry], _ query: String) -> [KBEntry] {
    let terms = query.lowercased().split(whereSeparator: { $0.isWhitespace }).map(String.init)
    if terms.isEmpty { return [] }
    return entries.filter { e in
        let hay = "\(e.title) \(e.id) \(e.topic)".lowercased()
        return terms.allSatisfy { hay.contains($0) }
    }
}

/// How long ago HQ last wrote the board, in the words the phone already uses.
///
/// The phone's board sheet says "updated 3h ago" under its title; the Mac said nothing at
/// all, while `updatedAt` sat on the model unused (2026-09-09). A board read as current
/// when it is hours stale is a failure that costs something real — it is HQ's picture of
/// the fleet, and the only reason to open it is that the picture can be trusted.
///
/// Split out from the view so the buckets can be tested at their edges rather than at one
/// convenient point in the middle of each.
func boardAgeText(_ secs: Int64, zh: Bool) -> String {
    if secs < 60 { return zh ? "刚刚更新" : "updated just now" }
    if secs < 3600 { return zh ? "\(secs / 60) 分钟前更新" : "updated \(secs / 60)m ago" }
    if secs < 48 * 3600 { return zh ? "\(secs / 3600) 小时前更新" : "updated \(secs / 3600)h ago" }
    return zh ? "\(secs / 86400) 天前更新" : "updated \(secs / 86400)d ago"
}
