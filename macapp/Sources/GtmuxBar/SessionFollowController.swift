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
        let w = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 420, height: 520),
                         styleMask: [.titled, .closable], backing: .buffered, defer: false)
        w.isReleasedWhenClosed = false
        w.title = l10n.tr("Conversation settings", "会话设置")
        w.contentView = NSHostingView(rootView: SessionFollowView(model: model, l10n: l10n))
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
    @Published var current = SessionFollowSettings()
    @Published var draft = SessionFollowSettings()
    @Published var loading = true
    @Published var saving = false
    @Published var error = ""
    @Published var saved = false
    var dirty: Bool { current.hq != draft.hq || current.notify != draft.notify || current.knowledge != draft.knowledge }
    init(agent: Agent, l10n: L10n, onSaved: @escaping () -> Void) {
        self.agent = agent; self.l10n = l10n; self.onSaved = onSaved
    }
    func load() { guard !saving else { return }; loading = true; run(["follow", agent.sessionID, "--json"], saving: false) }
    func save() { guard !loading && !saving && dirty else { return }; saving = true; run(SessionFollowSettings.arguments(id: agent.sessionID, settings: draft), saving: true) }
    func choose(_ hq: Bool) {
        draft.hq = hq
        if !hq { draft.notify = false; draft.knowledge = false }
        saved = false
    }
    private func run(_ args: [String], saving: Bool) {
        error = ""; saved = false
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            let result = GtmuxCLI.captureFull(args)
            DispatchQueue.main.async { [weak self] in
                guard let self else { return }
                self.loading = false; self.saving = false
                if result.status == 0, let data = result.stdout.data(using: .utf8),
                   let value = try? JSONDecoder().decode(SessionFollowSettings.self, from: data) {
                    self.current = value; self.draft = value; self.saved = saving
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
    @Environment(\.colorScheme) private var scheme
    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(alignment: .leading, spacing: 0) {
            ScrollView {
            VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 12) {
                AgentAvatar(agent: model.agent)
                VStack(alignment: .leading, spacing: 4) {
                    Text(model.agent.primary.isEmpty ? model.agent.agent : model.agent.primary).font(.headline).lineLimit(2)
                    Text(l10n.tr("Codex · ChatGPT desktop", "Codex · ChatGPT 桌面版")).font(.subheadline).foregroundStyle(p.fg2)
                }
            }.padding(.bottom, 12)
            Text(model.current.hq ? l10n.tr("HQ following", "HQ 跟进中") : l10n.tr("Status only", "仅显示状态"))
                .font(.caption).foregroundStyle(p.fg2).padding(.horizontal, 8).padding(.vertical, 4)
                .background(p.rowSelected).clipShape(RoundedRectangle(cornerRadius: 5))
            Divider().padding(.vertical, 16)
            if model.loading { ProgressView().frame(maxWidth: .infinity, minHeight: 160) }
            else {
                Text(l10n.tr("Follow mode", "跟进方式")).font(.subheadline).foregroundStyle(p.fg2).padding(.bottom, 10)
                Picker("", selection: Binding(get: { model.draft.hq }, set: { model.choose($0) })) {
                    Text(l10n.tr("Status only", "仅显示状态")).tag(false)
                    Text(l10n.tr("HQ follow", "HQ 跟进")).tag(true)
                }.pickerStyle(.radioGroup).labelsHidden().accessibilityIdentifier("follow-mode")
                Text(model.draft.hq ? l10n.tr("Read the conversation, analyze and report progress.", "读取对话，分析并汇报进展。") : l10n.tr("Show this conversation in the list. No HQ attention or notifications.", "只在列表中显示状态，不交给 HQ，也不发送通知。"))
                    .font(.subheadline).foregroundStyle(p.fg2).padding(.top, 8)
                if model.draft.hq {
                    Text(l10n.tr("HQ does not control ChatGPT for you.", "HQ 不会替你操作 ChatGPT。"))
                        .font(.subheadline).foregroundStyle(p.fg2).padding(.vertical, 12)
                    Divider().padding(.bottom, 14)
                    Toggle(l10n.tr("Notify me about this conversation", "接收此会话通知"), isOn: $model.draft.notify)
                        .accessibilityIdentifier("follow-notify")
                    Text(l10n.tr("Continue in ChatGPT desktop when action is needed.", "需要处理时，在 ChatGPT 桌面版继续。"))
                        .font(.caption).foregroundStyle(p.fg2).padding(.bottom, 12)
                    Toggle(l10n.tr("Allow knowledge capture", "允许沉淀到知识库"), isOn: $model.draft.knowledge)
                        .accessibilityIdentifier("follow-knowledge")
                    Text(l10n.tr("Collect reusable experience from new activity.", "仅采集后续可复用的经验。"))
                        .font(.caption).foregroundStyle(p.fg2)
                }
                if !model.error.isEmpty {
                    Text(model.error).font(.subheadline).padding(.top, 12)
                    Button(l10n.tr("Reload settings", "重新加载设置")) { model.load() }.padding(.top, 6)
                }
                if model.saved && !model.dirty { Text(l10n.tr("Settings saved", "设置已保存")).font(.caption).foregroundStyle(p.fg2).padding(.top, 10) }
            }
            }
            }.frame(maxWidth: .infinity, maxHeight: .infinity)
            Divider().padding(.vertical, 16)
            HStack {
                Text(l10n.tr("This conversation on this Mac", "仅限当前 Mac 的这段会话")).font(.caption).foregroundStyle(p.fg2)
                Spacer()
                Button(model.saving ? l10n.tr("Saving…", "保存中…") : !model.current.hq && model.draft.hq ? l10n.tr("Enable follow", "开启跟进") : model.current.hq && !model.draft.hq ? l10n.tr("Stop following", "停止跟进") : l10n.tr("Save", "保存")) { model.save() }
                    .buttonStyle(.borderedProminent).disabled(model.loading || model.saving || !model.dirty)
                    .accessibilityIdentifier("follow-save").keyboardShortcut(.defaultAction)
            }
        }.padding(22).frame(width: 420, height: 520).foregroundStyle(p.fg)
            .disabled(model.saving)
    }
}
