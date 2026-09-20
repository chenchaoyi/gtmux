import Foundation
import SwiftUI

/// What the Diagnostics section and its window read: the same log store every gtmux
/// process writes, through the same command a person would run.
///
/// The app is a consumer of the CLI here as everywhere else — `gtmux logs --stats --json`
/// for the summary, `gtmux logs --json` for the entries — so the numbers on screen and
/// the numbers in a terminal cannot disagree. The app writes to this store too (DiagLog),
/// which is why reading it back through the CLI is the honest direction.

struct LogStats: Decodable {
    var bytes: Int64 = 0
    var files: Int = 0
    var oldest: String = ""
    var retainDays: Int = 30
    var maxBytes: Int64 = 0
    var windowHours: Int = 0
    var entries: Int = 0
    var warnings: Int = 0
    var errors: Int = 0
    var debug: String = ""

    /// Problems in the window the stats were asked for: what the row leads with when
    /// there are any.
    var problems: Int { warnings + errors }
}

struct LogEntry: Decodable, Identifiable {
    let ts: String
    let level: String
    let component: String
    let kind: String
    let event: String
    var actor: String?
    var target: String?
    var outcome: String?
    var msg: String?
    var attrs: [String: AttrValue]?

    // The store has no entry id; the line's position is the only stable identity, and it
    // is assigned on load rather than decoded.
    var id: Int = 0

    private enum CodingKeys: String, CodingKey {
        case ts, level, component, kind, event, actor, target, outcome, msg, attrs
    }

    var isProblem: Bool { level == "warn" || level == "error" }

    /// The clock, as the store wrote it: 2026-09-20T09:36:05.123+08:00 → 09:36.
    var clock: String {
        let parts = ts.split(separator: "T")
        guard parts.count == 2, parts[1].count >= 5 else { return ts }
        return String(parts[1].prefix(5))
    }

    var day: String { String(ts.split(separator: "T").first ?? "") }

    /// The line under the title: who did it, to what, and how it ended, then the attrs
    /// the entry carried. Entries are English by contract, so this joins rather than
    /// translates.
    var detail: String {
        var bits: [String] = []
        if let a = actor, !a.isEmpty { bits.append(a) }
        if let t = target, !t.isEmpty { bits.append("→ " + t) }
        if let o = outcome, !o.isEmpty { bits.append(o) }
        for (k, v) in (attrs ?? [:]).sorted(by: { $0.key < $1.key }) {
            bits.append("\(k)=\(v.text)")
        }
        return bits.joined(separator: " · ")
    }
}

/// An attribute value can be a string, a number or a bool; the store writes whichever the
/// caller passed. Decoding it as a JSON scalar keeps a number a number, so 5495510 does
/// not come back as 5.49551e+06 the way a double-only decode once printed it.
enum AttrValue: Decodable {
    case text(String)
    case number(Double)
    case flag(Bool)

    init(from decoder: Decoder) throws {
        let c = try decoder.singleValueContainer()
        if let b = try? c.decode(Bool.self) { self = .flag(b); return }
        if let d = try? c.decode(Double.self) { self = .number(d); return }
        self = .text((try? c.decode(String.self)) ?? "")
    }

    var text: String {
        switch self {
        case .text(let s): return s
        case .flag(let b): return b ? "true" : "false"
        case .number(let d):
            return d == d.rounded() && abs(d) < 1e15
                ? String(Int64(d)) : String(d)
        }
    }
}

@MainActor
final class DiagnosticsStore: ObservableObject {
    static let shared = DiagnosticsStore()

    @Published private(set) var stats = LogStats()
    @Published private(set) var entries: [LogEntry] = []
    @Published private(set) var loading = false
    @Published private(set) var packing = false
    /// The last pack's result: a path when it worked, an error when it did not.
    @Published var packed: String?
    @Published var packError: String?

    /// Extra detail is a config value every process reads at startup, so the toggle
    /// reflects the file, never a copy the app keeps.
    var debugOn: Bool { !stats.debug.isEmpty }

    /// The window both the row and the window read: today, so "2 problems" means today's.
    private var sinceToday: String {
        let f = DateFormatter()
        f.dateFormat = "yyyy-MM-dd"
        return f.string(from: Date())
    }

    func refreshStats() {
        let since = sinceToday
        Task.detached(priority: .utility) {
            let out = GtmuxCLI.capture(["logs", "--since", since, "--stats", "--json"])
            let s = out.flatMap { try? JSONDecoder().decode(LogStats.self, from: $0) }
            await MainActor.run { if let s { self.stats = s } }
        }
    }

    /// Three days, not everything: the window has to open at once, and a problem you are
    /// chasing is in the last few days or it is in a bundle.
    func load(problemsOnly: Bool) {
        loading = true
        var args = ["logs", "--since", "3d", "--json"]
        if problemsOnly { args += ["--level", "warn"] }
        Task.detached(priority: .userInitiated) {
            let out = GtmuxCLI.capture(args) ?? Data()
            let dec = JSONDecoder()
            var rows: [LogEntry] = []
            for (i, line) in String(decoding: out, as: UTF8.self).split(separator: "\n").enumerated() {
                if var e = try? dec.decode(LogEntry.self, from: Data(line.utf8)) {
                    e.id = i
                    rows.append(e)
                }
            }
            let ordered = Array(rows.reversed()) // newest first
            await MainActor.run {
                self.entries = ordered
                self.loading = false
            }
        }
    }

    func setDebug(_ on: Bool) {
        let before = stats
        stats.debug = on ? "all" : ""       // the switch answers at once
        Task.detached(priority: .utility) {
            let r = GtmuxCLI.captureFull(["config", "debug", on ? "on" : "off"])
            if r.status != 0 { await MainActor.run { self.stats = before } }
            await MainActor.run { self.refreshStats() }
        }
    }

    /// Packing a report is `gtmux doctor --bundle` with a path of our own, so the file
    /// lands somewhere a person can find it and the app can point at it.
    func pack() {
        guard !packing else { return }
        packing = true
        packed = nil
        packError = nil
        let f = DateFormatter()
        f.dateFormat = "yyyyMMdd-HHmm"
        let path = NSHomeDirectory() + "/Desktop/gtmux-diagnostics-" + f.string(from: Date()) + ".tgz"
        Task.detached(priority: .userInitiated) {
            let r = GtmuxCLI.captureFull(["doctor", "--bundle", path])
            await MainActor.run {
                self.packing = false
                if r.status == 0 {
                    self.packed = path
                } else {
                    self.packError = r.stderr.isEmpty
                        ? (r.status < 0 ? "gtmux could not be run" : "exit \(r.status)") : r.stderr
                }
            }
        }
    }

    func copyAll() {
        let text = entries.reversed().map { e in
            "\(e.ts) \(e.component) \(e.level) \(e.event) \(e.msg ?? "") \(e.detail)"
        }.joined(separator: "\n")
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(text, forType: .string)
    }

    func revealStore() {
        NSWorkspace.shared.selectFile(nil, inFileViewerRootedAtPath: Paths.logsDir)
    }
}
