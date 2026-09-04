import XCTest
@testable import GtmuxBar

/// The readers exist because DESIGN §12's rule is about DRIVING the fleet, not reading it.
/// These pin the two things that keep that distinction true.
final class HQReaderTests: XCTestCase {
    private func entry(_ id: String, promoted: Int64 = 0, landed: Int64 = 0) -> KBEntry {
        let json = """
        {"id":"\(id)","topic":"pitfalls","title":"t","at":1,
         "promotedAt":\(promoted),"landedAt":\(landed)}
        """
        return try! JSONDecoder().decode(KBEntry.self, from: Data(json.utf8))
    }

    func testPendingIsPromotedAndNotYetCarried() {
        // The one step of the knowledge lifecycle that waits on a person.
        XCTAssertTrue(entry("a", promoted: 100).pending)
        XCTAssertFalse(entry("b", promoted: 100, landed: 200).pending, "a landed promotion is done")
        XCTAssertFalse(entry("c").pending, "an ordinary entry owes nobody anything")
    }

    func testTheDebtIsOrderedOldestFirst() {
        // Oldest first, because the oldest is the one rotting — the same order the phone
        // and `gtmux doctor` read it in.
        let store = HQReaderStore()
        store.setEntriesForTest([entry("new", promoted: 900), entry("old", promoted: 100),
                                 entry("done", promoted: 50, landed: 60), entry("plain")])
        XCTAssertEqual(store.pending.map { $0.id }, ["old", "new"])
    }

    func testABoardThatWasNeverWrittenDecodesAsOrdinary() {
        // A fresh HQ has written none; that is a state to render, not a failure.
        let doc = try! JSONDecoder().decode(BoardDoc.self, from: Data(#"{"exists":false}"#.utf8))
        XCTAssertFalse(doc.exists)
        XCTAssertNil(doc.text)
    }
}

/// The reader's own bookkeeping once it can ACT. These pin the two places where the act
/// path and the read path have to agree.
final class HQReaderActingTests: XCTestCase {
    private func entry(_ id: String, promoted: Int64 = 0, landed: Int64 = 0) -> KBEntry {
        let json = """
        {"id":"\(id)","topic":"pitfalls","title":"t","at":1,
         "promotedAt":\(promoted),"landedAt":\(landed)}
        """
        return try! JSONDecoder().decode(KBEntry.self, from: Data(json.utf8))
    }

    private func candidate(_ key: String, at: Int64, lesson: String = "l") -> KBCandidate {
        let json = """
        {"at":\(at),"topic":"pitfalls","key":"\(key)","lesson":"\(lesson)"}
        """
        return try! JSONDecoder().decode(KBCandidate.self, from: Data(json.utf8))
    }

    func testALandedPromotionChangesTheFingerprintThoughTheIDsAreIdentical() {
        // The poll republishes only what changed, and it used to compare ids alone. `land`
        // and `promote` leave the id list byte-identical, so a reader who landed a
        // promotion would have watched it sit in "waiting on you" until an unrelated write
        // shifted an id.
        let before = [entry("a", promoted: 100), entry("b")]
        let after = [entry("a", promoted: 100, landed: 200), entry("b")]
        XCTAssertEqual(before.map { $0.id }, after.map { $0.id }, "premise: the ids do not move")
        XCTAssertNotEqual(HQReaderStore.stamp(before), HQReaderStore.stamp(after))

        // And an unchanged read still compares equal, which is what keeps scroll position
        // and selection alive across a 20-second poll.
        XCTAssertEqual(HQReaderStore.stamp(before), HQReaderStore.stamp(before))
    }

    func testCandidatesFoldByKeyBecauseThatIsWhatDismissConsumes() {
        // `gtmux knowledge dismiss --capture <key>` removes EVERY pending line with that
        // key, so a row per line would offer three buttons that each do the same thing to
        // all three.
        let groups = groupCandidates([
            candidate("pitfalls/wrangler", at: 300, lesson: "first phrasing"),
            candidate("workflows/tag-then-ci", at: 100),
            candidate("pitfalls/wrangler", at: 400, lesson: "newest phrasing"),
        ])
        XCTAssertEqual(groups.map { $0.key }, ["workflows/tag-then-ci", "pitfalls/wrangler"],
                       "oldest group first — the oldest is the one rotting")
        let merged = groups.first { $0.key == "pitfalls/wrangler" }
        XCTAssertEqual(merged?.count, 2, "the row must say how many lines its dismiss takes")
        XCTAssertEqual(merged?.at, 300, "a group is as old as its oldest line")
        XCTAssertEqual(merged?.lesson, "newest phrasing")
    }

    func testCandidateGroupsRepublishWhenTheQueueDrains() {
        let before = groupCandidates([candidate("a", at: 1), candidate("a", at: 2)])
        let after = groupCandidates([candidate("a", at: 1)])
        XCTAssertEqual(before.map { $0.key }, after.map { $0.key }, "premise: the keys do not move")
        XCTAssertNotEqual(HQReaderStore.stamp(before), HQReaderStore.stamp(after))
    }

    func testAnEmptyReasonNeverReachesTheCLI() {
        // Every one of these verbs is refused by the CLI without a reason; spending a
        // process to be told so, and showing that refusal as if the reader had done
        // something wrong, is worse than saying it here.
        let store = HQReaderStore()
        let done = expectation(description: "reported")
        var message: String?
        store.perform(.retire(id: "pitfalls/x"), reason: "   ", l10n: L10n.shared) {
            message = $0
            done.fulfill()
        }
        wait(for: [done], timeout: 2)
        XCTAssertNotNil(message, "a blank reason must be refused, not sent")
    }
}
