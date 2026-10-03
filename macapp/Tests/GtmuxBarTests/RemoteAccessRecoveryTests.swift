import XCTest
@testable import GtmuxBar

final class RemoteAccessRecoveryTests: XCTestCase {
    private final class Harness {
        private let lock = NSLock()
        private var currentAddress = "https://old.example"
        private(set) var arguments: [String] = []
        let exitStatus: Int32
        let replacement: String?

        init(exitStatus: Int32 = 0, replacement: String? = nil) {
            self.exitStatus = exitStatus
            self.replacement = replacement
        }

        func address() -> String? {
            lock.lock(); defer { lock.unlock() }
            return currentAddress
        }

        func command(_ args: [String]) -> (status: Int32, stderr: String) {
            lock.lock(); defer { lock.unlock() }
            arguments = args
            if exitStatus == 0, let replacement { currentAddress = replacement }
            return (exitStatus, exitStatus == 0 ? "" : "Network unavailable")
        }
    }

    @MainActor func testFailedRecoveryIsReportedEvenWhenAnywhereRemainsInstalled() async {
        let harness = Harness(exitStatus: 1)
        let remote = RemoteAccess(command: harness.command, state: { (.anywhere, .cloudflare) }, address: harness.address)
        let finished = expectation(description: "failed recovery")
        remote.restoreConnection { success, changed in
            XCTAssertFalse(success)
            XCTAssertFalse(changed)
            finished.fulfill()
        }
        XCTAssertTrue(remote.recovering)
        XCTAssertTrue(remote.busy)
        await fulfillment(of: [finished], timeout: 3)
        XCTAssertEqual(remote.mode, .anywhere)
        XCTAssertEqual(remote.lastError, "Network unavailable")
        XCTAssertFalse(remote.busy)
        XCTAssertFalse(remote.recovering)
    }

    @MainActor func testReplacementReturnsTheNewAddressForFreshPairing() async {
        let harness = Harness(replacement: "https://new.example")
        let remote = RemoteAccess(command: harness.command, state: { (.anywhere, .cloudflare) }, address: harness.address)
        let finished = expectation(description: "new address")
        remote.restoreConnection { success, changed in
            XCTAssertTrue(success)
            XCTAssertTrue(changed)
            finished.fulfill()
        }
        await fulfillment(of: [finished], timeout: 3)
        XCTAssertEqual(remote.url, "https://new.example")
        XCTAssertNil(remote.lastError)
        XCTAssertEqual(harness.arguments, ["tunnel", "--backend", "cloudflare", "--service", "--recover", "--yes"])
    }

    @MainActor func testInPlaceRecoveryDoesNotRequirePairingAgain() async {
        let harness = Harness()
        let remote = RemoteAccess(command: harness.command, state: { (.anywhere, .cloudflare) }, address: harness.address)
        let finished = expectation(description: "same address")
        remote.restoreConnection { success, changed in
            XCTAssertTrue(success)
            XCTAssertFalse(changed)
            finished.fulfill()
        }
        await fulfillment(of: [finished], timeout: 3)
        XCTAssertEqual(remote.url, "https://old.example")
    }
}
