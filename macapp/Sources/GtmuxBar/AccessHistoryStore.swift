import Foundation
import Combine

@MainActor final class AccessHistoryStore: ObservableObject {
    @Published private(set) var entries: [AccessHistoryEntry] = []
    @Published private(set) var groups: [AccessHistoryGroup] = []
    @Published private(set) var known: [AccessHistoryGroup] = []
    @Published private(set) var operations: [AccessHistoryOperation] = []
    @Published private(set) var query = AccessHistoryFilter()
    @Published private(set) var loading = false
    @Published private(set) var failed = false
    @Published private(set) var unsupported = false
    @Published private(set) var next: String?
    @Published private(set) var retainDays = 30
    private var generation = 0
    private let run: ([String]) -> (status: Int32, stdout: String, stderr: String)
    init(run: @escaping ([String]) -> (status: Int32, stdout: String, stderr: String) = { GtmuxCLI.captureBounded($0, timeout: 20) }) { self.run = run }
    func select(_ filter: AccessHistoryFilter) {
        guard filter != query else { return }
        query = filter; entries = []; groups = []; next = nil
        load()
    }
    func selectGroup(_ group: AccessHistoryGroup) {
        var filter = query
        filter.deviceId = group.deviceId; filter.actor = group.deviceId == nil ? group.actor : nil; filter.groupBy = nil
        select(filter)
    }
    func load(earlier: Bool = false) {
        guard !earlier || (!loading && next != nil) else { return }
        generation += 1; let request = generation, filter = query
        loading = true; failed = false; unsupported = false
        var args = ["logs", "--access", "--json", "--limit", "100"] + filter.arguments
        if earlier, let next { args += ["--before", next] }
        let run = run
        Task.detached(priority: .userInitiated) {
            let result = run(args)
            let decoder = JSONDecoder(); decoder.keyDecodingStrategy = .convertFromSnakeCase
            let page = result.status == 0 ? try? decoder.decode(AccessHistoryPage.self, from: Data(result.stdout.utf8)) : nil
            await MainActor.run {
                guard request == self.generation else { return }
                if let page, !filter.active || page.query == filter {
                    let prior = earlier ? self.entries : []
                    var ids = Set(prior.map(\.id))
                    self.entries = prior + page.entries.filter { ids.insert($0.id).inserted }
                    self.groups = AccessHistoryGrouping.merge(earlier ? self.groups : [], page.groups ?? [], sum: true)
                    self.known = AccessHistoryGrouping.merge(self.known, page.groups ?? [], sum: false)
                    self.operations = page.operations ?? []
                    self.next = page.next; self.retainDays = page.retainDays
                } else { self.failed = true; self.unsupported = result.status == 2 || (filter.active && page != nil && page?.query != filter) }
                self.loading = false
            }
        }
    }
}

enum AccessHistoryGrouping {
    private static func date(_ raw: String) -> Date {
        let f = ISO8601DateFormatter(); f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return f.date(from: raw) ?? ISO8601DateFormatter().date(from: raw) ?? .distantPast
    }
    static func merge(_ prior: [AccessHistoryGroup], _ incoming: [AccessHistoryGroup], sum: Bool) -> [AccessHistoryGroup] {
        var out = prior; var index = Dictionary(uniqueKeysWithValues: prior.enumerated().map { ($0.element.key, $0.offset) })
        for group in incoming {
            if let i = index[group.key] {
                let old = out[i]
                var newest = date(group.lastAt) > date(old.lastAt) ? group : old
                if sum {
                    newest.count = old.count + group.count; newest.accepted = old.accepted + group.accepted
                    newest.refused = old.refused + group.refused; newest.failed = old.failed + group.failed
                    newest.firstAt = date(old.firstAt) < date(group.firstAt) ? old.firstAt : group.firstAt
                }
                out[i] = newest
            } else { index[group.key] = out.count; out.append(group) }
        }
        return out
    }
}
