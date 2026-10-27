import Foundation
import GtmuxUsageWidgets

public enum MacUsageWidgetContainer {
    public static let kind = "com.gtmux.menubar.usage"

    // macOS supports team-prefixed groups without a provisioning profile. This
    // display-only group is deliberately distinct from the iOS shared container.
    public static func groupIdentifier(team: String, group: String) -> String? {
        guard team.range(of: #"^[A-Z0-9]{10}$"#, options: .regularExpression) != nil,
              group == team + ".com.gtmux.menubar.usage" else { return nil }
        return group
    }

    public static var store: UsageSnapshotStore {
        let bundle = Bundle.main
        guard let team = bundle.object(forInfoDictionaryKey: "GtmuxSigningTeam") as? String,
              let group = bundle.object(forInfoDictionaryKey: "GtmuxUsageGroup") as? String,
              let identifier = groupIdentifier(team: team, group: group) else {
            return UsageSnapshotStore(directory: nil)
        }
        return UsageSnapshotStore(directory: FileManager.default
            .containerURL(forSecurityApplicationGroupIdentifier: identifier)?
            .appendingPathComponent("UsageWidget", isDirectory: true))
    }

    public static func opensUsage(_ url: URL) -> Bool {
        guard let parts = URLComponents(url: url, resolvingAgainstBaseURL: false) else { return false }
        return parts.scheme == "gtmux" && parts.host == "usage" &&
            (parts.path.isEmpty || parts.path == "/") && parts.user == nil &&
            parts.password == nil && parts.port == nil && parts.query == nil && parts.fragment == nil
    }
}

public struct MacUsageRefreshPolicy {
    public private(set) var refreshing = false
    private var lastStarted: Date?
    private var lastReloaded: Date?

    public init() {}

    public mutating func begin(configured: Bool, at date: Date) -> Bool {
        guard configured, !refreshing,
              lastStarted.map({ date.timeIntervalSince($0) >= 60 }) ?? true else { return false }
        refreshing = true
        lastStarted = date
        return true
    }

    public mutating func finish(published: Bool, at date: Date) -> Bool {
        refreshing = false
        guard published, lastReloaded.map({ date.timeIntervalSince($0) >= 300 }) ?? true else { return false }
        lastReloaded = date
        return true
    }
}
