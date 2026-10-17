import XCTest
@testable import GtmuxBar

@MainActor final class HQConversationTailRevealTests: XCTestCase {
    func testDelayedMeasurementNeedsTwoConsecutiveVisibleSamples() async {
        var attempts = 0
        let samples = [false, true, false, true, true]
        let result = await HQConversationTailReveal.run(isCurrent: { true }, reveal: { attempts += 1 },
            isVisible: { samples[attempts - 1] }, pause: { true })
        XCTAssertTrue(result)
        XCTAssertEqual(attempts, 5, "A transient visible frame is not a settled entry")
    }
    func testUnavailableTailStopsAtTheBoundForManualRetry() async {
        var attempts = 0
        let result = await HQConversationTailReveal.run(isCurrent: { true }, reveal: { attempts += 1 },
            isVisible: { false }, pause: { true })
        XCTAssertFalse(result)
        XCTAssertEqual(attempts, 24)
    }
    func testNavigationDuringLayoutCannotRevealAnotherDestination() async {
        var current = true
        var attempts = 0
        let result = await HQConversationTailReveal.run(isCurrent: { current }, reveal: { attempts += 1 },
            isVisible: { true }, pause: { current = false; return true })
        XCTAssertFalse(result)
        XCTAssertEqual(attempts, 1)
    }
    func testCancelledWaitCannotContinueScrolling() async {
        var attempts = 0
        let result = await HQConversationTailReveal.run(isCurrent: { true }, reveal: { attempts += 1 },
            isVisible: { true }, pause: { false })
        XCTAssertFalse(result)
        XCTAssertEqual(attempts, 1)
    }
    func testTaskCancellationStopsEvenIfTheWaitCompletes() async {
        var attempts = 0
        var waiting: CheckedContinuation<Bool, Never>?
        let task = Task { @MainActor in
            await HQConversationTailReveal.run(isCurrent: { true }, reveal: { attempts += 1 },
                isVisible: { true }, pause: { await withCheckedContinuation { waiting = $0 } })
        }
        for _ in 0..<100 {
            if waiting != nil { break }
            await Task.yield()
        }
        guard let waiting else { task.cancel(); XCTFail("Layout wait never began"); return }
        task.cancel()
        waiting.resume(returning: true)
        let result = await task.value
        XCTAssertFalse(result)
        XCTAssertEqual(attempts, 1)
    }
}
