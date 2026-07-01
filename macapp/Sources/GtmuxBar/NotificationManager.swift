import AppKit
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
final class NotificationManager: NSObject, UNUserNotificationCenterDelegate {
    static let shared = NotificationManager()

    private let queueDir: URL = URL(fileURLWithPath: NSHomeDirectory())
        .appendingPathComponent(".local/share/gtmux/notify", isDirectory: true)
    private var onJump: ((String) -> Void)?
    private var source: DispatchSourceFileSystemObject?
    private var dirFD: Int32 = -1

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
    private var lastKind: [String: String] = [:]      // pane → last posted kind (dedup)
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
        guard Bundle.main.bundleIdentifier != nil else {
            dbg("notifications: no bundle id (bare binary) — skipping setup")
            return
        }
        let center = UNUserNotificationCenter.current()
        center.delegate = self
        // The "done" category is static (just Jump). "input" categories are built
        // per-pane at post time so the buttons carry the agent's real 1/2/3 labels.
        let jump = UNNotificationAction(
            identifier: jumpAction, title: L10n.shared.tr("Jump", "跳转"), options: [.foreground])
        register(UNNotificationCategory(identifier: doneCategory, actions: [jump],
                                        intentIdentifiers: [], options: []))
        center.requestAuthorization(options: [.alert, .sound]) { granted, err in
            dbg("notifications: authorization granted=\(granted) err=\(String(describing: err))")
        }
        try? FileManager.default.createDirectory(at: queueDir, withIntermediateDirectories: true)
        drain()
        watch()
    }

    /// Withdraw waiting banners whose pane is no longer waiting (A2 auto-withdraw).
    /// Called from the agent poll so a prompt you answered elsewhere clears itself.
    func reconcile(waitingPanes: Set<String>) {
        let resolved = lastKind.filter { $0.value == "input" && !waitingPanes.contains($0.key) }.map { $0.key }
        guard !resolved.isEmpty else { return }
        UNUserNotificationCenter.current().removeDeliveredNotifications(
            withIdentifiers: resolved.map(waitID))
        for p in resolved { lastKind[p] = nil }
    }

    private func register(_ cat: UNNotificationCategory) {
        categories[cat.identifier] = cat
        UNUserNotificationCenter.current().setNotificationCategories(Set(categories.values))
    }

    // MARK: queue

    /// Watch the queue dir so a new request is posted near-instantly (a rename
    /// into the dir fires .write), not on the next poll tick.
    private func watch() {
        dirFD = open(queueDir.path, O_EVTONLY)
        guard dirFD >= 0 else { dbg("notifications: cannot watch \(queueDir.path)"); return }
        let s = DispatchSource.makeFileSystemObjectSource(
            fileDescriptor: dirFD, eventMask: [.write], queue: .main)
        s.setEventHandler { [weak self] in self?.drain() }
        s.setCancelHandler { [weak self] in
            if let fd = self?.dirFD, fd >= 0 { close(fd) }
            self?.dirFD = -1
        }
        s.resume()
        source = s
    }

    private func drain() {
        let fm = FileManager.default
        guard let files = try? fm.contentsOfDirectory(
            at: queueDir, includingPropertiesForKeys: nil,
            options: [.skipsHiddenFiles]) else { return }
        for f in files where f.pathExtension == "json" {
            defer { try? fm.removeItem(at: f) } // one-shot: always consume
            guard let data = try? Data(contentsOf: f),
                  let req = try? JSONDecoder().decode(Request.self, from: data) else { continue }
            // Drop stale requests so a backlog (app was closed) doesn't spam on launch.
            if req.ts > 0, Date().timeIntervalSince1970 - Double(req.ts) > 30 { continue }
            guard AppSettings.shared.notifications else { continue } // respect the toggle
            post(req)
        }
    }

    private func post(_ req: Request) {
        // Dedup: the same pane already showing the same kind isn't re-posted (A2).
        if lastKind[req.pane] == req.kind, !req.pane.isEmpty { return }

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
            content.interruptionLevel = .timeSensitive // pierce Focus — you're blocking it
            id = waitID(req.pane)
        } else {
            content.categoryIdentifier = doneCategory
            content.sound = nil // finished is calm
            id = req.pane.isEmpty ? UUID().uuidString : doneID(req.pane)
        }

        UNUserNotificationCenter.current().add(
            UNNotificationRequest(identifier: id, content: content, trigger: nil)
        ) { err in if let err = err { dbg("notifications: post failed \(err)") } }
        if !req.pane.isEmpty { lastKind[req.pane] = req.kind }
    }

    /// Register (and return the id of) a per-pane category whose action buttons are
    /// the agent's actual choices: "1 · Yes", "2 · …", … + a free-text reply. Falls
    /// back to text-reply + Jump when no menu parses.
    private func replyCategory(for pane: String) -> String {
        let data = GtmuxCLI.capture(["options", pane]) ?? Data("[]".utf8)
        let opts = (try? JSONDecoder().decode([ReplyOption].self, from: data)) ?? []
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
        let pane = response.notification.request.content.userInfo["pane"] as? String ?? ""
        let action = response.actionIdentifier

        switch action {
        case UNNotificationDismissActionIdentifier:
            break
        case replyTextAction:
            if let r = response as? UNTextInputNotificationResponse, !r.userText.isEmpty {
                onSendText?(pane, r.userText)
                lastKind[pane] = nil // answered → let it re-notify on a future prompt
            }
        case let a where a.hasPrefix(replyPrefix):
            if let n = Int(a.dropFirst(replyPrefix.count)) {
                onSend?(pane, n)
                lastKind[pane] = nil
            }
        default:
            // JUMP action or the default click → land on the pane.
            onJump?(pane)
        }
        handler()
    }
}
