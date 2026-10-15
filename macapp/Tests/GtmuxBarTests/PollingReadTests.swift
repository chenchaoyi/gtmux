import Combine
import Darwin
import Foundation
import XCTest
@testable import GtmuxBar

final class PollingReadTests: XCTestCase {
    private let snapshot = Data("[{\"pane_id\":\"%fixture\",\"agent\":\"Codex\",\"status\":\"idle\",\"task\":\"known\"}]".utf8)

    func testFailedReadRetainsFleetAndReleasesGateForRecovery() throws {
        let original = try JSONDecoder().decode([Agent].self, from: snapshot)
        let lock = NSLock()
        var fail = true
        let store = AgentStore(readPoll: { _ in
            lock.lock(); defer { lock.unlock() }
            return fail ? nil : self.snapshot
        })
        store.setAgentsForTesting(original)
        let unchanged = expectation(description: "failure does not publish an empty fleet")
        unchanged.isInverted = true
        let first = store.$agents.dropFirst().sink { _ in unchanged.fulfill() }
        store.refresh()
        wait(for: [unchanged], timeout: 0.15)
        first.cancel()
        XCTAssertEqual(store.agents.first?.task, "known")
        lock.lock(); fail = false; lock.unlock()
        let recovered = expectation(description: "the next read can publish")
        let second = store.$agents.dropFirst().sink { _ in recovered.fulfill() }
        store.refresh()
        wait(for: [recovered], timeout: 2)
        second.cancel()
        XCTAssertEqual(store.agents.first?.task, "known")
    }

    func testSlowStoreRefreshAdmitsOnlyOneCLIRead() {
        let started = expectation(description: "read started")
        let published = expectation(description: "snapshot published")
        let overlap = expectation(description: "no second CLI read while the first is held")
        overlap.isInverted = true
        let release = DispatchSemaphore(value: 0), lock = NSLock()
        var calls = 0
        let store = AgentStore(readPoll: { _ in
            lock.lock(); calls += 1; let index = calls; lock.unlock()
            if index == 1 { started.fulfill() } else { overlap.fulfill() }
            _ = release.wait(timeout: .now() + 2)
            return self.snapshot
        })
        let subscription = store.$agents.dropFirst().sink { _ in published.fulfill() }
        store.refresh()
        wait(for: [started], timeout: 1)
        for _ in 0..<20 { store.refresh() }
        wait(for: [overlap], timeout: 0.15)
        lock.lock(); let admitted = calls; lock.unlock()
        XCTAssertEqual(admitted, 1)
        release.signal()
        wait(for: [published], timeout: 2)
        subscription.cancel()
    }

    func testBoundedReadTimesOutAndNextProcessCanSucceed() {
        let slow = Process()
        slow.executableURL = URL(fileURLWithPath: "/bin/sleep")
        slow.arguments = ["10"]
        let start = Date()
        let failed = GtmuxCLI.runBounded(slow, timeout: 0.05, terminateDescendants: true)
        XCTAssertEqual(failed.status, -2)
        XCTAssertLessThan(Date().timeIntervalSince(start), 2)
        XCTAssertFalse(slow.isRunning)
        let next = Process()
        next.executableURL = URL(fileURLWithPath: "/usr/bin/printf")
        next.arguments = ["valid"]
        let result = GtmuxCLI.runBounded(next, timeout: 1, terminateDescendants: true)
        XCTAssertEqual(result.status, 0)
        XCTAssertEqual(result.stdout, "valid")
    }

    func testOutputFloodIsStoppedBeforeTheDeadline() {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/bin/sh")
        process.arguments = ["-c", "while :; do printf 'fixture output'; done"]
        let start = Date()
        let result = GtmuxCLI.runBounded(process, timeout: 5, outputLimit: 1024, terminateDescendants: true)
        XCTAssertEqual(result.status, -3)
        XCTAssertLessThan(Date().timeIntervalSince(start), 2)
        XCTAssertFalse(process.isRunning)
    }
    func testTimeoutStopsAnIgnoringDescendantToo() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: false)
        defer { try? FileManager.default.removeItem(at: dir) }
        let heartbeat = dir.appendingPathComponent("heartbeat")
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/bin/sh")
        process.arguments = ["-c", "trap '' TERM; /bin/sh -c 'trap \"\" TERM; while :; do printf x >> \"$1\"; /bin/sleep 0.03; done' probe \"$1\" & wait", "probe", heartbeat.path]
        defer { if kill(-process.processIdentifier, 0) == 0 { kill(-process.processIdentifier, SIGKILL) } }
        let result = GtmuxCLI.runBounded(process, timeout: 0.15, terminateDescendants: true)
        XCTAssertEqual(result.status, -2)
        let before = try Data(contentsOf: heartbeat)
        XCTAssertFalse(before.isEmpty)
        usleep(150_000)
        XCTAssertEqual(try Data(contentsOf: heartbeat), before, "a timed-out descendant must stop producing output")
    }

}
