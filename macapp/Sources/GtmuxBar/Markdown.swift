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

enum MDInline: Equatable {
    case text(String)
    case code(String)
    case bold(String)
}

enum MDBlock: Equatable {
    case heading(level: Int, spans: [MDInline])
    case paragraph([MDInline])
    case bullets([[MDInline]])
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
    static func parseBlocks(_ md: String) -> [MDBlock] {
        var out: [MDBlock] = []
        var para: [String] = []
        var bullets: [[MDInline]] = []
        var fence: [String]?

        func flushPara() {
            if !para.isEmpty {
                out.append(.paragraph(parseInline(para.joined(separator: " "))))
                para = []
            }
        }
        func flushBullets() {
            if !bullets.isEmpty {
                out.append(.bullets(bullets))
                bullets = []
            }
        }
        func flushAll() {
            flushPara()
            flushBullets()
        }

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
            if let item = bullet(line) {
                flushPara()
                bullets.append(parseInline(item))
                i += 1
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
            flushBullets()
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
