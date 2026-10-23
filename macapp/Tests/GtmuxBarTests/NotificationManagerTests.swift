import XCTest
import UserNotifications
@testable import GtmuxBar

private final class FixtureNotificationCenter: DesktopNotificationCenter {
    var state = DesktopNotificationAuthorization(permission: .allowed, banners: true, sounds: true)
    var requests: [UNNotificationRequest] = []
    var completions: [(Error?) -> Void] = []
    var removed: [String] = []
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
    func removeDelivered(_ identifiers: [String]) { removed += identifiers }
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
    private func enqueue(_ name: String, kind: String = "done", ts: Int = 1000) throws {
        try FileManager.default.createDirectory(at: queue, withIntermediateDirectories: true)
        let request: [String: Any] = ["kind": kind, "title": "Fixture", "subtitle": "%7 · Codex", "body": "Turn finished.", "pane": "%7", "session": "fixture", "ts": ts]
        try JSONSerialization.data(withJSONObject: request).write(to: queue.appendingPathComponent(name + ".json"), options: .atomic)
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
        XCTAssertEqual(center.requests.count, 1)
        center.completions[0](nil)
        await Task.yield()
        // Same second, pane, kind and copy: a separate queue event is still a new turn.
        try enqueue("second")
        subject.reconcile(waitingPanes: [])
        XCTAssertEqual(center.requests.count, 2)
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
        center.completions[0](NSError(domain: "fixture", code: 1))
        await Task.yield()
        try enqueue("next")
        subject.reconcile(waitingPanes: [])
        XCTAssertEqual(center.requests.count, 2)
    }

    @MainActor func testResolvedWaitsWithdrawEvenWhenAcceptanceIsLate() async throws {
        let center = FixtureNotificationCenter(), subject = manager(center)
        await start(subject)
        subject.reconcile(waitingPanes: ["%7"])
        try enqueue("waiting", kind: "input")
        subject.reconcile(waitingPanes: ["%7"])
        XCTAssertEqual(center.requests.count, 1)
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
        XCTAssertEqual(center.requests.count, 2)
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
        XCTAssertEqual(center.requests.count, 1)
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
}
