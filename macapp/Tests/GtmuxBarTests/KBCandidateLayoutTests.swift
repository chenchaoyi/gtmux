import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

/// Render the production views at the actual sidebar width. Model tests cannot
/// catch raw prose drawing beyond a list row or losing its tail in the detail.
final class KBCandidateLayoutTests: XCTestCase {
    private let longLesson = String(repeating: "需要明确测试能力的范围、质量要求和实际验证方式。", count: 24)
        + "\n/Users/example/gtmux/uploads/evidence.png"

    private func lead(_ lesson: String) -> KBCandidateGroup {
        KBCandidateGroup(key: "corrections/quality-requirements", topic: "corrections", lesson: lesson,
                         at: 1, count: 2, family: 1)
    }

    @MainActor private func render<V: View>(_ view: V, width: CGFloat) throws -> NSImage {
        let renderer = ImageRenderer(content: view.frame(width: width))
        renderer.scale = 1
        return try XCTUnwrap(renderer.nsImage)
    }

    @MainActor private func document(_ lesson: String) -> some View {
        KBCandidateDetail(candidate: lead(lesson), l10n: L10n.shared, p: Theme.Palette.of(.light)) {
            Button(L10n.shared.tr("Dismiss…", "驳回…")) {}
        }.content
    }

    @MainActor func testLongMultilineLeadStaysInsideACompactSidebarRow() throws {
        let row = KBCandidateRow(candidate: lead(longLesson), selected: false,
                                 p: Theme.Palette.of(.light), select: {})
        let image = try render(row, width: 292)
        XCTAssertEqual(image.size.width, 292, accuracy: 1)
        XCTAssertGreaterThan(image.size.height, 30)
        XCTAssertLessThanOrEqual(image.size.height, 70,
            "A long capture must remain a two-line preview, not displace or overlap following rows")
    }

    @MainActor func testFullLeadGrowsTheDetailDocumentRatherThanClippingIt() throws {
        for width: CGFloat in [378, 578] {
            let short = try render(document("A short lead"), width: width)
            let long = try render(document(longLesson), width: width)
            XCTAssertEqual(long.size.width, width, accuracy: 1)
            XCTAssertGreaterThan(long.size.height, short.size.height + 100,
                "The full original text must contribute to scrollable height")
        }
    }

    @MainActor func testTheLastLineIsActuallyDrawnInTheDetail() throws {
        let original = try render(document(longLesson), width: 578)
        let changedTail = try render(document(longLesson.replacingOccurrences(
            of: "evidence.png", with: "different.png")), width: 578)
        XCTAssertNotEqual(original.tiffRepresentation, changedTail.tiffRepresentation,
            "Changing only the last evidence path must change the drawing, not just the model")
    }

    @MainActor func testCandidateSelectionHasAVisibleMark() throws {
        let candidate = lead("A lead to review")
        let p = Theme.Palette.of(.light)
        let selected = try render(KBCandidateRow(candidate: candidate, selected: true, p: p, select: {}), width: 292)
        let ordinary = try render(KBCandidateRow(candidate: candidate, selected: false, p: p, select: {}), width: 292)
        XCTAssertNotEqual(selected.tiffRepresentation, ordinary.tiffRepresentation)
    }

    @MainActor func testChineseAndEnglishDocumentsFitAndProduceReviewEvidence() throws {
        let originalMode = L10n.shared.mode
        defer { L10n.shared.mode = originalMode }
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            let p = Theme.Palette.of(.light)
            let sidebar = VStack(spacing: 0) {
                KBCandidateRow(candidate: lead(longLesson), selected: true, p: p, select: {})
                KBCandidateRow(candidate: KBCandidateGroup(key: "second", topic: "corrections",
                    lesson: "doctor fix installs the menu bar again", at: 2, count: 1, family: 0),
                    selected: false, p: p, select: {})
                Spacer(minLength: 0)
            }.frame(width: 292)
            let view = HStack(alignment: .top, spacing: 0) {
                sidebar
                Divider()
                document(longLesson)
            }.frame(minHeight: 540).background(p.bg)
            let image = try render(view, width: 900)
            XCTAssertEqual(image.size.width, 900, accuracy: 1)
            if let directory = ProcessInfo.processInfo.environment["GTMUX_UI_EVIDENCE_DIR"] {
                let bitmap = try XCTUnwrap(NSBitmapImageRep(data: try XCTUnwrap(image.tiffRepresentation)))
                let data = try XCTUnwrap(bitmap.representation(using: .png, properties: [:]))
                try data.write(to: URL(fileURLWithPath: directory)
                    .appendingPathComponent("kb-candidate-detail-\(mode.rawValue).png"))
            }
        }
    }
}
