import XCTest
@testable import GtmuxBar

// These mirror mobileapp/src/screens/boardSections.test.ts case for case. Two surfaces
// of one product must not disagree about what the board's structure IS, and the way that
// drift starts is one side quietly answering a case the other never asked about.
final class BoardOutlineTests: XCTestCase {
    func testSplitsAtH2AndKeepsTheHeadingText() {
        let s = BoardOutline.parse("## One\nalpha\n\n## Two\nbeta\n")
        XCTAssertEqual(s.map(\.title), ["One", "Two"])
        XCTAssertEqual(s[0].body, "alpha")
        XCTAssertEqual(s[1].body, "beta")
    }

    func testKeepsContentBeforeTheFirstHeading() {
        let s = BoardOutline.parse("intro line\n\n## One\nalpha\n")
        XCTAssertEqual(s[0].title, "")
        XCTAssertEqual(s[0].body, "intro line")
        XCTAssertEqual(s[1].title, "One")
    }

    func testDoesNotSplitInsideAFence() {
        // The board quotes shell and JSON constantly. Splitting on a comment would cut a
        // section in half at a line that is not a heading at all.
        let md = "## One\n```sh\n## not a heading\necho hi\n```\nstill section one\n\n## Two\nb"
        let s = BoardOutline.parse(md)
        XCTAssertEqual(s.map(\.title), ["One", "Two"])
        XCTAssertTrue(s[0].body.contains("## not a heading"))
        XCTAssertTrue(s[0].body.contains("still section one"))
    }

    func testNestsH3EntriesUnderTheirSection() {
        // The board is TWO `##` sections holding several `###` entries, so stopping at
        // `##` gives a reader two rows: a wall when open, a screen of void when shut.
        let s = BoardOutline.parse("## One\nlead-in\n### A\naaa\n### B\nbbb\n")
        XCTAssertEqual(s.count, 1)
        XCTAssertEqual(s[0].body, "lead-in") // the section's OWN text, ending where A begins
        XCTAssertEqual(s[0].children.map(\.title), ["A", "B"])
        XCTAssertEqual(s[0].children[0].body, "aaa")
        XCTAssertEqual(s[0].children[1].body, "bbb")
    }

    func testDoesNotRepeatAChildBodyInsideItsParent() {
        let s = BoardOutline.parse("## One\n### A\naaa\n")
        XCTAssertEqual(s[0].body, "")
        XCTAssertEqual(s[0].children[0].body, "aaa")
    }

    func testEveryRowHasADistinctID() {
        let s = BoardOutline.parse("## Same\n### Same\na\n\n## Same\n### Same\nb")
        let ids = s.map(\.id) + s.flatMap { $0.children.map(\.id) }
        XCTAssertEqual(Set(ids).count, ids.count)
    }

    func testAnOrphanH3KeepsItsText() {
        let s = BoardOutline.parse("### orphan\ntext")
        XCTAssertEqual(s[0].title, "")
        XCTAssertTrue(s[0].body.contains("### orphan"))
    }

    func testDoesNotNestOnAnH3InsideAFence() {
        let s = BoardOutline.parse("## One\n```md\n### not an entry\n```\ntail")
        XCTAssertTrue(s[0].children.isEmpty)
        XCTAssertTrue(s[0].body.contains("### not an entry"))
    }

    func testPreservesTheAuthorOrder() {
        // The board is not chronological: a pinned handoff sits at the top and the newest
        // progress is appended at the bottom. Re-ordering would misrepresent both.
        let s = BoardOutline.parse("## 2026-08-13 handoff\na\n\n## 2026-08-12\nb\n\n## 2026-08-13 later\nc")
        XCTAssertEqual(s.map(\.title), ["2026-08-13 handoff", "2026-08-12", "2026-08-13 later"])
    }

    func testEmptyBoardIsEmpty() {
        XCTAssertTrue(BoardOutline.parse("").isEmpty)
        XCTAssertTrue(BoardOutline.parse("\n\n  \n").isEmpty)
    }

    func testDropsTheDocumentTitleFromThePreambleOnly() {
        // The reader has its own header saying "Situation board / 态势板". The file's `# `
        // title rendered directly under it as a second, larger one.
        let a = BoardOutline.parse("# gtmux HQ — 态势板\n\nYour durable posture.\n\n## ① 现状\n\nrow")
        XCTAssertEqual(a[0].body, "Your durable posture.")
        let b = BoardOutline.parse("## ① 现状\n\n# not a document title\n\nrow")
        XCTAssertTrue(b[0].body.contains("# not a document title"))
        let c = BoardOutline.parse("# 态势板\n\n## ① 现状\n\nrow")
        XCTAssertEqual(c.map(\.title), ["① 现状"])
    }

    // MARK: the count bubble

    func testCountsATableByItsRows() {
        let table = [
            "some prose first", "",
            "| pane | loc | 在做什么 |", "|---|---|---|",
            "| `%7` | HSS:0.0 | 答了第四问 |",
            "| `%10` | HSS:1.0 | 发 changelog |",
            "| `%46` | dup:0.0 | 查重复 |", "",
            "**船数 17**",
        ].joined(separator: "\n")
        XCTAssertEqual(BoardOutline.countOwn(table), 3)
    }

    func testCountsBulletsWhenThereIsNoTable() {
        XCTAssertEqual(BoardOutline.countOwn("- one\n- two\n  continued\n- three\n"), 3)
    }

    func testProseWithNothingCountableGetsNoBubble() {
        // An honest absence beats a confident irrelevance.
        XCTAssertNil(BoardOutline.countOwn("just a paragraph\n\nand another one\n"))
        XCTAssertNil(BoardOutline.countOwn(""))
    }

    func testOwnContentWinsOverSubHeadings() {
        // 「① 现状 — 在跑的 pane」 leads with a table of panes AND carries sub-headings.
        // Counting the sub-headings turned a 13 into a 4, hiding the number the title
        // just asked about.
        let s = BoardOutline.parse("""
        ## ① 现状 — 在跑的 pane

        | pane | 在做什么 |
        |---|---|
        | %7 | a |
        | %8 | b |
        | %9 | c |

        ### 附注 A
        aaa

        ### 附注 B
        bbb
        """)
        XCTAssertEqual(BoardOutline.count(s[0]), 3, "the table's rows, not the two sub-headings")
    }

    func testASectionWithNoBodyIsCountedByItsEntries() {
        let s = BoardOutline.parse("## ② 交接记录\n\n### a\nx\n\n### b\ny\n")
        XCTAssertEqual(BoardOutline.count(s[0]), 2)
    }
}

// The stacked row's fields, mirroring mobileapp's stackRows tests: the two surfaces must
// agree on what a row shows and, in particular, on what it HIDES when closed.
final class BoardRowTests: XCTestCase {
    private let header: [[MDInline]] = [[.text("pane")], [.text("loc")], [.text("在做什么")], [.text("等你定")]]

    func testPairsCellsWithTheirHeadings() {
        let row: [[MDInline]] = [[.code("%7")], [.text("HSS:0.0")], [.text("改报告")], [.text("—")]]
        let f = stackFields(header: header, row: row)
        XCTAssertEqual(f.map(\.label), ["loc", "在做什么"], "an em-dash cell is dropped, not labelled")
        XCTAssertEqual(mdPlain(f[0].value), "HSS:0.0")
    }

    func testDropsEmptyAndDashCells() {
        // Half the board's cells are `—` or blank, and a label with nothing after it is
        // noise — the same rule the phone applies, so a row is the same height on both.
        let row: [[MDInline]] = [[.code("%9")], [.text("")], [.text("-")], [.text("  ")]]
        XCTAssertTrue(stackFields(header: header, row: row).isEmpty)
    }

    func testSubtitleIsTheFirstFieldSoRowsCanBeToldApart() {
        // A column of bare pane ids says nothing about which row is which.
        let row: [[MDInline]] = [[.code("%7")], [.text("HSS AI Workspace:0.0")], [.text("改报告")]]
        XCTAssertEqual(rowSubtitle(header: header, row: row), "HSS AI Workspace:0.0")
    }

    func testSubtitleIsEmptyWhenTheRowHasNothingButItsHead() {
        XCTAssertEqual(rowSubtitle(header: header, row: [[.code("%7")]]), "")
    }

    func testSubtitleSkipsAnEmptyLeadingCell() {
        // The first NON-EMPTY field, since an empty one is dropped before this looks.
        let row: [[MDInline]] = [[.code("%7")], [.text("—")], [.text("改报告")]]
        XCTAssertEqual(rowSubtitle(header: header, row: row), "改报告")
    }
}

// The knowledge base is 386 entries across seven topics on the real machine, two of them
// holding 177 and 162. Flat, that is not a list anyone reads.
final class KnowledgeTopicsTests: XCTestCase {
    private func entry(_ id: String, _ topic: String) -> KBEntry {
        KBEntry(id: id, topic: topic, title: id, at: 0, promotedAt: nil, landedAt: nil,
                promoteWhy: nil, promoteTarget: nil, landedRef: nil, body: nil)
    }

    func testGroupsByTopicBiggestFirst() {
        // Biggest first because that is the order the phone shows, and a reader should
        // not have to learn it twice.
        let rows = [entry("a", "pitfalls"), entry("b", "workflows"),
                    entry("c", "pitfalls"), entry("d", "pitfalls")]
        let t = knowledgeTopics(rows)
        XCTAssertEqual(t.map(\.name), ["pitfalls", "workflows"])
        XCTAssertEqual(t[0].entries.count, 3)
    }

    func testTiesBreakOnNameSoTopicsDoNotSwapBetweenPolls() {
        let rows = [entry("a", "zebra"), entry("b", "alpha")]
        XCTAssertEqual(knowledgeTopics(rows).map(\.name), ["alpha", "zebra"])
    }

    func testKeepsEveryEntry() {
        // The flat list showed all 386 on purpose: "a cap is an entry the commander
        // cannot retire". Grouping must not quietly reintroduce a cap.
        let rows = (0..<50).map { entry("e\($0)", $0 % 3 == 0 ? "pitfalls" : "best-practices") }
        XCTAssertEqual(knowledgeTopics(rows).reduce(0) { $0 + $1.entries.count }, 50)
    }

    func testEmptyIsEmpty() {
        XCTAssertTrue(knowledgeTopics([]).isEmpty)
    }
    /// "Newest" is a VIEW over the same entries, not a bucket beside the topics.
    ///
    /// Both surfaces show a capped recent list above the folded topics, and a reader
    /// looking at "396 entries" over a list of twelve asked whether the recent ones were
    /// in a topic at all (2026-09-07). They are: every entry is grouped, so the topic
    /// counts add up to the whole base and nothing is reachable only through "newest".
    func testEveryEntryIsInATopicSoNewestIsOnlyAView() {
        var rows: [KBEntry] = []
        for i in 0..<40 {
            rows.append(self.entry("t\(i % 4)/\(i)", "t\(i % 4)"))
        }
        let topics = knowledgeTopics(rows)
        XCTAssertEqual(topics.reduce(0) { $0 + $1.entries.count }, rows.count,
                       "an entry outside every topic would be reachable only through the capped recent list")
        let grouped = Set(topics.flatMap { $0.entries }.map { $0.id })
        for e in rows {
            XCTAssertTrue(grouped.contains(e.id), "\(e.id) is in no topic")
        }
    }

    /// Find, and the rule it shares with the phone.
    ///
    /// 396 entries across 7 topics, and the only way in was knowing which topic holds the
    /// one you want (2026-09-09). The matcher is deliberately the same shape as the
    /// phone's `matchEntries` — one query has to behave the same on both, or the base
    /// feels like two different bases.
    func testFindMatchesTitleIdAndTopic() {
        let rows = [entry("pitfalls/ps-rss", "pitfalls"),
                    entry("corrections/no-link", "corrections")]
        XCTAssertEqual(matchKB(rows, "ps-rss").map(\.id), ["pitfalls/ps-rss"])
        XCTAssertEqual(matchKB(rows, "corrections").map(\.id), ["corrections/no-link"])
    }

    func testFindIgnoresCase() {
        let rows = [entry("pitfalls/PS-RSS", "pitfalls")]
        XCTAssertEqual(matchKB(rows, "ps-rss").count, 1)
        XCTAssertEqual(matchKB(rows, "PITFALLS").count, 1)
    }

    func testEveryTermMustMatchSoTwoWordsNarrow() {
        let rows = [entry("pitfalls/ps-rss", "pitfalls"), entry("workflows/tag", "workflows")]
        XCTAssertEqual(matchKB(rows, "pitfalls ps").count, 1)
        XCTAssertEqual(matchKB(rows, "pitfalls tag").count, 0, "two terms must narrow, not widen")
    }

    func testAnEmptyQueryMatchesNothingSoTheCallerShowsTheIndex() {
        let rows = [entry("pitfalls/x", "pitfalls")]
        XCTAssertTrue(matchKB(rows, "").isEmpty)
        XCTAssertTrue(matchKB(rows, "   ").isEmpty)
    }

    /// How fresh the board is — the Mac said nothing, while the phone has said it all along.
    ///
    /// Tested at the EDGES of each bucket, not one convenient point inside it: a boundary
    /// off by a second is exactly the kind of thing that reads fine in a screenshot.
    func testBoardAgeBuckets() {
        XCTAssertEqual(boardAgeText(0, zh: false), "updated just now")
        XCTAssertEqual(boardAgeText(59, zh: false), "updated just now")
        XCTAssertEqual(boardAgeText(60, zh: false), "updated 1m ago")
        XCTAssertEqual(boardAgeText(3599, zh: false), "updated 59m ago")
        XCTAssertEqual(boardAgeText(3600, zh: false), "updated 1h ago")
        XCTAssertEqual(boardAgeText(48 * 3600 - 1, zh: false), "updated 47h ago")
        XCTAssertEqual(boardAgeText(48 * 3600, zh: false), "updated 2d ago")
    }

    func testBoardAgeSpeaksChineseNaturally() {
        // Not a word-for-word translation of the English: Chinese puts the age first.
        XCTAssertEqual(boardAgeText(0, zh: true), "刚刚更新")
        XCTAssertEqual(boardAgeText(600, zh: true), "10 分钟前更新")
        XCTAssertEqual(boardAgeText(7200, zh: true), "2 小时前更新")
        XCTAssertEqual(boardAgeText(3 * 86400, zh: true), "3 天前更新")
    }
}

// The menu bar was the only surface that could not export the supervisor's memory: the
// CLI could, the phone could keep a copy, and the Mac app — running ON the machine where
// the board and the base actually live, and whose whole job is reading them — could not.
final class HQMemoryStateTests: XCTestCase {
    func testSizeReadsAsASize() {
        // The row says the SIZE, not a tick. "Exported" and "6 MB of irreplaceable notes
        // are exported" are different sentences, and only the second says what a loss
        // would cost. Same shape as the CLI's, so the two surfaces agree.
        XCTAssertEqual(HQMemoryState(bytes: 6_275_975).sizeText, "6.0 MB")
        XCTAssertEqual(HQMemoryState(bytes: 23_579).sizeText, "23 KB")
        XCTAssertEqual(HQMemoryState(bytes: 12).sizeText, "12 B")
    }

    func testAMachineWithNoSupervisorIsNotAnError() {
        // A fresh install has no HQ home. The bar says so and the button is disabled;
        // nothing about that is a failure state.
        let s = HQMemoryState()
        XCTAssertFalse(s.exists)
        XCTAssertEqual(s.snapshots, 0)
        XCTAssertEqual(s.offMachine, "")
    }


}

/// A board cell written by a machine.
///
/// HQ writes the situation board, and one cell there was measured at ~1,180 characters —
/// a single semicolon-joined investigation log that arrives as a wall of text
/// (2026-09-08). Both readers clamp a paragraph past the same budget, so the same board
/// reads the same way on the Mac and the phone.
final class ProseClampTests: XCTestCase {
    func testTheBudgetMatchesThePhone() {
        XCTAssertEqual(MDProseClampChars, 220, "the two surfaces would fold the same board differently")
    }

    func testLengthIsWhatAReaderFacesNotTheMarkup() {
        // Bold and code spans are text on screen; a length that ignored them would let a
        // wall through by writing it in bold.
        let spans: [MDInline] = [.text("船数 18"), .bold("(09-08"), .code("seq 35078"), .link("kb-entry")]
        XCTAssertEqual(mdPlainLength(spans), "船数 18".count + "(09-08".count + "seq 35078".count + "kb-entry".count)
    }

    func testAnOrdinarySentenceIsNotAWall() {
        XCTAssertLessThan(mdPlainLength([.text("HQ 已接管,舰队一切正常。")]), MDProseClampChars)
    }

    func testTheMeasuredCellWouldHaveBeenFolded() {
        XCTAssertGreaterThan(mdPlainLength([.text(String(repeating: "态", count: 1180))]), MDProseClampChars)
    }
}
