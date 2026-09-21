import AppKit
import SwiftUI
import XCTest
@testable import GtmuxBar

/// The export sheet has to HOLD.
///
/// It did not. The field label sat in a fixed 64pt box beside its field, "Passphrase"
/// does not fit that at 12pt, and it came out as "Passphras / e" — and no single width
/// could have held both it and 口令. The primary button read "Choose where and exp…" at
/// 440 wide, the warning wrapped to three lines pressed against the buttons, and the
/// strength reading shared a row with a display toggle.
///
/// The labels sit above their fields now, so the width question stops existing, and these
/// assert the properties that broke: the sheet holds its width whatever the state or the
/// language, and the strength line keeps its height when it has nothing to say, so the
/// buttons never move under the pointer while you type.
final class ExportSheetLayoutTests: XCTestCase {
    @MainActor private func size(_ setup: (HQExportFlow) -> Void) -> CGSize? {
        let flow = HQExportFlow()
        flow.remembered = false
        flow.editing = false
        setup(flow)
        let r = ImageRenderer(content: HQExportSheet(l10n: L10n.shared, flow: flow, onClose: {}))
        r.scale = 1
        return r.nsImage?.size
    }

    @MainActor func testTheSheetHoldsItsWidthInEveryState() {
        let states: [(String, (HQExportFlow) -> Void)] = [
            ("empty", { _ in }),
            ("good", { $0.passphrase = "correcthorsebattery"; $0.confirm = "correcthorsebattery" }),
            ("mismatch", { $0.passphrase = "correcthorse"; $0.confirm = "correcthors" }),
            ("short", { $0.passphrase = "abc" }),
            ("revealed", { $0.reveal = true; $0.passphrase = "correcthorsebattery" }),
            ("keychain", { $0.remembered = true; $0.passphrase = "correcthorsebattery" }),
        ]
        for (name, setup) in states {
            guard let s = size(setup) else { XCTFail("\(name) did not render"); continue }
            XCTAssertEqual(s.width, 480, accuracy: 0.5, "\(name) pushed the sheet off 480")
        }
    }

    /// Chinese labels and copy must not change the width either — a side label could not
    /// serve both languages, which is the whole reason the labels moved.
    @MainActor func testChineseDoesNotResizeIt() {
        let en = size { $0.passphrase = "correcthorsebattery"; $0.confirm = "correcthorsebattery" }
        L10n.shared.mode = .zh
        defer { L10n.shared.mode = .en }
        let zh = size { $0.passphrase = "correcthorsebattery"; $0.confirm = "correcthorsebattery" }
        XCTAssertEqual(en?.width, zh?.width)
    }

    /// The strength line is empty until you type and says something afterwards. If it
    /// collapsed, everything below it — including the button you are reaching for — would
    /// move the moment you touched the keyboard.
    @MainActor func testTheButtonsDoNotJumpWhileYouType() {
        let quiet = size { _ in }
        let speaking = size { $0.passphrase = "correcthorsebattery"; $0.confirm = "correcthorsebattery" }
        guard let a = quiet?.height, let b = speaking?.height else { XCTFail("no render"); return }
        XCTAssertEqual(a, b, accuracy: 0.5, "the sheet changed height when the hint appeared")
    }

    /// The keychain state is a line, not a form, so it is materially shorter — the check
    /// that the two states really are different shapes rather than the same one twice.
    @MainActor func testTheRememberedStateIsALineNotAForm() {
        let form = size { $0.passphrase = "correcthorsebattery"; $0.confirm = "correcthorsebattery" }
        let line = size { $0.remembered = true; $0.passphrase = "correcthorsebattery" }
        guard let f = form?.height, let l = line?.height else { XCTFail("no render"); return }
        XCTAssertLessThan(l, f - 60, "the remembered state is not collapsing to a line")
    }
}
