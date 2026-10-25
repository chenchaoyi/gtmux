import SwiftUI

struct HQWorkspaceParticipants: View {
    let l10n: L10n
    @ObservedObject var store: HQWorkspaceStore
    let matter: HQMatter
    @StateObject private var flow: HQParticipantManagement
    @Environment(\.dismiss) private var dismiss
    @Environment(\.hqWorkspaceSheetSize) private var sheetSize
    @State private var reading: HQParticipant?
    @State private var adjusting: String?

    init(l10n: L10n, store: HQWorkspaceStore, matter: HQMatter, flow: HQParticipantManagement = HQParticipantManagement()) {
        self.l10n = l10n; self.store = store; self.matter = matter
        _flow = StateObject(wrappedValue: flow)
    }
    private var current: HQMatter { store.snapshot?.matters.first { $0.id == matter.id } ?? matter }
    private var targets: [HQMatter] { (store.snapshot?.matters ?? []).filter { $0.id != current.id && $0.state != "closed" } }
    private var title: String {
        switch flow.page {
        case .list: return l10n.tr("Conversations & records", "对话与记录")
        case .add: return l10n.tr("Add a conversation", "添加对话")
        case .interval: return l10n.tr("Choose records to include", "调整收录范围")
        case .move: return l10n.tr("Move to another matter", "移到其他事项")
        case .organize: return l10n.tr("Organize this matter", "整理此事项")
        }
    }
    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 4) {
                    Text(title).font(HQTypography.title).accessibilityAddTraits(.isHeader)
                    Text(current.title).font(HQTypography.body).foregroundStyle(.secondary).lineLimit(2).help(current.title)
                }
                Spacer(minLength: 8)
                Button(l10n.tr("Done", "完成")) { dismiss() }.keyboardShortcut(.cancelAction)
            }
            if flow.page != .list {
                Button { flow.page = .list } label: { Label(l10n.tr("Back to conversations", "返回对话列表"), systemImage: "chevron.left") }
                    .buttonStyle(.borderless).accessibilityIdentifier("participants-back")
            }
            if let error = store.error {
                Text(error).font(HQTypography.body).foregroundStyle(Theme.Status.errored).textSelection(.enabled)
            }
            if let notice = flow.notice { Text(notice).font(HQTypography.caption).accessibilityIdentifier("participants-notice") }
            Divider()
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    switch flow.page {
                    case .list: list
                    case .add: HQParticipantAdd(l10n: l10n, store: store, matter: current, flow: flow)
                    case .interval(let id):
                        if let p = current.activeParticipants.first(where: { $0.id == id }) { interval(p) }
                        else { missing }
                    case .move(let id):
                        if let p = current.activeParticipants.first(where: { $0.id == id }) { move(p) }
                        else { missing }
                    case .organize: organize
                    }
                }.frame(maxWidth: .infinity, alignment: .leading).padding(.vertical, 4)
            }.disabled(store.acting)
            if store.acting { HQBrandLoading(label: l10n.tr("Saving…", "正在保存…")) }
        }.padding(20).frame(width: min(660, sheetSize.width), height: sheetSize.height)
            .font(HQTypography.body)
            .sheet(item: $reading) { HQParticipantReader(l10n: l10n, store: store, participant: $0, matterID: current.id) }
            .alert(item: $flow.confirmation) { proposal in
                Alert(title: Text(proposal.title), message: Text(proposal.detail), primaryButton: .default(Text(proposal.button)) {
                    store.perform(proposal.action) {
                        flow.finished(l10n.tr("Saved.", "已保存。"))
                        if let id = proposal.selection { store.select(.matter(id)); dismiss() }
                    }
                }, secondaryButton: .cancel(Text(l10n.tr("Cancel", "取消"))))
            }
    }
    private var missing: some View { Text(l10n.tr("This conversation was removed. Return to the list to refresh your choice.", "此对话已被移出，请返回列表重新选择。")) }
    private var list: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text(l10n.tr("Read this matter’s working conversations and background records.", "查看此事项的工作对话与背景记录。"))
                .font(HQTypography.body).foregroundStyle(.secondary)
            HStack {
                Text(l10n.tr("Included · \(current.activeParticipants.count)", "已收录 · \(current.activeParticipants.count)"))
                    .font(HQTypography.rowTitle)
                Spacer()
                Button(l10n.tr("Add conversation…", "添加对话…")) { flow.begin(.add, matter: current) }
                    .accessibilityIdentifier("participants-add")
            }
            if current.activeParticipants.isEmpty {
                Text(l10n.tr("No agent conversations included yet. Add one already working on this matter, or use a saved conversation as background.", "还没有收录对话。可以添加正在处理此事的对话，也可以引用过去的记录作为背景。"))
                    .foregroundStyle(.secondary).padding(.vertical, 20)
            }
            ForEach(current.activeParticipants) { p in
                VStack(alignment: .leading, spacing: 10) {
                    HQManagedConversationLabel(l10n: l10n, participant: p, current: candidate(p), saved: saved(p))
                    Text(HQParticipantRecord(participant: p, current: candidate(p)).scope(l10n))
                        .font(HQTypography.caption).foregroundStyle(.secondary)
                    HStack {
                        Button(l10n.tr("Read conversation", "查看原始对话")) { reading = p }
                            .accessibilityIdentifier("participants-read-\(p.id)")
                        Spacer()
                        Button(l10n.tr("Adjust…", "调整…")) { adjusting = adjusting == p.id ? nil : p.id }
                            .buttonStyle(.borderless)
                            .accessibilityLabel(l10n.tr("Adjust ", "调整 ") + p.conversation.name)
                            .accessibilityValue(adjusting == p.id ? l10n.tr("Expanded", "已展开") : l10n.tr("Collapsed", "已收起"))
                            .accessibilityIdentifier("participants-adjust-\(p.id)")
                    }
                    if adjusting == p.id {
                        Divider()
                        VStack(alignment: .leading, spacing: 10) {
                            Button(l10n.tr("Change included records…", "调整收录范围…")) { flow.begin(.interval(p.id), matter: current, participant: p) }
                                .accessibilityIdentifier("participants-range-open-\(p.id)")
                            Button(l10n.tr("Move to another matter…", "移到其他事项…")) { flow.begin(.move(p.id), matter: current, participant: p) }
                                .accessibilityIdentifier("participants-move-open-\(p.id)")
                            Button(l10n.tr("Remove from this matter…", "从此事项移除…")) { remove(p) }
                                .accessibilityIdentifier("participants-remove-\(p.id)")
                        }.buttonStyle(.borderless)

                    }
                }.padding(14).frame(maxWidth: .infinity, alignment: .leading)
                    .background(Color.primary.opacity(0.03), in: RoundedRectangle(cornerRadius: 9))
            }
            Divider()
            Button(l10n.tr("Split or merge this matter…", "拆分或合并此事项…")) { flow.begin(.organize, matter: current) }
                .buttonStyle(.borderless).accessibilityIdentifier("participants-organize")
        }
    }
    private func saved(_ p: HQParticipant) -> HQCandidate? { store.snapshot?.references?.first { $0.id == p.conversation.id } }
    private func candidate(_ p: HQParticipant) -> HQCandidate? { store.snapshot?.conversations.first { $0.id == p.conversation.id } }
    private func interval(_ p: HQParticipant) -> some View {
        VStack(alignment: .leading, spacing: 14) {
            HQManagedConversationLabel(l10n: l10n, participant: p, current: candidate(p), saved: saved(p))
            Text(l10n.tr("Choose which original messages can be read from this matter. This changes only the collected records; the agent keeps running.", "选择从此事项中可读取的原始问答范围。只改变收录内容，agent 会继续运行。"))
                .foregroundStyle(.secondary)
            HQParticipantRangeFields(l10n: l10n, flow: flow)
            Button(l10n.tr("Save range", "保存收录范围")) {
                store.perform(HQWorkspaceAction(op: "interval", matter: current.id, revision: flow.revision,
                    associationId: p.id, from: Int64(flow.from.timeIntervalSince1970), until: flow.ended ? Int64(flow.until.timeIntervalSince1970) : 0)) {
                    flow.finished(l10n.tr("Record range updated.", "收录范围已更新。"))
                }
            }.disabled(!flow.validInterval()).accessibilityIdentifier("participants-save-range")
        }
    }
    private func move(_ p: HQParticipant) -> some View {
        VStack(alignment: .leading, spacing: 14) {
            HQManagedConversationLabel(l10n: l10n, participant: p, current: candidate(p), saved: saved(p))
            Text(l10n.tr("Move only this conversation and its included range. Prior HQ discussion stays in this matter; the agent keeps running.", "只移动这条对话及其收录范围。已有的 HQ 讨论留在此事项，agent 会继续运行。"))
                .foregroundStyle(.secondary)
            destinationPicker
            Button(l10n.tr("Review move…", "确认移入…")) {
                guard let target = targets.first(where: { $0.id == flow.destination }) else { return }
                flow.confirmation = .init(action: HQWorkspaceAction(op: "move", matter: current.id, into: target.id,
                    revision: flow.revision, associationId: p.id), title: l10n.tr("Move this conversation?", "移动这条对话？"),
                    detail: l10n.tr("Move “\(p.conversation.name)” from “\(current.title)” to “\(target.title)”. Its record range is preserved. Prior HQ discussion stays here.", "将“\(p.conversation.name)”从“\(current.title)”移入“\(target.title)”，保留收录范围。已有的 HQ 讨论仍留在此事项。"), button: l10n.tr("Move conversation", "移动对话"))
            }.disabled(!targets.contains { $0.id == flow.destination }).accessibilityIdentifier("participants-move")
        }
    }
    private var destinationPicker: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(l10n.tr("Move into", "移入事项")).font(HQTypography.rowTitle)
            Picker(l10n.tr("Move into", "移入事项"), selection: $flow.destination) {
                Text(l10n.tr("Choose a matter", "选择事项")).tag("")
                ForEach(targets) { Text($0.title).tag($0.id) }
            }.labelsHidden().frame(maxWidth: .infinity).accessibilityIdentifier("participants-destination")
            if targets.isEmpty { Text(l10n.tr("No other open matters.", "暂无其他未结束事项。")) }
        }
    }
    private func remove(_ p: HQParticipant) {
        flow.confirmation = .init(action: HQWorkspaceAction(op: "unlink", matter: current.id, revision: current.revision, associationId: p.id),
            title: l10n.tr("Remove from this matter?", "从此事项移除？"),
            detail: l10n.tr("“\(p.conversation.name)” will no longer appear in “\(current.title)”. Its original messages and prior HQ discussion are kept. The agent keeps running.", "“\(p.conversation.name)”将不再显示在“\(current.title)”中。原始问答和已有的 HQ 讨论保留，agent 会继续运行。"), button: l10n.tr("Remove conversation", "移除此对话"))
    }
    private var organize: some View {
        VStack(alignment: .leading, spacing: 20) {
            VStack(alignment: .leading, spacing: 12) {
                Text(l10n.tr("Separate work into a new matter", "把部分工作分为新事项")).font(HQTypography.section)
                Text(l10n.tr("Select the conversations to move. Their ranges are kept; prior HQ discussion stays here, with a link to the new matter.", "选择要移走的对话，保留其收录范围。已有的 HQ 讨论留在此处，并保留新旧事项的来源关系。"))
                    .foregroundStyle(.secondary)
                ForEach(current.activeParticipants) { p in
                    Toggle(isOn: Binding(get: { flow.splitIDs.contains(p.id) }, set: { if $0 { flow.splitIDs.insert(p.id) } else { flow.splitIDs.remove(p.id) } })) {
                        Text(HQParticipantPresentation(participant: p, current: candidate(p)).title)
                    }.toggleStyle(.checkbox).accessibilityIdentifier("participants-split-\(p.id)")
                }
                TextField(l10n.tr("New matter name", "新事项名称"), text: $flow.splitName).textFieldStyle(.roundedBorder)
                    .accessibilityIdentifier("participants-split-name")
                Button(l10n.tr("Review split…", "确认拆分…")) {
                    let id = UUID().uuidString.lowercased()
                    flow.confirmation = .init(action: HQWorkspaceAction(op: "split", id: id, matter: current.id, title: flow.splitName,
                        revision: flow.revision, associations: Array(flow.splitIDs).sorted()), title: l10n.tr("Create a separate matter?", "拆分为新事项？"),
                        detail: l10n.tr("Move \(flow.splitIDs.count) selected conversations to “\(flow.splitName)”. Prior HQ discussion stays in “\(current.title)”.", "将选中的 \(flow.splitIDs.count) 条对话移入“\(flow.splitName)”。已有的 HQ 讨论留在“\(current.title)”。"), button: l10n.tr("Create and move", "创建并移入"), selection: id)
                }.disabled(flow.splitIDs.isEmpty || flow.splitName.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                    .accessibilityIdentifier("participants-split")
            }
            Divider()
            VStack(alignment: .leading, spacing: 12) {
                Text(l10n.tr("Combine this entire matter", "合并整个事项")).font(HQTypography.section)
                Text(l10n.tr("All included conversations and HQ discussion will be collected in the chosen matter. This matter will close; original records are retained.", "全部已收录对话和 HQ 讨论将汇入所选事项，此事项随之结束。原始记录保留。"))
                    .foregroundStyle(.secondary)
                destinationPicker
                Button(l10n.tr("Review merge…", "确认合并…")) {
                    guard let target = targets.first(where: { $0.id == flow.destination }) else { return }
                    flow.confirmation = .init(action: HQWorkspaceAction(op: "merge", matter: current.id, into: target.id, revision: flow.revision),
                        title: l10n.tr("Merge the entire matter?", "合并整个事项？"),
                        detail: l10n.tr("Collect all conversations and HQ discussion from “\(current.title)” in “\(target.title)”, then close “\(current.title)”. Original records are retained.", "将“\(current.title)”的全部对话和 HQ 讨论汇入“\(target.title)”，并结束“\(current.title)”。原始记录保留。"), button: l10n.tr("Merge and close", "合并并结束"), selection: target.id)
                }.disabled(!targets.contains { $0.id == flow.destination }).accessibilityIdentifier("participants-merge")
            }
        }
    }
}

struct HQManagedConversationLabel: View {
    let l10n: L10n
    let participant: HQParticipant
    let current: HQCandidate?
    var saved: HQCandidate? = nil
    var body: some View {
        let record = HQParticipantRecord(participant: participant, current: current, saved: saved)
        let info = record.presentation
        VStack(alignment: .leading, spacing: 5) {
            Text(info.title).font(HQTypography.rowTitle).fixedSize(horizontal: false, vertical: true)
            Text(info.metadata(l10n)).font(HQTypography.caption).foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            if info.current == nil, let loc = participant.conversation.loc, !loc.isEmpty {
                Text(l10n.tr("Last terminal: ", "最近终端：") + loc).font(HQTypography.caption).foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
            HStack(spacing: 6) {
                if let exact = info.current, let status = Status(rawValue: exact.status) { StatusBadge(status: status, size: 12) }
                Text(info.current == nil ? l10n.tr("Saved record · no current terminal found", "已保存记录 · 未找到当前终端")
                    : l10n.tr("In Terminal · ", "终端可用 · ") + hqConversationStatus(info.current!.status, l10n))
                    .font(HQTypography.caption)
            }
            Text(record.activity(l10n)).font(HQTypography.caption).foregroundStyle(.secondary)
        }.accessibilityElement(children: .combine)
    }
}
