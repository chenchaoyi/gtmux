import Foundation
import AppKit
import Darwin
import Security

/// A user-local, descriptor-scoped transport. It does not launch another process,
/// expose TCP, read paths in a request, or request recording permissions.
final class ScreenshotAgentSocket: @unchecked Sendable {
    struct Host: Codable {
        let schema_version: Int
        let pid: Int32
        let process_start: String
        let nonce: String
    }
    private let directory: String
    private let host: Host
    private let service: ScreenshotAgentService
    private let slots = DispatchSemaphore(value: 2)
    private let lock = NSLock()
    private var listener: Int32 = -1
    private var lease: Int32 = -1
    private var boundIdentity: (dev_t, ino_t)?

    @MainActor init(directory: String = Paths.screenshotBridgeDir, service: ScreenshotAgentService, processStart: Date? = nil) throws {
        self.directory = directory
        self.service = service
        var random = [UInt8](repeating: 0, count: 32)
        guard SecRandomCopyBytes(kSecRandomDefault, random.count, &random) == errSecSuccess else { throw ScreenshotAgentFailure.refused("host-unavailable", "Could not initialize screenshot host identity") }
        guard let birth = processStart ?? NSRunningApplication.current.launchDate else { throw ScreenshotAgentFailure.refused("host-unavailable", "Could not verify screenshot host process") }
        let formatter = ISO8601DateFormatter(); formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let start = formatter.string(from: birth)
        host = Host(schema_version: 1, pid: getpid(), process_start: start, nonce: random.map { String(format: "%02x", $0) }.joined())
    }

    func start() throws {
        try FileManager.default.createDirectory(atPath: directory, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        var info = stat()
        guard lstat(directory, &info) == 0, (info.st_mode & mode_t(S_IFMT)) == mode_t(S_IFDIR), info.st_uid == geteuid(), info.st_mode & 0o077 == 0 else { throw ScreenshotAgentFailure.refused("host-directory-invalid", "Screenshot host directory must be private and owned by this user") }
        let leasePath = directory + "/host.lock"
        let leaseFD = Darwin.open(leasePath, O_CREAT | O_RDWR | O_NOFOLLOW | O_CLOEXEC, 0o600)
        guard leaseFD >= 0 else { throw ScreenshotAgentFailure.refused("host-unavailable", "Could not open screenshot host lease") }
        guard flock(leaseFD, LOCK_EX | LOCK_NB) == 0 else { Darwin.close(leaseFD); throw ScreenshotAgentFailure.refused("host-busy", "Another screenshot host owns this endpoint") }
        lease = leaseFD
        let path = directory + "/agent.sock"
        if lstat(path, &info) == 0 {
            guard info.st_uid == geteuid(), (info.st_mode & mode_t(S_IFMT)) == mode_t(S_IFSOCK) else { stop(); throw ScreenshotAgentFailure.refused("host-endpoint-invalid", "Existing screenshot endpoint is not an owned socket") }
            // Our exclusive lease is the ownership boundary; never remove an
            // endpoint for a running compatible host that still holds its lease.
            guard unlink(path) == 0 else { stop(); throw ScreenshotAgentFailure.refused("host-unavailable", "Could not retire a stale screenshot endpoint") }
        }
        let fd = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else { stop(); throw ScreenshotAgentFailure.refused("host-unavailable", "Could not create screenshot socket") }
        _ = fcntl(fd, F_SETFD, FD_CLOEXEC)
        do {
            var address = try Self.address(path)
            let code = withUnsafePointer(to: &address) { ptr in ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) { Darwin.bind(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) } }
            guard code == 0, chmod(path, 0o600) == 0, Darwin.listen(fd, 4) == 0, lstat(path, &info) == 0 else { throw ScreenshotAgentFailure.refused("host-unavailable", "Could not bind screenshot endpoint") }
            boundIdentity = (info.st_dev, info.st_ino)
            let data = try JSONEncoder().encode(host)
            try data.write(to: URL(fileURLWithPath: directory + "/host.json"), options: .atomic)
            guard chmod(directory + "/host.json", 0o600) == 0 else { throw ScreenshotAgentFailure.refused("host-unavailable", "Could not protect screenshot host metadata") }
        } catch { Darwin.close(fd); stop(); throw error }
        lock.lock(); listener = fd; lock.unlock()
        DispatchQueue.global(qos: .utility).async { [weak self] in self?.acceptLoop(fd) }
    }

    func stop() {
        lock.lock(); let fd = listener; listener = -1; lock.unlock()
        if fd >= 0 { _ = Darwin.shutdown(fd, SHUT_RDWR); Darwin.close(fd) }
        if let identity = boundIdentity {
            var current = stat()
            let path = directory + "/agent.sock"
            if lstat(path, &current) == 0, current.st_dev == identity.0, current.st_ino == identity.1 { _ = unlink(path) }
            if let data = try? Data(contentsOf: URL(fileURLWithPath: directory + "/host.json")),
               let currentHost = try? JSONDecoder().decode(Host.self, from: data), currentHost.nonce == host.nonce { _ = unlink(directory + "/host.json") }
        }
        boundIdentity = nil
        if lease >= 0 { _ = flock(lease, LOCK_UN); Darwin.close(lease); lease = -1 }
    }

    private func acceptLoop(_ fd: Int32) {
        while true {
            lock.lock(); let live = listener == fd; lock.unlock()
            guard live else { return }
            let client = Darwin.accept(fd, nil, nil)
            if client < 0 { if errno == EINTR { continue }; return }
            guard slots.wait(timeout: .now()) == .success else { Darwin.close(client); continue }
            DispatchQueue.global(qos: .utility).async { [self] in receive(client) }
        }
    }

    private func receive(_ fd: Int32) {
        var files: [Int32] = []
        var id = ""
        let readEnd = ProcessInfo.processInfo.systemUptime+10
        do {
            _ = fcntl(fd, F_SETFD, FD_CLOEXEC)
            _ = fcntl(fd, F_SETFL, O_NONBLOCK)
            var one: Int32 = 1
            _ = setsockopt(fd, SOL_SOCKET, SO_NOSIGPIPE, &one, socklen_t(MemoryLayout<Int32>.size))
            guard Self.peerIsCurrentUser(fd) else { throw ScreenshotAgentFailure.refused("caller-unverified", "Screenshot caller is not a verified local user process") }
            let bytes = try Self.receiveFrame(fd, files: &files, end: readEnd)
            let request = try ScreenshotAgentRequest.decode(bytes)
            id = request.request_id
            guard request.host_nonce == host.nonce else { throw ScreenshotAgentFailure.refused("host-changed", "Screenshot host identity changed") }
            let budget = ScreenshotAgentBudget(end: min(readEnd, ScreenshotAgentBudget(deadlineMS: request.deadline_ms).end))
            try budget.check()
            try Self.validateDescriptors(files, operation: request.operation, count: request.descriptorCount)
            let resources = ScreenshotAgentResources(socket: fd, files: files)
            Task { @MainActor [self, resources] in
                let response: Data
                do {
                    let result = try await service.perform(request, files: resources.files, budget: budget)
                    try budget.check()
                    response = try Self.reply(id: request.request_id, result: result)
                } catch { response = Self.failure(id: request.request_id, error: error) }
                DispatchQueue.global(qos: .utility).async { [self, resources] in
                    defer { resources.close(); slots.signal() }
                    try? Self.sendFrame(response, to: resources.socket, end: budget.end)
                }
            }
        } catch {
            try? Self.sendFrame(Self.failure(id: id, error: error), to: fd, end: readEnd)
            files.forEach { Darwin.close($0) }; Darwin.close(fd); slots.signal()
        }
    }

    static func peerIsCurrentUser(_ fd: Int32) -> Bool {
        var uid: uid_t = 0, gid: gid_t = 0, pid: pid_t = 0
        var size = socklen_t(MemoryLayout<pid_t>.size)
        return getpeereid(fd, &uid, &gid) == 0 && uid == geteuid() && getsockopt(fd, SOL_LOCAL, LOCAL_PEERPID, &pid, &size) == 0 && pid > 1
    }

    static func address(_ path: String) throws -> sockaddr_un {
        var address = sockaddr_un()
        let bytes = Array(path.utf8)
        guard bytes.count < MemoryLayout.size(ofValue: address.sun_path) else { throw ScreenshotAgentFailure.refused("endpoint-too-long", "Screenshot endpoint path is too long") }
        address.sun_family = sa_family_t(AF_UNIX); address.sun_len = UInt8(MemoryLayout<sockaddr_un>.size)
        withUnsafeMutableBytes(of: &address.sun_path) { raw in raw.initializeMemory(as: UInt8.self, repeating: 0); raw.copyBytes(from: bytes) }
        return address
    }

    static func validateDescriptors(_ files: [Int32], operation: String, count: Int) throws {
        guard files.count == count else { throw ScreenshotAgentFailure.refused("descriptor-count", "Unexpected image/output descriptors") }
        let hasInput = ["import", "annotate", "edit"].contains(operation)
        var identities = Set<String>()
        for (index, fd) in files.enumerated() {
            var info = stat()
            let flags = fcntl(fd, F_GETFL)
            guard flags >= 0, fstat(fd, &info) == 0, (info.st_mode & mode_t(S_IFMT)) == mode_t(S_IFREG) else { throw ScreenshotAgentFailure.refused("descriptor-invalid", "Screenshot files must be regular file descriptors") }
            guard identities.insert("\(info.st_dev):\(info.st_ino)").inserted else { throw ScreenshotAgentFailure.refused("descriptor-invalid", "Image and output descriptors must be distinct files") }
            if hasInput && index == 0 {
                guard flags & O_ACCMODE == O_RDONLY, info.st_size > 0, info.st_size <= ScreenshotAgentRendering.maximumBytes else { throw ScreenshotAgentFailure.refused("descriptor-invalid", "Input must be a bounded read-only image descriptor") }
            } else {
                guard flags & O_ACCMODE != O_RDONLY, info.st_uid == geteuid(), info.st_size == 0, info.st_nlink == 1, info.st_mode & 0o077 == 0 else { throw ScreenshotAgentFailure.refused("descriptor-invalid", "Output must be a new private caller-owned file") }
            }
        }
    }

    private static func wait(_ fd: Int32, events: Int16, end: TimeInterval) throws {
        while true {
            let remaining = end-ProcessInfo.processInfo.systemUptime
            guard remaining > 0 else { throw ScreenshotAgentFailure.refused("deadline-exceeded", "Screenshot transport deadline expired") }
            var item = pollfd(fd: fd, events: events, revents: 0)
            let result = Darwin.poll(&item, 1, Int32(min(10000, max(1, remaining*1000))))
            if result < 0 && errno == EINTR { continue }
            guard result > 0, item.revents & events != 0 else { throw ScreenshotAgentFailure.refused("transport-unavailable", "Screenshot transport closed or timed out") }
            return
        }
    }

    static func receiveFrame(_ fd: Int32, files: inout [Int32], end: TimeInterval) throws -> Data {
        try wait(fd, events: Int16(POLLIN), end: end)
        var header = [UInt8](repeating: 0, count: 4)
        // Darwin CMSG alignment is 32-bit. Space for three descriptors and a
        // header; additional descriptors set MSG_CTRUNC and refuse the request.
        var control = [UInt8](repeating: 0, count: 24)
        var flags: Int32 = 0, controlLength = 0
        let received: Int = header.withUnsafeMutableBytes { h in
            control.withUnsafeMutableBytes { c in
                var iov = iovec(iov_base: h.baseAddress, iov_len: h.count)
                return withUnsafeMutablePointer(to: &iov) { p in
                    var message = msghdr(); message.msg_iov = p; message.msg_iovlen = 1
                    message.msg_control = c.baseAddress; message.msg_controllen = socklen_t(c.count)
                    let result = Darwin.recvmsg(fd, &message, 0)
                    flags = message.msg_flags; controlLength = Int(message.msg_controllen)
                    return result
                }
            }
        }
        if controlLength >= MemoryLayout<cmsghdr>.size {
            let cms = control.withUnsafeBytes { $0.loadUnaligned(as: cmsghdr.self) }
            if cms.cmsg_level == SOL_SOCKET && cms.cmsg_type == SCM_RIGHTS {
                let length = min(controlLength, Int(cms.cmsg_len))
                if length >= 12 { for offset in stride(from: 12, to: length-3, by: 4) {
                    let descriptor = control.withUnsafeBytes { $0.loadUnaligned(fromByteOffset: offset, as: Int32.self) }
                    _ = fcntl(descriptor, F_SETFD, FD_CLOEXEC); files.append(descriptor)
                } }
            }
        }
        guard received > 0, flags & MSG_CTRUNC == 0, files.count <= 3 else { throw ScreenshotAgentFailure.refused("transport-invalid", "Invalid or truncated screenshot descriptor frame") }
        if received < 4 {
            let rest = try read(fd, count: 4-received, end: end)
            header.replaceSubrange(received..<4, with: rest)
        }
        let size = header.reduce(0) { ($0 << 8) | Int($1) }
        guard size > 0, size <= 256 << 10 else { throw ScreenshotAgentFailure.refused("json-budget", "Screenshot request exceeds its JSON budget") }
        return try read(fd, count: size, end: end)
    }

    private static func read(_ fd: Int32, count: Int, end: TimeInterval) throws -> Data {
        var data = Data(), buffer = [UInt8](repeating: 0, count: min(64 << 10, count))
        while data.count < count {
            try wait(fd, events: Int16(POLLIN), end: end)
            let n = Darwin.read(fd, &buffer, min(buffer.count, count-data.count))
            if n < 0 && (errno == EINTR || errno == EAGAIN) { continue }
            guard n > 0 else { throw ScreenshotAgentFailure.refused("transport-closed", "Screenshot request was incomplete") }
            data.append(contentsOf: buffer.prefix(n))
        }
        return data
    }

    static func sendFrame(_ data: Data, to fd: Int32, end: TimeInterval) throws {
        guard data.count <= 256 << 10 else { throw ScreenshotAgentFailure.refused("json-budget", "Screenshot response exceeds its budget") }
        var size = UInt32(data.count).bigEndian
        var frame = withUnsafeBytes(of: &size) { Data($0) }; frame.append(data)
        try frame.withUnsafeBytes { raw in
            var offset = 0
            while offset < raw.count {
                try wait(fd, events: Int16(POLLOUT), end: end)
                let n = Darwin.write(fd, raw.baseAddress!.advanced(by: offset), min(64 << 10, raw.count-offset))
                if n < 0 && (errno == EINTR || errno == EAGAIN) { continue }
                guard n > 0 else { throw ScreenshotAgentFailure.refused("transport-closed", "Screenshot response was not confirmed") }
                offset += n
            }
        }
    }

    static func reply(id: String, result: Data) throws -> Data {
        let value = try JSONSerialization.jsonObject(with: result)
        return try JSONSerialization.data(withJSONObject: ["schema_version": 1, "request_id": id, "ok": true, "result": value])
    }
    static func failure(id: String, error: Error) -> Data {
        let problem = error as? ScreenshotAgentFailure ?? .refused("operation-failed", "The native screenshot operation failed")
        return (try? JSONSerialization.data(withJSONObject: ["schema_version": 1, "request_id": id, "ok": false,
            "error": ["code": problem.code, "message": problem.message, "retryable": false]])) ?? Data()
    }
}

private final class ScreenshotAgentResources: @unchecked Sendable {
    let socket: Int32
    let files: [Int32]
    private var closed = false
    init(socket: Int32, files: [Int32]) { self.socket = socket; self.files = files }
    func close() { guard !closed else { return }; closed = true; files.forEach { Darwin.close($0) }; Darwin.close(socket) }
    deinit { close() }
}
