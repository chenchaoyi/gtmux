import SwiftUI
import XCTest
@testable import GtmuxBar

final class BoardPaneReferenceTests: XCTestCase {
    private func targets(_ text: String, enabled: Bool = true) -> [String] {
        BoardPaneReference.attributed(text, enabled: enabled).runs.compactMap { run in
            run.link.flatMap(BoardPaneReference.target)
        }
    }

    func testExactTargetsInChineseEnglishInlineCodeAndTableCells() {
        for text in ["%29：需要处理；另见 %19。", "Review (%29), then %19.", "%29", "%0"] {
            let expected = text.contains("%19") ? ["%29", "%19"] : [text == "%0" ? "%0" : "%29"]
            XCTAssertEqual(targets(text), expected, text)
            XCTAssertEqual(String(BoardPaneReference.attributed(text).characters), text)
        }
        XCTAssertEqual(targets("%29 / %19 / %29"), ["%29", "%19", "%29"])
    }

    func testDoesNotInventTargetsFromNamesURLsPathsOrMalformedIDs() {
        for text in ["gtmux dev", "hq:0.0", "50% complete", "a%29", "%29abc", "%29_foo",
                     "%%29", "%01", "%29.0", "/tmp/%29/file", "https://example.com/%29",
                     "https://example.com/?p=%29", "%1234567890123456789"] {
            XCTAssertEqual(targets(text), [], text)
        }
    }

    func testKnowledgeRendererCanKeepPaneReferencesLiteral() {
        XCTAssertEqual(targets("See %29 and %19", enabled: false), [])
        XCTAssertEqual(String(BoardPaneReference.attributed("See %29", enabled: false).characters), "See %29")
    }

    func testInternalURLsAcceptOnlyOneCanonicalPaneArgument() throws {
        XCTAssertEqual(BoardPaneReference.target(try XCTUnwrap(URL(string: "gtmux-board://pane/29"))), "%29")
        XCTAssertEqual(BoardPaneReference.target(try XCTUnwrap(URL(string: "gtmux-board://pane/0"))), "%0")
        for raw in ["https://pane/29", "gtmux-board://other/29", "gtmux-board://pane/29?x=1",
                    "gtmux-board://pane/29#x", "gtmux-board://user@pane/29", "gtmux-board://pane:80/29",
                    "gtmux-board://pane/29/", "gtmux-board://pane/01", "gtmux-board://pane/%32%39",
                    "gtmux-board://pane/-19", "gtmux-board://pane/29/send", "gtmux-board://pane/"] {
            XCTAssertNil(BoardPaneReference.target(try XCTUnwrap(URL(string: raw))), raw)
        }
    }

    func testFailedFocusPreservesTheCLIReasonAndSuccessShowsNoError() {
        XCTAssertNil(BoardPaneReference.failureMessage(pane: "%29", status: 0, stderr: "notice", zh: false))
        let error = BoardPaneReference.failureMessage(pane: "%29", status: 1,
            stderr: "  pane %29 no longer exists\n", zh: false)
        XCTAssertTrue(error?.contains("%29") == true)
        XCTAssertTrue(error?.hasSuffix("pane %29 no longer exists") == true)
        XCTAssertTrue(BoardPaneReference.failureMessage(pane: "%29", status: -1,
            stderr: "", zh: true)?.contains("无法打开窗格 %29") == true)
    }

    @MainActor func testProseAndPaneRowsRenderAtNarrowAndWideWidthsInBothAppearances() throws {
        let blocks = Markdown.parseBlocks("需要处理 **%29**，另见 `%19`。\n\n| pane | loc | status |\n| --- | --- | --- |\n| %29 | design:0.0 | waiting |\n| %19 | gtmux dev:0.0 | working |")
        for scheme in [ColorScheme.light, .dark] {
            for width: CGFloat in [320, 720] {
                let p = Theme.Palette.of(scheme)
                let view = MarkdownBlocks(blocks: blocks, p: p, spacing: 9, foldRows: true, paneLinks: true)
                    .padding(14).frame(width: width).background(p.bg).environment(\.colorScheme, scheme)
                let renderer = ImageRenderer(content: view)
                let image = try XCTUnwrap(renderer.nsImage)
                XCTAssertEqual(image.size.width, width, accuracy: 1)
                XCTAssertGreaterThan(image.size.height, 60)
                XCTAssertLessThan(image.size.height, 200)
                if let directory = ProcessInfo.processInfo.environment["GTMUX_UI_EVIDENCE_DIR"] {
                    let bitmap = try XCTUnwrap(NSBitmapImageRep(data: try XCTUnwrap(image.tiffRepresentation)))
                    let data = try XCTUnwrap(bitmap.representation(using: .png, properties: [:]))
                    try data.write(to: URL(fileURLWithPath: directory)
                        .appendingPathComponent("board-pane-links-\(scheme)-\(Int(width)).png"))
                }
            }
        }
    }
}
