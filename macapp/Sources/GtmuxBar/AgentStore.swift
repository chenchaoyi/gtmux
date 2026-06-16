import AppKit
import Combine

/// One coding-agent pane, mirroring `gtmux agents --json` (the stable contract).
struct Agent: Codable, Identifiable {
    let paneID: String
    let session: String
    let loc: String
    let agent: String
    let status: String // waiting | working | idle | running
    let task: String
    let latest: Bool

    var id: String { paneID }

    enum CodingKeys: String, CodingKey {
        case paneID = "pane_id"
        case session, loc, agent, status, task, latest
    }
}

/// The most-urgent overall state, used for the status-bar icon.
enum AgentState {
    case waiting, working, idle, none

    static func of(_ agents: [Agent]) -> AgentState {
        if agents.contains(where: { $0.status == "waiting" }) { return .waiting }
        if agents.contains(where: { $0.status == "working" }) { return .working }
        return agents.isEmpty ? .none : .idle
    }

    var color: NSColor {
        switch self {
        case .waiting: return .systemRed
        case .working: return .systemTeal
        case .idle:    return .systemGreen
        case .none:    return .tertiaryLabelColor
        }
    }
}

/// AgentStore polls the CLI and publishes the current agents to SwiftUI. Kept a
/// plain (non-actor) ObservableObject so AppKit delegate code can read it on the
/// main thread without concurrency hops; the @Published write is marshaled to
/// main by refresh().
final class AgentStore: ObservableObject {
    @Published private(set) var agents: [Agent] = []

    func refresh() {
        DispatchQueue.global(qos: .userInitiated).async {
            let data = GtmuxCLI.capture(["agents", "--json"]) ?? Data("[]".utf8)
            let decoded = (try? JSONDecoder().decode([Agent].self, from: data)) ?? []
            DispatchQueue.main.async { self.agents = decoded }
        }
    }

    var waiting: Int { agents.filter { $0.status == "waiting" }.count }
    var working: Int { agents.filter { $0.status == "working" }.count }

    /// Badge text next to the icon: the most-urgent actionable count, else "".
    var badge: String {
        if waiting > 0 { return "\(waiting)" }
        if working > 0 { return "\(working)" }
        return ""
    }

    /// "5 agents · 1 waiting · 2 working · 2 idle"
    var summary: String {
        let n = agents.count
        if n == 0 { return "no agents" }
        var parts: [String] = []
        if waiting > 0 { parts.append("\(waiting) waiting") }
        parts.append("\(working) working")
        parts.append("\(n - waiting - working) idle")
        return "\(n) agent\(n == 1 ? "" : "s") · " + parts.joined(separator: " · ")
    }
}
