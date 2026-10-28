import XCTest
@testable import GtmuxBar

/// A share link gets no Terminal door. Since #1372 the serve refuses a share link a
/// terminal (a terminal would reach the whole tmux session, not only the shared panes),
/// yet both share sheets still printed `gtmux attach '<link>'` beside the QR and the
/// browser link: a command that always failed (%12, 2026-10-07). Pairing keeps its door —
/// a pair link makes the terminal one of the owner's devices.
final class GuestDeliveryTests: XCTestCase {
    @MainActor func testTheTerminalDoorIsDrawnOnlyWithACommand() {
        let l10n = L10n.shared
        let share = CodeDeliveryBlock(l10n: l10n, qrText: "https://x.dev/#code=GM4W-HCCQ",
                                      linkValue: "https://x.dev/#code=GM4W-HCCQ")
        XCTAssertFalse(share.showsTerminal, "a share link's card offered a terminal")
        let pair = CodeDeliveryBlock(l10n: l10n, qrText: "payload",
                                     linkValue: "https://x.dev/#c=ABCD",
                                     terminalValue: "gtmux attach 'https://x.dev/#c=ABCD'")
        XCTAssertTrue(pair.showsTerminal, "pairing lost its terminal door")
    }

    /// Guest views must not call the owner-only pairing builder or pass a terminal
    /// command to their delivery card. Pairing credentials are tested separately.
    func testGuestSheetsDoNotOfferAnAttachCommand() throws {
        let src = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("Sources/GtmuxBar/PairShareSheets.swift")
        let text = try String(contentsOf: src, encoding: .utf8)
        let guestStart = try XCTUnwrap(text.range(of: "struct NewShareSheet"))
        let guestViews = String(text[guestStart.lowerBound...])
        XCTAssertFalse(guestViews.contains("terminalValue:"))
        XCTAssertFalse(guestViews.contains("PairingCodeContent("))
        XCTAssertFalse(guestViews.contains("PairingDeliveryValues("))
    }
}
