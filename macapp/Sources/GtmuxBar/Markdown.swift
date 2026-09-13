import Foundation

// Markdown — the block model the situation board is written in.
//
// The Mac was showing the board's SOURCE: 545 lines of raw markdown in a monospaced dump,
// while the phone has rendered it since it shipped. Same document, two surfaces, one of
// them asking the reader to parse it themselves.
//
// The block vocabulary MIRRORS the mobile renderer's (mobileapp/src/ui/markdown.ts) on
// purpose — one design language across the surfaces, and a board that reads the same way
// wherever you open it. It covers what the board actually contains, measured: 1 `#`,
// 2 `##`, 72 `###`, one 6-column table, 50 bullets, 254 lines carrying inline code (the
// pane ids), 45 with bold. No fences and no quotes today; both are parsed anyway so an
// author who reaches for one is not met with raw syntax.
//
// Lists, both kinds, own their CONTINUATION lines (CommonMark's lazy continuation): the
// wrapped prose under "1. …" belongs to that item until a blank line or another block
// starts. The board's "还等你定的" section is numbered items with wrapped lines, and
// without this the Mac ran all of them together into one paragraph — "1. … 2. … 3. …" as
// prose — while the phone, which learned the rule on 2026-08-09, showed a list.

enum MDInline: Equatable {
    case text(String)
    case code(String)
    case bold(String)
    /// `[[another-entry]]` — the knowledge base's own cross-reference. It is a graph, and
    /// printing its edges as literal brackets both adds noise and hides the structure.
    case link(String)
}

enum MDBlock: Equatable {
    case heading(level: Int, spans: [MDInline])
    case paragraph([MDInline])
    case bullets([[MDInline]])
    /// `1. …` items; `start` is the first item's number, so a list that begins at 3 renders 3, 4, 5.
    case ordered([[MDInline]], start: Int)
    case code(String)
    case quote([MDInline])
    /// header + rows, each cell already split into spans.
    case table(header: [[MDInline]], rows: [[[MDInline]]])
    case rule
}

enum Markdown {
    /// parseInline splits a line into text, `code` and **bold** runs.
    ///
    /// Deliberately small: these three are what the board uses, and a parser that tried to
    /// be complete would be a second implementation of something the phone already has.
    /// An unmatched marker stays literal — guessing where a run ends silently restyles the
    /// rest of the sentence.
    static func parseInline(_ line: String) -> [MDInline] {
        var out: [MDInline] = []
        var buf = ""
        var i = line.startIndex

        func flush() {
            if !buf.isEmpty {
                out.append(.text(buf))
                buf = ""
            }
        }

        while i < line.endIndex {
            let c = line[i]
            if c == "[", line[i...].hasPrefix("[["),
               let close = range(of: "]]", in: line, from: line.index(i, offsetBy: 2)) {
                flush()
                out.append(.link(String(line[line.index(i, offsetBy: 2)..<close.lowerBound])))
                i = close.upperBound
                continue
            }
            if c == "`", let close = line[line.index(after: i)...].firstIndex(of: "`") {
                flush()
                out.append(.code(String(line[line.index(after: i)..<close])))
                i = line.index(after: close)
                continue
            }
            if c == "*", line[i...].hasPrefix("**"),
               let close = range(of: "**", in: line, from: line.index(i, offsetBy: 2)) {
                flush()
                out.append(.bold(String(line[line.index(i, offsetBy: 2)..<close.lowerBound])))
                i = close.upperBound
                continue
            }
            buf.append(c)
            i = line.index(after: i)
        }
        flush()
        return out
    }

    private static func range(of needle: String, in s: String, from: String.Index) -> Range<String.Index>? {
        guard from < s.endIndex else { return nil }
        return s.range(of: needle, range: from..<s.endIndex)
    }

    /// parseBlocks turns the document into blocks, in the author's order. Order is never
    /// rearranged: the board leads with what its writer put first.
    /// HTML comments are the author writing to themselves, not to the reader.
    ///
    /// The board opens with `<!-- 写法规则:一格一句话… -->`, a note HQ leaves for whoever
    /// edits it next. Rendered, it was the first thing under the section heading: an
    /// instruction addressed to someone else, in the most prominent place on the page.
    /// Stripped before parsing, not skipped as a block, because a comment can open and
    /// close mid-line.
    static func stripComments(_ md: String) -> String {
        var out = ""
        var rest = Substring(md)
        while let open = rest.range(of: "<!--") {
            out += rest[rest.startIndex..<open.lowerBound]
            guard let close = rest.range(of: "-->", range: open.upperBound..<rest.endIndex) else {
                // Unterminated: KEEP the rest. HTML would call it all a comment, but the
                // board is written by hand and a typo'd `<!--` would then blank the
                // document from that point with nothing on screen to say why. A stray
                // marker in the prose is the smaller failure, and it matches the TS twin.
                return out + rest[open.lowerBound...]
            }
            rest = rest[close.upperBound...]
        }
        return out + rest
    }

    static func parseBlocks(_ md: String) -> [MDBlock] {
        let md = stripComments(md)
        var out: [MDBlock] = []
        var para: [String] = []
        var fence: [String]?

        func flushPara() {
            if !para.isEmpty {
                out.append(.paragraph(parseInline(para.joined(separator: " "))))
                para = []
            }
        }
        func flushAll() { flushPara() }

        let lines = md.components(separatedBy: "\n")
        var i = 0
        while i < lines.count {
            let raw = lines[i]
            let line = raw.trimmingCharacters(in: .whitespaces)

            if var open = fence {
                if line.hasPrefix("```") || line.hasPrefix("~~~") {
                    out.append(.code(open.joined(separator: "\n")))
                    fence = nil
                } else {
                    open.append(raw)
                    fence = open
                }
                i += 1
                continue
            }
            if line.hasPrefix("```") || line.hasPrefix("~~~") {
                flushAll()
                fence = []
                i += 1
                continue
            }
            if line.isEmpty {
                flushAll()
                i += 1
                continue
            }
            if let h = heading(line) {
                flushAll()
                out.append(.heading(level: h.level, spans: parseInline(h.text)))
                i += 1
                continue
            }
            if line.hasPrefix(">") {
                flushAll()
                out.append(.quote(parseInline(String(line.dropFirst()).trimmingCharacters(in: .whitespaces))))
                i += 1
                continue
            }
            if isRule(line) {
                flushAll()
                out.append(.rule)
                i += 1
                continue
            }
            if bullet(line) != nil || ordered(line) != nil {
                flushAll()
                // One branch for both kinds, because what was broken is shared: an item's
                // continuation lines used to end the list and become a paragraph.
                let isOrdered = ordered(line) != nil
                let start = isOrdered ? (ordered(line)?.number ?? 1) : 1
                var items: [[MDInline]] = []
                while i < lines.count {
                    let l = lines[i].trimmingCharacters(in: .whitespaces)
                    let head: String?
                    if isOrdered { head = ordered(l)?.text } else { head = bullet(l) }
                    guard let first = head else { break }
                    var parts = [first]
                    i += 1
                    while i < lines.count, isLazyContinuation(lines, i) {
                        parts.append(lines[i].trimmingCharacters(in: .whitespaces))
                        i += 1
                    }
                    items.append(parseInline(parts.joined(separator: " ")))
                }
                out.append(isOrdered ? .ordered(items, start: start) : .bullets(items))
                continue
            }
            // A table needs its separator row to be one: `| a | b |` on its own is a
            // paragraph that happens to contain pipes.
            if line.hasPrefix("|"), i + 1 < lines.count, isTableSeparator(lines[i + 1]) {
                flushAll()
                let header = cells(line)
                var rows: [[[MDInline]]] = []
                var j = i + 2
                while j < lines.count, lines[j].trimmingCharacters(in: .whitespaces).hasPrefix("|") {
                    rows.append(cells(lines[j].trimmingCharacters(in: .whitespaces)))
                    j += 1
                }
                out.append(.table(header: header, rows: rows))
                i = j
                continue
            }
            para.append(line)
            i += 1
        }
        if let open = fence {
            // An UNCLOSED fence is truncated output, which is exactly when a reader most
            // wants what there is. The document's trailing blank lines are not part of it.
            var body = open
            while let last = body.last, last.trimmingCharacters(in: .whitespaces).isEmpty { body.removeLast() }
            out.append(.code(body.joined(separator: "\n")))
        }
        flushAll()
        return out
    }

    private static func heading(_ line: String) -> (level: Int, text: String)? {
        var n = 0
        var idx = line.startIndex
        while idx < line.endIndex, line[idx] == "#", n < 6 {
            n += 1
            idx = line.index(after: idx)
        }
        guard n > 0, idx < line.endIndex, line[idx] == " " else { return nil }
        return (n, String(line[line.index(after: idx)...]).trimmingCharacters(in: .whitespaces))
    }

    private static func bullet(_ line: String) -> String? {
        for marker in ["- ", "* ", "• "] where line.hasPrefix(marker) {
            return String(line.dropFirst(marker.count))
        }
        return nil
    }

    /// `12. text` → (12, "text"); anything else nil. Same shape as the phone's ORDERED regex.
    private static func ordered(_ line: String) -> (number: Int, text: String)? {
        var idx = line.startIndex
        var digits = ""
        while idx < line.endIndex, line[idx].isNumber, digits.count < 9 {
            digits.append(line[idx])
            idx = line.index(after: idx)
        }
        guard !digits.isEmpty, idx < line.endIndex, line[idx] == "." else { return nil }
        idx = line.index(after: idx)
        guard idx < line.endIndex, line[idx] == " " else { return nil }
        return (Int(digits) ?? 1, String(line[idx...]).trimmingCharacters(in: .whitespaces))
    }

    /// A line that starts no block of its own belongs to the list item above it (blank
    /// lines, fences, rules, headings, quotes, new items and tables end the item).
    private static func isLazyContinuation(_ lines: [String], _ i: Int) -> Bool {
        let line = lines[i].trimmingCharacters(in: .whitespaces)
        if line.isEmpty || line.hasPrefix("```") || line.hasPrefix("~~~") { return false }
        if isRule(line) || heading(line) != nil || line.hasPrefix(">") { return false }
        if bullet(line) != nil || ordered(line) != nil { return false }
        if line.hasPrefix("|"), i + 1 < lines.count, isTableSeparator(lines[i + 1]) { return false }
        return true
    }

    private static func isRule(_ line: String) -> Bool {
        let set = Set(line)
        return line.count >= 3 && (set == ["-"] || set == ["*"] || set == ["_"])
    }

    private static func isTableSeparator(_ raw: String) -> Bool {
        let line = raw.trimmingCharacters(in: .whitespaces)
        guard line.contains("-"), line.hasPrefix("|") || line.hasPrefix(":") || line.hasPrefix("-") else { return false }
        return line.allSatisfy { "|-: ".contains($0) }
    }

    private static func cells(_ line: String) -> [[MDInline]] {
        var body = line
        if body.hasPrefix("|") { body.removeFirst() }
        if body.hasSuffix("|") { body.removeLast() }
        return body.components(separatedBy: "|").map { parseInline($0.trimmingCharacters(in: .whitespaces)) }
    }
}
