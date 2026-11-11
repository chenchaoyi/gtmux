import AppKit
import ScreenCaptureKit
import XCTest
@testable import GtmuxBar

final class ScreenshotAgentLifecycleTests: XCTestCase {
    private func connect(_ directory: String, operation: String, deadline: TimeInterval) throws -> Int32 {
        let data = try Data(contentsOf: URL(fileURLWithPath: directory+"/host.json"))
        let host = try JSONDecoder().decode(ScreenshotAgentSocket.Host.self, from: data)
        let fd = socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else { throw NSError(domain: "fixture-socket", code: 1) }
        _ = fcntl(fd, F_SETFD, FD_CLOEXEC)
        var noSignal: Int32 = 1
        _ = setsockopt(fd, SOL_SOCKET, SO_NOSIGPIPE, &noSignal, socklen_t(MemoryLayout<Int32>.size))
        var address = try ScreenshotAgentSocket.address(directory+"/agent.sock")
        let connected = withUnsafePointer(to: &address) { ptr in
            ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) { Darwin.connect(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
        }
        guard connected == 0 else { close(fd); throw NSError(domain: "fixture-connect", code: Int(errno)) }
        var request: [String: Any] = ["schema_version": 1, "request_id": UUID().uuidString.replacingOccurrences(of: "-", with: "").lowercased(),
            "host_nonce": host.nonce, "operation": operation, "deadline_ms": Int64(Date().addingTimeInterval(deadline).timeIntervalSince1970*1000)]
        if operation == "sources" { request["app"] = "invalid.synthetic.fixture" }
        let body = try JSONSerialization.data(withJSONObject: request)
        var size = UInt32(body.count).bigEndian
        var frame = withUnsafeBytes(of: &size) { Data($0) }; frame.append(body)
        let sent = frame.withUnsafeBytes { Darwin.write(fd, $0.baseAddress, $0.count) }
        guard sent == frame.count else { close(fd); throw NSError(domain: "fixture-write", code: 1) }
        return fd
    }

    private func response(_ fd: Int32, seconds: TimeInterval = 0.7) async -> [String: Any]? {
        await withCheckedContinuation { continuation in
            DispatchQueue.global(qos: .utility).async {
                let end = ProcessInfo.processInfo.systemUptime+seconds
                func read(_ size: Int) -> Data? {
                    var data = Data()
                    while data.count < size {
                        var item = pollfd(fd: fd, events: Int16(POLLIN), revents: 0)
                        let left = end-ProcessInfo.processInfo.systemUptime
                        guard left > 0, poll(&item, 1, Int32(max(1, left*1000))) > 0 else { return nil }
                        var bytes = [UInt8](repeating: 0, count: size-data.count)
                        let n = Darwin.read(fd, &bytes, bytes.count)
                        guard n > 0 else { return nil }
                        data.append(contentsOf: bytes.prefix(n))
                    }
                    return data
                }
                guard let header = read(4) else { continuation.resume(returning: nil); return }
                let size = header.reduce(0) { ($0 << 8) | Int($1) }
                guard size > 0, size <= 256 << 10, let body = read(size) else { continuation.resume(returning: nil); return }
                continuation.resume(returning: (try? JSONSerialization.jsonObject(with: body)) as? [String: Any])
            }
        }
    }

    @MainActor private func waitClosed(_ host: ScreenshotAgentSocket) async {
        let end = ProcessInfo.processInfo.systemUptime+0.7
        while host.activeConnectionCount > 0 && ProcessInfo.processInfo.systemUptime < end {
            try? await Task.sleep(nanoseconds: 1_000_000)
        }
        XCTAssertEqual(host.activeConnectionCount, 0, "transport must close without waiting for the SDK callback")
    }

    @MainActor func testASC010RealSocketDeadlineDisconnectAndStopCloseRequestsWithoutSDKCompletion() async throws {
        for mode in ["deadline", "disconnect", "stop"] {
            let directory = "/tmp/gsl-"+String(UUID().uuidString.prefix(8))
            let sources = ScreenshotAgentSources(); sources.recordingAllowed = { true }
            let entered = expectation(description: "SDK entered "+mode)
            var callbacks: [(Result<SCShareableContent, Error>) -> Void] = []
            sources.contentProvider = { done in callbacks.append(done); entered.fulfill() }
            let service = ScreenshotAgentService(sources: sources) { _, _ in XCTFail("late editor opened") }
            let host = try ScreenshotAgentSocket(directory: directory, service: service, processStart: Date())
            try host.start()
            var client: Int32 = -1
            defer {
                if client >= 0 { close(client) }
                host.stop()
                callbacks.forEach { $0(.failure(ScreenshotAgentFailure.refused("fixture-late", "late synthetic provider"))) }
                try? FileManager.default.removeItem(atPath: directory)
            }
            client = try connect(directory, operation: "sources", deadline: mode == "deadline" ? 0.12 : 2)
            await fulfillment(of: [entered], timeout: 1)
            XCTAssertEqual(callbacks.count, 1)
            XCTAssertEqual(ScreenshotAgentWork.sdkAdmission.outstanding, 1)
            if mode == "disconnect" { close(client); client = -1 }
            if mode == "stop" { host.stop() }
            if client >= 0 { _ = await response(client) }
            await waitClosed(host)
            XCTAssertEqual(ScreenshotAgentWork.sdkAdmission.outstanding, 1, "timeout is not SDK completion")
            // Late OS completion settles its own process-wide admission only.
            callbacks.forEach { $0(.failure(ScreenshotAgentFailure.refused("fixture-late", "late synthetic provider"))) }
            callbacks.removeAll()
            XCTAssertEqual(ScreenshotAgentWork.sdkAdmission.outstanding, 0)
        }
    }

    @MainActor func testASC010QuarantinedSDKDoesNotBlockCapabilitiesOrOversubscribeAfterHostRestart() async throws {
        let directory = "/tmp/gsl-"+String(UUID().uuidString.prefix(8))
        let sources = ScreenshotAgentSources(); sources.recordingAllowed = { true }
        let entered = expectation(description: "first provider")
        var callbacks: [(Result<SCShareableContent, Error>) -> Void] = []
        sources.contentProvider = { done in callbacks.append(done); entered.fulfill() }
        let service = ScreenshotAgentService(sources: sources) { _, _ in XCTFail("not editing") }
        let host = try ScreenshotAgentSocket(directory: directory, service: service, processStart: Date())
        try host.start()
        var clients: [Int32] = []
        defer {
            clients.forEach { close($0) }; host.stop()
            callbacks.forEach { $0(.failure(ScreenshotAgentFailure.refused("fixture-late", "late synthetic provider"))) }
            try? FileManager.default.removeItem(atPath: directory)
        }
        let first = try connect(directory, operation: "sources", deadline: 0.12); clients.append(first)
        await fulfillment(of: [entered], timeout: 1)
        _ = await response(first)
        await waitClosed(host)
        for restart in [false, true] {
            if restart { host.stop(); try host.start() }
            let control = try connect(directory, operation: "capabilities", deadline: 1); clients.append(control)
            let controlReply = await response(control)
            XCTAssertEqual(controlReply?["ok"] as? Bool, true, "unrelated work is still usable")
            let blocked = try connect(directory, operation: "sources", deadline: 1); clients.append(blocked)
            let blockedReply = await response(blocked)
            XCTAssertEqual((blockedReply?["error"] as? [String: Any])?["code"] as? String, "native-busy")
            XCTAssertEqual(callbacks.count, 1, "stop/start must not replace an actual outstanding OS call")
        }
        callbacks.forEach { $0(.failure(ScreenshotAgentFailure.refused("fixture-late", "late synthetic provider"))) }
        callbacks.removeAll()
        XCTAssertEqual(ScreenshotAgentWork.sdkAdmission.outstanding, 0)
    }
}
