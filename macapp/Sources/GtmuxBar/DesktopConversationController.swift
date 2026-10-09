import AppKit
import SwiftUI

struct DesktopChatStep: Decodable, Equatable {
    let title: String
    let detail: String?
}
struct DesktopChatSegment: Decodable, Equatable {
    let text: String?
    let steps: [DesktopChatStep]?
}
struct DesktopChatTurn: Decodable, Equatable {
    let prompt: String
    let response: String
    let segments: [DesktopChatSegment]?
    let time: String?
}
struct DesktopChatSnapshot: Decodable, Equatable {
    let turns: [DesktopChatTurn]?
    let dropped: Int
    let etag: String?
    let unchanged: Bool?
}

final class DesktopConversationModel: ObservableObject {
    let agent: Agent
    @Published var turns: [DesktopChatTurn] = []
    @Published var dropped = 0
    @Published var loading = true
    @Published var failed = false
    private(set) var etag: String?
    private var stopped = true
    private var busy = false
    private var generation = 0
    private var timer: DispatchWorkItem?
    private let capture: ([String]) -> (status: Int32, stdout: String, stderr: String)
    private let active: () -> Bool
    init(agent: Agent, active: @escaping () -> Bool = { true },
         capture: @escaping ([String]) -> (status: Int32, stdout: String, stderr: String) = { GtmuxCLI.captureFull($0) }) {
        self.agent = agent; self.active = active; self.capture = capture
    }
    static func arguments(id: String, etag: String?) -> [String] {
        ["transcript", id, "--json"] + (etag.map { ["--etag", $0] } ?? [])
    }
    func start() { guard stopped else { return }; stopped = false; generation += 1; refresh() }
    func stop() { stopped = true; generation += 1; timer?.cancel(); timer = nil }
    func refresh() {
        guard !stopped && !busy else { return }
        timer?.cancel()
        guard active() else { schedule(); return }
        busy = true
        let token = generation
        let args = Self.arguments(id: agent.sessionID, etag: etag)
        let capture = capture
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            let result = capture(args)
            DispatchQueue.main.async { [weak self] in
                guard let self else { return }
                self.busy = false
                guard !self.stopped else { return }
                guard self.generation == token else { self.refresh(); return }
                self.loading = false
                if result.status == 0, let data = result.stdout.data(using: .utf8),
                   let snapshot = try? JSONDecoder().decode(DesktopChatSnapshot.self, from: data),
                   snapshot.unchanged == true || snapshot.turns != nil {
                    self.failed = false
                    if snapshot.unchanged != true {
                        self.turns = snapshot.turns ?? []; self.dropped = snapshot.dropped; self.etag = snapshot.etag
                    }
                } else { self.failed = true }
                self.schedule()
            }
        }
    }
    private func schedule() {
        guard !stopped else { return }
        let item = DispatchWorkItem { [weak self] in self?.refresh() }
        timer = item
        DispatchQueue.main.asyncAfter(deadline: .now() + 2, execute: item)
    }
}

final class DesktopConversationController: NSObject, NSWindowDelegate {
    static let shared = DesktopConversationController()
    private var window: NSWindow?
    private var model: DesktopConversationModel?
    func show(agent: Agent, l10n: L10n) {
        guard agent.isDesktop && !agent.sessionID.isEmpty else { return }
        model?.stop(); window?.close()
        let w = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 760, height: 640),
                         styleMask: [.titled, .closable, .resizable, .miniaturizable], backing: .buffered, defer: false)
        w.isReleasedWhenClosed = false
        w.minSize = NSSize(width: 520, height: 360)
        w.title = agent.primary.isEmpty ? l10n.tr("Desktop conversation", "桌面会话") : agent.primary
        let model = DesktopConversationModel(agent: agent, active: { [weak w] in w?.isVisible == true && w?.isMiniaturized != true && NSApp.isActive })
        self.model = model
        w.contentView = NSHostingView(rootView: DesktopConversationView(model: model, l10n: l10n))
        w.delegate = self; window = w; w.center()
        NSApp.activate(ignoringOtherApps: true); w.makeKeyAndOrderFront(nil); model.start()
    }
    func windowWillClose(_ notification: Notification) { model?.stop(); model = nil; window = nil }
}

struct DesktopConversationView: View {
    @ObservedObject var model: DesktopConversationModel
    @ObservedObject var l10n: L10n
    @Environment(\.colorScheme) private var scheme
    var body: some View {
        let p = Theme.Palette.of(scheme)
        VStack(spacing: 0) {
            HStack {
                Text(l10n.tr("Read only · Continue in ChatGPT desktop", "只读 · 请在 ChatGPT 桌面版继续对话"))
                    .font(.caption).foregroundStyle(p.fg2)
                Spacer()
                if model.loading { ProgressView().controlSize(.small) }
            }.padding(16)
            Divider()
            if model.failed {
                HStack {
                    Text(l10n.tr("Could not update the conversation. Check gtmux on this Mac.", "暂时无法更新对话，请检查本机 gtmux。"))
                        .font(.callout).frame(maxWidth: .infinity, alignment: .leading)
                    Button(l10n.tr("Retry", "重试")) { model.refresh() }
                }.padding(16)
            }
            ScrollViewReader { proxy in
                VStack(spacing: 0) {
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 18) {
                            if model.dropped > 0 { Text(l10n.tr("Earlier messages omitted", "已省略部分较早对话")).font(.caption).foregroundStyle(p.fg2) }
                            if !model.loading && !model.failed && model.turns.isEmpty {
                                Text(l10n.tr("No messages have been recorded yet.", "尚无已记录的对话。"))
                                    .foregroundStyle(p.fg2).padding(.top, 30)
                            }
                            ForEach(Array(model.turns.enumerated()), id: \.offset) { _, turn in
                                if !turn.prompt.isEmpty {
                                    VStack(alignment: .leading, spacing: 6) {
                                        Text(l10n.tr("You", "你")).font(.caption).foregroundStyle(p.fg2)
                                        Text(turn.prompt).textSelection(.enabled)
                                    }.padding(12).frame(maxWidth: .infinity, alignment: .leading)
                                        .background(p.rowSelected).clipShape(RoundedRectangle(cornerRadius: 8))
                                }
                                let segments = turn.segments ?? [DesktopChatSegment(text: turn.response, steps: nil)]
                                ForEach(Array(segments.enumerated()), id: \.offset) { _, segment in
                                    VStack(alignment: .leading, spacing: 8) {
                                        if let text = segment.text, !text.isEmpty {
                                            Text("Codex").font(.caption).foregroundStyle(p.fg2)
                                            Text(.init(text)).textSelection(.enabled).fixedSize(horizontal: false, vertical: true)
                                        }
                                        if let steps = segment.steps, !steps.isEmpty {
                                            DisclosureGroup(l10n.tr("\(steps.count) tool steps", "\(steps.count) 个工具步骤")) {
                                                ForEach(Array(steps.enumerated()), id: \.offset) { _, step in
                                                    VStack(alignment: .leading) {
                                                        Text(step.title).font(.callout).fontWeight(.medium)
                                                        if let detail = step.detail { Text(detail).font(.caption).foregroundStyle(p.fg2).textSelection(.enabled) }
                                                    }.frame(maxWidth: .infinity, alignment: .leading).padding(.vertical, 3)
                                                }
                                            }.font(.caption).foregroundStyle(p.fg2)
                                        }
                                    }.frame(maxWidth: .infinity, alignment: .leading)
                                }
                            }
                            Color.clear.frame(height: 1).id("latest")
                        }.padding(20).frame(maxWidth: 720).frame(maxWidth: .infinity)
                    }
                    HStack {
                        Spacer()
                        Button(l10n.tr("Latest messages ↓", "最新消息 ↓")) { proxy.scrollTo("latest", anchor: .bottom) }
                            .buttonStyle(.plain).font(.caption).foregroundStyle(p.fg2)
                    }.padding(.horizontal, 20).padding(.vertical, 10)
                }
            }
        }.frame(maxWidth: .infinity, maxHeight: .infinity).background(p.bg)
    }
}
