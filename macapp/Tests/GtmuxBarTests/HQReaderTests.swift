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
