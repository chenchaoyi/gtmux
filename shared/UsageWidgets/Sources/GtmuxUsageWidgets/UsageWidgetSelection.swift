import Foundation

/// System configuration needs stable, nonempty choice IDs. Namespacing actual
/// providers also keeps a custom provider named "automatic" distinct from auto.
public enum UsageWidgetSelection {
    public static let automaticID = "automatic"
    private static let prefix = "provider:"

    public static func provider(for identifier: String?) -> String {
        guard let identifier, identifier != automaticID else { return "" }
        // Retain older saved raw provider IDs when a widget updates in place.
        return identifier.hasPrefix(prefix) ? String(identifier.dropFirst(prefix.count)) : identifier
    }

    public static func choices(in snapshot: UsageWidgetSnapshot, automaticName: String) -> [UsageProviderChoice] {
        var seen = Set<String>()
        return [UsageProviderChoice(id: automaticID, name: automaticName)] + snapshot.accounts.compactMap {
            seen.insert($0.agent).inserted ? UsageProviderChoice(id: prefix + $0.agent, name: $0.name) : nil
        }
    }
}

public struct UsageProviderChoice: Identifiable, Equatable {
    public let id: String
    public let name: String
}
