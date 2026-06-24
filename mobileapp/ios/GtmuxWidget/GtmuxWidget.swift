import ActivityKit
import SwiftUI
import WidgetKit

// gtmux Live Activity — the live agent tally on the lock screen + Dynamic Island.
// The status marks are drawn to match the app's StatusBadge exactly (DESIGN §1,
// triple-encoded color + shape + glyph): waiting = red rounded square + two bars,
// working = cyan circle + static loading ring, idle = green circle + check,
// running = gray circle + dot. Color encodes status ONLY.

enum AgentStatus { case waiting, working, idle, running }

private func statusColor(_ s: AgentStatus) -> Color {
  switch s {
  case .waiting: return Color(red: 0.937, green: 0.267, blue: 0.267) // #EF4444
  case .working: return Color(red: 0.024, green: 0.714, blue: 0.831) // #06B6D4
  case .idle: return Color(red: 0.133, green: 0.773, blue: 0.369) // #22C55E
  case .running: return Color(red: 0.557, green: 0.557, blue: 0.576) // #8E8E93
  }
}

// StatusBadge — a faithful SwiftUI port of the app's StatusBadge SVG.
struct StatusBadge: View {
  let status: AgentStatus
  var size: CGFloat = 18

  var body: some View {
    ZStack {
      if status == .waiting {
        RoundedRectangle(cornerRadius: size * 0.28).fill(statusColor(status))
      } else {
        Circle().fill(statusColor(status))
      }
      glyph
    }
    .frame(width: size, height: size)
  }

  @ViewBuilder private var glyph: some View {
    switch status {
    case .waiting:
      HStack(spacing: size * 0.15) {
        Capsule().fill(.white).frame(width: size * 0.11, height: size * 0.44)
        Capsule().fill(.white).frame(width: size * 0.11, height: size * 0.44)
      }
    case .working:
      Circle()
        .stroke(.white, style: StrokeStyle(lineWidth: size * 0.10, lineCap: .round, dash: [size * 0.88, size * 0.40]))
        .frame(width: size * 0.46, height: size * 0.46)
    case .idle:
      Path { p in
        p.move(to: CGPoint(x: size * 0.30, y: size * 0.52))
        p.addLine(to: CGPoint(x: size * 0.44, y: size * 0.66))
        p.addLine(to: CGPoint(x: size * 0.71, y: size * 0.35))
      }
      .stroke(.white, style: StrokeStyle(lineWidth: size * 0.11, lineCap: .round, lineJoin: .round))
      .frame(width: size, height: size)
    case .running:
      Circle().fill(.white).frame(width: size * 0.26, height: size * 0.26)
    }
  }
}

// Tally — a status badge next to its count; dimmed when zero so the layout stays
// stable without drawing attention to empty buckets.
private struct Tally: View {
  let status: AgentStatus
  let n: Int
  var badge: CGFloat = 18
  var body: some View {
    HStack(spacing: 5) {
      StatusBadge(status: status, size: badge)
      Text("\(n)").font(.system(.title3, design: .rounded)).fontWeight(.bold).foregroundColor(.white)
    }
    .opacity(n > 0 ? 1 : 0.32)
  }
}

@main
struct GtmuxWidgetBundle: WidgetBundle {
  var body: some Widget {
    if #available(iOS 16.1, *) {
      GtmuxLiveActivity()
    }
  }
}

@available(iOS 16.1, *)
struct GtmuxLiveActivity: Widget {
  var body: some WidgetConfiguration {
    ActivityConfiguration(for: GtmuxActivityAttributes.self) { context in
      // Lock-screen / banner.
      HStack(spacing: 20) {
        Tally(status: .waiting, n: context.state.waiting)
        Tally(status: .working, n: context.state.working)
        Tally(status: .idle, n: context.state.idle)
        Spacer()
        Text("gtmux").font(.caption2).foregroundColor(.white.opacity(0.5))
      }
      .padding(.horizontal, 18)
      .padding(.vertical, 14)
      .activityBackgroundTint(Color.black.opacity(0.5))
      .activitySystemActionForegroundColor(.white)
    } dynamicIsland: { context in
      DynamicIsland {
        DynamicIslandExpandedRegion(.leading) { islandTally(.waiting, context.state.waiting) }
        DynamicIslandExpandedRegion(.center) { islandTally(.working, context.state.working) }
        DynamicIslandExpandedRegion(.trailing) { islandTally(.idle, context.state.idle) }
      } compactLeading: {
        StatusBadge(status: .waiting, size: 16)
      } compactTrailing: {
        Text("\(context.state.waiting)").foregroundColor(.white).fontWeight(.semibold)
      } minimal: {
        StatusBadge(status: .waiting, size: 16)
      }
      .keylineTint(statusColor(.waiting))
    }
  }

  @ViewBuilder private func islandTally(_ s: AgentStatus, _ n: Int) -> some View {
    HStack(spacing: 4) {
      StatusBadge(status: s, size: 16)
      Text("\(n)").fontWeight(.semibold)
    }
    .opacity(n > 0 ? 1 : 0.4)
  }
}
