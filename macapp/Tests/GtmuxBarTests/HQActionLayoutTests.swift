import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

final class HQActionLayoutTests: XCTestCase {
    @MainActor func testNarrowActionsStackInsteadOfBreakingLabels() throws {
        let language = L10n.shared.mode
        defer { L10n.shared.mode = language }
        for mode in [LangMode.en, .zh] {
            L10n.shared.mode = mode
            let actions: [KnowledgeAct] = [.feedback(url: "https://example.invalid"), .land(id: "synthetic"), .withdraw(id: "synthetic"), .retire(id: "synthetic")]
            let view = HQActionLayout {
                ForEach(actions, id: \.self) { act in
                    Button(act.copy(.shared).button) { XCTFail("Layout cannot invoke an action") }
                        .font(HQTypography.caption).fixedSize()
                }
            }
            let narrow = ImageRenderer(content: view.frame(width: 300))
            let wide = ImageRenderer(content: view.frame(width: 1000))
            let narrowSize = try XCTUnwrap(narrow.nsImage).size
            let wideSize = try XCTUnwrap(wide.nsImage).size
            XCTAssertLessThanOrEqual(narrowSize.width, 301)
            XCTAssertGreaterThan(narrowSize.height, wideSize.height + 40,
                "The action bar must stack intact controls when a horizontal row cannot fit")
        }
    }
}
