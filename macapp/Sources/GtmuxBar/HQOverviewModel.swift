import Foundation

struct HQOverviewSnapshot: Decodable {
    var generatedAt: Int64
    var since: Int64
    var needsInput: [HQOverviewItem]
    var following: [HQOverviewItem]
    var recent: [HQOverviewChange]
    var recentTotal: Int
}
struct HQOverviewItem: Decodable, Identifiable {
    var matterId: String
    var preview: HQOverviewPreview?
    var id: String { matterId }
}
struct HQOverviewPreview: Decodable {
    var messageId: String?
    var kind: String
    var body: String
    var at: Int64?
    var truncated: Bool?
}
struct HQOverviewChange: Decodable, Identifiable {
    var seq: UInt64
    var at: Int64
    var matterId: String
    var kind: String
    var state: String?
    var id: UInt64 { seq }
    func title(_ l: L10n) -> String {
        if kind == "update", let state {
            switch state {
            case "completed": return l.tr("Marked completed", "标记完成")
            case "closed": return l.tr("Matter closed", "结束事项")
            case "open": return l.tr("Following", "继续跟进")
            case "needs-input": return l.tr("Needs your input", "待你处理")
            default: break
            }
        }
        return hqActivityTitle(kind, l)
    }
}

enum HQOverviewModel {
    static func items(_ group: HQMatterGroup, in snapshot: HQWorkspaceSnapshot?) -> [HQOverviewItem] {
        let explicit = group.matters(in: snapshot)
        let records = group == .needsInput ? snapshot?.overview?.needsInput : snapshot?.overview?.following
        guard let records else { return explicit.prefix(3).map { HQOverviewItem(matterId: $0.id) } }
        let ids = Set(explicit.map(\.id))
        return Array(records.filter { ids.contains($0.matterId) }.prefix(3))
    }
    static func workers(_ matter: HQMatter, in snapshot: HQWorkspaceSnapshot?) -> [HQCandidate] {
        let ids = Set(matter.activeParticipants.filter {
            $0.role == "execution" && ($0.until == nil || $0.until == 0)
        }.map { $0.conversation.id })
        return (snapshot?.conversations ?? []).filter { ids.contains($0.id) }
    }
    static func snippet(_ body: String) -> AttributedString {
        var text = (try? AttributedString(markdown: body)) ?? AttributedString(body)
        // The row opens its source conversation. Inline links must not hijack it.
        for range in text.runs.map(\.range) { text[range].link = nil }
        return text
    }
}
