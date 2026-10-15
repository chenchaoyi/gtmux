import AppKit
import SwiftUI
import XCTest

@testable import GtmuxBar

@MainActor final class HQChatPresentationTests: XCTestCase {
    private var calendar: Calendar {
        var c = Calendar(identifier: .gregorian)
        c.timeZone = TimeZone(secondsFromGMT: 0)!
        return c
    }
    private func at(_ iso: String) -> Int64 { HQChatTimeline.timestamp(iso)! }

    func testSeparatorsUseSuccessiveValidSourceTimesAndFifteenMinuteBoundary() {
        let first = at("2026-10-15T09:00:00Z")
        let values: [Int64?] = [nil, first, first + 14 * 60, nil, 0, first + 28 * 60, first + 43 * 60]
        let result = HQChatTimeline.separators(values, calendar: calendar)
        XCTAssertEqual(result.map { $0 != nil }, [false, true, false, false, false, false, true])
        XCTAssertEqual(result.last!!, Date(timeIntervalSince1970: TimeInterval(first + 43 * 60)))
        XCTAssertEqual(HQChatTimeline.separators([], calendar: calendar).count, 0)
        XCTAssertNil(HQChatTimeline.timestamp("not-a-clock"))
        XCTAssertNil(HQChatTimeline.timestamp(nil))
        XCTAssertEqual(HQChatTimeline.timestamp("2026-10-15T09:00:00.123Z"), first)
    }
    func testMidnightCreatesSeparatorWithoutInventingClocksForUnknownRows() {
        let before = at("2026-10-14T23:59:00Z")
        let after = at("2026-10-15T00:01:00Z")
        let dates = HQChatTimeline.separators([before, nil, after], calendar: calendar)
        XCTAssertEqual(dates.map { $0 != nil }, [true, false, true])
        let now = Date(timeIntervalSince1970: TimeInterval(after + 3600))
        XCTAssertEqual(HQChatTimeline.label(dates[0]!, zh: false, now: now, calendar: calendar), "Yesterday 23:59")
        XCTAssertEqual(HQChatTimeline.label(dates[2]!, zh: true, now: now, calendar: calendar), "今天 00:01")
        let old = Date(timeIntervalSince1970: TimeInterval(at("2025-01-03T12:04:00Z")))
        XCTAssertEqual(HQChatTimeline.label(old, zh: false, now: now, calendar: calendar), "Jan 3 2025, 12:04")
    }
    func testCopyPreservesFullMarkdownAndFeedbackResetsAndReportsFailure() async throws {
        var copied: String?
        let feedback = HQChatCopyFeedback(
            delay: 10_000_000,
            write: {
                copied = $0
                return true
            })
        let body = "## Result\n\n**Bold** and `code`\n\n" + String(repeating: "Full reply.\n", count: 300)
        feedback.copy(body)
        XCTAssertEqual(copied, body)
        XCTAssertEqual(feedback.state, .copied)
        try await Task.sleep(nanoseconds: 25_000_000)
        XCTAssertEqual(feedback.state, .ready)
        let failed = HQChatCopyFeedback(write: { _ in false })
        failed.copy("unavailable clipboard")
        XCTAssertEqual(failed.state, .failed)
        failed.clear()
        XCTAssertEqual(failed.state, .ready)
    }
    // Projection coverage, not live accessibility acceptance. The isolated UI preview
    // separately checks that the seam and public text are both actually rendered.
    func testSeamTurnRetainsPublicTextAndSourceClockInTimeline() throws {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        let detail = try decoder.decode(
            HQWorkspaceDetail.self,
            from: Data(
                #"{"schema":1,"messages":[],"turns":[{"prompt":"Prompt after rotation","response":"Reply after rotation","time":"2026-10-15T09:00:00Z","session_break":{"kind":"new","at":100}}],"earlier_available":false,"changes":[],"events":[],"warnings":[]}"#
                    .utf8))
        let row = try XCTUnwrap(HQConversationRow.rows(detail).first)
        XCTAssertEqual(row.turn?.prompt, "Prompt after rotation")
        XCTAssertEqual(row.turn?.response, "Reply after rotation")
        XCTAssertEqual(row.turn?.sessionBreak?.kind, "new")
        XCTAssertEqual(row.at, at("2026-10-15T09:00:00Z"))
    }
}
