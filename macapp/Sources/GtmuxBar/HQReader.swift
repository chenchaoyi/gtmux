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
//
// The knowledge reader has since gained the four JUDGMENT calls (menubar-kb-actions):
// promote, land, retire, dismiss. The line moved once, deliberately, and it moved between
// judging what is already written and AUTHORING new prose, not between screens. Driving
// the fleet is untouched and still remote-only. See HQKnowledgeActions.swift for the four
// verbs and DESIGN §12 for the boundary as it now runs.

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

    enum CodingKeys: String, CodingKey {
        case id, topic, title, at, body
        case promotedAt, landedAt, promoteWhy, promoteTarget, landedRef
    }

    /// Promoted and not yet carried: the one part of the knowledge lifecycle that waits on
    /// a person.
    var pending: Bool { (promotedAt ?? 0) > 0 && (landedAt ?? 0) == 0 }
}

/// One pending-distill candidate, as `gtmux capture --list --json` prints it. A candidate
/// is NOT a knowledge entry: it is a cheap one-line notice any worker can drop, waiting on
/// the supervisor's quality gate.
struct KBCandidate: Decodable {
    let at: Int64
    let topic: String
    let key: String
    let lesson: String
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
        return KBCandidateGroup(key: key, topic: newest.topic, lesson: newest.lesson,
                                at: oldest, count: rows.count)
    }
    .sorted { $0.at < $1.at }
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
    @Published private(set) var candidates: [KBCandidateGroup] = []
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
            var cands: [KBCandidate] = []
            if let d = GtmuxCLI.capture(["capture", "--list", "--json"]) {
                cands = (try? JSONDecoder().decode([KBCandidate].self, from: d)) ?? []
            }
            let newest = rows.reversed().map { $0 } // newest first, as every surface shows them
            let groups = groupCandidates(cands)
            DispatchQueue.main.async {
                // Publish only what CHANGED. Every assignment re-renders the window, and
                // the poll re-reads the same documents every 20 seconds — a board that has
                // not been rewritten would otherwise throw away the reader's scroll
                // position and selection for nothing.
                if self.board?.updatedAt != doc?.updatedAt || self.board?.exists != doc?.exists {
                    self.board = doc
                }
                if HQReaderStore.stamp(self.entries) != HQReaderStore.stamp(newest) {
                    self.entries = newest
                }
                if HQReaderStore.stamp(self.candidates) != HQReaderStore.stamp(groups) {
                    self.candidates = groups
                }
                if self.loading { self.loading = false }
            }
        }
    }

    /// The fingerprint the "did anything change" test compares.
    ///
    /// Identity alone is not enough now that the window ACTS. `promote` and `land` leave
    /// the id list byte-identical and change only the lifecycle stamps, so a reader who
    /// landed a promotion would have watched the entry sit in "waiting on you" until
    /// something unrelated happened to shift an id.
    static func stamp(_ rows: [KBEntry]) -> [String] {
        rows.map { "\($0.id)|\($0.promotedAt ?? 0)|\($0.landedAt ?? 0)" }
    }

    static func stamp(_ groups: [KBCandidateGroup]) -> [String] {
        groups.map { "\($0.key)|\($0.count)" }
    }

    /// Test seam: the rows normally arrive from the CLI, off the main queue.
    func setEntriesForTest(_ rows: [KBEntry]) { entries = rows }
    func setCandidatesForTest(_ groups: [KBCandidateGroup]) { candidates = groups }

    /// What the commander owes, oldest first — the debt, not the newest news.
    var pending: [KBEntry] {
        entries.filter { $0.pending }.sorted { ($0.promotedAt ?? 0) < ($1.promotedAt ?? 0) }
    }

    // MARK: acting

    /// Run one judgment call and report what the CLI said, or nil when it worked.
    ///
    /// Two things are deliberate here.
    ///
    /// FIRST, the verb runs FROM THE HQ HOME, because that is the only place `gtmux
    /// knowledge` accepts a mutation from (the cwd-keyed role rule that keeps workers out
    /// of the quality gate). The app does not know or guess where that is: it asks the CLI
    /// (`gtmux hq --home`), the same answer `--board` gave to the same question. The gate
    /// is not widened by one inch, and the commander at this Mac is the same caller the
    /// phone's door already serves.
    ///
    /// SECOND, a failure comes back as the CLI's OWN WORDS. "%s has no pending promotion to
    /// land (gtmux knowledge promotions)" tells the reader what happened and what to do
    /// next; a house-style "the action failed" throws all of that away. The one message
    /// this app writes itself is for the case where there was no CLI output at all, because
    /// the process never started.
    func perform(_ act: KnowledgeAct, reason: String, l10n: L10n, done: @escaping (String?) -> Void) {
        let reason = reason.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !reason.isEmpty else {
            done(l10n.tr("a reason is required", "理由必填"))
            return
        }
        DispatchQueue.global(qos: .userInitiated).async {
            let home = GtmuxCLI.captureFull(["hq", "--home"])
            let path = home.stdout.split(separator: "\n").last.map(String.init) ?? ""
            if home.status != 0 || path.isEmpty {
                let msg = home.stderr.isEmpty
                    ? l10n.tr("could not locate the HQ home", "找不到中控目录")
                    : home.stderr
                DispatchQueue.main.async { done(msg) }
                return
            }
            let r = GtmuxCLI.captureFull(act.argv(reason: reason), cwd: path)
            if r.status == 0 {
                DispatchQueue.main.async {
                    done(nil)
                    // The window must show the new state, not the state that prompted the
                    // act: a retired entry is gone from the live set, a landed one is out
                    // of the debt.
                    self.refresh()
                }
                return
            }
            let msg = r.stderr.isEmpty
                ? l10n.tr("gtmux could not be run from \(path)", "无法在 \(path) 下执行 gtmux")
                : r.stderr
            DispatchQueue.main.async { done(msg) }
        }
    }
}

enum HQReaderTab: String, CaseIterable {
    case board, knowledge
}

/// Which pane of the knowledge tab is showing. The index is a list; opening an entry
/// replaces it, exactly as the phone's sheet does — the acts belong next to the prose they
/// are a judgment on, not on a row a reader is scanning past.
enum KnowledgePane: Equatable {
    case index
    case entry(id: String)
}

/// One act waiting on a reason and a confirmation.
struct PendingAct: Equatable {
    let act: KnowledgeAct
    /// What the reader is about to act on, shown back to them in the sheet.
    let subject: String
}

struct HQReaderView: View {
    let l10n: L10n
    @ObservedObject var store: HQReaderStore
    @State var tab: HQReaderTab
    @State private var pane: KnowledgePane = .index
    @State private var pendingAct: PendingAct?
    @State private var draft = ""
    @State private var actError: String?
    @State private var busy = false
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
        .onChange(of: tab) { pane = .index }
        .sheet(item: Binding(get: { pendingAct.map { ActSheetItem(pending: $0) } },
                             set: { if $0 == nil { closeSheet() } })) { item in
            actSheet(item.pending, p)
        }
    }

    // MARK: board

    @ViewBuilder private func boardBody(_ p: Theme.Palette) -> some View {
        if let b = store.board, b.exists, let text = b.text, !text.isEmpty {
            MarkdownDoc(markdown: text, p: p)
        } else {
            // A supervisor that has written no board is ordinary, not broken.
            empty(l10n.tr("No situation board yet — the supervisor writes one as it works",
                          "还没有态势板 —— 参谋长干着干着就会写一份"), p)
        }
    }

    // MARK: knowledge

    @ViewBuilder private func knowledgeBody(_ p: Theme.Palette) -> some View {
        switch pane {
        case .index:
            knowledgeIndex(p)
        case let .entry(id):
            if let e = store.entries.first(where: { $0.id == id }) {
                entryDetail(e, p)
            } else {
                // The entry went away under the reader (a retire, or HQ superseded it).
                // Saying so beats an empty pane with a back button.
                VStack(spacing: 0) {
                    backBar(l10n.tr("Knowledge", "知识库"), p)
                    empty(l10n.tr("That entry is no longer live", "这条已不在有效集里"), p)
                }
            }
        }
    }

    @ViewBuilder private func knowledgeIndex(_ p: Theme.Palette) -> some View {
        if store.entries.isEmpty && store.candidates.isEmpty {
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
                    if !store.candidates.isEmpty {
                        // The other queue awaiting a judgment call, and the only one whose
                        // whole content fits on its row — so it acts in place rather than
                        // opening a detail with nothing more to show.
                        sectionHead(l10n.tr("candidates", "待判定候选"), store.candidates.count, p, accent: false)
                        ForEach(store.candidates) { c in candidateRow(c, p) }
                    }
                    if !store.entries.isEmpty {
                        sectionHead(l10n.tr("newest", "最近"), store.entries.count, p, accent: false)
                        // Every entry, not the first 60. The cap was right while this list
                        // was eagerly laid out; it is wrong now that the window ACTS on
                        // entries, because a cap is an entry the commander cannot retire.
                        // The rows are lazy, so the base's real 330 cost what is on screen.
                        ForEach(store.entries) { e in row(e, p, showWhy: false) }
                    }
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

    /// An index row. It OPENS the entry; it does not act on it. The list stays a reading
    /// surface, and a judgment is made looking at the thing being judged.
    @ViewBuilder private func row(_ e: KBEntry, _ p: Theme.Palette, showWhy: Bool) -> some View {
        Button { pane = .entry(id: e.id) } label: {
            VStack(alignment: .leading, spacing: 3) {
                Text(e.title)
                    .font(.system(size: 12))
                    .foregroundStyle(p.fg)
                    .multilineTextAlignment(.leading)
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
                    Text("›").font(.system(size: 12)).foregroundStyle(p.fg3)
                }
            }
            .padding(.horizontal, 12).padding(.vertical, 7)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .overlay(alignment: .bottom) { Rectangle().fill(p.divider).frame(height: 0.5).padding(.leading, 12) }
    }

    @ViewBuilder private func candidateRow(_ c: KBCandidateGroup, _ p: Theme.Palette) -> some View {
        HStack(alignment: .top, spacing: 8) {
            VStack(alignment: .leading, spacing: 3) {
                Text(c.lesson)
                    .font(.system(size: 12))
                    .foregroundStyle(p.fg)
                    .textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
                HStack(spacing: 6) {
                    Text(c.topic).font(.system(size: 10)).foregroundStyle(p.fg3)
                    if c.count > 1 {
                        // The dismiss takes the whole key, so the row says how much that is.
                        Text(l10n.tr("\(c.count) lines", "\(c.count) 条"))
                            .font(.system(size: 10)).foregroundStyle(p.fg3)
                    }
                    Spacer(minLength: 4)
                }
            }
            actButton(.dismiss(key: c.key), subject: c.lesson, p)
        }
        .padding(.horizontal, 12).padding(.vertical, 7)
        .overlay(alignment: .bottom) { Rectangle().fill(p.divider).frame(height: 0.5).padding(.leading, 12) }
    }

    /// One entry, opened: its prose, its lifecycle, and the judgments available on it.
    @ViewBuilder private func entryDetail(_ e: KBEntry, _ p: Theme.Palette) -> some View {
        VStack(spacing: 0) {
            backBar(e.topic, p)
            ScrollView {
                VStack(alignment: .leading, spacing: 10) {
                    Text(e.title)
                        .font(.system(size: 13, weight: .semibold))
                        .foregroundStyle(p.fg)
                        .textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    Text(e.id).font(.system(size: 10, design: .monospaced)).foregroundStyle(p.fg3)
                        .textSelection(.enabled)

                    if e.pending {
                        VStack(alignment: .leading, spacing: 3) {
                            Text(l10n.tr("PROMOTED · waiting on you", "已晋升 · 待你带走"))
                                .font(.system(size: 9.5, weight: .semibold)).tracking(0.8)
                                .foregroundStyle(Theme.Status.waiting)
                            if let why = e.promoteWhy, !why.isEmpty {
                                Text(why).font(.system(size: 11)).foregroundStyle(p.fg2)
                            }
                            if let target = e.promoteTarget, !target.isEmpty {
                                Text("→ \(target)").font(.system(size: 11)).foregroundStyle(p.fg3)
                            }
                        }
                    } else if let ref = e.landedRef, !ref.isEmpty {
                        Text(l10n.tr("✓ landed \(ref)", "✓ 已落地 \(ref)"))
                            .font(.system(size: 11, weight: .semibold))
                            .foregroundStyle(Theme.Status.idle)
                    }

                    if let body = e.body, !body.isEmpty {
                        Text(body)
                            .font(.system(size: 12))
                            .foregroundStyle(p.fg2)
                            .textSelection(.enabled)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }

                    Divider().padding(.vertical, 2)

                    HStack(spacing: 8) {
                        ForEach(knowledgeActs(for: e), id: \.self) { act in
                            actButton(act, subject: e.title, p)
                        }
                        Spacer(minLength: 0)
                    }
                    // Authoring stays off this screen on purpose, and saying so beats
                    // leaving a reader hunting for a button that is not coming.
                    Text(l10n.tr("Writing an entry stays in the CLI: `gtmux knowledge add`",
                                 "写入新条目仍在 CLI：`gtmux knowledge add`"))
                        .font(.system(size: 10)).foregroundStyle(p.fg3)
                }
                .padding(14)
            }
        }
    }

    @ViewBuilder private func backBar(_ title: String, _ p: Theme.Palette) -> some View {
        HStack(spacing: 8) {
            Button { pane = .index } label: {
                HStack(spacing: 3) {
                    Text("‹").font(.system(size: 14))
                    Text(l10n.tr("Knowledge", "知识库")).font(.system(size: 11))
                }
                .foregroundStyle(p.fg2)
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            Text(title).font(.system(size: 11)).foregroundStyle(p.fg3).lineLimit(1)
            Spacer(minLength: 0)
        }
        .padding(.horizontal, 12).padding(.vertical, 8)
        .overlay(alignment: .bottom) { Rectangle().fill(p.divider).frame(height: 0.5) }
    }

    @ViewBuilder private func actButton(_ act: KnowledgeAct, subject: String, _ p: Theme.Palette) -> some View {
        let c = act.copy(l10n)
        Button {
            draft = ""
            actError = nil
            pendingAct = PendingAct(act: act, subject: subject)
        } label: {
            Text(c.button)
                .font(.system(size: 11))
                .foregroundStyle(act.removes ? Theme.Status.waiting : p.fg)
                .padding(.horizontal, 10).padding(.vertical, 5)
                .overlay(RoundedRectangle(cornerRadius: 7, style: .continuous)
                    .strokeBorder(p.divider, lineWidth: 1))
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }

    // MARK: the confirm sheet

    /// The one confirmation before anything is written. It names the verb and the subject,
    /// and its confirm button stays disabled until the reason exists — the same shape as
    /// the phone's, for the same reason: every one of these verbs is refused by the CLI
    /// without one anyway, and a reason typed to satisfy a dialog after the fact is worth
    /// less than one typed while looking at the entry.
    @ViewBuilder private func actSheet(_ pending: PendingAct, _ p: Theme.Palette) -> some View {
        let c = pending.act.copy(l10n)
        VStack(alignment: .leading, spacing: 10) {
            Text(c.title).font(.system(size: 13, weight: .semibold)).foregroundStyle(p.fg)
            Text(pending.subject)
                .font(.system(size: 11)).foregroundStyle(p.fg2).lineLimit(3)
                .frame(maxWidth: .infinity, alignment: .leading)
            Text(c.hint).font(.system(size: 11)).foregroundStyle(p.fg3)
                .fixedSize(horizontal: false, vertical: true)
            TextField(c.placeholder, text: $draft, axis: .vertical)
                .textFieldStyle(.roundedBorder)
                .lineLimit(2...5)
                .font(.system(size: 12))
                .disabled(busy)

            if let err = actError {
                // The CLI's own words, unedited. "has no pending promotion to land (gtmux
                // knowledge promotions)" says what happened AND what to do next.
                Text(err)
                    .font(.system(size: 11, design: .monospaced))
                    .foregroundStyle(Theme.Status.waiting)
                    .textSelection(.enabled)
                    .fixedSize(horizontal: false, vertical: true)
            }

            HStack(spacing: 8) {
                Text("gtmux knowledge \(verbWord(pending.act)) \(c.field)")
                    .font(.system(size: 10, design: .monospaced)).foregroundStyle(p.fg3)
                Spacer(minLength: 8)
                Button(l10n.tr("Cancel", "取消")) { closeSheet() }
                    .disabled(busy)
                Button(l10n.tr("Confirm", "确认")) { run(pending) }
                    .keyboardShortcut(.defaultAction)
                    .disabled(busy || draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
            }
        }
        .padding(16)
        .frame(width: 420)
        .background(p.bg)
    }

    private func verbWord(_ act: KnowledgeAct) -> String {
        switch act {
        case .promote: return "promote"
        case .land: return "land"
        case .retire: return "retire"
        case .dismiss: return "dismiss"
        }
    }

    private func run(_ pending: PendingAct) {
        busy = true
        actError = nil
        store.perform(pending.act, reason: draft, l10n: l10n) { err in
            busy = false
            if let err {
                actError = err
                return
            }
            pendingAct = nil
            draft = ""
            // A retired entry is gone from the live set and a landed one has left the
            // debt, so there is nothing behind this sheet worth returning to.
            pane = .index
        }
    }

    private func closeSheet() {
        guard !busy else { return }
        pendingAct = nil
        draft = ""
        actError = nil
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

/// `.sheet(item:)` wants an Identifiable; the pending act is identified by what it is
/// about to do to what.
private struct ActSheetItem: Identifiable {
    let pending: PendingAct
    var id: String { "\(pending.act)" }
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
