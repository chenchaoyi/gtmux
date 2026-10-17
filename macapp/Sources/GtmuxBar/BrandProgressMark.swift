import SwiftUI

/// Shared page-progress motion, distinct from an agent's working-state ring.
struct BrandProgressMark: View {
    var size: CGFloat = 36
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var pulse = false

    var body: some View {
        GtmuxLogo(size: size)
            .opacity(reduceMotion || pulse ? 1 : 0.4)
            .animation(reduceMotion ? nil : .easeInOut(duration: 0.65).repeatForever(autoreverses: true), value: pulse)
            .onAppear { pulse = true }
            .onDisappear { pulse = false }
            .accessibilityHidden(true)
    }
}
