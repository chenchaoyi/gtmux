import Combine
import Foundation

struct ReleaseVersion: Comparable {
    let parts: [Int]
    init?(_ text: String) {
        let value = text.hasPrefix("v") ? String(text.dropFirst()) : text
        let pieces = value.split(separator: ".", omittingEmptySubsequences: false)
        guard pieces.count == 3,
              pieces.allSatisfy({ !$0.isEmpty && $0.allSatisfy({ $0.isASCII && $0.isNumber }) }) else { return nil }
        let numbers = pieces.compactMap { Int($0) }
        guard numbers.count == 3 else { return nil }
        parts = numbers
    }
    var description: String { parts.map(String.init).joined(separator: ".") }
    static func < (lhs: Self, rhs: Self) -> Bool { lhs.parts.lexicographicallyPrecedes(rhs.parts) }
}

struct MacReleaseNote: Codable, Equatable, Identifiable {
    let version: String
    let en: [String]
    let zh: [String]
    var id: String { version }
    func lines(lang: String) -> [String] {
        let own = lang == "zh" ? zh : en
        return own.isEmpty ? (lang == "zh" ? en : zh) : own
    }
}

struct MacReleaseNotes: Codable, Equatable {
    let schema: Int
    let since: String
    let until: String
    let limit: Int
    let publishedThrough: String
    let oldestAvailable: String
    let entries: [MacReleaseNote]
    enum CodingKeys: String, CodingKey {
        case schema, since, until, limit, entries
        case publishedThrough = "published_through", oldestAvailable = "oldest_available"
    }
    func valid(for version: ReleaseVersion) -> Bool {
        guard schema == 1, since == "0.0.0", ReleaseVersion(until) == version, limit == 30,
              let published = ReleaseVersion(publishedThrough), published >= version,
              let oldest = ReleaseVersion(oldestAvailable), oldest <= published,
              Set(entries.map(\.version)).count == entries.count else { return false }
        var previous: ReleaseVersion?
        for entry in entries {
            guard let v = ReleaseVersion(entry.version), v <= version, v >= oldest,
                  !entry.en.isEmpty || !entry.zh.isEmpty,
                  previous == nil || v < previous! else { return false }
            previous = v
        }
        return true
    }
}

/// Acknowledgement tracks the installed app, independently of the update checker.
/// The CLI owns release parsing; this store only selects which versions to present.
final class WhatsNewStore: ObservableObject {
    static let shared = WhatsNewStore()
    static let seenKey = "whatsNew.v1.seen"
    private static let cacheKey = "whatsNew.v1.cache"
    enum Scope: Equatable { case history, upgrade(from: String) }
    @Published private(set) var scope: Scope = .history
    @Published private(set) var report: MacReleaseNotes?
    @Published private(set) var loading = false
    @Published private(set) var failed = false
    let version: String
    private let defaults: UserDefaults
    private let run: ([String]) -> (status: Int32, stdout: String, stderr: String)
    private var autoChecked = false
    private var request = 0

    init(defaults: UserDefaults = .standard,
         version: String = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "",
         run: @escaping ([String]) -> (status: Int32, stdout: String, stderr: String) = {
             GtmuxCLI.captureBounded($0, timeout: 15)
         }) {
        self.defaults = defaults; self.version = ReleaseVersion(version)?.description ?? version; self.run = run
        if let current = ReleaseVersion(version), let data = defaults.data(forKey: Self.cacheKey),
           let cached = try? JSONDecoder().decode(MacReleaseNotes.self, from: data), cached.valid(for: current) {
            report = cached
        }
    }

    /// Run before EnvironmentHealth overwrites its launch-version marker. It provides
    /// a migration baseline for users who already had the app before this feature.
    func prepareLaunch() {
        guard let current = ReleaseVersion(version) else { return }
        if let seen = defaults.string(forKey: Self.seenKey), ReleaseVersion(seen) != nil { return }
        let previous = defaults.string(forKey: "environmentHealth.v1.attempt").flatMap(ReleaseVersion.init)
        defaults.set((previous ?? current).description, forKey: Self.seenKey)
    }

    var upgradeDue: Bool {
        guard let current = ReleaseVersion(version),
              let seen = defaults.string(forKey: Self.seenKey).flatMap(ReleaseVersion.init) else { return false }
        return seen < current
    }

    var entries: [MacReleaseNote] {
        guard let report else { return [] }
        switch scope {
        case .history: return report.entries
        case .upgrade(let from):
            guard let lower = ReleaseVersion(from) else { return [] }
            return report.entries.filter { ReleaseVersion($0.version).map { $0 > lower } ?? false }
        }
    }
    var historyLimited: Bool {
        guard let oldest = report.flatMap({ ReleaseVersion($0.oldestAvailable) }) else { return false }
        switch scope {
        case .upgrade(let from): return ReleaseVersion(from).map { $0 < oldest } ?? false
        case .history: return ReleaseVersion(version).map { $0 < oldest } ?? false
        }
    }

    func autoCheck(onReady: @escaping () -> Void) {
        guard !autoChecked else { return }; autoChecked = true
        prepareLaunch()
        guard let current = ReleaseVersion(version),
              let seen = defaults.string(forKey: Self.seenKey).flatMap(ReleaseVersion.init), seen < current else { return }
        load(.upgrade(from: seen.description)) { [weak self] in
            guard let self else { return }
            if self.entries.isEmpty && !self.historyLimited { self.acknowledge() }
            else { onReady() }
        }
    }

    func showHistory() { autoChecked = true; prepareLaunch(); load(.history) }
    func retry() { load(scope, refresh: true) }

    private func load(_ target: Scope, refresh: Bool = false, onReady: @escaping () -> Void = {}) {
        request += 1
        let token = request
        scope = target; failed = false
        guard let current = ReleaseVersion(version) else { loading = false; failed = true; return }
        if !refresh, let report, report.valid(for: current) { loading = false; onReady(); return }
        loading = true
        let run = run, version = current.description
        DispatchQueue.global(qos: .utility).async { [weak self] in
            let result = run(["whatsnew", "--all", "--until", version, "--json"])
            DispatchQueue.main.async {
                guard let self, self.request == token else { return }
                self.loading = false
                guard result.status == 0, let data = result.stdout.data(using: .utf8),
                      let parsed = try? JSONDecoder().decode(MacReleaseNotes.self, from: data), parsed.valid(for: current) else {
                    self.failed = true
                    DiagLog.warn("whatsnew.read-failed", "could not read release notes", ["exit": Int(result.status)])
                    return
                }
                self.report = parsed; self.defaults.set(data, forKey: Self.cacheKey)
                onReady()
            }
        }
    }

    func acknowledge() {
        guard !loading, !failed, let current = ReleaseVersion(version), let report, report.valid(for: current) else { return }
        if let previous = defaults.string(forKey: Self.seenKey).flatMap(ReleaseVersion.init), previous > current { return }
        defaults.set(current.description, forKey: Self.seenKey)
    }
}
