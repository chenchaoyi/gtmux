import AppKit
import SwiftUI

/// HQState is the supervisor's at-a-glance state on the HQ medallion — the SAME
/// six-state model as the mobile HQ disc (MOBILE §17), so the two surfaces read the
/// supervisor identically. Resolved by `AgentStore.hqState(...)`.
enum HQState: Equatable {
    case absent    // no supervisor session (grey, start affordance)
    case hqCall    // the supervisor itself is waiting on you (red + "!")
    case needsYou  // ≥1 worker is waiting (red + count)
    case resource  // the machine is at the RED resource tier (red + "⚠")
    case working   // the supervisor is processing a turn (cyan)
    case normal    // all quiet (green)
}

/// The `gtmux resource --json` machine snapshot (only the fields the medallion needs).
/// `tier` is omitempty in Go → optional here (a missing tier decodes to nil = healthy).
struct ResourceReport: Decodable {
    struct Machine: Decodable { var tier: String? }
    var machine: Machine
}

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
// RestorePlan mirrors `gtmux restore --plan --json` — the sessions the last
// resurrect save would bring back, each with the agent conversations under it. The
// menu-bar restore row expands into this. Read-only; the CLI is the source of truth.
struct RestorePlan: Codable, Equatable {
    var savePath = ""
    var sessions: [RestorePlanSession] = []

    var agentCount: Int { sessions.reduce(0) { $0 + $1.agents.count } }
}

struct RestorePlanSession: Codable, Equatable, Identifiable {
    var name = ""
    var windows = 0
    var panes = 0
    var agents: [RestorePlanAgent] = []
    var id: String { name }
}

struct RestorePlanAgent: Codable, Equatable, Identifiable {
    var agent = ""
    var loc = ""
    var cwd = ""
    var sessionId = ""
    var goal = ""
    var alive = true
    var id: String { loc + sessionId }

    /// What to show as the conversation's label: its goal, else the directory.
    var label: String { goal.isEmpty ? cwd : goal }
}

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
    // "supervisor" marks the hq (中控) session — rendered as its OWN layer (the HQ
    // card above the sections), never stacked inside the status groups. "" = normal.
    var role = ""
    var project = ""
    var terminal = ""
    var tab = ""
    var activityAt = 0 // epoch seconds of last activity (for relative time); 0 = unknown
    var since = 0      // epoch seconds the current state began (for a "working 7m" duration)
    var icon = ""      // identity icon hint: a .app path (→ that app's real icon) or an image path
    // native (source=="native") only: the agent session id (adopt key) + whether
    // it can be adopted into tmux (resumable).
    var sessionID = ""
    var adoptable = false
    // errored-idle modifier: this idle session ended on an API/tool error. Surfaces
    // mark it with an amber ⚠ (NOT red — red is waiting). false = finished normally.
    var errored = false
    var errorText = ""
    // background-running modifier: this idle session's turn ended with background
    // work (a run_in_background shell, …) still in flight. Marked with an amber ⧗
    // (NOT red). false = truly finished. bgCount = how many; bgText = a short label.
    var bg = false
    var bgCount = 0
    var bgText = ""
    // watched (tiered-pane-control): this row is a user-promoted PLAIN pane, not a
    // coding agent — no agent status. Rendered in its OWN "watched" section below the
    // agent sections, visually distinct. false = a normal agent.
    var watched = false

    var id: String {
        if !paneID.isEmpty { return paneID }
        if isNative && !sessionID.isEmpty { return "native:\(sessionID)" }
        return "\(source):\(terminal):\(tab):\(project):\(agent)"
    }
    var state: Status { Status(rawValue: status) ?? .running }
    var isNative: Bool { source == "native" }
    var isSupervisor: Bool { role == "supervisor" }

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
        role = s(.role)
        project = s(.project); terminal = s(.terminal); tab = s(.tab)
        activityAt = (try? c.decode(Int.self, forKey: .activityAt)) ?? 0
        since = (try? c.decode(Int.self, forKey: .since)) ?? 0
        icon = s(.icon)
        sessionID = s(.sessionID); adoptable = b(.adoptable)
        errored = b(.errored); errorText = s(.errorText)
        bg = b(.bg); bgCount = (try? c.decode(Int.self, forKey: .bgCount)) ?? 0; bgText = s(.bgText)
        watched = b(.watched)
    }
    enum CodingKeys: String, CodingKey {
        case paneID = "pane_id"
        case session, window, pane, loc, agent, status, task, latest, activity, watched
        case source, role, project, terminal, tab, icon, since, adoptable, bg
        case activityAt = "activity_at"
        case sessionID = "session_id"
        case errored = "error"
        case errorText = "error_text"
        case bgCount = "bg_count"
        case bgText = "bg_text"
    }
}

/// AgentStore polls the CLI and publishes agents to SwiftUI. Plain (non-actor)
/// ObservableObject so AppKit reads it on the main thread; the @Published write
/// is marshaled to main by refresh().
final class AgentStore: ObservableObject {
    @Published private(set) var agents: [Agent] = []

    /// The restore plan — what `gtmux restore` would bring back — populated ONLY when
    /// nothing is running (the "just rebooted" moment the restore row appears), so the
    /// footer row can expand into the actual pending sessions. nil the rest of the time.
    @Published private(set) var restorePlan: RestorePlan?

    /// The machine's resource severity tier ("" | "amber" | "red"), from a slow
    /// `gtmux resource --json` poll. Feeds the HQ medallion's `resource` state — which
    /// reddens ONLY on "red" (a genuine bottleneck), never a soft "amber" (低噪, in
    /// parity with the mobile HQ disc).
    @Published private(set) var machineTier: String = ""

    func refresh() {
        DispatchQueue.global(qos: .userInitiated).async {
            let data = GtmuxCLI.capture(["agents", "--json"]) ?? Data("[]".utf8)
            let decoded = (try? JSONDecoder().decode([Agent].self, from: data)) ?? []
            // Fetch the restore plan on the same tick, but only when the fleet is
            // empty — that's the only moment the restore row shows, and it keeps the
            // extra shell-out off the hot path when agents are running.
            var plan: RestorePlan?
            if decoded.isEmpty {
                if let pd = GtmuxCLI.capture(["restore", "--plan", "--json"]) {
                    plan = try? JSONDecoder().decode(RestorePlan.self, from: pd)
                }
            }
            DispatchQueue.main.async {
                self.agents = decoded
                self.restorePlan = decoded.isEmpty ? plan : nil
            }
        }
    }

    /// Poll the machine resource tier (a SECOND, slow shell-out — the machine changes on
    /// a human/hardware cadence, not per fast tick). Driven by its own ~25s timer, not
    /// the fast agents refresh. `gtmux resource --json` is cheap (df/sysctl/ps) and does
    /// NOT trigger the expensive `/usage` command.
    func refreshResource() {
        DispatchQueue.global(qos: .utility).async {
            let tier = (GtmuxCLI.capture(["resource", "--json"])
                .flatMap { try? JSONDecoder().decode(ResourceReport.self, from: $0) }?
                .machine.tier) ?? ""
            DispatchQueue.main.async { self.machineTier = tier }
        }
    }

    /// Test seam: inject agents synchronously (used by unit tests).
    func setForTesting(_ a: [Agent]) { agents = a }

    /// Test seam: inject the resource tier synchronously.
    func setTierForTesting(_ t: String) { machineTier = t }

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

    /// The status-bar count (DESIGN §2 / HANDOFF P0.4): the most-urgent ACTIONABLE
    /// state's count — waiting, else working, else NOTHING. The done (idle) state
    /// carries NO count: its ✓ glyph already says "your turn", and a number there just
    /// adds noise to the calm state (this reverts the earlier ITERATIONS-D1 done-count
    /// per the newer HANDOFF authority). Counts INCLUDE HQ — the supervisor is one more
    /// thing that can be waiting on you (waiting/working don't exclude it).
    var badge: String {
        if waiting > 0 { return "\(waiting)" }
        if working > 0 { return "\(working)" }
        return ""
    }

    /// Agents grouped into the four sections, in fixed rank order, each non-empty
    /// section only. Applies fuzzy search.
    func sections(query: String) -> [(status: Status, agents: [Agent])] {
        var out: [(Status, [Agent])] = []
        for st in [Status.waiting, .working, .idle, .running] {
            // Native (non-tmux) sessions are their own category, not mixed into the
            // tmux status groups.
            // The supervisor renders as its own HQ card, never inside the sections.
            // Watched (opt-in plain) panes render in their OWN section below, not here.
            let group = agents.filter { $0.state == st && !$0.isNative && !$0.isSupervisor && !$0.watched && matches($0, query) }
            // Finished (idle): most-recently-finished first (its `since` is frozen at
            // last activity, so order stays stable). Other sections: by name.
            let rows = st == .idle
                ? group.sorted { $0.since > $1.since }
                : group.sorted { $0.primary.localizedCaseInsensitiveCompare($1.primary) == .orderedAscending }
            if !rows.isEmpty { out.append((st, rows)) }
        }
        return out
    }

    /// The live supervisor (中控) session, if any — rendered as the HQ card above
    /// the sections (its own layer, per the hq-presentation change).
    var supervisor: Agent? { agents.first { $0.isSupervisor } }

    /// Non-supervisor, non-native sessions waiting on you — the count that drives the HQ
    /// medallion's `needsYou` state (mirrors the mobile disc's worker filter).
    var workerWaiting: Int {
        agents.filter { !$0.isSupervisor && !$0.isNative && $0.state == .waiting }.count
    }

    /// The HQ medallion's state — the same six-state model + priority as the mobile HQ
    /// disc (MOBILE §17). Resource reddens ONLY on the RED tier (a soft amber does not).
    var hqState: HQState {
        AgentStore.hqState(supervisor: supervisor, waiting: workerWaiting, resourceCritical: machineTier == "red")
    }

    /// Pure HQ-state resolver (priority-ordered) — mirrors the mobile `discState` so both
    /// surfaces read the supervisor identically. Static + pure for unit testing.
    static func hqState(supervisor: Agent?, waiting: Int, resourceCritical: Bool) -> HQState {
        guard let hq = supervisor else { return .absent }
        if hq.state == .waiting { return .hqCall }   // HQ's own call is top
        if waiting > 0 { return .needsYou }          // a worker waiting outranks resource
        if resourceCritical { return .resource }     // a genuine bottleneck (red tier only)
        if hq.state == .working { return .working }
        return .normal
    }

    /// The HQ intelligence headline's KIND — a pure, testable classification of the
    /// chief-of-staff conclusion the view renders as a sentence. Extracted from the view
    /// so it is unit-testable AND can't silently disagree with the medallion color: EVERY
    /// red state (.hqCall / .needsYou / .resource) maps to a non-`allNormal` kind, so the
    /// row never shows red next to "all normal — nothing needs you". Fixes the reported
    /// contradiction where a red resource tier still read "all normal".
    enum FleetHeadline: Equatable {
        case call                              // HQ itself needs your decision
        case worker(name: String, others: Int) // exactly one worker waiting on you
        case workers(Int)                      // N workers waiting on you
        case resource                          // machine under resource pressure (red tier)
        case allNormal                         // a quiet fleet
    }

    /// Pure resolver for the HQ headline (hq-meta-layer). Priority mirrors hqState: HQ
    /// call > worker(s) waiting > resource pressure > all normal. Keying the resource
    /// branch HERE is what stops the "red row + 'all normal' text" divergence.
    static func fleetHeadline(state: HQState, agents: [Agent]) -> FleetHeadline {
        if state == .hqCall { return .call }
        let workers = agents.filter { !$0.isSupervisor && $0.source != "native" }
        let waiting = workers.filter { $0.state == .waiting }
        if waiting.count == 1 {
            let f = waiting[0]
            return .worker(name: f.session.isEmpty ? f.primary : f.session, others: workers.count - 1)
        }
        if waiting.count > 1 { return .workers(waiting.count) }
        if state == .resource { return .resource }
        return .allNormal
    }

    /// Sensed non-tmux (native) sessions — their own category, most-recent first.
    /// Sense-only: no jump/reply; adoptable ones can be pulled into tmux.
    func nativeSessions(query: String) -> [Agent] {
        agents.filter { $0.isNative && matches($0, query) }
            .sorted { $0.since > $1.since }
    }

    /// User-watched PLAIN panes (tiered-pane-control) — their OWN section below the
    /// agent sections, so a promoted dev-server / log pane rides along without ever
    /// competing with "which agent needs you". By loc, so the order is stable.
    func watchedPanes(query: String) -> [Agent] {
        agents.filter { $0.watched && matches($0, query) }
            .sorted { $0.loc.localizedCaseInsensitiveCompare($1.loc) == .orderedAscending }
    }

    /// Panes eligible for the web-shared-input allowlist (Preferences → Shared
    /// input). Real tmux panes only — a guest types via `tmux send-keys`, so
    /// native/hook-less rows can't be targets. Ordered like the radar (state
    /// rank → session title) so the host recognises each pane by the SAME
    /// identity (avatar + `primary` session name) shown in the session list,
    /// not an indistinguishable "Claude Code · %N".
    var shareablePanes: [Agent] {
        agents
            .filter { !$0.isNative && !$0.paneID.isEmpty }
            .sorted { l, r in
                if l.state.rank != r.state.rank { return l.state.rank < r.state.rank }
                return l.primary.localizedCaseInsensitiveCompare(r.primary) == .orderedAscending
            }
    }

    /// Flattened, ordered agent list (for keyboard navigation).
    func ordered(query: String) -> [Agent] {
        sections(query: query).flatMap { $0.agents }
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
