import Foundation

/// A read-only projection: the two stores keep their own IDs and are never rewritten.
struct HQActivityItem: Identifiable {
    enum Source: String { case workspace, journal }
    var id: String
    var at: Int64
    var sequence: UInt64
    var source: Source
    var kind: String
    var contextID: String?
    var context: String?
    var target: String?
    var summary: String?
    var pane: String?
    var sessionID: String?
    var outcome: String?
    var presentation: HQActivityPresentation { HQActivityPresentation(kind: kind, outcome: outcome) }
    func detail(_ l: L10n) -> String? { presentation.detail(summary, l10n: l) }
}

struct HQActivityEpisode: Identifiable {
    var items: [HQActivityItem]
    var id: String { items[0].id }
    var at: Int64 { items[0].at }
    var context: String? { items[0].context }
}

struct HQActivityDay: Identifiable {
    var id: String
    var at: Int64
    var episodes: [HQActivityEpisode]
}

enum HQActivityTimeline {
    static func items(_ detail: HQWorkspaceDetail?) -> [HQActivityItem] {
        guard let detail else { return [] }
        var result = detail.changes.map { c in
            HQActivityItem(
                id: "workspace:\(c.seq)", at: c.at, sequence: c.seq, source: .workspace, kind: c.kind,
                contextID: c.matter, context: c.matter.flatMap { detail.matterTitles?[$0] },
                target: c.target.map { detail.matterTitles?[$0] ?? $0 }, summary: c.detail)
        }
        var seen: [String: Int] = [:]
        for e in detail.events {
            let occurrence = seen[e.id, default: 0]
            seen[e.id] = occurrence + 1
            result.append(HQActivityItem(
                id: "journal:\(e.id)/\(occurrence)", at: e.ts, sequence: e.seq ?? 0, source: .journal,
                kind: e.event, summary: e.summary, pane: e.pane, sessionID: e.agentSession, outcome: e.outcome))
        }
        return result.sorted {
            if $0.at != $1.at { return $0.at > $1.at }
            // Sequences are comparable only within their own source. Ties across
            // stores have a stable display order, not an invented causal order.
            if $0.source != $1.source { return $0.source.rawValue < $1.source.rawValue }
            if $0.sequence != $1.sequence { return $0.sequence > $1.sequence }
            return $0.id < $1.id
        }
    }
    static func days(_ items: [HQActivityItem], calendar: Calendar = .current, category: HQActivityCategory? = nil) -> [HQActivityDay] {
        var result: [HQActivityDay] = []
        for item in items {
            let date = Date(timeIntervalSince1970: TimeInterval(max(0, item.at)))
            let key = item.at > 0 ? String(calendar.startOfDay(for: date).timeIntervalSince1970) : "unknown"
            if result.last?.id != key { result.append(HQActivityDay(id: key, at: item.at, episodes: [])) }
            let day = result.count - 1
            // Only adjacent, explicitly related workspace actions can form an episode.
            // Journal pane/session locators never fabricate a matter association.
            if let context = item.contextID, let last = result[day].episodes.last,
               let previous = last.items.last, previous.contextID == context,
               previous.source == .workspace, item.source == .workspace,
               item.at > 0, last.at - item.at <= 300 {
                result[day].episodes[result[day].episodes.count - 1].items.append(item)
            } else {
                result[day].episodes.append(HQActivityEpisode(items: [item]))
            }
        }
        // Filtering must not merge episodes separated by a now-hidden record.
        return result.compactMap { day in
            let episodes = day.episodes.compactMap { episode -> HQActivityEpisode? in
                let matches = episode.items.filter { $0.presentation.includes(in: category) }
                return matches.isEmpty ? nil : HQActivityEpisode(items: matches)
            }
            return episodes.isEmpty ? nil : HQActivityDay(id: day.id, at: day.at, episodes: episodes)
        }
    }
    static func clock(_ at: Int64, seconds: Bool = false, timeZone: TimeZone = .current) -> String {
        guard at > 0 else { return "—" }
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = timeZone
        formatter.dateFormat = seconds ? "HH:mm:ss" : "HH:mm"
        return formatter.string(from: Date(timeIntervalSince1970: TimeInterval(at)))
    }
    static func dateLabel(_ at: Int64, zh: Bool, timeZone: TimeZone = .current) -> String {
        guard at > 0 else { return zh ? "时间未知" : "Unknown time" }
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: zh ? "zh_CN" : "en_US")
        formatter.timeZone = timeZone
        formatter.dateFormat = zh ? "yyyy年M月d日 EEEE" : "EEEE, MMM d, yyyy"
        return formatter.string(from: Date(timeIntervalSince1970: TimeInterval(at)))
    }
}
