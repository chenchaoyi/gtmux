import SwiftUI

struct HQWorkspaceView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQWorkspaceStore
    @Environment(\.colorScheme) private var scheme
    @State private var manage = false
    @State private var rename = false
    @State private var titleDraft = ""

    var body: some View {
        NavigationSplitView {
            HQWorkspaceSidebar(l10n: l10n, store: store)
                .navigationSplitViewColumnWidth(min: 200, ideal: 235, max: 300)
        } detail: {
            VStack(spacing: 0) {
                if let error = store.error {
                    HStack(alignment: .top) {
                        Image(systemName: "exclamationmark.triangle").foregroundStyle(Theme.Status.errored)
                        Text(error).font(Theme.Font.task).textSelection(.enabled)
                        Spacer()
                        Button(l10n.tr("Reload", "重新读取")) { store.refresh() }
                    }.padding(12).background(Theme.Status.errored.opacity(0.06))
                }
                if store.selection == .activity {
                    HQWorkspaceActivityView(l10n: l10n, detail: store.detail)
                } else if store.selection == .newTask {
                    newTask
                } else {
                    detailBody
                }
                if store.selection != .activity {
                    Divider()
                    HQWorkspaceComposer(l10n: l10n, store: store)
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
        .navigationTitle("gtmux HQ")
        .toolbar { toolbar }
        .sheet(isPresented: $manage) {
            if let matter = store.currentMatter {
                HQWorkspaceParticipants(l10n: l10n, store: store, matter: matter)
            }
        }
        .sheet(isPresented: $rename) {
            VStack(alignment: .leading, spacing: 16) {
                Text(l10n.tr("Rename matter", "修改事项名称")).font(Theme.Font.title)
                TextField(l10n.tr("Name", "名称"), text: $titleDraft).textFieldStyle(.roundedBorder)
                HStack {
                    Spacer()
                    Button(l10n.tr("Cancel", "取消")) { rename = false }
                    Button(l10n.tr("Rename", "修改")) {
                        guard let matter = store.currentMatter else { return }
                        store.perform(
                            HQWorkspaceAction(
                                op: "update", matter: matter.id, title: titleDraft, revision: matter.revision)
                        ) { rename = false }
                    }.disabled(
                        titleDraft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || store.acting)
                }
            }.padding(22).frame(width: 400)
        }
        .onAppear { store.start() }
        .onDisappear { store.stop() }
    }

    @ViewBuilder private var detailBody: some View {
        if let matter = store.currentMatter {
            VStack(alignment: .leading, spacing: 10) {
                HStack(alignment: .top) {
                    VStack(alignment: .leading, spacing: 5) {
                        Text(matter.title).font(Theme.Font.title).textSelection(.enabled)
                        Text(matter.stateLabel(l10n)).font(Theme.Font.task)
                            .foregroundStyle(
                                matter.state == "needs-input" ? Theme.Status.waiting : Color.secondary)
                    }
                    Spacer()
                    Menu {
                        Button(l10n.tr("Rename…", "修改名称…")) {
                            titleDraft = matter.title
                            rename = true
                        }
                        Button(l10n.tr("Mark completed", "标记完成")) {
                            store.perform(
                                HQWorkspaceAction(
                                    op: "update", matter: matter.id, state: "completed",
                                    revision: matter.revision))
                        }
                        Button(l10n.tr("Continue following", "继续跟进")) {
                            store.perform(
                                HQWorkspaceAction(
                                    op: "update", matter: matter.id, state: "open", revision: matter.revision)
                            )
                        }
                        Button(l10n.tr("Close matter", "结束事项")) {
                            store.perform(
                                HQWorkspaceAction(
                                    op: "update", matter: matter.id, state: "closed",
                                    revision: matter.revision))
                        }
                    } label: {
                        Image(systemName: "ellipsis")
                    }.menuStyle(.borderlessButton).frame(width: 28)
                }
                HStack {
                    Text(l10n.tr("Conversations", "关联会话")).font(Theme.Font.session)
                    Spacer()
                    Button(l10n.tr("Manage…", "管理会话…")) { manage = true }
                }
                HQParticipantSummary(l10n: l10n, store: store, matter: matter)
                if let parent = matter.parent,
                    let source = store.snapshot?.matters.first(where: { $0.id == parent })
                {
                    Button(l10n.tr("Split from: \(source.title)", "拆分自：\(source.title)")) {
                        store.select(.matter(parent))
                    }.buttonStyle(.borderless).font(Theme.Font.window)
                }
                if let assessment = matter.assessment, !assessment.isEmpty {
                    DisclosureGroup(l10n.tr("HQ assessment", "HQ 最新汇报")) {
                        ScrollView {
                            MarkdownBody(markdown: assessment, p: Theme.Palette.of(scheme)).padding(.top, 6)
                        }.frame(maxHeight: 180)
                    }.font(Theme.Font.task)
                }
            }.padding(16)
            Divider()
            HQWorkspaceConversationView(l10n: l10n, store: store)
        } else if case .matter = store.selection {
            ContentUnavailableView(
                l10n.tr("Matter unavailable", "事项暂不可读"), systemImage: "doc.text",
                description: Text(l10n.tr("Reload, or choose another matter.", "请重新读取，或选择其他事项。")))
        } else {
            HStack {
                Text(l10n.tr("Talk to HQ", "与 HQ 对话")).font(Theme.Font.title)
                Spacer()
                if let supervisor = store.snapshot?.supervisor {
                    Text(hqConversationStatus(supervisor.status, l10n)).font(Theme.Font.task).foregroundStyle(
                        .secondary)
                } else {
                    Button(l10n.tr("Start HQ in Terminal", "在终端启动 HQ")) { GtmuxCLI.spawn(["hq"]) }
                }
            }.padding(16)
            Divider()
            HQWorkspaceConversationView(l10n: l10n, store: store)
        }
    }

    private var newTask: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(l10n.tr("Give HQ a task", "交代新任务")).font(Theme.Font.title)
            Text(
                l10n.tr(
                    "Describe the goal and any limits. HQ will organize the work and keep you informed.",
                    "说明要解决的问题和执行范围，HQ 会组织跟进并向你汇报。")
            )
            .font(.system(size: 14)).foregroundStyle(.secondary)
            if store.snapshot?.supervisor == nil {
                Label(
                    l10n.tr(
                        "HQ is not running. Your instruction will be saved pending delivery.",
                        "HQ 尚未运行，指令会保存为待送达。"), systemImage: "info.circle"
                )
                .font(Theme.Font.task)
                Button(l10n.tr("Start HQ in Terminal", "在终端启动 HQ")) { GtmuxCLI.spawn(["hq"]) }
            }
            Spacer()
        }.padding(22).frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
    }

    @ToolbarContentBuilder private var toolbar: some ToolbarContent {
        ToolbarItemGroup(placement: .primaryAction) {
            ForEach([HQReaderTab.board, .knowledge, .usage], id: \.self) { tab in
                Button {
                    HQReaderController.shared.show(l10n: l10n, tab: tab)
                } label: {
                    Image(systemName: tab.symbol)
                }.help(
                    tab == .knowledge
                        ? l10n.tr("Knowledge", "知识库")
                        : tab == .board ? l10n.tr("Situation board", "态势板") : l10n.tr("Usage", "用量"))
            }
            Button {
                store.refresh()
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .help(l10n.tr("Refresh", "重新读取")).disabled(store.loading || store.acting)
            Button {
                GtmuxCLI.spawn(["hq"])
            } label: {
                Image(systemName: "terminal")
            }
            .help(l10n.tr("Open HQ in Terminal", "在终端打开 HQ"))
        }
    }
}

struct HQWorkspaceSidebar: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQWorkspaceStore
    var body: some View {
        VStack(spacing: 0) {
            Button {
                store.select(.newTask)
            } label: {
                Label(l10n.tr("New task", "交代任务"), systemImage: "plus").frame(maxWidth: .infinity)
            }.controlSize(.large).padding(12).keyboardShortcut("n", modifiers: [.command])
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 16) {
                    group(l10n.tr("Needs your input", "待你处理"), states: ["needs-input"])
                    group(l10n.tr("Following", "正在跟进"), states: ["open"])
                    group(l10n.tr("Recent", "最近结束"), states: ["completed", "closed"])
                    Divider()
                    navigationRow(
                        l10n.tr("Talk to HQ", "与 HQ 对话"), symbol: "bubble.left", selection: .conversation)
                    navigationRow(l10n.tr("HQ activity", "HQ 运行记录"), symbol: "clock", selection: .activity)
                }.padding(10)
            }.background(Color(nsColor: .controlBackgroundColor))
            if store.snapshot?.matters.isEmpty == true {
                Text(l10n.tr("Create a matter to group related conversations.", "交代任务后，可将相关会话归入同一事项。"))
                    .font(Theme.Font.task).foregroundStyle(.secondary).padding(12)
            }
            if store.loading {
                ProgressView().controlSize(.small).padding(8).accessibilityLabel(
                    l10n.tr("Reading HQ", "正在读取 HQ"))
            }
        }
    }
    @ViewBuilder private func group(_ title: String, states: [String]) -> some View {
        let matters = (store.snapshot?.matters ?? []).filter { states.contains($0.state) }
        if !matters.isEmpty {
            VStack(alignment: .leading, spacing: 6) {
                Text(title).font(Theme.Font.window).foregroundStyle(.secondary).padding(.horizontal, 8)
                ForEach(matters) { matter in
                    Button {
                        store.select(.matter(matter.id))
                    } label: {
                        HStack(spacing: 8) {
                            Image(
                                systemName: matter.state == "needs-input"
                                    ? "exclamationmark.circle"
                                    : matter.state == "open" ? "circle.dotted" : "checkmark.circle"
                            )
                            .foregroundStyle(
                                matter.state == "needs-input" ? Theme.Status.waiting : Color.secondary)
                            Text(matter.title).font(Theme.Font.session).lineLimit(2)
                            Spacer(minLength: 0)
                        }.padding(9).frame(maxWidth: .infinity, alignment: .leading)
                            .background(
                                store.selection == .matter(matter.id)
                                    ? Color.primary.opacity(0.08) : Color.clear,
                                in: RoundedRectangle(cornerRadius: 7))
                    }.buttonStyle(.plain).accessibilityAddTraits(
                        store.selection == .matter(matter.id) ? .isSelected : []
                    ).accessibilityRemoveTraits(store.selection == .matter(matter.id) ? [] : .isSelected)
                }
            }

        }
    }
    private func navigationRow(_ title: String, symbol: String, selection: HQWorkspaceSelection) -> some View
    {
        Button {
            store.select(selection)
        } label: {
            Label(title, systemImage: symbol).font(Theme.Font.session).frame(
                maxWidth: .infinity, alignment: .leading
            ).padding(9)
                .background(
                    store.selection == selection ? Color.primary.opacity(0.08) : Color.clear,
                    in: RoundedRectangle(cornerRadius: 7))
        }.buttonStyle(.plain).accessibilityAddTraits(store.selection == selection ? .isSelected : [])
            .accessibilityRemoveTraits(store.selection == selection ? [] : .isSelected)
    }

}

struct HQWorkspaceComposer: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQWorkspaceStore
    var body: some View {
        VStack(alignment: .leading, spacing: 7) {
            Text(scopeLabel).font(Theme.Font.window).foregroundStyle(.secondary)
            HStack(alignment: .bottom, spacing: 10) {
                ZStack(alignment: .topLeading) {
                    if store.draft.isEmpty {
                        Text(l10n.tr("Describe your request…", "输入要求或问题…"))
                            .font(Theme.Font.session.weight(.regular)).foregroundStyle(.tertiary)
                            .padding(.horizontal, 12).padding(.top, 7).allowsHitTesting(false)
                    }
                    MultilineMessageField(
                        text: $store.draft, label: scopeLabel,
                        help: l10n.tr("Return for a new line; ⌘Return to send", "回车换行，⌘回车发送"),
                        canSend: { store.canSend }, onSend: { store.send() },
                        focusKey: store.selection.scope ?? "new-task")
                }.frame(height: 76).background(
                    Color(nsColor: .textBackgroundColor), in: RoundedRectangle(cornerRadius: 8)
                )
                .overlay(RoundedRectangle(cornerRadius: 8).strokeBorder(Color.primary.opacity(0.12)))
                Button(l10n.tr("Send to HQ", "发送给 HQ")) { store.send() }
                    .keyboardShortcut(.return, modifiers: [.command]).buttonStyle(.borderedProminent)
                    .disabled(!store.canSend)
            }
            if store.draft.utf8.count > 32768 {
                Text(
                    l10n.tr(
                        "This message is too long. Split it into shorter instructions.", "内容较长，请分成几条指令发送。")
                ).font(Theme.Font.window).foregroundStyle(Theme.Status.errored)
            }
            if store.currentMatter?.state == "closed" {
                Text(
                    l10n.tr("Continue following this matter to send another instruction.", "选择“继续跟进”后可发送新指令。")
                ).font(Theme.Font.window).foregroundStyle(.secondary)
            }
            if store.acting {
                Text(l10n.tr("Saving instruction…", "正在保存指令…")).font(Theme.Font.window).foregroundStyle(
                    .secondary)
            }
        }.padding(12)

    }
    private var scopeLabel: String {
        if let matter = store.currentMatter {
            return l10n.tr("Current matter: \(matter.title)", "当前事项：\(matter.title)")
        }
        if case .matter = store.selection {
            return l10n.tr("Matter unavailable · reload to continue", "事项暂不可用 · 请重新读取")
        }
        return store.selection == .newTask
            ? l10n.tr("New matter · Enter for a newline, ⌘Return to send", "新事项 · 回车换行，⌘回车发送")
            : l10n.tr("Global HQ conversation", "与 HQ 对话 · 不关联具体事项")
    }
}
