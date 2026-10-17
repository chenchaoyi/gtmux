import Foundation

struct AccessHistoryEntry: Decodable, Identifiable {
    let id: String
    let at: String
    let actor: String
    let name: String
    let scope: String
    let labelEn: String
    let labelZh: String
    let outcome: String
    var deviceId: String?
    var platform: String?
    var sourceIp: String?
    var method: String?
    var target: String?
    var status: Int?
    var durationMs: Int64?
    var operation: String?
    var sessionId: String?
}
struct AccessHistoryPage: Decodable {
    let entries: [AccessHistoryEntry]
    var next: String?
    let retainDays: Int
    var groups: [AccessHistoryGroup]?
    var query: AccessHistoryFilter?
    var operations: [AccessHistoryOperation]?
}

struct AccessHistoryFilter: Decodable, Equatable {
    var deviceId: String?
    var actor: String?
    var operation: String?
    var outcome: String?
    var groupBy: String?
    var hasFilters: Bool { deviceId != nil || actor != nil || operation != nil || outcome != nil }
    var active: Bool { hasFilters || groupBy != nil }
    var identity: String { deviceId.map { "device:" + $0 } ?? actor.map { "actor:" + $0 } ?? "" }
    var arguments: [String] {
        [("--device-id", deviceId), ("--actor", actor), ("--operation", operation), ("--outcome", outcome), ("--group-by", groupBy)]
            .flatMap { flag, value in value.map { [flag, $0] } ?? [] }
    }
}
struct AccessHistoryOperation: Decodable, Identifiable { let value: String; let en: String; let zh: String; var id: String { value } }
struct AccessHistoryGroup: Decodable, Identifiable {
    let key: String
    var actor: String
    var deviceId: String?
    var name: String
    var platform: String?
    var scope: String
    var sourceIp: String?
    var count: Int
    var accepted: Int
    var refused: Int
    var failed: Int
    var firstAt: String
    var lastAt: String
    var id: String { key }
}
