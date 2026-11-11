import AppKit
import XCTest
@testable import GtmuxBar

final class ScreenshotAgentWorkTests: XCTestCase {
    func testASC010CancelledCallbacksRemainBoundedUntilActualCompletion() async throws {
        let admission = ScreenshotAgentAdmission(limit: 1)
        let context = ScreenshotAgentCancellation(end: ProcessInfo.processInfo.systemUptime+2)
        defer { context.finish() }
        let entered = expectation(description: "provider entered")
        let returned = expectation(description: "cancelled caller returned")
        let lock = NSLock()
        var callback: ((Result<Int, Error>) -> Void)?
        let task = Task {
            defer { returned.fulfill() }
            return try await ScreenshotAgentWork.callback(context: context, admission: admission) { done in
                lock.lock(); callback = done; lock.unlock(); entered.fulfill()
            }
        }
        await fulfillment(of: [entered], timeout: 1)
        context.cancel()
        await fulfillment(of: [returned], timeout: 0.5)
        XCTAssertEqual(admission.outstanding, 1, "caller timeout must not claim SDK completion")
        let fresh = ScreenshotAgentCancellation(end: ProcessInfo.processInfo.systemUptime+2)
        defer { fresh.finish() }
        do {
            let _: Int = try await ScreenshotAgentWork.callback(context: fresh, admission: admission) { _ in XCTFail("oversold SDK permit") }
            XCTFail("pending SDK call replaced")
        } catch { XCTAssertEqual((error as? ScreenshotAgentFailure)?.code, "native-busy") }
        let finish = lock.withLock { let value = callback; callback = nil; return value }
        try XCTUnwrap(finish)(.success(7))
        finish?(.success(8)) // a misbehaving provider cannot release twice.
        do { _ = try await task.value; XCTFail("cancelled callback accepted") }
        catch { XCTAssertEqual((error as? ScreenshotAgentFailure)?.code, "request-cancelled") }
        XCTAssertEqual(admission.outstanding, 0)
        let result: Int = try await ScreenshotAgentWork.callback(context: fresh, admission: admission) { $0(.success(9)) }
        XCTAssertEqual(result, 9); XCTAssertEqual(admission.outstanding, 0)
        var editorOpens = 0
        XCTAssertThrowsError(try context.commit { editorOpens += 1 })
        XCTAssertEqual(editorOpens, 0)
        let tightened = ScreenshotAgentCancellation(end: ProcessInfo.processInfo.systemUptime+2)
        defer { tightened.finish() }
        let expired = ProcessInfo.processInfo.systemUptime-1
        tightened.limit(to: expired)
        XCTAssertEqual(tightened.end, expired, "deadline is a synchronous authority boundary, not just a scheduled timer")
        XCTAssertThrowsError(try tightened.commit { editorOpens += 1 })
        XCTAssertEqual(editorOpens, 0)
    }

    func testASC010BackgroundWorkOwnsBoundedDuplicatesAndCannotReuseClosedDescriptors() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: directory) }
        let url = directory.appendingPathComponent("private-producer")
        let fd = open(url.path, O_CREAT|O_EXCL|O_RDWR|O_CLOEXEC, 0o600)
        XCTAssertGreaterThanOrEqual(fd, 0)
        let files = ScreenshotAgentFiles([fd], owned: true)
        defer { files.close() }
        let context = ScreenshotAgentCancellation(end: ProcessInfo.processInfo.systemUptime+3)
        defer { context.finish() }
        let entered = expectation(description: "worker holding duplicate")
        let exited = expectation(description: "worker exited")
        let release = DispatchSemaphore(value: 0)
        let lock = NSLock(); var duplicate: Int32 = -1
        let task = Task {
            try await ScreenshotAgentWork.run(context: context, files: files, indices: [0]) { descriptors, budget -> Int in
                XCTAssertFalse(Thread.isMainThread, "image and IO work must not occupy MainActor")
                lock.lock(); duplicate = descriptors[0]; lock.unlock()
                entered.fulfill()
                guard release.wait(timeout: .now()+2) == .success else { throw ScreenshotAgentFailure.refused("fixture-timeout", "release missing") }
                defer { exited.fulfill() }
                try budget.check()
                XCTFail("late canceled work proceeded to output")
                return 1
            }
        }
        await fulfillment(of: [entered], timeout: 1)
        context.cancel(); files.close()
        XCTAssertEqual(fcntl(fd, F_GETFD), -1, "original closed by its sole transport owner")
        do { _ = try await task.value; XCTFail("cancelled work returned") }
        catch { XCTAssertEqual((error as? ScreenshotAgentFailure)?.code, "request-cancelled") }
        XCTAssertThrowsError(try files.duplicate([0]), "stale task cannot duplicate a reused fd number")
        XCTAssertEqual(ScreenshotAgentWork.ioAdmission.outstanding, 1, "actual blocked work still owns its bounded permit")
        let heldFD = lock.withLock { duplicate }
        XCTAssertGreaterThanOrEqual(fcntl(heldFD, F_GETFD), 0, "in-flight work holds its own inode, not a closed caller fd")
        release.signal()
        await fulfillment(of: [exited], timeout: 1)
        // The completion follows the body's defer and closes the owned copy.
        let settled = expectation(description: "work cleanup")
        DispatchQueue.global().async {
            let end = Date().addingTimeInterval(1)
            while ScreenshotAgentWork.ioAdmission.outstanding != 0 && Date() < end { Thread.sleep(forTimeInterval: 0.001) }
            settled.fulfill()
        }
        await fulfillment(of: [settled], timeout: 2)
        XCTAssertEqual(ScreenshotAgentWork.ioAdmission.outstanding, 0)
        XCTAssertEqual(fcntl(heldFD, F_GETFD), -1)
        XCTAssertEqual(try Data(contentsOf: url), Data(), "canceled phase wrote no private producer bytes")
        let borrowedFD = open(url.path, O_RDONLY|O_CLOEXEC)
        XCTAssertGreaterThanOrEqual(borrowedFD, 0); defer { close(borrowedFD) }
        let borrowed = ScreenshotAgentFiles([borrowedFD], owned: false)
        borrowed.close()
        XCTAssertGreaterThanOrEqual(fcntl(borrowedFD, F_GETFD), 0, "direct service tests retain caller ownership")
    }
}
