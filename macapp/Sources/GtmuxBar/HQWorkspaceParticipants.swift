import SwiftUI

struct HQConversationIcon: View {
    let agent: String
    let hint: String?
    var body: some View {
        if let image = AgentIcons.image(icon: hint ?? "", name: agent) {
            Image(nsImage: image).resizable().scaledToFit()
        } else {
            Text(String(agent.prefix(2)).uppercased()).font(Theme.Font.window).foregroundStyle(.secondary)
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
            Text(l10n.tr("No conversations linked.", "尚未关联会话。")).font(Theme.Font.window).foregroundStyle(
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
                                VStack(alignment: .leading, spacing: 2) {
                                    Text(p.conversation.name).font(Theme.Font.session).lineLimit(1)
                                    Text(
                                        p.role == "reference"
                                            ? l10n.tr("Reference", "历史参考")
                                            : candidate(p).map { hqConversationStatus($0.status, l10n) }
                                                ?? l10n.tr("Original record", "原始记录")
                                    ).font(Theme.Font.window).foregroundStyle(.secondary)
                                }
                            }.padding(8).background(
                                Color.primary.opacity(0.04), in: RoundedRectangle(cornerRadius: 7))
                        }.buttonStyle(.plain).help(l10n.tr("View original conversation", "查看原始对话"))
                    }
                }
            }.sheet(item: $selected) {
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
    @State private var detail: HQWorkspaceDetail?
    @State private var error: String?
    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                VStack(alignment: .leading, spacing: 4) {
                    Text(participant.conversation.name).font(Theme.Font.title)
                    Text(l10n.tr("Original conversation · read only", "原始对话 · 只读")).font(Theme.Font.window)
                        .foregroundStyle(.secondary)
                }
                Spacer()
                Button(l10n.tr("Open in Terminal", "在终端打开")) {
                    store.perform(
                        HQWorkspaceAction(op: "focus", matter: matterID, associationId: participant.id))
                }
                Button(l10n.tr("Done", "完成")) { dismiss() }.keyboardShortcut(.cancelAction)
            }
            Divider()
            if let commandError = store.error {
                Text(commandError).font(Theme.Font.task).foregroundStyle(Theme.Status.errored).textSelection(
                    .enabled)
            }
            if let error {
                Text(error).font(Theme.Font.task).textSelection(.enabled)
                Button(l10n.tr("Retry", "重试")) { load() }
            } else if let detail {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 20) {
                        ForEach(Array(detail.turns.enumerated()), id: \.offset) { _, turn in
                            if !turn.prompt.isEmpty {
                                HQChatBubble(
                                    l10n: l10n, user: true, label: l10n.tr("You", "你"), bodyText: turn.prompt)
                            }
                            HQChatResponse(
                                l10n: l10n, turn: turn, speakerLabel: participant.conversation.agent,
                                icon: store.snapshot?.conversations.first {
                                    $0.id == participant.conversation.id
                                }?.icon)
                        }
                        if detail.turns.isEmpty {
                            Text(l10n.tr("No turns in this interval.", "所选时段内没有对话。")).foregroundStyle(
                                .secondary)
                        }
                        ForEach(Array(Set(detail.warnings)).sorted(), id: \.self) {
                            Text(hqWorkspaceWarning($0, l10n)).font(Theme.Font.window).foregroundStyle(
                                .secondary)
                        }
                    }.padding(12)
                }
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }.padding(20).frame(minWidth: 580, idealWidth: 700, minHeight: 450, idealHeight: 600).onAppear {
            load()
        }
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

struct HQWorkspaceParticipants: View {
    let l10n: L10n
    @ObservedObject var store: HQWorkspaceStore
    let matter: HQMatter
    @Environment(\.dismiss) private var dismiss
    @State private var picked = ""
    @State private var reference = false
    @State private var from = Date()
    @State private var until = Date()
    @State private var destination = ""
    @State private var splitName = ""
    @State private var splitIDs = Set<String>()
    @State private var mergeConfirmation = false
    @State private var reading: HQParticipant?
    private var current: HQMatter { store.snapshot?.matters.first { $0.id == matter.id } ?? matter }
    private var targets: [HQMatter] {
        (store.snapshot?.matters ?? []).filter { $0.id != current.id && $0.state != "closed" }
    }
    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack {
                Text(l10n.tr("Manage conversations", "管理会话")).font(Theme.Font.title)
                Spacer()
                Button(l10n.tr("Done", "完成")) { dismiss() }.keyboardShortcut(.cancelAction)
            }
            Text(l10n.tr("Organize records without stopping or restarting agents.", "调整事项归属，不会停止或重启 agent。"))
                .font(Theme.Font.task).foregroundStyle(.secondary)
            if let error = store.error {
                Text(error).foregroundStyle(Theme.Status.errored).font(Theme.Font.task).textSelection(
                    .enabled)
            }
            ScrollView {
                VStack(alignment: .leading, spacing: 18) {
                    ForEach(current.activeParticipants) { p in
                        VStack(alignment: .leading, spacing: 9) {
                            HStack {
                                Toggle(
                                    isOn: Binding(
                                        get: { splitIDs.contains(p.id) },
                                        set: {
                                            if $0 { splitIDs.insert(p.id) } else { splitIDs.remove(p.id) }
                                        })
                                ) { EmptyView() }.toggleStyle(.checkbox).labelsHidden().help(
                                    l10n.tr("Select for splitting", "选择用于拆分"))
                                Text(p.conversation.name).font(Theme.Font.session).lineLimit(1)
                                Spacer()
                                Button(l10n.tr("View", "查看")) { reading = p }
                                Button(l10n.tr("Unlink", "移出")) {
                                    store.perform(
                                        HQWorkspaceAction(
                                            op: "unlink", matter: current.id, revision: current.revision,
                                            associationId: p.id))
                                }
                            }
                            HQParticipantInterval(l10n: l10n, participant: p, revision: current.revision) {
                                start, end, version in
                                store.perform(
                                    HQWorkspaceAction(
                                        op: "interval", matter: current.id, revision: version,
                                        associationId: p.id, from: start, until: end))
                            }
                            if !targets.isEmpty {
                                HStack {
                                    targetPicker
                                    Button(l10n.tr("Transfer", "转移")) {
                                        store.perform(
                                            HQWorkspaceAction(
                                                op: "move", matter: current.id, into: destination,
                                                revision: current.revision, associationId: p.id))
                                    }.disabled(destination.isEmpty)
                                }
                            }
                        }.padding(12).background(
                            Color.primary.opacity(0.03), in: RoundedRectangle(cornerRadius: 9))
                    }
                    GroupBox(l10n.tr("Add conversation", "关联会话")) {
                        VStack(alignment: .leading, spacing: 12) {
                            Picker(l10n.tr("Conversation", "会话"), selection: $picked) {
                                Text(l10n.tr("Choose conversation", "选择会话")).tag("")
                                ForEach(store.snapshot?.selectableConversations ?? []) {
                                    Text($0.name + " · " + $0.agent).tag($0.id)
                                }
                            }
                            Toggle(l10n.tr("Historical reference", "仅作为历史参考"), isOn: $reference)
                            DatePicker(l10n.tr("From", "开始时间"), selection: $from)
                            if reference { DatePicker(l10n.tr("Until", "结束时间"), selection: $until) }
                            HStack {
                                Text(l10n.tr("Only the selected interval is included.", "只纳入所选时段的记录。")).font(
                                    Theme.Font.window
                                ).foregroundStyle(.secondary)
                                Spacer()
                                Button(l10n.tr("Add", "关联")) { add() }.disabled(
                                    picked.isEmpty || from > Date() || reference && until <= from)
                            }
                        }.padding(8)
                    }
                    DisclosureGroup(l10n.tr("Organize matters", "整理事项")) {
                        VStack(alignment: .leading, spacing: 12) {
                            if !targets.isEmpty {
                                HStack {
                                    targetPicker
                                    Button(l10n.tr("Merge…", "合并…")) { mergeConfirmation = true }.disabled(
                                        destination.isEmpty)
                                }
                            }
                            Text(
                                l10n.tr(
                                    "Split selected conversations into a new matter. Prior discussion stays here with a provenance link.",
                                    "将勾选会话拆分为新事项。已有讨论保留在当前事项，并记录拆分关系。")
                            ).font(Theme.Font.window).foregroundStyle(.secondary)
                            HStack {
                                TextField(l10n.tr("New matter name", "新事项名称"), text: $splitName)
                                    .textFieldStyle(.roundedBorder)
                                Button(l10n.tr("Split", "拆分")) {
                                    let id = UUID().uuidString.lowercased()
                                    store.perform(
                                        HQWorkspaceAction(
                                            op: "split", id: id, matter: current.id, title: splitName,
                                            revision: current.revision, associations: Array(splitIDs))
                                    ) {
                                        store.select(.matter(id))
                                        dismiss()
                                    }
                                }.disabled(
                                    splitIDs.isEmpty
                                        || splitName.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                            }
                        }.padding(.top, 10)
                    }
                }.padding(.vertical, 4)
            }.disabled(store.acting)
        }.padding(20).frame(width: 620, height: 640)
            .sheet(item: $reading) {
                HQParticipantReader(l10n: l10n, store: store, participant: $0, matterID: current.id)
            }
            .confirmationDialog(
                l10n.tr("Merge this matter? Original records are retained.", "合并当前事项？原始记录保留，关联会话和讨论将汇入所选事项。"),
                isPresented: $mergeConfirmation, titleVisibility: .visible
            ) {
                Button(l10n.tr("Merge", "合并")) {
                    let into = destination
                    store.perform(
                        HQWorkspaceAction(
                            op: "merge", matter: current.id, into: into, revision: current.revision)
                    ) {
                        store.select(.matter(into))
                        dismiss()
                    }
                }
                Button(l10n.tr("Cancel", "取消"), role: .cancel) {}
            }
    }
    private var targetPicker: some View {
        Picker(l10n.tr("Target matter", "目标事项"), selection: $destination) {
            Text(l10n.tr("Choose matter", "选择事项")).tag("")
            ForEach(targets) { Text($0.title).tag($0.id) }
        }
    }
    private func add() {
        guard let c = store.snapshot?.selectableConversations.first(where: { $0.id == picked }) else {
            return
        }
        let a = HQParticipant(
            id: UUID().uuidString.lowercased(), conversation: c.conversation,
            role: reference ? "reference" : "execution", from: Int64(from.timeIntervalSince1970),
            until: reference ? Int64(until.timeIntervalSince1970) : nil)
        store.perform(
            HQWorkspaceAction(op: "associate", matter: current.id, revision: current.revision, association: a)
        )
    }
}
struct HQParticipantInterval: View {
    let l10n: L10n
    let participant: HQParticipant
    let revision: UInt64
    let save: (Int64, Int64, UInt64) -> Void
    @State private var baseRevision: UInt64
    @State private var from: Date
    @State private var until: Date
    @State private var ended: Bool
    init(
        l10n: L10n, participant: HQParticipant, revision: UInt64,
        save: @escaping (Int64, Int64, UInt64) -> Void
    ) {
        self.l10n = l10n
        self.participant = participant
        self.save = save
        self.revision = revision
        _baseRevision = State(initialValue: revision)
        _from = State(initialValue: Date(timeIntervalSince1970: TimeInterval(participant.from)))
        _until = State(
            initialValue: Date(
                timeIntervalSince1970: TimeInterval(participant.until ?? Int64(Date().timeIntervalSince1970)))
        )
        _ended = State(initialValue: (participant.until ?? 0) > 0)
    }
    var body: some View {
        DisclosureGroup(l10n.tr("Included interval", "关联时段")) {
            VStack(alignment: .leading, spacing: 8) {
                if baseRevision != revision {
                    Button(l10n.tr("Reload current interval", "重填最新时段")) {
                        from = Date(timeIntervalSince1970: TimeInterval(participant.from))
                        until = Date(
                            timeIntervalSince1970: TimeInterval(
                                participant.until ?? Int64(Date().timeIntervalSince1970)))
                        ended = (participant.until ?? 0) > 0
                        baseRevision = revision
                    }
                }
                DatePicker(l10n.tr("From", "开始时间"), selection: $from)
                Toggle(l10n.tr("Set end time", "指定结束时间"), isOn: $ended).disabled(
                    participant.role == "reference")
                if ended { DatePicker(l10n.tr("Until", "结束时间"), selection: $until) }
                Button(l10n.tr("Apply interval", "应用时段")) {
                    save(
                        Int64(from.timeIntervalSince1970), ended ? Int64(until.timeIntervalSince1970) : 0,
                        baseRevision)
                }.disabled(from > Date() || ended && until <= from)
            }.padding(.top, 8)
        }.font(Theme.Font.window)
    }
}
