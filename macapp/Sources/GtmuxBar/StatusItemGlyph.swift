import AppKit

/// Status-bar display mode (DESIGN §2/§8).
enum DisplayMode: String, CaseIterable {
    case dotCount      // glyph + count (default)
    case dot           // glyph only
    case hideWhenIdle  // hidden unless something is waiting on you
}

/// StatusItemGlyph renders the menu-bar shape-shift glyph (DESIGN §2): the shape
/// itself carries the state (calm ring / idle ✓ / working ring / waiting square +
/// double-bars), so it reads in peripheral vision, for colorblind users, and on
/// tinted menu bars. Non-template (keeps its color when the bar is tinted); the
/// count next to it is plain text (auto black/white).
enum StatusItemGlyph {
    static let size: CGFloat = 18

    /// Glyph for the most-urgent state, or the calm hollow ring when empty.
    static func image(mostUrgent: Status, empty: Bool) -> NSImage {
        if empty { return draw { ctx, r in calmRing(ctx, r) } }
        switch mostUrgent {
        case .waiting: return draw { ctx, r in waiting(ctx, r) }
        case .working: return draw { ctx, r in workingRing(ctx, r) }
        case .idle:    return draw { ctx, r in idleCheck(ctx, r) }
        case .running: return draw { ctx, r in runningDot(ctx, r) }
        }
    }

    // MARK: shapes

    private static func waiting(_ ctx: CGContext, _ r: CGRect) {
        let sq = NSBezierPath(roundedRect: r, xRadius: 3.5, yRadius: 3.5)
        Theme.Status.waitingNS.setFill(); sq.fill()
        // 0.5pt white rim for contrast on red/orange tinted bars (DESIGN §2).
        NSColor.white.withAlphaComponent(0.85).setStroke(); sq.lineWidth = 0.5; sq.stroke()
        // two vertical bars (pause)
        NSColor.white.setFill()
        let barW: CGFloat = 1.9, gap: CGFloat = 2.0, h = r.height * 0.46
        let y = r.midY - h / 2
        NSBezierPath(roundedRect: CGRect(x: r.midX - gap / 2 - barW, y: y, width: barW, height: h), xRadius: 0.9, yRadius: 0.9).fill()
        NSBezierPath(roundedRect: CGRect(x: r.midX + gap / 2, y: y, width: barW, height: h), xRadius: 0.9, yRadius: 0.9).fill()
    }

    private static func workingRing(_ ctx: CGContext, _ r: CGRect) {
        // static open ring — "in progress" via shape, never rotates (DESIGN §10).
        let ring = NSBezierPath()
        ring.appendArc(withCenter: CGPoint(x: r.midX, y: r.midY), radius: r.width / 2 - 1, startAngle: 70, endAngle: 400)
        Theme.Status.workingNS.setStroke(); ring.lineWidth = 2; ring.lineCapStyle = .round; ring.stroke()
    }

    private static func idleCheck(_ ctx: CGContext, _ r: CGRect) {
        let p = NSBezierPath()
        p.move(to: CGPoint(x: r.minX + 1.5, y: r.midY - 0.5))
        p.line(to: CGPoint(x: r.midX - 1.5, y: r.minY + 2.5))
        p.line(to: CGPoint(x: r.maxX - 0.5, y: r.maxY - 1.5))
        Theme.Status.idleNS.setStroke(); p.lineWidth = 2; p.lineJoinStyle = .round; p.lineCapStyle = .round; p.stroke()
    }

    private static func runningDot(_ ctx: CGContext, _ r: CGRect) {
        Theme.Status.noneNS.setFill()
        NSBezierPath(ovalIn: CGRect(x: r.midX - 2.5, y: r.midY - 2.5, width: 5, height: 5)).fill()
    }

    private static func calmRing(_ ctx: CGContext, _ r: CGRect) {
        let ring = NSBezierPath(ovalIn: r.insetBy(dx: 1, dy: 1))
        Theme.Status.noneNS.setStroke(); ring.lineWidth = 1.5; ring.stroke()
    }

    // MARK: draw helper

    private static func draw(_ body: (CGContext, CGRect) -> Void) -> NSImage {
        let img = NSImage(size: NSSize(width: size, height: size))
        img.lockFocus()
        let ctx = NSGraphicsContext.current!.cgContext
        body(ctx, CGRect(x: 2.5, y: 2.5, width: size - 5, height: size - 5))
        img.unlockFocus()
        img.isTemplate = false
        return img
    }
}
