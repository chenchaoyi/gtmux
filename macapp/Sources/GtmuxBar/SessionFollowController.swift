import AppKit
import SwiftUI

/// A small per-conversation utility window. The CLI owns the policy and revision;
/// the app keeps only a draft, never changes its badge before a successful receipt.
final class SessionFollowController: NSObject, NSWindowDelegate {
    static let shared = SessionFollowController()
    private var window: NSWindow?
    private var model: SessionFollowModel?

    func show(agent: Agent, l10n: L10n, onSaved: @escaping () -> Void) {
        guard model?.saving != true else { window?.makeKeyAndOrderFront(nil); return }
        window?.close()
        let model = SessionFollowModel(agent: agent, l10n: l10n, onSaved: onSaved)
        self.model = model
        let w = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 420, height: 230),
                         styleMask: [.titled, .closable], backing: .buffered, defer: false)
        w.isReleasedWhenClosed = false
        w.title = l10n.tr("Follow settings", "跟进设置")
        w.contentView = NSHostingView(rootView: SessionFollowView(model: model, l10n: l10n,
            onClose: { [weak w] in w?.close() }, onResize: { [weak w] height in
                w?.setContentSize(NSSize(width: 420, height: height))
            }))
        w.delegate = self
        window = w
        w.center()
        NSApp.activate(ignoringOtherApps: true)
        w.makeKeyAndOrderFront(nil)
        model.load()
    }

    func windowShouldClose(_ sender: NSWindow) -> Bool { model?.saving != true }
    func windowWillClose(_ notification: Notification) { window = nil; model = nil }
}

struct SessionFollowSettings: Codable, Equatable {
    var hq = false
    var notify = false
    var knowledge = false
    var revision = 0
    var followSince: Int?
    var knowledgeSince: Int?
    enum CodingKeys: String, CodingKey {
        case hq, notify, knowledge, revision
        case followSince = "follow_since", knowledgeSince = "knowledge_since"
    }
    static func arguments(id: String, settings: Self) -> [String] {
        ["follow", id, "--hq", settings.hq ? "on" : "off", "--notify", settings.notify ? "on" : "off",
         "--knowledge", settings.knowledge ? "on" : "off", "--revision", String(settings.revision), "--json"]
    }
}

final class SessionFollowModel: ObservableObject {
    let agent: Agent
    let l10n: L10n
    let onSaved: () -> Void
    private let capture: ([String]) -> (status: Int32, stdout: String, stderr: String)
    @Published var current = SessionFollowSettings()
    @Published var draft = SessionFollowSettings()
    @Published var loading = true
    @Published var loaded = false
    @Published var saving = false
    @Published var error = ""
    @Published var saved = false
    var dirty: Bool { current.hq != draft.hq || current.notify != draft.notify || current.knowledge != draft.knowledge }
    init(agent: Agent, l10n: L10n, onSaved: @escaping () -> Void,
         capture: @escaping ([String]) -> (status: Int32, stdout: String, stderr: String) = { GtmuxCLI.captureFull($0) }) {
        self.agent = agent; self.l10n = l10n; self.onSaved = onSaved; self.capture = capture
    }
    func load() { guard !saving else { return }; loading = true; run(["follow", agent.sessionID, "--json"], saving: false) }
    func save() { guard loaded && !loading && !saving && dirty else { return }; saving = true; run(SessionFollowSettings.arguments(id: agent.sessionID, settings: draft), saving: true) }
    func choose(_ hq: Bool) {
        draft.hq = hq
        if !hq { draft.notify = false; draft.knowledge = false }
        saved = false
    }
    private func run(_ args: [String], saving: Bool) {
        error = ""; saved = false
        let capture = self.capture
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            let result = capture(args)
            DispatchQueue.main.async { [weak self] in
                guard let self else { return }
                self.loading = false; self.saving = false
                if result.status == 0, let data = result.stdout.data(using: .utf8),
                   let value = try? JSONDecoder().decode(SessionFollowSettings.self, from: data) {
                    self.current = value; self.draft = value; self.loaded = true; self.saved = saving
                    if saving { self.onSaved() }
                } else {
                    self.error = self.l10n.tr("Could not load or save settings. Reload before trying again.", "无法读取或保存设置，请重新加载后重试。")
                }
            }
        }
    }
}

struct SessionFollowView: View {
    @ObservedObject var model: SessionFollowModel
    @ObservedObject var l10n: L10n
    var onClose: () -> Void = {}
    var onResize: (CGFloat) -> Void = { _ in }
    @Environment(\.colorScheme) private var scheme
    var contentHeight: CGFloat { model.loaded && model.draft.hq ? 380 : 230 }

    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 12) {
                AgentAvatar(agent: model.agent)
                VStack(alignment: .leading, spacing: 4) {
                    Text(model.agent.primary.isEmpty ? model.agent.agent : model.agent.primary)
                        .font(.headline).lineLimit(2)
                    Text(l10n.tr("ChatGPT desktop · This Mac", "ChatGPT 桌面版 · 当前 Mac"))
                        .font(.caption).foregroundStyle(p.fg2)
                }
            }
            Divider().padding(.vertical, 16)
            ScrollView {
                VStack(alignment: .leading, spacing: 0) {
                    if model.loading {
                        ProgressView(l10n.tr("Loading settings…", "正在加载设置…"))
                            .frame(maxWidth: .infinity, minHeight: 100)
                    } else if model.loaded {
                        settingRow(l10n.tr("HQ follow", "HQ 跟进"),
                                   detail: l10n.tr("Read the conversation and report progress.", "读取对话，分析并汇报进展。"),
                                   value: Binding(get: { model.draft.hq }, set: { model.choose($0) }),
                                   identifier: "follow-hq")
                        if model.draft.hq {
                            Text(l10n.tr("Notifications and knowledge", "通知与知识"))
                                .font(.caption).foregroundStyle(p.fg2).padding(.top, 20).padding(.bottom, 6)
                            VStack(spacing: 0) {
                                settingRow(l10n.tr("Conversation notifications", "会话通知"),
                                           detail: l10n.tr("Notify when input is needed or work finishes.", "需要处理或会话完成时通知你。"),
                                           value: $model.draft.notify, identifier: "follow-notify")
                                Divider().padding(.vertical, 12)
                                settingRow(l10n.tr("Save to knowledge base", "知识留存"),
                                           detail: l10n.tr("Keep reusable lessons from future activity.", "从开启后的活动中整理可复用经验。"),
                                           value: $model.draft.knowledge, identifier: "follow-knowledge")
                            }.padding(14).background(p.rowSelected).clipShape(RoundedRectangle(cornerRadius: 8))
                            Text(l10n.tr("Continue conversations in ChatGPT desktop.", "请在 ChatGPT 桌面版继续对话。"))
                                .font(.caption).foregroundStyle(p.fg2).padding(.top, 12)
                        } else if model.current.hq {
                            Text(l10n.tr("Existing records and knowledge will be kept.", "停止后保留已有记录和知识。"))
                                .font(.caption).foregroundStyle(p.fg2).padding(.top, 12)
                        }
                    }
                    if !model.error.isEmpty {
                        Text(model.error).font(.subheadline).padding(.top, 12)
                        Button(l10n.tr("Reload settings", "重新加载")) { model.load() }.padding(.top, 8)
                    }
                }.frame(maxWidth: .infinity, alignment: .leading)
            }
            Divider().padding(.vertical, 12)
            HStack(spacing: 12) {
                Text(model.dirty ? l10n.tr("Unsaved changes", "未保存") : model.saved ? l10n.tr("Saved", "已保存") : "")
                    .font(.caption).foregroundStyle(p.fg2)
                Spacer()
                Button(l10n.tr("Cancel", "取消"), action: onClose).keyboardShortcut(.cancelAction)
                Button(model.saving ? l10n.tr("Saving…", "保存中…") : !model.current.hq && model.draft.hq ? l10n.tr("Enable follow", "开启跟进") : model.current.hq && !model.draft.hq ? l10n.tr("Stop following", "停止跟进") : l10n.tr("Save", "保存")) { model.save() }
                    .buttonStyle(.borderedProminent).disabled(!model.loaded || model.loading || model.saving || !model.dirty)
                    .accessibilityIdentifier("follow-save").keyboardShortcut(.defaultAction)
            }
        }.padding(20).frame(width: 420, height: contentHeight).foregroundStyle(p.fg)
            .onChange(of: contentHeight) { _, height in onResize(height) }
            .disabled(model.saving)
    }

    private func settingRow(_ title: String, detail: String, value: Binding<Bool>, identifier: String) -> some View {
        let p = Theme.Palette.of(scheme)
        return HStack(alignment: .center, spacing: 16) {
            VStack(alignment: .leading, spacing: 4) {
                Text(title).font(.subheadline).fontWeight(.medium)
                Text(detail).font(.caption).foregroundStyle(p.fg2).fixedSize(horizontal: false, vertical: true)
            }.frame(maxWidth: .infinity, alignment: .leading)
            Toggle(title, isOn: value).labelsHidden().toggleStyle(.switch)
                .accessibilityLabel(title).accessibilityHint(detail).help(detail).accessibilityIdentifier(identifier)
        }
    }
}
