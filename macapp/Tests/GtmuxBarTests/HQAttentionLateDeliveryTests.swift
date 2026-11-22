import XCTest
@testable import GtmuxBar

final class HQAttentionLateDeliveryTests: XCTestCase {
    private let a = "workspace/resolved"
    private let b = "workspace/active"
    private let prefix = "gtmux-hq-attention-"

    @MainActor func testUnknownLateSuccessRetriesCleanupAfterSameSnapshotRecovers() async throws {
        let root = try fixtureDirectory()
        defer { try? FileManager.default.removeItem(at: root) }
        let snapshot = root.appendingPathComponent("hq-workspace.attention")
        try JSONEncoder().encode([a, b]).write(to: snapshot, options: .atomic)
        let center = PublicationNotificationCenter()
        center.presented = [delivered(b)]
        let manager = fixtureManager(center, root: root, now: { Date(timeIntervalSince1970: 1000) })
        try await start(manager)

        let row: [String: Any] = ["kind": "attention", "title": "HQ", "attention_id": a, "ts": 1000]
        try JSONSerialization.data(withJSONObject: row).write(to: root.appendingPathComponent("a.json"), options: .atomic)
        manager.reconcile(waitingPanes: [])
        let posted = try XCTUnwrap(center.posted)
        XCTAssertEqual(center.requests.map(\.identifier), [prefix + a])

        let verified = try JSONEncoder().encode([b])
        try verified.write(to: snapshot, options: .atomic)
        manager.reconcile(waitingPanes: [])
        await drainMainQueue()
        XCTAssertTrue(center.removed.isEmpty, "A was not delivered at the time of the sweep")

        try Data("temporarily invalid".utf8).write(to: snapshot, options: .atomic)
        center.presented.append(delivered(a))
        posted(nil)
        await drainMainQueue()
        XCTAssertTrue(center.removed.isEmpty, "an unavailable snapshot cannot withdraw either notice")

        try verified.write(to: snapshot, options: .atomic)
        manager.reconcile(waitingPanes: [])
        await drainMainQueue()
        XCTAssertEqual(center.removed, [prefix + a])
        XCTAssertEqual(center.presented.map(\.identifier), [prefix + b])
        XCTAssertEqual(center.requests.map(\.identifier), [prefix + a], "recovery must not add again")
    }

    @MainActor func testBoundedSweepRemovesLateDeliveryWithoutOldProcessCallback() async throws {
        let root = try fixtureDirectory()
        defer { try? FileManager.default.removeItem(at: root) }
        try JSONEncoder().encode([b]).write(to: root.appendingPathComponent("hq-workspace.attention"), options: .atomic)
        let center = PublicationNotificationCenter()
        center.presented = [delivered(b)]
        var now = Date(timeIntervalSince1970: 1000)
        let manager = fixtureManager(center, root: root, now: { now })
        try await start(manager)
        center.presented.append(delivered(a))

        now = now.addingTimeInterval(59)
        manager.reconcile(waitingPanes: [])
        await drainMainQueue()
        XCTAssertTrue(center.removed.isEmpty, "unchanged inventory must not cause a busy sweep")

        now = now.addingTimeInterval(1)
        manager.reconcile(waitingPanes: [])
        await drainMainQueue()
        XCTAssertEqual(center.removed, [prefix + a])
        XCTAssertEqual(center.presented.map(\.identifier), [prefix + b])
        XCTAssertTrue(center.requests.isEmpty)
    }

    private func fixtureDirectory() throws -> URL {
        let path = FileManager.default.temporaryDirectory.appendingPathComponent("hq-late-" + UUID().uuidString)
        try FileManager.default.createDirectory(at: path, withIntermediateDirectories: true)
        return path
    }

    private func delivered(_ identity: String) -> DeliveredDesktopNotification {
        DeliveredDesktopNotification(identifier: prefix + identity, title: "HQ", body: "", pane: "")
    }

    private func fixtureManager(_ center: PublicationNotificationCenter, root: URL, now: @escaping () -> Date) -> NotificationManager {
        NotificationManager(center: center, queueDirectory: root, enabled: { true }, options: { _ in [] },
                            bundleIdentifier: "com.example.gtmux.fixture", now: now)
    }

    @MainActor private func start(_ manager: NotificationManager) async throws {
        manager.start(onJump: { _ in XCTFail("unexpected notification click") })
        for _ in 0..<200 where !manager.authorization.allowsDelivery {
            try await Task.sleep(nanoseconds: 5_000_000)
        }
        XCTAssertTrue(manager.authorization.allowsDelivery)
        await drainMainQueue()
    }

    @MainActor private func drainMainQueue() async {
        await withCheckedContinuation { continuation in
            DispatchQueue.main.async { continuation.resume() }
        }
    }
}
