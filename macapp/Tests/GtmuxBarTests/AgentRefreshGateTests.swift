import Foundation
import XCTest
@testable import GtmuxBar

final class AgentRefreshGateTests: XCTestCase {
    func testOnlyOneConcurrentRefreshCanEnter() {
        let gate = AgentRefreshGate()
        let countLock = NSLock()
        var entered = 0

        DispatchQueue.concurrentPerform(iterations: 100) { _ in
            guard gate.enter() else { return }
            countLock.lock()
            entered += 1
            countLock.unlock()
        }

        XCTAssertEqual(entered, 1)
        gate.leave()
        XCTAssertTrue(gate.enter())
        gate.leave()
    }
}
