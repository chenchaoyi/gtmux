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
    enum Key: String { case machine, knowledge, board, usage, did }
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
    /// The subscription windows `gtmux usage --json` reports (limits.windows).
    var windows: [HQUsageWindow] = []
}

// MARK: usage

/// One subscription window as `gtmux usage --json` reports it under `limits.windows`.
struct HQUsageWindow: Decodable, Equatable {
    var label: String
    var pctUsed: Int
    var resetAt: String?
    var agent: String?
    var agentName: String?
    var resetUnix: Int64?
    enum CodingKeys: String, CodingKey {
        case label, agent
        case pctUsed = "pct_used", resetAt = "reset_at", agentName = "agent_name", resetUnix = "reset_unix"
    }
}

struct HQUsageSession: Decodable {
    var paneID: String?
    var loc: String?
    var agent: String?
    var agentKey: String
    var status: String?
    var tok: Int64
    var rate: Double
    var ctx: Double?
    var usageWarn: String?
    enum CodingKeys: String, CodingKey {
        case loc, agent, status, tok, rate, ctx
        case paneID = "pane_id", agentKey = "agent_key", usageWarn = "usage_warn"
    }
}

struct HQUsageType: Decodable {
    var agentKey: String
    var sessions: Int
    var tok: Int64
    var rate: Double
    enum CodingKeys: String, CodingKey { case sessions, tok, rate, agentKey = "agent_key" }
}

/// One local day of tokens across every agent (usage-daily-totals).
struct HQUsageDay: Decodable, Equatable {
    var date: String
    var out: Int64
    var `in`: Int64?
}

struct HQUsageAgentHistory: Decodable, Equatable {
    var agentKey: String
    var agentName: String?
    var todayOut: Int64
    var weekOut: Int64
    enum CodingKeys: String, CodingKey {
        case agentKey = "agent_key", agentName = "agent_name", todayOut = "today_out", weekOut = "week_out"
    }
}

/// Tokens by day: the last seven local days oldest first, and the two sums.
struct HQUsageHistory: Decodable, Equatable {
    var days: [HQUsageDay]?
    var todayOut: Int64?
    var weekOut: Int64?
    var byAgent: [HQUsageAgentHistory]?
    enum CodingKeys: String, CodingKey {
        case days, todayOut = "today_out", weekOut = "week_out", byAgent = "by_agent"
    }
}

/// The whole of `gtmux usage --json`, in the fields the reader shows.
struct HQUsageReport: Decodable {
    struct Limits: Decodable {
        var windows: [HQUsageWindow]?
        var warn: String?
    }
    var sessions: [HQUsageSession]?
    var types: [HQUsageType]?
    var limits: Limits?
    /// Absent from a CLI older than 1.0.23.
    var history: HQUsageHistory?
}

/// One bar of the seven-day chart, ready to draw — the phone's `DayBar`.
struct HQDayBar: Equatable {
    let date: String
    let out: Int64
    /// Height as a fraction of the tallest day, 0…1.
    let frac: Double
    /// Weekday initial in the reader's language.
    let weekday: String
    let today: Bool
    /// Direct label: today's bar, and the tallest day's.
    let labelled: Bool
}

/// hqDayBars turns the history into seven bars. A single neutral series (colour is a
/// status channel here) with direct labels on today and the tallest day only — a number
/// on every bar is the anti-pattern the dataviz method names.
func hqDayBars(_ days: [HQUsageDay], zh: Bool) -> [HQDayBar] {
    guard !days.isEmpty else { return [] }
    let maxOut = days.map { $0.out }.max() ?? 0
    let last = days[days.count - 1].date
    let tallest = days.max { $0.out < $1.out }
    let en = ["S", "M", "T", "W", "T", "F", "S"], zhs = ["日", "一", "二", "三", "四", "五", "六"]
    let f = DateFormatter()
    f.dateFormat = "yyyy-MM-dd"
    f.locale = Locale(identifier: "en_US_POSIX")
    return days.map { d in
        var wd = ""
        if let date = f.date(from: d.date) {
            let i = Calendar.current.component(.weekday, from: date) - 1
            wd = (zh ? zhs : en)[max(0, min(6, i))]
        }
        let isToday = d.date == last
        return HQDayBar(date: d.date, out: d.out, frac: maxOut > 0 ? Double(d.out) / Double(maxOut) : 0,
                        weekday: wd, today: isToday, labelled: isToday || (d == tallest && d.out > 0))
    }
}

/// hqWindowName strips the agent prefix the core puts on a label ("claude week (all
/// models)" → "week (all models)"), the phone's `planByAgent` rule.
func hqWindowName(_ w: HQUsageWindow) -> String {
    if let a = w.agent, !a.isEmpty, w.label.hasPrefix(a + " ") {
        return String(w.label.dropFirst(a.count + 1))
    }
    return w.label
}

/// hqPlanLabel is the short form the card's usage row uses — `claude wk`, `codex 5h`,
/// `claude Fable` — ported from the phone's `planLabel`/`compactWindow` so both surfaces
/// say the same thing about the same window.
func hqPlanLabel(_ w: HQUsageWindow, zh: Bool) -> String {
    let win = hqWindowName(w)
    var short: String
    if win.contains("all models") {
        short = zh ? "周" : "wk"
    } else if let open = win.firstIndex(of: "("), let close = win.firstIndex(of: ")"), open < close {
        let inner = String(win[win.index(after: open)..<close])
        short = inner.prefix(1).uppercased() + inner.dropFirst()
    } else if win.hasPrefix("session") {
        short = "5h"
    } else if win.hasPrefix("week") {
        short = zh ? "周" : "wk"
    } else {
        short = win
    }
    if let a = w.agent, !a.isEmpty { return a + " " + short }
    return short
}

/// hqTightestPerPlan keeps one window per plan — the one that runs out first — the
/// phone's `tightestPerPlan`: the row answers "where am I standing", and the tightest is
/// the answer; every window would truncate mid-number.
func hqTightestPerPlan(_ windows: [HQUsageWindow]) -> [HQUsageWindow] {
    var at: [String: Int] = [:]
    var out: [HQUsageWindow] = []
    for w in windows {
        let key = (w.agent?.isEmpty == false) ? w.agent! : w.label
        if let i = at[key] {
            if w.pctUsed > out[i].pctUsed { out[i] = w }
        } else {
            at[key] = out.count
            out.append(w)
        }
    }
    return out
}

private func usageRow(_ i: HQReportInput, zh: Bool) -> HQReportRow? {
    let wins = hqTightestPerPlan(i.windows)
    if wins.isEmpty { return nil }
    let value = wins.map { "\(hqPlanLabel($0, zh: zh)) \($0.pctUsed)%" }.joined(separator: " · ")
    return HQReportRow(key: .usage, label: zh ? "用量" : "usage", value: value, tone: .plain, door: .usage, wraps: false)
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
    if let u = usageRow(i, zh: zh) { rows.append(u) }
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
    @Published private(set) var windows: [HQUsageWindow] = []

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
            // `gtmux usage --json` reads the cached probe (limits.json); it does not run
            // the agent's /usage command itself.
            var windows: [HQUsageWindow] = []
            if let d = GtmuxCLI.capture(["usage", "--json"]),
               let u = try? JSONDecoder().decode(HQUsageReport.self, from: d) {
                windows = u.limits?.windows ?? []
            }
            DispatchQueue.main.async {
                if self.windows != windows { self.windows = windows }
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
                      boardUpdatedAt: boardUpdatedAt, acts: acts, windows: windows)
    }
}

/// hqCompactTok is a token count as a reader wants it: 2.9M, 830k, 412 — the phone's
/// `compactTok`, so a number reads the same on both screens.
func hqCompactTok(_ n: Int64) -> String {
    if n >= 1_000_000 { return String(format: "%.1fM", Double(n) / 1_000_000) }
    if n >= 1_000 { return "\(Int((Double(n) / 1000).rounded()))k" }
    return String(n)
}
