import AppKit
import Combine
import UserNotifications

/// NotificationManager delivers gtmux's desktop notifications natively — the job
/// that used to belong to terminal-notifier. The CLI hook drops JSON requests in
/// ~/.local/share/gtmux/notify/; this drains that queue and posts real
/// UNUserNotificationCenter banners (shown as "Gtmux", with the agent icon, a
/// Jump action, and a click that lands you on the pane that finished).
///
/// Posting requires a real app bundle + notification permission, so this is a
/// no-op when run as a bare binary (Bundle has no identifier) — guarded so dev
/// runs of `.build/release/GtmuxBar` don't crash on UNUserNotificationCenter.
final class NotificationManager: NSObject, ObservableObject, UNUserNotificationCenterDelegate {
    static let shared = NotificationManager()

    private let queueDir: URL
    private let center: DesktopNotificationCenter
    private let enabled: () -> Bool
    private let options: (String) -> [ReplyOption]
    private let bundleIdentifier: String?
    private let now: () -> Date
    @Published private(set) var authorization = DesktopNotificationAuthorization(permission: .checking)
    enum TestState { case idle, sending, accepted, blocked, failed }
    @Published private(set) var testState = TestState.idle
    private var readingSettings = false
    private var pendingTest = false
    private var nextSettingsRead = Date.distantPast
    private var nextWatchAttempt = Date.distantPast
    private var waitingBanners: Set<String> = []
    private var waitingSnapshot: Set<String>?

    init(center: DesktopNotificationCenter = SystemDesktopNotificationCenter(),
         queueDirectory: URL = URL(fileURLWithPath: Paths.data("notify"), isDirectory: true),
         enabled: @escaping () -> Bool = { AppSettings.shared.notifications },
         options: @escaping (String) -> [ReplyOption] = NotificationManager.readOptions,
         bundleIdentifier: String? = Bundle.main.bundleURL.pathExtension == "app" ? Bundle.main.bundleIdentifier : nil,
         now: @escaping () -> Date = Date.init) {
        self.center = center; self.queueDir = queueDirectory; self.enabled = enabled
        self.options = options; self.bundleIdentifier = bundleIdentifier; self.now = now
        super.init()
    }

    deinit { source?.cancel() }
    private var onJump: ((String) -> Void)?
    private var source: DispatchSourceFileSystemObject?

    /// Request: keep field names in sync with internal/notify `request`.
    struct Request: Decodable {
        var kind = "done"
        var title = ""
        var subtitle = ""
        var body = ""
        var pane = ""
        var session = ""
        var icon = ""
        var ts = 0
        // tolerate missing fields
        enum CodingKeys: String, CodingKey { case kind, title, subtitle, body, pane, session, icon, ts }
        init(from decoder: Decoder) throws {
            let c = try decoder.container(keyedBy: CodingKeys.self)
            func s(_ k: CodingKeys) -> String { (try? c.decode(String.self, forKey: k)) ?? "" }
            kind = (try? c.decode(String.self, forKey: .kind)) ?? "done"
            title = s(.title); subtitle = s(.subtitle); body = s(.body)
            pane = s(.pane); session = s(.session); icon = s(.icon)
            ts = (try? c.decode(Int.self, forKey: .ts)) ?? 0
        }
    }

    private let doneCategory = "GTMUX_DONE"
    private let jumpAction = "JUMP"
    private let replyTextAction = "REPLY_TEXT"
    private let replyPrefix = "REPLY_" // + N

    private var onSend: ((String, Int) -> Void)?      // pane, option number
    private var onSendText: ((String, String) -> Void)? // pane, free text
    private var categories: [String: UNNotificationCategory] = [:]

    private func waitID(_ pane: String) -> String { "gtmux-wait-\(pane)" }
    private func doneID(_ pane: String) -> String { "gtmux-done-\(pane)" }

    /// Wire up authorization, the actions, the queue watcher, and an initial drain.
    /// `onJump` lands on a pane; `onSend`/`onSendText` answer a waiting prompt from
    /// the notification itself (A2) without opening the app.
    func start(onJump: @escaping (String) -> Void,
               onSend: @escaping (String, Int) -> Void = { _, _ in },
               onSendText: @escaping (String, String) -> Void = { _, _ in }) {
        self.onJump = onJump
        self.onSend = onSend
        self.onSendText = onSendText
        guard bundleIdentifier != nil else {
            dbg("notifications: no bundle id (bare binary) — skipping setup")
            return
        }
        center.setDelegate(self)
        // The "done" category is static (just Jump). "input" categories are built
        // per-pane at post time so the buttons carry the agent's real 1/2/3 labels.
        let jump = UNNotificationAction(
            identifier: jumpAction, title: L10n.shared.tr("Jump", "跳转"), options: [.foreground])
        register(UNNotificationCategory(identifier: doneCategory, actions: [jump],
                                        intentIdentifiers: [], options: []))
        // Watch before the initial read, and wait for the authorization verdict.
        // A request arriving during setup must neither disappear nor race the prompt.
        watch()
        refreshAuthorization(requestIfNeeded: true)
    }

    /// The normal poll is also a recovery path for a lost filesystem event.
    func reconcile(waitingPanes: Set<String>) {
        guard bundleIdentifier != nil else { return }
        waitingSnapshot = waitingPanes
        let resolved = waitingBanners.subtracting(waitingPanes)
        if !resolved.isEmpty {
            center.removeDelivered(resolved.map(waitID))
            waitingBanners.subtract(resolved)
        }
        if now() >= nextSettingsRead { refreshAuthorization() }
        if source == nil { watch() }
        drain()
    }

    func refreshAuthorization(requestIfNeeded: Bool = false) {
        guard bundleIdentifier != nil, !readingSettings else { return }
        readingSettings = true
        center.settings { [weak self] state in
            DispatchQueue.main.async {
                guard let self else { return }
                if requestIfNeeded && state.permission == .notRequested {
                    self.center.requestAuthorization { [weak self] error in
                        DispatchQueue.main.async {
                            guard let self else { return }
                            if let error {
                                DiagLog.warn("notify.authorization", "notification permission request failed",
                                             ["error": error.localizedDescription])
                            }
                            self.readingSettings = false
                            self.refreshAuthorization()
                        }
                    }
                } else {
                    if self.authorization != state {
                        DiagLog.info("notify.authorization", "read macOS notification settings",
                                     ["allowed": state.allowsDelivery, "banners": state.banners, "sounds": state.sounds])
                    }
                    self.authorization = state
                    self.readingSettings = false
                    self.nextSettingsRead = self.now().addingTimeInterval(30)
                    self.drain()
                    if self.pendingTest {
                        self.pendingTest = false
                        self.postTest()
                    }
                }
            }
        }
    }

    func sendTest() {
        guard testState != .sending else { return }
        guard bundleIdentifier != nil else { testState = .blocked; return }
        testState = .sending
        pendingTest = true
        refreshAuthorization(requestIfNeeded: true)
    }

    private func postTest() {
        guard enabled(), authorization.allowsDelivery else { testState = .blocked; return }
        let content = UNMutableNotificationContent()
        content.title = "gtmux"
        content.body = L10n.shared.tr("This is a test notification. No agent was contacted.", "这是一条测试通知，不会向会话发送消息。")
        content.sound = .default
        content.userInfo = ["test": true]
        center.add(UNNotificationRequest(identifier: "gtmux-test-" + UUID().uuidString,
                                        content: content, trigger: nil)) { [weak self] error in
            DispatchQueue.main.async {
                self?.testState = error == nil ? .accepted : .failed
                DiagLog.act("act.notify.test", target: "", outcome: error == nil ? "ok" : "failed",
                            error == nil ? "submitted a test notification to macOS" : "test notification submission failed")
            }
        }
    }

    func openSystemSettings() {
        if let url = URL(string: "x-apple.systempreferences:com.apple.Notifications-Settings.extension?id=com.gtmux.menubar") {
            NSWorkspace.shared.open(url)
        }
    }

    private func register(_ cat: UNNotificationCategory) {
        categories[cat.identifier] = cat
        center.setCategories(Set(categories.values))
    }

    // MARK: queue

    /// Watch the queue dir so a new request is posted near-instantly (a rename
    /// into the dir fires .write), not on the next poll tick.
    private func watch() {
        guard source == nil, now() >= nextWatchAttempt else { return }
        nextWatchAttempt = now().addingTimeInterval(30)
        do {
            try FileManager.default.createDirectory(at: queueDir, withIntermediateDirectories: true,
                                                    attributes: [.posixPermissions: 0o700])
        } catch {
            DiagLog.warn("notify.queue", "cannot create notification queue")
            return
        }
        let fd = open(queueDir.path, O_EVTONLY)
        guard fd >= 0 else { DiagLog.warn("notify.queue", "cannot watch notification queue"); return }
        let watcher = DispatchSource.makeFileSystemObjectSource(
            fileDescriptor: fd, eventMask: [.write, .delete, .rename, .revoke], queue: .main)
        watcher.setEventHandler { [weak self, weak watcher] in
            guard let self, let watcher else { return }
            if !watcher.data.intersection([.delete, .rename, .revoke]).isEmpty {
                watcher.cancel()
                self.source = nil
                self.watch()
            }
            self.drain()
        }
        watcher.setCancelHandler { close(fd) }
        source = watcher
        nextWatchAttempt = .distantPast
        watcher.resume()
    }

    private func drain() {
        guard !readingSettings, authorization.permission != .checking else { return }
        let fm = FileManager.default
        guard let files = try? fm.contentsOfDirectory(
            at: queueDir, includingPropertiesForKeys: nil,
            options: [.skipsHiddenFiles]) else { return }
        for f in files.sorted(by: { $0.lastPathComponent < $1.lastPathComponent }) where f.pathExtension == "json" {
            defer { try? fm.removeItem(at: f) } // one-shot: always consume
            guard let data = try? Data(contentsOf: f),
                  let req = try? JSONDecoder().decode(Request.self, from: data) else {
                DiagLog.act("act.notify.post", target: "", outcome: "refused",
                            "notification request could not be read", ["reason": "invalid_request"])
                continue
            }
            // Drop stale requests so a backlog (app was closed) doesn't spam on launch.
            if req.ts > 0, now().timeIntervalSince1970 - Double(req.ts) > 30 {
                DiagLog.act("act.notify.post", target: req.pane, outcome: "refused",
                            "a notification was not shown", ["reason": "stale", "kind": req.kind])
                continue
            }
            guard enabled() else { // respect the toggle
                DiagLog.act("act.notify.post", target: req.pane, outcome: "refused",
                            "a notification was not shown", ["reason": "off", "kind": req.kind])
                continue
            }
            guard authorization.allowsDelivery else {
                DiagLog.act("act.notify.post", target: req.pane, outcome: "refused",
                            "notification permission is not available", ["reason": "system_permission", "kind": req.kind])
                continue
            }
            if req.kind == "input", let snapshot = waitingSnapshot, !req.pane.isEmpty, !snapshot.contains(req.pane) {
                DiagLog.act("act.notify.post", target: req.pane, outcome: "refused",
                            "the waiting prompt was already resolved", ["reason": "resolved", "kind": req.kind])
                continue
            }
            post(req)
        }
    }

    private func post(_ req: Request) {
        // Each consumed queue file is an event. Stable system identifiers replace
        // an older banner without suppressing a later turn on the same pane.
        let content = UNMutableNotificationContent()
        content.title = req.title.isEmpty ? "gtmux" : req.title
        if !req.subtitle.isEmpty { content.subtitle = req.subtitle }
        content.body = req.body
        content.userInfo = ["pane": req.pane]
        if !req.session.isEmpty { content.threadIdentifier = req.session } // coalesce per session
        if let att = attachment(req.icon, kind: req.kind) { content.attachments = [att] }

        let isInput = req.kind == "input"
        let id: String
        if isInput, !req.pane.isEmpty {
            // Build this pane's reply buttons from the agent's real 1/2/3 labels
            // (shared parser via `gtmux options`), plus a free-text reply.
            content.categoryIdentifier = replyCategory(for: req.pane)
            content.sound = .default
            content.interruptionLevel = .active // respect the user's Focus settings
            id = waitID(req.pane)
        } else {
            content.categoryIdentifier = doneCategory
            content.sound = nil // finished is calm
            id = req.pane.isEmpty ? UUID().uuidString : doneID(req.pane)
        }

        let pane = req.pane, kind = req.kind
        center.add(UNNotificationRequest(identifier: id, content: content, trigger: nil)) { [weak self] error in
            DispatchQueue.main.async {
                guard let self else { return }
                if let error {
                    DiagLog.act("act.notify.post", target: pane, outcome: "failed",
                                "notification submission failed", ["error": error.localizedDescription, "kind": kind])
                } else {
                    if kind == "input", !pane.isEmpty {
                        if let snapshot = self.waitingSnapshot, !snapshot.contains(pane) {
                            self.center.removeDelivered([self.waitID(pane)])
                        } else { self.waitingBanners.insert(pane) }
                    }
                    DiagLog.act("act.notify.post", target: pane, outcome: "ok",
                                "submitted a notification to macOS", ["kind": kind])
                }
            }
        }
    }

    /// Register (and return the id of) a per-pane category whose action buttons are
    /// the agent's actual choices: "1 · Yes", "2 · …", … + a free-text reply. Falls
    /// back to text-reply + Jump when no menu parses.
    private func replyCategory(for pane: String) -> String {
        let opts = options(pane)
        var actions = opts.prefix(3).map { o in
            UNNotificationAction(identifier: "\(replyPrefix)\(o.n)",
                                 title: "\(o.n) · \(o.label)", options: [])
        }
        actions.append(UNTextInputNotificationAction(
            identifier: replyTextAction,
            title: L10n.shared.tr("Reply…", "回复…"),
            options: [],
            textInputButtonTitle: L10n.shared.tr("Send", "发送"),
            textInputPlaceholder: L10n.shared.tr("Type a reply", "输入回复")))
        actions.append(UNNotificationAction(
            identifier: jumpAction, title: L10n.shared.tr("Jump", "跳转"), options: [.foreground]))

        let id = "GTMUX_REPLY_\(pane)"
        register(UNNotificationCategory(identifier: id, actions: actions,
                                        intentIdentifiers: [], options: []))
        return id
    }

    /// Build the notification thumbnail: the agent icon with a STATUS badge in the
    /// corner (kind-specific, matching the popover rows) so a needs-you banner reads
    /// differently at a glance from a finished one — waiting = red "stop" square with
    /// two bars, done = green ✓ disc. Rendered to a fresh temp PNG (the attachment
    /// moves the file, so we never consume the shared notify-icon.png).
    private func attachment(_ iconPath: String, kind: String) -> UNNotificationAttachment? {
        guard !iconPath.isEmpty, let base = NSImage(contentsOfFile: iconPath) else { return nil }
        let s: CGFloat = 96
        let img = NSImage(size: NSSize(width: s, height: s))
        img.lockFocus()
        base.draw(in: NSRect(x: 0, y: 0, width: s, height: s))
        // badge bottom-right (AppKit origin is bottom-left), like .badge in the popover
        let d: CGFloat = 42
        let rect = NSRect(x: s - d - 1, y: 1, width: d, height: d)
        let ring = rect.insetBy(dx: -3, dy: -3)
        if kind == "input" {
            NSColor.white.setFill(); NSBezierPath(roundedRect: ring, xRadius: 13, yRadius: 13).fill()
            Theme.Status.waitingNS.setFill(); NSBezierPath(roundedRect: rect, xRadius: 10, yRadius: 10).fill()
            NSColor.white.setFill() // two vertical bars = the "waiting for you" glyph
            let bw = d * 0.13, bh = d * 0.44, gap = d * 0.16
            let cy = rect.midY - bh / 2
            NSBezierPath(roundedRect: NSRect(x: rect.midX - gap / 2 - bw, y: cy, width: bw, height: bh), xRadius: bw / 2, yRadius: bw / 2).fill()
            NSBezierPath(roundedRect: NSRect(x: rect.midX + gap / 2, y: cy, width: bw, height: bh), xRadius: bw / 2, yRadius: bw / 2).fill()
        } else {
            NSColor.white.setFill(); NSBezierPath(ovalIn: ring).fill()
            Theme.Status.idleNS.setFill(); NSBezierPath(ovalIn: rect).fill()
            let p = NSBezierPath() // white check
            p.lineWidth = 4; p.lineCapStyle = .round; p.lineJoinStyle = .round
            p.move(to: NSPoint(x: rect.minX + d * 0.28, y: rect.minY + d * 0.52))
            p.line(to: NSPoint(x: rect.minX + d * 0.43, y: rect.minY + d * 0.36))
            p.line(to: NSPoint(x: rect.minX + d * 0.74, y: rect.minY + d * 0.66))
            NSColor.white.setStroke(); p.stroke()
        }
        img.unlockFocus()

        guard let tiff = img.tiffRepresentation, let rep = NSBitmapImageRep(data: tiff),
              let png = rep.representation(using: .png, properties: [:]) else { return nil }
        let tmp = URL(fileURLWithPath: NSTemporaryDirectory())
            .appendingPathComponent("gtmux-icon-\(UUID().uuidString).png")
        do {
            try png.write(to: tmp)
            return try UNNotificationAttachment(identifier: "icon", url: tmp, options: nil)
        } catch {
            dbg("notifications: attachment failed \(error)")
            return nil
        }
    }

    static func readOptions(_ pane: String) -> [ReplyOption] {
        let data = GtmuxCLI.capture(["options", pane]) ?? Data("[]".utf8)
        return (try? JSONDecoder().decode([ReplyOption].self, from: data)) ?? []
    }

    // MARK: UNUserNotificationCenterDelegate

    /// Show the banner + play the sound even when gtmux is the frontmost app.
    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler handler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        handler([.banner, .sound])
    }

    /// Click (or the Jump action) → land on the pane that finished.
    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        didReceive response: UNNotificationResponse,
        withCompletionHandler handler: @escaping () -> Void
    ) {
        if response.notification.request.content.userInfo["test"] as? Bool == true { handler(); return }
        let pane = response.notification.request.content.userInfo["pane"] as? String ?? ""
        let action = response.actionIdentifier

        switch action {
        case UNNotificationDismissActionIdentifier:
            break
        case replyTextAction:
            if let r = response as? UNTextInputNotificationResponse, !r.userText.isEmpty {
                onSendText?(pane, r.userText)
                waitingBanners.remove(pane)
            }
        case let a where a.hasPrefix(replyPrefix):
            if let n = Int(a.dropFirst(replyPrefix.count)) {
                onSend?(pane, n)
                waitingBanners.remove(pane)
            }
        default:
            // JUMP action or the default click → land on the pane.
            onJump?(pane)
        }
        handler()
    }
}
