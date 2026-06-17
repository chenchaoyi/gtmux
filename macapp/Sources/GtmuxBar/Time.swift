import Foundation

/// relativeTime renders a compact, stable-width duration since `epoch`
/// (DESIGN §3 right column): 7s / 3m / 2h / 5d. Pure (takes `now`) so it's
/// unit-testable. Seconds granularity under a minute distinguishes "just
/// started" from "been at it a while".
func relativeTime(_ epoch: Int, now: Int) -> String {
    guard epoch > 0 else { return "" }
    let d = max(0, now - epoch)
    if d < 60 { return "\(d)s" }
    if d < 3600 { return "\(d / 60)m" }
    if d < 86400 { return "\(d / 3600)h" }
    return "\(d / 86400)d"
}

extension Agent {
    /// Duration in the current state ("working 7m"): anchored to `since` (state
    /// start) when the hook provided it, else last activity.
    var relativeTimeLabel: String {
        relativeTime(since > 0 ? since : activityAt, now: Int(Date().timeIntervalSince1970))
    }
}
