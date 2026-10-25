import Combine
import Foundation

/// Editors capture a revision on entry; confirmations capture the exact proposed
/// action. Polling cannot silently change the scope the user is approving.
final class HQParticipantManagement: ObservableObject {
    enum Page: Equatable { case list, add, interval(String), move(String), organize }
    struct Confirmation: Identifiable {
        let id = UUID()
        let action: HQWorkspaceAction
        let title: String
        let detail: String
        let button: String
        var selection: String? = nil
    }
    @Published var page: Page = .list
    @Published var picked = ""
    @Published var reference = false
    @Published var from = Date()
    @Published var until = Date()
    @Published var ended = false
    @Published var destination = ""
    @Published var splitName = ""
    @Published var splitIDs = Set<String>()
    @Published var confirmation: Confirmation?
    @Published var notice: String?
    private(set) var revision: UInt64 = 0

    func begin(_ page: Page, matter: HQMatter, participant: HQParticipant? = nil) {
        self.page = page
        revision = matter.revision
        picked = ""
        destination = ""
        splitName = ""
        splitIDs = []
        reference = participant?.role == "reference"
        from = participant.map { Date(timeIntervalSince1970: TimeInterval($0.from)) } ?? Date()
        until = Date(timeIntervalSince1970: TimeInterval(participant?.until ?? Int64(Date().timeIntervalSince1970)))
        ended = reference || (participant?.until ?? 0) > 0
        notice = nil
    }
    func validInterval(now: Date = Date()) -> Bool {
        from.timeIntervalSince1970 > 0 && from <= now
            && (!(reference || ended) || until > from && until <= now)
    }
    func finished(_ text: String) { page = .list; notice = text }
}

struct HQParticipantRecord {
    let participant: HQParticipant
    let current: HQCandidate?
    var saved: HQCandidate? = nil
    var presentation: HQParticipantPresentation {
        HQParticipantPresentation(participant: participant, current: current)
    }
    func scope(_ l: L10n) -> String {
        if let end = participant.until, end > 0 {
            let kind = participant.role == "reference" ? l.tr("Historical reference", "历史参考") : l.tr("Collection ended", "已停止收录")
            return kind + " · " + Self.date(participant.from, l) + " – " + Self.date(end, l)
        }
        return l.tr("Collecting since ", "持续收录 · 从 ") + Self.date(participant.from, l)
    }
    static func date(_ time: Int64, _ l: L10n) -> String {
        guard time > 0 else { return l.tr("Time unavailable", "时间未知") }
        let f = DateFormatter()
        f.locale = Locale(identifier: l.tr("en_US", "zh_CN"))
        f.dateStyle = .short; f.timeStyle = .short
        return f.string(from: Date(timeIntervalSince1970: TimeInterval(time)))
    }
    func activity(_ l: L10n) -> String {
        if let exact = presentation.current, exact.since > 0 {
            return l.tr("In this state since ", "当前状态始于 ") + Self.date(exact.since, l)
        }
        if let saved, saved.id == participant.conversation.id, saved.since > 0 {
            return l.tr("Record updated: ", "记录更新于：") + Self.date(saved.since, l)
        }
        return l.tr("Current activity unavailable", "暂无当前活动信息")
    }
}
