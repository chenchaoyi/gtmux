import XCTest
@testable import GtmuxBar

/// The knowledge reader's JUDGMENT calls (menubar-kb-actions, then the audience exits of
/// hq-knowledge-engine). What has to stay true is that each one runs the CLI verb of the
/// same name, with the reason the CLI requires, and that the app never invents an action
/// the ledger would refuse.
final class HQKnowledgeActionsTests: XCTestCase {
    private func entry(_ id: String, promoted: Int64 = 0, landed: Int64 = 0,
                       audience: String? = nil, issueUrl: String? = nil,
                       kind: String? = nil, assumed: Bool = false, provenance: String? = nil, hits: Int? = nil) -> KBEntry {
        var fields = ["\"id\":\"\(id)\"", "\"topic\":\"pitfalls\"", "\"title\":\"t\"", "\"at\":1",
                      "\"promotedAt\":\(promoted)", "\"landedAt\":\(landed)"]
        if let a = audience { fields.append("\"audience\":\"\(a)\"") }
        if let u = issueUrl { fields.append("\"issueUrl\":\"\(u)\"") }
        if let k = kind { fields.append("\"kind\":\"\(k)\"") }
        if assumed { fields.append("\"kindAssumed\":true") }
        if let p = provenance { fields.append("\"provenance\":\"\(p)\"") }
        if let h = hits { fields.append("\"hits\":\(h)") }
        let json = "{" + fields.joined(separator: ",") + "}"
        return try! JSONDecoder().decode(KBEntry.self, from: Data(json.utf8))
    }

    func testEachActIsTheCLIVerbOfTheSameName() {
        // Not a re-implementation: the app spends a process and the ledger decides. A verb
        // spelled wrong here is an action that silently does nothing on a real machine.
        XCTAssertEqual(KnowledgeAct.promote(id: "pitfalls/x", audience: "").argv(reason: "charter-level"),
                       ["knowledge", "promote", "pitfalls/x", "--why", "charter-level"])
        XCTAssertEqual(KnowledgeAct.promote(id: "pitfalls/x", audience: "machine").argv(reason: "charter-level"),
                       ["knowledge", "promote", "pitfalls/x", "--why", "charter-level", "--for", "machine"])
        XCTAssertEqual(KnowledgeAct.promote(id: "pitfalls/x", audience: "repo:/w/api").argv(reason: "r"),
                       ["knowledge", "promote", "pitfalls/x", "--why", "r", "--for", "repo:/w/api"])
        XCTAssertEqual(KnowledgeAct.land(id: "pitfalls/x").argv(reason: "PR #888"),
                       ["knowledge", "land", "pitfalls/x", "--ref", "PR #888"])
        // carry is land WITHOUT a ref: gtmux writes it where the audience reads.
        XCTAssertEqual(KnowledgeAct.carry(id: "pitfalls/x").argv(reason: ""),
                       ["knowledge", "land", "pitfalls/x"])
        XCTAssertEqual(KnowledgeAct.withdraw(id: "pitfalls/x").argv(reason: "only here"),
                       ["knowledge", "withdraw", "pitfalls/x", "--why", "only here"])
        XCTAssertEqual(KnowledgeAct.retire(id: "pitfalls/x").argv(reason: "network was fixed"),
                       ["knowledge", "retire", "pitfalls/x", "--why", "network was fixed"])
        // A candidate is a spool line, not a ledger entry: dismiss names the KEY.
        XCTAssertEqual(KnowledgeAct.dismiss(key: "pitfalls/x").argv(reason: "already covered"),
                       ["knowledge", "dismiss", "--capture", "pitfalls/x", "--why", "already covered"])
        // feedback is a browser, not a process.
        XCTAssertEqual(KnowledgeAct.feedback(url: "https://x").argv(reason: ""), [])
        XCTAssertFalse(KnowledgeAct.feedback(url: "https://x").runsCLI)
    }

    func testTheReasonIsNeverDroppedOrReordered() {
        // `land` takes --ref and the others take --why. Sending a ref as a why would be
        // accepted by the CLI and record the wrong fact.
        for act in [KnowledgeAct.promote(id: "a", audience: ""), .land(id: "a"), .withdraw(id: "a"),
                    .retire(id: "a"), .dismiss(key: "a")] {
            let argv = act.argv(reason: "REASON")
            XCTAssertTrue(act.needsReason)
            XCTAssertEqual(argv.last, "REASON", "\(act) dropped or misplaced the reason")
            XCTAssertEqual(argv[argv.count - 2], act.copy(L10n.shared).field,
                           "\(act) used a flag its copy does not name")
        }
        // With an audience, --for rides AFTER the reason so the reason's position is stable.
        let argv = KnowledgeAct.promote(id: "a", audience: "hq").argv(reason: "REASON")
        XCTAssertEqual(argv[4], "REASON")
        XCTAssertEqual(Array(argv[5...]), ["--for", "hq"])
        XCTAssertFalse(KnowledgeAct.carry(id: "a").needsReason)
    }

    func testAPendingPromotionOffersTheExitItsAudienceHas() {
        // gtmux carries hq / machine / repo; a person opens the issue for everyone; a
        // promotion with no audience can only be landed by hand or withdrawn.
        for aud in ["hq", "machine", "repo"] {
            XCTAssertEqual(knowledgeActs(for: entry("a", promoted: 100, audience: aud)),
                           [.carry(id: "a"), .withdraw(id: "a"), .retire(id: "a")], aud)
        }
        XCTAssertEqual(knowledgeActs(for: entry("e", promoted: 100, audience: "everyone", issueUrl: "https://gh/new")),
                       [.feedback(url: "https://gh/new"), .land(id: "e"), .withdraw(id: "e"), .retire(id: "e")])
        // No URL (an older serve) → no dead button.
        XCTAssertEqual(knowledgeActs(for: entry("e2", promoted: 100, audience: "everyone")),
                       [.land(id: "e2"), .withdraw(id: "e2"), .retire(id: "e2")])
        XCTAssertEqual(knowledgeActs(for: entry("n", promoted: 100)),
                       [.land(id: "n"), .withdraw(id: "n"), .retire(id: "n")])

        let ordinary = knowledgeActs(for: entry("b"))
        XCTAssertEqual(ordinary, [.promote(id: "b", audience: ""), .retire(id: "b")])

        // A LANDED promotion is done, so the entry is ordinary again and can be re-promoted.
        let landed = knowledgeActs(for: entry("c", promoted: 100, landed: 200, audience: "machine"))
        XCTAssertEqual(landed, [.promote(id: "c", audience: ""), .retire(id: "c")])
    }

    func testOnlyTheTwoThatTakeSomethingAwayReadAsRemovals() {
        XCTAssertTrue(KnowledgeAct.retire(id: "a").removes)
        XCTAssertTrue(KnowledgeAct.dismiss(key: "a").removes)
        XCTAssertFalse(KnowledgeAct.promote(id: "a", audience: "").removes)
        XCTAssertFalse(KnowledgeAct.land(id: "a").removes)
        XCTAssertFalse(KnowledgeAct.withdraw(id: "a").removes) // the entry stays
        XCTAssertFalse(KnowledgeAct.carry(id: "a").removes)
    }

    func testTheAxesReadAsOneLine() {
        let l10n = L10n.shared
        let e = entry("a", audience: "machine", kind: "pitfalls", assumed: true, provenance: "mined", hits: 6)
        let line = e.axesLine(l10n)
        XCTAssertTrue(line.hasPrefix("pitfalls?"), line)
        XCTAssertTrue(line.contains("mined ×6"), line)
        XCTAssertTrue(line.contains(audienceShort("machine", l10n)), line)
        XCTAssertEqual(entry("b").axesLine(l10n), "", "a legacy row with no axes says nothing")
    }

    func testEveryActHasCopyInBothLanguages() {
        // A blank button is an action a reader cannot find; a missing hint is a reason
        // field with no explanation of what it becomes. carry and feedback take no text,
        // so their placeholder may be empty — the field is what must match.
        let l10n = L10n.shared
        for act in [KnowledgeAct.promote(id: "a", audience: ""), .land(id: "a"), .carry(id: "a"), .withdraw(id: "a"),
                    .retire(id: "a"), .dismiss(key: "a"), .feedback(url: "u")] {
            let c = act.copy(l10n)
            for (name, text) in [("button", c.button), ("title", c.title), ("hint", c.hint)] {
                XCTAssertFalse(text.trimmingCharacters(in: .whitespaces).isEmpty, "\(act) has no \(name)")
            }
            XCTAssertEqual(c.field.isEmpty, !act.needsReason, "\(act): a field iff a reason is needed")
            if act.needsReason {
                XCTAssertFalse(c.placeholder.isEmpty, "\(act) has no placeholder")
            }
        }
        for a in KnowledgeAudience.allCases {
            XCTAssertFalse(a.word(l10n).isEmpty)
        }
    }
}
