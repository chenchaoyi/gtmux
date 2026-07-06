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
                Circle().trim(from: 0.08, to: 0.92)
                    .stroke(.white, style: StrokeStyle(lineWidth: size * 0.12, lineCap: .round))
                    .frame(width: size * 0.56, height: size * 0.56)
                    .rotationEffect(.degrees(-80)) // static gap; never animates
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
        let key = agent.icon.isEmpty ? "name:\(agent.agent)" : agent.icon
        if let hit = cache[key] { return hit }
        guard let img = resolve(agent) else { return nil }
        cache[key] = img
        return img
    }

    private static func resolve(_ agent: Agent) -> NSImage? {
        let fm = FileManager.default
        let hint = agent.icon
        if !hint.isEmpty, fm.fileExists(atPath: hint) {
            if hint.hasSuffix(".app") { return NSWorkspace.shared.icon(forFile: hint) }
            return NSImage(contentsOfFile: hint)
        }
        // no-config drop-in: ~/.config/gtmux/icons/<agent-key>.png
        let slug = agent.agent.lowercased().replacingOccurrences(of: " ", with: "-")
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
