import AppKit
import SwiftUI

struct DesktopChatStep: Decodable, Equatable {
    let title: String
    let detail: String?
    var operation: String? = nil
    var input: String? = nil
    var inputKind: String? = nil
    var workdir: String? = nil
    var detailsTruncated: Bool? = nil
    enum CodingKeys: String, CodingKey {
        case title, detail, operation, input, workdir
        case inputKind = "input_kind", detailsTruncated = "details_truncated"
    }
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
    var agent: String? = nil
    var from: DesktopChatSender? = nil
    var sessionID: String? = nil
    enum CodingKeys: String, CodingKey {
        case prompt, response, segments, time, agent, from
        case sessionID = "session_id"
    }
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
        let args = Self.arguments(id: agent.sessionID, etag: etag) + (agent.homeID.isEmpty ? [] : ["--home-id", agent.homeID])
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
        guard agent.readableConversation && !agent.sessionID.isEmpty else { return }
        model?.stop(); window?.close()
        let w = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 840, height: 720),
                         styleMask: [.titled, .closable, .resizable, .miniaturizable], backing: .buffered, defer: false)
        w.isReleasedWhenClosed = false
        w.minSize = NSSize(width: 520, height: 420)
        w.title = agent.primary.isEmpty ? l10n.tr("conversation", "会话") : agent.primary
        let model = DesktopConversationModel(agent: agent, active: { [weak w] in w?.isVisible == true && w?.isMiniaturized != true && NSApp.isActive })
        self.model = model
        w.contentView = NSHostingView(rootView: DesktopConversationView(model: model, l10n: l10n))
        w.delegate = self; window = w; w.center()
        NSApp.activate(ignoringOtherApps: true); w.makeKeyAndOrderFront(nil); model.start()
    }
    func windowWillClose(_ notification: Notification) { model?.stop(); model = nil; window = nil }
}
