import AppKit
import Combine
import SwiftUI

// HQReader — the supervisor's two memories, readable at the Mac.
//
// DESIGN §12 said the menu-bar HQ card jumps to the pane and nothing else, because "the
// command centre lives on the phone and the web". That rule is about DRIVING the fleet —
// send, spawn, deciding — and it still holds: nothing here dispatches anything.
//
// Reading is a different act, and the Mac is where it is most wanted: the commander is
// already sitting at the machine whose situation the board describes. So the card gains
// two READERS and no controls. The knowledge base's own actions (land, retire) stay where
// they are — on the phone, and in the CLI two keystrokes away — so the red line between
// "look at HQ's memory" and "act through HQ" stays where DESIGN put it.
//
// A window, not a popover panel: the real board on this machine is 58 KB of markdown, and
// a document that long inside a 380pt popover is a worse answer than no answer. Same shape
// as the pane browser, for the same reason.

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

    enum CodingKeys: String, CodingKey {
        case id, topic, title, at
        case promotedAt, landedAt, promoteWhy, promoteTarget
    }

    /// Promoted and not yet carried: the one part of the knowledge lifecycle that waits on
    /// a person.
    var pending: Bool { (promotedAt ?? 0) > 0 && (landedAt ?? 0) == 0 }
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

final class HQReaderStore: ObservableObject {
    @Published private(set) var board: BoardDoc?
    @Published private(set) var entries: [KBEntry] = []
    @Published private(set) var loading = true

    private var timer: Timer?

    func start() {
        refresh()
        timer?.invalidate()
        // Slow: a board is a human synthesis rewritten a few times an hour, and the ledger
        // changes when HQ distills. Polling either one every second would spend a process
        // spawn to re-read the same document.
        timer = Timer.scheduledTimer(withTimeInterval: 20, repeats: true) { [weak self] _ in self?.refresh() }
    }

    func stop() {
        timer?.invalidate()
        timer = nil
    }

    func refresh() {
        DispatchQueue.global(qos: .userInitiated).async {
            var doc: BoardDoc?
            if let d = GtmuxCLI.capture(["hq", "--board", "--json"]) {
                doc = try? JSONDecoder().decode(BoardDoc.self, from: d)
            }
            var rows: [KBEntry] = []
            if let d = GtmuxCLI.capture(["knowledge", "list", "--json"]) {
                rows = (try? JSONDecoder().decode([KBEntry].self, from: d)) ?? []
            }
            let newest = rows.reversed().map { $0 } // newest first, as every surface shows them
            DispatchQueue.main.async {
                // Publish only what CHANGED. Every assignment re-renders the window, and
                // the poll re-reads the same two documents every 20 seconds — a board that
                // has not been rewritten would otherwise throw away the reader's scroll
                // position and selection for nothing.
                if self.board?.updatedAt != doc?.updatedAt || self.board?.exists != doc?.exists {
                    self.board = doc
                }
                if self.entries.map({ $0.id }) != newest.map({ $0.id }) {
                    self.entries = newest
                }
                if self.loading { self.loading = false }
            }
        }
    }

    /// Test seam: the entries normally arrive from the CLI, off the main queue.
    func setEntriesForTest(_ rows: [KBEntry]) { entries = rows }

    /// What the commander owes, oldest first — the debt, not the newest news.
    var pending: [KBEntry] {
        entries.filter { $0.pending }.sorted { ($0.promotedAt ?? 0) < ($1.promotedAt ?? 0) }
    }
}

enum HQReaderTab: String, CaseIterable {
    case board, knowledge
}

struct HQReaderView: View {
    let l10n: L10n
    @ObservedObject var store: HQReaderStore
    @State var tab: HQReaderTab
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(spacing: 0) {
            Picker("", selection: $tab) {
                Text(l10n.tr("Situation board", "态势板")).tag(HQReaderTab.board)
                Text(l10n.tr("Knowledge", "知识库")).tag(HQReaderTab.knowledge)
            }
            .pickerStyle(.segmented)
            .labelsHidden()
            .padding(10)

            Divider()

            if tab == .board {
                boardBody(p)
            } else {
                knowledgeBody(p)
            }
        }
        .frame(minWidth: 520, minHeight: 420)
        .background(p.bg)
        .onAppear { store.start() }
        .onDisappear { store.stop() }
    }

    @ViewBuilder private func boardBody(_ p: Theme.Palette) -> some View {
        if let b = store.board, b.exists, let text = b.text, !text.isEmpty {
            MarkdownDoc(markdown: text, p: p)
        } else {
            // A supervisor that has written no board is ordinary, not broken.
            empty(l10n.tr("No situation board yet — the supervisor writes one as it works",
                          "还没有态势板 —— 参谋长干着干着就会写一份"), p)
        }
    }

    @ViewBuilder private func knowledgeBody(_ p: Theme.Palette) -> some View {
        if store.entries.isEmpty {
            empty(l10n.tr("Nothing recorded yet", "还没有记录"), p)
        } else {
            ScrollView {
                // Lazy for the same reason the board is: rows are built as they are
                // needed, not all of them on every switch back to this tab.
                LazyVStack(alignment: .leading, spacing: 0) {
                    if !store.pending.isEmpty {
                        // What the commander owes leads, exactly as it does on the phone:
                        // carrying a promoted lesson somewhere durable is the only step of
                        // this lifecycle that waits on a person.
                        sectionHead(l10n.tr("waiting on you", "待你带走"), store.pending.count, p, accent: true)
                        ForEach(store.pending) { e in row(e, p, showWhy: true) }
                    }
                    sectionHead(l10n.tr("newest", "最近"), store.entries.count, p, accent: false)
                    ForEach(store.entries.prefix(60)) { e in row(e, p, showWhy: false) }
                }
                .padding(.bottom, 10)
            }
        }
    }

    @ViewBuilder private func sectionHead(_ text: String, _ n: Int, _ p: Theme.Palette, accent: Bool) -> some View {
        HStack(spacing: 6) {
            Text(text.uppercased()).font(.system(size: 9.5, weight: .semibold)).tracking(0.8)
            Text("\(n)").font(.system(size: 9.5, weight: .semibold))
            Spacer()
        }
        .foregroundStyle(accent ? Theme.Status.waiting : p.fg3)
        .padding(.horizontal, 12).padding(.top, 14).padding(.bottom, 6)
    }

    @ViewBuilder private func row(_ e: KBEntry, _ p: Theme.Palette, showWhy: Bool) -> some View {
        VStack(alignment: .leading, spacing: 3) {
            Text(e.title)
                .font(.system(size: 12))
                .foregroundStyle(p.fg)
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
            HStack(spacing: 6) {
                Text(e.topic).font(.system(size: 10)).foregroundStyle(p.fg3)
                if showWhy, let why = e.promoteWhy, !why.isEmpty {
                    Text(why).font(.system(size: 10)).foregroundStyle(p.fg2).lineLimit(1)
                }
                if showWhy, let target = e.promoteTarget, !target.isEmpty {
                    Text("→ \(target)").font(.system(size: 10)).foregroundStyle(p.fg3).lineLimit(1)
                }
                Spacer(minLength: 4)
            }
        }
        .padding(.horizontal, 12).padding(.vertical, 7)
        .overlay(alignment: .bottom) { Rectangle().fill(p.divider).frame(height: 0.5).padding(.leading, 12) }
    }

    @ViewBuilder private func empty(_ text: String, _ p: Theme.Palette) -> some View {
        VStack {
            Spacer()
            Text(text).font(.system(size: 12)).foregroundStyle(p.fg3).multilineTextAlignment(.center).padding(24)
            Spacer()
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

/// The board, rendered.
///
/// Blocks are built as they scroll into view (`LazyVStack`), which is what keeps the tab
/// switch instant: the whole document laid out at once is the bug this reader already had
/// once, in its first form as a single SwiftUI `Text`.
///
/// The board's own table is six columns of prose. No window width makes that a readable
/// grid, so a row renders as a CARD of labelled fields — the same shape the phone settled
/// on, for the same measurement, so the document reads the same wherever it is opened.
struct MarkdownDoc: View {
    let markdown: String
    let p: Theme.Palette

    var body: some View {
        let blocks = Markdown.parseBlocks(markdown)
        ScrollView {
            LazyVStack(alignment: .leading, spacing: 9) {
                ForEach(Array(blocks.enumerated()), id: \.offset) { _, b in
                    block(b)
                }
            }
            .padding(14)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    @ViewBuilder private func block(_ b: MDBlock) -> some View {
        switch b {
        case let .heading(level, spans):
            spansText(spans, size: level == 1 ? 17 : level == 2 ? 14.5 : 12.5, weight: .semibold)
                .padding(.top, level >= 3 ? 8 : 12)
        case let .paragraph(spans):
            spansText(spans, size: 12, weight: .regular)
        case let .bullets(items):
            VStack(alignment: .leading, spacing: 4) {
                ForEach(Array(items.enumerated()), id: \.offset) { _, item in
                    HStack(alignment: .firstTextBaseline, spacing: 7) {
                        Text("·").foregroundStyle(p.fg3)
                        spansText(item, size: 12, weight: .regular)
                    }
                }
            }
        case let .code(text):
            Text(text)
                .font(.system(size: 11.5, design: .monospaced))
                .foregroundStyle(p.fg)
                .textSelection(.enabled)
                .padding(9)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(RoundedRectangle(cornerRadius: 8).fill(p.rowSelected))
        case let .quote(spans):
            HStack(spacing: 8) {
                Rectangle().fill(p.divider).frame(width: 2)
                spansText(spans, size: 12, weight: .regular)
            }
        case let .table(header, rows):
            VStack(alignment: .leading, spacing: 8) {
                ForEach(Array(rows.enumerated()), id: \.offset) { _, row in
                    tableCard(header: header, row: row)
                }
            }
        case .rule:
            Rectangle().fill(p.divider).frame(height: 1).padding(.vertical, 4)
        }
    }

    /// One table row as a card: its first cell is the handle, the rest are labelled.
    @ViewBuilder private func tableCard(header: [[MDInline]], row: [[MDInline]]) -> some View {
        VStack(alignment: .leading, spacing: 5) {
            if let first = row.first {
                spansText(first, size: 12.5, weight: .semibold)
            }
            ForEach(Array(row.dropFirst().enumerated()), id: \.offset) { i, cell in
                HStack(alignment: .firstTextBaseline, spacing: 9) {
                    Text(plain(header.count > i + 1 ? header[i + 1] : []))
                        .font(.system(size: 10.5))
                        .foregroundStyle(p.fg3)
                        .frame(width: 62, alignment: .leading)
                    spansText(cell, size: 11.5, weight: .regular)
                }
            }
        }
        .padding(11)
        .frame(maxWidth: .infinity, alignment: .leading)
        .overlay(RoundedRectangle(cornerRadius: 10).strokeBorder(p.divider, lineWidth: 1))
    }

    /// Inline runs as one selectable Text: code in the monospace face the rest of the
    /// product uses for identifiers, bold at a weight that does not compete with a heading
    /// (the board carries roughly one bold span per line).
    private func spansText(_ spans: [MDInline], size: CGFloat, weight: Font.Weight) -> Text {
        spans.reduce(Text("")) { acc, span in
            switch span {
            case let .text(t):
                return acc + Text(t).font(.system(size: size, weight: weight)).foregroundColor(p.fg)
            case let .code(t):
                return acc + Text(t).font(.system(size: size - 0.5, design: .monospaced)).foregroundColor(p.fg2)
            case let .bold(t):
                return acc + Text(t).font(.system(size: size, weight: .semibold)).foregroundColor(p.fg)
            }
        }
    }

    private func plain(_ spans: [MDInline]) -> String {
        spans.map { span in
            switch span {
            case let .text(t): return t
            case let .code(t): return t
            case let .bold(t): return t
            }
        }.joined()
    }
}

/// The window, kept across closes so it reopens where the reader left it — the same shape
/// (and the same reason) as the pane browser's.
final class HQReaderController {
    static let shared = HQReaderController()
    private(set) var window: NSWindow?
    private let store = HQReaderStore()

    func show(l10n: L10n, tab: HQReaderTab) {
        if window == nil {
            let w = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: 640, height: 560),
                styleMask: [.titled, .closable, .resizable], backing: .buffered, defer: false)
            w.title = l10n.tr("gtmux HQ", "gtmux 中控")
            w.isReleasedWhenClosed = false
            window = w
        }
        // The content is rebuilt per open so the tab the card asked for is the one shown,
        // and so a closed window is not left holding a document in memory.
        window?.contentViewController = NSHostingController(
            rootView: HQReaderView(l10n: l10n, store: store, tab: tab))
        window?.center()
        NSApp.activate(ignoringOtherApps: true)
        window?.makeKeyAndOrderFront(nil)
    }
}
