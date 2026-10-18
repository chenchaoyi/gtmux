import XCTest
@testable import GtmuxBar

final class RadarSnapshotReaderTests: XCTestCase {
    func testSharedSnapshotAvoidsCLIAndNonRadarReadsKeepCLI() {
        var cliCalls = 0
        let reader = RadarSnapshotReader(readHTTP: { Data("[]".utf8) }, readCLI: { _ in cliCalls += 1; return Data("fallback".utf8) })
        XCTAssertEqual(reader.capture(["agents", "--json"]), Data("[]".utf8))
        XCTAssertEqual(cliCalls, 0)
        XCTAssertEqual(reader.capture(["resource", "--json"]), Data("fallback".utf8))
        XCTAssertEqual(cliCalls, 1)
    }
    func testMalformedAndFailedSnapshotFallbackWithBoundedRetry() {
        var time = Date(timeIntervalSince1970: 1000)
        var httpCalls = 0
        var cliCalls = 0
        var body = Data("not json".utf8)
        let reader = RadarSnapshotReader(readHTTP: { httpCalls += 1; return body }, readCLI: { _ in cliCalls += 1; return Data("[]".utf8) }, now: { time })
        XCTAssertNotNil(reader.capture(["agents", "--json"]))
        XCTAssertNotNil(reader.capture(["agents", "--json"]))
        XCTAssertEqual(httpCalls, 1)
        XCTAssertEqual(cliCalls, 2)
        body = Data("[]".utf8)
        time = time.addingTimeInterval(11)
        XCTAssertEqual(reader.capture(["agents", "--json"]), body)
        XCTAssertEqual(httpCalls, 2)
        XCTAssertEqual(cliCalls, 2)
    }
    func testEndpointIsLoopbackAndTokenNeverEntersURL() throws {
        func status(_ bind: String, _ port: Int) throws -> Data {
            try JSONSerialization.data(withJSONObject: ["state": "running", "updated": ISO8601DateFormatter().string(from: Date()), "staleAfter": 90, "detail": ["bind": bind, "port": port]])
        }
        let request = try XCTUnwrap(RadarSnapshotReader.request(status: try status("0.0.0.0", 8765), token: "fixture"))
        XCTAssertEqual(request.url?.absoluteString, "http://127.0.0.1:8765/api/agents")
        XCTAssertEqual(request.value(forHTTPHeaderField: "Authorization"), "Bearer fixture")
        XCTAssertNil(RadarSnapshotReader.request(status: try status("example.test", 8765), token: "fixture"))
        XCTAssertNil(RadarSnapshotReader.request(status: try status("127.0.0.1", 0), token: "fixture"))
        XCTAssertNil(RadarSnapshotReader.request(status: try status("127.0.0.1", 8765), token: "bad\nheader"))
        XCTAssertNil(RadarSnapshotReader.request(status: nil, token: "fixture"))
        let stale = try JSONSerialization.data(withJSONObject: ["state": "running", "updated": "2000-01-01T00:00:00Z", "staleAfter": 90, "detail": ["bind": "127.0.0.1", "port": 8765]])
        XCTAssertNil(RadarSnapshotReader.request(status: stale, token: "fixture"))
        let stopped = try JSONSerialization.data(withJSONObject: ["state": "stopped", "updated": ISO8601DateFormatter().string(from: Date()), "staleAfter": 90, "detail": ["bind": "127.0.0.1", "port": 8765]])
        XCTAssertNil(RadarSnapshotReader.request(status: stopped, token: "fixture"))
    }
    func testExplicitProfilesRetainTheirOwnCLIReader() {
        XCTAssertTrue(RadarSnapshotReader.usesStandaloneProfile(["GTMUX_BIN": "/fixture/gtmux"]))
        XCTAssertTrue(RadarSnapshotReader.usesStandaloneProfile(["DSH_HOME": "/fixture/dsh"]))
        XCTAssertTrue(RadarSnapshotReader.usesStandaloneProfile(["CODEX_HOME": "/fixture/codex"]))
        XCTAssertFalse(RadarSnapshotReader.usesStandaloneProfile([:]))
    }

    func testHTTPBoundaryRejectsRedirectErrorsAndOversizedBodies() throws {
        for path in ["valid", "unauthorized", "oversized", "redirect"] {
            let config = URLSessionConfiguration.ephemeral
            config.protocolClasses = [SnapshotFixtureProtocol.self]
            let request = URLRequest(url: try XCTUnwrap(URL(string: "https://radar.example.test/" + path)))
            let body = RadarSnapshotHTTP(configuration: config).read(request)
            if path == "valid" { XCTAssertEqual(body, Data("[]".utf8)) }
            else { XCTAssertNil(body, path) }
        }
    }
}

private final class SnapshotFixtureProtocol: URLProtocol, @unchecked Sendable {
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        guard let url = request.url else { return }
        if url.path == "/redirect" {
            let target = URLRequest(url: URL(string: "https://external.example.test/valid")!)
            let response = HTTPURLResponse(url: url, statusCode: 302, httpVersion: "HTTP/1.1", headerFields: ["Location": target.url!.absoluteString])!
            client?.urlProtocol(self, wasRedirectedTo: target, redirectResponse: response)
            return
        }
        let status = url.path == "/unauthorized" ? 403 : 200
        let response = HTTPURLResponse(url: url, statusCode: status, httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"])!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        let body = url.path == "/oversized" ? Data(repeating: 120, count: 4_194_305) : Data("[]".utf8)
        client?.urlProtocol(self, didLoad: body)
        client?.urlProtocolDidFinishLoading(self)
    }
    override func stopLoading() {}
}
