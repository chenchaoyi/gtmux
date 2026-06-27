import AppKit
import SwiftUI

/// Status is the agent state language (DESIGN §1). Color is the ONLY status
/// channel (never used for agent identity); shape + glyph carry it too.
enum Status: String, CaseIterable {
    case waiting, working, idle, running

    var color: Color {
        switch self {
        case .waiting: return Theme.Status.waiting
        case .working: return Theme.Status.working
        case .idle:    return Theme.Status.idle
        case .running: return Theme.Status.none
        }
    }

    var nsColor: NSColor {
        switch self {
        case .waiting: return Theme.Status.waitingNS
        case .working: return Theme.Status.workingNS
        case .idle:    return Theme.Status.idleNS
        case .running: return Theme.Status.noneNS
        }
    }

    /// Section/sort order: needs-you → working → idle → running (DESIGN §3).
    var rank: Int {
        switch self {
        case .waiting: return 0
        case .working: return 1
        case .idle:    return 2
        case .running: return 3
        }
    }
}

/// One agent pane, consuming `gtmux agents --json` (DESIGN §14). Tolerates older
/// JSON without the source/native fields (decodeIfPresent).
struct Agent: Identifiable, Equatable {
    var paneID = ""
    var session = ""
    var window = ""
    var pane = ""
    var loc = ""
    var agent = ""
    var status = "running"
    var task = ""
    var latest = false
    var activity = false
    // native-terminal generalization (DESIGN §7)
    var source = "tmux"
    var project = ""
    var terminal = ""
    var tab = ""
    var activityAt = 0 // epoch seconds of last activity (for relative time); 0 = unknown
    var since = 0      // epoch seconds the current state began (for a "working 7m" duration)
    var icon = ""      // identity icon hint: a .app path (→ that app's real icon) or an image path

    var id: String {
        paneID.isEmpty ? "\(source):\(terminal):\(tab):\(project):\(agent)" : paneID
    }
    var state: Status { Status(rawValue: status) ?? .running }
    var isNative: Bool { source == "native" }

    /// Row line 1 (bold): the coding agent's OWN session name — the title it sets
    /// on its pane (what you see as the terminal title), NOT the tmux session or a
    /// cwd-derived project. Falls back to the tmux session / native project when
    /// the agent set no title yet.
    var primary: String {
        if !task.isEmpty { return task }
        if isNative { return project.isEmpty ? terminal : project }
        return session.isEmpty ? loc : session
    }
    /// Row line 2 (dim): where it lives — tmux "session · %pane", or the native
    /// terminal. This is context for the bold agent session name above it.
    var secondary: String {
        if isNative { return terminal }
        let base = session.isEmpty ? loc : session
        return paneID.isEmpty ? base : "\(base) · \(paneID)"
    }

    /// CLI args to jump to this agent (DESIGN §7): tmux by pane id, native by
    /// terminal app + tab title.
    func jumpArgs() -> [String] {
        if isNative { return ["focus", "--terminal", terminal, "--tab", tab] }
        return ["focus", paneID]
    }
}

extension Agent: Decodable {
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        func s(_ k: CodingKeys) -> String { (try? c.decode(String.self, forKey: k)) ?? "" }
        func b(_ k: CodingKeys) -> Bool { (try? c.decode(Bool.self, forKey: k)) ?? false }
        paneID = s(.paneID); session = s(.session); window = s(.window); pane = s(.pane)
        loc = s(.loc); agent = s(.agent); task = s(.task)
        status = (try? c.decode(String.self, forKey: .status)) ?? "running"
        latest = b(.latest); activity = b(.activity)
        source = (try? c.decode(String.self, forKey: .source)) ?? "tmux"
        project = s(.project); terminal = s(.terminal); tab = s(.tab)
        activityAt = (try? c.decode(Int.self, forKey: .activityAt)) ?? 0
        since = (try? c.decode(Int.self, forKey: .since)) ?? 0
        icon = s(.icon)
    }
    enum CodingKeys: String, CodingKey {
        case paneID = "pane_id"
        case session, window, pane, loc, agent, status, task, latest, activity
        case source, project, terminal, tab, icon, since
        case activityAt = "activity_at"
    }
}

/// AgentStore polls the CLI and publishes agents to SwiftUI. Plain (non-actor)
/// ObservableObject so AppKit reads it on the main thread; the @Published write
/// is marshaled to main by refresh().
final class AgentStore: ObservableObject {
    @Published private(set) var agents: [Agent] = []

    func refresh() {
        DispatchQueue.global(qos: .userInitiated).async {
            let data = GtmuxCLI.capture(["agents", "--json"]) ?? Data("[]".utf8)
            let decoded = (try? JSONDecoder().decode([Agent].self, from: data)) ?? []
            DispatchQueue.main.async { self.agents = decoded }
        }
    }

    /// Test seam: inject agents synchronously (used by unit tests).
    func setForTesting(_ a: [Agent]) { agents = a }

    // counts
    var total: Int { agents.count }
    var waiting: Int { agents.filter { $0.state == .waiting }.count }
    var working: Int { agents.filter { $0.state == .working }.count }
    var idleCount: Int { total - waiting - working } // idle + running, matches CLI summary

    /// Most-urgent overall state, for the status-bar glyph (DESIGN §2).
    var mostUrgent: Status {
        if waiting > 0 { return .waiting }
        if working > 0 { return .working }
        if agents.contains(where: { $0.state == .idle }) { return .idle }
        return .running // also the calm/none case when empty
    }

    /// Count next to the lit grid cell (ITERATIONS D1 / §02): the most-urgent
    /// state's count — waiting, else working, else the done (idle) count, else "".
    /// (Extends DESIGN §2's waiting-else-working: D1's lit-cell shows a done count
    /// too, so the green ✓ cell carries "how many finished".)
    var badge: String {
        if waiting > 0 { return "\(waiting)" }
        if working > 0 { return "\(working)" }
        let idle = agents.filter { $0.state == .idle }.count
        if idle > 0 { return "\(idle)" }
        return ""
    }

    /// Agents grouped into the four sections, in fixed rank order, each non-empty
    /// section only. Applies the waiting-only filter and fuzzy search.
    func sections(waitingOnly: Bool, query: String) -> [(status: Status, agents: [Agent])] {
        var out: [(Status, [Agent])] = []
        for st in [Status.waiting, .working, .idle, .running] {
            if waitingOnly && st != .waiting { continue }
            let rows = agents.filter { $0.state == st && matches($0, query) }
                .sorted { lhs, rhs in lhs.primary.localizedCaseInsensitiveCompare(rhs.primary) == .orderedAscending }
            if !rows.isEmpty { out.append((st, rows)) }
        }
        return out
    }

    /// Flattened, ordered agent list (for keyboard navigation).
    func ordered(waitingOnly: Bool, query: String) -> [Agent] {
        sections(waitingOnly: waitingOnly, query: query).flatMap { $0.agents }
    }

    /// Fuzzy match over session/project/window/task/agent/pane (DESIGN §4).
    func matches(_ a: Agent, _ query: String) -> Bool {
        let q = query.trimmingCharacters(in: .whitespaces).lowercased()
        if q.isEmpty { return true }
        let hay = [a.session, a.project, a.window, a.terminal, a.tab, a.task, a.agent, a.paneID]
            .joined(separator: " ").lowercased()
        return Self.fuzzy(q, in: hay)
    }

    /// Subsequence fuzzy match (all chars of needle appear in order).
    static func fuzzy(_ needle: String, in hay: String) -> Bool {
        if hay.contains(needle) { return true }
        var it = hay.makeIterator()
        for ch in needle {
            var found = false
            while let h = it.next() {
                if h == ch { found = true; break }
            }
            if !found { return false }
        }
        return true
    }
}
