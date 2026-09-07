import SwiftUI

// BoardOutlineView — the board read as an outline at the Mac, matching the phone.
//
// The Mac showed the whole document as one scroll while the phone had already learned
// three times that the board is an ARCHIVE, not a card. Reading it should not feel like
// two different products depending on which screen you opened, so the structure, the
// defaults and the words are the phone's:
//
//   - `##` sections and their `###` entries, both collapsible
//   - the FIRST section opens, because HQ pins a "read this first" handoff at the top
//   - a count bubble saying what a section HOLDS
//   - a pane row folds, closed by default, carrying its first field so you can tell
//     which row is which
//
// The one thing deliberately NOT shared is the seeding mechanism: the phone re-seeds on
// a poll and had to be stopped from doing it, whereas this view is built once per open.

struct BoardOutlineView: View {
    let markdown: String
    let p: Theme.Palette

    @State private var open: Set<String> = []
    @State private var seeded = false

    var body: some View {
        let sections = BoardOutline.parse(markdown)
        ScrollView {
            LazyVStack(alignment: .leading, spacing: 0) {
                ForEach(sections) { sec in
                    sectionRow(sec, sections: sections)
                }
            }
            // Trailing room for the scroll indicator, which sat on top of the count
            // bubble and clipped it against the window edge.
            .padding(.leading, 14)
            .padding(.trailing, 20)
            .padding(.vertical, 8)
        }
        .onAppear {
            guard !seeded, let first = sections.first(where: { !$0.title.isEmpty }) else { return }
            seeded = true
            open = [first.id]
        }
    }

    @ViewBuilder private func sectionRow(_ sec: BoardSection, sections: [BoardSection]) -> some View {
        let shown = open.contains(sec.id) || sec.title.isEmpty
        VStack(alignment: .leading, spacing: 0) {
            if !sec.title.isEmpty {
                Button {
                    toggle(sec.id)
                } label: {
                    HStack(alignment: .firstTextBaseline, spacing: 8) {
                        Text(shown ? "▾" : "▸")
                            .font(.system(size: 11))
                            .foregroundStyle(p.fg3)
                            .frame(width: 11, alignment: .leading)
                        Text(sec.title)
                            .font(.system(size: 13.5, weight: .semibold))
                            .foregroundStyle(p.fg)
                            .lineLimit(shown ? nil : 2)
                            .multilineTextAlignment(.leading)
                        Spacer(minLength: 6)
                        if let n = BoardOutline.count(sec) {
                            Text("\(n)")
                                .font(.system(size: 10.5).monospacedDigit())
                                .foregroundStyle(p.fg3)
                                .padding(.horizontal, 5)
                                .padding(.vertical, 1)
                                .overlay(RoundedRectangle(cornerRadius: 8).stroke(p.divider, lineWidth: 1))
                        }
                    }
                    .contentShape(Rectangle())
                    .padding(.vertical, 9)
                }
                .buttonStyle(.plain)
                Divider().overlay(p.divider)
            }
            if shown {
                if !sec.body.isEmpty {
                    MarkdownBlocks(blocks: Markdown.parseBlocks(sec.body), p: p, spacing: 9, foldRows: true)
                        .padding(.bottom, 8)
                }
                ForEach(sec.children) { kid in
                    entryRow(kid)
                }
            }
        }
    }

    @ViewBuilder private func entryRow(_ kid: BoardSection) -> some View {
        let shown = open.contains(kid.id)
        VStack(alignment: .leading, spacing: 0) {
            Divider().overlay(p.divider)
            Button {
                toggle(kid.id)
            } label: {
                HStack(alignment: .firstTextBaseline, spacing: 8) {
                    Text(shown ? "▾" : "▸")
                        .font(.system(size: 11))
                        .foregroundStyle(p.fg3)
                        .frame(width: 11, alignment: .leading)
                    Text(kid.title)
                        .font(.system(size: 12.5, weight: .medium))
                        .foregroundStyle(p.fg2)
                        .lineLimit(shown ? nil : 2)
                        .multilineTextAlignment(.leading)
                    Spacer(minLength: 6)
                }
                .contentShape(Rectangle())
                .padding(.vertical, 7)
            }
            .buttonStyle(.plain)
            if shown, !kid.body.isEmpty {
                MarkdownBlocks(blocks: Markdown.parseBlocks(kid.body), p: p, spacing: 9, foldRows: true)
                    .padding(.bottom, 8)
            }
        }
        // An entry belongs to the section above it, and the indent is what says so.
        .padding(.leading, 18)
    }

    private func toggle(_ id: String) {
        if open.contains(id) { open.remove(id) } else { open.insert(id) }
    }
}

/// One stacked field of a table row: a column heading and its cell.
struct MDField: Equatable {
    let label: String
    let value: [MDInline]
}

/// stackFields pairs a row's cells with their column headings, dropping the empty ones.
///
/// Half the board's cells are `—` or blank, and a label with nothing after it is noise —
/// the same rule the phone's `stackRows` applies, so a row is the same height wherever it
/// is read.
func stackFields(header: [[MDInline]], row: [[MDInline]]) -> [MDField] {
    row.dropFirst().enumerated().compactMap { i, cell in
        let text = mdPlain(cell).trimmingCharacters(in: .whitespaces)
        guard !text.isEmpty, text != "—", text != "-" else { return nil }
        return MDField(label: mdPlain(header.count > i + 1 ? header[i + 1] : [])
            .trimmingCharacters(in: .whitespaces), value: cell)
    }
}

/// What a CLOSED row shows beside its head, so a reader can find the one they want
/// without opening any of them: the FIRST field, and only the first. In a stacked table
/// the leading column after the identity is the one saying WHICH thing this is (the board
/// puts `loc` there, beside the pane id). The rest are its contents, which is what
/// folding is hiding.
func rowSubtitle(header: [[MDInline]], row: [[MDInline]]) -> String {
    stackFields(header: header, row: row).first.map { mdPlain($0.value) } ?? ""
}

/// One table row as a card, foldable.
///
/// Closed by default when folding, and nothing seeds one open: unlike the board's pinned
/// first SECTION, no row here is the one you came for — the point is to see them all at
/// once.
struct TableCard: View {
    let header: [[MDInline]]
    let row: [[MDInline]]
    let p: Theme.Palette
    let fold: Bool
    let index: Int

    @State private var open = false

    var body: some View {
        let shut = fold && !open
        VStack(alignment: .leading, spacing: 5) {
            if fold {
                Button {
                    open.toggle()
                } label: {
                    HStack(alignment: .firstTextBaseline, spacing: 7) {
                        Text(shut ? "▸" : "▾")
                            .font(.system(size: 10.5))
                            .foregroundStyle(p.fg3)
                            .frame(width: 10, alignment: .leading)
                        mdSpansText(row.first ?? [], size: 12.5, weight: .semibold, p: p)
                            .lineLimit(1)
                        let sub = rowSubtitle(header: header, row: row)
                        if !sub.isEmpty {
                            Text(sub)
                                .font(.system(size: 10.5))
                                .foregroundStyle(p.fg3)
                                .lineLimit(1)
                        }
                        Spacer(minLength: 0)
                    }
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
            } else if let first = row.first {
                mdSpansText(first, size: 12.5, weight: .semibold, p: p)
            }
            if !shut {
                ForEach(Array(stackFields(header: header, row: row).enumerated()), id: \.offset) { _, f in
                    HStack(alignment: .firstTextBaseline, spacing: 9) {
                        Text(f.label)
                            .font(.system(size: 10.5))
                            .foregroundStyle(p.fg3)
                            .frame(width: 62, alignment: .leading)
                        mdSpansText(f.value, size: 11.5, weight: .regular, p: p)
                    }
                }
            }
        }
        // A CLOSED row is a row, not a card. The card exists to hold a block of labelled
        // fields; with the fields hidden it is chrome around one short line, and thirteen
        // of them read as a pile of boxes rather than a list you can scan. Open, the card
        // comes back and says where the block begins and ends.
        .padding(.horizontal, shut ? 2 : 11)
        .padding(.vertical, shut ? 5 : 11)
        .frame(maxWidth: .infinity, alignment: .leading)
        .overlay(alignment: .bottom) {
            if shut { Rectangle().fill(p.divider).frame(height: 1) }
        }
        .overlay {
            if !shut { RoundedRectangle(cornerRadius: 10).strokeBorder(p.divider, lineWidth: 1) }
        }
        // Outside the border: an open card needs air from the rows it sits between.
        .padding(.vertical, shut ? 0 : 5)
    }
}
