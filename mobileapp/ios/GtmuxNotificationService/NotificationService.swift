// Notification Service Extension — runs for every `mutable-content` remote push and
// attaches a KIND badge so the phone notification reads differently at a glance:
// waiting = a red "stop" disc with two bars, done = a green ✓ disc. iOS shows the
// attachment as the thumbnail on the right of the banner (the app icon on the left
// is fixed). The `kind` rides in the push's custom data (relay/apns.go).

import UIKit
import UserNotifications

class NotificationService: UNNotificationServiceExtension {
    private var contentHandler: ((UNNotificationContent) -> Void)?

    override func didReceive(_ request: UNNotificationRequest,
                             withContentHandler contentHandler: @escaping (UNNotificationContent) -> Void) {
        self.contentHandler = contentHandler
        guard let content = request.content.mutableCopy() as? UNMutableNotificationContent else {
            contentHandler(request.content)
            return
        }
        let kind = content.userInfo["kind"] as? String ?? ""
        if let url = Self.badgeImage(kind: kind),
           let att = try? UNNotificationAttachment(identifier: "gtmux-badge", url: url, options: nil) {
            content.attachments = [att]
        }
        contentHandler(content)
    }

    override func serviceExtensionTimeWillExpire() {
        contentHandler?(UNMutableNotificationContent())
    }

    /// Render the status badge to a temp PNG (transparent square). UIKit origin is
    /// top-left. Colors match the app's status language (waiting #EF4444, idle
    /// #22C55E).
    static func badgeImage(kind: String) -> URL? {
        let waiting = (kind == "waiting")
        let s: CGFloat = 120
        let img = UIGraphicsImageRenderer(size: CGSize(width: s, height: s)).image { _ in
            let rect = CGRect(x: 12, y: 12, width: s - 24, height: s - 24)
            if waiting {
                UIColor.white.setFill()
                UIBezierPath(roundedRect: rect.insetBy(dx: -7, dy: -7), cornerRadius: 34).fill()
                UIColor(red: 0xEF / 255, green: 0x44 / 255, blue: 0x44 / 255, alpha: 1).setFill()
                UIBezierPath(roundedRect: rect, cornerRadius: 26).fill()
                UIColor.white.setFill() // two vertical bars = "waiting for you"
                let bw = rect.width * 0.13, bh = rect.height * 0.44, gap = rect.width * 0.16
                let cy = rect.midY - bh / 2
                UIBezierPath(roundedRect: CGRect(x: rect.midX - gap / 2 - bw, y: cy, width: bw, height: bh), cornerRadius: bw / 2).fill()
                UIBezierPath(roundedRect: CGRect(x: rect.midX + gap / 2, y: cy, width: bw, height: bh), cornerRadius: bw / 2).fill()
            } else {
                UIColor.white.setFill()
                UIBezierPath(ovalIn: rect.insetBy(dx: -7, dy: -7)).fill()
                UIColor(red: 0x22 / 255, green: 0xC5 / 255, blue: 0x5E / 255, alpha: 1).setFill()
                UIBezierPath(ovalIn: rect).fill()
                let p = UIBezierPath() // white check
                p.lineWidth = 11
                p.lineCapStyle = .round
                p.lineJoinStyle = .round
                p.move(to: CGPoint(x: rect.minX + rect.width * 0.28, y: rect.minY + rect.height * 0.52))
                p.addLine(to: CGPoint(x: rect.minX + rect.width * 0.44, y: rect.minY + rect.height * 0.66))
                p.addLine(to: CGPoint(x: rect.minX + rect.width * 0.74, y: rect.minY + rect.height * 0.36))
                UIColor.white.setStroke()
                p.stroke()
            }
        }
        guard let png = img.pngData() else { return nil }
        let url = FileManager.default.temporaryDirectory.appendingPathComponent("gtmux-badge-\(waiting ? "waiting" : "done").png")
        try? png.write(to: url)
        return url
    }
}
