import AppKit
import SwiftUI

/// Neutral single-char monogram for an agent (DESIGN §6). Identity is shown by a
/// neutral label, NEVER a status color, and we never draw a third-party logo.
func agentMonogram(_ name: String) -> String {
    switch name {
    case "Claude Code": return "C"
    case "Codex":       return "Cx"
    case "Gemini":      return "G"
    case "Aider":       return "A"
    case "opencode":    return "oc"
    case "Crush":       return "Cr"
    case "Cursor":      return "Cu"
    case "Amp":         return "Am"
    default:
        let t = name.trimmingCharacters(in: .whitespaces)
        return t.isEmpty ? "·" : String(t.prefix(1)).uppercased()
    }
}

/// StatusBadge — the unified status mark (DESIGN §1): color + shape + glyph.
/// Square+double-bars = waiting, ring = working (static), check = idle, dot =
/// running. White glyph inside the status-colored shape.
struct StatusBadge: View {
    let status: Status
    var size: CGFloat = Theme.Size.badge
    /// Drives the working ring's rotation. Honours the system's Reduce Motion — a
    /// preference a glance tool has no business overriding.
    @State private var spin = false
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    // errored-idle: an amber ⚠ modifier replacing the green ✓ (the idle session
    // ended on an error). Never for non-idle states.
    var errored = false

    var body: some View {
        ZStack {
            base
            glyph
        }
        .frame(width: size, height: size)
    }

    @ViewBuilder private var base: some View {
        if errored {
            Circle().fill(Theme.Status.errored)
        } else if status == .waiting {
            RoundedRectangle(cornerRadius: size * 0.24, style: .continuous).fill(status.color)
        } else {
            Circle().fill(status.color)
        }
    }

    @ViewBuilder private var glyph: some View {
        if errored {
            Image(systemName: "exclamationmark")
                .font(.system(size: size * 0.56, weight: .bold)).foregroundStyle(.white)
        } else {
            switch status {
            case .waiting:
                HStack(spacing: size * 0.13) {
                    Capsule().fill(.white).frame(width: size * 0.12, height: size * 0.42)
                    Capsule().fill(.white).frame(width: size * 0.12, height: size * 0.42)
                }
            case .idle:
                Image(systemName: "checkmark")
                    .font(.system(size: size * 0.52, weight: .bold)).foregroundStyle(.white)
            case .working:
                // The ring TURNS (DESIGN §10, amended 2026-08-13). It is the one place
                // motion earns its keep: "working" is the only state that is a process
                // rather than a condition, and a still ring said so with a shape you had
                // to decode. Slow (2s) and linear, so it reads as alive rather than
                // urgent — urgency stays red, and red never moves.
                //
                // Only in surfaces the user has OPENED. The menu-bar status item stays
                // static: it is on screen all day, and motion there is ambient noise.
                Circle().trim(from: 0.08, to: 0.92)
                    .stroke(.white, style: StrokeStyle(lineWidth: size * 0.12, lineCap: .round))
                    .frame(width: size * 0.56, height: size * 0.56)
                    .rotationEffect(.degrees(spin ? 280 : -80))
                    .animation(reduceMotion ? nil : .linear(duration: 2).repeatForever(autoreverses: false),
                               value: spin)
                    .onAppear { spin = true }
            case .running:
                Circle().fill(.white).frame(width: size * 0.3, height: size * 0.3)
            }
        }
    }
}

/// AgentIcons resolves an agent's identity icon (DESIGN §6). The `icon` hint from
/// `agents --json` is either a ".app" path → that app's REAL icon (sourced from
/// the user's installed app via NSWorkspace, so no third-party logo is committed
/// to gtmux), or an image-file path. As a no-config convenience it also looks for
/// ~/.config/gtmux/icons/<agent-key>.png. Returns nil → the neutral monogram.
enum AgentIcons {
    private static var cache: [String: NSImage] = [:]

    static func image(for agent: Agent) -> NSImage? {
        image(icon: agent.icon, name: agent.agent)
    }

    /// Resolve by the raw hint + display name, so ANY surface — radar, palette, the
    /// pane browser — gets the same official icon without needing an `Agent` value.
    static func image(icon: String, name: String) -> NSImage? {
        let key = icon.isEmpty ? "name:\(name)" : icon
        if let hit = cache[key] { return hit }
        guard let img = resolve(icon: icon, name: name) else { return nil }
        cache[key] = img
        return img
    }

    private static func resolve(icon hint: String, name: String) -> NSImage? {
        let fm = FileManager.default
        if !hint.isEmpty, fm.fileExists(atPath: hint) {
            if hint.hasSuffix(".app") { return NSWorkspace.shared.icon(forFile: hint) }
            return NSImage(contentsOfFile: hint)
        }
        // no-config drop-in: ~/.config/gtmux/icons/<agent-key>.png
        let slug = name.lowercased().replacingOccurrences(of: " ", with: "-")
        let dropped = "\(NSHomeDirectory())/.config/gtmux/icons/\(slug).png"
        if fm.fileExists(atPath: dropped) { return NSImage(contentsOfFile: dropped) }
        return nil
    }
}

/// AgentAvatar — 30pt tile showing the agent's real icon when available, else a
/// neutral monogram, with the status badge overlaid bottom-right (DESIGN §3/§6).
struct AgentAvatar: View {
    let agent: Agent
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        avatar
            .frame(width: Theme.Size.avatar, height: Theme.Size.avatar)
            .overlay(alignment: .bottomTrailing) {
                StatusBadge(status: agent.state, errored: agent.errored)
                    .overlay(badgeRing)
                    .offset(x: 4, y: 4)
            }
    }

    @ViewBuilder private var avatar: some View {
        let p = Theme.Palette.of(scheme)
        if let icon = AgentIcons.image(for: agent) {
            Image(nsImage: icon)
                .resizable().interpolation(.high).scaledToFit()
                .clipShape(RoundedRectangle(cornerRadius: 7, style: .continuous))
        } else {
            RoundedRectangle(cornerRadius: 8, style: .continuous)
                .fill(scheme == .dark ? Color.white.opacity(0.09) : Color.black.opacity(0.05))
                .overlay(
                    Text(agentMonogram(agent.agent))
                        .font(.system(size: 12, weight: .semibold, design: .rounded))
                        .foregroundStyle(p.fg2))
        }
    }

    private var badgeRing: some View {
        Group {
            if agent.state == .waiting {
                RoundedRectangle(cornerRadius: Theme.Size.badge * 0.24, style: .continuous)
                    .stroke(scheme == .dark ? Color(hex: 0x1C1C1F) : Color(hex: 0xFCFCFD), lineWidth: 1.5)
            } else {
                Circle().stroke(scheme == .dark ? Color(hex: 0x1C1C1F) : Color(hex: 0xFCFCFD), lineWidth: 1.5)
            }
        }
    }
}

/// GtmuxLogo — the pane-grid mark (DESIGN §12): 2×2 grid, one cyan cell.
struct GtmuxLogo: View {
    var size: CGFloat = 16
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        let gap: CGFloat = 1.5
        let cell = (size - gap) / 2
        let neutral = scheme == .dark ? Color.white.opacity(0.32) : Color.black.opacity(0.30)
        VStack(spacing: gap) {
            HStack(spacing: gap) { tile(Theme.Status.working, cell); tile(neutral, cell) }
            HStack(spacing: gap) { tile(neutral, cell); tile(neutral, cell) }
        }
        .padding(2)
        .background(RoundedRectangle(cornerRadius: 4, style: .continuous)
            .fill(scheme == .dark ? Color.black.opacity(0.35) : Color.black.opacity(0.06)))
    }

    private func tile(_ color: Color, _ cell: CGFloat) -> some View {
        RoundedRectangle(cornerRadius: cell * 0.28, style: .continuous).fill(color).frame(width: cell, height: cell)
    }
}

/// HQMedallion — the circular HQ identity token SHARED with the mobile HQ disc
/// (MOBILE §17): the gtmux brand mark + an "HQ" wordmark inside a state RING, with a
/// corner BADGE. This is the menu-bar's answer to the phone's floating disc — the same
/// token as a fixed card avatar rather than a floating element. Ring/badge colors are
/// the authoritative status palette (DESIGN §9 / `Theme.Status`); the medallion adds NO
/// new colors. `badgeBG` is the card background it sits on, for the badge's cutout ring.
struct HQMedallion: View {
    let state: HQState
    var waitingCount: Int = 0
    var size: CGFloat = 30
    var badgeBG: Color = .clear

    var body: some View {
        Circle()
            .fill(Color.primary.opacity(0.04))
            .overlay(Circle().strokeBorder(ringColor, lineWidth: 2))
            .overlay(
                VStack(spacing: 0) {
                    GtmuxLogo(size: size * 0.42)
                    Text("HQ")
                        .font(.system(size: size * 0.24, weight: .heavy)).tracking(0.4)
                        .foregroundStyle(.primary.opacity(0.82))
                }
            )
            .frame(width: size, height: size)
            .opacity(state == .absent ? 0.5 : 1)
            .overlay(alignment: .topTrailing) { badge }
    }

    private var ringColor: Color {
        switch state {
        case .absent: return Theme.Status.none
        case .working: return Theme.Status.working
        case .normal: return Theme.Status.idle
        case .hqCall, .needsYou, .resource: return Theme.Status.waiting // red = attention
        }
    }

    // The corner badge disambiguates the red states: "!" (HQ itself) / count (a worker) /
    // "⚠" (a resource bottleneck). U+FE0E forces the monochrome glyph, matching the disc.
    private var badgeText: String? {
        switch state {
        case .hqCall: return "!"
        case .needsYou: return "\(waitingCount)"
        case .resource: return "\u{26A0}\u{FE0E}"
        default: return nil
        }
    }

    @ViewBuilder private var badge: some View {
        if let text = badgeText {
            Text(text)
                .font(.system(size: 9, weight: .heavy))
                .foregroundStyle(.white)
                .padding(.horizontal, 3)
                .frame(minWidth: 15, minHeight: 15)
                .background(Capsule().fill(Theme.Status.waiting))
                .overlay(Capsule().strokeBorder(badgeBG, lineWidth: 1.5))
                .offset(x: 5, y: -5)
        }
    }
}

/// PopoverClick decides whether an "open" is really the tail of a click that just closed
/// the panel.
///
/// A transient NSPopover is dismissed by AppKit on mouse-DOWN anywhere outside its window
/// — including on the status item that owns it. The status item's action fires on
/// mouse-UP, milliseconds later, and by then `popover.isShown` is false: the toggle reads
/// it as "closed, so open" and the panel the user just dismissed comes straight back.
/// Clicking again repeats it. The reported symptom is "clicking fast sometimes does
/// nothing, or lags" — it is not slowness, it is a close and an open cancelling out.
///
/// A separate function with no AppKit in it so the window can be reasoned about and
/// tested: too short and the echo gets through, too long and a real second click (the
/// user reopening on purpose) is swallowed. A deliberate reopen is a distinct gesture,
/// hundreds of milliseconds away; the echo is one mouse event.
enum PopoverClick {
    /// The span after a close during which an "open" is assumed to be the same click.
    static let echoWindow: TimeInterval = 0.2

    static func reopenIsAnEcho(ofCloseAt close: Date, now: Date) -> Bool {
        let dt = now.timeIntervalSince(close)
        return dt >= 0 && dt < echoWindow
    }
}
