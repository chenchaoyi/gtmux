import AppKit

/// Status-bar display mode (DESIGN §2/§8).
enum DisplayMode: String, CaseIterable {
    case dotCount      // glyph + count (default)
    case dot           // glyph only
    case hideWhenIdle  // hidden unless something is waiting on you
}

/// The three-pane brand stays recognizable, with a transparent status symbol in
/// the lower pane: pause / ring / check. Geometry carries state in grayscale too.
/// The neutral grid means no confirmed actionable state; count policy is unchanged.
enum StatusItemGlyph {
    static let size: CGFloat = 18

    /// The brand grid tinted by the most-urgent state, or a quiet neutral grid
    /// when empty / only running (nothing needs you).
    /// - Parameter awake: server mode is on (the Mac is being kept awake). It is drawn
    ///   as a small RED DOT on the mark, borrowing the recording-indicator language:
    ///   a lit dot that breathes says "still running" in a way nothing else does, and
    ///   being impossible to forget is precisely this feature's job.
    /// - Parameter phase: 0…1 breathing phase for that dot.
    static func image(mostUrgent: Status, empty: Bool, dark: Bool,
                      awake: Bool = false, phase: CGFloat = 0) -> NSImage {
        let lit: Status? = (empty || mostUrgent == .running) ? nil : mostUrgent
        return draw { _, full in
            grid(full, dark: dark, lit: lit)
            if let lit { statusSymbol(lit, in: full) }
            if awake { awakeDot(full, dark: dark, phase: phase) }
        }
    }

    /// The awake dot — a deliberate, documented exception to two design rules.
    ///
    /// DESIGN §9 reserves colour for agent state. This independent server-mode
    /// modifier is a bounded exception documented by MOTION-LOAD. It is allowed here
    /// because the recording indicator is the one visual language every user already
    /// reads as "this is still running, you left it on", and server mode's whole risk
    /// is being forgotten. The exception is bounded so it cannot be mistaken for a
    /// waiting agent:
    ///
    ///   * it is a SMALL DOT ON the mark — waiting turns the WHOLE mark red, which is
    ///     a completely different silhouette at a glance;
    ///   * it breathes slowly and shallowly (alpha 0.55…1.0), so it reads as "alive",
    ///     not as "alarm" — nothing else on the status item moves (inside the panel, a
    ///     working row's ring turns while the panel is open; DESIGN §10);
    ///   * a hairline halo keeps it legible when the mark underneath is itself red.
    private static func awakeDot(_ full: CGRect, dark: Bool, phase: CGFloat) {
        let d: CGFloat = 5.5
        let r = CGRect(x: full.maxX - d, y: full.maxY - d, width: d, height: d)
        // Ring first: separates the dot from whatever colour the mark is.
        let halo = NSBezierPath(ovalIn: r.insetBy(dx: -1, dy: -1))
        (dark ? NSColor.black : NSColor.white).withAlphaComponent(0.75).setFill()
        halo.fill()
        let alpha = 0.55 + 0.45 * (0.5 - 0.5 * cos(phase * 2 * .pi))
        Theme.Status.waitingNS.withAlphaComponent(alpha).setFill()
        NSBezierPath(ovalIn: r).fill()
    }

    // MARK: pane grid

    private static func grid(_ full: CGRect, dark: Bool, lit: Status?) {
        let gap: CGFloat = 2
        let cell = (full.width - gap) / 2           // square top cells
        let radius: CGFloat = 2
        let neutral = (dark ? NSColor.white : NSColor.black).withAlphaComponent(dark ? 0.55 : 0.45)
        let tint = lit?.nsColor ?? neutral

        let topY = full.minY + cell + gap
        let topLeft = CGRect(x: full.minX, y: topY, width: cell, height: cell)
        let topRight = CGRect(x: full.minX + cell + gap, y: topY, width: cell, height: cell)
        let bottom = CGRect(x: full.minX, y: full.minY, width: full.width, height: cell)

        tint.setFill()
        for r in [topLeft, topRight, bottom] {
            NSBezierPath(roundedRect: r, xRadius: radius, yRadius: radius).fill()
        }
    }

    // A cutout is independent of tint and menu-bar background. Keep it inside the
    // lower pane; the awake modifier occupies the upper-right pane instead.
    private static func statusSymbol(_ status: Status, in full: CGRect) {
        guard let context = NSGraphicsContext.current?.cgContext else { return }
        context.saveGState()
        context.setBlendMode(.clear)
        let cx = full.midX, cy = full.minY + 3.5
        switch status {
        case .waiting:
            context.fill(CGRect(x: cx - 3, y: cy - 2, width: 2, height: 4))
            context.fill(CGRect(x: cx + 1, y: cy - 2, width: 2, height: 4))
        case .working:
            context.setLineWidth(1.5)
            context.strokeEllipse(in: CGRect(x: cx - 2, y: cy - 2, width: 4, height: 4))
        case .idle:
            context.setLineWidth(1.5)
            context.setLineCap(.round); context.setLineJoin(.round)
            context.move(to: CGPoint(x: cx - 3, y: cy))
            context.addLine(to: CGPoint(x: cx - 1, y: cy - 1.5))
            context.addLine(to: CGPoint(x: cx + 3, y: cy + 2))
            context.strokePath()
        case .running: break
        }
        context.restoreGState()
    }

    // MARK: draw helper

    private static func draw(_ body: (CGContext, CGRect) -> Void) -> NSImage {
        let img = NSImage(size: NSSize(width: size, height: size))
        img.lockFocus()
        let ctx = NSGraphicsContext.current!.cgContext
        // 1pt margin so the 16×16 grid never clips against the 22pt bar.
        body(ctx, CGRect(x: 1, y: 1, width: size - 2, height: size - 2))
        img.unlockFocus()
        img.isTemplate = false // keep the state tint even when the bar is tinted.
        return img
    }
}
