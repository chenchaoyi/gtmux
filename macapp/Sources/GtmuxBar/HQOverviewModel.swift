import Foundation

struct HQOverviewSnapshot: Decodable {
    var humanInput: [HQOverviewItem]? = nil
    var awaitingHq: [HQOverviewItem]? = nil
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
    static func distinctQuestionPrompt(title: String, prompt: String) -> String? {
        func normalize(_ value: String) -> String {
            value.split(whereSeparator: { $0.isWhitespace }).joined(separator: " ")
        }
        let question = normalize(prompt)
        return !question.isEmpty && question != normalize(title) ? prompt : nil
    }
    static func items(_ group: HQMatterGroup, in snapshot: HQWorkspaceSnapshot?) -> [HQOverviewItem] {
        let explicit = group.matters(in: snapshot)
        let records: [HQOverviewItem]?
        switch group {
        case .needsInput: records = snapshot?.overview?.humanInput
        case .awaitingHQ: records = snapshot?.overview?.awaitingHq
        default: records = snapshot?.overview?.following
        }
        let ids = Set(explicit.map(\.id))
        var seen = Set<String>()
        var items: [HQOverviewItem] = []
        for item in records ?? [] where ids.contains(item.matterId) && seen.insert(item.matterId).inserted {
            items.append(item)
            if items.count == 3 { return items }
        }
        for matter in explicit where seen.insert(matter.id).inserted {
            items.append(HQOverviewItem(matterId: matter.id))
            if items.count == 3 { break }
        }
        return items
    }
    static func currentPreview(_ item: HQOverviewItem, matter: HQMatter) -> HQOverviewPreview? {
        if let assessment = matter.assessment, !assessment.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            // Metadata edits also change UpdatedAt; it is not an assessment date.
            return HQOverviewPreview(kind: "assessment", body: assessment)
        }
        return item.preview
    }
    static func remaining(_ group: HQMatterGroup, in snapshot: HQWorkspaceSnapshot?) -> Int {
        max(0, group.matters(in: snapshot).count - Set(items(group, in: snapshot).map(\.matterId)).count)
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
