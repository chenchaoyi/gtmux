import SwiftUI

struct HQParticipantAdd: View {
    let l10n: L10n
    @ObservedObject var store: HQWorkspaceStore
    let matter: HQMatter
    @ObservedObject var flow: HQParticipantManagement
    private var selected: HQCandidate? { store.snapshot?.selectableConversations.first { $0.id == flow.picked } }
    private var linked: Set<String> { Set(matter.activeParticipants.map { $0.conversation.id }) }
    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text(l10n.tr("Choose an agent conversation to collect here. This does not send it an instruction.", "选择要收录到此事项的 agent 对话。添加不会向它发送指令。"))
                .foregroundStyle(.secondary)
            if let c = selected {
                selectedConversation(c)
            } else {
                candidates(l10n.tr("Available in Terminal", "当前终端中的对话"), rows: store.snapshot?.conversations ?? [], historical: false)
                candidates(l10n.tr("Saved conversations", "过去保存的对话记录"), rows: store.snapshot?.references ?? [], historical: true)
            }
        }
    }
    private func candidates(_ title: String, rows: [HQCandidate], historical: Bool) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(title).font(HQTypography.section).accessibilityAddTraits(.isHeader)
            let available = rows.filter { !linked.contains($0.id) }
            if available.isEmpty {
                Text(l10n.tr("No more conversations available.", "暂无其他可添加的对话。"))
                    .font(HQTypography.caption).foregroundStyle(.secondary)
            }
            ForEach(available) { c in
                Button {
                    flow.picked = c.id
                    flow.reference = historical
                    flow.ended = historical
                    // Historical records have no reliable start clock. Keep the
                    // explicit interval fields rather than inventing session age.
                    flow.until = Date()
                    flow.from = Date()
                    if historical {
                        let recorded = Date(timeIntervalSince1970: TimeInterval(c.since > 0 ? c.since : Int64(Date().timeIntervalSince1970)))
                        flow.from = Calendar.current.startOfDay(for: recorded)
                    }
                } label: {
                    VStack(alignment: .leading, spacing: 6) {
                        Text(c.name).font(HQTypography.rowTitle).foregroundStyle(.primary)
                            .fixedSize(horizontal: false, vertical: true)
                        Text(c.agent + " · " + (historical ? l10n.tr("Saved record", "已保存记录") : hqConversationStatus(c.status, l10n)))
                            .font(HQTypography.caption).foregroundStyle(.secondary)
                        if let loc = c.loc { Text((historical ? l10n.tr("Last location: ", "最近位置：") : l10n.tr("Terminal: ", "终端位置：")) + loc).font(HQTypography.caption).foregroundStyle(.secondary) }
                        Text(relationship(c)).font(HQTypography.caption).foregroundStyle(.secondary)
                        if c.since > 0 {
                            Text((historical ? l10n.tr("Record updated: ", "记录更新于：") : l10n.tr("In this state since ", "当前状态始于 ")) + HQParticipantRecord.date(c.since, l10n))
                                .font(HQTypography.caption).foregroundStyle(.secondary)
                        }
                        Text(l10n.tr("Choose →", "选择此对话 →")).font(HQTypography.caption)
                    }.padding(12).frame(maxWidth: .infinity, alignment: .leading)
                        .background(Color.primary.opacity(0.03), in: RoundedRectangle(cornerRadius: 9))
                }.buttonStyle(.plain).accessibilityIdentifier("participants-choose-\(c.sessionId)")
            }
        }
    }
    private func relationship(_ c: HQCandidate) -> String {
        let others = (store.snapshot?.matters ?? []).filter { $0.id != matter.id && $0.activeParticipants.contains { $0.conversation.id == c.id } }.map(\.title)
        return others.isEmpty ? l10n.tr("Not included in another matter", "尚未收录到其他事项")
            : l10n.tr("Also in: ", "也收录于：") + others.joined(separator: "、")
    }
    private func selectedConversation(_ c: HQCandidate) -> some View {
        VStack(alignment: .leading, spacing: 14) {
            Text(c.name).font(HQTypography.section).fixedSize(horizontal: false, vertical: true)
            Text(c.agent + " · " + (c.status == "historical" ? l10n.tr("Saved record", "已保存记录") : hqConversationStatus(c.status, l10n)))
                .foregroundStyle(.secondary)
            Button(l10n.tr("Choose a different conversation", "重新选择对话")) { flow.picked = "" }
                .buttonStyle(.borderless).accessibilityIdentifier("participants-rechoose")
            if c.status != "historical" {
                Toggle(l10n.tr("Use existing records as background only", "只引用已有记录作为背景"), isOn: $flow.reference)
                    .onChange(of: flow.reference) { _, value in flow.ended = value; flow.until = Date() }
                    .accessibilityIdentifier("participants-reference")
            }
            Text(flow.reference
                ? l10n.tr("Include a fixed range as historical reference. Later messages will not be collected.", "作为历史参考收录指定范围，之后的新问答不会纳入。")
                : l10n.tr("Collect new messages from now on. To include earlier messages, change the start time below.", "默认从现在起持续收录新问答。需要带上之前的内容，可调整下方开始时间。"))
                .foregroundStyle(.secondary)
            if c.status == "historical" {
                Text(l10n.tr("Choose the range to quote. The start defaults to the day the record was updated; move it earlier to include more history.", "选择要引用的时间范围。默认从记录更新当天开始，可向前调整以包含更早内容。"))
                    .font(HQTypography.caption).foregroundStyle(.secondary)
            }
            if c.status == "historical" && !flow.reference {
                Text(l10n.tr("This conversation is no longer in Terminal. Choose it again to add its saved records.", "此对话已离开当前终端。请重新选择，以添加它保存的记录。"))
                    .font(HQTypography.caption).foregroundStyle(Theme.Status.errored)
            }
            HQParticipantRangeFields(l10n: l10n, flow: flow)
            Button(l10n.tr("Add to this matter", "添加到此事项")) {
                // Recheck the selected identity against the latest snapshot.
                guard let latest = selected, latest.status != "historical" || flow.reference else { return }
                let a = HQParticipant(id: UUID().uuidString.lowercased(), conversation: latest.conversation,
                    role: flow.reference ? "reference" : "execution", from: Int64(flow.from.timeIntervalSince1970),
                    until: flow.reference || flow.ended ? Int64(flow.until.timeIntervalSince1970) : nil)
                store.perform(HQWorkspaceAction(op: "associate", matter: matter.id, revision: flow.revision, association: a)) {
                    flow.finished(l10n.tr("Conversation added to this matter.", "已将对话添加到此事项。"))
                }
            }.disabled(!flow.validInterval() || c.status == "historical" && !flow.reference).accessibilityIdentifier("participants-add-save")
        }
    }
}

struct HQParticipantRangeFields: View {
    let l10n: L10n
    @ObservedObject var flow: HQParticipantManagement
    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(l10n.tr("Include messages starting at", "从这个时间开始收录")).font(HQTypography.rowTitle)
            DatePicker(l10n.tr("Start time", "开始时间"), selection: $flow.from, in: ...Date())
                .labelsHidden().accessibilityIdentifier("participants-from")
            if !flow.reference {
                Toggle(l10n.tr("Stop collecting at a specific time", "在指定时间停止收录"), isOn: $flow.ended)
                    .accessibilityIdentifier("participants-ended")
                    .onChange(of: flow.ended) { _, value in if value { flow.until = Date() } }
            }
            if flow.reference || flow.ended {
                Text(l10n.tr("Include messages until", "收录到这个时间为止")).font(HQTypography.rowTitle)
                DatePicker(l10n.tr("End time", "结束时间"), selection: $flow.until, in: ...Date())
                    .labelsHidden().accessibilityIdentifier("participants-until")
                Text(l10n.tr("Messages after this time stay in the original conversation. The agent is not stopped.", "此时间之后的问答仍保存在原始对话中，不会停止 agent。"))
                    .font(HQTypography.caption).foregroundStyle(.secondary)
            }
            if !flow.validInterval() {
                Text(l10n.tr("End time must be after the start and no later than now.", "结束时间须晚于开始时间，且不能晚于现在。"))
                    .font(HQTypography.caption).foregroundStyle(Theme.Status.errored)
            }
        }
    }
}
