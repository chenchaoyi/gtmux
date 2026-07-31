import AppKit

/// Status-bar display mode (DESIGN §2/§8).
enum DisplayMode: String, CaseIterable {
    case dotCount      // glyph + count (default)
    case dot           // glyph only
    case hideWhenIdle  // hidden unless something is waiting on you
}

/// StatusItemGlyph renders the menu-bar **brand** status item (ITERATIONS D1 /
/// mockup §02): the status item is ALWAYS the gtmux pane grid, so it reads as
/// gtmux in the bar. The WHOLE mark is tinted with the most-urgent state color
/// (waiting=red / working=cyan / idle=green) and the count is drawn next to it
/// (by AppDelegate, as the button title). When nothing needs you — empty, or
/// only background "running" tasks — the mark stays neutral and adapts to the bar.
///
/// Why the whole mark and not one cell: at ~16pt a single lit pane is too easy to
/// miss; tinting the entire mark makes the state unmistakable at a glance while
/// the grid silhouette keeps it on-brand. This trades away the per-state micro-
/// glyph (shape coding) on the status item specifically — the full triple-encoded
/// language (color + shape + glyph) still lives in the popover list and
/// notifications, where there's room for it. (Supersedes the older lone shape-
/// shift glyph and the one-lit-cell D1 draft.)
enum StatusItemGlyph {
    static let size: CGFloat = 18

    /// The brand grid tinted by the most-urgent state, or a quiet neutral grid
    /// when empty / only running (nothing needs you).
    /// - Parameter awake: server mode is on (the Mac is being kept awake). It is drawn
    ///   as a thin ring AROUND the existing mark rather than as a second status item:
    ///   one gtmux presence in the menu bar, in a slightly different state. A second
    ///   icon reads as a second app.
    static func image(mostUrgent: Status, empty: Bool, dark: Bool, awake: Bool = false) -> NSImage {
        let lit: Status? = (empty || mostUrgent == .running) ? nil : mostUrgent
        return draw { _, full in
            grid(full, dark: dark, lit: lit)
            if awake { awakeRing(full, dark: dark) }
        }
    }

    /// The awake ring: a hairline enclosing the mark, drawn in the 1pt margin the
    /// glyph already reserves, so it costs no extra width in the bar.
    ///
    /// Deliberately NEUTRAL, never a status colour — colour encodes the fleet's agent
    /// state (DESIGN §9) and server mode is a machine state. Shape carries this
    /// signal, exactly as the design language says it should. No animation.
    private static func awakeRing(_ full: CGRect, dark: Bool) {
        let r = full.insetBy(dx: -1, dy: -1)
        let path = NSBezierPath(roundedRect: r, xRadius: 4, yRadius: 4)
        path.lineWidth = 1
        (dark ? NSColor.white : NSColor.black).withAlphaComponent(dark ? 0.65 : 0.5).setStroke()
        path.stroke()
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
