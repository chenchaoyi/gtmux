import Combine
import Foundation

/// Window-owned interaction state. The core owns records; this store owns only
/// selection, drafts, disclosure/history windows and in-flight UI feedback.
@MainActor
final class HQWorkspaceStore: ObservableObject {
    @Published private(set) var snapshot: HQWorkspaceSnapshot?
    @Published private(set) var detail: HQWorkspaceDetail?
    @Published private(set) var selection: HQWorkspaceSelection = .conversation
    @Published var draft = ""
    @Published private(set) var error: String?
    @Published private(set) var loading = false
    @Published private(set) var acting = false
    @Published private(set) var delivering = false
    @Published private(set) var refreshSerial = 0
    private let client: HQWorkspaceServing
    private let queue = DispatchQueue(label: "gtmux.hq-workspace", qos: .userInitiated)
    private let actionQueue = DispatchQueue(label: "gtmux.hq-workspace.actions", qos: .userInitiated)
    private let deliveryQueue = DispatchQueue(label: "gtmux.hq-workspace.delivery", qos: .userInitiated)
    private var timer: Timer?
    private var visible = false
    private var generation = 0
    private var drafts: [HQWorkspaceSelection: String] = [:]
    private var oldMessages: [HQWorkspaceMessage] = []
    private var earlier = 0
    private var historyHasEarlier = false
    private var historyBefore: UInt64?
    private var retrySubmission: (selection: HQWorkspaceSelection, body: String, action: HQWorkspaceAction)?
    var currentMatter: HQMatter? {
        if case .matter(let id) = selection {
            if let matter = detail?.matter, matter.id == id { return matter }
            return snapshot?.matters.first { $0.id == id }
        }
        return nil
    }
    var canSend: Bool {
        if case .matter = selection, currentMatter == nil || currentMatter?.state == "closed" { return false }
        return !acting && selection != .activity && draft.utf8.count <= 32768
            && !draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }

    init(client: HQWorkspaceServing = HQWorkspaceClient()) { self.client = client }
    deinit { timer?.invalidate() }

    func start() {
        guard !visible else { return }
        visible = true
        refresh()
        timer = Timer.scheduledTimer(withTimeInterval: 8, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.refresh(quiet: true) }
        }
    }
    func stop() {
        visible = false
        timer?.invalidate()
        timer = nil
        drafts[selection] = draft
    }

    func select(_ target: HQWorkspaceSelection) {
        guard selection != target else { return }
        drafts[selection] = draft
        selection = target
        draft = drafts[target] ?? ""
        oldMessages = []
        earlier = 0
        historyHasEarlier = false
        historyBefore = nil
        detail = nil
        generation += 1
        error = nil
        refresh()
    }

    func refresh(quiet: Bool = false) {
        guard !loading, !acting else { return }
        loading = true
        let expected = selection
        let token = generation
        let hops = earlier
        let service = client
        let previousSnapshot = snapshot
        let previousDetail = detail
        queue.async { [weak self] in
            let result = Result { () -> (HQWorkspaceSnapshot, HQWorkspaceDetail?) in
                let snapshot = try service.snapshot()
                let activityUnchanged =
                    expected != .activity || previousSnapshot?.activityRevision == snapshot.activityRevision
                let unchanged =
                    quiet && activityUnchanged && previousSnapshot?.revision == snapshot.revision
                    && previousSnapshot?.sourceRevision == snapshot.sourceRevision && previousDetail != nil
                let detail =
                    unchanged
                    ? previousDetail
                    : try expected.scope.map { try service.detail($0, before: nil, earlier: hops) }
                return (snapshot, detail)
            }
            DispatchQueue.main.async {
                guard let self else { return }
                self.loading = false
                guard self.generation == token, self.selection == expected else {
                    if self.visible { self.refresh() }
                    return
                }
                switch result {
                case .success(let (snapshot, detail)):
                    self.snapshot = snapshot
                    self.detail = self.mergeHistory(detail)
                    self.error = nil
                    self.refreshSerial += 1
                case .failure(let failure): self.error = failure.localizedDescription
                }
            }
        }
    }

    private func mergeHistory(_ incoming: HQWorkspaceDetail?) -> HQWorkspaceDetail? {
        guard var incoming else { return nil }
        let newest = Set(incoming.messages.map(\.id))
        incoming.messages = oldMessages.filter { !newest.contains($0.id) } + incoming.messages
        if !oldMessages.isEmpty {
            incoming.earlierBefore = historyBefore
            incoming.messageEarlierAvailable = historyHasEarlier
            incoming.earlierAvailable = historyHasEarlier || incoming.transcriptEarlierAvailable == true
        }
        return incoming
    }

    func loadEarlier() {
        guard !loading, !acting, let scope = selection.scope, let detail else { return }
        let before = detail.messageEarlierAvailable == true ? detail.earlierBefore : nil
        guard before != nil || detail.transcriptEarlierAvailable == true else { return }
        if scope == "global", detail.transcriptEarlierAvailable == true { earlier = min(earlier + 1, 20) }
        let hops = earlier
        loading = true
        let expected = selection
        let token = generation
        let service = client
        queue.async { [weak self] in
            let result = Result { try service.detail(scope, before: before, earlier: hops) }
            DispatchQueue.main.async {
                guard let self else { return }
                self.loading = false
                guard self.selection == expected, self.generation == token else { return }
                switch result {
                case .success(let page):
                    let known = Set(self.oldMessages.map(\.id))
                    self.oldMessages = page.messages.filter { !known.contains($0.id) } + self.oldMessages
                    self.historyHasEarlier = page.messageEarlierAvailable == true
                    self.historyBefore = page.earlierBefore
                    var merged = detail
                    let IDs = Set(merged.messages.map(\.id))
                    merged.messages = page.messages.filter { !IDs.contains($0.id) } + merged.messages
                    merged.turns = page.turns
                    merged.transcriptEarlierAvailable = page.transcriptEarlierAvailable
                    merged.messageEarlierAvailable = page.messageEarlierAvailable
                    merged.earlierAvailable = page.earlierAvailable
                    merged.earlierBefore = page.earlierBefore
                    self.detail = merged
                case .failure(let failure): self.error = failure.localizedDescription
                }
            }
        }
    }

    func send() {
        guard canSend else { return }
        let text = draft
        let origin = selection
        let action: HQWorkspaceAction
        if let retry = retrySubmission, retry.selection == origin, retry.body == text {
            action = retry.action
        } else {
            let id = UUID().uuidString.lowercased()
            if origin == .newTask {
                let first =
                    text.split(separator: "\n", omittingEmptySubsequences: true).first.map(String.init)
                    ?? text
                action = HQWorkspaceAction(op: "create", id: id, title: String(first.prefix(60)), body: text)
            } else {
                action = HQWorkspaceAction(
                    op: "send", id: id, matter: origin.scope == "global" ? nil : origin.scope, body: text)
            }
            retrySubmission = (origin, text, action)
        }
        perform(action) { [weak self] in
            guard let self else { return }
            self.drafts[origin] = ""
            self.retrySubmission = nil
            if self.selection == origin {
                self.draft = ""
                if origin == .newTask, let id = action.id { self.select(.matter(id)) }
            }
            self.refresh()
            self.attemptPending()
        }
    }

    /// Runs only after a user submission or an explicit attempt; ordinary reads
    /// never drive the supervisor. The core handles background safe holds in serve.
    func attemptPending() {
        guard !delivering else { return }
        delivering = true
        let service = client
        deliveryQueue.async { [weak self] in
            let result = Result { try service.drain() }
            DispatchQueue.main.async {
                guard let self else { return }
                self.delivering = false
                if case .failure(let failure) = result { self.error = failure.localizedDescription }
                self.refresh()
            }
        }
    }

    func perform(_ action: HQWorkspaceAction, onSuccess: (() -> Void)? = nil) {
        guard !acting else { return }
        acting = true
        error = nil
        generation += 1
        let service = client
        actionQueue.async { [weak self] in
            let result = Result { try service.act(action) }
            DispatchQueue.main.async {
                guard let self else { return }
                self.acting = false
                switch result {
                case .success:
                    onSuccess?()
                    self.refresh()
                case .failure(let failure): self.error = failure.localizedDescription
                }
            }
        }
    }

    func readParticipant(
        _ association: HQParticipant, matter: String,
        completion: @escaping (Result<HQWorkspaceDetail, Error>) -> Void
    ) {
        let service = client
        queue.async {
            let result = Result { try service.participant(matter, association: association.id) }
            DispatchQueue.main.async { completion(result) }
        }
    }
}
