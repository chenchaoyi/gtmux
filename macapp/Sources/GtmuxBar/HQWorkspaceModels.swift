import Foundation

struct HQConversation: Codable, Hashable, Identifiable {
    var agent: String
    var homeId: String?
    var sessionId: String
    var name: String
    var pane: String?
    var loc: String?
    var id: String { [agent, homeId ?? "", sessionId].joined(separator: "/") }
}

struct HQParticipant: Codable, Identifiable, Hashable {
    var id: String
    var conversation: HQConversation
    var role: String
    var from: Int64
    var until: Int64?
    var removedAt: Int64?
    var active: Bool { removedAt == nil || removedAt == 0 }
}

struct HQMatter: Codable, Identifiable {
    var id: String
    var title: String
    var state: String
    var assessment: String?
    var createdAt: Int64
    var updatedAt: Int64
    var revision: UInt64
    var participants: [HQParticipant]
    var parent: String?
    var mergedInto: String?
    var activeParticipants: [HQParticipant] { participants.filter(\.active) }
    func stateLabel(_ l: L10n) -> String {
        switch state {
        case "needs-input": return l.tr("Needs your input", "待你处理")
        case "completed": return l.tr("Completed", "已完成")
        case "closed": return l.tr("Closed", "已结束")
        default: return l.tr("Following", "跟进中")
        }
    }
}

struct HQCandidate: Decodable, Identifiable {
    var agent: String
    var homeId: String?
    var sessionId: String
    var name: String
    var pane: String?
    var loc: String?
    var status: String
    var since: Int64
    var supervisor: Bool?
    var icon: String?
    var conversation: HQConversation {
        HQConversation(agent: agent, homeId: homeId, sessionId: sessionId, name: name, pane: pane, loc: loc)
    }
    var id: String { conversation.id }
}

struct HQWorkspaceSnapshot: Decodable {
    var schema: Int
    var revision: UInt64
    var sourceRevision: String?
    var activityRevision: Int64?
    var matters: [HQMatter]
    var conversations: [HQCandidate]
    var references: [HQCandidate]?
    var selectableConversations: [HQCandidate] { conversations + (references ?? []) }
    var supervisor: HQCandidate?
}

struct HQChatStep: Decodable {
    var kind: String
    var title: String
    var detail: String?
}
struct HQChatSegment: Decodable {
    var text: String?
    var steps: [HQChatStep]?
}
struct HQChatSender: Decodable {
    var kind: String
    var label: String
    var agent: String?
    var pane: String?
}
struct HQChatBreak: Decodable {
    var kind: String
    var at: Int64?
}
struct HQChatTurn: Decodable {
    var prompt: String
    var response: String
    var agent: String?
    var time: String?
    var segments: [HQChatSegment]?
    var from: HQChatSender?
    var sessionBreak: HQChatBreak?
    var signal: Bool?
}

struct HQWorkspaceMessage: Decodable, Identifiable {
    var id: String
    var matterId: String?
    var speaker: String
    var kind: String
    var body: String
    var at: Int64
    var seq: UInt64
    var replyTo: String?
    var delivery: String?
    var reason: String?
    var reply: HQChatTurn?
    var evidence: String?
    var target: HQConversation?
}

struct HQWorkspaceChange: Decodable, Identifiable {
    var seq: UInt64
    var at: Int64
    var kind: String
    var matter: String?
    var target: String?
    var detail: String?
    var id: UInt64 { seq }
}

struct HQWorkspaceEvent: Decodable, Identifiable {
    var ts: Int64
    var seq: UInt64?
    var event: String
    var summary: String?
    var pane: String?
    var agentSession: String?
    var outcome: String?
    var id: String { "\(seq ?? 0)/\(ts)/\(event)" }
}

struct HQWorkspaceDetail: Decodable {
    var matterTitles: [String: String]?
    var schema: Int
    var matter: HQMatter?
    var messages: [HQWorkspaceMessage]
    var turns: [HQChatTurn]
    var earlierAvailable: Bool
    var messageEarlierAvailable: Bool?
    var transcriptEarlierAvailable: Bool?
    var earlierBefore: UInt64?
    var changes: [HQWorkspaceChange]
    var events: [HQWorkspaceEvent]
    var warnings: [String]
}

struct HQWorkspaceAction: Encodable {
    var op: String
    var id: String?
    var matter: String?
    var into: String?
    var title: String?
    var body: String?
    var state: String?
    var kind: String?
    var replyTo: String?
    var revision: UInt64?
    var association: HQParticipant?
    var associationId: String?
    var associations: [String]?
    var from: Int64?
    var until: Int64?
    var acknowledgeUncertain: Bool?
}

enum HQWorkspaceSelection: Hashable {
    case matter(String)
    case conversation, activity, newTask
    var scope: String? {
        switch self {
        case .matter(let id): return id
        case .conversation: return "global"
        case .activity: return "activity"
        case .newTask: return nil
        }
    }
}
