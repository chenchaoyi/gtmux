import Combine
import Foundation

/// Cached feature data, read only for the selected visible workspace destination.
final class HQResourceStore: ObservableObject {
    @Published private(set) var board: BoardDoc?
    @Published private(set) var entries: [KBEntry] = []
    @Published private(set) var candidates: [KBCandidateGroup] = []
    @Published private(set) var resource: ResourceReport?
    @Published private(set) var paneNames: [String: String] = [:]
    @Published private(set) var usage: HQUsageReport?
    @Published private(set) var memory = HQMemoryState()
    @Published private(set) var refreshingPages: Set<HQResourcePage> = []
    @Published private(set) var loadedPages: Set<HQResourcePage> = []
    @Published private(set) var failedPages: Set<HQResourcePage> = []
    let knowledgeNavigation = HQKnowledgeNavigation()
    let usageNavigation = HQUsageNavigation()
    var topics: [KBTopic] { knowledgeTopics(entries) }
    var loading: Bool { refreshingPages.contains(.knowledge) }
    private(set) var activePage: HQResourcePage?
    var isPolling: Bool { timer != nil }
    private var timer: Timer?
    private let gates = Dictionary(
        uniqueKeysWithValues: HQResourcePage.allCases.map { ($0, AgentRefreshGate()) })
    private let read: ([String]) -> Data?

    init(read: @escaping ([String]) -> Data? = { GtmuxCLI.capturePoll($0) }) { self.read = read }
    deinit { timer?.invalidate() }

    func start(page: HQResourcePage) {
        guard activePage != page || timer == nil else { return }
        stop()
        activePage = page
        refresh()
        timer = Timer.scheduledTimer(withTimeInterval: 20, repeats: true) { [weak self] _ in
            self?.refresh()
        }
    }

    func stop() {
        timer?.invalidate()
        timer = nil
        activePage = nil
    }

    /// Each page has its own gate and snapshot. Late responses update only their
    /// own cache, never the selected destination's contents.
    func refresh() {
        guard let page = activePage, let gate = gates[page], gate.enter() else { return }
        refreshingPages.insert(page)
        let read = read
        DispatchQueue.global(qos: .userInitiated).async {
            let result = HQResourceRead(page: page, read: read)
            DispatchQueue.main.async {
                gate.leave()
                self.refreshingPages.remove(page)
                if result.hasPayload { self.loadedPages.insert(page) }
                if result.failed { self.failedPages.insert(page) } else { self.failedPages.remove(page) }
                if let doc = result.board,
                    self.board?.updatedAt != doc.updatedAt || self.board?.exists != doc.exists
                {
                    self.board = doc
                }
                if let rows = result.entries, Self.stamp(self.entries) != Self.stamp(rows) {
                    self.entries = rows
                }
                if let groups = result.candidates, Self.stamp(self.candidates) != Self.stamp(groups) {
                    self.candidates = groups
                }
                if let report = result.resource { self.resource = report }
                if let names = result.names, self.paneNames != names { self.paneNames = names }
                if let usage = result.usage { self.usage = usage }
                if let memory = result.memory { self.memory = memory }
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
        groups.map { "\($0.key)|\($0.at)|\($0.count)|\($0.family)|\($0.topic)|\($0.lesson)" }
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
                let msg =
                    home.stderr.isEmpty
                    ? l10n.tr(
                        "Couldn't find HQ's folder. Run gtmux hq once to create it.",
                        "找不到 HQ 的目录，先跑一次 gtmux hq 把它建好。")
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
            let msg =
                r.stderr.isEmpty
                ? l10n.tr("gtmux could not be run from \(path)", "无法在 \(path) 下执行 gtmux")
                : r.stderr
            DispatchQueue.main.async { done(msg) }
        }
    }
}

/// One bounded CLI read batch. Injected transport keeps fixtures away from real HQ.
private struct HQResourceRead {
    var board: BoardDoc?
    var entries: [KBEntry]?
    var candidates: [KBCandidateGroup]?
    var resource: ResourceReport?
    var names: [String: String]?
    var usage: HQUsageReport?
    var memory: HQMemoryState?
    var failed = false
    var hasPayload: Bool {
        board != nil || entries != nil || candidates != nil || resource != nil || usage != nil
    }

    init(page: HQResourcePage, read: ([String]) -> Data?) {
        func decode<T: Decodable>(_ args: [String]) -> T? {
            read(args).flatMap { try? JSONDecoder().decode(T.self, from: $0) }
        }
        switch page {
        case .board:
            board = decode(["hq", "--board", "--json"])
            failed = board == nil
        case .knowledge:
            let rows: [KBEntry]? = decode(["knowledge", "list", "--json"])
            let leads: [KBCandidate]? = decode(["capture", "--list", "--json"])
            entries = rows.map { Array($0.reversed()) }
            candidates = leads.map(groupCandidates)
            memory = read(["hq", "--records", "--json"]).flatMap(decodeHQMemoryState)
            failed = rows == nil || leads == nil || memory == nil
        case .machine:
            resource = decode(["resource", "--json"])
            let agents: [Agent]? = decode(["agents", "--json"])
            names = agents.map { rows in
                Dictionary(
                    rows.filter { !$0.paneID.isEmpty }.map { ($0.paneID, $0.primary) },
                    uniquingKeysWith: { _, last in last })
            }
            failed = resource == nil || agents == nil
        case .usage:
            usage = decode(["usage", "--json"])
            failed = usage == nil
        }
    }
}
