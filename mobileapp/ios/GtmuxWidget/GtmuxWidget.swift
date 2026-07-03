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

// BrandMark — the actual gtmux app icon (from the widget's asset catalog),
// rounded like a home-screen icon. Replaces the old "gtmux" wordmark / hand-drawn
// motif in the banner's trailing corner so the activity reads as unmistakably ours.
struct BrandMark: View {
  var size: CGFloat = 22
  var body: some View {
    Image("BrandIcon")
      .resizable()
      .interpolation(.high)
      .frame(width: size, height: size)
      .clipShape(RoundedRectangle(cornerRadius: size * 0.22, style: .continuous))
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

private func itemStatus(_ s: String) -> AgentStatus {
  switch s {
  case "waiting": return .waiting
  case "working": return .working
  case "idle": return .idle
  default: return .running
  }
}

// SessionRow — one listed session: [status badge] title …… time. The new
// "concrete" line so the activity shows real sessions, not just a count.
private struct SessionRow: View {
  let item: GtmuxActivityAttributes.Item
  var body: some View {
    HStack(spacing: 8) {
      StatusBadge(status: itemStatus(item.status), size: 13)
      Text(item.title.isEmpty ? "—" : item.title)
        .font(.subheadline).foregroundColor(.white.opacity(0.92)).lineLimit(1)
      Spacer(minLength: 6)
      if item.since > 0 {
        // Relative time rendered LOCALLY by SwiftUI — auto-updates on the lock
        // screen with no push, so the server only pushes on substantive changes.
        Text(Date(timeIntervalSince1970: TimeInterval(item.since)), style: .relative)
          .font(.caption).foregroundColor(.white.opacity(0.5)).monospacedDigit()
          .lineLimit(1)
      }
    }
  }
}

// SessionList — up to maxRows session rows + a "+N more" footer.
private struct SessionList: View {
  let items: [GtmuxActivityAttributes.Item]
  let more: Int
  var maxRows: Int = 3
  var body: some View {
    VStack(alignment: .leading, spacing: 6) {
      ForEach(Array(items.prefix(maxRows).enumerated()), id: \.offset) { _, it in
        SessionRow(item: it)
      }
      if more > 0 {
        Text("+\(more) more").font(.caption).foregroundColor(.white.opacity(0.45))
      }
    }
  }
}

// summaryLine — the bold header count, e.g. "2 waiting · 3 working" (waiting first),
// "1 idle", or "No agents".
private func summaryLine(_ st: GtmuxActivityAttributes.ContentState) -> String {
  var parts: [String] = []
  if st.waiting > 0 { parts.append("\(st.waiting) waiting") }
  if st.working > 0 { parts.append("\(st.working) working") }
  if parts.isEmpty { return st.idle > 0 ? "\(st.idle) idle" : "No agents" }
  return parts.joined(separator: " · ")
}

// headline / subtitle derive a glanceable summary: lead with WHERE needs you (the
// session), then the prompt. waiting wins, then working, then idle.
private func headline(_ st: GtmuxActivityAttributes.ContentState) -> String {
  if st.waiting > 0 {
    if !st.waitingSession.isEmpty { return st.waitingSession }
    return st.waitingTitle.isEmpty ? "Needs you" : st.waitingTitle
  }
  if st.working > 0 { return "Working" }
  return "All idle"
}
private func subtitle(_ st: GtmuxActivityAttributes.ContentState) -> String {
  if st.waiting > 0 {
    // Prefer the actual prompt; append "+N more" when several wait.
    let detail = st.waitingTitle.isEmpty ? "needs your input" : st.waitingTitle
    if st.waiting > 1 { return "\(detail) · +\(st.waiting - 1) more" }
    return detail
  }
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
      // Lock-screen / banner — detailed yet calm:
      //   [badge]  session (bold)         [app icon]
      //            prompt  (dim)
      //   ───────  N waiting · M working · K idle  ───────
      VStack(alignment: .leading, spacing: 10) {
        // Which Mac this activity tracks — a small header label so a phone paired to
        // several servers knows whose tally this is (static per activity).
        if !context.attributes.server.isEmpty {
          Text(context.attributes.server)
            .font(.caption2).fontWeight(.semibold)
            .foregroundColor(.white.opacity(0.55)).lineLimit(1)
        }
        HStack(alignment: .center, spacing: 12) {
          StatusBadge(status: primaryStatus(context.state), size: 26)
          Text(summaryLine(context.state))
            .font(.headline).fontWeight(.bold).foregroundColor(.white).lineLimit(1)
          Spacer(minLength: 8)
          BrandMark(size: 24)
        }
        // The concrete part: list the top in-flight sessions. Falls back to the
        // glanceable subtitle when there's nothing to list (idle-only / old push).
        if !context.state.items.isEmpty {
          SessionList(items: context.state.items, more: context.state.more)
        } else {
          Text(subtitle(context.state))
            .font(.caption).foregroundColor(.white.opacity(0.62)).lineLimit(1)
        }
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
            if !context.attributes.server.isEmpty {
              Text(context.attributes.server).font(.caption2).fontWeight(.semibold).foregroundColor(.secondary).lineLimit(1)
            }
            Text(headline(context.state)).font(.callout).fontWeight(.semibold).lineLimit(1)
            Text(subtitle(context.state)).font(.caption2).foregroundColor(.secondary).lineLimit(1)
          }
        }
        DynamicIslandExpandedRegion(.bottom) {
          if !context.state.items.isEmpty {
            SessionList(items: context.state.items, more: context.state.more, maxRows: 2)
          } else {
            MiniTally(waiting: context.state.waiting, working: context.state.working, idle: context.state.idle)
          }
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
