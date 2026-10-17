import Foundation

/// A bounded explicit navigation request, independent of the source/read/dispatch stores.
@MainActor enum HQConversationTailReveal {
    static func run(
        isCurrent: () -> Bool, reveal: () -> Void, isVisible: () -> Bool,
        pause: () async -> Bool = {
            do { try await Task.sleep(nanoseconds: 50_000_000); return true }
            catch { return false }
        }
    ) async -> Bool {
        var stable = 0
        for _ in 0..<24 {
            guard !Task.isCancelled, isCurrent() else { return false }
            reveal()
            guard await pause(), !Task.isCancelled, isCurrent() else { return false }
            stable = isVisible() ? stable + 1 : 0
            if stable >= 2 { return true }
        }
        return false
    }
}
