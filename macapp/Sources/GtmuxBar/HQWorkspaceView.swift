import SwiftUI

struct HQWorkspaceView: View {
    @ObservedObject var l10n: L10n
    @ObservedObject var store: HQWorkspaceStore
    @State private var manage = false
    @State private var rename = false
    @State private var titleDraft = ""

    var body: some View {
        NavigationSplitView {
            HQWorkspaceSidebar(l10n: l10n, store: store)
                .navigationSplitViewColumnWidth(min: 200, ideal: 235, max: 300)
        } detail: {
            VStack(spacing: 0) {
                if let error = store.error, store.selection.resourcePage == nil {
                    HStack(alignment: .top) {
                        Image(systemName: "exclamationmark.triangle").foregroundStyle(Theme.Status.errored)
                        Text(error).font(Theme.Font.task).textSelection(.enabled)
                        Spacer()
                        Button(l10n.tr("Reload", "重新读取")) { store.refresh() }
                    }.padding(12).background(Theme.Status.errored.opacity(0.06))
                }
                if let page = store.selection.resourcePage {
                    HQResourcesView(l10n: l10n, store: store.resources, page: page) { store.select(.overview) }
                } else if store.readPhase == .loading {
                    HQBrandLoading(label: l10n.tr("Loading HQ…", "正在载入 HQ…"))
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                } else if store.readPhase == .failed {
                    ContentUnavailableView(
                        l10n.tr("Unable to load HQ", "无法载入 HQ"), systemImage: "exclamationmark.triangle",
                        description: Text(l10n.tr("Use Reload to try again.", "请重新读取。")))
                } else if store.selection == .overview {
                    HQWorkspaceOverview(l10n: l10n, store: store)
                } else if store.selection == .activity {
                    HQWorkspaceActivityView(l10n: l10n, detail: store.detail)
                } else if store.selection == .newTask {
                    newTask
                } else {
                    detailBody
                }
                if store.selection.showsComposer {
                    Divider()
                    HQWorkspaceComposer(l10n: l10n, store: store)
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
        .environment(\.locale, Locale(identifier: l10n.lang == "zh" ? "zh_CN" : "en_US"))
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
                                HQMatterGroup.of(matter.state).status.color)
                        if let origin = matter.origin {
                            DisclosureGroup(origin.label(l10n)) {
                                Text(origin.ref).font(Theme.Font.task).textSelection(.enabled)
                            }.font(Theme.Font.task).foregroundStyle(.secondary)
                        }
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
                } else if store.snapshot != nil {
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
            if store.snapshot != nil, store.snapshot?.supervisor == nil {
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
            if store.loading, store.readPhase == .content, store.selection.resourcePage == nil {
                HQBrandLoading(label: l10n.tr("Refreshing HQ", "正在刷新 HQ"), compact: true)
            }
            Button {
                store.refreshCurrent()
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .help(l10n.tr("Refresh", "重新读取")).disabled(store.acting)
            Button {
                GtmuxCLI.spawn(["hq"])
            } label: {
                Image(systemName: "terminal")
            }
            .help(l10n.tr("Open HQ in Terminal", "在终端打开 HQ"))
        }
    }
}
