import SwiftUI

/// Page loading breathes; it does not use the agent-working ring. Unmounting ends it.
struct HQBrandLoading: View {
    var label: String
    var compact = false
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var pulse = false

    var body: some View {
        Group {
            if compact {
                mark
            } else {
                VStack(spacing: 14) {
                    mark
                    Text(label).font(Theme.Font.task).foregroundStyle(.secondary)
                }.padding(24)
            }
        }.accessibilityElement(children: .ignore).accessibilityLabel(label)
    }
    @ViewBuilder private var mark: some View {
        if reduceMotion {
            GtmuxLogo(size: compact ? 16 : 36).accessibilityHidden(true)
        } else {
            GtmuxLogo(size: compact ? 16 : 36)
                .opacity(pulse ? 1 : 0.4)
                .animation(.easeInOut(duration: 0.65).repeatForever(autoreverses: true), value: pulse)
                .onAppear { pulse = true }.onDisappear { pulse = false }
                .accessibilityHidden(true)
        }
    }
}
