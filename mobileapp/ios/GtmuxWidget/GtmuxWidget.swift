import ActivityKit
import SwiftUI
import WidgetKit

// gtmux Live Activity — an ambient "N waiting · M working · K idle" on the lock
// screen and in the Dynamic Island. Status colors follow the gtmux palette
// (waiting red / working cyan / idle green).

private let waitingColor = Color(red: 0.937, green: 0.267, blue: 0.267)
private let workingColor = Color(red: 0.024, green: 0.714, blue: 0.831)
private let idleColor = Color(red: 0.133, green: 0.773, blue: 0.369)

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
      // Lock-screen / banner presentation.
      HStack(spacing: 16) {
        count("⏸", context.state.waiting, waitingColor)
        count("⠿", context.state.working, workingColor)
        count("✓", context.state.idle, idleColor)
        Spacer()
        Text("gtmux").font(.caption2).foregroundColor(.secondary)
      }
      .padding(.horizontal, 18)
      .padding(.vertical, 14)
      .activityBackgroundTint(Color.black.opacity(0.45))
      .activitySystemActionForegroundColor(.white)
    } dynamicIsland: { context in
      DynamicIsland {
        DynamicIslandExpandedRegion(.leading) {
          label("⏸", context.state.waiting, waitingColor)
        }
        DynamicIslandExpandedRegion(.center) {
          label("⠿", context.state.working, workingColor)
        }
        DynamicIslandExpandedRegion(.trailing) {
          label("✓", context.state.idle, idleColor)
        }
      } compactLeading: {
        Text("⏸").foregroundColor(waitingColor)
      } compactTrailing: {
        Text("\(context.state.waiting)").foregroundColor(waitingColor).fontWeight(.bold)
      } minimal: {
        Text("\(context.state.waiting)").foregroundColor(waitingColor).fontWeight(.bold)
      }
    }
  }

  @ViewBuilder private func count(_ glyph: String, _ n: Int, _ color: Color) -> some View {
    HStack(spacing: 5) {
      Text(glyph).foregroundColor(color)
      Text("\(n)").font(.title3).fontWeight(.bold).foregroundColor(.white)
    }
  }

  @ViewBuilder private func label(_ glyph: String, _ n: Int, _ color: Color) -> some View {
    HStack(spacing: 4) {
      Text(glyph).foregroundColor(color)
      Text("\(n)").fontWeight(.semibold)
    }
    .font(.callout)
  }
}
