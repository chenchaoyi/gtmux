import AppKit

/// Status-bar display mode (DESIGN §2/§8).
enum DisplayMode: String, CaseIterable {
    case dotCount      // glyph + count (default)
    case dot           // glyph only
    case hideWhenIdle  // hidden unless something is waiting on you
}

/// StatusItemGlyph renders the menu-bar **brand** status item (ITERATIONS D1 /
/// mockup §02): the status item is ALWAYS the gtmux pane grid, so it reads as
/// gtmux in the bar. The TOP-RIGHT cell (= the pane that needs attention) lights
/// up with the most-urgent state — color + a micro-glyph (waiting double-bars /
/// working open ring / idle check) + the count next to it (drawn by AppDelegate
/// as the button title). The other cells stay neutral and adapt to the bar.
///
/// Supersedes the older lone shape-shift glyph (DESIGN §2/§12 "状态项 ≠ logo"):
/// D1 deliberately makes the status item brand-consistent. Still triple-encoded
/// (color + shape + count), so it's legible for colorblind / peripheral vision.
enum StatusItemGlyph {
    static let size: CGFloat = 18

    /// The brand grid with the top-right cell lit by the most-urgent state, or a
    /// quiet all-neutral grid when empty / only running (nothing needs you).
    static func image(mostUrgent: Status, empty: Bool, dark: Bool) -> NSImage {
        let lit: Status? = (empty || mostUrgent == .running) ? nil : mostUrgent
        return draw { _, full in grid(full, dark: dark, lit: lit) }
    }

    // MARK: pane grid

    private static func grid(_ full: CGRect, dark: Bool, lit: Status?) {
        let gap: CGFloat = 2
        let cell = (full.width - gap) / 2           // square top cells
        let radius: CGFloat = 2
        let neutral = (dark ? NSColor.white : NSColor.black).withAlphaComponent(dark ? 0.55 : 0.45)

        let topY = full.minY + cell + gap
        let topLeft = CGRect(x: full.minX, y: topY, width: cell, height: cell)
        let topRight = CGRect(x: full.minX + cell + gap, y: topY, width: cell, height: cell)
        let bottom = CGRect(x: full.minX, y: full.minY, width: full.width, height: cell)

        neutral.setFill()
        NSBezierPath(roundedRect: topLeft, xRadius: radius, yRadius: radius).fill()
        NSBezierPath(roundedRect: bottom, xRadius: radius, yRadius: radius).fill()

        guard let lit = lit else {
            NSBezierPath(roundedRect: topRight, xRadius: radius, yRadius: radius).fill()
            return
        }
        lit.nsColor.setFill()
        NSBezierPath(roundedRect: topRight, xRadius: radius, yRadius: radius).fill()
        // a 0.5pt white rim so a red/cyan cell keeps contrast on a tinted bar.
        NSColor.white.withAlphaComponent(0.7).setStroke()
        let rim = NSBezierPath(roundedRect: topRight.insetBy(dx: 0.25, dy: 0.25), xRadius: radius, yRadius: radius)
        rim.lineWidth = 0.5; rim.stroke()

        switch lit {
        case .waiting: waitingBars(in: topRight)
        case .working: workingRing(in: topRight)
        case .idle:    idleCheck(in: topRight)
        case .running: break
        }
    }

    // MARK: micro-glyphs (white, inside the lit cell)

    private static func waitingBars(in r: CGRect) {
        NSColor.white.setFill()
        let barW: CGFloat = 1.3, gap: CGFloat = 1.2, h = r.height * 0.56
        let y = r.midY - h / 2
        NSBezierPath(roundedRect: CGRect(x: r.midX - gap / 2 - barW, y: y, width: barW, height: h), xRadius: 0.6, yRadius: 0.6).fill()
        NSBezierPath(roundedRect: CGRect(x: r.midX + gap / 2, y: y, width: barW, height: h), xRadius: 0.6, yRadius: 0.6).fill()
    }

    private static func workingRing(in r: CGRect) {
        // static open ring — "in progress" via shape, never rotates (DESIGN §10).
        let ring = NSBezierPath()
        ring.appendArc(withCenter: CGPoint(x: r.midX, y: r.midY), radius: r.width * 0.30, startAngle: 70, endAngle: 400)
        NSColor.white.setStroke(); ring.lineWidth = 1.2; ring.lineCapStyle = .round; ring.stroke()
    }

    private static func idleCheck(in r: CGRect) {
        let p = NSBezierPath()
        p.move(to: CGPoint(x: r.minX + r.width * 0.24, y: r.midY - r.height * 0.02))
        p.line(to: CGPoint(x: r.midX - r.width * 0.04, y: r.minY + r.height * 0.26))
        p.line(to: CGPoint(x: r.maxX - r.width * 0.18, y: r.maxY - r.height * 0.24))
        NSColor.white.setStroke(); p.lineWidth = 1.3; p.lineJoinStyle = .round; p.lineCapStyle = .round; p.stroke()
    }

    // MARK: draw helper

    private static func draw(_ body: (CGContext, CGRect) -> Void) -> NSImage {
        let img = NSImage(size: NSSize(width: size, height: size))
        img.lockFocus()
        let ctx = NSGraphicsContext.current!.cgContext
        // 1pt margin so the 16×16 grid never clips against the 22pt bar.
        body(ctx, CGRect(x: 1, y: 1, width: size - 2, height: size - 2))
        img.unlockFocus()
        img.isTemplate = false // keep the lit cell's color even when the bar is tinted.
        return img
    }
}
