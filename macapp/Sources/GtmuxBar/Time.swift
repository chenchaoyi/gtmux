import Foundation

/// relativeTime renders a compact, stable-width "time since" label (DESIGN §3
/// right column): now / 3m / 2h / 5d. Pure (takes `now`) so it's unit-testable.
func relativeTime(_ epoch: Int, now: Int) -> String {
    guard epoch > 0 else { return "" }
    let d = max(0, now - epoch)
    if d < 45 { return "now" }
    if d < 3600 { return "\(max(1, d / 60))m" }
    if d < 86400 { return "\(d / 3600)h" }
    return "\(d / 86400)d"
}

extension Agent {
    var relativeTimeLabel: String {
        relativeTime(activityAt, now: Int(Date().timeIntervalSince1970))
    }
}
