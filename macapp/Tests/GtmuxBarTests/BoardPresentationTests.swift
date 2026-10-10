import XCTest
@testable import GtmuxBar

final class BoardPresentationTests: XCTestCase {
    private let table = "| pane | loc |\n|---|---|\n| `%19` | dev:0.0 |\n| `%20` | hq:0.0 |"

    func testOwnedHeadingsAreLocalizedWithoutChangingSourceOrIDs() {
        for source in ["① Now — live panes", "① 现状 — 在跑的 pane"] {
            let section = BoardOutline.parse("## \(source)\n\(table)")[0]
            let id = section.id
            XCTAssertEqual(BoardPresentation.title(section.title, zh: true), "会话概览")
            XCTAssertEqual(BoardPresentation.title(section.title, zh: false), "Sessions")
            XCTAssertEqual(section.title, source)
            XCTAssertEqual(section.id, id)
        }
        for source in ["② Handoff log — newest first", "② 交接记录 — 新的在最上面"] {
            XCTAssertEqual(BoardPresentation.title(source, zh: true), "交接记录")
            XCTAssertEqual(BoardPresentation.title(source, zh: false), "Handoff log")
        }
        let custom = "① Release 1, 2 — newest first"
        XCTAssertEqual(BoardPresentation.title(custom, zh: true), custom)
    }

    func testSessionTableAndUnlabelledNotesAreSeparateWithNoBlockLoss() {
        let section = BoardOutline.parse("## ① 现状 — 在跑的 pane\n\(table)\n\nHQ SID local-session；已核对 SessionEnd 与 SessionStart。\n\n- 磁盘检查结果\n- 保留 shell")[0]
        let groups = BoardPresentation.body(section)
        XCTAssertEqual(groups.map(\.notes), [false, true])
        XCTAssertEqual(groups.flatMap(\.blocks), Markdown.parseBlocks(section.body))
        XCTAssertEqual(BoardPresentation.count(section, children: []), 2)
        XCTAssertEqual(groups[1].blocks.count, 2)
    }

    func testLeadingProseAndExplicitHeadingsStayVisibleInAuthorOrder() {
        let section = BoardOutline.parse("## ① Now — live panes\nintro\n\n\(table)\n\nunlabelled\n\n#### Checks\nlabelled\n\n\(table)\n\nmore notes")[0]
        let groups = BoardPresentation.body(section)
        XCTAssertEqual(groups.map(\.notes), [false, true, false, true])
        XCTAssertEqual(groups.flatMap(\.blocks), Markdown.parseBlocks(section.body))
        XCTAssertEqual(groups[0].blocks.first, .paragraph([.text("intro")]))
    }

    func testFencedTableAndOtherSectionsAreNotReclassifiedAsNotes() {
        let current = BoardOutline.parse("## ① Now — live panes\n```text\n\(table)\n```\n\nparagraph")[0]
        XCTAssertEqual(BoardPresentation.body(current).map(\.notes), [false])
        let custom = BoardOutline.parse("## Custom\n\(table)\n\nparagraph")[0]
        XCTAssertEqual(BoardPresentation.body(custom).map(\.notes), [false])
        XCTAssertEqual(BoardPresentation.body(custom).flatMap(\.blocks), Markdown.parseBlocks(custom.body))
    }

    func testEmptyEntriesAreHiddenAndAttentionBodyIsDisplayedOnlyOnce() {
        let sections = BoardOutline.parse("## ① Now — live panes\n\(table)\n### Still waiting on you\n<!-- no decisions -->\n### 还等你定的\nApprove deployment\n### Empty\n\n## ② Handoff log — newest first\n### Handoff\nRecorded result")
        let ask = BoardPresentation.attention(sections)
        XCTAssertEqual(ask?.body, "Approve deployment")
        XCTAssertEqual(BoardPresentation.title(ask!.title, zh: true), "待你处理")
        let children = BoardPresentation.visible(sections[0].children, liftedID: ask?.id)
        XCTAssertTrue(children.isEmpty)
        XCTAssertEqual(BoardPresentation.count(sections[0], children: children), 2)
        XCTAssertEqual(BoardPresentation.visible(sections[1].children, liftedID: ask?.id).map(\.title), ["Handoff"])
    }

    func testLiftedTopLevelAttentionKeepsItsChildrenAccessible() {
        let sections = BoardOutline.parse("## Still waiting on you\nApprove deployment\n### Supporting evidence\nTest result\n## Custom\nmore")
        let ask = BoardPresentation.attention(sections)
        let visible = BoardPresentation.visible(sections, liftedID: ask?.id)
        XCTAssertEqual(visible.count, 2)
        XCTAssertEqual(visible[0].children[0].body, "Test result")
        let onlyAsk = BoardOutline.parse("## 还等你定的\nApprove deployment")
        XCTAssertTrue(BoardPresentation.visible(onlyAsk, liftedID: BoardPresentation.attention(onlyAsk)?.id).isEmpty)
        XCTAssertNil(BoardPresentation.attention(BoardOutline.parse("## 还等你定的\n<!-- empty -->")))
    }
}
