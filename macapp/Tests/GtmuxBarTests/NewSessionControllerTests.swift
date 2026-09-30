import AppKit
import XCTest
@testable import GtmuxBar

final class NewSessionControllerTests: XCTestCase {
    func testBlankNameLetsTmuxChoose() {
        XCTAssertEqual(NewSessionController.arguments(for: ""), ["new"])
        XCTAssertEqual(NewSessionController.arguments(for: " \n\t "), ["new"])
    }

    func testNamedSessionPassesOneTrimmedArgument() {
        XCTAssertEqual(NewSessionController.arguments(for: "  My project  "), ["new", "My project"])
        XCTAssertEqual(NewSessionController.arguments(for: "中文 项目"), ["new", "中文 项目"])
    }

    func testPromptOpensWithTheNameFieldReadyToType() {
        let controller = NewSessionController()
        let l10n = L10n.shared
        controller.show(l10n: l10n) { _ in XCTFail("Opening must not create a session") }
        defer {
            NSApp.windows.first(where: { $0.title == l10n.tr("New session", "新建会话") })?.close()
        }
        guard let window = NSApp.windows.first(where: { $0.title == l10n.tr("New session", "新建会话") }),
              let field = window.contentView?.subviews.compactMap({ $0 as? NSTextField })
                .first(where: { $0.accessibilityIdentifier() == "new-session-name" }) else {
            return XCTFail("The name field should be visible in the new-session window")
        }
        XCTAssertNotNil(field.currentEditor(), "The insertion point should be in the name field")
        XCTAssertTrue(window.firstResponder === field.currentEditor())
    }
}
