import Foundation

/// Periodic readers share serve's existing radar when available. Non-radar reads
/// and a missing/failed local service keep the same standalone CLI behavior.
final class RadarSnapshotReader: @unchecked Sendable {
    static let shared = RadarSnapshotReader(readHTTP: localSnapshot,
        readCLI: { GtmuxCLI.capturePoll($0) })
    private let readHTTP: () -> Data?
    private let readCLI: ([String]) -> Data?
    private let now: () -> Date
    private let lock = NSLock()
    private var retryAfter = Date.distantPast

    init(readHTTP: @escaping () -> Data?, readCLI: @escaping ([String]) -> Data?,
         now: @escaping () -> Date = Date.init) {
        self.readHTTP = readHTTP
        self.readCLI = readCLI
        self.now = now
    }

    func capture(_ args: [String]) -> Data? {
        guard args == ["agents", "--json"] else { return readCLI(args) }
        lock.lock()
        let mayTry = now() >= retryAfter
        lock.unlock()
        if mayTry {
            if let data = readHTTP(), (try? JSONDecoder().decode([Agent].self, from: data)) != nil {
                return data
            }
            lock.lock()
            retryAfter = now().addingTimeInterval(10)
            lock.unlock()
        }
        return readCLI(args)
    }

    static func request(status: Data?, token: String?, now: Date = Date()) -> URLRequest? {
        guard let status,
              let root = try? JSONSerialization.jsonObject(with: status) as? [String: Any],
              let state = root["state"] as? String, state == "running",
              let updated = root["updated"] as? String, let at = TunnelStatus.parseTime(updated),
              let staleAfter = root["staleAfter"] as? Double, staleAfter > 0,
              now.timeIntervalSince(at) <= staleAfter,
              let detail = root["detail"] as? [String: Any],
              let port = detail["port"] as? Int, (1...65535).contains(port),
              let bind = detail["bind"] as? String,
              let token = token?.trimmingCharacters(in: .whitespacesAndNewlines),
              !token.isEmpty, token.count <= 4096,
              !token.contains("\r"), !token.contains("\n") else { return nil }
        let host: String
        switch bind {
        case "127.0.0.1", "0.0.0.0", "localhost", "": host = "127.0.0.1"
        case "::1", "::": host = "[::1]"
        default: return nil
        }
        guard let url = URL(string: "http://\(host):\(port)/api/agents") else { return nil }
        var request = URLRequest(url: url, cachePolicy: .reloadIgnoringLocalCacheData, timeoutInterval: 1.2)
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        request.setValue("menubar", forHTTPHeaderField: "X-Gtmux-Actor")
        return request
    }

    static func usesStandaloneProfile(_ environment: [String: String]) -> Bool {
        ["GTMUX_BIN", "CODEX_HOME", "DSH_HOME"].contains { !(environment[$0] ?? "").isEmpty }
    }

    private static func localSnapshot() -> Data? {
        // Explicit CLI overrides also drive synthetic previews and test fixtures.
        let environment = ProcessInfo.processInfo.environment
        if usesStandaloneProfile(environment) { return nil }
        let status = FileManager.default.contents(atPath: Paths.data("status/serve.json"))
        let token = try? String(contentsOfFile: Paths.config("serve-token"), encoding: .utf8)
        guard let request = request(status: status, token: token) else { return nil }
        if let token { DiagLog.registerSecret(token.trimmingCharacters(in: .whitespacesAndNewlines)) }
        return RadarSnapshotHTTP().read(request)
    }
}

/// A single bounded request. Its serial delegate owns the buffer; the semaphore
/// publishes completion to the off-main poller. Redirects never receive credentials.
final class RadarSnapshotHTTP: NSObject, URLSessionDataDelegate, @unchecked Sendable {
    private let done = DispatchSemaphore(value: 0)
    private var data = Data()
    private var accepted = false
    private var result: Data?
    private let limit = 4_194_304
    private let configuration: URLSessionConfiguration

    init(configuration: URLSessionConfiguration = .ephemeral) { self.configuration = configuration }

    func read(_ request: URLRequest) -> Data? {
        let queue = OperationQueue()
        queue.maxConcurrentOperationCount = 1
        let config = configuration
        config.connectionProxyDictionary = [:]
        config.urlCache = nil
        config.httpCookieStorage = nil
        let session = URLSession(configuration: config, delegate: self, delegateQueue: queue)
        let task = session.dataTask(with: request)
        task.resume()
        guard done.wait(timeout: .now() + 1.5) == .success else {
            task.cancel()
            session.invalidateAndCancel()
            return nil
        }
        session.finishTasksAndInvalidate()
        return result
    }

    func urlSession(_ session: URLSession, task: URLSessionTask,
                    willPerformHTTPRedirection response: HTTPURLResponse, newRequest request: URLRequest,
                    completionHandler: @escaping (URLRequest?) -> Void) { completionHandler(nil) }

    func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive response: URLResponse,
                    completionHandler: @escaping (URLSession.ResponseDisposition) -> Void) {
        accepted = (response as? HTTPURLResponse)?.statusCode == 200 && response.expectedContentLength <= Int64(limit)
        completionHandler(accepted ? .allow : .cancel)
    }

    func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive chunk: Data) {
        guard accepted, chunk.count <= limit - data.count else {
            accepted = false
            data.removeAll()
            dataTask.cancel()
            return
        }
        data.append(chunk)
    }

    func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
        if error == nil, accepted { result = data }
        done.signal()
    }
}
