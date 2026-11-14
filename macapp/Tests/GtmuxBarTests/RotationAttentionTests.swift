import XCTest
import UserNotifications
@testable import GtmuxBar

private final class AttentionCenter: DesktopNotificationCenter {
    var state = DesktopNotificationAuthorization(permission: .allowed, banners: true, sounds: true)
    var requests: [UNNotificationRequest] = []
    var permissionRequests = 0
    func setDelegate(_ delegate: UNUserNotificationCenterDelegate) {}
    func settings(_ completion: @escaping (DesktopNotificationAuthorization) -> Void) { completion(state) }
    func requestAuthorization(_ completion: @escaping (Error?) -> Void) { permissionRequests += 1; completion(nil) }
    func setCategories(_ categories: Set<UNNotificationCategory>) {}
    func add(_ request: UNNotificationRequest, completion: @escaping (Error?) -> Void) { requests.append(request); completion(nil) }
    func removeDelivered(_ identifiers: [String]) {}
}

final class RotationAttentionTests: XCTestCase {
    // The targeted Go→Swift run supplies actual failRotation JSON here. Ordinary
    // Swift CI uses the same explicit wire contract, not a claim of Go execution.
    private func request() throws -> [String: Any] {
        if let path = ProcessInfo.processInfo.environment["GTMUX_TEST_ROTATION_NOTIFICATION"] {
            let data = try Data(contentsOf: URL(fileURLWithPath: path))
            return try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        }
        return ["kind": "attention", "title": "HQ handoff did not complete", "subtitle": "gtmux",
                "body": "no safe idle input box within 30 minutes", "pane": "%6", "session": "gtmux HQ", "ts": 1000]
    }

    @MainActor func testHRW011AttentionUsesActualConsumerWithoutWaitingAndRespectsGuards() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("gtmux-attention-" + UUID().uuidString)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: root) }
        let original = try request()
        XCTAssertEqual(original["kind"] as? String, "attention", "producer must not label failure as waiting or done")
        let ts = try XCTUnwrap(original["ts"] as? Int)
        for mode in ["attention", "off", "denied", "stale", "bare", "resolved-input", "waiting-input"] {
            let center = AttentionCenter()
            if mode == "denied" { center.state.permission = .denied }
            let queue = root.appendingPathComponent(mode)
            var optionReads = 0
            let subject = NotificationManager(center: center, queueDirectory: queue, enabled: { mode != "off" },
                options: { _ in optionReads += 1; return [] }, bundleIdentifier: mode == "bare" ? nil : "fixture.gtmux.attention",
                now: { Date(timeIntervalSince1970: Double(ts)) })
            subject.start(onJump: { _ in XCTFail("unexpected navigation") })
            if mode != "bare" {
                for _ in 0..<200 where subject.authorization.permission == .checking { try await Task.sleep(nanoseconds: 5_000_000) }
                XCTAssertNotEqual(subject.authorization.permission, .checking)
            }
            let waiting: Set<String> = mode == "waiting-input" ? ["%6"] : []
            subject.reconcile(waitingPanes: waiting)
            try FileManager.default.createDirectory(at: queue, withIntermediateDirectories: true)
            var req = original
            if mode == "stale" { req["ts"] = ts - 31 }
            if mode.hasSuffix("input") { req["kind"] = "input" }
            let path = queue.appendingPathComponent("failure.json")
            try JSONSerialization.data(withJSONObject: req).write(to: path, options: .atomic)
            subject.reconcile(waitingPanes: waiting)
            subject.reconcile(waitingPanes: waiting)
            for _ in 0..<4 { await Task.yield() }
            let shouldPost = mode == "attention" || mode == "waiting-input"
            XCTAssertEqual(center.requests.count, shouldPost ? 1 : 0, mode)
            XCTAssertEqual(center.permissionRequests, 0, "must not request system permission")
            if mode == "attention" {
                let posted = try XCTUnwrap(center.requests.first)
                XCTAssertEqual(posted.identifier, "gtmux-attention-%6")
                XCTAssertEqual(posted.content.categoryIdentifier, "GTMUX_DONE", "Jump-only category, no reply buttons")
                XCTAssertEqual(posted.content.interruptionLevel, .active)
                XCTAssertNotNil(posted.content.sound)
                XCTAssertEqual(posted.content.body, original["body"] as? String)
                XCTAssertEqual(optionReads, 0)
                XCTAssertFalse(FileManager.default.fileExists(atPath: path.path))
            }
        }
    }
}
