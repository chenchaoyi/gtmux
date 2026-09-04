import XCTest
@testable import GtmuxBar

/// The knowledge reader can now make four JUDGMENT calls (menubar-kb-actions). What has to
/// stay true is that each one runs the CLI verb of the same name, with the reason the CLI
/// requires, and that the app never invents an action the ledger would refuse.
final class HQKnowledgeActionsTests: XCTestCase {
    private func entry(_ id: String, promoted: Int64 = 0, landed: Int64 = 0) -> KBEntry {
        let json = """
        {"id":"\(id)","topic":"pitfalls","title":"t","at":1,
         "promotedAt":\(promoted),"landedAt":\(landed)}
        """
        return try! JSONDecoder().decode(KBEntry.self, from: Data(json.utf8))
    }

    func testEachActIsTheCLIVerbOfTheSameName() {
        // Not a re-implementation: the app spends a process and the ledger decides. A verb
        // spelled wrong here is an action that silently does nothing on a real machine.
        XCTAssertEqual(KnowledgeAct.promote(id: "pitfalls/x").argv(reason: "charter-level"),
                       ["knowledge", "promote", "pitfalls/x", "--why", "charter-level"])
        XCTAssertEqual(KnowledgeAct.land(id: "pitfalls/x").argv(reason: "PR #888"),
                       ["knowledge", "land", "pitfalls/x", "--ref", "PR #888"])
        XCTAssertEqual(KnowledgeAct.retire(id: "pitfalls/x").argv(reason: "network was fixed"),
                       ["knowledge", "retire", "pitfalls/x", "--why", "network was fixed"])
        // A candidate is a spool line, not a ledger entry: dismiss names the KEY.
        XCTAssertEqual(KnowledgeAct.dismiss(key: "pitfalls/x").argv(reason: "already covered"),
                       ["knowledge", "dismiss", "--capture", "pitfalls/x", "--why", "already covered"])
    }

    func testTheReasonIsNeverDroppedOrReordered() {
        // `land` takes --ref and the other three take --why. Sending a ref as a why would
        // be accepted by the CLI and record the wrong fact.
        for act in [KnowledgeAct.promote(id: "a"), .land(id: "a"), .retire(id: "a"), .dismiss(key: "a")] {
            let argv = act.argv(reason: "REASON")
            XCTAssertEqual(argv.last, "REASON", "\(act) dropped or misplaced the reason")
            XCTAssertEqual(argv[argv.count - 2], act.copy(L10n.shared).field,
                           "\(act) used a flag its copy does not name")
        }
    }

    func testAPendingPromotionOffersLandingItRatherThanPromotingItAgain() {
        // The CLI refuses a second promote while one is pending ("land it before promoting
        // again"), so offering the button would be offering an error message.
        let waiting = knowledgeActs(for: entry("a", promoted: 100))
        XCTAssertEqual(waiting, [.land(id: "a"), .retire(id: "a")])

        let ordinary = knowledgeActs(for: entry("b"))
        XCTAssertEqual(ordinary, [.promote(id: "b"), .retire(id: "b")])

        // A LANDED promotion is done, so the entry is ordinary again and can be re-promoted.
        let landed = knowledgeActs(for: entry("c", promoted: 100, landed: 200))
        XCTAssertEqual(landed, [.promote(id: "c"), .retire(id: "c")])
    }

    func testOnlyTheTwoThatTakeSomethingAwayReadAsRemovals() {
        XCTAssertTrue(KnowledgeAct.retire(id: "a").removes)
        XCTAssertTrue(KnowledgeAct.dismiss(key: "a").removes)
        XCTAssertFalse(KnowledgeAct.promote(id: "a").removes)
        XCTAssertFalse(KnowledgeAct.land(id: "a").removes)
    }

    func testEveryActHasCopyInBothLanguages() {
        // A blank button is an action a reader cannot find; a missing hint is a reason
        // field with no explanation of what it becomes.
        let l10n = L10n.shared
        for act in [KnowledgeAct.promote(id: "a"), .land(id: "a"), .retire(id: "a"), .dismiss(key: "a")] {
            let c = act.copy(l10n)
            for (name, text) in [("button", c.button), ("title", c.title),
                                 ("hint", c.hint), ("placeholder", c.placeholder)] {
                XCTAssertFalse(text.trimmingCharacters(in: .whitespaces).isEmpty,
                               "\(act) has no \(name)")
            }
        }
    }
}
