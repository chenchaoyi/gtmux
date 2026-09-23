import Foundation

/// TunnelStatus is `status/tunnel.json`, which the CLI's tunnel reporter writes for either
/// backend (openspec change `diagnostics`): whether this Mac is reachable from outside,
/// checked end to end the way the phone connects. The pairing window used to decide "the
/// tunnel is down" by matching phrases in cloudflared's log, for either backend, so a
/// Direct user's verdict came from a tunnel they were not running.
struct TunnelStatus: Equatable {
    let state: String      // "connecting" | "connected" | "down"
    let backend: String    // "direct" | "standard"
    let lastError: String
    let fresh: Bool        // written within its staleAfter; a stale status is not a fact

    static func read(now: Date = Date()) -> TunnelStatus? {
        guard let data = FileManager.default.contents(atPath: Paths.data("status/tunnel.json")) else { return nil }
        return parse(data, now: now)
    }

    /// parse decodes the file. Pure, for the tests.
    static func parse(_ data: Data, now: Date) -> TunnelStatus? {
        guard let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let state = obj["state"] as? String else { return nil }
        let detail = obj["detail"] as? [String: Any] ?? [:]
        let staleAfter = (obj["staleAfter"] as? Double) ?? Double(obj["staleAfter"] as? Int ?? 0)
        var fresh = false
        if let u = obj["updated"] as? String, let updated = parseTime(u), staleAfter > 0 {
            fresh = now.timeIntervalSince(updated) <= staleAfter
        }
        return TunnelStatus(state: state, backend: detail["backend"] as? String ?? "",
                            lastError: detail["lastError"] as? String ?? "", fresh: fresh)
    }

    /// Go writes RFC 3339 with up to nine fractional digits, or none.
    static func parseTime(_ s: String) -> Date? {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        if let d = f.date(from: s) { return d }
        f.formatOptions = [.withInternetDateTime]
        return f.date(from: s)
    }
}

/// ReachVerdict is what the pairing window says about its address.
enum ReachVerdict: Equatable {
    case checking
    case reachable
    /// The Mac cannot reach its own public address, but the tunnel reports itself up:
    /// the local network is in the way (a DNS that maps the name to a private address),
    /// and a phone on another network connects.
    case tunnelUpMacCannotSee
    case tunnelDown(String)
    case cannotReachYet

    /// verdict combines the window's own probe with the tunnel's status. Pure, for the
    /// tests. A status that is missing or stale says nothing, so it is never a verdict.
    static func of(probeOK: Bool?, status: TunnelStatus?) -> ReachVerdict {
        guard let ok = probeOK else { return .checking }
        if ok { return .reachable }
        guard let s = status, s.fresh else { return .cannotReachYet }
        switch s.state {
        case "connected": return .tunnelUpMacCannotSee
        case "down": return .tunnelDown(s.lastError)
        default: return .cannotReachYet
        }
    }

    /// Whether the address is NOT answering right now. A pairing window just after a move
    /// uses this to say "reconnecting" instead of "can't reach it yet", which is the same
    /// fact with the wrong conclusion drawn from it.
    var isNotReachable: Bool {
        switch self {
        case .reachable, .checking, .tunnelUpMacCannotSee: return false
        case .tunnelDown, .cannotReachYet: return true
        }
    }
}
