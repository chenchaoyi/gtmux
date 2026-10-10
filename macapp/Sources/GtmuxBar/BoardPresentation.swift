import Foundation

/// Presentation labels never rewrite HQ's source headings or expansion identifiers.
enum BoardPresentation {
    static func isSessions(_ title: String) -> Bool {
        ["① Now — live panes", "① Now", "① 现状 — 在跑的 pane", "① 现状"].contains(title)
    }

    static func title(_ source: String, zh: Bool) -> String {
        if isSessions(source) { return zh ? "会话概览" : "Sessions" }
        if ["② Handoff log — newest first", "② Handoff log", "② 交接记录 — 新的在最上面", "② 交接记录"].contains(source) {
            return zh ? "交接记录" : "Handoff log"
        }
        if isAskHeading(source) { return zh ? "待你处理" : "Needs your input" }
        return source
    }

    static func hasContent(_ section: BoardSection) -> Bool {
        !Markdown.parseBlocks(section.body).isEmpty || section.children.contains(where: hasContent)
    }

    /// The attention band owns its body; retain any nested entries in the outline.
    static func attention(_ sections: [BoardSection]) -> BoardSection? {
        sections.flatMap { [$0] + $0.children }.first {
            isAskHeading($0.title) && !Markdown.parseBlocks($0.body).isEmpty
        }
    }

    static func visible(_ sections: [BoardSection], liftedID: String?) -> [BoardSection] {
        sections.filter { section in
            hasContent(section) && (section.id != liftedID || section.children.contains(where: hasContent))
        }
    }

    static func count(_ section: BoardSection, children: [BoardSection]) -> Int? {
        if let own = BoardOutline.countOwn(section.body) { return own }
        return children.isEmpty ? nil : children.count
    }

    struct BodyGroup: Equatable {
        let notes: Bool
        let blocks: [MDBlock]
    }

    /// Only unlabelled blocks following the sessions table become HQ notes. Keep
    /// author headings visible, and preserve every block in its original order.
    static func body(_ section: BoardSection) -> [BodyGroup] {
        let blocks = Markdown.parseBlocks(section.body)
        guard isSessions(section.title) else { return [BodyGroup(notes: false, blocks: blocks)] }
        var groups: [BodyGroup] = []
        var afterTable = false
        var explicitlyLabelled = false
        for block in blocks {
            let notes: Bool
            switch block {
            case .table:
                afterTable = true
                explicitlyLabelled = false
                notes = false
            case .heading:
                explicitlyLabelled = true
                notes = false
            default:
                notes = afterTable && !explicitlyLabelled
            }
            if let last = groups.last, last.notes == notes {
                groups[groups.count - 1] = BodyGroup(notes: notes, blocks: last.blocks + [block])
            } else {
                groups.append(BodyGroup(notes: notes, blocks: [block]))
            }
        }
        return groups
    }
}
