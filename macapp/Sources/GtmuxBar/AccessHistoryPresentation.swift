import Foundation

struct AccessHistoryLine: Identifiable {
    let entry: AccessHistoryEntry
    let clock: String
    let day: String?
    var id: String { entry.id }
}

enum AccessHistoryPresentation {
    // One set of formatters per page, not per row. UI language and local time zone
    // are independent: forcing Chinese must not retain an English system date.
    static func lines(_ entries: [AccessHistoryEntry], zh: Bool, timeZone: TimeZone = .current) -> [AccessHistoryLine] {
        let parser = ISO8601DateFormatter()
        let fraction = ISO8601DateFormatter()
        fraction.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let day = DateFormatter(), clock = DateFormatter()
        for formatter in [day, clock] {
            formatter.locale = Locale(identifier: zh ? "zh_CN" : "en_US")
            formatter.calendar = Calendar(identifier: .gregorian)
            formatter.timeZone = timeZone
        }
        day.dateFormat = zh ? "yyyy年M月d日" : "MMM d, yyyy"
        clock.dateFormat = "HH:mm:ss"
        var previous: String?
        return entries.map { entry in
            let date = fraction.date(from: entry.at) ?? parser.date(from: entry.at)
            let label = date.map(day.string) ?? (zh ? "时间未知" : "Unknown date")
            let line = AccessHistoryLine(entry: entry, clock: date.map(clock.string) ?? entry.at,
                                         day: label == previous ? nil : label)
            previous = label
            return line
        }
    }

    static func name(_ e: AccessHistoryEntry, zh: Bool) -> String {
        name(scope: e.scope, value: e.name, zh: zh)
    }
    static func name(scope: String, value: String, zh: Bool) -> String {
        scope == "master" ? (zh ? "远程客户端" : "Remote client") :
        scope == "anonymous" ? (zh ? "未认证访问" : "Unauthenticated access") : value
    }
    static func operation(_ e: AccessHistoryEntry, zh: Bool) -> String {
        let target = e.target ?? e.sessionId.map { "Codex · " + $0 }
        return (zh ? e.labelZh : e.labelEn) + (target.map { " · " + $0 } ?? "")
    }
    static func outcome(_ e: AccessHistoryEntry, zh: Bool) -> String {
        let label = e.outcome == "ok" ? (zh ? "已接受" : "Accepted") :
            e.outcome == "refused" ? (zh ? "已拒绝" : "Refused") : (zh ? "失败" : "Failed")
        return label + (e.status.flatMap { $0 > 0 ? " · \($0)" : nil } ?? "")
    }
    static func metadata(_ e: AccessHistoryEntry, zh: Bool) -> String {
        var parts = [e.platform, e.sourceIp].compactMap { $0 }.filter { !$0.isEmpty }
        if let id = e.deviceId, !id.isEmpty { parts.append((zh ? "设备 " : "Device ") + id) }
        if let method = e.method, !method.isEmpty { parts.append(method) }
        if let ms = e.durationMs, ms > 0 { parts.append("\(ms) ms") }
        return parts.joined(separator: " · ")
    }
    static func copyText(_ e: AccessHistoryEntry, zh: Bool) -> String {
        [e.at, name(e, zh: zh), operation(e, zh: zh), outcome(e, zh: zh), metadata(e, zh: zh)]
            .filter { !$0.isEmpty }.joined(separator: "\t")
    }
}
