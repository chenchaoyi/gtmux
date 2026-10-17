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
}

