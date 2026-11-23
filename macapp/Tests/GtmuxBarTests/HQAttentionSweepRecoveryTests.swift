import XCTest
@testable import GtmuxBar

final class HQAttentionSweepRecoveryTests: XCTestCase {
    @MainActor func testSameSnapshotAfterReadOutageRetriesIncompleteSweep() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("hq-sweep-" + UUID().uuidString)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: root) }
        let path = root.appendingPathComponent("hq-workspace.attention")
        let resolved = "workspace/resolved", active = "workspace/active"
        let prefix = "gtmux-hq-attention-"
        let center = PublicationNotificationCenter()
        center.presented = [resolved, active].map {
            DeliveredDesktopNotification(identifier: prefix + $0, title: "HQ fixture", body: "", pane: "")
        }
        let manager = NotificationManager(center: center, queueDirectory: root, enabled: { true }, options: { _ in [] },
                                          bundleIdentifier: "com.example.gtmux.fixture", now: { Date(timeIntervalSince1970: 1000) })
        try JSONEncoder().encode([resolved, active]).write(to: path, options: .atomic)
        manager.start(onJump: { _ in XCTFail("unexpected notification click") })
        for _ in 0..<200 where !manager.authorization.allowsDelivery {
            try await Task.sleep(nanoseconds: 5_000_000)
        }
        XCTAssertTrue(manager.authorization.allowsDelivery)
        await drainMainQueue()

        let verified = try JSONEncoder().encode([active])
        center.holdDeliveredRead = true
        try verified.write(to: path, options: .atomic)
        manager.reconcile(waitingPanes: [])
        let captured = try XCTUnwrap(center.heldDeliveredRead)
        center.heldDeliveredRead = nil
        try Data("temporarily unreadable".utf8).write(to: path, options: .atomic)
        captured(center.presented)
        await drainMainQueue()
        XCTAssertTrue(center.removed.isEmpty, "unavailable scope cannot revoke either notice")

        center.holdDeliveredRead = false
        try verified.write(to: path, options: .atomic)
        manager.reconcile(waitingPanes: [])
        await drainMainQueue()
        XCTAssertEqual(center.removed, [prefix + resolved])
        XCTAssertEqual(center.presented.map(\.identifier), [prefix + active])
        XCTAssertTrue(center.requests.isEmpty, "cleanup recovery must not post a notification")
    }

    @MainActor private func drainMainQueue() async {
        await withCheckedContinuation { continuation in
            DispatchQueue.main.async { continuation.resume() }
        }
    }
}
