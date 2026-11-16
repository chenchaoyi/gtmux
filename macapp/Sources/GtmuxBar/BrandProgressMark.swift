import SwiftUI

/// Shared page-progress motion, distinct from an agent's working-state ring.
struct BrandProgressMark: View {
    var size: CGFloat = 36
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.scenePhase) private var scenePhase
    @State private var startedAt = Date()

    var body: some View {
        Group {
            if reduceMotion || scenePhase != .active {
                BrandProgressFrame(size: size, elapsed: 0)
            } else {
                TimelineView(.animation(minimumInterval: 1.0 / 30)) { context in
                    BrandProgressFrame(size: size, elapsed: context.date.timeIntervalSince(startedAt))
                }
            }
        }
            .onAppear { startedAt = Date() }
            .onChange(of: scenePhase) { _, phase in
                if phase == .active { startedAt = Date() }
            }
            .accessibilityHidden(true)
    }
}

/// The frame used by the live view, also renderable at an exact phase for pixel QA.
struct BrandProgressFrame: View {
    var size: CGFloat
    var elapsed: TimeInterval

    static func highlights(elapsed: TimeInterval) -> [Double] {
        let phase = max(0, elapsed).truncatingRemainder(dividingBy: 1.8) / 0.6
        let step = min(2, Int(phase))
        let fraction = phase - Double(step)
        let order = [1, 2, 0] // top-right → bottom → top-left, as on App and Web.
        let fade = max(0, (fraction - 0.65) / 0.35)
        var result = [0.0, 0.0, 0.0]
        result[order[step]] = 1 - fade
        result[order[(step + 1) % 3]] = fade
        return result
    }

    var body: some View {
        let highlights = Self.highlights(elapsed: elapsed)
        let neutral = Color(hex: 0x8E8E93)
        Canvas { context, _ in
            for (index, pane) in BrandMark.loadingPanes(side: size).enumerated() {
                let path = Path(roundedRect: pane, cornerRadius: size * 0.09, style: .continuous)
                context.fill(path, with: .color(neutral.opacity(0.5)))
                context.fill(path, with: .color(Theme.Status.working.opacity(highlights[index])))
            }
        }
        .frame(width: size, height: size)
        .accessibilityHidden(true)
    }
}
