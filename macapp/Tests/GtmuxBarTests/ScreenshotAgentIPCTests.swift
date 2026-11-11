import AppKit
import XCTest
@testable import GtmuxBar

final class ScreenshotAgentIPCTests: XCTestCase {
    func testASC002ProtocolAndDescriptorsRefuseUnexpectedAuthority() throws {
        let base: [String: Any] = ["schema_version": 1, "request_id": String(repeating: "a", count: 32), "host_nonce": "fixture", "operation": "capabilities", "deadline_ms": 1]
        XCTAssertEqual(try ScreenshotAgentRequest.decode(JSONSerialization.data(withJSONObject: base)).descriptorCount, 0)
        for pair in [("schema_version", 2 as Any), ("operation", "shell" as Any), ("path", "/tmp/private" as Any)] {
            var bad = base; bad[pair.0] = pair.1
            XCTAssertThrowsError(try ScreenshotAgentRequest.decode(JSONSerialization.data(withJSONObject: bad)))
        }
        XCTAssertThrowsError(try ScreenshotAgentBudget(end: 0).check())
        XCTAssertThrowsError(try ScreenshotAgentSocket.address(String(repeating: "x", count: 200)))
        let url = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        let fd = open(url.path, O_CREAT | O_EXCL | O_RDWR, 0o600); XCTAssertGreaterThanOrEqual(fd, 0)
        defer { close(fd); try? FileManager.default.removeItem(at: url) }
        XCTAssertThrowsError(try ScreenshotAgentSocket.validateDescriptors([fd], operation: "capture", count: 2))
        XCTAssertThrowsError(try ScreenshotAgentSocket.validateDescriptors([fd, fd], operation: "capture", count: 2))
        XCTAssertThrowsError(try ScreenshotAgentSocket.validateDescriptors([fd], operation: "edit", count: 1))
        XCTAssertEqual(write(fd, [UInt8(1)], 1), 1)
        XCTAssertThrowsError(try ScreenshotAgentSocket.validateDescriptors([fd], operation: "capture", count: 1))
    }
    @MainActor func testASC002HostLeasePreservesIncumbentAndHandsOverAfterStop() throws {
        let dir = "/tmp/gsc-"+String(UUID().uuidString.prefix(8))
        defer { try? FileManager.default.removeItem(atPath: dir) }
        let service = ScreenshotAgentService { _, _ in XCTFail("not editing") }
        let first = try ScreenshotAgentSocket(directory: dir, service: service, processStart: Date())
        let second = try ScreenshotAgentSocket(directory: dir, service: service, processStart: Date())
        try first.start(); defer { first.stop(); second.stop() }
        let before = try Data(contentsOf: URL(fileURLWithPath: dir+"/host.json"))
        XCTAssertThrowsError(try second.start()) { error in XCTAssertEqual((error as? ScreenshotAgentFailure)?.code, "host-busy") }
        XCTAssertEqual(try Data(contentsOf: URL(fileURLWithPath: dir+"/host.json")), before)
        first.stop(); try second.start()
        XCTAssertNotEqual(try Data(contentsOf: URL(fileURLWithPath: dir+"/host.json")), before)
        first.stop() // stale cleanup cannot delete the new holder's endpoint.
        XCTAssertTrue(FileManager.default.fileExists(atPath: dir+"/host.json"))
    }
    @MainActor func testASC006GoConsumerUsesActualSwiftSocketAndRenderer() async throws {
        guard let root = ProcessInfo.processInfo.environment["GTMUX_SCREENSHOT_GO_SOURCE"] else { throw XCTSkip("set isolated Go source for native contract run") }
        let dir = "/tmp/gsc-"+String(UUID().uuidString.prefix(8))
        try FileManager.default.createDirectory(atPath: dir, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        defer { try? FileManager.default.removeItem(atPath: dir) }
        let service = ScreenshotAgentService { _, _ in XCTFail("contract fixture must not open editor") }
        let host = try ScreenshotAgentSocket(directory: dir, service: service, processStart: Date())
        try host.start(); defer { host.stop() }
        let output = URL(fileURLWithPath: dir+"/go.log"); FileManager.default.createFile(atPath: output.path, contents: nil)
        let handle = try FileHandle(forWritingTo: output); defer { try? handle.close() }
        let process = Process(); process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
        process.arguments = ["go", "test", "./internal/screenshot", "-run", "^TestASC006NativeInterop$", "-count=1", "-timeout=30s"]
        process.currentDirectoryURL = URL(fileURLWithPath: root)
        var env = ProcessInfo.processInfo.environment; env["GTMUX_SCREENSHOT_FIXTURE"] = dir; process.environment = env
        process.standardOutput = handle; process.standardError = handle
        // The Go timeout starts after compilation; bound this fixture's child
        // lifetime as well, without touching any shared App or process.
        let timeout = DispatchSource.makeTimerSource(queue: .global(qos: .utility))
        timeout.schedule(deadline: .now()+90)
        timeout.setEventHandler { if process.isRunning { process.terminate(); DispatchQueue.global().asyncAfter(deadline: .now()+2) { if process.isRunning { kill(process.processIdentifier, SIGKILL) } } } }
        timeout.resume(); defer { timeout.cancel() }
        let status: Int32 = try await withCheckedThrowingContinuation { continuation in
            process.terminationHandler = { p in continuation.resume(returning: p.terminationStatus) }
            do { try process.run() } catch { continuation.resume(throwing: error) }
        }
        XCTAssertEqual(status, 0, String(data: (try? Data(contentsOf: output)) ?? Data(), encoding: .utf8) ?? "")
        XCTAssertTrue(FileManager.default.fileExists(atPath: dir+"/marked.capture.json"))
    }
}
