import SwiftUI

/// The blocks of a document, without a scroll view of its own — for a body that is
/// already inside one (a knowledge entry sits under its own title and banner).
struct MarkdownBody: View {
    let markdown: String
    let p: Theme.Palette

    var body: some View {
        MarkdownBlocks(blocks: Markdown.parseBlocks(markdown), p: p, spacing: 8)
    }
}

/// The blocks themselves, built as they scroll into view.
///
/// Lazy is not a nicety here: laying out a whole document at once is the bug this reader
/// shipped with once (34 KB in a single SwiftUI `Text`, and switching to the tab hung).
/// How long a paragraph may run before it is folded, and how much shows while folded.
/// Matches the phone's `PROSE_CLAMP_CHARS`, so the same board reads the same way on both.
let MDProseClampChars = 220
let MDProseClampLines = 4

/// The visible length of a run of spans — what a reader actually faces.
func mdPlainLength(_ spans: [MDInline]) -> Int {
    var n = 0
    for s in spans {
        switch s {
        case .text(let t), .code(let t), .bold(let t), .link(let t): n += t.count
        default: break
        }
    }
    return n
}

struct MarkdownBlocks: View {
    let blocks: [MDBlock]
    let p: Theme.Palette
    let spacing: CGFloat
    /// Each table row is a COLLAPSIBLE card, closed by default — for a table whose cells
    /// are paragraphs. The board's pane table is the case: one pane's status cell runs
    /// to several screens, so thirteen panes open is a document nobody reaches the end
    /// of. Opt-in, matching the phone: a knowledge entry's table is small and part of a
    /// sentence, and folding it would hide the answer.
    var foldRows: Bool = false
    /// A paragraph past `MDProseClampChars` renders as a few lines with a disclosure.
    /// For a surface whose author is a MACHINE: HQ writes the board, and one cell there
    /// was measured at ~1,180 characters — a single semicolon-joined investigation log —
    /// which arrives as a wall of text (2026-09-08). The reader cannot fix the writing;
    /// it must not hand the whole wall over at once. Opt-in, matching the phone: a
    /// knowledge entry is prose written FOR a reader, and folding it would hide the
    /// thing they opened.
    var clampProse: Bool = false
    /// Only situation-board prose links pane identifiers; knowledge documents stay literal.
    var paneLinks: Bool = false
    @State private var openProse: Set<Int> = []

    var body: some View {
        LazyVStack(alignment: .leading, spacing: spacing) {
            ForEach(Array(blocks.enumerated()), id: \.offset) { i, b in
                if clampProse, case .paragraph(let spans) = b, mdPlainLength(spans) > MDProseClampChars {
                    let open = openProse.contains(i)
                    VStack(alignment: .leading, spacing: 2) {
                        spansText(spans, size: 12, weight: .regular)
                            .lineLimit(open ? nil : MDProseClampLines)
                        Button {
                            if open { openProse.remove(i) } else { openProse.insert(i) }
                        } label: {
                            HStack(spacing: 4) {
                                BoardDisclosureGlyph(expanded: open, p: p)
                                Text(
                                    L10n.shared.tr(open ? "Collapse text" : "Read full text", open ? "收起全文" : "展开全文")
                                )
                                .font(.system(size: 11.5)).foregroundStyle(p.fg2)
                            }
                            .contentShape(Rectangle())
                        }
                        .buttonStyle(.plain)
                    }
                } else {
                    block(b)
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    @ViewBuilder private func block(_ b: MDBlock) -> some View {
        switch b {
        case .heading(let level, let spans):
            spansText(spans, size: level == 1 ? 17 : level == 2 ? 14.5 : 12.5, weight: .semibold)
                .padding(.top, level >= 3 ? 8 : 12)
        case .paragraph(let spans):
            spansText(spans, size: 12, weight: .regular)
        case .bullets(let items):
            VStack(alignment: .leading, spacing: 4) {
                ForEach(Array(items.enumerated()), id: \.offset) { _, item in
                    HStack(alignment: .firstTextBaseline, spacing: 7) {
                        Text("·").foregroundStyle(p.fg3)
                        spansText(item, size: 12, weight: .regular)
                    }
                }
            }
        case .ordered(let items, let start):
            // The number is the item's identity on the board ("定第 5 条"), so it is a
            // label in the margin, tabular so a two-digit list keeps one edge.
            VStack(alignment: .leading, spacing: 4) {
                ForEach(Array(items.enumerated()), id: \.offset) { i, item in
                    HStack(alignment: .firstTextBaseline, spacing: 7) {
                        Text("\(start + i).")
                            .font(.system(size: 12).monospacedDigit())
                            .foregroundStyle(p.fg3)
                            .frame(minWidth: 18, alignment: .trailing)
                        spansText(item, size: 12, weight: .regular)
                    }
                }
            }
        case .code(let text):
            Text(text)
                .font(.system(size: 11.5, design: .monospaced))
                .foregroundStyle(p.fg)
                .textSelection(.enabled)
                .padding(9)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(RoundedRectangle(cornerRadius: 8).fill(p.rowSelected))
        case .quote(let spans):
            HStack(spacing: 8) {
                Rectangle().fill(p.divider).frame(width: 2)
                spansText(spans, size: 12, weight: .regular)
            }
        case .table(let header, let rows):
            // No gap between CLOSED rows: they are separated by their own hairline, and
            // a gap as well would put each one back in a box of white space.
            VStack(alignment: .leading, spacing: foldRows ? 0 : 8) {
                ForEach(Array(rows.enumerated()), id: \.offset) { i, row in
                    TableCard(header: header, row: row, p: p, fold: foldRows, index: i, paneLinks: paneLinks)
                }
            }
        case .rule:
            Rectangle().fill(p.divider).frame(height: 1).padding(.vertical, 4)
        }
    }

    /// One table row as a card: its first cell is the handle, the rest are labelled.
    /// Inline runs as one selectable Text: code in the monospace face the rest of the
    /// product uses for identifiers, bold at a weight that does not compete with a heading
    /// (the board carries roughly one bold span per line).
    private func spansText(_ spans: [MDInline], size: CGFloat, weight: Font.Weight) -> Text {
        mdSpansText(spans, size: size, weight: weight, p: p, paneLinks: paneLinks)
    }

    private func plain(_ spans: [MDInline]) -> String { mdPlain(spans) }
}

/// Inline runs as one selectable Text: code in the monospace face the rest of the product
/// uses for identifiers, bold at a weight that does not compete with a heading (the board
/// carries roughly one bold span per line).
///
/// File scope because two views render spans now — the document blocks and the folding
/// table card — and a second copy is how the two would start to look different.
func mdSpansText(
    _ spans: [MDInline], size: CGFloat, weight: Font.Weight, p: Theme.Palette, paneLinks: Bool = false
) -> Text {
    func inline(_ text: String) -> Text {
        paneLinks ? Text(BoardPaneReference.attributed(text)) : Text(text)
    }
    return spans.reduce(Text("")) { acc, span in
        switch span {
        case .text(let t):
            return acc + inline(t).font(.system(size: size, weight: weight)).foregroundColor(p.fg)
        case .code(let t):
            return acc
                + inline(t).font(.system(size: size - 0.5, design: .monospaced)).foregroundColor(p.fg2)
        case .bold(let t):
            return acc + inline(t).font(.system(size: size, weight: .semibold)).foregroundColor(p.fg)
        case .link(let t):
            // An edge in the knowledge graph. Styled as one and stripped of its brackets:
            // the reader is looking at a reference, not at markup.
            return acc + inline(t).font(.system(size: size - 0.5)).foregroundColor(Theme.Status.working)
        }
    }
}

func mdPlain(_ spans: [MDInline]) -> String {
    spans.map { span in
        switch span {
        case .text(let t): return t
        case .code(let t): return t
        case .bold(let t): return t
        case .link(let t): return t
        }
    }.joined()
}
