import Foundation

/// A bounded explicit navigation request, independent of the source/read/dispatch stores.
@MainActor enum HQConversationTailReveal {
    static func run(
        isCurrent: () -> Bool, reveal: () -> Void, isVisible: () -> Bool,
        pause: () async -> Bool = {
            do { try await Task.sleep(nanoseconds: 50_000_000); return true }
            catch { return false }
        },
        traceAttempt: Int = 0
    ) async -> Bool {
        var stable = 0
        func ended(_ reason: String, _ iteration: Int, result: Bool = false) -> Bool {
            HQConversationTrace.record("reveal-exit", ["attempt": String(traceAttempt), "iteration": String(iteration),
                "stable": String(stable), "reason": reason])
            return result
        }
        for iteration in 0..<24 {
            guard !Task.isCancelled else { return ended("pre-cancelled", iteration) }
            guard isCurrent() else { return ended("pre-not-current", iteration) }
            reveal()
            HQConversationTrace.record("reveal-issued", ["attempt": String(traceAttempt), "iteration": String(iteration)])
            guard await pause() else { return ended("pause-false", iteration) }
            guard !Task.isCancelled else { return ended("post-cancelled", iteration) }
            guard isCurrent() else { return ended("post-not-current", iteration) }
            let visible = isVisible()
            stable = visible ? stable + 1 : 0
            HQConversationTrace.record("reveal-observed", ["attempt": String(traceAttempt), "iteration": String(iteration),
                "visible": String(visible), "stable": String(stable)])
            if stable >= 2 { return ended("stable-two", iteration, result: true) }
        }
        return ended("exhausted", 23)
    }
}
