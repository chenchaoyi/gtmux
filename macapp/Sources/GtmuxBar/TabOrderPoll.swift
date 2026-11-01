import Foundation

struct TabOrderRead {
    enum Outcome { case saved, skipped, failed }
    let outcome: Outcome
    let reason: String

    static func decode(status: Int32, stdout: String) -> TabOrderRead {
        if status == -2 { return .init(outcome: .failed, reason: "timeout") }
        if status == -1 { return .init(outcome: .failed, reason: "launch") }
        if status == -3 { return .init(outcome: .failed, reason: "output-limit") }
        guard let data = stdout.data(using: .utf8),
              let value = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let state = value["status"] as? String else {
            return .init(outcome: .failed, reason: "invalid-response")
        }
        if status == 0 && state == "saved" { return .init(outcome: .saved, reason: "") }
        if status == 0 && ["empty", "preserved"].contains(state) {
            return .init(outcome: .skipped, reason: "")
        }
        let reason = value["reason"] as? String ?? ""
        return .init(outcome: .failed, reason: ["timeout", "automation-denied", "script", "storage", "read-failed"].contains(reason) ? reason : "read-failed")
    }
}

/// One in-flight query, a bounded retry schedule, and one failure/recovery story.
/// Empty/degraded snapshots preserve the last saved order and never claim recovery.
final class TabOrderPoll {
    struct Event {
        let recovered: Bool
        let reason: String
        let failures: Int
        let duration: Int
        let retry: Int
    }
    private let lock = NSLock()
    private var running = false
    private var nextAttempt = 0.0
    private var failures = 0
    private var firstFailure = 0.0
    private var lastReport = 0.0
    private var reason = ""

    func begin(now: TimeInterval) -> Bool {
        lock.lock(); defer { lock.unlock() }
        guard !running && now >= nextAttempt else { return false }
        running = true
        return true
    }

    func finish(_ read: TabOrderRead, now: TimeInterval) -> Event? {
        lock.lock(); defer { lock.unlock() }
        running = false
        switch read.outcome {
        case .skipped:
            nextAttempt = now + (failures == 0 ? 12 : retryDelay())
            return nil
        case .saved:
            let event = failures == 0 ? nil : Event(recovered: true, reason: reason, failures: failures, duration: Int(max(0, now - firstFailure)), retry: 0)
            failures = 0
            reason = ""
            nextAttempt = now + 12
            return event
        case .failed:
            let changed = reason != read.reason
            failures += 1
            if failures == 1 { firstFailure = now }
            reason = read.reason
            let retry = retryDelay()
            nextAttempt = now + retry
            guard failures == 1 || changed || now - lastReport >= 300 else { return nil }
            lastReport = now
            return Event(recovered: false, reason: reason, failures: failures, duration: Int(max(0, now - firstFailure)), retry: Int(retry))
        }
    }

    private func retryDelay() -> TimeInterval {
        min(300, 12 * pow(2, Double(min(max(failures - 1, 0), 5))))
    }
}
