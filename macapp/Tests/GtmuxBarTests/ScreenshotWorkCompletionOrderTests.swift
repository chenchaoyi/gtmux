import AppKit
import XCTest
@testable import GtmuxBar

final class ScreenshotWorkCompletionOrderTests: XCTestCase {
    private final class Observation: @unchecked Sendable {
        private let lock = NSLock()
        private var descriptor: Int32 = -1
        private var releases = 0
        func set(_ fd: Int32) { lock.withLock { descriptor = fd } }
        func release(file: StaticString = #filePath, line: UInt = #line) {
            lock.withLock {
                XCTAssertGreaterThanOrEqual(descriptor, 0, file: file, line: line)
                XCTAssertEqual(fcntl(descriptor, F_GETFD), -1, "actual duplicate must close before returning its IO permit", file: file, line: line)
                releases += 1
            }
        }
        var count: Int { lock.withLock { releases } }
    }
    func testASC013SuccessAndErrorCloseActualDuplicatesBeforeAdmissionReturn() async throws {
        for fails in [false, true] {
            let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
            try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
            defer { try? FileManager.default.removeItem(at: directory) }
            let original = open(directory.appendingPathComponent("producer").path, O_CREAT|O_EXCL|O_RDWR|O_CLOEXEC, 0o600)
            XCTAssertGreaterThanOrEqual(original, 0); defer { close(original) }
            let files = ScreenshotAgentFiles([original], owned: false); defer { files.close() }
            let context = ScreenshotAgentCancellation(end: ProcessInfo.processInfo.systemUptime+2); defer { context.finish() }
            let observation = Observation()
            let admission = ScreenshotAgentAdmission(limit: 1) { observation.release() }
            do {
                let result: Int = try await ScreenshotAgentWork.run(context: context, files: files, indices: [0], admission: admission) { descriptors, _ in
                    observation.set(descriptors[0])
                    if fails { throw ScreenshotAgentFailure.refused("fixture-error", "synthetic body failure") }
                    return 17
                }
                XCTAssertFalse(fails); XCTAssertEqual(result, 17)
            } catch {
                XCTAssertTrue(fails); XCTAssertEqual((error as? ScreenshotAgentFailure)?.code, "fixture-error")
            }
            XCTAssertEqual(observation.count, 1)
            XCTAssertEqual(admission.outstanding, 0)
            XCTAssertGreaterThanOrEqual(fcntl(original, F_GETFD), 0, "caller still owns its original descriptor")
        }
    }
}
