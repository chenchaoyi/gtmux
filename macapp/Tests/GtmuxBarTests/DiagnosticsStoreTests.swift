import XCTest
@testable import GtmuxBar

// The Diagnostics window reads entries the CLI printed. What is tested here is the part
// that can silently go wrong between the two: a number that comes back as 5.49551e+06,
// a timestamp that loses its clock, an entry whose whole detail line disappears because
// one attribute was a bool.
final class DiagnosticsStoreTests: XCTestCase {
    private func decode(_ line: String) -> LogEntry {
        try! JSONDecoder().decode(LogEntry.self, from: Data(line.utf8))
    }

    func testWholeNumbersStayIntegers() {
        let e = decode(#"{"ts":"2026-09-20T09:36:05.123+08:00","level":"info","component":"serve","kind":"act","event":"act.upload","attrs":{"bytes":5495510,"ratio":0.5,"kept":true}}"#)
        XCTAssertTrue(e.detail.contains("bytes=5495510"), e.detail)
        XCTAssertTrue(e.detail.contains("ratio=0.5"), e.detail)
        XCTAssertTrue(e.detail.contains("kept=true"), e.detail)
    }

    func testClockAndDayComeOffTheStamp() {
        let e = decode(#"{"ts":"2026-09-20T09:36:05.123+08:00","level":"info","component":"cli","kind":"diag","event":"x"}"#)
        XCTAssertEqual(e.clock, "09:36")
        XCTAssertEqual(e.day, "2026-09-20")
    }

    func testDetailNamesWhoDidWhatAndHowItEnded() {
        let e = decode(#"{"ts":"2026-09-20T09:36:05.123+08:00","level":"warn","component":"serve","kind":"act","event":"act.pair","actor":"anonymous","target":"tunnel.example","outcome":"refused","msg":"a pairing code was not accepted","attrs":{"reason":"expired"}}"#)
        XCTAssertTrue(e.isProblem)
        XCTAssertEqual(e.detail, "anonymous · → tunnel.example · refused · reason=expired")
    }

    func testAnInfoEntryIsNotAProblem() {
        XCTAssertFalse(decode(#"{"ts":"2026-09-20T09:36:05.123+08:00","level":"info","component":"cli","kind":"diag","event":"x"}"#).isProblem)
    }

    // The row leads with problems when there are any, so the count has to add up both
    // kinds — a day with two errors and no warnings still says two.
    func testProblemsCountBothKinds() {
        var st = LogStats()
        st.warnings = 1
        st.errors = 2
        XCTAssertEqual(st.problems, 3)
        XCTAssertEqual(LogStats().problems, 0)
    }

    // The window said "The store holds Zero KB" while `gtmux logs --stats` said 60 KB on
    // the same Mac (2026-09-20). The CLI omits a key whose value is empty, and Swift's
    // synthesized Decodable REFUSES a missing key whatever default the property carries —
    // so with debug off, every field came back zero.
    func testStatsDecodeSurvivesAnOmittedKey() throws {
        let line = #"{"bytes":61890,"files":1,"oldest":"2026-09-20","retainDays":30,"maxBytes":104857600,"windowHours":17,"entries":252,"warnings":3,"errors":2}"#
        let st = try JSONDecoder().decode(LogStats.self, from: Data(line.utf8))
        XCTAssertEqual(st.bytes, 61890)
        XCTAssertEqual(st.problems, 5)
        XCTAssertEqual(st.debug, "")
    }

    // And an older CLI, which has no --stats at all, must not zero the row either: what
    // it cannot answer stays at its default rather than throwing the whole read away.
    func testStatsDecodeSurvivesAnOlderCLI() throws {
        let st = try JSONDecoder().decode(LogStats.self, from: Data(#"{"bytes":42}"#.utf8))
        XCTAssertEqual(st.bytes, 42)
        XCTAssertEqual(st.retainDays, 30)
    }
}
