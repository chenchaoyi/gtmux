import AppKit
import SwiftUI

struct DesktopConversationView: View {
    @ObservedObject var model: DesktopConversationModel
    @ObservedObject var l10n: L10n
    /// Native harnesses observe the tail marker, not LazyVStack's estimated document size.
    var scrollObserver: ((ScrollViewProxy, Bool, Bool) -> Void)? = nil
    @Environment(\.colorScheme) private var scheme
    @State private var collapsed: Set<String> = []
    @State private var showEvidence = false
    @State private var atTail = true
    @State private var tailObserved = false
    @State private var didInitialReveal = false
    @State private var positioning = true
    @State private var revealSerial: UInt64 = 0
    @StateObject private var scrollIntent = HQConversationScrollIntent()
    private var entries: [DesktopConversationEntry] {
        model.turns.enumerated().map { DesktopConversationEntry(id: $0.element.identity(index: $0.offset), turn: $0.element) }
    }
    private var ids: [String] { entries.map(\.id) }
    private var allCollapsed: Bool { !ids.isEmpty && ids.allSatisfy { collapsed.contains($0) } }

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(spacing: 0) {
            HStack(spacing: 10) {
                HQConversationIcon(agent: model.agent.agent, hint: model.agent.icon).frame(width: 28, height: 28)
                    .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 3) {
                    Text(nativeOriginLabel(model.agent, l10n: l10n)).font(HQTypography.navigation)
                    Text(l10n.tr("Continue in the original client", "请在原客户端继续对话"))
                        .font(HQTypography.caption).foregroundStyle(p.fg2)
                }
                Spacer(minLength: 8)
                Label(l10n.tr("Read only", "只读"), systemImage: "lock")
                    .font(HQTypography.caption).foregroundStyle(p.fg2)
                Button { showEvidence.toggle() } label: { Image(systemName: "info.circle") }
                    .buttonStyle(.plain).foregroundStyle(p.fg2)
                    .help(l10n.tr("Conversation source", "会话来源"))
                    .accessibilityLabel(l10n.tr("Conversation source", "会话来源"))
                    .popover(isPresented: $showEvidence) {
                        VStack(alignment: .leading, spacing: 8) {
                            Text(l10n.tr("Conversation source", "会话来源")).font(HQTypography.rowTitle)
                            Text(nativeEvidenceLabel(model.agent, l10n: l10n))
                                .font(HQTypography.caption).foregroundStyle(.secondary).textSelection(.enabled)
                        }.padding(16).frame(maxWidth: 340)
                    }
            }.padding(.horizontal, 20).padding(.vertical, 12)
            Divider()
            if model.failed {
                HStack(spacing: 12) {
                    Image(systemName: "exclamationmark.circle")
                    Text(model.turns.isEmpty
                        ? l10n.tr("Could not load the conversation.", "暂时无法加载对话。")
                        : l10n.tr("Could not refresh. Loaded messages are kept.", "暂时无法刷新，已加载的对话仍可阅读。"))
                        .frame(maxWidth: .infinity, alignment: .leading)
                    Button(l10n.tr("Retry", "重试")) { model.refresh() }.buttonStyle(.borderless)
                }.font(HQTypography.caption).foregroundStyle(p.fg2).padding(14)
            }
            ScrollViewReader { proxy in
                VStack(spacing: 0) {
                    HStack {
                        if model.loading { ProgressView().controlSize(.small); Text(l10n.tr("Loading conversation…", "正在加载对话…")).font(HQTypography.caption).foregroundStyle(p.fg2) }
                        Spacer()
                        if !model.turns.isEmpty {
                            Button(l10n.tr(allCollapsed ? "Expand all" : "Collapse all", allCollapsed ? "全部展开" : "全部折叠")) {
                                collapsed = allCollapsed ? [] : Set(ids)
                            }.buttonStyle(.plain).foregroundStyle(Color.accentColor).font(HQTypography.caption)
                                .accessibilityIdentifier("desktop-fold-all")
                        }
                    }.padding(.horizontal, 20).padding(.vertical, 10)
                    GeometryReader { viewport in
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 28) {
                            if model.dropped > 0 {
                                Text(l10n.tr("\(model.dropped) earlier turns omitted", "已省略 \(model.dropped) 轮较早对话"))
                                    .font(HQTypography.caption).foregroundStyle(p.fg2).frame(maxWidth: .infinity)
                            }
                            if !model.loading && !model.failed && model.turns.isEmpty {
                                ContentUnavailableView(l10n.tr("No recorded messages", "尚无已记录的对话"), systemImage: "text.bubble")
                            }
                            let separators = HQChatTimeline.separators(model.turns.map { HQChatTimeline.timestamp($0.time) })
                            ForEach(Array(entries.enumerated()), id: \.element.id) { index, entry in
                                if let date = separators[index] { HQChatTimeSeparator(date: date, l10n: l10n) }
                                let id = entry.id
                                let turn = entry.turn
                                DesktopConversationTurnView(turn: turn, agent: model.agent, l10n: l10n, expanded: !collapsed.contains(id)) {
                                    if collapsed.contains(id) { collapsed.remove(id) } else { collapsed.insert(id) }
                                }.id(id)
                            }
                            Color.clear.frame(height: 1).id("latest")
                                .background(GeometryReader { geometry in
                                    Color.clear.preference(key: DesktopConversationTail.self,
                                        value: geometry.frame(in: .named("desktopConversationScroll")).maxY)
                                })
                        }.padding(.horizontal, 24).padding(.vertical, 14)
                            .frame(maxWidth: 780).frame(maxWidth: .infinity)
                    }
                    .accessibilityIdentifier("desktop-conversation-scroll")
                    .modifier(HQConversationInitialAnchor())
                    .coordinateSpace(name: "desktopConversationScroll")
                    .onPreferenceChange(DesktopConversationTail.self) { y in
                        tailObserved = y < .greatestFiniteMagnitude
                        atTail = y <= viewport.size.height + 36
                        scrollObserver?(proxy, atTail, !positioning)
                    }
                    .task(id: revealSerial) {
                        guard didInitialReveal else { return }
                        positioning = true
                        let intent = scrollIntent.generation
                        scrollObserver?(proxy, atTail, false)
                        _ = await HQConversationTailReveal.run(
                            isCurrent: { intent == scrollIntent.generation },
                            reveal: { proxy.scrollTo("latest", anchor: .bottom) },
                            isVisible: { tailObserved && atTail })
                        guard !Task.isCancelled else { return }
                        positioning = false
                        scrollObserver?(proxy, atTail, true)
                    }
                    .onChange(of: model.turns.isEmpty, initial: true) { _, empty in
                        if !empty && !didInitialReveal { didInitialReveal = true; revealSerial &+= 1 }
                    }
                    .onChange(of: model.turns) { old, _ in
                        if !old.isEmpty && atTail { proxy.scrollTo("latest", anchor: .bottom) }
                    }
                    }
                    HStack {
                        Spacer()
                        Button { revealSerial &+= 1 } label: {
                            Label(l10n.tr("Latest messages", "最新消息"), systemImage: "arrow.down")
                        }.buttonStyle(.plain).font(HQTypography.caption).foregroundStyle(p.fg2).accessibilityIdentifier("desktop-latest")
                    }.padding(.horizontal, 20).padding(.vertical, 10)
                }
            }
        }.frame(maxWidth: .infinity, maxHeight: .infinity).background(p.bg)
            .environment(\.markdownTypography, .workspace)
            .onChange(of: ids) { _, values in collapsed.formIntersection(values) }
    }
}

struct DesktopConversationTurnView: View {
    let turn: DesktopChatTurn
    let agent: Agent
    let l10n: L10n
    let expanded: Bool
    let toggle: () -> Void
    @Environment(\.colorScheme) private var scheme
    @StateObject private var copy = HQChatCopyFeedback()
    private var p: Theme.Palette { .of(scheme) }

    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            if !turn.prompt.isEmpty {
                HStack(alignment: .top, spacing: 10) {
                    if turn.isHuman { Spacer(minLength: 44) } else { senderAvatar }
                    VStack(alignment: .leading, spacing: 6) {
                        HStack {
                            Text(turn.senderLabel(l10n)).font(HQTypography.caption.weight(.medium)).accessibilityIdentifier("desktop-prompt-sender")
                            Spacer(minLength: 0)

                        }.foregroundStyle(p.fg2)
                        MarkdownBody(markdown: turn.prompt, p: p).textSelection(.enabled)
                            .padding(12).frame(maxWidth: .infinity, alignment: .leading)
                            .background(p.rowSelected, in: RoundedRectangle(cornerRadius: 12))
                    }
                    if turn.isHuman { senderAvatar } else { Spacer(minLength: 44) }
                }
            }
            if !turn.fullReply.isEmpty || !(turn.segments ?? []).isEmpty {
                HStack(alignment: .top, spacing: 10) {
                    HQConversationIcon(agent: agent.agent, hint: agent.icon).frame(width: 28, height: 28).clipShape(RoundedRectangle(cornerRadius: 6)).accessibilityHidden(true)
                    VStack(alignment: .leading, spacing: 12) {
                        Text(agent.agent).font(HQTypography.caption.weight(.medium)).foregroundStyle(p.fg2)
                        if expanded {
                            VStack(alignment: .leading, spacing: 12) {
                            ForEach(Array(displaySegments.enumerated()), id: \.offset) { _, segment in
                                if let text = segment.text, !text.isEmpty {
                                    MarkdownBody(markdown: text, p: p).textSelection(.enabled)
                                }
                                if let steps = segment.steps, !steps.isEmpty {
                                    ConversationToolSteps(steps: steps, l10n: l10n)
                                }
                            }
                            }
                        } else {
                            Text(turn.fullReply.isEmpty ? l10n.tr("Recorded tool calls", "已记录工具调用") : turn.preview)
                                .font(HQTypography.body).foregroundStyle(p.fg2).lineLimit(3)
                        }
                        HStack(spacing: 14) {
                            if !turn.fullReply.isEmpty {
                                Button { copy.copy(turn.fullReply) } label: {
                                    Image(systemName: copy.state == .copied ? "checkmark" : "doc.on.doc")
                                }.buttonStyle(.plain).help(copyLabel).accessibilityLabel(copyLabel).accessibilityIdentifier("desktop-copy-reply")
                                ShareLink(item: turn.fullReply) { Image(systemName: "square.and.arrow.up") }
                                    .buttonStyle(.plain).help(l10n.tr("Share reply", "分享回复"))
                                    .accessibilityLabel(l10n.tr("Share reply", "分享回复"))
                            }
                            Spacer(minLength: 0)
                            Button(l10n.tr(expanded ? "Collapse reply" : "Expand reply", expanded ? "收起本轮" : "展开本轮"), action: toggle)
                                .buttonStyle(.plain).foregroundStyle(Color.accentColor).accessibilityIdentifier("desktop-reply-fold")
                        }.font(HQTypography.caption).foregroundStyle(p.fg2).padding(.top, 2)
                    }.frame(maxWidth: .infinity, alignment: .leading)
                }
            }
        }.onChange(of: turn.fullReply) { _, _ in copy.clear() }.onDisappear { copy.clear() }
    }
    private var displaySegments: [DesktopChatSegment] {
        guard let segments = turn.segments, !segments.isEmpty else { return [DesktopChatSegment(text: turn.response, steps: nil)] }
        return segments
    }
    private var copyLabel: String {
        switch copy.state {
        case .ready: return l10n.tr("Copy full reply", "复制回复全文")
        case .copied: return l10n.tr("Copied", "已复制")
        case .failed: return l10n.tr("Copy failed · try again", "复制失败 · 请重试")
        }
    }
    @ViewBuilder private var senderAvatar: some View {
        if turn.isHuman { HQUserAvatar().frame(width: 28, height: 28) }
        else if turn.from?.kind == "hq" { GtmuxLogo(size: 28) }
        else { HQConversationIcon(agent: turn.from?.label ?? "Agent", hint: turn.from?.agent).frame(width: 28, height: 28) }
    }
}

private struct DesktopConversationTail: PreferenceKey {
    static var defaultValue: CGFloat = .greatestFiniteMagnitude
    static func reduce(value: inout CGFloat, nextValue: () -> CGFloat) { value = nextValue() }
}
