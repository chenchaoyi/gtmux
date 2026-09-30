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
            NSApp.windows.first(where: { $0.title == l10n.tr("New session", "新建会话") && $0.isVisible })?.close()
        }
        guard let window = NSApp.windows.first(where: { $0.title == l10n.tr("New session", "新建会话") && $0.isVisible }),
              let field = window.contentView?.subviews.compactMap({ $0 as? NSTextField })
                .first(where: { $0.accessibilityIdentifier() == "new-session-name" }) else {
            return XCTFail("The name field should be visible in the new-session window")
        }
        XCTAssertNotNil(field.currentEditor(), "The insertion point should be in the name field")
        XCTAssertTrue(window.firstResponder === field.currentEditor())
    }

    @MainActor func testCompactFormLayoutAndActionsInBothLanguagesAndAppearances() throws {
        let l10n = L10n.shared
        let oldMode = l10n.mode
        defer { l10n.mode = oldMode }
        for mode in [LangMode.en, .zh] {
            l10n.mode = mode
            for dark in [false, true] {
                let controller = NewSessionController()
                var names: [String] = []
                controller.show(l10n: l10n) { names.append($0) }
                let window = try XCTUnwrap(NSApp.windows.first { $0.title == l10n.tr("New session", "新建会话") && $0.isVisible })
                window.appearance = NSAppearance(named: dark ? .darkAqua : .aqua)
                let content = try XCTUnwrap(window.contentView)
                content.wantsLayer = true
                window.appearance?.performAsCurrentDrawingAppearance {
                    content.layer?.backgroundColor = NSColor.windowBackgroundColor.cgColor
                }
                content.layoutSubtreeIfNeeded()
                let field = try XCTUnwrap(content.subviews.compactMap { $0 as? NSTextField }.first { $0.isEditable })
                let label = try XCTUnwrap(content.subviews.compactMap { $0 as? NSTextField }.first { !$0.isEditable })
                let buttons = content.subviews.compactMap { $0 as? NSButton }
                let create = try XCTUnwrap(buttons.first { $0.accessibilityIdentifier() == "new-session-create" })
                let cancel = try XCTUnwrap(buttons.first { $0.accessibilityIdentifier() == "new-session-cancel" })
                XCTAssertEqual(content.frame.width, 380, accuracy: 1)
                XCTAssertLessThan(content.frame.height, 140, "The form must not reserve a large empty area")
                XCTAssertEqual(label.frame.midY, field.frame.midY, accuracy: 1)
                XCTAssertEqual(field.frame.minY - create.frame.maxY, 20, accuracy: 1)
                XCTAssertGreaterThan(field.frame.width, 240)
                XCTAssertEqual(field.placeholderString, l10n.tr("Automatic", "自动命名"))
                XCTAssertTrue(window.firstResponder === field.currentEditor())
                XCTAssertEqual(create.keyEquivalent, "\r")
                XCTAssertEqual(cancel.keyEquivalent, "\u{1b}")
                XCTAssertTrue(field.nextKeyView === cancel)
                let rep = try XCTUnwrap(content.bitmapImageRepForCachingDisplay(in: content.bounds))
                content.cacheDisplay(in: content.bounds, to: rep)
                let png = try XCTUnwrap(rep.representation(using: .png, properties: [:]))
                try png.write(to: URL(fileURLWithPath: "/private/tmp/gtmux-new-session-\(mode == .zh ? "zh" : "en")-\(dark ? "dark" : "light").png"))
                field.stringValue = "Project 中文"
                create.performClick(nil)
                XCTAssertEqual(names, ["Project 中文"])
                XCTAssertFalse(window.isVisible)
                controller.show(l10n: l10n) { _ in XCTFail("Cancel must not create a session") }
                let reopened = try XCTUnwrap(NSApp.windows.first { $0.title == l10n.tr("New session", "新建会话") && $0.isVisible })
                let reopenedCancel = try XCTUnwrap(reopened.contentView?.subviews.compactMap { $0 as? NSButton }.first { $0.keyEquivalent == "\u{1b}" })
                reopenedCancel.performClick(nil)
                XCTAssertFalse(reopened.isVisible)
            }
        }
    }

}
