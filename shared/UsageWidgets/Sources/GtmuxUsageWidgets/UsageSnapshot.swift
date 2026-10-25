import Foundation

/// A display-only projection. Credentials, addresses, account IDs and transcripts
/// deliberately have no representation in this shared-container schema.
public struct UsageWidgetSnapshot: Codable, Equatable {
    public static let maximumBytes = 65_536
    public static let staleInterval: TimeInterval = 15 * 60
    public var schema: Int = 1
    public var server: String
    public var capturedAt: TimeInterval
    public var accounts: [UsageWidgetAccount]

    public init(server: String, capturedAt: TimeInterval, accounts: [UsageWidgetAccount]) {
        self.server = server; self.capturedAt = capturedAt; self.accounts = accounts
    }

    public static let empty = Self(server: "", capturedAt: 0, accounts: [])

    public func isStale(at date: Date) -> Bool {
        capturedAt <= 0 || capturedAt > date.timeIntervalSince1970 + 300 ||
            date.timeIntervalSince1970 - capturedAt >= Self.staleInterval
    }

    public func selected(_ provider: String) -> [UsageWidgetAccount] {
        provider.isEmpty ? accounts : accounts.filter { $0.agent == provider }
    }

    public func transitionDates(after date: Date) -> [Date] {
        var seconds: [TimeInterval] = [capturedAt + Self.staleInterval]
        for account in accounts {
            if let observed = account.observedAt { seconds.append(observed + Self.staleInterval) }
            for window in account.windows {
                if let reset = window.resetAt { seconds.append(reset) }
                seconds.append((window.observedAt ?? capturedAt) + Self.staleInterval)
            }
        }
        return Array(Set(seconds.filter { $0 > date.timeIntervalSince1970 && $0.isFinite }))
            .sorted().prefix(12).map { Date(timeIntervalSince1970: $0) }
    }

    /// Consume the same core response as the app's usage sheet, not local token
    /// estimates. Malformed/oversized reports fail without replacing old data.
    public static func project(report: Data, server: String, at date: Date) throws -> Self {
        guard report.count <= 2 * 1024 * 1024 else { throw SnapshotError.tooLarge }
        let raw = try JSONDecoder().decode(RawUsage.self, from: report)
        let windows = raw.limits?.windows ?? []
        let balances = raw.limits?.balances ?? []
        guard windows.count <= 64, balances.count <= 16 else { throw SnapshotError.tooLarge }
        let now = date.timeIntervalSince1970
        var accounts: [UsageWidgetAccount] = []
        for window in windows where window.stale != true {
            guard let agent = providerKey(window.agent), window.pctUsed.isFinite,
                  window.pctUsed >= 0, window.pctUsed <= 1000 else { continue }
            if let observed = window.observedAt, TimeInterval(observed) > now + 300 { continue }
            let reset = window.resetUnix.map(TimeInterval.init).flatMap { $0 > 0 ? $0 : nil }
            // A reached reset is unknown capacity, never a fabricated zero.
            guard reset == nil || reset! > now else { continue }
            let observed = window.observedAt.map(TimeInterval.init).flatMap { $0 > 0 && $0 <= now + 300 ? $0 : nil }
            let value = UsageWidgetWindow(label: clean(window.label, limit: 100), kind: clean(window.kind ?? "", limit: 32),
                model: clean(window.model ?? "", limit: 40), percent: window.pctUsed,
                resetAt: reset, observedAt: observed, tier: ["warn", "full"].contains(window.tier ?? "") ? window.tier : nil)
            if let index = accounts.firstIndex(where: { $0.id == agent }) {
                // Do not silently overwrite a different window with the same label.
                if !accounts[index].windows.contains(value) { accounts[index].windows.append(value) }
            } else {
                accounts.append(UsageWidgetAccount(id: agent, agent: agent,
                    name: clean(window.agentName ?? agent, limit: 60), windows: [value], wallets: []))
            }
        }
        for (index, balance) in balances.enumerated() {
            guard let agent = providerKey(balance.agent), balance.wallets.count <= 8,
                  balance.observedAt > 0, TimeInterval(balance.observedAt) <= now + 300 else { continue }
            let wallets = balance.wallets.compactMap { wallet -> UsageWidgetWallet? in
                guard wallet.amount.range(of: #"^-?[0-9]{1,20}(\.[0-9]{1,12})?$"#, options: .regularExpression) != nil else { return nil }
                let currency = clean(wallet.currency, limit: 8)
                guard !currency.isEmpty else { return nil }
                return UsageWidgetWallet(kind: clean(wallet.kind, limit: 32), currency: currency, amount: wallet.amount)
            }
            guard !wallets.isEmpty else { continue }
            // Distinct accounts retain distinct cards; balances are never added
            // across currencies, wallets or provider identities.
            accounts.append(UsageWidgetAccount(id: "\(agent)-balance-\(index)", agent: agent,
                name: clean(balance.agentName ?? agent, limit: 60), windows: [], wallets: wallets,
                observedAt: TimeInterval(balance.observedAt)))
        }
        for index in accounts.indices {
            accounts[index].windows.sort { $0.percent == $1.percent ? $0.label < $1.label : $0.percent > $1.percent }
        }
        accounts.sort {
            func current(_ account: UsageWidgetAccount) -> Bool {
                account.windows.contains { !$0.stale(at: date) } ||
                    (account.observedAt.map { now - $0 < Self.staleInterval } ?? false)
            }
            let freshA = current($0), freshB = current($1)
            if freshA != freshB { return freshA }
            let a = $0.windows.first?.percent ?? -1, b = $1.windows.first?.percent ?? -1
            return a == b ? $0.id < $1.id : a > b
        }
        let snapshot = Self(server: clean(server, limit: 80), capturedAt: now, accounts: accounts)
        guard try JSONEncoder().encode(snapshot).count <= maximumBytes else { throw SnapshotError.tooLarge }
        return snapshot
    }

    public func validated() -> Self? {
        guard schema == 1, capturedAt.isFinite, capturedAt >= 0, accounts.count <= 80,
              server == Self.clean(server, limit: 80), Set(accounts.map(\.id)).count == accounts.count else { return nil }
        for account in accounts {
            guard Self.providerKey(account.agent) != nil, account.id.range(of: #"^[a-z0-9_-]{1,80}$"#, options: .regularExpression) != nil,
                  account.name == Self.clean(account.name, limit: 60), account.windows.count <= 64,
                  account.wallets.count <= 8, account.observedAt.map({ $0.isFinite && $0 > 0 }) ?? true else { return nil }
            for window in account.windows {
                guard window.percent.isFinite, window.percent >= 0, window.percent <= 1000,
                      window.label == Self.clean(window.label, limit: 100),
                      window.kind == Self.clean(window.kind, limit: 32), window.model == Self.clean(window.model, limit: 40),
                      window.resetAt.map({ $0.isFinite && $0 > 0 }) ?? true,
                      window.observedAt.map({ $0.isFinite && $0 > 0 }) ?? true,
                      window.tier == nil || ["warn", "full"].contains(window.tier!) else { return nil }
            }
            for wallet in account.wallets {
                guard wallet.amount.range(of: #"^-?[0-9]{1,20}(\.[0-9]{1,12})?$"#, options: .regularExpression) != nil,
                      wallet.kind == Self.clean(wallet.kind, limit: 32), !wallet.currency.isEmpty,
                      wallet.currency == Self.clean(wallet.currency, limit: 8) else { return nil }
            }
        }
        return self
    }

    private static func providerKey(_ value: String?) -> String? {
        guard let value, value.range(of: #"^[a-z0-9_-]{1,40}$"#, options: .regularExpression) != nil else { return nil }
        return value
    }

    private static func clean(_ value: String, limit: Int) -> String {
        String(value.unicodeScalars.filter { !CharacterSet.controlCharacters.contains($0) })
            .trimmingCharacters(in: .whitespacesAndNewlines).prefix(limit).description
    }
}

public enum SnapshotError: Error { case tooLarge, invalid, unavailable }

public struct UsageWidgetAccount: Codable, Equatable, Identifiable {
    public var id: String
    public var agent: String
    public var name: String
    public var windows: [UsageWidgetWindow]
    public var wallets: [UsageWidgetWallet]
    public var observedAt: TimeInterval?
    public init(id: String, agent: String, name: String, windows: [UsageWidgetWindow], wallets: [UsageWidgetWallet], observedAt: TimeInterval? = nil) {
        self.id = id; self.agent = agent; self.name = name; self.windows = windows; self.wallets = wallets; self.observedAt = observedAt
    }
}

public struct UsageWidgetWindow: Codable, Equatable {
    public var label: String
    public var kind: String
    public var model: String
    public var percent: Double
    public var resetAt: TimeInterval?
    public var observedAt: TimeInterval?
    public var tier: String?
    public init(label: String, kind: String, model: String = "", percent: Double, resetAt: TimeInterval? = nil, observedAt: TimeInterval? = nil, tier: String? = nil) {
        self.label = label; self.kind = kind; self.model = model; self.percent = percent
        self.resetAt = resetAt; self.observedAt = observedAt; self.tier = tier
    }
    public func expired(at date: Date) -> Bool { resetAt.map { $0 <= date.timeIntervalSince1970 } ?? false }
    public func stale(at date: Date) -> Bool {
        observedAt.map { $0 > date.timeIntervalSince1970 + 300 || date.timeIntervalSince1970 - $0 >= UsageWidgetSnapshot.staleInterval } ?? false
    }
}

public struct UsageWidgetWallet: Codable, Equatable {
    public var kind: String
    public var currency: String
    public var amount: String
    public init(kind: String, currency: String, amount: String) { self.kind = kind; self.currency = currency; self.amount = amount }
}

private struct RawUsage: Decodable { var limits: RawLimits? }
private struct RawLimits: Decodable { var windows: [RawWindow]?; var balances: [RawBalance]? }
private struct RawWindow: Decodable {
    var agent: String?; var agentName: String?; var label: String; var kind: String?; var model: String?
    var pctUsed: Double; var resetUnix: Int64?; var observedAt: Int64?; var stale: Bool?; var tier: String?
    enum CodingKeys: String, CodingKey {
        case agent, label, kind, model, stale, tier
        case agentName = "agent_name", pctUsed = "pct_used", resetUnix = "reset_unix", observedAt = "observed_at"
    }
}
private struct RawBalance: Decodable {
    var agent: String; var agentName: String?; var observedAt: Int64; var wallets: [RawWallet]
    enum CodingKeys: String, CodingKey { case agent, wallets; case agentName = "agent_name", observedAt = "observed_at" }
}
private struct RawWallet: Decodable { var kind: String; var currency: String; var amount: String }
