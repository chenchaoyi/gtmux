import Foundation
import Network
import XCTest
@testable import GtmuxUsageWidgetHost

final class UsageHostTests: XCTestCase {
    func testSelectedPairingIsTheOnlyCredentialSource() throws {
        func store(_ active: String?, _ scope: String, _ token: String = "fixture") -> Data {
            try! JSONSerialization.data(withJSONObject: ["activeUrl": active as Any? ?? NSNull(),
                "servers": [["url": "https://studio.example", "token": token, "name": "Studio", "scope": scope]]])
        }
        XCTAssertEqual(WidgetOwnerConnection.load(selected: "https://studio.example", read: { store("https://studio.example", "owner") })?.token, "fixture")
        for data in [store(nil, "owner"), store("https://studio.example", "guest"), store("https://studio.example", "owner", "bad\nheader")] {
            XCTAssertNil(WidgetOwnerConnection.load(selected: "https://studio.example", read: { data }))
        }
        XCTAssertEqual(WidgetOwnerConnection.stillSelectedOwner("https://studio.example", read: { store(nil, "owner") }), false)
        XCTAssertNil(WidgetOwnerConnection.stillSelectedOwner("https://studio.example", read: { nil }), "Unavailable protected storage is not a logout")
        for url in ["file:///tmp/x", "https://user:secret@studio.example", "https://studio.example?token=secret", "https://studio.example/#x"] {
            XCTAssertNil(WidgetOwnerConnection.validatedURL(url))
        }
    }

    func testScopeBeforeUsageAndRedirectNeverForwardsBearer() async throws {
        let sink = try await UsageHTTPFixture { _ in (200, [:], Data("{}".utf8)) }
        let redirect = try await UsageHTTPFixture { _ in (302, ["Location": sink.url.absoluteString + "/sink"], Data()) }
        let refused = try await UsageHTTPFixture { _ in (200, [:], Data("{\"all\":false}".utf8)) }
        let owner = try await UsageHTTPFixture { path in
            (200, [:], Data((path == "/api/share" ? "{\"all\":true}" : "{\"limits\":{}}").utf8))
        }
        defer { sink.close(); redirect.close(); refused.close(); owner.close() }
        XCTAssertNil(awaitRead(redirect).value)
        XCTAssertNil(awaitRead(refused).value)
        XCTAssertEqual(awaitRead(owner).value, Data("{\"limits\":{}}".utf8))
        XCTAssertEqual(owner.paths, ["/api/share", "/api/usage"])
        XCTAssertEqual(refused.paths, ["/api/share"])
        XCTAssertTrue(sink.paths.isEmpty, "No redirect request reaches the other origin")
        XCTAssertTrue(owner.headers.allSatisfy { $0.contains("Authorization: Bearer fixture") })
    }

    func testRefusalAndBodyBounds() async throws {
        for status in [401, 403] {
            let server = try await UsageHTTPFixture { _ in (status, ["Content-Length": "3000000"], Data()) }
            let result = awaitRead(server); server.close()
            if case .failure(.refused) = result {} else { XCTFail("Authorization rejection must be explicit") }
        }
        for header in [true, false] {
            let server = try await UsageHTTPFixture { path in
                if path == "/api/share" { return (200, [:], Data("{\"all\":true}".utf8)) }
                return (200, header ? ["Content-Length": "2097153"] : [:], Data(repeating: 32, count: 2 * 1024 * 1024 + 1))
            }
            let result = awaitRead(server); server.close()
            if case .failure(.invalid) = result {} else { XCTFail("Oversized responses must be rejected") }
        }
    }

    // Run the real URLSession delegate and keep its main-queue completion alive.
    private func awaitRead(_ server: UsageHTTPFixture) -> Result<Data, WidgetUsageFailure> {
        let finished = expectation(description: "bounded native usage read")
        var result: Result<Data, WidgetUsageFailure> = .failure(.unavailable)
        let request = WidgetUsageRequest(connection: WidgetOwnerConnection(base: server.url, token: "fixture", name: "Studio")) {
            result = $0; finished.fulfill()
        }
        request.start(); wait(for: [finished], timeout: 5); request.cancel()
        return result
    }
}

private extension Result where Success == Data, Failure == WidgetUsageFailure {
    var value: Data? { try? get() }
}

/// Loopback only, with synthetic credentials. Tests exercise real redirects and
/// streaming response bounds, rather than calling delegate methods directly.
private final class UsageHTTPFixture: @unchecked Sendable {
    let listener: NWListener
    let queue = DispatchQueue(label: "usage-widget.http-fixture")
    var url: URL { URL(string: "http://127.0.0.1:\(listener.port!.rawValue)")! }
    private let lock = NSLock()
    private var records: [String] = []
    var paths: [String] { lock.lock(); defer { lock.unlock() }; return records.map { $0.components(separatedBy: " ")[1] } }
    var headers: [String] { lock.lock(); defer { lock.unlock() }; return records }

    init(response: @escaping (String) -> (Int, [String: String], Data)) async throws {
        let parameters = NWParameters.tcp
        parameters.requiredLocalEndpoint = .hostPort(host: "127.0.0.1", port: .any)
        listener = try NWListener(using: parameters)
        listener.newConnectionHandler = { [weak self] connection in
            guard let self else { connection.cancel(); return }
            connection.start(queue: self.queue)
            self.receive(connection, accumulated: Data(), response: response)
        }
        try await withCheckedThrowingContinuation { (ready: CheckedContinuation<Void, Error>) in
            var resumed = false
            listener.stateUpdateHandler = { state in
                guard !resumed else { return }
                if case .ready = state { resumed = true; ready.resume() }
                if case .failed(let error) = state { resumed = true; ready.resume(throwing: error) }
            }
            listener.start(queue: queue)
        }
    }
    private func receive(_ connection: NWConnection, accumulated: Data,
                         response: @escaping (String) -> (Int, [String: String], Data)) {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 8192) { [weak self] bytes, _, ended, error in
            guard let self else { connection.cancel(); return }
            let data = accumulated + (bytes ?? Data())
            guard data.count <= 16384, error == nil else { connection.cancel(); return }
            let text = String(decoding: data, as: UTF8.self)
            guard text.contains("\r\n\r\n") else {
                if ended { connection.cancel() } else { self.receive(connection, accumulated: data, response: response) }
                return
            }
            self.lock.lock(); self.records.append(text); self.lock.unlock()
            let path = text.components(separatedBy: " ").dropFirst().first ?? ""
            let (status, headers, body) = response(path)
            var head = "HTTP/1.1 \(status) Fixture\r\nConnection: close\r\n"
            for (key, value) in headers { head += "\(key): \(value)\r\n" }
            head += "\r\n"
            connection.send(content: Data(head.utf8) + body, completion: .contentProcessed { _ in connection.cancel() })
        }
    }
    func close() { listener.cancel() }
}
