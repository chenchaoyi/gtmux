import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

@MainActor final class ScreenshotActiveInputTests: XCTestCase {
    func testSSI003ActiveQueueAndDeliveryReceiptsStayDistinct() {
        let cases: [(String, ScreenshotSendResult)] = [
            (#"{"state":"queued","delivered":false,"evidence":"codex-active-accepted:turn-fixture"}"#, .activeTurnAccepted),
            (#"{"state":"queued","delivered":false,"evidence":"codex-active-already-accepted"}"#, .activeTurnAccepted),
            (#"{"state":"queued","delivered":false,"evidence":"codex-queue-accepted:q"}"#, .delivered(queued: true)),
            (#"{"state":"landed","delivered":true}"#, .delivered(queued: false))
        ]
        for (json, expected) in cases {
            XCTAssertEqual(ScreenshotSender.interpret(status: 0, stdout: json, stderr: ""), expected)
            XCTAssertTrue(expected.isSuccess)
            XCTAssertFalse(ScreenshotSender.interpret(status: 1, stdout: json, stderr: "fixture failure").isSuccess)
        }
        for json in [#"{"state":"unknown","delivered":true}"#, #"{"state":"queued","delivered":true}"#,
                     #"{"state":"landed","delivered":false}"#, #"{"state":"queued","delivered":false,"evidence":"codex-active-accepted:"}"#] {
            XCTAssertFalse(ScreenshotSender.interpret(status: 0, stdout: json, stderr: "").isSuccess)
        }
        XCTAssertEqual(ScreenshotSendResult.activeTurnAccepted.receiptState, "active-turn-accepted")
        XCTAssertEqual(ScreenshotSendResult.delivered(queued: true).receiptState, "queued")
        XCTAssertEqual(ScreenshotSendResult.delivered(queued: false).receiptState, "delivered")
        XCTAssertEqual(ScreenshotSender.interpret(status: 1, stdout: #"{"state":"refused-waiting","delivered":false,"evidence":"input-metadata-unavailable"}"#, stderr: ""), .refusedUnavailable("input-metadata-unavailable"))
        XCTAssertEqual(ScreenshotSender.interpret(status: 1, stdout: #"{"state":"refused-waiting","delivered":false,"evidence":"input-awaiting-confirmation"}"#, stderr: ""), .refusedWaiting)
    }

    func testSSI003BothLanguagesDescribeQueuedAndActiveInputTruthfully() {
        let saved = L10n.shared.mode
        defer { L10n.shared.mode = saved }
        for language in [LangMode.en, .zh] {
            L10n.shared.mode = language
            let queue = ScreenshotDeliveryOverlay(status: .result(.delivered(queued: true)), target: nil, l10n: .shared)
            let active = ScreenshotDeliveryOverlay(status: .result(.activeTurnAccepted), target: nil, l10n: .shared)
            XCTAssertEqual(queue.title, language == .zh ? "截图已排队" : "Screenshot queued")
            XCTAssertEqual(queue.detail, language == .zh ? "等待 Agent 下一轮处理" : "Waiting for the agent's next turn")
            XCTAssertEqual(active.title, language == .zh ? "已补充到当前任务" : "Added to the current task")
            XCTAssertEqual(active.detail, language == .zh ? "Agent 将在工作中读取" : "The agent will read it while working")
            XCTAssertNotEqual(queue.detail, active.detail)
            XCTAssertTrue(ScreenshotStatusText.text(.result(.activeTurnAccepted), target: "Fixture", l10n: .shared).contains("Fixture"))
        }
        XCTAssertTrue(ScreenshotEditorModel.blocksEditing(.result(.activeTurnAccepted)))
        XCTAssertFalse(ScreenshotEditorModel.blocksEditing(.result(.notConfirmed("fixture"))))
    }
}
