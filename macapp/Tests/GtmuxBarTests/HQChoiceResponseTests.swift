import AppKit
import SwiftUI
import XCTest

@testable import GtmuxBar

@MainActor final class HQChoiceResponseTests: XCTestCase {
    func testChoicePreparesFocusWithoutSendingAndPreservesEditedDraft() async throws {
        let fake = ChoiceFixture()
        let store = try await HQChoiceTestSupport.load(fake)
        defer { store.stop() }
        let question = try XCTUnwrap(store.currentMatter?.userInput)
        let serial = store.composerFocusSerial
        store.prepareHumanReply(question, choice: "Done")
        XCTAssertEqual(store.draft, question.prompt + "\nDone")
        XCTAssertEqual(store.preparedHumanChoice, "Done")
        XCTAssertEqual(store.composerFocusSerial, serial + 1)
        XCTAssertTrue(fake.actions.isEmpty)
        store.draft = "My edited draft"
        store.prepareHumanReply(question, choice: "Keep")
        XCTAssertEqual(store.draft, "My edited draft")
        XCTAssertNotNil(store.error)
        XCTAssertNil(store.preparedHumanChoice)
        var stale = question
        stale.id = "old-question"
        store.prepareHumanReply(stale, choice: "Done")
        XCTAssertEqual(store.draft, "My edited draft")
        XCTAssertNotNil(store.error)
        XCTAssertTrue(fake.actions.isEmpty)
    }
    func testSlowSourceDiscoveryCannotBlockReplyOrRollBackNewCanonicalData() async throws {
        let fake = ChoiceFixture()
        fake.storedOnly = true
        let gate = DispatchSemaphore(value: 0)
        fake.gate = gate
        let store = try await HQChoiceTestSupport.load(fake)
        defer {
            gate.signal()
            store.stop()
        }
        store.refreshSources(force: true)
        try await HQChoiceTestSupport.waitUntil { store.sourcesLoading }
        let question = try XCTUnwrap(store.currentMatter?.userInput)
        store.prepareHumanReply(question, choice: "Done")
        XCTAssertEqual(store.preparedHumanChoice, "Done")
        XCTAssertTrue(store.canSend)
        XCTAssertTrue(fake.actions.isEmpty)
        fake.replace {
            $0.revision = 2
            $0.matters[0].revision = 2
            $0.matters[0].userInput?.answerId = "saved-reply"
        }
        store.refresh()
        try await HQChoiceTestSupport.waitUntil { !store.loading }
        gate.signal()
        try await HQChoiceTestSupport.waitUntil { !store.sourcesLoading }
        XCTAssertEqual(store.snapshot?.revision, 2)
        XCTAssertEqual(store.currentMatter?.userInput?.answerId, "saved-reply")
        XCTAssertEqual(store.draft, question.prompt + "\nDone")
        XCTAssertTrue(fake.actions.isEmpty)
    }
    func testSourceTimeoutDoesNotBecomeTheQuestionOrDraftError() async throws {
        let fake = ChoiceFixture()
        fake.storedOnly = true
        fake.failSources = true
        let store = try await HQChoiceTestSupport.load(fake)
        defer { store.stop() }
        store.refreshSources(force: true)
        try await HQChoiceTestSupport.waitUntil { !store.sourcesLoading }
        XCTAssertNotNil(store.sourcesError)
        XCTAssertNil(store.error)
        XCTAssertFalse(store.sourcesKnown)
        let question = try XCTUnwrap(store.currentMatter?.userInput)
        store.prepareHumanReply(question, choice: "Keep")
        XCTAssertEqual(store.preparedHumanChoice, "Keep")
        XCTAssertTrue(store.canSend)
        XCTAssertTrue(fake.actions.isEmpty)
    }
    func testStoredClientUsesReadOnlyFlagAndOnlyLegacyUsageFallsBack() throws {
        var calls: [[String]] = []
        let client = HQWorkspaceClient(execute: { args, _ in
            calls.append(args)
            return (
                0,
                #"{"schema":1,"revision":1,"matters":[],"conversations":[],"sources_included":false}"#,
                ""
            )
        })
        XCTAssertEqual(try client.summary().sourcesIncluded, false)
        XCTAssertEqual(calls.count, 1)
        XCTAssertTrue(calls[0].contains("--stored"))
        calls = []
        let legacy = HQWorkspaceClient(execute: { args, _ in
            calls.append(args)
            return args.contains("--stored")
                ? (2, "", "")
                : (0, #"{"schema":1,"revision":1,"matters":[],"conversations":[]}"#, "")
        })
        _ = try legacy.summary()
        XCTAssertEqual(calls.count, 2)
        XCTAssertFalse(calls[1].contains("--stored"))
        calls = []
        let timeout = HQWorkspaceClient(execute: { args, _ in
            calls.append(args)
            return (-2, "", "timeout")
        })
        XCTAssertThrowsError(try timeout.summary())
        XCTAssertEqual(calls.count, 1)
        calls = []
        let denied = HQWorkspaceClient(execute: { args, _ in
            calls.append(args)
            return (2, "", "permission denied")
        })
        XCTAssertThrowsError(try denied.summary())
        XCTAssertEqual(calls.count, 1)
    }
    func testUnavailableSourceMetadataNeverClaimsTheSessionIsGone() {
        let participant: HQParticipant = choiceDecode(
            #"{"id":"association","conversation":{"agent":"codex","session_id":"synthetic-session","name":"Synthetic","pane":"%1"},"role":"execution","from":100}"#
        )
        let candidate: HQCandidate = choiceDecode(
            #"{"agent":"codex","session_id":"synthetic-session","name":"Synthetic","pane":"%1","status":"working","since":100}"#
        )
        let info = HQParticipantPresentation(
            participant: participant, current: candidate, currentKnown: false)
        XCTAssertNil(info.current)
        XCTAssertEqual(info.state(.shared), L10n.shared.tr("Session status unavailable", "会话状态待刷新"))
    }

}
