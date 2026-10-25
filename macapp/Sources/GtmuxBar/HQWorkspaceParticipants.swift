import SwiftUI

struct HQConversationIcon: View {
    let agent: String
    let hint: String?
    var body: some View {
        if let image = AgentIcons.image(icon: hint ?? "", name: agent) {
            Image(nsImage: image).resizable().scaledToFit()
        } else {
            Text(String(agent.prefix(2)).uppercased()).font(HQTypography.caption).foregroundStyle(.secondary)
        }
    }
}
struct HQParticipantSummary: View {
    let l10n: L10n
    @ObservedObject var store: HQWorkspaceStore
    let matter: HQMatter
    @State private var selected: HQParticipant?
    var body: some View {
        if matter.activeParticipants.isEmpty {
            Text(l10n.tr("No conversations linked.", "尚未关联会话。")).font(HQTypography.caption).foregroundStyle(
                .secondary)
        } else {
            ScrollView(.horizontal) {
                HStack(spacing: 8) {
                    ForEach(matter.activeParticipants) { p in
                        Button {
                            selected = p
                        } label: {
                            HStack(spacing: 7) {
                                HQConversationIcon(agent: p.conversation.agent, hint: candidate(p)?.icon)
                                    .frame(width: 22, height: 22)
                                HQParticipantLabel(l10n: l10n, participant: p, current: candidate(p))
                                    .frame(minWidth: 200, maxWidth: 330, alignment: .leading)
                            }.padding(8).background(
                                Color.primary.opacity(0.04), in: RoundedRectangle(cornerRadius: 7))
                        }.buttonStyle(.plain).help(l10n.tr("View original conversation", "查看原始对话"))
                    }
                }
            }.fixedSize(horizontal: false, vertical: true).sheet(item: $selected) {
                HQParticipantReader(l10n: l10n, store: store, participant: $0, matterID: matter.id)
            }
        }
    }
    private func candidate(_ p: HQParticipant) -> HQCandidate? {
        store.snapshot?.conversations.first { $0.id == p.conversation.id }
    }
}
struct HQParticipantReader: View {
    let l10n: L10n
    @ObservedObject var store: HQWorkspaceStore
    let participant: HQParticipant
    let matterID: String
    @Environment(\.dismiss) private var dismiss
    @Environment(\.hqWorkspaceSheetSize) private var sheetSize
    @State private var detail: HQWorkspaceDetail?
    @State private var error: String?
    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                VStack(alignment: .leading, spacing: 4) {
                    Text(participant.conversation.name).font(HQTypography.title).lineLimit(2)
                    HQParticipantLabel(l10n: l10n, participant: participant, current: current, showTitle: false)
                    Text(participant.conversation.sessionId).font(HQTypography.code).foregroundStyle(.secondary).lineLimit(1)
                        .textSelection(.enabled)
                        .accessibilityLabel(l10n.tr("Session ID", "会话 ID") + ": " + participant.conversation.sessionId)
                }
                Spacer()
                Button(l10n.tr("Open in Terminal", "在终端打开")) {
                    store.perform(
                        HQWorkspaceAction(op: "focus", matter: matterID, associationId: participant.id))
                }.disabled(current == nil)
                Button(l10n.tr("Done", "完成")) { dismiss() }.keyboardShortcut(.cancelAction)
                    .accessibilityIdentifier("participant-reader-done")
            }
            Divider()
            if let commandError = store.error {
                Text(commandError).font(HQTypography.body).foregroundStyle(Theme.Status.errored).textSelection(
                    .enabled)
            }
            if let error {
                Text(error).font(HQTypography.body).textSelection(.enabled)
                Button(l10n.tr("Retry", "重试")) { load() }
            } else if let detail {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 20) {
                        let separators = HQChatTimeline.separators(detail.turns.map { HQChatTimeline.timestamp($0.time) })
                        ForEach(Array(detail.turns.enumerated()), id: \.offset) { index, turn in
                            if let date = separators[index] { HQChatTimeSeparator(date: date, l10n: l10n) }
                            HQTranscriptExchange(
                                l10n: l10n, turn: turn, speakerLabel: HQParticipantPresentation(participant: participant, current: current).agentLabel,
                                icon: current?.icon)
                        }
                        if detail.turns.isEmpty {
                            Text(l10n.tr("No turns in this interval.", "所选时段内没有对话。")).foregroundStyle(
                                .secondary)
                        }
                        ForEach(Array(Set(detail.warnings)).sorted(), id: \.self) {
                            Text(hqWorkspaceWarning($0, l10n)).font(HQTypography.caption).foregroundStyle(
                                .secondary)
                        }
                    }.padding(12)
                }
            } else {
                HQBrandLoading(label: l10n.tr("Loading conversation…", "正在载入对话…"))
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }.padding(20).frame(width: sheetSize.width, height: min(600, sheetSize.height)).onAppear {
            load()
        }
    }
    private var current: HQCandidate? {
        store.snapshot?.conversations.first { $0.id == participant.conversation.id }
    }
    private func load() {
        error = nil
        store.readParticipant(participant, matter: matterID) { result in
            switch result {
            case .success(let value): detail = value
            case .failure(let failure): error = failure.localizedDescription
            }
        }
    }
}
