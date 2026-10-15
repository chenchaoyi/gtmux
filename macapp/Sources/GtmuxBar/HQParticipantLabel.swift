import SwiftUI

/// Pane numbers are locators, not identity. Only an exact current conversation may
/// advertise a current pane/status; saved locators are explicitly historical.
struct HQParticipantPresentation {
    let participant: HQParticipant
    let current: HQCandidate?
    init(participant: HQParticipant, current: HQCandidate?) {
        self.participant = participant
        self.current = current?.id == participant.conversation.id ? current : nil
    }
    var title: String {
        let name = current?.name ?? participant.conversation.name
        return name.isEmpty ? agentLabel : name
    }
    var agentLabel: String {
        switch participant.conversation.agent {
        case "codex": return "Codex"
        case "claude": return "Claude Code"
        default: return participant.conversation.agent
        }
    }
    func metadata(_ l: L10n) -> String {
        var parts = [agentLabel]
        if let pane = current?.pane, !pane.isEmpty {
            parts.append(pane)
            if let loc = current?.loc, !loc.isEmpty { parts.append(loc) }
        } else if let pane = participant.conversation.pane, !pane.isEmpty {
            parts.append(l.tr("Last pane \(pane)", "最近窗格 \(pane)"))
        }
        return parts.joined(separator: " · ")
    }
    func state(_ l: L10n) -> String {
        let status =
            current.map { hqConversationStatus($0.status, l) }
            ?? l.tr("Original record · no current session", "原始记录 · 无当前会话")
        return participant.role == "reference"
            ? l.tr("Reference", "历史参考") + " · " + status : status
    }
}

struct HQParticipantLabel: View {
    let l10n: L10n
    let participant: HQParticipant
    let current: HQCandidate?
    var showTitle = true
    var body: some View {
        let info = HQParticipantPresentation(participant: participant, current: current)
        VStack(alignment: .leading, spacing: 3) {
            if showTitle { Text(info.title).font(Theme.Font.session).lineLimit(1).help(info.title) }
            Text(info.metadata(l10n)).font(Theme.Font.window).foregroundStyle(.secondary).lineLimit(1)
            HStack(spacing: 5) {
                if let value = info.current?.status, let status = Status(rawValue: value) {
                    StatusBadge(status: status, size: 12)
                }
                Text(info.state(l10n)).font(Theme.Font.window).foregroundStyle(.secondary).lineLimit(1)
            }
        }.accessibilityElement(children: .combine)
    }
}
