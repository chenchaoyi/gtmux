import XCTest
@testable import GtmuxBar

/// The menu bar writes the same entries as the CLI into the same store, so `gtmux logs`
/// reads the app beside everything else. These pin the shape the Go reader parses, the
/// credentials it must never carry, and the file it appends to.
final class DiagLogTests: XCTestCase {
    private var dir = ""

    override func setUp() {
        dir = NSTemporaryDirectory() + "diaglog-" + UUID().uuidString
        DiagLog.dirOverride = dir
    }

    override func tearDown() {
        DiagLog.dirOverride = nil
        try? FileManager.default.removeItem(atPath: dir)
    }

    private func entries() -> [[String: Any]] {
        DiagLog.flush()
        let path = DiagLog.currentSegment(dir: dir, day: DiagLog.dayString(Date()))
        guard let text = try? String(contentsOfFile: path, encoding: .utf8) else { return [] }
        return text.split(separator: "\n").compactMap {
            try? JSONSerialization.jsonObject(with: Data($0.utf8)) as? [String: Any]
        }
    }

    func testAnActLandsInTheSharedSchema() {
        DiagLog.act("act.notify.post", target: "%7", outcome: "ok", "showed a notification", ["kind": "done"])
        let e = entries()
        XCTAssertEqual(e.count, 1)
        XCTAssertEqual(e.first?["component"] as? String, "menubar")
        XCTAssertEqual(e.first?["kind"] as? String, "act")
        XCTAssertEqual(e.first?["actor"] as? String, "menubar")
        XCTAssertEqual(e.first?["target"] as? String, "%7")
        XCTAssertEqual(e.first?["outcome"] as? String, "ok")
        XCTAssertEqual(e.first?["level"] as? String, "info")
        // The Go reader parses this layout: RFC 3339, milliseconds, a numeric offset.
        let ts = e.first?["ts"] as? String ?? ""
        XCTAssertNotNil(ts.range(of: #"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}[+-]\d{2}:\d{2}$"#,
                                 options: .regularExpression), ts)
    }

    func testCredentialsNeverReachTheStore() {
        let token = "tok_" + UUID().uuidString
        DiagLog.registerSecret(token)
        DiagLog.info("menubar.test", "calling with Bearer \(token) for https://host/#c=abc123",
                     ["token": "plain-value", "url": "https://host/p1#g=share-secret"])
        DiagLog.flush()
        let raw = (try? String(contentsOfFile: DiagLog.currentSegment(dir: dir, day: DiagLog.dayString(Date())),
                               encoding: .utf8)) ?? ""
        for secret in [token, "abc123", "plain-value", "share-secret"] {
            XCTAssertFalse(raw.contains(secret), "\(secret) reached the store: \(raw)")
        }
        XCTAssertTrue(raw.contains(DiagLog.redacted))
    }

    /// The switch reads as the CLI reads it, and config.json's `debug` counts, not only the
    /// shell variables: Extra detail used to record nothing from the menu bar (%12,
    /// 2026-10-06).
    func testTheDebugSwitchReadsLikeTheCLI() {
        XCTAssertTrue(DiagLog.switchOn("serve,menubar", for: "menubar"))
        XCTAssertTrue(DiagLog.switchOn(" menubar ", for: "menubar"))
        XCTAssertTrue(DiagLog.switchOn("all", for: "menubar"))
        XCTAssertTrue(DiagLog.switchOn("1", for: "menubar"))
        XCTAssertFalse(DiagLog.switchOn("serve,tunnel", for: "menubar"))
        XCTAssertFalse(DiagLog.switchOn("", for: "menubar"))
        XCTAssertEqual(DiagLog.configDebug(from: Data(#"{"debug":"menubar","other":1}"#.utf8)), "menubar")
        XCTAssertEqual(DiagLog.configDebug(from: Data(#"{"other":1}"#.utf8)), "")
        XCTAssertEqual(DiagLog.configDebug(from: Data("not json".utf8)), "")
        XCTAssertEqual(DiagLog.configDebug(from: nil), "")
    }

    func testExtraDetailTakesEffectWithoutARestart() {
        defer { DiagLog.noteConfigDebug(nil) }
        DiagLog.noteConfigDebug("")
        if DiagLog.envDebugOn { return } // a shell variable already turns it on here
        DiagLog.debug("menubar.trace", "before the switch")
        DiagLog.noteConfigDebug("all") // what Diagnostics does once `gtmux config debug on` succeeds
        DiagLog.debug("menubar.trace", "after the switch")
        DiagLog.flush()
        let raw = (try? String(contentsOfFile: DiagLog.currentSegment(dir: dir, day: DiagLog.dayString(Date())),
                               encoding: .utf8)) ?? ""
        XCTAssertFalse(raw.contains("before the switch"), raw)
        XCTAssertTrue(raw.contains("after the switch"), raw)
    }

    func testAnOversizedEntryStaysOneAtomicWrite() {
        DiagLog.info("menubar.test", String(repeating: "x", count: 10_000))
        let e = entries()
        XCTAssertEqual(e.count, 1)
        XCTAssertEqual(e.first?["truncated"] as? Bool, true)
        let raw = (try? Data(contentsOf: URL(fileURLWithPath:
            DiagLog.currentSegment(dir: dir, day: DiagLog.dayString(Date()))))) ?? Data()
        XCTAssertLessThanOrEqual(raw.count, DiagLog.maxEntry)
    }

    func testItAppendsToTheDaysLatestSegment() throws {
        try FileManager.default.createDirectory(atPath: dir, withIntermediateDirectories: true)
        let day = "2026-09-19"
        for name in ["\(day).jsonl", "\(day).1.jsonl", "\(day).2.jsonl", "2026-09-18.5.jsonl"] {
            FileManager.default.createFile(atPath: dir + "/" + name, contents: Data())
        }
        XCTAssertEqual(DiagLog.currentSegment(dir: dir, day: day), dir + "/\(day).2.jsonl")
        XCTAssertEqual(DiagLog.currentSegment(dir: dir, day: "2026-09-20"), dir + "/2026-09-20.jsonl")
    }

    /// The pairing window re-probes every few seconds; the log gets one entry per change
    /// of what it tells the user, with the probe's status behind it.
    func testTheReachVerdictIsLoggedOnlyWhenItChanges() {
        Pairing.logReach(.checking, httpStatus: nil, error: nil) // a known starting point
        Pairing.logReach(.tunnelDown("lookup failed"), httpStatus: 502, error: nil)
        Pairing.logReach(.tunnelDown("lookup failed"), httpStatus: 502, error: nil)
        Pairing.logReach(.reachable, httpStatus: 200, error: nil)
        let e = entries().filter { $0["event"] as? String == "reach.verdict" }
        XCTAssertEqual(e.count, 2)
        let first = e.first?["attrs"] as? [String: Any]
        XCTAssertEqual(first?["verdict"] as? String, "tunnel-down")
        XCTAssertEqual(first?["status"] as? Int, 502)
        XCTAssertEqual(e.first?["level"] as? String, "warn")
        XCTAssertEqual((e.last?["attrs"] as? [String: Any])?["verdict"] as? String, "reachable")
    }
}
