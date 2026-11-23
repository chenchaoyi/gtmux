import XCTest
import UserNotifications
@testable import GtmuxBar

final class PublicationNotificationCenter: DesktopNotificationCenter {
    var onListDelivered: (() -> Void)?
    var holdDeliveredRead = false
    var heldDeliveredRead: (([DeliveredDesktopNotification]) -> Void)?
    var presented: [DeliveredDesktopNotification] = []
    var requests: [UNNotificationRequest] = []
    var posted: ((Error?) -> Void)?
    var removed: [String] = []

    func setDelegate(_ delegate: UNUserNotificationCenterDelegate) {}
    func settings(_ completion: @escaping (DesktopNotificationAuthorization) -> Void) {
        completion(DesktopNotificationAuthorization(permission: .allowed))
    }
    func requestAuthorization(_ completion: @escaping (Error?) -> Void) { completion(nil) }
    func setCategories(_ categories: Set<UNNotificationCategory>) {}
    func add(_ request: UNNotificationRequest, completion: @escaping (Error?) -> Void) {
        requests.append(request)
        posted = completion
    }
    func removeDelivered(_ identifiers: [String]) {
        removed += identifiers
        presented.removeAll { identifiers.contains($0.identifier) }
    }
    func delivered(_ completion: @escaping ([DeliveredDesktopNotification]) -> Void) {
        onListDelivered?()
        if holdDeliveredRead {
            heldDeliveredRead = completion
        } else {
            completion(presented)
        }
    }
}

final class HQAttentionPublicationTests: XCTestCase {
    @MainActor func testRequestPublishedDuringDrainAndLateCallbackUseCurrentSnapshot() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("hq-publication-" + UUID().uuidString)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: root) }
        let center = PublicationNotificationCenter()
        let manager = NotificationManager(center: center, queueDirectory: root, enabled: { true }, options: { _ in [] },
                                          bundleIdentifier: "com.example.gtmux.fixture", now: { Date(timeIntervalSince1970: 1000) })
        manager.start(onJump: { _ in XCTFail("unexpected notification click") })
        await settle { manager.authorization.allowsDelivery }

        let identity = "workspace/question"
        let snapshot = root.appendingPathComponent("hq-workspace.attention")
        let request = root.appendingPathComponent("new-attention.json")
        try JSONEncoder().encode([String]()).write(to: snapshot, options: .atomic)
        center.onListDelivered = { [weak center] in
            center?.onListDelivered = nil
            do {
                try JSONEncoder().encode([identity]).write(to: snapshot, options: .atomic)
                let row: [String: Any] = ["kind": "attention", "title": "HQ", "attention_id": identity, "ts": 1000]
                try JSONSerialization.data(withJSONObject: row).write(to: request, options: .atomic)
            } catch { XCTFail("could not publish fixture: \(error)") }
        }
        manager.reconcile(waitingPanes: [])
        manager.reconcile(waitingPanes: [])
        let id = "gtmux-hq-attention-" + identity
        XCTAssertEqual(center.requests.map(\.identifier), [id])
        guard let posted = center.posted else { return }
        XCTAssertFalse(FileManager.default.fileExists(atPath: request.path))

        // No extra reconciliation call before the async system callback.
        try JSONEncoder().encode([String]()).write(to: snapshot, options: .atomic)
        posted(nil)
        await settle { center.removed.contains(id) }
    }

    @MainActor private func settle(_ condition: () -> Bool) async {
        for _ in 0..<200 {
            if condition() { return }
            try? await Task.sleep(nanoseconds: 5_000_000)
        }
        XCTFail("notification did not reach the expected state")
    }
}
