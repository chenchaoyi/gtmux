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

    /// The memberwise shape older callers and tests use, with the axes optional: a row
    /// that predates them is a row without them.
    init(id: String, topic: String, title: String, at: Int64?, promotedAt: Int64?, landedAt: Int64?,
         promoteWhy: String?, promoteTarget: String?, landedRef: String?, body: String?,
         kind: String? = nil, kindAssumed: Bool? = nil, provenance: String? = nil, hits: Int? = nil,
         audience: String? = nil, audienceRepo: String? = nil, status: String? = nil, issueUrl: String? = nil,
         lang: String? = nil, langAssumed: Bool? = nil, alt: KBAlt? = nil, sensitive: Bool? = nil) {
        self.id = id; self.topic = topic; self.title = title; self.at = at
        self.promotedAt = promotedAt; self.landedAt = landedAt
        self.promoteWhy = promoteWhy; self.promoteTarget = promoteTarget; self.landedRef = landedRef
        self.body = body
        self.kind = kind; self.kindAssumed = kindAssumed; self.provenance = provenance; self.hits = hits
        self.audience = audience; self.audienceRepo = audienceRepo; self.status = status; self.issueUrl = issueUrl
        self.lang = lang; self.langAssumed = langAssumed; self.alt = alt
        self.sensitive = sensitive
    }

    /// The axes as one metadata line: `pitfalls? · from mined ×6 · for this machine`.
    func axesLine(_ l10n: L10n) -> String {
        var parts: [String] = []
        if let k = kind, !k.isEmpty { parts.append(k + ((kindAssumed ?? false) ? "?" : "")) }
        if let pv = provenance, !pv.isEmpty {
            var s = l10n.tr("from ", "来自 ") + pv
            if let h = hits, h > 1 { s += " ×\(h)" }
            parts.append(s)
        }
        let aud = audienceShort(audience, l10n)
        if !aud.isEmpty { parts.append(l10n.tr("for ", "给 ") + aud) }
        if status == "hypothesis" { parts.append(l10n.tr("hypothesis", "待验证")) }
        if sensitive ?? false { parts.append(l10n.tr("sensitive · this Mac only", "敏感 · 只留本机")) }
        return parts.joined(separator: " · ")
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
        return KBCandidateGroup(key: key, topic: newest.topic, lesson: newest.lesson,
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
        for g in self where g.family > 0 { firstAt[g.family] = Swift.min(firstAt[g.family] ?? g.at, g.at) }
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

final class HQReaderStore: ObservableObject {
    @Published private(set) var board: BoardDoc?
    @Published private(set) var entries: [KBEntry] = []
    @Published private(set) var candidates: [KBCandidateGroup] = []
    /// The whole base grouped by topic, biggest first — derived from `entries`, so the
    /// two can never disagree about what is filed where.
    var topics: [KBTopic] { knowledgeTopics(entries) }
    @Published private(set) var loading = true
    /// The machine tab's reading (menubar-hq-report), refreshed only while that tab shows.
    @Published private(set) var resource: ResourceReport?
    /// Pane id → the session name the radar shows, so the per-agent table names what it
    /// measures instead of listing bare `%N`s.
    @Published private(set) var paneNames: [String: String] = [:]
    /// Set by the view: read the machine on the next ticks.
    var wantsMachine = false
    /// The usage tab's report, refreshed only while that tab shows.
    @Published private(set) var usage: HQUsageReport?
    var wantsUsage = false

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
        if wantsMachine { refreshMachine() }
        if wantsUsage { refreshUsage() }
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

    /// The machine tab's two reads: the resource report and the radar rows (for names).
    /// Both are cheap (df/sysctl/ps and the same `agents --json` the popover polls).
    func refreshMachine() {
        DispatchQueue.global(qos: .utility).async {
            let report = GtmuxCLI.capture(["resource", "--json"])
                .flatMap { try? JSONDecoder().decode(ResourceReport.self, from: $0) }
            var names: [String: String] = [:]
            if let d = GtmuxCLI.capture(["agents", "--json"]),
               let rows = try? JSONDecoder().decode([Agent].self, from: d) {
                for a in rows where !a.paneID.isEmpty { names[a.paneID] = a.primary }
            }
            DispatchQueue.main.async {
                self.resource = report
                if self.paneNames != names { self.paneNames = names }
            }
        }
    }

    /// The usage tab's read: the cached quota probe plus the per-session burn.
    func refreshUsage() {
        DispatchQueue.global(qos: .utility).async {
            let report = GtmuxCLI.capture(["usage", "--json"])
                .flatMap { try? JSONDecoder().decode(HQUsageReport.self, from: $0) }
            DispatchQueue.main.async { self.usage = report }
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
        guard !act.needsReason || !reason.isEmpty else {
            done(l10n.tr("a reason is required", "理由必填"))
            return
        }
        DispatchQueue.global(qos: .userInitiated).async {
            let home = GtmuxCLI.captureFull(["hq", "--home"])
            let path = home.stdout.split(separator: "\n").last.map(String.init) ?? ""
            if home.status != 0 || path.isEmpty {
                let msg = home.stderr.isEmpty
                    ? l10n.tr("could not locate the HQ home", "找不到 HQ 目录")
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
    /// The machine (menubar-hq-report): `gtmux resource` laid out to read — the tab the
    /// card's machine row opens, so a red tier has somewhere on the Mac to say what it is.
    case machine
    /// Usage (menubar-hq-usage): the quotas, who is burning them, the phone's UsageSheet
    /// on the Mac — the door the card's usage row opens.
    case usage
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
    @State private var actMode = 0        // 0 daily · 1 weekly · 2 cumulative (usage-activity)
    @State private var actPicked = ""     // a clicked day; today until one is clicked
    @State private var pane: KnowledgePane = .index
    @State private var pendingAct: PendingAct?
    @State private var openTopics: Set<String> = []
    @State private var mem = HQMemoryState()
    @State private var memError: String?
    /// The export sheet's state while it is up; nil between exports.
    @State private var exportFlow: HQExportFlow?
    @State private var draft = ""
    /// What someone typed into Find. Non-empty replaces the index with results — the
    /// index IS the browse answer, and two answers to one question is the confusion.
    @State private var query = ""
    /// The paragraph under "waiting on you" describes what a promotion IS. Read once;
    /// standing there forever it is height. Closed until asked for.
    @State private var whyOpen = false
    @State private var actError: String?
    @State private var busy = false
    /// The promote sheet's answer to "who must know it", and the path when it is a repo.
    @State private var audience: KnowledgeAudience = .machine
    @State private var repoPath = ""
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(spacing: 0) {
            HStack(spacing: 10) {
                Picker("", selection: $tab) {
                    Text(l10n.tr("Situation board", "态势板")).tag(HQReaderTab.board)
                    Text(l10n.tr("Knowledge", "知识库")).tag(HQReaderTab.knowledge)
                    Text(l10n.tr("Machine", "机器")).tag(HQReaderTab.machine)
                    Text(l10n.tr("Usage", "用量")).tag(HQReaderTab.usage)
                }
                .pickerStyle(.segmented)
                .labelsHidden()

                // Find. 396 entries across 7 topics on this machine, and until now the
                // only way in was knowing which topic holds the one you want — that is
                // knowledge about the knowledge base, not about the machine. It shares
                // the tab strip's row rather than taking a band of its own.
                if tab == .knowledge, pane == .index {
                    HStack(spacing: 6) {
                        Image(systemName: "magnifyingglass")
                            .font(.system(size: 12)).foregroundStyle(p.fg3)
                        TextField(
                            l10n.tr("Find in \(store.entries.count) entries",
                                    "在 \(store.entries.count) 条里找"),
                            text: $query)
                            .textFieldStyle(.plain)
                            .font(.system(size: 12))
                        if !query.trimmingCharacters(in: .whitespaces).isEmpty {
                            Button { query = "" } label: {
                                Image(systemName: "xmark.circle.fill")
                                    .font(.system(size: 12)).foregroundStyle(p.fg3)
                            }
                            .buttonStyle(.plain)
                        }
                    }
                    .padding(.horizontal, 8)
                    .padding(.vertical, 4)
                    .background(RoundedRectangle(cornerRadius: 6).fill(p.fg3.opacity(0.10)))
                    .frame(maxWidth: 280)
                }
            }
            .padding(10)

            Divider()

            // The memory, and the way out of this machine.
            //
            // This window sits ON the data — the board and the base it reads are files in
            // the HQ home — and it was the only surface that could not export them: the
            // CLI could, the phone could keep a copy, and the Mac app, running on the
            // machine where all of it lives, could not. The sentence about what protects
            // it is the CLI's own, not a paraphrase, so the two cannot drift.
            memoryBar(p)

            Divider()

            switch tab {
            case .board: boardBody(p)
            case .knowledge: knowledgeBody(p)
            case .machine: machineBody(p)
            case .usage: usageBody(p)
            }
        }
        // 292pt of list plus a detail pane wide enough to read a lesson in. The old
        // 520 floor was for one column.
        .frame(minWidth: 700, minHeight: 440)
        .background(p.bg)
        .onAppear {
            store.wantsMachine = tab == .machine
            store.wantsUsage = tab == .usage
            store.start()
            mem = readHQMemoryState()
        }
        .onDisappear { store.stop() }
        .onChange(of: tab) {
            pane = .index
            store.wantsMachine = tab == .machine
            store.wantsUsage = tab == .usage
            if tab == .machine { store.refreshMachine() }
            if tab == .usage { store.refreshUsage() }
        }
        .sheet(item: Binding(get: { pendingAct.map { ActSheetItem(pending: $0) } },
                             set: { if $0 == nil { closeSheet() } })) { item in
            actSheet(item.pending, p)
        }
        .sheet(isPresented: Binding(get: { exportFlow != nil },
                                    set: { if !$0 { exportFlow = nil; mem = readHQMemoryState() } })) {
            if let flow = exportFlow {
                HQExportSheet(l10n: l10n, flow: flow) {
                    exportFlow = nil
                    mem = readHQMemoryState()
                }
            }
        }
    }

    // MARK: board

    @ViewBuilder private func memoryBar(_ p: Theme.Palette) -> some View {
        HStack(spacing: 8) {
            if mem.exists {
                // The SIZE, not a tick: "exported" and "6 MB of irreplaceable notes are
                // exported" are different sentences, and only the second says what a loss
                // would cost.
                Text("\(l10n.tr("Records", "档案")) \(mem.sizeText) · \(mem.snapshots) \(l10n.tr("local snapshots", "份本地快照"))")
                    .font(.system(size: 11)).foregroundStyle(p.fg2)
                Text(mem.offMachine)
                    .font(.system(size: 11)).foregroundStyle(p.fg3)
                    .lineLimit(1).truncationMode(.tail)
            } else {
                Text(l10n.tr("no HQ records on this machine", "这台机器上没有 HQ 档案"))
                    .font(.system(size: 11)).foregroundStyle(p.fg3)
            }
            if mem.exists, mem.lastExportAt > 0 {
                // When the commander last carried it off themselves, and whether that copy
                // was locked — the one off-machine copy gtmux can vouch for.
                let age = relativeTime(Int(mem.lastExportAt), now: Int(Date().timeIntervalSince1970))
                Text(mem.lastExportEncrypted
                     ? l10n.tr("· last export \(age) ago, locked", "· 上次导出 \(age)前，已上锁")
                     : l10n.tr("· last export \(age) ago, not locked", "· 上次导出 \(age)前，未上锁"))
                    .font(.system(size: 11)).foregroundStyle(p.fg3).lineLimit(1)
            }
            Spacer(minLength: 8)
            if let e = memError {
                Text(e).font(.system(size: 11)).foregroundStyle(Theme.Status.waiting).lineLimit(1)
            }
            Button {
                memError = nil
                exportFlow = HQExportFlow()
            } label: {
                Text(l10n.tr("Export…", "导出…")).font(.system(size: 11.5))
            }
            .buttonStyle(.bordered)
            .controlSize(.small)
            .disabled(!mem.exists)
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 7)
    }

    /// How long ago HQ last wrote the board, in the words the phone uses.
    ///
    /// The phone's board sheet says "updated 3h ago" under its title; the Mac said nothing
    /// at all, and `updatedAt` was already on the model (2026-09-09). A board read as
    /// current when it is hours stale is a failure that costs something real: it is HQ's
    /// picture of the fleet, and the whole reason to open it is that the picture is
    /// trustworthy.
    private func boardAge(_ at: Int64?) -> String? {
        guard let at, at > 0 else { return nil }
        return boardAgeText(Int64(Date().timeIntervalSince1970) - at, zh: l10n.lang == "zh")
    }

    @ViewBuilder private func boardBody(_ p: Theme.Palette) -> some View {
        if let b = store.board, b.exists, let text = b.text, !text.isEmpty {
            // An OUTLINE, not the whole document: 74 KB on this machine, and reaching any
            // one entry meant dragging through the rest. Same reader as the phone.
            VStack(alignment: .leading, spacing: 0) {
                if let age = boardAge(b.updatedAt) {
                    HStack(spacing: 5) {
                        Circle()
                            .fill(Theme.Status.idle)
                            .frame(width: 6, height: 6)
                        Text(age).font(.system(size: 11)).foregroundStyle(p.fg3)
                        Spacer()
                    }
                    .padding(.horizontal, 14)
                    .padding(.top, 8)
                    .padding(.bottom, 2)
                }
                BoardOutlineView(markdown: text, p: p)
            }
        } else {
            // A supervisor that has written no board is ordinary, not broken.
            empty(l10n.tr("No situation board yet —HQ writes one as it works",
                          "还没有态势板 —— HQ 干着干着就会写一份"), p)
        }
    }

    // MARK: machine (menubar-hq-report)

    /// `gtmux resource`, laid out to read: the four readings, the core's own warning
    /// sentence, the per-agent table heaviest first, and the orphans it calls reclaimable
    /// with its own hint. READING ONLY — nothing here kills a process; that would be the
    /// first act in this window that touches the machine, and it gets its own change.
    @ViewBuilder private func machineBody(_ p: Theme.Palette) -> some View {
        if let r = store.resource {
            ScrollView {
                VStack(alignment: .leading, spacing: 10) {
                    readings(r.machine, p)
                    // The core's own words, or its silence: the tier is what the medallion
                    // reads, and this line is the reason behind it.
                    HStack(spacing: 6) {
                        Circle().fill(tierColor(r.machine.tier ?? "", p)).frame(width: 7, height: 7)
                        Text(machineSentence(r.machine)).font(.system(size: 11.5)).foregroundStyle(p.fg2)
                    }
                    Divider()
                    procTable(r, p)
                    if let orphans = r.orphans, !orphans.isEmpty {
                        Divider()
                        orphanList(orphans, p)
                    }
                }
                .padding(14)
            }
        } else {
            empty(l10n.tr("Reading the machine…", "正在读取机器…"), p)
        }
    }

    @ViewBuilder private func readings(_ m: ResourceReport.Machine, _ p: Theme.Palette) -> some View {
        let mem = m.memFreePct ?? 0
        let disk = m.diskUsePct ?? 0
        let diskFree = m.diskFreeGB ?? 0
        let warn = m.warn ?? ""
        let loadText = String(format: "%.2f × %d", m.loadRatio ?? 0, m.ncpu ?? 0)
        HStack(spacing: 8) {
            readingCard(zh ? "内存 \(mem)% 空闲" : "memory \(mem)% free",
                        memTierWord(m.memTier ?? ""), tone: memTierTone(m.memTier ?? ""), p)
            readingCard(zh ? "磁盘 \(disk)% 已用" : "disk \(disk)% used",
                        zh ? "剩 \(diskFree) GB" : "\(diskFree) GB left",
                        tone: warn.hasPrefix("disk") ? .amber : .plain, p)
            readingCard((zh ? "负载 " : "load ") + loadText, zh ? "核" : "cores",
                        tone: warn.hasPrefix("load") ? .amber : .plain, p)
            if let b = m.battery, b.present {
                readingCard(zh ? "电量 \(b.percent)%" : "power \(b.percent)%", batteryWord(b),
                            tone: (!b.onAC && b.percent <= 20) ? .amber : .plain, p)
            }
        }
    }

    private func batteryWord(_ b: ResourceReport.Battery) -> String {
        if b.onAC { return l10n.tr("AC", "电源") }
        if let t = b.timeLeft, !t.isEmpty { return l10n.tr("battery \(t)", "电池 \(t)") }
        return l10n.tr("battery", "电池")
    }

    @ViewBuilder private func procTable(_ r: ResourceReport, _ p: Theme.Palette) -> some View {
        let rows = (r.agents ?? [:]).map { (pane: $0.key, rss: $0.value.rssMB, cpu: $0.value.cpu) }
            .sorted { $0.rss != $1.rss ? $0.rss > $1.rss : $0.pane < $1.pane }
        let heaviest = rows.first?.rss ?? 0
        let red = (r.machine.tier ?? "") == "red"
        if rows.isEmpty {
            Text(l10n.tr("No agent process to measure", "没有可测量的 agent 进程"))
                .font(.system(size: 12)).foregroundStyle(p.fg3)
        } else {
            procHeader(p)
            ForEach(rows, id: \.pane) { row in
                procRow(pane: row.pane, name: store.paneNames[row.pane] ?? "", rss: row.rss, cpu: row.cpu,
                        hot: red && row.rss >= heaviest, p)
            }
        }
    }

    @ViewBuilder private func orphanList(_ orphans: [ResourceReport.Orphan], _ p: Theme.Palette) -> some View {
        Text(l10n.tr("Reclaimable — no live agent owns these", "可回收 —— 没有活着的 agent 拥有它们"))
            .font(.system(size: 11, weight: .bold)).foregroundStyle(p.fg3)
        ForEach(orphans) { o in
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 8) {
                    Text("pid \(o.pid)").font(Theme.Font.mono).foregroundStyle(p.fg3)
                    Text(o.comm).font(.system(size: 12)).foregroundStyle(p.fg)
                    if let k = o.kind, !k.isEmpty {
                        Text(k).font(.system(size: 10)).foregroundStyle(p.fg3)
                            .padding(.horizontal, 5).padding(.vertical, 1)
                            .background(RoundedRectangle(cornerRadius: 4).fill(p.fg3.opacity(0.12)))
                    }
                    Spacer()
                    Text("\(o.rssMB) MB · " + String(format: "%.1f%%", o.cpu))
                        .font(Theme.Font.mono).foregroundStyle(p.fg2)
                }
                if let h = o.hint, !h.isEmpty {
                    Text(h).font(.system(size: 11)).foregroundStyle(p.fg3).textSelection(.enabled)
                }
            }
            .padding(.vertical, 3)
        }
    }

    private var zh: Bool { l10n.lang == "zh" }

    private func memTierWord(_ tier: String) -> String {
        switch tier {
        case "critical": return l10n.tr("critical", "临界")
        case "warn": return l10n.tr("warn", "警戒")
        default: return l10n.tr("normal", "正常")
        }
    }

    private func memTierTone(_ tier: String) -> HQRowTone {
        switch tier {
        case "critical": return .red
        case "warn": return .amber
        default: return .plain
        }
    }

    private func tierColor(_ tier: String, _ p: Theme.Palette) -> Color {
        switch tier {
        case "red": return Theme.Status.waiting
        case "amber": return Theme.Status.errored
        default: return Theme.Status.idle
        }
    }

    /// The sentence under the readings: the core's warning verbatim when it has one.
    private func machineSentence(_ m: ResourceReport.Machine) -> String {
        if let w = m.warn, !w.isEmpty {
            switch m.tier ?? "" {
            case "red": return l10n.tr("Red tier — \(w). The medallion's ⚠ is this.", "红档 —— \(w)。徽章上的 ⚠ 说的就是它。")
            default: return l10n.tr("Amber — \(w). A heads-up, not a bottleneck.", "琥珀 —— \(w)。提个醒，不是瓶颈。")
            }
        }
        return l10n.tr("Nothing is short. Readings from gtmux resource.", "没有短缺。读数来自 gtmux resource。")
    }

    @ViewBuilder private func readingCard(_ value: String, _ sub: String, tone: HQRowTone, _ p: Theme.Palette) -> some View {
        VStack(alignment: .leading, spacing: 3) {
            Text(value).font(.system(size: 12, weight: .semibold)).foregroundStyle(p.fg).lineLimit(1)
            Text(sub).font(.system(size: 10.5))
                .foregroundStyle(tone == .red ? Theme.Status.waiting : tone == .amber ? Theme.Status.errored : p.fg3)
                .lineLimit(1)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 10).padding(.vertical, 8)
        .background(RoundedRectangle(cornerRadius: 8, style: .continuous).fill(p.rowSelected.opacity(0.5)))
        .overlay(RoundedRectangle(cornerRadius: 8, style: .continuous).strokeBorder(p.divider, lineWidth: 1))
    }

    @ViewBuilder private func procHeader(_ p: Theme.Palette) -> some View {
        HStack(spacing: 10) {
            Text("PANE").frame(width: 44, alignment: .leading)
            Text(l10n.tr("SESSION", "会话")).frame(maxWidth: .infinity, alignment: .leading)
            Text("RSS").frame(width: 72, alignment: .trailing)
            Text("CPU").frame(width: 60, alignment: .trailing)
        }
        .font(.system(size: 10, weight: .bold)).kerning(0.5).foregroundStyle(p.fg3)
    }

    @ViewBuilder private func procRow(pane: String, name: String, rss: Int, cpu: Double, hot: Bool, _ p: Theme.Palette) -> some View {
        HStack(spacing: 10) {
            Text(pane).font(Theme.Font.mono).foregroundStyle(p.fg3).frame(width: 44, alignment: .leading)
            Text(name).font(.system(size: 11.5)).foregroundStyle(p.fg).lineLimit(1)
                .frame(maxWidth: .infinity, alignment: .leading)
            Text("\(rss) MB").font(Theme.Font.mono).foregroundStyle(hot ? Theme.Status.waiting : p.fg2)
                .frame(width: 72, alignment: .trailing)
            Text(String(format: "%.1f%%", cpu)).font(Theme.Font.mono).foregroundStyle(hot ? Theme.Status.waiting : p.fg2)
                .frame(width: 60, alignment: .trailing)
        }
        .padding(.vertical, 4)
        .overlay(alignment: .top) { Divider().overlay(p.divider) }
    }

    // MARK: usage (menubar-hq-usage)

    /// The phone's usage sheet, on the Mac, in the CLI's order: the quota (the number no
    /// local arithmetic can produce) → who is burning it (per-agent totals, then the
    /// sessions, sorted by trouble) — the machine has its own tab here. A card at the top
    /// says the one thing the reader came for: which window is tightest.
    @ViewBuilder private func usageBody(_ p: Theme.Palette) -> some View {
        if let u = store.usage {
            ScrollView {
                VStack(alignment: .leading, spacing: 12) {
                    usageLead(u, p)
                    usagePlans(u, p)
                    if let h = u.history, let days = h.days, !days.isEmpty {
                        Divider()
                        usageTokens(h, days, p)
                    }
                    usageBurn(u, p)
                }
                .padding(14)
            }
        } else {
            empty(l10n.tr("Reading usage…", "正在读取用量…"), p)
        }
    }

    /// The tightest non-session window, said in one sentence. Session windows reset in
    /// hours, so a high one is a working day, not news (the phone's rule).
    @ViewBuilder private func usageLead(_ u: HQUsageReport, _ p: Theme.Palette) -> some View {
        let all = u.limits?.windows ?? []
        let weekly = all.filter { ($0.kind ?? hqWindowName($0).lowercased()) != "session" }
        let pool = weekly.isEmpty ? all : weekly
        if let t = pool.max(by: { $0.pctUsed < $1.pctUsed }) {
            let name = t.agentName ?? t.agent ?? ""
            HStack(spacing: 8) {
                Text("\(t.pctUsed)%").font(.system(size: 20, weight: .semibold)).foregroundStyle(p.fg)
                VStack(alignment: .leading, spacing: 2) {
                    Text(l10n.tr("tightest: \(name) \(hqWindowTitle(t, zh: zh))", "最紧：\(name) \(hqWindowTitle(t, zh: zh))"))
                        .font(.system(size: 12, weight: .semibold)).foregroundStyle(p.fg)
                    let r = hqResetTitle(t, zh: zh)
                    if !r.isEmpty {
                        Text(l10n.tr("resets \(r)", "重置于 \(r)")).font(.system(size: 11)).foregroundStyle(p.fg3)
                    }
                }
                Spacer()
            }
            .padding(10)
            .background(RoundedRectangle(cornerRadius: 8, style: .continuous).fill(p.rowSelected.opacity(0.5)))
            .overlay(RoundedRectangle(cornerRadius: 8, style: .continuous).strokeBorder(p.divider, lineWidth: 1))
        }
        if let w = u.limits?.warn, !w.isEmpty {
            Text(w).font(.system(size: 11.5)).foregroundStyle(Theme.Status.errored)
        }
    }

    /// Quotas grouped by agent, the name said once, each window a neutral bar: a bar's
    /// length speaks before a number does, and colour stays a status channel.
    @ViewBuilder private func usagePlans(_ u: HQUsageReport, _ p: Theme.Palette) -> some View {
        let wins = u.limits?.windows ?? []
        if wins.isEmpty {
            Text(l10n.tr("No plan readable — run the agent's /usage once", "读不到套餐 —— 在 agent 里跑一次 /usage"))
                .font(.system(size: 12)).foregroundStyle(p.fg3)
        } else {
            let groups = Dictionary(grouping: wins, by: { $0.agent ?? $0.label })
            let order = wins.map { $0.agent ?? $0.label }.reduce(into: [String]()) { if !$0.contains($1) { $0.append($1) } }
            ForEach(order, id: \.self) { key in
                let ws = groups[key] ?? []
                VStack(alignment: .leading, spacing: 6) {
                    Text(ws.first?.agentName ?? key).font(.system(size: 12, weight: .semibold)).foregroundStyle(p.fg)
                    ForEach(Array(ws.enumerated()), id: \.offset) { _, w in
                        HStack(spacing: 10) {
                            Text(hqWindowTitle(w, zh: zh)).font(.system(size: 11.5)).foregroundStyle(p.fg2)
                                .frame(width: 150, alignment: .leading).lineLimit(1)
                            GeometryReader { g in
                                ZStack(alignment: .leading) {
                                    RoundedRectangle(cornerRadius: 3).fill(p.fg3.opacity(0.18))
                                    RoundedRectangle(cornerRadius: 3).fill(p.fg3.opacity(0.7))
                                        .frame(width: g.size.width * CGFloat(min(max(w.pctUsed, 0), 100)) / 100)
                                }
                            }
                            .frame(height: 6)
                            Text("\(w.pctUsed)%").font(Theme.Font.mono).foregroundStyle(p.fg).frame(width: 40, alignment: .trailing)
                            Text(hqResetTitle(w, zh: zh)).font(.system(size: 10.5)).foregroundStyle(p.fg3).frame(width: 130, alignment: .leading).lineLimit(1)
                        }
                    }
                }
            }
        }
    }

    /// Tokens by day (usage-daily-totals): today and this week across every agent, seven
    /// bars — one neutral series, today's in the stronger ink, direct labels on today and
    /// the tallest day, weekday initials beneath — then the week's split per agent.
    @ViewBuilder private func usageTokens(_ h: HQUsageHistory, _ days: [HQUsageDay], _ p: Theme.Palette) -> some View {
        if let grid = hqActivity(h, weeks: 44, today: Date(), zh: zh) {
            usageActivity(h, grid, p)
        } else {
            usageDayBars(h, days, p)
        }
    }

    /// The year at a glance (usage-activity, 2026-09-15): three figures, the stats a
    /// reader asks of a year, then one of three pictures of the same series — the phone's
    /// block with 44 columns. The greens are GitHub's contribution ramp (the commander's
    /// choice, so the picture reads the same everywhere); a chart, not a status.
    @ViewBuilder private func usageActivity(_ h: HQUsageHistory, _ grid: HQActivityGrid, _ p: Theme.Palette) -> some View {
        let ramp = (scheme == .dark ? hqActivityRampDark : hqActivityRampLight).map { Color(hex: $0.0, opacity: $0.1) }
        let todayKey = grid.rows.flatMap { $0 }.first { $0.today }?.date ?? ""
        let readout = hqDayReadout(h, date: actPicked.isEmpty ? todayKey : actPicked, zh: zh)
        VStack(alignment: .leading, spacing: 8) {
            HStack(alignment: .firstTextBaseline, spacing: 18) {
                ForEach(Array(grid.figs.enumerated()), id: \.offset) { _, f in
                    VStack(alignment: .leading, spacing: 1) {
                        Text(f.0).font(.system(size: 20, weight: .semibold)).foregroundStyle(p.fg)
                        Text(f.1).font(.system(size: 10.5)).foregroundStyle(p.fg3)
                    }
                }
                Spacer()
                Picker("", selection: $actMode) {
                    Text(l10n.tr("daily", "按天")).tag(0)
                    Text(l10n.tr("weekly", "按周")).tag(1)
                    Text(l10n.tr("cumulative", "累计")).tag(2)
                }
                .pickerStyle(.segmented).labelsHidden().frame(width: 200)
            }
            HStack(spacing: 12) {
                ForEach(grid.stats, id: \.self) { s in Text(s).font(.system(size: 11)).foregroundStyle(p.fg2) }
                Spacer()
                Text(grid.range).font(.system(size: 10.5)).foregroundStyle(p.fg3)
            }
            if actMode == 0 {
                VStack(alignment: .leading, spacing: 3) {
                    ZStack(alignment: .topLeading) {
                        ForEach(Array(grid.months.enumerated()), id: \.offset) { _, m in
                            Text(m.1).font(.system(size: 10)).foregroundStyle(p.fg3).offset(x: CGFloat(26 + m.0 * 14))
                        }
                    }
                    .frame(height: 13, alignment: .topLeading)
                    ForEach(Array(grid.rows.enumerated()), id: \.offset) { ri, row in
                        HStack(spacing: 3) {
                            Text(grid.rowLabels[ri]).font(.system(size: 9.5)).foregroundStyle(p.fg3).frame(width: 23, alignment: .trailing)
                            ForEach(Array(row.enumerated()), id: \.offset) { _, c in
                                if c.level < 0 {
                                    Color.clear.frame(width: 11, height: 11)
                                } else {
                                    let ringed = actPicked.isEmpty ? c.today : actPicked == c.date
                                    RoundedRectangle(cornerRadius: 2.5, style: .continuous)
                                        .fill(ramp[c.level])
                                        .overlay(RoundedRectangle(cornerRadius: 2.5, style: .continuous).stroke(ringed ? (actPicked.isEmpty ? p.fg2 : p.fg) : .clear, lineWidth: 1.5))
                                        .frame(width: 11, height: 11)
                                        .help("\(c.date) · \(hqCompactTok(c.out))")
                                        .onTapGesture { actPicked = c.date }
                                }
                            }
                        }
                    }
                    HStack(spacing: 3) {
                        Text(readout).font(Theme.Font.mono).foregroundStyle(p.fg2).lineLimit(1)
                        Spacer()
                        Text(l10n.tr("Less", "少")).font(.system(size: 9.5)).foregroundStyle(p.fg3)
                        ForEach(Array(ramp.enumerated()), id: \.offset) { _, c in RoundedRectangle(cornerRadius: 2).fill(c).frame(width: 9, height: 9) }
                        Text(l10n.tr("More", "多")).font(.system(size: 9.5)).foregroundStyle(p.fg3)
                    }
                    .padding(.top, 4).padding(.leading, 26)
                }
            } else if actMode == 1 {
                VStack(alignment: .leading, spacing: 3) {
                    HStack(alignment: .bottom, spacing: 3) {
                        ForEach(Array(grid.weekBars.enumerated()), id: \.offset) { _, b in
                            RoundedRectangle(cornerRadius: 2, style: .continuous)
                                .fill(ramp[b.level])
                                .overlay(RoundedRectangle(cornerRadius: 2, style: .continuous).stroke(b.current ? p.fg2 : .clear, lineWidth: 1.5))
                                .frame(height: max(2, 60 * b.frac))
                                .frame(maxWidth: .infinity)
                                .help("\(b.start) · \(hqCompactTok(b.out))")
                        }
                    }
                    .frame(height: 64)
                    HStack(spacing: 3) {
                        ForEach(Array(grid.weekBars.enumerated()), id: \.offset) { _, b in
                            Text(b.month.isEmpty ? " " : b.month).font(.system(size: 9)).foregroundStyle(p.fg3).lineLimit(1).frame(maxWidth: .infinity)
                        }
                    }
                    if let top = grid.weekBars.max(by: { $0.out < $1.out }) {
                        Text(l10n.tr("the week of \(top.start) was the highest · \(hqCompactTok(top.out)) · the ringed bar is this week, still running",
                                     "\(top.start) 那周最高 · \(hqCompactTok(top.out)) · 描边的是本周，还没过完"))
                            .font(.system(size: 11)).foregroundStyle(p.fg2).lineLimit(1).padding(.top, 4)
                    }
                }
            } else {
                VStack(alignment: .leading, spacing: 4) {
                    GeometryReader { g in
                        Path { path in
                            let n = grid.cumulative.count
                            guard n > 1 else { return }
                            for (i, v) in grid.cumulative.enumerated() {
                                let pt = CGPoint(x: g.size.width * CGFloat(i) / CGFloat(n - 1), y: g.size.height * (1 - CGFloat(v)))
                                if i == 0 { path.move(to: pt) } else { path.addLine(to: pt) }
                            }
                        }
                        .stroke(ramp[4], style: StrokeStyle(lineWidth: 2, lineCap: .round, lineJoin: .round))
                    }
                    .frame(height: 64)
                    HStack {
                        Text(grid.range).font(.system(size: 10)).foregroundStyle(p.fg3)
                        Spacer()
                        Text(grid.cumulativeLabel).font(.system(size: 13, weight: .semibold)).foregroundStyle(p.fg)
                    }
                }
            }
            usageAgentSplit(h, p)
        }
    }

    /// The per-agent split under either picture.
    @ViewBuilder private func usageAgentSplit(_ h: HQUsageHistory, _ p: Theme.Palette) -> some View {
        if let agents = h.byAgent, !agents.isEmpty {
            VStack(alignment: .leading, spacing: 3) {
                ForEach(agents, id: \.agentKey) { a in
                    HStack(spacing: 10) {
                        Text(a.agentName ?? a.agentKey).font(.system(size: 11.5)).foregroundStyle(p.fg)
                            .frame(width: 150, alignment: .leading)
                        Spacer()
                        Text(l10n.tr("today \(hqCompactTok(a.todayOut))", "今天 \(hqCompactTok(a.todayOut))"))
                            .font(Theme.Font.mono).foregroundStyle(p.fg2).frame(width: 110, alignment: .trailing)
                        Text(l10n.tr("week \(hqCompactTok(a.weekOut))", "本周 \(hqCompactTok(a.weekOut))"))
                            .font(Theme.Font.mono).foregroundStyle(p.fg).frame(width: 110, alignment: .trailing)
                    }
                }
            }
        }
    }

    /// An older CLI (no `activity`): the seven-day bars as before.
    @ViewBuilder private func usageDayBars(_ h: HQUsageHistory, _ days: [HQUsageDay], _ p: Theme.Palette) -> some View {
        let bars = hqDayBars(days, zh: zh)
        let today = h.todayOut ?? days.last?.out ?? 0
        let week = h.weekOut ?? days.reduce(0) { $0 + $1.out }
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .firstTextBaseline, spacing: 18) {
                VStack(alignment: .leading, spacing: 1) {
                    Text(hqCompactTok(today)).font(.system(size: 20, weight: .semibold)).foregroundStyle(p.fg)
                    Text(l10n.tr("today", "今天")).font(.system(size: 10.5)).foregroundStyle(p.fg3)
                }
                VStack(alignment: .leading, spacing: 1) {
                    Text(hqCompactTok(week)).font(.system(size: 20, weight: .semibold)).foregroundStyle(p.fg)
                    Text(l10n.tr("this week", "本周")).font(.system(size: 10.5)).foregroundStyle(p.fg3)
                }
                Spacer()
                Text(l10n.tr("output tokens, every agent, by local day", "输出 token · 全部 agent · 按本地日期"))
                    .font(.system(size: 10.5)).foregroundStyle(p.fg3)
            }
            HStack(alignment: .bottom, spacing: 8) {
                ForEach(Array(bars.enumerated()), id: \.offset) { _, b in
                    VStack(spacing: 3) {
                        Text(b.labelled ? hqCompactTok(b.out) : " ")
                            .font(.system(size: 9.5, design: .monospaced)).foregroundStyle(b.today ? p.fg : p.fg2)
                        RoundedRectangle(cornerRadius: 2, style: .continuous)
                            .fill(b.today ? p.fg2 : p.fg3.opacity(0.55))
                            .frame(height: max(2, 56 * b.frac))
                            .frame(maxWidth: .infinity)
                            .help("\(b.date) · \(hqCompactTok(b.out))")
                        Text(b.weekday).font(.system(size: 9.5)).foregroundStyle(b.today ? p.fg : p.fg3)
                    }
                }
            }
            .frame(height: 84)
            if let agents = h.byAgent, !agents.isEmpty {
                VStack(alignment: .leading, spacing: 3) {
                    ForEach(agents, id: \.agentKey) { a in
                        HStack(spacing: 10) {
                            Text(a.agentName ?? a.agentKey).font(.system(size: 11.5)).foregroundStyle(p.fg)
                                .frame(width: 150, alignment: .leading)
                            Spacer()
                            Text(l10n.tr("today \(hqCompactTok(a.todayOut))", "今天 \(hqCompactTok(a.todayOut))"))
                                .font(Theme.Font.mono).foregroundStyle(p.fg2).frame(width: 110, alignment: .trailing)
                            Text(l10n.tr("week \(hqCompactTok(a.weekOut))", "本周 \(hqCompactTok(a.weekOut))"))
                                .font(Theme.Font.mono).foregroundStyle(p.fg).frame(width: 110, alignment: .trailing)
                        }
                    }
                }
            }
        }
    }

    /// Who is burning it: sessions by trouble — the alerted first, then by burn rate,
    /// then by context share; the parked fold into one count row.
    @ViewBuilder private func usageBurn(_ u: HQUsageReport, _ p: Theme.Palette) -> some View {
        Divider()
        let ranked = (u.sessions ?? []).sorted { a, b in
            let aw = (a.usageWarn ?? "").isEmpty ? 0 : 1, bw = (b.usageWarn ?? "").isEmpty ? 0 : 1
            if aw != bw { return aw > bw }
            if a.rate != b.rate { return a.rate > b.rate }
            if (a.ctx ?? 0) != (b.ctx ?? 0) { return (a.ctx ?? 0) > (b.ctx ?? 0) }
            return (a.paneID ?? "") < (b.paneID ?? "")
        }
        let shown = ranked.filter { !($0.usageWarn ?? "").isEmpty || $0.rate > 0 }
        let rest = ranked.filter { ($0.usageWarn ?? "").isEmpty && $0.rate <= 0 }
        if !shown.isEmpty {
            VStack(alignment: .leading, spacing: 0) {
                ForEach(Array(shown.enumerated()), id: \.offset) { _, s in
                    HStack(spacing: 10) {
                        Text(s.paneID ?? "—").font(Theme.Font.mono).foregroundStyle(p.fg3).frame(width: 44, alignment: .leading)
                        VStack(alignment: .leading, spacing: 1) {
                            Text(s.loc ?? "").font(.system(size: 11.5)).foregroundStyle(p.fg).lineLimit(1)
                            if let w = s.usageWarn, !w.isEmpty {
                                Text(w).font(.system(size: 10.5)).foregroundStyle(Theme.Status.errored).lineLimit(1)
                            }
                        }
                        .frame(maxWidth: .infinity, alignment: .leading)
                        Text(hqCompactTok(s.tok)).font(Theme.Font.mono).foregroundStyle(p.fg2).frame(width: 60, alignment: .trailing)
                        Text(String(format: "%.0f/s", s.rate)).font(Theme.Font.mono).foregroundStyle(p.fg3).frame(width: 56, alignment: .trailing)
                        Text("ctx \(Int(((s.ctx ?? 0) * 100).rounded()))%").font(Theme.Font.mono).foregroundStyle(p.fg3).frame(width: 64, alignment: .trailing)
                    }
                    .padding(.vertical, 4)
                    .overlay(alignment: .top) { Divider().overlay(p.divider) }
                }
            }
        }
        if !rest.isEmpty {
            Text(l10n.tr("\(rest.count) parked sessions · \(hqCompactTok(rest.reduce(0) { $0 + $1.tok })) so far",
                         "\(rest.count) 个停着的会话 · 累计 \(hqCompactTok(rest.reduce(0) { $0 + $1.tok }))"))
                .font(.system(size: 11)).foregroundStyle(p.fg3)
        }
    }

    // MARK: knowledge

    @ViewBuilder private func knowledgeBody(_ p: Theme.Palette) -> some View {
        // LIST BESIDE DETAIL. This is a window with 640pt of width and the content was one
        // 12pt-padded column, so reading an entry meant leaving the list and coming back —
        // a screen change to answer "what does this one say?" (2026-09-09).
        //
        // The phone keeps its drill-in: it has no width to spare, and its answer to the
        // same problem is returning you to the list at the place you left.
        HStack(spacing: 0) {
            knowledgeList(p)
                .frame(width: 292)
            Divider()
            knowledgeDetail(p)
                .frame(maxWidth: .infinity)
        }
    }

    /// The right pane: the selected entry, or a line saying to pick one.
    @ViewBuilder private func knowledgeDetail(_ p: Theme.Palette) -> some View {
        switch pane {
        case .index:
            empty(l10n.tr("Pick an entry on the left", "在左边选一条"), p)
        case let .entry(id):
            if let e = store.entries.first(where: { $0.id == id }) {
                entryDetail(e, p)
            } else {
                // The entry went away under the reader (a retire, or HQ superseded it).
                // Saying so beats an empty pane with a back button.
                VStack(spacing: 0) {
                    topicLine(l10n.tr("Knowledge", "知识库"), p)
                    empty(l10n.tr("That entry is no longer live", "这条已不在有效集里"), p)
                }
            }
        }
    }

    @ViewBuilder private func knowledgeList(_ p: Theme.Palette) -> some View {
        let q = query.trimmingCharacters(in: .whitespaces)
        if !q.isEmpty {
            // Results REPLACE the index. The index is the browse answer; showing both is
            // two answers to one question.
            let hits = matchKB(store.entries, q)
            if hits.isEmpty {
                empty(l10n.tr("Nothing matches “\(q)”", "没有匹配「\(q)」的条目"), p)
            } else {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 0) {
                        sectionHead(l10n.tr("results", "结果"), hits.count, p, accent: false)
                        ForEach(hits) { e in row(e, p, showWhy: false) }
                    }
                    .padding(.bottom, 10)
                }
            }
        } else if store.entries.isEmpty && store.candidates.isEmpty {
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
                        // What a promotion IS, said where it matters. This base is one of
                        // THREE places a rule can live — gtmux's shipped charter, the
                        // operator's own LOCAL.md, and this machine's ledger — and a
                        // reader who takes it for all three files a lesson everyone needed
                        // on one machine. Same sentence as the phone's; the long form is
                        // docs/design/knowledge-layers.md.
                        Button {
                            whyOpen.toggle()
                        } label: {
                            Text(whyOpen
                                 ? l10n.tr("What this is ⌃", "这是什么 ⌃")
                                 : l10n.tr("What this is ⌄", "这是什么 ⌄"))
                                .font(.system(size: 11))
                                .foregroundStyle(p.fg2)
                        }
                        .buttonStyle(.plain)
                        .padding(.horizontal, 12)
                        .padding(.bottom, 6)
                        if whyOpen {
                            Text(l10n.tr(
                            "Entries HQ judged bigger than this machine. It has written the brief; carry each into somewhere durable — your LOCAL.md, a project’s AGENTS.md, a team runbook, or gtmux itself — then mark it landed.",
                            "这些是 HQ 判断「比这台机器大」的条目。它已写好带走简报,等你把它搬进一个持久的地方(你的 LOCAL.md、某个项目的 AGENTS.md、团队 runbook,或 gtmux 自己的仓库),再回来标记落地。"))
                            .font(.system(size: 11))
                            .foregroundStyle(p.fg3)
                            .fixedSize(horizontal: false, vertical: true)
                            .padding(.horizontal, 12)
                            .padding(.bottom, 6)
                        }
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
                        // What just landed, at a glance. A lesson recorded wrong is not
                        // inert — it is echoed into every dispatch — so spot-checking the
                        // recent writes is the cheapest way to catch one, and it is the
                        // question this list is opened with.
                        // The count is what this section SHOWS, not the size of the base.
                        // "NEWEST 396" over a list of twelve names the wrong thing; the
                        // total is already in the window's own header.
                        sectionHead(l10n.tr("newest", "最近"), min(KBRecentCount, store.entries.count), p, accent: false)
                        // What "newest" is, relative to the topics below it. These entries
                        // are ALSO in their topic — the topic counts add up to the whole
                        // base — and a reader looking at both lists asked, fairly, whether
                        // the recent ones were in a topic at all (2026-09-07). Same
                        // sentence as the phone's.
                        Text(l10n.tr("across every topic — each one also sits under its topic below",
                                     "跨全部主题 · 这几条同时也在下面各自的主题里"))
                            .font(.system(size: 11))
                            .foregroundStyle(p.fg3)
                            .padding(.horizontal, 12)
                            .padding(.bottom, 6)
                        ForEach(store.entries.prefix(KBRecentCount)) { e in row(e, p, showWhy: false) }

                        // And the whole base, BY TOPIC, folded. This list used to be all
                        // 386 entries in one run — deliberately uncapped, because "a cap
                        // is an entry the commander cannot retire". That reasoning still
                        // holds and is why the cap above is safe: every entry is here,
                        // inside its topic, so none has become unreachable. Seven rows
                        // instead of 386.
                        sectionHead(l10n.tr("topics", "主题"), store.topics.count, p, accent: false)
                        ForEach(store.topics) { t in
                            topicGroup(t, p)
                        }
                    }
                }
                .padding(.bottom, 10)
            }
        }
    }

    /// One topic, folded. Closed by default: the point of grouping is to see the whole
    /// vocabulary at once, and nothing here is the row you came for.
    @ViewBuilder private func topicGroup(_ t: KBTopic, _ p: Theme.Palette) -> some View {
        let shown = openTopics.contains(t.name)
        Button {
            if shown { openTopics.remove(t.name) } else { openTopics.insert(t.name) }
        } label: {
            HStack(spacing: 8) {
                Text(shown ? "▾" : "▸")
                    .font(.system(size: 11)).foregroundStyle(p.fg3).frame(width: 11, alignment: .leading)
                Text(t.name).font(.system(size: 12.5, weight: .medium)).foregroundStyle(p.fg)
                Spacer(minLength: 6)
                Text("\(t.entries.count)")
                    .font(.system(size: 10.5).monospacedDigit()).foregroundStyle(p.fg3)
            }
            .contentShape(Rectangle())
            .padding(.horizontal, 12)
            .padding(.vertical, 7)
        }
        .buttonStyle(.plain)
        Divider().overlay(p.divider)
        if shown {
            ForEach(t.entries) { e in row(e, p, showWhy: false) }
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
                Text(e.displayTitle(l10n.lang))
                    .font(.system(size: 12))
                    .foregroundStyle(p.fg)
                    .multilineTextAlignment(.leading)
                    .frame(maxWidth: .infinity, alignment: .leading)
                HStack(spacing: 6) {
                    if e.sensitive ?? false {
                        // The lock says "yours, kept here" — the same mark on every surface.
                        Image(systemName: "lock.fill").font(.system(size: 12)).foregroundStyle(p.fg3)
                            .help(l10n.tr("Sensitive — stays on this Mac", "敏感 —— 只留本机"))
                    }
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
                    if c.family > 0 {
                        // One lesson spread over several lines: the family number is
                        // what `knowledge add --capture k1,k2,…` consumes at once.
                        Text(l10n.tr("≈ family \(c.family)", "≈ 同一件事 \(c.family)"))
                            .font(.system(size: 10)).foregroundStyle(p.fg2)
                    }
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
            // No back control: the list is beside this, not behind it. The topic line
            // stays because it says WHERE this entry lives, which the list only implies.
            topicLine(e.topic, p)
            ScrollView {
                VStack(alignment: .leading, spacing: 10) {
                    Text(e.displayTitle(l10n.lang))
                        .font(.system(size: 13, weight: .semibold))
                        .foregroundStyle(p.fg)
                        .textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    Text(e.id).font(.system(size: 10, design: .monospaced)).foregroundStyle(p.fg3)
                        .textSelection(.enabled)
                    // The three axes, where the reader judges: what it is, where it came
                    // from and how often, who must know it.
                    let axes = e.axesLine(l10n)
                    if !axes.isEmpty {
                        Text(axes).font(.system(size: 10.5)).foregroundStyle(p.fg2)
                    }

                    if e.pending {
                        VStack(alignment: .leading, spacing: 3) {
                            Text(l10n.tr("PROMOTED · waiting on you", "已晋升 · 待你带走"))
                                .font(.system(size: 9.5, weight: .semibold)).tracking(0.8)
                                .foregroundStyle(Theme.Status.waiting)
                            if let why = e.promoteWhy, !why.isEmpty {
                                Text(why).font(.system(size: 11)).foregroundStyle(p.fg2)
                            }
                            let aud = audienceShort(e.audience, l10n)
                            if !aud.isEmpty {
                                Text(l10n.tr("for: ", "给：") + aud + ((e.audienceRepo ?? "").isEmpty ? "" : " · " + (e.audienceRepo ?? "")))
                                    .font(.system(size: 11)).foregroundStyle(p.fg3)
                            } else if let target = e.promoteTarget, !target.isEmpty {
                                Text("→ \(target)").font(.system(size: 11)).foregroundStyle(p.fg3)
                            } else {
                                Text(l10n.tr("no audience chosen — withdraw, then promote again saying who must know it",
                                             "没选读者 —— 撤回后重新晋升，说清给谁看"))
                                    .font(.system(size: 11)).foregroundStyle(p.fg3)
                            }
                        }
                    } else if let ref = e.landedRef, !ref.isEmpty {
                        Text(l10n.tr("✓ landed \(ref)", "✓ 已落地 \(ref)"))
                            .font(.system(size: 11, weight: .semibold))
                            .foregroundStyle(Theme.Status.idle)
                    }

                    if let body = e.resolved(l10n.lang).body, !body.isEmpty {
                        // Entries are written in markdown, the same as the board: tables of
                        // evidence, `code` for identifiers, and [[links]] to sibling
                        // entries. Printed raw it was the reader's job to parse them —
                        // and the links, which are the base's structure, read as noise.
                        MarkdownBody(markdown: body, p: p)
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

    /// The detail pane's own heading — where this entry lives.
    @ViewBuilder private func topicLine(_ title: String, _ p: Theme.Palette) -> some View {
        HStack(spacing: 6) {
            Text(title).font(.system(size: 11.5, weight: .medium)).foregroundStyle(p.fg2)
            Spacer()
        }
        .padding(.horizontal, 14).padding(.top, 10).padding(.bottom, 6)
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
            if case let .feedback(url) = act {
                // The everyone audience's exit is a browser, not a process: the issue
                // page is already prefilled from the brief.
                if let u = URL(string: url) { NSWorkspace.shared.open(u) }
                return
            }
            draft = ""
            actError = nil
            audience = .machine
            repoPath = ""
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
            if case .promote = pending.act {
                // Who must know it — the one question promote asks. Four answers, each
                // with an exit a person can actually take (D4).
                Picker(l10n.tr("Who must know it", "这条给谁看"), selection: $audience) {
                    ForEach(KnowledgeAudience.allCases, id: \.self) { a in
                        Text(a.word(l10n)).tag(a)
                    }
                }
                .pickerStyle(.radioGroup)
                .font(.system(size: 11))
                .disabled(busy)
                if audience == .repo {
                    TextField(l10n.tr("repository path, e.g. ~/work/api", "仓库路径，例如 ~/work/api"), text: $repoPath)
                        .textFieldStyle(.roundedBorder)
                        .font(.system(size: 12))
                        .disabled(busy)
                }
            }
            if pending.act.needsReason {
                TextField(c.placeholder, text: $draft, axis: .vertical)
                    .textFieldStyle(.roundedBorder)
                    .lineLimit(2...5)
                    .font(.system(size: 12))
                    .disabled(busy)
            }

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
                Text(("gtmux knowledge \(verbWord(pending.act)) \(c.field)").trimmingCharacters(in: .whitespaces))
                    .font(.system(size: 10, design: .monospaced)).foregroundStyle(p.fg3)
                Spacer(minLength: 8)
                Button(l10n.tr("Cancel", "取消")) { closeSheet() }
                    .disabled(busy)
                Button(l10n.tr("Confirm", "确认")) { run(pending) }
                    .keyboardShortcut(.defaultAction)
                    .disabled(busy || !canConfirm(pending))
            }
        }
        .padding(16)
        .frame(width: 420)
        .background(p.bg)
    }

    private func verbWord(_ act: KnowledgeAct) -> String {
        switch act {
        case .promote: return "promote"
        case .land, .carry: return "land"
        case .withdraw: return "withdraw"
        case .retire: return "retire"
        case .dismiss: return "dismiss"
        case .feedback: return ""
        }
    }

    /// The reason exists when one is needed, and a repository audience has its path.
    private func canConfirm(_ pending: PendingAct) -> Bool {
        if pending.act.needsReason && draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty { return false }
        if case .promote = pending.act, audience == .repo,
           repoPath.trimmingCharacters(in: .whitespaces).isEmpty { return false }
        return true
    }

    /// The act as it will run: a promote picks up the audience chosen in the sheet.
    private func resolved(_ act: KnowledgeAct) -> KnowledgeAct {
        guard case let .promote(id, _) = act else { return act }
        let path = repoPath.trimmingCharacters(in: .whitespaces)
        let forValue = audience == .repo ? "repo:" + (path as NSString).expandingTildeInPath : audience.rawValue
        return .promote(id: id, audience: forValue)
    }

    private func run(_ pending: PendingAct) {
        busy = true
        actError = nil
        store.perform(resolved(pending.act), reason: draft, l10n: l10n) { err in
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
        case let .text(t), let .code(t), let .bold(t), let .link(t): n += t.count
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
    @State private var openProse: Set<Int> = []

    var body: some View {
        LazyVStack(alignment: .leading, spacing: spacing) {
            ForEach(Array(blocks.enumerated()), id: \.offset) { i, b in
                if clampProse, case let .paragraph(spans) = b, mdPlainLength(spans) > MDProseClampChars {
                    let open = openProse.contains(i)
                    VStack(alignment: .leading, spacing: 2) {
                        spansText(spans, size: 12, weight: .regular)
                            .lineLimit(open ? nil : MDProseClampLines)
                        Button {
                            if open { openProse.remove(i) } else { openProse.insert(i) }
                        } label: {
                            Text(open ? "▴" : "▾").font(.system(size: 11)).foregroundStyle(p.fg3)
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
        case let .ordered(items, start):
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
            // No gap between CLOSED rows: they are separated by their own hairline, and
            // a gap as well would put each one back in a box of white space.
            VStack(alignment: .leading, spacing: foldRows ? 0 : 8) {
                ForEach(Array(rows.enumerated()), id: \.offset) { i, row in
                    TableCard(header: header, row: row, p: p, fold: foldRows, index: i)
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
        mdSpansText(spans, size: size, weight: weight, p: p)
    }

    private func plain(_ spans: [MDInline]) -> String { mdPlain(spans) }
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
                contentRect: NSRect(x: 0, y: 0, width: 820, height: 580),
                styleMask: [.titled, .closable, .resizable], backing: .buffered, defer: false)
            w.title = l10n.tr("gtmux HQ", "gtmux HQ")
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

/// Inline runs as one selectable Text: code in the monospace face the rest of the product
/// uses for identifiers, bold at a weight that does not compete with a heading (the board
/// carries roughly one bold span per line).
///
/// File scope because two views render spans now — the document blocks and the folding
/// table card — and a second copy is how the two would start to look different.
func mdSpansText(_ spans: [MDInline], size: CGFloat, weight: Font.Weight, p: Theme.Palette) -> Text {
    spans.reduce(Text("")) { acc, span in
        switch span {
        case let .text(t):
            return acc + Text(t).font(.system(size: size, weight: weight)).foregroundColor(p.fg)
        case let .code(t):
            return acc + Text(t).font(.system(size: size - 0.5, design: .monospaced)).foregroundColor(p.fg2)
        case let .bold(t):
            return acc + Text(t).font(.system(size: size, weight: .semibold)).foregroundColor(p.fg)
        case let .link(t):
            // An edge in the knowledge graph. Styled as one and stripped of its brackets:
            // the reader is looking at a reference, not at markup.
            return acc + Text(t).font(.system(size: size - 0.5)).foregroundColor(Theme.Status.working)
        }
    }
}

func mdPlain(_ spans: [MDInline]) -> String {
    spans.map { span in
        switch span {
        case let .text(t): return t
        case let .code(t): return t
        case let .bold(t): return t
        case let .link(t): return t
        }
    }.joined()
}
