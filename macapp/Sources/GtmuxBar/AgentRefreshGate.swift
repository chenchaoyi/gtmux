import Foundation

/// Allows only one agents refresh at a time, even when the polling timer fires
/// again before a slow CLI query has finished.
final class AgentRefreshGate {
    private let lock = NSLock()
    private var isEntered = false

    func enter() -> Bool {
        lock.lock()
        defer { lock.unlock() }
        guard !isEntered else { return false }
        isEntered = true
        return true
    }

    func leave() {
        lock.lock()
        defer { lock.unlock() }
        isEntered = false
    }
}
