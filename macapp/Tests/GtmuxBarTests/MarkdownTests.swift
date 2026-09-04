import XCTest
@testable import GtmuxBar

/// The board is written in markdown and the Mac was showing its source. These pin the
/// shapes the real board actually contains — measured on it: 1 `#`, 2 `##`, 72 `###`, one
/// 6-column table, 50 bullets, 254 lines carrying inline code, 45 with bold.
final class MarkdownTests: XCTestCase {
    func testInlineCodeAndBold() {
        XCTAssertEqual(Markdown.parseInline("旧 `%12` → 新 `%6`"),
                       [.text("旧 "), .code("%12"), .text(" → 新 "), .code("%6")])
        XCTAssertEqual(Markdown.parseInline("**船数 17**(现取)"),
                       [.bold("船数 17"), .text("(现取)")])
    }

    func testAnUnmatchedMarkerStaysLiteral() {
        // Guessing where a run ends would silently restyle the rest of the sentence.
        XCTAssertEqual(Markdown.parseInline("a ` b"), [.text("a ` b")])
        XCTAssertEqual(Markdown.parseInline("100% ** done"), [.text("100% ** done")])
    }

    func testHeadingsKeepTheirLevel() {
        let blocks = Markdown.parseBlocks("# 态势板\n\n## ① 现状\n\n### %7\n")
        XCTAssertEqual(blocks, [
            .heading(level: 1, spans: [.text("态势板")]),
            .heading(level: 2, spans: [.text("① 现状")]),
            .heading(level: 3, spans: [.text("%7")]),
        ])
    }

    func testTheBoardsTableParsesIntoHeaderAndRows() {
        let md = """
        | pane | loc | 在做什么 |
        |---|---|---|
        | `%7` | HSS:0.0 | 答了第四问 |
        | `%10` | HSS:1.0 | 发 changelog |
        """
        guard case let .table(header, rows)? = Markdown.parseBlocks(md).first else {
            return XCTFail("expected a table, got \(Markdown.parseBlocks(md))")
        }
        XCTAssertEqual(header.count, 3)
        XCTAssertEqual(rows.count, 2)
        XCTAssertEqual(rows[0][0], [.code("%7")])
        XCTAssertEqual(rows[1][2], [.text("发 changelog")])
    }

    func testPipesWithoutASeparatorAreProse() {
        // `a | b` in a sentence is not a table, and treating it as one would eat the line.
        let blocks = Markdown.parseBlocks("| this is just | a line with pipes\n")
        guard case .paragraph = blocks.first else {
            return XCTFail("expected a paragraph, got \(blocks)")
        }
    }

    func testBulletsGroupAndBreak() {
        let blocks = Markdown.parseBlocks("- one\n- two\n\npara\n\n- three\n")
        XCTAssertEqual(blocks.count, 3)
        if case let .bullets(items) = blocks[0] {
            XCTAssertEqual(items.count, 2)
        } else {
            XCTFail("first block is not a bullet list: \(blocks[0])")
        }
        if case let .bullets(items) = blocks[2] {
            XCTAssertEqual(items.count, 1, "a list after a paragraph is its own list")
        } else {
            XCTFail("third block is not a bullet list: \(blocks[2])")
        }
    }

    func testAFenceKeepsItsContentVerbatim() {
        let blocks = Markdown.parseBlocks("```sh\ngtmux hq --board\n  indented\n```\n")
        XCTAssertEqual(blocks, [.code("gtmux hq --board\n  indented")])
    }

    func testAnUnclosedFenceStillYieldsItsContent() {
        // Truncated output is exactly when a reader most wants to see what there is.
        XCTAssertEqual(Markdown.parseBlocks("```\nhalf a thing\n"), [.code("half a thing")])
    }
}
