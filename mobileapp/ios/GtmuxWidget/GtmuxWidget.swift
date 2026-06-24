import ActivityKit
import SwiftUI
import WidgetKit

// gtmux Live Activity — the live agent tally on the lock screen + Dynamic Island.
// Status marks match the app's StatusBadge exactly (DESIGN §1, color + shape +
// glyph): waiting = red rounded square + two bars, working = cyan circle + static
// loading ring, idle = green circle + check, running = gray circle + dot.

enum AgentStatus { case waiting, working, idle, running }

private func statusColor(_ s: AgentStatus) -> Color {
  switch s {
  case .waiting: return Color(red: 0.937, green: 0.267, blue: 0.267) // #EF4444
  case .working: return Color(red: 0.024, green: 0.714, blue: 0.831) // #06B6D4
  case .idle: return Color(red: 0.133, green: 0.773, blue: 0.369) // #22C55E
  case .running: return Color(red: 0.557, green: 0.557, blue: 0.576) // #8E8E93
  }
}

// StatusBadge — faithful SwiftUI port of the app's StatusBadge SVG.
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

// BrandMark — the gtmux app-icon motif (2×2 pane grid, top-right cell cyan).
// Replaces the plain "gtmux" wordmark in the banner's trailing corner.
struct BrandMark: View {
  var size: CGFloat = 22
  var body: some View {
    let cell = size * 0.40
    let gap = size * 0.12
    let r = cell * 0.26
    let neutral = Color.white.opacity(0.42)
    VStack(spacing: gap) {
      HStack(spacing: gap) {
        RoundedRectangle(cornerRadius: r).fill(neutral).frame(width: cell, height: cell)
        RoundedRectangle(cornerRadius: r).fill(statusColor(.working)).frame(width: cell, height: cell)
      }
      RoundedRectangle(cornerRadius: r).fill(neutral).frame(width: cell * 2 + gap, height: cell)
    }
    .frame(width: size, height: size)
  }
}

// MiniTally — the small "[badge]N · [badge]M · [badge]K" detail line; a bucket
// dims to near-zero when empty so the row stays stable and uncluttered.
private struct MiniTally: View {
  let waiting: Int, working: Int, idle: Int
  var body: some View {
    HStack(spacing: 10) {
      cell(.waiting, waiting)
      cell(.working, working)
      cell(.idle, idle)
    }
  }
  @ViewBuilder private func cell(_ s: AgentStatus, _ n: Int) -> some View {
    HStack(spacing: 4) {
      StatusBadge(status: s, size: 12)
      Text("\(n)").font(.caption2).fontWeight(.semibold).foregroundColor(.white.opacity(0.85))
    }
    .opacity(n > 0 ? 1 : 0.3)
  }
}

// headline / subtitle derive a glanceable summary: lead with WHO needs you.
private func headline(_ st: GtmuxActivityAttributes.ContentState) -> String {
  if st.waiting > 0 { return st.waitingTitle.isEmpty ? "Needs you" : st.waitingTitle }
  if st.working > 0 { return "Working" }
  return "All idle"
}
private func subtitle(_ st: GtmuxActivityAttributes.ContentState) -> String {
  if st.waiting > 1 { return "needs you · +\(st.waiting - 1) more waiting" }
  if st.waiting == 1 { return "needs your input" }
  if st.working > 0 { return "\(st.working) running · \(st.idle) idle" }
  return "nothing needs you"
}
private func primaryStatus(_ st: GtmuxActivityAttributes.ContentState) -> AgentStatus {
  if st.waiting > 0 { return .waiting }
  if st.working > 0 { return .working }
  return .idle
}

@main
struct GtmuxWidgetBundle: WidgetBundle {
  var body: some Widget {
    if #available(iOS 16.1, *) { GtmuxLiveActivity() }
  }
}

@available(iOS 16.1, *)
struct GtmuxLiveActivity: Widget {
  var body: some WidgetConfiguration {
    ActivityConfiguration(for: GtmuxActivityAttributes.self) { context in
      // Lock-screen / banner: [big badge] headline + tally ........ [brand mark]
      HStack(spacing: 12) {
        StatusBadge(status: primaryStatus(context.state), size: 30)
        VStack(alignment: .leading, spacing: 3) {
          Text(headline(context.state))
            .font(.headline).fontWeight(.bold).foregroundColor(.white)
            .lineLimit(1)
          MiniTally(waiting: context.state.waiting, working: context.state.working, idle: context.state.idle)
        }
        Spacer(minLength: 8)
        BrandMark(size: 24)
      }
      .padding(.horizontal, 16)
      .padding(.vertical, 12)
      .activityBackgroundTint(Color.black.opacity(0.55))
      .activitySystemActionForegroundColor(.white)
    } dynamicIsland: { context in
      DynamicIsland {
        DynamicIslandExpandedRegion(.leading) {
          StatusBadge(status: primaryStatus(context.state), size: 26)
        }
        DynamicIslandExpandedRegion(.trailing) {
          BrandMark(size: 22)
        }
        DynamicIslandExpandedRegion(.center) {
          VStack(spacing: 2) {
            Text(headline(context.state)).font(.callout).fontWeight(.semibold).lineLimit(1)
            Text(subtitle(context.state)).font(.caption2).foregroundColor(.secondary).lineLimit(1)
          }
        }
        DynamicIslandExpandedRegion(.bottom) {
          MiniTally(waiting: context.state.waiting, working: context.state.working, idle: context.state.idle)
        }
      } compactLeading: {
        StatusBadge(status: primaryStatus(context.state), size: 16)
      } compactTrailing: {
        Text("\(context.state.waiting > 0 ? context.state.waiting : context.state.working)")
          .foregroundColor(.white).fontWeight(.semibold)
      } minimal: {
        StatusBadge(status: primaryStatus(context.state), size: 16)
      }
      .keylineTint(statusColor(primaryStatus(context.state)))
    }
  }
}
