// Notification Service Extension — runs for every `mutable-content` remote push and
// attaches a KIND badge so the phone notification reads differently at a glance:
// waiting = a red "stop" glyph with two bars, done = a green ✓. The glyph sits SMALL
// and centered on a dark app-icon-style tile with generous margin — because iOS
// blows the attachment up to a full-width HERO when the banner is long-pressed, so a
// full-bleed color block looked like a giant slab. The dark tile makes the expanded
// view read as a calm icon while the collapsed thumbnail still carries the status
// color. The `kind` rides in the push's custom data (relay/apns.go).

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

    /// Render the status badge to a temp PNG: a dark rounded app-icon tile with the
    /// status glyph small + centered (generous dark margin). UIKit origin is top-left.
    /// Colors match the app's status language (waiting #EF4444, done #22C55E) on the
    /// app's avatar-container tone (#1C1C1F).
    static func badgeImage(kind: String) -> URL? {
        let waiting = (kind == "waiting")
        let s: CGFloat = 240
        let img = UIGraphicsImageRenderer(size: CGSize(width: s, height: s)).image { _ in
            // Dark app-icon tile so the EXPANDED hero reads as a calm icon, not a
            // full-bleed color slab.
            UIColor(red: 0x1C / 255, green: 0x1C / 255, blue: 0x1F / 255, alpha: 1).setFill()
            UIBezierPath(roundedRect: CGRect(x: 0, y: 0, width: s, height: s), cornerRadius: s * 0.225).fill()

            // Status glyph, ~half the tile, centered.
            let g = s * 0.5
            let rect = CGRect(x: (s - g) / 2, y: (s - g) / 2, width: g, height: g)
            if waiting {
                UIColor(red: 0xEF / 255, green: 0x44 / 255, blue: 0x44 / 255, alpha: 1).setFill()
                UIBezierPath(roundedRect: rect, cornerRadius: g * 0.26).fill()
                UIColor.white.setFill() // two vertical bars = "waiting for you"
                let bw = rect.width * 0.15, bh = rect.height * 0.44, gap = rect.width * 0.16
                let cy = rect.midY - bh / 2
                UIBezierPath(roundedRect: CGRect(x: rect.midX - gap / 2 - bw, y: cy, width: bw, height: bh), cornerRadius: bw / 2).fill()
                UIBezierPath(roundedRect: CGRect(x: rect.midX + gap / 2, y: cy, width: bw, height: bh), cornerRadius: bw / 2).fill()
            } else {
                UIColor(red: 0x22 / 255, green: 0xC5 / 255, blue: 0x5E / 255, alpha: 1).setFill()
                UIBezierPath(ovalIn: rect).fill()
                let p = UIBezierPath() // white check
                p.lineWidth = g * 0.1
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
