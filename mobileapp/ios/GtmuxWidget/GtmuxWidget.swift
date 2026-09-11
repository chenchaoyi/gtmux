import ActivityKit
import SwiftUI
import WidgetKit

// gtmux Live Activity — the live agent tally on the lock screen + Dynamic Island.
// Status marks match the app's StatusBadge exactly (DESIGN §1, color + shape +
// glyph): waiting = red rounded square + two bars, working = cyan circle + static
// loading ring, idle = green circle + check, running = gray circle + dot.

enum AgentStatus { case waiting, working, idle, running }

// L — the widget's half of the app's bilingual rule.
//
// Everything else a user reads is en+zh; this card was English only, so a Chinese user's
// lock screen — the one surface they see without opening anything — spoke a different
// language from the rest of the product (2026-09-11).
//
// A widget extension cannot read the app's GTMUX_LANG preference, and its own process
// follows the SYSTEM language, which is what a lock screen should follow anyway.
func zhLocale() -> Bool {
  (Locale.preferredLanguages.first ?? "en").hasPrefix("zh")
}
func L(_ en: String, _ zh: String) -> String { zhLocale() ? zh : en }

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
      // The ring is STATIC here, and deliberately so — the app's turns (DESIGN §10) but a
      // Live Activity cannot. MEASURED on device 2026-08-13 (iPhone 15 Pro Max, iOS 26.6):
      // a `.repeatForever` rotation shipped in this very view and did not run. WidgetKit
      // renders timeline snapshots; only system-driven views update between them.
      //
      // Nothing is missing, because the motion is already here in the place the platform
      // does support it: each row's elapsed time is `Text(style: .relative)` and counts up
      // on its own. Same message — this is happening, and for how long — carried by the
      // mechanism each surface actually has. Do not try to fake it by pushing updates to
      // advance a rotation: Live Activity updates are rate-limited, and spending the push
      // budget and the battery on a turning ring is a bad trade for a Lock Screen.
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

// OfflineTag — a small muted-red marker for a STALE activity (the server stopped
// refreshing it, so the tally is old). Makes a frozen card unmistakably not-live.
private struct OfflineTag: View {
  var body: some View {
    HStack(spacing: 3) {
      Circle().fill(statusColor(.waiting)).frame(width: 5, height: 5)
      Text(L("offline", "已断开")).font(.caption2).fontWeight(.semibold)
        .foregroundColor(statusColor(.waiting).opacity(0.95)).lineLimit(1)
    }
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

// tint — the card's ground, coloured by what is happening. A glance at the lock screen
// answers "does something need me" before a word is read; a flat black card made that a
// reading task. Very low alpha: this is a wash, not a fill, and the status marks stay the
// thing that CARRIES the state (DESIGN §1 — colour + shape + glyph).
private func tint(_ s: AgentStatus, stale: Bool) -> LinearGradient {
  let c = stale ? Color.white.opacity(0.02) : statusColor(s).opacity(0.15)
  return LinearGradient(
    colors: [c, stale ? Color.white.opacity(0.02) : statusColor(s).opacity(0.03), Color.white.opacity(0.02)],
    startPoint: .top, endPoint: .bottom)
}

// ServerLine — WHICH Mac this card is about, and how the fleet stands.
//
// gtmux pairs with several Macs and the phone switches between them (App.tsx keys the
// whole store by the Mac's url, so a switch ends this activity and starts a fresh one).
// Two Macs mean two cards that look alike, so the name is the card's IDENTITY, not a
// caption: it leads the row, next to a dot in the fleet's colour.
private struct ServerLine: View {
  let server: String
  let state: GtmuxActivityAttributes.ContentState
  let stale: Bool
  var showCounts: Bool = true
  var body: some View {
    HStack(spacing: 7) {
      Circle().fill(stale ? statusColor(.running) : statusColor(primaryStatus(state)))
        .frame(width: 5, height: 5)
      Text(server.isEmpty ? "gtmux" : server)
        .font(.caption).fontWeight(.semibold)
        .foregroundColor(.white.opacity(stale ? 0.45 : 0.6)).lineLimit(1)
      if stale { OfflineTag() }
      Spacer(minLength: 6)
      if showCounts && !stale {
        MiniTally(waiting: state.waiting, working: state.working, idle: state.idle)
      }
      BrandMark(size: 20)
    }
  }
}

// PrimaryBand — the one thing that matters right now, at full size.
//
// It replaces a bold COUNT ("2 waiting · 3 working"). A count cannot answer the question
// you unlock the phone to ask — which session wants me, and what does it want — and the
// answer was already in the payload, shown only in the fallback nobody reaches.
private struct PrimaryBand: View {
  let state: GtmuxActivityAttributes.ContentState
  let stale: Bool
  var body: some View {
    HStack(alignment: .center, spacing: 11) {
      StatusBadge(status: stale ? .running : primaryStatus(state), size: 26)
      VStack(alignment: .leading, spacing: 1) {
        HStack(alignment: .firstTextBaseline, spacing: 10) {
          Text(primaryTitle(state))
            .font(.system(size: 16, weight: .bold)).foregroundColor(.white)
            .lineLimit(1).minimumScaleFactor(0.85)
          Spacer(minLength: 4)
          trailing
        }
        if let d = primaryDetail(state) {
          Text(d).font(.system(size: 12.5)).foregroundColor(.white.opacity(0.7)).lineLimit(1)
        }
      }
    }
  }

  @ViewBuilder private var trailing: some View {
    if stale {
      // A stopped clock, not a running one: the card is showing an old reading, and a
      // timer still counting up would say the opposite.
      if let since = state.items.first?.since, since > 0 {
        Text(Date(timeIntervalSince1970: TimeInterval(since)), style: .relative)
          .font(.system(size: 12.5)).foregroundColor(.white.opacity(0.45)).monospacedDigit().lineLimit(1)
      }
    } else if state.waiting > 0, let since = state.items.first?.since, since > 0 {
      // Rendered locally by SwiftUI, so it moves with no push at all.
      Text(Date(timeIntervalSince1970: TimeInterval(since)), style: .timer)
        .font(.system(size: 15, weight: .bold)).foregroundColor(statusColor(.waiting))
        .monospacedDigit().lineLimit(1).frame(minWidth: 46, alignment: .trailing)
    } else if state.working > 0, let since = longestSince(state) {
      Text(Date(timeIntervalSince1970: TimeInterval(since)), style: .timer)
        .font(.system(size: 13)).foregroundColor(.white.opacity(0.5))
        .monospacedDigit().lineLimit(1).frame(minWidth: 44, alignment: .trailing)
    } else if state.idle > 0 {
      Text(L("\(state.idle) idle", "\(state.idle) 个空闲")).font(.system(size: 13)).foregroundColor(.white.opacity(0.5)).lineLimit(1)
    }
  }
}

// primaryTitle / primaryDetail — what the band says, by what is true.
private func primaryTitle(_ st: GtmuxActivityAttributes.ContentState) -> String {
  if st.waiting > 0 {
    if !st.waitingSession.isEmpty { return st.waitingSession }
    return st.waitingTitle.isEmpty ? L("Needs you", "有人等你") : st.waitingTitle
  }
  if st.working > 0 { return L("\(st.working) running", "\(st.working) 个在跑") }
  if st.idle > 0 { return L("All quiet", "都停下来了") }
  // A card that exists before its first push. "No agents" read as a verdict about the
  // fleet; this says what it is actually doing.
  return L("Waiting for your Mac…", "正在等 Mac 报到…")
}
private func primaryDetail(_ st: GtmuxActivityAttributes.ContentState) -> String? {
  if st.waiting > 0 {
    let d = st.waitingTitle.isEmpty ? L("needs your input", "在等你回话") : st.waitingTitle
    return st.waiting > 1 ? L("\(d) · +\(st.waiting - 1) more waiting", "\(d) · 另有 \(st.waiting - 1) 个在等") : d
  }
  return nil
}
private func longestSince(_ st: GtmuxActivityAttributes.ContentState) -> Int? {
  st.items.filter { $0.since > 0 }.map(\.since).min()
}
private func primaryStatus(_ st: GtmuxActivityAttributes.ContentState) -> AgentStatus {
  if st.waiting > 0 { return .waiting }
  if st.working > 0 { return .working }
  if st.idle > 0 { return .idle }
  return .running
}

// secondary lists the sessions the primary band is NOT already showing, capped at two.
// The count strip says how many there are in total, so a "+N more" line would spend a
// line of a 160pt budget repeating it.
private func secondary(_ st: GtmuxActivityAttributes.ContentState) -> [GtmuxActivityAttributes.Item] {
  let rest = st.waiting > 0 ? Array(st.items.dropFirst()) : st.items
  return Array(rest.prefix(2))
}

// isStale reads the activity's stale flag, guarded: the flag is iOS 16.2+ while this
// widget target is 16.1. On 16.1 there's no stale-date mechanism, so it's never stale.
@available(iOS 16.1, *)
private func isStale(_ context: ActivityViewContext<GtmuxActivityAttributes>) -> Bool {
  if #available(iOS 16.2, *) { return context.isStale }
  return false
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
      // Lock screen. THE HEIGHT BUDGET IS 160pt and this layout measures ~136 — every
      // line added here has to come out of another. That is why the prompt is one line,
      // the secondary list is two rows, and the remainder is left to the count strip
      // rather than spelled out (the first draft of this design measured 206 and would
      // simply have been cut off).
      VStack(alignment: .leading, spacing: 9) {
        ServerLine(server: context.attributes.server, state: context.state, stale: isStale(context))
        PrimaryBand(state: context.state, stale: isStale(context))
        if !secondary(context.state).isEmpty {
          Divider().overlay(Color.white.opacity(0.09))
          VStack(alignment: .leading, spacing: 7) {
            ForEach(Array(secondary(context.state).enumerated()), id: \.offset) { _, it in
              SessionRow(item: it)
            }
          }
        }
        if isStale(context) {
          Text(L("This is how it stood when the Mac last reported.", "这是 Mac 最后一次报告时的情况。"))
            .font(.system(size: 12.5)).foregroundColor(.white.opacity(0.5)).lineLimit(1)
        }
      }
      .opacity(isStale(context) ? 0.62 : 1)
      .padding(.horizontal, 14)
      .padding(.vertical, 12)
      .background(tint(primaryStatus(context.state), stale: isStale(context)))
      .activityBackgroundTint(Color.black.opacity(0.55))
      .activitySystemActionForegroundColor(.white)
    } dynamicIsland: { context in
      DynamicIsland {
        DynamicIslandExpandedRegion(.center) {
          ServerLine(server: context.attributes.server, state: context.state, stale: isStale(context))
            .opacity(isStale(context) ? 0.62 : 1)
        }
        DynamicIslandExpandedRegion(.bottom) {
          VStack(alignment: .leading, spacing: 9) {
            PrimaryBand(state: context.state, stale: isStale(context))
            if let first = secondary(context.state).first {
              SessionRow(item: first)
            }
          }
          .opacity(isStale(context) ? 0.62 : 1)
        }
      } compactLeading: {
        StatusBadge(status: primaryStatus(context.state), size: 16)
      } compactTrailing: {
        // Waiting: how LONG. The count is already said by the badge beside it, and "it
        // has been waiting four minutes" is the number that decides whether you reach
        // for the phone. Otherwise the count, where duration decides nothing.
        if context.state.waiting > 0, let since = context.state.items.first?.since, since > 0 {
          Text(Date(timeIntervalSince1970: TimeInterval(since)), style: .timer)
            .foregroundColor(statusColor(.waiting)).fontWeight(.bold)
            .monospacedDigit().frame(maxWidth: 54)
        } else {
          Text("\(context.state.waiting > 0 ? context.state.waiting : context.state.working)")
            .foregroundColor(.white).fontWeight(.semibold)
        }
      } minimal: {
        StatusBadge(status: primaryStatus(context.state), size: 16)
      }
      .keylineTint(statusColor(primaryStatus(context.state)))
    }
  }
}
