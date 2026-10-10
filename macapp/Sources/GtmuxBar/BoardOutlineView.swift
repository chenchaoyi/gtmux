import SwiftUI

// The source outline and author order match the phone. Mac presentation labels
// distinguish the sessions table, supplemental HQ notes and handoff entries.

struct BoardOutlineView: View {
    let markdown: String
    let p: Theme.Palette
    let l10n: L10n

    @State private var openingPane = false
    @State private var focusError: String?
    @State private var open: Set<String> = []
    @State private var seeded = false

    var body: some View {
        let sections = BoardOutline.parse(markdown)
        let ask = BoardPresentation.attention(sections)
        let visible = BoardPresentation.visible(sections, liftedID: ask?.id)
        ScrollView {
            LazyVStack(alignment: .leading, spacing: 0) {
                if let ask {
                    askBand(ask, p)
                }
                ForEach(visible) { sec in
                    sectionRow(sec, liftedID: ask?.id)
                }
            }
            // Trailing room for the scroll indicator, which sat on top of the count
            // bubble and clipped it against the window edge.
            .padding(.leading, 14)
            .padding(.trailing, 20)
            .padding(.vertical, 8)
        }
        .overlay(alignment: .topTrailing) {
            if openingPane {
                ProgressView().controlSize(.small).padding(8)
                    .accessibilityLabel(l10n.tr("Opening pane", "正在打开窗格"))
            }
        }
        .environment(\.openURL, OpenURLAction { url in
            guard let pane = BoardPaneReference.target(url) else { return .discarded }
            focus(pane)
            return .handled
        })
        .alert(l10n.tr("Unable to open pane", "无法打开窗格"), isPresented: Binding(
            get: { focusError != nil }, set: { if !$0 { focusError = nil } }
        )) {
            Button(l10n.tr("OK", "好"), role: .cancel) { focusError = nil }
        } message: {
            Text(focusError ?? "")
        }
        .onAppear {
            guard !seeded, let first = visible.first(where: { !$0.title.isEmpty }) else { return }
            seeded = true
            open = [first.id]
        }
    }

    /// The lifted section: HQ's own words, under a heading in the attention colour.
    @ViewBuilder private func askBand(_ sec: BoardSection, _ p: Theme.Palette) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 6) {
                Image(systemName: "exclamationmark.circle.fill")
                    .font(.system(size: 12)).foregroundStyle(Theme.Status.waiting)
                Text(BoardPresentation.title(sec.title, zh: l10n.lang == "zh"))
                    .font(.system(size: 12.5, weight: .semibold))
                    .foregroundStyle(Theme.Status.waiting)
                Spacer()
            }
            MarkdownBlocks(blocks: Markdown.parseBlocks(sec.body), p: p, spacing: 7, paneLinks: true)
        }
        .padding(.horizontal, 11)
        .padding(.vertical, 9)
        .background(RoundedRectangle(cornerRadius: 9).fill(Theme.Status.waiting.opacity(0.07)))
        .overlay(RoundedRectangle(cornerRadius: 9).stroke(Theme.Status.waiting.opacity(0.28), lineWidth: 1))
        .padding(.bottom, 10)
    }

    @ViewBuilder private func sectionRow(_ sec: BoardSection, liftedID: String?) -> some View {
        let shown = open.contains(sec.id) || sec.title.isEmpty
        let children = BoardPresentation.visible(sec.children, liftedID: liftedID)
        let title = BoardPresentation.title(sec.title, zh: l10n.lang == "zh")
        VStack(alignment: .leading, spacing: 0) {
            if !sec.title.isEmpty {
                Button {
                    toggle(sec.id)
                } label: {
                    HStack(alignment: .center, spacing: 8) {
                        BoardDisclosureGlyph(expanded: shown, p: p)
                        Text(title)
                            .font(.system(size: 13.5, weight: .semibold))
                            .foregroundStyle(p.fg)
                            .lineLimit(shown ? nil : 2)
                            .multilineTextAlignment(.leading)
                        Spacer(minLength: 6)
                        if let n = BoardPresentation.count(sec, children: children) {
                            Text("\(n)")
                                .font(.system(size: 10.5).monospacedDigit())
                                .foregroundStyle(p.fg2)
                                .padding(.horizontal, 5)
                                .padding(.vertical, 1)
                                .overlay(RoundedRectangle(cornerRadius: 8).stroke(p.divider, lineWidth: 1))
                        }
                    }
                    .contentShape(Rectangle())
                    .frame(minHeight: 44)
                }
                .buttonStyle(.plain)
                .help(BoardPresentation.isSessions(sec.title)
                      ? l10n.tr("Sessions recorded in HQ’s latest board update.", "HQ 最近一次更新时记录的会话。")
                      : title == l10n.tr("Handoff log", "交接记录")
                        ? l10n.tr("Notes retained for HQ handoff and continuity.", "HQ 保留的交接和工作记录。") : title)
                .accessibilityValue(l10n.tr(shown ? "Expanded" : "Collapsed", shown ? "已展开" : "已收起"))
                Divider().overlay(p.divider)
            }
            if shown {
                if sec.id != liftedID {
                    ForEach(Array(BoardPresentation.body(sec).enumerated()), id: \.offset) { i, group in
                        if group.notes {
                            notesRow(group.blocks, id: "\(sec.id):notes:\(i)")
                                .padding(.leading, 18)
                        } else {
                            MarkdownBlocks(blocks: group.blocks, p: p, spacing: 9, foldRows: true, clampProse: true, paneLinks: true)
                                .padding(.bottom, 8)
                        }
                    }
                }
                ForEach(children) { kid in
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
                HStack(alignment: .center, spacing: 8) {
                    BoardDisclosureGlyph(expanded: shown, p: p)
                    Text(BoardPresentation.title(kid.title, zh: l10n.lang == "zh"))
                        .font(.system(size: 12.5, weight: .medium))
                        .foregroundStyle(p.fg2)
                        .lineLimit(shown ? nil : 2)
                        .multilineTextAlignment(.leading)
                    Spacer(minLength: 6)
                }
                .contentShape(Rectangle())
                .frame(minHeight: 40)
            }
            .buttonStyle(.plain)
            .accessibilityValue(l10n.tr(shown ? "Expanded" : "Collapsed", shown ? "已展开" : "已收起"))
            if shown, !kid.body.isEmpty {
                MarkdownBlocks(blocks: Markdown.parseBlocks(kid.body), p: p, spacing: 9, foldRows: true, clampProse: true, paneLinks: true)
                    .padding(.bottom, 8)
            }
        }
        // An entry belongs to the section above it, and the indent is what says so.
        .padding(.leading, 18)
    }

    private func notesRow(_ blocks: [MDBlock], id: String) -> some View {
        let shown = open.contains(id)
        return VStack(alignment: .leading, spacing: 0) {
            Button { toggle(id) } label: {
                HStack(spacing: 8) {
                    BoardDisclosureGlyph(expanded: shown, p: p)
                    Text(l10n.tr("HQ notes", "HQ 备注"))
                        .font(.system(size: 12.5, weight: .medium)).foregroundStyle(p.fg2)
                    Spacer(minLength: 0)
                }
                .frame(minHeight: 40).contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .help(l10n.tr("Supplemental handoff and diagnostic notes from HQ.", "HQ 记录的交接和检查情况等补充信息。"))
            .accessibilityValue(l10n.tr(shown ? "Expanded" : "Collapsed", shown ? "已展开" : "已收起"))
            if shown {
                MarkdownBlocks(blocks: blocks, p: p, spacing: 9, foldRows: true, paneLinks: true)
                    .padding(.leading, 40).padding(.bottom, 12)
            }
        }
    }

    private func focus(_ pane: String) {
        guard !openingPane else { return }
        openingPane = true
        let zh = l10n.lang == "zh"
        DispatchQueue.global(qos: .userInitiated).async {
            let result = GtmuxCLI.captureResult(["focus", pane])
            DispatchQueue.main.async {
                openingPane = false
                focusError = BoardPaneReference.failureMessage(
                    pane: pane, status: result.status, stderr: result.stderr, zh: zh)
            }
        }
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
    var paneLinks: Bool = false

    @State private var open = false

    private var disclosureLabel: String {
        let verb = open ? L10n.shared.tr("Collapse", "收起") : L10n.shared.tr("Expand", "展开")
        return "\(verb) \(mdPlain(row.first ?? []))"
    }

    var body: some View {
        let shut = fold && !open
        VStack(alignment: .leading, spacing: 5) {
            if fold {
                HStack(alignment: .center, spacing: 7) {
                    Button { open.toggle() } label: {
                        BoardDisclosureGlyph(expanded: !shut, p: p)
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel(Text(disclosureLabel))
                    .accessibilityValue(L10n.shared.tr(open ? "Expanded" : "Collapsed", open ? "已展开" : "已收起"))
                    mdSpansText(row.first ?? [], size: 12.5, weight: .semibold, p: p, paneLinks: paneLinks)
                        .lineLimit(1)
                    Button { open.toggle() } label: {
                        let sub = rowSubtitle(header: header, row: row)
                        HStack {
                            Text(sub)
                                .font(.system(size: 11.5))
                                .foregroundStyle(p.fg2)
                                .lineLimit(1)
                            Spacer(minLength: 0)
                        }
                        .frame(minHeight: 32)
                        .contentShape(Rectangle())
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel(Text(disclosureLabel))
                    .accessibilityValue(L10n.shared.tr(open ? "Expanded" : "Collapsed", open ? "已展开" : "已收起"))
                }
            } else if let first = row.first {
                mdSpansText(first, size: 12.5, weight: .semibold, p: p, paneLinks: paneLinks)
            }
            if !shut {
                ForEach(Array(stackFields(header: header, row: row).enumerated()), id: \.offset) { _, f in
                    HStack(alignment: .firstTextBaseline, spacing: 9) {
                        Text(f.label)
                            .font(.system(size: 10.5))
                            .foregroundStyle(p.fg3)
                            .frame(width: 62, alignment: .leading)
                        mdSpansText(f.value, size: 11.5, weight: .regular, p: p, paneLinks: paneLinks)
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

/// Shared shape and contrast for the board's section, entry and pane controls.
struct BoardDisclosureGlyph: View {
    let expanded: Bool
    let p: Theme.Palette

    var body: some View {
        Image(systemName: expanded ? "chevron.down" : "chevron.right")
            .font(.system(size: 13, weight: .semibold))
            .foregroundStyle(p.fg2)
            .frame(width: 32, height: 32)
            .contentShape(Rectangle())
            .accessibilityHidden(true)
    }
}
