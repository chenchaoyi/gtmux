import AppKit
import CryptoKit
import SwiftUI

/// The mobile UserAvatar's supplied 40 × 40 geometry, shared without a new identity.
struct HQUserAvatar: View {
    var body: some View {
        Canvas { context, size in
            context.scaleBy(x: size.width / 40, y: size.height / 40)
            context.fill(
                Path(ellipseIn: CGRect(x: 0, y: 0, width: 40, height: 40)),
                with: .linearGradient(
                    Gradient(colors: [
                        Color(red: 34 / 255, green: 211 / 255, blue: 238 / 255),
                        Color(red: 14 / 255, green: 116 / 255, blue: 144 / 255),
                    ]),
                    startPoint: .zero, endPoint: CGPoint(x: 40, y: 40)))
            context.stroke(
                Path(roundedRect: CGRect(x: 9, y: 7, width: 22, height: 28), cornerRadius: 4),
                with: .color(.white), lineWidth: 2.4)
            context.fill(
                Path(roundedRect: CGRect(x: 15.5, y: 4, width: 9, height: 4), cornerRadius: 1.5),
                with: .color(.white))
            context.fill(
                Path(ellipseIn: CGRect(x: 16.6, y: 13.6, width: 6.8, height: 6.8)), with: .color(.white))
            var shoulders = Path()
            shoulders.move(to: CGPoint(x: 13.5, y: 30))
            shoulders.addQuadCurve(to: CGPoint(x: 20, y: 23), control: CGPoint(x: 13.5, y: 23))
            shoulders.addQuadCurve(to: CGPoint(x: 26.5, y: 30), control: CGPoint(x: 26.5, y: 23))
            shoulders.closeSubpath()
            context.fill(shoulders, with: .color(.white))
        }.accessibilityHidden(true)
    }
}

struct HQConversationRow: Identifiable {
    var id: String
    var at: Int64
    var order: UInt64 = 0
    var message: HQWorkspaceMessage?
    var turn: HQChatTurn?
    var changeToken: String {
        let text =
            (message?.body ?? "") + (message?.delivery ?? "") + (message?.reply?.response ?? "")
            + (turn?.response ?? "")
        return id + SHA256.hash(data: Data(text.utf8)).map { String(format: "%02x", $0) }.joined()
    }
    static func rows(_ detail: HQWorkspaceDetail?) -> [HQConversationRow] {
        guard let detail else { return [] }
        var rows = detail.messages.map { HQConversationRow(id: $0.id, at: $0.at, order: $0.seq, message: $0) }
        var occurrences: [String: Int] = [:]
        for turn in detail.turns {
            let source = (turn.time ?? "") + turn.prompt + (turn.sessionBreak?.kind ?? "")
            let key = SHA256.hash(data: Data(source.utf8)).map { String(format: "%02x", $0) }.joined()
            let format = ISO8601DateFormatter()
            format.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
            let occurrence = occurrences[key, default: 0]
            occurrences[key] = occurrence + 1
            let date =
                format.date(from: turn.time ?? "") ?? ISO8601DateFormatter().date(from: turn.time ?? "")
            rows.append(
                HQConversationRow(
                    id: "source-" + key + "-\(occurrence)", at: Int64(date?.timeIntervalSince1970 ?? 0),
                    turn: turn))
        }
        return rows.sorted {
            $0.at == $1.at ? ($0.order == $1.order ? $0.id < $1.id : $0.order < $1.order) : $0.at < $1.at
        }
    }
}

private struct HQTailPreference: PreferenceKey {
    static var defaultValue: CGFloat = 0
    static func reduce(value: inout CGFloat, nextValue: () -> CGFloat) { value = nextValue() }
}

struct HQWorkspaceConversationView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQWorkspaceStore
    @State private var atTail = true
    @State private var newContent = false
    @State private var uncertain: HQWorkspaceMessage?
    var body: some View {
        let rows = HQConversationRow.rows(store.detail)
        GeometryReader { bounds in
            ScrollViewReader { proxy in
                ZStack(alignment: .bottom) {
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 20) {
                            if store.detail?.earlierAvailable == true {
                                Button(l10n.tr("Earlier conversation", "更早的对话")) { store.loadEarlier() }
                                    .disabled(store.loading).frame(maxWidth: .infinity)
                            }
                            if rows.isEmpty {
                                Text(l10n.tr("No conversation yet.", "还没有对话。"))
                                    .font(Theme.Font.task).foregroundStyle(.secondary).padding(.vertical, 40)
                                    .frame(maxWidth: .infinity)
                            }
                            ForEach(rows) { row in
                                if let message = row.message {
                                    messageRow(message)
                                } else if let turn = row.turn {
                                    HQTranscriptExchange(l10n: l10n, turn: turn)
                                }
                            }
                            ForEach(Array(Set(store.detail?.warnings ?? [])).sorted(), id: \.self) { code in
                                Text(hqWorkspaceWarning(code, l10n)).font(Theme.Font.window).foregroundStyle(
                                    .secondary)
                            }
                            Color.clear.frame(height: 1).id("tail")
                                .background(
                                    GeometryReader { geometry in
                                        Color.clear.preference(
                                            key: HQTailPreference.self,
                                            value: geometry.frame(in: .named("workspaceScroll")).maxY)
                                    })
                        }.padding(18)
                    }.coordinateSpace(name: "workspaceScroll")
                        .onChange(of: store.selection) { _, _ in
                            atTail = true
                            newContent = false
                        }
                        .onPreferenceChange(HQTailPreference.self) { y in
                            atTail = y <= bounds.size.height + 36
                        }
                        .onChange(of: rows.map(\.changeToken)) { old, new in
                            guard old != new else { return }
                            if atTail || old.isEmpty {
                                proxy.scrollTo("tail", anchor: .bottom)
                            } else if Array(new.suffix(old.count)) != old {
                                newContent = true
                            }
                        }
                    if newContent {
                        Button(l10n.tr("New messages ↓", "新消息 ↓")) {
                            proxy.scrollTo("tail", anchor: .bottom)
                            newContent = false
                        }
                        .buttonStyle(.borderedProminent).padding(12)
                    }
                }
            }
        }
        .confirmationDialog(
            l10n.tr(
                "This instruction may already have reached HQ. Sending again could repeat the action.",
                "这条指令可能已送达 HQ，再次发送可能重复执行。"),
            isPresented: Binding(get: { uncertain != nil }, set: { if !$0 { uncertain = nil } }),
            titleVisibility: .visible
        ) {
            Button(l10n.tr("Send again", "确认再次发送")) {
                if let message = uncertain {
                    store.perform(HQWorkspaceAction(op: "retry", id: message.id, acknowledgeUncertain: true))
                    { store.attemptPending() }
                }
                uncertain = nil
            }
            Button(l10n.tr("Cancel", "取消"), role: .cancel) { uncertain = nil }
        }
    }
    @ViewBuilder private func messageRow(_ message: HQWorkspaceMessage) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            HQChatBubble(
                l10n: l10n, user: message.speaker == "user",
                label: message.speaker == "user" ? l10n.tr("You", "你") : "HQ", bodyText: message.body)
            if message.speaker == "user" {
                HStack(spacing: 8) {
                    Spacer(minLength: 40)
                    Text(hqDeliveryLabel(message, l10n)).font(Theme.Font.window).foregroundStyle(
                        message.delivery == "uncertain" ? Theme.Status.errored : Color.secondary)
                    if message.delivery == "pending" {
                        Button(
                            store.delivering ? l10n.tr("Checking…", "正在核对…") : l10n.tr("Try delivery", "尝试送达")
                        ) { store.attemptPending() }.disabled(store.delivering)
                        Button(l10n.tr("Withdraw", "撤回")) {
                            store.perform(HQWorkspaceAction(op: "cancel", id: message.id))
                        }
                    } else if message.delivery == "uncertain" {
                        Button(l10n.tr("Send again…", "再次发送…")) { uncertain = message }
                    }
                }.buttonStyle(.borderless).disabled(store.acting)
                if let evidence = message.evidence, !evidence.isEmpty {
                    DisclosureGroup(l10n.tr("Delivery evidence", "送达凭据")) {
                        Text(evidence).font(.system(size: 11, design: .monospaced)).textSelection(.enabled)
                    }.font(Theme.Font.window).foregroundStyle(.secondary).padding(.leading, 40)
                }
                if let reply = message.reply { HQChatResponse(l10n: l10n, turn: reply) }
            }
        }
    }
}

struct HQTranscriptExchange: View {
    let l10n: L10n
    let turn: HQChatTurn
    var body: some View {
        if let sessionBreak = turn.sessionBreak {
            Label(l10n.tr("HQ conversation changed", "HQ 会话已切换"), systemImage: "arrow.triangle.2.circlepath")
                .font(Theme.Font.window).foregroundStyle(.secondary).help(sessionBreak.kind)
        } else if turn.signal == true {
            DisclosureGroup(l10n.tr("Supervision signal and processing", "观察信号与处理过程")) {
                VStack(alignment: .leading, spacing: 10) {
                    Text(turn.prompt).font(.system(size: 11, design: .monospaced)).textSelection(.enabled)
                    HQChatResponse(l10n: l10n, turn: turn)
                }.padding(.top, 8)
            }.font(Theme.Font.task).foregroundStyle(.secondary)
        } else {
            VStack(alignment: .leading, spacing: 12) {
                if !turn.prompt.isEmpty {
                    HQChatBubble(
                        l10n: l10n, user: turn.from == nil || turn.from?.kind == "user",
                        label: turn.from?.label ?? l10n.tr("You", "你"), bodyText: turn.prompt)
                }
                HQChatResponse(l10n: l10n, turn: turn)
            }
        }
    }
}

struct HQChatResponse: View {
    let l10n: L10n
    let turn: HQChatTurn
    var speakerLabel = "HQ"
    var icon: String?
    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            if let segments = turn.segments, !segments.isEmpty {
                ForEach(Array(segments.enumerated()), id: \.offset) { _, segment in
                    if let text = segment.text, !text.isEmpty {
                        HQChatBubble(l10n: l10n, user: false, label: speakerLabel, bodyText: text, icon: icon)
                    }
                    if let steps = segment.steps, !steps.isEmpty {
                        DisclosureGroup(l10n.tr("Work details · \(steps.count)", "执行详情 · \(steps.count)")) {
                            ForEach(Array(steps.enumerated()), id: \.offset) { _, step in
                                VStack(alignment: .leading, spacing: 4) {
                                    Text(step.title).font(Theme.Font.session)
                                    if let detail = step.detail {
                                        Text(detail).font(.system(size: 11, design: .monospaced))
                                            .textSelection(.enabled)
                                    }
                                }.padding(.vertical, 6)
                            }
                        }.font(Theme.Font.window).padding(.leading, 40)
                    }
                }
            } else if !turn.response.isEmpty {
                HQChatBubble(
                    l10n: l10n, user: false, label: speakerLabel, bodyText: turn.response, icon: icon)
            }
        }
    }
}

struct HQChatBubble: View {
    let l10n: L10n
    let user: Bool
    let label: String
    let bodyText: String
    var icon: String?
    @Environment(\.colorScheme) private var scheme
    @State private var copied = false
    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            if user {
                Spacer(minLength: 45)
            } else if label == "HQ" {
                GtmuxLogo(size: 28)
            } else {
                HQConversationIcon(agent: label, hint: icon).frame(width: 28, height: 28)
            }
            VStack(alignment: .leading, spacing: 7) {
                HStack {
                    Text(label).font(Theme.Font.window).foregroundStyle(.secondary)
                    Spacer()
                    Button {
                        NSPasteboard.general.clearContents()
                        NSPasteboard.general.setString(bodyText, forType: .string)
                        copied = true
                    } label: {
                        Image(systemName: copied ? "checkmark" : "doc.on.doc")
                    }
                    .buttonStyle(.borderless).help(l10n.tr("Copy full text", "复制全文"))
                }
                MarkdownBody(markdown: bodyText, p: Theme.Palette.of(scheme))
            }.padding(12).background(
                user ? Theme.Status.working.opacity(0.07) : Color.primary.opacity(0.025),
                in: RoundedRectangle(cornerRadius: 10))
            if user { HQUserAvatar().frame(width: 28, height: 28) } else { Spacer(minLength: 45) }
        }.onChange(of: bodyText) { _, _ in copied = false }
    }
}

func hqDeliveryLabel(_ m: HQWorkspaceMessage, _ l: L10n) -> String {
    switch m.delivery {
    case "delivered": return l.tr("Delivered to HQ", "已送达 HQ")
    case "accepted-queue": return l.tr("Accepted in HQ queue", "已进入 HQ 队列")
    case "submitting": return l.tr("Confirming delivery…", "正在确认送达…")
    case "cancelled": return l.tr("Withdrawn before delivery", "已撤回，未送达")
    case "uncertain": return l.tr("Delivery unconfirmed", "未能确认送达")
    default:
        switch m.reason {
        case "hq-busy": return l.tr("Saved · HQ is working", "已保存 · 等待 HQ 空闲")
        case "hq-waiting": return l.tr("Saved · HQ needs input in Terminal", "已保存 · HQ 正在等待终端输入")
        case "refused-draft": return l.tr("Saved · preserving HQ input draft", "已保存 · HQ 输入框有草稿")
        case "hq-input-locked": return l.tr("Saved · another HQ input is in progress", "已保存 · 等待其他 HQ 输入结束")
        case "hq-input-lock-failed": return l.tr("Saved · HQ input lock unavailable", "已保存 · HQ 输入锁暂不可用")
        case "matter-closed": return l.tr("Saved · matter is closed", "已保存 · 事项已结束，暂停送达")
        case "hq-offline": return l.tr("Saved · HQ is not running", "已保存 · HQ 尚未运行")
        default: return l.tr("Saved · pending delivery", "已保存 · 待送达")
        }
    }
}

func hqConversationStatus(_ status: String, _ l: L10n) -> String {
    switch status {
    case "working": return l.tr("Working", "运行中")
    case "waiting": return l.tr("Needs input", "等待输入")
    case "idle": return l.tr("Idle", "空闲")
    default: return l.tr("Status unavailable", "状态未知")
    }
}
func hqWorkspaceWarning(_ code: String, _ l: L10n) -> String {
    switch code {
    case "reply-source-limit":
        return l.tr(
            "Reply previews cover the latest eight HQ conversation sources. Older original records are retained.",
            "回复预览覆盖最近八个 HQ 会话来源，更早的原始记录仍保留。")
    case "reply-source-unavailable":
        return l.tr(
            "An original reply record could not be read. Saved instructions remain available.",
            "部分原始回复暂不可读，已保存的指令仍可查看。")
    case "large-reply-original-only":
        return l.tr(
            "A large source reply is available in the original conversation, not in this bounded preview.",
            "超长回复请在原始会话中查看，当前预览未载入全文。")
    case "source-tail-limited":
        return l.tr(
            "Source history is a bounded preview. Open the original conversation for the full record.",
            "原始历史为有限预览，完整记录请在原始会话中查看。")
    case "activity-limited": return l.tr("Showing the latest 200 activity records.", "显示最近 200 条运行记录。")
    case "participant-history-limited":
        return l.tr(
            "Showing up to 200 recent turns within the selected interval. Older source records are unchanged.",
            "当前显示所选时段内最近 200 轮对话，更早的原始记录仍保留。")
    case "undated-turn-omitted":
        return l.tr(
            "Records without a source timestamp cannot be placed in this interval.", "没有来源时间的记录无法归入所选时段。")
    default: return code
    }
}
