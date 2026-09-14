import Foundation

// HQCardReport — the chief-of-staff report the HQ card expands into (menubar-hq-report).
//
// The rows are the phone's (`mobileapp/src/screens/hqHeaderModel.ts`): the same keys, the
// same absence rule (a row with nothing to say is not drawn), the same tones. This file
// is the judgment — what goes on the card and in which colour — kept pure so it can be
// pinned in both languages; `MenuView` only draws what it is handed.

/// The colour a row may take. Red is the machine at the red tier; amber is the one row
/// that may owe you something past its line. Nothing else on the card is coloured.
enum HQRowTone: Equatable {
    case plain, amber, red
}

struct HQReportRow: Equatable {
    enum Key: String { case machine, knowledge, board, did }
    let key: Key
    /// The key column, in the reader's language.
    let label: String
    let value: String
    let tone: HQRowTone
    /// The reader tab this row opens, when it is a door.
    let door: HQReaderTab?
    /// May take a second line. Only the machine row at red: a truncated reading is worse
    /// than a wrapped one, and that row is the reason the card opened.
    let wraps: Bool
}

/// One kind of act with its 24-hour count, in the fixed order the phone's tally uses.
struct HQActTally: Equatable {
    let kind: String
    let n: Int
}

/// What the report is built from. Optionals mean "not loaded yet"; a missing input hides
/// its row rather than printing a zero.
struct HQReportInput {
    var machine: ResourceReport.Machine?
    var orphans = 0
    /// The knowledge base's live entry count. nil until read.
    var entries: Int?
    /// Promoted-and-not-yet-landed entries — the debt only the commander can settle.
    var owed = 0
    var owedOldestSecs: Int64?
    /// When HQ last wrote the board. nil = no board.
    var boardUpdatedAt: Int64?
    var acts: [HQActTally] = []
}

/// The phone's `PROMOTION_STALE_SECS`: two weeks, the line `gtmux doctor` uses. Only past
/// it does the knowledge row turn amber — work in the queue is normal, work rotting is not.
let hqOwedStaleSecs: Int64 = 14 * 24 * 3600

/// hqReportRows builds the table. Order: the machine leads only when it is red (it is
/// then the reason the card opened); otherwise knowledge · board · machine · did.
func hqReportRows(_ i: HQReportInput, zh: Bool) -> [HQReportRow] {
    var rows: [HQReportRow] = []
    let machine = machineRow(i, zh: zh)
    if let machine, machine.tone == .red { rows.append(machine) }
    if let k = knowledgeRow(i, zh: zh) { rows.append(k) }
    if let b = boardRow(i, zh: zh) { rows.append(b) }
    if let machine, machine.tone != .red { rows.append(machine) }
    if let d = didRow(i, zh: zh) { rows.append(d) }
    return rows
}

private func machineRow(_ i: HQReportInput, zh: Bool) -> HQReportRow? {
    guard let m = i.machine else { return nil }
    var parts: [String] = []
    if let mem = m.memFreePct {
        let tier: String
        switch m.memTier ?? "" {
        case "critical": tier = zh ? "（临界）" : " (critical)"
        case "warn": tier = zh ? "（警戒）" : " (warn)"
        default: tier = ""
        }
        parts.append(zh ? "内存 \(mem)% 空闲\(tier)" : "memory \(mem)% free\(tier)")
    }
    if let disk = m.diskUsePct {
        let free = m.diskFreeGB.map { zh ? "（剩 \($0) GB）" : " (\($0) GB left)" } ?? ""
        parts.append(zh ? "磁盘 \(disk)% 已用\(free)" : "disk \(disk)% used\(free)")
    }
    if let load = m.loadRatio, let n = m.ncpu {
        parts.append((zh ? "负载 " : "load ") + String(format: "%.1f×%d", load, n))
    }
    if let b = m.battery, b.present, !b.onAC {
        parts.append(zh ? "电池 \(b.percent)%" : "battery \(b.percent)%")
    }
    if i.orphans > 0 {
        parts.append(zh ? "\(i.orphans) 个孤儿进程可回收" : "\(i.orphans) orphan\(i.orphans == 1 ? "" : "s") reclaimable")
    }
    guard !parts.isEmpty else { return nil }
    let tone: HQRowTone
    switch m.tier ?? "" {
    case "red": tone = .red
    case "amber": tone = .amber
    default: tone = .plain
    }
    return HQReportRow(key: .machine, label: zh ? "机器" : "machine", value: parts.joined(separator: " · "),
                       tone: tone, door: .machine, wraps: tone == .red)
}

private func knowledgeRow(_ i: HQReportInput, zh: Bool) -> HQReportRow? {
    let n = i.entries ?? 0
    if n == 0 && i.owed == 0 { return nil } // no base: no row, rather than a row saying zero
    var value = zh ? "\(n) 条" : "\(n) entries"
    var tone = HQRowTone.plain
    if i.owed > 0 {
        value += zh ? " · \(i.owed) 条待你带走" : " · \(i.owed) waiting on you"
        if let old = i.owedOldestSecs, old > 0 {
            value += zh ? " · 最久 \(hqAgeWord(old, zh: true))" : " · oldest \(hqAgeWord(old, zh: false))"
            if old >= hqOwedStaleSecs { tone = .amber }
        }
    }
    return HQReportRow(key: .knowledge, label: zh ? "知识库" : "knowledge", value: value,
                       tone: tone, door: .knowledge, wraps: false)
}

private func boardRow(_ i: HQReportInput, zh: Bool) -> HQReportRow? {
    guard let at = i.boardUpdatedAt, at > 0 else { return nil }
    let age = boardAgeText(Int64(Date().timeIntervalSince1970) - at, zh: zh)
    return HQReportRow(key: .board, label: zh ? "态势板" : "board", value: age,
                       tone: .plain, door: .board, wraps: false)
}

private func didRow(_ i: HQReportInput, zh: Bool) -> HQReportRow? {
    let top = i.acts.filter { $0.n > 0 }.prefix(3)
    if top.isEmpty { return nil }
    let body = top.map { "\(hqActVerb($0.kind, zh: zh)) \($0.n)" }.joined(separator: " · ")
    return HQReportRow(key: .did, label: zh ? "它做了" : "HQ did", value: (zh ? "24 小时：" : "24h: ") + body,
                       tone: .plain, door: nil, wraps: false)
}

/// "16d" / "16 天" — the phone's `relTime` unit ladder, worded per language.
func hqAgeWord(_ secs: Int64, zh: Bool) -> String {
    if secs < 3600 { return zh ? "\(max(secs, 60) / 60) 分钟" : "\(max(secs, 60) / 60)m" }
    if secs < 86400 { return zh ? "\(secs / 3600) 小时" : "\(secs / 3600)h" }
    return zh ? "\(secs / 86400) 天" : "\(secs / 86400)d"
}

// MARK: acts

/// The verbs, ported from the phone's `hqActsModel.ts` so one act is one word on both
/// screens. A kind gtmux has not named yet falls back to its last segment, hyphens opened.
func hqActVerb(_ kind: String, zh: Bool) -> String {
    switch kind {
    case "gtmux:audit:send": return zh ? "派活" : "dispatched"
    case "gtmux:audit:reap": return zh ? "回收" : "reclaimed"
    case "gtmux:audit:knowledge": return zh ? "记账" : "recorded"
    case "gtmux:audit:rotate": return zh ? "轮换" : "rotated"
    case "gtmux:audit:hq-session": return zh ? "换班" : "handed over"
    case "gtmux:self-check": return zh ? "自审" : "self-audit"
    case "gtmux:distill": return zh ? "沉淀" : "distilled"
    case "gtmux:wake-degraded": return zh ? "唤醒通道异常" : "wake channel degraded"
    default:
        let tail = kind.split(separator: ":").last.map(String.init) ?? kind
        return tail.replacingOccurrences(of: "-", with: " ")
    }
}

/// The strip's order is FIXED, not by count (the phone's `tallyOrder`): the most numerous
/// act is not the most consequential, and a row that reorders as numbers move is re-read
/// every glance. Alarms lead.
private let hqActOrder = [
    "gtmux:wake-degraded",
    "gtmux:audit:send",
    "gtmux:audit:reap",
    "gtmux:audit:knowledge",
    "gtmux:self-check",
    "gtmux:distill",
    "gtmux:audit:rotate",
    "gtmux:audit:hq-session",
]

/// hqActTally counts each kind of act inside the window. The plumbing (`wake-delivered` /
/// `wake-dropped`) is already gone by the time this sees the records — `gtmux events
/// --acts` drops it — but the same partition is applied here too, so a stream read some
/// other way cannot count the knocks as work.
func hqActTally(_ records: [(kind: String, ts: Int64)], now: Int64, windowSecs: Int64) -> [HQActTally] {
    var by: [String: Int] = [:]
    for r in records where r.kind.hasPrefix("gtmux:") && now - r.ts <= windowSecs {
        if r.kind == "gtmux:audit:wake-delivered" || r.kind == "gtmux:audit:wake-dropped" { continue }
        by[r.kind, default: 0] += 1
    }
    let rank = { (k: String) -> Int in hqActOrder.firstIndex(of: k) ?? hqActOrder.count }
    return by.map { HQActTally(kind: $0.key, n: $0.value) }
        .sorted { rank($0.kind) != rank($1.kind) ? rank($0.kind) < rank($1.kind) : $0.kind < $1.kind }
}

// MARK: opening

/// hqCardShouldAutoOpen: the expansion opens itself the moment the card turns red —
/// once, on ENTERING an attention state. Leaving it does not close the card (the reader
/// may still be reading), and staying in it does not reopen what they closed.
func hqCardShouldAutoOpen(from old: HQState, to new: HQState) -> Bool {
    let attention: (HQState) -> Bool = { $0 == .hqCall || $0 == .needsYou || $0 == .resource }
    return attention(new) && !attention(old)
}

// MARK: store

/// One journal line of `gtmux events --json` — the two fields the tally needs.
private struct HQEventLine: Decodable {
    let event: String
    let ts: Int64
}

/// HQCardStore reads what the expansion shows, and only while it is open (design D4):
/// the promotion queue, the entry count, the board's timestamp, and the last day's acts.
/// Collapsed, the card costs nothing beyond the resource poll the medallion already runs.
final class HQCardStore: ObservableObject {
    @Published private(set) var entries: Int?
    @Published private(set) var owed = 0
    @Published private(set) var owedOldestSecs: Int64?
    @Published private(set) var boardUpdatedAt: Int64?
    @Published private(set) var acts: [HQActTally] = []

    private var timer: Timer?

    func start() {
        refresh()
        timer?.invalidate()
        // A minute: the board is rewritten a few times an hour, the ledger when HQ
        // distills, the acts as they happen — none of it a per-second matter.
        timer = Timer.scheduledTimer(withTimeInterval: 60, repeats: true) { [weak self] _ in self?.refresh() }
    }

    func stop() {
        timer?.invalidate()
        timer = nil
    }

    /// Every read goes through the CLI and runs from the app's own cwd — NEVER the HQ
    /// home — so the events read is a bystander's and cannot advance HQ's watermark.
    func refresh() {
        DispatchQueue.global(qos: .utility).async {
            let now = Int64(Date().timeIntervalSince1970)
            var count: Int?
            if let d = GtmuxCLI.capture(["knowledge", "list", "--json"]),
               let rows = try? JSONDecoder().decode([KBEntry].self, from: d) {
                count = rows.count
            }
            var owed = 0
            var oldest: Int64?
            if let d = GtmuxCLI.capture(["knowledge", "promotions", "--json"]),
               let rows = try? JSONDecoder().decode([KBEntry].self, from: d) {
                let pending = rows.filter { $0.pending }
                owed = pending.count
                if let first = pending.compactMap({ $0.promotedAt }).min() { oldest = now - first }
            }
            var board: Int64?
            if let d = GtmuxCLI.capture(["hq", "--board", "--json"]),
               let doc = try? JSONDecoder().decode(BoardDoc.self, from: d), doc.exists {
                board = doc.updatedAt
            }
            var tally: [HQActTally] = []
            if let d = GtmuxCLI.capture(["events", "--since", "24h", "--acts", "--json"]),
               let text = String(data: d, encoding: .utf8) {
                let lines = text.split(separator: "\n").compactMap {
                    try? JSONDecoder().decode(HQEventLine.self, from: Data($0.utf8))
                }
                tally = hqActTally(lines.map { (kind: $0.event, ts: $0.ts) }, now: now, windowSecs: 24 * 3600)
            }
            DispatchQueue.main.async {
                if self.entries != count { self.entries = count }
                if self.owed != owed { self.owed = owed }
                if self.owedOldestSecs != oldest { self.owedOldestSecs = oldest }
                if self.boardUpdatedAt != board { self.boardUpdatedAt = board }
                if self.acts != tally { self.acts = tally }
            }
        }
    }

    /// The report's inputs, joined with the machine snapshot the radar store already holds.
    func input(resource: ResourceReport?) -> HQReportInput {
        HQReportInput(machine: resource?.machine, orphans: resource?.orphans?.count ?? 0,
                      entries: entries, owed: owed, owedOldestSecs: owedOldestSecs,
                      boardUpdatedAt: boardUpdatedAt, acts: acts)
    }
}
