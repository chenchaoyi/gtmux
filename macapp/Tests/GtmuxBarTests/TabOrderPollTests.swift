import Foundation
import XCTest
@testable import GtmuxBar

final class TabOrderPollTests: XCTestCase {
    func testConcurrentTicksAdmitOneRead() {
        let p = TabOrderPoll(), lock = NSLock()
        var admitted = 0
        DispatchQueue.concurrentPerform(iterations: 100) { _ in
            if p.begin(now: 0) {
                lock.lock(); admitted += 1; lock.unlock()
            }
        }
        XCTAssertEqual(admitted, 1)
        _ = p.finish(.init(outcome: .saved, reason: ""), now: 0)
        XCTAssertTrue(p.begin(now: 12))
    }

    func testFailureBackoffPreservesCountsAndRecoversOnce() {
        let p = TabOrderPoll()
        let failure = TabOrderRead(outcome: .failed, reason: "timeout")
        XCTAssertTrue(p.begin(now: 0))
        XCTAssertFalse(p.begin(now: 0))
        XCTAssertEqual(p.finish(failure, now: 0)?.failures, 1)
        XCTAssertFalse(p.begin(now: 11))
        XCTAssertTrue(p.begin(now: 12))
        XCTAssertNil(p.finish(failure, now: 12))
        XCTAssertFalse(p.begin(now: 35))
        XCTAssertTrue(p.begin(now: 36))
        XCTAssertNil(p.finish(.init(outcome: .skipped, reason: ""), now: 36))
        XCTAssertFalse(p.begin(now: 59))
        XCTAssertTrue(p.begin(now: 60))
        let recovered = p.finish(.init(outcome: .saved, reason: ""), now: 60)
        XCTAssertEqual(recovered?.failures, 2)
        XCTAssertEqual(recovered?.duration, 60)
        XCTAssertEqual(recovered?.recovered, true)
        XCTAssertTrue(p.begin(now: 72))
        XCTAssertNil(p.finish(.init(outcome: .saved, reason: ""), now: 72))
    }

    func testChangedFailureIsVisibleAndRetryIsCapped() {
        let p = TabOrderPoll()
        var now = 0.0
        var delay = 12.0
        for n in 1...12 {
            XCTAssertTrue(p.begin(now: now))
            let event = p.finish(.init(outcome: .failed, reason: n == 2 ? "storage" : "timeout"), now: now)
            if n <= 3 { XCTAssertNotNil(event, "changed causes must not be quieted") }
            if n >= 7 { XCTAssertEqual(event?.retry, 300) }
            delay = min(300, 12 * pow(2, Double(min(n - 1, 5))))
            XCTAssertFalse(p.begin(now: now + delay - 1))
            now += delay
        }
    }

    func testOnlyVerifiedSaveMayRecoverAndUnsafeReasonsAreNotLogged() {
        XCTAssertEqual(TabOrderRead.decode(status: 0, stdout: "").reason, "invalid-response")
        XCTAssertEqual(TabOrderRead.decode(status: -2, stdout: "").reason, "timeout")
        let failed = TabOrderRead.decode(status: 1, stdout: "{\"status\":\"unavailable\",\"reason\":\"private text\"}")
        XCTAssertEqual(failed.reason, "read-failed")
        let p = TabOrderPoll()
        XCTAssertTrue(p.begin(now: 0))
        _ = p.finish(failed, now: 0)
        XCTAssertTrue(p.begin(now: 12))
        XCTAssertNil(p.finish(TabOrderRead.decode(status: 0, stdout: "{\"status\":\"preserved\"}"), now: 12))
        XCTAssertTrue(p.begin(now: 24))
        XCTAssertNotNil(p.finish(TabOrderRead.decode(status: 0, stdout: "{\"status\":\"saved\"}"), now: 24))
    }
}
