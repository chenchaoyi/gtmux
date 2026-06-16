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

    private let category = "GTMUX_AGENT"
    private let jumpAction = "JUMP"

    /// Wire up authorization, the Jump action, the queue watcher, and an initial
    /// drain. `onJump` jumps to a pane id (or `focus --last` when it's empty).
    func start(onJump: @escaping (String) -> Void) {
        self.onJump = onJump
        guard Bundle.main.bundleIdentifier != nil else {
            dbg("notifications: no bundle id (bare binary) — skipping setup")
            return
        }
        let center = UNUserNotificationCenter.current()
        center.delegate = self
        let jump = UNNotificationAction(
            identifier: jumpAction,
            title: L10n.shared.tr("Jump", "跳转"),
            options: [.foreground])
        center.setNotificationCategories([
            UNNotificationCategory(identifier: category, actions: [jump],
                                   intentIdentifiers: [], options: [])
        ])
        center.requestAuthorization(options: [.alert, .sound]) { granted, err in
            dbg("notifications: authorization granted=\(granted) err=\(String(describing: err))")
        }
        try? FileManager.default.createDirectory(at: queueDir, withIntermediateDirectories: true)
        drain()
        watch()
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
        let content = UNMutableNotificationContent()
        content.title = req.title.isEmpty ? "gtmux" : req.title
        if !req.subtitle.isEmpty { content.subtitle = req.subtitle }
        content.body = req.body
        content.categoryIdentifier = category
        content.userInfo = ["pane": req.pane]
        if !req.session.isEmpty { content.threadIdentifier = req.session } // coalesce per session
        // "needs you" is the urgent one → sound; "finished" is calm → silent.
        content.sound = req.kind == "input" ? .default : nil
        if let att = attachment(req.icon) { content.attachments = [att] }

        let request = UNNotificationRequest(
            identifier: UUID().uuidString, content: content, trigger: nil)
        UNUserNotificationCenter.current().add(request) { err in
            if let err = err { dbg("notifications: post failed \(err)") }
        }
    }

    /// Copy the icon to a unique temp file before attaching: UNNotificationAttachment
    /// takes ownership of (moves) the file, and we must not consume the shared
    /// cached icon at notify-icon.png.
    private func attachment(_ iconPath: String) -> UNNotificationAttachment? {
        guard !iconPath.isEmpty, FileManager.default.fileExists(atPath: iconPath) else { return nil }
        let tmp = URL(fileURLWithPath: NSTemporaryDirectory())
            .appendingPathComponent("gtmux-icon-\(UUID().uuidString).png")
        do {
            try FileManager.default.copyItem(at: URL(fileURLWithPath: iconPath), to: tmp)
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
        if response.actionIdentifier == UNNotificationDismissActionIdentifier {
            handler(); return
        }
        onJump?(pane)
        handler()
    }
}
