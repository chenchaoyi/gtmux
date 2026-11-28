import XCTest
import UserNotifications
@testable import GtmuxBar

private final class FixtureNotificationCenter: DesktopNotificationCenter {
    var state = DesktopNotificationAuthorization(permission: .allowed, banners: true, sounds: true)
    var requests: [UNNotificationRequest] = []
    var completions: [(Error?) -> Void] = []
    var removed: [String] = []
    var presented: [DeliveredDesktopNotification] = []
    var holdSettings = false
    var settingsCompletion: ((DesktopNotificationAuthorization) -> Void)?
    var permissionRequests = 0
    var settingsReads = 0
    func setDelegate(_ delegate: UNUserNotificationCenterDelegate) {}
    func settings(_ completion: @escaping (DesktopNotificationAuthorization) -> Void) {
        settingsReads += 1
        if holdSettings { settingsCompletion = completion } else { completion(state) }
    }
    func requestAuthorization(_ completion: @escaping (Error?) -> Void) {
        permissionRequests += 1
        state = DesktopNotificationAuthorization(permission: .allowed, banners: true, sounds: true)
        completion(nil)
    }
    func setCategories(_ categories: Set<UNNotificationCategory>) {}
    func add(_ request: UNNotificationRequest, completion: @escaping (Error?) -> Void) {
        requests.append(request); completions.append(completion)
    }
    func removeDelivered(_ identifiers: [String]) {
        removed += identifiers
        presented.removeAll { identifiers.contains($0.identifier) }
    }
    func delivered(_ completion: @escaping ([DeliveredDesktopNotification]) -> Void) { completion(presented) }
}

final class NotificationManagerTests: XCTestCase {
    private var root: URL!
    override func setUpWithError() throws {
        root = FileManager.default.temporaryDirectory.appendingPathComponent("gtmux-notification-test-" + UUID().uuidString)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        DiagLog.dirOverride = root.appendingPathComponent("logs").path
    }
    override func tearDownWithError() throws {
        DiagLog.flush(); DiagLog.dirOverride = nil
        try FileManager.default.removeItem(at: root)
    }
    private var queue: URL { root.appendingPathComponent("queue") }
    private func manager(_ center: FixtureNotificationCenter, enabled: @escaping () -> Bool = { true }) -> NotificationManager {
        NotificationManager(center: center, queueDirectory: queue, enabled: enabled, options: { _ in [] },
                            bundleIdentifier: "com.example.gtmux.fixture", now: { Date(timeIntervalSince1970: 1000) })
    }
    private func enqueue(_ name: String, kind: String = "done", ts: Int = 1000,
                         attentionID: String = "", body: String = "Turn finished.") throws {
        try FileManager.default.createDirectory(at: queue, withIntermediateDirectories: true)
        let request: [String: Any] = ["kind": kind, "title": "Fixture", "subtitle": "%7 · Codex", "body": body,
                                      "pane": attentionID.isEmpty ? "%7" : "", "session": "fixture", "ts": ts,
                                      "attention_id": attentionID]
        try JSONSerialization.data(withJSONObject: request).write(to: queue.appendingPathComponent(name + ".json"), options: .atomic)
    }
    private func publishAttention(_ scope: String, _ keys: [String]) throws {
        try FileManager.default.createDirectory(at: queue, withIntermediateDirectories: true)
        try JSONEncoder().encode(keys).write(to: queue.appendingPathComponent("hq-\(scope).attention"), options: .atomic)
    }

    @MainActor private func settle(_ condition: () -> Bool) async {
        for _ in 0..<200 {
            if condition() { return }
            try? await Task.sleep(nanoseconds: 5_000_000)
        }
        XCTFail("asynchronous notification operation did not settle")
    }
    @MainActor private func start(_ manager: NotificationManager) async {
        manager.start(onJump: { _ in XCTFail("test unexpectedly jumped to a session") })
        await settle { manager.authorization.permission != .checking }
    }

    @MainActor func testBareOrTestHostNeverStartsTheSystemDeliveryPath() async {
        let center = FixtureNotificationCenter()
        let subject = NotificationManager(center: center, queueDirectory: queue, bundleIdentifier: nil)
        subject.start(onJump: { _ in XCTFail("jumped") })
        subject.reconcile(waitingPanes: [])
        subject.refreshAuthorization()
        subject.sendTest()
        await Task.yield()
        XCTAssertEqual(center.settingsReads, 0)
        XCTAssertTrue(center.requests.isEmpty)
        XCTAssertFalse(FileManager.default.fileExists(atPath: queue.path))
        XCTAssertEqual(subject.testState, .blocked)
    }

    @MainActor func testLaterTurnsOnTheSamePaneAreNotSilenced() async throws {
        let center = FixtureNotificationCenter()
        let subject = manager(center)
        await start(subject)
        try enqueue("first")
        subject.reconcile(waitingPanes: [])
        await settle { center.requests.count == 1 }
        center.completions[0](nil)
        await Task.yield()
        // Same second, pane, kind and copy: a separate queue event is still a new turn.
        try enqueue("second")
        subject.reconcile(waitingPanes: [])
        await settle { center.requests.count == 2 }
        XCTAssertEqual(center.requests.map(\.identifier), ["gtmux-done-%7", "gtmux-done-%7"])
        XCTAssertNil(center.requests.last?.content.sound)
        subject.reconcile(waitingPanes: [])
        XCTAssertEqual(center.requests.count, 2, "a consumed file must not replay on the next poll")
    }

    @MainActor func testAnAsynchronousFailureDoesNotPoisonTheNextTurn() async throws {
        let center = FixtureNotificationCenter(), subject = manager(center)
        await start(subject)
        try enqueue("failed")
        subject.reconcile(waitingPanes: [])
        await settle { center.requests.count == 1 }
        center.completions[0](NSError(domain: "fixture", code: 1))
        await Task.yield()
        try enqueue("next")
        subject.reconcile(waitingPanes: [])
        await settle { center.requests.count == 2 }
    }

    @MainActor func testResolvedWaitsWithdrawEvenWhenAcceptanceIsLate() async throws {
        let center = FixtureNotificationCenter(), subject = manager(center)
        await start(subject)
        subject.reconcile(waitingPanes: ["%7"])
        try enqueue("waiting", kind: "input")
        subject.reconcile(waitingPanes: ["%7"])
        await settle { center.requests.count == 1 }
        XCTAssertEqual(center.requests[0].content.interruptionLevel, .active)
        XCTAssertNotNil(center.requests[0].content.sound)
        subject.reconcile(waitingPanes: [])
        center.completions[0](nil)
        await settle { center.removed.contains("gtmux-wait-%7") }
        try enqueue("resolved", kind: "input")
        subject.reconcile(waitingPanes: [])
        XCTAssertEqual(center.requests.count, 1, "do not redisplay an already resolved prompt")
        subject.reconcile(waitingPanes: ["%7"])
        try enqueue("new-question", kind: "input")
        subject.reconcile(waitingPanes: ["%7"])
        await settle { center.requests.count == 2 }
    }

    @MainActor func testHQAttentionUpdatesItsIdentityAndWithdrawsLateAcceptance() async throws {
        let center = FixtureNotificationCenter(), subject = manager(center)
        await start(subject)
        let first = "workspace/matter-a", second = "workspace/matter-b"
        try publishAttention("workspace", [first, second])
        try enqueue("a-before", kind: "attention", attentionID: first, body: "Check the saved attempt.")
        try enqueue("b", kind: "attention", attentionID: second, body: "Answer the question.")
        subject.reconcile(waitingPanes: [])
        await settle { center.requests.count == 2 }

        try enqueue("a-after", kind: "attention", attentionID: first, body: "Answer the new question.")
        subject.reconcile(waitingPanes: [])
        await settle { center.requests.count == 3 }
        let firstID = "gtmux-hq-attention-\(first)", secondID = "gtmux-hq-attention-\(second)"
        XCTAssertEqual(center.requests.map(\.identifier), [firstID, secondID, firstID])
        XCTAssertEqual(center.requests.last?.content.body, "Answer the new question.")

        try publishAttention("workspace", [second])
        subject.reconcile(waitingPanes: [])
        for i in [0, 2] { center.completions[i](nil) }
        await settle { center.removed.contains(firstID) }
        XCTAssertFalse(center.removed.contains(secondID))
        try enqueue("a-stale", kind: "attention", attentionID: first)
        subject.reconcile(waitingPanes: [])
        XCTAssertEqual(center.requests.count, 3, "closed work must not reappear from the queued alert")
    }

    @MainActor func testHQAttentionRestartWithdrawsOwnedLegacyAndResolvedNoticesOnly() async throws {
        let center = FixtureNotificationCenter(), subject = manager(center, enabled: { false })
        let legacyDesk = UUID().uuidString, legacyClaimed = UUID().uuidString, legacyPending = UUID().uuidString
        let legacyNormal = UUID().uuidString, missing = "gtmux-hq-attention-relay/missing"
        let good = "gtmux-hq-attention-relay/active", unknown = "gtmux-hq-attention-workspace/unverified"
        center.presented = [
            .init(identifier: legacyDesk, title: "HQ workspace deadline overdue", body: "old clock", pane: ""),
            .init(identifier: legacyClaimed, title: "HQ report still needs attention", body: "gtmux relay show claimed and verify", pane: ""),
            .init(identifier: legacyPending, title: "HQ report still needs attention", body: "gtmux relay show active and verify", pane: ""),
            .init(identifier: legacyNormal, title: "Other notification", body: "must survive", pane: ""),
            .init(identifier: missing, title: "HQ report", body: "", pane: ""),
            .init(identifier: good, title: "HQ report", body: "", pane: ""),
            .init(identifier: unknown, title: "HQ workspace", body: "", pane: ""),
        ]
        try publishAttention("relay", ["relay/active"])
        try Data("not valid JSON".utf8).write(to: queue.appendingPathComponent("hq-workspace.attention"))
        await start(subject)
        await settle { Set(center.removed) == [legacyClaimed, missing] }
        try enqueue("pending-check", kind: "attention", attentionID: "workspace/unverified")
        subject.reconcile(waitingPanes: [])
        let pending = queue.appendingPathComponent("pending-check.json")
        XCTAssertTrue(FileManager.default.fileExists(atPath: pending.path))
        try publishAttention("workspace", [])
        subject.reconcile(waitingPanes: [])
        XCTAssertFalse(FileManager.default.fileExists(atPath: pending.path))
        await settle { Set(center.removed) == [legacyClaimed, missing, legacyDesk, unknown] }
        XCTAssertTrue(center.requests.isEmpty)
        XCTAssertEqual(Set(center.presented.map(\.identifier)), [legacyNormal, legacyPending, good])
    }

    @MainActor func testStartupWaitsForPermissionAndDrainsEventsFromThatGap() async throws {
        let center = FixtureNotificationCenter(), subject = manager(center)
        center.holdSettings = true
        subject.start(onJump: { _ in XCTFail("jumped") })
        try enqueue("during-permission")
        subject.reconcile(waitingPanes: [])
        XCTAssertTrue(FileManager.default.fileExists(atPath: queue.appendingPathComponent("during-permission.json").path))
        XCTAssertTrue(center.requests.isEmpty)
        center.holdSettings = false
        center.settingsCompletion?(center.state)
        await settle { center.requests.count == 1 }
    }

    @MainActor func testPermissionCanRecoverWithoutRestartingTheApp() async throws {
        let center = FixtureNotificationCenter(), subject = manager(center)
        center.state = DesktopNotificationAuthorization(permission: .denied)
        await start(subject)
        try enqueue("denied")
        subject.reconcile(waitingPanes: [])
        XCTAssertTrue(center.requests.isEmpty)
        center.state = DesktopNotificationAuthorization(permission: .allowed, banners: false, sounds: false)
        subject.refreshAuthorization()
        await settle { subject.authorization.allowsDelivery }
        XCTAssertFalse(subject.authorization.banners)
        try enqueue("allowed")
        subject.reconcile(waitingPanes: [])
        await settle { center.requests.count == 1 }
    }

    @MainActor func testPermissionRefreshKeepsTheQueueUntilTheFreshVerdict() async throws {
        let center = FixtureNotificationCenter(), subject = manager(center)
        center.state = DesktopNotificationAuthorization(permission: .denied)
        await start(subject)
        center.holdSettings = true
        subject.refreshAuthorization()
        try enqueue("permission-changing")
        subject.reconcile(waitingPanes: [])
        XCTAssertTrue(center.requests.isEmpty)
        XCTAssertTrue(FileManager.default.fileExists(atPath: queue.appendingPathComponent("permission-changing.json").path))
        center.holdSettings = false
        center.settingsCompletion?(DesktopNotificationAuthorization(permission: .allowed, banners: true, sounds: true))
        await settle { center.requests.count == 1 }
    }

    @MainActor func testAnUndecidedPermissionIsRequestedOnce() async {
        let center = FixtureNotificationCenter(), subject = manager(center)
        center.state = DesktopNotificationAuthorization(permission: .notRequested)
        await start(subject)
        XCTAssertEqual(center.permissionRequests, 1)
        subject.refreshAuthorization()
        await Task.yield()
        XCTAssertEqual(center.permissionRequests, 1)
    }

    @MainActor func testOffAndStaleRequestsDoNotPost() async throws {
        let center = FixtureNotificationCenter(), subject = manager(center, enabled: { false })
        await start(subject)
        try enqueue("off")
        try enqueue("stale", ts: 900)
        subject.reconcile(waitingPanes: [])
        XCTAssertTrue(center.requests.isEmpty)
        XCTAssertEqual(try FileManager.default.contentsOfDirectory(atPath: queue.path).count, 0)
    }

    @MainActor func testExplicitTestHasNoAgentTargetAndReportsAcceptanceOnly() async {
        let center = FixtureNotificationCenter(), subject = manager(center)
        await start(subject)
        subject.sendTest()
        await settle { center.requests.count == 1 }
        let content = center.requests[0].content
        XCTAssertEqual(content.title, "gtmux")
        XCTAssertEqual(content.userInfo["test"] as? Bool, true)
        XCTAssertNil(content.userInfo["pane"])
        XCTAssertTrue(content.categoryIdentifier.isEmpty)
        XCTAssertEqual(subject.testState, .sending)
        subject.refreshAuthorization()
        await Task.yield()
        await Task.yield()
        XCTAssertEqual(center.requests.count, 1, "permission refresh must not resubmit an in-flight test")
        center.completions[0](nil)
        await settle { subject.testState == .accepted }
    }

    @MainActor func testInputNotificationResolvesOptionsOffMainActorBeforePosting() async throws {
        let center = FixtureNotificationCenter()
        let lock = NSLock()
        var ranOffMain = false
        let subject = NotificationManager(
            center: center,
            queueDirectory: queue,
            enabled: { true },
            options: { _ in
                let offMain = !Thread.isMainThread
                lock.lock()
                ranOffMain = offMain
                lock.unlock()
                return [ReplyOption(n: 1, label: "Approve")]
            },
            bundleIdentifier: "com.example.gtmux.fixture",
            now: { Date(timeIntervalSince1970: 1000) }
        )
        await start(subject)
        subject.reconcile(waitingPanes: ["%7"])
        try enqueue("input-off-main", kind: "input")
        subject.reconcile(waitingPanes: ["%7"])
        await settle { center.requests.count == 1 }
        lock.lock()
        let verifiedOffMain = ranOffMain
        lock.unlock()
        XCTAssertTrue(verifiedOffMain, "options CLI lookup must run off the main thread")
        XCTAssertEqual(center.requests[0].content.categoryIdentifier, "GTMUX_REPLY_%7")
    }
}
