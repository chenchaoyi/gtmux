import Foundation
import Darwin

/// Cancellation revokes request authority. It does not pretend that an OS callback
/// or a regular-file syscall can be forcibly interrupted.
final class ScreenshotAgentCancellation: @unchecked Sendable {
    private var deadline: TimeInterval
    private let lock = NSRecursiveLock()
    var end: TimeInterval { lock.lock(); defer { lock.unlock() }; return deadline }
    private var failure: ScreenshotAgentFailure?
    private var handlers: [UUID: (ScreenshotAgentFailure) -> Void] = [:]
    private var timer: DispatchSourceTimer?

    init(end: TimeInterval) {
        self.deadline = end
        let timer = DispatchSource.makeTimerSource(queue: .global(qos: .utility))
        self.timer = timer
        timer.schedule(deadline: .now()+max(0, end-ProcessInfo.processInfo.systemUptime))
        timer.setEventHandler { [weak self] in self?.cancel(.refused("deadline-exceeded", "Screenshot request deadline expired")) }
        timer.resume()
    }
    deinit { timer?.cancel() }
    func finish() { timer?.cancel() }
    func limit(to end: TimeInterval) {
        lock.lock(); defer { lock.unlock() }
        deadline = min(deadline, end)
        timer?.schedule(deadline: .now()+max(0, deadline-ProcessInfo.processInfo.systemUptime))
    }
    func cancel(_ error: ScreenshotAgentFailure = .refused("request-cancelled", "Screenshot request was cancelled")) {
        lock.lock()
        guard failure == nil else { lock.unlock(); return }
        failure = error
        let callbacks = Array(handlers.values); handlers.removeAll()
        lock.unlock()
        timer?.cancel()
        callbacks.forEach { $0(error) }
    }
    func check() throws {
        if ProcessInfo.processInfo.systemUptime >= end { cancel(.refused("deadline-exceeded", "Screenshot request deadline expired")) }
        lock.lock(); let error = failure; lock.unlock()
        if let error { throw error }
    }
    /// Keep cancellation and the editor's irreversible commit point ordered.
    /// Once the action begins, cancellation cannot retroactively undo a window.
    func commit<T>(_ action: () throws -> T) throws -> T {
        lock.lock(); defer { lock.unlock() }
        try check()
        return try action()
    }
    func onCancel(_ callback: @escaping (ScreenshotAgentFailure) -> Void) -> () -> Void {
        let id = UUID()
        lock.lock(); let error = failure
        if error == nil { handlers[id] = callback }
        lock.unlock()
        if let error { callback(error) }
        return { [weak self] in
            guard let self else { return }
            self.lock.lock(); self.handlers.removeValue(forKey: id); self.lock.unlock()
        }
    }
    var budget: ScreenshotAgentBudget { ScreenshotAgentBudget(end: end, cancellation: self) }
}

/// A bounded admission permit is released by actual completion, never by a
/// caller timing out. Thus uncancellable work cannot create a replacement queue.
final class ScreenshotAgentAdmission: @unchecked Sendable {
    private let lock = NSLock()
    private let limit: Int
    private var count = 0
    init(limit: Int) { self.limit = limit }
    func take() -> Bool {
        lock.lock(); defer { lock.unlock() }
        guard count < limit else { return false }
        count += 1; return true
    }
    func release() { lock.lock(); count -= 1; lock.unlock() }
    var outstanding: Int { lock.lock(); defer { lock.unlock() }; return count }
}

private final class ScreenshotAgentCompletion<Value>: @unchecked Sendable {
    private let lock = NSLock()
    private var continuation: CheckedContinuation<Value, Error>?
    private var cleanup: (() -> Void)?
    private var resolved = false
    init(_ continuation: CheckedContinuation<Value, Error>) { self.continuation = continuation }
    func install(_ cleanup: @escaping () -> Void) {
        lock.lock(); let done = resolved
        if !done { self.cleanup = cleanup }
        lock.unlock()
        if done { cleanup() }
    }
    func resolve(_ value: Result<Value, Error>) {
        lock.lock()
        guard !resolved else { lock.unlock(); return }
        resolved = true
        let target = continuation, cleanup = self.cleanup
        continuation = nil; self.cleanup = nil
        lock.unlock()
        cleanup?(); target?.resume(with: value)
    }
}

/// Original request descriptors are closed by the transport owner. Background
/// work owns separate duplicates acquired under this lock; stale tasks can never
/// use a descriptor number after it has been closed and reused by the process.
final class ScreenshotAgentFiles: @unchecked Sendable {
    let count: Int
    private let lock = NSLock()
    private let owned: Bool
    private var files: [Int32]?
    init(_ files: [Int32], owned: Bool) { self.files = files; self.count = files.count; self.owned = owned }
    func duplicate(_ indices: [Int]) throws -> ScreenshotAgentFiles {
        lock.lock(); defer { lock.unlock() }
        guard let files, indices.allSatisfy({ files.indices.contains($0) }) else {
            throw ScreenshotAgentFailure.refused("request-cancelled", "Screenshot descriptors are no longer available")
        }
        var result: [Int32] = []
        for index in indices {
            let fd = fcntl(files[index], F_DUPFD_CLOEXEC, 0)
            guard fd >= 0 else {
                result.forEach { Darwin.close($0) }
                throw ScreenshotAgentFailure.refused("descriptor-unavailable", "Could not acquire screenshot work descriptors")
            }
            result.append(fd)
        }
        return ScreenshotAgentFiles(result, owned: true)
    }
    func values() throws -> [Int32] {
        lock.lock(); defer { lock.unlock() }
        guard let files else { throw ScreenshotAgentFailure.refused("request-cancelled", "Screenshot descriptors were closed") }
        return files
    }
    func close() {
        lock.lock(); let current = files; files = nil
        if owned { current?.forEach { Darwin.close($0) } }
        lock.unlock()
    }
    deinit { close() }
}

enum ScreenshotAgentWork {
    // Process-wide, including a host stop/start in the same process. SDK calls
    // have no cancellation API; one outstanding callback is quarantined until it
    // really arrives, with no socket, image output descriptor or editor closure.
    static let sdkAdmission = ScreenshotAgentAdmission(limit: 1)
    static let ioAdmission = ScreenshotAgentAdmission(limit: 2)

    static func callback<Value>(context: ScreenshotAgentCancellation, admission: ScreenshotAgentAdmission,
                                begin: (@escaping (Result<Value, Error>) -> Void) -> Void) async throws -> Value {
        try context.check()
        return try await withCheckedThrowingContinuation { continuation in
            let completion = ScreenshotAgentCompletion(continuation)
            completion.install(context.onCancel { completion.resolve(.failure($0)) })
            do { try context.check() } catch { completion.resolve(.failure(error)); return }
            guard admission.take() else {
                completion.resolve(.failure(ScreenshotAgentFailure.refused("native-busy", "An earlier native operation is still completing")))
                return
            }
            do { try context.check() } catch { admission.release(); completion.resolve(.failure(error)); return }
            let once = NSLock()
            var finished = false
            begin { value in
                once.lock()
                guard !finished else { once.unlock(); return }
                finished = true; once.unlock()
                admission.release()
                // Resolve only once. A canceled caller discards this late value.
                completion.resolve(value)
            }
        }
    }

    static func run<Value>(context: ScreenshotAgentCancellation, files: ScreenshotAgentFiles, indices: [Int],
                           body: @escaping ([Int32], ScreenshotAgentBudget) throws -> Value) async throws -> Value {
        try await callback(context: context, admission: ioAdmission) { done in
            DispatchQueue.global(qos: .utility).async {
                do {
                    try context.check()
                    let owned = try files.duplicate(indices); defer { owned.close() }
                    let value = try body(owned.values(), context.budget)
                    try context.check()
                    done(.success(value))
                } catch { done(.failure(error)) }
            }
        }
    }
}
