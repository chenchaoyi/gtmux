import Combine
import Foundation

/// Window-owned interaction state. The core owns records; this store owns only
/// selection, drafts, disclosure/history windows and in-flight UI feedback.
@MainActor
final class HQWorkspaceStore: ObservableObject {
    @Published private(set) var snapshot: HQWorkspaceSnapshot?
    @Published private(set) var detail: HQWorkspaceDetail?
    @Published private(set) var selection: HQWorkspaceSelection = .overview
    @Published var draft = ""
    @Published private(set) var error: String?
    @Published private(set) var loading = false
    @Published private(set) var acting = false
    @Published private(set) var delivering = false
    @Published private(set) var refreshSerial = 0
    @Published private(set) var conversationRevealSerial = 0
    @Published private(set) var composerFocusSerial = 0
    @Published private(set) var sourcesKnown = false
    @Published private(set) var sourcesLoading = false
    @Published private(set) var sourcesError: String?
    @Published private(set) var collapsedSections: Set<HQSidebarSection> = []
    let resources: HQResourceStore
    private let client: HQWorkspaceServing
    private let queue = DispatchQueue(label: "gtmux.hq-workspace", qos: .userInitiated)
    private let actionQueue = DispatchQueue(label: "gtmux.hq-workspace.actions", qos: .userInitiated)
    private let deliveryQueue = DispatchQueue(label: "gtmux.hq-workspace.delivery", qos: .userInitiated)
    private let sourcesQueue = DispatchQueue(label: "gtmux.hq-workspace.sources", qos: .utility)
    private var lastSourcesAttempt: Date?
    private var timer: Timer?
    private var visible = false
    private var generation = 0
    private var preparedHumanDrafts: [HQWorkspaceSelection: String] = [:]
    private var replyTargets: [HQWorkspaceSelection: String] = [:]
    private var drafts: [HQWorkspaceSelection: String] = [:]
    private var oldMessages: [HQWorkspaceMessage] = []
    private var earlier = 0
    private var historyHasEarlier = false
    private var historyBefore: UInt64?
    private var retrySubmission: (selection: HQWorkspaceSelection, body: String, action: HQWorkspaceAction)?
    var readPhase: HQReadPhase {
        let loaded = selection.scope == nil ? snapshot != nil : detail != nil || currentMatter != nil
        return HQReadPhase(loaded: loaded, failed: error != nil && !loading)
    }

    func toggleSection(_ section: HQSidebarSection) {
        if collapsedSections.contains(section) { collapsedSections.remove(section) }
        else { collapsedSections.insert(section) }
    }

    var currentMatter: HQMatter? {
        if case .matter(let id) = selection {
            let saved = snapshot?.matters.first { $0.id == id }
            if let matter = detail?.matter, matter.id == id {
                return (saved?.revision ?? 0) > matter.revision ? saved : matter
            }
            return saved
        }
        return nil
    }
    func currentCandidate(for conversation: HQConversation) -> HQCandidate? {
        guard sourcesKnown else { return nil }
        return snapshot?.conversations.first { $0.id == conversation.id }
    }
    var canSend: Bool {
        if case .matter = selection, currentMatter == nil || currentMatter?.state == "closed" { return false }
        return !acting && selection.showsComposer && draft.utf8.count <= 32768
            && !draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }

    init(client: HQWorkspaceServing = HQWorkspaceClient(), resources: HQResourceStore = HQResourceStore()) {
        self.client = client
        self.resources = resources
    }
    deinit { timer?.invalidate() }

    func start() {
        guard !visible else { return }
        visible = true
        updateResourceLifecycle()
        refresh()
        timer = Timer.scheduledTimer(withTimeInterval: 8, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.refresh(quiet: true) }
        }
    }
    func stop() {
        visible = false
        resources.stop()
        timer?.invalidate()
        timer = nil
        drafts[selection] = draft
    }

    func select(_ target: HQWorkspaceSelection) {
        let reveal = target.showsComposer && target.scope != nil
        if reveal { conversationRevealSerial &+= 1 }
        guard selection != target else {
            if reveal { refresh(quiet: true) }
            return
        }
        drafts[selection] = draft
        selection = target
        updateResourceLifecycle()
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

    private func updateResourceLifecycle() {
        if visible, let page = selection.resourcePage { resources.start(page: page) }
        else { resources.stop() }
    }

    func refreshCurrent() {
        if selection.resourcePage != nil { resources.refresh() }
        else { refresh() }
    }

    // Prepare a global request for review, never submit it from an empty-state button.
    // Keep both the current scope's draft and any existing global draft intact.
    @discardableResult
    func prepareExistingWorkReview(body: String) -> Bool {
        let globalDraft = selection == .conversation ? draft : (drafts[.conversation] ?? "")
        select(.conversation)
        guard globalDraft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            return false
        }
        draft = body
        return true
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
                let snapshot = try service.summary()
                // Canonical questions do not wait for history rendering or optional
                // session discovery. Publication remains bound to this selection.
                DispatchQueue.main.async {
                    guard let self, self.generation == token, self.selection == expected else { return }
                    self.acceptSummary(snapshot)
                }
                let activityUnchanged =
                    expected != .activity || previousSnapshot?.activityRevision == snapshot.activityRevision
                let unchanged =
                    quiet && snapshot.sourcesIncluded != false && activityUnchanged && previousSnapshot?.revision == snapshot.revision
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
                    self.acceptSummary(snapshot)
                    self.detail = self.mergeHistory(detail)
                    self.error = nil
                    self.refreshSerial += 1
                case .failure(let failure): self.error = failure.localizedDescription
                }
            }
        }
    }

    private func acceptSummary(_ value: HQWorkspaceSnapshot) {
        var value = value
        if value.sourcesIncluded == false {
            value.conversations = snapshot?.conversations ?? []
            value.references = snapshot?.references
            value.supervisor = snapshot?.supervisor
        } else {
            sourcesKnown = true
        }
        snapshot = value
        if value.sourcesIncluded == false { refreshSources() }
    }

    // Only source metadata is merged back. A slow old full snapshot must never
    // roll back newer questions, answers, revisions or the user's local draft.
    func refreshSources(force: Bool = false) {
        guard !sourcesLoading, visible || force,
              force || lastSourcesAttempt.map({ Date().timeIntervalSince($0) >= 60 }) ?? true else { return }
        sourcesLoading = true
        sourcesError = nil
        lastSourcesAttempt = Date()
        let service = client
        sourcesQueue.async { [weak self] in
            let result = Result {
                let value = try service.snapshot()
                guard value.sourcesIncluded != false else { throw HQWorkspaceError.command("Session source metadata unavailable") }
                return value
            }
            DispatchQueue.main.async {
                guard let self else { return }
                self.sourcesLoading = false
                switch result {
                case .success(let sources):
                    guard var current = self.snapshot else { return }
                    current.conversations = sources.conversations
                    current.references = sources.references
                    current.supervisor = sources.supervisor
                    self.snapshot = current
                    self.sourcesKnown = true
                    self.sourcesError = nil
                case .failure:
                    self.sourcesKnown = false
                    self.sourcesError = L10n.shared.tr("Session details are temporarily unavailable. Your reply is unaffected.", "会话状态暂时无法读取，不影响答复。")
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
                guard self.selection == expected, self.generation == token else {
                    if self.visible { self.refresh() }
                    return
                }
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

    // Choosing an option only prepares a draft. Preserve existing user text;
    // the Send action still performs the normal durable submission.
    func prepareHumanReply(_ question: HQHumanInput, choice: String) {
        guard let active = currentMatter?.userInput, active.id == question.id, active.answerId == nil else {
            error = L10n.shared.tr("This question has changed. Reload it before replying.", "问题已更新，请重新读取后答复。")
            return
        }
        let response = question.prompt + "\n" + choice
        guard draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || draft == preparedHumanDrafts[selection] else {
            error = L10n.shared.tr("Keep or send your current draft before choosing a reply.", "请先处理当前草稿，再选择答复。")
            return
        }
        draft = response
        preparedHumanDrafts[selection] = response
        replyTargets[selection] = question.id
        composerFocusSerial &+= 1
    }

    var preparedHumanChoice: String? {
        guard let question = currentMatter?.userInput, replyTargets[selection] == question.id,
              draft == preparedHumanDrafts[selection] else { return nil }
        let prefix = question.prompt + "\n"
        return draft.hasPrefix(prefix) ? String(draft.dropFirst(prefix.count)) : nil
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
                    op: "send", id: id, matter: origin.scope == "global" ? nil : origin.scope, body: text, replyTo: replyTargets[origin])
            }
            retrySubmission = (origin, text, action)
        }
        perform(action) { [weak self] in
            guard let self else { return }
            self.drafts[origin] = ""
            self.replyTargets.removeValue(forKey: origin)
            self.preparedHumanDrafts.removeValue(forKey: origin)
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
