import Combine
import Foundation

/// Server mode as the menu bar sees it: a pure consumer of `gtmux awake --json`,
/// exactly like AgentStore consumes `gtmux agents --json`. No logic is duplicated here —
/// the CLI stays the single source of truth for what the machine is doing.
///
/// FOOTGUN (hit before, in the pane browser): every field Go marks `omitempty` is ABSENT
/// from the JSON when zero, and a non-optional Swift property makes Codable fail the whole
/// decode — the symptom is a permanently empty view, not a parse error. So anything
/// optional on the Go side is Optional here.
struct ServerModeStatus: Decodable {
    struct Guard: Decodable {
        let installed: Bool
        let healthy: Bool
    }
    struct Platform: Decodable {
        let ok: Bool
        let verified: Bool
        let reason: String?
        let osVersion: String?

        enum CodingKeys: String, CodingKey {
            case ok, verified, reason
            case osVersion = "os_version"
        }
    }
    struct Exit: Decodable {
        let at: Int
        let reason: String
    }

    let state: String            // on | off | lapsed
    let tier: String?
    let since: Int?
    let power: String            // ac | battery
    let batteryPct: Int?
    let guardStatus: Guard
    let systemDisableSleep: Bool
    let ownedByGtmux: Bool
    let lastExit: Exit?
    let platform: Platform

    enum CodingKeys: String, CodingKey {
        case state, tier, since, power, platform
        case batteryPct = "battery_pct"
        case guardStatus = "guard"
        case systemDisableSleep = "system_disablesleep"
        case ownedByGtmux = "owned_by_gtmux"
        case lastExit = "last_exit"
    }

    var isOn: Bool { systemDisableSleep }

    /// Whether the indicator should ask for attention rather than sit quietly.
    /// Deliberately narrow: red means "a human should look at this", nothing else.
    /// (Same discipline as the HQ medallion — a soft amber must not read as red.)
    var needsAttention: Bool {
        if state == "lapsed" { return true }
        if isOn && !guardStatus.healthy { return true }   // nothing could restore sleep
        if isOn && power == "battery", let pct = batteryPct, pct <= 30 { return true }
        return false
    }

    var attentionReason: String? {
        if state == "lapsed" {
            return L10n.shared.tr("stopped working — closing the lid will sleep this Mac",
                                 "已失效 —— 现在合盖会休眠")
        }
        if isOn && !guardStatus.healthy {
            return L10n.shared.tr("the safety guard is missing", "恢复睡眠的守护缺失")
        }
        if isOn && power == "battery", let pct = batteryPct, pct <= 30 {
            return L10n.shared.tr("on battery, \(pct)% left", "电池供电，还剩 \(pct)%")
        }
        return nil
    }
}

final class ServerModeStore: ObservableObject {
    static let shared = ServerModeStore()

    @Published private(set) var status: ServerModeStatus?

    private var timer: Timer?

    /// Polled on a slow cadence: this is a machine state that changes when a human
    /// decides it does, not something that needs the radar's 1.5s refresh.
    func start(interval: TimeInterval = 20) {
        refresh()
        timer?.invalidate()
        timer = Timer.scheduledTimer(withTimeInterval: interval, repeats: true) { [weak self] _ in
            self?.refresh()
        }
    }

    func refresh() {
        DispatchQueue.global(qos: .utility).async {
            guard let data = GtmuxCLI.capture(["awake", "--json"]),
                  let decoded = try? JSONDecoder().decode(ServerModeStatus.self, from: data)
            else { return }
            DispatchQueue.main.async { self.status = decoded }
        }
    }

    /// Turning it off from the menu bar. `--yes` because the menu item IS the
    /// confirmation — the user just clicked "turn off" on an indicator whose whole
    /// purpose is to be the off switch.
    func turnOff(completion: @escaping () -> Void) {
        DispatchQueue.global(qos: .userInitiated).async {
            _ = GtmuxCLI.captureResult(["awake", "off", "--yes"])
            DispatchQueue.main.async { self.refresh(); completion() }
        }
    }

    func turnOn(completion: @escaping (Bool, String) -> Void) {
        DispatchQueue.global(qos: .userInitiated).async {
            let r = GtmuxCLI.captureResult(["awake", "on", "--yes"])
            DispatchQueue.main.async { self.refresh(); completion(r.status == 0, r.stderr) }
        }
    }
}
